package cache

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/projectroot"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/store"
)

const CacheFileName = "incremental.cache"

// cachePayloadVersion is mixed into ComputeConfigHash so that a change to
// the on-disk payload schema (FileEntry / FindingColumns JSON shape, the
// columnar binary format, etc.) automatically invalidates older entries
// instead of silently failing to deserialize at read time.
//
// Bump when the cached payload schema changes in a non-backwards-compatible
// way. Old entries become unreachable (different RuleSetHash) and will be
// garbage-collected by the next `krit cache clean` or LRU pass.
const cachePayloadVersion = "v1"

// DefaultDir returns Krit's repo-local incremental cache directory.
func DefaultDir(repoDir string) string {
	if repoDir == "" {
		repoDir = "."
	}
	return filepath.Join(repoDir, ".krit", "cache")
}

// FileEntry holds cached analysis results for a single file.
type FileEntry struct {
	Hash    string                 `json:"hash"`
	ModTime int64                  `json:"modTime"`
	Size    int64                  `json:"size"`
	Columns scanner.FindingColumns `json:"-"`
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
//
// Files, Version, RuleHash, and ScanPaths are guarded by filesMu: the
// daemon shares a single *Cache across concurrent analyze RPCs (see
// internal/cli/serve/serve.go), and a background periodic-flush
// goroutine may iterate the map and serialise the header while a
// pipeline run mutates them. All access to those fields must go
// through the exported methods, which take filesMu read/write as
// appropriate.
type Cache struct {
	Version   string               `json:"version"`
	RuleHash  string               `json:"ruleHash"`
	ScanPaths []string             `json:"scanPaths,omitempty"`
	Files     map[string]FileEntry `json:"files"`

	// filesMu guards Files plus the Version, RuleHash, and ScanPaths
	// header fields. Heavy I/O (file hashing, os.Stat) is performed
	// outside the lock; only the map and header reads/writes are
	// serialised.
	filesMu sync.RWMutex

	// store, when non-nil, backs all reads and writes instead of the
	// in-memory Files map.  Entries are persisted per-file so Save becomes
	// a no-op.  Set via AttachStore after loading.
	backingStore     *store.FileStore
	storeRuleSetHash [16]byte

	// mutated flags any UpdateEntryColumns since the last MarkFlushed.
	// The daemon's periodic-flush goroutine reads this via
	// MutatedSinceFlush to skip Save on idle ticks.
	mutated atomic.Bool
}

// Result holds the outcome of checking files against the cache.
type Result struct {
	CachedColumns      scanner.FindingColumns
	CachedPaths        map[string]bool // paths that were cache hits
	CachedHashes       map[string]string
	TotalCached        int
	TotalFiles         int
	GitDirtyPathsKnown bool
	GitDirtyPathCount  int
}

// Stats records cache hit/miss statistics for reporting.
type Stats struct {
	HitRate   float64 `json:"hitRate"`
	Cached    int     `json:"cached"`
	Total     int     `json:"total"`
	LoadDurMs int64   `json:"loadDurationMs,omitempty"`
	SaveDurMs int64   `json:"saveDurationMs,omitempty"`
}

// FilePath returns the full path to the cache file for the given directory
// and scan paths. When cacheDir is non-empty and scanPaths are provided, the
// filename is derived from a hash of the scan paths to avoid collisions.
func FilePath(cacheDir string, scanPaths []string) string {
	if cacheDir == "" {
		return ""
	}
	if len(scanPaths) == 0 {
		return filepath.Join(cacheDir, CacheFileName)
	}
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
	hash := hashutil.HashHex([]byte(strings.Join(sorted, "\x00")))[:12]
	return filepath.Join(cacheDir, "krit-"+hash+".cache")
}

// ResolveCacheDir returns the cache directory and file path to use. If cacheDir
// is set via --cache-dir, it is used with a hashed filename. Otherwise Krit
// writes under the repo-local .krit/cache directory.
func ResolveCacheDir(cacheDir string, scanPaths []string) (dir string, filePath string) {
	if cacheDir != "" {
		return cacheDir, FilePath(cacheDir, scanPaths)
	}
	dir = DefaultDir(projectroot.Find(scanPaths))
	return dir, FilePath(dir, scanPaths)
}

// Load reads the cache file from the given path.
// Returns an empty cache if the file does not exist or is invalid.
func Load(cacheFilePath string) *Cache {
	data, err := os.ReadFile(cacheFilePath)
	if err != nil {
		return &Cache{Files: make(map[string]FileEntry)}
	}
	if hasBinaryMagic(data) {
		if c, ok := decodeBinary(data); ok {
			if c.Files == nil {
				c.Files = make(map[string]FileEntry)
			}
			return c
		}
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
// and safe concurrent access.  When a store is attached, Save is a no-op
// because entries are persisted individually on each UpdateEntry call.
//
// Save holds Cache.filesMu read-locked across encodeBinary so concurrent
// writers cannot tear the map mid-iteration. The compressed byte buffer
// is written to disk after the lock is released.
func (c *Cache) Save(cacheFilePath string) error {
	if c.backingStore != nil {
		return nil
	}
	dir := filepath.Dir(cacheFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}
	c.filesMu.RLock()
	data, err := encodeBinary(c)
	c.filesMu.RUnlock()
	if err != nil {
		return fmt.Errorf("encode cache: %w", err)
	}
	if err := fsutil.WriteFileAtomic(cacheFilePath, data, 0644); err != nil {
		return fmt.Errorf("write cache: %w", err)
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
		if e.Name() == CacheFileName || (strings.HasPrefix(e.Name(), "krit-") && strings.HasSuffix(e.Name(), ".cache")) {
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
	return hashutil.HashHex([]byte(strings.Join(sorted, ",")))[:16]
}

// ComputeConfigHash computes a hash from active rule names, the resolved config,
// and whether editorconfig is enabled. This ensures the cache is invalidated
// when config-derived thresholds change (e.g., MaxLineLength from editorconfig).
func ComputeConfigHash(ruleNames []string, cfg *config.Config, editorConfigEnabled bool) string {
	sorted := make([]string, len(ruleNames))
	copy(sorted, ruleNames)
	sort.Strings(sorted)

	h := hashutil.Hasher().New()
	h.Write([]byte(strings.Join(sorted, ",")))

	h.Write([]byte("|payload="))
	h.Write([]byte(cachePayloadVersion))

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
// Routed through the shared hashutil.Memo so a file hashed by other
// cache subsystems in the same run pays SHA-256 once.
func ComputeFileHash(path string) string {
	hx, err := hashutil.Default().HashFile(path, nil)
	if err != nil {
		return ""
	}
	return hx[:16]
}

// computeFileHash32 returns the full 32-byte SHA-256 of a file's content.
// Used as the FileHash component of a store.Key.
func computeFileHash32(path string) ([32]byte, error) {
	return hashutil.Default().HashFileRaw(path, nil)
}

// ParseRuleSetHash converts a 32-hex-char config hash string (from
// ComputeConfigHash) into the 16-byte form used in store.Key.
func ParseRuleSetHash(hexStr string) [16]byte {
	b, _ := hex.DecodeString(hexStr)
	var out [16]byte
	copy(out[:], b)
	return out
}

// AttachStore configures c to read and write all incremental cache entries
// through s instead of the in-memory Files map.  ruleSetHash must be the
// parsed form of ComputeConfigHash (use ParseRuleSetHash).
//
// Once attached, CheckFiles consults the store; UpdateEntryColumns writes to
// the store; Save becomes a no-op.  The existing JSON file is no longer read
// after AttachStore — the first warm run regenerates entries in the store.
func (c *Cache) AttachStore(s *store.FileStore, ruleSetHash [16]byte) {
	c.backingStore = s
	c.storeRuleSetHash = ruleSetHash
}

// SetHeader updates the cache header fields (Version, RuleHash, and
// optionally ScanPaths) atomically with respect to concurrent Save and
// CheckFiles readers. If scanPaths is empty, ScanPaths is left
// unchanged; non-empty slices are defensively copied.
func (c *Cache) SetHeader(version, ruleHash string, scanPaths []string) {
	c.filesMu.Lock()
	defer c.filesMu.Unlock()
	c.Version = version
	c.RuleHash = ruleHash
	if len(scanPaths) > 0 {
		c.ScanPaths = append([]string(nil), scanPaths...)
	}
}

// headerMatches reports whether the cached header is compatible with
// the requested ruleHash and scanPaths, under filesMu.RLock. Returns
// false if the rule hash differs, or (when both sides are non-empty)
// if the scan paths diverge.
func (c *Cache) headerMatches(ruleHash string, scanPaths []string) bool {
	c.filesMu.RLock()
	defer c.filesMu.RUnlock()
	if c.RuleHash != ruleHash {
		return false
	}
	if len(scanPaths) > 0 && len(c.ScanPaths) > 0 {
		if !scanPathsMatch(c.ScanPaths, scanPaths) {
			return false
		}
	}
	return true
}

// CheckFiles checks which files can use cached results and which need reanalysis.
// Returns cached findings and a set of paths that are cache hits.
// When scanPaths is non-empty, the cache is invalidated if the scan paths differ.
func (c *Cache) CheckFiles(filePaths []string, ruleHash string, scanPaths ...string) *Result {
	result := &Result{
		CachedPaths:  make(map[string]bool),
		CachedHashes: make(map[string]string),
		TotalFiles:   len(filePaths),
	}
	collector := scanner.NewFindingCollector(0)

	if c.backingStore != nil {
		return c.checkFilesFromStore(filePaths, result, collector)
	}

	if !c.headerMatches(ruleHash, scanPaths) {
		return result
	}
	dirtyPaths, dirtyOK := gitDirtyPathSet(scanPaths)
	result.GitDirtyPathsKnown = dirtyOK
	result.GitDirtyPathCount = len(dirtyPaths)

	for _, path := range filePaths {
		abs, _ := filepath.Abs(path)
		c.filesMu.RLock()
		entry, ok := c.Files[abs]
		c.filesMu.RUnlock()
		if !ok {
			continue
		}
		if dirtyOK && !dirtyPaths[abs] {
			result.CachedPaths[path] = true
			result.CachedHashes[path] = entry.Hash
			collector.AppendColumns(&entry.Columns)
			result.TotalCached++
			continue
		}
		if !NeedsReanalysis(path, entry) {
			result.CachedPaths[path] = true
			result.CachedHashes[path] = entry.Hash
			collector.AppendColumns(&entry.Columns)
			result.TotalCached++
		}
	}
	result.CachedColumns = *collector.Columns()

	return result
}

// CheckFilesIncremental is CheckFiles but stats only the paths in the
// dirty set; non-dirty paths skip the os.Stat/hash check and are
// reported as cache hits whenever they have a cache entry. Returns the
// same shape as CheckFiles.
//
// Callers MUST be holding a WorkspaceState whose watcher has been
// running continuously since the last invocation — otherwise the dirty
// set is incomplete and findings drift silently.
//
// The backing-store path is unaffected (entries there are content-hash
// keyed and the lookup already amortizes to O(1) per file).
func (c *Cache) CheckFilesIncremental(
	filePaths []string,
	dirty []string,
	ruleHash string,
	scanPaths ...string,
) *Result {
	result := &Result{
		CachedPaths:  make(map[string]bool),
		CachedHashes: make(map[string]string),
		TotalFiles:   len(filePaths),
	}
	collector := scanner.NewFindingCollector(0)

	if c.backingStore != nil {
		return c.checkFilesFromStore(filePaths, result, collector)
	}

	if !c.headerMatches(ruleHash, scanPaths) {
		return result
	}

	dirtySet := make(map[string]struct{}, len(dirty))
	for _, p := range dirty {
		dirtySet[absPath(p)] = struct{}{}
	}

	for _, path := range filePaths {
		abs := absPath(path)
		c.filesMu.RLock()
		entry, ok := c.Files[abs]
		c.filesMu.RUnlock()
		if !ok {
			continue
		}
		if _, isDirty := dirtySet[abs]; !isDirty {
			result.CachedPaths[path] = true
			result.CachedHashes[path] = entry.Hash
			collector.AppendColumns(&entry.Columns)
			result.TotalCached++
			continue
		}
		if !NeedsReanalysis(path, entry) {
			result.CachedPaths[path] = true
			result.CachedHashes[path] = entry.Hash
			collector.AppendColumns(&entry.Columns)
			result.TotalCached++
		}
	}
	result.CachedColumns = *collector.Columns()
	return result
}

// MutatedSinceFlush reports whether UpdateEntryColumns has been called
// since the last MarkFlushed.
func (c *Cache) MutatedSinceFlush() bool {
	if c == nil {
		return false
	}
	return c.mutated.Load()
}

// MarkFlushed clears the mutation flag. Call after a successful Save.
func (c *Cache) MarkFlushed() {
	if c == nil {
		return
	}
	c.mutated.Store(false)
}

// ShouldSkipFullSaveForSmallDelta reports whether a JSON-backed cache run
// should avoid rewriting the full cache file after a tiny dirty worktree
// delta. Dirty files are still reanalyzed on the next run unless their entry
// was persisted elsewhere, so this trades repeated dirty-file hits for lower
// save latency without reusing stale findings.
func (c *Cache) ShouldSkipFullSaveForSmallDelta(result *Result, maxMisses int) bool {
	if c == nil || c.backingStore != nil || result == nil || maxMisses <= 0 {
		return false
	}
	if !result.GitDirtyPathsKnown || result.GitDirtyPathCount == 0 {
		return false
	}
	misses := result.TotalFiles - result.TotalCached
	return misses > 0 && misses <= maxMisses && result.TotalCached > 0
}

func gitDirtyPathSet(scanPaths []string) (map[string]bool, bool) {
	if len(scanPaths) == 0 {
		return nil, false
	}
	rawRoot := scanPaths[0]
	if abs, err := filepath.Abs(rawRoot); err == nil {
		rawRoot = abs
	}
	root := normalizeAbsPath(rawRoot)
	if info, err := os.Stat(root); err == nil && !info.IsDir() {
		root = filepath.Dir(root)
		rawRoot = filepath.Dir(rawRoot)
	}
	gitBin, err := exec.LookPath("git")
	if err != nil {
		return nil, false
	}
	ctx := context.Background()
	topBytes, err := exec.CommandContext(ctx, gitBin, "-C", root, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return nil, false
	}
	rawTop := strings.TrimSpace(string(topBytes))
	if rawTop == "" {
		return nil, false
	}
	top := normalizeAbsPath(rawTop)
	// The scanner feeds tracked files here on Git-backed runs. Avoid
	// expanding untracked files because large Android checkouts can have
	// megabytes of ignored/generated strays, and unchanged tracked files are
	// the hot path this shortcut is meant to protect.
	out, err := exec.CommandContext(ctx, gitBin, "-C", top, "diff", "--name-only", "-z", "--diff-filter=ACMR", "HEAD", "--").Output()
	if err != nil {
		return nil, false
	}
	dirty := make(map[string]bool)
	addDirtyRelPaths(dirty, rawTop, string(out))
	addDirtyRelPaths(dirty, top, string(out))
	addDirtyRelPaths(dirty, rawRoot, string(out))
	return dirty, true
}

func addDirtyRelPaths(dirty map[string]bool, top, out string) {
	for _, rel := range strings.Split(out, "\x00") {
		if rel == "" {
			continue
		}
		path := filepath.Join(top, filepath.FromSlash(rel))
		if abs, err := filepath.Abs(path); err == nil {
			dirty[abs] = true
		}
	}
}

// absPath returns filepath.Abs(path) but short-circuits when path is
// already absolute. CheckFilesIncremental iterates tens of thousands
// of paths per call on warm daemons; skipping the Getwd+Clean for the
// already-canonical case is measurable.
func absPath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

func normalizeAbsPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return abs
}

// checkFilesFromStore performs cache lookup via the unified store.
// Each file is keyed by its full SHA-256 content hash + the active rule-set
// hash, so a content change or rule change automatically produces a miss.
func (c *Cache) checkFilesFromStore(filePaths []string, result *Result, collector *scanner.FindingCollector) *Result {
	for _, path := range filePaths {
		fh, err := computeFileHash32(path)
		if err != nil {
			continue
		}
		key := store.Key{
			FileHash:    fh,
			RuleSetHash: c.storeRuleSetHash,
			Kind:        store.KindIncremental,
		}
		data, ok := c.backingStore.Get(key)
		if !ok {
			continue
		}
		var cols scanner.FindingColumns
		if err := json.Unmarshal(data, &cols); err != nil {
			continue
		}
		result.CachedPaths[path] = true
		result.CachedHashes[path] = hex.EncodeToString(fh[:])
		collector.AppendColumns(&cols)
		result.TotalCached++
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
	if c.backingStore != nil {
		fh, err := computeFileHash32(path)
		if err != nil {
			return
		}
		data, err := json.Marshal(columns)
		if err != nil {
			return
		}
		key := store.Key{
			FileHash:    fh,
			RuleSetHash: c.storeRuleSetHash,
			Kind:        store.KindIncremental,
		}
		_ = c.backingStore.Put(key, data)
		c.mutated.Store(true)
		return
	}
	abs, _ := filepath.Abs(path)
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	// Compute hash outside the lock — SHA-256 + file I/O can be tens of
	// milliseconds on large files and would otherwise stall every reader.
	entry := FileEntry{
		Hash:    ComputeFileHash(path),
		ModTime: info.ModTime().UnixMilli(),
		Size:    info.Size(),
		Columns: columns,
	}
	c.filesMu.Lock()
	c.Files[abs] = entry
	c.filesMu.Unlock()
	c.mutated.Store(true)
}

// Prune removes entries for files that no longer exist.
//
// Stat calls are performed outside the cache lock so concurrent readers
// and writers are not blocked on disk I/O; only the snapshot and the
// final delete pass hold the lock.
func (c *Cache) Prune() {
	c.filesMu.RLock()
	paths := make([]string, 0, len(c.Files))
	for path := range c.Files {
		paths = append(paths, path)
	}
	c.filesMu.RUnlock()

	stale := make([]string, 0)
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			stale = append(stale, path)
		}
	}
	if len(stale) == 0 {
		return
	}
	c.filesMu.Lock()
	for _, path := range stale {
		delete(c.Files, path)
	}
	c.filesMu.Unlock()
}
