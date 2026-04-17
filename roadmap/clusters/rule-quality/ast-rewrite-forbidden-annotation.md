# AstRewriteForbiddenAnnotation

**Cluster:** [rule-quality](README.md) · **Status:** planned ·
**Severity:** n/a (refactor)

## Current problem

`ForbiddenAnnotationRule.CheckNode` uses `hasAnnotationNamed` which
does `strings.Contains(nodeText, "@AnnotationName")` — text search
on the full declaration including comments and string literals.

## Proposed fix

Walk `modifiers` → `annotation` children. Each `annotation` has a
`constructor_invocation` or `user_type` child whose text is the
annotation simple name. Compare that child's text against the
forbidden list instead of searching the full node text.

This is the same fix needed for `ForbiddenOptIn`, `ForbiddenSuppress`,
and any rule using the `hasAnnotationNamed` helper.

## Rules covered

- `ForbiddenAnnotation`
- `ForbiddenOptIn`
- `ForbiddenSuppress`

## Source

`internal/rules/style_forbidden.go` lines ~327, ~388, ~435
Helper: `hasAnnotationNamed` in `internal/rules/annotations.go`

## Links

- Parent: [`roadmap/67-rule-quality.md`](../../67-rule-quality.md)
