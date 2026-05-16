package pipeline

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// TestWorkspaceState_CodeIndexSnapshot_RoundTrip pins the basic
// store/load contract: a stored index plus meta round-trips through
// LoadCodeIndexSnapshot until the next Store. The watcher's
// fingerprint-keyed CodeIndex slot is unrelated — this snapshot must
// survive its invalidations.
func TestWorkspaceState_CodeIndexSnapshot_RoundTrip(t *testing.T) {
	w := NewWorkspaceState("")

	if idx, meta, ok := w.LoadCodeIndexSnapshot(); ok || idx != nil || meta.Fingerprint != "" {
		t.Fatalf("empty workspace: want (nil, zero, false), got (%p, %q, %v)", idx, meta.Fingerprint, ok)
	}

	first := &scanner.CodeIndex{Fingerprint: "fp-1"}
	firstMeta := scanner.CrossFileCacheMeta{Fingerprint: "fp-1", KotlinFiles: 100}
	w.StoreCodeIndexSnapshot(first, firstMeta)

	gotIdx, gotMeta, ok := w.LoadCodeIndexSnapshot()
	if !ok {
		t.Fatalf("after Store: want ok=true")
	}
	if gotIdx != first {
		t.Errorf("load returned different pointer: want %p, got %p", first, gotIdx)
	}
	if gotMeta.Fingerprint != "fp-1" || gotMeta.KotlinFiles != 100 {
		t.Errorf("meta mismatch: got %+v", gotMeta)
	}

	// A second Store overwrites — the prior pointer is replaced, not
	// retained. The daemon only ever needs the most recent snapshot.
	second := &scanner.CodeIndex{Fingerprint: "fp-2"}
	w.StoreCodeIndexSnapshot(second, scanner.CrossFileCacheMeta{Fingerprint: "fp-2"})
	gotIdx, _, _ = w.LoadCodeIndexSnapshot()
	if gotIdx != second {
		t.Errorf("expected replaced pointer; got %p, want %p", gotIdx, second)
	}
}

// TestWorkspaceState_CodeIndexSnapshot_SurvivesCodeIndexInvalidation
// pins the cross-slot independence contract: the watcher's
// InvalidateCodeIndex drops the fingerprint-keyed CodeIndex slot, but
// the snapshot must NOT be cleared — that's the whole point of the
// snapshot being a separate slot.
func TestWorkspaceState_CodeIndexSnapshot_SurvivesCodeIndexInvalidation(t *testing.T) {
	w := NewWorkspaceState("")
	idx := &scanner.CodeIndex{Fingerprint: "fp"}
	w.StoreCodeIndexSnapshot(idx, scanner.CrossFileCacheMeta{Fingerprint: "fp"})

	w.InvalidateCodeIndex()

	if got, _, ok := w.LoadCodeIndexSnapshot(); !ok || got != idx {
		t.Errorf("snapshot must survive InvalidateCodeIndex; got (%p, %v) want (%p, true)", got, ok, idx)
	}
}

// TestWorkspaceState_CodeIndexSnapshot_InvalidateAllClears confirms
// the snapshot DOES drop when the whole workspace is reset — that's
// the codeIndex-slot's symmetric correctness contract.
func TestWorkspaceState_CodeIndexSnapshot_InvalidateAllClears(t *testing.T) {
	w := NewWorkspaceState("")
	w.StoreCodeIndexSnapshot(&scanner.CodeIndex{}, scanner.CrossFileCacheMeta{})

	w.InvalidateAll()

	if got, _, ok := w.LoadCodeIndexSnapshot(); ok || got != nil {
		t.Errorf("InvalidateAll must clear snapshot; got (%p, %v)", got, ok)
	}
}

// TestWorkspaceState_CodeIndexSnapshot_NilSafety mirrors the safety
// contract the rest of WorkspaceState's caches honor.
func TestWorkspaceState_CodeIndexSnapshot_NilSafety(t *testing.T) {
	var w *WorkspaceState
	idx, _, ok := w.LoadCodeIndexSnapshot()
	if idx != nil || ok {
		t.Errorf("nil receiver Load must return (nil, _, false); got (%p, %v)", idx, ok)
	}
	w.StoreCodeIndexSnapshot(&scanner.CodeIndex{}, scanner.CrossFileCacheMeta{}) // must not panic
}

// TestWorkspaceState_CodeIndexSnapshot_NilClear lets the daemon drop
// the snapshot explicitly by passing nil.
func TestWorkspaceState_CodeIndexSnapshot_NilClear(t *testing.T) {
	w := NewWorkspaceState("")
	w.StoreCodeIndexSnapshot(&scanner.CodeIndex{}, scanner.CrossFileCacheMeta{Fingerprint: "fp"})
	w.StoreCodeIndexSnapshot(nil, scanner.CrossFileCacheMeta{})
	if _, _, ok := w.LoadCodeIndexSnapshot(); ok {
		t.Errorf("Store(nil, _) must clear the snapshot")
	}
}
