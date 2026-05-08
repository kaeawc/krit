package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/kaeawc/krit/internal/fsutil"
)

const resDirFingerprintCacheVersion = 1

type resDirFingerprintCache struct {
	path   string
	loaded bool
	data   resDirFingerprintCacheData
}

type resDirFingerprintCacheData struct {
	Version int                          `json:"version"`
	Entries map[string]resDirFingerprint `json:"entries"`
}

type resDirFingerprint struct {
	Fingerprint string                  `json:"fingerprint"`
	Files       []resDirFingerprintFile `json:"files"`
}

type resDirFingerprintFile struct {
	Rel     string `json:"rel"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"modTime"`
}

func newResDirFingerprintCache(cacheDir string) *resDirFingerprintCache {
	if cacheDir == "" {
		return nil
	}
	return &resDirFingerprintCache{path: filepath.Join(cacheDir, "resource-fingerprints.json")}
}

func (c *resDirFingerprintCache) fingerprint(resDir string) string {
	files, ok := snapshotResDirFingerprintFiles(resDir)
	if !ok {
		return resDirContentFingerprint(resDir)
	}
	c.load()
	if entry, ok := c.data.Entries[resDir]; ok && sameResDirFingerprintFiles(entry.Files, files) && entry.Fingerprint != "" {
		return entry.Fingerprint
	}
	fp := resDirContentFingerprint(resDir)
	c.data.Entries[resDir] = resDirFingerprint{Fingerprint: fp, Files: files}
	_ = c.save()
	return fp
}

func (c *resDirFingerprintCache) load() {
	if c == nil || c.loaded {
		return
	}
	c.loaded = true
	c.data = resDirFingerprintCacheData{
		Version: resDirFingerprintCacheVersion,
		Entries: make(map[string]resDirFingerprint),
	}
	raw, err := os.ReadFile(c.path)
	if err != nil {
		return
	}
	var data resDirFingerprintCacheData
	if err := json.Unmarshal(raw, &data); err != nil || data.Version != resDirFingerprintCacheVersion || data.Entries == nil {
		return
	}
	c.data = data
}

func (c *resDirFingerprintCache) save() error {
	if c == nil || c.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}
	raw, err := json.Marshal(c.data)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(c.path, raw, 0o644)
}

func snapshotResDirFingerprintFiles(resDir string) ([]resDirFingerprintFile, bool) {
	files := make([]resDirFingerprintFile, 0)
	err := filepath.WalkDir(resDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(resDir, path)
		if err != nil {
			return err
		}
		files = append(files, resDirFingerprintFile{
			Rel:     rel,
			Size:    info.Size(),
			ModTime: info.ModTime().UnixNano(),
		})
		return nil
	})
	if err != nil {
		return nil, false
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Rel < files[j].Rel })
	return files, true
}

func sameResDirFingerprintFiles(a, b []resDirFingerprintFile) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
