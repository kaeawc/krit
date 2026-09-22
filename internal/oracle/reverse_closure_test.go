package oracle

import (
	"reflect"
	"sort"
	"testing"
)

// TestExpandStaleOraclePaths_PullsInReverseDependents verifies that a changed
// file drags every file whose stored dependency closure contains it into the
// stale set, so the daemon re-analyzes dependents rather than merging their
// stale facts.
func TestExpandStaleOraclePaths_PullsInReverseDependents(t *testing.T) {
	dir := t.TempDir()
	cacheDir, err := CacheDir(dir)
	if err != nil {
		t.Fatalf("CacheDir: %v", err)
	}
	a := writeTempFile(t, dir, "A.kt", "package demo\nfun a() = \"value\"\n")
	b := writeTempFile(t, dir, "B.kt", "package demo\nfun b() = a()\n")
	if _, err := WriteFreshEntries(cacheDir,
		&Data{Version: 1, Files: map[string]*File{a: {}, b: {}}},
		&CacheDepsFile{Version: 1, Files: map[string]*CacheDepsEntry{
			a: {DepPaths: nil},
			b: {DepPaths: []string{a}},
		}},
	); err != nil {
		t.Fatalf("WriteFreshEntries: %v", err)
	}

	got := ExpandStaleOraclePaths(nil, cacheDir, []string{a, b}, []string{a})
	if want := []string{a, b}; !reflect.DeepEqual(sortedCopy(got), want) {
		t.Fatalf("A changed must pull in dependent B: got %v, want %v", got, want)
	}
}

// TestExpandStaleOraclePaths_SeedsUnreadableEntry pins the review fix: a file
// whose cache entry cannot be loaded contributes no reverse edge, so a change
// to something it depends on could not otherwise reach it. It must be seeded
// into the work set directly so its stale facts are recomputed rather than
// silently merged forward.
func TestExpandStaleOraclePaths_SeedsUnreadableEntry(t *testing.T) {
	dir := t.TempDir()
	cacheDir, err := CacheDir(dir)
	if err != nil {
		t.Fatalf("CacheDir: %v", err)
	}
	a := writeTempFile(t, dir, "A.kt", "package demo\nfun a() = \"value\"\n")
	// B exists on disk and depends on A, but has no cache entry (never
	// written / evicted / corrupted), so its reverse edge to A is unknown.
	b := writeTempFile(t, dir, "B.kt", "package demo\nfun b() = a()\n")
	if _, err := WriteFreshEntries(cacheDir,
		&Data{Version: 1, Files: map[string]*File{a: {}}},
		&CacheDepsFile{Version: 1, Files: map[string]*CacheDepsEntry{a: {DepPaths: nil}}},
	); err != nil {
		t.Fatalf("WriteFreshEntries: %v", err)
	}

	got := ExpandStaleOraclePaths(nil, cacheDir, []string{a, b}, []string{a})
	if want := []string{a, b}; !reflect.DeepEqual(sortedCopy(got), want) {
		t.Fatalf("unreadable-entry file B must be seeded: got %v, want %v", got, want)
	}
}

// TestExpandStaleOraclePaths_TwoFileCycle verifies the BFS terminates and
// returns both members when two files depend on each other.
func TestExpandStaleOraclePaths_TwoFileCycle(t *testing.T) {
	dir := t.TempDir()
	cacheDir, err := CacheDir(dir)
	if err != nil {
		t.Fatalf("CacheDir: %v", err)
	}
	a := writeTempFile(t, dir, "A.kt", "package demo\nfun a() = b()\n")
	b := writeTempFile(t, dir, "B.kt", "package demo\nfun b() = a()\n")
	if _, err := WriteFreshEntries(cacheDir,
		&Data{Version: 1, Files: map[string]*File{a: {}, b: {}}},
		&CacheDepsFile{Version: 1, Files: map[string]*CacheDepsEntry{
			a: {DepPaths: []string{b}},
			b: {DepPaths: []string{a}},
		}},
	); err != nil {
		t.Fatalf("WriteFreshEntries: %v", err)
	}

	got := ExpandStaleOraclePaths(nil, cacheDir, []string{a, b}, []string{a})
	if want := []string{a, b}; !reflect.DeepEqual(sortedCopy(got), want) {
		t.Fatalf("mutually dependent files: got %v, want %v", got, want)
	}
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}
