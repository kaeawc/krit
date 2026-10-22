package serve

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/devjar"
	"github.com/kaeawc/krit/internal/pipeline"
)

func TestDistrustEmptyDirtySet(t *testing.T) {
	host := pipeline.ProjectHostState{
		ResidentFiles:      pipeline.NewWorkspaceState(t.TempDir()),
		AnalysisCacheDirty: []string{},
	}
	distrustEmptyDirtySet(&host)
	if host.AnalysisCacheDirty != nil {
		t.Errorf("AnalysisCacheDirty = %#v, want nil so every cache entry is validated", host.AnalysisCacheDirty)
	}
	if host.ResidentFiles != nil {
		t.Errorf("ResidentFiles still set, want files parsed from disk")
	}
}

// TestAnalyzeProject_CallInsideEditWindowDoesNotPoisonLaterCall reproduces the
// #832 window without a real watcher. On the first event of a save the watcher
// bumps the source-mtime version and drops the file's resident state, and only
// then touches the path. A call that lands between the two drains an empty
// dirty set. It must still report the edited file's findings, and the call
// after the touch must agree: before the fix the first call trusted the cached
// per-file findings of the edited file and saved them under the new content's
// fingerprint, which the second call then replayed. The two-writes-in-a-burst
// stage of TestAnalyzeProject_EditRacingWatcherEventIsSeenAfterEvent covers the
// stale resident tree of a window the watcher has not invalidated yet.
func TestAnalyzeProject_CallInsideEditWindowDoesNotPoisonLaterCall(t *testing.T) {
	// No JVM helper: this is about source findings, and a shared oracle
	// daemon would make the test depend on what else is running.
	t.Setenv(devjar.DirEnv, t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	socket, state := startServerForTest(t)
	path := writeKotlinFile(t, state.root, "Lib.kt", "package demo\n\nclass Box\n")
	call := func() daemon.AnalyzeProjectResult {
		t.Helper()
		var res daemon.AnalyzeProjectResult
		if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
			daemon.AnalyzeProjectArgs{NoFir: true, AllRules: true}, &res); err != nil {
			t.Fatalf("analyze-project: %v", err)
		}
		return res
	}
	hasFindingAtLine := func(res daemon.AnalyzeProjectResult, line string) bool {
		return strings.Contains(string(res.Findings), `"line":`+line+`,`)
	}

	call() // cold
	control := call()
	if !control.Stats.FindingsBundleHit {
		t.Fatalf("no-op control did not hit the findings bundle: %+v", control.Stats)
	}
	if !hasFindingAtLine(control, "3") {
		t.Fatalf("seed run has no finding on the class declaration at line 3:\n%s", control.Findings)
	}

	// Save: two blank lines move the class to line 5.
	writeKotlinFile(t, state.root, filepath.Base(path), "package demo\n\n\n\nclass Box\n")
	state.workspace.BumpSourceMTimeVersion()
	state.workspace.Invalidate(path)

	raced := call()
	if raced.Stats.DirtyFiles != 0 {
		t.Fatalf("the window call must drain an empty dirty set, got %+v", raced.Stats)
	}
	if !hasFindingAtLine(raced, "5") || hasFindingAtLine(raced, "3") {
		t.Errorf("call inside the edit window reports stale findings:\n%s", raced.Findings)
	}

	state.workspace.Touch(path)
	after := call()
	if after.Stats.DirtyFiles != 1 {
		t.Fatalf("the call after the touch must drain the edit, got %+v", after.Stats)
	}
	if !hasFindingAtLine(after, "5") || hasFindingAtLine(after, "3") {
		t.Errorf("call after the watcher reported the edit replays stale findings:\n%s", after.Findings)
	}
}
