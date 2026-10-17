package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// A project-scope rule that panics during the delta's cross-file phase must
// not be swallowed: the delta declines, the full path records the error,
// and the degraded result is not cached for the next identical request.
func TestRunProject_BodyOnlyEdit_CrossPhasePanicDeclinesDelta(t *testing.T) {
	dir := t.TempDir()
	cacheRoot := t.TempDir()
	usePath := writeBodyEditProject(t, dir, useBefore)
	altPath := filepath.Join(dir, "Alt.kt")

	boom := api.FakeRule("BoomWhenMarked", api.WithNeeds(api.NeedsParsedFiles), api.WithCheck(func(ctx *api.Context) {
		for _, f := range ctx.ParsedFiles {
			if strings.Contains(string(f.Content), "BOOM") {
				panic("boom")
			}
		}
		ctx.Emit(scanner.Finding{File: altPath, Line: 1, Col: 1, Message: "ok"})
	}))
	rules := []*api.Rule{boom}
	for i := 0; i < 2; i++ {
		res, _ := runBodyEditWithTracker(t, dir, cacheRoot, rules, ProjectHostState{})
		if res.FindingsCount != 1 {
			t.Fatalf("setup run findings = %d, want 1", res.FindingsCount)
		}
	}
	edited := "package demo\n\nfun useFresh(): Int {\n    // BOOM\n    return fresh()\n}\n"
	if err := os.WriteFile(usePath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	warm, path := runBodyEditWithTracker(t, dir, cacheRoot, rules, ProjectHostState{})
	if path == "cacheHitDeltaBundle" {
		t.Errorf("a cross-phase panic must decline the delta path")
	}
	if len(warm.Stats.Errors) == 0 {
		t.Errorf("rule panic was swallowed: no error recorded")
	}
	again, _ := runBodyEditWithTracker(t, dir, cacheRoot, rules, ProjectHostState{})
	if again.FindingsBundleHit {
		t.Errorf("degraded result was cached and replayed as a bundle hit")
	}
	if len(again.Stats.Errors) == 0 {
		t.Errorf("repeat request lost the recorded panic")
	}
}

// An index-backed cross-file rule reports on an untouched file using a line
// from the edited file's references. A blank-line edit keeps the structural
// fingerprint but moves that line; the delta must regenerate the row.
func TestRunProject_BodyOnlyEdit_CrossFileRuleRowOnOtherFileRefreshed(t *testing.T) {
	dir := t.TempDir()
	cacheRoot := t.TempDir()
	usePath := writeBodyEditProject(t, dir, useBefore)
	altPath := filepath.Join(dir, "Alt.kt")

	rule := api.FakeRule("RefLine", api.WithNeeds(api.NeedsCrossFile), api.WithCheck(func(ctx *api.Context) {
		if ctx.CodeIndex == nil {
			return
		}
		for _, ref := range ctx.CodeIndex.References {
			if ref.Name == "fresh" && strings.HasSuffix(ref.File, "Use.kt") {
				ctx.Emit(scanner.Finding{File: altPath, Line: 1, Col: 1, Message: fmt.Sprintf("fresh used at line %d", ref.Line)})
			}
		}
	}))
	rules := []*api.Rule{rule}
	for i := 0; i < 2; i++ {
		runBodyEditWithTracker(t, dir, cacheRoot, rules, ProjectHostState{})
	}
	edited := "package demo\n\nfun useFresh(): Int {\n\n\n\n    return fresh()\n}\n"
	if err := os.WriteFile(usePath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	warm, path := runBodyEditWithTracker(t, dir, cacheRoot, rules, ProjectHostState{})
	if path != "cacheHitDeltaBundle" {
		t.Fatalf("dispatchPath = %q, want cacheHitDeltaBundle", path)
	}
	if warm.FindingsCount != 1 {
		t.Fatalf("findings = %d, want exactly 1 (stale row kept or duplicated)", warm.FindingsCount)
	}
	if !strings.Contains(string(warm.JSON), "fresh used at line 7") {
		t.Errorf("row on the untouched file is stale; output: %s", warm.JSON)
	}
}
