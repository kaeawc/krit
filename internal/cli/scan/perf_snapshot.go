package scan

import (
	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/perf"
)

// perfSnapshot bundles the optional --perf payload that gets stitched
// into OutputPhase.Run: aggregated timings, per-cache stats, and the
// parse-cache budget snapshot. All three are nil when --perf is off.
type perfSnapshot struct {
	Timings []perf.TimingEntry
	Caches  []cacheutil.NamedCacheStats
	Budget  *cacheutil.BudgetReport
}

// capturePerfSnapshot reads the timing/cache state at the moment
// OutputPhase needs it. Returns a zero-value snapshot when perfEnabled
// is false; otherwise pulls timings from the tracker (when the tracker
// itself is enabled) and snapshots all named caches plus the parse-cache
// budget. Pulled out of scan.Run so the conditional structure has one
// named owner instead of two adjacent if blocks.
func capturePerfSnapshot(perfEnabled bool, tracker perf.Tracker) perfSnapshot {
	var snap perfSnapshot
	if !perfEnabled {
		return snap
	}
	if tracker != nil && tracker.IsEnabled() {
		snap.Timings = tracker.GetTimings()
	}
	snap.Caches = cacheutil.AllStats()
	b := cacheutil.Budget(cacheutil.DefaultParseCacheCapBytes)
	snap.Budget = &b
	return snap
}
