# AstRewriteAlsoCouldBeApply

**Cluster:** [rule-quality](README.md) · **Status:** planned ·
**Severity:** n/a (refactor)

## Current problem

`AlsoCouldBeApplyRule.CheckNode` uses text matching to detect
`.also { it.x = y }` patterns where `.apply { x = y }` would be
cleaner.

## Proposed fix

Match `call_expression` with callee `also` that has a trailing
`lambda_literal`. Walk the lambda body's statements: if every
statement's receiver is `it` (the `simple_identifier` child of
a `navigation_expression` starts with `it`), the pattern matches.
AST walk, no text search.

## Source

`internal/rules/style_idiomatic_data.go` line ~107

## Links

- Parent: [`roadmap/67-rule-quality.md`](../../67-rule-quality.md)
