# MigrateTypeinferResolve

**Cluster:** [flat-tree-migration](README.md) · **Status:** completed ·
**Phase:** cleanup tail

## What it does

Move core type/expression resolution in `internal/typeinfer` off
node-based helpers.

Primary files:

- `internal/typeinfer/api.go`
- `internal/typeinfer/resolve.go`
- `internal/typeinfer/helpers.go`

Scope includes:

- type-node parsing
- call-expression inference
- navigation-expression inference
- type-argument extraction
- helper primitives that still operate on `*sitter.Node`

## Current state

The live resolution path is now flat-native:

- `internal/typeinfer/api.go` exposes flat resolver APIs for resolve, lookup,
  nullability, and annotation access
- `internal/typeinfer/resolve.go` keeps flat callers on flat data for type,
  call, navigation, and type-argument resolution
- collection factory calls on the flat path preserve type arguments
- rule-side flat callers no longer route through hidden node-walk logic

Compatibility wrappers that still accept `*sitter.Node` remain for fallback
paths and tests, but production resolution no longer depends on them.

## Acceptance criteria

- Live resolution logic for flat callers stays on flat data.
- Core helper functions used by resolution are flat-native.
- Rule-side callers can use flat resolver APIs without hidden node bridges.
- Existing `internal/typeinfer` and `internal/rules` tests pass.

## Links

- Parent: [`roadmap/68-flat-tree-migration.md`](../../68-flat-tree-migration.md)
- Depends on: [`migrate-typeinfer-api.md`](migrate-typeinfer-api.md)
- Parallel with: [`migrate-typeinfer-declarations.md`](migrate-typeinfer-declarations.md)
