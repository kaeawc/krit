# Krit

Go binary for Kotlin static analysis. Parses Kotlin (and Java) with tree-sitter, runs 481 rules via single-pass AST dispatch, outputs JSON/SARIF/Checkstyle. Includes LSP server (11 capabilities), MCP server (8 tools + 3 prompts), and binary autofix.

## Key Rules
- Go only
- After implementation changes, run `go build -o krit ./cmd/krit/ && go vet ./...`
- Unit tests should be fast. Run with `go test ./... -count=1`
- Use tree-sitter AST nodes for structural analysis, regex for line-based pattern matching
- New rules implement `DispatchRule` (node-targeted) or `LineRule` (line scanning), never legacy `Check()`
- All rules must have positive and negative test fixtures in `tests/fixtures/`
- Auto-fixes must produce ktfmt-compatible output
- Fixes must declare a safety level: `FixCosmetic`, `FixIdiomatic`, or `FixSemantic`

## Rule Implementation Guardrails

Recent rule fixes repeatedly came from the same evidence bugs. Before adding, porting, or broadening a rule:

- Prefer tree-sitter flat AST, identifiers, navigation chains, imports, and source index facts over `strings.Contains`, raw prefixes, or broad regexes.
- If a line/text scanner is unavoidable, make it lexical-state aware: skip `//`, `/* ... */`, KDoc, trailing comments, regular strings with escapes, raw triple-quoted strings, and Gradle string literals.
- Require receiver/owner proof for common method names and local lookalikes: `System.out/err`, Android `Context`/`Activity`/`Service`, lifecycle methods, database APIs, logging APIs, and ignored-return fallbacks.
- Stop body walks at real scope boundaries such as nested functions, lambdas, anonymous functions, classes/objects, and local declarations that can shadow names.
- Walk all relevant operands, siblings, and ancestors; do not inspect only the first child or stop at the first unrelated call expression unless that is a real semantic boundary.
- Verify actual parser shape for important constructs (`x!!`, Elvis, safe calls, infix expressions, raw strings) with focused parser/helper tests instead of assuming node names.
- For Java-capable rules, add Java positives and Java local-lookalike negatives. Confirm Java parse, dispatch, suppression, source-index, module/dead-code, and output paths all participate.
- Keep config schema validation and runtime matching in sync, especially for regex options and implicit anchoring.

Bug fixes should include the regression test that would have failed before the fix: lexical negatives, nested-scope negatives, local-lookalike negatives, Java parity cases, or helper unit tests as appropriate.

## Project Structure

- `cmd/krit/` - CLI entry point
- `cmd/krit-lsp/` - LSP server (11 capabilities: diagnostics, code actions, formatting, hover, symbols, definition, references, rename, completion, incremental, config)
- `cmd/krit-mcp/` - MCP server (8 tools, 3 prompts, 2 resources for AI agent integration)
- `internal/rules/` - All 481 rule implementations (dispatch, line, manifest, resource, gradle, icon, source)
- `internal/scanner/` - Tree-sitter parsing, cross-file index, bloom filter, baselines, suppression, compiled queries
- `internal/lsp/` - LSP protocol and server implementation
- `internal/mcp/` - MCP protocol, tools, resources, prompts
- `internal/output/` - JSON, plain text, SARIF, Checkstyle formatters
- `internal/fixer/` - Auto-fix application engine (text + binary)
- `internal/android/` - Android project detection, manifest/resource/icon parsing, XML AST
- `internal/typeinfer/` - Type inference with parallel indexing
- `internal/oracle/` - Kotlin Analysis API daemon (AppCDS, CRaC, persistent PID)
- `internal/module/` - Gradle module discovery and dependency graph
- `internal/perf/` - Performance timing (--perf)
- `internal/config/` - YAML config loading
- `internal/cache/` - Incremental analysis cache
- `internal/schema/` - JSON Schema generation from rule registry
- `internal/tsxml/` - Tree-sitter XML language binding
- `tests/fixtures/` - 1,044 test fixtures (464 positive, 466 negative, 114 fixable)
- `playground/` - Sample Kotlin projects for integration testing
- `editors/` - VS Code, Neovim, IntelliJ editor configs
- `krit-gradle-plugin/` - Gradle plugin (check/format/baseline)
- `homebrew/` / `scoop/` / `winget/` - Package manager manifests

## Build & Validate

```bash
go build -o krit ./cmd/krit/   # Build binary
go vet ./...                    # Lint Go code
go test ./... -count=1          # Run all tests
go test ./internal/rules/ -run TestPositiveFixtures -v  # Fixture tests
go test ./internal/scanner/ -v  # Index/bloom filter tests
```

## Architecture

The dispatcher walks the AST once per file:
1. `FlatDispatchRule` receives nodes by type (378 rules)
2. `LineRule` scans file.Lines in a single pass (86 rules)
3. `AggregateRule` collects nodes during walk, finalizes after (0 currently)
4. Legacy rules walk the tree themselves (22 remaining). The v1 Rule interface still requires `Check(file)`; every rule inherits a no-op via one of five base types (`FlatDispatchBase`, `LineBase`, `ManifestBase`, `ResourceBase`, `GradleBase`), so no per-rule Check stubs are needed.

Cross-file dead code detection indexes Kotlin declarations, then cross-references against Kotlin + Java + XML. Bloom filter provides O(1) reference lookups.

## Adding a Rule

1. Create the rule struct in the appropriate `internal/rules/*.go` file
2. Implement `DispatchRule` (preferred) or `LineRule`
3. Register in `init()` via `v2.Register(&YourRule{...})`
4. Declare default-activity and config options via the checked-in `Meta()` descriptor. Keep `internal/rules/registry_all.go`, defaults, and metadata in sync.
5. Create `tests/fixtures/positive/<category>/YourRule.kt` (code that triggers)
6. Create `tests/fixtures/negative/<category>/YourRule.kt` (code that doesn't trigger)
7. Optionally add auto-fix: implement `IsFixable()`, add `FixLevel()`, populate `f.Fix` in `CheckNode()`

## Configuration

Loads `krit.yml` / `.krit.yml` from project root. Format matches detekt's YAML for migration compatibility. The `--config FILE` flag overrides auto-detection.

## Suppression

`@Suppress("RuleName")` on any declaration suppresses that rule for the declaration's scope. Built into the dispatcher's AST walk (zero extra cost). Also supports `@Suppress("all")`, `@SuppressWarnings`, and `detekt:` prefixes.
