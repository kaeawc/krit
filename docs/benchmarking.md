# Benchmarking analysis

`internal/devtools/benchanalysis` benchmarks krit on a local corpus. Every run
uses an explicit backend mode and a named cache state, and the harness checks
that the run did what was asked before it counts the timing.

```bash
go build -o krit ./cmd/krit/
make fir-jar types-jar            # only for the oracle and FIR modes
go run ./internal/devtools/benchanalysis \
    -corpus ~/src/AutoMobile \
    -modes structural,fir-oracle,kaa-oracle,fir-checkers \
    -states cold,warm-disk,warm-daemon,body-edit,abi-edit \
    -edit-file path/inside/corpus/Some.kt \
    -runs 7 -out bench-out
```

It writes `bench-out/results.json` (metadata and every run, raw) and
`bench-out/summary.md` (one row per mode and state).

## Modes

| Mode | krit arguments | Verified by |
|---|---|---|
| `structural` | `--no-type-oracle --no-fir` | no `oracleBackend` entry in the perf report |
| `fir-oracle` | `--oracle-backend=fir --no-fir` | `oracleBackend.backend == "fir"` |
| `kaa-oracle` | `--oracle-backend=kaa --no-fir` | `oracleBackend.backend == "kaa"` |
| `fir-checkers` | `--oracle-backend=fir --fir` | as `fir-oracle`, plus `firCheckOutcome.status == "ok"` |

`oracleBackend` (under `typeOracle/jvmAnalyze`) records the backend and jar
that actually served the oracle, after any fallback. `firCheckOutcome` (under
`firCheck`) records the FIR checker pass's file and rule counts and status.
A run that shows a different backend, or no oracle when one was asked for (a
missing jar, for example), is rejected rather than timed.

Compare modes only where the workload is the same. `structural` doesn't run a
JVM at all. `fir-oracle` and `kaa-oracle` produce the same facts with
different engines, so they compare directly. `fir-checkers` adds the rule
checkers: on a cold run they share the oracle's compilation (the
`firCheckRider` entry reads `shared`), and otherwise they add a second
compile.

## Cache states

`--no-cache` does not clear every layer, so each state says exactly what it
resets and what it keeps. Unless noted, the corpus is a copy under
`-out/corpus` (`-isolate`, on by default), so edits never touch the original.

| State | Reset before each measured run | Kept warm | Process |
|---|---|---|---|
| `cold` | `<corpus>/.krit` (incremental, parse, oracle, FIR and findings-bundle caches), the krit daemon, JVM helpers | nothing | fresh `--no-daemon` process, fresh JVMs |
| `warm-disk` | the krit daemon, JVM helpers | `<corpus>/.krit` | fresh `--no-daemon` process, fresh JVMs |
| `warm-daemon` | nothing | `.krit`, krit daemon, JVM helpers | warm daemon, no edit |
| `body-edit` | nothing | as `warm-daemon` | appends a comment line to `-edit-file` |
| `abi-edit` | nothing | as `warm-daemon` | appends a public top-level function to `-edit-file` |

Warm states run once, unmeasured, to fill the caches. JVM helpers always get a
private registry (`KRIT_DAEMON_REGISTRY_DIR`) and are scoped to the krit
process that started them (`KRIT_EPHEMERAL_DAEMONS=1`), so "fresh JVMs" really
means no reuse, and nothing the harness starts outlives it. Caches under
`~/.krit` (downloaded jars, AOT/CDS archives) are not reset; the metadata
records the jar hashes. FIR AOT stays off (see #768).

## Verification

Every non-cold run is checked against a clean scan: the harness copies the
corpus exactly as it was for that run into a new directory and scans it with
every layer and process cold. The findings checksum (file, line, column, rule
and message, with paths relative to the scan root, order ignored) must match.
A mismatch rejects the run and is reported. This is how the relative-root
affected-set replay bug was found.

A run is also rejected when krit exits with anything but 0 (no findings) or 1
(findings), when the JSON report can't be parsed, or when the mode check
fails. Rejected runs are listed in `summary.md` and kept in `results.json`,
and never count toward timings.

## What is recorded

- Per run: the exact command and krit-related environment, exit code, wall
  time, user and system CPU, peak RSS, load average, the JSON report's
  `durationMs`, every perf phase (as `parent/child` paths), perf attributes,
  per-cache hit and miss counts, the findings count and checksum, rule count,
  `error:`/`warning:` lines, and the applied edit.
- Metadata: start time, corpus path and git revision (and whether it's dirty),
  krit revision and binary hash and `--version`, SHA-256 of every helper jar
  krit could pick up, Go, Java and Kotlin versions, OS, CPU count, load
  average, the corpus's build files, and the harness options.

CPU and RSS cover the krit process and the children it waited for. Daemons
(the krit daemon and its JVM helpers in warm states) aren't included; their
cost shows up in wall time only.

## Noise

Run on an otherwise idle machine and check the recorded load averages. Use at
least five measured runs; the summary reports median, p90 and min–max.
Missing classpath entries (for example an Android project without its Gradle
model) change what the oracle resolves; check the jar and classpath warnings
recorded with each run before comparing results across machines.

The older scripts are still available: `scripts/benchmark-oracle.sh` (one cold
KAA run with the JVM-internal breakdown) and `scripts/bench-oracle-backend.sh`
(KAA versus FIR wall time).
