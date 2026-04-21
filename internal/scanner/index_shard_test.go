package scanner

import (
	"bytes"
	"encoding/gob"
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
	store := newPackStore(cacheDir)

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
	if err := store.SaveShard(want); err != nil {
		t.Fatalf("SaveShard: %v", err)
	}
	got, ok := store.LoadShard(want.Path, want.ContentHash)
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

// TestFileShardFileFieldRehydrated verifies the v5 wire format strips
// Symbol.File / Reference.File before encoding and reconstructs them
// from fileShard.Path on load. The on-disk savings depend on this
// invariant — a regression that re-serialised File would undo Step 2
// of issue #351 silently.
func TestFileShardFileFieldRehydrated(t *testing.T) {
	cacheDir := CrossFileCacheDir(t.TempDir())
	store := newPackStore(cacheDir)

	const path = "/abs/path/to/Foo.kt"
	want := &fileShard{
		Path:        path,
		ContentHash: "h",
		Symbols: []Symbol{
			{Name: "Foo", Kind: "class", Visibility: "public", File: path, Line: 1},
		},
		References: []Reference{
			{Name: "Bar", File: path, Line: 2},
			{Name: "Baz", File: path, Line: 3, InComment: true},
		},
	}
	if err := store.SaveShard(want); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, ok := store.LoadShard(path, "h")
	if !ok {
		t.Fatalf("expected hit")
	}
	for _, s := range got.Symbols {
		if s.File != path {
			t.Errorf("Symbol.File = %q, want %q (rehydrated from Path)", s.File, path)
		}
	}
	for _, r := range got.References {
		if r.File != path {
			t.Errorf("Reference.File = %q, want %q (rehydrated from Path)", r.File, path)
		}
	}
	if !got.References[1].InComment {
		t.Errorf("InComment lost in round-trip")
	}
}

// TestShardBlobIsZstdCompressed confirms the on-disk blob is zstd-framed
// (magic 0x28B52FFD). A regression that wrote raw gob would silently
// undo Step 1 of issue #351; the magic-byte check makes that loud.
func TestShardBlobIsZstdCompressed(t *testing.T) {
	s := &fileShard{
		Path:        "a.kt",
		ContentHash: "h",
		References:  []Reference{{Name: "Foo", File: "a.kt", Line: 1}},
	}
	blob, err := encodeShardBlob(s)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if len(blob) < 4 {
		t.Fatalf("blob too short: %d bytes", len(blob))
	}
	// zstd magic, little-endian: 28 B5 2F FD.
	if blob[0] != 0x28 || blob[1] != 0xB5 || blob[2] != 0x2F || blob[3] != 0xFD {
		t.Fatalf("blob not zstd-framed: %x", blob[:4])
	}
}

// TestShardBlobShrinksOnRepetition is a smoke test that the
// gob+zstd envelope actually compresses. A reference-heavy shard
// with repeated names should shrink several-fold versus the same
// payload encoded as plain gob.
func TestShardBlobShrinksOnRepetition(t *testing.T) {
	const n = 500
	refs := make([]Reference, n)
	for i := range refs {
		// Two-name vocabulary so the wire form is dominated by
		// near-zero-entropy structure that zstd should crush.
		name := "Alpha"
		if i%2 == 0 {
			name = "Beta"
		}
		refs[i] = Reference{Name: name, File: "/abs/long/path/to/Source.kt", Line: i}
	}
	s := &fileShard{
		Path:        "/abs/long/path/to/Source.kt",
		ContentHash: "h",
		References:  refs,
	}
	compressed, err := encodeShardBlob(s)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var raw bytes.Buffer
	if err := gob.NewEncoder(&raw).Encode(s); err != nil {
		t.Fatalf("gob encode for baseline: %v", err)
	}
	if len(compressed)*3 >= raw.Len() {
		t.Errorf("compression weak: zstd=%d gob=%d (want zstd*3 < gob)", len(compressed), raw.Len())
	}
	t.Logf("compressed=%d bytes, raw-gob=%d bytes (%.1fx)", len(compressed), raw.Len(),
		float64(raw.Len())/float64(len(compressed)))
}

func TestFileShardContentHashMismatchIsMiss(t *testing.T) {
	store := newPackStore(CrossFileCacheDir(t.TempDir()))
	if err := store.SaveShard(&fileShard{Path: "a.kt", ContentHash: "h1"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, ok := store.LoadShard("a.kt", "h2"); ok {
		t.Fatalf("expected miss on different hash")
	}
}

// TestFileShardReopenPersists exercises the cross-process path: save
// via one store, discard it, reopen a new store against the same
// cacheDir, and confirm the shard is still there. This is what
// actually matters - the in-memory round-trip is covered above.
func TestFileShardReopenPersists(t *testing.T) {
	cacheDir := CrossFileCacheDir(t.TempDir())
	a := newPackStore(cacheDir)
	if err := a.SaveShard(&fileShard{Path: "a.kt", ContentHash: "h"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	b := newPackStore(cacheDir)
	if _, ok := b.LoadShard("a.kt", "h"); !ok {
		t.Fatalf("expected hit after reopen")
	}
}

func TestFileShardCorruptIsMiss(t *testing.T) {
	cacheDir := CrossFileCacheDir(t.TempDir())
	store := newPackStore(cacheDir)
	if err := store.SaveShard(&fileShard{Path: "a.kt", ContentHash: "h"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	// Overwrite the pack file with garbage. A new store must treat it as
	// empty (no crash, every Load a miss).
	packPath := store.packFor(shardKey("a.kt", "h")).path
	if err := os.WriteFile(packPath, []byte("not-a-pack"), 0o644); err != nil {
		t.Fatalf("corrupt: %v", err)
	}
	fresh := newPackStore(cacheDir)
	if _, ok := fresh.LoadShard("a.kt", "h"); ok {
		t.Fatalf("expected miss on corrupted pack")
	}
}

// TestFileShardCrcTamperIsMiss verifies that a single-bit flip inside a
// blob region invalidates only that key, not the whole pack.
func TestFileShardCrcTamperIsMiss(t *testing.T) {
	cacheDir := CrossFileCacheDir(t.TempDir())
	store := newPackStore(cacheDir)
	if err := store.SaveShard(&fileShard{Path: "a.kt", ContentHash: "h",
		References: []Reference{{Name: "R", File: "a.kt", Line: 1}}}); err != nil {
		t.Fatalf("save a: %v", err)
	}
	if err := store.SaveShard(&fileShard{Path: "b.kt", ContentHash: "h",
		References: []Reference{{Name: "R", File: "b.kt", Line: 1}}}); err != nil {
		t.Fatalf("save b: %v", err)
	}

	// Flip a byte in a's pack. Pack a and pack b may or may not be the
	// same file; tamper the one that holds key a.
	packPath := store.packFor(shardKey("a.kt", "h")).path
	data, err := os.ReadFile(packPath)
	if err != nil {
		t.Fatalf("read pack: %v", err)
	}
	// Flip the last byte - safely inside the blob region regardless of
	// which blob comes last.
	data[len(data)-1] ^= 0xFF
	if err := os.WriteFile(packPath, data, 0o644); err != nil {
		t.Fatalf("tamper: %v", err)
	}

	// Re-open and check: the flipped key misses, the other (if in a
	// different pack) still hits.
	fresh := newPackStore(cacheDir)
	packForA := fresh.packFor(shardKey("a.kt", "h"))
	packForB := fresh.packFor(shardKey("b.kt", "h"))
	_, hitA := fresh.LoadShard("a.kt", "h")
	_, hitB := fresh.LoadShard("b.kt", "h")
	if packForA == packForB {
		// Both keys live in the same pack. Tampering the last blob's
		// final byte flips whichever was placed last; the other should
		// still hit.
		if hitA && hitB {
			t.Fatalf("expected exactly one miss after tamper, got both hits")
		}
	} else {
		if hitA {
			t.Fatalf("expected a.kt to miss after CRC tamper")
		}
		if !hitB {
			t.Fatalf("expected b.kt to hit (separate pack)")
		}
	}
}

// TestPackStoreSweepsLegacyShardsDir verifies that writing any shard
// removes a pre-v3 shards/ directory. Keeps ~750 MB of dead data from
// accumulating after the upgrade.
func TestPackStoreSweepsLegacyShardsDir(t *testing.T) {
	cacheDir := CrossFileCacheDir(t.TempDir())
	legacy := filepath.Join(cacheDir, legacyShardsSubdir, "ab")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatalf("mkdir legacy: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "dead.gob"), []byte("junk"), 0o644); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	store := newPackStore(cacheDir)
	if err := store.SaveShard(&fileShard{Path: "a.kt", ContentHash: "h"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, legacyShardsSubdir)); !os.IsNotExist(err) {
		t.Fatalf("expected legacy shards dir removed, got err=%v", err)
	}
}

func TestFileShardEmptyCacheDirIsNoop(t *testing.T) {
	var store *packStore = newPackStore("")
	if store != nil {
		t.Fatalf("expected nil store for empty cacheDir")
	}
	if _, ok := store.LoadShard("a.kt", "h"); ok {
		t.Fatalf("expected miss from nil store")
	}
	if err := store.SaveShard(&fileShard{Path: "a.kt", ContentHash: "h"}); err != nil {
		t.Fatalf("expected silent no-op save on nil store, got %v", err)
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
	store := newPackStore(cacheDir)

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

	if err := store.SaveShard(&fileShard{Path: fa.Path, ContentHash: ha, Symbols: symsA, References: refsA}); err != nil {
		t.Fatalf("save shard a: %v", err)
	}
	if err := store.SaveShard(&fileShard{Path: fb.Path, ContentHash: hb, Symbols: symsB, References: refsB}); err != nil {
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
	store := newPackStore(cacheDir)

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

	if err := store.SaveShard(&fileShard{
		Path:        "a.kt",
		ContentHash: "h",
		References:  refs,
		Bloom:       encoded,
	}); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, ok := store.LoadShard("a.kt", "h")
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
	store := newPackStore(cacheDir)

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
		if err := store.SaveShard(s); err != nil {
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
// pack files and atomic rename must tolerate many goroutines touching
// different shards at the same time without corruption.
func TestSaveFileShardConcurrent(t *testing.T) {
	cacheDir := CrossFileCacheDir(t.TempDir())
	store := newPackStore(cacheDir)
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
			if err := store.SaveShard(s); err != nil {
				t.Errorf("save: %v", err)
			}
		}(i)
	}
	wg.Wait()

	// Spot-check a handful: after all writes, every "last writer per
	// path" entry must read back.
	seen := map[string]bool{}
	for i := N - 1; i >= 0; i-- {
		p := filepath.Join("/tmp", "f"+string(rune('A'+i%26))+".kt")
		if seen[p] {
			continue
		}
		seen[p] = true
		if _, ok := store.LoadShard(p, "h"); !ok {
			t.Errorf("expected shard hit for %s after concurrent writes", p)
		}
	}
}
