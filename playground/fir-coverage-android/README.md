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
| `:app` | `//app` | `com.android.application` with view binding (Bazel: generated into a `.srcjar` by `tools/viewbinding`) | #1574, #1626 |
| `:benchmark` | `//benchmark` | `com.android.test` module targeting `:app` | #1572 |

Versions: AGP 9.5.3 (built-in Kotlin), Kotlin 2.4.21, KSP 2.3.14, Wire 7.2.2,
Moshi 1.15.2, Gradle 9.9.2; Bazel 9.3.3 with rules_kotlin 2.4.21 and
rules_android 0.8.1. Keep `gradle/libs.versions.toml` and `MODULE.bazel` on
the same library versions.

## What CI measures

| Result | How |
| --- | --- |
| `gradle-model-cli` | `./gradlew kritExportModel`, then `krit --fir -v .` over the whole build: the plugin README's "FIR on an unbuilt checkout" recipe. |
| `gradle-kritCheck` | `./gradlew kritCheck --configuration-cache` on a clean checkout. Each project's `kritCheck` scans that project; the krit binary is `ci/krit-verbose.sh`, which runs krit with `-v` to capture the `FIR verdict` line. |
| `bazel` | `bazel build //...` and `bazel test //:krit_test`. `krit_test` is Go-only today, so this reports no FIR runs until the FIR aspect lands (#1587). |
| `gradle-configuration-cache` | A second `kritCheck` reuses the configuration cache. |
| `bazel-action-cache` | A second `bazel build //...` runs no actions. |

`ci/fir_coverage.py` sums the verbose `FIR verdict` lines, writes the table to
the job summary, and fails the `report` job when a result is below its entry
in `ci/fir-coverage-thresholds.json`, or a check marked `required` fails.

**Thresholds only go up.** When a fix raises coverage, the report lists the new
values; raise the thresholds in the same pull request. Never lower one to
get a green run: a drop is a regression to fix or an issue to file on #1594.

## Run it locally

Needs JDK 21, the Android SDK (`ANDROID_HOME`, platform 36 and build-tools
36.0.0), Go, and Bazelisk. From the repository root:

```bash
go build -o playground/fir-coverage-android/.krit-bin/dist/krit ./cmd/krit/
make fir-jar   # or set KRIT_FIR_JAR
cd playground/fir-coverage-android

# Gradle
./gradlew kritExportModel -PkritBinary="$PWD/.krit-bin/dist/krit"
.krit-bin/dist/krit --no-daemon --fir -v . 2>&1 | grep -E 'FIR verdict:|FIR gated'

# Bazel (uses .krit-bin/dist/krit through override_repo in MODULE.bazel)
bazel build //...
bazel test //:krit_test
```

Use `--no-daemon` (or stop `krit serve`) when you change `KRIT_FIR_JAR`: a
running project daemon keeps the environment it started with.
