package serve

import (
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
		daemon.AnalyzeProjectArgs{}, &first); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if first.Stats.FindingsBundleHit {
		t.Errorf("first call (no prior bundle) must report FindingsBundleHit=false; got %+v", first.Stats)
	}

	var second daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{}, &second); err != nil {
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

func TestAnalyzeProject_BodyOnlyEditReusesFindingsBundle(t *testing.T) {
	socket, state := startServerForTest(t)
	writeKotlinFile(t, state.root, "A.kt",
		"package demo\n\nclass A {\n    fun answer() = 1\n}\n")

	var first daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{}, &first); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if first.Stats.FindingsBundleHit {
		t.Fatalf("first call (no prior bundle) must report FindingsBundleHit=false; got %+v", first.Stats)
	}

	writeKotlinFile(t, state.root, "A.kt",
		"package demo\n\nclass A {\n    fun answer() = 22\n}\n")

	var second daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{}, &second); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if !second.Stats.FindingsBundleHit {
		t.Fatalf("body-only edit with stable structural fingerprint must report FindingsBundleHit=true; got %+v", second.Stats)
	}
	if second.Stats.PhaseTimingsMs.Dispatch != 0 || second.Stats.PhaseTimingsMs.CrossFile != 0 {
		t.Errorf("body-only bundle hit must bypass dispatch+crossfile; got dispatch=%dms crossfile=%dms",
			second.Stats.PhaseTimingsMs.Dispatch, second.Stats.PhaseTimingsMs.CrossFile)
	}
}
