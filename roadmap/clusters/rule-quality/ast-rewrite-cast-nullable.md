# AstRewriteCastNullable

**Cluster:** [rule-quality](README.md) · **Status:** planned ·
**Severity:** n/a (refactor)

## Rules covered

- `CastNullableToNonNullableType`
- `CastToNullableType`

## Current problem

Both rules use `strings.Contains(text, "as?")` or
`strings.SplitN(text, " as ", 2)` on the full node text to
distinguish safe casts from unsafe casts. The target type is
extracted via string splitting.

## Proposed fix

`as_expression` nodes in tree-sitter-kotlin have structured
children: the expression being cast, the `as` keyword child, and
the target type child. Walk children directly:
- Child 0 = expression
- Child matching "as" or "as?" = operator
- Last child = type

No string splitting needed.

## Source

`internal/rules/potentialbugs_nullsafety.go` lines ~1560, ~1609

## Links

- Parent: [`roadmap/67-rule-quality.md`](../../67-rule-quality.md)
