# MigrateTypeinferDeclarations

**Cluster:** [flat-tree-migration](README.md) · **Status:** completed ·
**Phase:** cleanup tail

## What it does

Move declaration indexing in `internal/typeinfer` off node-based tree
walking and onto the flat tree.

Primary files:

- `internal/typeinfer/declarations.go`
- `internal/typeinfer/imports.go`

Scope includes:

- import and package extraction
- class/function/property/object indexing
- member extraction
- supertype extraction
- delegate / initializer indexing helpers that still depend on node walks

## Current state

The production declaration-indexing path is now flat-native:

- `internal/typeinfer/imports.go` extracts package/import headers from the
  flat tree
- `internal/typeinfer/declarations.go` indexes source-file declarations from
  flat root index `0`
- class/function/property/object indexing all have flat-native entrypoints
- member extraction, supertype extraction, enum entry extraction, and
  delegate/property type resolution all have flat-native helpers on the live
  path

Compatibility wrappers that still accept `*sitter.Node` remain in place for
tests and fallback paths, but production indexing no longer depends on node
walking.

## Acceptance criteria

- Declaration indexing no longer requires `*sitter.Node` traversal.
- Imports and package extraction are flat-native.
- Class/function/property/object indexing is flat-native.
- Existing `internal/typeinfer` tests pass.

## Links

- Parent: [`roadmap/68-flat-tree-migration.md`](../../68-flat-tree-migration.md)
- Depends on: [`migrate-typeinfer-api.md`](migrate-typeinfer-api.md)
- Parallel with: [`migrate-typeinfer-resolve.md`](migrate-typeinfer-resolve.md)
