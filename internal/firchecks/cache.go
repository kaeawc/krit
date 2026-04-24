package firchecks

// cache.go — on-disk content-hash cache of FIR finding results.
//
// Each entry is keyed by the content hash of the source file and stored at:
//   {repo}/.krit/fir-cache/entries/{hash[:2]}/{hash[2:]}.json
//
// Entries record the finding list plus a closure fingerprint so
// dep-closure edits invalidate downstream entries. Poison markers for
// checker-crash files are supported via Crashed/CrashError. This is
// directly parallel to internal/oracle/cache.go.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/kaeawc/krit/internal/hashutil"
)

// FirCacheVersion is bumped when the entry layout changes incompatibly.
const FirCacheVersion = 1

// FirCacheEntry is one file's cached FIR findings.
type FirCacheEntry struct {
	V                  int          `json:"v"`
	ContentHash        string       `json:"content_hash"`
	FilePath           string       `json:"file_path"`
	Findings           []FirFinding `json:"findings"`
	ClosureFingerprint string       `json:"closure_fingerprint,omitempty"`
	// Crashed marks a poison entry: the file deterministically crashes the FIR checker.
	Crashed    bool   `json:"crashed,omitempty"`
	CrashError string `json:"crash_error,omitempty"`
}

// Hot-path counters.
var (
	firCacheHits   atomic.Int64
	firCacheMisses atomic.Int64
	firCacheWrites atomic.Int64
)

// FirCacheStats holds snapshot counters.
type FirCacheStats struct {
	Hits   int64
	Misses int64
	Writes int64
}

// Stats returns a snapshot of the cache hit/miss/write counters.
func Stats() FirCacheStats {
	return FirCacheStats{
		Hits:   firCacheHits.Load(),
		Misses: firCacheMisses.Load(),
		Writes: firCacheWrites.Load(),
	}
}

// CacheDir returns (and creates) {repoDir}/.krit/fir-cache.
func CacheDir(repoDir string) (string, error) {
	dir := filepath.Join(repoDir, ".krit", "fir-cache")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create fir cache dir: %w", err)
	}
	return dir, nil
}

// entryPath returns the sharded JSON path for a given content hash.
func entryPath(cacheDir, hash string) string {
	if len(hash) < 3 {
		return filepath.Join(cacheDir, "entries", hash+".json")
	}
	return filepath.Join(cacheDir, "entries", hash[:2], hash[2:]+".json")
}

// LoadCacheEntry reads a cache entry for the given content hash.
// Returns (nil, nil) on miss.
func LoadCacheEntry(cacheDir, hash string) (*FirCacheEntry, error) {
	path := entryPath(cacheDir, hash)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var entry FirCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("unmarshal fir cache entry: %w", err)
	}
	if entry.V != FirCacheVersion || entry.ContentHash != hash {
		os.Remove(path)
		return nil, nil
	}
	return &entry, nil
}

// WriteCacheEntry writes a cache entry to disk.
func WriteCacheEntry(cacheDir string, entry *FirCacheEntry) error {
	path := entryPath(cacheDir, entry.ContentHash)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("mkdir fir cache: %w", err)
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal fir cache entry: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write fir cache entry: %w", err)
	}
	firCacheWrites.Add(1)
	return nil
}

// ContentHash returns the hex content hash for the file at path.
func ContentHash(path string) (string, error) {
	return hashutil.Default().HashFile(path, nil)
}

// ClassifyFiles partitions files into cache hits and misses. Hits carry
// their CacheEntry; misses need JVM analysis.
func ClassifyFiles(cacheDir string, files []string) (hits []*FirCacheEntry, misses []string) {
	for _, p := range files {
		hash, err := ContentHash(p)
		if err != nil {
			misses = append(misses, p)
			firCacheMisses.Add(1)
			continue
		}
		entry, err := LoadCacheEntry(cacheDir, hash)
		if err != nil || entry == nil {
			misses = append(misses, p)
			firCacheMisses.Add(1)
			continue
		}
		hits = append(hits, entry)
		firCacheHits.Add(1)
	}
	return hits, misses
}

// WriteFreshEntries writes cache entries for each file analyzed in a
// CheckResponse. Crash markers from resp.Crashed are written as poison
// entries.
func WriteFreshEntries(cacheDir string, files []string, resp *CheckResponse) int {
	// Build per-path finding index from the response.
	byPath := map[string][]FirFinding{}
	for _, f := range resp.Findings {
		byPath[f.Path] = append(byPath[f.Path], f)
	}

	written := 0
	for _, p := range files {
		hash, err := ContentHash(p)
		if err != nil {
			continue
		}
		// Poison marker for crashed files.
		if crashMsg, crashed := resp.Crashed[p]; crashed {
			entry := &FirCacheEntry{
				V:           FirCacheVersion,
				ContentHash: hash,
				FilePath:    p,
				Crashed:     true,
				CrashError:  crashMsg,
			}
			if WriteCacheEntry(cacheDir, entry) == nil {
				written++
			}
			continue
		}
		findings := byPath[p] // nil → empty slice, still a valid cache entry
		entry := &FirCacheEntry{
			V:           FirCacheVersion,
			ContentHash: hash,
			FilePath:    p,
			Findings:    findings,
		}
		if WriteCacheEntry(cacheDir, entry) == nil {
			written++
		}
	}
	return written
}
