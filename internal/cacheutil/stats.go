package cacheutil

import (
	"math"
	"sort"
)

// CacheStats is a point-in-time snapshot of one on-disk cache. Counters
// are maintained on the hot path via atomic int64 so Stats() returns in
// O(1) without a per-lookup lock. Entries and Bytes reflect the running
// in-memory view maintained by each subsystem; a full disk walk is
// reserved for Probe() (not yet uniformly implemented) triggered by
// --verbose.
type CacheStats struct {
	Entries        int   `json:"entries"`
	Bytes          int64 `json:"bytes"`
	Hits           int64 `json:"hits"`
	Misses         int64 `json:"misses"`
	Evictions      int64 `json:"evictions"`
	LastWriteUnix  int64 `json:"lastWriteUnix,omitempty"`
	AsyncQueued    int64 `json:"asyncQueued,omitempty"`
	AsyncCompleted int64 `json:"asyncCompleted,omitempty"`
	AsyncFailed    int64 `json:"asyncFailed,omitempty"`
	AsyncBytes     int64 `json:"asyncBytes,omitempty"`
}

// StatsProvider is an optional extension to Registered. A Registered
// cache that also implements StatsProvider shows up in AllStats().
type StatsProvider interface {
	Stats() CacheStats
}

// NamedCacheStats pairs a cache name with its current stats.
type NamedCacheStats struct {
	Name  string     `json:"name"`
	Stats CacheStats `json:"stats"`
}

// AllStats returns a snapshot of stats for every registered cache that
// also implements StatsProvider. Order matches registration order.
func AllStats() []NamedCacheStats {
	regs := AllRegistered()
	out := make([]NamedCacheStats, 0, len(regs))
	for _, r := range regs {
		sp, ok := r.(StatsProvider)
		if !ok {
			continue
		}
		out = append(out, NamedCacheStats{Name: r.Name(), Stats: sp.Stats()})
	}
	return out
}

// BudgetRow is a single cache's contribution to the global cap.
type BudgetRow struct {
	Name     string  `json:"name"`
	Bytes    int64   `json:"bytes"`
	PctOfCap float64 `json:"pctOfCap"`
}

// BudgetReport is the aggregate slice-of-cap view across registered
// caches. Rows are sorted by Bytes descending so the biggest consumer is
// visible first. CapBytes is the conceptual global cap; PctOfCap is
// rounded to two decimal places.
type BudgetReport struct {
	CapBytes  int64       `json:"capBytes"`
	UsedBytes int64       `json:"usedBytes"`
	PerCache  []BudgetRow `json:"perCache"`
}

// Budget returns a BudgetReport built from the current stats registry.
// capBytes is the conceptual global cap (e.g. DefaultParseCacheCapBytes)
// used only to compute pctOfCap; pass <=0 to emit zeroed percentages.
// Called from --perf paths: cold and warm runs both produce a report —
// on a cold run usedBytes is 0 and PerCache rows all have Bytes=0.
func Budget(capBytes int64) BudgetReport {
	stats := AllStats()
	rows := make([]BudgetRow, 0, len(stats))
	var used int64
	for _, s := range stats {
		rows = append(rows, BudgetRow{
			Name:     s.Name,
			Bytes:    s.Stats.Bytes,
			PctOfCap: pctOfCap(s.Stats.Bytes, capBytes),
		})
		used += s.Stats.Bytes
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Bytes != rows[j].Bytes {
			return rows[i].Bytes > rows[j].Bytes
		}
		return rows[i].Name < rows[j].Name
	})
	return BudgetReport{
		CapBytes:  capBytes,
		UsedBytes: used,
		PerCache:  rows,
	}
}

func pctOfCap(bytes, cap int64) float64 {
	if cap <= 0 {
		return 0
	}
	p := float64(bytes) / float64(cap)
	return math.Round(p*100) / 100
}
