# AutoMobile baseline, 2026-10-27 (noisy run)

> **Noisy run. Treat these numbers as indicative only.** The machine was a
> shared 4-vCPU cloud container with other agent sessions building and testing
> at the same time. The 1-minute load average ranged from about 2 to 34 during
> the run, and the memory cgroup OOM-killed some processes, including three
> krit runs. The budgets proposed below come from these medians, so they are
> deliberately loose. Re-derive them on a quiet machine before anything
> enforces them.

This run is for #787. It uses the `internal/devtools/benchanalysis` harness
described in [benchmarking.md](../benchmarking.md), and covers all four modes
in all five cache states. The raw data is in
[`2026-10-27-automobile-noisy/`](2026-10-27-automobile-noisy/):

- `*.results.json`: the harness output with every run, minified.
- `*.summary.md`: the harness summaries.
- `budget-inputs.csv`: the per-state spread and budget inputs used below.
- `conditions.txt`: `uptime` and `free -m` before and after each invocation.

## Machine and inputs

| Item | Value |
|---|---|
| Machine | 4 vCPU Intel Xeon @ 2.10 GHz, 16 GB RAM, no swap. Shared container with other sessions running `go test`, builds and krit daemons |
| Load average (1/5/15 min) | 3.32 / 4.93 / 7.01 at the start. Peaked near 34 (1 min) during the first structural invocation. About 2.5–3.5 during the FIR invocations. Each run's own reading is in `loadAvg` |
| Free memory | 1.5 GB free and 2.8 GB available at the start. 11 GB free at the end. The memory cgroup OOM-killed four krit processes between 15:54 and 16:10 UTC (see "Rejected runs") |
| Krit | `ee8369ac` (the `go build` VCS stamp in the binary, which was not modified), binary SHA-256 `7aba8efc…` |
| Helper jars | `krit-fir.jar` `ed864bea…` and `krit-types.jar` `3a2ded77…` (from `make fir-jar types-jar`, passed through `KRIT_FIR_JAR` and `KRIT_TYPES_JAR`) |
| Toolchain | Go 1.27.1, OpenJDK 21.0.12.1 (no Leyden AOT), Kotlin 2.4.20 |
| Corpus | `kaeawc/auto-mobile@5d51ab06` (shallow clone), scanned from the repo root |
| Corpus config | An untracked `krit.yml` that sets `oracle.classpath` to kotlinx-coroutines-core-jvm 1.8.0, kotlinx-serialization-core-jvm 1.11.0 and kotlinx-serialization-json-jvm 1.11.0. There is no exported Gradle model, so Android and Compose symbols stay unresolved |
| Edit file | `android/test-plan-validation/src/main/kotlin/dev/jasonpearson/automobile/validation/ToolResultModels.kt` |

The `structural.results.json` metadata names krit revision `1f5b7b8c`. That
value is wrong. The harness read `git rev-parse HEAD` from the directory it was
started in, which was a different checkout. The binary was the same `ee8369ac`
build, as its SHA-256 confirms. The harness now reads the revision from the
binary's VCS stamp (this PR), so the later invocations record `ee8369ac`.

## Commands

Each invocation used its own `KRIT_DAEMON_REGISTRY_DIR` and `-out` directory:

```bash
export KRIT_FIR_JAR=~/.krit/jars/dev/<hash>/krit-fir.jar
export KRIT_TYPES_JAR=~/.krit/jars/dev/<hash>/krit-types.jar
go build -o krit ./cmd/krit/
go run ./internal/devtools/benchanalysis -corpus <auto-mobile> -krit ./krit \
  -modes <mode> -states <states> -edit-file <edit file> -runs <n> -out <dir>
```

| File set | Mode | States | `-runs` | Started (UTC) |
|---|---|---|---|---|
| `structural` | structural | all five | 5 | 15:50 (heaviest load) |
| `kaa-oracle` | kaa-oracle | all five | 5 | 17:12 |
| `structural-rerun` | structural | warm-disk | 5 | 18:45 |
| `fir-oracle` | fir-oracle | all five | 3 | 18:49 |
| `fir-checkers` | fir-checkers | all five | 3 | 19:40 |
| `structural-cold-rerun` | structural | cold | 5 | 20:40 |
| `kaa-oracle-cold-rerun` | kaa-oracle | cold | 3 | 20:42 |

`-verify` was on for every invocation. Each non-cold run was checked against a
clean scan of the same contents.

- **Fewer repeats for FIR.** The FIR modes use 3 runs because each verified
  run costs a full cold scan, which took about 2.5 minutes here.
- **Reruns.** `structural` was repeated because its cold runs ran at load
  20–34 and its warm-disk fill run was OOM-killed. `kaa-oracle` cold was
  repeated to check that its unexpectedly high cost is real.
- **Aborted start.** `conditions.txt` shows a `kaa-oracle` start at 16:30 that
  was aborted. Its results were discarded.

## Results

All times are wall-clock seconds over accepted measured runs, using the
harness's nearest-rank percentiles. With 3 runs, p25, p75 and p90 are just the
minimum, middle and maximum values.

| mode | state | accepted/measured | min | p25 | median | p75 | p90 | max | load1 range |
|---|---|---|---|---|---|---|---|---|---|
| structural | cold | 5/5 | 30.9 | 31.3 | 31.7 | 32.0 | 32.7 | 32.7 | 2.1–3.5 |
| structural | warm-disk | 5/5 | 2.8 | 2.9 | 3.1 | 3.1 | 5.2 | 5.2 | 3.3–4.0 |
| structural | warm-daemon | 5/5 | 0.1 | 0.1 | 0.2 | 0.2 | 17.2 | 17.2 | 9.4–22.0 |
| structural | body-edit | 5/5 | 0.3 | 0.3 | 0.3 | 0.4 | 0.5 | 0.5 | 6.8–12.4 |
| structural | abi-edit | 5/5 | 0.3 | 0.3 | 0.3 | 0.3 | 0.4 | 0.4 | 7.1–11.6 |
| kaa-oracle | cold | 8/8 | 144.4 | 147.2 | 149.4 | 151.0 | 154.7 | 154.7 | 2.9–6.1 |
| kaa-oracle | warm-disk | 5/5 | 4.6 | 4.6 | 4.6 | 4.7 | 5.4 | 5.4 | 5.0–5.5 |
| kaa-oracle | warm-daemon | 0/5 | rejected: daemon findings differ from a clean scan | | | | | | |
| kaa-oracle | body-edit | 0/5 | rejected: daemon findings differ from a clean scan | | | | | | |
| kaa-oracle | abi-edit | 0/5 | rejected: daemon findings differ from a clean scan | | | | | | |
| fir-oracle | cold | 3/3 | 130.4 | 130.4 | 131.7 | 132.3 | 132.3 | 132.3 | 3.1–3.4 |
| fir-oracle | warm-disk | 3/3 | 6.8 | 6.8 | 7.5 | 7.7 | 7.7 | 7.7 | 3.0–3.3 |
| fir-oracle | warm-daemon | 3/3 | 2.9 | 2.9 | 3.1 | 3.2 | 3.2 | 3.2 | 2.6–3.3 |
| fir-oracle | body-edit | 3/3 | 56.3 | 56.3 | 59.9 | 74.5 | 74.5 | 74.5 | 3.0–3.1 |
| fir-oracle | abi-edit | 3/3 | 69.2 | 69.2 | 70.2 | 71.3 | 71.3 | 71.3 | 2.9–3.3 |
| fir-checkers | cold | 3/3 | 138.9 | 138.9 | 139.6 | 141.9 | 141.9 | 141.9 | 3.0–3.2 |
| fir-checkers | warm-disk | 3/3 | 8.4 | 8.4 | 8.8 | 9.1 | 9.1 | 9.1 | 2.9–3.2 |
| fir-checkers | warm-daemon | 3/3 | 68.1 | 68.1 | 69.3 | 70.6 | 70.6 | 70.6 | 2.9–3.4 |
| fir-checkers | body-edit | 3/3 | 71.9 | 71.9 | 76.4 | 79.3 | 79.3 | 79.3 | 2.7–3.3 |
| fir-checkers | abi-edit | 3/3 | 80.2 | 80.2 | 84.6 | 87.2 | 87.2 | 87.2 | 2.9–3.2 |

Where each row comes from:

- **structural cold** is from `structural-cold-rerun`. The first invocation,
  run at load 20–34, measured a median of 145.3 s (3 of 5 accepted, 123–312 s).
  That is about 4.6× the quieter result. Keep this in mind when reading any
  number taken under load.
- **structural warm-disk** is from `structural-rerun`.
- **structural daemon and edit rows** are from the first, heavily loaded
  `structural` invocation.
- **kaa-oracle cold** pools both kaa invocations: 5 runs at 144–155 s and 3
  runs at 147–152 s.

CPU and memory:

| mode | state | median CPU s (krit + waited helpers) | median peak RSS MB (krit + waited helpers) | median daemon CPU s | median daemon peak RSS MB | findings | oracle |
|---|---|---|---|---|---|---|---|
| structural | cold | 102.5 | 1868 | n/a | n/a | 3890 | n/a |
| structural | warm-disk | 5.0 | 661 | n/a | n/a | 3890 | n/a |
| structural | warm-daemon | 0.1 | 27 | 0.0 | 763 | 3890 | n/a |
| structural | body-edit | 0.1 | 27 | 0.2 | 2043 | 3890 | n/a |
| structural | abi-edit | 0.1 | 27 | 0.2 | 1983 | 3891 | n/a |
| kaa-oracle | cold | 535.9 | 3785 | n/a | n/a | 3845 | ran |
| kaa-oracle | warm-disk | 7.6 | 1038 | n/a | n/a | 3845 | findings-bundle-hit |
| fir-oracle | cold | 378.6 | 2931 | n/a | n/a | 4047 | ran |
| fir-oracle | warm-disk | 12.5 | 1447 | n/a | n/a | 4047 | findings-bundle-hit |
| fir-oracle | warm-daemon | 0.1 | 27 | 4.3 | 5403 | 4047 | findings-bundle-hit |
| fir-oracle | body-edit | 0.1 | 27 | 110.8 | 6179 | 4047 | ran |
| fir-oracle | abi-edit | 0.1 | 27 | 152.8 | 6134 | 4048 | ran |
| fir-checkers | cold | 393.6 | 3066 | n/a | n/a | 4010 | ran |
| fir-checkers | warm-disk | 18.6 | 1317 | n/a | n/a | 4010 | findings-bundle-hit |
| fir-checkers | warm-daemon | 0.2 | 38 | 146.5 | 6875 | 4010 | ran |
| fir-checkers | body-edit | 0.1 | 38 | 151.8 | 6066 | 4010 | ran |
| fir-checkers | abi-edit | 0.2 | 38 | 170.8 | 6258 | 4011 | ran |

Daemon CPU covers the long-lived daemons: `krit serve` plus the JVM helpers it
started. It is measured as a delta over the run. Daemon RSS is the sum of the
daemons' lifetime VmHWM, so it includes the unmeasured fill run. Every
accepted non-cold run matched its clean-scan checksum.

### Rejected runs

- **OOM kills, structural.** cold runs 1 and 5 and the warm-disk fill run
  exited with -1 (killed by a signal). The kernel log shows the memory cgroup
  OOM-killing `krit` processes of 1.5–1.8 GB RSS at the same times, while other
  sessions were also using the container's memory. The warm-disk state was
  then re-run (`structural-rerun`).
- **Daemon parity, kaa-oracle.** All 15 warm-daemon, body-edit and abi-edit
  runs were rejected. The daemon reports one more finding than a clean
  in-process scan. This is a real difference, not noise, and it reproduces by
  hand on a fresh copy:
  - `krit --no-daemon --oracle-backend=kaa --no-fir .` reports 3845 findings.
  - The same scan served by the daemon (`perf: served by krit daemon`) reports
    3846.
  - The extra finding is `AbstractClassCanBeConcreteClass` on
    `android/build-logic/src/main/kotlin/dev/jasonpearson/automobile/buildlogic/test/JUnitRunStart.kt:19`
    (`abstract class JUnitRunStartService : BuildService<…>, LongSupplier`). It
    is a false positive: `BuildService.getParameters()` is abstract.
  - The rule gives up when a supertype doesn't resolve, which is what happens
    in-process. In the daemon path, the resolver or oracle facts evidently
    resolve those supertypes without their abstract members.
  - This needs its own issue. kaa-oracle has no daemon numbers until it is
    fixed.

## Bottleneck

- **The oracle JVM dominates every oracle mode.**
  - Cold `indexPhaseRun/typeOracle` medians: kaa 119.4 s of 149.1 s, fir-oracle
    100.3 s of 131.4 s, fir-checkers 104.4 s of 139.3 s.
  - That is 75–80% of wall time. Most of it is the helper process itself
    (`jvmAnalyze/kritTypesProcess`, 92–109 s).
  - The Go side matches structural cold (31.7 s): `crossFileAnalysis` about
    25 s and `dispatchPhaseRun` about 15.5 s, with the phases overlapping.
- **KAA is now slower than FIR on this corpus.** KAA cold takes 536 CPU-s
  against FIR's 379, and peaks at 3.8 GB RSS against 2.9 GB.
  - The thread-summed KAA timings point at diagnostics:
    `kotlinTimings/kotlinDiagnostics` is about 273 CPU-s of the 397 CPU-s
    `kotlinFileAnalysisSession` total.
  - The 2026-10-17 provisional baseline in #787 measured kaa cold at 42.7 s
    with a stdlib-only classpath. This run declares 3 kotlinx jars. The rerun
    at load about 3–5 reproduced 147–152 s, so the difference is not load.
  - The cause is not isolated. It could be the classpath, the krit-types
    changes since `4143cd21`, or both.
- **The FIR edit path still recompiles the module (#740).**
  - Body-edit costs about 60 s and abi-edit about 70 s for fir-oracle, with
    111–153 daemon CPU-s.
  - Almost all of that is `typeOracle/jvmAnalyze` inside the daemon (about
    53 s of the 60 s).
- **fir-checkers misses the daemon's warm path.**
  - With no edit at all, a warm-daemon `--fir` request takes 69 s and
    146 daemon CPU-s, and `typeOracle/jvmAnalyze` runs (about 58 s).
  - The findings-bundle cache reports a hit for that request.
  - fir-oracle in the same state is served from the bundle in 3.1 s.
  - So the daemon's `--fir` path reruns the FIR compile even when nothing
    changed. This needs its own issue.
- **Daemon memory (#810).**
  - The FIR daemons' lifetime peak RSS is 5.4–6.9 GB. That is krit serve plus
    krit-fir, and it includes the fill compile.
  - In-process FIR runs peak at 2.9–3.1 GB.
  - A resident FIR daemon on this corpus therefore needs about 2× the memory
    of a one-shot scan.
- **An outlier in structural warm-daemon.** Run 2 took 17.2 s. Almost all of
  it (16.5 s) was in `bundleHit/sourcePaths`, against 5–8 ms in the other
  runs. That run happened at load 17. It is worth checking on a quiet machine
  whether it repeats.

## Proposed budgets

These are a proposal only. Nothing enforces them yet.

**Rule.**

- **Wall time and CPU:** budget = median × 1.5.
- **Peak RSS:** budget = median × 1.25.
- **Floor:** any wall or CPU budget below 1 s is raised to 1 s, because
  process startup and scheduler jitter dominate sub-second runs.
- **Rounding:** every budget is rounded up to two significant figures.
- **Which CPU and RSS:**
  - In-process states (cold and warm-disk) budget the krit process: krit plus
    the helpers it waited for.
  - Daemon states budget the daemons. The client process does almost nothing
    in those states, so budgeting it would catch nothing.

**Why these factors.**

- Within a state on this run, the spread was mostly small. p90 was within 1.2×
  of the median, except where one outlier ran under heavy load (structural
  warm-disk 1.7×, structural warm-daemon 80×, fir-oracle body-edit 1.25×).
- Across load levels the spread was not small: the same structural cold scan
  took 31.7 s at load 2–3 and 145 s at load 20–34.
- 1.5× catches a real regression of 50% or more while tolerating moderate
  load. It cannot absorb a fully oversubscribed machine.
- RSS barely depends on CPU load, so it gets a tighter factor.

| mode | state | n | wall budget s | CPU budget s | RSS budget MB |
|---|---|---|---|---|---|
| structural | cold | 5 | 48 | 160 | 2400 |
| structural | warm-disk | 5 | 4.7 | 7.5 | 830 |
| structural | warm-daemon | 5 | 1 | 1 (daemon) | 960 (daemon) |
| structural | body-edit | 5 | 1 | 1 (daemon) | 2600 (daemon) |
| structural | abi-edit | 5 | 1 | 1 (daemon) | 2500 (daemon) |
| kaa-oracle | cold | 8 | 230 | 810 | 4800 |
| kaa-oracle | warm-disk | 5 | 7 | 12 | 1300 |
| kaa-oracle | warm-daemon | 0 | none: all runs rejected | | |
| kaa-oracle | body-edit | 0 | none: all runs rejected | | |
| kaa-oracle | abi-edit | 0 | none: all runs rejected | | |
| fir-oracle | cold | 3 | 200 | 570 | 3700 |
| fir-oracle | warm-disk | 3 | 12 | 19 | 1900 |
| fir-oracle | warm-daemon | 3 | 4.8 | 6.5 (daemon) | 6800 (daemon) |
| fir-oracle | body-edit | 3 | 90 | 170 (daemon) | 7800 (daemon) |
| fir-oracle | abi-edit | 3 | 110 | 230 (daemon) | 7700 (daemon) |
| fir-checkers | cold | 3 | 210 | 600 | 3900 |
| fir-checkers | warm-disk | 3 | 14 | 28 | 1700 |
| fir-checkers | warm-daemon | 3 | 110 | 220 (daemon) | 8600 (daemon) |
| fir-checkers | body-edit | 3 | 120 | 230 (daemon) | 7600 (daemon) |
| fir-checkers | abi-edit | 3 | 130 | 260 (daemon) | 7900 (daemon) |

**Caveats.**

- **Re-derive on a quiet machine.** Do it with at least 5 runs in every state
  and the same corpus revision, `krit.yml` and jars. On a quiet machine, a
  tighter factor such as 1.2× for time is reasonable. Until then, a budget
  breach here means "look", not "fail".
- **Daemon edit rows for structural.** These were measured at load 7–22. Their
  1 s floor, not their median, sets the budget.
- **Budgets that should shrink.** fir-checkers warm-daemon reflects the missed
  warm path described above, and kaa cold reflects the unexplained jump from
  the earlier baseline. Both budgets should come down once those are fixed.
  They are not targets.
- **No CI wiring.** These budgets are not wired into CI, and this PR doesn't
  propose doing so on shared runners. One cold FIR row takes about 2.5 minutes
  per run plus verification, and these measurements show the shared-machine
  variance is larger than any useful budget margin. A scheduled job on a
  dedicated runner, comparing against medians, would be the place for it.
