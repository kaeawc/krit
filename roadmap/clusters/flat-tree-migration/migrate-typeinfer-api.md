# MigrateTypeinferAPI

**Cluster:** [flat-tree-migration](README.md) · **Status:** completed ·
**Phase:** cleanup tail

## What it does

Define the flat-native public resolver surface for `internal/typeinfer`
and start retiring the node-based API.

Current node-based methods in `internal/typeinfer/resolver.go`:

- `ResolveNode(node *sitter.Node, file *scanner.File)`
- `ResolveByName(name string, atNode *sitter.Node, file *scanner.File)`
- `IsNullable(node *sitter.Node, file *scanner.File)`
- `AnnotationValue(node *sitter.Node, file *scanner.File, ...)`

This item establishes the replacement API shape so follow-on work can
parallelize cleanly.

## Current state

Partial progress has landed:

- flat-native entrypoints like `ResolveFlatNode(...)`,
  `ResolveByNameFlat(...)`, and `IsNullableFlat(...)` exist
- many flat callers now use those methods directly

Still left:

- `resolver.go` still exposes node-era methods
- `AnnotationValue(...)` is still node-first at the public API boundary
- several tests and callers still go through node wrappers

## Acceptance criteria

- The target flat-native resolver API is defined in `resolver.go`.
- Callers have flat-native equivalents for the live use cases:
  - resolve by flat idx
  - nullability by flat idx
  - annotation lookup by flat idx or equivalent flat context
- New code paths stop adding fresh `*sitter.Node` resolver dependencies.

## Links

- Parent: [`roadmap/68-flat-tree-migration.md`](../../68-flat-tree-migration.md)
- Blocks: [`migrate-typeinfer-declarations.md`](migrate-typeinfer-declarations.md),
  [`migrate-typeinfer-resolve.md`](migrate-typeinfer-resolve.md),
  [`migrate-typeinfer-scopes.md`](migrate-typeinfer-scopes.md)
