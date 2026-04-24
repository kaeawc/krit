# Rule Registry Refactor: V2 Only, No Codegen, No Python

Date: 2026-04-24

## Implementation Status

Completed in this worktree:

- Proved the v2 registry is complete enough to be the only production runtime registry with invariant tests for metadata, aliases, default-active parity, and dispatcher classification.
- Moved fix-level lookup fully onto `v2.Rule.Fix`; removed the `OriginalV1` fix-level fallback and the dead per-rule `FixLevel()` methods.
- Converted the consolidated rule registration file from generated output to checked-in Go source at `internal/rules/registry_all.go`.
- Deleted rule registry/codegen packages and freshness tests: `internal/codegen/`, `generate_verify_test.go`, and `registry_verify_test.go`.
- Removed Python files and Python invocations from active build/test/dev scripts. JSON-counting script needs are now covered by `internal/devtools/jsonstat`.
- Removed stale descriptor source hashes and generated-file headers from rule metadata descriptors.
- Removed the v2 dispatcher's dormant legacy fallback bucket; empty explicit `NodeTypes` rules are no longer run as once-per-file legacy rules.
- Verified with `go build -o krit ./cmd/krit/`, `go build -o krit-lsp ./cmd/krit-lsp/`, `go build -o krit-mcp ./cmd/krit-mcp/`, `go vet ./...`, and `go test ./... -count=1`.

Remaining cleanup that is intentionally separate from codegen/Python deletion:

- Rename the `zz_meta_*_gen.go` metadata files to non-generated filenames if we want filenames to reflect the new ownership model.
- Rename or replace `v2.Rule.OriginalV1`; it is now config/state plumbing, not a fix-level or legacy-dispatch fallback.
- Split `internal/rules/registry_all.go` by rule source/category for easier review and rule-author ergonomics.

## Goal

Make the v2 rule registry the only rule registry in Krit, then delete the older/generated registry infrastructure and all Python/codegen dependencies.

Target end state:

- `internal/rules/v2` owns the only runtime rule model, registry, metadata, default-active state, config options, fix metadata, language routing, and dispatch capabilities.
- Rule registration is normal Go source, not generated source.
- There are no `zz_*_gen.go` rule registry files, no `internal/codegen/`, no `go generate` requirement for rules, no `build/rule_inventory.json`, and no `python3` usage in build/test/dev scripts.
- Rule authors add or change a rule by editing the rule's Go source and fixtures/tests only.

## Current State

The CLI, pipeline, LSP/MCP-facing paths already use v2 rules:

- CLI imports `internal/rules/v2` and filters `rules.ActiveRulesV2(...)`.
- `pipeline.BuildDispatcher` builds `rules.NewDispatcherV2(...)`.
- MCP/LSP rule lookup reads `v2.Registry`.
- Cross-file, module-aware, manifest, resource, Gradle, resolver, oracle, and parsed-file phases all route through `*v2.Rule.Needs`.

The remaining registry complexity is mostly source ownership and metadata/config:

- `internal/rules/v2/rule.go` has the global `v2.Registry` and `v2.Register`.
- `internal/rules/registry_all.go` has 629 `v2.Register(...)` calls in one generated file.
- `internal/rules/zz_meta*_gen.go` contains generated `Meta() registry.RuleDescriptor` methods and `zz_meta_index_gen.go` contains `AllMetaProviders()` / `metaByName()`.
- `internal/rules/registry/` is a separate descriptor/config runtime package used by generated metadata.
- `internal/rules/config.go`, `defaults.go`, and `meta_lookup.go` bridge from v2 rules back to `OriginalV1` and generated `Meta()` descriptors.
- `v2.Rule.OriginalV1` remains the anchor for config mutation, some tests, precision fallback, fix-level fallback, and module tuning.
- `V2Dispatcher` still has a `legacyRules` bucket and `LegacyRuleMs` metrics, even though production registration should not need the old once-per-file fallback.

Generated/codegen/Python surfaces found:

- Codegen packages: `internal/codegen/cmd/krit-gen`, `internal/codegen/cmd/
- Generated registry files: `internal/rules/registry_all.go` plus generated `zz_meta*_gen.go` files.
- Freshness tests: `internal/rules/generate_verify_test.go`, `internal/rules/registry_verify_test.go`.
- Python files: `tools/rule_inventory.py`, `tools/oracle_fingerprint_check.py`, `scripts/detect_stub_rules.py`, `scripts/migrate_to_v2.py`, `scripts/simplify_v2_migrations.py`, `scripts/final_v2_migration.py`, `scripts/github/deploy_pages.py`.
- Python invocations also exist in `Makefile`, `scripts/integration-test.sh`, benchmark/regression scripts, badge generation, workflow validation, and docs serving.

Baseline check run during this audit:

```bash
go test ./internal/rules -run 'Test(ConfigParity|ConfigParity_AliasRegistrations|V2Dispatcher|GeneratedFilesUpToDate|RegistryFileUpToDate)' -count=1
```

Result: `ok github.com/kaeawc/krit/internal/rules 1.230s`

## Main Risks

1. **Config behavior drift.** Generated `Meta()` descriptors currently preserve defaults, aliases, option transforms, and custom apply hooks. Moving this into v2 must preserve the 100% config parity currently guarded by `TestConfigParity`.
2. **Alias registrations.** Four Gradle aliases intentionally register under names different from their primary `Meta().ID`: `GradleCompatible`, `GradleDependency`, `GradleDynamicVersion`, `StringShouldBeInt`.
3. **Stateful rule structs.** Many v2 checks still close over concrete rule structs for thresholds, caches, regexes, and helper methods. The v2 registry can own metadata without forcing every rule implementation to become stateless in the same iteration.
4. **Generated registration size.** `registry_all.go` is about 18k lines. Converting it mechanically is high-volume and should be split by source/category to keep review possible.
5. **Python removal is wider than registry.** Removing "any python completely" includes non-registry tools and shell one-liners, not just `tools/rule_inventory.py`.

## Proposed Architecture

### V2 Rule Owns Metadata

Extend `v2.Rule` so generated `registry.RuleDescriptor` is no longer needed:

```go
type Rule struct {
    ID            string
    Category      string
    Description   string
    Sev           Severity
    DefaultActive bool
    Options       []ConfigOption
    CustomApply   func(ConfigTarget, ConfigSource)

    NodeTypes []string
    Needs     Capabilities
    Languages []scanner.Language
    Fix       FixLevel
    Confidence float64

    Check func(*Context)

    // Temporary migration-only field, renamed away from OriginalV1.
    ConfigTarget any
}
```

Move or duplicate the descriptor runtime into `internal/rules/v2`:

- `ConfigSource`
- `ConfigOption`
- `OptionType`
- `ApplyConfig`
- default-active helpers
- option alias resolution
- custom apply hook

During migration, keep `ConfigTarget any` as the concrete rule struct that options mutate. Once every test and helper stops relying on `OriginalV1`, remove `OriginalV1` entirely.

### Hand-Owned Registration

Replace `registry_all.go` with normal Go registration files, preferably split by existing rule source/category:

- `register_accessibility.go`
- `register_android_gradle.go`
- `register_complexity.go`
- etc.

Each file should contain plain Go source with registration functions. No `Code generated` header. No extractor.

Shape:

```go
func registerComplexityRules(reg *v2.Registry) {
    r := &LongMethodRule{
        AllowedLines: 60,
    }
    reg.Register(&v2.Rule{
        ID: "LongMethod",
        Category: "complexity",
        Description: "Detects functions that exceed the configured line threshold.",
        Sev: v2.SeverityWarning,
        DefaultActive: true,
        NodeTypes: []string{"function_declaration"},
        Options: []v2.ConfigOption{
            v2.IntOption("allowedLines", 60, func(value int) {
                r.AllowedLines = value
            }, v2.WithAliases("threshold")),
        },
        Check: r.check,
        ConfigTarget: r,
    })
}
```

The first pass can preserve existing rule structs and methods. A later cleanup can remove `BaseRule` from structs where it only exists to feed generated metadata.

### Lazy Registry Construction

Prefer replacing package `init()` registration with an explicit/lazy registry builder:

```go
func AllRules() []*v2.Rule
func DefaultActiveRules() []*v2.Rule
func RuleByID(id string) (*v2.Rule, bool)
```

`v2.Registry` can remain as a compatibility variable for one iteration, but the final state should avoid mutation through init-order side effects.

## Milestone Draft

Milestone: **V2-only rule registry, no codegen/Python**

Success criteria:

- `go build -o krit ./cmd/krit/ && go vet ./...` passes.
- `go test ./... -count=1` passes.
- `rg 'internal/codegen|krit-gen|
- No production code references `OriginalV1`, `MetaProvider`, `RuleDescriptor`, `AllMetaProviders`, `metaByName`, `ApplyConfigViaRegistry`, or rule-dispatch `legacyRules`.
- Rule count, default-active count, fixable count, and rule IDs are unchanged except for explicitly approved cleanup.
- Config behavior is covered by v2-native tests, not legacy parity tests.

## Issues / Iteration Plan

### Issue 1: Add V2 Registry Invariant Tests

Purpose: prove the current v2 registry is complete before deleting compatibility layers.

Tasks:

- Add a v2 registry audit test that records and asserts:
  - no duplicate IDs except the known alias pairs, or explicitly documents duplicates if they are valid
  - every registered rule has ID/category/description/severity
  - every registered rule has `Check` or a valid project/aggregate phase
  - no registered production rule uses the `legacyRules` classifier
  - every default-inactive rule is represented by v2 metadata
  - every fixable rule has a v2 `Fix` safety level
- Add a test that computes dispatcher stats for all rules and fails if `legacyCount > 0`.
- Add a test that compares current rule ID/default-active/fixable snapshots to an audited checked-in Go snapshot, replacing generator freshness tests as the drift guard.

Validation:

```bash
go test ./internal/rules -run 'TestV2.*Registry|TestV2Dispatcher' -count=1
```

### Issue 2: Move Rule Descriptor Runtime Into V2

Purpose: make `v2.Rule` the source of truth for metadata/config.

Tasks:

- Add `DefaultActive`, `Options`, and `CustomApply` to `v2.Rule`.
- Move `OptionType`, `ConfigOption`, `ConfigSource`, and config apply logic from `internal/rules/registry` into `internal/rules/v2`.
- Add temporary conversion helpers so existing generated `Meta()` descriptors can populate the new v2 fields while registration is still generated.
- Update `rules.ApplyConfig` to apply directly from `*v2.Rule`, not `MetaForV2Rule`.
- Keep alias active override semantics intact.
- Update schema collection and rule listing to use v2 metadata.

Validation:

```bash
go test ./internal/rules ./internal/schema ./cmd/krit -run 'TestConfig|TestSchema|TestList' -count=1
go test ./... -count=1
```

Exit criteria:

- Production `ApplyConfig` no longer calls `MetaForV2Rule`, `metaByName`, or `registry.ApplyConfig`.
- Generated `Meta()` files may still exist, but only as migration input to populate v2 fields.

### Issue 3: Remove `OriginalV1` From Runtime Paths

Purpose: stop treating v2 rules as wrappers around older rule structs.

Tasks:

- Rename migration state from `OriginalV1` to a neutral `ConfigTarget` or `State` field.
- Update config mutation, fix-level fallback, precision, module tuning, and tests to use v2 fields or explicit v2 extension hooks.
- Add explicit fields/hooks for remaining cases:
  - `ModuleNeeds` / module tuning
  - fix level
  - precision
  - config target
- Update tests that assert concrete types through `OriginalV1` to use rule lookup helpers or rule state accessors.
- Remove `OriginalV1` from `v2.Rule`.

Validation:

```bash
rg 'OriginalV1|V1|legacy' internal/rules internal/pipeline cmd/krit internal/lsp internal/mcp
go test ./internal/rules ./internal/pipeline ./cmd/krit -count=1
```

Exit criteria:

- No production dependency on `OriginalV1`.
- Remaining `legacy` references are cache/file-format history only, not rule runtime.

### Issue 4: Convert Generated Meta to V2-Owned Source

Purpose: delete `krit-gen`, `tools/rule_inventory.py`, and `zz_meta*_gen.go`.

Tasks:

- Move all default-active and option data from generated `Meta()` methods into v2 registrations or adjacent hand-owned metadata files.
- Preserve the four custom config transforms currently hand-written:
  - `ForbiddenImportRule`
  - `LayerDependencyViolationRule`
  - `NewerVersionAvailableRule`
  - `PublicToInternalLeakyAbstractionRule`
- Replace `AllMetaProviders()` and `metaByName()` with v2 registry lookup/index helpers.
- Delete:
  - `internal/rules/zz_meta*_gen.go`
  - `internal/rules/generate_verify_test.go`
  - `internal/codegen/cmd/krit-gen`
  - `tools/rule_inventory.py`
  - `build/rule_inventory.json` references
  - `//go:generate` in `internal/rules/rule.go`

Validation:

```bash
rg 'zz_meta|krit-gen|rule_inventory|AllMetaProviders|metaByName|MetaProvider|RuleDescriptor' internal cmd tools scripts Makefile
go test ./internal/rules ./internal/schema ./cmd/krit -count=1
```

Exit criteria:

- No generated metadata files.
- No metadata codegen.
- New rule workflow requires no generator.

### Issue 5: Replace `registry_all.go` With Hand-Owned Registration

Purpose: remove the generated v2 registration file and extractor.

Tasks:

- Split `registry_all.go` into source-owned registration files by existing rule source or category.
- Preserve registration order where output ordering or tests depend on it.
- Preserve every `NodeTypes`, `Needs`, `Languages`, `Fix`, `Confidence`, `Oracle`, `OracleCallTargets`, `OracleDeclarationNeeds`, `TypeInfo`, and Android dependency declaration.
- Add a registry snapshot test to detect accidental rule loss.
- Delete:
  - `internal/rules/registry_all.go`
  - `internal/rules/registry_verify_test.go`
  - `internal/codegen/cmd/

Validation:

```bash
go test ./internal/rules -run 'TestV2.*Registry|TestV2Dispatcher|TestConfig|TestRule' -count=1
go test ./... -count=1
```

Exit criteria:

- No generated rule registration file.
- No extractor.
- Rule registration is normal Go source.

### Issue 6: Delete Rule Runtime Compatibility Leftovers

Purpose: finish "v2 registry only" after generated surfaces are gone.

Tasks:

- Remove `internal/rules/registry/` if its runtime was fully moved into v2.
- Delete `config_via_registry.go` and legacy parity tests that compare against deleted migration paths.
- Remove `legacyRules` from `V2Dispatcher`, `LegacyRuleMs` from stats, and related perf bucket names.
- Make `v2.Register` reject a rule with empty non-nil `NodeTypes` unless it has a declared project/line/aggregate capability.
- Revisit `BaseRule`; either delete it or shrink it to a non-registry helper if many rules still use `Finding()`.
- Update docs and AGENTS/CONTRIBUTING instructions to remove generator workflow.

Validation:

```bash
rg 'legacyRules|LegacyRuleMs|ApplyConfigViaRegistry|internal/rules/registry|BaseRule|go generate|codegen' internal cmd docs AGENTS.md CONTRIBUTING.md
go test ./... -count=1
```

Exit criteria:

- Only the v2 registry model remains for rules.

### Issue 7: Remove Python Completely

Purpose: make the repo build/test/dev workflow Python-free.

Tasks:

- Delete completed migration scripts:
  - `scripts/migrate_to_v2.py`
  - `scripts/simplify_v2_migrations.py`
  - `scripts/final_v2_migration.py`
  - `scripts/detect_stub_rules.py` if replaced by Go tests
- Rewrite `tools/oracle_fingerprint_check.py` as Go:
  - preferred: add a `krit oracle-fingerprint --check/--update` subcommand, or
  - add a small Go tool under `tools/` if it is not part of CLI UX
- Replace shell `python3 -c` JSON/time snippets with Go helpers or Krit subcommands.
- Replace workflow YAML validation with a Go-based validator using existing `gopkg.in/yaml.v3`.
- Rewrite or replace `scripts/github/deploy_pages.py` if docs serving/deploy remains needed.
- Remove `scripts/github/pyproject.toml`.
- Update `Makefile`, CI, docs, AGENTS.md, CLAUDE.md, and CONTRIBUTING.md.

Validation:

```bash
find . -type f \( -name '*.py' -o -name 'pyproject.toml' \) -not -path './.git/*'
rg 'python3|python |\\.py' . -g '!scratch/**' -g '!roadmap/**'
go build -o krit ./cmd/krit/ && go vet ./...
go test ./... -count=1
```

Exit criteria:

- No Python source files.
- No Python invocations in build/test/dev scripts.

### Issue 8: Final Docs and Release Notes

Purpose: make the new workflow obvious and remove stale migration language.

Tasks:

- Update AGENTS.md working rules:
  - remove `python3 tools/rule_inventory.py`
  - remove `go generate ./internal/rules/...`
  - describe v2 registration/metadata fields
- Update CONTRIBUTING.md new-rule workflow.
- Update `roadmap/clusters/core-infra/codegen-registry.md` as historical/completed-and-removed, or replace it with a concise architecture note.
- Add a migration note for downstream contributors: no generated files, no Python setup.

Validation:

```bash
rg 'rule_inventory|krit-gen|
```

## Recommended Sequence

1. Land invariant tests first. They are the safety net for the rest of the refactor.
2. Make v2 own metadata/config while generated files still exist. This keeps behavior stable and isolates config drift.
3. Remove `OriginalV1` and the descriptor package dependency from runtime paths.
4. Delete generated meta/codegen.
5. Replace generated registration source.
6. Remove dispatcher/runtime compatibility leftovers.
7. Remove all non-registry Python.
8. Update docs.

This order avoids doing the riskiest mechanical deletion before the runtime has a v2-native source of truth.

## Validation Matrix

Minimum validation per major PR:

```bash
go build -o krit ./cmd/krit/
go vet ./...
go test ./internal/rules ./internal/pipeline ./internal/schema ./cmd/krit -count=1
```

Full validation before each milestone close:

```bash
go build -o krit ./cmd/krit/ && go vet ./...
go test ./... -count=1
```

Behavioral validation before deleting generated registration:

- Compare `krit --list-rules --all` before/after for rule IDs, default-active, fixability, precision, severity, and category.
- Run fixture suites and at least one real-world project scan with JSON output diffed after stable sorting.
- Run `--all-rules`, `--no-type-inference`, and Android fixture paths to cover deferred phases.

## Open Decisions

- Whether final rule registration should keep package `init()` or move to explicit lazy `rules.AllRules()`.
- Whether `BaseRule` should be deleted in this milestone or left as a helper after registry ownership moves to v2.
- Whether Python removal must include historical roadmap references or only executable/dev workflow references.
- Whether docs deploy support should be rewritten now or moved out of this repo.
