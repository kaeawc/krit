package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/fsutil"
)

type resourceSourceEntry struct {
	path string
	hash string
}

type resourceSourceBundleManifest struct {
	Version   int               `json:"version"`
	Key       string            `json:"key"`
	BundleKey string            `json:"bundleKey"`
	Hashes    map[string]string `json:"hashes"`
}

// resourceSourceBundleManifestVersion is bumped to 2 alongside the canonical
// 16-char hash-width migration: a v1 manifest stores full-width (64-char)
// hashes, which would width-mismatch every path against the new 16-char
// currentHashes and force a needless full re-sweep. Bumping invalidates stale
// manifests cleanly so the first warm run after upgrade rebuilds once.
const resourceSourceBundleManifestVersion = 2

func resourceSourceBundleManifestPath(cacheDir, key string) string {
	if cacheDir == "" || key == "" {
		return ""
	}
	if len(key) >= 2 {
		return filepath.Join(cacheDir, "resource-source-bundles", key[:2], key[2:]+".json")
	}
	return filepath.Join(cacheDir, "resource-source-bundles", key+".json")
}

func loadResourceSourceBundleManifest(cacheDir, key string) (resourceSourceBundleManifest, bool) {
	path := resourceSourceBundleManifestPath(cacheDir, key)
	if path == "" {
		return resourceSourceBundleManifest{}, false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return resourceSourceBundleManifest{}, false
	}
	var manifest resourceSourceBundleManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return resourceSourceBundleManifest{}, false
	}
	if manifest.Version != resourceSourceBundleManifestVersion || manifest.Key != key || manifest.BundleKey == "" || len(manifest.Hashes) == 0 {
		return resourceSourceBundleManifest{}, false
	}
	return manifest, true
}

func saveResourceSourceBundleManifest(cacheDir, key, bundleKey string, hashes map[string]string) error {
	path := resourceSourceBundleManifestPath(cacheDir, key)
	if path == "" || bundleKey == "" || len(hashes) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// Store canonical-width hashes so a cold (64-char memo) save and a warm
	// (16-char cache) delta compare against byte-identical values.
	copyHashes := make(map[string]string, len(hashes))
	for path, hash := range hashes {
		copyHashes[path] = canonResourceSourceHash(hash)
	}
	raw, err := json.Marshal(resourceSourceBundleManifest{
		Version:   resourceSourceBundleManifestVersion,
		Key:       key,
		BundleKey: bundleKey,
		Hashes:    copyHashes,
	})
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, raw, 0o644)
}

// lookupResourceSourceBundleManifest returns the resource-source bundle
// manifest for key, consulting the daemon-resident mirror first (a map
// lookup) before falling back to the on-disk JSON read. A disk hit
// populates the mirror so subsequent warm analyzes skip the read.
func (in AndroidInput) lookupResourceSourceBundleManifest(key string) (resourceSourceBundleManifest, bool) {
	if in.ResidentResourceSourceManifest != nil {
		if manifest, ok := in.ResidentResourceSourceManifest(key); ok {
			return manifest, true
		}
	}
	manifest, ok := loadResourceSourceBundleManifest(in.CacheDir, key)
	if ok && in.StoreResidentResourceSourceManifest != nil {
		in.StoreResidentResourceSourceManifest(key, manifest)
	}
	return manifest, ok
}

// persistResourceSourceBundleManifest mirrors the manifest into the
// daemon-resident cache synchronously, then writes it to disk — off the
// warm critical path via BackgroundSave when wired, inline otherwise.
// Because the resident mirror is updated before this returns, a deferred
// (not-yet-flushed) disk write never costs the next analyze a re-sweep;
// only a daemon restart before the flush falls back to a recompute.
func (in AndroidInput) persistResourceSourceBundleManifest(key, bundleKey string, hashes map[string]string) {
	if key == "" || bundleKey == "" || len(hashes) == 0 {
		return
	}
	if in.StoreResidentResourceSourceManifest != nil {
		// Store canonical-width hashes so the resident value is
		// byte-identical to what loadResourceSourceBundleManifest would
		// return after the disk write lands.
		canon := make(map[string]string, len(hashes))
		for path, hash := range hashes {
			canon[path] = canonResourceSourceHash(hash)
		}
		in.StoreResidentResourceSourceManifest(key, resourceSourceBundleManifest{
			Version:   resourceSourceBundleManifestVersion,
			Key:       key,
			BundleKey: bundleKey,
			Hashes:    canon,
		})
	}
	write := func() { _ = saveResourceSourceBundleManifest(in.CacheDir, key, bundleKey, hashes) }
	if in.BackgroundSave != nil {
		in.BackgroundSave(write)
	} else {
		write()
	}
}

const mergedResourceIndexBundleVersion = 1

type mergedResourceIndexBundlePayload struct {
	Version int                    `json:"version"`
	Key     string                 `json:"key"`
	Index   *android.ResourceIndex `json:"index"`
}

func mergedResourceIndexBundlePath(cacheDir, key string) string {
	if cacheDir == "" || key == "" {
		return ""
	}
	if len(key) >= 2 {
		return filepath.Join(cacheDir, "resource-index-bundles", key[:2], key[2:]+".bin")
	}
	return filepath.Join(cacheDir, "resource-index-bundles", key+".bin")
}

func loadMergedResourceIndexBundle(cacheDir, key string) (*android.ResourceIndex, bool) {
	path := mergedResourceIndexBundlePath(cacheDir, key)
	if path == "" {
		return nil, false
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	var payload mergedResourceIndexBundlePayload
	if err := cacheutil.DecodeZstdGob(f, &payload); err != nil {
		return nil, false
	}
	if payload.Version != mergedResourceIndexBundleVersion || payload.Key != key || payload.Index == nil {
		return nil, false
	}
	return payload.Index, true
}

func saveMergedResourceIndexBundle(cacheDir, key string, idx *android.ResourceIndex) error {
	path := mergedResourceIndexBundlePath(cacheDir, key)
	if path == "" || idx == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := cacheutil.EncodeZstdGob(mergedResourceIndexBundlePayload{
		Version: mergedResourceIndexBundleVersion,
		Key:     key,
		Index:   idx,
	})
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, raw, 0o644)
}
