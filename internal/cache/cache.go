package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/scanner"
)

const CacheFileName = ".krit-cache"

// FileEntry holds cached analysis results for a single file.
type FileEntry struct {
	Hash     string                 `json:"hash"`
	ModTime  int64                  `json:"modTime"`
	Size     int64                  `json:"size"`
	Columns  scanner.FindingColumns `json:"-"`
}

type fileEntryJSON struct {
	Hash     string                  `json:"hash"`
	ModTime  int64                   `json:"modTime"`
	Size     int64                   `json:"size"`
	Findings []scanner.Finding       `json:"findings,omitempty"`
	Columns  *scanner.FindingColumns `json:"columns,omitempty"`
}

// MarshalJSON persists the cache in columnar form.
func (e FileEntry) MarshalJSON() ([]byte, error) {
	payload := fileEntryJSON{
		Hash:    e.Hash,
		ModTime: e.ModTime,
		Size:    e.Size,
	}
	if e.Columns.Len() > 0 {
		clone := e.Columns.Clone()
		payload.Columns = &clone
	}
	return json.Marshal(payload)
}

// UnmarshalJSON accepts both the new columnar cache encoding and the legacy
// findings array so older cache files still load cleanly.
func (e *FileEntry) UnmarshalJSON(data []byte) error {
	var payload fileEntryJSON
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}

	e.Hash = payload.Hash
	e.ModTime = payload.ModTime
	e.Size = payload.Size
	e.Columns = scanner.FindingColumns{}
	if payload.Columns != nil {
		e.Columns = payload.Columns.Clone()
		return nil
	}
	if len(payload.Findings) > 0 {
		e.Columns = scanner.CollectFindings(payload.Findings)
	}
	return nil
}

// Cache holds the entire incremental analysis cache.
type Cache struct {
	Version   string               `json:"version"`
	RuleHash  string               `json:"ruleHash"`
	ScanPaths []string             `json:"scanPaths,omitempty"`
	Files     map[string]FileEntry `json:"files"`
}

// CacheResult holds the outcome of checking files against the cache.
type CacheResult struct {
	CachedColumns scanner.FindingColumns
	CachedPaths   map[string]bool // paths that were cache hits
	TotalCached   int
	TotalFiles    int
}

// CacheStats records cache hit/miss statistics for reporting.
type CacheStats struct {
	HitRate   float64 `json:"hitRate"`
	Cached    int     `json:"cached"`
	Total     int     `json:"total"`
	LoadDurMs int64   `json:"loadDurationMs,omitempty"`
	SaveDurMs int64   `json:"saveDurationMs,omitempty"`
}

// CacheFilePath returns the full path to the cache file for the given directory
// and scan paths. When cacheDir is non-empty and scanPaths are provided, the
// filename is derived from a hash of the scan paths to avoid collisions.
func CacheFilePath(cacheDir string, scanPaths []string) string {
	if cacheDir == "" {
		// Legacy behavior: store in scan directory
		return ""
	}
	if len(scanPaths) == 0 {
		return filepath.Join(cacheDir, CacheFileName)
	}
	h := sha256.New()
	// Use sorted absolute paths for deterministic hashing
	sorted := make([]string, len(scanPaths))
	for i, p := range scanPaths {
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = p
		}
		sorted[i] = abs
	}
	sort.Strings(sorted)
	h.Write([]byte(strings.Join(sorted, "\x00")))
	hash := hex.EncodeToString(h.Sum(nil))[:12]
	return filepath.Join(cacheDir, "krit-"+hash+".cache")
}

// ResolveCacheDir returns the cache directory and file path to use.
// If cacheDir is set via --cache-dir, it is used (with hashed filename).
// Otherwise the legacy behavior of storing in the first scan path is used.
func ResolveCacheDir(cacheDir string, scanPaths []string) (dir string, filePath string) {
	if cacheDir != "" {
		return cacheDir, CacheFilePath(cacheDir, scanPaths)
	}
	// Legacy: use first scan path directory
	if len(scanPaths) == 0 {
		return ".", filepath.Join(".", CacheFileName)
	}
	d, _ := filepath.Abs(scanPaths[0])
	if info, err := os.Stat(d); err == nil && !info.IsDir() {
		d = filepath.Dir(d)
	}
	return d, filepath.Join(d, CacheFileName)
}

// Load reads the cache file from the given path.
// Returns an empty cache if the file does not exist or is invalid.
func Load(cacheFilePath string) *Cache {
	data, err := os.ReadFile(cacheFilePath)
	if err != nil {
		return &Cache{Files: make(map[string]FileEntry)}
	}
	var c Cache
	if err := json.Unmarshal(data, &c); err != nil {
		return &Cache{Files: make(map[string]FileEntry)}
	}
	if c.Files == nil {
		c.Files = make(map[string]FileEntry)
	}
	return &c
}

// LoadFromDir reads the cache file from the given directory (legacy API).
// Deprecated: Use Load with a full file path from ResolveCacheDir.
func LoadFromDir(dir string) *Cache {
	return Load(filepath.Join(dir, CacheFileName))
}

// Save writes the cache file atomically to the given path.
// It writes to a temporary file first, then renames for crash safety
// and safe concurrent access.
func (c *Cache) Save(cacheFilePath string) error {
	dir := filepath.Dir(cacheFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}
	data, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal cache: %w", err)
	}
	// Write to temp file then atomic rename
	tmpFile := cacheFilePath + fmt.Sprintf(".tmp.%d.%d", time.Now().UnixNano(), rand.Int63())
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		os.Remove(tmpFile) // best effort cleanup
		return fmt.Errorf("write cache temp: %w", err)
	}
	if err := os.Rename(tmpFile, cacheFilePath); err != nil {
		os.Remove(tmpFile) // best effort cleanup
		return fmt.Errorf("rename cache: %w", err)
	}
	return nil
}

// SaveToDir writes the cache file to the given directory (legacy API).
// Deprecated: Use Save with a full file path from ResolveCacheDir.
func (c *Cache) SaveToDir(dir string) error {
	return c.Save(filepath.Join(dir, CacheFileName))
}

// Clear removes the cache file at the given path.
func Clear(cacheFilePath string) error {
	err := os.Remove(cacheFilePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ClearDir removes the cache file from the given directory (legacy API).
func ClearDir(dir string) error {
	return Clear(filepath.Join(dir, CacheFileName))
}

// ClearSharedCache removes all cache files from a shared cache directory.
func ClearSharedCache(cacheDir string) error {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "krit-") && strings.HasSuffix(e.Name(), ".cache") {
			os.Remove(filepath.Join(cacheDir, e.Name()))
		}
	}
	return nil
}

// ComputeRuleHash computes a hash from the sorted list of active rule names.
// This ensures cache invalidation when rules change.
// Deprecated: Use ComputeConfigHash for config-aware cache invalidation.
func ComputeRuleHash(ruleNames []string) string {
	sorted := make([]string, len(ruleNames))
	copy(sorted, ruleNames)
	sort.Strings(sorted)
	h := sha256.New()
	h.Write([]byte(strings.Join(sorted, ",")))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// ComputeConfigHash computes a hash from active rule names, the resolved config,
// and whether editorconfig is enabled. This ensures the cache is invalidated
// when config-derived thresholds change (e.g., MaxLineLength from editorconfig).
func ComputeConfigHash(ruleNames []string, cfg *config.Config, editorConfigEnabled bool) string {
	sorted := make([]string, len(ruleNames))
	copy(sorted, ruleNames)
	sort.Strings(sorted)

	h := sha256.New()
	h.Write([]byte(strings.Join(sorted, ",")))

	// Include editorconfig marker so "with editorconfig" and "without" produce different hashes
	if editorConfigEnabled {
		h.Write([]byte("|editorconfig=true"))
	}

	// Include the full resolved config data so threshold changes invalidate the cache
	if cfg != nil {
		if data := cfg.Data(); data != nil {
			serialized, err := json.Marshal(data)
			if err == nil {
				h.Write([]byte("|config="))
				h.Write(serialized)
			}
		}
	}

	return hex.EncodeToString(h.Sum(nil))[:16]
}

// NeedsReanalysis checks whether a file needs to be re-analyzed.
// Fast path: compare modtime + size. Slow path: content hash.
func NeedsReanalysis(path string, entry FileEntry) bool {
	info, err := os.Stat(path)
	if err != nil {
		return true
	}
	// Fast path: same modtime and size means same content
	if info.ModTime().UnixMilli() == entry.ModTime && info.Size() == entry.Size {
		return false
	}
	// Slow path: compute content hash
	hash := ComputeFileHash(path)
	return hash != entry.Hash
}

// ComputeFileHash computes a SHA-256 hash of the file content (truncated to 16 hex chars).
func ComputeFileHash(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// CheckFiles checks which files can use cached results and which need reanalysis.
// Returns cached findings and a set of paths that are cache hits.
// When scanPaths is non-empty, the cache is invalidated if the scan paths differ.
func (c *Cache) CheckFiles(filePaths []string, ruleHash string, scanPaths ...string) *CacheResult {
	result := &CacheResult{
		CachedPaths: make(map[string]bool),
		TotalFiles:  len(filePaths),
	}
	collector := scanner.NewFindingCollector(0)

	// If rule hash changed, everything needs reanalysis
	if c.RuleHash != ruleHash {
		return result
	}

	// If scan paths are provided and cached scan paths differ, invalidate
	if len(scanPaths) > 0 && len(c.ScanPaths) > 0 {
		if !scanPathsMatch(c.ScanPaths, scanPaths) {
			return result
		}
	}

	for _, path := range filePaths {
		abs, _ := filepath.Abs(path)
		entry, ok := c.Files[abs]
		if !ok {
			continue
		}
		if !NeedsReanalysis(path, entry) {
			result.CachedPaths[path] = true
			collector.AppendColumns(&entry.Columns)
			result.TotalCached++
		}
	}
	result.CachedColumns = *collector.Columns()

	return result
}

// scanPathsMatch returns true if two sets of scan paths are equivalent
// (same absolute paths after sorting).
func scanPathsMatch(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aa := make([]string, len(a))
	bb := make([]string, len(b))
	for i, p := range a {
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = p
		}
		aa[i] = abs
	}
	for i, p := range b {
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = p
		}
		bb[i] = abs
	}
	sort.Strings(aa)
	sort.Strings(bb)
	for i := range aa {
		if aa[i] != bb[i] {
			return false
		}
	}
	return true
}

// UpdateEntry updates the cache for a single file after analysis.
func (c *Cache) UpdateEntry(path string, findings []scanner.Finding) {
	c.updateEntry(path, scanner.CollectFindings(findings))
}

// UpdateEntryColumns updates the cache for a single file after analysis using
// columnar findings without reconstituting []Finding.
func (c *Cache) UpdateEntryColumns(path string, columns *scanner.FindingColumns) {
	if columns == nil {
		c.updateEntry(path, scanner.FindingColumns{})
		return
	}
	c.updateEntry(path, columns.Clone())
}

func (c *Cache) updateEntry(path string, columns scanner.FindingColumns) {
	abs, _ := filepath.Abs(path)
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	c.Files[abs] = FileEntry{
		Hash:    ComputeFileHash(path),
		ModTime: info.ModTime().UnixMilli(),
		Size:    info.Size(),
		Columns: columns,
	}
}

// Prune removes entries for files that no longer exist.
func (c *Cache) Prune() {
	for path := range c.Files {
		if _, err := os.Stat(path); err != nil {
			delete(c.Files, path)
		}
	}
}
