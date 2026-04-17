# MigrateTypeinferScopes

**Cluster:** [flat-tree-migration](README.md) · **Status:** completed ·
**Phase:** cleanup tail

## What it does

Move scope building and smart-cast analysis in `internal/typeinfer`
off node-based parent/child traversal.

Primary file:

- `internal/typeinfer/scopes.go`

Scope includes:

- `if` null-check extraction
- `is`-check extraction
- conjunction smart-casts
- `when` subject/branch analysis
- `requireNotNull` handling
- elvis early-exit handling
- lambda parameter / implicit `it` inference
- destructuring and loop-variable scope handling

## Current state

The production scope-building path is now flat-native:

- `buildScopesFlat(0, ...)` handles source-file root traversal directly
- `if` / `when` smart-cast analysis uses flat child and flat parent traversal
- function/lambda parameter indexing is flat-native on the live path
- loop-variable and destructuring scope population use flat-native helpers when
  indexing from the flat tree

Compatibility wrappers that still accept `*sitter.Node` remain for fallback
paths and tests, but live scope construction no longer depends on node
traversal.

## Acceptance criteria

- Scope construction and smart-cast analysis are flat-native.
- Parent climbing uses flat parent traversal.
- No live scope-building path requires `*sitter.Node`.
- Existing `internal/typeinfer` tests pass.

## Links

- Parent: [`roadmap/68-flat-tree-migration.md`](../../68-flat-tree-migration.md)
- Depends on:
  - [`migrate-typeinfer-api.md`](migrate-typeinfer-api.md)
  - [`migrate-typeinfer-declarations.md`](migrate-typeinfer-declarations.md)
  - [`migrate-typeinfer-resolve.md`](migrate-typeinfer-resolve.md)
