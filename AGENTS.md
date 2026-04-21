# Krit

Krit is a Go-first static analyzer for Kotlin, Java, and Android projects. It parses with tree-sitter, runs rules through a generated v2 registry and single-pass AST dispatcher, emits JSON/SARIF/Checkstyle, and ships autofix, LSP, MCP, Gradle/editor integrations, Android XML/resource analysis, and a Kotlin type oracle.

## Working Rules
- Keep core analyzer and rule work in Go. Edit Kotlin/Gradle only for `krit-gradle-plugin/` or `tools/krit-types/`.
- Use tree-sitter AST/flat nodes for structural analysis; use regex only for line-oriented checks.
- New rules use the v2 pipeline: implement the local rule struct with the existing bases (`FlatDispatchBase`, `LineBase`, `ManifestBase`, `ResourceBase`, `GradleBase`), expose node types or line/project capabilities through generated registry metadata, and never add legacy `Check(file)` walks.
- Rules that need project context must declare the right capability (`NeedsCrossFile`, `NeedsModuleIndex`, `NeedsParsedFiles`, `NeedsManifest`, `NeedsResources`, `NeedsGradle`, `NeedsTypeInfo`, etc.) so the dispatcher/pipeline can provide indexes, Android data, or type info.
- New rules require positive and negative fixtures under `tests/fixtures/`; fixable rules also need fixable fixtures.
- Auto-fixes must be ktfmt-compatible and declare a safety level: `FixCosmetic`, `FixIdiomatic`, or `FixSemantic`.

## Build & Validate

```bash
go build -o krit ./cmd/krit/   # Build CLI
go vet ./...                    # Lint Go code
go test ./... -count=1          # Full Go test suite
```

After implementation changes, run `go build -o krit ./cmd/krit/ && go vet ./...`. Run `go test ./... -count=1` for test validation; use focused package tests while iterating.

For rule metadata or registry changes, run:

```bash
python3 tools/rule_inventory.py
go generate ./internal/rules/...
```

## Git

Use branch prefix `work/` for agent-created branches.

## Project Map
- `cmd/krit/`, `cmd/krit-lsp/`, `cmd/krit-mcp/` - CLI, LSP server, MCP server.
- `internal/rules/` - rule implementations, bases, generated metadata, v2 dispatcher, registry tests.
- `internal/scanner/` - tree-sitter parsing, flat AST helpers, suppression, cross-file indexes, bloom-filter caches.
- `internal/pipeline/`, `internal/deadcode/`, `internal/module/` - project analysis orchestration, dead-code passes, Gradle module graph.
- `internal/android/`, `internal/tsxml/` - Android manifests, resources, icons, Gradle/XML analysis.
- `internal/typeinfer/`, `internal/oracle/` - source-level inference and Kotlin Analysis API daemon.
- `internal/lsp/`, `internal/mcp/`, `internal/output/`, `internal/fixer/` - protocol servers, formatters, fix engine.
- `internal/config/`, `internal/cache/`, `internal/schema/`, `internal/perf/` - config, incremental cache, schema generation, timing.
- `tests/fixtures/` - positive, negative, and fixable rule fixtures.
- `playground/`, `docs/`, `editors/`, `homebrew/`, `scoop/`, `winget/` - integration samples, docs, editor/package manager assets.

## Architecture Notes

The dispatcher classifies generated `v2.Rule` entries by `NodeTypes`, `Languages`, and `Needs` capabilities. Node-targeted rules receive matching flat AST nodes during one file walk; line rules scan `file.Lines`; cross-file, module-aware, parsed-file, Android, Gradle, and aggregate rules run in pipeline phases after the needed indexes are built.

Cross-file analysis indexes Kotlin declarations plus Kotlin, Java, and XML references. Bloom filters provide fast negative reference checks and are cached per shard for warm runs.

Suppression is built into the dispatcher via `@Suppress("RuleName")`, `@Suppress("all")`, `@SuppressWarnings`, and `detekt:` prefixes.

Config loads `krit.yml` / `.krit.yml` from the project root, with `--config FILE` as an override. The schema and rule descriptors come from generated `Meta()` implementations; hand-write `meta_YourRule.go` and add the struct to `excludedStructs` only for config shapes the generator cannot express.
