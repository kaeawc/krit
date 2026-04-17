# AstRewriteAudit

**Cluster:** [rule-quality](README.md) · **Status:** planned ·
**Severity:** n/a (audit)

## What it is

A one-time pass through all 77 string-matching CheckNode rules to
categorize each as:

- **Rewritten** — converted to AST walk (items above)
- **Hybrid** — string match kept as fast-path, confirmed by AST
- **Justified** — string match is correct because the node content
  is inherently textual (comments, string literals, regex patterns)

## Deliverable

A table in this file (or a companion markdown) listing every rule,
its category, and whether a rewrite was applied. This serves as the
completion artifact for the AST-rewrite effort.

## Acceptance criteria

- Every rule in the 77-rule list is categorized
- Every "rewritten" rule passes its existing fixtures unchanged
- Every "justified" rule has a code comment explaining why
- `krit --rule-audit` across the 6-repo integration set shows
  no regression from the rewrites

## Links

- Parent: [`roadmap/67-rule-quality.md`](../../67-rule-quality.md)
