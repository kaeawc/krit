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

func TestActiveFirRules_MapsEnabledCatalogRules(t *testing.T) {
	active := ActiveFirRules([]string{"InjectDispatcher", "UnrelatedRule", "InjectDispatcher"}, false)
	if len(active.Names) != 1 || active.Names[0] != "INJECT_DISPATCHER" {
		t.Fatalf("expected one FIR checker name, got %#v", active.Names)
	}
	if len(active.Filters) != 1 || active.Filters[0].Name != "INJECT_DISPATCHER" {
		t.Fatalf("expected one FIR filter, got %#v", active.Filters)
	}
}

// Verifies the thorough projection is non-mutating. Daemon mode issues
// alternating balanced and thorough requests against the same shared
// firRuleFilters map; this test confirms the projection returns clones
// without leaking changes back to the global registry.
func TestActiveFirRules_ThoroughProjectsExtraIdentifiers(t *testing.T) {
	const id = "ThoroughProbeRule"
	t.Cleanup(func() { delete(firRuleFilters, id) })
	firRuleFilters[id] = FirFilterRule{
		Name: "THOROUGH_PROBE",
		Filter: &FirFilterSpec{
			Identifiers:             []string{"base"},
			ThoroughOnlyIdentifiers: []string{"extra"},
			ThoroughOnlyAllFiles:    true,
		},
	}

	balanced := ActiveFirRules([]string{id}, false)
	if balanced.Filters[0].Filter.AllFiles {
		t.Errorf("balanced must not promote AllFiles; got AllFiles=true")
	}
	if len(balanced.Filters[0].Filter.Identifiers) != 1 {
		t.Errorf("balanced identifiers must be unprojected; got %v", balanced.Filters[0].Filter.Identifiers)
	}

	thorough := ActiveFirRules([]string{id}, true)
	if !thorough.Filters[0].Filter.AllFiles {
		t.Errorf("thorough must project ThoroughOnlyAllFiles; got AllFiles=false")
	}
	want := []string{"base", "extra"}
	if len(thorough.Filters[0].Filter.Identifiers) != len(want) {
		t.Fatalf("thorough identifiers = %v; want %v", thorough.Filters[0].Filter.Identifiers, want)
	}
	for i := range want {
		if thorough.Filters[0].Filter.Identifiers[i] != want[i] {
			t.Errorf("identifiers[%d] = %q; want %q", i, thorough.Filters[0].Filter.Identifiers[i], want[i])
		}
	}

	registry := firRuleFilters[id]
	if len(registry.Filter.Identifiers) != 1 || registry.Filter.AllFiles {
		t.Fatalf("global firRuleFilters mutated by projection: %+v", registry.Filter)
	}
}
