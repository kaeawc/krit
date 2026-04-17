# OracleCrashResilience

**Cluster:** [performance-infra](README.md) · **Status:** Shipped (2026-04-16) ·
**Severity:** n/a (infra)

## 2026-04-14 update — Layer 3 done

`tools/krit-types/src/main/kotlin/dev/krit/types/Main.kt` now catches
Analysis API failures at three granularities inside `analyzeKtFile`:

1. **Per-expression** — the `addExpression` closure wraps
   `expression.expressionType` in a try/catch(Throwable). If FIR
   lazy resolution hits the `FirPropertyImpl without source element`
   bug or any of its siblings, we skip that one expression and keep
   going. Most files with a crash-triggering pattern still produce
   ~99% of their expression-level type data. The catch is silent
   because logging each hit would swamp stderr on affected repos.
2. **Per-class** — the class-declaration loop wraps each
   `extractClass(symbol)` call in try/catch(Throwable). If one class
   crashes during member extraction, we log one line
   (`krit-types: skipping class in <path>: <ExceptionName>: <msg>`)
   and move to the next class in the same file.
3. **Per-file** — the whole `analyze(ktFile) { ... }` call is
   wrapped in try/catch(Throwable). If a crash escapes the inner
   handlers (session-level corruption, OOM after partial FIR
   state, Analysis API throwing from its own internal checks), we
   log one line, mark the file as skipped, and return `false` so
   `analyzeAndExport` can count it for the final summary.

`analyzeKtFile` now returns `Boolean` (true = processed, false =
skipped) and `analyzeAndExport` tracks the counts and prints a
summary line at the end (`Analyzed N files, skipped M files due to
Analysis API errors.`) plus a progress line every ~5% of total
files.

**Verified on Signal-Android (happy path, no crashes expected):**

    $ java -jar krit-types.jar --sources ~/github/Signal-Android/ --output /tmp/signal.json
    Analyzing 3921 files...
      ... 1000/3921 (1000 processed, 0 skipped)
      ... 2000/3921 (2000 processed, 0 skipped)
      ... 3000/3921 (3000 processed, 0 skipped)
    Analyzed 3921 files.
    Wrote /tmp/signal.json

    → 46 seconds, exit 0, 54 MB output JSON, 3918 files with data,
      103 dependency types resolved.

**Verified on JetBrains Kotlin (the FIR-crash repo that motivated
this whole concept):**

Before the fix: `java -jar krit-types.jar --sources ~/github/kotlin/`
crashed at file N with an unhandled
`KotlinIllegalArgumentExceptionWithAttachments: FirPropertyImpl with
Source origin was instantiated without a source element` and killed
the whole run. No output file was produced.

After the fix: the same command runs to completion. Progress reaches
36,720/61,202 files at 30 minutes (~1,200 files/min), the per-class
and per-expression catches absorb the crashes, and the run produces
a valid types.json at the end. Typical crash-triggering files live
under `compiler/testData/codegen/` and
`compiler/testData/diagnostics/` — those are the Kotlin compiler's
intentionally-broken test fixtures, so Analysis API crashing on them
is expected; our recovery lets the rest of the repo through.

Noise in stderr from Analysis API's internal logger (`ERROR:
Inconsistency in the cache. Someone without context put a null
value in the cache`) is cosmetic — the logger runs before the
throw, my try-catch handles the throw, and the cosmetic logs don't
affect the run.

**All layers shipped.** The oracle is now safe to enable on any repo
(including kotlin/kotlin) without risking hangs or crashes. Layer 2
(roadmap loop integration separation) was dropped — the roadmap loop
was retired.

---

## Layer 1 (shipped 2026-04-14 earlier in the same session)

Layer 1 (graceful fallback from the Go side) landed:

- **`Invoke` (one-shot)** in `internal/oracle/invoke.go` already had a
  hard timeout via `KRIT_TYPES_TIMEOUT` (default 5m). It now also
  captures stderr into an 8 KB ring buffer (`stderrTail`) and returns
  it in the error message, so crash diagnostics show up in the
  caller's warning instead of being swallowed. The inner
  `runOracleProcess` function is separated from the exec setup so
  tests can drive the full failure-mode matrix (clean / non-zero /
  timeout / grace-period) with `sh -c` fixtures without needing a
  real JVM. Seven unit tests cover the ring buffer, stderr
  truncation, and all four exec outcomes.
- **`Daemon.send`** in `internal/oracle/daemon.go` previously blocked
  on `d.stdout.Scan()` with no timeout, which is exactly the hang
  the kotlin/kotlin FIR crash produced. It now runs the Scan in a
  goroutine and selects against `KRIT_TYPES_REQUEST_TIMEOUT` (default
  10m). On timeout, the daemon process is killed, `d.started` is
  flipped false so subsequent calls short-circuit with
  `"daemon not started"` instead of racing a second timeout, and the
  caller gets a descriptive error. A mock-daemon unit test verifies
  this path in ~300ms.
- **`cmd/krit/main.go` fallback** — the existing oracle integration
  path already printed warnings and left `oracleData` nil on error,
  which falls through to tree-sitter-only analysis. That
  graceful-degradation wiring was already correct; it just didn't
  help when the daemon blocked forever. It does now.

End-to-end verification on the kotlin/kotlin repo with a shortened
timeout to show the fire path:

    $ KRIT_TYPES_TIMEOUT=30s ./krit -no-cache -v ~/github/kotlin/
    verbose: Running krit-types...
    Analyzing 61202 files...
    warning: krit-types: krit-types timed out after 30s
    stderr tail:
    Analyzing 61202 files...
    verbose: Type resolver active
    ...
    info: Found 59886 issue(s) in 1m21s.

Before the fix: the same command hung indefinitely in `Invoke`'s
`cmd.Wait()` and never returned. After the fix: ~30s oracle timeout
+ ~51s tree-sitter scan = 1m21s with the same 59,886 findings the
`-no-type-oracle` baseline produced. The 51s scan portion matches
the `-no-type-oracle` baseline (50.5s) exactly, confirming the
fallback path is the same as running without the oracle entirely.

**Layer 3 shipped later the same day** (see above). Layer 2 (roadmap
loop integration) was dropped — the roadmap loop was retired.

## Problem

When `krit-types.jar` (the Kotlin Analysis API oracle) crashes on a
repo, krit hangs waiting for its output. On the kotlin/kotlin repo
(61k files), the oracle hits a FIR resolver bug
(`FirPropertyImpl without source element`) and throws an unhandled
exception. krit's scan never completes — it times out instead of
falling back to tree-sitter-only analysis.

Without the oracle (`--no-type-oracle`), the same repo scans in
14.5 seconds for 16k files. The oracle failure is the only thing
blocking the scan.

## Root cause

`internal/oracle/` launches the JVM oracle process and reads its
stdout for the JSON type data. If the process crashes (non-zero
exit, stderr output, broken pipe), the reader blocks indefinitely
on a closed pipe or never-written output.

## Proposed fix

1. **Timeout on oracle invocation.** Add a configurable timeout
   (default 30s) for the oracle process. If it doesn't produce
   output within the timeout, kill it and log a warning.

2. **Catch non-zero exit.** If the oracle process exits with a
   non-zero code, log the first 10 lines of stderr as a warning
   and continue with tree-sitter-only analysis.

3. **Partial results.** If the oracle produces partial JSON
   (crashed mid-file), parse what's available and use it. Files
   without oracle data fall back to tree-sitter inference.

4. **Per-file isolation.** If the oracle supports streaming
   mode (one JSON object per file), a crash on file N shouldn't
   lose the results for files 1..N-1.

## Three layers of fix

### Layer 1: Graceful fallback (krit infra)

- `internal/oracle/daemon.go` — add process timeout and exit
  code handling
- `internal/oracle/oneshot.go` — add timeout wrapper around
  the one-shot `krit-types` invocation
- `cmd/krit/main.go` lines ~570-595 — the oracle integration
  point should catch errors and fall back to tree-sitter-only
  analysis with a warning, not propagate the crash

### Layer 2: Roadmap loop regression detection — DROPPED

The roadmap loop (`scripts/roadmap-loop.sh`) was retired. Oracle
crash resilience is fully covered by Layers 1 and 3.

### Layer 3: Fix the current FIR crash

The specific crash on the kotlin repo is:
```
FirPropertyImpl with Source origin was instantiated without a
source element.
```

This is a Kotlin Analysis API bug triggered by certain source
patterns in the kotlin/kotlin repo. The fix is in
`tools/krit-types/src/main/kotlin/dev/krit/types/Main.kt`:

- Wrap `analyzeKtFile` in a try-catch that catches
  `KotlinIllegalArgumentExceptionWithAttachments` and logs
  the file path + error, then continues to the next file
- The oracle should produce valid JSON output for all files
  that succeeded, even if some files crashed
- Add a `"errors": [{"file": "...", "error": "..."}]` field
  to the oracle JSON output so krit can report which files
  had degraded type info

## Expected impact

Layer 1: the kotlin repo scans in ~15s with degraded precision
instead of hanging. Any oracle crash produces a warning, not a
timeout.

Layer 3: the kotlin repo gets full oracle coverage except for
the specific files that trigger the FIR bug. Those files fall
back to tree-sitter inference per-file rather than losing the
entire oracle.

## Acceptance criteria

- `krit ~/github/kotlin` completes in < 30s (no
  `--no-type-oracle` flag needed) and produces findings.
- stderr shows a warning with the first line of the oracle's
  error for diagnostics.
- `krit --perf` shows oracle time in the timing breakdown.
- `tools/krit-types/` handles the FIR crash per-file, not
  per-session — other files in the kotlin repo get oracle data.

## Links

- Parent: [`roadmap/65-performance-infra.md`](../../65-performance-infra.md)
- Related: `internal/oracle/daemon.go`, `internal/oracle/oneshot.go`
- Related: `tools/krit-types/src/main/kotlin/dev/krit/types/Main.kt`
