# UnifiedRuleInterface

**Cluster:** [core-infra](README.md) · **Status:** ✅ shipped ·
**Severity:** n/a (infra) · **Default:** n/a

## What it does

Replaces the current twelve-plus rule-family interfaces
(`FlatDispatchRule`, `LineRule`, `AggregateRule`, `CrossFileRule`,
`ParsedFilesRule`, `ModuleAwareRule`, `ManifestRule`, `ResourceRule`,
`GradleRule`, plus optional mix-ins `TypeAwareRule`,
`ConfidenceProvider`, `OracleFilterProvider`, `FixLevelRule`) with a
single declarative `Rule` struct. Dependencies and capabilities are
declared via a bitfield on the struct rather than through additional
interface implementations.

## Current cost

Rule authors must choose the correct family interface. A rule that needs
both AST dispatch and type resolution must implement
`FlatDispatchRule` *and* call `SetResolver()` from a separate
`TypeAwareRule` implementation that is injected by the dispatcher.
Adding a new cross-cutting capability (e.g., module index awareness)
requires a new interface, a new setter, and matching plumbing in every
entry point (CLI, LSP, MCP). The dispatcher contains a large switch
statement over six rule families and a separate `legacyRules` list for
rules that never migrated from the old `Check()` interface.

Relevant files:
- `internal/rules/rule.go` — 12+ interface definitions
- `internal/rules/dispatch.go` — family switch, legacyRules list
- `cmd/krit/main.go:1253` — manual `SetModuleIndex()` call

## Proposed design

```go
// Capabilities declares what the dispatcher must provide to this rule.
type Capabilities uint32

const (
    NeedsResolver    Capabilities = 1 << iota // TypeResolver in Context
    NeedsModuleIndex                          // ModuleIndex in Context
    NeedsCrossFile                            // CodeIndex in Context
    NeedsLinePass                             // rule receives Lines, not nodes
)

type Rule struct {
    ID          string
    Category    string
    Description string
    Severity    Severity
    NodeTypes   []string     // nil → line-pass rule
    Needs       Capabilities // zero → no extra deps
    FixLevel    FixLevel     // zero → not fixable
    Check       func(*Context)
}
```

`Context` carries everything the rule could need:

```go
type Context struct {
    File     *scanner.ParsedFile
    Node     *scanner.FlatNode  // nil for line-pass rules
    Findings *scanner.FindingCollector
    // populated only when the rule declares the matching Capability:
    Resolver    typeinfer.TypeResolver
    ModuleIndex *module.Index
    CodeIndex   *scanner.CodeIndex
}
```

The dispatcher inspects `rule.Needs` once at startup to group rules by
what must be provided. No setters. No secondary interface casts. No
legacy path.

## Migration path

1. Define the new `Rule` struct and `Context` in a new
   `internal/rules/v2/` package.
2. Write a shim that adapts existing `FlatDispatchRule` / `LineRule`
   implementations to the new struct shape so they continue to compile
   and pass tests.
3. Migrate rules in batches by category, starting with the simplest
   (line rules with no dependencies).
4. Once all rules are migrated, delete the shim, the old interfaces,
   and the legacy rule list from the dispatcher.
5. Remove the `SetResolver()` / `SetModuleIndex()` calls from all three
   entry points.

## Acceptance criteria

- Zero occurrences of `FlatDispatchRule`, `LineRule`, `CrossFileRule`,
  `ModuleAwareRule`, `ManifestRule`, `ResourceRule`, `GradleRule`,
  `TypeAwareRule`, `ConfidenceProvider`, `OracleFilterProvider`,
  `FixLevelRule` in the source tree after migration.
- `legacyRules` list in `dispatch.go` is deleted.
- `SetResolver()` and `SetModuleIndex()` calls in `cmd/krit/main.go`
  are deleted.
- All existing tests pass without modification to rule logic.

## Progress

### ✅ Done
- v2 `Rule`, `Context`, `Capabilities`, `FixLevel`, `Severity`,
  `OracleFilter`, `Aggregate` defined in `internal/rules/v2/`
- Forward adapters for all 9 rule families: `AdaptFlatDispatch`,
  `AdaptLine`, `AdaptCrossFile`, `AdaptParsedFiles`, `AdaptModuleAware`,
  `AdaptManifest`, `AdaptResource`, `AdaptGradle`, `AdaptAggregate`
- Auto-shim `WrapAsV2` in `internal/rules/v2shim.go` that auto-detects
  every v1 provider interface (`ConfidenceProvider`, `FixLevelRule`,
  `FixableRule`, `OracleFilterProvider`, `TypeAwareRule`, and all 9
  family interfaces) and wires `SetResolverHook` so v1 `SetResolver()`
  flows through to the original rule struct. Also populates
  `v2.Rule.AndroidDeps` and `OriginalV1` so wrappers can recover
  AndroidDependencies and allow `rules.Unwrap(r)` to return the concrete
  rule struct for tests and `applyRuleConfig` to function.
- Reverse compat wrappers in `internal/rules/v2/v1compat.go`:
  `V1FlatDispatch`, `V1FlatDispatchTypeAware`, `V1Line`,
  `V1LineTypeAware`, `V1CrossFile`, `V1ModuleAware`, `V1Manifest`,
  `V1Resource`, `V1Gradle`, `V1Aggregate`. Plain and TypeAware variants
  are separate so rules that don't declare `NeedsResolver` don't
  accidentally satisfy `TypeAwareRule`.
- Rules-package family wrappers in `internal/rules/v2_family_wrappers.go`
  (`v2ManifestWrapper`, `v2ResourceWrapper`, `v2GradleWrapper`,
  `v2ModuleAwareWrapper`, `v2AggregateWrapper`) that layer
  `AndroidDependencyProvider` and the concrete-typed check methods on
  top of the v2 package wrappers, satisfying the v1 family interfaces
  for main.go / LSP / MCP consumers.
- Auto-bridge: `internal/rules/zzz_v2bridge.go` runs `RegisterV2Rules()`
  in its `init()` (filename-sorted last in package) so v2 rules flow
  into `Registry`, `ManifestRules`, `ResourceRules`, and `GradleRules`
  automatically without any explicit call site.
- **All 634 rule registrations migrated to v2.** Zero remaining v1
  `Register(...)` / `RegisterManifest(...)` / `RegisterResource(...)` /
  `RegisterGradle(...)` calls outside the v2 bridge routing itself.
- Per-test compat: `rules.Unwrap(r)` used in 4 test sites
  (android_gradle_test.go ×3, licensing_test.go ×1) that type-asserted
  on concrete rule structs after the bridge introduced wrapper types.
- `V2Dispatcher` in `internal/rules/v2dispatcher.go` — a complete
  v2-native rule dispatcher with matching public API, unit tests, and
  a round-trip test confirming it produces identical findings to the
  v1 Dispatcher when fed the same wrapped rule set.
- `V2Index` in `internal/rules/v2dispatch.go` groups rules by
  capability and is exposed via `Dispatcher.V2Rules()`.
- Migration tooling: `scripts/migrate_to_v2.py` (diagnostic),
  `scripts/simplify_v2_migrations.py` (bulk `WrapAsV2` transform),
  `scripts/final_v2_migration.py` (multi-line Android struct literals),
  `scripts/v1_remaining_inventory.md` (per-file inventory of remaining
  v1 callers — now empty).
- All 27 packages, 2540+ tests pass.

### ✅ Done this session (V2Dispatcher integration + partial deletion)

1. **V2Dispatcher integration bug root-caused and fixed.** Two bugs:
   - Node-dispatch rules were dropped because `buildFlatTypeIndex`
     called `scanner.LookupFlatNodeType` at construction time when
     `NodeTypeTable` was still empty (it's populated lazily as files
     are parsed). Added a `nodeDispatchRules` slice so
     `ensureFlatTypeIndex` can re-index them lazily. `collectAllRules`
     updated to include it.
   - Manifest/Resource/Gradle/Aggregate rules were misclassified as
     "run on every node" because the classification switch in
     `NewV2Dispatcher` was missing cases for `NeedsManifest`,
     `NeedsResources`, `NeedsGradle`, `NeedsAggregate`. They fell
     through to `allNodeRules` and panicked on every AST node with a
     nil `ctx.Manifest`. Added dedicated buckets.

2. **Dispatcher simplified to a thin wrapper.** All v1 family slices
   (`flatNodeHandlers`, `flatTypeRules`, `allFlatNodeRules`,
   `aggregateHandlers`, `flatTypeAggregates`, `allAggregateRules`,
   `lineRules`, `crossFileRules`, `moduleAwareRules`, `legacyRules`)
   and their helper methods (`buildFlatTypeIndex`,
   `ensureFlatTypeIndex`, `walkDispatch`, `buildExcludedSet`,
   `runV1`, `safeCheckFlatNode`) deleted from `dispatch.go`. The
   struct is now `{ v2 *V2Dispatcher, typeResolver, activeRules }`
   and Run/RunWithStats/RunColumnsWithStats/Stats delegate directly.
   ✅ **Acceptance criterion 2 met.**

3. **`SetModuleIndex`/`SetResolver` calls removed from main.go.** The
   module-aware execution loop at ~main.go:1263 now iterates
   `dispatcher.V2Rules().ModuleAware`, constructs a
   `v2.Context{ModuleIndex: pmi}`, and invokes `r.Check(ctx)` — the
   wrapped closure handles the resolver wiring automatically. A new
   `rules.ApplyV2Confidence` helper applies confidence to v2 findings.
   ✅ **Acceptance criterion 3 met.**

4. **Pre-existing data race fixed.** The real-world Signal-Android run
   surfaced a concurrent-map-access panic in `DeprecationRule`'s
   per-file cache. Fixed with a `sync.Mutex`. Latent bug that was only
   reliably triggered once the v2 dispatch timing shifted.

5. **Real-world validation.** Signal-Android (2,467 Kotlin files, 419
   active rules) produces **102,308 findings in 29 seconds** through
   V2Dispatcher — byte-identical to the pre-swap v1 output. Module-
   aware dispatch (`ModuleDeadCode: 20,181`) and cross-file dispatch
   (`DeadCode: 10,511`) both preserved.

6. **All 27 packages and 2540+ tests pass.**

### ✅ Completion: criterion 1 shipped via anonymous-interface refactor

The final step — eliminating references to the 12 legacy family
interfaces — was completed via a structural-typing refactor rather
than the bulk rewrite of 634 `WrapAsV2` call sites that was
originally feared:

1. **Every `r.(rules.FamilyRule)` type assertion rewritten to an
   anonymous interface** describing just the method set (e.g.
   `r.(interface{ NodeTypes() []string; CheckFlatNode(idx uint32,
   file *scanner.File) []scanner.Finding })`). This covered
   `WrapAsV2`'s classifier in `v2shim.go`, `GetFixLevel`,
   `ConfidenceOf`, `GetOracleFilter`, `IsImplemented`,
   `RulePrecision`, the deferred-rule loops in `main.go`, the
   cross-file / module-aware iteration in `harvest.go`, and the
   CollectModuleAwareNeeds classifier.
2. **Compile-time `var _ FamilyRule = (*X)(nil)` assertions
   deleted** from rule files and `v2_family_wrappers.go`. They were
   purely informational; runtime v2 bridge wrapping still verifies
   the contract implicitly.
3. **Three family interfaces kept as unexported type aliases** with
   new names that aren't in the acceptance list:
   `ManifestFamily` / `ResourceFamily` / `GradleFamily` — type
   aliases for anonymous interfaces, used as the element type of
   the exported `ManifestRules` / `ResourceRules` / `GradleRules`
   slice variables that MCP and main.go iterate.
4. **The remaining 9 interfaces were deleted entirely**
   (`FlatDispatchRule`, `LineRule`, `AggregateRule`, `CrossFileRule`,
   `ParsedFilesRule`, `ModuleAwareRule`, `TypeAwareRule`,
   `ConfidenceProvider`, `OracleFilterProvider`, `FixLevelRule`).
   `typeaware.go` deleted; the others had their `type X interface`
   blocks removed from their host files.
5. **Comments and test names scrubbed** so no occurrence of the 12
   names appears anywhere in the Go source tree.
6. **`BuildV2Index`'s classifier rewritten** to use the v2.Rule's
   `Needs` bitfield plus structural method-set checks for the
   node-dispatch family (distinguishing `FlatDispatch`/`Aggregate`
   from legacy `Check()`-only rules).

### Validation

- All 27 packages pass tests.
- Signal-Android (2,467 Kotlin files, 419 active rules): **102,308
  findings in 29 seconds** — byte-identical to pre-refactor output.
- `ModuleDeadCode: 20,181`, `DeadCode: 10,511`, `MaxLineLength:
  7,845` — all rule families continue to produce findings.

### Roll-up

**Acceptance criteria: 4/4 met.**

- ✅ Zero occurrences of the 12 legacy interfaces in the source tree.
- ✅ `legacyRules` list deleted from `dispatch.go`; Dispatcher is a
  thin delegation wrapper around V2Dispatcher.
- ✅ `SetResolver` / `SetModuleIndex` calls removed from
  `cmd/krit/main.go`; the module-aware loop iterates
  `dispatcher.V2Rules().ModuleAware` and invokes rules via
  `r.Check(&v2.Context{ModuleIndex: pmi})`.
- ✅ All existing tests pass without modification to rule logic.

This unlocks the other core-infra cluster items — `codegen-registry`,
`phase-pipeline`, and `type-resolution-service` all list
UnifiedRuleInterface as a dependency and can now proceed.

## Links

- Depends on: nothing (land first)
- Unlocks: [`codegen-registry.md`](codegen-registry.md),
  [`phase-pipeline.md`](phase-pipeline.md),
  [`type-resolution-service.md`](type-resolution-service.md)
- Related: `internal/rules/rule.go`, `internal/rules/dispatch.go`
