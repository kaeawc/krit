# AstRewriteExplicitCollectionElementAccessMethod

**Cluster:** [rule-quality](README.md) · **Status:** planned ·
**Severity:** n/a (refactor)

## Current problem

`ExplicitCollectionElementAccessMethodRule.CheckNode` uses string
matching to detect `.get(index)` calls that could be `[index]`.

## Proposed fix

Match `call_expression` whose callee is a `navigation_expression`
ending in `get`. Check that the receiver is a collection type (via
typeinfer or import-based heuristic). The indexing replacement
(`[index]`) is already the autofix path — the detection should
match structurally to avoid false positives on non-collection
`.get()` methods.

## Source

`internal/rules/style_idiomatic_data.go` line ~90

## Links

- Parent: [`roadmap/67-rule-quality.md`](../../67-rule-quality.md)
