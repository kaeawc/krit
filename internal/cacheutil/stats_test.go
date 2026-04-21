package cacheutil_test

import (
	"sync"
	"testing"

	"github.com/kaeawc/krit/internal/cacheutil"
)

type statsCache struct {
	name  string
	stats cacheutil.CacheStats
}

func (s *statsCache) Name() string                           { return s.name }
func (s *statsCache) Clear(cacheutil.ClearContext) error     { return nil }
func (s *statsCache) Stats() cacheutil.CacheStats            { return s.stats }

type noStatsCache struct{ name string }

func (n *noStatsCache) Name() string                       { return n.name }
func (n *noStatsCache) Clear(cacheutil.ClearContext) error { return nil }

func TestAllStats_OnlyStatsProviders(t *testing.T) {
	cacheutil.ClearRegistryForTesting()
	t.Cleanup(cacheutil.ClearRegistryForTesting)

	cacheutil.Register(&statsCache{name: "a", stats: cacheutil.CacheStats{Entries: 3, Bytes: 100, Hits: 5}})
	cacheutil.Register(&noStatsCache{name: "b"})
	cacheutil.Register(&statsCache{name: "c", stats: cacheutil.CacheStats{Hits: 9}})

	got := cacheutil.AllStats()
	if len(got) != 2 {
		t.Fatalf("expected 2 stats entries, got %d", len(got))
	}
	if got[0].Name != "a" || got[0].Stats.Entries != 3 || got[0].Stats.Bytes != 100 || got[0].Stats.Hits != 5 {
		t.Errorf("unexpected first entry: %+v", got[0])
	}
	if got[1].Name != "c" || got[1].Stats.Hits != 9 {
		t.Errorf("unexpected second entry: %+v", got[1])
	}
}

func TestAllStats_EmptyRegistry(t *testing.T) {
	cacheutil.ClearRegistryForTesting()
	t.Cleanup(cacheutil.ClearRegistryForTesting)

	got := cacheutil.AllStats()
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %d entries", len(got))
	}
}

// TestAllStats_ConcurrentSafe exercises a read-only registry under load
// to confirm AllStats is race-clean against concurrent Stats() callers.
func TestAllStats_ConcurrentSafe(t *testing.T) {
	cacheutil.ClearRegistryForTesting()
	t.Cleanup(cacheutil.ClearRegistryForTesting)

	cacheutil.Register(&statsCache{name: "c1", stats: cacheutil.CacheStats{Hits: 1}})
	cacheutil.Register(&statsCache{name: "c2", stats: cacheutil.CacheStats{Misses: 1}})

	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = cacheutil.AllStats()
			}
		}()
	}
	wg.Wait()
}
