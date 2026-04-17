# Performance infrastructure cluster

Parent: [`roadmap/65-performance-infra.md`](../../65-performance-infra.md)

Structural changes to close the performance gap with native tooling
without rewriting in another language. Each item is independently
landable and measurable via `krit --perf`.

## Foundation (land first)

- [`flat-node-representation.md`](flat-node-representation.md)
- [`string-interning.md`](string-interning.md)

## Core optimizations (land after foundation)

- [`columnar-finding-storage.md`](columnar-finding-storage.md)
- [`precompiled-query-expansion.md`](precompiled-query-expansion.md)
- [`zero-copy-node-text.md`](zero-copy-node-text.md)

## Migration (delivers the actual speedup)

- [`dispatcher-flat-tree-migration.md`](dispatcher-flat-tree-migration.md)

## Resilience

- [`oracle-crash-resilience.md`](oracle-crash-resilience.md)

## Warm-run speedup (surfaced 2026-04-17 via `--perf` on Signal-Android)

Oracle warm cache delivered the advertised 178× on that phase (168ms
from 30s). But `crossFileAnalysis` now dominates the warm run at 3,580ms
out of 7,036ms total. These three items together target the
oracle-analog of each remaining warm-run phase:

- [`cross-file-index-cache.md`](cross-file-index-cache.md) — cache
  bloom filter + Kotlin/Java/XML reference maps between runs. Biggest
  single lever: 3,580ms → ~300ms.
- [`parse-result-cache.md`](parse-result-cache.md) — content-addressable
  tree-sitter parse cache. 593ms → ~120ms.
- [`cross-rule-hotspot-visible-for-testing.md`](cross-rule-hotspot-visible-for-testing.md)
  — one cross-file rule alone eats 723ms on Signal warm; narrow rule
  hotspot fix to bring that under 100ms. Template for a broader
  hotspot pass once shipped.

Combined target: warm Signal 7s → ~3.5s end-to-end, without regressing
finding equivalence.

## Polish layer

- [`per-file-arena-allocation.md`](per-file-arena-allocation.md)
- [`worker-pinned-parallel-scan.md`](worker-pinned-parallel-scan.md)
