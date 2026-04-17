# AstRewriteCommitTransaction

**Cluster:** [rule-quality](README.md) · **Status:** planned ·
**Severity:** n/a (refactor)

## Current problem

`CommitTransactionRule.CheckNode` uses `strings.Contains(text,
"beginTransaction")` and checks for `.commit()` via string match.
Same comment/string-literal FP risk as CommitPrefEdits.

## Proposed fix

Walk `call_expression` chain: match `beginTransaction()` callee,
then walk sibling statements for `commit()` on the same receiver.
Use `scanner.FindChild` on the navigation_expression chain rather
than text search.

## Source

`internal/rules/android_correctness.go` line ~127

## Links

- Parent: [`roadmap/67-rule-quality.md`](../../67-rule-quality.md)
