package android

import "testing"

func TestResourceAndIconFuturesAcceptWorkerCap(t *testing.T) {
	resourceResDir := setupTestResDir(t)

	resourceFuture := NewResourceScanFuture(resourceResDir, nil, 1)
	idx, stats, err := resourceFuture.Await()
	if err != nil {
		t.Fatalf("ResourceScanFuture.Await: %v", err)
	}
	if idx == nil || len(idx.Layouts) == 0 {
		t.Fatalf("expected resource index to be populated, got %#v", idx)
	}
	if stats.LayoutDirCount == 0 || stats.ValuesDirCount == 0 {
		t.Fatalf("expected resource stats to be populated, got %#v", stats)
	}

	iconFuture := NewIconScanFuture(setupIconTestResDir(t), nil, 1)
	iconIdx, err := iconFuture.Await()
	if err != nil {
		t.Fatalf("IconScanFuture.Await: %v", err)
	}
	if iconIdx == nil || len(iconIdx.Icons) == 0 {
		t.Fatalf("expected icon index to be populated, got %#v", iconIdx)
	}
}
