package cache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// TestCheckFilesIncremental_NonDirtyPathsHitWithoutStat verifies the
// "skip stat for clean paths" behavior. fileA has a cache entry but is
// not in the dirty set, so the cache returns it as a hit even though
// we mutate the file on disk to differ from the cached metadata. A
// CheckFiles call would stat it and miss; CheckFilesIncremental must
// trust the watcher and return the cached row.
func TestCheckFilesIncremental_NonDirtyPathsHitWithoutStat(t *testing.T) {
	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.kt")
	if err := os.WriteFile(fileA, []byte("fun a() {}"), 0644); err != nil {
		t.Fatal(err)
	}
	absA, _ := filepath.Abs(fileA)

	c := &Cache{
		RuleHash: "h",
		Files: map[string]FileEntry{
			absA: {
				Hash:    "stale", // intentionally wrong hash
				ModTime: 1,       // intentionally wrong mtime
				Size:    1,       // intentionally wrong size
				Columns: testFindingColumns([]scanner.Finding{
					{File: fileA, Rule: "X", Message: "cached"},
				}),
			},
		},
	}

	// Dirty set is empty: the watcher has not reported any change.
	res := c.CheckFilesIncremental([]string{fileA}, nil, "h")
	if res.TotalCached != 1 {
		t.Fatalf("expected 1 cached hit, got %d", res.TotalCached)
	}
	if !res.CachedPaths[fileA] {
		t.Fatal("fileA should be a hit (not in dirty set)")
	}
	if res.CachedColumns.Len() != 1 {
		t.Fatalf("expected 1 cached finding, got %d", res.CachedColumns.Len())
	}
}

// TestCheckFilesIncremental_DirtyPathRechecksAndMisses verifies that a
// path in the dirty set goes through NeedsReanalysis. The cache entry
// has stale metadata so the recheck misses and the path is reported
// as a miss.
func TestCheckFilesIncremental_DirtyPathRechecksAndMisses(t *testing.T) {
	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.kt")
	if err := os.WriteFile(fileA, []byte("fun a() {}"), 0644); err != nil {
		t.Fatal(err)
	}
	absA, _ := filepath.Abs(fileA)

	c := &Cache{
		RuleHash: "h",
		Files: map[string]FileEntry{
			absA: {Hash: "stale", ModTime: 1, Size: 1},
		},
	}

	res := c.CheckFilesIncremental([]string{fileA}, []string{fileA}, "h")
	if res.TotalCached != 0 {
		t.Fatalf("expected dirty path to miss, got %d hits", res.TotalCached)
	}
	if res.CachedPaths[fileA] {
		t.Fatal("dirty fileA should NOT be a cache hit")
	}
}

// TestCheckFilesIncremental_DirtyPathStillHitsWhenMetadataMatches
// verifies the "dirty but unchanged" case: the watcher noticed an
// event (maybe a chmod or touch) but the content hasn't changed —
// NeedsReanalysis returns false and the entry is still served.
func TestCheckFilesIncremental_DirtyPathStillHitsWhenMetadataMatches(t *testing.T) {
	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.kt")
	if err := os.WriteFile(fileA, []byte("fun a() {}"), 0644); err != nil {
		t.Fatal(err)
	}
	absA, _ := filepath.Abs(fileA)
	info, _ := os.Stat(fileA)

	c := &Cache{
		RuleHash: "h",
		Files: map[string]FileEntry{
			absA: {
				Hash:    ComputeFileHash(fileA),
				ModTime: info.ModTime().UnixMilli(),
				Size:    info.Size(),
			},
		},
	}

	res := c.CheckFilesIncremental([]string{fileA}, []string{fileA}, "h")
	if res.TotalCached != 1 {
		t.Fatalf("expected dirty-but-unchanged path to hit, got %d", res.TotalCached)
	}
}

// TestCheckFilesIncremental_RuleHashMismatchInvalidates verifies the
// rule-hash-drift safety check still applies. A drift forces all
// paths to misses regardless of dirty-set state.
func TestCheckFilesIncremental_RuleHashMismatchInvalidates(t *testing.T) {
	c := &Cache{
		RuleHash: "old",
		Files:    map[string]FileEntry{"/x.kt": {Hash: "h"}},
	}
	res := c.CheckFilesIncremental([]string{"/x.kt"}, nil, "new")
	if res.TotalCached != 0 {
		t.Fatalf("rule-hash drift should miss; got %d hits", res.TotalCached)
	}
}

// TestMutatedSinceFlush tracks UpdateEntryColumns calls between
// MarkFlushed boundaries. The daemon's periodic flush goroutine uses
// MutatedSinceFlush to short-circuit Save when nothing changed.
func TestMutatedSinceFlush(t *testing.T) {
	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.kt")
	if err := os.WriteFile(fileA, []byte("fun a() {}"), 0644); err != nil {
		t.Fatal(err)
	}

	c := &Cache{Files: map[string]FileEntry{}}
	if c.MutatedSinceFlush() {
		t.Fatal("fresh cache should report no mutation")
	}
	cols := testFindingColumns(nil)
	c.UpdateEntryColumns(fileA, &cols)
	if !c.MutatedSinceFlush() {
		t.Fatal("UpdateEntryColumns should record a mutation")
	}
	c.MarkFlushed()
	if c.MutatedSinceFlush() {
		t.Fatal("MarkFlushed should reset the counter")
	}
}
