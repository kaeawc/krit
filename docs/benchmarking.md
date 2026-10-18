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

`oracleBackend` (under `typeOracle`, and under `typeOracle/jvmAnalyze` for an
in-process run) records the backend that actually served the oracle, after any
fallback. The in-process entry also records the jar; the entry written for a
resident daemon has `source=resident-daemon` and no jar, and is written only
after the daemon's analysis succeeded and its facts loaded. `firCheckOutcome` (under
`firCheck`) records the FIR checker pass's file and rule counts and status.
A run that shows a different backend, or no oracle when one was asked for (a
missing jar, for example), is rejected rather than timed.

One exception: a measured run in the `warm-disk` or `warm-daemon` state may
skip the oracle when the perf report shows the whole-run findings bundle served
the result. The evidence must be per request: `dispatchBundleLoad` outcome
`hit` or `preloaded` together with `dispatchPath` `cacheHitFullBundle`, or the
top-level `bundleHit` entry a daemon writes when it answers from its
response cache, but only with its `bundleOutcome` child marked `hit` (the
scope is also left behind by a lookup that missed). Delta and affected-set bundle hits, cache hit and miss
counters (which are lifetime totals of the process or daemon) and the edit
states never qualify, because those runs must still run the oracle. A skipped
run is accepted only with `-verify` on (its findings are then checked against
a clean scan), its `oracleSkip` field and the summary's `oracle` column record
the reason (for example `findings-bundle-hit`), and cold runs, unmeasured fill
runs and clean baselines never get the exception. So a missing jar still fails
the fill run that should have filled the caches, and the state is skipped.

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

### FIR AOT status (#768)

The FIR checker JVM launches without the Leyden AOT cache. The failure that
motivated this (a cache that passes the provenance checks but returns no
findings) could not be reproduced on JDK 25.0.4.1 with the current jar:
record, create, and reuse gave findings identical to a no-AOT run across
training workloads (startup only, one rule on one file, all 80 rules on 150
fixture files) and reuse workloads, including different heap flags and
working directories. Once AOT is re-enabled, a daemon launched from a cache is checked before it
is trusted, with the daemon's built-in `canary` request: krit-fir compiles one
tiny built-in source with a known `DefaultLocale` finding in an isolated
session. The cost is one small compile, independent of the project the daemon
serves (an earlier design reused the project's source dirs, which compiled the
whole project a second time). If the finding does not come back, or the jar
predates the request, the cache is discarded for the process and the request
is answered by a daemon launched without AOT. That daemon is not canaried; if
it registers none of the requested checkers the call fails with
`errFIRDegraded`.

Registry rules: an AOT-launched daemon is not registered until its canary
passes, so another krit process can neither reuse nor retire it mid-check; if
the process ends first, the daemon is closed. Every registered daemon carries
launch metadata (written atomically); a reconnecting run reuses a daemon only
if the metadata is readable for that PID and says non-AOT or verified, and
retires everything else, including daemons with missing or unreadable
metadata. Prerequisite for re-enabling AOT for the oracle-role krit-fir
daemons: they share this launch path but have no canary of their own, so an
AOT oracle-role daemon would never be verified and would be closed after each
use.

## Verification

Every non-cold run is checked against a clean scan: the harness copies the
corpus exactly as it was for that run into a new directory and scans it with
every layer and process cold. The findings checksum (file, line, column, rule
and message, with paths relative to the scan root, order ignored) must match.
A mismatch rejects the run and is reported. This is how the relative-root
affected-set replay bug was found.

A run in a daemon state is rejected when krit prints any `krit daemon ...
falling back to in-process` warning (unavailable, call failed, rejected
request) or when its oracle entry lacks `source=resident-daemon`, or when krit's stderr
lacks the `perf: served by krit daemon` marker it prints under `-perf` after a
delegated scan. The harness also refuses `-extra-args` containing `--no-daemon`
when a daemon state is selected. After an in-process fallback the per-run cache
baseline is kept, so the next daemon run's counters are still diffed correctly.
The daemon was then not used. The daemon's unix socket path must fit in
`sun_path` (about 100 bytes), so keep `-out` short. A run is also rejected when krit exits with anything but 0 (no findings) or 1
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

CPU and RSS cover the krit process and the children it waited for. Long-lived
daemons (krit serve, the krit-fir and krit-types oracle JVMs, the checker
daemon) are not children, so they are reported in separate columns: before and
after each run the harness reads `/proc/<pid>/stat` (utime+stime) and
`/proc/<pid>/status` (VmHWM, VmRSS) for every PID in the helper registry and
`.krit/daemon.pid`, plus any `krit serve --root <corpus>` process found by command line (auto-spawned daemons write no pidfile). "Daemon CPU ms" is the delta over the run (a daemon the run
started counts in full); "daemon lifetime-peak RSS MB" is the sum of the daemons'
lifetime VmHWM. Linux only; elsewhere the columns read `n/a`. A daemon that
exits during the run is not sampled. Raw values are in each run's `daemon`
object. In daemon states the per-cache hit and miss counts are the difference from the previous
run's reading, because a daemon's counters accumulate for its lifetime, so a stale
bundle hit is never taken as evidence for the current run.

## Noise

Run on an otherwise idle machine and check the recorded load averages. Use at
least five measured runs; the summary reports median, p90 and min–max.
Missing classpath entries (for example an Android project without its Gradle
model) change what the oracle resolves; check the jar and classpath warnings
recorded with each run before comparing results across machines.

The older scripts are still available: `scripts/benchmark-oracle.sh` (one cold
KAA run with the JVM-internal breakdown) and `scripts/bench-oracle-backend.sh`
(KAA versus FIR wall time).
