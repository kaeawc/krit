# Kotlin compiler settings

Krit analyzes Kotlin with an embedded Kotlin 2.4 compiler (`krit-fir`). A project pinned to an older
language or API level, or that enables an opt-in or an experimental feature,
must be analyzed under *its* semantics, not the embedded compiler's defaults:
annotation-target defaults, context parameters, explicit backing fields, `when`
guards and opt-in requirements all change what the compiler reports.

Krit therefore reads the project's effective Kotlin settings from the Gradle
model, carries them per source set through the Go pipeline, applies an
allowlisted subset in `krit-fir`, and folds them into every cache identity.

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
| `compilerPluginClasspath` | The compile task's compiler plugin jars (`pluginClasspath`), in order. |
| `compilerPluginOptions` | Its plugin options (`pluginOptions`) as `plugin:<id>:<key>=<value>`, in order. |

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
compilation today, so only one set of settings can apply. Krit applies language
version, API version and progressive mode **only when every source set agrees**.
Where source sets disagree (or one has no settings), that field keeps the
embedded default and a note names the disagreeing source sets. Semantic flags
use the intersection: only what every source set enables is applied.

Opt-ins use the **sorted, deduplicated union**: every marker enabled by any
source set is applied, even when another source set has no opt-ins. Opt-ins
only suppress experimental-API diagnostics; the intersection can falsely
report "This API is experimental" for code whose source set already enables
the required marker. The union avoids those false errors, but can hide a real
missing opt-in in another source set. A note reports differing opt-ins and
this trade-off; the whole-repo compilation cannot validate each source set's
opt-in requirements independently.

For per-module `analyzeModules` requests, `ModuleSpecsFromModel` builds one
`ModuleSpec` per source set whose `compilerArgs` hold that source set's own
settings, and `krit-fir` compiles each module under only its own arguments.
Changing one module's settings recompiles only that module. (A module that
falls back to compiling an upstream module's sources from source uses the
requesting module's settings.)

## Compiler plugins

Compiler plugins (kotlinx.serialization, Compose, Parcelize, all-open, no-arg,
Metro, ...) generate declarations the frontend resolves against, such as
`Foo.serializer()`. Without them a module that uses one compiles with errors:
the files that use generated API land in `errorFiles`, and a module with
compiler errors gets no binary snapshot, so incremental FIR checking
(on by default) never engages on it.

`krit-fir` therefore loads the project's plugins next to its own. They are a
krit-owned field (`KotlinSettings.Plugins`), never compiler arguments: `-Xplugin`
and `-P` stay protected. Per source set Krit takes the compile task's
`compilerPluginClasspath` and `compilerPluginOptions` and:

* leaves out the Kotlin Gradle plugin's own scripting support
  (`kotlin-scripting-*`, `kotlin-script-runtime`, and their `kotlin-stdlib`,
  `annotations` and `kotlinx-coroutines-core` dependencies), which KGP puts on
  every compilation: `krit-fir` does not compile scripts, and the embeddable
  scripting plugin cannot load into its compiler (one note says so);
* rejects, with the reason, a jar that does not exist, an option that is not
  `plugin:<id>:<key>=<value>`, and the Compose `reportsDestination` /
  `metricsDestination` options (Krit must not write build reports);
* applies no option without a plugin jar.

The whole-repo compilation loads the **union** of every source set's plugins: a
plugin only acts on declarations that use it, so loading it for the whole
repository lets the source sets that need it compile cleanly without changing
the others. One plugin artifact is never loaded in two versions (the first is
kept, with a note), and an option whose values differ between source sets is
left at the plugin default (with a note). A module compilation
(`analyzeModules`) loads only its own module's plugins.

Without a Gradle model, list the plugin jars in `krit.yml` (see
[configuration](configuration.md#oracle-classpath)):

```yaml
oracle:
  compilerPlugins:
    - /path/to/kotlin-serialization-compiler-plugin-embeddable-2.4.21.jar
  compilerPluginOptions:
    - plugin:org.jetbrains.kotlinx.serialization:disableIntrinsic=true
```

These are added to the model's plugins and filtered the same way.

`krit-fir` loads the plugins once in a probe compile (a one-line file, per
plugin set and process). A set its compiler cannot load (a plugin built
against an incompatible Kotlin compiler, or a broken jar) is left out: the
session analyzes as it did before plugins existed, logs `krit-fir: compiler
plugins not loaded, analyzing without them (...)` and returns it as a response
`warning`. Gradle exports the embeddable plugin builds (KGP compiles with the
embeddable compiler); `krit-fir` embeds the plain compiler, and the
kotlinx.serialization embeddable plugin loads into it. A plugin whose
frontend references relocated IntelliJ classes would not, and is reported by
the probe.

When a module still compiles with errors (no plugin information, or a plugin
that could not be loaded), the incremental planner says why it fell back: the
`firCheckBroker` perf entry reads `full (slice_unavailable: COMPILE_ERRORS:
compile errors in 6 files)` instead of `full (slice_unavailable)`.

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

The allowlist exists in two copies kept equal by a Go test: `internal/gradlemodel/kotlin_settings.go`
and `KotlinCompilerSettings.kt` in `tools/krit-fir`.

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
| Oracle (`krit-fir` one-shot) | `--kotlin-arg ARG`, repeated, plus the cache scope below. |
| FIR checker daemon | `--kotlin-arg` at launch and `kotlinArgs` on each check request; a request whose effective settings differ rebuilds the session. |
| `krit serve` daemon | `oracle_kotlin` in the analyze-project request. |
| `analyzeModules` | per module, in `ModuleSpec.compilerArgs`. |
| Compiler plugins (`krit-fir` only) | `--compiler-plugin JAR` / `--compiler-plugin-option plugin:<id>:<key>=<value>` at launch, `compilerPluginClasspath` / `compilerPluginOptions` on check requests and per `ModuleSpec`. A `krit-fir` jar that predates them gets the other settings and a one-time warning. |

The persistent oracle daemons (the `krit-fir` oracle and `krit serve`'s resident daemon) are keyed on the settings: a daemon is
launched with `--kotlin-arg`, reused only for identical settings, and a settings
change starts another. A `krit-fir` jar that predates
`--kotlin-arg` (for example via `KRIT_FIR_JAR`) is detected from its contents;
Krit then leaves the flags off, warns once, and the oracle keeps working under
that jar's embedded defaults. `-Xnullability-annotations=` and `-Xjsr305=` values
may contain `@`.

## Cache identities

A settings-only change (same sources, same classpath, same rules) refreshes the
affected results. The canonical settings identity
(`KotlinSettings.Identity`; empty for the embedded defaults, so existing cache
keys are unchanged) is part of:

* the oracle store scope (`oracle.NewStoreScopeWithSettings`),
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
Compiler plugins add each jar's path, size and modification time and every
option, so a plugin added, removed, upgraded or rebuilt in place is a
settings change; settings without plugins keep their identity. Inside
`krit-fir`, the incremental compile key (and with it the persisted bundle in
`.krit/fir-incremental`) also takes each loaded plugin jar's file identity,
including its entries' CRCs, and every option.

## Known limits

* On the `--fir` preflight path Krit does not (and did not before) read the
  model's source roots or JVM target; only the classpath and, now, the Kotlin
  settings come from the model there.
* The legacy per-file entries are not keyed by classpath or JVM target (tracked
  in issue #827); only the store scope is.
* The language server's workspace index uses the resident `krit-fir` oracle
  daemon, which is started once per jar and source roots and does not receive
  the settings; a scan does.
* Per-source-set `languageSettings` declared only on a Kotlin Multiplatform
  source set (not on the compilation's `compilerOptions`) are not read.
* The oracle still compiles the repository as one unit; per-module requests
  are available through `ModuleSpecsFromModel` (no production caller yet) but the default scan does not
  use them yet.
