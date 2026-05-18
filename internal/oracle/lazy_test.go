package oracle

import (
	"container/list"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func writeLazyOracleJSON(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "types.json")
	data := Data{
		Version: 1,
		Files:   map[string]*File{},
		Dependencies: map[string]*Class{
			"com.example.Foo": {
				FQN:        "com.example.Foo",
				Kind:       "class",
				Visibility: "public",
			},
		},
	}
	body, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal oracle data: %v", err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write oracle data: %v", err)
	}
	return path
}

func TestLazyLookupDefersLoadUntilLookup(t *testing.T) {
	path := writeLazyOracleJSON(t, t.TempDir())
	lazy := NewLazyLookup(path, nil)

	if lazy.Loaded() {
		t.Fatal("new lazy lookup should not be loaded")
	}
	if got := lazy.Stats(); got != (Stats{}) {
		t.Fatalf("Stats should not force load, got %+v", got)
	}
	if lazy.Loaded() {
		t.Fatal("Stats forced oracle load")
	}

	info := lazy.LookupClass("Foo")
	if info == nil {
		t.Fatal("LookupClass returned nil")
	}
	if info.FQN != "com.example.Foo" {
		t.Fatalf("FQN = %q, want com.example.Foo", info.FQN)
	}
	if !lazy.Loaded() {
		t.Fatal("lookup should load oracle data")
	}
}

func TestLazyLookupReportsLoadErrorOnce(t *testing.T) {
	var reported int
	lazy := NewLazyLookup(filepath.Join(t.TempDir(), "missing.json"), func(error) {
		reported++
	})

	if got := lazy.LookupClass("Missing"); got != nil {
		t.Fatalf("LookupClass = %+v, want nil", got)
	}
	if got := lazy.LookupFunction("Missing.fn"); got != nil {
		t.Fatalf("LookupFunction = %+v, want nil", got)
	}
	if reported != 1 {
		t.Fatalf("load errors reported %d times, want 1", reported)
	}
	if lazy.Err() == nil {
		t.Fatal("expected retained load error")
	}
}

func TestLazyLookupPreload_LoadsAhead(t *testing.T) {
	path := writeLazyOracleJSON(t, t.TempDir())
	lazy := NewLazyLookup(path, nil)
	lazy.Preload()

	// Spin briefly waiting for the goroutine to land. Generous bound
	// because cold-cache test runners can drift.
	deadline := time.Now().Add(time.Second)
	for !lazy.Loaded() && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	if !lazy.Loaded() {
		t.Fatalf("Preload did not populate within 1s")
	}
}

func TestLazyLookupPreload_IdempotentWithLookup(t *testing.T) {
	path := writeLazyOracleJSON(t, t.TempDir())
	lazy := NewLazyLookup(path, nil)
	for i := 0; i < 5; i++ {
		lazy.Preload()
	}
	info := lazy.LookupClass("Foo")
	if info == nil {
		t.Fatalf("LookupClass after Preload returned nil")
	}
	if info.FQN != "com.example.Foo" {
		t.Fatalf("FQN = %q; want com.example.Foo", info.FQN)
	}
}

func TestLazyLookupPreload_NilSafe(t *testing.T) {
	var l *LazyLookup
	l.Preload() // must not panic
	if got := (&LazyLookup{}).Loaded(); got {
		t.Fatalf("empty path should not load; Loaded() = %v", got)
	}
	(&LazyLookup{}).Preload() // path == "" — must not panic or load
}

// resetPreloadCache drops every entry from the package-level preload LRU
// so cap assertions are not perturbed by earlier tests in the same
// process. Mirrors the locking discipline of preloadStateFor.
func resetPreloadCache() {
	preloadMu.Lock()
	defer preloadMu.Unlock()
	preloadByPath = map[string]*list.Element{}
	preloadLRU.Init()
}

// TestPreloadCache_EvictsOldestPastCap is the regression test for the
// unbounded preloadByPath map leak: writing more than preloadCacheCap
// distinct paths used to grow the map without bound. The cap must hold
// and the oldest entry must be evicted.
func TestPreloadCache_EvictsOldestPastCap(t *testing.T) {
	resetPreloadCache()

	// Insert cap+overflow distinct paths. They never need to exist on
	// disk — preloadStateFor only reserves the cache slot.
	overflow := 10
	total := preloadCacheCap + overflow
	for i := 0; i < total; i++ {
		_ = preloadStateFor(fmt.Sprintf("/fake/oracle/path-%d.json", i))
	}

	if got := preloadCacheLen(); got != preloadCacheCap {
		t.Fatalf("preload cache len = %d after %d inserts; want cap %d",
			got, total, preloadCacheCap)
	}

	// The first `overflow` paths must have been evicted; the most
	// recent `preloadCacheCap` must remain.
	preloadMu.Lock()
	for i := 0; i < overflow; i++ {
		path := fmt.Sprintf("/fake/oracle/path-%d.json", i)
		if _, ok := preloadByPath[path]; ok {
			preloadMu.Unlock()
			t.Fatalf("expected evicted path %q to be absent", path)
		}
	}
	for i := overflow; i < total; i++ {
		path := fmt.Sprintf("/fake/oracle/path-%d.json", i)
		if _, ok := preloadByPath[path]; !ok {
			preloadMu.Unlock()
			t.Fatalf("expected retained path %q to remain", path)
		}
	}
	preloadMu.Unlock()
}

// TestPreloadCache_TouchPromotesToFront confirms LRU recency: re-asking
// for an existing path must protect it from eviction even as new paths
// stream in.
func TestPreloadCache_TouchPromotesToFront(t *testing.T) {
	resetPreloadCache()

	// Fill to cap.
	for i := 0; i < preloadCacheCap; i++ {
		_ = preloadStateFor(fmt.Sprintf("/fake/oracle/old-%d.json", i))
	}
	// Touch the oldest entry so it becomes most-recent.
	hotPath := "/fake/oracle/old-0.json"
	_ = preloadStateFor(hotPath)

	// Push a fresh path; the now-LRU entry (old-1) should be evicted,
	// not the touched hot entry.
	_ = preloadStateFor("/fake/oracle/fresh.json")

	preloadMu.Lock()
	defer preloadMu.Unlock()
	if _, ok := preloadByPath[hotPath]; !ok {
		t.Fatalf("touched path %q was evicted; LRU recency broken", hotPath)
	}
	if _, ok := preloadByPath["/fake/oracle/old-1.json"]; ok {
		t.Fatalf("expected LRU entry old-1.json to be evicted")
	}
}

// TestPreloadCache_ConcurrentInsertsRespectCap is the -race friendly
// stress test: many goroutines racing through preloadStateFor must
// neither corrupt the map nor blow past the cap.
func TestPreloadCache_ConcurrentInsertsRespectCap(t *testing.T) {
	resetPreloadCache()

	const workers = 16
	const perWorker = 64

	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				path := fmt.Sprintf("/fake/oracle/w%d-p%d.json", id, i)
				_ = preloadStateFor(path)
			}
		}(w)
	}
	wg.Wait()

	if got := preloadCacheLen(); got > preloadCacheCap {
		t.Fatalf("preload cache len = %d after concurrent inserts; want <= %d",
			got, preloadCacheCap)
	}
}
