**Cluster:** performance-core · **Est.:** 1 day · **Depends on:** none

## What

`collectIndexDataSharded` (`internal/scanner/index.go:187-323`) and
`collectIndexDataInternal` (`internal/scanner/index.go:328-407`)
currently lock a shared `sync.Mutex` once per file to append per-file
symbols + references into shared result slices:

```go
// index.go:257-260 — hot path, runs once per file × both passes
mu.Lock()
symbols = append(symbols, syms...)
refs = append(refs, fileRefs...)
mu.Unlock()
```

On Signal-Android the sharded collector processes 3,442 Kotlin files +
3,259 Java files across 16 workers — ~6,700 lock/unlock pairs on a
single mutex. Each goroutine holds the mutex while copying two slices
into the shared buffers (tens to hundreds of elements per file), so
the critical section is non-trivial and contention on cold is real.

A per-worker local buffer that drains into the shared slice **once
after `wg.Wait()`** eliminates the per-file serialisation point — the
same pattern already used by `IndexFilesParallel` in
`internal/typeinfer/parallel.go:79-143`, which returns per-worker
slices and merges serially after the worker pool is done.

## Measurement

Cold-run phase breakdown on Signal-Android (16 cores, commit
`e1257dc`, `scratch/benchmark-runbook.md`):

| Phase | Cold | Warm |
|---|---:|---:|
| kotlinIndexCollection | 1,049 ms | — |
| javaReferenceCollection | 648 ms | — |
| xmlReferenceCollection | 217 ms | — |
| crossFileAnalysis total | 7,642 ms | 512 ms |

The lock hold includes a slice grow (go runtime copies backing array
on capacity doubling) ~1× per 64 appends. For 6,700 appends that's
~100 slice-copy events under the lock, each of which blocks other
workers for microseconds. Contention scales with worker count — 16
workers maximises it.

Rough projection: moving append out of the critical section removes
~100–200 ms from cold kotlinIndexCollection + javaReferenceCollection
combined. Warm is unaffected (shard cache served).

## Plan

1. Change `collectIndexDataSharded` to return per-worker `[][]Symbol`
   + `[][]Reference` slices instead of append-under-mutex.
2. Size pre-allocation: each worker pre-allocates a local slice with
   capacity matching its expected job count × avg syms/refs per file
   (a cheap heuristic — even a fixed 64-element prealloc beats zero).
3. After `wg.Wait()` returns, merge per-worker slices into the single
   output slice on the caller's goroutine with one pass — known final
   size from `sum(len(per-worker))`, so one allocation.
4. Apply the same pattern to `collectIndexDataInternal` (the
   non-sharded path at `index.go:328-407`). Same mutex, same
   fix.
5. The bloom-union path (`mergeBloom`, lines 203-213) already uses its
   own `bloomMu` — leave that alone; the bloom merge happens once per
   shard and doesn't benefit from the local-buffer approach.

## Expectations

- Cold `kotlinIndexCollection` drops 50–100 ms.
- Cold `javaReferenceCollection` drops 30–80 ms.
- Combined crossFileAnalysis savings 100–200 ms.
- No behavioural change — the aggregated symbols/refs slice contents
  must be byte-identical to the current output after merge.
- Worker pool sizing unchanged (`workers` from caller).

## Validate

- `go test -race ./internal/scanner/ -count=1` — critical, since the
  change is exclusively about synchronisation.
- Bench: add a micro-benchmark
  `BenchmarkCollectIndexDataSharded_16Workers` that processes a fixed
  1000-file synthetic corpus; record ns/op and heap allocs before /
  after.
- Signal-Android cold run: `kotlinIndexCollection +
  javaReferenceCollection` drop by ≥100 ms combined.
- Finding-equivalence via `./krit --report json` — ordering is
  allowed to differ (sort before compare).

## Risks

- **Ordering change.** The current implementation interleaves per-file
  results in goroutine-completion order; the local-buffer version
  produces them in worker-completion order. Downstream consumers
  (bloom construction, lookup map build) must not depend on input
  ordering. Quick audit: bloom build is commutative; lookup map is
  aggregative. Confirm with a canary on Signal-Android finding
  equivalence.
- **Memory headroom.** Per-worker local slices raise peak memory
  transiently — each worker's buffer grows independently. At 16
  workers × ~6 MB peak per worker = ~100 MB transient; well inside
  the budget, but worth noting vs. the current single-slice pattern
  that reuses the capacity.

## Opportunities

- Exposes a reusable `parallel.CollectByWorker[T]` helper in
  `cacheutil` or `pipeline` — same shape already open-coded in
  `typeinfer/parallel.go` and would benefit the XML-reference
  collector if it ever grows past the current 217 ms.
- Sets up the pattern for future sharded collectors (icon index,
  resource index partials) to avoid repeating the mutex-append shape.

## Dependencies

- **Blocked by:** none.
- **Related:** #346 (PerShardBloomUnion) — touched the same file but
  doesn't overlap this code path.

## References

- Code: `internal/scanner/index.go:187-323` (sharded),
  `internal/scanner/index.go:328-407` (non-sharded)
- Prior art: `internal/typeinfer/parallel.go:79-143`
  (`IndexFilesParallel` already uses per-worker slices)
- Measurement: `scratch/benchmarks/2026-04-21_043420_7a182cb_cold.json`
