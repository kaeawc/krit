# AstRewriteDoubleMutability

**Cluster:** [rule-quality](README.md) · **Status:** planned ·
**Severity:** n/a (refactor)

## Current problem

`DoubleMutabilityForCollectionRule.CheckNode` was rewritten to use
configurable type lists but still does string matching on the
property declaration text to detect `var` + mutable-collection-type
co-occurrence. A `var` property whose KDoc mentions "MutableList"
could theoretically trigger.

## Proposed fix

Walk the `property_declaration` AST children:
1. Check for `var` via the `binding_pattern_kind` child (text "var")
2. Check the type annotation child (`user_type` / `nullable_type`)
   for the mutable collection simple name
3. If no type annotation, check the initializer `call_expression`
   callee against the factory set

All checks on specific child nodes, not on the full declaration text.

## Source

`internal/rules/potentialbugs_types.go` line ~285

## Links

- Parent: [`roadmap/67-rule-quality.md`](../../67-rule-quality.md)
