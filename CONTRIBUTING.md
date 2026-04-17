# Contributing to Krit

## Quick Start

```bash
make build          # Build krit, krit-lsp, krit-mcp (~1.5s)
make test           # Run all tests (~4s)
make ci             # Full CI: build + vet + test + integration + regression (~25s)
```

## Build

```bash
make build          # Recommended (injects version via ldflags)
go build -o krit ./cmd/krit/   # Manual
go vet ./...
```

## Run Tests

```bash
make test           # All tests
make integration    # Playground + CLI + LSP + MCP integration tests
make regression     # Verify playground finding counts
make bench          # Performance benchmarks
make watch          # Re-run tests on file changes (requires fswatch)
```

## Adding a New Rule

1. Create the rule struct in the appropriate `internal/rules/*.go` file.
2. Implement `DispatchRule` (preferred) or `LineRule`.
3. Register in an `init()` via `v2.Register(&YourRule{...})` (or the
   manifest / resource / gradle equivalent) — the generator consolidates
   this into `zz_registry_gen.go` at generate-time, so authors still
   write the register call normally.
4. If the rule has config options, refresh generated metadata:
   `python3 tools/rule_inventory.py && go generate ./internal/rules/...`.
   For rules with exotic config (multi-field writes, whole-config reads,
   value transforms), hand-write `meta_YourRule.go` and add the struct
   type to `excludedStructs` in
   `internal/codegen/cmd/krit-gen/main.go`.
5. Rule default-activity is declared via `DefaultActive` in the
   generated `Meta()` descriptor — no separate `DefaultInactive` edit.
6. Create `tests/fixtures/positive/<category>/YourRule.kt` (code that triggers the rule).
7. Create `tests/fixtures/negative/<category>/YourRule.kt` (code that does not trigger).
8. Optionally add auto-fix: implement `IsFixable()`, add `FixLevel()`, populate `f.Fix` in `CheckNode()`.

The CI tests `TestGeneratedFilesUpToDate` and `TestRegistryFileUpToDate`
fail if step 4 is skipped.

Fixes must declare a safety level: `FixCosmetic`, `FixIdiomatic`, or `FixSemantic`.
Auto-fixes must produce ktfmt-compatible output.

## Project Structure

```
cmd/krit/          CLI entry point
cmd/krit-lsp/      LSP server
cmd/krit-mcp/      MCP server for AI agents
internal/rules/    480 rule implementations
internal/scanner/  Tree-sitter parsing, queries, helpers
internal/lsp/      LSP protocol + server
internal/mcp/      MCP protocol + tools + prompts
internal/fixer/    Auto-fix engine (text + binary)
internal/android/  Android project analysis
internal/typeinfer/ Type inference + parallel indexing
internal/oracle/   Kotlin Analysis API daemon
internal/module/   Gradle module discovery
tests/fixtures/    607 positive + negative .kt fixtures
playground/        Sample Kotlin projects for integration tests
editors/           VS Code, Neovim, IntelliJ configs
docs/              MkDocs documentation site (22 pages)
scripts/           Build, test, install, CI scripts
```

## PR Conventions

- Keep PRs focused on a single change.
- Include positive and negative test fixtures for new rules.
- Run `make ci` before submitting (build + vet + test + integration + regression).
- Use clear, descriptive commit messages.
- New rules must use `DispatchBase` (preferred) or `LineBase`.
