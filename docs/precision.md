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
    "verdict": "tp",
    "note": "A non-domain numeric literal should be named."
  }
]
```

`verdict` must be one of:

- `tp`: a true positive.
- `fp`: a false positive.
- `unknown`: retained as a triage note but counted as unlabeled.

A label joins a current finding by the exact triple `(rule, relPath, lineHash)`. `lineHash` is the first 12 hexadecimal characters of the SHA-256 hash of the trimmed source line. This keeps a label attached when unrelated edits move that line, while a change to the finding's source line deliberately breaks the join and returns it to the unlabeled pool. There is no fuzzy matching. Copy the three signature fields from the corresponding snapshot entry when adding a label.

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
