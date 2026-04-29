---
name: krit-capability-migration
description: Use when deciding whether a Krit rule truly needs Kotlin Analysis API, NeedsTypeInfo, NeedsResolver, parsed files, cross-file analysis, or library model facts, and when moving rules off KAA by replacing oracle facts with AST, source type inference, imports, project indexes, or Gradle-derived library profile.
---

# Krit Capability Migration

Use this when reducing KAA usage or clarifying a rule's `Needs*` declaration.

## Capability Definitions

- **No Needs**: local AST, line text, imports, and cheap file-local helpers are enough.
- **NeedsResolver**: source-level type inference is required, but Kotlin Analysis API is not. Prefer `TypeInfo: PreferResolver`.
- **NeedsTypeInfo**: legacy type-aware bucket. It does not automatically mean KAA. A rule only participates in KAA if it has `NeedsOracle`, `Oracle`, `OracleCallTargets`, `OracleDeclarationNeeds`, or oracle diagnostics.
- **NeedsOracle**: rule requires Kotlin Analysis API facts that source inference cannot safely provide.
- **NeedsParsedFiles**: rule needs all parsed source files, but not necessarily symbol/reference indexes.
- **NeedsCrossFile**: rule needs the cross-file symbol/reference index.
- **NeedsModuleIndex**: rule needs Gradle module boundaries or per-module symbol/reference data.
- **LibraryFacts (implicit)**: all rules receive `ctx.LibraryFacts` automatically — no `Needs` flag required. Rules should call `librarymodel.EnsureFacts(ctx.LibraryFacts)` when their behavior depends on whether a library (Room, Compose, Hilt, etc.) is present.

If a rule can be 100% confident with AST + imports + source inference, remove oracle metadata and prefer resolver-only or no-needs.

## Library Model as an Alternative to KAA

Before migrating a rule to use KAA for library-presence detection, check whether `LibraryFacts` already answers the question:

| Oracle fact needed | Library model alternative |
|--------------------|---------------------------|
| Is Room present? | `librarymodel.EnsureFacts(ctx.LibraryFacts).Database.Room.Enabled` |
| Is Compose present? | `facts.Profile.MayUseAnyDependency(Coordinate{"androidx.compose.runtime","runtime"})` |
| Is Hilt/Dagger present? | `facts.Profile.MayUseAnyDependency(Coordinate{"com.google.dagger","hilt-android"}, ...)` |
| Min SDK version? | `facts.Profile.Android.MinSdkVersion` |
| Kotlin version? | `facts.Profile.Kotlin.EffectiveCompilerVersion()` |

`LibraryFacts` is derived from `internal/librarymodel/catalog.go` (TOML version catalogs) and `internal/librarymodel/profile.go` (Gradle build files). It is populated before rule dispatch and requires no extra `Needs` declaration.

When `facts.Profile.DependencyExtractionComplete` is false, treat the library as potentially present. Never use absence of a dependency as a hard guard unless completeness is confirmed.

## Migration Workflow

1. Find active KAA participants:

```bash
rg -n "NeedsOracle|NeedsTypeInfo|OracleCallTargets|OracleDeclarationNeeds|PreferOracle" internal/rules/registry_*.go internal/rules/*.go
```

2. For each rule, list the exact oracle API used:
   - call target FQN
   - suspend marker
   - annotations on call target
   - expression type/nullability
   - class supertypes/members
   - diagnostics

3. Challenge each fact:
   - Can imports/FQNs prove the library?
   - Can tree-sitter shape prove the construct?
   - Can `internal/typeinfer` prove enough?
   - Can `NeedsParsedFiles` summaries prove it?
   - Can the cross-file index prove references/ownership?

4. If KAA remains necessary, narrow it:
   - add `OracleCallTargets` with bounded `CalleeNames`, `TargetFQNs`, lexical hints, or lexical skips
   - add `OracleDeclarationNeeds`
   - avoid nil declaration needs, which force full declaration extraction
   - avoid `AllCalls` unless there is no safe alternative

5. Add tests that lock the capability decision:
   - rule-level behavior tests
   - `oracle_filter_narrowing_test.go` entries for KAA rules
   - tests asserting resolver-only rules do not contribute to KAA

## AT + Typeinfer Proof Checklist

Move a rule off KAA only when at least one proof path is complete:

- Import/FQN proof: the file imports or mentions the canonical library FQN, and local lookalikes are negative-tested.
- Receiver proof: the receiver name/type is specific enough via source inference or declaration text.
- Owner proof: lifecycle/callback/main-thread rules verify actual owner supertypes/interfaces, not only method names.
- Boundary proof: async rules account for background, deferred, and event-callback boundaries.
- Cross-file proof: summaries or indexes resolve helper calls without guessing across ambiguous overloads.

## Benchmark After Migration

Run cold KAA and no-oracle per-rule timing:

```bash
KRIT="$PWD/krit" scripts/benchmark-oracle.sh ~/github/Signal-Android 2
./krit -no-cache -no-type-oracle -perf -perf-rules -f json -q \
  -o /tmp/krit_signal_perf_no_oracle.json ~/github/Signal-Android || true
```

Successful migration should not increase total findings unexpectedly, should not broaden per-rule cost, and should reduce KAA file/call/declaration workload when the migrated rule was active.

## Final Validation

Use focused tests while iterating, then:

```bash
go build -o krit ./cmd/krit/
go vet ./...
go test ./... -count=1
```
