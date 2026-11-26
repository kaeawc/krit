package testutil

import (
	"os"
	"testing"
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
