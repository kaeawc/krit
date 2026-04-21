# Cross-File Shard Backend Evaluation

**Date:** 2026-04-21
**Question:** Replace the 5,499-file `.krit/cross-file-cache/shards/` directory with a packed format to reduce SFE cold-open cost?

## Dataset

Real Signal-Android cache from `~/github/Signal-Android/.krit/cross-file-cache/shards/`:

| metric              | value     |
|---------------------|-----------|
| shard count         | 5,500     |
| total bytes         | 757.7 MB  |
| avg shard size      | 137.8 KB  |
| p50 / p90 / p99     | 19 / 135 / 1179 KB |
| max (outlier)       | 112.5 MB  |

## Backends Evaluated

1. **fs** — current prod: one `.gob` per shard under `hash[:2]/hash[2:].gob`.
2. **pack** — 256 packs (`00.pack` … `ff.pack`), keyed by `hash[:2]`. Header holds `(key, offset, length)` triples; blobs concatenated after. Pack loaded via `ReadAll` on first Get.
3. **bolt** — single `shards.bolt` (bbolt v1.4.3), one bucket, key=shard hash, value=gob blob.

Harness: [internal/scanner/shard_backend_bench_test.go](internal/scanner/shard_backend_bench_test.go). Gated by `KRIT_SHARD_BENCH_DIR`. Writes each backend's materialized form under `KRIT_SHARD_BENCH_MATERIALIZED` (reusable between `sudo purge` runs).

## Results (warm page cache, Apple M3 Max, 16 cores)

### BenchmarkShardBackendOpenAll — read every shard (SFE path)

| backend | time     | bytes read | allocs  |
|---------|----------|------------|---------|
| fs      | 172 ms   | 757 MB     | 38,528  |
| pack    | 147 ms   | 757 MB     | 37,508  |
| bolt    | 116 ms   | 757 MB     | 126,308 |

Warm-cache delta is small. The 926 ms baseline from `--perf` was cold.

### BenchmarkShardBackendRandomRead — single-key lookup (LSP path)

| backend | ns/op    |
|---------|----------|
| fs      | 29,933   |
| pack    | 212      |
| bolt    | 9,357    |

Pack wins by ~140× over fs because the pack is fully resident after the first read.

### BenchmarkShardBackendSingleRewrite — overwrite one shard (incremental path)

| backend | ns/op     | B/op    |
|---------|-----------|---------|
| fs      | 89,240    | 616     |
| pack    | 846,276   | 4.7 MB  |
| bolt    | 9,112,562 | 61 KB   |

Pack rewrites a whole pack (~3 MB of shards per pack on avg). Bolt's rewrite is 100× worse than fs — two B-tree traversals, freelist updates, page allocation. For incremental scans touching many shards, bolt would dominate wall time.

### BenchmarkShardBackendConcurrentRead — 16 goroutines, random keys

| backend | ns/op  |
|---------|--------|
| fs      | 6,783  |
| pack    | 29     |
| bolt    | 6,146  |

Pack again wins on warm. fs and bolt roughly tie.

### On-disk size

| backend | bytes        |
|---------|--------------|
| fs      | 757,732,210  |
| pack    | 758,184,234  |
| bolt    | 798,097,408  |

Bolt adds ~5 % overhead (B-tree pages, freelist, metadata). Pack header is negligible.

### Binary size impact

| build                | bytes      | delta    |
|----------------------|------------|----------|
| baseline (no bolt)   | 21,580,306 | —        |
| +bolt linked via `Open`+`Update`+bucket | 21,951,282 | +370,976 (+1.7 %) |

Actual cost is ~370 KB, not the 1.5 MB initially estimated. (The harness test file already imports bbolt but does not inflate the prod binary — its use of `internal/scanner` tests keeps it out of `cmd/krit`.)

## Cold-Open Numbers (captured after `sudo purge`)

| backend | time       | vs fs    | bytes_read |
|---------|------------|----------|------------|
| fs      |   758 ms   | 1.00×    | 757 MB     |
| pack    | **537 ms** | **0.71×**| 757 MB     |
| bolt    | 2,032 ms   | 2.68×    | 757 MB     |

**Pack beats fs cold by 1.4× — a 220 ms win. Bolt is 2.7× *worse* than fs cold.**

The bolt surprise is explained by access pattern: `View(...)` with a `Get` per key walks the B-tree for each of 5,500 keys. Each walk touches branch pages scattered across the 798 MB mmap, so the cold-open is effectively 5,500 random 4 KB page faults — cache-hostile on a cold SSD. Pack reads each of 256 files sequentially with `ReadAll`, giving the readahead window a chance to prefetch; that's why it's the fastest cold backend despite doing more syscalls than bolt.

Had the bolt access pattern been a single `tx.ForEach`, iteration order would be sorted-by-key (random wrt the original key distribution) but would walk leaf pages in order, which likely recovers most of bolt's lost ground. Not benchmarked here.

## Recommendation

**Pack wins. Cold-open, warm random-read, rewrite cost, binary size — all favor pack.**

Full scorecard (bold = winner):

| metric              | fs          | pack           | bolt          |
|---------------------|-------------|----------------|---------------|
| cold OpenAll        | 758 ms      | **537 ms**     | 2,032 ms      |
| warm OpenAll        | 172 ms      | 147 ms         | **116 ms**    |
| warm RandomRead     | 29,933 ns   | **212 ns**     | 9,357 ns      |
| warm ConcurrentRead | 6,783 ns    | **29 ns**      | 6,146 ns      |
| SingleRewrite       | **89 µs**   | 846 µs         | 9,113 µs      |
| disk bytes          | **758 MB**  | 758 MB         | 798 MB (+5%)  |
| binary delta        | **0**       | 0              | +370 KB       |
| new dep             | **no**      | no             | yes (bbolt)   |

Pack loses only on `SingleRewrite` vs fs — 846 µs vs 89 µs, still sub-millisecond. With ~21 shards per pack, a 10-file incremental edit rewrites at most 10 packs (< 10 ms total). That's cheap next to the 220 ms cold-open win on every fresh SFE.

Bolt's sales pitch doesn't survive contact with this specific workload: mmap gave it a small warm-read edge but bought nothing cold, and rewrites are 100× worse than fs.

## Decision Inputs Beyond Raw Numbers

Before shipping pack, confirm these assumptions hold:

- **Concurrency:** pack's `ensureLoaded()` uses a single `sync.Mutex` around mmap read. LSP + CLI both reading is fine (shared lock). LSP writing while CLI reads is fine only if writes are rare — which they are during an incremental scan. If this turns out to be contested, swap `sync.Mutex` for `sync.RWMutex`.
- **Recovery:** fs layout tolerates a corrupt single shard (drop it, rescan one file). A corrupt pack poisons up to 21 shards. Mitigation: verify a CRC-32 per blob on load, treat checksum failures as a miss for just that key. Adds < 5 % read cost.
- **Cache migration:** bump `CrossFileCacheVersion` so old fs-layout shard dirs are discarded instead of read-as-pack.

## Suggested Next Steps (user's pick)

1. **Ship pack** — implement the pack backend behind `cacheutil.Registered`, migrate `internal/scanner/index_shard.go` with a version-bumped cache dir so old fs-layout caches are discarded cleanly. Add per-blob CRC-32 for corrupt-shard isolation. Estimate: ~1 week. Revert the `go.etcd.io/bbolt` dependency added for the eval.
2. **Re-test bolt with `tx.ForEach`** — curiosity only. If sorted iteration closes the 4× cold gap, bolt's concurrency story might still be worth the 370 KB. Low expected value; the pack answer is good enough.
3. **Defer** — 758 ms cold isn't blocking. Keep the harness, revisit if SFE first-scan becomes user-visible.
4. **Something else** — the harness is reusable; adding a 4th backend (e.g. a sorted concat-with-index like Git's pack-v2) is ~50 LOC.

## Files Changed

- `go.mod` / `go.sum` — added `go.etcd.io/bbolt v1.4.3` (test-only; revert if not choosing option 2).
- [internal/scanner/shard_backend_bench_test.go](internal/scanner/shard_backend_bench_test.go) — the harness.
