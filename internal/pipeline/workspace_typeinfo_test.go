package pipeline

import (
	"testing"

	"github.com/kaeawc/krit/internal/typeinfer"
)

// TestWorkspaceState_FileTypeInfoCache_StoreAndLookup pins the
// resident cache contract: store + lookup returns the same pointer,
// distinct paths don't collide, idempotent stores keep the first
// pointer so identity-based comparisons survive across analyses.
func TestWorkspaceState_FileTypeInfoCache_StoreAndLookup(t *testing.T) {
	w := NewWorkspaceState("")

	if got, ok := w.LookupFileTypeInfo("/tmp/A.kt"); ok || got != nil {
		t.Fatalf("empty cache: got=%v ok=%v, want nil/false", got, ok)
	}

	first := &typeinfer.FileTypeInfo{Path: "/tmp/A.kt"}
	w.StoreFileTypeInfo("/tmp/A.kt", first)
	got, ok := w.LookupFileTypeInfo("/tmp/A.kt")
	if !ok {
		t.Fatalf("post-store lookup: ok=false")
	}
	if got != first {
		t.Errorf("post-store lookup returned different pointer")
	}

	// Distinct path: must not collide.
	if _, ok := w.LookupFileTypeInfo("/tmp/B.kt"); ok {
		t.Errorf("LookupFileTypeInfo leaked across paths")
	}

	// Idempotent store: a later Store under the same path keeps the
	// first pointer (matches the ResidentFiles cache's identity
	// semantics — rules compare *FileTypeInfo by pointer).
	second := &typeinfer.FileTypeInfo{Path: "/tmp/A.kt"}
	w.StoreFileTypeInfo("/tmp/A.kt", second)
	got2, _ := w.LookupFileTypeInfo("/tmp/A.kt")
	if got2 != first {
		t.Errorf("second Store replaced pointer; want stable identity")
	}
}

// TestWorkspaceState_FileTypeInfoCache_InvalidateDropsEntry verifies
// the watcher hook: Invalidate(path) clears the typeInfo entry
// alongside the parsed-tree entry, so the next analyze re-indexes
// the file. Without this hook a stale FileTypeInfo would survive
// across edits the watcher saw.
func TestWorkspaceState_FileTypeInfoCache_InvalidateDropsEntry(t *testing.T) {
	w := NewWorkspaceState("")
	info := &typeinfer.FileTypeInfo{Path: "/tmp/A.kt"}
	w.StoreFileTypeInfo("/tmp/A.kt", info)
	w.Invalidate("/tmp/A.kt")
	if got, ok := w.LookupFileTypeInfo("/tmp/A.kt"); ok || got != nil {
		t.Errorf("post-Invalidate lookup: got=%v ok=%v, want nil/false", got, ok)
	}
}

// TestWorkspaceState_FileTypeInfoCache_NilSafety mirrors the safety
// contract the rest of WorkspaceState's resident caches honor.
func TestWorkspaceState_FileTypeInfoCache_NilSafety(t *testing.T) {
	var w *WorkspaceState
	if got, ok := w.LookupFileTypeInfo("k"); ok || got != nil {
		t.Errorf("nil receiver: got=%v ok=%v, want nil/false", got, ok)
	}
	w.StoreFileTypeInfo("k", &typeinfer.FileTypeInfo{}) // must not panic

	w = NewWorkspaceState("")
	w.StoreFileTypeInfo("", &typeinfer.FileTypeInfo{}) // path-empty store is a no-op via normalizeKey
	w.StoreFileTypeInfo("/p.kt", nil)                  // nil info is a no-op
	if got, ok := w.LookupFileTypeInfo("/p.kt"); ok || got != nil {
		t.Errorf("Store(nil) leaked into cache: got=%v ok=%v", got, ok)
	}
}
