package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/perf"
)

// The Analysis API drops files strictly larger than the limit, so a file of
// exactly the limit is still analyzed: leaving two of them unanswered is a
// mismatch, not an expected skip.
func TestWriteSkippedPoisonEntries_FilesAtTheLimitAreUnexplained(t *testing.T) {
	tmp := t.TempDir()
	cacheDir, err := CacheDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, name := range []string{"A.kt", "B.kt"} {
		p := writeTempFile(t, tmp, name, "class X\n")
		if err := os.Truncate(p, maxJVMSourceBytes); err != nil {
			t.Fatal(err)
		}
		files = append(files, p)
	}
	captureWarnings(t)

	writeSkippedPoisonEntries(nil, cacheDir, []string{tmp}, files, &Data{Files: map[string]*File{}}, &CacheDepsFile{}, perf.New(false), false)

	if hits, _ := ClassifyFiles(cacheDir, files); len(hits) != 0 {
		t.Fatalf("files at the limit must not get markers, hits=%d", len(hits))
	}
}

func TestPathUnderRoot(t *testing.T) {
	sep := string(filepath.Separator)
	root := filepath.Join(sep, "a", "root")
	cases := []struct {
		abs, root string
		want      bool
	}{
		{root, root, true},
		{filepath.Join(root, "f.kt"), root, true},
		{filepath.Join(sep, "a", "rootX", "f.kt"), root, false},
		{filepath.Join(sep, "a", "f.kt"), sep, true},
		{sep, sep, true},
	}
	for _, c := range cases {
		if got := pathUnderRoot(c.abs, c.root); got != c.want {
			t.Errorf("pathUnderRoot(%q, %q) = %v, want %v", c.abs, c.root, got, c.want)
		}
	}
}

// A file in a sibling directory whose name merely starts with the root's name
// is outside the source root, so leaving it unanswered is expected.
func TestExpectedlySkipped_SiblingPrefixIsOutsideRoot(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "root")
	for _, d := range []string{root, filepath.Join(tmp, "rootX")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	sibling := writeTempFile(t, tmp, filepath.Join("rootX", "f.kt"), "class F\n")
	inside := writeTempFile(t, tmp, filepath.Join("root", "f.kt"), "class F\n")
	roots := []string{filepath.Clean(root)}
	if !expectedlySkipped(sibling, roots) {
		t.Error("a file under a sibling directory sharing the root's name prefix is outside the root")
	}
	if expectedlySkipped(inside, roots) {
		t.Error("a small file under the root should be unexplained when unanswered")
	}
}
