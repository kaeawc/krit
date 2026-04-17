# MigrateTypeinferParallelIndexing

**Cluster:** [flat-tree-migration](README.md) · **Status:** completed ·
**Phase:** cleanup tail

## What it does

Revisit `internal/typeinfer/parallel.go`, which still reparses file
content and walks `tree.RootNode()` for per-file indexing.

This item decides whether the parallel indexing path:

- stays as a local parse-only compatibility path, or
- becomes fully flat-native once declaration and scope indexing are ready

## Current state

Decision made: `parallel.go` is now fully flat-native.

`IndexFileParallel(...)` no longer reparses Kotlin source or walks
`tree.RootNode()`. It requires a parsed `scanner.File` with a populated
`FlatTree` and indexes imports, declarations, and scopes from flat root index
`0`.

## Acceptance criteria

- The intended long-term role of `parallel.go` is explicit in code.
- `parallel.go` no longer reparses and walks node trees for indexing.
- Existing `internal/typeinfer` benchmarks and tests pass.

## Links

- Parent: [`roadmap/68-flat-tree-migration.md`](../../68-flat-tree-migration.md)
- Depends on:
  - [`migrate-typeinfer-declarations.md`](migrate-typeinfer-declarations.md)
  - [`migrate-typeinfer-scopes.md`](migrate-typeinfer-scopes.md)
