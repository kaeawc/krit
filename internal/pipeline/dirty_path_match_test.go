package pipeline

import (
	"path/filepath"
	"testing"
)

// TestDirtyPathsAllInManifest_AbsoluteDirtyVsRelativeManifest is the
// regression guard for the warm+ABI 40s tax observed on kotlin-corpus
// post-#590. The watcher Touches with absolute fsnotify paths (its
// registered root is absolute) while the manifest stores paths exactly
// as scanner.CollectKotlinFiles emits them — relative when the CLI
// passes "." (the common `krit .` invocation). Without
// scanRoots-aware relativization, the watcher's absolute dirty path
// misses every manifest key, dirtyPathsAllInManifest returns false,
// and preparseSourcePaths falls back to a 30-40s CollectKotlinFiles
// walk every warm+ABI cycle.
//
// With scanRoots threaded through, the fast path fires whenever the
// dirty entries resolve to any manifest key under a reasonable path
// reading.
func TestDirtyPathsAllInManifest_AbsoluteDirtyVsRelativeManifest(t *testing.T) {
	repoRoot, err := filepath.Abs(t.TempDir())
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	// Manifest mirrors CollectKotlinFiles output when args.Paths=["."].
	manifest := map[string]string{
		"libraries/stdlib/samples/test/iterators.kt": "hash1",
		"other/file.kt": "hash2",
	}
	dirty := []string{
		filepath.Join(repoRoot, "libraries/stdlib/samples/test/iterators.kt"),
	}

	// Without scanRoots, the lookup misses and forces the slow walk.
	if dirtyPathsAllInManifest(dirty, manifest, nil) {
		t.Fatal("dirtyPathsAllInManifest must miss without scanRoots — the test fixture is wrong if this passes")
	}
	// With scanRoots, the absolute dirty path is relativized to the
	// manifest's "libraries/.../iterators.kt" key and matches.
	if !dirtyPathsAllInManifest(dirty, manifest, []string{repoRoot}) {
		t.Errorf("dirtyPathsAllInManifest(abs-dirty, rel-manifest, scanRoots) = false; want true")
	}
}

// TestDirtyPathsAllInManifest_RelativeDirtyVsAbsoluteManifest covers
// the symmetric case: CLI passes an absolute scan path so the
// manifest's keys are absolute, but the dirty set somehow contains a
// relative form (e.g. test harness, future watcher refactor). The
// join-under-root branch catches it.
func TestDirtyPathsAllInManifest_RelativeDirtyVsAbsoluteManifest(t *testing.T) {
	repoRoot, err := filepath.Abs(t.TempDir())
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	abs := filepath.Join(repoRoot, "src/Foo.kt")
	manifest := map[string]string{abs: "hash1"}
	dirty := []string{"src/Foo.kt"}

	if !dirtyPathsAllInManifest(dirty, manifest, []string{repoRoot}) {
		t.Errorf("relative dirty + absolute manifest under same root must match via scanRoots join")
	}
}

// TestDirtyPathsAllInManifest_DirtyOutsideManifest pins the failure
// case: a dirty path that genuinely isn't in the manifest (e.g., a
// newly-added .kt file) must still force the slow walk so the new
// path is picked up.
func TestDirtyPathsAllInManifest_DirtyOutsideManifest(t *testing.T) {
	repoRoot, err := filepath.Abs(t.TempDir())
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	manifest := map[string]string{"src/Existing.kt": "hash1"}
	dirty := []string{filepath.Join(repoRoot, "src/Brandnew.kt")}

	if dirtyPathsAllInManifest(dirty, manifest, []string{repoRoot}) {
		t.Errorf("dirty path not in manifest must fall through to full walk")
	}
}

// TestDirtyPathsAllInManifest_MultipleScanRootsTryEach covers the
// multi-root invocation (`krit src1 src2`): a dirty entry under either
// root resolves correctly.
func TestDirtyPathsAllInManifest_MultipleScanRootsTryEach(t *testing.T) {
	a, err := filepath.Abs(t.TempDir())
	if err != nil {
		t.Fatalf("abs a: %v", err)
	}
	b, err := filepath.Abs(t.TempDir())
	if err != nil {
		t.Fatalf("abs b: %v", err)
	}
	manifest := map[string]string{
		"Foo.kt":          "h1",
		"deep/dir/Bar.kt": "h2",
	}
	dirty := []string{
		filepath.Join(b, "deep/dir/Bar.kt"),
	}

	if !dirtyPathsAllInManifest(dirty, manifest, []string{a, b}) {
		t.Errorf("dirty under second scan root must still match")
	}
}

// TestDirtyPathsAllInManifest_LegacyDirectMatchStillWorks pins the
// pre-fix path: when both dirty and manifest are in the same form
// (both relative or both absolute), the direct lookup wins without
// touching scanRoots. Preserves the fast common case.
func TestDirtyPathsAllInManifest_LegacyDirectMatchStillWorks(t *testing.T) {
	manifest := map[string]string{"src/Foo.kt": "h1"}

	// Direct hit with empty scanRoots — the no-roots branch must still
	// succeed when the path conventions match.
	if !dirtyPathsAllInManifest([]string{"src/Foo.kt"}, manifest, nil) {
		t.Errorf("direct-match (rel-rel) must succeed without scanRoots")
	}
}

// TestDirtyPathsAllInManifest_NilDirtyMeansNoOpinion preserves the
// existing contract: nil dirty (host has no opinion / watcher
// uninitialised) forces the walk.
func TestDirtyPathsAllInManifest_NilDirtyMeansNoOpinion(t *testing.T) {
	if dirtyPathsAllInManifest(nil, map[string]string{"x.kt": "h"}, []string{"/repo"}) {
		t.Errorf("nil dirty must not be treated as clean")
	}
}

// TestDirtyPathsAllInManifest_EmptyDirtyOnEmptyManifestMatches is the
// degenerate "no files" edge — both empty, no work to do, fast path.
func TestDirtyPathsAllInManifest_EmptyDirtyOnEmptyManifestMatches(t *testing.T) {
	if !dirtyPathsAllInManifest([]string{}, map[string]string{}, nil) {
		t.Errorf("empty dirty + empty manifest is the clean case")
	}
}
