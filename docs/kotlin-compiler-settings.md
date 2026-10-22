# Kotlin compiler settings

Krit analyzes Kotlin with an embedded Kotlin 2.4 compiler (`krit-fir`, and
`krit-types` for the Kotlin Analysis API fallback). A project pinned to an older
language or API level, or that enables an opt-in or an experimental feature,
must be analyzed under *its* semantics, not the embedded compiler's defaults:
annotation-target defaults, context parameters, explicit backing fields, `when`
guards and opt-in requirements all change what the compiler reports.

Krit therefore reads the project's effective Kotlin settings from the Gradle
model, carries them per source set through the Go pipeline, applies an
allowlisted subset in both backends, and folds them into every cache identity.

## What is exported

`./gradlew kritExportModel` writes model **schema 2**. Each source set gets a
`kotlin` object read from the compile task's `compilerOptions` (Kotlin Gradle
plugin 1.8+, falling back to the legacy `kotlinOptions`; each value is read
independently, so an older plugin still exports what it has):

| Field | Meaning |
| --- | --- |
| `languageVersion`, `apiVersion` | As configured, e.g. `"2.1"`; `""` when not set. |
| `optIn` | Opt-in markers, sorted. |
| `freeCompilerArgs` | Raw `freeCompilerArgs`, in order (later flags win). |
| `progressiveMode` | `true` when progressive mode is on. |
| `pluginVersion` | The Kotlin Gradle plugin version, used only to derive a default. |

The exporter never fills in defaults. Krit reads schema 1 and 2 models; a
schema 1 model (older plugin) has no `kotlin` objects. A source set with no Kotlin
compile task (Java-only) also exports no `kotlin` object; Krit skips such a source
set when merging, so it neither sets nor vetoes a setting. The plugin version is
read from the Kotlin Gradle plugin itself (`getKotlinPluginVersion`), falling
back to `kotlin.coreLibrariesVersion` only when that is unavailable.

## Effective settings

Per source set, Krit computes:

1. `languageVersion`: an explicit value, else the major.minor of the Kotlin
   Gradle plugin (the level Gradle compiled the project with). A
   `-language-version` in `freeCompilerArgs` wins over the DSL value, as in
   Gradle.
2. `apiVersion`: an explicit value, else the language version.
3. Opt-ins: `optIn` plus `-opt-in=` / `-Xopt-in=` in `freeCompilerArgs`.
4. Flags: the allowlisted entries of `freeCompilerArgs`, in order; repeating a
   flag or a `-XXLanguage:` feature keeps the last occurrence.

### When the model is absent or incomplete

The embedded compiler's own defaults apply, and Krit says so instead of doing
it silently:

* no model, `--no-gradle-model`, a schema 1 model, a source set without a
  Kotlin compile task, or an unknown plugin version all resolve to "embedded
  compiler defaults";
* `--verbose` prints `gradle model: kotlin compiler settings: embedded compiler
  defaults` with the reason, and `--perf` records an `oracleKotlinSettings`
  entry (`source=embedded-default|gradle-model`, `applied`, `identity`,
  `notes`, `notApplied`).

### Whole-repo compilations and module compilations

The oracle and the FIR checker pass compile the whole repository as one
compilation today, so only one set of settings can apply. Krit applies a field
**only when every source set agrees on it**; it never takes one module's value
and imposes it on another. Where source sets disagree (or one has no settings)
that field keeps the embedded default and a note names the disagreeing source
sets. Opt-ins and flags use the intersection: only what every source set
enables is applied.

For per-module `analyzeModules` requests, `ModuleSpecsFromModel` builds one
`ModuleSpec` per source set whose `compilerArgs` hold that source set's own
settings, and `krit-fir` compiles each module under only its own arguments.
Changing one module's settings recompiles only that module. (A module that
falls back to compiling an upstream module's sources from source uses the
requesting module's settings.)

## The allowlist

Only settings that change how source is analyzed are applied: the language
and API level, opt-ins, progressive mode, and these flags:

`-Xannotation-default-target=`, `-Xannotation-target-all`, `-Xcontext-parameters`,
`-Xcontext-receivers`, `-Xcontext-sensitive-resolution`,
`-Xconsistent-data-class-copy-visibility`,
`-Xdata-flow-based-exhaustiveness`, `-Xenhance-type-parameter-types-to-def-not-null`,
`-Xexplicit-backing-fields`, `-Xexpect-actual-classes`, `-Xjsr305=`,
`-Xjvm-default=`, `-Xmulti-dollar-interpolation`, `-Xnested-type-aliases`,
`-Xnon-local-break-continue`, `-Xnullability-annotations=`,
`-Xreturn-value-checker=`, `-Xtype-enhancement-improvements-strict-mode`,
`-Xunrestricted-builder-inference`, `-Xwhen-guards`, and
`-XXLanguage:+Feature` / `-XXLanguage:-Feature`.

Krit-owned arguments are **protected** and can never be set from a model or a
request: output (`-d`), classpath (`-classpath`, `-cp`), `-jvm-target`,
`-jdk-home`, `-Xjdk-release`, plugins (`-Xplugin`, `-P`), `-module-name`,
`-Xfriend-paths`, the HMPP fragment flags (`-Xfragment*`, `-Xmulti-platform`,
`-Xcommon-sources`) and the stdlib/reflect switches.

Everything else (`-Werror`, codegen flags, unknown flags, malformed values)
is **reported, not applied**: Krit lists each one with the reason in verbose
output and `--perf`, and `krit-fir` returns a `warning` in the response and
logs it on stderr.

The allowlist exists in three copies kept equal by a Go test: `internal/gradlemodel/kotlin_settings.go`,
`KotlinCompilerSettings.kt` in `tools/krit-fir` and in `tools/krit-types`.

### Language levels the embedded compiler no longer supports

The embedded Kotlin 2.4 compiler cannot analyze under a language level it has
dropped (1.9 and older; Krit's `MinSupportedKotlinLanguage` is 2.0). Such a
level is **not** silently replaced: Krit reports `languageVersion 1.9 is not
supported by the embedded compiler (2.0 or newer); its default applies`, does
not apply or identity-hash it, and the helpers independently do the same. An
`apiVersion` that is below the minimum, or above the language version, is
likewise not applied. `-progressive=false` turns progressive mode off.

## Where the settings go

| Consumer | How |
| --- | --- |
| Oracle (`krit-fir` / `krit-types` one-shot) | `--kotlin-arg ARG`, repeated, plus the cache scope below. |
| FIR checker daemon | `--kotlin-arg` at launch and `kotlinArgs` on each check request; a request whose effective settings differ rebuilds the session. |
| `krit serve` daemon | `oracle_kotlin` in the analyze-project request. |
| `analyzeModules` | per module, in `ModuleSpec.compilerArgs`. |

The persistent oracle daemons (`krit-types` and `krit-fir` oracle, their pool,
and `krit serve`'s resident daemon) are keyed on the settings: a daemon is
launched with `--kotlin-arg`, reused only for identical settings, and a settings
change starts another. A `krit-types` / `krit-fir` jar that predates
`--kotlin-arg` (for example via `KRIT_TYPES_JAR`) is detected from its contents;
Krit then leaves the flags off, warns once, and the oracle keeps working under
that jar's embedded defaults. `-Xnullability-annotations=` and `-Xjsr305=` values
may contain `@`.

## Cache identities

A settings-only change (same sources, same classpath, same rules) refreshes the
affected results. The canonical settings identity
(`KotlinSettings.Identity`; empty for the embedded defaults, so existing cache
keys are unchanged) is part of:

* the oracle store scope (`oracle.NewStoreScopeWithSettings`) for both backends,
  the facts sidecar of a cached `types.json` (so a settings-only change cannot
  reuse it), and the legacy per-file entry directory used without `.krit/store`
  (keyed by the settings identity alone, under `types-cache/scopes/<hash>` in
  the versioned root and pruned to the four most recent; no settings keep the
  existing directory unchanged). The freshness gate reads the same directory;
* the FIR check cache fingerprint and the FIR daemon registry key
  (`internal/firchecks`), and the prefetch match that lets a check ride on the
  oracle compilation;
* the `krit-fir` module input hash;
* the findings bundle fingerprint (`RunFingerprint.CompilerSettings`), so the
  bundle cache and delta planner miss when only the settings changed. A bundle
  miss names `compilerSettings` among the changed components.

Notes, rejected arguments and opt-in order do not affect the identity.

## Known limits

* On the `--fir` preflight path Krit does not (and did not before) read the
  model's source roots or JVM target; only the classpath and, now, the Kotlin
  settings come from the model there.
* The legacy per-file entries are not keyed by classpath or JVM target (tracked
  in issue #827); only the store scope is.
* The language server's workspace index uses the resident Kotlin Analysis API
  daemon, which is started once per jar and source roots and does not receive
  the settings; a scan does.
* Per-source-set `languageSettings` declared only on a Kotlin Multiplatform
  source set (not on the compilation's `compilerOptions`) are not read.
* The oracle still compiles the repository as one unit; per-module requests
  are available through `ModuleSpecsFromModel` (no production caller yet) but the default scan does not
  use them yet.
