package typeinfer

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestSetBinSymbolReader_FallbackOnSourceMiss(t *testing.T) {
	r := NewResolver()
	called := 0
	r.SetBinSymbolReader(func(fqn string) *binarySymbolClass {
		called++
		if fqn == "androidx.fragment.app.Fragment" {
			return &binarySymbolClass{
				Name:       "Fragment",
				FQN:        "androidx.fragment.app.Fragment",
				Kind:       "class",
				Supertypes: []string{"androidx.activity.ComponentActivity"},
				IsAbstract: false,
			}
		}
		return nil
	})

	got := r.ClassHierarchy("androidx.fragment.app.Fragment")
	if got == nil {
		t.Fatal("expected ClassHierarchy to fall through to binary reader")
	}
	if got.Name != "Fragment" || got.FQN != "androidx.fragment.app.Fragment" {
		t.Errorf("ClassHierarchy = %+v, want Fragment", got)
	}
	if got.Kind != "class" {
		t.Errorf("Kind = %q, want \"class\"", got.Kind)
	}
	if len(got.Supertypes) != 1 || got.Supertypes[0] != "androidx.activity.ComponentActivity" {
		t.Errorf("Supertypes = %v", got.Supertypes)
	}
	if called == 0 {
		t.Error("binary reader was not consulted")
	}

	// Unknown FQN: reader returns nil, then hardcoded tables are tried.
	if got := r.ClassHierarchy("totally.unknown.Type"); got != nil {
		t.Errorf("expected nil for unknown type, got %+v", got)
	}
}

func TestSetBinSymbolReader_SourceWinsOverBinary(t *testing.T) {
	src := `
package com.example
class Local
`
	file := parseTestFile(t, src)
	r := NewResolver()
	r.IndexFilesParallel([]*scanner.File{file}, 1)
	r.SetBinSymbolReader(func(fqn string) *binarySymbolClass {
		if fqn == "Local" || fqn == "com.example.Local" {
			return &binarySymbolClass{
				Name:       "Local",
				FQN:        "com.example.Local",
				Kind:       "interface", // would be wrong vs source
				Supertypes: []string{"binary.Stub"},
			}
		}
		return nil
	})
	got := r.ClassHierarchy("Local")
	if got == nil {
		t.Fatal("expected source-side hit")
	}
	if got.Kind == "interface" {
		t.Errorf("source class should win; got binary Kind %q", got.Kind)
	}
	if len(got.Supertypes) > 0 && got.Supertypes[0] == "binary.Stub" {
		t.Errorf("source supertypes should win; got %v", got.Supertypes)
	}
}

func TestSetBinSymbolReader_NilClears(t *testing.T) {
	r := NewResolver()
	r.SetBinSymbolReader(func(string) *binarySymbolClass {
		return &binarySymbolClass{Name: "X", FQN: "X"}
	})
	if got := r.ClassHierarchy("X"); got == nil {
		t.Fatal("setup precondition failed")
	}
	r.SetBinSymbolReader(nil)
	if got := r.ClassHierarchy("X"); got != nil {
		t.Errorf("expected nil after clear, got %+v", got)
	}
}
