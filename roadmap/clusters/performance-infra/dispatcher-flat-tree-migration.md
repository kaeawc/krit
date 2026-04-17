# DispatcherFlatTreeMigration

**Cluster:** [performance-infra](README.md) · **Status:** deferred ·
**Severity:** n/a (infra)

**Superseded by [`roadmap/68-flat-tree-migration.md`](../../68-flat-tree-migration.md)**
and the [`flat-tree-migration/`](../flat-tree-migration/) cluster
which breaks this into 32 individually-landable items.

## Problem

`FlatNode`, `FlatTree`, and `flattenTree` exist in
`internal/scanner/flat.go` and are built on every parsed file
(`scanner.go:127`). But the dispatcher in `internal/rules/dispatch.go`
still walks the original `*sitter.Tree` via cgo, and all 285
`DispatchRule` implementations still take `*sitter.Node`. The flat
tree is computed and thrown away.

This means the flat-node-representation item shipped the data
structure but not the migration that delivers the 3-5x speedup.
The cgo bottleneck (49% of wall time on Signal-Android) remains.

## Benchmark baseline

Signal-Android (2,467 files):
- dispatchWalk: 16,736ms cumulative across workers
- dispatchRuleCallbacks: 17,103ms cumulative
- Wall time for ruleExecution: 1,208ms
- Total: 2,415ms

## Proposed migration

### Phase 1: Dispatcher walks FlatTree

Rewrite `(*Dispatcher).walkDispatch` in `dispatch.go` to iterate
`file.FlatTree.Nodes` (a `[]FlatNode` slice) instead of recursing
through `*sitter.Node` children via cgo. Node type matching becomes
an integer comparison against `FlatNode.Type` using the
`NodeTypeTable` index.

The dispatcher still calls `rule.CheckNode(node, file)` with a
`*sitter.Node` — but the node is looked up once from the flat tree
index, not walked recursively. This halves the cgo calls: one
lookup per matching node instead of one per child per walk step.

### Phase 2: Compatibility wrappers

Add `FlatNodeRef` adapter methods to `scanner.File` so rules can
call `file.FlatFindChild(idx, "type")`, `file.FlatNodeText(idx)`,
`file.FlatHasModifier(idx, "abstract")` etc. These operate on the
flat slice with zero cgo.

Rules can be migrated incrementally: change `CheckNode` to use the
flat helpers where possible, falling back to `*sitter.Node` for
complex walks that the flat API doesn't cover yet.

### Phase 3: Bulk rule migration

Migrate rules in batches by category. Each batch:
1. Replace `scanner.FindChild(node, ...)` → `file.FlatFindChild(idx, ...)`
2. Replace `scanner.NodeText(node, content)` → `file.FlatNodeText(idx)`
3. Replace `scanner.HasModifier(node, content, ...)` → `file.FlatHasModifier(idx, ...)`
4. Run fixtures to verify identical output

Priority order: rules with the most `CheckNode` cgo calls first
(complexity, naming, style — the categories that walk the most
children).

### Phase 4: Drop cgo fallback

Once all rules use flat helpers, remove the `Tree *sitter.Tree`
field from `scanner.File` (keep only `FlatTree`). The parser pool
stays (cgo needed for parsing) but everything after parsing is
pure Go.

## Expected impact

3-5x on the dispatch phase (from 1,208ms to ~300ms on Signal).
Total scan time from 2.4s to ~1.2s. The kotlin repo (14.5s
tree-sitter-only) would drop to ~8s.

## Acceptance criteria

- `krit --perf` on Signal-Android shows ≥ 2x improvement in the
  `ruleExecution` timing bucket after Phase 1.
- All 285 dispatch rules produce identical output before and after
  migration (verified by `krit --rule-audit` across the 6-repo
  integration set).
- Zero `*sitter.Node` references in rule code after Phase 4.

## Links

- Parent: [`roadmap/65-performance-infra.md`](../../65-performance-infra.md)
- Depends on: [`flat-node-representation.md`](flat-node-representation.md) (shipped)
- Depends on: [`string-interning.md`](string-interning.md) (shipped)
- Depends on: [`zero-copy-node-text.md`](zero-copy-node-text.md) (planned)
