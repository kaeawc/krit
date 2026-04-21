**Cluster:** performance-core · **Est.:** 1 day · **Depends on:** none

## What

`buildCodeIndexWithBloom` (`internal/scanner/index.go:420-467`)
constructs the by-name reverse-lookup maps — `refCountByName`,
`refFilesByName`, and their in-comment-excluded twins — by iterating
over every collected reference and lazily allocating inner maps on
first miss:

```go
// Hot loop: ~1.5M–6M iterations on Signal-Android
for _, ref := range refs {
    idx.refCountByName[ref.Name]++
    if idx.refFilesByName[ref.Name] == nil {
        idx.refFilesByName[ref.Name] = make(map[string]bool) // alloc storm
    }
    idx.refFilesByName[ref.Name][ref.File] = true
    if !ref.InComment {
        idx.refCountByNameCode[ref.Name]++
        if idx.refFilesByNameCode[ref.Name] == nil {
            idx.refFilesByNameCode[ref.Name] = make(map[string]bool) // ditto
        }
        idx.refFilesByNameCode[ref.Name][ref.File] = true
    }
    idx.refBloom.AddString(ref.Name)
}
```

For a Signal-scale corpus (~5M refs, ~120K unique names, ~6,700 files)
the inner `map[string]bool`s hit the runtime allocator hundreds of
thousands of times, then expand as more `(name, file)` pairs are
inserted. Each resize rehashes, which dominates the wall time.

A two-pass pre-grouped build avoids the alloc storm:

1. **Group pass** — sort refs by `Name` (or bucket them via a
   `map[string][]int` of indices) in one linear scan.
2. **Construct pass** — for each unique name, allocate the inner map
   with the *exact* capacity needed (`len(fileSet)`), populate it in
   one shot, done.

This trades one O(n log n) sort for eliminating the lazy-allocate +
resize cycle on ~120K inner maps.

## Measurement

Cold-run phase breakdown on Signal-Android (benchmark runbook, commit
`e1257dc`):

| Phase | Cold | Warm | SFE |
|---|---:|---:|---:|
| indexBuild | 2,586 ms | 172 ms | 1,669 ms |
| lookupMapBuild | 321 ms | 0 ms (cached) | 382 ms |
| crossFileAnalysis | 7,642 ms | 512 ms | 2,006 ms |

`lookupMapBuild` is a sub-phase of `indexBuild`. The 321 ms cold / 382
ms SFE figures are almost entirely this loop and its allocator
pressure. On SFE specifically it's ≥19% of crossFileAnalysis.

**Target:** cold `lookupMapBuild` 321 → ≤200 ms; SFE 382 → ≤230 ms.

## Plan

1. **Pre-bucket refs by name.** One linear scan into a
   `map[string][]Reference` (bucketing by name) or sort `refs` in
   place by `(Name, File)`. Choose based on microbench — a
   `slices.SortFunc` on `[]Reference` avoids the extra map but
   reallocates the slice.
2. **Build inner maps with known capacity.** For each unique name,
   count distinct files (set semantics on `(name, file)`), allocate
   `refFilesByName[name] = make(map[string]bool, distinctFileCount)`,
   then insert.
3. **Apply the same pre-group to the `!InComment` pair** so the code
   variant benefits equally.
4. **Bloom add deduplication.** `AddString` is idempotent on the
   bloom, but currently called once per ref. After bucketing, call
   `AddString` once per unique name. Small CPU win (~50 ms
   projected), negligible if the bucket pass isn't free.
5. If sort-in-place wins the microbench, wire it behind
   `buildCodeIndexWithBloom` so callers don't need to know.

## Expectations

- Cold `lookupMapBuild` drops 100–200 ms.
- SFE `lookupMapBuild` drops a similar amount (it's the same code
  path with one shard miss vs. all-hits).
- Inner-map allocation count drops from ~120K to ~120K but with
  correct capacity — no resize churn.
- Heap allocation (`go test -count=1 -benchmem`) on the per-build
  benchmark drops ≥30%.
- No output change: `refCountByName`, `refFilesByName`, and bloom
  final state must be byte-identical to the current implementation.

## Validate

- Add `BenchmarkBuildCodeIndexFromRefs_SignalScale` using a
  representative ref distribution (pareto on name frequency, 120K
  distinct names, 5M refs). Report ns/op and B/op before + after.
- Round-trip test on a synthetic fixture: pre-grouped output ==
  lazy-allocated output for every map key.
- Signal-Android: cold + SFE `lookupMapBuild` improvements meet the
  target range.
- `go test -race ./internal/scanner/ -count=1` clean — no shared
  mutation, but the fact that this runs after `collectIndexDataSharded`
  means it inherits whatever bucketing assumptions we make.

## Risks

- **Sort stability.** If we sort refs in-place, downstream code that
  walks `refs` in collection order would break. Audit uses of
  `refs` after `buildCodeIndexWithBloom`; if any depend on order,
  sort a copy.
- **Bucket memory.** The intermediate `map[string][]Reference` adds
  transient memory proportional to total ref count. For 5M refs at
  ~40 bytes per bucketed entry, ~200 MB transient. Acceptable vs.
  peak usage today; flag if it pushes us past the cache budget cap.
- **Small-corpus regression.** On tiny projects (hundreds of refs)
  the sort/bucket overhead is larger than the lazy-alloc it
  replaces. Microbench across small + medium + large corpus sizes
  before committing.

## Opportunities

- Sorted-by-name refs open the door to a **binary-searched flat ref
  index** that skips the `map[string]map[string]bool` entirely —
  a follow-up issue if this one's savings don't suffice.
- Pairs with #02 (per-worker local slices in sharded collection) —
  if collection returns per-worker slices, the bucketing pass can
  merge-sort them in O(n) instead of O(n log n).

## References

- Code: `internal/scanner/index.go:420-467`
- Bloom: `bloom.BloomFilter.AddString` — idempotent, safe to dedupe
- Measurement: `scratch/benchmarks/2026-04-21_043420_7a182cb_cold.json`,
  `…_sfe.json`
