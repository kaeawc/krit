# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Krit

Go-first static analyzer for Kotlin, Java, and Android projects. Krit parses
source with tree-sitter, runs rules through the checked-in v2 registry and
single-pass AST dispatcher, and emits JSON, SARIF, Checkstyle, LSP, MCP, and
autofix output. Source-level inference handles most type-aware checks; rules
can opt into JVM-backed Kotlin Analysis API/FIR helper facts (`tools/krit-types/`,
`tools/krit-fir/`, `tools/krit-java-facts/`) when source analysis is not enough.

## Key Rules

- Keep analyzer and rule work in Go. Edit Kotlin/Gradle only for
  `krit-gradle-plugin/` or `tools/krit-*/`.
- After implementation changes, run all four:
  `go build -o krit ./cmd/krit/ && go vet ./... && golangci-lint run ./... && go test ./... -count=1`.
- **`golangci-lint run ./...` is required** — `go vet` does not catch
  gofmt drift, unused functions/imports, or many other lint classes that
  CI enforces. Skipping it just causes a CI round-trip. It's especially
  easy to forget after a refactor or deletion that leaves orphan helpers;
  make it part of every validation pass.
- Run `go test ./... -count=1` for full test validation; use focused
  package tests while iterating.
- Use tree-sitter AST/flat nodes for structural analysis and regex only for
  line-oriented checks.
- New rules use the v2 pipeline: implement the local rule struct with the
  existing bases (`FlatDispatchBase`, `LineBase`, `ManifestBase`, `ResourceBase`,
  `GradleBase`), expose dispatch metadata in the checked-in registry, and
  declare the capabilities the dispatcher must provide. Never add standalone
  `Check(file)` tree walks.
- Rules that need project context must declare the matching capability:
  `NeedsCrossFile`, `NeedsModuleIndex`, `NeedsParsedFiles`, `NeedsManifest`,
  `NeedsResources`, `NeedsGradle`, `NeedsResolver`, `NeedsOracle`, `NeedsTypeInfo`,
  and related type-information hints. `make lint-rules` enforces the
  `NeedsResolver` / `NeedsOracle` declaration gate.
- Before adding a rule, confirm it belongs in the built-in registry: see
  `docs/rule-scope.md` for the qualification checklist (opt-in / experimental /
  project-local rules go elsewhere).
- Add positive and negative fixtures under `tests/fixtures/<category>/`;
  fixable rules also need fixable fixtures.
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

## Fixture Conventions

Files under `tests/fixtures/negative/**` and `tests/fixtures/fixable/**`
intentionally contain suspicious, non-idiomatic, unsafe, invalid, or fixable
code so Krit rules and autofixes can be tested. Treat them as test input, not
production code — do not flag style or correctness issues there during review.
Only call out fixture-mechanics problems: wrong category path, malformed
`.expected` pair, missing companion fixture, or changes that prevent the test
harness from parsing the fixture.

## Project Structure

- `cmd/krit/` - CLI entry point
- `cmd/krit-lsp/` - LSP server
- `cmd/krit-mcp/` - MCP server
- `internal/rules/` - rule implementations, v2 registry metadata
  (`registry_*.go`), dispatcher, config, and rule tests
- `internal/ruleslinter/` - capability-drift / ad-hoc-cache gate enforced by
  `make lint-rules`
- `internal/scanner/` - tree-sitter parsing, flat AST helpers, suppression,
  cross-file indexes, baselines, and bloom-filter caches
- `internal/pipeline/`, `internal/deadcode/`, `internal/module/` - project
  analysis orchestration, dead-code passes, Gradle module discovery and
  dependency graph
- `internal/android/`, `internal/tsxml/` - Android manifests, resources, icons,
  Gradle/XML analysis
- `internal/typeinfer/`, `internal/oracle/`, `internal/firchecks/` -
  source-level Kotlin/Java type inference, Kotlin Analysis API daemon, FIR
  helper integration
- `internal/librarymodel/`, `internal/javafacts/` - library / Gradle catalog
  profile facts and Java source facts
- `internal/lsp/`, `internal/mcp/`, `internal/output/`, `internal/fixer/` -
  protocol servers, formatters, and fix application
- `internal/config/`, `internal/cache/`, `internal/schema/`, `internal/perf/` -
  config, incremental cache, schema generation, and timing
- `tests/fixtures/` - positive, negative, and fixable rule fixtures, organized
  by category (`a11y`, `android`, `coroutines`, `performance`, ...)
- `tools/krit-types/`, `tools/krit-fir/`, `tools/krit-java-facts/` - JVM-backed
  helpers for compiler-grade facts
- `krit-gradle-plugin/`, `playground/`, `docs/`, `editors/`, `homebrew/`,
  `scoop/`, `winget/` - integrations, samples, docs, and package manifests

## Build & Validate

The Makefile is the canonical entry point — `make ci` mirrors what CI runs.

```bash
make build          # Builds krit, krit-lsp, krit-mcp with version ldflags
make test           # go test ./... -count=1
make vet            # go vet ./...
make lint-rules     # Enforce NeedsResolver/NeedsOracle capability declarations
make integration    # Full integration suite (build + playground + CLI/LSP/MCP)
make regression     # Verify playground regression expectations
make ci             # build + vet + test + integration + regression
make bench          # Performance benchmarks
make schema         # Regenerate schemas/krit-config.schema.json
```

Manual equivalents (use during iteration):

```bash
go build -o krit ./cmd/krit/
go vet ./...
golangci-lint run ./...
go test ./... -count=1
```

Focused package tests:

```bash
go test ./internal/rules/ -run TestPositiveFixtures -v
go test ./internal/scanner/ -v
go test ./internal/pipeline/ -v
```

**Always run `make integration` (or `make ci`) before pushing.** It catches
issues — missing binary outputs, LSP/MCP regressions, integration-only test
failures — that `go test ./...` alone does not exercise.

## Architecture

The v2 registry describes each rule's ID, category, severity, dispatch nodes,
languages, capabilities, fix level, confidence, and executable callback. The
dispatcher classifies registered rules once, walks each file's flat AST once,
routes matching nodes to node-targeted rules, runs line rules over
`file.Lines`, and lets project-scope phases (cross-file, module-aware,
parsed-file, Android, Gradle, aggregate) run after the required indexes are
built.

Cross-file dead-code detection indexes Kotlin declarations and Kotlin, Java,
and XML references. Bloom filters provide fast negative reference checks and
are cached per shard for warm runs.

Suppression is built into the dispatcher: `@Suppress("RuleName")` on any
declaration suppresses that rule for the declaration's scope, and the
dispatcher also honors `@Suppress("all")`, `@SuppressWarnings`, and `detekt:`
prefixes.

## Adding a Rule

1. Create the rule struct in the appropriate `internal/rules/*.go` file and
   embed the matching base (`FlatDispatchBase`, `LineBase`, `ManifestBase`,
   `ResourceBase`, `GradleBase`).
2. Implement the rule's local `check` method using the registry callback context.
3. Register it in the matching `internal/rules/registry_*.go` file.
4. Declare `NodeTypes`, `Languages`, `Needs`, Android dependency bits, type
   hints, `Fix`, `Confidence`, and `Implementation` as needed.
5. Add or update the `Meta()` descriptor and config/default metadata; set
   `DefaultActive` to control whether the rule runs by default.
6. Create positive and negative fixtures under `tests/fixtures/`.
7. For autofix rules, add fixable fixtures and set the fix safety level.

## Configuration

Krit loads `krit.yml` / `.krit.yml` from the project root, with
`--config FILE` as an override. `krit --init` writes a starter config. The
schema and rule descriptors come from checked-in Go `Meta()` implementations;
`make schema` regenerates `schemas/krit-config.schema.json`.

## Git

Agent-created branches use the `work/` prefix (see `AGENTS.md`).
