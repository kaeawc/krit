//go:build unix

package firchecks

import (
	"os"
	"syscall"
	"testing"
)

func TestWriteCacheEntry_HonorsUmask(t *testing.T) {
	old := syscall.Umask(0o077)
	defer syscall.Umask(old)
	cacheDir, _ := CacheDir(t.TempDir())
	e := &FirCacheEntry{V: FirCacheVersion, ContentHash: "h", FilePath: "/src/A.kt"}
	if err := WriteCacheEntry(cacheDir, e); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(entryPath(cacheDir, "/src/A.kt", "h", ""))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("stat=%v err=%v, want 0600 under umask 077", info, err)
	}
}
