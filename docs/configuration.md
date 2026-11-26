# Configuration

Krit reads `krit.yml` or `.krit.yml` from the project root. It accepts the same rule-set/rule/key layout used by common Kotlin analyzer YAML configs, so migration usually starts by copying an existing config and renaming it.

## Resolution order

1. `--config FILE` flag (explicit override).
2. `krit.yml` or `.krit.yml`, found by walking *upward* from the analyzed
   directory. The closest-to-source file wins; the walk stops at the
   worktree root (`.git` / `.hg` / `.jj` marker, file or directory), at
   `$HOME`, or at the filesystem root.
3. Built-in defaults.

The walk-up step means a single config at the repo root covers every
subproject without per-module wiring, while a subproject can still
override by dropping its own `krit.yml` closer to the source.

## Example

```yaml
complexity:
  LongMethod:
    active: true
    threshold: 120
  NestedBlockDepth:
    threshold: 6

naming:
  FunctionNaming:
    ignoreAnnotated: ['Composable']

style:
  MagicNumber:
    ignoreAnnotated: ['Preview']
    ignoreNumbers: ['-1', '0', '1', '2']
  ReturnCount:
    max: 4
```

## Enabling and disabling rules

```yaml
style:
  WildcardImport:
    active: true
  ForbiddenComment:
    active: false
```

Run everything at once (including opt-in rules):

```bash
krit --all-rules .
```

`--only-scope` and `--exclude-scope` select rules by scope, `file` for rules
that look at one file at a time and `project` for the rest; see
[Hermetic and build-system runs](hermetic-builds.md#sharding-rules-by-scope).

## Kotlin scripts

`.kts` files (Gradle build scripts, `*.main.kts`) are scanned with a script-aware rule policy. The `scripts:` block selects it; see [Kotlin scripts](kotlin-scripts.md).

## Ignore patterns

Source-file collection always skips built-in test data directories (`/test/data/`,
`/testData/`, `/testdata/`, `/test-data/`, `/compiler-tests/`, and
`/compilerTests/`) and test source-set resource directories under
`src/<set>/resources/` when `<set>` is `test`, `androidTest`, `testFixtures`,
or ends in `Test` (for example, `src/commonTest/resources/`).
A directory is skipped only when it lies below the scanned directory, so a
project checked out under an ancestor such as `/home/runner/work/testdata/`,
or a scan of a `testdata/` directory itself, is still analysed. The rest of a
marker may sit above it: `krit src` from a module still skips
`src/test/resources/`.
Directory scans also skip documentation samples: a Kotlin or Java file under an
`examples` or `references` directory (for example `docs/skill/references/examples/`)
is not analysed when it is outside any `src` tree and no build file
(`build.gradle[.kts]`, `settings.gradle[.kts]`, `pom.xml`, `BUILD[.bazel]`,
`module.yaml`) exists in any directory between it and the Git root. Such
snippets are not compiled, so project rules would report duplicate declarations
in them. An example project with its own build file, a module with an `examples`
directory beside its build file, and anything outside a Git checkout are still
analysed.
Built-in skips apply only to directory scans, so files named explicitly on the command line are analyzed unless a user exclude pattern matches them.
The Gradle plugin's `krit check`, the pre-commit hook and Bazel `krit_test` /
`krit_aspect` pass each source file explicitly, so a file under `testdata/` or
`src/test/resources/` that they list is analysed; files in the tool and cache
directories below are still skipped.

Directory scans also skip these tool and cache directories at any depth:
`.git`, `.krit`, `.krit-cache`, `.krit-types`, `.gradle`, `.idea`, `.kotlin`,
`.claude`, `.codex`, and `.grit`. Project output such as `build/`,
`node_modules/`, `target/`, and `vendor/` is not on that list: Krit skips it
because it applies every `.gitignore` from the Git root down to the scanned
path. A project that does not ignore its `build/` directory in Git will have
generated sources analyzed.

Krit cannot tell documentation samples, fixtures, or example projects from real
sources. Keep them out of a scan with a `.gitignore` entry, or exclude them per
rule with glob patterns, relative to the project root, in `excludes`:

```yaml
naming:
  FunctionNaming:
    ignoreAnnotated: ['Composable', 'Test']
    excludes:
      - '**/test/**'
      - '**/docs/**/examples/**'
      - '**/samples/**'
```

`excludes` applies to that rule only; a rule set can carry its own `excludes`
list (for example under `naming:`) to cover every rule in it. Supported
pattern forms are `**/dir/**` (the directory part may contain `*` and `**`),
`**/*Suffix.kt`, `**/Name.kt`, and a plain glob that matches the file name.
To keep a path out of every rule, add it to `.gitignore` or run krit on the
source roots only (`krit app/src features/*/src`).

## Android policy rules

Android-specific thresholds also live in the same file:

```yaml
android-lint:
  OldTargetApi:
    threshold: 29
  MinSdkTooLow:
    threshold: 16
  NewerVersionAvailable:
    recommendedVersions:
      - "androidx.appcompat:appcompat=1.5.0"
```

Unset values fall back to built-in defaults.

## Baselines

Freeze current findings, catch regressions:

```bash
krit --create-baseline baseline.xml .
krit --baseline baseline.xml .
```

Krit XML baselines use the `SmellBaseline` document shape with `ManuallySuppressedIssues`, `CurrentIssues`, and `RuleName:path:signature` IDs. Filename-only IDs and module-relative IDs are both recognized so existing baseline files can be reused during migration.

## Cache location

Generated repo-local state lives under `.krit/` by default. The incremental analysis cache uses `.krit/cache/`; parse, resource, type, file-walk, and library-profile indexes use sibling directories under the same root. The FIR checker cache uses `.krit/fir-cache/`; krit-fir also persists its incremental-checking state, the type oracle's per-file facts and the binary snapshot in `.krit/fir-incremental/` (the three most recent compile keys are kept) so a restarted daemon or a one-shot run starts warm. Incremental FIR checking is on by default; set `KRIT_FIR_INCREMENTAL=0` to compile the whole module on every FIR check instead. `--cache-dir DIR` only overrides the incremental analysis cache; `--no-cache` skips the incremental analysis cache, the FIR checker cache and the FIR incremental bundle.

Set `KRIT_STATE_DIR` to move all of that generated state, including the type oracle output, out of the project. This lets a scan run over a read-only source tree. Each project root gets its own `<name>-<hash>/.krit/` subdirectory, so several projects can share one `KRIT_STATE_DIR`. Inputs kept under `.krit/`, such as an exported Gradle model, are still read from the project. A daemon keeps its socket, lock, PID file and log in `<project>/.krit/`, so with `KRIT_STATE_DIR` set a scan neither starts nor reuses a daemon unless `--daemon-socket` names one, and `krit daemon start` still needs a writable project directory.
Per-user state (installed jars, JVM AOT/CDS archives and the daemon registry) lives under `~/.krit/`; set `KRIT_HOME` to move it. With `KRIT_HOME` set, the per-user cache (helper logs, compiled Java helpers and krit-fir's binary snapshots), which otherwise lives in the OS user cache directory, moves to `$KRIT_HOME/user-cache/`. Sandboxed runners, such as build-system actions, can point both variables at directories the action owns, together with `HOME`, `TMPDIR` and `KRIT_BUILDID_CACHE_BASE`. The JVM ignores `TMPDIR`, so also set `JAVA_TOOL_OPTIONS=-Djava.io.tmpdir=<dir>` to keep krit-fir's temporary files inside them.

## FIR scan requirements

CLI scans run the FIR checker by default, including `krit --daemon`. Pass
`--no-fir` to disable it. Before analyzing files, Krit requires Java 21 or
newer, a resolvable `krit-fir.jar`,
and a declared classpath source. If any requirement is missing, or the FIR
checker fails mid-scan, the scan warns on stderr, lists the default-on rules
that fell back to their Go implementation, and continues with normal exit
codes. Krit finds `java` under `$JAVA_HOME/bin` when `JAVA_HOME` is set and on
`PATH` otherwise; see [Hermetic and build-system runs](hermetic-builds.md).
JSON reports carry `summary.firDegraded` and SARIF reports carry a
`toolExecutionNotifications` entry. Pass `--fail-on-degraded` to exit `2` with
the same message instead, for CI. With `--no-fir`, Krit uses the Go-only
checker path without a warning.

### FIR coverage

FIR can only analyze a file the embedded compiler compiles without errors; a
file with a compiler error (often an unresolved reference to a library the
classpath lacks) is *gated* and keeps Go's findings. Whenever the pass gates
files, the scan prints one line to stderr (not with `-q`):

```
info: FIR analyzed 3,875 of 27,213 files (14%); 23,338 gated by compile errors, so Go-only findings stand for them (top causes: Unresolved reference 'androidx.compose' x12,034, ...). Run with -v for the list.
```

JSON reports carry the same numbers as `summary.fir`, and SARIF reports as
`runs[0].properties.fir`:

```json
"fir": {"authoritative": 3875, "gated": 23338, "excluded": 120,
        "topCauses": [{"cause": "Unresolved reference 'androidx.compose'", "count": 12034}]}
```

`excluded` counts scripts and files outside every JVM compilation. `withheld`
(present when non-zero) counts the Go findings of FIR-required rules dropped in
gated files (see `fir.requireAuthoritativeRules` below); the stderr line then
ends its first clause with `, except N withheld for FIR-required rules`. Up to five
causes are listed, most frequent first. An unresolved reference is reported as
the package prefix of the import that names it, so a missing library shows up
as one cause; other messages have their quoted names blanked (`'…'`).
`--min-fir-coverage=0.8` exits `2`, like `--fail-on-degraded`, when FIR
analyzed less than that share of the files it checked (authoritative plus
gated), or when the FIR pass did not run; such scans run in-process. When at
least a quarter of a Gradle model's classpath is missing on disk, the stale
model warning adds that FIR results will be mostly gated.

For Gradle projects, run `kritExportModel` (the Gradle plugin's `kritCheck`
task already depends on it). Krit discovers `.krit/gradle-model/`; a stale
model or classpath jars missing on disk produces a warning and analysis
continues. Missing jars may be unbuilt project module outputs; build those
modules or rerun `kritExportModel`.
For an explicit model location use `--gradle-model DIR`. An empty exported
classpath is valid.

To get `kritExportModel` and `kritCheck` in every Kotlin/Android module of a
multi-module build, apply the settings plugin once. Both plugins are on Maven
Central, so `mavenCentral()` in `pluginManagement` is enough:

```kotlin
// settings.gradle.kts
pluginManagement {
    repositories {
        mavenCentral()
        gradlePluginPortal()
    }
}

plugins {
    id("dev.jasonpearson.krit.settings") version "0.5.0"
}
```

From a Gradle init script instead (no edits to the project), resolve the
plugin on the init script's classpath and apply it by class:

```kotlin
// init.gradle.kts, passed with: ./gradlew -I init.gradle.kts kritExportModel
initscript {
    repositories { mavenCentral() }
    dependencies {
        classpath("dev.jasonpearson.krit:krit-settings-plugin:0.5.0")
    }
}

beforeSettings {
    pluginManager.apply(dev.jasonpearson.krit.settings.KritSettingsPlugin::class.java)
}
```

See [krit-settings-plugin](https://github.com/kaeawc/krit/blob/main/krit-settings-plugin/README.md)
for the `krit { }` settings block.

For Maven, generate the classpath with
`mvn dependency:build-classpath -Dmdep.outputFile=classpath.txt`, then split
the file's platform-separated paths into a YAML list:

```yaml
oracle:
  classpath:
    - /absolute/path/to/dependency-one.jar
    - /absolute/path/to/dependency-two.jar
```

For Bazel, a type-aware `krit_test` reads the classpath from the build
graph (see [Type-aware krit_test](integrations.md#type-aware-krit_test)).
To scan outside Bazel, `bazel run @krit//:export_model -- //...` writes
`.krit/bazel-model/`, which krit discovers like a Gradle model (see
[Exporting a model](integrations.md#exporting-a-model-for-krit-the-lsp-and-mcp)).
Without the krit Bazel module, inspect the compile action for your `java_library` or
`kt_jvm_library` target with `bazel aquery --include_commandline
'deps(//app:lib)'`. Copy its resolved compile-classpath jars into Krit's
configuration, for example:

```yaml
oracle:
  classpath:
    - /absolute/path/to/bazel-out/.../libdependency.jar
```

For a stdlib-only project, declare `oracle.classpath: []` explicitly.

Air-gapped installations can point `KRIT_FIR_JAR` at a local `krit-fir.jar`.
That override is checked before any download. Tagged releases otherwise
download the matching jar and verify its published SHA-256 checksum.

To retain Go findings for selected rules even after FIR checks, configure:

```yaml
fir:
  goAuthoritativeRules:
    - InjectDispatcher
```

FIR verdicts for the listed rule IDs are ignored; other FIR rules still apply.

When FIR cannot compile a file (a compile error, missing generated code, an
unresolved classpath entry), Go's findings normally stand there. For
**FIR-required** rules they don't: these rules' Go implementations guess at
types, so in a gated file their findings are mostly false positives. Their Go
findings in files FIR could not compile are withheld, and they are reported only
where FIR's verdict is authoritative (#1675):

```yaml
fir:
  requireAuthoritativeRules:   # default when unset:
    - CastNullableToNonNullableType
    - UnnecessaryNotNullOperator
    - UnnecessarySafeCall
    - UnsafeCast
    - UselessCallOnNotNull
    - UselessElvisOnNonNull
```

An empty list (`requireAuthoritativeRules: []`) keeps every Go finding in
gated files, as before. Where krit-fir itself failed (a crashed file, a checker
that threw, or a pass that lost every verdict, which prints the `--fir checker
failed` warning), Go's findings stand for these rules too. A rule listed here
that krit-fir has no checker for, or that `goAuthoritativeRules` also lists,
keeps its Go findings. `-v` reports
`withheld=N` per rule. The default summary and the JSON/SARIF FIR coverage
(`summary.fir.withheld`) report the total.

## Analysis depth

Krit exposes a single `analysis.depth` dial that selects how much
compiler-backed analysis runs. It is the supported way to trade cold-run
time against precision; individual `--no-*` flags still work and beat
the preset.

```yaml
analysis:
  depth: balanced   # fast | balanced | thorough
```

| Preset    | What it does                                                                                          |
|-----------|-------------------------------------------------------------------------------------------------------|
| `fast`    | Skips the JVM type oracle. Source-level inference still runs. Best for low-latency local checks.      |
| `balanced`| (Default) Source inference + JVM type oracle.                                                         |
| `thorough`| Balanced plus a targeted-resolution pre-pass for compiler facts. The FIR checker also runs, as it does for every CLI depth by default. |

Precedence, highest first: explicit individual flag (e.g.
`--no-type-oracle`, `--no-fir`) → `--depth=<preset>` → `analysis.depth` → `balanced`.

Compare `fast` and `balanced` on a representative target before changing CI
defaults. `fast` is appropriate for projects that do not enable rules requiring
oracle facts.

## Oracle classpath

`oracle.classpath` is a list of jar paths under the top-level `oracle:` block;
it extends the classpath seen by the JVM-backed type oracle.

```yaml
oracle:
  classpath:
    - libs/android.jar
```

`oracle.compilerPlugins` lists Kotlin compiler plugin jars (kotlinx.serialization,
Compose, Parcelize, ...) that `krit-fir` loads in addition to those of the
Gradle model, and `oracle.compilerPluginOptions` their options as
`plugin:<id>:<key>=<value>`. Use them when there is no Gradle model; see
[Kotlin compiler settings](kotlin-compiler-settings.md#compiler-plugins).

```yaml
oracle:
  compilerPlugins:
    - /path/to/kotlin-serialization-compiler-plugin-embeddable-2.4.21.jar
```

The Kotlin standard library is supplied automatically when the effective
classpath does not already include a Kotlin stdlib jar. Other libraries, such
as the Android SDK, AndroidX, and application dependencies, still need to be
listed explicitly for their types to resolve.

## Migrating from detekt

Krit imports an existing detekt config automatically (`config/detekt/detekt.yml`,
`detekt.yml`, or `.detekt.yml`) and layers `krit.yml` on top, so krit.yml values
win. Set `detekt: {enabled: false}` or pass `--no-detekt-config` to opt out, and
use `detekt: {configFile: path}` or `--detekt-config path` to name the file.
See [Migrating from detekt](detekt-migration.md) for the full mapping table,
threshold semantics, `excludes`/`includes`/`ignoreAnnotated`, and baseline
import.

## Android Lint configuration

krit's Android rules port Android Lint issues, and a project that already
turned an issue off for Lint does not get krit's port of it either. krit
reads every `lint.xml` (`<issue id="MissingTranslation" severity="ignore"/>`)
and the `disable` lists of Gradle `lint { }` and `lintOptions { }` blocks
(`disable += listOf("MissingTranslation")`, Groovy `disable 'X', 'Y'`) found
up to five directories below the scan root, outside source sets and build
outputs. An issue ignored only for some paths (`<ignore path="..."/>`) is not
turned off. Lint applies a module's config to that module; krit applies every
config it finds to the whole scan.

The match uses the rule's Lint issue IDs, the same IDs `@SuppressLint` and
`tools:ignore` accept. Setting a rule's `active` in `krit.yml` wins over the
Lint config. `--verbose` lists the files and issues that were read. Set
`androidLintConfig: {enabled: false}` to opt out.

## ktlint and line-length settings in .editorconfig

A project that formats with ktlint records its style choices in
`.editorconfig`, and krit reads them from the `.editorconfig` files above the
scan root (up to `root = true` or the repository root), in `[*]`, `[*.kt]` and
`[*.{kt,kts}]` sections:

- `ktlint_standard_<rule> = disabled` turns off krit's rule for the same
  check: `no-wildcard-imports` (WildcardImport), `max-line-length`
  (MaxLineLength), `filename` (MatchingDeclarationName), `function-naming`,
  `package-name`, `class-naming`, `enum-entry-name-case`, `final-newline`
  (NewLineAtEndOfFile), `no-trailing-spaces` (TrailingWhitespace),
  `no-unused-imports` (UnusedImport), `modifier-order`, `no-empty-file`
  (EmptyKotlinFile) and `no-empty-class-body` (EmptyClassBlock).
- `ij_kotlin_packages_to_use_import_on_demand` packages join
  WildcardImport's `excludeImports`.
- `max_line_length` sets MaxLineLength's limit (`off` turns the rule off), so
  krit reports the line length the project's formatter enforces.

These sit under the detekt config and `krit.yml`, which win where they set
the same rule. `--verbose` reports what was applied; set
`ktlintEditorConfig: {enabled: false}` to opt out. `--enable-editorconfig`
additionally applies the other standard properties (`indent_style`,
`insert_final_newline`, ...), which override krit.yml.

## Gradle model

`./gradlew kritExportModel` writes `.krit/gradle-model/` with each Gradle
project's source sets and compiler classpaths. Krit discovers this directory
from the nearest Gradle settings root when scanning a project. Use
`--gradle-model DIR` to select a directory explicitly, or `--no-gradle-model`
to disable discovery. An explicit directory takes precedence when both flags
are set. Existing boot classpath entries come first, then model classpath
entries, `oracle.classpath` from `krit.yml`, and finally the `CLASSPATH`
environment variable. Duplicate paths keep their first position.

A scan of one project compiles the sources of the projects it depends on as
context. `--gradle-project-outputs FILE` names their built outputs instead, as
`{"schema": 1, "projects": {":lib": ["/repo/lib/build/libs/lib.jar"]}}`.
A listed project goes on the classpath through those jars (or class directories),
and its sources are not compiled. The Gradle plugin writes this file when
`krit { fir { useBuiltDependencies = true } }` is set. Such scans run in-process.

In a Bazel workspace, `bazel run @krit//:export_model -- //...` writes the
same schema to `.krit/bazel-model/` (one project per target), which krit
discovers the same way; a directory with both prefers `.krit/gradle-model/`.
See [Exporting a model](integrations.md#exporting-a-model-for-krit-the-lsp-and-mcp).

Schema 2 models also carry each source set's Kotlin compiler settings
(`languageVersion`, `apiVersion`, opt-ins, semantic flags). Krit applies them to
the embedded compiler so a project is analyzed under its own language level, and
includes them in cache identities; see
[Kotlin compiler settings](kotlin-compiler-settings.md).

### Generated-source jars (`.srcjar`)

Build systems that package generated code as source jars (Bazel's KSP, kapt,
Wire/protobuf, SQLDelight and view-binding outputs) pass them with
`--generated-srcjar PATH`, repeated once per archive, or list them per source
set in the model as `generatedSourceJars`. Each srcjar is treated like a
`generatedSourceDirs` entry: its Kotlin files are compiled as FIR checker and
type-oracle context, so code that uses a generated Kotlin type is not gated,
and nothing in it is reported. Its `.java` files are unpacked too and, like
every `.java` file under a source or generated directory, given to the FIR
compilation as Java sources: Kotlin that uses a generated Java type (view
binding, BuildConfig, Wire-Java) resolves, and the Java itself is neither
compiled nor checked ([#1626](https://github.com/kaeawc/krit/issues/1626)).
An edit to a Java file invalidates the FIR caches the same way a Kotlin edit
does. Other entries are ignored.

The Kotlin compiler only reads sources from disk, so krit unpacks each srcjar
once into `$KRIT_HOME/srcjars/<sha256 of the archive>/` (default
`~/.krit/srcjars/`) and reuses that directory while the archive is unchanged.
Hermetic runners such as Bazel actions should point `KRIT_HOME` at a
directory they own. A srcjar that cannot be read is a warning and the scan
continues without it.
