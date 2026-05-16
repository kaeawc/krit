package pipeline

import (
	"testing"

	"github.com/kaeawc/krit/internal/javafacts"
)

// TestWorkspaceState_JavaSourceIndex_HitCacheUntilBump pins the
// version-counter contract: a cache hit short-circuits the build
// closure entirely; a watcher bump rotates the version and the
// next call rebuilds. The closure-side check confirms the
// fast-path doesn't merely read+return — it must skip build()
// to recoup the ~100 ms saving.
func TestWorkspaceState_JavaSourceIndex_HitCacheUntilBump(t *testing.T) {
	w := NewWorkspaceState("")
	var builds int
	build := func() *javafacts.SourceIndex {
		builds++
		return &javafacts.SourceIndex{}
	}

	first := w.JavaSourceIndex(build)
	if builds != 1 {
		t.Fatalf("first call: builds=%d, want 1", builds)
	}
	again := w.JavaSourceIndex(build)
	if builds != 1 {
		t.Errorf("second call after no bump must hit cache; builds=%d, want 1", builds)
	}
	if again != first {
		t.Errorf("cached entry must return identical pointer; got %p, want %p", again, first)
	}

	w.BumpJavaSourceVersion()

	third := w.JavaSourceIndex(build)
	if builds != 2 {
		t.Errorf("post-bump call must rebuild; builds=%d, want 2", builds)
	}
	if third == first {
		t.Errorf("post-bump return must be a fresh pointer")
	}
}

// TestWorkspaceState_JavaSourceIndex_ConcurrentBumpInvalidates
// mirrors the race semantics that motivate snapshotting the
// version BEFORE the build runs: a watcher event during the build
// must NOT result in a "clean" cached entry under a now-stale
// version.
func TestWorkspaceState_JavaSourceIndex_ConcurrentBumpInvalidates(t *testing.T) {
	w := NewWorkspaceState("")

	// Build closure simulates a watcher event mid-flight.
	idx := w.JavaSourceIndex(func() *javafacts.SourceIndex {
		w.BumpJavaSourceVersion()
		return &javafacts.SourceIndex{}
	})

	// The returned index is correct for the request, but the cache
	// MUST NOT have stored it under the stale version — the next
	// call should rebuild instead of returning a value the watcher
	// already invalidated.
	var builds int
	_ = w.JavaSourceIndex(func() *javafacts.SourceIndex {
		builds++
		return idx
	})
	if builds == 0 {
		t.Errorf("post-race call must rebuild (the stale version mustn't survive)")
	}
}

// TestWorkspaceState_JavaSourceIndex_NilSafety mirrors the safety
// contract the rest of WorkspaceState's caches honor.
func TestWorkspaceState_JavaSourceIndex_NilSafety(t *testing.T) {
	var w *WorkspaceState
	called := 0
	got := w.JavaSourceIndex(func() *javafacts.SourceIndex {
		called++
		return &javafacts.SourceIndex{}
	})
	if got == nil {
		t.Errorf("nil receiver must still return the build result")
	}
	if called != 1 {
		t.Errorf("nil receiver must call build; called=%d, want 1", called)
	}
	w.BumpJavaSourceVersion() // must not panic
}
