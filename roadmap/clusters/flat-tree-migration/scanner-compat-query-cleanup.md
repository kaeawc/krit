# ScannerCompatQueryCleanup

**Cluster:** [flat-tree-migration](README.md) · **Status:** completed ·
**Phase:** cleanup tail

## What it does

Clean up node-based scanner compatibility code that is no longer part of
the active flat dispatch path.

Primary files:

- `internal/scanner/scanner.go`
- `internal/scanner/suppress.go`
- `internal/scanner/index.go`

Scope includes:

- retiring or isolating old node-based helper functions
- removing obsolete node-based suppression/indexing paths once no live
  callers remain
- deciding whether scanner query compatibility stays or is retired

## Current state

This is now closed as cleanup/documentation work:

- the active Kotlin rule-dispatch path is flat-native after parse
- `internal/scanner/query.go` is already gone from production code
- suppression indexing is flat-only via `BuildSuppressionIndexFlat(...)`
- the remaining node helper utilities in `internal/scanner/scanner.go` are
  explicitly isolated compatibility shims for residual node-based callers

## Acceptance criteria

- The active scanner runtime path is clearly flat-native after parse.
- Compatibility helpers are either removed or explicitly isolated.
- Scanner query compatibility is either retired or documented as an
  isolated compatibility subsystem.
- Existing `internal/scanner` and `internal/rules` tests pass.

## Links

- Parent: [`roadmap/68-flat-tree-migration.md`](../../68-flat-tree-migration.md)
- Parallel with: all `typeinfer` cleanup items
