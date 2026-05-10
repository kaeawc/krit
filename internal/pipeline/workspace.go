package pipeline

import (
	"context"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/librarymodel"
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
	xfileMu      sync.Mutex
	libraryFacts xfileSlot[*librarymodel.Facts]
	codeIndex    xfileSlot[*scanner.CodeIndex]
	dependents   xfileSlot[*scanner.DependentsIndex]
	resolver     xfileSlot[typeinfer.TypeResolver]
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

// Invalidate drops the cached entry for path. Safe to call when no
// entry exists.
func (w *WorkspaceState) Invalidate(path string) {
	if w == nil {
		return
	}
	key := normalizeKey(path)
	w.mu.Lock()
	delete(w.parsed, key)
	w.mu.Unlock()
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
	w.xfileMu.Unlock()
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
	}
}

// InvalidateLibraryFacts drops the cached *librarymodel.Facts. Called
// when a Gradle build script or version catalog changes — the next
// LibraryFacts call rebuilds.
func (w *WorkspaceState) InvalidateLibraryFacts() {
	if w == nil {
		return
	}
	w.xfileMu.Lock()
	w.libraryFacts = xfileSlot[*librarymodel.Facts]{}
	w.xfileMu.Unlock()
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
