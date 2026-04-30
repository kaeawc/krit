# Krit

Go-first static analyzer for Kotlin, Java, and Android projects. Krit parses
source with tree-sitter, runs rules through the checked-in v2 registry and
single-pass AST dispatcher, and emits JSON, SARIF, Checkstyle, LSP, MCP, and
autofix output.

## Key Rules

- Keep analyzer and rule work in Go. Edit Kotlin/Gradle only for
  `krit-gradle-plugin/` or `tools/krit-types/`.
- After implementation changes, run `go build -o krit ./cmd/krit/ && go vet ./...`.
- Run `go test ./... -count=1` for full validation; use focused package tests while iterating.
- Use tree-sitter AST/flat nodes for structural analysis and regex only for
  line-oriented checks.
- New rules use the v2 pipeline: implement the local rule struct with the
  existing bases, expose dispatch metadata through `v2.Register`, and declare
  the capabilities the dispatcher must provide.
- Rules that need project context must declare the matching capability:
  `NeedsCrossFile`, `NeedsModuleIndex`, `NeedsParsedFiles`, `NeedsManifest`,
  `NeedsResources`, `NeedsGradle`, `NeedsResolver`, `NeedsOracle`, and related
  type-information hints.
- Add positive and negative fixtures under `tests/fixtures/`; fixable rules
  also need fixable fixtures.
- Auto-fixes must produce ktfmt-compatible output and declare `FixCosmetic`,
  `FixIdiomatic`, or `FixSemantic`.

## Rule Implementation Guardrails

Recent rule fixes repeatedly came from the same evidence bugs. Before adding,
porting, or broadening a rule:

- Prefer tree-sitter flat AST, identifiers, navigation chains, imports, and
  source index facts over `strings.Contains`, raw prefixes, or broad regexes.
- If a line/text scanner is unavoidable, make it lexical-state aware: skip
  comments, KDoc, escaped strings, raw strings, and Gradle string literals.
- Require receiver/owner proof for common method names and local lookalikes:
  `System.out/err`, Android `Context`/`Activity`/`Service`, lifecycle methods,
  database APIs, logging APIs, and ignored-return fallbacks.
- Stop body walks at real scope boundaries such as nested functions, lambdas,
  anonymous functions, classes/objects, and local declarations that can shadow
  names.
- Walk all relevant operands, siblings, and ancestors; do not inspect only the
  first child or stop at the first unrelated call expression unless that is a
  real semantic boundary.
- Verify parser shape for important constructs (`x!!`, Elvis, safe calls,
  infix expressions, raw strings) with focused parser/helper tests.
- For Java-capable rules, add Java positives and Java local-lookalike negatives.
- Keep config schema validation and runtime matching in sync, especially for
  regex options and implicit anchoring.

Bug fixes should include the regression test that would have failed before the
fix: lexical negatives, nested-scope negatives, local-lookalike negatives, Java
parity cases, or helper unit tests as appropriate.

## Project Structure

- `cmd/krit/` - CLI entry point
- `cmd/krit-lsp/` - LSP server
- `cmd/krit-mcp/` - MCP server
- `internal/rules/` - rule implementations, v2 registry metadata, dispatcher,
  config, and rule tests
- `internal/scanner/` - tree-sitter parsing, flat AST helpers, suppression,
  cross-file indexes, baselines, and bloom-filter caches
- `internal/pipeline/` - project analysis orchestration
- `internal/android/` - Android manifests, resources, icons, Gradle/XML analysis
- `internal/typeinfer/` - source-level Kotlin/Java type inference
- `internal/oracle/` - Kotlin Analysis API daemon
- `internal/module/` - Gradle module discovery and dependency graph
- `internal/lsp/`, `internal/mcp/`, `internal/output/`, `internal/fixer/` -
  protocol servers, formatters, and fix application
- `internal/config/`, `internal/cache/`, `internal/schema/`, `internal/perf/` -
  config, incremental cache, schema generation, and timing
- `tests/fixtures/` - positive, negative, and fixable rule fixtures
- `krit-gradle-plugin/`, `playground/`, `docs/`, `editors/`, `homebrew/`,
  `scoop/`, `winget/` - integrations, samples, docs, and package manifests

## Build & Validate

```bash
go build -o krit ./cmd/krit/   # Build binary
go vet ./...                    # Lint Go code
go test ./... -count=1          # Run all tests
go test ./internal/rules/ -run TestPositiveFixtures -v
go test ./internal/scanner/ -v
```

## Architecture

The v2 registry describes each rule's ID, category, severity, dispatch nodes,
languages, capabilities, fix level, confidence, and executable callback. The
dispatcher classifies registered rules once, walks each file's flat AST once,
routes matching nodes to node-targeted rules, runs line rules over `file.Lines`,
and lets project-scope phases run after the required indexes are built.

Cross-file dead-code detection indexes Kotlin declarations and Kotlin, Java,
and XML references. Bloom filters provide fast negative reference checks and
are cached per shard for warm runs.

## Adding a Rule

1. Create the rule struct in the appropriate `internal/rules/*.go` file.
2. Implement the rule's local `check` method using `*v2.Context`.
3. Register it with `v2.Register(&v2.Rule{...})` in the matching registry file.
4. Declare `NodeTypes`, `Languages`, `Needs`, Android dependency bits, type
   hints, `Fix`, `Confidence`, and `Implementation` as needed.
5. Add or update the `Meta()` descriptor and config/default metadata.
6. Create positive and negative fixtures under `tests/fixtures/`.
7. For autofix rules, add fixable fixtures and set the fix safety level.

## Configuration

Krit loads `krit.yml` / `.krit.yml` from the project root, with
`--config FILE` as an override. The schema and rule descriptors come from
checked-in Go source.

## Suppression

`@Suppress("RuleName")` on any declaration suppresses that rule for the
declaration's scope. Suppression is built into the dispatcher and also supports
`@Suppress("all")`, `@SuppressWarnings`, and `detekt:` prefixes.
