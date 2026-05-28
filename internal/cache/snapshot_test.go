package cache

import (
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// TestEncodeSnapshot_WriteRoundTrip pins that EncodeSnapshot +
// WriteSnapshot persist exactly what the monolithic Save would, so the
// daemon's deferred (snapshot-then-async-write) path stays byte-faithful
// to the synchronous one.
func TestEncodeSnapshot_WriteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, CacheFileName)

	c := &Cache{
		Version:  "1.0.0",
		RuleHash: "abc123",
		Files: map[string]FileEntry{
			"/src/a.kt": {Hash: "h1", ModTime: 1000, Size: 100},
			"/src/b.kt": {Hash: "h2", ModTime: 2000, Size: 200, Columns: testFindingColumns([]scanner.Finding{
				{File: "/src/b.kt", Line: 5, Col: 1, Severity: "warning", RuleSet: "style", Rule: "MaxLen", Message: "too long"},
			})},
		},
	}

	data, shouldWrite, err := c.EncodeSnapshot()
	if err != nil {
		t.Fatalf("EncodeSnapshot: %v", err)
	}
	if !shouldWrite {
		t.Fatal("expected shouldWrite=true for a store-less cache")
	}
	if err := WriteSnapshot(cachePath, data); err != nil {
		t.Fatalf("WriteSnapshot: %v", err)
	}

	loaded := Load(cachePath)
	if loaded.Version != c.Version || loaded.RuleHash != c.RuleHash {
		t.Fatalf("header mismatch: got version=%s ruleHash=%s", loaded.Version, loaded.RuleHash)
	}
	if len(loaded.Files) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(loaded.Files))
	}
	bEntry := loaded.Files["/src/b.kt"]
	if got := bEntry.Columns.Len(); got != 1 {
		t.Fatalf("expected 1 cached row for b.kt, got %d", got)
	}
}

// TestEncodeSnapshot_ImmuneToLaterMutation is the core safety property of
// the async-save split: the byte buffer captured by EncodeSnapshot must
// reflect the cache state at encode time, even if the cache is mutated
// before the (deferred) WriteSnapshot lands. This is what makes it safe
// to write on a background worker while the next analyze keeps mutating
// the resident Cache.
func TestEncodeSnapshot_ImmuneToLaterMutation(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, CacheFileName)

	c := &Cache{
		Version:  "1.0.0",
		RuleHash: "rh1",
		Files: map[string]FileEntry{
			"/src/a.kt": {Hash: "h1", ModTime: 1000, Size: 100},
		},
	}

	// Capture the snapshot, THEN mutate the live cache the way a
	// subsequent analyze would (new entry + header change).
	data, _, err := c.EncodeSnapshot()
	if err != nil {
		t.Fatalf("EncodeSnapshot: %v", err)
	}
	cols := testFindingColumns([]scanner.Finding{{File: "/src/z.kt", Line: 1, Col: 1, Severity: "error", RuleSet: "x", Rule: "Y", Message: "m"}})
	c.UpdateEntryColumns("/src/z.kt", &cols)
	c.SetHeader("9.9.9", "rh2", []string{"/src"})

	// Writing the pre-mutation snapshot must persist the pre-mutation
	// state, not the mutated live cache.
	if err := WriteSnapshot(cachePath, data); err != nil {
		t.Fatalf("WriteSnapshot: %v", err)
	}
	loaded := Load(cachePath)
	if loaded.RuleHash != "rh1" {
		t.Errorf("snapshot leaked later header mutation: ruleHash=%s want rh1", loaded.RuleHash)
	}
	if _, ok := loaded.Files["/src/z.kt"]; ok {
		t.Errorf("snapshot leaked later-added entry /src/z.kt")
	}
	if len(loaded.Files) != 1 {
		t.Errorf("expected 1 entry in snapshot, got %d", len(loaded.Files))
	}
}
