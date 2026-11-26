package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/testutil"
)

// A same-size rewrite inside one mtime tick keeps the file's stat, so an
// entry recorded for a just-written file must not let the mtime fast path
// vouch for it (#1614).
func TestUpdateEntrySmudgesRacyModTime(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "A.kt")
	if err := os.WriteFile(filePath, []byte("class A"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := &Cache{Files: make(map[string]FileEntry)}
	c.UpdateEntryColumns(filePath, nil)
	abs, _ := filepath.Abs(filePath)
	entry := c.Files[abs]
	if entry.ModTime != racyModTime {
		t.Fatalf("ModTime = %d for a file modified just now, want racyModTime", entry.ModTime)
	}

	testutil.RewriteKeepingStat(t, filePath, "class B")
	if !NeedsReanalysis(filePath, entry) {
		t.Fatal("a same-size rewrite with the recorded mtime was served from the cache")
	}
}

func TestUpdateEntryKeepsOldModTimeForFastPath(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "A.kt")
	if err := os.WriteFile(filePath, []byte("class A"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(filePath, old, old); err != nil {
		t.Fatal(err)
	}
	c := &Cache{Files: make(map[string]FileEntry)}
	c.UpdateEntryColumns(filePath, nil)
	abs, _ := filepath.Abs(filePath)
	entry := c.Files[abs]
	if entry.ModTime != old.UnixMilli() {
		t.Fatalf("ModTime = %d, want the file's mtime %d", entry.ModTime, old.UnixMilli())
	}
	if NeedsReanalysis(filePath, entry) {
		t.Fatal("an old, unchanged file must hit the mtime fast path")
	}
}
