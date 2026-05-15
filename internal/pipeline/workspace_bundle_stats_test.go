package pipeline

import "testing"

// TestWorkspaceState_BundleStatsClean_HitMissCycle exercises the
// memo's full lifecycle: empty → marked clean → hit → bumped → miss.
// The version-counter dance is the correctness gate, so this test
// is the contract: any future refactor that lets a stale memo
// survive a watcher event must fail here.
func TestWorkspaceState_BundleStatsClean_HitMissCycle(t *testing.T) {
	w := NewWorkspaceState("")
	const key = "abc123"

	if w.BundleStatsClean(key) {
		t.Fatalf("empty memo must report clean=false")
	}

	v := w.SourceMTimeVersion()
	w.MarkBundleStatsClean(key, v)

	if !w.BundleStatsClean(key) {
		t.Fatalf("post-mark same-version must report clean=true")
	}

	w.BumpSourceMTimeVersion()
	if w.BundleStatsClean(key) {
		t.Fatalf("post-bump same-key must report clean=false (stale version)")
	}

	// Mark again at the new version; should turn clean.
	w.MarkBundleStatsClean(key, w.SourceMTimeVersion())
	if !w.BundleStatsClean(key) {
		t.Fatalf("re-mark at new version must report clean=true")
	}
}

// TestWorkspaceState_BundleStatsClean_KeyIsolation confirms two
// distinct manifest keys don't share memo entries — a clean record
// for one project must not silently bypass the stat sweep for
// another.
func TestWorkspaceState_BundleStatsClean_KeyIsolation(t *testing.T) {
	w := NewWorkspaceState("")
	w.MarkBundleStatsClean("project-a", w.SourceMTimeVersion())
	if w.BundleStatsClean("project-b") {
		t.Errorf("BundleStatsClean leaked across keys")
	}
}

// TestWorkspaceState_BundleStatsClean_ConcurrentBumpInvalidates
// pins the race semantics that motivate the version-snapshot
// pattern in preparseBundleFingerprintTracked: if a watcher event
// fires AFTER we snapshot SourceMTimeVersion but BEFORE we Mark,
// the memo must not register clean — otherwise the next analyze
// would skip a stat sweep that's actually required.
func TestWorkspaceState_BundleStatsClean_ConcurrentBumpInvalidates(t *testing.T) {
	w := NewWorkspaceState("")
	const key = "k"
	preStat := w.SourceMTimeVersion()
	// Simulate a watcher event during the stat sweep.
	w.BumpSourceMTimeVersion()
	// Caller hands back the pre-stat version it captured.
	w.MarkBundleStatsClean(key, preStat)
	if w.BundleStatsClean(key) {
		t.Fatalf("memo recorded under stale pre-stat version must NOT report clean")
	}
}

// TestWorkspaceState_BundleStatsClean_NilSafety verifies the methods
// are safe on a nil receiver (the LSP/MCP test seam that constructs
// a nil WorkspaceState relies on this).
func TestWorkspaceState_BundleStatsClean_NilSafety(t *testing.T) {
	var w *WorkspaceState
	if w.BundleStatsClean("k") {
		t.Errorf("nil receiver must report clean=false")
	}
	w.MarkBundleStatsClean("k", 0) // must not panic
	w.BumpSourceMTimeVersion()     // must not panic
	if v := w.SourceMTimeVersion(); v != 0 {
		t.Errorf("nil receiver SourceMTimeVersion: got %d, want 0", v)
	}
}
