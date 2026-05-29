package pipeline

import (
	"strconv"
	"testing"
)

func manifestForTest(bundleKey string) resourceSourceBundleManifest {
	return resourceSourceBundleManifest{
		Version:   resourceSourceBundleManifestVersion,
		Key:       "ignored", // ResidentResourceSourceManifest keys on the arg, not this
		BundleKey: bundleKey,
		Hashes:    map[string]string{"/src/A.kt": "aaaa"},
	}
}

// TestWorkspaceState_ResidentResourceManifest_StoreAndLookup pins the basic
// store/lookup contract: a stored manifest is visible to the next lookup keyed
// on the same key.
func TestWorkspaceState_ResidentResourceManifest_StoreAndLookup(t *testing.T) {
	w := &WorkspaceState{}
	if _, ok := w.ResidentResourceSourceManifest("k"); ok {
		t.Error("empty cache must miss")
	}
	w.StoreResidentResourceSourceManifest("k", manifestForTest("b"))
	got, ok := w.ResidentResourceSourceManifest("k")
	if !ok {
		t.Fatal("stored manifest not returned")
	}
	if got.BundleKey != "b" {
		t.Errorf("BundleKey = %q, want b", got.BundleKey)
	}
}

// TestWorkspaceState_ResidentResourceManifest_NilSafety mirrors the rest of
// WorkspaceState's nil-tolerant API: methods on a nil receiver must not panic.
func TestWorkspaceState_ResidentResourceManifest_NilSafety(t *testing.T) {
	var w *WorkspaceState
	if _, ok := w.ResidentResourceSourceManifest("k"); ok {
		t.Error("nil receiver must miss")
	}
	w.StoreResidentResourceSourceManifest("k", manifestForTest("b")) // must not panic
}

// TestWorkspaceState_ResidentResourceManifest_EmptyKeyIgnored pins the guard:
// an empty key neither stores nor looks up.
func TestWorkspaceState_ResidentResourceManifest_EmptyKeyIgnored(t *testing.T) {
	w := &WorkspaceState{}
	w.StoreResidentResourceSourceManifest("", manifestForTest("b"))
	if _, ok := w.ResidentResourceSourceManifest(""); ok {
		t.Error("empty-key store must not insert")
	}
}

// TestWorkspaceState_ResidentResourceManifest_FIFOEvictsOldest pins the bound:
// inserting past residentResourceManifestCapacity evicts the oldest entry.
func TestWorkspaceState_ResidentResourceManifest_FIFOEvictsOldest(t *testing.T) {
	w := &WorkspaceState{}
	for i := 0; i < residentResourceManifestCapacity+1; i++ {
		w.StoreResidentResourceSourceManifest("k"+strconv.Itoa(i), manifestForTest("b"))
	}
	if _, ok := w.ResidentResourceSourceManifest("k0"); ok {
		t.Error("oldest entry must be evicted past capacity")
	}
	for i := 1; i <= residentResourceManifestCapacity; i++ {
		key := "k" + strconv.Itoa(i)
		if _, ok := w.ResidentResourceSourceManifest(key); !ok {
			t.Errorf("entry %q must remain after FIFO eviction", key)
		}
	}
}

// TestWorkspaceState_ResidentResourceManifest_RefreshDoesNotEvict pins the
// re-store semantics: writing an existing key updates the value in place
// without rotating the FIFO. This is the warm-cycle case where the same
// path-set-addressed key is re-saved every analyze.
func TestWorkspaceState_ResidentResourceManifest_RefreshDoesNotEvict(t *testing.T) {
	w := &WorkspaceState{}
	for i := 0; i < residentResourceManifestCapacity; i++ {
		w.StoreResidentResourceSourceManifest("k"+strconv.Itoa(i), manifestForTest("orig"))
	}
	w.StoreResidentResourceSourceManifest("k0", manifestForTest("refreshed"))
	got, ok := w.ResidentResourceSourceManifest("k0")
	if !ok || got.BundleKey != "refreshed" {
		t.Fatalf("refresh must update value in place; got ok=%v bundleKey=%q", ok, got.BundleKey)
	}
	for i := 0; i < residentResourceManifestCapacity; i++ {
		key := "k" + strconv.Itoa(i)
		if _, ok := w.ResidentResourceSourceManifest(key); !ok {
			t.Errorf("refresh must NOT evict any entry; %q gone", key)
		}
	}
}

// TestWorkspaceState_ResidentResourceManifest_InvalidateAllClears verifies the
// resident manifest mirror is dropped on InvalidateAll, so a config/rule change
// that resets the workspace cannot serve a stale manifest into the delta path.
func TestWorkspaceState_ResidentResourceManifest_InvalidateAllClears(t *testing.T) {
	w := NewWorkspaceState("") // InvalidateAll touches the parsedLRU list
	w.StoreResidentResourceSourceManifest("k", manifestForTest("b"))
	if _, ok := w.ResidentResourceSourceManifest("k"); !ok {
		t.Fatal("precondition: manifest should be present before InvalidateAll")
	}
	w.InvalidateAll()
	if _, ok := w.ResidentResourceSourceManifest("k"); ok {
		t.Error("InvalidateAll must clear the resident manifest mirror")
	}
}
