package oracle

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestForEachParallelVisitsEveryItemOnce(t *testing.T) {
	items := make([]string, 500)
	for i := range items {
		items[i] = string(rune('a'+i%26)) + string(rune('A'+i/26))
	}
	var count atomic.Int64
	seen := make([]atomic.Int32, len(items))
	index := map[string]int{}
	for i, item := range items {
		index[item] = i
	}
	forEachParallel(items, func(item string) {
		count.Add(1)
		seen[index[item]].Add(1)
	})
	if int(count.Load()) != len(items) {
		t.Fatalf("visited %d items, want %d", count.Load(), len(items))
	}
	for i := range seen {
		if seen[i].Load() != 1 {
			t.Fatalf("item %d visited %d times", i, seen[i].Load())
		}
	}
}

func TestForEachParallelHandlesEmptyAndSingle(t *testing.T) {
	forEachParallel(nil, func(string) { t.Fatal("no item expected") })
	var n int
	forEachParallel([]string{"only"}, func(string) { n++ })
	if n != 1 {
		t.Fatalf("single item visited %d times", n)
	}
}

func TestCollectKotlinSourceFilesListsAFileReachedByRelativeAndAbsoluteRootsOnce(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "A.kt"), []byte("class A"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	files, err := collectKotlinSourceFiles([]string{"src", src})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("got %d entries for one file: %v", len(files), files)
	}
}

func TestCollectKotlinSourceFilesKeepsRootOrderAcrossParallelWalks(t *testing.T) {
	dir := t.TempDir()
	var roots, want []string
	for i := range 40 {
		root := filepath.Join(dir, "m", string(rune('a'+i%26))+string(rune('A'+i/26)))
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(root, "F.kt")
		if err := os.WriteFile(file, []byte("class F"), 0o644); err != nil {
			t.Fatal(err)
		}
		roots = append(roots, root)
		want = append(want, file)
	}
	// A repeated root must not repeat its files, and later roots keep their place.
	roots = append(roots, roots[3])
	got, err := collectKotlinSourceFiles(roots)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d files, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("file %d = %s, want %s", i, got[i], want[i])
		}
	}
}

func TestBlobHashesOfMatchesPerFileHashes(t *testing.T) {
	files := map[string]*File{}
	for i := range 200 {
		files[filepath.Join("/r", string(rune('a'+i%26)), string(rune('A'+i/26))+".kt")] = &File{
			Package:      "p" + string(rune('a'+i%26)),
			Declarations: []*Class{{FQN: "p.C" + string(rune('A'+i/26))}},
		}
	}
	got := blobHashesOf(files)
	if len(got) != len(files) {
		t.Fatalf("got %d hashes for %d files", len(got), len(files))
	}
	for path, file := range files {
		if got[path] != BlobHash(file) {
			t.Fatalf("%s: parallel hash differs from BlobHash", path)
		}
	}
	if len(blobHashesOf(nil)) != 0 {
		t.Error("no files should give no hashes")
	}
}
