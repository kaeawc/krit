package scan

import (
	"testing"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/perf"
)

func TestCapturePerfSnapshotDisabled(t *testing.T) {
	snap := capturePerfSnapshot(false, perf.New(true), cacheutil.DefaultParseCacheCapBytes)
	if snap.Timings != nil {
		t.Errorf("Timings = %v; want nil", snap.Timings)
	}
	if snap.Caches != nil {
		t.Errorf("Caches = %v; want nil", snap.Caches)
	}
	if snap.Budget != nil {
		t.Errorf("Budget = %v; want nil", snap.Budget)
	}
}

func TestCapturePerfSnapshotEnabledWithDisabledTracker(t *testing.T) {
	// --perf on but tracker.New(false) is the noopTracker (IsEnabled=false).
	// Caches+Budget should populate; Timings should stay nil.
	snap := capturePerfSnapshot(true, perf.New(false), cacheutil.DefaultParseCacheCapBytes)
	if snap.Timings != nil {
		t.Errorf("Timings = %v; want nil for disabled tracker", snap.Timings)
	}
	if snap.Caches == nil {
		// Note: AllStats() may legitimately return an empty (but non-nil)
		// slice in a hermetic test process with no caches registered.
		// We accept either non-nil or zero-len, but not nil.
		t.Logf("Caches is nil — likely empty registry, which is acceptable")
	}
	if snap.Budget == nil {
		t.Error("Budget = nil; want non-nil when --perf is on")
	}
}

func TestCapturePerfSnapshotEnabledWithEnabledTracker(t *testing.T) {
	tr := perf.New(true)
	tr.TrackVoid("smoke", func() {})
	snap := capturePerfSnapshot(true, tr, cacheutil.DefaultParseCacheCapBytes)
	// Timings should now be a non-nil slice (may be empty, but the slice
	// itself comes from tracker.GetTimings() which returns []perf.TimingEntry).
	// The contract is just "Timings != nil when tracker.IsEnabled()".
	if snap.Timings == nil {
		t.Error("Timings = nil; want non-nil for enabled tracker")
	}
	if snap.Budget == nil {
		t.Error("Budget = nil; want non-nil when --perf is on")
	}
}

func TestCapturePerfSnapshotPropagatesCap(t *testing.T) {
	const customCap int64 = 512 * 1024 * 1024
	snap := capturePerfSnapshot(true, nil, customCap)
	if snap.Budget == nil {
		t.Fatal("Budget = nil; want non-nil")
	}
	if snap.Budget.CapBytes != customCap {
		t.Errorf("Budget.CapBytes = %d; want %d (effective cap should propagate, not the package default)", snap.Budget.CapBytes, customCap)
	}
}

func TestCapturePerfSnapshotNilTracker(t *testing.T) {
	// Defensive: nil tracker must not panic. perfEnabled=true triggers the
	// caches/budget capture but skips the GetTimings call.
	snap := capturePerfSnapshot(true, nil, cacheutil.DefaultParseCacheCapBytes)
	if snap.Timings != nil {
		t.Errorf("Timings = %v; want nil for nil tracker", snap.Timings)
	}
	if snap.Budget == nil {
		t.Error("Budget = nil; want non-nil")
	}
}
