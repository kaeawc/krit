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
// budget. capBytes is the effective parse-cache cap (after --parse-cache-cap-mb
// and krit.yml have been applied) — passing a stale default would make
// pctOfCap misreport against the wrong ceiling.
func capturePerfSnapshot(perfEnabled bool, tracker perf.Tracker, capBytes int64) perfSnapshot {
	var snap perfSnapshot
	if !perfEnabled {
		return snap
	}
	if tracker != nil && tracker.IsEnabled() {
		snap.Timings = tracker.GetTimings()
	}
	snap.Caches = cacheutil.AllStats()
	b := cacheutil.Budget(capBytes)
	snap.Budget = &b
	return snap
}
