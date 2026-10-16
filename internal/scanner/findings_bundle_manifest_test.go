package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFindingsBundleManifestKey_SeparatesCallerSpellings pins that two
// scans of the same directory spelled from different working
// directories get different manifests. The manifest stores file paths
// in the caller's spelling, so sharing one let a repo-root scan of
// playground/kotlin-webservice/ reuse the ../../playground/... paths a
// cmd/krit test run had written, and the daemon analyzed zero files.
func TestFindingsBundleManifestKey_SeparatesCallerSpellings(t *testing.T) {
	repo := t.TempDir()
	project := filepath.Join(repo, "playground", "app")
	nested := filepath.Join(repo, "cmd", "krit")
	for _, dir := range []string{project, nested} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	t.Chdir(nested)
	fromNested := FindingsBundleManifestKey(project, []string{"../../playground/app/"})
	fromNestedAgain := FindingsBundleManifestKey(project, []string{"../../playground/app/"})

	t.Chdir(repo)
	fromRoot := FindingsBundleManifestKey(project, []string{"playground/app/"})
	absolute := FindingsBundleManifestKey(project, []string{project})

	if fromNested != fromNestedAgain {
		t.Fatalf("key is not stable for one spelling: %s vs %s", fromNested, fromNestedAgain)
	}
	if fromNested == fromRoot {
		t.Errorf("relative spellings from different working directories share key %s", fromRoot)
	}
	if fromRoot == absolute {
		t.Errorf("relative and absolute spellings share key %s", absolute)
	}
}

// TestFindingsBundleManifestKey_SeparatesSameSpellingInDifferentDirs
// keeps the absolute component: one relative spelling scanned from two
// different directories names two different projects.
func TestFindingsBundleManifestKey_SeparatesSameSpellingInDifferentDirs(t *testing.T) {
	repo := t.TempDir()
	a := filepath.Join(repo, "a")
	b := filepath.Join(repo, "b")
	for _, dir := range []string{a, b} {
		if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	t.Chdir(a)
	fromA := FindingsBundleManifestKey(repo, []string{"src"})
	t.Chdir(b)
	fromB := FindingsBundleManifestKey(repo, []string{"src"})

	if fromA == fromB {
		t.Errorf("same spelling from different directories shares key %s", fromA)
	}
}
