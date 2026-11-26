package scan

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// gitAvailable reports whether `git` is on the PATH.
func gitAvailable(t *testing.T) bool {
	t.Helper()
	_, err := exec.LookPath("git")
	return err == nil
}

// gitInit runs `git init` in dir with author config so subsequent commits
// don't fail in CI.
func gitInit(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-q"},
		{"-c", "init.defaultBranch=main", "config", "user.email", "test@example.com"},
		{"-c", "init.defaultBranch=main", "config", "user.name", "Test"},
	} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// gitAdd stages the given paths in dir.
func gitAdd(t *testing.T, dir string, paths ...string) {
	t.Helper()
	args := append([]string{"-C", dir, "add", "--"}, paths...)
	cmd := exec.Command("git", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
}

var ktFilters = FilewalkFilters{Extensions: []string{".kt", ".kts", ".java"}}

func sortedFiles(files []string) []string {
	out := append([]string(nil), files...)
	sort.Strings(out)
	return out
}

// writeFile creates parent dirs and writes content to path.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// TestFilewalkCache_ColdThenWarm verifies that a warm run returns the same
// sorted file list as the cold run.
func TestFilewalkCache_ColdThenWarm(t *testing.T) {
	root := t.TempDir()
	cache := t.TempDir()

	writeFile(t, filepath.Join(root, "main.kt"), "")
	writeFile(t, filepath.Join(root, "util.kts"), "")
	writeFile(t, filepath.Join(root, "sub", "Foo.java"), "")
	writeFile(t, filepath.Join(root, "sub", "README.md"), "")

	cold, err := CollectFilesCached([]string{root}, ktFilters, cache)
	if err != nil {
		t.Fatalf("cold walk: %v", err)
	}
	warm, err := CollectFilesCached([]string{root}, ktFilters, cache)
	if err != nil {
		t.Fatalf("warm walk: %v", err)
	}

	coldSorted := sortedFiles(cold)
	warmSorted := sortedFiles(warm)
	if len(coldSorted) != len(warmSorted) {
		t.Fatalf("cold=%d files, warm=%d files", len(coldSorted), len(warmSorted))
	}
	for i := range coldSorted {
		if coldSorted[i] != warmSorted[i] {
			t.Errorf("index %d: cold=%q warm=%q", i, coldSorted[i], warmSorted[i])
		}
	}
}

// TestFilewalkCache_NewFile is the critical correctness test: adding a file
// to a tracked directory must advance that directory's mtime, causing the
// next walk to include the new file.
func TestFilewalkCache_NewFile(t *testing.T) {
	root := t.TempDir()
	cache := t.TempDir()

	writeFile(t, filepath.Join(root, "A.kt"), "")

	first, err := CollectFilesCached([]string{root}, ktFilters, cache)
	if err != nil {
		t.Fatalf("first walk: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("expected 1 file before addition, got %d", len(first))
	}

	writeFile(t, filepath.Join(root, "B.kt"), "")

	second, err := CollectFilesCached([]string{root}, ktFilters, cache)
	if err != nil {
		t.Fatalf("second walk: %v", err)
	}
	if len(second) != 2 {
		t.Fatalf("expected 2 files after addition, got %d: %v", len(second), second)
	}
}

// TestFilewalkCache_DeletedFile verifies that removing a file causes the next
// walk to omit it.
func TestFilewalkCache_DeletedFile(t *testing.T) {
	root := t.TempDir()
	cache := t.TempDir()

	a := filepath.Join(root, "A.kt")
	b := filepath.Join(root, "B.kt")
	writeFile(t, a, "")
	writeFile(t, b, "")

	first, err := CollectFilesCached([]string{root}, ktFilters, cache)
	if err != nil {
		t.Fatalf("first walk: %v", err)
	}
	if len(first) != 2 {
		t.Fatalf("expected 2 files before deletion, got %d", len(first))
	}

	if err := os.Remove(b); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	second, err := CollectFilesCached([]string{root}, ktFilters, cache)
	if err != nil {
		t.Fatalf("second walk: %v", err)
	}
	if len(second) != 1 {
		t.Fatalf("expected 1 file after deletion, got %d: %v", len(second), second)
	}
}

// TestFilewalkCache_NewSubdir verifies that adding a subdirectory (which
// advances the parent dir's mtime) causes the next walk to include files
// in the new subdir.
func TestFilewalkCache_NewSubdir(t *testing.T) {
	root := t.TempDir()
	cache := t.TempDir()

	writeFile(t, filepath.Join(root, "A.kt"), "")

	first, err := CollectFilesCached([]string{root}, ktFilters, cache)
	if err != nil {
		t.Fatalf("first walk: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("expected 1 file before subdir creation, got %d", len(first))
	}

	writeFile(t, filepath.Join(root, "pkg", "B.kt"), "")

	second, err := CollectFilesCached([]string{root}, ktFilters, cache)
	if err != nil {
		t.Fatalf("second walk: %v", err)
	}
	if len(second) != 2 {
		t.Fatalf("expected 2 files after subdir creation, got %d: %v", len(second), second)
	}
}

// TestFilewalkCache_FilterChange verifies that changing the filter
// configuration misses the entire cache and produces a fresh walk.
func TestFilewalkCache_FilterChange(t *testing.T) {
	root := t.TempDir()
	cache := t.TempDir()

	writeFile(t, filepath.Join(root, "A.kt"), "")
	writeFile(t, filepath.Join(root, "B.java"), "")

	ktOnly := FilewalkFilters{Extensions: []string{".kt"}}
	first, err := CollectFilesCached([]string{root}, ktOnly, cache)
	if err != nil {
		t.Fatalf("first walk: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("expected 1 .kt file, got %d: %v", len(first), first)
	}

	both := FilewalkFilters{Extensions: []string{".kt", ".java"}}
	second, err := CollectFilesCached([]string{root}, both, cache)
	if err != nil {
		t.Fatalf("second walk: %v", err)
	}
	if len(second) != 2 {
		t.Fatalf("expected 2 files after filter change, got %d: %v", len(second), second)
	}
}

// TestFilewalkCache_RootChange verifies that changing the roots misses the
// cache and returns results appropriate for the new roots.
func TestFilewalkCache_RootChange(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()
	cache := t.TempDir()

	writeFile(t, filepath.Join(rootA, "A.kt"), "")
	writeFile(t, filepath.Join(rootB, "B.kt"), "")

	first, err := CollectFilesCached([]string{rootA}, ktFilters, cache)
	if err != nil {
		t.Fatalf("first walk: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("expected 1 file from rootA, got %d", len(first))
	}

	second, err := CollectFilesCached([]string{rootB}, ktFilters, cache)
	if err != nil {
		t.Fatalf("second walk: %v", err)
	}
	if len(second) != 1 {
		t.Fatalf("expected 1 file from rootB, got %d", len(second))
	}
	if sortedFiles(second)[0] != filepath.Join(rootB, "B.kt") {
		t.Errorf("unexpected file: %v", second)
	}
}

// TestFilewalkCache_StaleDir verifies that a directory present in the cache
// but absent from the filesystem is silently skipped.
func TestFilewalkCache_StaleDir(t *testing.T) {
	root := t.TempDir()
	cache := t.TempDir()

	sub := filepath.Join(root, "sub")
	writeFile(t, filepath.Join(sub, "A.kt"), "")

	first, err := CollectFilesCached([]string{root}, ktFilters, cache)
	if err != nil {
		t.Fatalf("first walk: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("expected 1 file, got %d", len(first))
	}

	// Remove the subdirectory entirely — now root's mtime changes, so the
	// second walk re-reads the root entry and finds no "sub" child.
	if err := os.RemoveAll(sub); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}

	second, err := CollectFilesCached([]string{root}, ktFilters, cache)
	if err != nil {
		t.Fatalf("second walk: %v", err)
	}
	if len(second) != 0 {
		t.Fatalf("expected 0 files after subdir removal, got %d: %v", len(second), second)
	}
}

// TestFilewalkCache_NoCacheDir verifies that passing an empty cacheDir
// falls back to a full walk on every call without error.
func TestFilewalkCache_NoCacheDir(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "A.kt"), "")

	first, err := CollectFilesCached([]string{root}, ktFilters, "")
	if err != nil {
		t.Fatalf("first walk: %v", err)
	}
	second, err := CollectFilesCached([]string{root}, ktFilters, "")
	if err != nil {
		t.Fatalf("second walk: %v", err)
	}
	if len(first) != 1 || len(second) != 1 {
		t.Errorf("expected 1 file each walk, got %d and %d", len(first), len(second))
	}
}

// TestFilewalkCache_PrunedDirs verifies that DefaultPrunedDir entries
// are never collected. `build/` and similar project-output dirs are
// expected to be pruned via the project's .gitignore matcher rather
// than the hard-coded list — separate coverage lives in the
// fileignore matcher tests.
func TestFilewalkCache_PrunedDirs(t *testing.T) {
	root := t.TempDir()
	cache := t.TempDir()

	writeFile(t, filepath.Join(root, "src", "Main.kt"), "")
	writeFile(t, filepath.Join(root, ".gradle", "Main.kt"), "") // pruned
	writeFile(t, filepath.Join(root, ".claude", "Main.kt"), "") // pruned
	writeFile(t, filepath.Join(root, ".grit", "Main.kt"), "")   // pruned

	files, err := CollectFilesCached([]string{root}, ktFilters, cache)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 file (src only), got %d: %v", len(files), files)
	}
}

// TestFilewalkCache_CorruptCache verifies that a truncated or corrupt cache
// file causes a full re-walk rather than an error.
func TestFilewalkCache_CorruptCache(t *testing.T) {
	root := t.TempDir()
	cache := t.TempDir()

	writeFile(t, filepath.Join(root, "A.kt"), "")

	// Write garbage into the cache index.
	if err := os.WriteFile(filewalkIndexPath(cache), []byte("garbage"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	files, err := CollectFilesCached([]string{root}, ktFilters, cache)
	if err != nil {
		t.Fatalf("walk with corrupt cache: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d: %v", len(files), files)
	}
}

// TestCollectFilesCached_GitFastPath verifies that a git work-tree top
// is enumerated via `git ls-files` rather than the directory-mtime walk.
// We assert the right files come back; coverage of the bypass itself is
// implicit (the cache index file is never created in this path).
func TestCollectFilesCached_GitFastPath(t *testing.T) {
	if !gitAvailable(t) {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "Tracked.kt"), "")
	writeFile(t, filepath.Join(root, "module", "Inner.java"), "")
	writeFile(t, filepath.Join(root, "Untracked.kt"), "")
	writeFile(t, filepath.Join(root, ".gitignore"), "build/\n")
	writeFile(t, filepath.Join(root, "build", "Generated.kt"), "")

	gitInit(t, root)
	gitAdd(t, root, "Tracked.kt", filepath.Join("module", "Inner.java"))

	cache := t.TempDir()
	files, err := CollectFilesCached([]string{root}, ktFilters, cache)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	// Untracked files that .gitignore does not exclude are listed, so a new
	// file is analyzed before it is `git add`-ed (#1059); ignored build
	// outputs are not.
	got := sortedFiles(files)
	want := []string{"Tracked.kt", "Untracked.kt", "Inner.java"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %d: %v", want, len(got), got)
	}
	for _, p := range got {
		if filepath.Base(p) == "Generated.kt" {
			t.Errorf("git fast path returned ignored file %q", p)
		}
	}

	// Cache index should NOT exist — git path bypasses the walk cache.
	if _, err := os.Stat(filewalkIndexPath(cache)); err == nil {
		t.Errorf("git fast path unexpectedly wrote a directory-mtime cache file")
	}
}

func TestCollectFilesCachedGitFastPathPrunesTrackedIdeaTemplates(t *testing.T) {
	if !gitAvailable(t) {
		t.Skip("git not on PATH")
	}
	root := filepath.Join(t.TempDir(), ".claude", "worktrees", "repo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(root, "src", "App.kt")
	template := filepath.Join(root, ".idea", "fileTemplates", "internal", "Kotlin Class.kt")
	writeFile(t, keep, "class App")
	writeFile(t, template, "${NAME}")
	gitInit(t, root)
	gitAdd(t, root, "src/App.kt", ".idea/fileTemplates/internal/Kotlin Class.kt")
	files, err := CollectFilesCached([]string{root}, ktFilters, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != keep {
		t.Fatalf("tracked source files = %v, want only %s", files, keep)
	}
	explicit, err := CollectFilesCached([]string{template}, ktFilters, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(explicit) != 0 {
		t.Fatalf("explicit template source files = %v", explicit)
	}
}

type fakeTrackedFileIndex struct {
	files []string
	ok    bool
	calls int
}

func (f *fakeTrackedFileIndex) Files(root string) ([]string, bool) {
	f.calls++
	return append([]string(nil), f.files...), f.ok
}

func TestCollectFilesCachedWithIndex_UsesProvidedTrackedFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "Tracked.kt"), "")
	writeFile(t, filepath.Join(root, "module", "Inner.java"), "")
	writeFile(t, filepath.Join(root, "Untracked.kt"), "")
	idx := &fakeTrackedFileIndex{
		ok:    true,
		files: []string{"Tracked.kt", filepath.Join("module", "Inner.java")},
	}

	cache := t.TempDir()
	files, err := CollectFilesCachedWithIndex([]string{root}, ktFilters, cache, idx)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if idx.calls != 1 {
		t.Fatalf("Files calls = %d, want 1", idx.calls)
	}
	got := sortedFiles(files)
	if len(got) != 2 {
		t.Fatalf("expected 2 tracked files, got %d: %v", len(got), got)
	}
	for _, p := range got {
		if filepath.Base(p) == "Untracked.kt" {
			t.Errorf("tracked index path returned untracked file %q", p)
		}
	}
	if _, err := os.Stat(filewalkIndexPath(cache)); err == nil {
		t.Errorf("tracked index path unexpectedly wrote a directory-mtime cache file")
	}
}

// TestCollectFilesCached_NonGitFallsBackToCachedWalk verifies that a
// non-git root still uses the directory-mtime cache path.
func TestCollectFilesCached_NonGitFallsBackToCachedWalk(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "A.kt"), "")
	writeFile(t, filepath.Join(root, "module", "B.java"), "")

	cache := t.TempDir()
	files, err := CollectFilesCached([]string{root}, ktFilters, cache)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d: %v", len(files), files)
	}
	if _, err := os.Stat(filewalkIndexPath(cache)); err != nil {
		t.Errorf("non-git fallback should have populated the walk cache: %v", err)
	}
}

// TestFilewalkFilters_Hash verifies that different filter configs produce
// different hashes and the same config produces the same hash.
func TestFilewalkFilters_Hash(t *testing.T) {
	a := FilewalkFilters{Extensions: []string{".kt"}}.Hash()
	b := FilewalkFilters{Extensions: []string{".kt"}}.Hash()
	c := FilewalkFilters{Extensions: []string{".java"}}.Hash()

	if a != b {
		t.Errorf("same filter should produce same hash: %q vs %q", a, b)
	}
	if a == c {
		t.Errorf("different filter should produce different hash")
	}
}

func TestSaveFilewalkCacheLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 2; i++ {
		if err := saveFilewalkCache(dir, &filewalkPayload{}); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("want only the cache file, got %v", entries)
	}
	if _, ok := loadFilewalkCache(dir); !ok {
		t.Fatal("saved cache did not load")
	}
}

// TestFilewalkCache_RacyDirNotCached guards the racily-clean case: a directory
// changed within the filesystem's timestamp granularity of being listed keeps
// its mtime, so caching its listing would hide the change from the next walk.
// Recently modified directories must be listed again; older ones are cached.
func TestFilewalkCache_RacyDirNotCached(t *testing.T) {
	root := t.TempDir()
	cache := t.TempDir()
	old := filepath.Join(root, "old")
	writeFile(t, filepath.Join(old, "A.kt"), "")
	writeFile(t, filepath.Join(root, "B.kt"), "")
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}

	if _, err := CollectFilesCached([]string{root}, ktFilters, cache); err != nil {
		t.Fatalf("walk: %v", err)
	}
	p, ok := loadFilewalkCache(cache)
	if !ok {
		t.Fatal("no filewalk cache written")
	}
	if _, cached := p.Dirs[old]; !cached {
		t.Errorf("directory last modified an hour ago was not cached")
	}
	if _, cached := p.Dirs[root]; cached {
		t.Errorf("directory modified just now was cached")
	}
}

// Regression for #1549: the built-in test-data exclusion matched the
// absolute path, so a project under an ancestor directory named testdata
// scanned nothing, and an explicitly named file under testdata/ was dropped.
func TestCollectFilesCached_TestDataMatchedBelowRootOnly(t *testing.T) {
	setup := func(t *testing.T) (root, app, inner string) {
		root = filepath.Join(t.TempDir(), "testdata", "proj")
		app = filepath.Join(root, "src", "main", "kotlin", "App.kt")
		inner = filepath.Join(root, "src", "test", "testdata", "Stub.kt")
		writeFile(t, app, "")
		writeFile(t, inner, "")
		return root, app, inner
	}
	check := func(t *testing.T, files []string, want string) {
		t.Helper()
		if len(files) != 1 || files[0] != want {
			t.Fatalf("collected %v, want only %q", files, want)
		}
	}

	t.Run("walk", func(t *testing.T) {
		root, app, _ := setup(t)
		cache := t.TempDir()
		for _, run := range []string{"cold", "warm"} {
			files, err := CollectFilesCached([]string{root}, ktFilters, cache)
			if err != nil {
				t.Fatalf("%s walk: %v", run, err)
			}
			check(t, files, app)
		}
	})

	t.Run("git", func(t *testing.T) {
		if !gitAvailable(t) {
			t.Skip("git not on PATH")
		}
		root, app, _ := setup(t)
		gitInit(t, root)
		files, err := CollectFilesCached([]string{root}, ktFilters, "")
		if err != nil {
			t.Fatal(err)
		}
		check(t, files, app)
	})

	// Markers may span the scan root (#1699 review): `krit src` from a
	// module directory still skips test resources and test data.
	t.Run("roots inside a module", func(t *testing.T) {
		module := filepath.Join(t.TempDir(), "app")
		app := filepath.Join(module, "src", "main", "kotlin", "App.kt")
		writeFile(t, app, "")
		writeFile(t, filepath.Join(module, "src", "test", "resources", "Fixture.kt"), "")
		writeFile(t, filepath.Join(module, "src", "commonTest", "resources", "F2.kt"), "")
		writeFile(t, filepath.Join(module, "src", "test", "data", "D.kt"), "")
		for _, root := range []string{module, filepath.Join(module, "src")} {
			files, err := CollectFilesCached([]string{root}, ktFilters, "")
			if err != nil {
				t.Fatal(err)
			}
			check(t, files, app)
		}
		files, err := CollectFilesCached([]string{filepath.Join(module, "src", "test")}, ktFilters, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(files) != 0 {
			t.Fatalf("rooted at src/test, expected test resources and data skipped, got %v", files)
		}
	})

	t.Run("explicit file", func(t *testing.T) {
		_, _, inner := setup(t)
		files, err := CollectFilesCached([]string{inner}, ktFilters, "")
		if err != nil {
			t.Fatal(err)
		}
		check(t, files, inner)

		excluded := FilewalkFilters{Extensions: ktFilters.Extensions, Excludes: []string{"Stub.kt"}}
		files, err = CollectFilesCached([]string{inner}, excluded, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(files) != 0 {
			t.Fatalf("user exclude should still drop an explicit file, got %v", files)
		}
	})
}
