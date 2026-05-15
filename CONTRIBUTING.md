# Contributing to Krit

## Quick Start

```bash
make build          # Build krit, krit-lsp, krit-mcp
make test           # Run all tests
make ci             # Full CI: build + vet + test + integration + regression
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
make regression     # Verify playground regression expectations
make bench          # Performance benchmarks
make watch          # Re-run tests on file changes (requires fswatch)
```

## Adding a New Rule

Before implementing, confirm the rule belongs in the built-in registry: see
[`docs/rule-scope.md`](docs/rule-scope.md) for the qualification checklist
and the rules that should live as opt-in / experimental / project-local
instead.

1. Create the rule struct in the appropriate `internal/rules/*.go` file.
2. Embed the appropriate base (`FlatDispatchBase`, `LineBase`, `ManifestBase`,
   `ResourceBase`, `GradleBase`, or a resource-specific base).
3. Register the rule in the relevant `internal/rules/registry_*.go` file with the appropriate
   v2 metadata (`NodeTypes`, `Needs`, `Languages`, `Fix`, and Android data
   dependencies where relevant).
4. If the rule has config options, add or update its `Meta()` descriptor.
   Keep metadata, defaults, and registry fields in sync with tests.
5. Rule default-activity is declared via `DefaultActive` in the checked-in
   `Meta()` descriptor.
6. Create `tests/fixtures/positive/<category>/YourRule.kt` (code that triggers the rule).
7. Create `tests/fixtures/negative/<category>/YourRule.kt` (code that does not trigger).
8. Optionally add auto-fix: populate `f.Fix` in the rule and set the v2
   registry `Fix` safety level.

Fixes must declare a safety level: `FixCosmetic`, `FixIdiomatic`, or `FixSemantic`.
Auto-fixes must produce ktfmt-compatible output.

## Project Structure

```
cmd/krit/          CLI entry point
cmd/krit-lsp/      LSP server
cmd/krit-mcp/      MCP server for AI agents
internal/rules/    Rule implementations, registry metadata, dispatcher
internal/scanner/  Tree-sitter parsing, queries, helpers
internal/lsp/      LSP protocol + server
internal/mcp/      MCP protocol + tools + prompts
internal/fixer/    Auto-fix engine (text + binary)
internal/android/  Android project analysis
internal/typeinfer/ Source-level type inference + parallel indexing
internal/oracle/   JVM-backed Kotlin Analysis API daemon
internal/module/   Gradle module discovery
tests/fixtures/    Positive, negative, and fixable fixtures
playground/        Sample Kotlin projects for integration tests
editors/           VS Code, Neovim, IntelliJ configs
docs/              MkDocs documentation site
scripts/           Build, test, install, CI scripts
tools/krit-types/  JVM/KAA helper for compiler-backed type facts
tools/krit-fir/    JVM/FIR helper for compiler checks
```

## Rule Output Stability

Each rule carries a `Stability` field (`api.Stability`) that describes
its *output-shape* commitment — separate from `Maturity` (lifecycle).
Consumers of krit JSON, SARIF, baseline files, and CI gates read this
to decide whether the rule is safe to pin against:

- `StabilityFrozen` — message text, finding location, and fix range
  will not change.
- `StabilityStable` — bug-fix changes only.
- `StabilityEvolving` — shape may change between minor versions.

The default for unannotated rules is `StabilityStable` (experimental
rules default to `StabilityEvolving`). Promotions (Evolving → Stable →
Frozen) are always allowed. **Downgrading a rule from `StabilityFrozen`
to `StabilityEvolving` requires a major-version bump** — the field
exists precisely so external baselines and dashboards can rely on it.

`baseline-audit` warns when a baselined finding belongs to a rule whose
effective stability is `StabilityEvolving`.

## PR Conventions

- Keep PRs focused on a single change.
- Include positive and negative test fixtures for new rules.
- Run `make ci` before submitting (build + vet + test + integration + regression).
- Use clear, descriptive commit messages.
- New rules must use the v2 registry and the appropriate local base.
