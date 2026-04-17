# AstRewriteErrorUsageWithThrowable

**Cluster:** [rule-quality](README.md) · **Status:** planned ·
**Severity:** n/a (refactor)

## Current problem

`ErrorUsageWithThrowableRule.CheckNode` uses string matching to
detect `error(...)` calls where the argument is a Throwable.

## Proposed fix

Match `call_expression` whose `simple_identifier` child is `error`.
Check the first argument's type via typeinfer — if it resolves to
Throwable or a subclass, flag. Falls back to text heuristic only
when the resolver returns unknown.

## Source

`internal/rules/exceptions.go` line ~932

## Links

- Parent: [`roadmap/67-rule-quality.md`](../../67-rule-quality.md)
