package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/hashutil"
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

// An edit that lands between the analysis read and the cache write must not
// pair the old findings with the new content, even when the new stat is old
// enough to trust.
func TestUpdateEntryFromContentKeysTheAnalyzedBytes(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "A.kt")
	analyzed := []byte("class A")
	if err := os.WriteFile(filePath, []byte("class B"), 0o644); err != nil {
		t.Fatal(err)
	}
	testutil.Age(t, filePath)
	c := &Cache{RuleHash: "rh", Files: make(map[string]FileEntry)}
	c.UpdateEntryFromContent(filePath, analyzed, nil, "")
	abs, _ := filepath.Abs(filePath)
	entry := c.Files[abs]
	if entry.Hash != hashutil.HashHex(analyzed)[:16] {
		t.Fatalf("entry hash %s is not the analyzed bytes' hash", entry.Hash)
	}
	if !NeedsReanalysis(filePath, entry) {
		t.Fatal("findings for the analyzed bytes were served for the file's newer content")
	}
}
