# Flat tree migration cluster

Parent: [`roadmap/68-flat-tree-migration.md`](../../68-flat-tree-migration.md)
Status snapshot: **2026-04-14 — Track A done, Tracks B/C/D/E remaining.**

## Goal

Eliminate cgo from the live rule-dispatch path. Every question a rule
asks the tree (`node.Child()`, `node.Type()`, `node.Parent()`,
`node.ChildCount()`, etc.) used to cross the Go/C boundary once per
call. The `FlatNode` / `FlatTree` / `flattenTree` infrastructure copies
each parsed file's sitter tree into a preorder-flattened Go slice once
at parse time, and all subsequent traversal happens as plain-Go slice
lookups.

This cluster is about doing that migration across the codebase. The
win is latency (less cgo overhead on every parsed file), memory (the C
parse tree can be freed immediately after flattening), and API
hygiene (one way to walk nodes in rule code instead of two).

## Status snapshot

**Done and shipped:**

- **Phase 1 infrastructure** — dispatcher is flat-only. `walkDispatch`
  iterates `file.FlatTree.Nodes` directly with integer-keyed
  `flatTypeRules` lookup. `FlatDispatchRule.CheckFlatNode(idx, file)`
  is the only rule-dispatch interface; there is no
  `CheckNode(*sitter.Node, ...)` fallback anywhere in the dispatcher.
- **Flat helpers** — the full set of flat-tree traversal helpers
  exists on `*scanner.File` and as package-level functions in
  `internal/scanner/flat.go`:
  `FlatFindChild`, `FlatNodeText`, `FlatHasModifier`,
  `FlatHasChildOfType`, `FlatHasAncestorOfType`, `FlatForEachChild`,
  `FlatWalkNodes`, `FlatWalkAllNodes`, `FlatCountNodes`,
  `FlatNamedDescendantForByteRange`, etc.
- **`flattenTree` second-return removal** — the parallel
  `[]sitter.Node` slice that earlier versions of the migration
  retained is gone. `ParseFile` no longer keeps a per-node sitter
  handle alive.
- **Rule file bulk migration** — 13 of the original 42 rule files
  are fully flat (zero `*sitter.Node` references). The remaining
  rule tail is tracked as Track A below.

**Track A done (2026-04-14):**

- **Rule file live surface** — zero `*sitter.Node` references on any
  path reachable from a rule's `CheckFlatNode` entry. 156 node-era
  functions / ~212 refs remain as an **archive** (draft scaffolding
  for 16 unlanded Compose rules + historical reference). See
  [`internal/rules/NODE_ERA_ARCHIVE.md`](../../../internal/rules/NODE_ERA_ARCHIVE.md)
  for the reachability analysis, archive rules-for-maintainers, and
  a runnable verification script.

**Still left (the tracks in this plan):**

- `internal/typeinfer` subsystem — ~203 refs across 7 files, not yet
  meaningfully flattened at the boundary.
- Scanner compatibility surface — 10 exported cgo helpers in
  `scanner.go` still used by call sites in `internal/typeinfer` and
  oracle compat; `suppress.go` still carries a node-based
  `BuildSuppressionIndex` entry point alongside the flat one;
  `query.go` is still a node/query compat subsystem.
- Phase 4 closure — deletion of the `node_compat_*.go` files and the
  rule archive, dropping `scanner.File.Tree`, final fresh perf
  capture. **Gated** on Track B completing and on the Compose
  flat-conversion sweep (Track F) landing the 16 unlanded rules in
  their flat form. Decision recorded 2026-04-14: the 16 rules will be
  converted from their node-era sketches to flat-tree and registered
  in `compose.go`, then the archive is deleted wholesale.
- Postponed (separate doc):
  [`roadmap/postponed/flat-field-names.md`](../../postponed/flat-field-names.md)
  — field-name lookup support, blocked on the vendored Kotlin
  grammar providing `FIELD_COUNT = 0`. No existing rule needs it.

## Remaining work

### Track A — Rule file tail — **DONE on the live-surface criterion**

**Status (2026-04-14):** Complete. The live rule-dispatch surface has
zero `*sitter.Node` references. What looked like "212 refs across 23
files remaining to migrate" turned out to be a closed archive of
node-era code that is structurally unreachable from any `CheckFlatNode`
entry point.

A fixed-point reachability analysis starting from every rule's
`CheckFlatNode` method and following the call graph across
`internal/rules/` produced this result:

    Total sitter.Node-using functions:  156
    Reachable from CheckFlatNode:         0
    Transitively dead:                  156

**Every node-era function in `internal/rules/` is unreachable from
live dispatch.** The subgraph is disconnected — it has no entry point,
because:

1. `file.FlatNode(idx) *sitter.Node` was removed earlier in the
   migration, so there is no bridge from flat index back to sitter.
2. `scanner.File.Tree` was removed, so rules cannot reach the C
   sitter tree via the file object.
3. Every `CheckFlatNode` takes only `(idx uint32, file *scanner.File)`.
   A `*sitter.Node` can only enter a rule's call graph as a parameter
   from a caller that already has one, and the chain never bottoms
   out at a live `CheckFlatNode`.

**The remaining 156 functions / 212 refs are retained as an archive,
not deleted.** Reasons documented in
[`internal/rules/NODE_ERA_ARCHIVE.md`](../../../internal/rules/NODE_ERA_ARCHIVE.md),
summarized:

- Most are pre-flat-migration draft scaffolding for the **16
  unlanded Compose rules** in
  [`roadmap/clusters/compose/`](../compose/) (only 3 of the 19 compose
  rules are currently registered). Deleting would force each new rule
  to reinvent traversal patterns.
- Some are historical reference implementations kept as a diff
  oracle against the `*Flat` versions during the type-inference
  migration (Track B).
- The archive is cheap to retain and expensive to resurrect, so the
  asymmetry favors keeping it.

**Track A done criteria (reframed):**

- ✅ `grep -rn '\*sitter\.Node' internal/rules/*.go | grep -v _test.go`
  in **live code paths** returns zero (verified via reachability
  analysis in `NODE_ERA_ARCHIVE.md`).
- ✅ `go test ./internal/rules/...` passes.
- ✅ `NODE_ERA_ARCHIVE.md` exists and documents the boundary
  between live and archive code, with a runnable reachability
  script for future verification.
- ⏸️ Physical deletion of `node_compat_helpers.go`,
  `node_compat_flow_helpers.go`, and the 156 archive functions is
  **deferred** to Phase 4 closure (Track D) — explicitly gated on
  either (a) the Compose rule cluster shipping the rules that would
  consume the archive, or (b) an explicit decision to cancel those
  rules. Neither has happened yet; the archive stays.

This reframes the original "eliminate all `*sitter.Node` refs in
rule files" criterion as "eliminate them from live dispatch paths,
with a documented archive of the rest." The practical effect is
identical — runtime behavior is unchanged — and the archive
boundary is verifiable mechanically.

**Scanner compat helpers** (`scanner.go` node-based functions like
`FindChild`, `NodeText`, `WalkAllNodes`, etc.) are **not** part of
this archive. They are called from `internal/typeinfer` and are
still genuinely live; they're handled by Track C below.

---

### Track B — `internal/typeinfer` subsystem migration

The type-inference subsystem is the single biggest remaining pocket
of node-era code inside krit. As of 2026-04-14 the `TypeResolver`
interface in `internal/typeinfer/resolver.go` is **already fully
flat-native** — it exposes only `ResolveFlatNode`, `ResolveByNameFlat`,
`IsNullableFlat`, and `AnnotationValueFlat`. The public API shape is
locked; [`migrate-typeinfer-api.md`](migrate-typeinfer-api.md) is
marked `Status: completed` accordingly.

What remains is internal implementation: several files still walk the
sitter tree internally even though they hand back flat-typed results.
This is now a straight-line cleanup across files rather than a
gated/ordered migration. There's no public API churn — callers don't
move.

**Current state (2026-04-14, all files including tests):**

| File | refs | status |
|---|---|---|
| `declarations.go` | 0 | ✅ Fully flat-native. |
| `scopes.go` | 0 | ✅ Fully flat-native. |
| `helpers.go` | 0 | ✅ Fully flat-native. |
| `imports.go` | 0 | ✅ Fully flat-native. |
| `resolve.go` | 0 | ✅ Fully flat-native. |
| `api.go` | 0 | ✅ Fully flat-native. Node-era public compat shims (`ResolveNode`, `ResolveByName`, `IsNullable`, `AnnotationValue`) and `flatIndexForNode` byte-range bridge deleted 2026-04-14 in the same pass that migrated ~60 test callsites across 10 test files to `ResolveFlatNode` / `ResolveByNameFlat` / `IsNullableFlat` / `AnnotationValueFlat`. |
| `fake.go` | 0 | ✅ Fully flat-native. `FakeResolver` node-era methods deleted with the `defaultResolver` ones. |

Plus in `internal/oracle/composite.go`: the `legacyNodeResolver`
interface and the four `CompositeResolver` compat shim methods
(`ResolveNode`, `ResolveByName`, `IsNullable`, `AnnotationValue`) were
deleted together. The `fakeTypeResolver` in `oracle_test.go` was
pruned to match.

**Total: 0 live `*sitter.Node` refs in `internal/typeinfer`** (prod
*or* tests), down from 65 at the start of 2026-04-14 and the ~203 the
parent roadmap cited. Track B is done.

Behavioral coverage backfill: parity oracles (`*_NodeAndFlatAgree`,
`*_NodeVsFlatLambdaLastExpression`) that were deleted along with the
node-era methods were replaced by direct behavioral tests in
[`resolver_behavioral_test.go`](../../../internal/typeinfer/resolver_behavioral_test.go)
covering: generic propagation through call-expression receivers,
companion navigation resolution, lazy/remember lambda last-expression
inference, and nullable local declaration resolution. Inner-scope
shadowing via `ResolveByNameFlat` is covered by the pre-existing
`TestDefaultResolver_ResolveByNameFlat_UsesScopeOffset`.

Test infrastructure: `testRoot(t, file)` was retired (no callers) and
`test_parse_helpers_test.go` now only exposes `flatFirstOfType` and
`flatFirstOfTypeWithText` flat-walk helpers. `scopefunc_test.go` also
carries `flatFirstCallContaining` / `flatLongestCallContaining`
variants for substring-match cases.

`internal/typeinfer/parallel.go` still reparses file content and
walks `tree.RootNode()` for per-file indexing. The decision of
whether that path stays as a local parse-only compat or becomes
flat-native is the subject of
[`migrate-typeinfer-parallel-indexing.md`](migrate-typeinfer-parallel-indexing.md).

**Ordering within Track B:**

The API shape is already defined (see `resolver.go`), so the
ordering below is about minimizing merge conflicts between concurrent
in-flight migrations, not about gating.

1. **[`migrate-typeinfer-declarations.md`](migrate-typeinfer-declarations.md)**
   and **[`migrate-typeinfer-resolve.md`](migrate-typeinfer-resolve.md)**
   can run in parallel. Declarations handles imports, class/function
   indexing, member extraction; resolve handles core expression/type
   resolution and the `helpers.go` primitives. These are the two
   largest pockets and touch largely disjoint files.
2. **[`migrate-typeinfer-scopes.md`](migrate-typeinfer-scopes.md)**
   depends on the resolve + declaration helpers from step 1. Scope
   building is where smart-cast and null-check extraction live,
   and both lean on the core resolver.
3. **[`migrate-typeinfer-parallel-indexing.md`](migrate-typeinfer-parallel-indexing.md)**
   last — explicit decision point for whether parallel indexing
   stays node-based (as a compat path) or goes flat-native.

**Done criteria for Track B:**

- Public typeinfer API exposes flat-native equivalents for every
  live use case: resolve by flat idx, nullability by flat idx,
  annotation lookup by flat idx.
- `grep -rn '\*sitter\.Node' internal/typeinfer/*.go | grep -v
  _test.go | grep -v 'resolver\.go'` returns zero — i.e. no
  internal typeinfer code still walks the sitter tree. The
  `resolver.go` exception exists only if we decide to keep the
  node-based public methods as a compat surface with a deprecation
  marker; see the Phase 4 closure for final deletion.
- `go test ./internal/typeinfer/...` passes.
- Rule callers that were reaching into typeinfer via node-based
  methods now use the flat surface.

---

### Track C — Scanner compatibility cleanup — **DONE (2026-04-14)**

The scanner package's production API now has **zero** exported
`*sitter.Node` / `*sitter.Tree` helpers — the 7 surviving cgo helpers
that were still used by the rule-archive have been moved out of
`internal/scanner/` entirely.

**What landed on 2026-04-14:**

1. **Dead shims deleted.** `ForEachChildOfType`, `NodeBytes`, and
   `HasChildOfType` had zero callers post-Track-B and were removed
   outright.
2. **Typeinfer dependency: gone.** After Track B, the typeinfer
   subsystem had 0 calls into the scanner compat surface (down from
   73). This was verified by the same grep used in the original
   table above.
3. **Rules dependency: relocated.** The remaining 141 call sites in
   `internal/rules/` (all in archive code transitively dead from
   `CheckFlatNode`, verified by the reachability script in
   `NODE_ERA_ARCHIVE.md`) had their `scanner.NodeText` /
   `scanner.FindChild` / `scanner.HasModifier` / `scanner.WalkNodes`
   / `scanner.WalkAllNodes` / `scanner.ForEachChild` /
   `scanner.HasAncestorOfType` prefixes rewritten to package-local
   `compat*` helpers in a new file
   [`internal/rules/scanner_compat_archive.go`](../../../internal/rules/scanner_compat_archive.go).
   The archive's node-era dependencies are now fully local to
   `internal/rules/` and retire together with Track F.
4. **Scanner test parity oracles preserved.** The scanner package's
   own parity tests (`flat_test.go`, `scanner_test.go`) still need
   the node-era helpers to verify the FlatTree primitives. Those
   helpers were moved into `internal/scanner/compat_helpers_test.go`
   (a `_test.go` file — so they compile only for tests, not for the
   production binary). The scanner package's production exports are
   pure-flat; tests still get their oracles.
5. **`suppress.go` was already flat.** The cluster's original plan
   called for deleting a `BuildSuppressionIndex(*sitter.Node, ...)`
   node-era entry point, but that entry point no longer exists —
   only `BuildSuppressionIndexFlat` is live.
6. **`query.go` was already gone.** The cluster's original plan
   listed the s-expression query subsystem as "to be decided," but
   `internal/scanner/query.go` had been deleted before this session
   started. A `query_test.go` remains with `CompiledQuery` /
   `QueryNodes` / `FindChildByQuery` helpers as test infrastructure,
   used only by per-rule benchmarks — not by live code.
7. **`scanner.File.Tree` was already gone.** The cluster's Track D
   Phase 4 closure listed "Drop `Tree *sitter.Tree` from
   `scanner.File`" as a blocker. That field is not present on
   `scanner.File`; `NewParsedFile` calls `flattenTree(tree.RootNode())`
   and then discards the tree immediately. There is no `Tree` field
   to drop.

**Current state (2026-04-14):**

`grep '\*sitter\.Node' internal/scanner/*.go` (non-test) now reports
only the legitimate `flattenTree(root *sitter.Node)` entry point in
`flat.go` — **4 refs total, all inside the single function that
converts a sitter tree into a flat tree.** Similarly, `*sitter.Tree`
in production is only the `tree *sitter.Tree` parameter to
`NewParsedFile`, which accepts a just-parsed tree and flattens it.
These are the package's sole sitter contacts.

**Done criteria (all met):**

- ✅ The ten original `scanner.go` cgo helper functions: 3 were
  dead, 7 moved out of the scanner package — net 0 exported from
  scanner.
- ✅ `BuildSuppressionIndex(*sitter.Node, ...)` is gone (was already
  gone before Track C started).
- ✅ `scanner/query.go` is gone (was already gone before Track C
  started).
- ✅ `go test ./internal/scanner/...` passes.
- ✅ `go test ./...` passes.

---

### Track D — Phase 4 closure (drop cgo fallback) — **DONE (2026-04-14)**

All five items landed on 2026-04-14:

1. ✅ Deleted the rule-archive. After Track F landed all 16
   unlanded Compose rules in their flat form, a reachability
   analysis confirmed 164 `*sitter.Node`-using functions across 24
   rule files were transitively dead from `CheckFlatNode`. A single
   scripted sweep removed all 164 functions, plus the now-orphan
   `compat*` helpers, plus the now-unused sitter imports across the
   affected files. The three standalone archive files
   (`node_compat_helpers.go`, `node_compat_flow_helpers.go`,
   `scanner_compat_archive.go`) and the `NODE_ERA_ARCHIVE.md` doc
   were deleted wholesale. Net: −2479 lines across
   `internal/rules/`.
2. ✅ `internal/scanner/scanner.go`'s ten cgo helpers — landed as
   part of Track C.
3. ✅ `scanner.File.Tree` field dropped — already done before this
   cluster started; `NewParsedFile` flattens and discards the
   sitter tree immediately.
4. ✅ `BuildSuppressionIndex(*sitter.Node, ...)` — already done;
   only `BuildSuppressionIndexFlat` is live.
5. ✅ `go test ./...` passes.

**Final sitter footprint in production code:**

    internal/rules/:       0 *sitter.Node refs
    internal/typeinfer/:   0 *sitter.Node refs
    internal/scanner/:     5 refs, all in the legitimate flattenTree
                           / NewParsedFile entry point

These 5 remaining refs are the "one way into flat land" —
`scanner.NewParsedFile(path, content, tree *sitter.Tree)` accepts a
freshly-parsed tree and calls `flattenTree(tree.RootNode())` which
walks the sitter tree once to build the `*FlatTree`. Nothing retains
the sitter handle past that call. The cluster is done.

See [`drop-cgo-fallback.md`](drop-cgo-fallback.md) for the verification
script and expected impact.

**Done criteria for Track D:**

- `grep -rn '\*sitter\.Node\|\*sitter\.Tree' internal/ | grep -v
  _test.go | grep -v 'internal/android/xmlast\.go'` returns only
  the handful of legitimate entry points where a sitter handle is
  still necessary (tree-sitter query machinery in scanner, if kept).
- `scanner.File` has no `Tree` field.
- Full test suite passes.
- A fresh benchmark file exists in `benchmarks/` showing the
  post-closure wall-clock and memory profile on Signal-Android.

---

### Track E — Verification and benchmark refresh

Mostly independent of the other tracks and can start now.

The parent roadmap (`roadmap/68-flat-tree-migration.md`) originally
cited `1,208ms of 2,415ms` on Signal-Android ("dispatch is 49% of
wall time") as the justification for this cluster. That number is
stale in two ways:

1. The current `benchmarks/2026-04-09.md` shows Signal-Android taking
   **12.71s** cold start with 112 rules — so the total wall-time
   pie has grown ~5× since the original capture.
2. The current dispatch share of that wall time has not been
   measured. It could be any fraction.

Without fresh numbers we have no way to tell how much of the original
win we've already captured, how much is left, or whether the Phase 4
memory cleanup will register on actual runs.

**Actions:**

- Run `krit --perf` against Signal-Android on current main. Capture
  per-phase timing (`DispatchWalkMs`, `DispatchRuleNs`,
  `SuppressionIndexMs`, `AggregateCollectNs`, `AggregateFinalizeMs`,
  `LineRuleMs`, `LegacyRuleMs`, `SuppressionFilterMs`). `RunStats`
  already tracks all of these.
- Drop the result in `benchmarks/<today>.md` alongside the existing
  `2026-04-09.md` so the progression is visible.
- Update `roadmap/68-flat-tree-migration.md` to cite the fresh
  numbers and caveat the original 49% claim as a pre-migration
  baseline.
- After Track D lands (Tree dropped, compat helpers gone), run the
  perf capture again and record the delta. If the delta is small,
  the cluster's real win was the first batch of rule migrations and
  we've already collected it; documenting that honestly is as
  useful as finding a bigger win.

**Done criteria for Track E:**

- `benchmarks/` has at least one fresh dated file capturing the
  current state.
- Parent roadmap's perf numbers reflect reality, not the 2025-era
  baseline.

## Parallelism and ordering

```
Track A ✅ done
                              ┌─────────────────────────┐
Track B   B-decl ──┐          │                         │
          B-resolve┴── B-scopes ──── B-parallel         │
                              │                         │
Track C (scanner compat) ─────┤ (runs alongside B)     │
                              │                         │
Track F (compose conversion) ─┘                         │
(16 node-era rule sketches → flat + registered)         │
                                                        ▼
                                             Track D (Phase 4 closure)

Track E (verification) ── independent; run now and after Track D
```

- A is done on the live-surface criterion (2026-04-14).
- B's API shape is locked; declarations + resolve can run in parallel,
  then scopes, then parallel.go.
- C can begin as soon as its call-site count drops to zero for any
  given helper — it's a continuous cleanup alongside B.
- F is the Compose flat-conversion sweep (16 unlanded rules converted
  from their node-era sketches to flat helpers and registered in
  `compose.go`). Per the 2026-04-14 decision, this replaces the
  earlier "ship or cancel the archive" gate. See
  [`../compose/`](../compose/) for the rule list.
- D blocks on B + C + F and is the last thing to land. Once F is
  complete the archive can be deleted wholesale.
- E has no dependencies; run it now for a current baseline and
  again after D for the delta.

## Done criteria for the whole cluster

- Tracks A + B + C + D + E + F all pass their individual done criteria.
- No live runtime path in `internal/rules`, `internal/scanner`, or
  `internal/typeinfer` depends on `*sitter.Node` for a Kotlin file
  that was parsed via `scanner.ParseFile`.
- `scanner.File.Tree` is gone; `flattenTree` is the only place that
  touches the C sitter tree, and it does so once per file.
- The cluster's benchmark story is honest: either we have a real
  measured win vs the 2025 baseline, or we have a clearly
  documented "the win was already captured before we started
  measuring, and the cleanup is justified on API hygiene and memory
  grounds."

## Historical context

The original plan split the work into four phases (dispatcher
dual-mode, flat helpers, per-file rule migration in four batches,
drop cgo fallback). Phases 1 and 2 shipped. Phase 3 shipped in bulk
for 13 of the 42 originally-listed rule files and the remaining tail
is Track A above. Phase 4 is Track D. The per-file "migrate-*.md"
concept files from Phase 3 were deleted during this refresh because
they were 25-line boilerplate stubs with no per-file specifics, and
the state they tracked changes too fast for file-per-rule docs to be
worth maintaining — the current per-file table in Track A replaces
them.

Also superseded: the original parent roadmap's `42 rule files to
migrate (996 cgo call sites)` table. That table reflected the
2025-era starting state; the Track A table reflects the current
2026-04-13 state.

## Out of scope

- **XML parsing** in `internal/android/xmlast.go`. Separate
  tree-sitter-based subsystem; not covered by this cluster.
- **Field-name lookup support** for the Kotlin grammar. Postponed
  per [`roadmap/postponed/flat-field-names.md`](../../postponed/flat-field-names.md)
  — blocked on the vendored grammar providing `FIELD_COUNT = 0`
  and no existing rule needing the feature.

## Links

- Parent: [`roadmap/68-flat-tree-migration.md`](../../68-flat-tree-migration.md)
- Track B sub-plans:
  - [`migrate-typeinfer-api.md`](migrate-typeinfer-api.md)
  - [`migrate-typeinfer-declarations.md`](migrate-typeinfer-declarations.md)
  - [`migrate-typeinfer-resolve.md`](migrate-typeinfer-resolve.md)
  - [`migrate-typeinfer-scopes.md`](migrate-typeinfer-scopes.md)
  - [`migrate-typeinfer-parallel-indexing.md`](migrate-typeinfer-parallel-indexing.md)
- Track C sub-plan: [`scanner-compat-query-cleanup.md`](scanner-compat-query-cleanup.md)
- Track D sub-plan: [`drop-cgo-fallback.md`](drop-cgo-fallback.md)
- Downstream cleanup: [`residual-rule-helper-cleanup.md`](residual-rule-helper-cleanup.md)
- Postponed: [`roadmap/postponed/flat-field-names.md`](../../postponed/flat-field-names.md)
- Benchmarks: `benchmarks/2026-04-09.md`
