# PositionToExpressionResolver

**Cluster:** [lsp](README.md) · **Status:** open ·
**Severity:** n/a (infra) · **Default:** n/a · **Est.:** 2 days

## What it does

Maps an LSP `{uri, line, character}` position to the oracle expression
id that covers that position, so navigation handlers can look up the
resolved FQN and type.

Without this adapter, milestone 1's FQN index is unreachable from an
LSP client — the client sends `line:col`, the oracle is keyed by
expression id.

## Current cost

Three coordinate systems in play:

1. **LSP positions** — UTF-16 code unit offsets, 0-based line, 0-based
   character. Defined by the LSP spec.
2. **Tree-sitter byte offsets** — UTF-8 byte offsets into the file.
   Used by `scanner.File.FlatStartByte` / `FlatEndByte`.
3. **Oracle source ranges** — currently stored by `krit-types` as
   `{startOffset, endOffset}` in Kotlin's PSI byte offsets, which
   match tree-sitter UTF-8 byte offsets except when BOMs or CRLF
   are present.

The existing LSP code converts LSP positions to byte offsets via
`flatByteOffsetAtPosition` in
[`internal/lsp/definition.go:51`](../../../internal/lsp/definition.go:51),
which handles UTF-16 → UTF-8 approximately (it treats one LSP
character as one UTF-8 byte — wrong for emoji, full-width CJK, and
multi-byte symbols). That's fine for single-file textual lookup on
ASCII-heavy Kotlin code, less fine when the oracle disagrees about
where a byte lives.

## Proposed design

### Coordinate adapter

```go
// internal/lsp/coords.go

// PositionToByteOffset converts an LSP Position to a UTF-8 byte offset
// in file content, handling UTF-16 surrogates correctly.
func PositionToByteOffset(content []byte, pos Position) int

// ByteOffsetToPosition is the inverse.
func ByteOffsetToPosition(content []byte, offset int) Position
```

Replaces the current byte-counting approximation with a proper
line-index + UTF-16-width walker. Memoize the line-start table on
`scanner.File` since we already maintain one for diagnostics.

### Expression lookup

```go
// internal/oracle/resolver.go

// ExpressionAtPosition returns the innermost expression covering
// (file, byteOffset) in the assembled oracle, or nil if no expression
// covers that offset.
func (idx *Index) ExpressionAtPosition(file string, offset int) *ExprInfo
```

Implementation: per-file sorted expression ranges (built once during
index construction), binary search for the range enclosing `offset`,
then walk to the innermost (smallest span) match.

The oracle's expressions are a subset of AST nodes — not every byte
is covered. When the cursor is on whitespace or a keyword, the
resolver returns nil and the navigation handler falls back to tree-sitter
identifier-at-cursor logic.

### Identifier fallback

When there's no oracle expression at the cursor (e.g., a simple
identifier reference that the oracle didn't record as a standalone
expression, like a type name in a parameter list), fall back to
tree-sitter to find the identifier text, then look up by name via a
secondary index keyed by simple name with scope disambiguation:

```go
func (idx *Index) FindDeclarationBySimpleName(name string, scope *ScopeContext) []*DeclLocation
```

Scope context comes from walking up the AST at the cursor (file
imports, enclosing package, enclosing class). Returns all candidates
that could resolve in that scope; LSP clients render a disambiguation
picker when there's more than one.

## Files to touch

- `internal/lsp/coords.go` — new, ~150 lines, proper UTF-16 ↔ UTF-8
- `internal/oracle/resolver.go` — new, ~200 lines, position → expression
- `internal/oracle/resolver_test.go` — unit tests
- `internal/lsp/definition.go` — callers migrate to new resolver in
  milestone 3 (this milestone only adds the adapter, doesn't wire it
  to handlers)

## Testing

- Fixture 1: position on a property-access chain `a.b.c` — resolver
  should pick the innermost matching expression (`a.b.c`, `a.b`, `a`)
  based on cursor column within each identifier.
- Fixture 2: file with emoji in a string literal before the cursor.
  UTF-16 → UTF-8 conversion must land on the right byte.
- Fixture 3: cursor on whitespace / comment / keyword. Expect nil
  expression; fallback should kick in.
- Fixture 4: CRLF line endings. Line index must account for the `\r`.
- Fixture 5: identifier-fallback with shadowed names (local `x` vs
  enclosing class `x`). Verify scope disambiguation candidates.

## Risks

- **UTF-16 width bugs**. Most common source of off-by-one errors in
  LSP servers. Fuzz test with random unicode content.
- **Oracle expression coverage gaps**. Kotlin Analysis API produces
  expressions for method calls, property accesses, and some
  declarations, but not every AST node. Non-covered positions need
  fallback, which is lower fidelity.
- **Scope context build cost**. Building `ScopeContext` on every
  query by walking AST up could be expensive in deep nesting. Cache
  per-file per-edit.

## Blocking

- None beyond milestone 1's output format.

## Blocked by

- [`fqn-symbol-index.md`](fqn-symbol-index.md) (milestone 1) — we
  reuse the same per-file expression map that milestone 1 builds.

## Links

- Parent cluster: [`lsp/README.md`](README.md)
- Consumer: [`navigation-handlers-rewrite.md`](navigation-handlers-rewrite.md)
- Relevant: [`internal/lsp/definition.go:51-96`](../../../internal/lsp/definition.go:51)
  (current coordinate approximation)
