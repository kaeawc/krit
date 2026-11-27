package scan

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/perf"
)

// The file-walk cache replays a directory's listing while the directory's
// mtime is unchanged. --no-cache, the fresh-analysis baseline, must walk the
// tree itself: a stale listing would hide a file it should scan (#1831).
func TestNoCacheFileWalkIgnoresStaleListing(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil { // repository marker for the cache dir
		t.Fatal(err)
	}
	src := filepath.Join(dir, "src")
	if err := os.Mkdir(src, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(src, name), []byte("package p\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-time.Hour)
	write("A.kt")
	if err := os.Chtimes(src, old, old); err != nil {
		t.Fatal(err)
	}
	collect := func(noCache bool) []string {
		t.Helper()
		f := freshScanFlags(t)
		*f.NoCache = noCache
		r := &runner{f: f, paths: []string{dir}, tracker: perf.New(false), trackedFiles: newTrackedFilesIndex(noCache)}
		if code, err := r.collectFiles(); code != 0 || err != nil {
			t.Fatalf("collectFiles: code %d, err %v", code, err)
		}
		return r.files
	}
	if got := collect(false); len(got) != 1 {
		t.Fatalf("seeding walk found %v, want A.kt", got)
	}
	// B.kt appears without the directory's mtime moving, as a write within
	// the filesystem's timestamp granularity can leave it.
	write("B.kt")
	if err := os.Chtimes(src, old, old); err != nil {
		t.Fatal(err)
	}
	if got := collect(false); len(got) != 1 {
		t.Fatalf("a cached walk found %v; the seed no longer reproduces a stale listing", got)
	}
	if got := collect(true); len(got) != 2 {
		t.Fatalf("--no-cache walk found %v, want A.kt and B.kt", got)
	}
}
