package snapshot

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".krit", "snapshots")

	in := &Blob{
		SchemaVersion: SchemaVersion,
		KritVersion:   "test",
		CommitSHA:     "abcdef0123456789abcdef0123456789abcdef01",
		CapturedAt:    1700000000000,
		RepoRoot:      dir,
		Modules: []Module{
			{Path: ":app", Dir: "app", Dependencies: []ModuleDep{{Path: ":core", Configuration: "implementation"}}},
			{Path: ":core", Dir: "core", Consumers: []string{":app"}},
		},
		Files: []File{
			{Path: "app/Main.kt", Module: ":app", Language: "kotlin", Lines: 12, Bytes: 200},
		},
		Symbols: []Symbol{
			{Name: "main", Kind: "function", FQN: "com.example.MainKt.main", File: "app/Main.kt", Line: 1, Language: "kotlin"},
		},
	}

	path, err := Save(root, in)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if !strings.Contains(path, in.CommitSHA[:2]) || !strings.Contains(path, in.CommitSHA) {
		t.Fatalf("blob path %q does not include sha prefix/sha", path)
	}

	got, err := Load(root, in.CommitSHA)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.CommitSHA != in.CommitSHA || got.SchemaVersion != in.SchemaVersion {
		t.Fatalf("load mismatch: got %+v", got)
	}
	if len(got.Modules) != 2 || got.Modules[0].Path != ":app" {
		t.Fatalf("modules round-trip mismatch: %+v", got.Modules)
	}
	if len(got.Files) != 1 || got.Files[0].Lines != 12 {
		t.Fatalf("files round-trip mismatch: %+v", got.Files)
	}
	if len(got.Symbols) != 1 || got.Symbols[0].FQN != "com.example.MainKt.main" {
		t.Fatalf("symbols round-trip mismatch: %+v", got.Symbols)
	}
}

func TestSaveRejectsEmptySHA(t *testing.T) {
	if _, err := Save(t.TempDir(), &Blob{}); err == nil {
		t.Fatal("expected error for empty CommitSHA")
	}
}

func TestListSortedAndScopedToBlobs(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".krit", "snapshots")

	shas := []string{
		"ffffffffffffffffffffffffffffffffffffffff",
		"00000000000000000000000000000000aaaaaaaa",
		"77777777777777777777777777777777cccccccc",
	}
	for _, s := range shas {
		if _, err := Save(root, &Blob{SchemaVersion: SchemaVersion, CommitSHA: s, CapturedAt: 1}); err != nil {
			t.Fatalf("Save %s: %v", s, err)
		}
	}

	entries, err := List(root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != len(shas) {
		t.Fatalf("expected %d entries, got %d", len(shas), len(entries))
	}
	for i := 1; i < len(entries); i++ {
		if entries[i-1].CommitSHA >= entries[i].CommitSHA {
			t.Fatalf("entries not sorted: %v", entries)
		}
	}
	for _, e := range entries {
		if e.Bytes <= 0 {
			t.Fatalf("expected non-empty blob bytes, got %d for %s", e.Bytes, e.CommitSHA)
		}
	}
}

func TestListMissingDirReturnsNil(t *testing.T) {
	entries, err := List(filepath.Join(t.TempDir(), "no-such-dir"))
	if err != nil {
		t.Fatalf("List on missing dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty list, got %v", entries)
	}
}
