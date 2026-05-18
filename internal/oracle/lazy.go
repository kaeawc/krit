package oracle

import (
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

var (
	preloadMu     sync.Mutex
	preloadByPath = map[string]*preloadState{}
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
	if state := preloadByPath[path]; state != nil {
		return state
	}
	state := &preloadState{done: make(chan struct{})}
	preloadByPath[path] = state
	return state
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
