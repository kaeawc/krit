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

func TestBudget_SortedDescending(t *testing.T) {
	cacheutil.ClearRegistryForTesting()
	t.Cleanup(cacheutil.ClearRegistryForTesting)

	cacheutil.Register(&statsCache{name: "small", stats: cacheutil.CacheStats{Bytes: 10}})
	cacheutil.Register(&statsCache{name: "large", stats: cacheutil.CacheStats{Bytes: 100}})
	cacheutil.Register(&statsCache{name: "medium", stats: cacheutil.CacheStats{Bytes: 50}})

	const cap int64 = 200
	got := cacheutil.Budget(cap)
	if got.CapBytes != cap {
		t.Fatalf("CapBytes = %d, want %d", got.CapBytes, cap)
	}
	if got.UsedBytes != 160 {
		t.Fatalf("UsedBytes = %d, want 160", got.UsedBytes)
	}
	if len(got.PerCache) != 3 {
		t.Fatalf("PerCache len = %d, want 3", len(got.PerCache))
	}
	wantOrder := []string{"large", "medium", "small"}
	for i, row := range got.PerCache {
		if row.Name != wantOrder[i] {
			t.Errorf("row %d: Name = %q, want %q", i, row.Name, wantOrder[i])
		}
	}
	if got.PerCache[0].PctOfCap != 0.50 {
		t.Errorf("large PctOfCap = %v, want 0.50", got.PerCache[0].PctOfCap)
	}
}

func TestBudget_EmptyAndZeroCap(t *testing.T) {
	cacheutil.ClearRegistryForTesting()
	t.Cleanup(cacheutil.ClearRegistryForTesting)

	got := cacheutil.Budget(0)
	if got.UsedBytes != 0 || len(got.PerCache) != 0 {
		t.Fatalf("empty registry should yield empty report, got %+v", got)
	}

	cacheutil.Register(&statsCache{name: "a", stats: cacheutil.CacheStats{Bytes: 5}})
	got = cacheutil.Budget(0)
	if got.CapBytes != 0 || got.PerCache[0].PctOfCap != 0 {
		t.Errorf("cap<=0 should yield zero pct, got %+v", got)
	}
}

func TestBudget_SumWithinOnePercent(t *testing.T) {
	cacheutil.ClearRegistryForTesting()
	t.Cleanup(cacheutil.ClearRegistryForTesting)

	const cap int64 = 1000
	sizes := []int64{123, 456, 78, 9}
	var total int64
	for i, b := range sizes {
		cacheutil.Register(&statsCache{name: string(rune('a' + i)), stats: cacheutil.CacheStats{Bytes: b}})
		total += b
	}
	got := cacheutil.Budget(cap)
	if got.UsedBytes != total {
		t.Fatalf("UsedBytes = %d, want %d", got.UsedBytes, total)
	}
	var sum int64
	for _, r := range got.PerCache {
		sum += r.Bytes
	}
	if sum != got.UsedBytes {
		t.Errorf("per-cache bytes sum %d != UsedBytes %d", sum, got.UsedBytes)
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
