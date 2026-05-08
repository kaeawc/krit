package trackedfiles

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type fakeRunner struct {
	files []string
	ok    bool
	calls int
}

func (f *fakeRunner) List(root string) ([]string, bool) {
	f.calls++
	return append([]string(nil), f.files...), f.ok
}

func TestCachedIndex_MemoizesPerRoot(t *testing.T) {
	runner := &fakeRunner{files: []string{"A.kt", "app/build.gradle.kts"}, ok: true}
	idx := NewCachedIndex(runner)

	first, ok := idx.Files("/repo")
	if !ok {
		t.Fatal("first Files returned ok=false")
	}
	second, ok := idx.Files("/repo")
	if !ok {
		t.Fatal("second Files returned ok=false")
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("second listing = %v, want %v", second, first)
	}
}

func TestCachedIndex_ReturnsDefensiveCopies(t *testing.T) {
	runner := &fakeRunner{files: []string{"A.kt"}, ok: true}
	idx := NewCachedIndex(runner)

	files, ok := idx.Files("/repo")
	if !ok {
		t.Fatal("Files returned ok=false")
	}
	files[0] = "mutated"

	again, ok := idx.Files("/repo")
	if !ok {
		t.Fatal("Files returned ok=false")
	}
	if got := again[0]; got != "A.kt" {
		t.Fatalf("cached file = %q, want A.kt", got)
	}
}

func TestCachedIndex_MemoizesMiss(t *testing.T) {
	runner := &fakeRunner{ok: false}
	idx := NewCachedIndex(runner)

	if _, ok := idx.Files("/repo"); ok {
		t.Fatal("first Files returned ok=true")
	}
	if _, ok := idx.Files("/repo"); ok {
		t.Fatal("second Files returned ok=true")
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
}

type fakeMetadataProvider struct {
	snapshot Snapshot
	ok       bool
	calls    int
}

func (f *fakeMetadataProvider) Snapshot(root string) (Snapshot, bool) {
	f.calls++
	return f.snapshot, f.ok
}

func TestPersistentRunner_HitsDiskCache(t *testing.T) {
	root := t.TempDir()
	snapshot := Snapshot{
		Root:         root,
		GitDir:       filepath.Join(root, ".git"),
		IndexPath:    filepath.Join(root, ".git", "index"),
		IndexSize:    12,
		IndexModTime: 34,
		Head:         "ref: refs/heads/main",
	}
	store := DiskStore{}
	if err := store.Save(root, snapshot, []string{"A.kt", "app/build.gradle.kts"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	runner := &fakeRunner{files: []string{"fresh.kt"}, ok: true}
	idx := NewCachedIndex(NewPersistentRunnerWithStore(runner, &fakeMetadataProvider{snapshot: snapshot, ok: true}, store))

	files, ok := idx.Files(root)
	if !ok {
		t.Fatal("Files returned ok=false")
	}
	if runner.calls != 0 {
		t.Fatalf("runner calls = %d, want 0", runner.calls)
	}
	want := []string{"A.kt", "app/build.gradle.kts"}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("files = %v, want %v", files, want)
	}
}

func TestPersistentRunner_StaleSnapshotMissesAndSaves(t *testing.T) {
	root := t.TempDir()
	oldSnapshot := Snapshot{Root: root, GitDir: filepath.Join(root, ".git"), IndexPath: filepath.Join(root, ".git", "index"), IndexSize: 1, IndexModTime: 2, Head: "old"}
	newSnapshot := oldSnapshot
	newSnapshot.IndexModTime = 3
	store := DiskStore{}
	if err := store.Save(root, oldSnapshot, []string{"old.kt"}); err != nil {
		t.Fatalf("Save old: %v", err)
	}
	runner := &fakeRunner{files: []string{"new.kt"}, ok: true}
	idx := NewCachedIndex(NewPersistentRunnerWithStore(runner, &fakeMetadataProvider{snapshot: newSnapshot, ok: true}, store))

	files, ok := idx.Files(root)
	if !ok {
		t.Fatal("Files returned ok=false")
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if !reflect.DeepEqual(files, []string{"new.kt"}) {
		t.Fatalf("files = %v, want [new.kt]", files)
	}
	if cached, ok := store.Load(root, newSnapshot); !ok || !reflect.DeepEqual(cached, []string{"new.kt"}) {
		t.Fatalf("saved cache = %v ok=%v, want [new.kt] true", cached, ok)
	}
}

func TestPersistentRunner_CorruptCacheFallsBack(t *testing.T) {
	root := t.TempDir()
	snapshot := Snapshot{Root: root, GitDir: filepath.Join(root, ".git"), IndexPath: filepath.Join(root, ".git", "index"), IndexSize: 1, IndexModTime: 2, Head: "head"}
	if err := os.MkdirAll(filepath.Dir(diskEntryPath(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(diskEntryPath(root), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{files: []string{"fallback.kt"}, ok: true}
	files, ok := NewPersistentRunnerWithStore(runner, &fakeMetadataProvider{snapshot: snapshot, ok: true}, DiskStore{}).List(root)
	if !ok {
		t.Fatal("List returned ok=false")
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if !reflect.DeepEqual(files, []string{"fallback.kt"}) {
		t.Fatalf("files = %v, want [fallback.kt]", files)
	}
}

func TestPersistentRunner_MetadataMissFallsBack(t *testing.T) {
	runner := &fakeRunner{files: []string{"fallback.kt"}, ok: true}
	files, ok := NewPersistentRunnerWithStore(runner, &fakeMetadataProvider{ok: false}, DiskStore{}).List(t.TempDir())
	if !ok {
		t.Fatal("List returned ok=false")
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if !reflect.DeepEqual(files, []string{"fallback.kt"}) {
		t.Fatalf("files = %v, want [fallback.kt]", files)
	}
}

func TestFileMetadataProvider_SupportsGitDirFile(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, "..", "actual-git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: ../actual-git\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "index"), []byte("index"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	snapshot, ok := (FileMetadataProvider{}).Snapshot(root)
	if !ok {
		t.Fatal("Snapshot returned ok=false")
	}
	if snapshot.GitDir != filepath.Clean(gitDir) {
		t.Fatalf("GitDir = %q, want %q", snapshot.GitDir, filepath.Clean(gitDir))
	}
	if snapshot.Head != "ref: refs/heads/main" {
		t.Fatalf("Head = %q", snapshot.Head)
	}
}
