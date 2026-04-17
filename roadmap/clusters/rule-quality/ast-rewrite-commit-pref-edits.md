# AstRewriteCommitPrefEdits

**Cluster:** [rule-quality](README.md) · **Status:** planned ·
**Severity:** n/a (refactor)

## Current problem

`CommitPrefEditsRule.CheckNode` uses `strings.Contains(text, ".edit()")`
and `strings.Contains(text, ".commit()")` on the full node text. A
comment like `// don't forget to call .edit()` triggers the rule.

## Proposed fix

Walk the call chain structurally:
1. Match `call_expression` whose callee ends in `.edit`
2. Walk sibling statements in the same block for a
   `call_expression` whose callee ends in `.commit` or `.apply`
   on the same receiver
3. Flag only when the structural chain is broken

## Example false positive (current)

```kotlin
// Call .edit() on the prefs object, then .commit()
fun docs() {
    // This comment triggers the rule because it contains .edit()
}
```

## Source

`internal/rules/android_correctness.go` line ~97

## Links

- Parent: [`roadmap/67-rule-quality.md`](../../67-rule-quality.md)
