# WorkerPinnedParallelScan

**Cluster:** [performance-infra](README.md) · **Status:** shipped ·
**Severity:** n/a (infra) · **Default:** n/a

## What it does

Pin each scan worker to an OS thread with thread-local buffers,
intern pools, and finding collectors. Eliminates cross-thread cache
invalidation and mutex contention on the global intern pool during
the hot scan phase.

## Current state

`scanner.ScanFiles` uses a goroutine pool with a channel semaphore.
Files are dispatched round-robin. Each goroutine can run on any OS
thread, and the Go scheduler migrates goroutines between threads
during a file scan. The global `sync.Mutex`-protected finding slice
is appended to under lock after each file.

## Proposed model

```go
func ScanFilesParallel(paths []string, workers int) *FindingColumns {
    results := make(chan *localResult, workers)

    for w := 0; w < workers; w++ {
        go func(workerID int, files []string) {
            runtime.LockOSThread()
            defer runtime.UnlockOSThread()

            local := &localResult{
                intern:   NewLocalPool(globalIntern),
                findings: NewFindingCollector(),
                arena:    NewBumpArena(),
            }

            for _, path := range files {
                local.arena.Reset()
                file := parseFileInArena(local.arena, path)
                flat := flattenTreeInArena(local.arena, file.Tree)
                findings := runRules(flat, file, local.intern)
                local.findings.AppendAll(findings)
            }

            results <- local
        }(w, partition(paths, w, workers))
    }

    // Merge worker-local results
    global := NewFindingColumns()
    for w := 0; w < workers; w++ {
        r := <-results
        global.MergeFrom(r.findings)
    }
    return global
}
```

Key properties:
- `runtime.LockOSThread()` pins the goroutine so per-worker data
  stays in L1/L2 cache for the duration of the worker's file batch.
- Each worker has its own `LocalPool` (no mutex on the hot path),
  `FindingCollector` (no mutex), and `BumpArena` (no mutex).
- File assignment is a static partition, not a channel — avoids
  channel contention and keeps each worker's file set contiguous
  in the filesystem (sorted paths partition into directory-local
  batches, improving OS page cache behavior).
- Merge happens once at the end, not per-file.

## Expected impact

10–20% improvement on multi-core machines (4+ cores) due to
eliminated false sharing and better L1/L2 residency. Diminishing
returns beyond 8 cores because tree-sitter parsing itself is
single-threaded per parser instance (already pooled).

The bigger win is that this model *composes* with the other
performance items: the arena, the intern pool, and the finding
collector are all designed to be thread-local first, merged second.
Without worker pinning, they'd need per-operation locking.

## Acceptance criteria

- `go test -bench -cpu 1,2,4,8` shows near-linear scaling up to
  4 cores on a 1000-file synthetic benchmark.
- No mutex contention visible in `go tool pprof -contentionprofile`.
- File-to-worker assignment is deterministic (same partition for
  the same input), so benchmark results are reproducible.

## Links

- Parent: [`roadmap/65-performance-infra.md`](../../65-performance-infra.md)
- Depends on: [`string-interning.md`](string-interning.md)
  (LocalPool is the per-worker facade)
- Depends on: [`per-file-arena-allocation.md`](per-file-arena-allocation.md)
  (BumpArena is per-worker)
- Depends on: [`columnar-finding-storage.md`](columnar-finding-storage.md)
  (FindingCollector is per-worker, merged at end)
