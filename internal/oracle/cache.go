package oracle

// On-disk incremental cache for the krit-types oracle.
//
// Each source .kt file's analysis result is stored as a content-addressable
// JSON entry keyed by the content hash of its bytes. The primary disk backend
// packs those JSON blobs into first-byte shard packs; the legacy one-json-file
// layout remains readable as a migration fallback. Each entry carries a
// "closure" of the file's direct source-dependency paths plus a fingerprint
// computed by hashing the current on-disk contents of those deps. A cache
// lookup is a HIT only if (a) the content hash matches (b) every dep path still
// exists on disk and (c) the recomputed closure fingerprint matches the stored
// one.
//
// This file owns:
//   - CacheEntry schema + JSON serde (via encoding/json)
//   - ContentHash / LoadEntry / WriteEntry / VerifyClosure
//   - ClassifyFiles: batch hit/miss partitioning used by the Invoke wrapper
//
// Correctness bar is findings-equivalent, not byte-identical — see the
// roadmap doc at roadmap/clusters/performance-infra/oracle-file-hash-cache.md.

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/store"
)

// Hot-path counters for the on-disk oracle cache. Tracks hits/misses
// across ClassifyFiles calls and writes through WriteFreshEntries. A
// probe-once disk walk populates Entries/Bytes on first Stats() call
// when a cache dir has been observed.
var (
	oracleCacheHits      atomic.Int64
	oracleCacheMisses    atomic.Int64
	oracleCacheWrites    atomic.Int64
	oracleCacheLastWrite atomic.Int64
	oracleCacheDirSeen   atomic.Pointer[string]
	oracleCacheEntries   atomic.Int64
	oracleCacheBytes     atomic.Int64
	oracleCacheProbed    atomic.Bool
)

func recordOracleDir(cacheDir string) {
	if cacheDir == "" {
		return
	}
	c := cacheDir
	oracleCacheDirSeen.Store(&c)
}

// CacheVersion is bumped whenever the on-disk entry layout changes in a
// way that invalidates previously-written entries. A version mismatch on
// read is treated as a miss and the offending entry is deleted.
const CacheVersion = 2

// CacheEntry is one file's cached oracle analysis. The JSON field names
// are intentionally short because there can be tens of thousands of these
// in a single repo.
type CacheEntry struct {
	V           int                     `json:"v"`
	ContentHash string                  `json:"content_hash"`
	FilePath    string                  `json:"file_path"`
	FileResult  *OracleFile             `json:"file_result"`
	PerFileDeps map[string]*OracleClass `json:"per_file_deps,omitempty"`
	Closure     CacheClosure            `json:"closure"`
	// CallFilterFingerprint is empty for unfiltered/broad oracle entries.
	// Filtered entries only satisfy lookups for the same filter. Unfiltered
	// entries are a safe superset and can satisfy filtered runs.
	CallFilterFingerprint string `json:"call_filter_fingerprint,omitempty"`
	// DeclarationProfileFingerprint is empty for entries produced with the
	// full declaration export profile (pre-profile behavior). Narrow-profile
	// entries only satisfy lookups at the same fingerprint — the
	// broader-superset rule that applies to CallFilterFingerprint also
	// applies here: empty = contains every field = satisfies any lookup.
	DeclarationProfileFingerprint string `json:"declaration_profile_fingerprint,omitempty"`
	// Approximation tags the dep-closure tracking method used when the
	// entry was written. Any mismatch with the current runtime's
	// approximation is treated as a miss — lets us upgrade the tracker
	// without leaving stale entries from a weaker approximation lying
	// around.
	Approximation string `json:"approximation,omitempty"`
	// Crashed marks this entry as a poison marker: the file with this
	// exact content deterministically crashes krit-types during analysis.
	// FileResult and PerFileDeps are nil for crash entries; they contribute
	// nothing to the assembled oracle. ClassifyFiles still returns them as
	// hits so the caller skips JVM launch. Invalidated automatically on a
	// content-hash change (content edit) or a CacheVersion bump (Kotlin or
	// krit-types upgrade).
	Crashed    bool   `json:"crashed,omitempty"`
	CrashError string `json:"crash_error,omitempty"`
}

// CacheClosure records this file's direct source-file dependencies and the
// fingerprint computed over their content at write time.
type CacheClosure struct {
	DepPaths    []string `json:"dep_paths"`
	Fingerprint string   `json:"fingerprint"`
}

// CacheDir returns the cache root for a repo. The directory is created if
// it doesn't exist.
func CacheDir(repoDir string) (string, error) {
	dir := filepath.Join(repoDir, ".krit", "types-cache")
	tokens := []cacheutil.SchemaToken{
		{Name: "version", Value: fmt.Sprintf("%d", CacheVersion)},
		{Name: "hash", Value: hashutil.HasherName()},
	}
	if oracleCacheTokenMismatch(dir, tokens) {
		_ = os.RemoveAll(filepath.Join(dir, oraclePackSubdir))
	}
	vd := cacheutil.VersionedDir{
		Root:       dir,
		EntriesDir: "entries",
		Tokens:     tokens,
	}
	if _, err := vd.Open(); err != nil {
		return "", fmt.Errorf("create cache dir: %w", err)
	}
	return dir, nil
}

func oracleCacheTokenMismatch(dir string, tokens []cacheutil.SchemaToken) bool {
	for _, token := range tokens {
		data, err := os.ReadFile(filepath.Join(dir, token.Name))
		if err != nil {
			continue
		}
		if string(data) != token.Value {
			return true
		}
	}
	return false
}

// FindRepoDir picks a repo root for the cache. Uses the first scan path,
// falling back to its parent if that path is a file.
func FindRepoDir(scanPaths []string) string {
	if len(scanPaths) == 0 {
		return ""
	}
	p := scanPaths[0]
	if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
		return filepath.Dir(p)
	}
	return p
}

// ContentHash returns the content hash of the file at path in lowercase
// hex. Used both as the cache key and as the building block for closure
// fingerprints. Routed through the process-scoped hashutil.Memo so the
// same file hashed by multiple cache subsystems within one run only
// pays the hash cost once.
func ContentHash(path string) (string, error) {
	return hashutil.Default().HashFile(path, nil)
}

// entryPath returns the disk path for a given content hash inside cacheDir.
// Format: entries/{hash[:2]}/{hash[2:]}.json — two-level sharding so no
// single directory grows past ~256 shards even in huge repos.
func entryPath(cacheDir, hash string) string {
	return cacheutil.ShardedEntryPath(filepath.Join(cacheDir, "entries"), hash, ".json")
}

// LoadEntry reads and parses a cache entry from the packed backend, falling
// back to the legacy per-entry JSON file if the pack is missing or corrupt.
// Returns (nil, nil) if neither backend has the entry. Corrupt JSON returns an
// error so the caller can degrade to a miss.
func LoadEntry(cacheDir, hash string) (*CacheEntry, error) {
	ps := newOraclePackStore(cacheDir)
	if entry, err := ps.LoadEntry(hash); err == nil && entry != nil {
		return entry, nil
	} else if err != nil {
		if legacy, legacyErr := loadLegacyEntry(cacheDir, hash); legacyErr == nil && legacy != nil {
			return legacy, nil
		}
		return nil, err
	}
	return loadLegacyEntry(cacheDir, hash)
}

func loadLegacyEntry(cacheDir, hash string) (*CacheEntry, error) {
	path := entryPath(cacheDir, hash)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return parseCacheEntryData(data, path)
}

func parseCacheEntryData(data []byte, source string) (*CacheEntry, error) {
	var e CacheEntry
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, fmt.Errorf("parse cache entry %s: %w", source, err)
	}
	if e.V != CacheVersion {
		return nil, fmt.Errorf("cache entry version %d != %d", e.V, CacheVersion)
	}
	return &e, nil
}

// closureFingerprint hashes the sorted concatenation of each dep's content
// hash. Sorting makes the fingerprint order-independent — the Go side may
// walk the dep list in any order without producing a different hash.
//
// hashCache is a shared memoization map keyed by absolute dep path. It
// lets a single ClassifyFiles call hash each unique file once, even when
// the file appears in N other files' closures. On kotlin/kotlin with
// ~16k files averaging ~15 deps each, this reduces ~240k file reads
// to ~16k unique reads. Pass nil for a fresh-hash-every-time slow path.
func closureFingerprint(depPaths []string, hashCache map[string]string) (string, error) {
	if len(depPaths) == 0 {
		// A file with no source deps still needs a deterministic
		// fingerprint; hash the empty string's hash so the field is
		// populated and "no deps" is distinguishable from "uninitialized".
		return hashutil.HashHex(nil), nil
	}
	perDep := make([]string, 0, len(depPaths))
	for _, p := range depPaths {
		if hashCache != nil {
			if h, ok := hashCache[p]; ok {
				perDep = append(perDep, h)
				continue
			}
		}
		h, err := ContentHash(p)
		if err != nil {
			// A missing dep is itself a reason to miss — bubble up so
			// the caller treats this as a cache miss.
			return "", err
		}
		if hashCache != nil {
			hashCache[p] = h
		}
		perDep = append(perDep, h)
	}
	sort.Strings(perDep)
	h := hashutil.Hasher().New()
	for _, p := range perDep {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifyClosure recomputes the dep-closure fingerprint from disk and
// compares against the entry's stored fingerprint. Returns true iff the
// two match. Any I/O error while reading deps is treated as a miss.
// Crashed entries bypass closure verification: they have no dep list by
// construction (the crash happened before any deps were collected), and
// their validity is content-hash-only — if the bytes that crashed before
// are unchanged, the same crash will deterministically recur.
//
// hashCache can be nil (cold path) or a shared map (ClassifyFiles passes
// its own map so multiple VerifyClosure calls share content-hash results
// across the whole classify pass).
func VerifyClosure(entry *CacheEntry, hashCache map[string]string) bool {
	if entry.Crashed {
		return true
	}
	fp, err := closureFingerprint(entry.Closure.DepPaths, hashCache)
	if err != nil {
		return false
	}
	return fp == entry.Closure.Fingerprint
}

func cacheScopeCompatible(entry *CacheEntry, currentCallFilter string) bool {
	return cacheScopeCompatibleV2(entry, currentCallFilter, "")
}

// cacheScopeCompatibleV2 extends cacheScopeCompatible with the declaration
// profile scope. Both the call filter and declaration profile follow the
// "empty = broad superset" convention: an entry with an empty fingerprint
// contains every field and therefore satisfies any lookup; a non-empty
// fingerprint only satisfies an identical lookup.
func cacheScopeCompatibleV2(entry *CacheEntry, currentCallFilter, currentDeclarationProfile string) bool {
	if entry == nil {
		return false
	}
	if currentCallFilter == "" {
		if entry.CallFilterFingerprint != "" {
			return false
		}
	} else if entry.CallFilterFingerprint != "" && entry.CallFilterFingerprint != currentCallFilter {
		return false
	}
	if currentDeclarationProfile == "" {
		if entry.DeclarationProfileFingerprint != "" {
			return false
		}
	} else if entry.DeclarationProfileFingerprint != "" && entry.DeclarationProfileFingerprint != currentDeclarationProfile {
		return false
	}
	return true
}

// IndexCacheHashes walks the legacy cache entries directory once and returns a
// set of content hashes that have a legacy JSON entry on disk. Packed entries
// are indexed per shard when the shard is loaded.
//
// Returned map is nil on directory walk error; callers treat nil the
// same as "don't trust the index, fall back to per-file LoadEntry".
func IndexCacheHashes(cacheDir string) map[string]bool {
	root := filepath.Join(cacheDir, "entries")
	index := make(map[string]bool, 4096)
	err := filepath.Walk(root, func(p string, info os.FileInfo, werr error) error {
		if werr != nil {
			if os.IsNotExist(werr) {
				return nil
			}
			return werr
		}
		if info.IsDir() {
			return nil
		}
		name := info.Name()
		if !filepath.IsAbs(p) {
			// Shouldn't happen — Walk always returns absolute paths when
			// given an absolute root. Defensive.
			return nil
		}
		if filepath.Ext(name) != ".json" {
			return nil
		}
		// Entry layout: entries/{hash[:2]}/{hash[2:]}.json
		// Reconstruct the content hash from the two-level sharded path.
		parent := filepath.Base(filepath.Dir(p))
		rest := name[:len(name)-len(".json")]
		if len(parent) != 2 || len(rest) < 3 {
			return nil
		}
		index[parent+rest] = true
		return nil
	})
	if err != nil {
		return nil
	}
	return index
}

// WriteEntry atomically writes a cache entry: temp file in the same
// directory, then rename into place. This avoids torn writes if the
// process crashes mid-write.
func WriteEntry(cacheDir string, entry *CacheEntry) error {
	entry.V = CacheVersion
	data, err := marshalCacheEntry(entry)
	if err != nil {
		return err
	}
	return writeEntryData(cacheDir, entry, data)
}

func marshalCacheEntry(entry *CacheEntry) ([]byte, error) {
	data, err := json.Marshal(entry)
	if err != nil {
		return nil, fmt.Errorf("marshal cache entry: %w", err)
	}
	return data, nil
}

func writeEntryData(cacheDir string, entry *CacheEntry, data []byte) error {
	return writeEntriesData(cacheDir, []oracleEncodedEntryWrite{{hash: entry.ContentHash, data: data}})
}

// ClassifyFiles partitions paths into cache hits and cache misses. For each
// input path it computes the content hash, loads any matching entry, and
// verifies the closure fingerprint. Errors on individual files degrade to
// a miss for that file (with a best-effort cleanup of the corrupt entry)
// so a single bad entry can't poison the whole run.
//
// Performance: ClassifyFiles keeps one pack store per pass, so each requested
// pack shard is loaded at most once, and builds an in-memory index for the
// legacy fallback directory. It also shares a single content-hash memoization
// map across all VerifyClosure calls in the pass. On kotlin/kotlin (~16k files,
// ~15 deps each), the shared hash cache turns ~240k redundant dep reads into
// ~16k unique reads.
//
// The returned `hits` slice has one entry per hit with `FilePath` already
// set to the canonical path (synthesized if the stored entry was written
// for a content-identical but differently-named file). The `misses` slice
// contains the subset of `paths` that need to be sent to krit-types.
func ClassifyFiles(cacheDir string, paths []string) (hits []*CacheEntry, misses []string) {
	return ClassifyFilesScoped(cacheDir, paths, "")
}

func ClassifyFilesScoped(cacheDir string, paths []string, callFilterFingerprint string) (hits []*CacheEntry, misses []string) {
	return ClassifyFilesScopedV2(cacheDir, paths, callFilterFingerprint, "")
}

func ClassifyFilesScopedV2(cacheDir string, paths []string, callFilterFingerprint, declarationProfileFingerprint string) (hits []*CacheEntry, misses []string) {
	recordOracleDir(cacheDir)
	hits = make([]*CacheEntry, 0, len(paths))
	misses = make([]string, 0)
	defer func() {
		oracleCacheHits.Add(int64(len(hits)))
		oracleCacheMisses.Add(int64(len(misses)))
	}()

	// Pre-walk the legacy entries directory so pack misses can avoid statting
	// the old per-entry path. Nil index means walk failed — fall back to
	// per-file legacy checks.
	index := IndexCacheHashes(cacheDir)
	packs := newOraclePackStore(cacheDir)

	// Shared content-hash memoization. Seed it with each path's own
	// hash as we compute it, so subsequent files that list it as a dep
	// reuse the cached value instead of re-reading from disk.
	hashCache := make(map[string]string, len(paths))

	for _, p := range paths {
		hash, err := ContentHash(p)
		if err != nil {
			// File disappeared or unreadable: skip silently — the caller
			// walked the tree to get here, so any disappearance is racy
			// and not our problem to fix.
			continue
		}
		hashCache[p] = hash

		entry, packErr := packs.LoadEntry(hash)
		var loadErr error
		if entry == nil {
			if index != nil && !index[hash] {
				misses = append(misses, p)
				continue
			}
			entry, loadErr = loadLegacyEntry(cacheDir, hash)
			if loadErr != nil {
				// Corrupt or version-mismatched legacy entry — best effort delete.
				_ = os.Remove(entryPath(cacheDir, hash))
				misses = append(misses, p)
				continue
			}
		} else if packErr != nil {
			if index != nil && index[hash] {
				entry, loadErr = loadLegacyEntry(cacheDir, hash)
			}
			if loadErr != nil || entry == nil {
				misses = append(misses, p)
				continue
			}
		}
		if entry == nil {
			misses = append(misses, p)
			continue
		}
		if !cacheScopeCompatibleV2(entry, callFilterFingerprint, declarationProfileFingerprint) {
			misses = append(misses, p)
			continue
		}
		if !VerifyClosure(entry, hashCache) {
			misses = append(misses, p)
			continue
		}
		// Content-addressable hit: the bytes at path `p` match an entry
		// written for some file (possibly a different path, e.g. an
		// identical boilerplate or test-fixture file). Since the cache
		// key is content hash, the analysis result is identical regardless
		// of which path triggered the write. Project the stored FileResult
		// + PerFileDeps onto the caller's path by synthesizing a shallow
		// copy with FilePath rewritten — downstream AssembleOracle uses
		// the FilePath to populate the Files map key.
		if entry.FilePath != p {
			synthetic := *entry
			synthetic.FilePath = p
			hits = append(hits, &synthetic)
			continue
		}
		hits = append(hits, entry)
	}
	return hits, misses
}

// AssembleOracle builds an OracleData from a set of cache hits plus any
// freshly-analyzed files from a krit-types run. Dependencies are unioned
// across all sources, with cache hits losing to fresh data on conflict
// (fresh wins so a re-analysis with new classpath is reflected).
// Crashed (poison) entries contribute nothing — they're in `hits` only so
// the caller skips re-analysis, not because they have data to emit.
func AssembleOracle(hits []*CacheEntry, fresh *OracleData) *OracleData {
	out := &OracleData{
		Version:      1,
		Files:        map[string]*OracleFile{},
		Dependencies: map[string]*OracleClass{},
	}
	if fresh != nil && fresh.KotlinVersion != "" {
		out.KotlinVersion = fresh.KotlinVersion
	}
	for _, e := range hits {
		if e.Crashed {
			continue
		}
		if e.FileResult != nil {
			out.Files[e.FilePath] = e.FileResult
		}
		for fqn, cls := range e.PerFileDeps {
			if _, exists := out.Dependencies[fqn]; !exists {
				out.Dependencies[fqn] = cls
			}
		}
	}
	if fresh != nil {
		for path, fr := range fresh.Files {
			out.Files[path] = fr
		}
		for fqn, cls := range fresh.Dependencies {
			// Fresh wins: the most recent JVM run is the ground truth.
			out.Dependencies[fqn] = cls
		}
	}
	return out
}

func mergeOracleData(parts ...*OracleData) *OracleData {
	out := &OracleData{
		Version:      1,
		Files:        map[string]*OracleFile{},
		Dependencies: map[string]*OracleClass{},
	}
	for _, part := range parts {
		if part == nil {
			continue
		}
		if out.KotlinVersion == "" {
			out.KotlinVersion = part.KotlinVersion
		}
		for path, file := range part.Files {
			out.Files[path] = file
		}
		for fqn, cls := range part.Dependencies {
			out.Dependencies[fqn] = cls
		}
	}
	return out
}

// CacheDepsFile is the schema of the --cache-deps-out JSON emitted by
// krit-types. Keys in Files are source file paths. Crashed maps a file
// path to the short error string for files whose analyzeKtFile outer
// catch fired — those get poison-marker entries instead of regular ones.
type CacheDepsFile struct {
	Version       int                        `json:"version"`
	Approximation string                     `json:"approximation"`
	Files         map[string]*CacheDepsEntry `json:"files"`
	Crashed       map[string]string          `json:"crashed,omitempty"`
}

// CacheDepsEntry is one file's dep-closure fragment.
type CacheDepsEntry struct {
	DepPaths    []string                `json:"depPaths"`
	PerFileDeps map[string]*OracleClass `json:"perFileDeps"`
}

func mergeCacheDeps(parts ...*CacheDepsFile) *CacheDepsFile {
	var sawPart bool
	out := &CacheDepsFile{
		Version: 1,
		Files:   map[string]*CacheDepsEntry{},
		Crashed: map[string]string{},
	}
	for _, part := range parts {
		if part == nil {
			continue
		}
		sawPart = true
		if out.Approximation == "" {
			out.Approximation = part.Approximation
		}
		for path, entry := range part.Files {
			out.Files[path] = entry
		}
		for path, msg := range part.Crashed {
			out.Crashed[path] = msg
		}
	}
	if !sawPart {
		return nil
	}
	return out
}

type freshEntryWriteStats struct {
	requestedFiles       int64
	crashFiles           int64
	written              int64
	skipped              int64
	bytes                int64
	depPaths             int64
	uniqueDepPaths       map[string]struct{}
	poisonWrites         int64
	contentHashNs        int64
	closureFingerprintNs int64
	marshalNs            int64
	storePutNs           int64
	atomicWriteNs        int64
	packWrites           int64
	sizeTop              []freshEntrySize
}

type freshEntrySize struct {
	path  string
	bytes int64
}

type freshOracleEntryJob struct {
	path          string
	fileResult    *OracleFile
	depPaths      []string
	perFileDeps   map[string]*OracleClass
	approximation string
	crashed       bool
	crashError    string
}

type oracleCacheHashMemo struct {
	mu     sync.RWMutex
	values map[string]string
}

func newOracleCacheHashMemo(size int) *oracleCacheHashMemo {
	if size < 1 {
		size = 1
	}
	return &oracleCacheHashMemo{values: make(map[string]string, size)}
}

func (m *oracleCacheHashMemo) contentHash(path string) (string, error) {
	if m == nil {
		return ContentHash(path)
	}
	m.mu.RLock()
	if h, ok := m.values[path]; ok {
		m.mu.RUnlock()
		return h, nil
	}
	m.mu.RUnlock()

	h, err := ContentHash(path)
	if err != nil {
		return "", err
	}

	m.mu.Lock()
	if existing, ok := m.values[path]; ok {
		m.mu.Unlock()
		return existing, nil
	}
	m.values[path] = h
	m.mu.Unlock()
	return h, nil
}

func closureFingerprintWithMemo(depPaths []string, memo *oracleCacheHashMemo) (string, error) {
	if memo == nil {
		return closureFingerprint(depPaths, nil)
	}
	if len(depPaths) == 0 {
		return hashutil.HashHex(nil), nil
	}
	perDep := make([]string, 0, len(depPaths))
	for _, p := range depPaths {
		h, err := memo.contentHash(p)
		if err != nil {
			return "", err
		}
		perDep = append(perDep, h)
	}
	sort.Strings(perDep)
	h := hashutil.Hasher().New()
	for _, p := range perDep {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func freshOracleEntryJobs(fresh *OracleData, deps *CacheDepsFile) []freshOracleEntryJob {
	if fresh == nil {
		return nil
	}
	count := len(fresh.Files)
	if deps != nil {
		count += len(deps.Crashed)
	}
	jobs := make([]freshOracleEntryJob, 0, count)
	approx := ""
	if deps != nil {
		approx = deps.Approximation
	}
	for path, fr := range fresh.Files {
		var depEntry *CacheDepsEntry
		if deps != nil {
			depEntry = deps.Files[path]
		}
		var depPaths []string
		var perFileDeps map[string]*OracleClass
		if depEntry != nil {
			depPaths = append([]string(nil), depEntry.DepPaths...)
			perFileDeps = cloneOracleClassMap(depEntry.PerFileDeps)
		}
		jobs = append(jobs, freshOracleEntryJob{
			path:          path,
			fileResult:    fr,
			depPaths:      depPaths,
			perFileDeps:   perFileDeps,
			approximation: approx,
		})
	}
	if deps != nil {
		for path, errMsg := range deps.Crashed {
			jobs = append(jobs, freshOracleEntryJob{
				path:          path,
				approximation: approx,
				crashed:       true,
				crashError:    errMsg,
			})
		}
	}
	return jobs
}

func cloneOracleClassMap(in map[string]*OracleClass) map[string]*OracleClass {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]*OracleClass, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func newFreshEntryWriteStats(fresh *OracleData, deps *CacheDepsFile) *freshEntryWriteStats {
	stats := &freshEntryWriteStats{uniqueDepPaths: map[string]struct{}{}}
	if fresh != nil {
		stats.requestedFiles = int64(len(fresh.Files))
	}
	if deps != nil {
		stats.crashFiles = int64(len(deps.Crashed))
	}
	return stats
}

func (s *freshEntryWriteStats) recordDepPaths(paths []string) {
	s.depPaths += int64(len(paths))
	for _, p := range paths {
		s.uniqueDepPaths[p] = struct{}{}
	}
}

func (s *freshEntryWriteStats) recordSize(path string, bytes int64) {
	s.bytes += bytes
	item := freshEntrySize{path: path, bytes: bytes}
	if len(s.sizeTop) < 25 {
		s.sizeTop = append(s.sizeTop, item)
		return
	}
	minIdx := 0
	minBytes := s.sizeTop[0].bytes
	for i := 1; i < len(s.sizeTop); i++ {
		if s.sizeTop[i].bytes < minBytes {
			minIdx = i
			minBytes = s.sizeTop[i].bytes
		}
	}
	if bytes > minBytes {
		s.sizeTop[minIdx] = item
	}
}

func (s *freshEntryWriteStats) emit(t perf.Tracker, storeBacked bool) {
	if t == nil || !t.IsEnabled() {
		return
	}
	perf.AddEntryDetails(t, "freshEntryContentHash", time.Duration(s.contentHashNs), map[string]int64{"files": s.requestedFiles + s.crashFiles}, nil)
	perf.AddEntryDetails(t, "freshEntryClosureFingerprint", time.Duration(s.closureFingerprintNs), map[string]int64{
		"entries":        s.requestedFiles,
		"depPaths":       s.depPaths,
		"uniqueDepPaths": int64(len(s.uniqueDepPaths)),
	}, nil)
	perf.AddEntryDetails(t, "freshEntryMarshal", time.Duration(s.marshalNs), map[string]int64{"entries": s.written, "bytes": s.bytes}, nil)
	if storeBacked {
		perf.AddEntryDetails(t, "freshEntryStorePut", time.Duration(s.storePutNs), map[string]int64{"entries": s.written}, nil)
	} else {
		perf.AddEntryDetails(t, "freshEntryAtomicWrite", time.Duration(s.atomicWriteNs), map[string]int64{"entries": s.written}, nil)
		perf.AddEntryDetails(t, "oraclePackWrite", time.Duration(s.atomicWriteNs), map[string]int64{"packs": s.packWrites, "entries": s.written}, nil)
	}
	perf.AddEntryDetails(t, "freshEntryPoisonWrites", 0, map[string]int64{"entries": s.poisonWrites}, nil)
	perf.AddEntryDetails(t, "freshEntrySummary", 0, map[string]int64{
		"requestedFiles": s.requestedFiles,
		"crashFiles":     s.crashFiles,
		"written":        s.written,
		"skipped":        s.skipped,
		"bytes":          s.bytes,
		"depPaths":       s.depPaths,
		"uniqueDepPaths": int64(len(s.uniqueDepPaths)),
		"poisonWrites":   s.poisonWrites,
	}, nil)
	if len(s.sizeTop) > 0 {
		sort.Slice(s.sizeTop, func(i, j int) bool {
			return s.sizeTop[i].bytes > s.sizeTop[j].bytes
		})
		children := make([]perf.TimingEntry, 0, len(s.sizeTop))
		var totalBytes int64
		for _, item := range s.sizeTop {
			totalBytes += item.bytes
			children = append(children, perf.TimingEntry{
				Name:       item.path,
				DurationMs: 0,
				Metrics:    map[string]int64{"bytes": item.bytes},
				Attributes: map[string]string{"file": item.path},
			})
		}
		perf.AddEntries(t, []perf.TimingEntry{{
			Name:     "freshEntrySizeTop25",
			Metrics:  map[string]int64{"bytes": totalBytes, "entries": int64(len(children))},
			Children: children,
		}})
	}
}

// LoadCacheDeps reads the --cache-deps-out JSON produced by krit-types.
func LoadCacheDeps(path string) (*CacheDepsFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read cache deps: %w", err)
	}
	var cdf CacheDepsFile
	if err := json.Unmarshal(data, &cdf); err != nil {
		return nil, fmt.Errorf("parse cache deps: %w", err)
	}
	return &cdf, nil
}

// WriteFreshEntries writes cache entries for the files that krit-types
// just analyzed. For each fresh FileResult, it looks up the corresponding
// dep fragment in the CacheDepsFile, computes the closure fingerprint
// from the current disk state of the dep paths, and writes an atomic
// entry under cacheDir. Shares a content-hash memoization across all
// per-entry closure computations — on kotlin this cuts the ~16k × 15
// = 240k redundant dep reads down to ~16k unique reads.
func WriteFreshEntries(
	cacheDir string,
	fresh *OracleData,
	deps *CacheDepsFile,
) (int, error) {
	return WriteFreshEntriesWithTracker(cacheDir, fresh, deps, nil)
}

func WriteFreshEntriesWithTracker(
	cacheDir string,
	fresh *OracleData,
	deps *CacheDepsFile,
	tracker perf.Tracker,
) (int, error) {
	return WriteFreshEntriesWithTrackerScoped(cacheDir, fresh, deps, tracker, "")
}

func WriteFreshEntriesWithTrackerScoped(
	cacheDir string,
	fresh *OracleData,
	deps *CacheDepsFile,
	tracker perf.Tracker,
	callFilterFingerprint string,
) (int, error) {
	return WriteFreshEntriesWithTrackerScopedV2(cacheDir, fresh, deps, tracker, callFilterFingerprint, "")
}

func WriteFreshEntriesWithTrackerScopedV2(
	cacheDir string,
	fresh *OracleData,
	deps *CacheDepsFile,
	tracker perf.Tracker,
	callFilterFingerprint, declarationProfileFingerprint string,
) (int, error) {
	if fresh == nil {
		return 0, nil
	}
	written := 0
	stats := newFreshEntryWriteStats(fresh, deps)
	defer stats.emit(tracker, false)
	hashCache := make(map[string]string, len(fresh.Files))
	writes := make([]oracleEncodedEntryWrite, 0, len(fresh.Files))
	pendingPoisonWrites := int64(0)
	for path, fr := range fresh.Files {
		hashStart := time.Now()
		hash, err := ContentHash(path)
		stats.contentHashNs += time.Since(hashStart).Nanoseconds()
		if err != nil {
			stats.skipped++
			continue
		}
		hashCache[path] = hash
		var depEntry *CacheDepsEntry
		if deps != nil {
			depEntry = deps.Files[path]
		}
		var depPaths []string
		var perFileDeps map[string]*OracleClass
		if depEntry != nil {
			depPaths = depEntry.DepPaths
			perFileDeps = depEntry.PerFileDeps
		}
		stats.recordDepPaths(depPaths)
		fpStart := time.Now()
		fp, err := closureFingerprint(depPaths, hashCache)
		stats.closureFingerprintNs += time.Since(fpStart).Nanoseconds()
		if err != nil {
			// A dep disappeared mid-run — skip this entry rather than
			// writing one we know is stale.
			stats.skipped++
			continue
		}
		approx := ""
		if deps != nil {
			approx = deps.Approximation
		}
		entry := &CacheEntry{
			V:           CacheVersion,
			ContentHash: hash,
			FilePath:    path,
			FileResult:  fr,
			PerFileDeps: perFileDeps,
			Closure: CacheClosure{
				DepPaths:    depPaths,
				Fingerprint: fp,
			},
			Approximation:                 approx,
			CallFilterFingerprint:         callFilterFingerprint,
			DeclarationProfileFingerprint: declarationProfileFingerprint,
		}
		entry.V = CacheVersion
		marshalStart := time.Now()
		data, err := marshalCacheEntry(entry)
		stats.marshalNs += time.Since(marshalStart).Nanoseconds()
		if err != nil {
			stats.skipped++
			continue
		}
		writes = append(writes, oracleEncodedEntryWrite{hash: entry.ContentHash, data: data})
		stats.recordSize(path, int64(len(data)))
	}
	// Poison-entry markers for files that deterministically crashed
	// analyzeKtFile. They have no FileResult and no deps; a subsequent run
	// with the same content hash will classify them as hits and skip the
	// JVM. The closure check is bypassed because `Crashed=true`.
	if deps != nil {
		approx := deps.Approximation
		for path, errMsg := range deps.Crashed {
			hashStart := time.Now()
			hash, err := ContentHash(path)
			stats.contentHashNs += time.Since(hashStart).Nanoseconds()
			if err != nil {
				stats.skipped++
				continue
			}
			entry := &CacheEntry{
				V:                             CacheVersion,
				ContentHash:                   hash,
				FilePath:                      path,
				Crashed:                       true,
				CrashError:                    errMsg,
				Approximation:                 approx,
				CallFilterFingerprint:         callFilterFingerprint,
				DeclarationProfileFingerprint: declarationProfileFingerprint,
				Closure: CacheClosure{
					DepPaths:    nil,
					Fingerprint: "",
				},
			}
			entry.V = CacheVersion
			marshalStart := time.Now()
			data, err := marshalCacheEntry(entry)
			stats.marshalNs += time.Since(marshalStart).Nanoseconds()
			if err != nil {
				stats.skipped++
				continue
			}
			writes = append(writes, oracleEncodedEntryWrite{hash: entry.ContentHash, data: data})
			stats.recordSize(path, int64(len(data)))
			pendingPoisonWrites++
		}
	}
	if len(writes) > 0 {
		writeStart := time.Now()
		if err := writeEntriesData(cacheDir, writes); err != nil {
			stats.atomicWriteNs += time.Since(writeStart).Nanoseconds()
			stats.skipped += int64(len(writes))
			return 0, nil
		}
		stats.atomicWriteNs += time.Since(writeStart).Nanoseconds()
		stats.packWrites = countOraclePackGroups(writes)
		stats.written += int64(len(writes))
		stats.poisonWrites = pendingPoisonWrites
		written = len(writes)
	}
	return written, nil
}

// oracleVersionHash returns the 16-byte RuleSetHash used for oracle store
// keys.  It encodes CacheVersion so that a version bump automatically
// produces a different key prefix, invalidating all prior oracle entries.
func oracleVersionHash() [16]byte {
	h := hashutil.HashBytes([]byte(fmt.Sprintf("oracle-v%d", CacheVersion)))
	var out [16]byte
	copy(out[:], h[:])
	return out
}

// oracleStoreKey builds the store.Key for a given content hash (hex string).
func oracleStoreKey(contentHash string) store.Key {
	b, _ := hex.DecodeString(contentHash)
	var fh [32]byte
	copy(fh[:], b)
	return store.Key{
		FileHash:    fh,
		RuleSetHash: oracleVersionHash(),
		Kind:        store.KindOracle,
	}
}

// LoadEntryFromStore retrieves a CacheEntry from s by content hash.
// Returns (nil, nil) on a miss.  Treats any malformed value as a miss.
func LoadEntryFromStore(s *store.FileStore, contentHash string) (*CacheEntry, error) {
	data, ok := s.Get(oracleStoreKey(contentHash))
	if !ok {
		return nil, nil
	}
	var e CacheEntry
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, fmt.Errorf("parse oracle store entry %s: %w", contentHash, err)
	}
	if e.V != CacheVersion {
		return nil, fmt.Errorf("oracle store entry version %d != %d", e.V, CacheVersion)
	}
	return &e, nil
}

// WriteEntryToStore persists a CacheEntry in s keyed by its content hash.
func WriteEntryToStore(s *store.FileStore, entry *CacheEntry) error {
	entry.V = CacheVersion
	data, err := marshalCacheEntry(entry)
	if err != nil {
		return err
	}
	return writeEntryDataToStore(s, entry, data)
}

func writeEntryDataToStore(s *store.FileStore, entry *CacheEntry, data []byte) error {
	return s.Put(oracleStoreKey(entry.ContentHash), data)
}

// ClassifyFilesWithStore is like ClassifyFiles but reads from s instead of
// the legacy cacheDir layout.  Falls back to ClassifyFiles(cacheDir, paths)
// when s is nil.
func ClassifyFilesWithStore(s *store.FileStore, cacheDir string, paths []string) (hits []*CacheEntry, misses []string) {
	return ClassifyFilesWithStoreScoped(s, cacheDir, paths, "")
}

func ClassifyFilesWithStoreScoped(s *store.FileStore, cacheDir string, paths []string, callFilterFingerprint string) (hits []*CacheEntry, misses []string) {
	return ClassifyFilesWithStoreScopedV2(s, cacheDir, paths, callFilterFingerprint, "")
}

// ClassifyFilesWithStoreScopedV2 is ClassifyFilesWithStoreScoped extended
// with the declaration profile fingerprint. Entries are treated as hits
// only when both the call filter and declaration profile scopes are
// compatible; the usual "empty fingerprint = broad superset" rule applies
// to both axes.
func ClassifyFilesWithStoreScopedV2(s *store.FileStore, cacheDir string, paths []string, callFilterFingerprint, declarationProfileFingerprint string) (hits []*CacheEntry, misses []string) {
	if s == nil {
		return ClassifyFilesScopedV2(cacheDir, paths, callFilterFingerprint, declarationProfileFingerprint)
	}
	recordOracleDir(cacheDir)
	hits = make([]*CacheEntry, 0, len(paths))
	misses = make([]string, 0)
	hashCache := make(map[string]string, len(paths))
	defer func() {
		oracleCacheHits.Add(int64(len(hits)))
		oracleCacheMisses.Add(int64(len(misses)))
	}()

	for _, p := range paths {
		hash, err := ContentHash(p)
		if err != nil {
			continue
		}
		hashCache[p] = hash

		entry, err := LoadEntryFromStore(s, hash)
		if err != nil || entry == nil {
			misses = append(misses, p)
			continue
		}
		if !cacheScopeCompatibleV2(entry, callFilterFingerprint, declarationProfileFingerprint) {
			misses = append(misses, p)
			continue
		}
		if !VerifyClosure(entry, hashCache) {
			misses = append(misses, p)
			continue
		}
		if entry.FilePath != p {
			synthetic := *entry
			synthetic.FilePath = p
			hits = append(hits, &synthetic)
			continue
		}
		hits = append(hits, entry)
	}
	return hits, misses
}

// WriteFreshEntriesToStore is like WriteFreshEntries but writes to s instead
// of the legacy cacheDir layout.  Falls back to WriteFreshEntries when s is nil.
func WriteFreshEntriesToStore(
	s *store.FileStore,
	cacheDir string,
	fresh *OracleData,
	deps *CacheDepsFile,
) (int, error) {
	return WriteFreshEntriesToStoreWithTracker(s, cacheDir, fresh, deps, nil)
}

func WriteFreshEntriesToStoreWithTracker(
	s *store.FileStore,
	cacheDir string,
	fresh *OracleData,
	deps *CacheDepsFile,
	tracker perf.Tracker,
) (int, error) {
	return WriteFreshEntriesToStoreWithTrackerScoped(s, cacheDir, fresh, deps, tracker, "")
}

func WriteFreshEntriesToStoreWithTrackerScoped(
	s *store.FileStore,
	cacheDir string,
	fresh *OracleData,
	deps *CacheDepsFile,
	tracker perf.Tracker,
	callFilterFingerprint string,
) (int, error) {
	return WriteFreshEntriesToStoreWithTrackerScopedV2(s, cacheDir, fresh, deps, tracker, callFilterFingerprint, "")
}

func WriteFreshEntriesToStoreWithTrackerScopedV2(
	s *store.FileStore,
	cacheDir string,
	fresh *OracleData,
	deps *CacheDepsFile,
	tracker perf.Tracker,
	callFilterFingerprint, declarationProfileFingerprint string,
) (int, error) {
	if s == nil {
		return WriteFreshEntriesWithTrackerScopedV2(cacheDir, fresh, deps, tracker, callFilterFingerprint, declarationProfileFingerprint)
	}
	if fresh == nil {
		return 0, nil
	}
	written := 0
	stats := newFreshEntryWriteStats(fresh, deps)
	defer stats.emit(tracker, true)
	hashCache := make(map[string]string, len(fresh.Files))
	approx := ""
	if deps != nil {
		approx = deps.Approximation
	}
	for path, fr := range fresh.Files {
		hashStart := time.Now()
		hash, err := ContentHash(path)
		stats.contentHashNs += time.Since(hashStart).Nanoseconds()
		if err != nil {
			stats.skipped++
			continue
		}
		hashCache[path] = hash
		var depEntry *CacheDepsEntry
		if deps != nil {
			depEntry = deps.Files[path]
		}
		var depPaths []string
		var perFileDeps map[string]*OracleClass
		if depEntry != nil {
			depPaths = depEntry.DepPaths
			perFileDeps = depEntry.PerFileDeps
		}
		stats.recordDepPaths(depPaths)
		fpStart := time.Now()
		fp, err := closureFingerprint(depPaths, hashCache)
		stats.closureFingerprintNs += time.Since(fpStart).Nanoseconds()
		if err != nil {
			stats.skipped++
			continue
		}
		entry := &CacheEntry{
			V:                             CacheVersion,
			ContentHash:                   hash,
			FilePath:                      path,
			FileResult:                    fr,
			PerFileDeps:                   perFileDeps,
			Approximation:                 approx,
			CallFilterFingerprint:         callFilterFingerprint,
			DeclarationProfileFingerprint: declarationProfileFingerprint,
			Closure: CacheClosure{
				DepPaths:    depPaths,
				Fingerprint: fp,
			},
		}
		entry.V = CacheVersion
		marshalStart := time.Now()
		data, err := marshalCacheEntry(entry)
		stats.marshalNs += time.Since(marshalStart).Nanoseconds()
		if err != nil {
			stats.skipped++
			continue
		}
		putStart := time.Now()
		if err := writeEntryDataToStore(s, entry, data); err != nil {
			stats.storePutNs += time.Since(putStart).Nanoseconds()
			stats.skipped++
			continue
		}
		stats.storePutNs += time.Since(putStart).Nanoseconds()
		stats.recordSize(path, int64(len(data)))
		stats.written++
		written++
	}
	if deps != nil {
		for path, errMsg := range deps.Crashed {
			hashStart := time.Now()
			hash, err := ContentHash(path)
			stats.contentHashNs += time.Since(hashStart).Nanoseconds()
			if err != nil {
				stats.skipped++
				continue
			}
			entry := &CacheEntry{
				V:                             CacheVersion,
				ContentHash:                   hash,
				FilePath:                      path,
				Crashed:                       true,
				CrashError:                    errMsg,
				Approximation:                 approx,
				CallFilterFingerprint:         callFilterFingerprint,
				DeclarationProfileFingerprint: declarationProfileFingerprint,
				Closure:                       CacheClosure{},
			}
			entry.V = CacheVersion
			marshalStart := time.Now()
			data, err := marshalCacheEntry(entry)
			stats.marshalNs += time.Since(marshalStart).Nanoseconds()
			if err != nil {
				stats.skipped++
				continue
			}
			putStart := time.Now()
			if err := writeEntryDataToStore(s, entry, data); err != nil {
				stats.storePutNs += time.Since(putStart).Nanoseconds()
				stats.skipped++
				continue
			}
			stats.storePutNs += time.Since(putStart).Nanoseconds()
			stats.recordSize(path, int64(len(data)))
			stats.poisonWrites++
			stats.written++
			written++
		}
	}
	return written, nil
}

// CacheStats walks the entries directory for reporting (count, byte size).
// Used by the CLI -verbose path and by the end-to-end benchmark script.
func CacheStats(cacheDir string) (count int, bytes int64, err error) {
	root := filepath.Join(cacheDir, "entries")
	err = filepath.Walk(root, func(p string, info os.FileInfo, werr error) error {
		if werr != nil {
			if os.IsNotExist(werr) {
				return nil
			}
			return werr
		}
		if info.IsDir() {
			return nil
		}
		count++
		bytes += info.Size()
		return nil
	})
	if err != nil {
		return count, bytes, err
	}
	packCount, packBytes, packErr := oraclePackStats(cacheDir)
	count += packCount
	bytes += packBytes
	if packErr != nil {
		err = packErr
	}
	return count, bytes, err
}

type oracleCacheEntry struct{}

func (oracleCacheEntry) Name() string { return "oracle-cache" }
func (oracleCacheEntry) Clear(ctx cacheutil.ClearContext) error {
	oracleCacheEntries.Store(0)
	oracleCacheBytes.Store(0)
	oracleCacheProbed.Store(false)
	if ctx.RepoDir == "" {
		return nil
	}
	return os.RemoveAll(filepath.Join(ctx.RepoDir, ".krit", "types-cache"))
}
func (oracleCacheEntry) Stats() cacheutil.CacheStats {
	// Probe disk lazily on first call so Entries/Bytes reflect pre-run
	// state. Subsequent calls reuse the cached value; a full reprobe
	// is reserved for an explicit Probe() path (not yet wired in).
	if !oracleCacheProbed.Load() {
		if dir := oracleCacheDirSeen.Load(); dir != nil && *dir != "" {
			if count, bytes, err := CacheStats(*dir); err == nil {
				oracleCacheEntries.Store(int64(count))
				oracleCacheBytes.Store(bytes)
			}
		}
		oracleCacheProbed.Store(true)
	}
	return cacheutil.CacheStats{
		Entries:       int(oracleCacheEntries.Load()),
		Bytes:         oracleCacheBytes.Load(),
		Hits:          oracleCacheHits.Load(),
		Misses:        oracleCacheMisses.Load(),
		LastWriteUnix: oracleCacheLastWrite.Load(),
	}
}

func init() {
	cacheutil.Register(oracleCacheEntry{})
}
