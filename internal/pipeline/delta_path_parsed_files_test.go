package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// A NeedsParsedFiles rule is promised the complete project slice and may
// report on any file, so the delta path must run it against the complete
// parse set and replace all of its prior rows, not just the edited file's.
// The finding below is reported on an untouched file and depends on the
// edited file's body, both when it appears and when it goes away.
func TestRunProject_BodyOnlyEdit_ParsedFilesRuleMatchesClean(t *testing.T) {
	dir := t.TempDir()
	cacheRoot := t.TempDir()
	usePath := writeBodyEditProject(t, dir, useBefore)
	altPath := filepath.Join(dir, "Alt.kt")

	parsedRule := api.FakeRule("ProjectWideMarker", api.WithNeeds(api.NeedsParsedFiles), api.WithCheck(func(ctx *api.Context) {
		for _, f := range ctx.ParsedFiles {
			if strings.Contains(string(f.Content), "MARKER") {
				ctx.Emit(scanner.Finding{File: altPath, Line: 1, Col: 1, Message: "marker somewhere in project"})
				return
			}
		}
	}))
	rules := []*api.Rule{parsedRule}
	for i := 0; i < 2; i++ {
		if res, _ := runBodyEditWithTracker(t, dir, cacheRoot, rules, ProjectHostState{}); res.FindingsCount != 0 {
			t.Fatalf("setup run findings = %d, want 0", res.FindingsCount)
		}
	}
	edited := "package demo\n\nfun useFresh(): Int {\n    // MARKER\n    return fresh()\n}\n"
	if err := os.WriteFile(usePath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	warm, path := runBodyEditWithTracker(t, dir, cacheRoot, rules, ProjectHostState{})
	clean, err := RunProject(context.Background(), ProjectInput{
		Args: ProjectArgs{
			Config:      config.NewConfig(),
			Paths:       []string{dir},
			ActiveRules: rules,
			Format:      "json",
			Version:     "test",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if clean.FindingsCount != 1 {
		t.Fatalf("test premise broken: clean findings = %d, want 1", clean.FindingsCount)
	}
	if warm.FindingsCount != clean.FindingsCount {
		t.Errorf("warm findings = %d, clean = %d (dispatchPath %q)", warm.FindingsCount, clean.FindingsCount, path)
	}
	if path != "cacheHitDeltaBundle" {
		t.Errorf("dispatchPath = %q, want cacheHitDeltaBundle", path)
	}

	// Removing the marker must drop the stale row on the untouched file.
	writeBodyEditProject(t, dir, useBefore)
	reverted, path := runBodyEditWithTracker(t, dir, cacheRoot, rules, ProjectHostState{})
	if reverted.FindingsCount != 0 {
		t.Errorf("after revert findings = %d, want 0 (stale row kept; dispatchPath %q)", reverted.FindingsCount, path)
	}
}
