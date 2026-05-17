package typeinfer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestIndexFilesParallelCachedWithTrackerReusesUnchangedFiles(t *testing.T) {
	dir := t.TempDir()
	cacheDir := TypeIndexCacheDir(dir)
	aPath := filepath.Join(dir, "A.kt")
	if err := os.WriteFile(aPath, []byte("package demo\nclass A { val name: String = \"a\" }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseFile(context.Background(), aPath)
	if err != nil {
		t.Fatal(err)
	}

	first := NewResolver()
	hits, misses := first.IndexFilesParallelCachedWithTracker([]*scanner.File{file}, 1, cacheDir, nil)
	if hits != 0 || misses != 1 {
		t.Fatalf("first cache summary = hits %d misses %d, want 0/1", hits, misses)
	}
	if got := first.ResolveImport("String", file); got != "kotlin.String" {
		t.Fatalf("first resolver import String = %q", got)
	}

	second := NewResolver()
	hits, misses = second.IndexFilesParallelCachedWithTracker([]*scanner.File{file}, 1, cacheDir, nil)
	if hits != 1 || misses != 0 {
		t.Fatalf("second cache summary = hits %d misses %d, want 1/0", hits, misses)
	}
	if got := second.ResolveImport("String", file); got != "kotlin.String" {
		t.Fatalf("cached resolver import String = %q", got)
	}
}

func TestIndexFilesParallelCachedInvalidatesChangedFileOnly(t *testing.T) {
	dir := t.TempDir()
	cacheDir := TypeIndexCacheDir(dir)
	aPath := filepath.Join(dir, "A.kt")
	bPath := filepath.Join(dir, "B.kt")
	if err := os.WriteFile(aPath, []byte("class A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bPath, []byte("class B\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	aFile, err := scanner.ParseFile(context.Background(), aPath)
	if err != nil {
		t.Fatal(err)
	}
	bFile, err := scanner.ParseFile(context.Background(), bPath)
	if err != nil {
		t.Fatal(err)
	}

	initial := NewResolver()
	initial.IndexFilesParallelCachedWithTracker([]*scanner.File{aFile, bFile}, 2, cacheDir, nil)

	if err := os.WriteFile(bPath, []byte("class C\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bChanged, err := scanner.ParseFile(context.Background(), bPath)
	if err != nil {
		t.Fatal(err)
	}
	updated := NewResolver()
	hits, misses := updated.IndexFilesParallelCachedWithTracker([]*scanner.File{aFile, bChanged}, 2, cacheDir, nil)
	if hits != 1 || misses != 1 {
		t.Fatalf("changed cache summary = hits %d misses %d, want 1/1", hits, misses)
	}
	if updated.ClassHierarchy("A") == nil {
		t.Fatalf("unchanged file contribution was not reused")
	}
	if updated.ClassHierarchy("B") != nil {
		t.Fatalf("changed file retained stale class B")
	}
	if updated.ClassHierarchy("C") == nil {
		t.Fatalf("changed file did not index class C")
	}
}
