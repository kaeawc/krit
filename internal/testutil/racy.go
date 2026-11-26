package testutil

import (
	"os"
	"testing"
	"time"
)

// RewriteKeepingStat replaces path's content with a same-size payload and
// restores its previous mtime: what a rewrite inside one filesystem
// timestamp tick looks like to stat, made deterministic for tests of
// stat-keyed caches.
func RewriteKeepingStat(t testing.TB, path, content string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if int64(len(content)) != info.Size() {
		t.Fatalf("rewrite must keep the size: %d bytes, %s has %d", len(content), path, info.Size())
	}
	if err := os.WriteFile(path, []byte(content), info.Mode().Perm()); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
}

// Age dates path an hour back, as an unchanged file looks to a later
// process once it is older than hashutil.RacyWindow.
func Age(t testing.TB, path string) {
	t.Helper()
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
}
