package oracle

import (
	"sync"

	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// LazyLookup defers oracle JSON deserialization until the first semantic
// lookup. Warm runs with complete findings-cache hits can carry this through
// the resolver without paying jsonLoad at all.
type LazyLookup struct {
	path    string
	onError func(error)

	once   sync.Once
	loaded *Oracle
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
	l.once.Do(func() {
		l.loaded, l.err = Load(l.path)
		if l.err != nil && l.onError != nil {
			l.onError(l.err)
		}
	})
	return l.loaded
}

// Loaded reports whether the JSON has been deserialized successfully.
func (l *LazyLookup) Loaded() bool {
	return l != nil && l.loaded != nil
}

// Err reports the load error after a lookup attempts to load the JSON.
func (l *LazyLookup) Err() error {
	if l == nil {
		return nil
	}
	return l.err
}

func (l *LazyLookup) Stats() Stats {
	if l != nil && l.loaded != nil {
		return l.loaded.Stats()
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
