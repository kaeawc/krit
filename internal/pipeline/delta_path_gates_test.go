package pipeline

import (
	"context"
	"os"
	"sync/atomic"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/perf"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Delta-path gate tests for #823: a fingerprint-stable body edit is served
// by re-dispatching the edited file, never by replaying the prior bundle.

// dispatchPathOf returns the "dispatchPath" label recorded by the run's
// tracker ("" when the dispatch layer was not reached).
func dispatchPathOf(tracker perf.Tracker) string {
	for _, e := range tracker.GetTimings() {
		if e.Name == "dispatchPath" {
			return e.Attributes["path"]
		}
	}
	return ""
}

func runBodyEditWithTracker(t *testing.T, dir, cacheRoot string, rules []*api.Rule, host ProjectHostState) (ProjectResult, string) {
	t.Helper()
	tracker := perf.New(true)
	host.Tracker = tracker
	host.FindingsBundleStore = scanner.DiskFindingsBundleStore{}
	host.FindingsBundleCacheRoot = cacheRoot
	res, err := RunProject(context.Background(), ProjectInput{
		Args: ProjectArgs{
			Config:      config.NewConfig(),
			Paths:       []string{dir},
			ActiveRules: rules,
			Format:      "json",
			Version:     "test",
		},
		Host: host,
	})
	if err != nil {
		t.Fatalf("RunProject: %v", err)
	}
	return res, dispatchPathOf(tracker)
}

func TestRunProject_BodyOnlyEdit_TakesDeltaPath(t *testing.T) {
	dir := t.TempDir()
	cacheRoot := t.TempDir()
	usePath := writeBodyEditProject(t, dir, useBefore)
	rules := []*api.Rule{bodyEditRule()}
	for i := 0; i < 2; i++ {
		runBodyEditWithTracker(t, dir, cacheRoot, rules, ProjectHostState{})
	}
	if err := os.WriteFile(usePath, []byte(useAfter), 0o644); err != nil {
		t.Fatal(err)
	}
	res, path := runBodyEditWithTracker(t, dir, cacheRoot, rules, ProjectHostState{})
	if path != "cacheHitDeltaBundle" {
		t.Errorf("dispatchPath = %q, want cacheHitDeltaBundle", path)
	}
	if res.FindingsBundleHit {
		t.Errorf("body edit must not be reported as a bundle hit")
	}
	if res.FindingsCount != 1 {
		t.Errorf("findings = %d, want 1 (added by the edit)", res.FindingsCount)
	}
}

// The delta path must decline (falling back to a correct full dispatch)
// when the oracle facts fingerprint differs: a changed fact for another
// file can change the edited file's findings.
func TestRunProject_BodyOnlyEdit_OracleFactChangeDeclinesDelta(t *testing.T) {
	dir := t.TempDir()
	cacheRoot := t.TempDir()
	usePath := writeBodyEditProject(t, dir, useBefore)
	rules := []*api.Rule{bodyEditRule()}
	facts := func(version string) ProjectHostState {
		return ProjectHostState{OracleBlobHash: func(string) string { return version }}
	}
	for i := 0; i < 2; i++ {
		runBodyEditWithTracker(t, dir, cacheRoot, rules, facts("v1"))
	}
	if err := os.WriteFile(usePath, []byte(useAfter), 0o644); err != nil {
		t.Fatal(err)
	}
	res, path := runBodyEditWithTracker(t, dir, cacheRoot, rules, facts("v2"))
	if path == "cacheHitDeltaBundle" || path == "cacheHitFullBundle" {
		t.Errorf("dispatchPath = %q: oracle fact change must not reuse the prior bundle", path)
	}
	if res.FindingsCount != 1 {
		t.Errorf("findings = %d, want 1", res.FindingsCount)
	}
}

// The delta path must decline when the CrossFile (structural) fingerprint
// differs, i.e. when the edit changes a declaration.
func TestRunProject_DeclarationEdit_DeclinesDelta(t *testing.T) {
	dir := t.TempDir()
	cacheRoot := t.TempDir()
	usePath := writeBodyEditProject(t, dir, useBefore)
	rules := []*api.Rule{bodyEditRule()}
	for i := 0; i < 2; i++ {
		runBodyEditWithTracker(t, dir, cacheRoot, rules, ProjectHostState{})
	}
	changed := useAfter + "\nfun another(): Int = 3\n"
	if err := os.WriteFile(usePath, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	res, path := runBodyEditWithTracker(t, dir, cacheRoot, rules, ProjectHostState{})
	if path == "cacheHitDeltaBundle" || path == "cacheHitFullBundle" {
		t.Errorf("dispatchPath = %q: structural change must not take the delta path", path)
	}
	if res.FindingsCount != 1 {
		t.Errorf("findings = %d, want 1", res.FindingsCount)
	}
}

// On the delta path an index-backed cross-file rule must still see the
// whole-project CodeIndex, not a one-file index.
func TestRunProject_BodyOnlyEdit_DeltaKeepsFullCodeIndex(t *testing.T) {
	dir := t.TempDir()
	cacheRoot := t.TempDir()
	usePath := writeBodyEditProject(t, dir, useBefore)

	var calls, nilIndex, indexedFiles int64
	crossRule := api.FakeRule("CrossIndexProbe", api.WithNeeds(api.NeedsCrossFile), api.WithCheck(func(ctx *api.Context) {
		atomic.AddInt64(&calls, 1)
		if ctx.CodeIndex == nil {
			atomic.AddInt64(&nilIndex, 1)
			return
		}
		atomic.StoreInt64(&indexedFiles, int64(len(ctx.CodeIndex.Files)))
	}))
	rules := []*api.Rule{bodyEditRule(), crossRule}
	for i := 0; i < 2; i++ {
		runBodyEditWithTracker(t, dir, cacheRoot, rules, ProjectHostState{})
	}
	if err := os.WriteFile(usePath, []byte(useAfter), 0o644); err != nil {
		t.Fatal(err)
	}
	atomic.StoreInt64(&calls, 0)
	_, path := runBodyEditWithTracker(t, dir, cacheRoot, rules, ProjectHostState{})
	if path != "cacheHitDeltaBundle" {
		t.Fatalf("dispatchPath = %q, want cacheHitDeltaBundle", path)
	}
	if atomic.LoadInt64(&calls) == 0 {
		t.Fatalf("cross-file rule did not run on the delta path")
	}
	if atomic.LoadInt64(&nilIndex) != 0 {
		t.Errorf("CodeIndex was nil on the delta path")
	}
	if got := atomic.LoadInt64(&indexedFiles); got != 3 {
		t.Errorf("CodeIndex.Files = %d, want 3 (whole project)", got)
	}
}
