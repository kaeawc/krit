**Cluster:** performance-infra · **Est.:** 2–3 days · **Depends on:** #352 (cross-file shard zstd + KSHC codec)

## Context

#352 shrank the cross-file shard cache by ~95% (from ~159 MB projected
down to ~24 MB measured on Signal-Android) using four techniques: zstd
compression of each blob, stripping per-record redundant fields,
interning enum strings as uint8, and replacing gob with a framed
columnar binary (KSHC) carrying an intra-shard name table. The
`klauspost/compress/zstd` dependency is now already in the repo.

Three other on-disk caches are still raw gob and ripe for the same
treatment. This issue applies the well-understood parts of the #352
playbook (zstd + drop-redundant-fields) to those caches, and opens
the door to a follow-on columnar migration for the parse cache's
`[]FlatNode` payload — the one place where columnar will genuinely
help beyond zstd alone.

## Baseline (Signal-Android, post-#352, cold-run rev `e1257dc`)

Measured directly on disk (`du -sh`, `zstd -3 -q` sampling):

| Cache | Current | Entries | Avg/entry | zstd-3 ratio | Projected | Saves |
|---|---:|---:|---:|---:|---:|---:|
| parse-cache/kotlin | 82 MB | 1,788 | 46 KB | 2.2× | ~37 MB | ~45 MB |
| parse-cache/java | 71 MB | 1,656 | 43 KB | 2.2× | ~32 MB | ~39 MB |
| parse-cache/xml | 3.9 MB | 678 | 5.8 KB | 2.0× | ~2 MB | ~2 MB |
| resource-cache | 60 MB | 103 | 604 KB | 5–6× (large-entry dominated; 6.2× on the 1.15 MB median-weight sample) | ~10–12 MB | ~48–50 MB |
| cross-file/payload.gob | 20 MB | 1 (monolithic) | 20 MB | 2.9× | ~7 MB | ~13 MB |
| **total** | **237 MB** | | | | **~88–90 MB** | **~147 MB (~62%)** |

(cross-file `packs-v1/` at 24 MB is already zstd+KSHC from #352 and
is excluded from this plan.)

Compression ratios measured on representative on-disk samples:

```
parse-cache/kotlin sample (19,414 B gob)   → 9,084 B zstd-3   (2.1×)
parse-cache/kotlin large  (45,955 B gob)   → 20,863 B zstd-3  (2.2×)
parse-cache/xml sample     (3,645 B gob)   → 1,739 B zstd-3   (2.0×)
resource-cache small       (1,687 B gob)   → 990 B zstd-3     (1.7×)
resource-cache large   (1,153,453 B gob)   → 185,815 B zstd-3 (6.2×)
cross-file payload.gob (20,182,766 B gob)  → 6,850,559 B zstd-3 (2.9×)
```

## Why this now

- Dependency cost already paid: `klauspost/compress/zstd` is in `go.mod` post-#352.
- Warm-read wall wins. Resource cache reads 60 MB on every warm run
  (observed 118 ms `androidProjectAnalysis`); compressed to ~12 MB,
  this should drop to ~30–40 ms once decompression CPU (~12 ms at
  ~1 GB/s) is amortised against the 5× I/O reduction.
- Monolithic `payload.gob` at 20 MB dominates `.krit/` size for
  small-to-medium projects that don't stress the per-file shards.
  Wrapping it is a 20-line change.
- Drop-redundant-fields — trivial at this scale (the per-file caches
  save <500 KB total in field stripping alone), but worth sweeping
  at the same time to align with the post-#352 design.

## Plan (ordered by ROI / effort)

### Step 1 — zstd-wrap all three caches (small PR, high ROI)

Wrap the gob encode/decode in each cache's save/load paths with zstd,
same pattern as #352's shard blob wrap. Bump per-cache version
constants so old uncompressed entries are rejected as misses (zstd
magic check handles this naturally for load).

Files:
- `internal/scanner/parse_cache.go` — `saveEntry` (lines 391–453),
  `loadByHash` (lines 318–353). Bump `parseCacheVersion` 3 → 4.
- `internal/android/xml_parse_cache.go` — same shape, bump
  `xmlParseCacheVersion` 1 → 2.
- `internal/android/resource_cache.go` — same shape, bump
  `resourceCacheVersion` 1 → 2.
- `internal/scanner/index_cache.go` — monolithic
  `SaveCrossFileCacheIndex` / `LoadCrossFileCacheIndex` around
  `payload.gob`. Bump `CrossFileCacheVersion`.

Expected: ~147 MB disk savings, 5–80 ms per-phase warm-read wins
where the cache is hot in page cache.

### Step 2 — Drop redundant metadata fields (same PR as Step 1)

Mirror the #352 Step-2 pattern. Per-cache:

- `parseCacheEntry` (`parse_cache.go:60-67`): drop `Version`,
  `GrammarVer`, `ContentHash`, `Language`. All four are already
  validated via sidecar files (`version`, `grammar-version`, `hash`)
  or derivable from the owning `langCache`. Savings ~137 B × 3,444
  entries = ~470 KB. Small, but simplifies the entry shape and
  removes per-load duplicate validation.
- `resourceCacheEntry` (`resource_cache.go:53-59`): drop `Version`,
  `Fingerprint`, `ResDir`, `Kinds`. Already the cache key or trivially
  re-derived. Savings ~170 B × 103 = ~17 KB.
- `xmlParseCacheEntry` (`xml_parse_cache.go:63-69`): drop `Version`,
  `GrammarVer`, `ContentHash`, `Language`. Savings ~130 B × 678 =
  ~88 KB.

These field-drops are negligible bytes individually but worth bundling
into the Step-1 PR for structural consistency.

### Step 3 — Columnar binary for parse-cache `[]FlatNode` (follow-on PR)

The parse cache's payload is `[]FlatNode` — a 40-byte-per-node struct
array. On top of zstd's 2.2× there is another 1.5–2× available from
an explicit columnar layout (split Type / Parent / StartByte / …
into parallel arrays with delta-encoding on monotonic columns and
varint on small-valued columns). Projection: parse cache drops from
the Step-1 ~71 MB to ~30–40 MB.

This also eliminates `remapEntryNodes` (`parse_cache.go:358-370`),
which currently re-translates local NodeTypeTable indices into the
global process table on every load — O(n) per file, ~2M uint16
swaps per warm run across 3,442 files. A canonical
grammar-version-indexed NodeTypeTable (persisted once per grammar
version in the existing sidecar dir) makes load a pointer-set
instead of a remap.

Bigger change (~200 lines of codec + tests, similar to KSHC in
#352). Carve out as its own issue / PR after Step 1+2 land.

### Step 4 — Columnar for resource-cache `ResourceIndex` (stretch)

`ResourceIndex` is 13 separate `map[string]*X` fields plus
`[]Drawables` and `[]ExtraTextEntry`. Repeated string keys (XML
attribute names, file paths inside `StringsLocation`) are the obvious
candidates for a name table. A columnar encoding would add another
~1.5× on top of zstd, dropping resource-cache from ~12 MB to ~8 MB.
Smaller absolute win than Step 3; lower priority.

## Validation

- **Round-trip:** per-cache, encode then decode a representative
  entry; assert byte-level equivalence of the reconstructed payload
  (same test shape as `TestFileShardRoundTrip` from #352).
- **Magic check:** on-disk header matches zstd magic for Step 1,
  `KSHC` magic for Step 3. Regression test guards against silent
  fallback to gob.
- **`go test -race ./... -count=1`** clean.
- **Cold + warm benchmark delta.** Run the benchmark runbook
  (`scratch/benchmark-runbook.md`) on Signal-Android before and
  after each step; report `parse`, `crossFileAnalysis`,
  `androidProjectAnalysis` wall times and the `.krit/` total size.
- **Finding-equivalence.** `./krit --report json ~/github/Signal-
  Android` byte-identical to pre-change, modulo finding ordering.

## Risks

- **Version churn.** Step 1 bumps four version constants.
  Pre-Step-1 caches become full misses on first run — users with
  big caches pay a one-time cold equivalent. Document in the PR
  body with the "delete `.krit/` to opt in cleanly" guidance from
  prior cache PRs.
- **Decompression CPU vs. page-cache reality.** On hot page cache
  the current warm reads are ~100 ms for parse cache (gob decode is
  the bottleneck, not I/O). Adding zstd decompress on top is ~70 ms
  of CPU; net warm wall could be ±20 ms. We expect a slight
  improvement because smaller payloads also mean less heap pressure
  inside gob, but measure and back out zstd for any cache where net
  wall regresses.
- **Resource-cache large-entry dominance.** The 6.2× sample is
  measured on a 1.15 MB entry. Smaller entries compress less
  (1.7× on the 1.7 KB sample). Expected average on the full corpus
  is 5× based on the size distribution (median 837 KB), but the
  projection assumes the large entries dominate total bytes — which
  they do (top 10 entries = ~10 MB of 60 MB). Validate on the real
  corpus before committing to the savings claim in the PR body.
- **Dep footprint.** `klauspost/compress/zstd` is already pulled in
  by #352, so no additional dep cost.

## Opportunities

- Applies the #352 playbook end-to-end across the cache layer. After
  this lands the only raw-gob cache remaining is the LRU index files
  (`lru-index.gob`) — tiny, not worth touching.
- Sets the pattern for future caches (oracle cache, icon index cache
  if split out) — default to zstd+columnar from day one.
- Once parse cache is columnar (Step 3), the saved `remapEntryNodes`
  cost compounds with any future warm-load perf work.

## Dependencies

- **Blocked by:** none (the shared dep landed in #352).
- **Depends on:** #352 for the zstd dependency and the pattern it
  established.
- **Related:** existing `scratch/cache-infra-consolidation.md` — this
  issue is the concrete execution of the "apply the #352 playbook
  broader" note in that doc.

## Links

- Prior: #352 (cross-file shard zstd + KSHC codec), #351
  (design / targets for shards)
- Code entry points:
  - `internal/scanner/parse_cache.go:318-353, 391-453`
  - `internal/android/xml_parse_cache.go:192-276`
  - `internal/android/resource_cache.go:132-211`
  - `internal/scanner/index_cache.go:650-807` (monolithic)
- Measurement: benchmark runbook, cold results in
  `scratch/benchmarks/2026-04-21_043420_7a182cb_*.json`
