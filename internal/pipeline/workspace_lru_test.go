package pipeline

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// kotlinOfSize produces parseable Kotlin source of roughly target
// bytes. Used so each cache entry has a known, distinct content-hash
// and a predictable len(file.Content) that drives byte accounting.
func kotlinOfSize(name string, target int) []byte {
	// Each fun line adds ~30 bytes; pad with a comment block to hit target.
	const header = "package demo\n\nclass %s {\n    fun greet() = \"hi\"\n}\n"
	src := fmt.Sprintf(header, name)
	if len(src) >= target {
		return []byte(src)
	}
	pad := strings.Repeat("// pad\n", (target-len(src))/7+1)
	return []byte(src + pad)
}

func TestWorkspaceState_LRUEvictsOldestWhenOverCap(t *testing.T) {
	ws := NewWorkspaceState("/tmp/test")
	ctx := context.Background()

	a := kotlinOfSize("A", 200)
	b := kotlinOfSize("B", 200)
	c := kotlinOfSize("C", 200)

	// Cap big enough for two entries but not three.
	ws.SetMaxParsedBytes(int64(len(a) + len(b) + 1))

	if _, err := ws.ParseFile(ctx, "A.kt", a); err != nil {
		t.Fatalf("parse A: %v", err)
	}
	if _, err := ws.ParseFile(ctx, "B.kt", b); err != nil {
		t.Fatalf("parse B: %v", err)
	}
	// A is now LRU. Inserting C must evict A.
	if _, err := ws.ParseFile(ctx, "C.kt", c); err != nil {
		t.Fatalf("parse C: %v", err)
	}

	stats := ws.Stats()
	if stats.ParsedEntries != 2 {
		t.Errorf("entries: got %d, want 2", stats.ParsedEntries)
	}
	if stats.ParsedBytes > stats.MaxParsedBytes {
		t.Errorf("parsedBytes %d exceeds cap %d", stats.ParsedBytes, stats.MaxParsedBytes)
	}
	if stats.ParsedEvictions != 1 {
		t.Errorf("evictions: got %d, want 1", stats.ParsedEvictions)
	}
	if _, ok := ws.LookupParsedByPath("A.kt"); ok {
		t.Error("A.kt should have been evicted")
	}
	if _, ok := ws.LookupParsedByPath("B.kt"); !ok {
		t.Error("B.kt should still be resident")
	}
	if _, ok := ws.LookupParsedByPath("C.kt"); !ok {
		t.Error("C.kt should be resident (just inserted)")
	}
}

func TestWorkspaceState_LRUHitPromotesToMRU(t *testing.T) {
	ws := NewWorkspaceState("/tmp/test")
	ctx := context.Background()

	a := kotlinOfSize("A", 200)
	b := kotlinOfSize("B", 200)
	c := kotlinOfSize("C", 200)

	ws.SetMaxParsedBytes(int64(len(a) + len(b) + 1))

	if _, err := ws.ParseFile(ctx, "A.kt", a); err != nil {
		t.Fatalf("parse A: %v", err)
	}
	if _, err := ws.ParseFile(ctx, "B.kt", b); err != nil {
		t.Fatalf("parse B: %v", err)
	}
	// Hit on A promotes A to MRU; now B is LRU.
	if _, err := ws.ParseFile(ctx, "A.kt", a); err != nil {
		t.Fatalf("re-parse A: %v", err)
	}
	// Inserting C must now evict B, not A.
	if _, err := ws.ParseFile(ctx, "C.kt", c); err != nil {
		t.Fatalf("parse C: %v", err)
	}

	if _, ok := ws.LookupParsedByPath("A.kt"); !ok {
		t.Error("A.kt should have survived after promotion")
	}
	if _, ok := ws.LookupParsedByPath("B.kt"); ok {
		t.Error("B.kt should have been evicted (was LRU after A promotion)")
	}
	if _, ok := ws.LookupParsedByPath("C.kt"); !ok {
		t.Error("C.kt should be resident")
	}
}

func TestWorkspaceState_InvalidateUpdatesBytes(t *testing.T) {
	ws := NewWorkspaceState("/tmp/test")
	ctx := context.Background()
	a := kotlinOfSize("A", 300)

	if _, err := ws.ParseFile(ctx, "A.kt", a); err != nil {
		t.Fatalf("parse A: %v", err)
	}
	before := ws.Stats()
	if before.ParsedBytes == 0 {
		t.Fatal("ParsedBytes should be non-zero after first parse")
	}

	ws.Invalidate("A.kt")
	after := ws.Stats()
	if after.ParsedEntries != 0 {
		t.Errorf("entries after invalidate: got %d, want 0", after.ParsedEntries)
	}
	if after.ParsedBytes != 0 {
		t.Errorf("bytes after invalidate: got %d, want 0", after.ParsedBytes)
	}
}

func TestWorkspaceState_InvalidateAllResetsAccounting(t *testing.T) {
	ws := NewWorkspaceState("/tmp/test")
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		path := fmt.Sprintf("F%d.kt", i)
		if _, err := ws.ParseFile(ctx, path, kotlinOfSize(fmt.Sprintf("F%d", i), 200)); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
	}
	if ws.Stats().ParsedBytes == 0 {
		t.Fatal("ParsedBytes should be non-zero before InvalidateAll")
	}
	ws.InvalidateAll()
	stats := ws.Stats()
	if stats.ParsedEntries != 0 || stats.ParsedBytes != 0 {
		t.Errorf("after InvalidateAll: entries=%d bytes=%d, want both 0", stats.ParsedEntries, stats.ParsedBytes)
	}
	// Re-insert should still work and account correctly.
	if _, err := ws.ParseFile(ctx, "G.kt", kotlinOfSize("G", 200)); err != nil {
		t.Fatalf("parse after reset: %v", err)
	}
	if ws.Stats().ParsedEntries != 1 {
		t.Errorf("re-insert: entries=%d, want 1", ws.Stats().ParsedEntries)
	}
}

func TestWorkspaceState_SetMaxParsedBytesEvictsImmediately(t *testing.T) {
	ws := NewWorkspaceState("/tmp/test")
	ctx := context.Background()

	srcs := make([][]byte, 4)
	for i := range srcs {
		srcs[i] = kotlinOfSize(fmt.Sprintf("F%d", i), 200)
		if _, err := ws.ParseFile(ctx, fmt.Sprintf("F%d.kt", i), srcs[i]); err != nil {
			t.Fatalf("parse F%d: %v", i, err)
		}
	}
	// Drop the cap below current usage; eviction should run.
	ws.SetMaxParsedBytes(int64(len(srcs[0]) + 1))
	stats := ws.Stats()
	if stats.ParsedEntries != 1 {
		t.Errorf("after shrink: entries=%d, want 1", stats.ParsedEntries)
	}
	if stats.ParsedBytes > stats.MaxParsedBytes {
		t.Errorf("after shrink: bytes %d > cap %d", stats.ParsedBytes, stats.MaxParsedBytes)
	}
	if stats.ParsedEvictions < 3 {
		t.Errorf("after shrink: evictions=%d, want >=3", stats.ParsedEvictions)
	}
}

func TestWorkspaceState_ZeroCapDisablesEviction(t *testing.T) {
	ws := NewWorkspaceState("/tmp/test")
	ctx := context.Background()
	// Default cap is 0 (unlimited).
	for i := 0; i < 50; i++ {
		path := fmt.Sprintf("F%d.kt", i)
		if _, err := ws.ParseFile(ctx, path, kotlinOfSize(fmt.Sprintf("F%d", i), 500)); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
	}
	stats := ws.Stats()
	if stats.ParsedEntries != 50 {
		t.Errorf("entries: got %d, want 50 (no cap)", stats.ParsedEntries)
	}
	if stats.ParsedEvictions != 0 {
		t.Errorf("evictions: got %d, want 0 (unlimited)", stats.ParsedEvictions)
	}
}

func TestWorkspaceState_ContentChangeReusesByteSlot(t *testing.T) {
	ws := NewWorkspaceState("/tmp/test")
	ctx := context.Background()
	a1 := kotlinOfSize("A", 200)
	a2 := kotlinOfSize("A", 400) // same path, different content & size

	if _, err := ws.ParseFile(ctx, "A.kt", a1); err != nil {
		t.Fatalf("first: %v", err)
	}
	firstBytes := ws.Stats().ParsedBytes
	if _, err := ws.ParseFile(ctx, "A.kt", a2); err != nil {
		t.Fatalf("second: %v", err)
	}
	stats := ws.Stats()
	if stats.ParsedEntries != 1 {
		t.Errorf("entries: got %d, want 1 (same path)", stats.ParsedEntries)
	}
	if stats.ParsedBytes == firstBytes {
		t.Errorf("bytes did not update on content change: %d", stats.ParsedBytes)
	}
	if stats.ParsedBytes != int64(len(a2)) {
		t.Errorf("bytes: got %d, want %d", stats.ParsedBytes, len(a2))
	}
}
