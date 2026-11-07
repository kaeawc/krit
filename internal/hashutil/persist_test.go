package hashutil

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPersistedMemoServesUnchangedFilesAndDropsRacyOnes(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "old.kt")
	fresh := filepath.Join(dir, "fresh.kt")
	for _, p := range []string{old, fresh} {
		if err := os.WriteFile(p, []byte("class A"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}
	first := NewMemo()
	for _, p := range []string{old, fresh} {
		if _, err := first.HashFile(p, nil); err != nil {
			t.Fatal(err)
		}
	}
	store := filepath.Join(dir, "memo.gob")
	if err := first.SavePersisted(store); err != nil {
		t.Fatal(err)
	}

	second := NewMemo()
	if added := second.LoadPersisted(store); added != 1 {
		t.Fatalf("expected only the old file to be trusted, added %d", added)
	}
	if _, err := second.HashFile(old, nil); err != nil {
		t.Fatal(err)
	}
	if hits, misses := second.Stats(); hits != 1 || misses != 0 {
		t.Errorf("the old file should hit the loaded memo, got %d hits %d misses", hits, misses)
	}
	if _, err := second.HashFile(fresh, nil); err != nil {
		t.Fatal(err)
	}
	if _, misses := second.Stats(); misses != 1 {
		t.Errorf("a file inside the racy window must be hashed again, got %d misses", misses)
	}

	// A file whose size or time changed since the save is hashed again.
	if err := os.WriteFile(old, []byte("class Bigger"), 0o644); err != nil {
		t.Fatal(err)
	}
	third := NewMemo()
	third.LoadPersisted(store)
	hx, err := third.HashFile(old, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, misses := third.Stats(); misses != 1 {
		t.Errorf("a changed file must miss, got %d misses", misses)
	}
	want, _ := NewMemo().HashFile(old, nil)
	if hx != want {
		t.Error("hash of a changed file is stale")
	}
	if NewMemo().LoadPersisted(filepath.Join(dir, "missing.gob")) != 0 {
		t.Error("a missing file should add nothing")
	}
}
