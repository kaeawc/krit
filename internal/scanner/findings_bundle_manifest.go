package scanner

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/hashutil"
)

// StatFile is the default FileStatProvider: a thin os.Stat wrapper
// that returns the size + modtime tuple stored in FindingsBundleManifest.
// Callers needing a stub for tests should pass their own provider to
// StaleOracleCandidates.
func StatFile(path string) (FileStat, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return FileStat{}, false
	}
	return FileStat{
		Size:            info.Size(),
		ModTimeUnixNano: info.ModTime().UnixNano(),
	}, true
}

// FindingsBundleManifest persists the data the ConservativeDeltaPlanner
// needs to decide whether a single-file edit can take the delta path:
//
//   - The prior run's RunFingerprint (so the planner can compare
//     stability of each non-SourceSet field).
//   - The prior run's per-file content hashes (so we can detect
//     exactly which files changed since last run).
//   - The prior run's per-file structural fingerprints, used to prove
//     body-only Kotlin edits can replay the prior findings bundle.
//   - The BundleKey of the prior bundle so we know which findings to
//     load and merge into.
//
// Keyed by a stable run identifier (the project root + sorted scan
// paths) so distinct projects don't trample each other.
// findingsBundleManifestVersion bumps whenever the on-disk schema
// changes in a way that prior daemons would mis-read. Bumping this
// invalidates every cached manifest so callers fall back to recompute;
// daemons forward-compat by virtue of treating "wrong version" as
// "missing" in LoadFindingsBundleManifestFromPath.
//
// v1 → v2: AbiHashes added. v1 readers silently ignore the new field;
// v2 readers tolerate v1 manifests at load time (treated as ok=false
// in LoadFindingsBundleManifestFromPath so the next save rewrites).
const findingsBundleManifestVersion = 2

type FindingsBundleManifest struct {
	Version       int                 `json:"version"`
	Key           string              `json:"key"`
	BundleKey     string              `json:"bundleKey"`
	Fingerprint   RunFingerprint      `json:"fingerprint"`
	ContentHashes map[string]string   `json:"contentHashes"`
	StructuralFPs map[string]string   `json:"structuralFps,omitempty"`
	FileStats     map[string]FileStat `json:"fileStats,omitempty"`
	// AbiHashes is per-file public-API hash computed via
	// arch.ExtractAbiSignatures + arch.HashAbiSignatures. Populated by
	// buildManifestData when bundleEnabled. The oracle freshness gate
	// reads this field to distinguish body-only edits (skip dependent
	// invalidation) from public-ABI edits (eventually drives
	// transitive invalidation in a future iteration; v1 only invalidates
	// the edited file itself via stat comparison). Kotlin files only;
	// Java entries are absent.
	AbiHashes map[string]string `json:"abiHashes,omitempty"`
}

type FileStat struct {
	Size            int64 `json:"size"`
	ModTimeUnixNano int64 `json:"modTimeUnixNano"`
}

// FindingsBundleManifestKey derives a stable manifest identifier from
// a project root + sorted scan paths. The repoDir is included so
// daemons running against multiple projects don't collide.
func FindingsBundleManifestKey(repoDir string, scanPaths []string) string {
	if repoDir == "" {
		return ""
	}
	sorted := append([]string(nil), scanPaths...)
	for i, p := range sorted {
		if abs, err := filepath.Abs(p); err == nil {
			sorted[i] = abs
		}
	}
	h := hashutil.Hasher().New()
	_, _ = h.Write([]byte(repoDir))
	_, _ = h.Write([]byte{0})
	for _, p := range sorted {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
	}
	return hashutil.HashHex(h.Sum(nil))
}

// FindingsBundleManifestPath returns the on-disk location of the
// manifest for the given (repoDir, key) pair. Exported so daemon
// callers can build a key-by-path cache without re-implementing the
// shard-prefix scheme.
func FindingsBundleManifestPath(repoDir, key string) string {
	return findingsBundleManifestPath(repoDir, key)
}

func findingsBundleManifestPath(repoDir, key string) string {
	if repoDir == "" || key == "" {
		return ""
	}
	if len(key) >= 2 {
		return filepath.Join(FindingsBundleCacheDir(repoDir), "manifests", key[:2], key[2:]+".json")
	}
	return filepath.Join(FindingsBundleCacheDir(repoDir), "manifests", key+".json")
}

// LoadFindingsBundleManifest reads the manifest for the given run key,
// returning (manifest, true) on success or (zero, false) when it's
// missing or invalid. Manifest-version mismatches are treated as
// missing so a krit upgrade doesn't serve stale entries.
func LoadFindingsBundleManifest(repoDir, key string) (FindingsBundleManifest, bool) {
	path := findingsBundleManifestPath(repoDir, key)
	if path == "" {
		return FindingsBundleManifest{}, false
	}
	m, ok := LoadFindingsBundleManifestFromPath(path)
	if !ok {
		return FindingsBundleManifest{}, false
	}
	if m.Key != key {
		return FindingsBundleManifest{}, false
	}
	return m, true
}

// LoadFindingsBundleManifestFromPath reads the manifest from an
// already-resolved path. Daemon callers that maintain a path-keyed
// cache use this entry point so the key-equality check (which only
// the disk-key path can perform) is the caller's responsibility — the
// daemon already knows the entry came from a path it generated for
// the current scan set.
func LoadFindingsBundleManifestFromPath(path string) (FindingsBundleManifest, bool) {
	if path == "" {
		return FindingsBundleManifest{}, false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return FindingsBundleManifest{}, false
	}
	var m FindingsBundleManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return FindingsBundleManifest{}, false
	}
	if m.Version != findingsBundleManifestVersion || len(m.ContentHashes) == 0 {
		return FindingsBundleManifest{}, false
	}
	return m, true
}

// FileStatProvider returns the on-disk stat for a path. Callers pass
// os.Stat-backed implementations in production; tests substitute a
// stub for deterministic input.
type FileStatProvider func(path string) (FileStat, bool)

// ContentHashProvider returns the current content hash of a path. Used
// by StaleOracleCandidates to disambiguate stat-drift caused by mtime
// bumps (gradle regen, git checkout, IDE touch) from real content
// changes. Returning (_, false) leaves the path on the stale list —
// the caller already saw stat drift, so the hash fallback should be
// strictly additive.
type ContentHashProvider func(path string) (string, bool)

// StaleOracleCandidates returns the subset of paths whose file stat
// differs from the prior manifest entry. Used by the oracle
// freshness gate to identify .kt files that need a partial KAA
// reanalyze without paying for a full content-hash recompute first
// — the InvokeCachedWithOptions classifier then verifies via SHA-256
// inside the JVM-bound miss path. A nil or empty prior manifest is
// treated as "everything is potentially stale" so we never silently
// reuse an oracle JSON that has no manifest evidence behind it; the
// caller should special-case this if the historical lazy-load
// behavior is desired (e.g. one-shot CLI with no daemon).
//
// stat is required and must be non-nil. Files whose stat is
// unavailable (deleted, unreadable) are treated as stale.
//
// hash is optional. When non-nil and a path's stat differs from prior
// but its content hash matches prior.ContentHashes[path], the path
// is treated as fresh — covering the common mtime-only drift case
// (git checkout, gradle re-emit with identical bytes, IDE touch).
// Hashing is only paid for paths that already failed the stat check,
// so the cost is bounded by the size of the stat-drifted subset.
func StaleOracleCandidates(paths []string, prior FindingsBundleManifest, stat FileStatProvider, hash ContentHashProvider) []string {
	if stat == nil {
		return paths
	}
	if len(prior.FileStats) == 0 && len(prior.ContentHashes) == 0 {
		return paths
	}
	priorStats := prior.FileStats
	priorHashes := prior.ContentHashes
	stale := make([]string, 0)
	contentMatches := func(path string) bool {
		if hash == nil || len(priorHashes) == 0 {
			return false
		}
		priorHash, hadHash := priorHashes[path]
		if !hadHash || priorHash == "" {
			return false
		}
		current, ok := hash(path)
		if !ok || current == "" {
			return false
		}
		return current == priorHash
	}
	for _, path := range paths {
		current, ok := stat(path)
		if !ok {
			stale = append(stale, path)
			continue
		}
		if priorStats == nil {
			// Manifest predates FileStats — fall back to "if prior
			// had a content hash we trust it, otherwise stale". This
			// keeps freshly-upgraded daemons from re-running every
			// file when the on-disk manifest format is one version
			// behind.
			if _, hadHash := priorHashes[path]; !hadHash {
				stale = append(stale, path)
			}
			continue
		}
		priorStat, hadStat := priorStats[path]
		if !hadStat {
			// New path not in prior manifest — content-hash fallback
			// can't help (no prior hash to compare against).
			stale = append(stale, path)
			continue
		}
		if priorStat == current {
			continue
		}
		if contentMatches(path) {
			continue
		}
		stale = append(stale, path)
	}
	return stale
}

// SaveFindingsBundleManifest persists the run manifest atomically.
// Best-effort: errors are returned but callers (RunProject's save
// path) typically log-and-continue rather than failing the whole
// verb on a manifest write error.
func SaveFindingsBundleManifest(repoDir, key string, manifest FindingsBundleManifest) error {
	path := findingsBundleManifestPath(repoDir, key)
	if path == "" {
		return nil
	}
	manifest.Version = findingsBundleManifestVersion
	manifest.Key = key
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, raw, 0o644)
}
