package pipeline

import (
	"reflect"
	"sort"
	"testing"
)

// TestWorkspaceDrainDirty_StableUnderShuffle asserts DrainDirty
// returns paths in canonical (lexicographic) order regardless of the
// order in which Touch was called. The test exhausts every
// permutation of a small path set so a regression that reverts to
// map-iteration order has no statistical chance of passing.
//
// Mirrors the permutation-test scaffold introduced in PR #36
// (internal/scanner/index_workers_determinism_test.go).
func TestWorkspaceDrainDirty_StableUnderShuffle(t *testing.T) {
	paths := []string{
		"/src/main/kotlin/zzz.kt",
		"/src/main/kotlin/aaa.kt",
		"/src/main/kotlin/mmm.kt",
		"/src/main/kotlin/bbb.kt",
		"/src/main/kotlin/yyy.kt",
	}
	want := append([]string(nil), paths...)
	sort.Strings(want)

	for _, perm := range allPermutations(len(paths)) {
		w := NewWorkspaceState("/tmp/repo")
		for _, idx := range perm {
			w.Touch(paths[idx])
		}
		got := w.DrainDirty()
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("perm %v: got %v, want %v", perm, got, want)
		}
	}
}

// TestWorkspaceDrainDirty_ClearsAfterDrain confirms the second drain
// returns nil (dirty-set is cleared, not just snapshotted).
func TestWorkspaceDrainDirty_ClearsAfterDrain(t *testing.T) {
	w := NewWorkspaceState("/tmp/repo")
	w.Touch("/a.kt")
	w.Touch("/b.kt")

	if got := w.DrainDirty(); len(got) != 2 {
		t.Fatalf("first drain: got %d, want 2", len(got))
	}
	if got := w.DrainDirty(); got != nil {
		t.Fatalf("second drain: got %v, want nil", got)
	}

	// Touching after drain repopulates.
	w.Touch("/c.kt")
	got := w.DrainDirty()
	if !reflect.DeepEqual(got, []string{"/c.kt"}) {
		t.Fatalf("post-drain Touch: got %v, want [/c.kt]", got)
	}
}

// TestWorkspaceDrainDirty_DedupsRepeatedTouches confirms the dirty
// set is a set, not a list — repeated Touch on the same path
// collapses to a single entry.
func TestWorkspaceDrainDirty_DedupsRepeatedTouches(t *testing.T) {
	w := NewWorkspaceState("/tmp/repo")
	for i := 0; i < 100; i++ {
		w.Touch("/same.kt")
	}
	got := w.DrainDirty()
	if !reflect.DeepEqual(got, []string{"/same.kt"}) {
		t.Fatalf("got %v, want [/same.kt]", got)
	}
}

// TestWorkspaceDrainDirty_NilReceiverIsSafe — the workspace pointer
// can be nil at call sites that opt out of caching; both Touch and
// DrainDirty must handle that gracefully.
func TestWorkspaceDrainDirty_NilReceiverIsSafe(t *testing.T) {
	var w *WorkspaceState
	w.Touch("/x.kt")
	if got := w.DrainDirty(); got != nil {
		t.Fatalf("nil receiver DrainDirty: got %v, want nil", got)
	}
	if got := w.DirtyCount(); got != 0 {
		t.Fatalf("nil receiver DirtyCount: got %d, want 0", got)
	}
}

// TestWorkspaceDirtyCount_PeeksWithoutDraining confirms DirtyCount
// is a read-only inspector: Touched paths remain in the dirty-set
// after multiple DirtyCount calls, and only DrainDirty clears them.
func TestWorkspaceDirtyCount_PeeksWithoutDraining(t *testing.T) {
	w := NewWorkspaceState("/tmp/repo")
	w.Touch("/a.kt")
	w.Touch("/b.kt")

	if got := w.DirtyCount(); got != 2 {
		t.Fatalf("first DirtyCount: got %d, want 2", got)
	}
	if got := w.DirtyCount(); got != 2 {
		t.Fatalf("second DirtyCount: got %d, want 2 (peek must not drain)", got)
	}

	dirty := w.DrainDirty()
	if len(dirty) != 2 {
		t.Fatalf("DrainDirty: got %d, want 2", len(dirty))
	}
	if got := w.DirtyCount(); got != 0 {
		t.Fatalf("after Drain: DirtyCount = %d, want 0", got)
	}
}

// TestWorkspaceTouch_NormalizesPaths confirms Touch routes paths
// through the same normalization the parsed-entry cache uses, so a
// caller that touches "./a.kt" and a verb that parses "a.kt" agree
// on the dirty-set membership.
func TestWorkspaceTouch_NormalizesPaths(t *testing.T) {
	w := NewWorkspaceState("/tmp/repo")
	w.Touch("./foo.kt")
	w.Touch("foo.kt") // same key after normalization
	got := w.DrainDirty()
	if len(got) != 1 {
		t.Fatalf("expected 1 path after dedup; got %v", got)
	}
}

// TestWorkspaceInvalidateAllClearsDirty — InvalidateAll drops the
// parse cache; it should also drop any pending dirty-set so the next
// DrainDirty doesn't surface stale entries that no longer correspond
// to live cache.
func TestWorkspaceInvalidateAllClearsDirty(t *testing.T) {
	w := NewWorkspaceState("/tmp/repo")
	w.Touch("/a.kt")
	w.InvalidateAll()
	if got := w.DrainDirty(); got != nil {
		t.Fatalf("InvalidateAll should clear dirty; got %v", got)
	}
}

// allPermutations returns every permutation of [0, n). Used to
// exhaustively probe DrainDirty across all possible Touch orderings
// for small n.
func allPermutations(n int) [][]int {
	base := make([]int, n)
	for i := range base {
		base[i] = i
	}
	var result [][]int
	var permute func(arr []int, start int)
	permute = func(arr []int, start int) {
		if start == len(arr)-1 {
			cp := make([]int, len(arr))
			copy(cp, arr)
			result = append(result, cp)
			return
		}
		for i := start; i < len(arr); i++ {
			arr[start], arr[i] = arr[i], arr[start]
			permute(arr, start+1)
			arr[start], arr[i] = arr[i], arr[start]
		}
	}
	permute(base, 0)
	return result
}
