package serve

import (
	"encoding/json"
	"testing"

	"github.com/kaeawc/krit/internal/daemon"
)

// TestAnalyzeProject_WarmCallSetsAnalysisCacheDirty pins the
// wire-through contract that opts cacheCheck into the incremental
// CheckFilesIncremental path: a warm analyze (not the first cold
// call) must populate Host.AnalysisCacheDirty with the watcher's
// drained dirty set, even when that set is empty. nil signals
// "use the legacy stat-every-file CheckFiles path" — only correct
// for the first cold call where the watcher hasn't run yet.
func TestAnalyzeProject_WarmCallSetsAnalysisCacheDirty(t *testing.T) {
	socket, state := startServerForTest(t)
	writeKotlinFile(t, state.root, "A.kt",
		"package demo\n\nclass A {\n    fun a() {}\n}\n")

	// First call is "cold": warms the daemon but doesn't exercise
	// the incremental path (state.coldDone goes false→true here).
	var first daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{}, &first); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if first.Stats.Cold != true {
		t.Errorf("first call must report Cold=true; got %v", first.Stats.Cold)
	}

	// Second call is warm. The daemon should now treat the
	// (empty) dirty set as authoritative and route cacheCheck
	// through CheckFilesIncremental — surfacing through the
	// stats payload as DirtyFiles=0 on a clean run.
	var second daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{}, &second); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if second.Stats.Cold {
		t.Errorf("second call must report Cold=false; got %v", second.Stats.Cold)
	}
	if second.Stats.DirtyFiles != 0 {
		t.Errorf("clean warm call: DirtyFiles=%d, want 0", second.Stats.DirtyFiles)
	}
	// FindingsBundleHit on the second call confirms the warm
	// path actually fired (otherwise we'd be re-dispatching
	// rules without consulting the cache at all).
	if !second.Stats.FindingsBundleHit {
		t.Errorf("expected FindingsBundleHit=true on byte-identical second call; got %+v", second.Stats)
	}
	// Sanity: findings count is stable across warm calls.
	if first.Stats.FindingsCount != second.Stats.FindingsCount {
		t.Errorf("findings drift across warm calls: %d vs %d",
			first.Stats.FindingsCount, second.Stats.FindingsCount)
	}
	// And the formatted bytes are byte-identical (the bundle-hit
	// fast path returned cached output).
	var firstShape, secondShape map[string]json.RawMessage
	if err := json.Unmarshal(first.Findings, &firstShape); err != nil {
		t.Fatalf("decode first findings: %v", err)
	}
	if err := json.Unmarshal(second.Findings, &secondShape); err != nil {
		t.Fatalf("decode second findings: %v", err)
	}
	if string(firstShape["findings"]) != string(secondShape["findings"]) {
		t.Errorf("findings array drifted across warm calls")
	}
}
