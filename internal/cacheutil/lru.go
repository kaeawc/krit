package cacheutil

// SizeCapLRU is a size-capped LRU index over a sharded cache directory.
// Entries are addressed by content hash. On-disk entry files live at
// {EntriesRoot}/{hash[:2]}/{hash[2:]}{Ext}. Access-time bookkeeping
// lives in a gob sidecar at IndexPath so the LRU never relies on
// filesystem atime (mount-option-dependent; `noatime` would silently
// break it).
//
// Eviction runs in two steps: (1) sort in-memory entries by access time
// ascending and (2) remove oldest entries until the total size is at or
// below LowWaterFrac * CapBytes (default 0.80 — a grace margin keeps
// high-churn workloads from thrashing at the cap boundary). A
// best-effort lock file at LockPath prevents concurrent krit processes
// from evicting the same entries twice; when the lock is held, other
// processes skip eviction entirely (last-write-wins).

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/kaeawc/krit/internal/fsutil"
)

// DefaultParseCacheCapBytes is the default cap for the parse cache.
// Sized for very large monorepos (e.g. the Kotlin compiler at ~19k source
// files needs ~430 MB to avoid LRU thrashing on warm runs). Smaller caps
// caused multi-second background flushes on every warm scan as entries
// were evicted and re-encoded; 1 GB has comfortable headroom for the
// largest repos we benchmark against.
const DefaultParseCacheCapBytes int64 = 1024 * 1024 * 1024

const (
	lruSidecarVersion    uint32 = 1
	lruDefaultLowWater          = 0.80
	lruStaleLockDuration        = 60 * time.Second
)

// SizeCapLRU is a reusable LRU cap for a sharded cache dir.
type SizeCapLRU struct {
	EntriesRoot  string  // absolute path to the entries subtree
	IndexPath    string  // absolute path to the sidecar index file
	LockPath     string  // absolute path to the eviction lock file
	Ext          string  // entry file extension, including leading dot
	CapBytes     int64   // size cap in bytes; <=0 disables eviction
	LowWaterFrac float64 // evict to this fraction of CapBytes (default 0.80)
	Remove       func(hash string) error
	RemoveBatch  func(hashes []string) error
	TrustIndex   bool // skip per-entry filesystem validation for packed stores

	mu       sync.Mutex
	entries  map[string]lruEntry
	total    int64
	dirty    bool
	opened   bool
	deferred bool // when true, MaybeEvict is a no-op; Evict still runs
}

type lruEntry struct {
	Access int64 // unix nanos
	Size   int64 // bytes on disk
}

type lruSidecar struct {
	Version uint32
	Entries map[string]lruEntry
}

type lruPair struct {
	hash string
	e    lruEntry
}

// LRUStats is a snapshot for metrics.
type LRUStats struct {
	Entries int
	Bytes   int64
	Cap     int64
}

// Open loads the sidecar index. A missing or corrupt sidecar triggers a
// rebuild by walking EntriesRoot and using file mtime as the initial
// access time. Safe to call multiple times; subsequent calls are no-ops.
func (l *SizeCapLRU) Open() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.opened {
		return nil
	}
	if l.LowWaterFrac <= 0 || l.LowWaterFrac >= 1 {
		l.LowWaterFrac = lruDefaultLowWater
	}
	if err := l.loadLocked(); err != nil {
		return err
	}
	l.opened = true
	return nil
}

func (l *SizeCapLRU) loadLocked() error {
	l.entries = make(map[string]lruEntry)
	l.total = 0

	f, err := os.Open(l.IndexPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return l.rebuildLocked()
		}
		return fmt.Errorf("open lru index: %w", err)
	}
	defer f.Close()

	var side lruSidecar
	if err := gob.NewDecoder(f).Decode(&side); err != nil {
		// Corrupt sidecar: rebuild from disk rather than fail hard.
		return l.rebuildLocked()
	}
	if side.Version != lruSidecarVersion {
		return l.rebuildLocked()
	}
	if l.TrustIndex {
		for hash, e := range side.Entries {
			l.entries[hash] = e
			l.total += e.Size
		}
		return nil
	}

	// Cross-check against the filesystem: drop entries whose on-disk
	// file has been removed (e.g. --clear-cache entries wipe between
	// runs), and re-sync sizes when they disagree.
	for hash, e := range side.Entries {
		path := ShardedEntryPath(l.EntriesRoot, hash, l.Ext)
		info, statErr := os.Stat(path)
		if statErr != nil {
			continue
		}
		size := info.Size()
		l.entries[hash] = lruEntry{Access: e.Access, Size: size}
		l.total += size
	}
	return nil
}

// rebuildLocked walks EntriesRoot and reconstructs the index from
// file sizes + mtimes. Used on first run and after sidecar corruption.
func (l *SizeCapLRU) rebuildLocked() error {
	l.entries = make(map[string]lruEntry)
	l.total = 0
	if _, err := os.Stat(l.EntriesRoot); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	err := filepath.WalkDir(l.EntriesRoot, func(path string, d os.DirEntry, werr error) error {
		if werr != nil {
			return nil //nolint:nilerr // Walk callback skip-and-continue: per-entry error means skip this entry
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != l.Ext {
			return nil
		}
		hash := hashFromShardedPath(l.EntriesRoot, path, l.Ext)
		if hash == "" {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			return nil //nolint:nilerr // skip-and-continue: per-file read/parse error inside Walk callback
		}
		size := info.Size()
		l.entries[hash] = lruEntry{Access: info.ModTime().UnixNano(), Size: size}
		l.total += size
		return nil
	})
	if err != nil {
		return fmt.Errorf("rebuild lru: %w", err)
	}
	l.dirty = true
	return nil
}

// hashFromShardedPath reverses ShardedEntryPath: strips root + ext and
// rejoins the shard prefix with the filename stem. Returns "" when the
// layout does not match.
func hashFromShardedPath(root, path, ext string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return ""
	}
	if filepath.Ext(rel) != ext {
		return ""
	}
	stem := rel[:len(rel)-len(ext)]
	shard, name := filepath.Split(stem)
	shard = filepath.Clean(shard)
	if shard == "." || name == "" {
		return ""
	}
	if shard == "_" {
		return name
	}
	return shard + name
}

// Touch records a cache hit for hash at time now. A no-op for unknown
// hashes; callers should only Touch hashes known to be on disk.
func (l *SizeCapLRU) Touch(hash string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[hash]
	if !ok {
		return
	}
	e.Access = time.Now().UnixNano()
	l.entries[hash] = e
	l.dirty = true
}

// Record registers a Save of hash at size bytes. Updates the running
// total and access time. Replaces any prior entry (sizes may shift
// across grammar-version bumps).
func (l *SizeCapLRU) Record(hash string, size int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if prev, ok := l.entries[hash]; ok {
		l.total -= prev.Size
	}
	l.entries[hash] = lruEntry{Access: time.Now().UnixNano(), Size: size}
	l.total += size
	l.dirty = true
}

// Forget removes hash from the index. Callers pair this with an
// os.Remove of the underlying file (e.g. on decode error).
func (l *SizeCapLRU) Forget(hash string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if e, ok := l.entries[hash]; ok {
		l.total -= e.Size
		delete(l.entries, hash)
		l.dirty = true
	}
}

// SetDeferEvictions toggles whether MaybeEvict performs work. When on,
// hot-path callers (write batches) skip eviction so they don't pay the
// full sort+delete cost on every batch. Pair with an Evict() call at
// teardown to apply the cap once.
func (l *SizeCapLRU) SetDeferEvictions(on bool) {
	l.mu.Lock()
	l.deferred = on
	l.mu.Unlock()
}

// MaybeEvict runs eviction when the cap is exceeded. Returns the number
// of entries removed and any error persisting the sidecar. When another
// process holds the lock, skips with (0, nil) — last-write-wins on the
// race is acceptable; the cap is a soft target. A no-op while
// SetDeferEvictions(true) is active; callers should Evict() at teardown.
func (l *SizeCapLRU) MaybeEvict() (int, error) {
	l.mu.Lock()
	if l.deferred || l.CapBytes <= 0 || l.total <= l.CapBytes {
		l.mu.Unlock()
		return 0, nil
	}
	l.mu.Unlock()
	return l.evict()
}

// Evict runs eviction unconditionally (subject only to the cap and lock
// gating). Use this at teardown after deferring evictions during the
// hot write path so the cap is applied once instead of per-batch.
func (l *SizeCapLRU) Evict() (int, error) {
	l.mu.Lock()
	if l.CapBytes <= 0 || l.total <= l.CapBytes {
		l.mu.Unlock()
		return 0, nil
	}
	l.mu.Unlock()
	return l.evict()
}

func (l *SizeCapLRU) evict() (int, error) {
	release, ok, err := l.acquireLock()
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, nil
	}
	defer release()

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.total <= l.CapBytes {
		return 0, nil
	}

	target := int64(float64(l.CapBytes) * l.LowWaterFrac)
	if target < 0 {
		target = 0
	}

	sorted := make([]lruPair, 0, len(l.entries))
	for h, e := range l.entries {
		sorted = append(sorted, lruPair{h, e})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].e.Access < sorted[j].e.Access
	})

	victims := make([]lruPair, 0)
	for _, p := range sorted {
		if l.total <= target {
			break
		}
		victims = append(victims, p)
		l.total -= p.e.Size
	}

	removed, err := l.removeVictims(victims)
	if err != nil {
		return 0, err
	}

	for _, p := range sorted[len(victims):] {
		if l.total <= target {
			break
		}
		if removeErr := l.removeEntry(p.hash); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			continue
		}
		delete(l.entries, p.hash)
		l.total -= p.e.Size
		removed++
	}
	l.dirty = true

	if err := l.flushLocked(); err != nil {
		return removed, err
	}
	return removed, nil
}

func (l *SizeCapLRU) removeVictims(victims []lruPair) (int, error) {
	if len(victims) == 0 {
		return 0, nil
	}
	if l.RemoveBatch != nil {
		return l.removeVictimsBatch(victims)
	}
	return l.removeVictimsOne(victims)
}

func (l *SizeCapLRU) removeVictimsBatch(victims []lruPair) (int, error) {
	hashes := make([]string, 0, len(victims))
	for _, p := range victims {
		hashes = append(hashes, p.hash)
	}
	if err := l.RemoveBatch(hashes); err != nil {
		for _, p := range victims {
			l.total += p.e.Size
		}
		return 0, err
	}
	for _, p := range victims {
		delete(l.entries, p.hash)
	}
	return len(victims), nil
}

func (l *SizeCapLRU) removeVictimsOne(victims []lruPair) (int, error) {
	removed := 0
	for _, p := range victims {
		if err := l.removeEntry(p.hash); err != nil && !errors.Is(err, os.ErrNotExist) {
			l.total += p.e.Size
			continue
		}
		delete(l.entries, p.hash)
		removed++
	}
	return removed, nil
}

func (l *SizeCapLRU) removeEntry(hash string) error {
	if l.Remove != nil {
		return l.Remove(hash)
	}
	path := ShardedEntryPath(l.EntriesRoot, hash, l.Ext)
	return os.Remove(path)
}

// Flush persists the index to the sidecar file if it has changed.
// Idempotent; safe to call on shutdown.
func (l *SizeCapLRU) Flush() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.dirty {
		return nil
	}
	return l.flushLocked()
}

func (l *SizeCapLRU) flushLocked() error {
	if err := os.MkdirAll(filepath.Dir(l.IndexPath), 0o755); err != nil {
		return fmt.Errorf("mkdir lru index: %w", err)
	}
	side := lruSidecar{
		Version: lruSidecarVersion,
		Entries: l.entries,
	}
	err := fsutil.WriteFileAtomicStream(l.IndexPath, 0o644, func(w io.Writer) error {
		return gob.NewEncoder(w).Encode(&side)
	})
	if err != nil {
		return err
	}
	l.dirty = false
	return nil
}

// Stats returns a snapshot for --perf / metrics.
func (l *SizeCapLRU) Stats() LRUStats {
	l.mu.Lock()
	defer l.mu.Unlock()
	return LRUStats{
		Entries: len(l.entries),
		Bytes:   l.total,
		Cap:     l.CapBytes,
	}
}

// acquireLock creates LockPath with O_EXCL. Returns (release, true,
// nil) on success; (_, false, nil) when another holder is present and
// the lock is fresh; (_, false, err) on filesystem errors. Stale locks
// (> lruStaleLockDuration old) are broken automatically so a crashed
// krit process doesn't leak the cap forever.
func (l *SizeCapLRU) acquireLock() (release func(), acquired bool, err error) {
	if err := os.MkdirAll(filepath.Dir(l.LockPath), 0o755); err != nil {
		return nil, false, fmt.Errorf("mkdir lru lock dir: %w", err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		f, openErr := os.OpenFile(l.LockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if openErr == nil {
			_ = f.Close()
			return func() { _ = os.Remove(l.LockPath) }, true, nil
		}
		if !errors.Is(openErr, os.ErrExist) {
			return nil, false, fmt.Errorf("acquire lru lock: %w", openErr)
		}
		info, statErr := os.Stat(l.LockPath)
		if statErr != nil {
			// Lock disappeared between OpenFile and Stat — retry once.
			continue
		}
		if time.Since(info.ModTime()) < lruStaleLockDuration {
			return nil, false, nil
		}
		// Stale: forcibly remove and retry.
		_ = os.Remove(l.LockPath)
	}
	return nil, false, nil
}
