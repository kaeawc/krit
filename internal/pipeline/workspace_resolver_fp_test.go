package pipeline

import (
	"testing"
)

// TestWorkspaceState_ResolverFingerprint_HitCacheUntilBump pins the
// version-counter contract: a cache hit short-circuits the build
// closure entirely; a watcher source-version bump rotates the
// version and the next call rebuilds. The closure-side check
// confirms the fast-path doesn't merely read+return — it must skip
// build() to recoup the ~135 ms saving from hashing 18k Kotlin files.
func TestWorkspaceState_ResolverFingerprint_HitCacheUntilBump(t *testing.T) {
	w := NewWorkspaceState("")
	var builds int
	build := func() string {
		builds++
		return "fp-v1"
	}

	first := w.ResolverFingerprint(build)
	if builds != 1 {
		t.Fatalf("first call: builds=%d, want 1", builds)
	}
	again := w.ResolverFingerprint(build)
	if builds != 1 {
		t.Errorf("second call after no bump must hit cache; builds=%d, want 1", builds)
	}
	if again != first {
		t.Errorf("cached fingerprint must match prior value; got %q, want %q", again, first)
	}

	w.BumpSourceMTimeVersion()

	third := w.ResolverFingerprint(func() string {
		builds++
		return "fp-v2"
	})
	if builds != 2 {
		t.Errorf("post-bump call must rebuild; builds=%d, want 2", builds)
	}
	if third != "fp-v2" {
		t.Errorf("post-bump return must reflect fresh build; got %q, want %q", third, "fp-v2")
	}
}

// TestWorkspaceState_ResolverFingerprint_ConcurrentBumpInvalidates
// mirrors the race semantics that motivate snapshotting the version
// BEFORE the build runs: a watcher event during the build must NOT
// result in a "clean" cached entry under a now-stale version.
func TestWorkspaceState_ResolverFingerprint_ConcurrentBumpInvalidates(t *testing.T) {
	w := NewWorkspaceState("")

	// Build closure simulates a watcher event mid-flight.
	fp := w.ResolverFingerprint(func() string {
		w.BumpSourceMTimeVersion()
		return "fp-during-race"
	})
	if fp != "fp-during-race" {
		t.Errorf("returned fingerprint must come from the build call; got %q", fp)
	}

	// The returned fingerprint is correct for the request, but the
	// cache MUST NOT have stored it under the stale version — the
	// next call should rebuild instead of returning a value the
	// watcher already invalidated.
	var builds int
	_ = w.ResolverFingerprint(func() string {
		builds++
		return "fp-post-race"
	})
	if builds == 0 {
		t.Errorf("post-race call must rebuild (the stale version mustn't survive)")
	}
}

// TestWorkspaceState_ResolverFingerprint_NilSafety mirrors the
// safety contract the rest of WorkspaceState's caches honor.
func TestWorkspaceState_ResolverFingerprint_NilSafety(t *testing.T) {
	var w *WorkspaceState
	called := 0
	got := w.ResolverFingerprint(func() string {
		called++
		return "fp-nil"
	})
	if got != "fp-nil" {
		t.Errorf("nil receiver must still return the build result; got %q", got)
	}
	if called != 1 {
		t.Errorf("nil receiver must call build; called=%d, want 1", called)
	}
}

// TestWorkspaceState_ResolverFingerprint_InvalidateAllClears confirms
// InvalidateAll drops the cached fingerprint alongside every other
// resident slot — symmetric with the javaSourceIndex / resolver slots.
func TestWorkspaceState_ResolverFingerprint_InvalidateAllClears(t *testing.T) {
	w := NewWorkspaceState("")
	var builds int
	build := func() string {
		builds++
		return "fp"
	}

	_ = w.ResolverFingerprint(build)
	_ = w.ResolverFingerprint(build)
	if builds != 1 {
		t.Fatalf("warm cache state: builds=%d, want 1", builds)
	}

	w.InvalidateAll()

	_ = w.ResolverFingerprint(build)
	if builds != 2 {
		t.Errorf("InvalidateAll must drop the cached fingerprint; builds=%d, want 2", builds)
	}
}
