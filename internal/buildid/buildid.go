// Package buildid identifies the running Krit build and JVM backends for caches.
package buildid

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
)

const maxHashes = 64

var (
	readBuildInfo = debug.ReadBuildInfo
	executable    = os.Executable
	userCacheDir  = os.UserCacheDir
	tokenOnce     = new(sync.Once)
	token         string
	hashesMu      sync.Mutex
	hashes        = make(map[fileKey]string)
)

type fileKey struct {
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	Mtime int64  `json:"mtime"`
	Inode uint64 `json:"inode"`
}

type hashEntry struct {
	Key  fileKey `json:"key"`
	Hash string  `json:"hash"`
}

// Token identifies this build. Clean VCS builds use their revision; other
// builds use the executable bytes so local rule edits invalidate caches.
func Token() string {
	tokenOnce.Do(func() {
		if info, ok := readBuildInfo(); ok {
			var revision, modified string
			for _, setting := range info.Settings {
				switch setting.Key {
				case "vcs.revision":
					revision = setting.Value
				case "vcs.modified":
					modified = setting.Value
				}
			}
			if revision != "" && modified == "false" {
				token = "rev:" + revision
				return
			}
		}
		if path, err := executable(); err == nil {
			if resolved, err := filepath.EvalSymlinks(path); err == nil {
				path = resolved
			}
			if hash, err := HashFile(path); err == nil {
				token = "exe:" + hash
				return
			}
		}
		token = "unknown"
	})
	return token
}

// HashFile returns the SHA-256 hex digest of a file's contents. Cache failures
// are ignored; the file itself remains the source of truth.
func HashFile(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return hashFileDirect(path)
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return hashFileDirect(path)
	}
	key := fileKey{Path: path, Size: info.Size(), Mtime: info.ModTime().UnixNano(), Inode: fileInode(info)}
	hashesMu.Lock()
	defer hashesMu.Unlock()
	if hash, ok := hashes[key]; ok {
		return hash, nil
	}
	cachePath := hashCachePath()
	if cachePath != "" {
		for _, entry := range readHashes(cachePath) {
			if entry.Key == key && validHash(entry.Hash) {
				hashes[key] = entry.Hash
				return entry.Hash, nil
			}
		}
	}
	hash, err := hashFileDirect(path)
	if err != nil {
		return "", err
	}
	hashes[key] = hash
	if cachePath != "" {
		// Re-read just before writing so sequential processes retain each other's
		// entries. Concurrent writers may lose an entry, which only costs speed.
		entries := readHashes(cachePath)
		filtered := entries[:0]
		for _, entry := range entries {
			if entry.Key != key {
				filtered = append(filtered, entry)
			}
		}
		filtered = append(filtered, hashEntry{Key: key, Hash: hash})
		// Insertion order is oldest first; evict the oldest above the cap.
		if len(filtered) > maxHashes {
			filtered = filtered[len(filtered)-maxHashes:]
		}
		_ = writeHashes(cachePath, filtered)
	}
	return hash, nil
}

func hashFileDirect(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func hashCachePath() string {
	// An explicit base allows isolated runs (including sandboxed benchmarks)
	// to exercise the persistent memo without changing the process HOME.
	if base := os.Getenv("KRIT_BUILDID_CACHE_BASE"); base != "" {
		return filepath.Join(base, "krit", "buildid-hashes.json")
	}
	base, err := userCacheDir()
	if err != nil || base == "" {
		return ""
	}
	return filepath.Join(base, "krit", "buildid-hashes.json")
}

func readHashes(path string) []hashEntry {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var entries []hashEntry
	if json.Unmarshal(data, &entries) != nil || len(entries) > maxHashes {
		return nil
	}
	return entries
}

func validHash(hash string) bool {
	if len(hash) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(hash)
	return err == nil
}

func writeHashes(path string, entries []hashEntry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".buildid-hashes-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

// JarToken identifies jar bytes through the same cross-process memo as HashFile.
func JarToken(path string) string {
	if path == "" {
		return "missing"
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "missing"
	}
	hash, err := HashFile(path)
	if err != nil {
		return "unknown"
	}
	return hash
}
