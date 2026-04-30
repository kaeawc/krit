# CodegenRegistry

**Cluster:** [core-infra](README.md) · **Status:** ✅ shipped ·
**Severity:** n/a (infra) · **Default:** n/a

## What it does

Replaces the combination of 68 per-rule `init()` registrations, a global
mutable `DefaultInactive` map, and a ~370-line `applyRuleConfig()`
switch statement with a code-generated registry. Rule metadata,
defaults, and config schema are declared once (on the rule struct via
a `Meta() registry.RuleDescriptor` method); generators produce the
registration glue, the config application logic, and the inputs to JSON
schema.

## Current cost (pre-migration, historical)

Every new rule required three coordinated edits to shared
infrastructure:

1. `init()` call in its own file calling `Register(&YourRule{...})`.
2. An entry in `internal/rules/defaults.go` if the rule was opt-in.
3. A case branch in the 370-line `internal/rules/config.go`
   `applyRuleConfig()` switch — or the rule's config fields were
   silently never loaded.

Point 3 was the most dangerous: there was no compile-time check that
the switch was exhaustive. A rule author who forgot the case shipped a
rule whose config did nothing. The switch grew to ~370 lines and was
difficult to diff or review.

The `DefaultInactive` map was a global `map[string]bool` modified by
`ApplyConfig()`. Tests had to snapshot and restore it manually
(`snapshotDefaultInactive()`). There were no thread-safety guarantees
if config reloads happened concurrently (e.g., in the LSP server on a
workspace config change).

Relevant files (pre-migration):
- `internal/rules/config.go:72–370` — the switch statement (deleted)
- `internal/rules/defaults.go` — hand-maintained `DefaultInactive` map
  (now lazy-computed from `AllMetaProviders()`)
- Every rule file — `func init() { Register(...) }` (consolidated)

## Shipped design

### `Meta() registry.RuleDescriptor` per rule

Every rule struct exposes a `Meta()` method returning a
`registry.RuleDescriptor`. The descriptor is strongly-typed Go data
with closures for config `Apply` — not string tags — so the compiler
verifies field assignments and supports the `CustomApply` escape hatch
for rules with exotic config shapes.

```go
func (*LongMethodRule) Meta() registry.RuleDescriptor {
    return registry.RuleDescriptor{
        ID:            "LongMethod",
        Ruleset:       "complexity",
        DefaultActive: true,
        Options: []registry.Option{
            {
                Key: "allowedLines",
                Apply: func(r registry.RuleInstance, v any) error {
                    rr := r.(*LongMethodRule)
                    rr.AllowedLines = toInt(v, rr.AllowedLines)
                    return nil
                },
            },
        },
    }
}
```

Most rules get their `Meta()` generated; four exotic rules
(`ForbiddenImportRule`, `LayerDependencyViolationRule`,
`NewerVersionAvailableRule`, `PublicToInternalLeakyAbstractionRule`)
have hand-written `meta_*.go` files because their config parsing
can't be expressed as a list of `Options`:

- `ForbiddenImportRule`: a single config key writes to *two* struct
  fields simultaneously.
- `LayerDependencyViolationRule`: reads the whole config tree via
  `arch.ParseLayerConfig` and uses `registry.CustomApply`.
- `NewerVersionAvailableRule`: transforms `[]string` to
  `[]libMinVersion` via `parseRecommendedVersionSpecs`.
- `PublicToInternalLeakyAbstractionRule`: transforms int-percent to
  float64-fraction.

These structs are listed in `excludedStructs` inside
`internal/codegen/cmd/krit-gen/main.go` so the generator skips them.

### `internal/rules/registry/` package

Runtime types and APIs for the descriptor system:

- `RuleDescriptor`, `Option`, `CustomApply`, `MetaProvider` interface
- `Registry` and `RuleInstance` wrapper types
- Test fakes in `fake_config.go` for unit-testing config application

### `krit-gen` (`internal/codegen/cmd/krit-gen`)

Reads `build/rule_inventory.json` and emits per-source-file
`zz_meta_<src>_gen.go` files plus `zz_meta_index_gen.go` (a
name→descriptor index + `AllMetaProviders()`). Runs in `-verify` mode
in CI to detect drift.

### `

Extracts `v2.Register(...)` / manifest / resource / gradle
registrations from rule source files and emits
`internal/rules/registry_all.go`, replacing the 68 per-rule
`init()` blocks with one consolidated init.

### `tools/rule_inventory.py`

Python inventory producer. Parses rule source files (init bodies and
hand-written `Meta()` method bodies) to produce
`build/rule_inventory.json`, which is the input to `krit-gen`. The
two-stage pipeline (Python inventory → Go generator) is a deliberate
trade-off — see Deviations below.

## Progress / what shipped

Eight commits on `main` landed the migration:

| Commit | Phase | Summary |
|---|---|---|
| `2a162e5` | 1A | `internal/rules/registry/` package: descriptor, runtime, fakes |
| `ae75597` | 1B | `tools/rule_inventory.py` + initial `build/rule_inventory.json` |
| `99811bc` | 2C | `krit-gen` emits per-file `Meta()` methods from inventory |
| `59ab119` | 2D | 628 rules gain `Meta()` methods via generated `zz_meta_*_gen.go` |
| `0902ea1` | 3A | `ConfigAdapter` + `ApplyConfigViaRegistry` + 561/561 parity harness gating rollout |
| `88202d5` | 3C | `ApplyConfig` cut over to the registry; older switch + hand-maintained `DefaultInactive` map deleted |
| `43d4c3b` | 3E | `go:generate` directives + CI freshness test `TestGeneratedFilesUpToDate`; inventory script refresh |
| `122e9b6` | 3F-inv / 3G | `

Artifacts in-tree today:

- **66 generated files**: 64 `zz_meta_<src>_gen.go` + 1
  `zz_meta_index_gen.go` + 1 `registry_all.go`.
- **4 hand-written `meta_*.go` overrides** (plus `meta_lookup.go`, the
  generic `MetaForRule` helper that handles alias fallbacks).
- **3 remaining `init()` calls in `internal/rules/`** — all
  infrastructure, none per-rule: `registry_all.go` (one consolidated
  init for all 628 rules), `zzz_v2bridge.go` (v2 bridge population),
  `defaults.go` (lazy `DefaultInactive` population hook).
- **~1500 lines deleted** from `config.go` + `defaults.go` + schema
  plumbing in aggregate. `config.go` shrank from ~370 lines of switch
  to 94 lines total; `defaults.go` from a hand-maintained literal map
  to 85 lines of lazy-init plumbing.
- **628 rules total**, 97 with config options, 323 opt-in by default.
- **561/561 parity passes** between the older config path and the registry
  code path before Phase 3C deleted the older path.

## Authoring a new rule (today's workflow)

1. Write the rule struct + `Check` method + `v2.Register(...)`
   call (or the manifest / resource / gradle equivalent) in a normal
   rule file. No separate schema file, no switch-statement edit, no
   `DefaultInactive` edit.
2. If the rule has config options, either:
   - **(a) Generate `Meta()` normally.** After adding the rule, run
     `python3 tools/rule_inventory.py` to refresh
     `build/rule_inventory.json`, then
     `go generate ./internal/rules/...` to emit the `Meta()` method
     into the matching `zz_meta_*_gen.go` file.
   - **(b) Hand-write `meta_RuleName.go`** for exotic config
     (multi-field writes, whole-config reads, value transforms,
     `registry.CustomApply` escape hatch). Add the struct type to
     `excludedStructs` in
     `internal/codegen/cmd/krit-gen/main.go` so the generator skips it.
3. Run `python3 tools/rule_inventory.py && go generate ./internal/rules/...`.
4. `go test ./... -count=1` — the CI freshness tests
   `TestGeneratedFilesUpToDate` (krit-gen) and `TestRegistryFileUpToDate`
   (

## Acceptance criteria

| Criterion | Status | Notes |
|---|---|---|
| No `func init()` calls in `internal/rules/` per-rule | ✅ | Three remaining inits (`registry_all.go`, `zzz_v2bridge.go`, `defaults.go`) are infrastructure, not rule registrations. |
| `applyRuleConfig()` deleted | ✅ | Cut over in commit `88202d5`. Replaced by `ApplyConfigViaRegistry` which walks `Meta().Options`. |
| `DefaultInactive` global map replaced | ✅ (with deviation) | Now **lazy-runtime-initialized** from the generated `AllMetaProviders()` index via `ensureDefaultInactive()` + `sync.Once`, not a compile-time `const` set. Functionally equivalent — see Deviations. |
| Adding a new rule requires only the rule's file | ✅ (with nuance) | No shared infrastructure edit, but authors must invoke `python3 tools/rule_inventory.py && go generate` to refresh generated inputs. The CI freshness test forces this. |
| CI fails if generated files are out of date | ✅ | `TestGeneratedFilesUpToDate` (re-runs `krit-gen -verify`) and `TestRegistryFileUpToDate` (re-runs `
| All existing config-driven tests pass | ✅ | Full suite green; 561/561 parity between the older config path and the registry path before deleting the older path. |

## Deviations from original proposal

1. **`Meta()` method, not struct tags.** The original doc proposed
   Go struct tags (`krit:"default=60,config=allowed-lines"`). The
   shipped design uses a `Meta() registry.RuleDescriptor` method
   instead. Rationale:
   - Struct tags are stringly-typed; the compiler cannot verify field
     names or types.
   - Tags cannot express closures — `Apply` is a real
     `func(RuleInstance, any) error` that the compiler checks.
   - Tags cannot express `CustomApply` escape hatches for exotic
     config shapes (four such rules exist).
   - A method-returning-descriptor is idiomatic Go.
2. **`DefaultInactive` is lazy-runtime-initialized, not a compile-time
   const.** `ensureDefaultInactive()` walks `AllMetaProviders()` once
   (via `sync.Once`) and populates the map from each descriptor's
   `DefaultActive` field. The generator could in principle emit a
   compile-time map, but lazy init composes better with the v2 bridge
   init ordering and with alias registrations (four aliases whose
   registered name differs from their primary struct ID). Functionally
   equivalent for config and test purposes; no behavioral difference
   visible to rule authors.
3. **Two-stage regeneration (Python → Go).**
   `tools/rule_inventory.py` parses rule source files and emits JSON;
   `krit-gen` consumes the JSON. A future cleanup could move inventory
   parsing into `krit-gen` itself using `go/ast`, eliminating the
   Python stage. Not shipped — the split pipeline works, has a CI
   freshness gate, and the cost of unification is not yet worth the
   disruption.

## Links

- Depends on: [`unified-rule-interface.md`](unified-rule-interface.md) ✅
- Unlocks: [`cache-unification.md`](cache-unification.md) (rule
  version hashes can be derived from descriptor checksums)
- Related: `internal/rules/registry/`, `internal/rules/config.go`,
  `internal/rules/defaults.go`, `internal/codegen/cmd/krit-gen/`,
  `internal/codegen/cmd/
  `tools/rule_inventory.py`, `build/rule_inventory.json`
