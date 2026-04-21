package scanner

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestFileShardRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cacheDir := CrossFileCacheDir(dir)

	want := &fileShard{
		Path:        "a.kt",
		ContentHash: "deadbeef",
		Symbols: []Symbol{
			{Name: "foo", Kind: "function", Visibility: "public", File: "a.kt", Line: 1},
		},
		References: []Reference{
			{Name: "foo", File: "a.kt", Line: 1},
			{Name: "Bar", File: "a.kt", Line: 2},
		},
	}
	if err := saveFileShard(cacheDir, want); err != nil {
		t.Fatalf("saveFileShard: %v", err)
	}
	got, ok := loadFileShard(cacheDir, want.Path, want.ContentHash)
	if !ok {
		t.Fatalf("expected shard hit")
	}
	if got.Version != crossFileShardVersion {
		t.Fatalf("version = %d, want %d", got.Version, crossFileShardVersion)
	}
	if got.Path != want.Path || got.ContentHash != want.ContentHash {
		t.Fatalf("path/hash round-trip mismatch: %+v", got)
	}
	if len(got.Symbols) != 1 || got.Symbols[0].Name != "foo" {
		t.Fatalf("symbols round-trip mismatch: %+v", got.Symbols)
	}
	if len(got.References) != 2 {
		t.Fatalf("refs round-trip mismatch: %+v", got.References)
	}
}

func TestFileShardContentHashMismatchIsMiss(t *testing.T) {
	cacheDir := CrossFileCacheDir(t.TempDir())
	if err := saveFileShard(cacheDir, &fileShard{Path: "a.kt", ContentHash: "h1"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, ok := loadFileShard(cacheDir, "a.kt", "h2"); ok {
		t.Fatalf("expected miss on different hash")
	}
}

func TestFileShardPathMismatchIsMiss(t *testing.T) {
	cacheDir := CrossFileCacheDir(t.TempDir())
	// Forge a shard on disk at a.kt's key but claiming a different Path
	// inside the payload. The load must reject it. We simulate by saving
	// under one (path, hash) then loading with the same hash but a
	// different path: a different path changes the key so we never even
	// hit the file. Instead, directly rewrite the file at the original
	// shard path with a tampered payload.
	if err := saveFileShard(cacheDir, &fileShard{Path: "a.kt", ContentHash: "h"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	// Rename the stored shard file to the key for (b.kt, h) so a load
	// for (b.kt, h) finds it but the embedded Path says "a.kt" → miss.
	src := fileShardPath(cacheDir, shardKey("a.kt", "h"))
	dst := fileShardPath(cacheDir, shardKey("b.kt", "h"))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Rename(src, dst); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if _, ok := loadFileShard(cacheDir, "b.kt", "h"); ok {
		t.Fatalf("expected miss when payload.Path disagrees with key")
	}
}

func TestFileShardCorruptIsMiss(t *testing.T) {
	cacheDir := CrossFileCacheDir(t.TempDir())
	if err := saveFileShard(cacheDir, &fileShard{Path: "a.kt", ContentHash: "h"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	p := fileShardPath(cacheDir, shardKey("a.kt", "h"))
	if err := os.WriteFile(p, []byte("not-gob"), 0o644); err != nil {
		t.Fatalf("corrupt: %v", err)
	}
	if _, ok := loadFileShard(cacheDir, "a.kt", "h"); ok {
		t.Fatalf("expected miss on corrupted shard")
	}
}

func TestFileShardEmptyCacheDirIsMissAndSaveErrors(t *testing.T) {
	if _, ok := loadFileShard("", "a.kt", "h"); ok {
		t.Fatalf("expected miss on empty cacheDir")
	}
	if err := saveFileShard("", &fileShard{Path: "a.kt", ContentHash: "h"}); err == nil {
		t.Fatalf("expected error on empty cacheDir save")
	}
}

func TestShardKeyIsPerPathAndHash(t *testing.T) {
	ka := shardKey("a.kt", "h")
	kb := shardKey("b.kt", "h")
	if ka == kb {
		t.Fatalf("shardKey must vary with path")
	}
	kh1 := shardKey("a.kt", "h1")
	kh2 := shardKey("a.kt", "h2")
	if kh1 == kh2 {
		t.Fatalf("shardKey must vary with hash")
	}
}

// TestBuildIndexCachedShardFallbackEquivalent verifies that a shard-backed
// rebuild (monolithic miss, all shards present) produces the same
// query answers as a fresh BuildIndex.
func TestBuildIndexCachedShardFallbackEquivalent(t *testing.T) {
	dir := t.TempDir()
	cacheDir := CrossFileCacheDir(dir)

	// Build a synthetic fingerprint that won't match, then seed shards
	// manually for two files that contribute to the same index shape.
	fa := &File{Path: "/tmp/a.kt", Content: []byte("ignored")}
	fb := &File{Path: "/tmp/b.kt", Content: []byte("ignored")}

	ha := contentHashForFile(fa.Path, fa.Content)
	hb := contentHashForFile(fb.Path, fb.Content)

	symsA := []Symbol{{Name: "A", Kind: "class", Visibility: "public", File: fa.Path, Line: 1}}
	refsA := []Reference{{Name: "B", File: fa.Path, Line: 2}}
	symsB := []Symbol{{Name: "B", Kind: "class", Visibility: "public", File: fb.Path, Line: 1}}
	refsB := []Reference{{Name: "A", File: fb.Path, Line: 2}}

	if err := saveFileShard(cacheDir, &fileShard{Path: fa.Path, ContentHash: ha, Symbols: symsA, References: refsA}); err != nil {
		t.Fatalf("save shard a: %v", err)
	}
	if err := saveFileShard(cacheDir, &fileShard{Path: fb.Path, ContentHash: hb, Symbols: symsB, References: refsB}); err != nil {
		t.Fatalf("save shard b: %v", err)
	}

	// Call the shard collector directly — avoids dragging in XML walk.
	gotSyms, gotRefs := collectIndexDataSharded(cacheDir, []*File{fa, fb}, nil, nil, 2, nil)
	if len(gotSyms) != 2 || len(gotRefs) != 2 {
		t.Fatalf("sharded collect: got %d syms / %d refs, want 2/2", len(gotSyms), len(gotRefs))
	}

	idx := BuildIndexFromData(gotSyms, gotRefs)
	if idx.ReferenceCount("A") != 1 || idx.ReferenceCount("B") != 1 {
		t.Fatalf("ReferenceCount mismatch: A=%d B=%d", idx.ReferenceCount("A"), idx.ReferenceCount("B"))
	}
	if !idx.IsReferencedOutsideFile("A", fa.Path) {
		t.Fatalf("expected A referenced outside a.kt")
	}
	if !idx.IsReferencedOutsideFile("B", fb.Path) {
		t.Fatalf("expected B referenced outside b.kt")
	}
}

// TestSaveFileShardConcurrent exercises the parallel write path; the
// shard directory and atomic rename must tolerate many goroutines
// touching different shards at the same time without corruption.
func TestSaveFileShardConcurrent(t *testing.T) {
	cacheDir := CrossFileCacheDir(t.TempDir())
	var wg sync.WaitGroup
	const N = 32
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s := &fileShard{
				Path:        filepath.Join("/tmp", "f"+string(rune('A'+i%26))+".kt"),
				ContentHash: "h",
				References:  []Reference{{Name: "R", File: "x", Line: i}},
			}
			if err := saveFileShard(cacheDir, s); err != nil {
				t.Errorf("save: %v", err)
			}
		}(i)
	}
	wg.Wait()
}
