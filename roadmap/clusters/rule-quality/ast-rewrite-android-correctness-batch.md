# AstRewriteAndroidCorrectnessBatch

**Cluster:** [rule-quality](README.md) · **Status:** planned ·
**Severity:** n/a (refactor)

## Rules covered

Batch rewrite for string-matching rules in
`internal/rules/android_correctness.go` and
`android_correctness_checks.go`:

- `SetTextI18nRule` — `strings.Contains(text, ".setText(\"")` →
  AST: call_expression callee `setText`, string_literal argument
- `DefaultLocaleRule` — regex for `.toLowerCase()`/`.toUpperCase()` →
  AST: call_expression callee name match
- `NestedScrollingRule` — text heuristic for nested scroll containers →
  AST: call_expression chain depth check
- `ScrollViewCountRule` — text match for ScrollView child count →
  AST: walk class body for ScrollView children
- `AppCompatMethodRule` — regex for framework method names →
  AST: call_expression callee name against allowlist

## Approach

Each rule gets the same treatment:
1. Replace `strings.Contains(nodeText, ...)` with
   `scanner.FindChild` + callee name comparison
2. Keep the rule's skip logic (test file, annotation, etc.)
3. Verify with existing positive/negative fixtures

## Source

`internal/rules/android_correctness.go`,
`internal/rules/android_correctness_checks.go`

## Links

- Parent: [`roadmap/67-rule-quality.md`](../../67-rule-quality.md)
