# AstRewriteAndroidSourceExtraBatch

**Cluster:** [rule-quality](README.md) · **Status:** planned ·
**Severity:** n/a (refactor)

## Rules covered

Batch rewrite for string-matching rules in
`internal/rules/android_source_extra.go`:

- `GridLayoutRule` — text match for `columnCount` attribute
- `InstantiatableRule` — text match for constructor presence
- `LayoutInflationRule` — `strings.Contains` for `inflate(`
- `MissingPermissionRule` — text match for permission API names
- `ViewConstructorRule` — text match for constructor signatures
- `ViewTagRule` — text match for `setTag(` with framework objects

## Approach

Most of these use `strings.Contains(nodeText, "apiName(")` where
the AST has a `call_expression` with a `simple_identifier` or
`navigation_expression` callee child. Replace text search with
callee-name extraction via `scanner.FindChild` or the existing
`callExpressionName` helper in `performance.go`.

## Source

`internal/rules/android_source_extra.go`

## Links

- Parent: [`roadmap/67-rule-quality.md`](../../67-rule-quality.md)
