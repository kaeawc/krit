# AstRewriteDataClassImmutable

**Cluster:** [rule-quality](README.md) · **Status:** planned ·
**Severity:** n/a (refactor)

## Current problem

`DataClassShouldBeImmutableRule.CheckNode` uses
`strings.Contains(text, "var ")` on the full class text to detect
mutable properties. A string literal containing "var " inside the
class triggers the rule.

## Proposed fix

Walk `class_parameter` children of the `primary_constructor`:
1. For each `class_parameter`, check `binding_pattern_kind` child
   for "var"
2. Also walk `class_body` → `property_declaration` children for
   `var` properties declared in the body

No text search on the full class.

## Source

`internal/rules/style_classes.go` line ~359

## Links

- Parent: [`roadmap/67-rule-quality.md`](../../67-rule-quality.md)
