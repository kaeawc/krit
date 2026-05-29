package pipeline

import (
	"encoding/hex"
	"sort"

	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/scanner"
)

// HasWarmResourceSourceBundle reports whether the Android findings cache can
// cover resource-backed source rules without parsed source files.
func HasWarmResourceSourceBundle(cacheDir string, sourcePaths, resDirs []string, ruleHash, libraryFactsFP, javaSemanticFactsFP string) bool {
	if cacheDir == "" || ruleHash == "" || len(sourcePaths) == 0 {
		return false
	}
	sourceSetFP, ok := resourceSourceSetFingerprintFromPaths(sourcePaths)
	if !ok {
		return false
	}
	key := scanner.AndroidFindingsKey(scanner.AndroidFindingsKeyInputs{
		Kind:                scanner.AndroidFindingsKindResourceSourceBundle,
		RuleHash:            ruleHash,
		LibraryFactsFP:      libraryFactsFP,
		JavaSemanticFactsFP: javaSemanticFactsFP,
		InputFP:             sourceSetFP,
		Extra:               mergedResourceIndexFingerprint(resDirs),
	})
	_, ok = scanner.LoadAndroidFindings(cacheDir, key)
	return ok
}

// HasWarmResourceSourceBundleWithHashes is the same cache probe as
// HasWarmResourceSourceBundle, but reuses hashes already validated by the
// incremental findings cache.
func HasWarmResourceSourceBundleWithHashes(cacheDir string, sourcePaths, resDirs []string, sourceHashes map[string]string, ruleHash, libraryFactsFP, javaSemanticFactsFP string) bool {
	if cacheDir == "" || ruleHash == "" || len(sourcePaths) == 0 || len(sourceHashes) == 0 {
		return false
	}
	entries := make([]resourceSourceEntry, 0, len(sourcePaths))
	for _, path := range sourcePaths {
		hash := sourceHashes[path]
		if path == "" || hash == "" {
			return false
		}
		entries = append(entries, resourceSourceEntry{path: path, hash: hash})
	}
	sourceSetFP, ok := resourceSourceEntriesFingerprint(entries)
	if !ok {
		return false
	}
	key := scanner.AndroidFindingsKey(scanner.AndroidFindingsKeyInputs{
		Kind:                scanner.AndroidFindingsKindResourceSourceBundle,
		RuleHash:            ruleHash,
		LibraryFactsFP:      libraryFactsFP,
		JavaSemanticFactsFP: javaSemanticFactsFP,
		InputFP:             sourceSetFP,
		Extra:               mergedResourceIndexFingerprint(resDirs),
	})
	_, ok = scanner.LoadAndroidFindings(cacheDir, key)
	return ok
}

// EnsureWarmResourceSourceBundleWithHashes checks the fast hash-reuse bundle
// key first. If only the older full-content-hash key exists, it aliases that
// payload to the hash-reuse key so later warm runs do not need to rehash every
// source file before skipping parse.
func EnsureWarmResourceSourceBundleWithHashes(cacheDir string, sourcePaths, resDirs []string, sourceHashes map[string]string, ruleHash, libraryFactsFP, javaSemanticFactsFP string) bool {
	if HasWarmResourceSourceBundleWithHashes(cacheDir, sourcePaths, resDirs, sourceHashes, ruleHash, libraryFactsFP, javaSemanticFactsFP) {
		return true
	}
	sourceSetFP, ok := resourceSourceSetFingerprintFromPaths(sourcePaths)
	if !ok {
		return false
	}
	mergedFP := mergedResourceIndexFingerprint(resDirs)
	fullKey := scanner.AndroidFindingsKey(scanner.AndroidFindingsKeyInputs{
		Kind:                scanner.AndroidFindingsKindResourceSourceBundle,
		RuleHash:            ruleHash,
		LibraryFactsFP:      libraryFactsFP,
		JavaSemanticFactsFP: javaSemanticFactsFP,
		InputFP:             sourceSetFP,
		Extra:               mergedFP,
	})
	cached, ok := scanner.LoadAndroidFindings(cacheDir, fullKey)
	if !ok {
		return false
	}
	shortEntries := make([]resourceSourceEntry, 0, len(sourcePaths))
	for _, path := range sourcePaths {
		hash := sourceHashes[path]
		if path == "" || hash == "" {
			return false
		}
		shortEntries = append(shortEntries, resourceSourceEntry{path: path, hash: hash})
	}
	shortFP, ok := resourceSourceEntriesFingerprint(shortEntries)
	if !ok {
		return false
	}
	shortKey := scanner.AndroidFindingsKey(scanner.AndroidFindingsKeyInputs{
		Kind:                scanner.AndroidFindingsKindResourceSourceBundle,
		RuleHash:            ruleHash,
		LibraryFactsFP:      libraryFactsFP,
		JavaSemanticFactsFP: javaSemanticFactsFP,
		InputFP:             shortFP,
		Extra:               mergedFP,
	})
	_ = scanner.SaveAndroidFindings(cacheDir, shortKey, cached)
	return true
}

// HasWarmResourceSourceBundleManifest reports whether a previous full
// resource-source bundle is available for the same source path set. This is
// the cache prerequisite for parsing only changed source files and applying a
// resource-source findings delta later in AndroidPhase.
func HasWarmResourceSourceBundleManifest(cacheDir string, sourcePaths, resDirs []string, ruleHash, libraryFactsFP, javaSemanticFactsFP string) bool {
	if cacheDir == "" || ruleHash == "" || len(sourcePaths) == 0 {
		return false
	}
	mergedFP := mergedResourceIndexFingerprint(resDirs)
	in := AndroidInput{
		RuleHash:            ruleHash,
		LibraryFactsFP:      libraryFactsFP,
		JavaSemanticFactsFP: javaSemanticFactsFP,
	}
	manifestKey, ok := in.resourceSourceBundleManifestKey(sourcePaths, mergedFP)
	if !ok {
		return false
	}
	manifest, ok := loadResourceSourceBundleManifest(cacheDir, manifestKey)
	if !ok {
		return false
	}
	_, ok = scanner.LoadAndroidFindings(cacheDir, manifest.BundleKey)
	return ok
}

func resourceSourceSetFingerprint(files []*scanner.File) (string, bool) {
	memo := hashutil.Default()
	entries := make([]resourceSourceEntry, 0, len(files))
	for _, file := range files {
		if file == nil || file.Path == "" {
			return "", false
		}
		var provider func() ([]byte, error)
		if len(file.Content) > 0 {
			content := file.Content
			provider = func() ([]byte, error) { return content, nil }
		}
		srcHash, err := memo.HashFile(file.Path, provider)
		if err != nil {
			return "", false
		}
		entries = append(entries, resourceSourceEntry{path: file.Path, hash: srcHash})
	}
	return resourceSourceEntriesFingerprint(entries)
}

func resourceSourceSetFingerprintFromPaths(paths []string) (string, bool) {
	memo := hashutil.Default()
	entries := make([]resourceSourceEntry, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			return "", false
		}
		srcHash, err := memo.HashFile(path, nil)
		if err != nil {
			return "", false
		}
		entries = append(entries, resourceSourceEntry{path: path, hash: srcHash})
	}
	return resourceSourceEntriesFingerprint(entries)
}

func resourceSourcePathSetFingerprint(paths []string) (string, bool) {
	if len(paths) == 0 {
		return "", false
	}
	h := hashutil.Hasher().New()
	sorted := append([]string(nil), paths...)
	sort.Strings(sorted)
	for _, path := range sorted {
		if path == "" {
			return "", false
		}
		h.Write([]byte(path))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), true
}

func resourceSourceEntriesFingerprintFromHashes(paths []string, hashes map[string]string) (string, bool) {
	entries := make([]resourceSourceEntry, 0, len(paths))
	for _, path := range paths {
		hash := hashes[path]
		if path == "" || hash == "" {
			return "", false
		}
		entries = append(entries, resourceSourceEntry{path: path, hash: hash})
	}
	return resourceSourceEntriesFingerprint(entries)
}

func resourceSourceEntriesFingerprint(entries []resourceSourceEntry) (string, bool) {
	h := hashutil.Hasher().New()
	sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })
	for _, e := range entries {
		h.Write([]byte(e.path))
		h.Write([]byte{0})
		h.Write([]byte(canonResourceSourceHash(e.hash)))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), true
}

// resourceSourceHashWidth is the canonical width, in hex characters, of every
// content hash that participates in a resource-source cache key, bundle
// fingerprint, or bundle manifest. The analysis cache's JSON backend records
// 16-char truncated hashes (cache.ComputeFileHash), while memo.HashFile and the
// unified store produce full 64-char hashes. Canonicalizing every hash to a
// single width before it enters a key lets a warm run reuse the per-file hashes
// the analysis cache already validated this run — without re-stat'ing every
// source file — and still land on the exact key a cold (full-hash) run wrote.
// 16 hex chars (64 bits) matches the truncation cache.ComputeFileHash already
// uses for the primary per-file analysis cache, so this adds no collision risk
// beyond what the cache subsystem already accepts.
const resourceSourceHashWidth = 16

// canonResourceSourceHash truncates a content hash to resourceSourceHashWidth so
// 16- and 64-char hashes for the same file produce identical resource-source
// keys. Shorter inputs are returned unchanged.
func canonResourceSourceHash(hash string) string {
	if len(hash) > resourceSourceHashWidth {
		return hash[:resourceSourceHashWidth]
	}
	return hash
}

// resourceSourceSetFingerprintReusingHashes computes the same fingerprint as
// resourceSourceSetFingerprint(in.SourceFiles), but pulls each file's content
// hash from in.SourceHashes when present, hashing only the files the analysis
// cache didn't cover — on a warm+ABI run that's just the changed file.
//
// By design this reuses the analysis cache's per-file hashes, so it inherits
// the cache's staleness model rather than the always-rehash-from-content
// behavior of resourceSourceSetFingerprint: under the git-dirty / watcher
// incremental shortcuts (see cache.CheckFiles / CheckFilesIncremental), a
// SourceHashes entry can be a prior run's hash that wasn't re-validated this
// run. That is intentional — it makes the resource-source bundle hit/miss share
// the same freshness guarantee as the main analysis cache that produced those
// hashes, instead of being spuriously fresher. Genuinely changed files are
// absent from SourceHashes (the cache only carries hashes for hits) and fall
// through to the rehash below, which is what preserves warm+ABI correctness.
//
// Because every fingerprint hash is canonicalized to resourceSourceHashWidth
// (see resourceSourceEntriesFingerprint / canonResourceSourceHash), the cache's
// 16-char hashes and a from-scratch 64-char rehash produce a byte-identical
// fingerprint, so reuse never shifts the key; only the per-file os.Stat +
// content-hash storm over thousands of unchanged files is avoided. Returns
// false (matching resourceSourceSetFingerprint) if any file is nil/path-less or
// an uncached file can't be hashed.
func (in AndroidInput) resourceSourceSetFingerprintReusingHashes() (string, bool) {
	memo := hashutil.Default()
	entries := make([]resourceSourceEntry, 0, len(in.SourceFiles))
	for _, file := range in.SourceFiles {
		if file == nil || file.Path == "" {
			return "", false
		}
		hash := in.SourceHashes[file.Path]
		if hash == "" {
			var provider func() ([]byte, error)
			if len(file.Content) > 0 {
				content := file.Content
				provider = func() ([]byte, error) { return content, nil }
			}
			h, err := memo.HashFile(file.Path, provider)
			if err != nil {
				return "", false
			}
			hash = h
		}
		entries = append(entries, resourceSourceEntry{path: file.Path, hash: hash})
	}
	return resourceSourceEntriesFingerprint(entries)
}
