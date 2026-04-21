package cacheutil_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/cacheutil"
)

// writeEntry writes data to the sharded entry path for hash and returns
// that path.
func writeEntry(t *testing.T, root, hash string, data []byte) string {
	t.Helper()
	p := cacheutil.ShardedEntryPath(root, hash, ".bin")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

func newLRU(t *testing.T, capBytes int64) (*cacheutil.SizeCapLRU, string) {
	t.Helper()
	dir := t.TempDir()
	entries := filepath.Join(dir, "entries")
	if err := os.MkdirAll(entries, 0o755); err != nil {
		t.Fatalf("mkdir entries: %v", err)
	}
	l := &cacheutil.SizeCapLRU{
		EntriesRoot: entries,
		IndexPath:   filepath.Join(dir, "lru-index.gob"),
		LockPath:    filepath.Join(dir, "lru.lock"),
		Ext:         ".bin",
		CapBytes:    capBytes,
	}
	if err := l.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	return l, entries
}

// hash64 returns a 64-char ASCII hex-like string derived from seed. Used
// to keep the sharded-path layout realistic.
func hash64(seed byte) string {
	out := make([]byte, 64)
	for i := range out {
		out[i] = "0123456789abcdef"[int(seed+byte(i))%16]
	}
	return string(out)
}

func TestSizeCapLRU_EvictsOldestFirst(t *testing.T) {
	l, entries := newLRU(t, 300)

	// Four 100-byte entries; cap=300, low-water=240. Pre-touching h1
	// and then crossing the cap with h4 should evict the two coldest
	// (h2, h3) and leave h1 alone. time.Sleep between steps so the
	// nanosecond clock has a chance to tick — otherwise access times
	// can collide and the eviction order becomes non-deterministic.
	h1 := hash64(1)
	h2 := hash64(2)
	h3 := hash64(3)
	h4 := hash64(4)

	writeEntry(t, entries, h1, make([]byte, 100))
	l.Record(h1, 100)
	time.Sleep(2 * time.Millisecond)
	writeEntry(t, entries, h2, make([]byte, 100))
	l.Record(h2, 100)
	time.Sleep(2 * time.Millisecond)
	writeEntry(t, entries, h3, make([]byte, 100))
	l.Record(h3, 100)
	time.Sleep(2 * time.Millisecond)

	// h1 is accessed recently — should survive eviction.
	l.Touch(h1)
	time.Sleep(2 * time.Millisecond)

	writeEntry(t, entries, h4, make([]byte, 100))
	l.Record(h4, 100)

	removed, err := l.MaybeEvict()
	if err != nil {
		t.Fatalf("MaybeEvict: %v", err)
	}
	if removed == 0 {
		t.Fatal("expected at least one eviction")
	}

	for _, cold := range []string{h2, h3} {
		if _, err := os.Stat(cacheutil.ShardedEntryPath(entries, cold, ".bin")); !os.IsNotExist(err) {
			t.Fatalf("cold entry %s… should have been evicted, stat err=%v", cold[:8], err)
		}
	}
	if _, err := os.Stat(cacheutil.ShardedEntryPath(entries, h1, ".bin")); err != nil {
		t.Fatalf("h1 (touched) should survive: %v", err)
	}
	if _, err := os.Stat(cacheutil.ShardedEntryPath(entries, h4, ".bin")); err != nil {
		t.Fatalf("h4 (newest) should survive: %v", err)
	}

	stats := l.Stats()
	if stats.Bytes > int64(float64(stats.Cap)*0.81) {
		t.Fatalf("post-evict total = %d, want <= ~80%% of cap %d", stats.Bytes, stats.Cap)
	}
}

func TestSizeCapLRU_UnderCap_NoEviction(t *testing.T) {
	l, entries := newLRU(t, 1024)
	h := hash64(7)
	writeEntry(t, entries, h, make([]byte, 100))
	l.Record(h, 100)
	removed, err := l.MaybeEvict()
	if err != nil {
		t.Fatalf("MaybeEvict: %v", err)
	}
	if removed != 0 {
		t.Fatalf("expected no eviction under cap, got %d removed", removed)
	}
}

func TestSizeCapLRU_DisabledCapIsNoop(t *testing.T) {
	l, entries := newLRU(t, 0)
	h := hash64(9)
	writeEntry(t, entries, h, make([]byte, 10_000))
	l.Record(h, 10_000)
	removed, err := l.MaybeEvict()
	if err != nil {
		t.Fatalf("MaybeEvict: %v", err)
	}
	if removed != 0 {
		t.Fatalf("disabled cap should never evict, got %d", removed)
	}
}

func TestSizeCapLRU_SidecarRoundTrip(t *testing.T) {
	dir := t.TempDir()
	entries := filepath.Join(dir, "entries")
	_ = os.MkdirAll(entries, 0o755)

	l1 := &cacheutil.SizeCapLRU{
		EntriesRoot: entries,
		IndexPath:   filepath.Join(dir, "lru-index.gob"),
		LockPath:    filepath.Join(dir, "lru.lock"),
		Ext:         ".bin",
		CapBytes:    1 << 20,
	}
	if err := l1.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}

	h := hash64(11)
	writeEntry(t, entries, h, make([]byte, 256))
	l1.Record(h, 256)
	if err := l1.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	// Fresh instance reads the sidecar back.
	l2 := &cacheutil.SizeCapLRU{
		EntriesRoot: entries,
		IndexPath:   filepath.Join(dir, "lru-index.gob"),
		LockPath:    filepath.Join(dir, "lru.lock"),
		Ext:         ".bin",
		CapBytes:    1 << 20,
	}
	if err := l2.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	stats := l2.Stats()
	if stats.Entries != 1 || stats.Bytes != 256 {
		t.Fatalf("sidecar round-trip lost entries: entries=%d bytes=%d", stats.Entries, stats.Bytes)
	}
}

func TestSizeCapLRU_RebuildsFromDiskWhenSidecarMissing(t *testing.T) {
	dir := t.TempDir()
	entries := filepath.Join(dir, "entries")
	_ = os.MkdirAll(entries, 0o755)

	// Seed entries *before* Open, so Open's rebuild-from-disk path runs.
	writeEntry(t, entries, hash64(21), make([]byte, 128))
	writeEntry(t, entries, hash64(22), make([]byte, 256))

	l := &cacheutil.SizeCapLRU{
		EntriesRoot: entries,
		IndexPath:   filepath.Join(dir, "lru-index.gob"),
		LockPath:    filepath.Join(dir, "lru.lock"),
		Ext:         ".bin",
		CapBytes:    1 << 20,
	}
	if err := l.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	stats := l.Stats()
	if stats.Entries != 2 || stats.Bytes != 128+256 {
		t.Fatalf("rebuild missed entries: entries=%d bytes=%d", stats.Entries, stats.Bytes)
	}
}

func TestSizeCapLRU_ConcurrentEvictionsDoNotCorrupt(t *testing.T) {
	l, entries := newLRU(t, 200)

	// Populate well beyond cap.
	for i := 0; i < 20; i++ {
		h := hash64(byte(30 + i))
		writeEntry(t, entries, h, make([]byte, 50))
		l.Record(h, 50)
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = l.MaybeEvict()
		}()
	}
	wg.Wait()

	stats := l.Stats()
	// Total must never exceed cap after eviction drains.
	if stats.Bytes > stats.Cap {
		t.Fatalf("post-evict total %d exceeds cap %d", stats.Bytes, stats.Cap)
	}
}

func TestSizeCapLRU_ForgetRemovesEntry(t *testing.T) {
	l, entries := newLRU(t, 1024)
	h := hash64(41)
	writeEntry(t, entries, h, make([]byte, 100))
	l.Record(h, 100)
	l.Forget(h)
	stats := l.Stats()
	if stats.Entries != 0 || stats.Bytes != 0 {
		t.Fatalf("Forget left residue: entries=%d bytes=%d", stats.Entries, stats.Bytes)
	}
}
