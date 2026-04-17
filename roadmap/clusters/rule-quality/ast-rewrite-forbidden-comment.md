# AstRewriteForbiddenComment

**Cluster:** [rule-quality](README.md) · **Status:** planned ·
**Severity:** n/a (refactor)

## Current problem

`ForbiddenCommentRule.CheckNode` dispatches on `comment` nodes but
uses regex on the comment text. This is one of the justified cases
— comments ARE text, and tree-sitter doesn't decompose comment
content into sub-nodes.

## Proposed action

Mark as **justified string match**. Add a code comment explaining
why this rule can't be AST-rewritten. No code change needed.

The one improvement: ensure the regex only matches inside the
comment body (after `//` or between `/*` and `*/`), not in the
leading whitespace or the comment markers themselves.

## Source

`internal/rules/style_forbidden.go` line ~99

## Links

- Parent: [`roadmap/67-rule-quality.md`](../../67-rule-quality.md)
