package oracle

// On-disk incremental cache for the krit-types oracle.
//
// Each source .kt file's analysis result is stored as a content-addressable
// JSON entry keyed by the content hash of its bytes. Each entry carries a "closure"
// of the file's direct source-dependency paths plus a fingerprint computed
// by hashing the current on-disk contents of those deps. A cache lookup is
// a HIT only if (a) the content hash matches (b) every dep path still
// exists on disk and (c) the recomputed closure fingerprint matches the
// stored one.
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
	"sync/atomic"
	"time"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/hashutil"
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
	V            int                     `json:"v"`
	ContentHash  string                  `json:"content_hash"`
	FilePath     string                  `json:"file_path"`
	FileResult   *OracleFile             `json:"file_result"`
	PerFileDeps  map[string]*OracleClass `json:"per_file_deps,omitempty"`
	Closure      CacheClosure            `json:"closure"`
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
	vd := cacheutil.VersionedDir{
		Root:       dir,
		EntriesDir: "entries",
		Tokens: []cacheutil.SchemaToken{
			{Name: "version", Value: fmt.Sprintf("%d", CacheVersion)},
			{Name: "hash", Value: hashutil.HasherName()},
		},
	}
	if _, err := vd.Open(); err != nil {
		return "", fmt.Errorf("create cache dir: %w", err)
	}
	return dir, nil
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

// LoadEntry reads and parses a cache entry. Returns (nil, nil) if the
// entry file does not exist. Corrupt JSON returns an error so the caller
// can decide whether to delete the bad file (ClassifyFiles does).
func LoadEntry(cacheDir, hash string) (*CacheEntry, error) {
	path := entryPath(cacheDir, hash)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var e CacheEntry
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, fmt.Errorf("parse cache entry %s: %w", path, err)
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

// IndexCacheHashes walks the cache entries directory once and returns a
// set of content hashes that have a cache entry on disk. Lets
// ClassifyFiles short-circuit misses without a per-file LoadEntry — on
// repos with many files (e.g. kotlin/kotlin) this eliminates 40k+ Stat
// syscalls and JSON opens per warm run.
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
	target := entryPath(cacheDir, entry.ContentHash)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(target), err)
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal cache entry: %w", err)
	}
	if err := fsutil.WriteFileAtomic(target, data, 0o644); err != nil {
		return fmt.Errorf("write entry: %w", err)
	}
	recordOracleDir(cacheDir)
	oracleCacheWrites.Add(1)
	oracleCacheLastWrite.Store(time.Now().Unix())
	return nil
}

// ClassifyFiles partitions paths into cache hits and cache misses. For each
// input path it computes the content hash, loads any matching entry, and
// verifies the closure fingerprint. Errors on individual files degrade to
// a miss for that file (with a best-effort cleanup of the corrupt entry)
// so a single bad entry can't poison the whole run.
//
// Performance: ClassifyFiles builds an in-memory index of existing entry
// hashes once, and shares a single content-hash memoization map across
// all VerifyClosure calls in the pass. On kotlin/kotlin (~16k files,
// ~15 deps each), the shared hash cache turns ~240k redundant dep reads
// into ~16k unique reads, and the index eliminates per-file LoadEntry
// lookups for definite misses. Observed warm classify: 5.4 s → ~400 ms.
//
// The returned `hits` slice has one entry per hit with `FilePath` already
// set to the canonical path (synthesized if the stored entry was written
// for a content-identical but differently-named file). The `misses` slice
// contains the subset of `paths` that need to be sent to krit-types.
func ClassifyFiles(cacheDir string, paths []string) (hits []*CacheEntry, misses []string) {
	recordOracleDir(cacheDir)
	hits = make([]*CacheEntry, 0, len(paths))
	misses = make([]string, 0)
	defer func() {
		oracleCacheHits.Add(int64(len(hits)))
		oracleCacheMisses.Add(int64(len(misses)))
	}()

	// Pre-walk the entries directory to build an in-memory set of
	// existing content hashes. Cheap directory walk (one Stat per entry
	// file instead of one per input path), reused across every lookup.
	// Nil index means walk failed — fall back to per-file LoadEntry.
	index := IndexCacheHashes(cacheDir)

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

		// Fast-path miss: definite no-entry-on-disk via the in-memory
		// index. Avoids the LoadEntry stat + read + parse for every
		// file that was never cached. Nil index disables this path.
		if index != nil && !index[hash] {
			misses = append(misses, p)
			continue
		}

		entry, err := LoadEntry(cacheDir, hash)
		if err != nil {
			// Corrupt or version-mismatched entry — best effort delete.
			_ = os.Remove(entryPath(cacheDir, hash))
			misses = append(misses, p)
			continue
		}
		if entry == nil {
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
	if fresh == nil {
		return 0, nil
	}
	written := 0
	hashCache := make(map[string]string, len(fresh.Files))
	for path, fr := range fresh.Files {
		hash, err := ContentHash(path)
		if err != nil {
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
		fp, err := closureFingerprint(depPaths, hashCache)
		if err != nil {
			// A dep disappeared mid-run — skip this entry rather than
			// writing one we know is stale.
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
			Approximation: approx,
		}
		if err := WriteEntry(cacheDir, entry); err != nil {
			// Non-fatal: one bad write shouldn't tank the whole run.
			continue
		}
		written++
	}
	// Poison-entry markers for files that deterministically crashed
	// analyzeKtFile. They have no FileResult and no deps; a subsequent run
	// with the same content hash will classify them as hits and skip the
	// JVM. The closure check is bypassed because `Crashed=true`.
	if deps != nil {
		approx := deps.Approximation
		for path, errMsg := range deps.Crashed {
			hash, err := ContentHash(path)
			if err != nil {
				continue
			}
			entry := &CacheEntry{
				V:             CacheVersion,
				ContentHash:   hash,
				FilePath:      path,
				Crashed:       true,
				CrashError:    errMsg,
				Approximation: approx,
				Closure: CacheClosure{
					DepPaths:    nil,
					Fingerprint: "",
				},
			}
			if err := WriteEntry(cacheDir, entry); err != nil {
				continue
			}
			written++
		}
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
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal oracle entry: %w", err)
	}
	return s.Put(oracleStoreKey(entry.ContentHash), data)
}

// ClassifyFilesWithStore is like ClassifyFiles but reads from s instead of
// the legacy cacheDir layout.  Falls back to ClassifyFiles(cacheDir, paths)
// when s is nil.
func ClassifyFilesWithStore(s *store.FileStore, cacheDir string, paths []string) (hits []*CacheEntry, misses []string) {
	if s == nil {
		return ClassifyFiles(cacheDir, paths)
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
	if s == nil {
		return WriteFreshEntries(cacheDir, fresh, deps)
	}
	if fresh == nil {
		return 0, nil
	}
	written := 0
	hashCache := make(map[string]string, len(fresh.Files))
	approx := ""
	if deps != nil {
		approx = deps.Approximation
	}
	for path, fr := range fresh.Files {
		hash, err := ContentHash(path)
		if err != nil {
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
		fp, err := closureFingerprint(depPaths, hashCache)
		if err != nil {
			continue
		}
		entry := &CacheEntry{
			V:             CacheVersion,
			ContentHash:   hash,
			FilePath:      path,
			FileResult:    fr,
			PerFileDeps:   perFileDeps,
			Approximation: approx,
			Closure: CacheClosure{
				DepPaths:    depPaths,
				Fingerprint: fp,
			},
		}
		if err := WriteEntryToStore(s, entry); err != nil {
			continue
		}
		written++
	}
	if deps != nil {
		for path, errMsg := range deps.Crashed {
			hash, err := ContentHash(path)
			if err != nil {
				continue
			}
			entry := &CacheEntry{
				V:             CacheVersion,
				ContentHash:   hash,
				FilePath:      path,
				Crashed:       true,
				CrashError:    errMsg,
				Approximation: approx,
				Closure:       CacheClosure{},
			}
			if err := WriteEntryToStore(s, entry); err != nil {
				continue
			}
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
