# Integrations

Krit's shipped integration today is the **raw CLI**, which emits
SARIF / Checkstyle / JSON / plain output for any CI runner or
pre-commit hook.

Editor (LSP) and AI-agent (MCP) integrations also ship as separate
binaries (`krit-lsp`, `krit-mcp`) installed alongside `krit`. For
configuring `krit-mcp` with Claude Code, Claude Desktop, Codex, or
any MCP-compatible client see [MCP setup](mcp.md).

## Bazel

The `krit` Bazel module (`bazel/` in this repository) registers krit's
release binaries as Bazel toolchains, one per execution platform, so krit's
version is an ordinary `bazel_dep` that Dependabot or Renovate can bump:

```starlark
# MODULE.bazel
bazel_dep(name = "krit", version = "x.y.z")
```

```bash
bazel run @krit//:krit -- --base-path "$PWD" "$PWD"
```

`@krit//:krit-lsp` and `@krit//:krit-mcp` are available too. Bazel
resolves the toolchain (`@krit//toolchains:toolchain_type`) for the
platform that runs each action, so with remote execution krit's actions run
the binary built for the remote workers, and it downloads only the archive
of a platform it resolves. The archive is verified against the release's
`checksums.txt`. To pin the checksums
yourself, or to run a different release than the module's version:

```starlark
krit = use_extension("@krit//:extensions.bzl", "krit")
krit.toolchain(
    version = "0.3.0",
    sha256 = {"krit_0.3.0_linux_amd64.tar.gz": "<sha256>"},
)
```

Linux (amd64, arm64), macOS (amd64, arm64) and Windows (amd64) are
supported. To run another krit build, register a `krit_toolchain` of your
own ahead of the module's:

```starlark
# BUILD.bazel
load("@krit//:defs.bzl", "krit_toolchain")

krit_toolchain(
    name = "my_krit",
    krit = "//tools:krit",  # an executable target
)

toolchain(
    name = "my_krit_toolchain",
    toolchain = ":my_krit",
    toolchain_type = "@krit//toolchains:toolchain_type",
)
```

and `register_toolchains("//:my_krit_toolchain")` in `MODULE.bazel`. krit itself needs no Bazel build to analyze a Bazel workspace;
see [Bazel workspaces](hermetic-builds.md#bazel-workspaces).

### krit_test

`krit_test` runs krit as a Bazel test, so `bazel test //...` lints the
sources and Bazel caches the result until a source or the config changes.
The test fails when krit reports a finding:

```starlark
load("@krit//:defs.bzl", "krit_test")
load("@rules_kotlin//kotlin:jvm.bzl", "kt_jvm_library")

kt_jvm_library(
    name = "lib",
    srcs = glob(["src/main/kotlin/**/*.kt"]),
)

krit_test(
    name = "lib_krit",
    targets = [":lib"],
    config = "//:krit.yml",
)
```

| Attribute | |
| --- | --- |
| `targets` | Targets whose `srcs` are analyzed: `kt_jvm_library`, `kt_android_library`, `java_library`, or any rule with a `srcs` attribute. Only each target's own `.kt`, `.kts` and `.java` files are analyzed, not its dependencies'. |
| `srcs` | Kotlin and Java files to analyze directly. |
| `config` | `krit.yml` passed as `--config`. |
| `baseline` | Baseline file passed as `--baseline`. |
| `format` | Report format printed to the test log (default `plain`). |
| `args`, `data` | Extra krit flags, placed before the files, and files they refer to (`$(rootpath ...)`). |

By default the test runs Go-only and in-process: no daemon, no incremental
cache, no FIR checker, no type oracle and no Java facts helper. It needs no
JDK or network and reports the same findings on every machine. Rules that
rely on the JVM fall back to their source-level checks, as with `--no-fir`.
For FIR, see [Type-aware krit_test](#type-aware-krit_test). Rules that
look across modules (dead code, unused resources) only see the test's
sources; run those as a whole-repository `bazel run @krit//:krit` step (see
[Sharding rules by scope](hermetic-builds.md#sharding-rules-by-scope)).
Reported paths are relative to the workspace root.

`krit_test` names its sources explicitly, so krit analyzes them even under
test-data and test-resource directories (`testdata/`, `src/test/resources/`,
...); it still skips files in tool and cache directories such as `.gradle/`,
`.idea/` and `.kotlin/`. A `krit_test` whose sources are all skipped fails
instead of passing without checking anything. `krit_test` is a shell script, so it runs on Linux and
macOS, not Windows.

#### Type-aware krit_test

`type_aware = True` also runs krit's FIR checker and type oracle, with what
Bazel already knows instead of a hand-maintained `oracle.classpath`:

```starlark
krit_test(
    name = "lib_krit",
    targets = [":lib"],
    type_aware = True,
)
```

- **Classpath.** Each target's compile classpath, its
  `JavaInfo.transitive_compile_time_jars` without the target's own jars:
  the deps' (and their exports') header or ABI jars and, for rules_kotlin
  targets, the Kotlin stdlib the toolchain adds. For `srcs` outside any
  target, list the libraries they use in `deps`. krit reads the classpath
  from a generated model in the [Gradle model](configuration.md) format
  (`--gradle-model`).
- **Kotlin settings.** For rules_kotlin targets, what their `KotlinCompile`
  action uses: the Kotlin toolchain's `language_version`, `api_version` and
  `jvm_target`, the target's `kotlinc_opts` (or else the toolchain's
  `kotlinc_options`) as kotlinc flags, and the compiler plugins of its
  `plugins` and of its deps' `exported_compiler_plugins`
  (`KtCompilerPluginInfo`: jars, id and options), plus KSP and kapt
  generated sources (`KtJvmInfo.additional_generated_source_jars`). krit
  filters them through the same allowlist as a Gradle model's settings
  ([Kotlin compiler settings](kotlin-compiler-settings.md)), so both build
  systems reject the same flags. Plugin options that name the compile
  action's own directories (`{generatedClasses}`, `{temp}`, ...) are left
  out.
- **Android.** For rules_android and rules_kotlin Android targets
  (`android_library`, `kt_android_library`, `android_binary`): the Android
  SDK toolchain's `android.jar` (`AndroidSdkInfo`) as the boot classpath,
  the R class jars (`AndroidLibraryResourceClassJarProvider`,
  `AndroidIdeInfo.resource_jar`) so `R.string.x` resolves, and the target's
  own `manifest` and `resource_files`, so krit's manifest and resource rules
  run on them (they are opt-in: enable them in `krit.yml`). The res/
  directories reach krit through a generated BUILD file whose
  `resource_files` globs name them. minSdk and targetSdk come from the
  manifest's `<uses-sdk>` when it sits next to or above the sources.
- **JDK.** Bazel's Java runtime toolchain
  (`@bazel_tools//tools/jdk:current_java_runtime`) is `JAVA_HOME`. krit-fir
  needs Java 21 or newer, so pin a hermetic runtime in `.bazelrc`:
  `common --java_runtime_version=remotejdk_21`.
- **krit-fir.** The `krit-fir-vX.Y.Z.jar` of the same release, verified
  against its `checksums.txt` (or a `krit.toolchain(sha256 = ...)` entry).
  It is a repository of its own, fetched only when a type-aware test is
  built. krit downloads nothing at test time (`KRIT_NO_JAR_DOWNLOAD=1`).
- **No silent fallback.** The test passes `--fail-on-degraded`, so a FIR
  pass that cannot run fails the test instead of caching a Go-only result.

Every input is declared, so Bazel caches the test (locally or remotely)
until a source, a dependency's ABI, the config, the JDK or krit changes.
Each run starts a cold JVM (no daemon), which adds a few seconds per test.
A `krit_test` sees only its targets: rules that look across modules still
need a whole-repository run.

To test an unreleased checkout of the module, point `KRIT_BAZEL_LOCAL_DIST`
at a directory with a locally built `krit` and `krit-fir.jar`; it replaces
the release download (the local build becomes the one krit toolchain). `scripts/bazel-e2e-local.sh` does that for
`bazel/e2e`, and checks that a second run after `bazel clean` is served
entirely from the disk cache.

### krit_aspect

`krit_aspect` runs krit with FIR inside `bazel build`, as one action per
Kotlin or Java target, so a whole monorepo is analyzed without a test target
per module and Bazel caches (locally or remotely) and can remotely execute
every action:

```bash
bazel build //... --aspects=@krit//:defs.bzl%krit_aspect --output_groups=krit
```

Each target with `JavaInfo` and `.kt`, `.kts` or `.java` files in `srcs`
(`kt_jvm_library`, `kt_android_library`, `java_library`, `android_library`,
their test variants) gets `<target>.krit.sarif` and `<target>.krit.txt` in
the `krit` output group, next to its other outputs in `bazel-bin`. A target
tagged `no-krit` is skipped. The aspect does not propagate to dependencies:
each target is analyzed once, against its deps' compile jars and not their
sources, so editing a library re-runs only the targets whose classpath
changed (see [Caching and remote execution](#caching-and-remote-execution)
for keeping that to the library itself). `.srcjar` files in `srcs` are
compiled as generated-source context and not reported on.

The action compiles each target with its rules_kotlin settings (language
and API version, `kotlinc_opts`, compiler plugins, KSP/kapt sources) and,
for Android targets, `android.jar` and the R class jars, as
[type-aware krit_test](#type-aware-krit_test) does. The manifest and
resource rules are project-scope:
[krit_project_report](#krit_project_report) runs them (or every target's
action with `--@krit//:scope=all`). It reads only declared
inputs (sources, classpath, plugin jars, krit, the krit-fir jar, the Java
runtime, `krit.yml`, the baseline), runs no daemon,
uses a scratch `$HOME` and never downloads anything. A FIR pass that cannot
run fails the action (`--fail-on-degraded`). The Java runtime is Bazel's
*exec* Java runtime, so pin it to JDK 21 or newer next to the target one:

```bash
# .bazelrc
common --java_runtime_version=remotejdk_21
common --tool_java_runtime_version=remotejdk_21
```

| Flag | |
| --- | --- |
| `--@krit//:config=//:krit.yml` | `krit.yml` for every action (`--config`). |
| `--@krit//:baseline=//:krit-baseline.xml` | Baseline of known findings (`--baseline`). |
| `--@krit//:fail_on_findings` | Adds a [validation action](https://bazel.build/extending/rules#validation_actions) per target that fails the build when the target has findings, printing them. It runs with Bazel's default `--run_validations`. |
| `--@krit//:scope=all` | Run every rule per target, not only the file-scope ones (see below). |
| `--@krit//:fir=false` | Go-only analysis, without the JDK or krit-fir. |
| `--@krit//:worker=false` | No persistent worker: each Krit action starts its own krit-fir JVM. |
| `--@krit//:large_target_sources=400` | Actions that scan at least this many files run in the `krit_large` exec group (see below). |
| `--@krit//:min_sdk_version=N`, `--@krit//:target_sdk_version=N` | The Android `minSdk` / `targetSdk` of sources whose own manifest declares none (`krit --min-sdk` / `--target-sdk`). Bazel builds often set them only on the `android_binary`; an `android_binary`'s own action uses its `manifest_values` when they are numbers. `krit_project_report` reads them too, and `krit_test` takes the krit flags in `args`. |

The Krit actions run `krit bazel-worker` as Bazel
[persistent workers](https://bazel.build/remote/persistent) (JSON protocol,
`supports-workers`). Each worker keeps one krit-fir JVM warm between the
actions it runs, while every action is still a fresh krit process with
empty per-run state and incremental FIR state off, so no target's sources
or results reach another action. The worker's JVM serves every target with
the same Kotlin compiler settings, plugins and JDK: each action sends its
own sources and classpath, and krit-fir builds a new compiler session for
them in the warm JVM. A target with other settings starts a new JVM, which
replaces the old one. On a 24-target benchmark (`scripts/bazel-worker-bench.sh`,
4 cores, `--jobs=2`) the Krit actions of a full build took 7.0 s instead of
82.8 s with one JVM per action (mean 0.54 s against 6.69 s), and an edited
target's action 0.5 s instead of 5.0 s. Each worker holds at most one JVM,
which stops with the worker; `--worker_max_instances=Krit=N` bounds them. With workers off
(`--@krit//:worker=false`, `--strategy=Krit=sandboxed`, remote execution)
each action starts and stops its own JVM.

Put the flags in `.bazelrc` under a config, e.g.
`build:krit --aspects=@krit//:defs.bzl%krit_aspect --output_groups=+krit`,
then `bazel build --config=krit //...`. Other rules can read a target's
reports through the `KritReportInfo` provider.

Each action sees one target, so by default it runs only krit's
**file-scope** rules (`--only-scope file`; `krit --list-rules --format json`
reports each rule's `scope`). A project-scope rule there would be wrong:
DeadCode would report a function as unused when its only caller is in
another target. `--@krit//:scope=all` runs every rule per target anyway,
which suits a build with one target per module and no cross-target rules.

#### Caching and remote execution

A Krit action's key includes its sources and `.srcjar`s, its deps' compile
jars, its Kotlin settings and compiler plugin jars, `android.jar`, the R
jars and its manifest and resources, krit, the krit-fir jar, the Java
runtime, `krit.yml`, the baseline, the `--@krit//` flags that shape its
command line, and, with remote execution, the execution platform's
properties. Every path in its command line and in the model it reads is
relative to the execution root, and the reports name workspace-relative
paths, so a remote cache shares the action between machines.
`scripts/bazel-e2e-local.sh` checks that no Krit action key names an
absolute path. Two settings of yours decide whether that holds:

- **ABI jars for Kotlin.** For `java_library` the compile jar is a header
  jar, but rules_kotlin puts the full class jar into a `kt_jvm_library`'s
  compile jar unless its toolchain makes ABI jars. Then any edit to a library
  re-runs the Krit action (and the Kotlin compile) of every target above it.
  Turn on compile avoidance with `experimental_use_abi_jars = True` in
  `define_kt_toolchain`, or `--define=experimental_use_abi_jars=1`. An
  implementation-only edit then re-runs only the library's own action. In
  `scripts/bazel-worker-bench.sh`'s 24-target chain, an edit to the bottom
  target re-ran 24 Krit actions without ABI jars and 1 with them.
- **A hermetic JDK.** The Java runtime's files are inputs of every Krit
  action, so a JDK that differs between machines (`local_jdk` mirrors the
  machine's own) changes every key, and a `java_runtime` with an absolute
  `java_home` puts that path into the action's environment. Use a
  `remotejdk_*` runtime (above).

For scheduling, Bazel gets an estimate of each action's CPU and memory from
the number of files it scans and reports on (generated srcjars are not
counted). Below 50 files that is 1 CPU and 512 MB with
FIR, from 50 files 2 CPUs and 1 GB, and from `--@krit//:large_target_sources`
(400) files 2 CPUs and 2 GB. Go-only actions (`--@krit//:fir=false`) get a
quarter of that memory, and 4 CPUs when large. The estimate applies to local
execution (`resource_set`).

For remote execution, an aspect's actions never see a target's own
`exec_properties`, so the large actions run in their own exec group,
`krit_large`. It resolves to the same execution platform as the other
actions. A platform's `krit_large.`-prefixed `exec_properties` apply to the
large actions only:

```starlark
platform(
    name = "rbe",
    parents = ["@rbe_config//config:platform"],
    exec_properties = {
        "krit_large.Pool": "large",
        "krit_large.dockerMemory": "8g",
    },
)
```

### krit_project_report

`krit_project_report` runs the **project-scope** rules (cross-file and
module rules such as DeadCode, rules over all parsed files, Android manifest
and resource rules) once over a whole build:

```starlark
load("@krit//:defs.bzl", "krit_project_report")

krit_project_report(
    name = "krit_project",
    deps = ["//app:binary"],
)
```

It collects the main repository's `srcs`, `manifest` and `resource_files` of
`deps` and of everything they depend on (`deps`, `exports`, `runtime_deps`,
`associates`) and runs one Go-only krit action (no JVM) that writes
`krit_project.krit.sarif` and `.krit.txt`. Bazel caches it until one of those
files, or the config, changes. `config` defaults to `--@krit//:config`;
`fail_on_findings` (`auto`, `true`, `false`) adds a validation action, `auto`
following `--@krit//:fail_on_findings`. `srcs` adds files outside the graph.
Together with `krit_aspect` it reports what one whole-repository krit run
reports, since the two scopes partition the rules.

Module-aware rules (module dependency rules) need krit's module graph. For
file scan paths krit discovers it from the nearest directory above the
**first** file that has a Gradle settings file or a Bazel `MODULE.bazel` /
`WORKSPACE`, looking no higher than a repository root (`.git`) or a Bazel
execution root. A file in no such build has no module graph. This action's
files sit below the execution root, which holds no BUILD files in a sandbox,
so the graph is empty here and those rules find nothing yet (#1694).

### Exporting a model for krit, the LSP and MCP

`krit --fir` run outside Bazel (editors through `krit-lsp`, `krit-mcp`,
scripts) needs the same classpath. Export it from the build graph:

```bash
bazel run @krit//:export_model -- //...
```

This builds the `krit_model` output group of `krit_model_aspect`
(`@krit//:defs.bzl`) for the targets, which writes one model per Kotlin or
Java target and builds the jars and srcjars it names, and installs the
models into `.krit/bazel-model/` with absolute paths: sources in the
workspace, jars and generated files in Bazel's execution root (`bazel-out`,
`external`). Each target is a project in the
[Gradle model](configuration.md#gradle-model) schema: its label is
the project path, its deps' labels its project dependencies, and its source
set has the target's source directories, compile classpath, generated
srcjars, Kotlin settings and Android inputs, as for `krit_aspect`. krit,
`krit-lsp` and `krit-mcp` discover `.krit/bazel-model/` like
`.krit/gradle-model/`, at or above the scanned path; `--gradle-model DIR`
also accepts it. Arguments are passed to `bazel build`, and `BAZEL` names
the launcher (default `bazel`).

Re-run it after changing BUILD files or dependencies. After `bazel clean`
the exported jars are gone; krit warns (`bazel model ... lists N classpath
entries missing on disk`) and resolves their types as unknown until the
next export.

## GitHub Actions

The repository root is a GitHub Action. It installs a krit release, puts
`krit`, `krit-lsp` and `krit-mcp` on `PATH`, and can run a scan:

```yaml
permissions:
  contents: read
  security-events: write   # only for upload-sarif

steps:
  - uses: actions/checkout@v4
  - uses: kaeawc/krit@v0.4.0
    with:
      run: 'true'
      args: --no-fir
      upload-sarif: 'true'
```

- **Version**: the action installs the release it is pinned to, so
  `kaeawc/krit@v0.4.0` runs krit 0.4.0, and Dependabot's `github-actions`
  ecosystem bumps the action and the CLI together. A SHA pin, or the
  floating major tag `@v0` (moved to every stable release), installs the
  release tagged on that commit. `version:` overrides it; `latest` works but
  is not reproducible.
- **Verification**: the archive is checked against the release's
  `checksums.txt` and the install fails if it is missing, does not list the
  archive, or does not match. `verify-attestation: 'true'` also checks the
  GitHub build-provenance attestation with `gh attestation verify`.
- **Platforms**: Linux amd64/arm64 (musl amd64 detected, or forced with
  `libc: musl`), macOS amd64/arm64 and Windows amd64. The binary is cached
  with `actions/cache` unless `cache: 'false'`.
- **Run** (`run: 'true'`): `paths`, `args`, `config` and `baseline` pass
  through to krit, which writes `sarif-file` (default `krit.sarif`) with
  locations relative to the workspace. The `findings-count` output is the
  number of SARIF results (counted with `jq`), and `fail-on-findings`
  (default `true`) fails the step when it is non-zero. `upload-sarif: 'true'`
  uploads the report to code scanning under `category` (default `krit`).

Outputs: `version`, `path`, `findings-count`, `sarif-file`, `exit-code`.

`kaeawc/krit/.github/actions/krit-action` is the older composite action
(annotations through a problem matcher, a `fir` input that sets up Java). It
now shares the root action's fail-closed download and counts findings with
`jq`; new workflows should use the root action.

### Caching `.krit/` between CI runs

`--fir` spends most of a cold run compiling. The FIR checker cache
(`.krit/fir-cache/`) keys each file by its path relative to the repository
and by the content of every input, never by an absolute path or an mtime:
sources, classpath jars, the `krit-fir` jar. A `.krit/` saved by one build
therefore hits for unchanged files in another, even when the checkout is at
another absolute path (another agent's workspace) or the jars sit under
another `GRADLE_USER_HOME`. Any changed input re-runs FIR for its
compilation. On a 195-file module with an 80-jar classpath, a restored
`.krit/` takes a `--fir` run from about 25 s to about 1.5 s.

Some checkers apply path heuristics (`/samples/`, `playground`, configured
`nonProductionPathMarkers`) to a file's absolute path. When the checkout root
itself contains such a marker, or a word like `test`, `sample` or `debug`,
the root stays part of the key and a cache moved there misses rather than
replaying verdicts computed elsewhere. Typical CI workspace paths
(`/home/runner/work/app/app`, `/builds/group/app`) don't.

Restore the last build's `.krit/` before the scan and save it from the
default branch:

```yaml
steps:
  - uses: actions/checkout@v4
  - uses: actions/cache/restore@v4
    with:
      path: .krit
      key: krit-${{ runner.os }}-${{ github.sha }}
      restore-keys: krit-${{ runner.os }}-
  - uses: kaeawc/krit@v0.4.0
    with:
      run: 'true'
      args: --fir
  - if: github.ref == 'refs/heads/main'
    uses: actions/cache/save@v4
    with:
      path: .krit
      key: krit-${{ runner.os }}-${{ github.sha }}
```

Restoring by prefix doesn't replay another commit's findings: the caches
check their entries against the current content (the per-file analysis cache
`.krit/cache/` compares git blob IDs, not "clean against HEAD"), so a cache
from another commit only costs misses. One known gap applies to warm runs in
general, not just to restored caches: `.krit/cache/` keys a file's findings
by that file alone, so resolver-backed rules can keep a stale finding when a
declaration in another file changes (#1778). `.krit/cache/` and the oracle caches still key on absolute paths and
hit only when the checkout path is the same. `.krit/fir-incremental/` (the
JVM's incremental state) is keyed by absolute paths and jar mtimes on the
`krit-fir` side and is rebuilt after a move; restoring it is harmless.

## Pre-commit

```yaml
# .pre-commit-config.yaml
repos:
  - repo: https://github.com/kaeawc/krit
    rev: <pinned tag>
    hooks:
      - id: krit          # analyze source files
      - id: krit-fix      # auto-fix source files (optional)
```

`krit` runs analysis and fails the commit on findings. `krit-fix` runs
`krit --fix` over the staged files — use one or the other, not both in
the same hook stage.

FIR runs by default and needs Java 21+ plus a declared classpath source.
Run `./gradlew kritExportModel` before the hook, or set `oracle.classpath`
in `krit.yml`. To use Go analysis only, add `args: [--no-fir]` to the hook.

## Output formats for CI

The older `krit-action`'s `fir` input defaults to `true` and sets up Temurin
Java 21. Before the action runs, export a Gradle model with
`./gradlew kritExportModel` or set `oracle.classpath` in `krit.yml`.
Set `fir: 'false'` to run with `--no-fir`.

| Format     | Use case                    | Flag |
|------------|-----------------------------|------|
| SARIF      | GitHub Code Scanning        | `--format sarif` |
| Checkstyle | Jenkins, other CI tools     | `--format checkstyle` |
| JSON       | Custom processing (default) | `--format json` |
| Plain      | Human-readable logs         | `--format plain` |

### Several reports from one run

`--report FORMAT=PATH`, repeated, writes one file per report from a single
analysis run, so enabling SARIF and JSON does not scan twice:

```bash
krit --report sarif=build/krit.sarif --report json=build/krit.json src/
```

Every report renders the same final findings (after baseline, `--diff`,
`--warnings-as-errors` and `--min-confidence`). Like `-o`, each file is staged
next to its target and only replaces it once the run produced a complete
report; a run that exits with code 2 leaves the previous files in place. With
only `FORMAT=PATH` reports, stdout gets no report (the summary still goes to
stderr); add `-f`/`--format` or `-o` to also write the usual output. A bare
`--report FORMAT` keeps its old meaning, an alias for `-f`. Such scans run
in-process rather than through the daemon.

### Finding paths

Every format reports each finding's file path the same way, whatever form the
scan path took (`krit .`, `krit src`, `krit /abs/project`) and whether or not
a daemon serves the scan:

- A file under the base path is reported relative to it, with forward slashes.
  The base path is `--base-path` when set, otherwise the first scan path (its
  directory when it is a file).
- A file outside the base path is reported absolute.
- `--absolute-paths` reports every file as absolute instead.

Baselines and `--fix` are unaffected: they match and edit files on disk.

### Baseline IDs

A baseline entry names a finding as `Rule:file:$Rule$message`, where `file` is
the finding's path relative to the base path above. Both sides are made
absolute (and, if needed, symlink-resolved) first, so the ID is the same
whether the scan runs in-process or through the daemon, with or without FIR,
from a cold or a warm `.krit/`, and whether `--base-path` is `.` or absolute.
A baseline created in CI therefore matches a local daemon run. Matching also
accepts the file's base name, so baselines written by Krit versions that fell
back to base names keep working.

### JSON finding fields

Each entry in `findings` has these fields:

| Field | Meaning |
|-------|---------|
| `file`, `line`, `column` | 1-based position of the flagged node: the declaration name for function, class and property rules, the call or operator for expression rules, the attribute (or element start when the attribute is absent) for XML rules. |
| `startByte`, `endByte` | Byte span in the file, starting at `line`/`column`. Present on every finding whose position maps into a readable file; `endByte` is always greater than `startByte`. Rules that know the flagged node report its full range (a cast, a call, a function declaration). For the others, Kotlin and Java findings end with the innermost syntax node that starts at the position (usually the identifier or keyword), and XML findings end with the attribute (`name="value"`) or the element name. Use `line` and `column` for navigation. |
| `ruleSet`, `rule`, `severity`, `message` | Rule identity and report text. |
| `fixable`, `fixLevel` | `fixable` is true when the finding carries an autofix; `fixLevel` (`cosmetic`, `idiomatic`, `semantic`) describes its safety and is omitted otherwise. |
| `confidence` | Number in (0, 1], always written with a decimal point (`1.0`, `0.75`). Compiler-confirmed findings score higher than name-based heuristics. |
| `effort` | `trivial` (one line or one attribute), `local` (one file), `refactor` (several files) or `architectural`. |

## Suggested fixes (CLI and IDE)

Some rules emit ordered, user-selectable **suggested fixes** instead
of a single autofix (see [Suggested fixes](suggested-fixes.md)). The
two surfaces below are how integrations apply one.

### CLI: `krit apply-suggestion`

```bash
krit --format=json . > findings.json

# List all findings that carry suggestions and their suggestion ids:
krit apply-suggestion --list findings.json

# Preview what a specific suggestion would change:
krit apply-suggestion \
  --finding 'PreferVal:src/main/kotlin/Example.kt:5:3' \
  --suggestion use-val \
  --dry-run \
  findings.json

# Apply it:
krit apply-suggestion \
  --finding 'PreferVal:src/main/kotlin/Example.kt:5:3' \
  --suggestion use-val \
  findings.json
```

The verb reuses the autofix application path
(`fixer.ApplyAllFixesColumns`), so cross-file edits, byte/line spans,
overlap deduplication, and ktfmt-shaped output match `krit --fix`.
Informational suggestions (no `edits`, only `applicationToken`) are
rejected — they are for the integration to interpret, not for the CLI
to write.

### IDE / LSP

Integrations should render each `suggestedFixes[]` entry as its own
native quick-fix / code-action, **in the order the rule emitted them**
— do not sort by id, title, or safety level:

- Register one quick-fix per machine-applicable suggestion (those with
  non-empty `edits`).
- When suggestions are present, suppress the generic "apply autofix"
  entry so the user is never offered both choices at once.
- Surface informational suggestions (no `edits`) as intentions that
  route through the integration's help/`applicationToken` handling.

For LSP wrappers, expose each suggestion as a distinct `CodeAction`
with a stable `title` (the rule-provided `Title`) and a `data` blob
that carries `{findingId, suggestionId}` so the resolve step can run
`krit apply-suggestion` with deterministic ids.
