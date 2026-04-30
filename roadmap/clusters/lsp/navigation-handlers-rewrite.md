# NavigationHandlersRewrite

**Cluster:** [lsp](README.md) · **Status:** open ·
**Severity:** n/a (infra) · **Default:** n/a · **Est.:** 2–3 days

## What it does

Rewrites `handleDefinition`, `handleReferences`, `handleRename`, and
`handleHover` in [`internal/lsp/definition.go`](../../../internal/lsp/definition.go)
and [`internal/lsp/server.go`](../../../internal/lsp/server.go) to
consume the oracle index (milestone 1) via the position resolver
(milestone 2). Single-file textual walkers remain as fallback when
the oracle is offline or the cursor lands on a non-expression position.

This milestone is what users feel. After it lands, clicking
`CoroutineScope` actually jumps to its declaration; Find References
returns cross-file results; Rename produces a workspace-wide edit.

## Current cost

All four handlers do single-file textual work today:

- `handleDefinition` — walks open file's AST for a name match,
  returns nil if not found. Can't reach symbols in other files.
  [`definition.go:13-41`](../../../internal/lsp/definition.go:13)
- `handleReferences` — walks open file's AST for all identifier text
  matches. No scope, no type, lots of false positives on shadowed
  names. [`definition.go:240-281`](../../../internal/lsp/definition.go:240)
- `handleRename` — same single-file walker, produces a `WorkspaceEdit`
  that only edits the current file. Dangerous — renaming a public
  API only updates the declaring file. [`definition.go:300-347`](../../../internal/lsp/definition.go:300)
- `handleHover` — shows only krit rule findings for the hovered line.
  [`server.go:674-718`](../../../internal/lsp/server.go:674)

## Proposed design

### Handler structure

Each handler follows the same shape:

```go
func (s *Server) handleDefinition(req *Request) {
    params := ...
    offset := PositionToByteOffset(fileContent, params.Position)

    // Preferred path: oracle index
    if idx := s.oracleIndex(); idx != nil {
        if expr := idx.ExpressionAtPosition(filePath, offset); expr != nil {
            if decl, ok := idx.FindDeclarationByFQN(expr.ResolvedFQN); ok {
                s.sendResponse(req.ID, declLocation(decl), nil)
                return
            }
        }
        // Fallback to simple-name lookup within scope
        if name := identifierAtOffset(file, offset); name != "" {
            if candidates := idx.FindDeclarationBySimpleName(name, scopeAt(file, offset)); len(candidates) > 0 {
                s.sendResponse(req.ID, declLocations(candidates), nil)
                return
            }
        }
    }

    // Oracle unavailable or no match — fall back to existing textual walker
    s.handleDefinitionTextualFallback(req, params)
}
```

Each handler gets a `handle<Op>TextualFallback` suffix variant that wraps the
current implementation unchanged. The fallback path preserves behavior when the
oracle daemon is down or still warming up, so users are no worse off than today.

### `handleDefinition`

Primary: oracle FQN → decl location. Jumps across files.

Fallback: current textual walker. No change.

### `handleReferences`

Primary: oracle `FindReferencesByFQN` returning the full workspace
list. Honors `context.includeDeclaration` via the `IsDeclaration`
flag on each `ReferenceLocation`.

Fallback: current single-file textual walker.

### `handleRename`

Primary: oracle `FindReferencesByFQN` → build a `WorkspaceEdit` with
edits grouped by file URI. Validate the new name against Kotlin
identifier rules (`isValidKotlinIdentifier`) before emitting.

Fallback: current single-file rename, **with a warning** appended to
the response as an LSP `showMessage` notification explaining the edit
is file-local only. Loud fallback is important because unnoticed
partial renames are worse than a failed rename.

### `handleHover`

Primary: compose a markdown hover that includes:

- Existing rule findings for the line (unchanged).
- Type signature from `TypeAtExpression`.
- Declaration signature from the FQN index.
- Link to the declaration location for click-through.

Fallback: current rule-finding-only hover.

## Capability advertisement

Already advertised — no protocol change. But the quality becomes real,
so the "Krit (extension)" output channel's activation log should note
whether the oracle is available:

```
[oracle] index ready: 2483 files, 41829 decls, 162403 refs
[oracle] navigation in oracle-index mode
```

or

```
[oracle] unavailable (daemon not ready); navigation in textual-fallback mode
```

## Files to touch

- `internal/lsp/definition.go` — all four handlers get oracle path,
  textual fallback path gets a temporary explicit name
- `internal/lsp/server.go` — `handleHover` gets the same treatment
- `internal/lsp/oracle_index.go` — new, thin server-side accessor
  that holds the `*oracle.Index` and refresh state
- `internal/lsp/server_test.go` — add oracle-path tests using a
  scripted oracle stub; keep existing textual-path tests as the
  fallback-mode assertions

## Testing

- Oracle available: fixture multi-file project, run each handler, assert
  results come from the index.
- Oracle unavailable: stub returns nil index. Assert handlers fall back
  to textual path with no user-visible error.
- Rename with oracle: produces multi-file `WorkspaceEdit`, apply it,
  verify the edited project still parses and `krit --list-rules` still
  succeeds.
- Rename without oracle: produces single-file `WorkspaceEdit` AND a
  `showMessage` warning.
- Hover composition: assert rule findings and type info are both
  present in the markdown.

## Risks

- **Performance regression**. A cold cache on the first goto-def
  forces an oracle cold-start, which is 45s on Signal cold. Mitigation:
  milestone 5 builds the index at initialize time; meanwhile, the
  first query shows a `$/progress` notification rather than hanging.
- **Fallback shadowing real failures**. If the oracle returns a wrong
  answer, the handler might have returned that before falling back.
  Log oracle hits/misses in verbose mode so we can tell.
- **Rename scope expansion surprise**. Users who rename a local `val x`
  could be shocked if we follow it through an accidental type inference
  chain and touch other files. Conservative mitigation: local-only
  declarations (inside function bodies) always use the single-file
  walker, never the workspace index. Only top-level or class-member
  names trigger workspace-wide rename.

## Blocking

- Nothing downstream needs this to be in place, but the whole cluster
  after this one either depends on it (milestone 4) or amplifies it
  (milestones 5 and 6).

## Blocked by

- [`fqn-symbol-index.md`](fqn-symbol-index.md) (milestone 1)
- [`position-to-expression-resolver.md`](position-to-expression-resolver.md)
  (milestone 2)

## Links

- Parent cluster: [`lsp/README.md`](README.md)
- Current implementations: [`internal/lsp/definition.go`](../../../internal/lsp/definition.go),
  [`internal/lsp/server.go`](../../../internal/lsp/server.go)
- Consumer: [`didchange-oracle-refresh.md`](didchange-oracle-refresh.md)
