package trackedfiles

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
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

// Regression for #1059: the git-backed index lists untracked files that are
// not ignored, and keeps them out of the persisted tracked listing, whose
// snapshot a new untracked file does not change.
func TestGitIndex_IncludesUntrackedNotIgnored(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	write := func(rel string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q")
	write("Tracked.kt")
	write(".gitignore")
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("build/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")

	store := &memoryStore{}
	index := NewCachedIndex(NewPersistentRunnerWithStore(GitRunner{}, FileMetadataProvider{}, store))
	index.untracked = GitUntrackedFiles
	write("New.kt")
	write("build/Out.kt")

	files, ok := index.Files(root)
	if !ok {
		t.Fatal("expected a git listing")
	}
	sort.Strings(files)
	if want := []string{".gitignore", "New.kt", "Tracked.kt"}; !reflect.DeepEqual(files, want) {
		t.Errorf("files = %v, want %v", files, want)
	}
	if want := []string{".gitignore", "Tracked.kt"}; !reflect.DeepEqual(store.files, want) {
		t.Errorf("persisted = %v, want only tracked %v", store.files, want)
	}
}

type memoryStore struct{ files []string }

func (s *memoryStore) Load(string, Snapshot) ([]string, bool) { return nil, false }
func (s *memoryStore) Save(_ string, _ Snapshot, files []string) error {
	s.files = append([]string(nil), files...)
	sort.Strings(s.files)
	return nil
}

// Entries are written without fsync, so a crash can leave one that still parses and matches the
// snapshot but lists different paths. It must load as a miss.
func TestDiskStore_ParseableDamagedEntryIsMiss(t *testing.T) {
	root := t.TempDir()
	snapshot := Snapshot{Root: root, GitDir: filepath.Join(root, ".git"), IndexPath: filepath.Join(root, ".git", "index"), IndexSize: 1, IndexModTime: 2, Head: "head"}
	store := DiskStore{}
	if err := store.Save(root, snapshot, []string{"A.kt", "B.kt"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, ok := store.Load(root, snapshot); !ok {
		t.Fatal("intact entry did not load")
	}
	path := diskEntryPath(root)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	damaged := bytes.Replace(raw, []byte(`"B.kt"`), []byte(`"C.kt"`), 1)
	if bytes.Equal(damaged, raw) {
		t.Fatal("entry does not contain B.kt")
	}
	if err := os.WriteFile(path, damaged, 0o644); err != nil {
		t.Fatal(err)
	}
	if files, ok := store.Load(root, snapshot); ok {
		t.Fatalf("damaged entry loaded as %v, want miss", files)
	}
}
