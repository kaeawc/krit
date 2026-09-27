# Corpus precision

Krit tracks the exact normalized findings from reference corpora in `.krit/corpus-snapshots/`. These committed snapshots make per-rule, per-line finding changes reviewable instead of relying only on broad count ranges. A separate, sparse label file in `.krit/corpus-labels/` turns reviewed findings into a per-rule precision measurement.

The harness always scans the in-repository `kotlin-webservice` and `android-app` playgrounds. It invokes the built `./krit` binary, so build it before running the harness:

```sh
go build -o krit ./cmd/krit/
```

## Check and update snapshots

Check current findings against every available committed snapshot:

```sh
go run ./internal/devtools/corpusprecision
# or
make corpus-snapshot
```

The check prints added and removed `file:line` locations grouped by corpus and rule and exits 1 when findings drift. If the change is intentional, regenerate the snapshots:

```sh
go run ./internal/devtools/corpusprecision --update
```

Use `--corpus` to restrict any mode to one named corpus:

```sh
go run ./internal/devtools/corpusprecision --update --corpus kotlin-webservice
```

Snapshots contain only the corpus name, a best-effort Git commit SHA, and sorted normalized findings. They contain no timestamp, duration, or absolute path, so two updates from unchanged inputs are byte-identical.

## Triage labels

Labels are a sparse JSON list. Add entries only for findings that have actually been reviewed:

```json
[
  {
    "rule": "MagicNumber",
    "relPath": "src/main/kotlin/com/example/Application.kt",
    "lineHash": "0123456789ab",
    "col": 1,
    "verdict": "tp",
    "note": "A non-domain numeric literal should be named."
  }
]
```

`verdict` must be one of:

- `tp`: a true positive.
- `fp`: a false positive.
- `unknown`: retained as a triage note but counted as unlabeled.

A label joins a current finding by the exact quadruple `(rule, relPath, lineHash, col)`. `lineHash` is the first 12 hexadecimal characters of the SHA-256 hash of the trimmed source line. This keeps a label attached when unrelated edits move that line, while a change to the finding's source line deliberately breaks the join and returns it to the unlabeled pool. There is no fuzzy matching. Copy the four signature fields from the corresponding snapshot entry when adding a label.

Run the precision report with:

```sh
go run ./internal/devtools/corpusprecision --precision
# or
make corpus-precision
```

For each labeled corpus, the report prints per-rule true positives, false positives, unlabeled findings, and `tp / (tp + fp)`. Precision is `n/a` when a rule has no `tp` or `fp` labels. The overall line aggregates those counts, and coverage is `(tp + fp) / all observed findings`. Missing labels and `unknown` verdicts count as unlabeled. A corpus without a label file is skipped with a note.

## Add an external corpus

External corpora follow the same opt-in environment-variable pattern as the repository's `KOTLIN_CORPUS` and `SIGNAL_ANDROID_CORPUS` tests. The `metro` entry is wired to `KRIT_CORPUS_METRO`; when the variable is unset, the harness silently omits that corpus.

For example, with a Metro checkout in a sibling directory:

```sh
KRIT_CORPUS_METRO=../metro go run ./internal/devtools/corpusprecision --update --corpus metro
KRIT_CORPUS_METRO=../metro go run ./internal/devtools/corpusprecision --precision --corpus metro
```

The environment variable may contain an absolute path at runtime, but local paths are never written into snapshots. Updating an available external corpus creates `.krit/corpus-snapshots/metro.json`; add `.krit/corpus-labels/metro.json` only when triage begins. Do not commit a machine-specific corpus path.

## Compiler parity

Compiler parity compares Krit's rule findings with Kotlin compiler diagnostics for the same defect, surfaced through Krit's JVM oracle. It tracks the rules whose compiler factory maps one-to-one onto the rule: `UnsafeCast` (`CAST_NEVER_SUCCEEDS`), `UselessElvisOnNonNull` (`USELESS_ELVIS`), `UnreachableCode` (`UNREACHABLE_CODE`), `UnnecessaryNotNullOperator` (`UNNECESSARY_NOT_NULL_ASSERTION`), `UnnecessarySafeCall` (`UNNECESSARY_SAFE_CALL`), and `Deprecation` (`DEPRECATION`). Both oracle backends must retain a factory before it is added here. See [fir-checker-candidates.md](fir-checker-candidates.md) for which compiler diagnostics can be projected.

The report runs with `--all-rules` so that opt-in mapped rules such as `Deprecation` produce findings. For rules that project the compiler's verdict, it measures the rule end to end, including how it anchors each diagnostic to a syntax node.

Run the report across every available corpus, or scope it to one corpus:

```sh
go run ./internal/devtools/corpusprecision --compiler-parity
go run ./internal/devtools/corpusprecision --compiler-parity --corpus kotlin-webservice
```

The mode requires a built `krit-fir` jar and fails loudly when it is unavailable. `agree` means a Go finding matched the compiler diagnostic, `go-only` is a likely false positive relative to the compiler, and `compiler-only` is a likely false negative. Agreement is `agree / (agree + go-only + compiler-only)`.

## FIR vs Go validation

Build the FIR checker jar with `cd tools/krit-fir && ./gradlew shadowJar`, then prepare each Gradle corpus with:

```sh
scripts/corpus-gradle-model.sh playground/kotlin-webservice
# For a corpus that needs another JDK:
scripts/corpus-gradle-model.sh "$KRIT_CORPUS_METRO" --java-home "$JDK_HOME"
make fir-validate
```

The model script builds the local Gradle plugin and applies it through a temporary init script. It writes classpaths to the corpus's `.krit/gradle-model/` and fails if Gradle cannot export them or tracked corpus files change. The Make target skips corpora without an exported model and external corpora whose `KRIT_CORPUS_<NAME>` variable is unset. Direct `go run ./internal/devtools/corpusprecision --fir-compare` also permits scans without a model; they use `--no-gradle-model` and have limited classpath coverage. Optional external variables are `KRIT_CORPUS_METRO`, `KRIT_CORPUS_SQLDELIGHT`, `KRIT_CORPUS_COIL`, and `KRIT_CORPUS_SIGNAL_ANDROID`.

Each `.krit/corpus-fir/<corpus>.json` records whether it used a Gradle model, the verbose FIR verdict, source-file coverage, and an independent join of the Go and FIR JSON findings by rule and location. `discrepancy` flags counts that differ. The merge uses range and same-line matching and leaves Go findings on gated or excluded files intact, so an exact location join can disagree. Up to 25 go-dropped and fir-added examples per rule carry the existing `(rule, relPath, lineHash, col)` label signature. `docs/fir-validation.md` aggregates verdict counts across corpora and shows gated and excluded coverage. `gated%` divides compiler-error, crash, and generated-source gated files by authoritative plus gated files. `excluded%` divides excluded files by all authoritative, gated, and excluded files. Per-rule `files-gated` in Krit's current verbose output is the **global** gated-file count, repeated for every rule; the harness records it as reported. A high agreement rate with low file coverage is not evidence that the Go implementation can be removed.
