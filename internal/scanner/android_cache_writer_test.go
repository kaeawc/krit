package scanner

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestAndroidCacheWriter_QueuedSavesPersist(t *testing.T) {
	dir := t.TempDir()
	w := NewAndroidCacheWriter(2)
	defer w.Close()

	keys := []string{
		AndroidFindingsKey(AndroidFindingsKeyInputs{Kind: AndroidFindingsKindManifest, InputFP: "a"}),
		AndroidFindingsKey(AndroidFindingsKeyInputs{Kind: AndroidFindingsKindGradle, InputFP: "b"}),
		AndroidFindingsKey(AndroidFindingsKeyInputs{Kind: AndroidFindingsKindResources, InputFP: "c"}),
	}
	cols := sampleAndroidFindings()
	for _, k := range keys {
		w.Save(dir, k, cols)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	for _, k := range keys {
		if _, ok := LoadAndroidFindings(dir, k); !ok {
			t.Fatalf("expected hit for key %q after flush; entry path %s",
				k, androidEntryPath(dir, k))
		}
	}

	stats := w.Stats()
	if stats.Queued+stats.SyncSaves < 3 {
		t.Errorf("expected at least 3 saves recorded, got queued=%d sync=%d", stats.Queued, stats.SyncSaves)
	}
	if stats.Completed != int64(len(keys)) {
		t.Errorf("expected %d completed, got %d", len(keys), stats.Completed)
	}
	if stats.Failed != 0 {
		t.Errorf("expected 0 failed, got %d", stats.Failed)
	}
}

func TestAndroidCacheWriter_NilSaveIsNoop(t *testing.T) {
	dir := t.TempDir()
	var w *AndroidCacheWriter
	w.Save(dir, "deadbeef", sampleAndroidFindings())
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, ok := LoadAndroidFindings(dir, "deadbeef"); ok {
		t.Fatal("nil writer Save unexpectedly persisted")
	}
}

func TestAndroidCacheWriter_EmptyKeyOrDirIgnored(t *testing.T) {
	dir := t.TempDir()
	w := NewAndroidCacheWriter(1)
	defer w.Close()

	w.Save("", "k", sampleAndroidFindings())
	w.Save(dir, "", sampleAndroidFindings())
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	stats := w.Stats()
	if stats.Queued != 0 || stats.SyncSaves != 0 {
		t.Errorf("expected no work recorded, got queued=%d sync=%d", stats.Queued, stats.SyncSaves)
	}
	// Empty inputs must not have created any entries on disk.
	if entries, _ := readDirRecursive(filepath.Join(dir, "entries")); len(entries) != 0 {
		t.Errorf("expected no entries on disk, found %d", len(entries))
	}
}

func TestAndroidCacheWriter_SaturatedQueueFallsBackToSync(t *testing.T) {
	dir := t.TempDir()
	// Single worker, queue=1 → saturate with many concurrent submits so
	// at least some fall back to the synchronous path.
	w := NewAndroidCacheWriter(1)
	defer w.Close()

	const n = 64
	for i := 0; i < n; i++ {
		key := AndroidFindingsKey(AndroidFindingsKeyInputs{
			Kind:    AndroidFindingsKindManifest,
			InputFP: string(rune('a' + i)),
		})
		w.Save(dir, key, sampleAndroidFindings())
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	stats := w.Stats()
	if stats.Completed != int64(n) {
		t.Errorf("expected %d completed, got %d (failed=%d)", n, stats.Completed, stats.Failed)
	}
}

// readDirRecursive returns every regular file path beneath dir. A
// missing dir is treated as zero entries.
func readDirRecursive(dir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		out = append(out, p)
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return out, nil
}
