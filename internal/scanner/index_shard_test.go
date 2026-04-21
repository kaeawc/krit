package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/bits-and-blooms/bloom/v3"
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
	// Rename a saved shard under a different key so a load for that key
	// finds it, but the embedded Path disagrees → miss.
	if err := saveFileShard(cacheDir, &fileShard{Path: "a.kt", ContentHash: "h"}); err != nil {
		t.Fatalf("save: %v", err)
	}
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
	gotSyms, gotRefs, gotBloom := collectIndexDataSharded(cacheDir, []*File{fa, fb}, nil, nil, 2, nil)
	if len(gotSyms) != 2 || len(gotRefs) != 2 {
		t.Fatalf("sharded collect: got %d syms / %d refs, want 2/2", len(gotSyms), len(gotRefs))
	}
	if gotBloom == nil {
		t.Fatalf("expected unioned bloom, got nil")
	}
	if !gotBloom.TestString("A") || !gotBloom.TestString("B") {
		t.Fatalf("unioned bloom missing A or B")
	}

	idx := BuildIndexFromDataWithBloom(gotSyms, gotRefs, gotBloom, nil)
	if idx.ReferenceCount("A") != 1 || idx.ReferenceCount("B") != 1 {
		t.Fatalf("ReferenceCount mismatch: A=%d B=%d", idx.ReferenceCount("A"), idx.ReferenceCount("B"))
	}
	if !idx.IsReferencedOutsideFile("A", fa.Path) {
		t.Fatalf("expected A referenced outside a.kt")
	}
	if !idx.IsReferencedOutsideFile("B", fb.Path) {
		t.Fatalf("expected B referenced outside b.kt")
	}
	if !idx.MayHaveReference("A") || !idx.MayHaveReference("B") {
		t.Fatalf("index bloom missing A or B after adopting prebuilt")
	}
}

// TestFileShardBloomRoundTrip checks that a shard persisted with a
// bloom reloads with the same MayContain answers as the original.
func TestFileShardBloomRoundTrip(t *testing.T) {
	cacheDir := CrossFileCacheDir(t.TempDir())

	refs := []Reference{
		{Name: "Alpha", File: "a.kt", Line: 1},
		{Name: "Beta", File: "a.kt", Line: 2},
		{Name: "Gamma", File: "a.kt", Line: 3},
	}
	bf := buildShardBloomFromRefs(refs)
	encoded, err := encodeShardBloom(bf)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if len(encoded) == 0 {
		t.Fatalf("expected non-empty encoded bloom")
	}

	if err := saveFileShard(cacheDir, &fileShard{
		Path:        "a.kt",
		ContentHash: "h",
		References:  refs,
		Bloom:       encoded,
	}); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, ok := loadFileShard(cacheDir, "a.kt", "h")
	if !ok {
		t.Fatalf("expected shard hit")
	}
	loaded, err := decodeShardBloom(got.Bloom)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if loaded == nil {
		t.Fatalf("decoded bloom is nil")
	}

	for _, name := range []string{"Alpha", "Beta", "Gamma"} {
		if !loaded.TestString(name) {
			t.Errorf("decoded bloom missing %q", name)
		}
		if bf.TestString(name) != loaded.TestString(name) {
			t.Errorf("bloom answer diverged for %q", name)
		}
	}
}

// TestEncodeShardBloomNilIsEmpty ensures that an empty shard encodes
// to nothing so empty-reference files don't waste disk.
func TestEncodeShardBloomNilIsEmpty(t *testing.T) {
	data, err := encodeShardBloom(nil)
	if err != nil {
		t.Fatalf("encode nil: %v", err)
	}
	if data != nil {
		t.Fatalf("encode(nil) = %d bytes, want nil", len(data))
	}
	bf, err := decodeShardBloom(nil)
	if err != nil {
		t.Fatalf("decode nil: %v", err)
	}
	if bf != nil {
		t.Fatalf("decode(nil) returned non-nil bloom")
	}
}

// TestShardBloomUnionEqualsDirectAdd checks that unioning per-shard
// blooms produces identical MayContain answers to adding every name
// to a single aggregate bloom directly. This is the correctness
// property the warm-load path relies on.
func TestShardBloomUnionEqualsDirectAdd(t *testing.T) {
	shardRefs := [][]Reference{
		{{Name: "Foo"}, {Name: "Bar"}},
		{{Name: "Baz"}, {Name: "Qux"}},
		{{Name: "Foo"}, {Name: "Quux"}}, // overlap with shard 0
	}

	// Union path: build per-shard, merge.
	union := newShardBloom()
	for _, refs := range shardRefs {
		sb := buildShardBloomFromRefs(refs)
		if sb == nil {
			continue
		}
		if err := union.Merge(sb); err != nil {
			t.Fatalf("merge: %v", err)
		}
	}

	// Direct path: one bloom, AddString everything.
	direct := newShardBloom()
	for _, refs := range shardRefs {
		for _, r := range refs {
			direct.AddString(r.Name)
		}
	}

	for _, name := range []string{"Foo", "Bar", "Baz", "Qux", "Quux", "Missing", "NotThere"} {
		if union.TestString(name) != direct.TestString(name) {
			t.Errorf("union/direct diverged for %q: union=%v direct=%v",
				name, union.TestString(name), direct.TestString(name))
		}
	}
}

// TestCollectIndexDataShardedUnionBloomCoversRefs end-to-ends the
// shard-path bloom: on a shard-cache-hit rebuild, every ref name the
// collector returns must be TestString-positive on the unioned bloom.
func TestCollectIndexDataShardedUnionBloomCoversRefs(t *testing.T) {
	cacheDir := CrossFileCacheDir(t.TempDir())

	fa := &File{Path: "/tmp/a.kt", Content: []byte("ignored")}
	fb := &File{Path: "/tmp/b.kt", Content: []byte("ignored")}
	ha := contentHashForFile(fa.Path, fa.Content)
	hb := contentHashForFile(fb.Path, fb.Content)

	refsA := []Reference{{Name: "Foo", File: fa.Path, Line: 1}, {Name: "Bar", File: fa.Path, Line: 2}}
	refsB := []Reference{{Name: "Baz", File: fb.Path, Line: 1}}

	// Simulate writes from a previous run: shards with per-shard blooms.
	for _, s := range []*fileShard{
		{Path: fa.Path, ContentHash: ha, References: refsA},
		{Path: fb.Path, ContentHash: hb, References: refsB},
	} {
		enc, err := encodeShardBloom(buildShardBloomFromRefs(s.References))
		if err != nil {
			t.Fatalf("encode shard %s: %v", s.Path, err)
		}
		s.Bloom = enc
		if err := saveFileShard(cacheDir, s); err != nil {
			t.Fatalf("save shard %s: %v", s.Path, err)
		}
	}

	_, refs, bf := collectIndexDataSharded(cacheDir, []*File{fa, fb}, nil, nil, 2, nil)
	if bf == nil {
		t.Fatalf("expected non-nil unioned bloom")
	}
	for _, r := range refs {
		if !bf.TestString(r.Name) {
			t.Errorf("unioned bloom missing ref %q", r.Name)
		}
	}
	for _, name := range []string{"Foo", "Bar", "Baz"} {
		if !bf.TestString(name) {
			t.Errorf("unioned bloom missing %q", name)
		}
	}
}

// TestShardBloomVersionMismatchRejected guards against a future
// (m, k) change slipping into a shard without a version bump.
func TestShardBloomVersionMismatchRejected(t *testing.T) {
	// Build a bloom at a non-matching capacity; decodeShardBloom must
	// reject it so stale (m,k) shards don't corrupt the aggregate.
	different, err := encodeShardBloom(func() *bloom.BloomFilter {
		return bloom.NewWithEstimates(shardBloomCapacity+1, shardBloomFPR)
	}())
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if _, err := decodeShardBloom(different); err == nil {
		t.Fatalf("expected decode error on mismatched (m,k)")
	}
}

// TestShardBloomUnionFPRSmallScale sanity-checks that the union bloom
// does not overflow to ~100% FPR at a realistic shard count. Signal
// scale is ~5500 shards with ~500K unique names; this test uses
// smaller numbers that still exercise the same math.
func TestShardBloomUnionFPRSmallScale(t *testing.T) {
	const shards = 500
	const refsPerShard = 200
	const totalPresent = shards * refsPerShard

	union := newShardBloom()
	for s := 0; s < shards; s++ {
		refs := make([]Reference, refsPerShard)
		for i := 0; i < refsPerShard; i++ {
			refs[i] = Reference{Name: fmt.Sprintf("sym-%d-%d", s, i)}
		}
		sb := buildShardBloomFromRefs(refs)
		if err := union.Merge(sb); err != nil {
			t.Fatalf("merge: %v", err)
		}
	}

	// Every inserted name must TestString-positive.
	for s := 0; s < shards; s += 50 {
		for i := 0; i < refsPerShard; i += 25 {
			if !union.TestString(fmt.Sprintf("sym-%d-%d", s, i)) {
				t.Errorf("false negative for sym-%d-%d", s, i)
			}
		}
	}

	// Probe never-inserted names; with 100K items in a 1M-cap bloom,
	// FPR should be comfortably under 1%.
	const probes = 10000
	falsePos := 0
	for i := 0; i < probes; i++ {
		if union.TestString(fmt.Sprintf("nope-%d", i)) {
			falsePos++
		}
	}
	fpr := float64(falsePos) / float64(probes)
	t.Logf("union bloom: %d items stored, observed FPR = %.4f (probes=%d)", totalPresent, fpr, probes)
	if fpr > 0.05 {
		t.Errorf("union FPR = %.4f exceeds 5%% budget", fpr)
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
