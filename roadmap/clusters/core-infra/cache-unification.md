# CacheUnification

**Cluster:** [core-infra](README.md) · **Status:** planned ·
**Severity:** n/a (infra) · **Default:** n/a

## What it does

Replaces four independent caching systems — the incremental analysis
cache, the experiment matrix baseline cache, the on-disk oracle cache,
and the detekt XML baseline suppression — with one content-hash-keyed
store. Each layer currently has its own invalidation strategy; a
single unified store makes staleness reasoning tractable.

## Current cost

Four caches exist with no shared abstraction:

| Cache | Location | Invalidation | Purpose |
|---|---|---|---|
| Incremental analysis | `internal/cache/` | mtime + file hash | Skip re-scanning unchanged files |
| Experiment matrix | `internal/experiment/` | manual run flag | Baseline finding counts for benchmark comparisons |
| Oracle on-disk | `internal/oracle/` | oracle version hash | Avoid re-running JVM type analysis on unchanged files |
| Detekt baseline | `internal/scanner/baseline.go` | manual `--update-baseline` | Suppress pre-existing findings |

Problems:
- Cache invalidation logic is duplicated four times with different
  heuristics. When a rule is updated, only the incremental cache is
  automatically invalidated; the oracle cache and matrix baseline are
  not, so stale data silently persists.
- The experiment matrix cache has a known jitter issue (documented in
  `roadmap/` notes): ~8 `UnsafeCallOnNullableType` findings vary
  between runs due to cache interactions that are difficult to reason
  about.
- The detekt baseline is in XML for migration-compatibility reasons
  but is slow to parse and hard to diff.
- No cache is observable: there is no command to inspect what is
  cached, when entries expire, or what would be invalidated by a rule
  change.

Relevant files:
- `internal/cache/` — incremental cache
- `internal/experiment/` — matrix baseline
- `internal/oracle/` — oracle cache
- `internal/scanner/baseline.go` — detekt XML baseline

## Proposed design

A single `internal/store/` package with a content-hash-keyed
key-value interface:

```go
// internal/store/store.go

type Store interface {
    // Get retrieves a cached value by key. Returns (nil, false) on miss.
    Get(key Key) ([]byte, bool)
    // Put stores a value. Overwrites any existing entry for the key.
    Put(key Key, value []byte) error
    // Invalidate removes all entries whose key contains the given rule IDs.
    Invalidate(ruleIDs ...string) error
    // Stats returns a summary of cache utilisation.
    Stats() StoreStats
}

type Key struct {
    FileHash   [32]byte // SHA-256 of file content
    RuleSetHash [16]byte // hash of active rule IDs + versions
    Kind       StoreKind // incremental | oracle | matrix
}
```

Each existing cache is reimplemented as a thin wrapper that maps its
current data model onto `Key` + `[]byte`. The detekt XML baseline is
migrated to a JSON format (or kept for import/export only).

Rule version hashes are derived from the rule's struct definition
checksum (emitted by the code generator from
[`codegen-registry.md`](codegen-registry.md)), so updating a rule
automatically invalidates its cached findings without manual
intervention.

`krit cache stats` and `krit cache clear` subcommands expose the
store for inspection and maintenance.

## Migration path

1. Define the `Store` interface and a file-backed implementation in
   `internal/store/`.
2. Migrate `internal/cache/` to use the new store (smallest cache,
   best test coverage — land first).
3. Migrate the oracle cache.
4. Migrate the experiment matrix cache.
5. Migrate the detekt baseline to the store for suppression storage;
   keep detekt XML import/export as a conversion utility.
6. Delete the four original cache implementations.
7. Add `krit cache stats` and `krit cache clear` subcommands.

## Acceptance criteria

- `internal/cache/`, `internal/experiment/` (cache portion), and
  oracle cache files no longer have independent invalidation logic.
- Updating a rule's implementation automatically invalidates its
  cached findings on the next scan.
- `krit cache stats` shows hit rate, entry count, and total size.
- `krit cache clear --rule=RuleName` removes only entries for the
  specified rule.
- The experiment matrix jitter issue is resolved: repeated runs of
  the same corpus produce identical finding counts.

## Links

- Depends on: [`codegen-registry.md`](codegen-registry.md) (rule
  version hashes require the generated registry to include checksums)
- Related: `internal/cache/`, `internal/scanner/baseline.go`,
  `internal/oracle/`, `internal/experiment/`
