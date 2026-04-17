# AstRewriteAvoidReferentialEquality

**Cluster:** [rule-quality](README.md) · **Status:** planned ·
**Severity:** n/a (refactor)

## Current problem

`AvoidReferentialEqualityRule.CheckNode` uses
`strings.TrimSpace(scanner.NodeText(node.Child(0), ...))` and
compares against `"null"`. The null-literal check works but the
overall equality operator detection relies on the node having
exactly 3 children with the middle one being `===` or `!==`.

## Proposed fix

The current implementation is close to structural — the main
improvement is to use `FindChild(node, "===")` or check the
operator child type directly rather than counting children and
trimming text. Also: if tree-sitter exposes the operator as a
named child, use that instead of positional indexing.

## Source

`internal/rules/potentialbugs_types.go` line ~70

## Links

- Parent: [`roadmap/67-rule-quality.md`](../../67-rule-quality.md)
