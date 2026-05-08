package scanner

import (
	"sync"
	"testing"
)

// TestParseCache_StatsCounters verifies that hot-path hits/misses are
// reflected in the unified CacheStats snapshot.
func TestParseCache_StatsCounters(t *testing.T) {
	repo := t.TempDir()
	pc, err := NewParseCache(repo)
	if err != nil {
		t.Fatalf("NewParseCache: %v", err)
	}

	before := pc.Stats()

	src := largeSource()
	path := writeKotlin(t, repo, "S.kt", src)

	// First parse: miss (no cached entry), writes an entry.
	if _, err := ParseKotlinFileCached(path, pc); err != nil {
		t.Fatalf("parse 1: %v", err)
	}
	// Second load: should be a hit.
	if _, ok := pc.Load("", []byte(src)); !ok {
		t.Fatalf("expected hit on second Load")
	}
	// Miss: random content never cached.
	if _, ok := pc.Load("", []byte(largeSource()+"\nfun z() = 0\n")); ok {
		t.Fatalf("expected miss on unknown content")
	}

	got := pc.Stats()
	if got.Hits-before.Hits < 1 {
		t.Errorf("expected at least 1 new hit, got %d", got.Hits-before.Hits)
	}
	if got.Misses-before.Misses < 1 {
		t.Errorf("expected at least 1 new miss, got %d", got.Misses-before.Misses)
	}
	if got.Entries < 1 {
		t.Errorf("expected entries >= 1 after save, got %d", got.Entries)
	}
	if got.Bytes <= 0 {
		t.Errorf("expected bytes > 0 after save, got %d", got.Bytes)
	}
	if got.LastWriteUnix == 0 {
		t.Errorf("expected non-zero LastWriteUnix after save")
	}
}

// TestParseCache_StatsRaceClean exercises the hot-path counters under
// concurrent Load/Save traffic to confirm atomic increments are race
// clean. Run with -race.
func TestParseCache_StatsRaceClean(t *testing.T) {
	repo := t.TempDir()
	pc, err := NewParseCache(repo)
	if err != nil {
		t.Fatalf("NewParseCache: %v", err)
	}
	src := largeSource()

	// Seed one entry so Load can both hit and miss.
	if _, err := ParseKotlinFileCached(writeKotlin(t, repo, "A.kt", src), pc); err != nil {
		t.Fatalf("seed parse: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_, _ = pc.Load("", []byte(src))
				_ = pc.Stats()
			}
		}()
	}
	wg.Wait()

	got := pc.Stats()
	if got.Hits == 0 {
		t.Errorf("expected Hits > 0 after concurrent load, got %d", got.Hits)
	}
}
