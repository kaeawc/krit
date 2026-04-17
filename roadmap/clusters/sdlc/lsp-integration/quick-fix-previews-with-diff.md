# QuickFixPreviewsWithDiff

**Cluster:** [sdlc/lsp-integration](README.md) · **Status:** planned · **Severity:** n/a (LSP)

## Concept

For `fix level = idiomatic` / `semantic` changes, show the exact
edit preview as a diff before applying — current UX is "apply and
hope".

## Shape

LSP quick-fix menu item → shows unified diff → "Apply" / "Cancel".

## Infra reuse

- Existing binary-fix engine (already emits exact byte ranges).
- Existing LSP code-action handler.

## Links

- Parent: [`../README.md`](../README.md)
