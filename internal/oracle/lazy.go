package oracle

import (
	"container/list"
	"sync"
	"sync/atomic"

	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

type preloadState struct {
	once   sync.Once
	done   chan struct{}
	loaded *Oracle
	err    error
}

// preloadCacheCap bounds the number of distinct oracle JSON paths the
// package-level preload cache retains. Each entry pins an *Oracle (tens
// of MB for large repos) so an unbounded map leaks memory in long-running
// daemons (`krit serve`, LSP) that touch a stream of distinct paths.
// The cap is generous for realistic multi-project daemon use; once a
// LazyLookup resolves it holds its own oracle pointer, so evicting a
// completed entry only forces a re-Load on a future first lookup for
// the same path.
const preloadCacheCap = 64

type preloadEntry struct {
	path  string
	state *preloadState
}

var (
	preloadMu     sync.Mutex
	preloadByPath = map[string]*list.Element{}
	preloadLRU    = list.New() // front = most-recently-used; back = LRU
)

// PreloadPath kicks off oracle JSON deserialization for path and keeps the
// result available for any later LazyLookup created for the same file.
func PreloadPath(path string) {
	if path == "" {
		return
	}
	state := preloadStateFor(path)
	go state.load(path)
}

func preloadStateFor(path string) *preloadState {
	preloadMu.Lock()
	defer preloadMu.Unlock()
	if elem := preloadByPath[path]; elem != nil {
		preloadLRU.MoveToFront(elem)
		return elem.Value.(*preloadEntry).state
	}
	state := &preloadState{done: make(chan struct{})}
	entry := &preloadEntry{path: path, state: state}
	elem := preloadLRU.PushFront(entry)
	preloadByPath[path] = elem
	evictOldestCompletedLocked()
	return state
}

// evictOldestCompletedLocked trims the LRU until either the cap is met
// or no entries are evictable. Evicting an entry whose load goroutine
// is still running would let a later LazyLookup for the same path
// create a fresh state and re-Load the (~tens-of-MB) oracle, defeating
// the documented cap; under LSP burst traffic the doubled loads
// stacked into noticeable goroutine and heap pressure. The cap is
// therefore soft against in-flight loads — once they complete the
// next eviction pass reclaims their slots.
//
// Caller must hold preloadMu.
func evictOldestCompletedLocked() {
	for preloadLRU.Len() > preloadCacheCap {
		victim := oldestCompletedLocked()
		if victim == nil {
			return
		}
		preloadLRU.Remove(victim)
		delete(preloadByPath, victim.Value.(*preloadEntry).path)
	}
}

// oldestCompletedLocked returns the oldest LRU element whose load has
// finished, or nil if every entry is still in flight.
func oldestCompletedLocked() *list.Element {
	for e := preloadLRU.Back(); e != nil; e = e.Prev() {
		if preloadStateDone(e.Value.(*preloadEntry).state) {
			return e
		}
	}
	return nil
}

func preloadStateDone(s *preloadState) bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

// preloadCacheLen returns the current number of cached entries. Test-only
// helper; not exported.
func preloadCacheLen() int {
	preloadMu.Lock()
	defer preloadMu.Unlock()
	return preloadLRU.Len()
}

func (s *preloadState) load(path string) {
	s.once.Do(func() {
		s.loaded, s.err = Load(path)
		close(s.done)
	})
}

// LazyLookup defers oracle JSON deserialization until the first semantic
// lookup. Warm runs with complete findings-cache hits can carry this through
// the resolver without paying jsonLoad at all.
type LazyLookup struct {
	path    string
	onError func(error)

	once   sync.Once
	result atomic.Pointer[lazyResult]
}

// lazyResult packages the oracle/err pair so a single atomic store publishes
// both fields, letting Loaded/Err/Stats observers skip sync.Once without a
// data race.
type lazyResult struct {
	oracle *Oracle
	err    error
}

// NewLazyLookup returns a Lookup backed by path. onError is called once if the
// load fails; nil is accepted.
func NewLazyLookup(path string, onError func(error)) *LazyLookup {
	return &LazyLookup{path: path, onError: onError}
}

func (l *LazyLookup) get() *Oracle {
	if l == nil || l.path == "" {
		return nil
	}
	l.once.Do(l.load)
	if r := l.result.Load(); r != nil {
		return r.oracle
	}
	return nil
}

// load is the once-guarded body invoked by both get() and Preload.
// Pulled out as a method so Preload can hand it to once.Do without
// allocating a fresh closure per call.
func (l *LazyLookup) load() {
	state := preloadStateFor(l.path)
	state.load(l.path)
	<-state.done
	l.result.Store(&lazyResult{oracle: state.loaded, err: state.err})
	if state.err != nil && l.onError != nil {
		l.onError(state.err)
	}
}

// Preload kicks off the JSON deserialization in a background goroutine
// so the first lookup observes a warm sync.Once instead of paying the
// load latency itself. On large repos (Kotlin compiler: ~41 MB
// types.json) the deferred load was ~500 ms, surfacing in per-rule
// timings as whichever rule happened to fire first — Preload moves
// that wall time off the rule path. Idempotent: multiple Preload
// calls (or Preload followed by a real lookup) coalesce on the same
// sync.Once.
func (l *LazyLookup) Preload() {
	if l == nil || l.path == "" {
		return
	}
	PreloadPath(l.path)
	go l.once.Do(l.load)
}

// Loaded reports whether the JSON has been deserialized successfully.
func (l *LazyLookup) Loaded() bool {
	if l == nil {
		return false
	}
	r := l.result.Load()
	return r != nil && r.oracle != nil
}

// Err reports the load error after a lookup attempts to load the JSON.
func (l *LazyLookup) Err() error {
	if l == nil {
		return nil
	}
	if r := l.result.Load(); r != nil {
		return r.err
	}
	return nil
}

func (l *LazyLookup) Stats() Stats {
	if l == nil {
		return Stats{}
	}
	if r := l.result.Load(); r != nil && r.oracle != nil {
		return r.oracle.Stats()
	}
	return Stats{}
}

func (l *LazyLookup) LookupClass(name string) *typeinfer.ClassInfo {
	if o := l.get(); o != nil {
		return o.LookupClass(name)
	}
	return nil
}

func (l *LazyLookup) LookupSealedVariants(name string) []string {
	if o := l.get(); o != nil {
		return o.LookupSealedVariants(name)
	}
	return nil
}

func (l *LazyLookup) LookupEnumEntries(name string) []string {
	if o := l.get(); o != nil {
		return o.LookupEnumEntries(name)
	}
	return nil
}

func (l *LazyLookup) IsSubtype(a, b string) bool {
	if o := l.get(); o != nil {
		return o.IsSubtype(a, b)
	}
	return false
}

func (l *LazyLookup) Dependencies() map[string]*Class {
	if o := l.get(); o != nil {
		return o.Dependencies()
	}
	return nil
}

func (l *LazyLookup) LookupFunction(key string) *typeinfer.ResolvedType {
	if o := l.get(); o != nil {
		return o.LookupFunction(key)
	}
	return nil
}

func (l *LazyLookup) LookupExpression(filePath string, line, col int) *typeinfer.ResolvedType {
	if o := l.get(); o != nil {
		return o.LookupExpression(filePath, line, col)
	}
	return nil
}

func (l *LazyLookup) LookupExpressionFlat(file *scanner.File, idx uint32) *typeinfer.ResolvedType {
	if o := l.get(); o != nil {
		return o.LookupExpressionFlat(file, idx)
	}
	return nil
}

func (l *LazyLookup) LookupAnnotations(key string) []string {
	if o := l.get(); o != nil {
		return o.LookupAnnotations(key)
	}
	return nil
}

func (l *LazyLookup) LookupCallTarget(filePath string, line, col int) string {
	if o := l.get(); o != nil {
		return o.LookupCallTarget(filePath, line, col)
	}
	return ""
}

func (l *LazyLookup) LookupCallTargetFlat(file *scanner.File, idx uint32) string {
	if o := l.get(); o != nil {
		return o.LookupCallTargetFlat(file, idx)
	}
	return ""
}

func (l *LazyLookup) LookupCallTargetSuspend(filePath string, line, col int) (bool, bool) {
	if o := l.get(); o != nil {
		return o.LookupCallTargetSuspend(filePath, line, col)
	}
	return false, false
}

func (l *LazyLookup) LookupCallTargetSuspendFlat(file *scanner.File, idx uint32) (bool, bool) {
	if o := l.get(); o != nil {
		return o.LookupCallTargetSuspendFlat(file, idx)
	}
	return false, false
}

func (l *LazyLookup) LookupCallTargetAnnotations(filePath string, line, col int) []string {
	if o := l.get(); o != nil {
		return o.LookupCallTargetAnnotations(filePath, line, col)
	}
	return nil
}

func (l *LazyLookup) LookupCallTargetAnnotationsFlat(file *scanner.File, idx uint32) []string {
	if o := l.get(); o != nil {
		return o.LookupCallTargetAnnotationsFlat(file, idx)
	}
	return nil
}

func (l *LazyLookup) LookupDiagnostics(filePath string) []Diagnostic {
	if o := l.get(); o != nil {
		return o.LookupDiagnostics(filePath)
	}
	return nil
}

func (l *LazyLookup) LookupDiagnosticsForFlatRange(file *scanner.File, idx uint32) []Diagnostic {
	if o := l.get(); o != nil {
		return o.LookupDiagnosticsForFlatRange(file, idx)
	}
	return nil
}

var _ Lookup = (*LazyLookup)(nil)
