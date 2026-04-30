# CodegenRegistry

**Cluster:** [core-infra](README.md) · **Status:** ✅ shipped ·
**Severity:** n/a (infra) · **Default:** n/a

## What Shipped

Krit now declares rule metadata once through checked-in Go descriptors and uses
that metadata for defaults, config loading, schema generation, and registry
validation.

The shipped architecture is:

- `internal/rules/v2/metadata.go` owns descriptor runtime types:
  `RuleDescriptor`, `ConfigOption`, `OptionType`, `MetaProvider`,
  `ConfigSource`, and the `ApplyConfig` helpers.
- Rule structs expose `Meta() v2.RuleDescriptor`. Generated descriptors live in
  `internal/rules/zz_meta_*_gen.go`; hand-written descriptors live in
  `internal/rules/meta_*.go`.
- `internal/rules/zz_meta_index_gen.go` provides `AllMetaProviders()` and the
  descriptor index used by defaults, config, schema, and invariant tests.
- `internal/rules/registry_*.go` files register executable v2 rules with
  `v2.Register`.
- The old `internal/rules/registry` compatibility package has been deleted.

Descriptors intentionally remain in the `rules` package because Go methods must
be declared in the same package as their receiver type. Runtime ownership still
belongs to `v2`; the descriptor methods simply attach v2 metadata to local rule
structs.

## Previous Cost

Before this migration, a new rule required coordinated edits across shared
runtime state:

1. A per-rule `init()` registration.
2. A hand-maintained `DefaultInactive` entry for opt-in rules.
3. A branch in the old config switch to load rule-specific options.

That shape made missing config implementation easy to ship: a rule could expose
config docs while never reading the value at runtime. The v2 descriptor path now
keeps schema and config application tied to the same checked-in metadata.

## Authoring Workflow

1. Implement the rule in Go and register it through the relevant
   `internal/rules/registry_*.go` file with `v2.Register`.
2. Add or update its `Meta() v2.RuleDescriptor`:
   - Generated descriptors come from `tools/rule_inventory.py` plus
     `go generate ./internal/rules/...`.
   - Exotic config shapes use a local `meta_*.go` file with `CustomApply`.
3. Add positive and negative fixtures, and fixable fixtures when the rule
   provides autofix.
4. Run `python3 tools/rule_inventory.py && go generate ./internal/rules/...`.
5. Validate with `go build -o krit ./cmd/krit/ && go vet ./...` and
   `go test ./... -count=1`.

## Acceptance Criteria

| Criterion | Status | Notes |
|---|---|---|
| Rule execution is v2-driven | ✅ | Registered rules are `v2.Rule` entries dispatched by the v2 pipeline. |
| Config loading uses descriptors | ✅ | `rules.ApplyConfig` uses `MetaForV2Rule` and `v2.ApplyConfig`. |
| Defaults come from descriptors | ✅ | `DefaultInactive` is lazily initialized from `AllMetaProviders()`. |
| Schema generation uses descriptors | ✅ | `internal/schema` reads `v2.RuleDescriptor` values. |
| Compatibility registry package removed | ✅ | No `internal/rules/registry` package or imports remain. |
| Test-only migration harness removed from production | ✅ | Config parity helpers live only in `config_parity_test.go`. |
| Generated files have freshness coverage | ✅ | CI verifies generated metadata and registry files. |

## Deviations

1. **Descriptors are methods, not struct tags.** The method form lets config
   application use typed closures and supports `CustomApply` for rules whose
   config cannot be expressed as simple scalar or list options.
2. **Descriptors live beside rules.** This is required by Go receiver method
   rules. The types and runtime helpers are still owned by `internal/rules/v2`.
3. **Inventory generation remains two-stage.** `tools/rule_inventory.py`
   produces `build/rule_inventory.json`, and `krit-gen` consumes it. Moving the
   inventory parser into Go remains possible, but it is separate cleanup rather
   than part of the completed v2 migration.

## Links

- Depends on: [`unified-rule-interface.md`](unified-rule-interface.md) ✅
- Related: `internal/rules/v2/metadata.go`, `internal/rules/config.go`,
  `internal/rules/defaults.go`, `internal/rules/zz_meta_index_gen.go`,
  `internal/codegen/cmd/krit-gen/`, `tools/rule_inventory.py`,
  `build/rule_inventory.json`
