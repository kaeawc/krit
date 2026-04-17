# HoverShowsRuleDocs

**Cluster:** [sdlc/lsp-integration](README.md) · **Status:** planned · **Severity:** n/a (LSP)

## Concept

When hovering on a krit finding in the editor, show the rule's
documentation, rationale, and the configured severity / default
state.

## Shape

Hover over a squiggly underline → pop-up with `MagicNumber` rule
description + link to `roadmap/clusters/...`.

## Infra reuse

- Existing LSP hover handler.
- Per-concept markdown in `roadmap/clusters/` (this very tree).

## Links

- Parent: [`../README.md`](../README.md)
