package serve

import (
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/daemon"
)

// TestAnalyzeProject_BundleHitOnIdenticalSecondCall pins the
// contract that the daemon's whole-run findings cache fires on the
// second of two byte-identical analyze-project calls. A regression
// in the bundle key derivation would surface here rather than at
// benchmark time.
func TestAnalyzeProject_BundleHitOnIdenticalSecondCall(t *testing.T) {
	socket, state := startServerForTest(t)
	writeKotlinFile(t, state.root, "A.kt",
		"package demo\n\nclass A {\n    fun a() {}\n}\n")
	writeKotlinFile(t, state.root, "B.kt",
		"package demo\n\nclass B {\n    fun b() {}\n}\n")

	var first daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{NoFir: true}, &first); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if first.Stats.FindingsBundleHit {
		t.Errorf("first call (no prior bundle) must report FindingsBundleHit=false; got %+v", first.Stats)
	}

	var second daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{NoFir: true}, &second); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if !second.Stats.FindingsBundleHit {
		t.Errorf("identical second call must report FindingsBundleHit=true; got %+v\n"+
			"a false here means the bundle key drifts across identical calls",
			second.Stats)
	}
	if second.Stats.FindingsCount != first.Stats.FindingsCount {
		t.Errorf("bundle hit must replay the same finding count: first=%d second=%d",
			first.Stats.FindingsCount, second.Stats.FindingsCount)
	}
}

// TestAnalyzeProject_BodyOnlyEditRedispatchesEditedFile pins #823: a
// body-only edit the daemon observes (watcher Touch + mtime bump) must not
// replay the prior bundle. The edited file is re-dispatched via the delta
// path, so a finding added inside the body appears and the result matches
// a fresh daemon scanning the edited tree.
func TestAnalyzeProject_BodyOnlyEditRedispatchesEditedFile(t *testing.T) {
	const before = "package demo\n\nclass A {\n    fun answer(): Int {\n        val x: Int? = 1\n        return x ?: 0\n    }\n}\n"
	const after = "package demo\n\nclass A {\n    fun answer(): Int {\n        val x: Int? = 1\n        return x!!\n    }\n}\n"

	socket, state := startServerForTest(t)
	writeKotlinFile(t, state.root, "A.kt", before)

	var first daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{NoFir: true}, &first); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if first.Stats.FindingsBundleHit {
		t.Fatalf("first call (no prior bundle) must report FindingsBundleHit=false; got %+v", first.Stats)
	}

	writeKotlinFile(t, state.root, "A.kt", after)
	// What the file watcher does for a .kt edit (scheduleKotlinInvalidate).
	edited := filepath.Join(state.root, "A.kt")
	state.workspace.Invalidate(edited)
	state.workspace.InvalidateCodeIndex()
	state.workspace.InvalidateDependents()
	state.workspace.InvalidateResolver()
	state.workspace.Touch(edited)
	state.workspace.BumpSourceMTimeVersion()

	var second daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{NoFir: true}, &second); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if second.Stats.DirtyFiles != 1 {
		t.Fatalf("edit must be observed as dirty; got DirtyFiles=%d", second.Stats.DirtyFiles)
	}
	if second.Stats.FindingsBundleHit {
		t.Fatalf("observed body edit must not replay the prior bundle; got %+v", second.Stats)
	}

	cleanSocket, cleanState := startServerForTest(t)
	writeKotlinFile(t, cleanState.root, "A.kt", after)
	var clean daemon.AnalyzeProjectResult
	if err := daemon.Call(cleanSocket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{NoFir: true}, &clean); err != nil {
		t.Fatalf("clean call: %v", err)
	}
	if clean.Stats.FindingsCount <= first.Stats.FindingsCount {
		t.Fatalf("test premise broken: edit should add a finding (before=%d clean=%d)",
			first.Stats.FindingsCount, clean.Stats.FindingsCount)
	}
	if second.Stats.FindingsCount != clean.Stats.FindingsCount {
		t.Errorf("warm findings = %d, clean = %d\nfirst: %s\nwarm: %s\nclean: %s", second.Stats.FindingsCount, clean.Stats.FindingsCount, first.Findings, second.Findings, clean.Findings)
	}
}
