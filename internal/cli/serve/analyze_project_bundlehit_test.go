package serve

import (
	"testing"

	"github.com/kaeawc/krit/internal/daemon"
)

// TestAnalyzeProject_BundleHitOnIdenticalSecondCall is observability
// for #139: the daemon's whole-run findings cache should fire on the
// second of two byte-identical analyze-project calls. If it doesn't,
// the warm path is paying full dispatch+cross-file cost every call —
// the leading hypothesis on the kotlin-corpus regression.
//
// The test pins the contract through the wire format
// (AnalyzeProjectStats.FindingsBundleHit) so a regression in the
// bundle key derivation surfaces in CI rather than at benchmark
// time.
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
			"if this fails, the bundle key is drifting across calls — see #139",
			second.Stats)
	}
	if second.Stats.FindingsCount != first.Stats.FindingsCount {
		t.Errorf("bundle hit must replay the same finding count: first=%d second=%d",
			first.Stats.FindingsCount, second.Stats.FindingsCount)
	}
}
