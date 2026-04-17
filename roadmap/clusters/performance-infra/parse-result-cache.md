# ParseResultCache

**Cluster:** [performance-infra](README.md) · **Status:** open ·
**Severity:** n/a (infra) · **Default:** n/a · **Est.:** 3–4 days

## What it does

Caches tree-sitter parse output to disk, keyed by `SHA-256(file_content)`
and grammar version. Skips re-parsing files whose content hasn't changed
since the last run. Saves ~600ms on warm Signal-Android runs.

## Current cost

On Signal-Android warm:

| Phase | Time |
|---|---:|
| typeOracle (cached) | 168ms |
| **parse** | **593ms** ← pure function of file content, recomputed every run |
| typeIndex | 1,743ms |
| crossFileAnalysis | 3,580ms |

The parse phase walks 2,467 files through tree-sitter-kotlin. Output is
deterministic given `(file_content, grammar_version)`. Today it's the
third-largest warm cost after `crossFileAnalysis` and `typeIndex`.

## Proposed design

### Cache layout

```
{repo}/.krit/parse-cache/
├── version                         # int; bump nukes entries
├── grammar-version                 # tree-sitter-kotlin commit hash / version
└── entries/
    └── {hash[:2]}/{hash[2:]}.gob   # serialized FlatTree
```

Keyed on `SHA-256(file_content)`, same layout as the oracle cache.
Grammar version stored separately so a tree-sitter-kotlin upgrade
silently invalidates every entry.

### What to serialize

The parse result consumed by krit is the `FlatNode` array plus the node
type table (`NodeTypeTable`). Both are plain structs, `gob`-friendly.
The raw tree-sitter `Tree` pointer is **not** serialized — it's
per-process and disposable. `FlatTree` is constructed from the tree
once; from then on, the rest of krit uses `FlatTree`.

```go
type parseCacheEntry struct {
    Version       uint32     // cache-schema version
    GrammarVer    string     // e.g. "tree-sitter-kotlin-6acedc2"
    ContentHash   string     // sha256 of file content
    NodeTypeTable []string   // global node-type indices for this entry
    Nodes         []FlatNode // compact 40-byte nodes
}
```

The per-entry `NodeTypeTable` maps the entry's `FlatNode.Type` field
(uint16) back to the global `scanner.NodeTypeTable`. On load, walk the
nodes and remap their `Type` indices into the current process's global
table.

### Load/miss path

At parse phase per-file:

1. Hash file content (already done by oracle cache — reuse).
2. Try load from `parse-cache/entries/{hash[:2]}/{hash[2:]}.gob`.
3. If present and grammar version matches → deserialize → remap node
   types → done. Expected cost: ~50µs/file.
4. Else → tree-sitter parse as today → write cache entry.

Saves roughly 0.24ms/file on hot path at Signal scale → 593ms drops to
~120ms (4–5× that phase).

### Bulk invalidation

`--clear-cache` already exists for `.krit-cache`. Extend to also
clear `.krit/parse-cache/`. Add a shared helper `ClearAllCaches()`.

### Interaction with shared hash cache

Both oracle cache and parse cache hash file contents. They should share
a process-scoped `hashCache map[string][32]byte` (added during the
2026-04-14/15 oracle work; see
`scratch/oracle-cache-optimization.md`). One hash per file per run.

## Files to touch

- `internal/scanner/parse_cache.go` — new: entry struct, save, load,
  type-table remap
- `internal/scanner/parse_cache_test.go` — round-trip, grammar-version
  mismatch, corrupt entry tolerance
- `internal/scanner/scanner.go` — `ParseKotlinFileCached(path, content)`
  fast-path
- `cmd/krit/main.go` — `--no-parse-cache` flag
- Grammar version wiring: embed at build time via `go:embed` of
  `go.mod` tree-sitter-kotlin version, or compute via `go list -m`.

## Testing

- Round-trip: parse file, save, load in fresh process, compare
  `FlatNode` arrays.
- Grammar version bump: mutate cached entry's `GrammarVer` → cache
  miss, re-parse.
- Content change: mutate file content by 1 byte → different hash →
  cache miss, re-parse.
- Concurrent writes: two goroutines writing the same hash → last-write
  wins without corruption (atomic rename).
- Large file: ~10k-line Kotlin file serialize/deserialize round-trip
  under 100ms.

## Measured target

On Signal warm: parse phase 593ms → ~120ms (4–5× that phase),
contributing to overall from 7s to ~6.5s. On its own this is modest,
but it stacks with [`cross-file-index-cache.md`](cross-file-index-cache.md).
Together the two caches would drop warm Signal from 7s to ~3.5s.

## Risks

- **Serialization overhead may dominate gains on small files.** A
  500-byte Kotlin file parses in <1ms but serializing + loading 500
  bytes of gob isn't free. Benchmark the crossover; skip cache for
  files under a size threshold.
- **Storage growth.** Signal at ~2,500 files × ~5KB/entry ≈ 12MB —
  acceptable. kotlin/kotlin at 18k files × 5KB ≈ 90MB — cap with an
  LRU eviction.
- **Type-table divergence between processes.** `NodeTypeTable` is
  populated lazily as nodes are seen. A fresh process's table has
  fewer entries than the cache's, requiring the remap step. Test this
  explicitly.

## Blocking

- None.

## Blocked by

- None.

## Links

- Parent cluster: [`performance-infra/README.md`](README.md)
- Pairs with: [`cross-file-index-cache.md`](cross-file-index-cache.md)
  (same cache pattern, different phase)
- Related: [`oracle-file-hash-cache.md`](oracle-file-hash-cache.md)
  (shipped; same design pattern applied to type oracle)
- Related: [`core-infra/cache-unification.md`](../core-infra/cache-unification.md)
  (long-term — unify parse / oracle / cross-file under one store)
