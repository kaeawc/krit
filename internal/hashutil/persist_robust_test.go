package hashutil

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func writeAged(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("class "+name), 0o644); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(p, past, past); err != nil {
		t.Fatal(err)
	}
	return p
}

func savedMemo(t *testing.T, dir string, files ...string) string {
	t.Helper()
	m := NewMemo()
	for _, f := range files {
		if _, err := m.HashFile(f, nil); err != nil {
			t.Fatal(err)
		}
	}
	store := filepath.Join(dir, "memo.gob")
	if err := m.SavePersisted(store); err != nil {
		t.Fatal(err)
	}
	return store
}

func TestLoadPersistedToleratesCorruption(t *testing.T) {
	dir := t.TempDir()
	good := savedMemo(t, dir, writeAged(t, dir, "a.kt"))
	data, err := os.ReadFile(good)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{
		"empty":     {},
		"garbage":   []byte("not a gob stream at all \x00\xff\xfe"),
		"truncated": data[:len(data)/2],
		"flipped":   append(append([]byte(nil), data[:len(data)/3]...), 0xff, 0xff, 0xff, 0xff),
	}
	for name, body := range cases {
		path := filepath.Join(dir, name+".gob")
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatal(err)
		}
		if added := NewMemo().LoadPersisted(path); added != 0 {
			t.Errorf("%s: corrupt memo added %d entries", name, added)
		}
	}
}

func TestLoadPersistedIgnoresOtherHasherAndVersion(t *testing.T) {
	dir := t.TempDir()
	store := savedMemo(t, dir, writeAged(t, dir, "a.kt"))
	if NewMemo().LoadPersisted(store) != 1 {
		t.Fatal("control load should add the saved entry")
	}
	for name, mutate := range map[string]func(*persistedMemo){
		"hasher":  func(p *persistedMemo) { p.Hasher = "some-other-hash" },
		"version": func(p *persistedMemo) { p.Version = persistedMemoVersion + 1 },
	} {
		f, err := os.Open(store)
		if err != nil {
			t.Fatal(err)
		}
		saved, ok := decodePersisted(f)
		f.Close()
		if !ok {
			t.Fatal("decode control failed")
		}
		mutate(&saved)
		other := filepath.Join(dir, name+".gob")
		if err := writePersisted(other, saved); err != nil {
			t.Fatal(err)
		}
		if added := NewMemo().LoadPersisted(other); added != 0 {
			t.Errorf("%s mismatch still added %d entries", name, added)
		}
	}
}

func TestSavePersistedDropsDeletedFiles(t *testing.T) {
	dir := t.TempDir()
	keep, gone := writeAged(t, dir, "keep.kt"), writeAged(t, dir, "gone.kt")
	m := NewMemo()
	for _, p := range []string{keep, gone} {
		if _, err := m.HashFile(p, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	store := filepath.Join(dir, "memo.gob")
	if err := m.SavePersisted(store); err != nil {
		t.Fatal(err)
	}
	if added := NewMemo().LoadPersisted(store); added != 1 {
		t.Fatalf("loaded %d entries, want only the surviving file", added)
	}
}

func TestPersistedMemoConcurrentRunsKeepAReadableFile(t *testing.T) {
	dir := t.TempDir()
	file := writeAged(t, dir, "a.kt")
	store := filepath.Join(dir, "memo.gob")
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m := NewMemo()
			m.LoadPersisted(store)
			_, _ = m.HashFile(file, nil)
			if err := m.SavePersisted(store); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if NewMemo().LoadPersisted(store) != 1 {
		t.Fatal("memo unreadable after concurrent saves")
	}
	left, _ := filepath.Glob(filepath.Join(dir, "memo.gob.tmp.*"))
	if len(left) != 0 {
		t.Fatalf("temp files left behind: %v", left)
	}
}
