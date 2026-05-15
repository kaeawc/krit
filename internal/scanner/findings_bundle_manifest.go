package scanner

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/hashutil"
)

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
const findingsBundleManifestVersion = 1

type FindingsBundleManifest struct {
	Version       int                 `json:"version"`
	Key           string              `json:"key"`
	BundleKey     string              `json:"bundleKey"`
	Fingerprint   RunFingerprint      `json:"fingerprint"`
	ContentHashes map[string]string   `json:"contentHashes"`
	StructuralFPs map[string]string   `json:"structuralFps,omitempty"`
	FileStats     map[string]FileStat `json:"fileStats,omitempty"`
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
