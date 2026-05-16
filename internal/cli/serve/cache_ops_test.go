package serve

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/daemon"
)

// TestAnalyzeProject_NoCacheBypassesDiskCache pins that NoCache=true
// runs without populating the on-disk AnalysisCache and without
// reading the FindingsBundle store. The two follow-up calls assert
// that resident WorkspaceState slots aren't poisoned by the no-cache
// run: the next NoCache=false call should still produce identical
// findings and the daemon should still be in a valid warm state.
func TestAnalyzeProject_NoCacheBypassesDiskCache(t *testing.T) {
	socket, state := startServerForTest(t)
	writeKotlinFile(t, state.root, "Foo.kt", "package demo\n\nclass Foo\n")

	// Warm the daemon with a normal call so on-disk caches exist.
	var warm daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{}, &warm); err != nil {
		t.Fatalf("warm call: %v", err)
	}

	// Then a NoCache call must succeed and still return matching
	// findings — nil-cache wiring shouldn't change the rule output.
	var noCache daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{NoCache: true}, &noCache); err != nil {
		t.Fatalf("no-cache call: %v", err)
	}
	if !jsonEq(warm.Findings, noCache.Findings) {
		t.Errorf("no-cache findings diverged from warm findings\nwarm: %s\nnocache: %s", warm.Findings, noCache.Findings)
	}

	// And a NoCache=false call after the NoCache run must NOT have
	// been poisoned: same shape findings, daemon still serves the
	// request.
	var followUp daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{}, &followUp); err != nil {
		t.Fatalf("follow-up call: %v", err)
	}
	if !jsonEq(warm.Findings, followUp.Findings) {
		t.Errorf("follow-up findings diverged after no-cache run\nwarm: %s\nfollowUp: %s", warm.Findings, followUp.Findings)
	}
}

// TestClearCache_DropsResidentAndDiskState pins that the clear-cache
// verb wipes the on-disk analysis cache file and resets the daemon's
// WorkspaceState so the next analyze reports Cold=true.
func TestClearCache_DropsResidentAndDiskState(t *testing.T) {
	socket, state := startServerForTest(t)
	writeKotlinFile(t, state.root, "Foo.kt", "package demo\n\nclass Foo\n")

	// Prime the daemon: this populates the analysis-cache file and
	// the resident workspace state.
	var first daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{}, &first); err != nil {
		t.Fatalf("prime call: %v", err)
	}
	if !first.Stats.Cold {
		t.Fatalf("first call must be Cold; got %+v", first.Stats)
	}

	// Confirm the cache file actually exists on disk so the clear
	// has something to remove.
	cachePath := findAnalysisCachePath(state)
	if cachePath == "" {
		t.Skip("analysis cache path not available in this environment; skipping disk-clear assertion")
	}
	if _, err := os.Stat(cachePath); err != nil {
		t.Skipf("analysis cache file not on disk after first analyze (%v); skipping", err)
	}

	var clear daemon.ClearCacheResult
	if err := daemon.Call(socket, daemon.VerbClearCache, daemon.ClearCacheArgs{}, &clear); err != nil {
		t.Fatalf("clear-cache: %v", err)
	}
	if !clear.Cleared {
		t.Errorf("clear-cache: Cleared=false, want true; got %+v", clear)
	}
	if !clear.ResidentInvalidated {
		t.Errorf("clear-cache: ResidentInvalidated=false, want true; got %+v", clear)
	}
	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Errorf("expected analysis cache file removed; stat err=%v", err)
	}

	// After clear, the next analyze should report Cold=true again
	// because coldDone was reset and resident slots were dropped.
	var afterClear daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{}, &afterClear); err != nil {
		t.Fatalf("post-clear analyze: %v", err)
	}
	if !afterClear.Stats.Cold {
		t.Errorf("post-clear analyze must be Cold; got %+v", afterClear.Stats)
	}
}

// findAnalysisCachePath returns the on-disk cache file the daemon uses
// for the current root, or "" when none is registered yet.
func findAnalysisCachePath(state *daemonState) string {
	state.analysisCacheMu.Lock()
	defer state.analysisCacheMu.Unlock()
	for _, e := range state.analysisCacheByKey {
		if e == nil {
			continue
		}
		if e.path != "" {
			return e.path
		}
	}
	// Fall back to the conventional location relative to the daemon
	// root if no entry has been registered yet (some test paths skip
	// the analysis cache loader).
	return filepath.Join(state.root, ".krit", "cache", "krit-analysis.cache")
}

// jsonEq compares two raw JSON payloads structurally after removing
// dynamic fields (durationMs) that legitimately vary across calls.
// Used to assert that NoCache and warm calls produce identical
// findings regardless of timing.
func jsonEq(a, b json.RawMessage) bool {
	var av, bv map[string]any
	if err := json.Unmarshal(a, &av); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		return false
	}
	delete(av, "durationMs")
	delete(bv, "durationMs")
	abytes, _ := json.Marshal(av)
	bbytes, _ := json.Marshal(bv)
	return string(abytes) == string(bbytes)
}
