# GoToTest

**Cluster:** [sdlc/lsp-integration](README.md) · **Status:** planned · **Severity:** n/a (LSP)

## Concept

Resolve a symbol to the test file(s) that cover it; expose as an
LSP command / context menu entry.

## Shape

Cursor on `UserRepository.get` in `src/main/` → code action
`Go to test: UserRepositoryTest.kt` (or one action per matching test
file) → jump to the test declaration range, not just the file top.

If no test-side references exist, the action should be omitted rather
than opening a fuzzy file search.

## Dispatch

- Surface the entry from `handleCodeAction` in
  `internal/lsp/server.go`, since the feature is a context-menu action
  on the current symbol.
- Resolve the symbol under cursor with the existing helpers in
  `internal/lsp/definition.go`: `identifierAtPosition(...)` and
  `findDeclaration(...)`.
- Extend `internal/lsp/protocol.go` so `CodeAction` can carry a
  command payload for `krit.goToTest`; wire `workspace/executeCommand`
  through `Server.handleMessage(...)` in `internal/lsp/server.go`.
- Return existing LSP `Location` values from `internal/lsp/protocol.go`
  so the command result can reuse the same URI/range shape as
  `textDocument/definition` and `textDocument/references`.

## Infra reuse

- Workspace root already comes from `Server.rootURI` and `uriToPath(...)`
  in `internal/lsp/server.go`.
- Build the cross-file symbol/reference view with
  `scanner.CollectKotlinFiles(...)`, `scanner.ParseFile(...)`, and
  `scanner.BuildIndex(...)` from `internal/scanner/`.
- Use `CodeIndex.Symbols` plus `CodeIndex.ReferenceFiles(name)` in
  `internal/scanner/index.go` to map a public declaration in `src/main/`
  to referencing files under `src/test/` or `src/androidTest/`.
- Test-source partitioning should reuse the same path conventions already
  encoded in `internal/rules/package_dependency_cycle.go`
  (`shouldSkipPackageDependencyCycleFile`) and
  `internal/rules/deadcode.go` (`shouldSkipSymbol`), but inverted to
  keep only test roots.
- This is the same core data split described by
  [`../testing-infra/untested-public-api.md`](../testing-infra/untested-public-api.md):
  non-test public symbols on one side, test-file references on the
  other.
- Coverage for the eventual implementation belongs in
  `internal/lsp/server_test.go`, next to the existing definition and
  references handler tests.

## Links

- Parent: [`../README.md`](../README.md)
