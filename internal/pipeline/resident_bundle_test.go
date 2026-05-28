package pipeline

import (
	"strconv"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// TestWorkspaceState_ResidentBundle_StoreAndLookup pins the basic
// store/lookup contract of the resident bundle cache: a successful
// StoreResidentBundle is visible to the next ResidentBundle call
// keyed on the same bundleKey.
func TestWorkspaceState_ResidentBundle_StoreAndLookup(t *testing.T) {
	w := &WorkspaceState{}
	cols := &scanner.FindingColumns{}

	if got := w.ResidentBundle("k"); got != nil {
		t.Errorf("empty cache must miss; got %v", got)
	}
	w.StoreResidentBundle("k", cols)
	if got := w.ResidentBundle("k"); got != cols {
		t.Errorf("stored bundle not returned; got %v want %v", got, cols)
	}
}

// TestWorkspaceState_ResidentBundle_NilSafety mirrors the rest of
// WorkspaceState's nil-tolerant API: methods on a nil receiver must
// not panic. Lets test fixtures and the CLI path keep zero-value
// hosts without special-casing the new slot.
func TestWorkspaceState_ResidentBundle_NilSafety(t *testing.T) {
	var w *WorkspaceState
	if got := w.ResidentBundle("k"); got != nil {
		t.Errorf("nil receiver must return nil; got %v", got)
	}
	w.StoreResidentBundle("k", &scanner.FindingColumns{}) // must not panic
}

// TestWorkspaceState_ResidentBundle_FIFOEvictsOldest pins the bound:
// inserting more than residentBundleCapacity distinct entries evicts
// the oldest. Without the bound, daemons servicing many warm+ABI
// cycles would accumulate decoded FindingColumns (10-20 MB each on
// kotlin-corpus scale).
func TestWorkspaceState_ResidentBundle_FIFOEvictsOldest(t *testing.T) {
	w := &WorkspaceState{}
	// Capacity+1 inserts — the first one must evict.
	for i := 0; i < residentBundleCapacity+1; i++ {
		w.StoreResidentBundle("k"+strconv.Itoa(i), &scanner.FindingColumns{})
	}
	if got := w.ResidentBundle("k0"); got != nil {
		t.Errorf("oldest entry must be evicted past capacity; got %v", got)
	}
	for i := 1; i <= residentBundleCapacity; i++ {
		key := "k" + strconv.Itoa(i)
		if got := w.ResidentBundle(key); got == nil {
			t.Errorf("entry %q must remain after FIFO eviction; got nil", key)
		}
	}
}

// TestWorkspaceState_ResidentBundle_RefreshDoesNotEvict pins the
// re-store semantics: writing to an existing key updates the value
// in place without rotating the FIFO. Used by the structural-replay
// path where the same priorKey may get refreshed across analyzes
// without bumping a "new" entry into the eviction list.
func TestWorkspaceState_ResidentBundle_RefreshDoesNotEvict(t *testing.T) {
	w := &WorkspaceState{}
	// Fill to capacity.
	for i := 0; i < residentBundleCapacity; i++ {
		w.StoreResidentBundle("k"+strconv.Itoa(i), &scanner.FindingColumns{})
	}
	// Refresh the oldest — must NOT trigger eviction of any entry.
	refreshed := &scanner.FindingColumns{}
	w.StoreResidentBundle("k0", refreshed)
	if got := w.ResidentBundle("k0"); got != refreshed {
		t.Errorf("refresh must update value; got %v", got)
	}
	for i := 0; i < residentBundleCapacity; i++ {
		key := "k" + strconv.Itoa(i)
		if got := w.ResidentBundle(key); got == nil {
			t.Errorf("refresh must NOT evict any entry; %q gone", key)
		}
	}
}

// TestWorkspaceState_ResidentBundle_NilValuesIgnored pins the
// defensive contract: StoreResidentBundle with a nil cols must NOT
// insert a sentinel into the map (would shadow legitimate future
// stores) and must NOT consume a FIFO slot.
func TestWorkspaceState_ResidentBundle_NilValuesIgnored(t *testing.T) {
	w := &WorkspaceState{}
	w.StoreResidentBundle("k", nil)
	if got := w.ResidentBundle("k"); got != nil {
		t.Errorf("nil store must not insert; got %v", got)
	}
	w.StoreResidentBundle("", &scanner.FindingColumns{})
	if got := w.ResidentBundle(""); got != nil {
		t.Errorf("empty-key store must not insert; got %v", got)
	}
}

// TestResidentBundleLookup_RoutesToHostHelper pins the CLI-path
// contract: when host.ResidentBundle isn't wired the helper returns
// nil without nil-deref, mirroring residentBundleStash on the write
// side. Tests covering CLI runs and bundle-test fixtures get
// zero-value hosts; the lookup must degrade gracefully.
func TestResidentBundleLookup_RoutesToHostHelper(t *testing.T) {
	if got := residentBundleLookup(ProjectHostState{}, "k"); got != nil {
		t.Errorf("nil ResidentBundle must return nil; got %v", got)
	}
	if got := residentBundleLookup(ProjectHostState{}, ""); got != nil {
		t.Errorf("empty key must return nil; got %v", got)
	}
}
