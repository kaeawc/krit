package pipeline

import (
	"context"

	"github.com/kaeawc/krit/internal/javafacts"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/scanner"
)

// DaemonCaches groups the function-typed callback hooks the daemon
// passes through ProjectHostState. They are all optional — CLI callers
// leave them nil and pay the cold-path cost. The grouping is a naming
// cleanup only: ProjectHostState embeds DaemonCaches anonymously, so
// existing reads (host.CodeIndexSnapshotLoader, etc.) still compile via
// Go field promotion. Only struct-literal initializers that previously
// set these fields directly on ProjectHostState now wrap them in
// DaemonCaches{...}.
type DaemonCaches struct {
	// JavaSemanticFactsLoader lazily builds JavaSemanticFacts after parse
	// when active Java rules request compiler-backed facts.
	JavaSemanticFactsLoader func(context.Context, []string, []*scanner.File, *librarymodel.Facts, perf.Tracker) (*javafacts.Facts, string, error)

	// CodeIndexSnapshotLoader returns the daemon-resident prior CodeIndex
	// and its meta, surviving CodeIndexCache invalidations. Used by
	// runCodeIndexBuild to reuse the in-memory prior across .kt edits.
	// *WorkspaceState.LoadCodeIndexSnapshot satisfies the shape.
	CodeIndexSnapshotLoader func() (*scanner.CodeIndex, scanner.CrossFileCacheMeta, bool)
	// CodeIndexSnapshotSaver records the just-built CodeIndex and meta
	// as the next daemon-resident snapshot.
	// *WorkspaceState.StoreCodeIndexSnapshot satisfies the shape.
	CodeIndexSnapshotSaver func(*scanner.CodeIndex, scanner.CrossFileCacheMeta)

	// JavaSourceIndexCache lets CrossFilePhase short-circuit the
	// ~100 ms content-hash key SourceIndexForFiles otherwise computes
	// on every warm call. *WorkspaceState.JavaSourceIndex satisfies it.
	JavaSourceIndexCache func(build func() *javafacts.SourceIndex) *javafacts.SourceIndex

	// ResolverFingerprintCache returns a cached resolverFingerprint
	// instead of recomputing it. The fingerprint hashes every Kotlin
	// file's content. *WorkspaceState.ResolverFingerprint satisfies it.
	ResolverFingerprintCache func(build func() string) string

	// GradleFindingsCache memoizes per-gradle-file rule-dispatch findings
	// across analyzes. Daemon callers wire WorkspaceState.GradleFindings;
	// CLI passes nil (on-disk AndroidCacheWriter covers that case).
	GradleFindingsCache func(key string, build func() scanner.FindingColumns) scanner.FindingColumns

	// BundleStatsClean / MarkBundleStatsClean are the daemon's
	// watcher-gated cache for the manifest fileStatsMatch sweep. When
	// BundleStatsClean(key) returns true, preparseBundleFingerprintTracked
	// skips the os.Stat sweep. Either both nil or both set together.
	BundleStatsClean     func(bundleKey string) bool
	MarkBundleStatsClean func(bundleKey string, version uint64)

	// SourceMTimeVersion returns the watcher's current version counter.
	// nil behaves like a constant 0 (no caching).
	SourceMTimeVersion func() uint64

	// BundleOutput / StoreBundleOutput are the daemon-side cache for
	// pre-formatted bundle-hit JSON. *WorkspaceState satisfies both.
	BundleOutput      func(bundleKey string) *CachedBundleOutput
	StoreBundleOutput func(bundleKey string, output *CachedBundleOutput)

	// ResidentBundle / StoreResidentBundle are the daemon-side
	// in-memory mirror of the on-disk FindingsBundleStore. Hit by
	// previewPostParseBundleHit before consulting the disk store to
	// skip the ~90 ms zstd+gob decode when the same bundle was
	// already loaded earlier in this daemon session. Bounded to a
	// small number of entries (residentBundleCapacity) so daemon
	// memory growth stays predictable. *WorkspaceState satisfies
	// both.
	ResidentBundle      func(bundleKey string) *scanner.FindingColumns
	StoreResidentBundle func(bundleKey string, cols *scanner.FindingColumns)

	// BackgroundSave runs fn on the daemon's single background-save
	// worker, taking the ~300 ms findings-bundle + delta-manifest disk
	// writes off the warm analyze critical path. The resident/in-memory
	// mirrors are updated synchronously before this is called, so a
	// not-yet-flushed disk write only costs a recompute, never a stale
	// read. nil in CLI mode (callers run fn inline instead). Flushed on
	// daemon shutdown. *WorkspaceState.EnqueueBackgroundSave satisfies it.
	BackgroundSave func(fn func())
}
