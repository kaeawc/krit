package librarymodel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/hashutil"
)

const (
	projectProfileCacheDirName = "librarymodel-cache"
	projectProfileCacheVersion = 1
)

var (
	projectProfileCacheHits      atomic.Int64
	projectProfileCacheMisses    atomic.Int64
	projectProfileCacheBytes     atomic.Int64
	projectProfileCacheLastWrite atomic.Int64
)

func init() {
	cacheutil.Register(projectProfileCacheRegistered{})
}

type projectProfileCacheRegistered struct{}

func (projectProfileCacheRegistered) Name() string { return projectProfileCacheDirName }
func (projectProfileCacheRegistered) Clear(ctx cacheutil.ClearContext) error {
	return os.RemoveAll(ProjectProfileCacheDir(ctx.RepoDir))
}
func (projectProfileCacheRegistered) Stats() cacheutil.CacheStats {
	return cacheutil.CacheStats{
		Bytes:         projectProfileCacheBytes.Load(),
		Hits:          projectProfileCacheHits.Load(),
		Misses:        projectProfileCacheMisses.Load(),
		LastWriteUnix: projectProfileCacheLastWrite.Load(),
	}
}

// ProjectProfileCacheDir returns the repository-local cache root for Gradle
// library model profiles.
func ProjectProfileCacheDir(repoDir string) string {
	if repoDir == "" {
		return ""
	}
	return filepath.Join(repoDir, ".krit", projectProfileCacheDirName)
}

type projectProfileCachePayload struct {
	Version     int            `json:"version"`
	Hasher      string         `json:"hasher"`
	PathsKey    string         `json:"pathsKey"`
	Fingerprint string         `json:"fingerprint"`
	InputPaths  []string       `json:"inputPaths"`
	Profile     ProjectProfile `json:"profile"`
}

// LoadProjectProfileCache loads a profile only when all known Gradle/catalog
// inputs still match. New standard catalog or settings files in ancestor dirs
// also change the fingerprint and force a miss.
func LoadProjectProfileCache(cacheDir string, paths []string) (ProjectProfile, bool) {
	if cacheDir == "" {
		return ProjectProfile{}, false
	}
	payloadPath := projectProfileCachePath(cacheDir, paths)
	data, err := os.ReadFile(payloadPath)
	if err != nil {
		projectProfileCacheMisses.Add(1)
		return ProjectProfile{}, false
	}
	var payload projectProfileCachePayload
	if err := json.Unmarshal(data, &payload); err != nil ||
		payload.Version != projectProfileCacheVersion ||
		payload.Hasher != hashutil.HasherName() ||
		payload.PathsKey != projectProfilePathsKey(paths) {
		projectProfileCacheMisses.Add(1)
		return ProjectProfile{}, false
	}
	if got := projectProfileFingerprint(paths, payload.InputPaths); got != payload.Fingerprint {
		projectProfileCacheMisses.Add(1)
		return ProjectProfile{}, false
	}
	projectProfileCacheHits.Add(1)
	projectProfileCacheBytes.Store(int64(len(data)))
	return payload.Profile, true
}

// SaveProjectProfileCache persists the profile plus every discovered catalog
// input so warm runs can avoid repeating settings/build-logic discovery.
func SaveProjectProfileCache(cacheDir string, paths []string, profile ProjectProfile) error {
	if cacheDir == "" {
		return nil
	}
	inputs := projectProfileInputPaths(paths, profile)
	payload := projectProfileCachePayload{
		Version:     projectProfileCacheVersion,
		Hasher:      hashutil.HasherName(),
		PathsKey:    projectProfilePathsKey(paths),
		Fingerprint: projectProfileFingerprint(paths, inputs),
		InputPaths:  inputs,
		Profile:     profile,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}
	path := projectProfileCachePath(cacheDir, paths)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	projectProfileCacheBytes.Store(int64(len(data)))
	projectProfileCacheLastWrite.Store(time.Now().Unix())
	return nil
}

func projectProfileCachePath(cacheDir string, paths []string) string {
	return filepath.Join(cacheDir, "profile-"+projectProfilePathsKey(paths)[:16]+".json")
}

func projectProfilePathsKey(paths []string) string {
	return hashutil.HashHex([]byte(strings.Join(normalizePaths(paths), "\x00")))
}

func projectProfileInputPaths(paths []string, profile ProjectProfile) []string {
	var inputs []string
	inputs = append(inputs, paths...)
	for _, source := range profile.CatalogSources {
		inputs = append(inputs, source.Path)
	}
	return normalizePaths(inputs)
}

func projectProfileFingerprint(paths, extraInputs []string) string {
	type item struct {
		Kind string `json:"kind"`
		Path string `json:"path"`
		Hash string `json:"hash,omitempty"`
	}
	var items []item
	for _, path := range normalizePaths(append(append([]string(nil), paths...), extraInputs...)) {
		items = append(items, item{Kind: "file", Path: path, Hash: fileHashOrMissing(path)})
	}
	for _, dir := range projectProfileAncestorDirs(paths) {
		items = append(items, item{Kind: "catalog-dir", Path: dir, Hash: catalogDirFingerprint(dir)})
	}
	data, _ := json.Marshal(items)
	return hashutil.HashHex(data)
}

func projectProfileAncestorDirs(paths []string) []string {
	seen := make(map[string]bool)
	var dirs []string
	for _, path := range normalizePaths(paths) {
		for dir := filepath.Dir(path); dir != "." && dir != string(filepath.Separator); dir = filepath.Dir(dir) {
			if !seen[dir] {
				seen[dir] = true
				dirs = append(dirs, dir)
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
		}
	}
	sort.Strings(dirs)
	return dirs
}

func catalogDirFingerprint(dir string) string {
	var parts []string
	for _, name := range []string{"settings.gradle", "settings.gradle.kts"} {
		path := filepath.Join(dir, name)
		parts = append(parts, path+"="+fileHashOrMissing(path))
	}
	matches, err := filepath.Glob(filepath.Join(dir, "gradle", "*.versions.toml"))
	if err == nil {
		sort.Strings(matches)
		for _, path := range matches {
			parts = append(parts, path+"="+fileHashOrMissing(path))
		}
	}
	return hashutil.HashHex([]byte(strings.Join(parts, "\x00")))
}

func fileHashOrMissing(path string) string {
	if path == "" {
		return "missing"
	}
	hash, err := hashutil.Default().HashFile(path, nil)
	if err != nil {
		return "missing"
	}
	return hash
}

func normalizePaths(paths []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
		path = filepath.Clean(path)
		if seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}
