package pipeline

import (
	"context"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/javafacts"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// WorkspaceState is the long-lived in-memory parse cache shared by
// analysis requests in a long-running process (LSP, MCP, future
// krit-daemon). Keys are normalized file paths; values are content-hash
// gated *scanner.File entries.
//
// The map is unbounded. Callers are responsible for invalidating
// entries that are no longer relevant (the LSP server does this on
// didClose). All public methods are safe for concurrent use.
type WorkspaceState struct {
	repoRoot string

	mu     sync.RWMutex
	parsed map[string]parsedEntry
	// dirty tracks paths that have been invalidated since the last
	// DrainDirty call. Populated by Touch (intended to be called
	// alongside Invalidate from the file watcher); drained by daemon
	// verbs that report "files changed since last analyze" stats.
	// Guarded by mu so the dirty-set is consistent with the parsed
	// cache snapshot a verb sees.
	dirty map[string]struct{}

	hits   atomic.Int64
	misses atomic.Int64

	// Cross-file warm state. Each slot holds at most one value plus
	// the fingerprint it was built under; a request with a different
	// fingerprint discards the prior value and rebuilds. Slots are
	// independent so a libraryFacts rebuild doesn't drop the
	// (much more expensive) codeIndex.
	xfileMu        sync.Mutex
	libraryFacts   xfileSlot[*librarymodel.Facts]
	codeIndex      xfileSlot[*scanner.CodeIndex]
	dependents     xfileSlot[*scanner.DependentsIndex]
	resolver       xfileSlot[typeinfer.TypeResolver]
	oracleFilter   xfileSlot[*oracle.CallTargetFilterSummary]
	androidProject xfileSlot[*android.Project]

	gradleMu       sync.Mutex
	gradleFindings map[string]scanner.FindingColumns

	// sourceMTimeVersion is bumped on every source-path watcher event
	// (Kotlin / Java edits, Gradle / version-catalog edits — anything
	// that could shift a file's stat tuple). Daemon callers compare
	// the bundle-stats-clean memo against this version to decide
	// whether they can skip the 18k-file `os.Stat` sweep at the top
	// of preparseBundleFingerprint.
	sourceMTimeVersion atomic.Uint64

	statsCleanMu sync.Mutex
	// bundleStatsClean maps manifestKey → sourceMTimeVersion at which
	// fileStatsMatch last succeeded for that key. A later call with
	// the same key and the same version can skip the stat sweep
	// entirely. Entries are never explicitly removed — a stale entry
	// just fails the version check on the next call.
	bundleStatsClean map[string]uint64

	bundleOutputMu sync.Mutex
	// bundleOutput caches the pre-formatted findings JSON bytes
	// produced by a successful bundle-hit serve. Keyed by the
	// FindingsBundleKey (which already encodes rules, config, source
	// set, library facts) so a different fingerprint gets a fresh
	// entry naturally — no manual invalidation needed. The cached
	// bytes are content-derived, so two consecutive bundle-hits with
	// the same key produce byte-identical findings JSON and can
	// reuse the same buffer.
	bundleOutput map[string]*CachedBundleOutput

	javaSourceMu sync.Mutex
	// javaSourceVersion is bumped on every .java watcher event. The
	// resident javaSourceIndex slot uses it to detect whether its
	// last build is still valid without re-hashing every Java file's
	// content (the SourceIndexForFiles path otherwise spends ~100 ms
	// per 2 k Java files just computing its content-hash key, even
	// on a warm cache hit).
	javaSourceVersion uint64
	// cachedJavaSourceIndex is the last successful build, valid while
	// cachedJavaSourceVersion == javaSourceVersion. A bump on the
	// watcher side rotates the version; the next lookup misses and
	// rebuilds via SourceIndexForFiles.
	cachedJavaSourceIndex   *javafacts.SourceIndex
	cachedJavaSourceVersion uint64

	codeIndexSnapshotMu sync.Mutex
	// codeIndexSnapshot is the last successfully built *CodeIndex,
	// retained across watcher invalidations of the fingerprint-keyed
	// codeIndex slot. scanner.BuildIndexCachedWithPrior consults it
	// via a host-provided loader so an overlay rebuild reuses the
	// in-memory prior instead of re-decoding the ~1M-symbol payload
	// from disk (~2.6 s on the kotlin corpus). BuildIndexIncremental
	// mutates the prior in place; per-project analyze serialization
	// keeps readers from observing a half-mutated snapshot.
	codeIndexSnapshot     *scanner.CodeIndex
	codeIndexSnapshotMeta scanner.CrossFileCacheMeta

	resolverFpMu sync.Mutex
	// cachedResolverFp is the last computed resolverFingerprint
	// string, valid while cachedResolverFpVersion ==
	// sourceMTimeVersion. Recomputing the fingerprint hashes 18 k
	// Kotlin file contents (~135 ms on the kotlin corpus), so a
	// resident memo gated on the watcher-driven source version skips
	// that work whenever no Kotlin / Java / Gradle event has fired
	// since the last successful compute. A nil receiver / empty
	// fingerprint always recomputes (no caching).
	cachedResolverFp        string
	cachedResolverFpVersion uint64
	cachedResolverFpPresent bool

	typeInfoMu sync.Mutex
	// typeInfo caches per-file *typeinfer.FileTypeInfo across analyzes.
	// The watcher's Invalidate(path) drops the corresponding entry —
	// same correctness contract as the resident parsed-trees cache.
	// On the kotlin corpus warm baseline this turns 18 k disk-cache
	// hits into 18 k map lookups: ~185 ms → ~5 ms in
	// typeIndex.perFileExtraction.
	typeInfo map[string]*typeinfer.FileTypeInfo
}

// CachedBundleOutput holds everything OutputPhase needs to assemble
// the JSON envelope on a warm bundle hit without re-formatting 87 k
// findings. findingsBytes is the pre-built compact JSON array; the
// summary fields (totals, by-ruleSet, by-rule, fixableCount) are
// derived from those findings and stable for the lifetime of the
// FindingsBundleKey.
type CachedBundleOutput struct {
	FindingsBytes []byte
	Total         int
	ByRuleSet     map[string]int
	ByRule        map[string]int
	FixableCount  int
}

// xfileSlot pairs a fingerprint with a value. Zero value means
// "no cached entry yet".
type xfileSlot[T any] struct {
	fingerprint string
	value       T
	present     bool
}

type parsedEntry struct {
	contentHash string
	file        *scanner.File
}

// NewWorkspaceState returns an empty workspace state rooted at repoRoot.
// repoRoot is informational today and reserved for future cross-file
// extension; passing "" is valid.
func NewWorkspaceState(repoRoot string) *WorkspaceState {
	return &WorkspaceState{
		repoRoot: repoRoot,
		parsed:   make(map[string]parsedEntry),
	}
}

// RepoRoot returns the repo root the state was constructed for.
func (w *WorkspaceState) RepoRoot() string { return w.repoRoot }

// ParseFile returns a parsed *scanner.File for (path, content). When a
// cached entry exists for path with a matching content hash, it's
// returned without re-parsing; otherwise ParseSingle runs and the
// result is stored. A nil receiver delegates to ParseSingle so callers
// can pass an optional cache without nil-checks.
func (w *WorkspaceState) ParseFile(ctx context.Context, path string, content []byte) (*scanner.File, error) {
	file, _, err := w.ParseFileWithHit(ctx, path, content)
	return file, err
}

// ParseFileWithHit is ParseFile that also reports whether the returned
// file came from the cache. Callers that need per-call attribution
// (logging, daemon protocol responses) should prefer this form over
// inspecting Stats(), which aggregates across all calls.
func (w *WorkspaceState) ParseFileWithHit(ctx context.Context, path string, content []byte) (*scanner.File, bool, error) {
	if w == nil {
		file, err := ParseSingle(ctx, path, content)
		return file, false, err
	}
	key := normalizeKey(path)
	hash := hashutil.Default().HashContent(key, content)

	w.mu.RLock()
	entry, ok := w.parsed[key]
	w.mu.RUnlock()
	if ok && entry.contentHash == hash {
		w.hits.Add(1)
		return entry.file, true, nil
	}

	file, err := ParseSingle(ctx, path, content)
	if err != nil {
		return nil, false, err
	}

	w.mu.Lock()
	// Re-check so a racing winner's pointer is the one every caller
	// observes — important for callers that compare *scanner.File by
	// identity. The freshly parsed file is discarded; same content,
	// same content hash.
	if existing, ok := w.parsed[key]; ok && existing.contentHash == hash {
		w.mu.Unlock()
		w.hits.Add(1)
		return existing.file, true, nil
	}
	w.parsed[key] = parsedEntry{contentHash: hash, file: file}
	w.mu.Unlock()
	w.misses.Add(1)
	return file, false, nil
}

// LookupParsedByPath returns the cached *scanner.File for path
// without re-reading or re-hashing content. The watcher's Invalidate
// hook is the correctness boundary: if the file changed and fsnotify
// missed the event, the caller will replay a stale parse — same
// best-effort contract the watcher already promises.
//
// Returns (nil, false) when no entry exists for path. Pointer-stable:
// repeated calls return the same *scanner.File until Invalidate
// drops it.
func (w *WorkspaceState) LookupParsedByPath(path string) (*scanner.File, bool) {
	if w == nil {
		return nil, false
	}
	key := normalizeKey(path)
	w.mu.RLock()
	entry, ok := w.parsed[key]
	w.mu.RUnlock()
	if !ok {
		return nil, false
	}
	return entry.file, true
}

// StoreParsed records file as the cached entry for path. content is
// hashed and stored alongside so ParseFileWithHit callers (LSP/MCP)
// can still content-gate. Idempotent: a second StoreParsed for the
// same (path, content hash) is a no-op.
func (w *WorkspaceState) StoreParsed(path string, content []byte, file *scanner.File) {
	if w == nil || file == nil {
		return
	}
	key := normalizeKey(path)
	hash := hashutil.Default().HashContent(key, content)
	w.mu.Lock()
	if existing, ok := w.parsed[key]; ok && existing.contentHash == hash {
		w.mu.Unlock()
		return
	}
	w.parsed[key] = parsedEntry{contentHash: hash, file: file}
	w.mu.Unlock()
}

// Invalidate drops the cached entry for path. Safe to call when no
// entry exists. Also drops the file's *typeinfer.FileTypeInfo —
// both caches share the watcher event source, so dropping them
// together keeps the resolver and parsed-tree views consistent.
func (w *WorkspaceState) Invalidate(path string) {
	if w == nil {
		return
	}
	key := normalizeKey(path)
	w.mu.Lock()
	delete(w.parsed, key)
	w.mu.Unlock()
	w.typeInfoMu.Lock()
	delete(w.typeInfo, key)
	w.typeInfoMu.Unlock()
}

// LookupFileTypeInfo returns the resident *typeinfer.FileTypeInfo
// for path, or (nil, false) on miss. Implements
// typeinfer.ResidentFileTypeInfoCache so the resolver's
// extractFilesParallelCached can short-circuit the disk read on
// warm runs.
func (w *WorkspaceState) LookupFileTypeInfo(path string) (*typeinfer.FileTypeInfo, bool) {
	if w == nil {
		return nil, false
	}
	key := normalizeKey(path)
	w.typeInfoMu.Lock()
	info, ok := w.typeInfo[key]
	w.typeInfoMu.Unlock()
	return info, ok
}

// StoreFileTypeInfo records info as the resident entry for path. A
// second store with the same path keeps the previously-stored
// pointer to preserve identity (rules that compare *FileTypeInfo by
// pointer need this).
func (w *WorkspaceState) StoreFileTypeInfo(path string, info *typeinfer.FileTypeInfo) {
	if w == nil || info == nil {
		return
	}
	key := normalizeKey(path)
	w.typeInfoMu.Lock()
	if w.typeInfo == nil {
		w.typeInfo = make(map[string]*typeinfer.FileTypeInfo)
	}
	if _, exists := w.typeInfo[key]; !exists {
		w.typeInfo[key] = info
	}
	w.typeInfoMu.Unlock()
}

// JavaSourceIndex returns the daemon-resident *javafacts.SourceIndex
// when its version still matches the watcher's javaSourceVersion;
// otherwise it invokes build, stores the result under the current
// version, and returns it. Caller-supplied build is what guards
// correctness (it sees the live parsed-file set); the slot just
// short-circuits the ~100 ms content-hash key SourceIndexForFiles
// pays on every warm call.
//
// nil receiver always builds (no caching) so the CLI's
// in-process path doesn't accidentally hit a stale resident slot.
func (w *WorkspaceState) JavaSourceIndex(build func() *javafacts.SourceIndex) *javafacts.SourceIndex {
	if w == nil {
		return build()
	}
	w.javaSourceMu.Lock()
	if w.cachedJavaSourceIndex != nil && w.cachedJavaSourceVersion == w.javaSourceVersion {
		v := w.cachedJavaSourceIndex
		w.javaSourceMu.Unlock()
		return v
	}
	currentVersion := w.javaSourceVersion
	w.javaSourceMu.Unlock()

	idx := build()

	w.javaSourceMu.Lock()
	// Only store when the version we observed is still current —
	// a watcher event during build() advances the counter and the
	// next call must rebuild.
	if w.javaSourceVersion == currentVersion {
		w.cachedJavaSourceIndex = idx
		w.cachedJavaSourceVersion = currentVersion
	}
	w.javaSourceMu.Unlock()
	return idx
}

// BumpJavaSourceVersion advances the version counter so the next
// JavaSourceIndex call rebuilds. Called by the watcher on every
// .java file event.
func (w *WorkspaceState) BumpJavaSourceVersion() {
	if w == nil {
		return
	}
	w.javaSourceMu.Lock()
	w.javaSourceVersion++
	w.javaSourceMu.Unlock()
}

// Touch records that path has changed on disk since the last
// DrainDirty. Intended to be called alongside Invalidate from the
// file watcher so consumers can later ask "what changed since I last
// looked?" — useful for daemon verbs that want to report DirtyFiles
// in their response stats and for incremental analysis paths.
//
// Touch never blocks on parse work and is safe under concurrent
// callers; it grabs the same mu the parsed cache uses so a snapshot
// pair (DrainDirty + Stats) can be observed atomically by holding mu.
func (w *WorkspaceState) Touch(path string) {
	if w == nil {
		return
	}
	key := normalizeKey(path)
	w.mu.Lock()
	if w.dirty == nil {
		w.dirty = make(map[string]struct{})
	}
	w.dirty[key] = struct{}{}
	w.mu.Unlock()
}

// DirtyCount returns the number of paths currently in the dirty
// set without draining it. Useful for tests and observability that
// need to peek without consuming.
func (w *WorkspaceState) DirtyCount() int {
	if w == nil {
		return 0
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.dirty)
}

// DrainDirty returns the paths Touch'd since the last call, in
// sorted order, and clears the internal dirty-set. The sort is the
// determinism contract: callers (verb response payloads, log lines,
// cache fingerprints) need stable iteration order regardless of map
// iteration randomness.
//
// Returns nil when no Touch has occurred since the last drain.
func (w *WorkspaceState) DrainDirty() []string {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	if len(w.dirty) == 0 {
		w.mu.Unlock()
		return nil
	}
	paths := make([]string, 0, len(w.dirty))
	for p := range w.dirty {
		paths = append(paths, p)
	}
	w.dirty = nil
	w.mu.Unlock()
	sort.Strings(paths)
	return paths
}

// InvalidateAll drops every cached entry. Used when the workspace
// itself becomes stale (e.g. a future fsnotify path detects a config
// change that affects all parses).
func (w *WorkspaceState) InvalidateAll() {
	if w == nil {
		return
	}
	w.mu.Lock()
	w.parsed = make(map[string]parsedEntry)
	w.dirty = nil
	w.mu.Unlock()

	w.xfileMu.Lock()
	w.libraryFacts = xfileSlot[*librarymodel.Facts]{}
	w.codeIndex = xfileSlot[*scanner.CodeIndex]{}
	w.dependents = xfileSlot[*scanner.DependentsIndex]{}
	w.resolver = xfileSlot[typeinfer.TypeResolver]{}
	w.oracleFilter = xfileSlot[*oracle.CallTargetFilterSummary]{}
	w.androidProject = xfileSlot[*android.Project]{}
	w.xfileMu.Unlock()
	w.gradleMu.Lock()
	w.gradleFindings = nil
	w.gradleMu.Unlock()
	w.typeInfoMu.Lock()
	w.typeInfo = nil
	w.typeInfoMu.Unlock()
	w.javaSourceMu.Lock()
	w.cachedJavaSourceIndex = nil
	w.javaSourceVersion++
	w.javaSourceMu.Unlock()
	w.resolverFpMu.Lock()
	w.cachedResolverFp = ""
	w.cachedResolverFpPresent = false
	w.resolverFpMu.Unlock()
	w.codeIndexSnapshotMu.Lock()
	w.codeIndexSnapshot = nil
	w.codeIndexSnapshotMeta = scanner.CrossFileCacheMeta{}
	w.codeIndexSnapshotMu.Unlock()
}

// LibraryFacts returns the cached *librarymodel.Facts when its
// fingerprint matches; otherwise build is invoked, the result is
// stored, and returned. A nil receiver always builds (no caching).
//
// build runs without xfileMu held to keep parallel readers
// non-blocking; the second writer with the same fingerprint sees its
// own value discarded in favour of the winning entry. fingerprint ""
// disables caching for this call so callers can opt out without a
// type-side change.
func (w *WorkspaceState) LibraryFacts(fingerprint string, build func() *librarymodel.Facts) *librarymodel.Facts {
	if w == nil || fingerprint == "" {
		return build()
	}
	w.xfileMu.Lock()
	if w.libraryFacts.present && w.libraryFacts.fingerprint == fingerprint {
		v := w.libraryFacts.value
		w.xfileMu.Unlock()
		return v
	}
	w.xfileMu.Unlock()

	v := build()

	w.xfileMu.Lock()
	if w.libraryFacts.present && w.libraryFacts.fingerprint == fingerprint {
		// Lost the race; return the winning entry for pointer
		// stability.
		v = w.libraryFacts.value
	} else {
		w.libraryFacts = xfileSlot[*librarymodel.Facts]{fingerprint: fingerprint, value: v, present: true}
	}
	w.xfileMu.Unlock()
	return v
}

// CodeIndex is LibraryFacts for *scanner.CodeIndex. Same semantics:
// build only fires on a fingerprint mismatch; concurrent races
// converge on a single cached pointer.
func (w *WorkspaceState) CodeIndex(fingerprint string, build func() *scanner.CodeIndex) *scanner.CodeIndex {
	if w == nil || fingerprint == "" {
		return build()
	}
	w.xfileMu.Lock()
	if w.codeIndex.present && w.codeIndex.fingerprint == fingerprint {
		v := w.codeIndex.value
		w.xfileMu.Unlock()
		return v
	}
	w.xfileMu.Unlock()

	v := build()

	w.xfileMu.Lock()
	if w.codeIndex.present && w.codeIndex.fingerprint == fingerprint {
		v = w.codeIndex.value
	} else {
		w.codeIndex = xfileSlot[*scanner.CodeIndex]{fingerprint: fingerprint, value: v, present: true}
	}
	w.xfileMu.Unlock()
	return v
}

// Dependents is LibraryFacts for *scanner.DependentsIndex. Same
// semantics: build only fires on a fingerprint mismatch; concurrent
// races converge on a single cached pointer. The fingerprint should be
// the same one driving CodeIndex so the two slots stay aligned.
func (w *WorkspaceState) Dependents(fingerprint string, build func() *scanner.DependentsIndex) *scanner.DependentsIndex {
	if w == nil || fingerprint == "" {
		return build()
	}
	w.xfileMu.Lock()
	if w.dependents.present && w.dependents.fingerprint == fingerprint {
		v := w.dependents.value
		w.xfileMu.Unlock()
		return v
	}
	w.xfileMu.Unlock()

	v := build()

	w.xfileMu.Lock()
	if w.dependents.present && w.dependents.fingerprint == fingerprint {
		v = w.dependents.value
	} else {
		w.dependents = xfileSlot[*scanner.DependentsIndex]{fingerprint: fingerprint, value: v, present: true}
	}
	w.xfileMu.Unlock()
	return v
}

// InvalidateDependents drops the cached DependentsIndex so the next
// Dependents call rebuilds from current sources. Called by the file
// watcher whenever a Kotlin file changes — the dependents map is
// affected by edits to import_header nodes.
func (w *WorkspaceState) InvalidateDependents() {
	if w == nil {
		return
	}
	w.xfileMu.Lock()
	w.dependents = xfileSlot[*scanner.DependentsIndex]{}
	w.xfileMu.Unlock()
}

// CrossFileStats reports whether the cross-file slots are populated.
// Used by tests and verbose diagnostics.
type CrossFileStats struct {
	HasLibraryFacts bool
	HasCodeIndex    bool
	HasDependents   bool
	HasResolver     bool
	HasOracleFilter bool
}

// CrossFileStats returns a snapshot of the cross-file slots.
func (w *WorkspaceState) CrossFileStats() CrossFileStats {
	if w == nil {
		return CrossFileStats{}
	}
	w.xfileMu.Lock()
	defer w.xfileMu.Unlock()
	return CrossFileStats{
		HasLibraryFacts: w.libraryFacts.present,
		HasCodeIndex:    w.codeIndex.present,
		HasDependents:   w.dependents.present,
		HasResolver:     w.resolver.present,
		HasOracleFilter: w.oracleFilter.present,
	}
}

// InvalidateLibraryFacts drops the cached *librarymodel.Facts. Called
// when a Gradle build script or version catalog changes — the next
// LibraryFacts call rebuilds. Also drops the AndroidProject cache
// since its GradlePaths set could have changed.
func (w *WorkspaceState) InvalidateLibraryFacts() {
	if w == nil {
		return
	}
	w.xfileMu.Lock()
	w.libraryFacts = xfileSlot[*librarymodel.Facts]{}
	w.androidProject = xfileSlot[*android.Project]{}
	w.xfileMu.Unlock()
	w.gradleMu.Lock()
	w.gradleFindings = nil
	w.gradleMu.Unlock()
}

// GradleFindings memoizes per-file gradle rule findings across
// analyzes, keyed by (content hash + rule hash). On a 200-gradle-file
// monorepo (kotlin) the per-file Read+Parse+Dispatch cost is ~7 ms
// each, ~1.4 s total per warm analyze — even though gradle files
// rarely change. Memoizing the findings drops this to ~0 ms on warm
// reruns.
//
// The watcher's InvalidateLibraryFacts hook (fired on build.gradle /
// version-catalog edits) clears the whole map, so any gradle dependency
// change forces re-run of every gradle rule. nil receiver disables
// caching.
func (w *WorkspaceState) GradleFindings(key string, build func() scanner.FindingColumns) scanner.FindingColumns {
	if w == nil || key == "" {
		return build()
	}
	w.gradleMu.Lock()
	if w.gradleFindings != nil {
		if cached, ok := w.gradleFindings[key]; ok {
			w.gradleMu.Unlock()
			return cached
		}
	}
	w.gradleMu.Unlock()

	v := build()

	w.gradleMu.Lock()
	if w.gradleFindings == nil {
		w.gradleFindings = make(map[string]scanner.FindingColumns)
	}
	w.gradleFindings[key] = v
	w.gradleMu.Unlock()
	return v
}

// BumpSourceMTimeVersion increments the watcher-driven version
// counter that bundle-stats-clean memos compare against. The file
// watcher calls this on every source-path event (Kotlin / Java /
// Gradle / version-catalog) so the next analyze knows whether its
// "stats matched" memo is still valid. Safe for concurrent use.
func (w *WorkspaceState) BumpSourceMTimeVersion() {
	if w == nil {
		return
	}
	w.sourceMTimeVersion.Add(1)
}

// SourceMTimeVersion returns the current value of the version
// counter. Callers snapshot this before computing fileStatsMatch and
// hand it back to MarkBundleStatsClean — that way a watcher event
// fired DURING the stat sweep correctly invalidates the memo.
func (w *WorkspaceState) SourceMTimeVersion() uint64 {
	if w == nil {
		return 0
	}
	return w.sourceMTimeVersion.Load()
}

// BundleStatsClean reports whether fileStatsMatch last succeeded for
// bundleKey at the current sourceMTimeVersion — i.e. nothing has
// fired the watcher since the last successful sweep, so the 18k
// os.Stat calls would necessarily produce the same answer. Returns
// false on any miss (key never seen, or a watcher event has fired);
// the caller falls through to the real stat sweep.
func (w *WorkspaceState) BundleStatsClean(bundleKey string) bool {
	if w == nil || bundleKey == "" {
		return false
	}
	currentVersion := w.sourceMTimeVersion.Load()
	w.statsCleanMu.Lock()
	stored, ok := w.bundleStatsClean[bundleKey]
	w.statsCleanMu.Unlock()
	return ok && stored == currentVersion
}

// MarkBundleStatsClean records that fileStatsMatch succeeded for
// bundleKey at version. Pair with SourceMTimeVersion() at the start
// of the sweep: a concurrent watcher event between the snapshot and
// the mark advances the counter, the stored version no longer
// matches, and the next call re-stats — closing the race window.
func (w *WorkspaceState) MarkBundleStatsClean(bundleKey string, version uint64) {
	if w == nil || bundleKey == "" {
		return
	}
	w.statsCleanMu.Lock()
	if w.bundleStatsClean == nil {
		w.bundleStatsClean = make(map[string]uint64)
	}
	w.bundleStatsClean[bundleKey] = version
	w.statsCleanMu.Unlock()
}

// BundleOutput returns the cached formatted-bytes envelope for the
// given bundle key, or nil on miss. Callers (the bundle-hit
// fast path in tryLoadFindingsBundleBeforeParse) use the cached
// bytes verbatim and only re-emit the dynamic envelope fields
// (durationMs, perf stats) around them.
func (w *WorkspaceState) BundleOutput(bundleKey string) *CachedBundleOutput {
	if w == nil || bundleKey == "" {
		return nil
	}
	w.bundleOutputMu.Lock()
	defer w.bundleOutputMu.Unlock()
	return w.bundleOutput[bundleKey]
}

// StoreBundleOutput records the freshly-formatted findings bytes +
// summary so the next bundle-hit for the same key can skip the
// ~24 ms format pass entirely. The cache is content-keyed by
// fingerprint; a rules-config change rotates the key, so a stale
// entry can never serve a different rule set's findings.
func (w *WorkspaceState) StoreBundleOutput(bundleKey string, output *CachedBundleOutput) {
	if w == nil || bundleKey == "" || output == nil {
		return
	}
	w.bundleOutputMu.Lock()
	if w.bundleOutput == nil {
		w.bundleOutput = make(map[string]*CachedBundleOutput)
	}
	w.bundleOutput[bundleKey] = output
	w.bundleOutputMu.Unlock()
}

// AndroidProject memoizes the detected Android project across calls.
// fingerprint must capture every input that affects detection (today
// just the project root, since DetectProject is path-only and the
// watcher invalidates this slot whenever a build.gradle / version
// catalog changes). nil receiver always builds (no caching).
func (w *WorkspaceState) AndroidProject(fingerprint string, build func() *android.Project) *android.Project {
	if w == nil || fingerprint == "" {
		return build()
	}
	w.xfileMu.Lock()
	if w.androidProject.present && w.androidProject.fingerprint == fingerprint {
		v := w.androidProject.value
		w.xfileMu.Unlock()
		return v
	}
	w.xfileMu.Unlock()

	v := build()

	w.xfileMu.Lock()
	if w.androidProject.present && w.androidProject.fingerprint == fingerprint {
		v = w.androidProject.value
	} else {
		w.androidProject = xfileSlot[*android.Project]{fingerprint: fingerprint, value: v, present: true}
	}
	w.xfileMu.Unlock()
	return v
}

// InvalidateCodeIndex drops the cached *scanner.CodeIndex. Called
// when any source file changes — the cross-file index aggregates
// across all sources, so any edit invalidates it. The next CodeIndex
// call rebuilds.
func (w *WorkspaceState) InvalidateCodeIndex() {
	if w == nil {
		return
	}
	w.xfileMu.Lock()
	w.codeIndex = xfileSlot[*scanner.CodeIndex]{}
	w.xfileMu.Unlock()
}

// Resolver memoizes a typeinfer.TypeResolver across calls. Same
// semantics as CodeIndex: build only fires on a fingerprint mismatch;
// concurrent races converge on a single cached pointer. The
// fingerprint must capture every input that affects resolver state
// (Kotlin file paths + content hashes today). A nil receiver always
// builds (no caching).
func (w *WorkspaceState) Resolver(fingerprint string, build func() typeinfer.TypeResolver) typeinfer.TypeResolver {
	if w == nil || fingerprint == "" {
		return build()
	}
	w.xfileMu.Lock()
	if w.resolver.present && w.resolver.fingerprint == fingerprint {
		v := w.resolver.value
		w.xfileMu.Unlock()
		return v
	}
	w.xfileMu.Unlock()

	v := build()

	w.xfileMu.Lock()
	if w.resolver.present && w.resolver.fingerprint == fingerprint {
		v = w.resolver.value
	} else {
		w.resolver = xfileSlot[typeinfer.TypeResolver]{fingerprint: fingerprint, value: v, present: true}
	}
	w.xfileMu.Unlock()
	return v
}

// LoadCodeIndexSnapshot returns the daemon's last-successfully-built
// CodeIndex plus the meta it was built from. The third return value
// is false when no snapshot has been stored yet. Unlike the
// fingerprint-keyed CodeIndex slot, this snapshot survives watcher
// invalidations — scanner.BuildIndexCachedWithPrior uses it to skip
// the disk decode of the prior payload during the overlay rebuild.
//
// Callers must NOT mutate the returned index outside the
// scanner.BuildIndexIncremental code path that is documented to
// adopt the prior in place. The daemon's per-project analyze mutex
// keeps that mutation exclusive to one in-flight build.
func (w *WorkspaceState) LoadCodeIndexSnapshot() (*scanner.CodeIndex, scanner.CrossFileCacheMeta, bool) {
	if w == nil {
		return nil, scanner.CrossFileCacheMeta{}, false
	}
	w.codeIndexSnapshotMu.Lock()
	defer w.codeIndexSnapshotMu.Unlock()
	if w.codeIndexSnapshot == nil {
		return nil, scanner.CrossFileCacheMeta{}, false
	}
	return w.codeIndexSnapshot, w.codeIndexSnapshotMeta, true
}

// StoreCodeIndexSnapshot records the just-built CodeIndex and its
// meta as the new resident snapshot. The daemon's runCodeIndexBuild
// calls this on every successful BuildIndexCachedWithPrior return so
// the next analyze can skip the disk decode of the prior payload.
//
// nil idx clears the snapshot — useful for InvalidateAll-style
// resets when the workspace state itself becomes stale.
func (w *WorkspaceState) StoreCodeIndexSnapshot(idx *scanner.CodeIndex, meta scanner.CrossFileCacheMeta) {
	if w == nil {
		return
	}
	w.codeIndexSnapshotMu.Lock()
	w.codeIndexSnapshot = idx
	w.codeIndexSnapshotMeta = meta
	w.codeIndexSnapshotMu.Unlock()
}

// ResolverFingerprint returns the resolver fingerprint string,
// reusing the last successful compute when no source-path watcher
// event has fired since (i.e. sourceMTimeVersion is unchanged). On
// a miss it snapshots the current version, invokes build (without
// holding the mutex), and stores the result if the version is still
// the snapshot — a concurrent watcher event between snapshot and
// store invalidates the would-be entry so the next call recomputes.
//
// build must produce a fingerprint string deterministic in its
// inputs; the cache only short-circuits when nothing watched has
// changed, so build's inputs are guaranteed identical on a hit. A
// nil receiver always builds (no caching).
func (w *WorkspaceState) ResolverFingerprint(build func() string) string {
	if w == nil {
		return build()
	}
	currentVersion := w.sourceMTimeVersion.Load()
	w.resolverFpMu.Lock()
	if w.cachedResolverFpPresent && w.cachedResolverFpVersion == currentVersion {
		v := w.cachedResolverFp
		w.resolverFpMu.Unlock()
		return v
	}
	w.resolverFpMu.Unlock()

	fp := build()

	w.resolverFpMu.Lock()
	if w.sourceMTimeVersion.Load() == currentVersion {
		w.cachedResolverFp = fp
		w.cachedResolverFpVersion = currentVersion
		w.cachedResolverFpPresent = true
	}
	w.resolverFpMu.Unlock()
	return fp
}

// InvalidateResolver drops the cached resolver. Called when any
// Kotlin source file changes — typeinfer's ImportTable / class /
// extension state aggregates across files, so any edit invalidates
// the whole slot. Belt-and-suspenders alongside the fingerprint
// check.
func (w *WorkspaceState) InvalidateResolver() {
	if w == nil {
		return
	}
	w.xfileMu.Lock()
	w.resolver = xfileSlot[typeinfer.TypeResolver]{}
	w.xfileMu.Unlock()
}

// OracleFilter memoizes the oracle CallTargetFilterSummary across
// calls. Same semantics as Resolver: build only fires on a
// fingerprint mismatch; concurrent races converge on a single cached
// pointer. nil receiver always builds (no caching).
func (w *WorkspaceState) OracleFilter(fingerprint string, build func() *oracle.CallTargetFilterSummary) *oracle.CallTargetFilterSummary {
	if w == nil || fingerprint == "" {
		return build()
	}
	w.xfileMu.Lock()
	if w.oracleFilter.present && w.oracleFilter.fingerprint == fingerprint {
		v := w.oracleFilter.value
		w.xfileMu.Unlock()
		return v
	}
	w.xfileMu.Unlock()

	v := build()

	w.xfileMu.Lock()
	if w.oracleFilter.present && w.oracleFilter.fingerprint == fingerprint {
		v = w.oracleFilter.value
	} else {
		w.oracleFilter = xfileSlot[*oracle.CallTargetFilterSummary]{fingerprint: fingerprint, value: v, present: true}
	}
	w.xfileMu.Unlock()
	return v
}

// InvalidateOracleFilter drops the cached oracle filter. The watcher's
// per-Kotlin-edit invalidation hook calls this so the next analyze
// rebuilds. The fingerprint check would catch this anyway; this is
// belt-and-suspenders symmetric with the other slot invalidators.
func (w *WorkspaceState) InvalidateOracleFilter() {
	if w == nil {
		return
	}
	w.xfileMu.Lock()
	w.oracleFilter = xfileSlot[*oracle.CallTargetFilterSummary]{}
	w.xfileMu.Unlock()
}

// WorkspaceStats is a point-in-time snapshot of cache utilisation,
// useful for testing and verbose logging.
type WorkspaceStats struct {
	ParsedEntries int
	Hits          int64
	Misses        int64
}

// Stats returns a snapshot of the current cache state.
func (w *WorkspaceState) Stats() WorkspaceStats {
	if w == nil {
		return WorkspaceStats{}
	}
	w.mu.RLock()
	n := len(w.parsed)
	w.mu.RUnlock()
	return WorkspaceStats{
		ParsedEntries: n,
		Hits:          w.hits.Load(),
		Misses:        w.misses.Load(),
	}
}

// normalizeKey collapses different spellings of the same path so two
// callers that disagree on absolute-vs-relative form share one entry.
// Empty paths pass through (in-memory buffers without a real file).
func normalizeKey(path string) string {
	if path == "" {
		return path
	}
	return filepath.Clean(path)
}
