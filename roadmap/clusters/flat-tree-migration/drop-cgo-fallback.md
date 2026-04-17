# DropCgoFallback

**Cluster:** [flat-tree-migration](README.md) · **Status:** in progress ·
**Phase:** cleanup tail (final closure)

## What it does

Final closure of the flat-tree migration. The dispatcher fallback path
was already removed — live Kotlin rule dispatch is flat-only today.
What remains is deleting the compatibility surfaces that let the
migration run incrementally, and capturing a fresh performance baseline
now that the work is essentially done.

This item blocks on the `typeinfer` and scanner-compat tracks because
those two subsystems still use the compat helpers this item wants to
delete.

## What's already done

- `walkDispatch` in `internal/rules/dispatch.go` iterates
  `file.FlatTree.Nodes` directly; no recursive sitter-node descent
  exists on the live path.
- `FlatDispatchRule.CheckFlatNode(idx uint32, file *scanner.File)` is
  the only dispatch interface — there is no `CheckNode(*sitter.Node,
  ...)` fallback anywhere in the rule registration code.
- The `flattenTree` function no longer returns a parallel
  `[]sitter.Node` slice; the `file.FlatNode(idx)` escape hatch that
  earlier versions of this cluster referenced does not exist.
- Rule-side cgo usage has dropped from ~996 call sites to ~212 and is
  actively shrinking as the rule tail completes.
- `scanner.File` no longer retains `Tree *sitter.Tree`; parsed files keep only
  `FlatTree` plus source text.
- `internal/typeinfer` and scanner-compat concept tracks are now complete; the
  remaining blockers are rule-side node-era code and residual compatibility
  interfaces still used by `internal/oracle`.

## What's still left

### 1. Delete rule-side compatibility helpers

Two files exist purely to keep the remaining node-era rule code
compiling:

- `internal/rules/node_compat_helpers.go` — `callName()` and related
  node-based extraction helpers.
- `internal/rules/node_compat_flow_helpers.go` —
  `ifConditionAndThenBody()`, `ifConditionThenElseBodies()`, and other
  positional-child extraction helpers.

Both get deleted once the rule files that call them migrate to flat
equivalents. Verification gate:

```bash
grep -rn '\*sitter\.Node' internal/rules/*.go | grep -v _test.go
# must return zero
```

### 2. Delete scanner.go compatibility functions

`internal/scanner/scanner.go:423–533` still exports ten node-based
helper functions: `ForEachChild`, `ForEachChildOfType`, `NodeBytes`,
`NodeText`, `WalkNodes`, `WalkAllNodes`, `FindChild`, `HasModifier`,
`HasChildOfType`, `HasAncestorOfType`. These are still called from
both `internal/rules/` (~141 call sites, shrinking) and
compatibility wrappers outside rules. These are now explicitly marked
deprecated in code; once the remaining callers drop to zero they're
deleted wholesale.

### 3. Drop `scanner.File.Tree *sitter.Tree`

This is already done in the current tree:

- `scanner.File` stores `FlatTree` only.
- `ParseFile` / `NewParsedFile` flatten immediately and do not retain the
  cgo tree on the file object.
- The remaining work here is the final post-closure benchmark capture.

Expected impact on a 2,435-file Signal-Android run: the C parse tree
for each file can be freed right after `flattenTree` returns, rather
than at GC time after the whole pipeline completes. That's the main
memory win this cluster was pursuing.

### 4. Fresh performance capture

The original parent-roadmap justification cites `1,208ms of 2,415ms`
on Signal-Android — "dispatch is 49% of wall time." That number is
from before this cluster started and the current
`benchmarks/2026-04-09.md` shows Signal-Android taking 12.71s cold
start with 112 rules, so we don't actually know what the win looks
like today.

Before declaring the cluster done:

- Run `krit --perf` against Signal-Android on main. Capture
  `DispatchWalkMs`, `DispatchRuleNs`, `flattenTree` cost, and total
  wall time broken out by phase (`RunStats` already tracks these).
- Drop the capture into `benchmarks/<date>.md` so future work has a
  reference point.
- Separately run `krit --perf` after step 3 completes (Tree dropped)
  to show the memory/wall-time delta from the final cleanup step. If
  the delta is too small to care about, we've still completed the
  architectural cleanup — the benchmark just sets honest expectations
  for the next caller who might propose a similar effort.

## Acceptance criteria

- `grep -rn '\*sitter\.Node' internal/rules/*.go` (excluding tests)
  returns zero.
- `internal/rules/node_compat_helpers.go` and
  `internal/rules/node_compat_flow_helpers.go` are deleted.
- `internal/scanner/scanner.go:423–533` — the remaining cgo helpers — are
  deleted, or exported via a deprecation comment that points callers to
  `FlatTree` helpers.
- `scanner.File.Tree` field is gone. `ParseFile` does not retain the
  parse tree beyond `flattenTree`.
- Full `go test ./...` passes.
- A fresh `krit --perf` capture exists in `benchmarks/` and the
  parent roadmap's performance claim is updated to match.

## Links

- Parent: [`roadmap/68-flat-tree-migration.md`](../../68-flat-tree-migration.md)
- Blocks on:
  - Rule tail completion (tracked in README, workers are on it)
  - [`migrate-typeinfer-api.md`](migrate-typeinfer-api.md) and
    downstream typeinfer items (they're the last callers of the
    scanner compat helpers)
  - [`scanner-compat-query-cleanup.md`](scanner-compat-query-cleanup.md)
- Benchmarks: `benchmarks/2026-04-09.md` (current baseline)
