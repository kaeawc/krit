package snapshot

import (
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
)

func TestSaveManifestPopulatesIndex(t *testing.T) {
	root := t.TempDir()
	m := &Manifest{
		SchemaVersion: ManifestSchemaVersion,
		CommitSHA:     "abc123def456",
		CapturedAt:    100,
		KritVersion:   "test",
		Files:         3,
		Symbols:       7,
		Modules:       1,
	}
	if _, err := SaveManifest(root, m); err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}
	idx, err := LoadIndex(root)
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}
	if idx == nil || len(idx.Entries) != 1 {
		t.Fatalf("expected 1 entry in index; got %+v", idx)
	}
	if idx.Entries[0].CommitSHA != m.CommitSHA {
		t.Errorf("entry sha = %q; want %q", idx.Entries[0].CommitSHA, m.CommitSHA)
	}
	if idx.Entries[0].Files != 3 {
		t.Errorf("entry counts not preserved: %+v", idx.Entries[0])
	}
}

func TestSaveManifestUpsertReplacesByCommitSHA(t *testing.T) {
	root := t.TempDir()
	first := &Manifest{
		SchemaVersion: ManifestSchemaVersion,
		CommitSHA:     "aaaa1111",
		Files:         1,
	}
	if _, err := SaveManifest(root, first); err != nil {
		t.Fatalf("first save: %v", err)
	}
	updated := *first
	updated.Files = 99
	if _, err := SaveManifest(root, &updated); err != nil {
		t.Fatalf("second save: %v", err)
	}

	idx, err := LoadIndex(root)
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}
	if len(idx.Entries) != 1 {
		t.Fatalf("upsert should replace by sha; got %d entries", len(idx.Entries))
	}
	if idx.Entries[0].Files != 99 {
		t.Errorf("expected updated count 99; got %d", idx.Entries[0].Files)
	}
}

func TestLoadManifestsPrefersIndex(t *testing.T) {
	root := t.TempDir()
	for _, sha := range []string{"bb22", "aa11", "cc33"} {
		if _, err := SaveManifest(root, &Manifest{
			SchemaVersion: ManifestSchemaVersion,
			CommitSHA:     sha,
		}); err != nil {
			t.Fatalf("SaveManifest %s: %v", sha, err)
		}
	}
	got, err := LoadManifests(root)
	if err != nil {
		t.Fatalf("LoadManifests: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 entries; got %d", len(got))
	}
	want := []string{"aa11", "bb22", "cc33"}
	have := make([]string, len(got))
	for i, m := range got {
		have[i] = m.CommitSHA
	}
	sort.Strings(have)
	for i := range want {
		if have[i] != want[i] {
			t.Errorf("index entries[%d] = %q; want %q", i, have[i], want[i])
		}
	}
}

func TestLoadManifestsFallsBackWhenIndexMissing(t *testing.T) {
	root := t.TempDir()
	// Save a manifest the legacy way, then nuke the index file so the
	// fallback path has to fire.
	if _, err := SaveManifest(root, &Manifest{
		SchemaVersion: ManifestSchemaVersion,
		CommitSHA:     "abcd1234",
	}); err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}
	if err := os.Remove(filepath.Join(root, indexFileName)); err != nil {
		t.Fatalf("nuke index: %v", err)
	}
	// Need a blob alongside the manifest for List() to enumerate it.
	// Cheaper to just point List at the existing manifest dir: the
	// fallback also needs that. Skip List dependency by writing a
	// stub blob.
	dir, err := shaDir(root, "abcd1234")
	if err != nil {
		t.Fatalf("shaDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, blobFileName), []byte("stub"), 0o644); err != nil {
		t.Fatalf("write blob: %v", err)
	}

	got, err := LoadManifests(root)
	if err != nil {
		t.Fatalf("LoadManifests: %v", err)
	}
	if len(got) != 1 || got[0].CommitSHA != "abcd1234" {
		t.Fatalf("fallback path failed; got %+v", got)
	}
}

func TestUpsertIndexConcurrentSavesAllSurvive(t *testing.T) {
	root := t.TempDir()
	const workers = 8
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sha := []byte("aaaa0000")
			sha[7] = byte('0' + i)
			_, err := SaveManifest(root, &Manifest{
				SchemaVersion: ManifestSchemaVersion,
				CommitSHA:     string(sha),
			})
			if err != nil {
				t.Errorf("save %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()
	idx, err := LoadIndex(root)
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}
	if len(idx.Entries) != workers {
		t.Errorf("expected %d entries after concurrent upserts; got %d", workers, len(idx.Entries))
	}
}
