# Oracle Freshness Gate

Reference for the warm-path KAA cache invalidation model used by Krit's
single-pass pipeline. Read this before changing `IndexPhase.Run`,
`FindingsBundleManifest`, or any oracle short-circuit code.

## The Problem

The oracle (`tools/krit-types/`) runs the Kotlin Analysis API on the JVM and
emits a `types.json` snapshot consumed by `NeedsOracle` / `NeedsTypeInfo` rules.
Invoking it is expensive — multiple seconds even on small modules — so warm
runs lazy-load the cached `types.json` and skip the JVM call entirely.

Pre-gate behavior had a silent correctness bug: any cached `types.json` was
served verbatim, even after `.kt` files had been edited. Rules that depended
on resolved type signatures would happily report findings against stale facts,
including against deleted or renamed declarations, until the user manually
cleared the cache.

## v2: Stat-Diff Freshness Gate

`runAutoDetectOracle` (`internal/pipeline/index.go` ~line 1014) and
`runDaemonOracle` (~line 793) now consult `FindingsBundleManifest.FileStats`
before serving cached oracle output. Each entry is a
`{size, modTimeUnixNano}` pair captured at the prior run's save point.

Flow on a warm run:

1. Load the prior `FindingsBundleManifest` for the current
   `(repoDir, scanPaths)` key.
2. `StaleOracleCandidates` walks the current `.kt` working set and returns
   every file whose on-disk stat differs from the manifest entry.
3. If the set is empty, serve cached `types.json` as before
   (`freshnessGateFresh` perf event).
4. Otherwise route the stale set through `oracle.InvokeCachedWithOptions`
   with the partial-reanalyze options, or through the daemon's
   `analyzeFiles` command if a resident KAA session is up
   (`freshnessGateStale` perf event, count = stale set size). The rest of
   the cache is preserved and merged.

Manifest version was bumped so a Krit upgrade doesn't read pre-gate
manifests. Missing `FileStats` is treated as "no prior state" — full
reanalyze.

## v3: ABI-Hash + Transitive Promotion

Stat-diff alone underinvalidates: when file `A.kt` changes its public
surface, any file that references `A`'s symbols also needs reanalysis even
though that file's own stat is unchanged. v3 closes this gap.

Each save now persists per-file `arch.HashAbiSignatures` values into
`FindingsBundleManifest.AbiHashes`. The integration step in `IndexPhase.Run`
runs after the stat-diff candidate set is computed:

1. For each stat-diff candidate, recompute the current ABI hash.
2. If the hash equals the manifest value, the change was body-only — the
   file goes through the partial reanalyze but no dependents are added.
3. If the hash differs, call
   `idx.TransitiveDependents(names, excludeFile)` on the cross-file
   `CodeIndex` to find every file that textually references the changed
   identifiers, and add those to the stale set.
4. Promoted dependents are reanalyzed in the same partial-oracle call.

Ordering requirement: `IndexPhase.Run` builds the cross-file `CodeIndex`
*before* `runOracle` so the transitive lookup is available during the
freshness gate. Do not reorder these phases.

## Control Surfaces

- `--no-cache-oracle` — disables the on-disk cache entirely, forces a full
  oracle invocation. Use during rule development when oracle output format
  itself changes.
- Delete `.krit/types.json` — drops the cache so the next warm run is
  effectively cold. Cheaper than `--no-cache-oracle` when you only want to
  reset once.

## Code Locations

- `internal/pipeline/index.go` — `runOracle`, `runAutoDetectOracle`,
  `runDaemonOracle`, and the integration step that consumes
  `StaleOracleCandidates` and `TransitiveDependents`.
- `internal/scanner/findings_bundle_manifest.go` — `FindingsBundleManifest`
  struct (`FileStats`, `AbiHashes`), `StaleOracleCandidates` helper, save
  and load paths.
- `internal/oracle/` — `InvokeCachedWithOptions` and the partial-reanalyze
  options consumed by both autodetect and daemon paths.

## Perf Events

- `freshnessGateFresh` — manifest matched the working set; cached
  `types.json` served unchanged.
- `freshnessGateStale` — at least one file was reanalyzed. The event count
  is the post-promotion stale set size; pair with cold-vs-warm runs to spot
  over- or under-invalidation.
