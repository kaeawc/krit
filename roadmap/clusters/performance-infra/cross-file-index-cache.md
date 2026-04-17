# CrossFileIndexCache

**Cluster:** [performance-infra](README.md) · **Status:** open ·
**Severity:** n/a (infra) · **Default:** n/a · **Est.:** 4–6 days

## What it does

Caches the cross-file CodeIndex — bloom filter, Kotlin/Java/XML
reference maps, lookup tables — between runs, keyed by a fingerprint
over the file set and their content hashes. On warm re-runs with no
file changes, the index loads from disk in ~100ms instead of being
rebuilt from scratch.

## Current cost

On Signal-Android warm (`./krit ~/github/Signal-Android --perf`):

| Phase | Warm time |
|---|---:|
| typeOracle (cached) | 168ms ✅ |
| parse | 593ms |
| typeIndex | 1,743ms |
| **crossFileAnalysis** | **3,580ms** ← biggest warm bottleneck |
|   └─ indexBuild | 2,353ms |
|     └─ kotlinIndexCollection | 1,523ms |
|     └─ javaReferenceCollection | 307ms |
|     └─ lookupMapBuild | 449ms |
|   └─ crossRules | 729ms |
| **total** | **7,036ms** |

The oracle cache shipped and works beautifully (168ms warm vs ~30s
cold, 178× speedup on that phase). But `crossFileAnalysis` is now
the dominant warm-run cost — rebuilt from scratch every invocation
even though its output is a pure function of file contents.

## Proposed design

### Cache key

Fingerprint = `SHA-256(sorted(file_path + ":" + sha256(content)))` across
every file included in index building (Kotlin, Java, XML). Any file
add / delete / content-change invalidates the whole fingerprint, and
a full rebuild happens on that run.

For incremental updates (one file edited), use a **composable
fingerprint**: per-file hash + per-file index contribution, union them
at load time. File edit only invalidates that file's contribution.

### Cache layout

```
{repo}/.krit/cross-file-cache/
├── version                               # integer; bump nukes entries
├── meta.json                             # fingerprint, file-set hash, timestamps
├── bloom.bin                             # serialized bits-and-blooms filter
├── kotlin-refs.json                      # Kotlin reference index shards
├── java-refs.json                        # Java reference index shards
├── xml-refs.json                         # XML reference index shards
└── lookup-maps.json                      # name→file maps, etc.
```

### Load/rebuild decision

At cross-file-analysis phase start:

1. Compute current fingerprint from the file set.
2. Load `meta.json` if present.
3. If fingerprint matches → `gob.Decode` each file into memory → skip
   rebuild. Expected cost: ~100ms disk + ~50ms deserialize on Signal.
4. If fingerprint differs → run current build path → persist result.

### Serialization format

Use `encoding/gob` for the bloom filter (opaque bits) and JSON for the
reference maps (human-debuggable). Both formats measured on Signal
ought to be under 10MB.

### Incremental mode (stretch)

`lookupMapBuild` (449ms) and `kotlinIndexCollection` (1,523ms) both
compose naturally: per-file contributions are independent. A second
phase of this work stores per-file contributions separately and
recombines them on load, so a single-file edit invalidates one entry
not the whole index. Expected warm with single-file edit: ~300ms
cross-file phase instead of ~3s.

## Files to touch

- `internal/scanner/index.go` — add `BuildIndexCached(...)` that tries
  cache first, falls back to `BuildIndex(...)`, persists on success
- `internal/scanner/index_cache.go` — new: fingerprint, serialize,
  deserialize, load/save
- `internal/scanner/index_cache_test.go` — round-trip + invalidation
  tests
- `cmd/krit/main.go` — `--no-cross-file-cache` flag (mirror of
  `--no-cache-oracle`), wiring
- `.krit/cross-file-cache/` — new cache dir, add to `.gitignore`
  (already covered by `/.krit/`)

## Testing

- Round-trip: build index, serialize, deserialize, run a cross-file
  rule, results match.
- Invalidation: add a file → fingerprint differs → rebuilds.
- Invalidation: change content of an indexed file → fingerprint differs
  → rebuilds.
- Concurrent runs: two `krit` processes analyzing the same repo should
  not corrupt the cache (file locking or atomic rename).
- Cache corruption: truncated file, wrong version, missing meta.json →
  silent rebuild, warn via `--verbose`.

## Measured target

On Signal-Android warm, reduce `crossFileAnalysis` from 3,580ms to
~300ms (12× that phase) → overall from 7s to ~3.5s (2× end-to-end).

With the incremental variant (stretch), single-file edits could drop
to sub-second total.

## Risks

- **Cache coherency bugs** are the hardest part. Bloom filter with
  wrong contents produces false negatives in dead-code rules → silent
  missing findings. Mitigation: fingerprint every input, include rule
  registry version, bump cache version on any `internal/scanner/index.go`
  shape change.
- **Disk cost** scales with project. Signal at ~10MB is fine;
  kotlin/kotlin might be 50MB+. Document and add a `--clear-cache`
  parallel for `.krit/cross-file-cache/`.
- **Stat storm**. Fingerprint computation requires hashing every file
  (already done by oracle cache). Reuse the oracle cache's hash map
  instead of re-hashing.

## Blocking

- None.

## Blocked by

- None. This is a standalone optimization pass.

## Links

- Parent cluster: [`performance-infra/README.md`](README.md)
- Adjacent: [`oracle-file-hash-cache.md`](oracle-file-hash-cache.md)
  (type oracle equivalent; this doc is its cross-file sibling)
- Related: [`core-infra/cache-unification.md`](../core-infra/cache-unification.md)
  — long-term direction is one unified content-hash-keyed store replacing
  multiple ad-hoc caches
