# Oracle optimization session — 2026-04-14/15 summary

Full session log of the krit-types oracle optimization push. Covers what
we tried, what shipped, what didn't, and what we learned.

## Session bookends

Started: krit-types on kotlin/kotlin took **43 minutes** cold.
Currently: projected **~1-2 seconds** warm on Signal-Android, **~5 minutes**
cold on kotlin, **target ~1-2 seconds** warm on kotlin (pending measurement).

## The shipped commits on main

In chronological order:

| SHA | Title | Measured impact |
|---|---|---|
| `a5b7651` | RFC-8259 JSON escaping | correctness on kotlin (parseable JSON output) |
| `e768768` | `--exclude GLOB` flag | kotlin **7.75×** (43 min → 5.56 min) |
| `f974eca` | Reapply G1GC + StringDedup JVM flags | defensive against CPU contention; noise quiescent |
| ~~`8b3c78b`~~ → `552073c` | Strategy 1 process sharding (reverted) | memory-bandwidth bound, capped at 1.4× |
| `279de70` | On-disk content-hash cache | Signal **6.6× warm-no-edit** (32.5 → 4.9 s) |
| `e4483c5` | Expressions rewrite (`resolveToCall`) | Signal 1.115× direct, **2.5× full krit run**, JSON -38% |
| `d67ae02` | Rule classification infrastructure + 20 rules | correctness PASS, perf neutral until more rules audited |
| `535c5df` | Postpone gRPC FIR server | roadmap stub |
| `c40f8c9` | Cache poison-entry markers for FIR-crash files | insurance, not exercised today |
| `c17d1fc` | Always emit FileResult so empty files cache | Signal warm **4.9 → 0.56 s** (8.8× further) |
| (in flight) | Classify in-memory hash index + closure hash memoization + build/ walker fix | kotlin warm target ~1-2 s |

## Paths we tried and rejected

### Strategy 2: in-JVM multi-session parallelism — BLOCKED

Research verdict: **KT-64167** documents that `KotlinCoreEnvironment.ourApplicationEnvironment`
is a JVM-wide singleton. Multiple concurrent `buildStandaloneAnalysisAPISession`
instances can't safely share VFS, PSI stub indices, or FirLazyResolveContractChecker
ThreadLocals. Each session gets its own MockProject but the shared app layer
is a hard blocker to concurrency.

### Strategy 3: warmup + parallel extraction — PARTIAL / NO

Built the spike. On Signal-Android, `lazyResolveToPhaseRecursively(BODY_RESOLVE)`
as warmup + parallel extraction was byte-identical to sequential (1.16× wall
ceiling). On kotlin/kotlin, the **warmup itself** perturbed FIR state enough
that extraction returned a different set of synthetic members — drops
`java.lang.AutoCloseable` + ~500 lines of generated members. Deterministic
drift, not a race. Running `--jobs 1` with the warmup pass still produced
the drift. Conclusion: `lazyResolveToPhaseRecursively` is not idempotent with
`memberScope.declarations` on the kotlin corpus.

Speedup cap in the best case was ~1.2× because warmup is ~90% of the FIR
work. The warmup does more total resolve work than lazy on-demand
extraction, which is why the "warm phase is 22× faster than sequential
extraction" signal on Signal didn't translate to a 22× overall speedup.

### Strategy 1: process-level sharding — REVERTED

Implemented `--shard I/N` + `InvokeSharded` orchestrator + JSON merger with
a correctness gate. Correctness passed (bit-identical after omitempty
normalization). But the speedup curve was hardware-bound:

- Signal-Android: N=2 was 1.16×, N=4 was 0.88× (slowdown), N=8 was 0.54×
- kotlin/kotlin: N=2 was 1.38×, N=4 was 1.40×, N=8 was 0.97× (slowdown)

Diagnosis: each shard's FIR cache wasn't just for its assigned files — it
was for the transitive closure through lazy FIR resolution, which on
tightly-coupled code ends up being most of the project. 8 shards × 80% of
project FIR = 6.4× memory amplification, and the Apple Silicon unified
memory bandwidth saturated before all CPU cores could be used. CPU was
available; DRAM bandwidth wasn't.

The reverted commits are in git history (`8b3c78b` → `552073c`). Kept so
future readers can see the investigation.

## Open questions for further sessions

### Go reimplementation of Kotlin Analysis API

Asked during session. Verdict: **theoretically possible, practically a
multi-year effort**. Analysis API is the entire Kotlin compiler frontend:
FIR construction, type inference with subtyping + generics, smart casts,
flow analysis, overload resolution, Java interop. Comparable tools like
rust-analyzer took 8 years of a full-time team. Not a side project.

The pragmatic answer is what krit already does: tree-sitter for 80% of
cheap queries, JVM oracle subprocess for the 20% that need full precision.
Making that subprocess faster (this session's work) pays off in weeks.
Replacing it pays off in years.

### Kotlin warm ceiling

Signal-Android at 0.56 s warm-no-edit is dominated by SHA-256 hashing of
the 2,431 source files. Kotlin warm target is ~1-2 s which is similarly
hash-dominated for 18,358 files. Beyond that, the next bottleneck is:

- Cache classify I/O (if LoadEntry per-hit dominates)
- Whole-JSON assembly at the end (16k entries merged into one oracle JSON)

If either becomes annoying, the natural next move is storing per-file
analyses in a SQLite cache.db instead of per-file JSON entries under
`entries/`. Single file, indexed lookups, one fopen instead of 16k.
Pre-profile before committing to the change.

### Full rule audit (remaining 220)

Track 2 shipped 20-rule sample classification + filter infrastructure.
Full audit of the remaining 220 rules is ~2-3 days of rule-reading. The
sample rate was 16/20 tree-sitter-only, suggesting a full audit could
drop oracle workload 30-50% on typical codebases. Not done this session.

### JDK classpath for resolveToCall precision

Surfaced during the expressions rewrite. `resolveToCall()` returns null
for stdlib calls because krit doesn't currently pass a JDK classpath
to krit-types. The lexical fallback works, but 71% of Signal-Android
call-targets could be more precise FQN-formatted if a classpath was
provided. ~30 min fix: detect $JAVA_HOME and pass via `--classpath`.

## Cross-cutting learnings

1. **Trust measurements over stories.** The "3 FIR-crash files" floor
   story turned out to be 6 empty files. The `build/` discrepancy I
   blamed on `FindSourceDirs` was actually my own walker's `build/`
   prune. Every hypothesis in this session that didn't come with a
   measurement was wrong or misleading.

2. **Machine state dominates absolute numbers.** Contention from
   concurrent benchmarks inflated Signal-Android baselines from 45 s to
   61 s — the 1.15× JVM tuning "win" was contention tolerance, not
   quiescent throughput. Re-measure before committing. The real pattern:
   take quiescent numbers, document the machine state, and compare
   relative ratios between config variants rather than absolute times.

3. **Cache invariants matter more than cache data.** The cache layer
   shipped correctly but had three subtle bugs that each produced real
   warm-run correctness regressions:
   - Empty FileResults never cached → warm missed 6 files
   - Dedup-victim path check → identical-content files re-analyzed
   - build/ pruning → 1,709 files silently absent from warm assembly
   Each bug was cheap to fix but would have been silent on Signal. Only
   kotlin's scale and file-layout surfaced them. Test with more than
   one corpus.

4. **Memoization beats algorithmic improvements when the inputs are
   small.** The `closureFingerprint` hot path wasn't expensive per call
   — it was just called 240k times on the same 16k unique files. Adding
   a shared hash cache across ClassifyFiles is 5 lines; no algorithmic
   rewrite, 10× speedup on the closure phase.

5. **Process isolation is the wrong parallelism model for memory-bound
   analyzers.** Strategy 1's failure taught us: when the per-worker
   state is the transitive dependency closure (not the assigned chunk),
   each process rebuilds near-full state. N copies of the full state is
   N× memory without N× CPU. Only works when the per-worker state
   scales with the chunk, not the global.

## Worktrees left on disk

None — all merged or reverted. Branches preserved:

- `wt/parallel-jobs` (Strategy 2 investigation — see `ee3aef4`)
- `wt/strategy3-warmup` (warmup spike — see `1877929`)
- `wt/strategy1-sharding` (sharding investigation — see `c66e4a7`)
- `wt/async-profiler` (flame graphs — see `41aeec7`)
- `wt/jvm-tuning` (tuning sweep — see `80d76b1`)
- `wt/expressions-audit` (sample classification — see `6efbb13`)
