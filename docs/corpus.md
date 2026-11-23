# Public corpus benchmark

`internal/devtools/corpusbench` runs krit against a pinned set of public
Kotlin, Android, and KMP repositories and records whether each scan finishes
and how well it goes. Use it to check that a change helps or hurts on real
code, and to measure each rule and project-model fix before and after.

```bash
go build -o krit ./cmd/krit/
make corpus-bench                                # clone missing repos, run all
make corpus-bench CORPUS_ARGS="-only ktor,moshi" # a subset
go run ./internal/devtools/corpusbench -out corpus-new -compare corpus-out/results.json
```

## Corpus

`tests/corpus/repos.json` lists each repository with its URL, a pinned
commit, its build system, and a few shape tags. The current set covers:

| Shape | Repositories |
|---|---|
| Gradle Kotlin DSL libraries | okhttp, moshi, detekt |
| Kotlin Multiplatform | kotlinx.coroutines, ktor, kotlinx.serialization, KaMPKit |
| Android apps | nowinandroid, architecture-samples, compose-samples, element-x-android (Compose); iosched (views) |
| Gradle Groovy DSL | leakcanary, picasso |
| Bazel | rules_kotlin, bazelbuild/examples, rules_android |

Clones live outside this repository, in `$KRIT_CORPUS_ROOT` (default
`~/.krit/corpus`). With `-fetch`, the harness shallow-fetches exactly the
pinned commit and skips Git LFS payloads. To change a pin, edit the manifest
on purpose and put before/after numbers in the PR.

## What it measures

Each repository gets one cold scan (the harness deletes `<repo>/.krit` before
and after) with `-no-cache -no-daemon -no-type-oracle -no-fir -f json`, so no
JVM is involved and the numbers show krit's Go-only speed. Pass more flags
with `-krit-args`.

| Field | Meaning |
|---|---|
| `status` | `ok`, `crash` (exit code other than 0/1, or a panic on stderr), `timeout`, `bad-output` (invalid JSON), `missing` (no clone) |
| `wallMs`, `maxRssMB` | wall time and peak resident memory of the krit process |
| `syntaxErrorFiles` | Kotlin files whose tree-sitter tree has ERROR or MISSING nodes after krit's pre-parse rewrites. Findings inside those spans are dropped, so they are blind spots. Samples list the first error line of each file |
| `detectedBuild`, `modules`, `moduleCoverage` | the build system found at the root, the Gradle modules `module.DiscoverModules` sees (and how many have source roots), and the share of `.kt` files inside a discovered module |
| `androidManifests`, `catalogSources`, `dependencies`, `dependencyExtractionComplete` | what Android detection and the library profile found |
| `findingsByRule` | finding counts per rule |
| `warnings` | distinct `warning:` / `krit:` / `error` lines from stderr |

`-out DIR` writes `results.json` (the raw data), `summary.md` (totals, a row
per repository, findings by rule, and problems), and each repository's krit
JSON report and stderr. With `-compare`, every table also shows the earlier
run.

## Triage

To judge findings, sample them from the saved JSON report next to their
source lines, then mark each sample as a true positive, a false positive, or
noise (correct but not useful). The `krit-dogfood` and `krit-rule-vetting`
skills describe the workflow. Each false-positive fix still needs a
negative fixture or regression test in this repository, because the corpus
is not part of `make test`.
