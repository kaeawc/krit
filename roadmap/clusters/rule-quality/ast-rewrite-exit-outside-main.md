# AstRewriteExitOutsideMain

**Cluster:** [rule-quality](README.md) · **Status:** planned ·
**Severity:** n/a (refactor)

## Current problem

`ExitOutsideMainRule.CheckNode` uses string matching on the call
text to detect `System.exit()` / `exitProcess()`.

## Proposed fix

Match `call_expression` callee name via AST child walk. Check the
`simple_identifier` or `navigation_expression` last segment for
`exit` / `exitProcess`. Verify the enclosing function is not
`main` by walking ancestors.

## Source

`internal/rules/potentialbugs_lifecycle.go` line ~37

## Links

- Parent: [`roadmap/67-rule-quality.md`](../../67-rule-quality.md)
