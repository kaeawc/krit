# Core infrastructure cluster

Architectural changes that would yield the largest long-term leverage on
maintainability, consistency, and performance — surfaced by a greenfield
analysis of the current codebase. Each item targets a specific seam where
organic growth has left two or more divergent code paths that must be kept
in sync.

No single parent overview doc — derived from the 2026-04 greenfield
architecture survey.

These items are not about adding rules. They are about changing the
substrate so that future rules, entry points, and features can be written
once rather than adapted for each code path.

## Status (2026-04-16)

**4 of 13 shipped.** The keystone (UnifiedRuleInterface) landed in commit
`6157298`, unblocking every dependent item below. CodegenRegistry
shipped across 8 commits (`2a162e5` → `122e9b6`), which unblocks
`cache-unification`.

## Dependency graph

```
Foundation                Pipeline              Finding path
───────────               ─────────             ────────────
UnifiedRuleInterface ✅ ──┬── PhasePipeline ⏳ ──┬── FindingRepresentationUnification ⏳
                         │   (main.go → 6      │   (delete []Finding path)
                         │    named phases)    │
                         └── UnifiedFileModel ⏳
                             (ParsedFile +
                              Language tag)

Type / suppression        Shared protocol       Error handling
──────────────────        ───────────────       ──────────────
TypeResolutionService ⏳   SharedJsonRpcLayer ✅  ErrorHandlingStandardization ✅
  │                                             InitTuiSplit ✅
  └── OracleFilterInversion ⏳

SuppressionMiddleware ⏳ (depends on PhasePipeline)

Registry / caching
──────────────────
CodegenRegistry ✅
  └── CacheUnification ⏳ (depends on CodegenRegistry for rule version hashes — now unblocked)
```

## Shipped (4)

- [`unified-rule-interface.md`](unified-rule-interface.md) ✅ —
  rule execution is fully on `v2.Rule`; V2Dispatcher handles per-file
  dispatch and registry invariants reject duplicate IDs or non-executable
  registrations
- [`error-handling-standardization.md`](error-handling-standardization.md) ✅ —
  panics in formatters replaced with errors, structured `DispatchError`
  collection, leveled logging in LSP/MCP
- [`init-tui-split.md`](init-tui-split.md) ✅ — 2,104-line `init.go`
  split into 9 focused files
- [`shared-jsonrpc-layer.md`](shared-jsonrpc-layer.md) ✅ —
  `internal/jsonrpc/` shared Content-Length framing between LSP and MCP
- [`codegen-registry.md`](codegen-registry.md) ✅ —
  68 per-rule `init()` bodies, ~370-line `applyRuleConfig()` switch,
  and hand-maintained `DefaultInactive` map replaced by a generated
  `Meta()` registry; 628 rules, 66 generated files, 4 hand-written
  overrides for exotic config

## Next up — unblocked by UnifiedRuleInterface

### Pipeline

- [`phase-pipeline.md`](phase-pipeline.md) — rewrite the 2,342-line
  `cmd/krit/main.go` as explicit phases shared by CLI, LSP, and MCP.
  **Highest impact remaining.**
- [`unified-file-model.md`](unified-file-model.md) — one `ParsedFile`
  type with a language tag; Android manifest/resource/Gradle rules flow
  through the main dispatcher

### Finding path

- [`finding-representation-unification.md`](finding-representation-unification.md) —
  commit to the columnar representation everywhere; delete the
  `[]Finding` struct path

### Type / suppression

- [`type-resolution-service.md`](type-resolution-service.md) — pass
  the type resolver through `Context` instead of manual setter
  injection; oracle becomes a pluggable backend
- [`oracle-filter-inversion.md`](oracle-filter-inversion.md) — flip the
  oracle-filter default: rules must opt *in* to oracle calls, not opt
  out. Depends on type-resolution-service
- [`suppression-middleware.md`](suppression-middleware.md) — merge
  annotation-based and baseline suppression into one filter applied once
  per file. Depends on phase-pipeline

### Registry / caching

- [`cache-unification.md`](cache-unification.md) — replace four
  independent cache layers with one content-hash-keyed store. Depends on
  codegen-registry (✅ done, now actionable)
