package javafacts

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/kaeawc/krit/internal/hashutil"
)

const Version = 1

type Facts struct {
	Version int         `json:"version"`
	Calls   []CallFact  `json:"calls"`
	Classes []ClassFact `json:"classes"`

	// index maps a normalized (file, line, col) position to the first
	// matching fact. It is built on the first lookup, so Calls and Classes
	// must not change after that.
	indexOnce sync.Once
	calls     map[factKey]int
	classes   map[factKey]int
}

type factKey struct {
	file      string
	line, col int
}

type CallFact struct {
	File         string   `json:"file"`
	Line         int      `json:"line"`
	Col          int      `json:"col"`
	Callee       string   `json:"callee"`
	ReceiverType string   `json:"receiverType"`
	MethodOwner  string   `json:"methodOwner,omitempty"`
	Element      string   `json:"element"`
	ReturnType   string   `json:"returnType"`
	Annotations  []string `json:"annotations,omitempty"`
}

type ClassFact struct {
	File          string   `json:"file"`
	Line          int      `json:"line"`
	Col           int      `json:"col"`
	Name          string   `json:"name"`
	QualifiedName string   `json:"qualifiedName"`
	Supertypes    []string `json:"supertypes"`
}

// Fingerprint returns a stable hex digest of f used as part of cache keys
// for analysis phases whose findings depend on Java semantic facts.
func (f *Facts) Fingerprint() string {
	var buf []byte
	buf = append(buf, "javafacts.Facts/v"...)
	buf = append(buf, byte(Version))
	if f == nil {
		buf = append(buf, "|nil"...)
		return hashutil.HashHex(buf)
	}
	data, err := json.Marshal(f)
	if err != nil {
		buf = append(buf, "|err:"...)
		buf = append(buf, err.Error()...)
		return hashutil.HashHex(buf)
	}
	buf = append(buf, '|')
	buf = append(buf, data...)
	return hashutil.HashHex(buf)
}

func Parse(data []byte) (*Facts, error) {
	var facts Facts
	if err := json.Unmarshal(data, &facts); err != nil {
		return nil, fmt.Errorf("parse java facts JSON: %w", err)
	}
	if facts.Version != Version {
		return nil, fmt.Errorf("unsupported java facts version: %d", facts.Version)
	}
	return &facts, nil
}

func (f *Facts) ReceiverType(file string, line, col int) string {
	if call, ok := f.CallAt(file, line, col); ok {
		return call.ReceiverType
	}
	return ""
}

func (f *Facts) MethodOwner(file string, line, col int) string {
	if call, ok := f.CallAt(file, line, col); ok {
		return call.MethodOwner
	}
	return ""
}

func (f *Facts) ReturnType(file string, line, col int) string {
	if call, ok := f.CallAt(file, line, col); ok {
		return call.ReturnType
	}
	return ""
}

func (f *Facts) CallAnnotations(file string, line, col int) []string {
	if call, ok := f.CallAt(file, line, col); ok {
		return append([]string{}, call.Annotations...)
	}
	return nil
}

func (f *Facts) HasCallAnnotation(file string, line, col int, names ...string) bool {
	if len(names) == 0 {
		return false
	}
	annotations := f.CallAnnotations(file, line, col)
	for _, got := range annotations {
		for _, want := range names {
			if got == want || simpleName(got) == simpleName(want) {
				return true
			}
		}
	}
	return false
}

func (f *Facts) CallAt(file string, line, col int) (CallFact, bool) {
	if f == nil {
		return CallFact{}, false
	}
	f.buildIndex()
	if i, ok := f.calls[factKey{normalizePath(file), line, col}]; ok {
		return f.Calls[i], true
	}
	return CallFact{}, false
}

func (f *Facts) ClassSupertypes(file string, line, col int) []string {
	if f == nil {
		return nil
	}
	f.buildIndex()
	if i, ok := f.classes[factKey{normalizePath(file), line, col}]; ok {
		return append([]string{}, f.Classes[i].Supertypes...)
	}
	return nil
}

// buildIndex indexes Calls and Classes by normalized position. Rules look
// up a fact per Java call node, so a linear scan here is quadratic in the
// project's Java call count.
func (f *Facts) buildIndex() {
	f.indexOnce.Do(func() {
		norm := map[string]string{}
		normalized := func(path string) string {
			n, ok := norm[path]
			if !ok {
				n = normalizePath(path)
				norm[path] = n
			}
			return n
		}
		f.calls = make(map[factKey]int, len(f.Calls))
		for i, call := range f.Calls {
			key := factKey{normalized(call.File), call.Line, call.Col}
			if _, dup := f.calls[key]; !dup {
				f.calls[key] = i
			}
		}
		f.classes = make(map[factKey]int, len(f.Classes))
		for i, class := range f.Classes {
			key := factKey{normalized(class.File), class.Line, class.Col}
			if _, dup := f.classes[key]; !dup {
				f.classes[key] = i
			}
		}
	})
}

// normalizePath returns the absolute, cleaned spelling of path so relative
// and absolute spellings of the same file compare equal.
func normalizePath(path string) string {
	clean := filepath.Clean(path)
	if abs, err := filepath.Abs(clean); err == nil {
		return abs
	}
	return clean
}

func simpleName(value string) string {
	if value == "" {
		return ""
	}
	for i := len(value) - 1; i >= 0; i-- {
		if value[i] == '.' || value[i] == '$' {
			return value[i+1:]
		}
	}
	return value
}
