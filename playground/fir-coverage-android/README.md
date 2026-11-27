# FIR coverage sample (Gradle and Bazel)

A small multi-module Android project that builds with both Gradle and
Bazel. The [FIR coverage workflow](../../.github/workflows/fir-coverage.yml)
(#1579) analyzes it through krit's Gradle plugin and its Bazel module, and
checks that FIR keeps analyzing at least as many files as before. Coverage is
FIR-authoritative files / (authoritative + gated) files; a gated file is one
krit-fir could not compile, so its Go-only findings stand (#1594).

Each module covers a case that has broken FIR coverage before:

| Gradle project | Bazel package | Case | Issues |
| --- | --- | --- | --- |
| `:model` | `//model` | kotlinx-serialization compiler plugin; module-wide `-opt-in` | #1571, #1577 |
| `:proto` | `//proto` | Wire-generated Kotlin (Bazel: Wire CLI into a `.srcjar`) | #1574, #1586, #1588 |
| `:data` | `//data` | KSP (Moshi codegen) | #1574, #1586 |
| `:app` | `//app` | `com.android.application` with view binding (Bazel: generated into a `.srcjar` by `tools/viewbinding`), and Kotlin calling same-module Java | #1574, #1626 |
| `:benchmark` | `//benchmark` | `com.android.test` module targeting `:app` | #1572 |

Versions: AGP 9.5.3 (built-in Kotlin), Kotlin 2.4.21, KSP 2.3.14, Wire 7.2.2,
Moshi 1.15.2, Gradle 9.9.2; Bazel 9.3.3 with rules_kotlin 2.4.21 and
rules_android 0.8.1. Keep `gradle/libs.versions.toml` and `MODULE.bazel` on
the same library versions.

## What CI measures

| Result | How |
| --- | --- |
| `gradle-model-cli` | `./gradlew kritExportModel`, then `krit --fir -v .` over the whole build: the plugin README's "FIR on an unbuilt checkout" recipe. |
| `gradle-kritCheckAll` | `./gradlew kritCheckAll --configuration-cache -PkritExportGenerated=false` on a clean checkout: the root aggregate task, one krit scan of the whole build (#1580), which runs the generated-source producers itself (#1686). |
| `gradle-kritCheck` | `./gradlew kritCheck --configuration-cache -PkritExportGenerated=false` on a clean checkout: one krit scan per project, each running its project's generated-source producers. |
| `bazel-krit_aspect` | `bazel build //... --aspects=@krit//:defs.bzl%krit_aspect --output_groups=krit`: one FIR action per target (#1587), summed from each `<target>.krit.sarif`'s `properties.fir`. (`bazel test //:krit_test` also runs; it is Go-only.) |
| `gradle-configuration-cache` | A second `kritCheckAll` reuses the configuration cache. |
| `gradle-up-to-date` | That second `kritCheckAll` is up to date: krit does not run. |
| `bazel-action-cache` | A second `bazel build //...` runs no actions. |
| `bazel-krit_aspect-cache` | After `bazel clean`, every Krit action of a second aspect build is a disk-cache hit. |
| `findings-parity` | `kritCheckAll`'s SARIF and `krit_aspect`'s per-target SARIFs report the same findings, keyed by rule, sample-relative path and line, and at least one (#1677). `app/.../ParityProbe.kt` holds known findings: two from source alone, and a safe call and Elvis that rest on `:model`'s types, so FIR must resolve the full classpath on both sides. The report lists the keys only one side has. |

The Gradle runs use `ci/krit-verbose.sh` as the krit binary. The plugin runs
krit with `-q`; the wrapper runs it with `-v` instead, to capture the `FIR verdict`
line.

`ci/fir_coverage.py` sums the verbose `FIR verdict` lines (Gradle) or the
SARIF `properties.fir` records (Bazel), writes the table to
the job summary, and fails the `report` job when a result is below its entry
in `ci/fir-coverage-thresholds.json`, has more FIR checker errors or failed
Krit actions than its `maxCheckerErrors` (default 0), or a check marked
`required` fails.

**Thresholds only go up.** When a fix raises coverage, the report lists the new
values; raise the thresholds in the same pull request. Never lower one to
get a green run: a drop is a regression to fix or an issue to file on #1594.

## Run it locally

Needs JDK 21, the Android SDK (`ANDROID_HOME`, platform 36 and build-tools
36.0.0), Go, and Bazelisk. From the repository root:

```bash
go build -o playground/fir-coverage-android/.krit-bin/dist/krit ./cmd/krit/
make fir-jar   # writes ~/.krit/jars/dev/<hash>/krit-fir.jar; export KRIT_FIR_JAR to that path
cd playground/fir-coverage-android

# Gradle
./gradlew kritExportModel -PkritBinary="$PWD/.krit-bin/dist/krit"
.krit-bin/dist/krit --no-daemon --fir -v . 2>&1 | grep -E 'FIR verdict:|FIR gated'

# Bazel (uses .krit-bin/dist/krit and .krit-bin/dist/krit-fir.jar through
# override_repo of krit_toolchains and krit_fir in MODULE.bazel)
cp "${KRIT_FIR_JAR:?path to the krit-fir jar}" .krit-bin/dist/krit-fir.jar
bazel build //...
bazel test //:krit_test
bazel build //... --aspects=@krit//:defs.bzl%krit_aspect --output_groups=krit --@krit//:config=//:krit.yml
```

Use `--no-daemon` (or stop `krit serve`) when you change `KRIT_FIR_JAR`: a
running project daemon keeps the environment it started with.
