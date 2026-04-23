=== Leyden AOT Benchmark — Signal-Android, JDK 25.0.2 ===
Date: 2026-04-23
Platform: Darwin arm64
Config: `-daemon -no-cache -q`, cold daemon start each run
Branch: work/magical-margulis-29302a (issue #143)
Binary: krit dev (worktree, Leyden AOT implementation)
JDK: openjdk version "25.0.2" 2026-01-20 (Homebrew)
Signal-Android: 2436 Kotlin files

## Headline numbers

| Phase | Description | Wall time (run 1) | Wall time (run 2) |
|-------|-------------|------------------:|------------------:|
| 1 (baseline) | No AOT files, cold JVM start | 35480ms | 35733ms |
| 2 (create) | Build AOT cache from .aotconf | 6566ms | — |
| 3 (use cache) | Load pre-linked classes from .aot | 3323ms | 3395ms |

**Speedup Phase 3 vs baseline: ~10.6×  (35.6s → 3.4s)**

## Phase lifecycle

### Phase 1 — record (no AOT files)
JVM flags added: `-XX:AOTMode=record -XX:AOTConfiguration=~/.krit/cache/krit-types-<hash>.aotconf`

The daemon starts normally and records which classes are loaded/linked.
When the daemon exits, the JVM writes the AOT configuration file.

- `.aotconf` written: 127,565,824 bytes (127 MB)
- Cold start time: ~35.5s

### Phase 2 — create (config exists, no cache)
`buildLeydenAOTCache()` runs `java -XX:AOTMode=create ...` synchronously before
launching the daemon. This AOT compilation step takes ~3s (included in the 6.5s
wall time for the combined create+first-use run).

- `.aot` cache written at `~/.krit/cache/krit-types-<hash>.aot`
- Run time: 6566ms (includes ~3s create step + analysis)

### Phase 3 — use cache (.aot exists)
JVM flags added: `-XX:AOTCache=~/.krit/cache/krit-types-<hash>.aot`

Classes are pre-linked; JVM initialization is dramatically faster.

- Steady-state cold start: ~3.4s  (vs ~35.5s without cache)
- Speedup: 10.6×

## Comparison to baseline (2026-04-22, JDK 21, AppCDS-less)

The April 22 baseline measured ~21s total with `kritTypesProcess ≈ 18s` under JDK 21
with no AppCDS. The JDK 25 + Leyden Phase 3 result is ~3.4s total — an 83% reduction.

Note: the baseline used the one-shot oracle path (not `-daemon`); the April 23 runs
use `-daemon` mode. Direct comparison requires controlling for mode, but the order-of-
magnitude reduction in JVM startup overhead is the expected Leyden AOT effect.

## Bug fixed during benchmark

Initial runs failed: combining `-XX:AOTMode=record` with `-XX:ArchiveClassesAtExit`
(AppCDS training) causes "Error occurred during initialization of VM" in JDK 25.
Fixed by gating AppCDS on `jdkMajor < 25` — Leyden subsumes AppCDS in JDK 25+.
Committed in the same PR as a follow-up fix.

## Cache file locations

Both caches are keyed by the JAR's content hash (first 12 hex chars of SHA-256):
- AppCDS (.jsa): JDK 13–24 only, `~/.krit/cache/krit-types-<hash>.jsa`
- Leyden config (.aotconf): JDK 25+, `~/.krit/cache/krit-types-<hash>.aotconf`
- Leyden cache (.aot): JDK 25+, `~/.krit/cache/krit-types-<hash>.aot`

When the JAR is updated (new krit-types build), all three hashes rotate and the
AOT lifecycle restarts from Phase 1 automatically.
