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

## Polish layer

- [`per-file-arena-allocation.md`](per-file-arena-allocation.md)
- [`worker-pinned-parallel-scan.md`](worker-pinned-parallel-scan.md)
