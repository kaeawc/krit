# PrecompiledQueryExpansion

**Cluster:** [performance-infra](README.md) · **Status:** abandoned ·
**Severity:** n/a (infra) · **Default:** n/a

## What it was

Expand the use of tree-sitter's native S-expression query engine to
cover the ~200 rules that are simple "match node type + check text"
patterns. Moves the hot-path filtering from Go into C where
tree-sitter's query engine runs.

## Why it was abandoned

Benchmarks showed compiled queries were **472× slower** than the
equivalent Go helper (`FindChild`: 830 ns/op vs `CompiledQuery`:
391,631 ns/op). The Go/C FFI boundary, C-side traversal, result
marshaling back into `map[string]*sitter.Node`, and per-match
allocation overhead far exceeded the cost of iterating a handful of
Go pointers directly.

The "58% helper-call reduction" claimed in earlier roadmap docs
counted source-code call sites, not wall-clock time. No end-to-end
benchmark against a real repo was ever recorded.

Item 68's flat-tree migration (landed 2026-04-14) achieved the same
performance goal via a different mechanism: replacing `*sitter.Node`
trees with `[]FlatNode` arrays, making `FlatFindChild` (index-range
scan) and `FlatHasAncestor` (O(1) parent lookup) the production
primitives. This made the query approach irrelevant.

All query infrastructure (`CompiledQuery`, `MustCompileQuery`, `Exec`,
`ExecFirst`, `ExecCount`, `HasMatch`, `QueryNodes`, `FindChildByQuery`)
and 15 tests were deleted 2026-04-16.

## Links

- Parent: [`roadmap/65-performance-infra.md`](../../65-performance-infra.md)
- Superseded by: item 68 flat-tree migration
