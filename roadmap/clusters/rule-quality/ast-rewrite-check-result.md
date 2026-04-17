# AstRewriteCheckResult

**Cluster:** [rule-quality](README.md) · **Status:** planned ·
**Severity:** n/a (refactor)

## Current problem

`CheckResultRule.CheckNode` partially uses AST but falls back to
`strings.Contains` for some receiver detection. The string-match
path can trigger on comments or string literals containing the
matched API name.

## Proposed fix

Fully structural: walk the `call_expression` callee chain, resolve
the receiver via `navigation_expression` children, and check
whether the return value is consumed by inspecting the parent node
type (statement_expression = discarded, anything else = used).

## Source

`internal/rules/android_correctness.go` line ~177

## Links

- Parent: [`roadmap/67-rule-quality.md`](../../67-rule-quality.md)
