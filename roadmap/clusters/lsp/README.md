# LSP semantic navigation cluster

Parent: [`roadmap/09-lsp-server.md`](../../09-lsp-server.md) (Phase 1–5
shipped 2026-04-08 — diagnostics, code actions, formatting, hover,
symbols, incremental sync, config reload).

## Scope

This cluster turns `krit-lsp` from a diagnostics-and-quick-fix server
into a navigation-capable language server by consuming the existing
Kotlin Analysis API oracle as a symbol index. All goto-definition,
find-references, rename, and hover operations become cross-file and
cross-project, rather than single-file textual walks.

## Motivation

The current navigation handlers in [`internal/lsp/definition.go`](../../../internal/lsp/definition.go)
are honest stubs: `findDeclarationFlat` and `findAllIdentifiersFlat`
walk only the open file's AST and match by identifier text. This means:

- Go to Definition on `CoroutineScope` never finds the declaration in
  `kotlinx.coroutines` — only same-file matches, or nothing.
- Find References returns every identifier node with the matching text
  in the current file; a local `val x` and a class member `x` are
  indistinguishable.
- Rename is single-file-only and unsafe to run on any shared symbol.
- Hover shows only rule findings, never type information.

Krit already pays the cost of running the oracle (`krit-types` JVM
subprocess + on-disk content-addressable cache) for rule-time type
information. That same oracle already emits per-file declarations
with positions and kinds, and expressions with resolved-call FQNs.
Every piece of data a proper navigation handler needs is already
being computed — it's just not indexed for reverse lookup or exposed
to the LSP.

## Goals

- Go to Definition jumps to the actual declaration site, in this file
  or any other file in the project.
- Find References returns every resolved use of a symbol across the
  workspace, not every text match in one file.
- Rename is workspace-wide and scope-aware.
- Hover shows type signatures from the oracle, not just rule findings.
- First query cold-start stays under ~2s on Signal-scale projects;
  warm queries stay under ~100ms.
- When the oracle is unavailable (cold fall-back, degraded mode), the
  current textual handlers remain as a fallback rather than returning
  nothing.

## Non-goals

- Replacing the rule engine with a semantic analyzer.
- Editor-side UI changes (completion popups, signature help UI, etc.).
  The existing VS Code / Neovim / IntelliJ client extensions already
  consume whatever capabilities the server advertises.
- Semantic rename of symbols defined in external JARs (we can jump
  *to* them via decompiled sources but not mutate them).

## Dependency graph

```
Foundation               Core handlers            Edit loop          Reach
──────────               ─────────────            ─────────          ─────
fqn-symbol-index ────┬── navigation-handlers ── didchange-oracle ── workspace-index
                     │       rewrite              refresh              initialize
position-to-expr ────┘                                                    │
  resolver                                                                │
                                                                          └── jar-source
                                                                              navigation
                                                                              (optional)
```

Milestones 1 and 2 are prerequisites for everything else. Milestone 3
is what the user feels. Milestone 4 keeps the feel alive while editing.
Milestones 5 and 6 are reach: one for first-open responsiveness on
large projects, one for cross-JAR navigation.

## Milestones

| # | Item | Est. days |
|---|------|-----------|
| 1 | [`fqn-symbol-index.md`](fqn-symbol-index.md) — reverse-lookup oracle API: `FindDeclarationByFQN`, `FindReferencesByFQN`, `TypeAtPosition` | 2–3 |
| 2 | [`position-to-expression-resolver.md`](position-to-expression-resolver.md) — map LSP position → oracle expression id → resolved FQN | 2 |
| 3 | [`navigation-handlers-rewrite.md`](navigation-handlers-rewrite.md) — rewrite `handleDefinition` / `handleReferences` / `handleRename` / `handleHover` to consume the oracle index, keep textual walkers as fallback | 2–3 |
| 4 | [`didchange-oracle-refresh.md`](didchange-oracle-refresh.md) — debounced `didChange` → oracle re-analyze for the changed file; invalidate reverse index entries | 2 |
| 5 | [`workspace-index-initialize.md`](workspace-index-initialize.md) — build the cross-file reverse index on `initialize`; background warm-up; indexing-progress reporting via `$/progress` | 3–5 |
| 6 | [`jar-source-navigation.md`](jar-source-navigation.md) — Analysis API decompile for stdlib/third-party, synthetic `jar://` URIs, jump into stdlib sources (optional) | 3–5 |

**Total 1–5: ~2 weeks.** Milestone 6 is optional polish.

## Acceptance criteria

- Goto-def on `CoroutineScope` from an Android app file jumps into
  `kotlinx-coroutines-core`'s decompiled source (milestone 6) or at
  minimum returns a signature-stub location with the FQN visible in
  hover (milestones 1–5).
- Find References on a top-level function returns every call site in
  the workspace, not just in the same file.
- Rename of a class member across a multi-module project produces a
  `WorkspaceEdit` touching every reference file; applying it and
  re-running `krit-lsp` shows zero diagnostics from the rename itself.
- On Signal-Android: warm initialize + first goto-def < 2s; subsequent
  goto-def < 100ms.
- On kotlin/kotlin: warm initialize + first goto-def < 5s; subsequent
  goto-def < 200ms.

## Capability advertisement

As of today, the server advertises `definitionProvider`,
`referencesProvider`, `renameProvider`, `completionProvider`,
`hoverProvider`, `documentSymbolProvider` — but the single-file
textual implementation means the first four return misleading results.

Until milestone 3 lands, one mitigation is to stop advertising these
capabilities so clients don't show broken goto-def. That's a tradeoff
decision tracked in [`distribution-readiness.md`](../release-engineering/distribution-readiness.md)
(doc claims vs reality). Once milestone 3 ships, the capabilities
become real and the mitigation is reverted.

## Out-of-scope follow-ups

- **Signature help** (`textDocument/signatureHelp`) — needs parser
  cursor context in the middle of a call expression. Oracle has the
  data; handler doesn't exist. Straightforward 2-3 day add after
  milestone 3.
- **Semantic tokens** (`textDocument/semanticTokens`) — full syntax
  coloring from the oracle instead of tree-sitter grammar. Large
  payoff for Neovim/other editors without Kotlin grammars; separate
  cluster entirely.
- **Inlay hints** (parameter names, inferred types) — oracle has the
  data. Post-navigation cluster.

## Related

- Parent LSP doc: [`roadmap/09-lsp-server.md`](../../09-lsp-server.md)
- Oracle infrastructure: [`internal/oracle/`](../../../internal/oracle/)
- Type inference service: [`internal/typeinfer/`](../../../internal/typeinfer/)
- Related refactors:
  [`core-infra/type-resolution-service.md`](../core-infra/type-resolution-service.md)
  (oracle-through-context), [`core-infra/cache-unification.md`](../core-infra/cache-unification.md)
  (shared content-hash cache)
