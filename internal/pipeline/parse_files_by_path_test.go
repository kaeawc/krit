package pipeline

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// TestParseFilesByPath_ParsesRequestedSubset verifies the helper parses only
// the requested Kotlin paths and returns them as *scanner.File — crucially NOT
// the whole project (the daemon footgun the non-nil slice guards against).
func TestParseFilesByPath_ParsesRequestedSubset(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "A.kt")
	b := filepath.Join(dir, "B.kt")
	c := filepath.Join(dir, "C.kt")
	writeKt(t, a, "package test\nclass A\n")
	writeKt(t, b, "package test\nclass B\n")
	writeKt(t, c, "package test\nclass C\n")

	args := ProjectArgs{Config: config.NewConfig(), Paths: []string{dir}}
	kt, jv, err := parseFilesByPath(context.Background(), args, ProjectHostState{}, []string{a}, nil)
	if err != nil {
		t.Fatalf("parseFilesByPath: %v", err)
	}
	if len(jv) != 0 {
		t.Errorf("no Java requested, want 0 Java files; got %d", len(jv))
	}
	if len(kt) != 1 {
		t.Fatalf("requested one Kotlin file, got %d (whole-project collection?)", len(kt))
	}
	if kt[0].Path != a {
		t.Errorf("parsed path = %q, want %q", kt[0].Path, a)
	}
}

// TestParseFilesByPath_EmptyInput returns nil without touching the parser.
func TestParseFilesByPath_EmptyInput(t *testing.T) {
	args := ProjectArgs{Config: config.NewConfig(), Paths: []string{t.TempDir()}}
	kt, jv, err := parseFilesByPath(context.Background(), args, ProjectHostState{}, nil, nil)
	if err != nil {
		t.Fatalf("parseFilesByPath: %v", err)
	}
	if kt != nil || jv != nil {
		t.Errorf("empty input must return nil,nil; got kt=%v jv=%v", kt, jv)
	}
}

// TestParseFilesByPath_JavaWithCrossFileRule confirms Java paths are parsed
// when the active rule set drives Java collection (NeedsCrossFile).
func TestParseFilesByPath_JavaWithCrossFileRule(t *testing.T) {
	dir := t.TempDir()
	j := filepath.Join(dir, "Widget.java")
	writeJavaSource(t, j, "package test;\nclass Widget {}\n")

	rule := api.FakeRule("CrossRule", api.WithNeeds(api.NeedsCrossFile))
	args := ProjectArgs{
		Config:      config.NewConfig(),
		Paths:       []string{dir},
		ActiveRules: []*api.Rule{rule},
	}
	kt, jv, err := parseFilesByPath(context.Background(), args, ProjectHostState{}, nil, []string{j})
	if err != nil {
		t.Fatalf("parseFilesByPath: %v", err)
	}
	if len(kt) != 0 {
		t.Errorf("no Kotlin requested, want 0; got %d", len(kt))
	}
	if len(jv) != 1 || jv[0].Path != j {
		t.Fatalf("requested one Java file %q; got %v", j, pathsOf(jv))
	}
}
