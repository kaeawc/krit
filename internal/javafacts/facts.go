package javafacts

import (
	"encoding/json"
	"fmt"
)

const Version = 1

type Facts struct {
	Version int         `json:"version"`
	Calls   []CallFact  `json:"calls"`
	Classes []ClassFact `json:"classes"`
}

type CallFact struct {
	File         string `json:"file"`
	Line         int    `json:"line"`
	Col          int    `json:"col"`
	Callee       string `json:"callee"`
	ReceiverType string `json:"receiverType"`
	Element      string `json:"element"`
	ReturnType   string `json:"returnType"`
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
	if f == nil {
		return ""
	}
	for _, call := range f.Calls {
		if call.File == file && call.Line == line && call.Col == col {
			return call.ReceiverType
		}
	}
	return ""
}
