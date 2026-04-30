package javafacts

import (
	"encoding/json"
	"fmt"
	"path/filepath"
)

const Version = 1

type Facts struct {
	Version int         `json:"version"`
	Calls   []CallFact  `json:"calls"`
	Classes []ClassFact `json:"classes"`
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
	for _, call := range f.Calls {
		if sameFilePath(call.File, file) && call.Line == line && call.Col == col {
			return call, true
		}
	}
	return CallFact{}, false
}

func (f *Facts) ClassSupertypes(file string, line, col int) []string {
	if f == nil {
		return nil
	}
	for _, class := range f.Classes {
		if sameFilePath(class.File, file) && class.Line == line && class.Col == col {
			return append([]string{}, class.Supertypes...)
		}
	}
	return nil
}

func sameFilePath(a, b string) bool {
	if a == b {
		return true
	}
	cleanA := filepath.Clean(a)
	cleanB := filepath.Clean(b)
	if cleanA == cleanB {
		return true
	}
	absA, errA := filepath.Abs(cleanA)
	absB, errB := filepath.Abs(cleanB)
	return errA == nil && errB == nil && absA == absB
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
