package cacheutil

// CacheStats is a point-in-time snapshot of one on-disk cache. Counters
// are maintained on the hot path via atomic int64 so Stats() returns in
// O(1) without a per-lookup lock. Entries and Bytes reflect the running
// in-memory view maintained by each subsystem; a full disk walk is
// reserved for Probe() (not yet uniformly implemented) triggered by
// --verbose.
type CacheStats struct {
	Entries       int   `json:"entries"`
	Bytes         int64 `json:"bytes"`
	Hits          int64 `json:"hits"`
	Misses        int64 `json:"misses"`
	Evictions     int64 `json:"evictions"`
	LastWriteUnix int64 `json:"lastWriteUnix,omitempty"`
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
