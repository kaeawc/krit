package oracle

import (
	"reflect"
	"testing"
)

func TestSplitForcedMisses_Empty(t *testing.T) {
	t.Parallel()
	ktFiles := []string{"/a.kt", "/b.kt"}
	classify, forced := splitForcedMisses(ktFiles, nil)
	// With no hint, the classifier should see every file and forcedInScan must be empty.
	if !reflect.DeepEqual(classify, ktFiles) {
		t.Errorf("classify = %v, want %v", classify, ktFiles)
	}
	if forced != nil {
		t.Errorf("forced = %v, want nil", forced)
	}
}

func TestSplitForcedMisses_OneInScan(t *testing.T) {
	t.Parallel()
	ktFiles := []string{"/a.kt", "/b.kt", "/c.kt"}
	classify, forced := splitForcedMisses(ktFiles, []string{"/b.kt"})
	wantClassify := []string{"/a.kt", "/c.kt"}
	if !reflect.DeepEqual(classify, wantClassify) {
		t.Errorf("classify = %v, want %v", classify, wantClassify)
	}
	wantForced := []string{"/b.kt"}
	if !reflect.DeepEqual(forced, wantForced) {
		t.Errorf("forced = %v, want %v", forced, wantForced)
	}
}

func TestSplitForcedMisses_AllInScan(t *testing.T) {
	t.Parallel()
	ktFiles := []string{"/a.kt", "/b.kt"}
	classify, forced := splitForcedMisses(ktFiles, []string{"/a.kt", "/b.kt"})
	if len(classify) != 0 {
		t.Errorf("classify = %v, want empty", classify)
	}
	wantForced := []string{"/a.kt", "/b.kt"}
	if !reflect.DeepEqual(forced, wantForced) {
		t.Errorf("forced = %v, want %v", forced, wantForced)
	}
}

func TestSplitForcedMisses_OutOfScanIgnored(t *testing.T) {
	t.Parallel()
	// /z.kt is in the hint but not in the scan set; it must be dropped.
	ktFiles := []string{"/a.kt", "/b.kt"}
	classify, forced := splitForcedMisses(ktFiles, []string{"/b.kt", "/z.kt"})
	wantClassify := []string{"/a.kt"}
	if !reflect.DeepEqual(classify, wantClassify) {
		t.Errorf("classify = %v, want %v", classify, wantClassify)
	}
	wantForced := []string{"/b.kt"}
	if !reflect.DeepEqual(forced, wantForced) {
		t.Errorf("forced = %v, want %v", forced, wantForced)
	}
}

func TestSplitForcedMisses_DuplicateHintsDeduped(t *testing.T) {
	t.Parallel()
	// Duplicates in the hint should only cause the file to appear once
	// in forcedInScan — the map dedupes.
	ktFiles := []string{"/a.kt"}
	classify, forced := splitForcedMisses(ktFiles, []string{"/a.kt", "/a.kt"})
	if len(classify) != 0 {
		t.Errorf("classify = %v, want empty", classify)
	}
	wantForced := []string{"/a.kt"}
	if !reflect.DeepEqual(forced, wantForced) {
		t.Errorf("forced = %v, want %v", forced, wantForced)
	}
}
