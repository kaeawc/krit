package snapshot

import (
	"path/filepath"
	"testing"
)

func TestSaveLoadManifestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".krit", "snapshots")

	in := &Manifest{
		SchemaVersion: ManifestSchemaVersion,
		CommitSHA:     "cccccccccccccccccccccccccccccccccccccccc",
		ParentSHAs:    []string{"dddddddddddddddddddddddddddddddddddddddd"},
		CapturedAt:    1700000000000,
		KritVersion:   "v0.1.1",
		BlobSchema:    SchemaVersion,
		MetricsSchema: MetricsSchemaVersion,
		Files:         42,
		Symbols:       128,
		Modules:       3,
	}
	path, err := SaveManifest(root, in)
	if err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}
	if filepath.Ext(path) != ".json" {
		t.Fatalf("expected .json manifest, got %s", path)
	}
	got, err := LoadManifest(root, in.CommitSHA)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if got.CommitSHA != in.CommitSHA || got.KritVersion != in.KritVersion {
		t.Fatalf("manifest round-trip mismatch: %+v", got)
	}
	if len(got.ParentSHAs) != 1 || got.ParentSHAs[0] != in.ParentSHAs[0] {
		t.Fatalf("ParentSHAs round-trip mismatch: %+v", got.ParentSHAs)
	}
	if got.Files != 42 || got.Symbols != 128 || got.Modules != 3 {
		t.Fatalf("count round-trip mismatch: %+v", got)
	}
}

func TestSaveManifestRejectsEmptySHA(t *testing.T) {
	if _, err := SaveManifest(t.TempDir(), &Manifest{}); err == nil {
		t.Fatal("expected error for empty CommitSHA")
	}
}

func TestCaptureManifestPopulatesFromResult(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".krit", "snapshots")

	res := &Result{
		Blob: &Blob{
			SchemaVersion: SchemaVersion,
			CommitSHA:     "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
			CapturedAt:    1700000000000,
			Files:         []File{{Path: "a.kt"}, {Path: "b.kt"}},
			Symbols:       []Symbol{{Name: "x"}},
			Modules:       []Module{{Path: ":app"}},
		},
		Metrics: &Metrics{SchemaVersion: MetricsSchemaVersion, CommitSHA: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"},
	}

	// Pass empty repoRoot to skip the git lookup and avoid a flaky test
	// in environments without git or in non-repo temp dirs.
	if _, err := CaptureManifest(root, res, "", "v0.0.0-test"); err != nil {
		t.Fatalf("CaptureManifest: %v", err)
	}
	got, err := LoadManifest(root, res.Blob.CommitSHA)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if got.Files != 2 || got.Symbols != 1 || got.Modules != 1 {
		t.Fatalf("counts not derived from blob: %+v", got)
	}
	if got.KritVersion != "v0.0.0-test" {
		t.Fatalf("KritVersion: %s", got.KritVersion)
	}
	if got.BlobSchema != SchemaVersion || got.MetricsSchema != MetricsSchemaVersion {
		t.Fatalf("schema versions not stamped: %+v", got)
	}
}

func TestLoadManifestsAcrossSnapshots(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".krit", "snapshots")

	for _, sha := range []string{
		"1111111111111111111111111111111111111111",
		"2222222222222222222222222222222222222222",
	} {
		// Create a graph blob so List() picks up the entry.
		if _, err := Save(root, &Blob{SchemaVersion: SchemaVersion, CommitSHA: sha, CapturedAt: 1}); err != nil {
			t.Fatalf("Save: %v", err)
		}
		if _, err := SaveManifest(root, &Manifest{
			SchemaVersion: ManifestSchemaVersion,
			CommitSHA:     sha,
			CapturedAt:    1,
			BlobSchema:    SchemaVersion,
		}); err != nil {
			t.Fatalf("SaveManifest: %v", err)
		}
	}
	manifests, err := LoadManifests(root)
	if err != nil {
		t.Fatalf("LoadManifests: %v", err)
	}
	if len(manifests) != 2 {
		t.Fatalf("expected 2 manifests, got %d", len(manifests))
	}
}
