package cacheutil

import (
	"os"
	"path/filepath"
	"testing"
)

func writeEntry(t *testing.T, root, name string) string {
	t.Helper()
	path := filepath.Join(root, "ab", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestResetOnBuildChange(t *testing.T) {
	root := filepath.Join(t.TempDir(), "cache")
	if err := resetOnBuildChange(root, "build-a"); err != nil {
		t.Fatal(err)
	}
	entry := writeEntry(t, root, "one")

	// The same build keeps its entries, also after the in-process memo is gone.
	if err := resetOnBuildChange(root, "build-a"); err != nil || !exists(entry) {
		t.Fatalf("same build must keep entries: err=%v exists=%v", err, exists(entry))
	}
	buildResetSeen.Delete(root)
	if err := resetOnBuildChange(root, "build-a"); err != nil || !exists(entry) {
		t.Fatalf("same build, new process must keep entries: err=%v exists=%v", err, exists(entry))
	}

	// Another build empties the directory and records itself.
	if err := resetOnBuildChange(root, "build-b"); err != nil {
		t.Fatal(err)
	}
	if exists(entry) {
		t.Fatal("entry written by build-a survived build-b")
	}
	if got, _ := os.ReadFile(filepath.Join(root, buildIdentityFile)); string(got) != "build-b" {
		t.Fatalf("recorded identity = %q, want build-b", got)
	}
}

func TestResetOnBuildChangeEmptiesDirectoryWithoutRecord(t *testing.T) {
	root := filepath.Join(t.TempDir(), "cache")
	entry := writeEntry(t, root, "legacy")
	if err := resetOnBuildChange(root, "build-a"); err != nil {
		t.Fatal(err)
	}
	if exists(entry) {
		t.Fatal("entries from an unknown build must be removed")
	}
}

func TestBuildScopedVersionDiffersPerBuild(t *testing.T) {
	if BuildScopedVersion("3") == BuildScopedVersion("4") {
		t.Fatal("version change must change the scoped version")
	}
}
