# Krit Gradle Plugin

Gradle plugin that integrates the [krit](https://github.com/kaeawc/krit) Kotlin static analysis tool into your build. Krit is a Go binary that parses Kotlin with tree-sitter and runs its built-in rule set via single-pass AST dispatch, outputting JSON/SARIF/Checkstyle.

> **Multi-module repos:** for a single root config that covers every Kotlin /
> Java / Android subproject, use the [settings plugin](../krit-settings-plugin/README.md)
> instead of applying this one per module.

## Setup

The plugin is published to Maven Central, so include `mavenCentral()` in
your plugin repositories:

```kotlin
// settings.gradle.kts
pluginManagement {
    repositories {
        mavenCentral()
        gradlePluginPortal()
    }
}
```

Add the plugin to your `build.gradle.kts`:

```kotlin
plugins {
    id("dev.jasonpearson.krit") version "0.5.0"
}
```

That's it for most projects — drop a `krit.yml` at the repo root (or next
to the build file) and `./gradlew kritCheck` works. The CLI walks upward
from each analyzed directory looking for `krit.yml` / `.krit.yml`,
stopping at the worktree boundary (`.git` / `.hg` / `.jj`), so one config
at the root covers every subproject without per-module wiring.

### Common configuration

```kotlin
krit {
    config = file("krit.yml")               // optional; explicit override of walk-up discovery
    baseline = file("krit-baseline.xml")    // optional
    ignoreFailures = false                  // fail the build on findings

    reports {
        sarif.required = true               // default
        json.required = true
    }
}
```

### Custom rules

Wire a sibling rule project through the standard `dependencies` block:

```kotlin
dependencies {
    kritCustomRules(project(":my-rules"))
}
```

See [`krit-custom-rule-plugin`](../krit-custom-rule-plugin/README.md) for
authoring the rule module itself.

### Advanced / escape hatches

For overrides most projects never need:

```kotlin
krit {
    advanced {
        toolVersion = "0.5.0"                       // pin the binary version (defaults to the plugin version)
        binary = file("/usr/local/bin/krit")        // skip the download entirely
        parallel = 4                                // -j
        noCache = false                             // --no-cache
        typeInference = true
        allRules = false
        fixLevel = "idiomatic"                      // cosmetic | idiomatic | semantic
        source.setFrom("src/main/kotlin")           // override auto-detection
        reportsDir = layout.buildDirectory.dir("reports/krit")
    }
}
```

### Reports

Four formats are supported: SARIF (the default), JSON, plain text, and Checkstyle. Each can be toggled independently, and output locations can be customized:

```kotlin
krit {
    reports {
        sarif.required = true                                   // default
        json.required = true
        checkstyle {
            required = true
            outputLocation = file("build/reports/checkstyle/krit.xml")
        }
    }
}
```

Reports are written to `build/reports/krit/` unless overridden via `advanced.reportsDir`.
One krit run writes every enabled report (`--report FORMAT=PATH`), with or without
custom rules. A krit release without `--report FORMAT=PATH` (0.5.0 and earlier) gets one
run per report instead.

### Per-Source-Set Tasks

When the Kotlin JVM plugin or Android Gradle Plugin is applied, krit automatically registers per-source-set or per-variant tasks:

**Kotlin JVM projects:**
```bash
./gradlew kritCheckMain   # Analyze src/main/kotlin
./gradlew kritCheckTest   # Analyze src/test/kotlin
./gradlew kritCheck       # Analyze all sources
```

**Android projects:**
```bash
./gradlew kritCheckDebug    # Analyze debug variant sources
./gradlew kritCheckRelease  # Analyze release variant sources
./gradlew kritCheck         # Analyze all sources
```

## Tasks

### `kritExportModel`

Exports resolved compile classpaths for krit. Each applied JVM, Android, or Kotlin
Multiplatform project
registers one `kritExportModel` task and writes one file under the build root's
`.krit/gradle-model/` directory. The root project's file is `_root.json`;
`:feature:login` writes `feature__login.json`. Running `gradle kritExportModel`
selects the task in every project where it exists through ordinary Gradle task-name
selection. No separate aggregate step is needed. Each file contains that project's
entry, with external artifacts and project dependencies recorded separately. Schema 2
contains a `sourceSets` list (schema 1, written by older plugin versions, is the same
without each source set's `kotlin` object). JVM exports `main`, `test`, and (when enabled)
`testFixtures`. Android (`com.android.application`, `com.android.library`,
`com.android.dynamic-feature` and `com.android.test`) exports the selected production
variant plus available unit and instrumented test variants. A `com.android.test` module
(instrumentation, benchmark or baseline-profile tests) has only `main`, exported as kind
`main` with its `targetProjectPath` in `projectDeps`. Kotlin Multiplatform exports the `main` and `test`
compilations of JVM and Android JVM targets; other targets are skipped. Android uses
`debug` or the alphabetically first available production variant if it is absent.

Each source-set entry retains `name`, `platform`, `variant`, `sourceDirs`, `classpath`,
`bootClasspath`, and `projectDeps`, and adds `kind` (`main`, `test`, `testFixtures`, or
`androidTest`), `generatedSourceDirs`, `generatedClasspath`, `jvmTarget`, and `kotlin`: the
compilation's `languageVersion`, `apiVersion`, `optIn`, `freeCompilerArgs`,
`progressiveMode` and the Kotlin Gradle plugin's `pluginVersion`, each empty or `false`
when not configured (krit computes the effective values; see
[Kotlin compiler settings](../docs/kotlin-compiler-settings.md)).
`sourceDirs` excludes paths beneath the project's build directory; those registered
paths appear in `generatedSourceDirs` even before files exist. Each file also records
`rootDir` and `gradleUserHome`, the build root and Gradle user home it was exported
under. krit rebases paths under them when the model is read somewhere else (see
[CI recipe: skip Gradle](#ci-recipe-skip-gradle-when-the-build-has-not-changed)). Android generated
class jars are discovered best-effort from compile artifact views. Missing values
are empty lists or an empty `jvmTarget` string.

```kotlin
krit {
    exportModel = true       // default: this project's kritCheck depends on its kritExportModel
    fir = true               // default: run the FIR checker pass during kritCheck
    exportGenerated = false  // default: export declared paths without running code generation
    androidVariant = "debug" // default; choose another Android variant if needed
    exportTests = true       // default; false exports (and resolves) production source sets only
}
```

With `exportTests = false`, test sources that a scan still reads (`kritCheck`'s default
`src/test/kotlin`, `kritCheckTest`, `kritCheckAll`) get no classpath from the model, so FIR
gates them. Use it when the scans leave test sources out.
Set `exportModel = false` to run `kritCheck` without generating the model.
The task can still be run explicitly.
Set `fir = false` to pass `--no-fir` to `kritCheck` when FIR is unavailable.
`sourceDirs` and `generatedSourceDirs` come from the build's own source-set APIs: AGP's
Variant API for Android (`variant.sources.java.all` and `variant.sources.kotlin.all`,
AGP 8.0+), and the Kotlin plugin's `kotlin` and `generatedKotlin` source sets for JVM and
Kotlin Multiplatform. Those include every root a generator registered with the build,
such as view binding and data binding, BuildConfig, AIDL, SQLDelight, Wire and KSP, so
the model lists generated roots even before they exist.

The roots carry the tasks that produce them. With `fir = true` (the default), every check
task (`kritCheck`, `kritCheck<Variant>`, `kritCheck<SourceSet>`, each by its own `fir`)
depends on the producers of each production source set's roots, plus the Android R-class
tasks and kapt stub tasks, so FIR compiles against generated code on an unbuilt checkout.
`kritCheckAll` runs them too when its own FIR is on: it then selects each project's model as
built by that project's producers, and the root project's own producers run before the root
scan. A `kritCheckAll` with FIR off runs none. `kritBaseline` runs them like `kritCheck`. The
tasks are generators, not the compile of the analyzed source set. KSP and kapt take their
compilation's classpath as an input, though, so in a multi-module build they can still bring in
the compiles of upstream projects. Test source sets' generated roots are
recorded but not produced, because their KSP/kapt runs against the compiled production
classes. Generated Android resources need no wiring of their own: AGP's R-class tasks
already depend on every `variant.sources.res` producer. Set `exportGenerated = true` to
run the same generators for `kritExportModel` on its own, so generated sources and
R-class jars exist on disk after export.

#### CI recipe: FIR on an unbuilt checkout

`./gradlew kritCheck` runs the generators FIR needs without compiling. To run krit from
the CLI instead, export with `exportGenerated = true` in the analyzed projects (for example
`krit { exportGenerated = providers.environmentVariable("CI").isPresent }`):

```bash
./gradlew kritExportModel          # runs generators only; no compile tasks
krit --fir -v . 2>&1 | grep -E 'FIR verdict|FIR gated'
```

The `FIR verdict` line counts authoritative and gated files, and each `FIR gated` line
names a file's first compile error. An `Unresolved reference` to a generated package
(`databinding`, a SQLDelight database package, `BuildConfig`) that remains means a
generator that registers its output outside the Variant API, or one that writes Java
sources, which the FIR check does not compile yet (#1626).

### `kritCheck`

Runs krit analysis on all configured Kotlin sources. Produces a SARIF report at `build/reports/krit/krit.sarif`. Wired into the `check` lifecycle automatically.

```bash
./gradlew kritCheck
```

### `kritCheckAll`

One krit scan of the whole build: a single krit process, and so a single FIR
session, over the build root, reading every project's `kritExportModel` output.
Registered on the root project by `dev.jasonpearson.krit.aggregate`, which the
[settings plugin](../krit-settings-plugin/README.md) applies automatically.
Without the settings plugin, apply it to the root project and list the projects:

```kotlin
// build.gradle.kts (root)
plugins { id("dev.jasonpearson.krit.aggregate") version "0.5.0" }

dependencies {
    kritAggregate(project(":app"))
    kritAggregate(project(":lib"))
}
```

Each project with the krit plugin publishes its model on a `kritModelElements`
variant, and `kritCheckAll` resolves them through these project dependencies.
That way it never reads another project's state, and it stays configuration
cache and Isolated Projects safe. A listed project without the krit plugin is
skipped. The root `krit { }` block configures the scan (config, baseline, FIR,
binary, reports). Reports go to `build/reports/krit/all/`. Each finding in the JSON
and SARIF reports names its project (`project`, `properties.project`), and the
reports count findings per project (`byProject`, see
[configuration](../docs/configuration.md)). `kritCheckAll` logs those
counts:

```
krit: findings by project
  :feature:login           3 (1 error, 2 warnings)
  :app                     1 (0 errors, 1 warning)
  (outside every project)  1 (0 errors, 1 warning)
```

The task is up to date until a model, or a Kotlin, Java, XML, Gradle, TOML or
`krit.yml` file under the root (outside `build/`, `.gradle/` and `.krit/`), changes.
`advanced.moduleShards` passes `KRIT_MODULE_SHARDS` to the scan
(`"auto"` or a worker count analyzes modules on several worker JVMs against their
built dependency jars; see [module sharding](../docs/module-sharding.md)). Unset,
krit compiles the whole repository in one session.

**Which task should CI use?** `kritCheckAll`. Per-project `kritCheck` starts one
krit process and FIR JVM per project, and compiles unbuilt upstream projects'
sources again for each dependent project. Keep `kritCheck` for single-module work.

### `kritFormat`

Applies krit auto-fixes to Kotlin sources. The fix level is controlled by the `fixLevel` extension property.

```bash
./gradlew kritFormat
```

### `kritBaseline`

Creates a baseline file that captures all current findings. Subsequent `kritCheck` runs with the baseline configured will only report new issues.

```bash
./gradlew kritBaseline
```

Then reference the baseline in your configuration:

```kotlin
krit {
    baseline = file("build/reports/krit/baseline.xml")
}
```

## Configuration cache and Isolated Projects

`kritCheck`, `kritCheck<SourceSet>`, `kritBaseline`, `kritFormat`, `kritExportModel` and
`kritCheckAll` store and reuse the configuration cache with Isolated Projects on
(`org.gradle.isolated-projects=true`), with no reported problems. TestKit tests in
`src/test` check each one in a multi-project build. Task actions read only their
own properties, and every location, such as the default SARIF report, is wired at
configuration time.

## FIR: JDK and krit-fir jar

FIR runs on a [Java toolchain](https://docs.gradle.org/current/userguide/toolchains.html),
not on whatever `JAVA_HOME` the Gradle daemon has. The default is a toolchain for
the newer of Java 21 (FIR's minimum) and the JVM Gradle runs on. A build whose
Gradle runs on JDK 17 then gets a JDK 21 from Gradle's toolchain detection or
provisioning. Pick another one with:

```kotlin
krit {
    fir { javaLauncher = javaToolchains.launcherFor { languageVersion = JavaLanguageVersion.of(25) } }
}
```

The krit-fir jar can be a normal Gradle dependency in the `kritFir` configuration.
Declare it to pin a version: `dependencies { kritFir("dev.jasonpearson.krit:krit-fir:0.5.0") }`.
It then resolves from the project's repositories, so offline builds, enterprise mirrors and
[dependency verification](https://docs.gradle.org/current/userguide/dependency_verification.html)
apply as for any other jar, and a pin that can't resolve fails the build. With
nothing declared, the plugin resolves `dev.jasonpearson.krit:krit-fir:<krit version>`
from the project's repositories when one of them has it. Otherwise krit downloads
the jar itself, as before: from GitHub or Maven Central, or from the mirror
`KRIT_JAR_REPOSITORY` names, and with that variable set the plugin leaves the
download to krit. With `advanced.binary` set there is no default (the binary's
version is unknown), and krit locates the jar that matches its own build. A
`KRIT_FIR_JAR` environment variable overrides all of these. The jar and the JDK
(its version and vendor, not its path) are task inputs. With `fir = false`,
neither is resolved.

Outside a `krit { }` block, `krit.fir` is the on/off switch; the options are
`krit.firOptions` (for example `krit.firOptions.javaLauncher`).

### Built project dependencies

By default, a per-project `kritCheck` with FIR has krit compile the sources of
every project the checked project depends on, transitively, as context. In a deep
build that repeats the same upstream compiles in every dependent check. Opt in to
resolve project dependencies from their compiled classes instead:

```kotlin
krit { fir { useBuiltDependencies = true } }
```

`kritCheck` then depends on the upstream compile tasks through the exported
source sets' compile classpaths (jars for JVM projects, AGP's
`android-classes-jar` for Android libraries), so those results come from the build
cache when available. krit compiles only the checked project's own sources, and
reads the upstream classes from `--gradle-project-outputs`. The classes are an
ABI-normalized input: an upstream change that keeps the ABI, such as a method body,
does not rerun the check. Projects of included builds are left to krit as
before. The classpath is the one Gradle compiles with, so
upstream implementation dependencies are not visible, as in a normal compile.
This needs a krit release after 0.5.0.

### Shared FIR JVMs

krit keeps a FIR JVM warm between runs, but it starts one per source-dir set, so
the per-project checks of an N-project build used to leave N JVMs running (each
up to a quarter of memory). The plugin's `kritFirDaemonPool` build service has
the build's krit tasks share a small pool instead. Each task leases a slot for
its krit run (`KRIT_FIR_DAEMON_POOL`), and every request retargets that JVM to
the project at hand. The JVMs stay warm for later builds of the same root and
exit after 30 idle minutes.

```kotlin
krit {
    fir {
        maxDaemons = 2   // default: warm JVMs, and the most FIR tasks that run at once
        maxHeap = "4g"   // default: a quarter of memory per JVM
    }
}
```

`maxDaemons` is capped by `--max-workers`. Projects share a JVM only when they
agree on the JVM target, Kotlin compiler settings (including compiler plugins) and
the JDK, so a build that mixes, say, Compose and plain modules keeps one warm JVM
per slot and setting. `maxHeap` applies to the JVMs a run starts. Configure both the
same way in every project (for example from a convention plugin): the build
service takes them from the first project that applies krit. When the extension
turns FIR off, krit tasks don't use the pool and run without its limit; a single
task's `fir = false` skips the lease but still counts toward the limit. The pool
needs a krit release after 0.5.0; older ones ignore it.

#### CI recipe: skip Gradle when the build has not changed

A CI job that only needs the model can skip Gradle when no build input changed.
`krit gradle-model key` prints a key that hashes those inputs: settings and build
scripts, version catalogs, lockfiles, `gradle/verification-metadata.xml`,
`gradle.properties`, `local.properties`, the wrapper's Gradle version, and the `src/main`
sources of `buildSrc`, `build-logic` and every build nested under the root (any directory
with its own settings file, such as `includeBuild("gradle/plugins")`). It covers root-relative paths and file contents only, so
checkouts at different paths get the same key (`krit gradle-model key --list` shows the
files). Cache `.krit/gradle-model` under it, together with the Gradle user home's
dependency caches the model's jars live in, which `gradle/actions/setup-gradle` already
restores:

```yaml
- uses: gradle/actions/setup-gradle@<sha> # restores ~/.gradle/caches
- id: model-key
  run: echo "key=$(krit gradle-model key)" >> "$GITHUB_OUTPUT"
- id: model
  uses: actions/cache@<sha>
  with:
    path: .krit/gradle-model
    key: krit-gradle-model-${{ runner.os }}-${{ steps.model-key.outputs.key }}
- if: steps.model.outputs.cache-hit != 'true'
  run: ./gradlew kritExportModel && krit gradle-model key --write
- run: krit --fir .
```

`--write` stamps the key, with the content hash of every file it covers, in
`.krit/gradle-model/inputs.key`. From then on krit compares the build with the stamp,
not with modification times (a restored model always looks newer). It re-hashes the
files the stamp lists, without walking the tree, and warns
`gradle model … is stale: <file> changed since it was exported` when one differs.
`kritExportModel` deletes the stamp whenever it rewrites a model. A model restored into another checkout path, or onto a machine with another
`GRADLE_USER_HOME`, still resolves. krit rebases paths under the recorded `rootDir` onto
the directory that holds `.krit/gradle-model`, and paths under a recorded
`gradleUserHome` that no longer exists onto `$GRADLE_USER_HOME` (default `~/.gradle`).
Missing jars (a Gradle cache that wasn't restored) produce the usual missing-classpath
warning. Rerun `kritExportModel` then.

The key doesn't cover inputs outside the build's files: environment variables or
system properties that build scripts read, plugins resolved through dynamic versions
without a lockfile, and included builds outside the root (`includeBuild("../shared")`).
A build that depends on those should add them to the cache key. A nested build that
isn't included, such as a sample, still counts as build logic: editing its sources
changes the key and costs one re-export. The stale check sees changes to the files the
stamp lists. A build file added later changes the key, so a cache keyed on it misses.

## Build cache

`kritCheck` and `kritCheckAll` are cacheable, and their cache keys are relocatable:
the same build checked out at another path (another CI agent, a developer machine)
reuses their entries. The exported model is keyed with its paths made location
independent (relative to the build root or the Gradle caches), so each source
set's classpath keeps its identity, and the jars it lists are keyed by content. A
dependency re-published at the same path, such as a SNAPSHOT, reruns the check. With FIR on,
the krit-fir jar and the FIR JDK are inputs too. `advanced.parallel` and `advanced.noCache`
are not inputs because they don't change the findings. Report paths are relative
to the project directory (the build root for `kritCheckAll`), so a replayed
report is valid in the new checkout.

Not keyed yet: for per-project `kritCheck`, upstream projects' sources and models,
unless `fir { useBuiltDependencies = true }` makes their compiled classes an input
(see [Built project dependencies](#built-project-dependencies)). A per-project
check reads the whole `.krit/gradle-model` directory. Use `kritCheckAll`, whose
key covers every project's model and sources, or clean the entries when an
upstream project changes.

## Binary Resolution

The plugin downloads the correct platform-specific krit binary from GitHub Releases and caches it in `~/.gradle/krit/`. Supported platforms: `darwin-arm64`, `darwin-amd64`, `linux-arm64`, `linux-amd64`, `windows-amd64`. To skip the download and use a local binary, set `advanced.binary` (see [Advanced / escape hatches](#advanced--escape-hatches) above).

## Requirements

- Gradle 8.0+
- JDK 17+ for the plugin's JVM 17 bytecode. FIR runs on a Java 21+ toolchain (see [FIR: JDK and krit-fir jar](#fir-jdk-and-krit-fir-jar)); set `fir = false` for Go-only analysis.
