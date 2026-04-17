# AstRewriteCharArrayToStringCall

**Cluster:** [rule-quality](README.md) · **Status:** planned ·
**Severity:** n/a (refactor)

## Current problem

`CharArrayToStringCallRule.CheckNode` uses string matching on the
node text to find `.toString()` calls on CharArray receivers.

## Proposed fix

Walk the `call_expression` callee chain: check that the function
name child is `toString` and the receiver's resolved type (via
typeinfer) or text type annotation is `CharArray`. Falls back to
text check only when the type resolver returns unknown.

## Source

`internal/rules/potentialbugs_types.go` line ~537

## Links

- Parent: [`roadmap/67-rule-quality.md`](../../67-rule-quality.md)
