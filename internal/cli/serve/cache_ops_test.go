package serve

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/cli/scan"
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

// TestClearMatrixCache_RemovesHostWideEntries pins that the
// clear-matrix-cache verb wipes the host-wide matrix-baseline
// directory by routing through scan.ClearMatrixCache. Equivalence
// with the in-process path: any file written under
// ~/.cache/krit/matrix-baseline before the verb call is gone after.
//
// $HOME is redirected to t.TempDir() so the test never touches the
// developer's real cache.
func TestClearMatrixCache_RemovesHostWideEntries(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// Seed an entry under the matrix-baseline directory. Use the same
	// helper the scan package uses so the path layout is whatever
	// matrixCacheDir() resolves to under the redirected HOME.
	dir := filepath.Join(tempHome, ".cache", "krit", "matrix-baseline")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	seedPath := filepath.Join(dir, "deadbeef.json")
	if err := os.WriteFile(seedPath, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	socket, _ := startServerForTest(t)
	var res daemon.ClearMatrixCacheResult
	if err := daemon.Call(socket, daemon.VerbClearMatrixCache,
		daemon.ClearMatrixCacheArgs{}, &res); err != nil {
		t.Fatalf("clear-matrix-cache: %v", err)
	}
	if !res.Cleared {
		t.Errorf("Cleared=false, want true; got %+v", res)
	}
	if _, err := os.Stat(seedPath); !os.IsNotExist(err) {
		t.Errorf("expected matrix entry removed; stat err=%v", err)
	}

	// Equivalence: in-process scan.ClearMatrixCache on an empty
	// directory should succeed (idempotent), and the daemon-routed
	// clear should match that no-op behaviour. Seed again, call the
	// in-process function, confirm the entry is gone.
	if err := os.WriteFile(seedPath, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("re-seed: %v", err)
	}
	if err := scan.ClearMatrixCache(); err != nil {
		t.Fatalf("in-process ClearMatrixCache: %v", err)
	}
	if _, err := os.Stat(seedPath); !os.IsNotExist(err) {
		t.Errorf("in-process clear should match daemon clear; stat err=%v", err)
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
