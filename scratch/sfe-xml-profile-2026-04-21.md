# Single-File-Edit + XML Profile Run — 2026-04-21

Profiled Signal-Android (3877 Kotlin, 3259 Java, 9292 XML files) on
`main` after PRs #311–#330. Three scenarios: **warm-all-cached**,
**single-file-edit (SFE)**, **no-parse-cache (cold Java/Kotlin)**.
Oracle disabled (`--no-type-oracle`) throughout.

## Phase timing summary

| Phase | warm-all-cached | SFE (1 file changed) | no-parse-cache |
|---|---:|---:|---:|
| parse | 93ms | 122ms | 615ms |
| **crossFileAnalysis** | **533ms** | **1384ms** | **1754ms** |
| → javaIndexing | 168ms | 175ms | 524ms |
| → kotlinIndexCollection | — (monolithic hit) | 120ms | 126ms |
| → javaReferenceCollection | — | 18ms | 23ms |
| → xmlReferenceCollection | — | 3ms | 4ms |
| → **lookupMapBuild** | **0ms (cached)** | **473ms** | **471ms** |
| → crossRules (VFT) | 152ms | 151ms | 151ms |
| **androidProjectAnalysis** | **314ms** | **308ms** | **297ms** |
| → resourceDirScan (wall) | 267ms | 262ms | 248ms |
| → valuesXMLParseCPU | 535ms | 542ms | 501ms |
| → layoutDirScan | 71ms | 71ms | 70ms |
| total wall | 2822ms* | 2084ms | 2895ms |

*warm-all-cached run used `--no-cache` (incremental cache off) so
ruleExecution was full (1668ms); SFE and no-parse-cache used the
incremental cache (ruleExecution ≈1ms, 1 file re-run).

## Cache stats on SFE run

| cache | hits | misses | notes |
|---|---:|---:|---|
| cross-file-cache | 0 | 1 | monolithic miss (content hash changed) |
| cross-file-shards | 5497 | 1 | shard path active; only 1 file re-sharded |
| parse-cache | 3441 | 1 | changed .kt file re-parsed |
| oracle-cache | 0 | 0 | disabled |

## Findings

### N11 — PerShardBloomUnion: WARRANTED

**Evidence:** `lookupMapBuild` = 473ms on SFE vs 0ms on full warm hit.

The shard path works correctly (5497/5498 shard hits). `#330`'s
sharding covers data collection (`kotlinIndexCollection` 120ms)
but the lookup maps + bloom filter are rebuilt from all aggregated
shard data every time the monolithic index misses. That rebuild
costs 473ms and accounts for 46% of `crossFileAnalysis` on SFE.

`#309`'s monolithic lookup+bloom cache saves this cost on full-warm
hits (no file changed) but provides zero benefit the moment one file
changes — the hash invalidates the whole monolithic entry, and the
shard path has no per-shard bloom to union.

**Decision: open N11.** Target: per-shard bloom union brings SFE
`lookupMapBuild` from 473ms → <50ms. Overall SFE crossFileAnalysis
target: ≤ ~600ms (from 1384ms).

### N12 — XMLParseCacheExtension: SCOPE REVISION REQUIRED

Two distinct XML parse paths, different costs, different approaches:

**Path A — Values XML (`valuesXMLParseCPU` 501–542ms CPU, 248–267ms wall)**

- Code: `internal/android/resources.go:765` — `encoding/xml` stdlib
  decoder, not tree-sitter. Produces `ResourceIndex`.
- Constant across all three runs — no cache shortcircuit available
  at the parse-tree level.
- Would require a **ResourceIndex-level cache** (cache the semantic
  output, not a parse tree). This is a heavier change: the cache
  key would be `(resDir, sha256(all values XML files in dir))`,
  the value a serialised `ResourceIndex`.
- Savings: ~262ms wall / ~530ms CPU per run.

**Path B — Layout/Manifest XML (`layoutDirScan` 71ms wall)**

- Code: `internal/android/xmlast.go:71` — tree-sitter via
  `internal/tsxml`. Produces `*XMLNode` (tree-sitter AST).
- Same `FlatNode` gob serializer reuse as `#308` Java cache is
  possible here.
- Savings: ~71ms wall per run.

**N12 should be split into two issues:**

- `ResourceIndexCache` (Path A, ~262ms wall) — semantic cache of
  `ResourceIndex` per (resDir, content-fingerprint). Higher value,
  more design work.
- `XMLLayoutParseCache` (Path B, ~71ms wall) — extend FlatNode
  parse cache with `xml/` subtree. Low-risk, follows `#308` exactly.
  Smaller absolute win.

**Java parse cache savings (for reference):**

- `javaIndexing` with cache: 168ms
- `javaIndexing` without cache: 524ms
- Parse cache saves **~356ms** per warm run. Same pattern would
  apply to XMLLayoutParseCache (smaller corpus).

## Action items

1. Update issue #335 (PerShardBloomUnion) with measurement data.
2. Update issue #336 (XMLParseCacheExtension) to reflect the two-path
   split; rename to `ResourceIndexCache` or split into two issues.
3. File new issue for `XMLLayoutParseCache` (tree-sitter layout cache)
   as the lower-risk / lower-reward counterpart.
