# Kotlin compiler settings

Krit analyzes Kotlin with an embedded Kotlin 2.4 compiler (`krit-fir`, currently 2.4.21). A project pinned to an older
language or API level, or that enables an opt-in or an experimental feature,
must be analyzed under *its* semantics, not the embedded compiler's defaults:
annotation-target defaults, context parameters, explicit backing fields, `when`
guards and opt-in requirements all change what the compiler reports.

Krit therefore reads the project's effective Kotlin settings from the Gradle
model (under Bazel, the krit module's aspect writes the same model fields from
rules_kotlin's toolchain, `kotlinc_opts` and compiler plugins; see
[Bazel](integrations.md#type-aware-krit_test)), carries them per source set through the Go pipeline, applies an
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

### Scan-context compilations and module compilations

The oracle and the FIR checker pass compile the selected scan roots and their
dependency context as one compilation, not necessarily the whole repository.
Compiler settings are still merged across the full model, so only one set of
settings can apply. Krit applies language
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
this trade-off.

### Settings-group sessions (`--fir`)

With a Gradle model, the `--fir` checker pass then splits the check along
settings boundaries (#1577). It groups the scanned source sets by their exact
compile configuration: compiler arguments (language and API version, opt-ins,
progressive mode, flags), compiler plugin jars and options, and JVM target.
Every group whose arguments or plugins differ from what the merged session
applied gets one session of its own: its source sets are compiled together as
one module with the group's settings, against their dependencies' built jars,
or those dependencies' sources as context when they were not built (up to 3000
files, as for module compilations). Groups that the merged session already
compiles exactly need no session, so a model whose source sets all agree runs
one session as before.

A group whose arguments and plugins match the merged session but whose JVM
target is lower than the merged session's (the highest exported target) keeps
the merged verdicts by default (#1648). Few FIR diagnostics depend on the
target, mixed targets are common in Android builds, and each such group would
compile its sources again. `-v` names these groups so the cost can be
measured:

```
verbose: FIR settings: 1 settings groups differ from the merged session only in JVM target (merged: 17) and keep its verdicts; KRIT_FIR_SPLIT_JVM_TARGET=1 compiles each on its own: :legacy:main [(compiler defaults) jvm=11]
```

`KRIT_FIR_SPLIT_JVM_TARGET=1` gives each of them a session at its own target.
A group that declares no target compiles at the merged session's, so it never
needs one.

A group session's verdict replaces the merged one for its files when it
answers the file (a file gated by a dropped flag or plugin option becomes
authoritative) or when it gates the file on a missing opt-in, an error the
source set's real build reports and the opt-in union hid, or on an error at
the source set's own JVM target (`Using @JvmRecord is only allowed with
-jvm-target 16 or later`, `Cannot inline bytecode built with JVM target 17
...`), which the merged session's higher target hid. A group session that
gates a file for any other reason keeps the merged answer, so a session that
lacks some context never turns an authoritative file into a gated one. The
source-set recheck for duplicate declarations (see [Known limits](#known-limits))
runs after the split. Multiplatform source sets (HMPP fragments), source sets
whose unbuilt dependency context was too large, and groups that share a source
file with another group keep the merged session.

`-v` reports the session count, the grouping key and each group:

```
verbose: FIR settings: 3 sessions for 2 settings groups (grouping key: ...); merged session plus 2 compiled on their own:
verbose: FIR settings: session 2: :core:main, :other:main [(compiler defaults)]
verbose: FIR settings: session 3: :feature:main [-opt-in=core.Exp -Xcontext-sensitive-resolution]
verbose: FIR settings: rechecked 3 files with their own source set's settings in 5s: 1 gated files now authoritative, 1 now gated on a missing opt-in, 0 on their own JVM target, 0 kept the merged verdict
```

When the split does not run, `-v` says why, for example
`verbose: FIR settings: not split: no Gradle source set under the scan paths ...`.
The source-set recheck logs `verbose: FIR source sets: not rechecking ...`
the same way. Gradle records resolved paths (on macOS `/tmp` is
`/private/tmp`), so before planning, the model's project and source
directories are rewritten in the scan path's spelling when one is a symlink
of the other (#1674).

Group sessions run in the module worker JVMs (`KRIT_WORKER_JAVA`,
`KRIT_WORKER_AOT`; see [module sharding](module-sharding.md)) and are cached
by content under `.krit/module-facts/settings-groups`.
`KRIT_MODULE_SHARDS=merged` turns the split off and keeps the single merged
session. The type oracle still compiles with the merged settings.

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
the files that use generated API land in `errorFiles`, and every incremental
FIR check (on by default) has to compile those files again from source.

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

The scan-context compilation loads the **union** of every modeled source set's plugins: a
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

A module that still compiles with errors (no plugin information, or a plugin
that could not be loaded) keeps incremental checking as long as the errors are
in a few files: the binary snapshot is built from the other files, and the
files with errors have no binary, so every incremental check compiles them from
source. "A few" is the planner's stale bound: fewer than 16 files, or at most
5% of the module. Past it, or with a compiler error not tied to a file, the
module gets no snapshot and the planner says why it fell back: the
`firCheckBroker` perf entry reads `full (slice_unavailable: COMPILE_ERRORS:
compile errors in 60 files)` instead of `full (slice_unavailable)`.

## The allowlist

Only settings that change how source is analyzed are applied: the language
and API level, opt-ins, progressive mode, and these flags:

* Language features: `-Xallow-condition-implies-returns-contracts`,
  `-Xallow-contracts-on-more-functions`, `-Xallow-holdsin-contract`,
  `-Xallow-reified-type-in-catch`, `-Xallow-returns-result-of`,
  `-Xannotation-default-target=`, `-Xannotation-target-all`,
  `-Xannotations-in-metadata`, `-Xcollection-literals`,
  `-Xcompanion-blocks-and-extensions`, `-Xcontext-parameters`,
  `-Xcontext-receivers`, `-Xcontext-sensitive-resolution`,
  `-Xconsistent-data-class-copy-visibility`, `-Xdata-flow-based-exhaustiveness`,
  `-Xeager-lambda-analysis`, `-Xexplicit-backing-fields`,
  `-Xexplicit-context-arguments`, `-Xexpect-actual-classes`,
  `-Xintrinsic-const-evaluation`, `-Xlocal-type-aliases`,
  `-Xmulti-dollar-interpolation`, `-Xname-based-destructuring=`,
  `-Xnested-type-aliases`, `-Xnon-local-break-continue`,
  `-Xunrestricted-builder-inference`, `-Xwhen-guards`, and
  `-XXLanguage:+Feature` / `-XXLanguage:-Feature`.
* Java interop and nullability: `-Xenhance-type-parameter-types-to-def-not-null`,
  `-Xjspecify-annotations=`, `-Xjsr305=`, `-Xnullability-annotations=`,
  `-Xsupport-compatqual-checker-framework-annotations=`,
  `-Xtype-enhancement-improvements-strict-mode`, `-Xjvm-default=` and `-jvm-default=`.
* Dependency loading (without them the frontend reports errors the real build
  does not): `-Xallow-kotlin-package`, `-Xallow-unstable-dependencies`,
  `-Xskip-metadata-version-check`, `-Xskip-prerelease-check`.
* Diagnostics: `-Xreturn-value-checker=`.

`-Xjsr305=` and `-Xnullability-annotations=` are string arrays in the
compiler: every occurrence applies, and a later one replaces an earlier one
only for the same subject (the global state, `under-migration`, or one
`@annotation`).

The list is audited against the `compilerArgument` definitions of the
embedded compiler (`compiler/arguments` in the Kotlin repository): a flag
is applied when it enables a `LanguageFeature`, changes how Java or
dependency classes are typed or loaded, or turns a frontend check on. Flags
that only change codegen (`-Xlambdas`, `-Xsam-conversions`, `-Xstring-concat`,
assertion and inlining switches), only change warning severity
(`-Xexplicit-api`, `-Xwarning-level`, `-Xsuppress-warning`,
`-Xsuppress-version-warnings`), or are obsolete (`-Xnew-inference`,
`-Xinline-classes`) are not applied. A krit-fir test parses every
allowlisted flag with the embedded compiler, so a flag the compiler does not
know fails the build instead of being silently ignored.

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

The default scan, including `--fir` preflight, reads the model's source roots
and JVM target. Compilation context includes scanned source sets, their own
main and testFixtures sources when scanning test, androidTest, or testFixtures
sets, and transitively referenced project dependencies' main sources (including
generated roots). Test, androidTest, and testFixtures scans also include
dependencies' testFixtures roots: exported project paths do not distinguish
`testFixtures(project(...))` from ordinary project dependencies. Dependency
sources outside the scan paths are compilation context, not additional checked
files.
Unscanned test and androidTest source sets and unrelated model projects are
not added as context. Source sets explicitly covered by the scan paths remain
included, so one compile of main, test and androidTest reports a class declared
in both test and androidTest as a redeclaration, which the real build never
sees. With `--fir`, the default scan compiles the scanned source sets together
and then rechecks only the projects with a file gated on `Redeclaration` or
`Conflicting overloads`. The recheck compiles each of that project's scanned
source sets on its own: a test or androidTest set sees its project's main (a
built jar, or its sources as context) but never a sibling set. Its verdicts
replace the merged compile's for that project's gated files; files the merged
compile answered keep that answer. A project with one scanned set is rechecked
too, since the merged compile also puts its files next to other projects'
sets. A project keeps its merged verdicts when a set's unbuilt dependency
sources exceed 3000 files or one of its source files belongs to another
project too. A source set whose compile fails keeps its merged verdicts, while
the project's other sets still take theirs.

A gated file outside every exported source set is in a variant directory the
model did not export, such as `src/release` next to an exported debug variant
or another flavor's directory. Android builds compile one variant at a time,
and each build type or flavor may declare its own copy of a class, so these
duplicates are false too. The recheck compiles each such source set
(`src/<set>/kotlin` and `src/<set>/java`) on its own, with the sets its
variant would add:
- the base set of its kind: `src/main`, or `src/test` and `src/androidTest`
  for `test…` and `androidTest…` sets;
- the sets its name combines: `src/freeRelease` gets `src/free` and
  `src/release`;
- the exported set's generated sources and dependency context.

Those sets are compiled but not checked. The exported variant's own
directories (`src/debug` for a debug model) are never added. Each unit takes
its classpath, compiler settings and friends from the project's exported set of
the same kind. A project without one, or a project that keeps its merged
verdicts, keeps them for its variant directories too. At most 16 variant
sets are rechecked per scan; `--verbose` lists the others.

These rechecks approximate the variant, and a few files stay gated, now on
the error the approximation reports:
- a flavor directory such as `src/paid` is compiled without any build-type
  directory, so a reference to a class only `src/debug` and `src/release`
  declare stays unresolved;
- the classpath is the exported variant's, so a set that uses its own
  dependencies (`prodImplementation(...)` for a `src/prod` flavor) leaves
  them unresolved;
- `test…` sets see only test-side directories next to the exported variant's
  production code;
- `testFixtures` sets are not rechecked.

Exporting every variant's source sets from the Gradle plugin would remove
these limits, at the cost of resolving each variant's classpath in every
build.
The recheck reads the same Gradle model as the scan (`--gradle-model`,
`--no-gradle-model`) and keeps any configured `oracle.classpath` or
`CLASSPATH` entries. It replaces FIR check verdicts only; the type oracle's facts still
come from the merged compile. `--verbose` reports it as a `FIR source sets:`
line. Rechecked sets are cached by content in the project state directory
(`.krit/module-facts/source-sets`), so only edited sets recompile; `--no-cache`
uses a temporary directory instead.

`KRIT_MODULE_SHARDS` selects the compile:

| Value | Compile |
|---|---|
| unset | Scanned source sets together; projects with a cross-source-set duplicate declaration are rechecked one source set at a time. |
| `merged` | Scanned source sets together, with no recheck. |
| `isolate` | Every scanned source set on its own, when a project has several scanned sets and every set has its full dependency context; otherwise the default. |
| a number or `auto` | Module sharding (see [module sharding](module-sharding.md)). |

Compiling every set on its own is not the default because it costs much more
than the merged compile. In cold `--fir` scans of a small public multi-module
project, `isolate` took clearly longer and used several times the CPU time and
peak memory of the process tree, since each set compiles its project's main
sources again on one of several JVMs. On a large Android project it also used
several times the CPU time, and the isolated run left more files without facts
than the module-sharding coverage limit allows, so Krit discarded it and ran
the merged compile as well. The recheck pays the isolated cost only for the
projects that need it.

Missing projects or roots cannot be supplied from `projectDeps`
alone; their source sets must be present in the exported model.
Source-root matching resolves symlinks on both sides, falling back to absolute
paths when resolution fails. If no source set matches any scan path, Krit falls
back to all model source roots and reports that fallback with `--verbose`.

* The legacy per-file entries are not keyed by classpath or JVM target (tracked
  in issue #827); only the store scope is.
* The language server's workspace index uses the resident `krit-fir` oracle
  daemon, which is started once per jar and source roots and does not receive
  the settings; a scan does.
* Per-source-set `languageSettings` declared only on a Kotlin Multiplatform
  source set (not on the compilation's `compilerOptions`) are not read.
* The oracle still compiles the selected roots and dependency context as one
  unit; scoped scans do not compile unrelated model projects unless no source
  set matches. The default scan uses per-source-set requests
  (`ModuleSpecsFromModel`) only for the duplicate-declaration recheck.
