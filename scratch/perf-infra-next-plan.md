# Perf / Core Infra — Next Plan

Snapshot date: 2026-04-21, after the `#292 … #310` burst landed (PRs
#311–#330). Captures what's left on the floor, what each proposed
issue is for, how they depend on each other, and the order to work
them in.

This document is the working brief for filing the N-issues below and
for re-reading in a week to check progress has stayed on the spine.

## Landed context

Closed this week (in merge order): #292 CacheInfraConsolidation (PR
#311) • #293 RuleInventoryPyDeadCodeSweep (PR #312) • #295
RuleInventoryJSONGeneratedFilesTracking (PR #313) • #296
V2DispatcherVerboseSkipDiagnostic (PR #314) • #303
GoroutineSafeFindingCollector (PR #315) • #297
LSPTypeAwareRuleIntegrationTest (PR #316) • #298
ScenarioBFindingDeltaInvestigation (PR #317) • #305
SharedContentHashMemo (PR #318) • #294 NeedsResolverOracleLinterGate
(PR #319) • #301 CrossRuleHotspotsKotlinCorpus (PR #320 +
disposition PR #323, closed 2026-04-21) • #299
ParseCacheMinFileSizeBenchmark (PR #321) • #310 FasterContentHash
(PR #322) • #302 NeedsResolverNeedsOracleUnification (PR #324) •
#306 NarrowAllFilesOracleRules (PR #325) • #304 CrossRuleConcurrency
(PR #326) • #308 JavaParseAndIndexCache (PR #327) • #309
CrossFileLookupMapBloomCache (PR #328) • #300 ParseCacheDiskCapLRU
(PR #329) • #307 CrossFileCachePhase2Incremental (PR #330).

Primitives now in the tree:

- `internal/fsutil/atomic.go` — atomic write (tempfile + fsync + rename).
- `internal/hashutil/{hash,memo}.go` — xxh3-256 content hasher with
  per-run memo keyed by `(path, size, mtime)`.
- `internal/cacheutil/{registry,lru,sharded,versioned_dir}.go` —
  `Registered` interface, `Register` / `ClearAll` / `AllRegistered`,
  on-disk LRU, sharded path helper, version-gated directory helper.
- `internal/scanner/parse_cache.go` — Kotlin + Java sibling subtrees
  sharing the `FlatNode` gob serializer; per-language grammar-version
  sidecars; LRU-capped.
- `internal/scanner/index_cache.go` / `index_shard.go` — whole-index
  cache plus per-file shards; lookup maps + bloom persisted
  monolithically, rebuilt on aggregate path on single-file edits.
- `internal/oracle/filter.go` — `NeedsOracle` + identifier/file-filter
  narrowing for Deprecation / IgnoredReturnValue / UnreachableCode,
  with `OracleFilterSummary.Fingerprint` emission.
- `internal/ruleslinter` — compile-time gate that rules declaring
  `NeedsResolver` / `NeedsOracle` / `NeedsTypeInfo` actually call the
  matching backend (and vice versa).
- `internal/rules/v2/rule.go` + `internal/pipeline/crossfile.go` —
  `NeedsConcurrent` capability plumbed through cross-file rule
  execution.

The prior backlog N1–N10 from the 2026-04-20 chat has been re-evaluated
against this tree. N2 (CacheSetRegistry), N6 (WorkerLocalCollector
ContextAPI), N9 (UnifiedParseCacheSerializer Kotlin+Java) and
#301 (CrossRuleHotspotsKotlinCorpus) are retired. N1
(NoNameShadowingScopeIndex) is parked per the disposition in
`rule-hotspots-and-shared-indexes.md`: 1.6s warm on kotlin/kotlin is
below the threshold that would justify re-attempting after the
prior finding-collapse incident. The live set is below.

## Issues to file

Seven items. Four are follow-ups on work that shipped the scaffolding
but not the acceptance criterion; three are new, uncovered by reading
the recently-merged PR bodies and the in-tree code.

### N3 — UnifiedCacheStatsReport

**Cluster:** core-infra · **Est.:** 1–2 days

**What.** Extend `cacheutil.Registered` with an optional `Stats()
CacheStats` method that every registered subsystem (parse
Kotlin/Java, cross-file index, cross-file shards, lookup+bloom,
oracle) implements. `--perf` emits one `caches` section that
aggregates them; `--verbose` prints a one-line-per-cache summary.

```go
type CacheStats struct {
    Entries   int
    Bytes     int64
    Hits      int64
    Misses    int64
    Evictions int64
    LastWriteUnix int64
}
```

**Why now.** We have five on-disk caches after this week's burst and
the pipeline's single `cache.CacheStats` struct ([internal/cache/cache.go:97](internal/cache/cache.go)) only
covers the incremental-analysis cache. Every other cache prints ad
hoc counters (e.g. `internal/oracle/cache.go:691 CacheStats` is a
one-off, parse cache has its own, shards print from `index_shard.go`).
Without unified stats we can't answer "which cache ate the LRU cap"
— the question CacheBudgetAttribution (N13) depends on.

**Expectations.**

- All five caches register and implement `Stats()`.
- `--perf` JSON grows a `caches` array; existing top-level
  `cacheStats` stays for back-compat.
- Hit / miss / eviction counters maintained on the hot path without
  a lock per lookup (atomic int64 is fine).
- `go test -race ./... -count=1` clean.

**Validate.**

- Populate each cache, read the registry, assert
  `Entries`/`Bytes`/`Hits` match expected.
- `--perf` output on Signal-Android surfaces all five caches with
  non-zero hit rates on the warm run.

**Risks.**

- `Stats()` on a 200MB cache should not walk the disk every call —
  maintain running counters, probe disk only in a dedicated `Probe()`
  call that `--verbose` triggers.
- Counter drift across process restarts is fine (we don't need to
  persist hit/miss across runs) but `Entries` / `Bytes` must match
  the actual on-disk state on first `Stats()` call after load.

**Dependencies.**

- Blocked by: none.
- Blocks: N13 CacheBudgetAttribution.

### N4 — OracleFingerprintCIGate

**Cluster:** core-infra · **Est.:** 1–2 days

**What.** CI check that records the oracle filter input-set
fingerprint (`OracleFilterSummary.Fingerprint`, emitted at
[internal/oracle/filter.go:58](internal/oracle/filter.go)) against a checked-in baseline per
(repo × rule-set) and fails the build if it shifts without an
accompanying baseline update.

**Why now.** PR #317 landed the fingerprint emission and
`scratch/oracle-narrowing-scenario-b-delta.md` explicitly names this
as the "minimal, language-neutral signal the issue's Opportunities
section asks for". The regression fixture called out in #298 is
deferred because it requires a live `krit-types.jar` in CI; the
fingerprint gate is the low-cost proxy.

**Expectations.**

- `.krit/oracle-fingerprints.json` per test corpus checked in.
- Script under `tools/` that runs `krit --perf` against the test
  playgrounds, extracts `typeOracle/filterFingerprint/<hex>`,
  compares to the baseline.
- GitHub Actions workflow step that runs the script and fails
  non-zero on drift.
- Failure message names the drifted (repo, rule-set, old-hex,
  new-hex) tuple and points to the baseline-update command.

**Validate.**

- Intentional narrowing change to one of the three `AllFiles: true`
  rules makes the gate fire with a readable diff.
- A no-op refactor to unrelated code leaves the fingerprint stable.
- Baseline update is a single CLI command, not a hand-edit.

**Risks.**

- Playground corpus drift (someone adds a `.kt` file to
  `playground/`) triggers the gate harmlessly. Mitigate by
  fingerprinting only the OracleFilterSummary output, not the full
  file list — the fingerprint is already over the narrowed set, so
  a new file that doesn't match the identifier filter is invisible.
- Non-determinism from file-system walk order (filter.go sorts
  before hashing — confirm stays sorted).

**Dependencies.**

- Blocked by: none.

### N5 — NeedsConcurrentLinterGate

**Cluster:** core-infra · **Est.:** <1 day

**What.** Extend `internal/ruleslinter` (from PR #319) with a check
that flags a rule using concurrent state (goroutine spawning, shared
slice mutation across workers, `sync.WaitGroup` usage inside
`CheckNode`) without declaring `NeedsConcurrent` in `Meta()`, and
vice versa — a rule that declares `NeedsConcurrent` but never
needs it.

**Why now.** PR #326 shipped the capability and the cross-file
concurrent path; lint keeps future rules honest. Same pattern as
#294's NeedsResolver/Oracle gate.

**Expectations.**

- Lint failure message: `rule X uses concurrent finding collector
  but does not declare NeedsConcurrent in Meta()`.
- Runs under `go vet ./...` or as a CI step alongside the existing
  ruleslinter.
- All current rules pass on introduction.

**Validate.**

- Strip `NeedsConcurrent` from a concurrent rule's Meta() — lint
  fails.
- Add `NeedsConcurrent` to a rule that never touches shared state —
  lint fails with "declared but unused".
- Existing CI stays green.

**Risks.**

- Detecting "uses concurrent state" structurally is squishy. Start
  narrow: flag `MergeCollectors` callers and rules whose file-level
  analysis scheme is ParallelPerFile. Accept false negatives over
  false positives.

**Dependencies.**

- Blocked by: none.

### N7 — TypeInfoBackendPreferenceHint

**Cluster:** core-infra · **Est.:** 2–3 days

**What.** After #324 unified `NeedsResolver` / `NeedsOracle` under
`NeedsTypeInfo`, a rule that prefers one backend has no way to say
so. Add a `TypeInfoPreference` field to the capability:

```go
type NeedsTypeInfo struct {
    PreferBackend TypeInfoBackend  // PreferAny | PreferResolver | PreferOracle
    Required      bool             // false → skip silently if backend unavailable
}
```

When both backends are configured, the dispatcher honors the hint;
otherwise it falls through to whatever's wired.

**Why now.** The unification in #324 collapsed two flags into one for
migration reasons; the per-rule routing question was explicitly
deferred. Concrete use case: rules that only need type hierarchy
lookups should prefer the in-process resolver (cheaper); rules that
need full FQN resolution (call targets, overload resolution) should
prefer the oracle.

**Expectations.**

- `NeedsTypeInfo` struct extended, backward-compatible default is
  `PreferAny`.
- Dispatcher respects the hint when both backends are available.
- Documented in `roadmap/type-resolution-service.md` with the
  decision matrix.

**Validate.**

- `go test ./internal/dispatcher/ -count=1` green.
- Rule declaring `PreferResolver` runs against the resolver even
  when the oracle is also configured.
- Backwards compat: existing rules (no preference) behave identically.

**Risks.**

- Rule authors guess wrong and force the expensive backend. Mitigate
  with a comment template that names the backend's cost and what
  kind of lookup it's good at.
- Metadata descriptor churn. Keep the field optional so existing
  descriptor files stay valid.

**Dependencies.**

- Blocked by: none (#324 merged).

### N11 — PerShardBloomUnion

**Cluster:** perf-infra · **Est.:** 2–3 days, **contingent on
measurement**

**What.** PR #330 shards per-file contributions but still rebuilds
the monolithic lookup maps + bloom filter on the aggregate path on
every miss. Per-shard blooms unioned at warm-load time would make
the single-file-edit rebuild O(shards-touched) instead of O(all-
shards).

**Why now.** Only if warranted. Run `--perf` first on a single-
file-edit scenario against main with the full #330 path. If the
aggregate rebuild (lookup maps + bloom) dominates (>100ms warm),
file the issue. If not, park.

**Expectations (if filed).**

- Per-shard `bloom.MarshalBinary` stored alongside the shard.
- Warm-load path unions blooms across shards instead of rebuilding.
- False-positive rate documented and within the union-of-N-blooms
  band.

**Validate.**

- Round-trip: per-shard-save → union-load → same MayContain answers
  on a known corpus.
- Single-file-edit microbenchmark shows the expected wall-time drop.
- FPR measured at Signal and kotlin/kotlin scale, documented.

**Risks.**

- Union bloom FPR rises with shard count. At 18k shards (kotlin/
  kotlin) a naive union has high FPR; may need tiered blooms or
  mini-blooms-by-prefix.
- Shard format bump; version the shard payload independently of
  the monolithic cache.

**Dependencies.**

- Gated by a measurement run (see priority order).
- Related: N3 UnifiedCacheStatsReport helps the measurement.

### N12 — XMLParseCacheExtension

**Cluster:** perf-infra · **Est.:** 2 days

**What.** Extend the #308 parse-cache pattern (sibling subtrees under
`.krit/parse-cache/`) to XML files. `loadXMLFilesForCache` in #330
parallelized the walk but still re-parses every XML on every run.

**Why now.** Pattern is proven for Java. Cost is not currently
known; measure XML parse time in the `javaIndexing` phase first and
file if it's material.

**Expectations.**

- `.krit/parse-cache/xml/{entries,…}` subtree analogous to
  kotlin/java, sharing the serializer.
- `xmlIndexing` phase consults the cache.
- LRU / `--clear-cache` covers XML automatically via the registry.

**Validate.**

- Warm XML parse phase reduced by >=80%.
- Grammar-version sidecar for tree-sitter-xml; bump invalidates.
- Finding-equivalence preserved on a repo with meaningful XML usage
  (any Android app).

**Risks.**

- Tree-sitter-xml's FlatNode shape may differ — measure before
  committing to the shared serializer.
- If XML parse cost is trivial, this is wasted motion. Measure
  first.

**Dependencies.**

- Measurement-gated.

### N13 — CacheBudgetAttribution

**Cluster:** core-infra · **Est.:** 1 day (on top of N3)

**What.** Per-cache slice-of-cap visibility. When the LRU evicts,
we want to know which of the five caches used which fraction of
the global 200MB cap before we start raising/lowering per-cache
caps or introducing per-cache budgets.

`--perf` and `--verbose` grow a `cacheBudget` section:

```json
{
  "cacheBudget": {
    "capBytes": 209715200,
    "usedBytes": 187654321,
    "perCache": [
      {"name": "parse-cache/kotlin", "bytes": 92000000, "pctOfCap": 0.44},
      {"name": "parse-cache/java",   "bytes": 31000000, "pctOfCap": 0.15},
      ...
    ]
  }
}
```

**Why now.** PR #328 called out "+5MB at Signal scale" from the
lookup-bloom persistence amplifying #300's cap concern. Without
attribution we can't raise or tier the cap by evidence.

**Expectations.**

- Attribution derived from `cacheutil.Registered.Stats().Bytes`.
- No new on-disk state — attribution is an aggregate view of what
  the registry already knows.

**Validate.**

- Populate caches with known sizes; attribution sums match.
- Rows sort descending by `bytes` so the biggest offender is visible
  first.

**Risks.**

- Attribution drift under concurrent mutation. Take a snapshot
  under the registry lock, accept minor staleness.

**Dependencies.**

- Blocked by: N3 UnifiedCacheStatsReport.

## Dependency graph

```
  [cacheutil.Registered — merged]
        │
        ├─► N3 UnifiedCacheStatsReport ──┬─► N13 CacheBudgetAttribution
        │                                │
        │                                └─► (enables N11 measurement)
        │
        ├─► N11 PerShardBloomUnion            ← gate on measurement
        │
        └─► N12 XMLParseCacheExtension        ← gate on measurement

  [PR #317 fingerprint — merged]
        │
        └─► N4 OracleFingerprintCIGate

  [PR #319 linter — merged] + [#326 NeedsConcurrent — merged]
        │
        └─► N5 NeedsConcurrentLinterGate

  [#324 NeedsTypeInfo — merged]
        │
        └─► N7 TypeInfoBackendPreferenceHint

  (parked) N1 NoNameShadowingScopeIndex
```

## Priority order

1. **N5 NeedsConcurrentLinterGate** (<1d). Lock in capability-
   declaration discipline while #326 is fresh. No dependencies.
2. **N3 UnifiedCacheStatsReport** (1–2d). Prerequisite for
   measuring N11 and for N13. Turns the next cache / oracle
   narrowing conversation from guess to evidence.
3. **N4 OracleFingerprintCIGate** (1–2d). Shuts the door on silent
   Scenario-B-style deltas. The infra is already emitting — just
   wire the gate.
4. **Measurement run** against main on single-file-edit (cross-
   file phase2) and XML-heavy repo. Outputs decide N11 and N12.
5. **N13 CacheBudgetAttribution** (1d after N3). Closes the loop on
   #300's global cap.
6. **N11 PerShardBloomUnion** (2–3d). Only if step 4 says the
   monolithic rebuild still dominates.
7. **N12 XMLParseCacheExtension** (2d). Only if step 4 says XML
   parse cost is material.
8. **N7 TypeInfoBackendPreferenceHint** (2–3d). Ergonomics / routing
   cleanup, not on any critical path.

## Out of scope for this plan

- #119 DidChangeOracleRefresh, #143 Daemon Leyden AOT Phase 4, #144
  Full String Interning, #148 DistributionReadiness — tracked
  separately; not perf/core-infra continuation.
- New rule issues (#204–#262). Unrelated cluster.
- N1 NoNameShadowingScopeIndex — parked per scratch disposition.
  Revisit if the rule's warm cost on kotlin/kotlin rises above
  ~3s or if a separate scope-analysis consumer appears.

## Revisit cadence

Re-read this doc after any N lands, and after any profiling run in
step 4. Retire issues as they close; update the graph when a gate
measurement falsifies a planned item.
