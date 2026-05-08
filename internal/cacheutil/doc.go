// Package cacheutil is the shared infrastructure layer for every
// on-disk cache in krit. It centralizes the disk-cache architecture so
// individual subsystems (parse trees, XML, resource indexes, FIR
// findings, library-model profiles, findings bundles, cross-findings,
// android findings, the analysis cache) reuse one coherent set of
// primitives instead of growing bespoke equivalents.
//
// # The disk-cache umbrella
//
// Every persistent on-disk cache in the project should:
//
//   - Implement Backend (Registered + StatsProvider) so it appears in
//     AllRegistered/AllStats and can be cleared by the CLI's --clear-caches
//     path. Caches that intentionally have no per-entry stats may
//     implement Registered alone, but Backend is the default contract.
//   - Use SizeCapLRU for size-bounded entries.
//   - Use AsyncWriter for off-hot-path persistence.
//   - Use VersionedDir to invalidate on schema/grammar changes.
//   - Use ShardedEntryPath for the on-disk layout
//     ({root}/{hash[:2]}/{hash[2:]}{ext}).
//   - Use EncodeZstdGob/DecodeZstdGob for the wire format unless a
//     domain-specific format is required.
//
// New persistent caches MUST register here at init() so the global
// stats/clear/budget reporting picks them up automatically. Caches that
// memoize per-run derived facts (per-file imports, per-node summaries,
// etc.) belong in internal/filefacts/ instead — that is the in-memory
// run-scoped cache layer.
//
// The split:
//
//   - cacheutil  → primitives + global registry for persistent on-disk
//     caches.
//   - filefacts  → run-scoped per-file/per-node memoization.
//   - rule-local sync.Maps → not allowed (enforced by ruleslinter).
package cacheutil
