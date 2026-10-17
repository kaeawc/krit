package scan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/perf"
)

// A pure-Kotlin project must leave allJavaPaths non-nil (empty) so later
// phases do not walk the tree again looking for Java files.
func TestCollectFiles_NoJavaLeavesEmptyNonNilJavaPaths(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "A.kt"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &runner{paths: []string{dir}, tracker: perf.New(false)}
	if code, err := r.collectFiles(); err != nil || code != 0 {
		t.Fatalf("collectFiles: code=%d err=%v", code, err)
	}
	if len(r.files) != 1 {
		t.Fatalf("kotlin files=%d, want 1", len(r.files))
	}
	if r.allJavaPaths == nil || len(r.allJavaPaths) != 0 {
		t.Fatalf("allJavaPaths=%#v, want empty non-nil", r.allJavaPaths)
	}
}
