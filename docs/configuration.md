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

## Kotlin scripts

`.kts` files (Gradle build scripts, `*.main.kts`) are scanned with a script-aware rule policy. The `scripts:` block selects it; see [Kotlin scripts](kotlin-scripts.md).

## Ignore patterns

Source-file collection always skips built-in test data directories (`/test/data/`,
`/testData/`, `/testdata/`, `/test-data/`, `/compiler-tests/`, and
`/compilerTests/`) and test source-set resource directories under
`src/<set>/resources/` when `<set>` is `test`, `androidTest`, `testFixtures`,
or ends in `Test` (for example, `src/commonTest/resources/`).
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
codes. JSON reports carry `summary.firDegraded` and SARIF reports carry a
`toolExecutionNotifications` entry. Pass `--fail-on-degraded` to exit `2` with
the same message instead, for CI. With `--no-fir`, Krit uses the Go-only
checker path without a warning.

For Gradle projects, run `kritExportModel` (the Gradle plugin's `kritCheck`
task already depends on it). Krit discovers `.krit/gradle-model/`; a stale
model or classpath jars missing on disk produces a warning and analysis
continues. Missing jars may be unbuilt project module outputs; build those
modules or rerun `kritExportModel`.
For an explicit model location use `--gradle-model DIR`. An empty exported
classpath is valid.

For Maven, generate the classpath with
`mvn dependency:build-classpath -Dmdep.outputFile=classpath.txt`, then split
the file's platform-separated paths into a YAML list:

```yaml
oracle:
  classpath:
    - /absolute/path/to/dependency-one.jar
    - /absolute/path/to/dependency-two.jar
```

For Bazel, inspect the compile action for your `java_library` or
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

## Gradle model

`./gradlew kritExportModel` writes `.krit/gradle-model/` with each Gradle
project's source sets and compiler classpaths. Krit discovers this directory
from the nearest Gradle settings root when scanning a project. Use
`--gradle-model DIR` to select a directory explicitly, or `--no-gradle-model`
to disable discovery. An explicit directory takes precedence when both flags
are set. Existing boot classpath entries come first, then model classpath
entries, `oracle.classpath` from `krit.yml`, and finally the `CLASSPATH`
environment variable. Duplicate paths keep their first position.

Schema 2 models also carry each source set's Kotlin compiler settings
(`languageVersion`, `apiVersion`, opt-ins, semantic flags). Krit applies them to
the embedded compiler so a project is analyzed under its own language level, and
includes them in cache identities; see
[Kotlin compiler settings](kotlin-compiler-settings.md).
