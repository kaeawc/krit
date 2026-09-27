package buildid

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"testing"
)

func isolatedHashCache(t *testing.T) string {
	t.Helper()
	t.Setenv("KRIT_BUILDID_CACHE_BASE", "")
	base := t.TempDir()
	oldDir, oldHashes := userCacheDir, hashes
	userCacheDir = func() (string, error) { return base, nil }
	hashes = make(map[fileKey]string)
	t.Cleanup(func() { userCacheDir, hashes = oldDir, oldHashes })
	return filepath.Join(base, "krit", "buildid-hashes.json")
}

func TestHashCachePathOverride(t *testing.T) {
	base := t.TempDir()
	t.Setenv("KRIT_BUILDID_CACHE_BASE", base)
	if got, want := hashCachePath(), filepath.Join(base, "krit", "buildid-hashes.json"); got != want {
		t.Fatalf("hashCachePath() = %q, want %q", got, want)
	}
}

func TestTokenMemoizedAndExecutableFallback(t *testing.T) {
	isolatedHashCache(t)
	path := filepath.Join(t.TempDir(), "krit")
	if err := os.WriteFile(path, []byte("first build"), 0600); err != nil {
		t.Fatal(err)
	}
	oldInfo, oldExe, oldOnce, oldToken := readBuildInfo, executable, tokenOnce, token
	t.Cleanup(func() { readBuildInfo, executable, tokenOnce, token = oldInfo, oldExe, oldOnce, oldToken })
	readBuildInfo = func() (*debug.BuildInfo, bool) { return nil, false }
	executable = func() (string, error) { return path, nil }
	tokenOnce = new(sync.Once)
	token = ""
	first := Token()
	if !strings.HasPrefix(first, "exe:") {
		t.Fatalf("Token() = %q, want exe: prefix", first)
	}
	if err := os.WriteFile(path, []byte("second build"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := Token(); got != first {
		t.Fatalf("memoized Token() = %q, want %q", got, first)
	}
}

func TestJarTokenChangesWithContents(t *testing.T) {
	isolatedHashCache(t)
	path := filepath.Join(t.TempDir(), "krit-fir.jar")
	if err := os.WriteFile(path, []byte("a"), 0600); err != nil {
		t.Fatal(err)
	}
	first := JarToken(path)
	if err := os.WriteFile(path, []byte("different bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if second := JarToken(path); first == second {
		t.Fatal("jar content change kept the same token")
	}
}

func TestTokenFallsBackForDirtyAndMissingRevision(t *testing.T) {
	isolatedHashCache(t)
	path := filepath.Join(t.TempDir(), "krit")
	if err := os.WriteFile(path, []byte("fallback build"), 0600); err != nil {
		t.Fatal(err)
	}
	wantHash := sha256.Sum256([]byte("fallback build"))
	want := "exe:" + hex.EncodeToString(wantHash[:])
	oldInfo, oldExe, oldOnce, oldToken := readBuildInfo, executable, tokenOnce, token
	t.Cleanup(func() { readBuildInfo, executable, tokenOnce, token = oldInfo, oldExe, oldOnce, oldToken })
	executable = func() (string, error) { return path, nil }
	for _, tc := range []struct {
		name     string
		settings []debug.BuildSetting
	}{
		{"dirty revision", []debug.BuildSetting{{Key: "vcs.revision", Value: "abc"}, {Key: "vcs.modified", Value: "true"}}},
		{"empty revision", []debug.BuildSetting{{Key: "vcs.revision", Value: ""}, {Key: "vcs.modified", Value: "false"}}},
		{"absent revision", []debug.BuildSetting{{Key: "vcs.modified", Value: "false"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			readBuildInfo = func() (*debug.BuildInfo, bool) { return &debug.BuildInfo{Settings: tc.settings}, true }
			tokenOnce = new(sync.Once)
			token = ""
			if got := Token(); got != want {
				t.Fatalf("Token() = %q, want %q", got, want)
			}
		})
	}
}

func TestPersistentHashMemo(t *testing.T) {
	cachePath := isolatedHashCache(t)
	path := filepath.Join(t.TempDir(), "jar")
	if err := os.WriteFile(path, []byte("initial bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	first, err := HashFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	var entries []hashEntry
	if err := json.Unmarshal(data, &entries); err != nil || len(entries) != 1 {
		t.Fatalf("persistent cache is not complete JSON: %v, entries=%d", err, len(entries))
	}
	// A process-like reset must recover the digest from disk. Making the file
	// unreadable proves this lookup did not re-open and rehash the file.
	hashes = make(map[fileKey]string)
	if err := os.Chmod(path, 0000); err != nil {
		t.Fatal(err)
	}
	if got, err := HashFile(path); err != nil || got != first {
		t.Fatalf("disk memo lookup = %q, %v; want %q", got, err, first)
	}
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	// A corrupt memo is a miss, and hashing still succeeds.
	if err := os.WriteFile(cachePath, []byte("not JSON"), 0600); err != nil {
		t.Fatal(err)
	}
	hashes = make(map[fileKey]string)
	if got, err := HashFile(path); err != nil || got != first {
		t.Fatalf("corrupt memo fallback = %q, %v; want %q", got, err, first)
	}
	if data, err := os.ReadFile(cachePath); err != nil || json.Unmarshal(data, &entries) != nil {
		t.Fatalf("atomic replacement left partial JSON: %v", err)
	}
	if leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(cachePath), ".buildid-hashes-*")); len(leftovers) != 0 {
		t.Fatalf("temporary files left after replacement: %v", leftovers)
	}
}

func TestPersistentHashMemoUnavailableAndBounded(t *testing.T) {
	cachePath := isolatedHashCache(t)
	dir := t.TempDir()
	for i := 0; i <= maxHashes; i++ {
		path := filepath.Join(dir, string(rune('a'+i)))
		if err := os.WriteFile(path, []byte(path), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := HashFile(path); err != nil {
			t.Fatal(err)
		}
	}
	if entries := readHashes(cachePath); len(entries) != maxHashes {
		t.Fatalf("persistent memo has %d entries, want %d", len(entries), maxHashes)
	}
	userCacheDir = func() (string, error) { return "", os.ErrPermission }
	hashes = make(map[fileKey]string)
	path := filepath.Join(dir, "fallback")
	if err := os.WriteFile(path, []byte("fallback"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := HashFile(path); err != nil {
		t.Fatalf("unavailable cache blocked direct hashing: %v", err)
	}
}
