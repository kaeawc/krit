package firchecks

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestCollectFirCheckFiles_AllFilesShortCircuit(t *testing.T) {
	rules := []FirFilterRule{
		{Name: "AllRule", Filter: &FirFilterSpec{AllFiles: true}},
	}
	files := []*scanner.File{
		{Path: "/src/A.kt", Content: []byte("fun a() {}")},
		{Path: "/src/B.kt", Content: []byte("fun b() {}")},
	}
	summary := CollectFirCheckFiles(rules, files)
	if !summary.AllFiles {
		t.Error("expected AllFiles=true when any rule has AllFiles")
	}
	if summary.MarkedFiles != 2 {
		t.Errorf("expected 2 marked files, got %d", summary.MarkedFiles)
	}
}

func TestCollectFirCheckFiles_IdentifierMatching(t *testing.T) {
	rules := []FirFilterRule{
		{Name: "FlowRule", Filter: &FirFilterSpec{Identifiers: []string{"collect {"}}},
	}
	files := []*scanner.File{
		{Path: "/src/A.kt", Content: []byte("flow.collect { v -> println(v) }")},
		{Path: "/src/B.kt", Content: []byte("fun other() {}")},
	}
	summary := CollectFirCheckFiles(rules, files)
	if summary.AllFiles {
		t.Error("expected AllFiles=false")
	}
	if summary.MarkedFiles != 1 {
		t.Errorf("expected 1 marked file, got %d", summary.MarkedFiles)
	}
	if len(summary.Paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(summary.Paths))
	}
}

func TestCollectFirCheckFiles_NoRules(t *testing.T) {
	files := []*scanner.File{
		{Path: "/src/A.kt", Content: []byte("fun a() {}")},
	}
	summary := CollectFirCheckFiles(nil, files)
	if summary.MarkedFiles != 0 {
		t.Errorf("expected 0 marked files for no rules, got %d", summary.MarkedFiles)
	}
}

func TestCollectFirCheckFiles_EmptyIdentifiers(t *testing.T) {
	rules := []FirFilterRule{
		{Name: "EmptyRule", Filter: &FirFilterSpec{Identifiers: []string{}}},
	}
	files := []*scanner.File{
		{Path: "/src/A.kt", Content: []byte("fun a() {}")},
	}
	summary := CollectFirCheckFiles(rules, files)
	if summary.Paths == nil {
		t.Error("expected empty Paths slice, not nil")
	}
	if len(summary.Paths) != 0 {
		t.Errorf("expected 0 paths, got %d", len(summary.Paths))
	}
}

func TestCollectFirCheckFiles_NilFilter(t *testing.T) {
	// nil Filter is treated as AllFiles: true (conservative default).
	rules := []FirFilterRule{
		{Name: "UnauditedRule", Filter: nil},
	}
	files := []*scanner.File{
		{Path: "/src/A.kt", Content: []byte("fun a() {}")},
	}
	summary := CollectFirCheckFiles(rules, files)
	if !summary.AllFiles {
		t.Error("expected AllFiles=true for nil filter")
	}
}
