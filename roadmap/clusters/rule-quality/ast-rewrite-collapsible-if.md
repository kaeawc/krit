# AstRewriteCollapsibleIfStatements

**Cluster:** [rule-quality](README.md) · **Status:** planned ·
**Severity:** n/a (refactor)

## Current problem

`CollapsibleIfStatementsRule.CheckNode` partially uses AST but
falls back to text checks to determine if the inner body is a
single `if` statement.

## Proposed fix

Fully structural: `if_expression` → `control_structure_body` →
single child is another `if_expression` with no `else` branch.
All three checks are AST child walks.

## Source

`internal/rules/style_expressions.go` line ~738

## Links

- Parent: [`roadmap/67-rule-quality.md`](../../67-rule-quality.md)
