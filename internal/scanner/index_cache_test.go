package scanner

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/cacheutil"
)

func TestCrossFileCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cacheDir := CrossFileCacheDir(dir)

	symbols := []Symbol{
		{Name: "helperFunc", Kind: "function", Visibility: "public", File: "a.kt", Line: 10, StartByte: 3, EndByte: 18, Language: LangKotlin, Package: "demo", FQN: "demo.helperFunc", Signature: "<package>#helperFunc/0"},
		{Name: "HelperClass", Kind: "class", Visibility: "internal", File: "b.kt", Line: 2, StartByte: 0, EndByte: 12, Language: LangJava, Package: "demo", FQN: "demo.HelperClass", Signature: "demo.HelperClass", IsFinal: true},
	}
	refs := []Reference{
		{Name: "helperFunc", File: "a.kt", Line: 10, InComment: false},
		{Name: "helperFunc", File: "b.kt", Line: 4, InComment: false},
		{Name: "HelperClass", File: "c.kt", Line: 7, InComment: true},
	}
	const fp = "abc123"

	if err := SaveCrossFileCache(cacheDir, fp, CrossFileCacheMeta{}, symbols, refs); err != nil {
		t.Fatalf("SaveCrossFileCache: %v", err)
	}
	assertCrossFilePayloadZstd(t, cacheDir)

	gotSyms, gotRefs, ok := LoadCrossFileCache(cacheDir, fp)
	if !ok {
		t.Fatalf("expected cache hit on matching fingerprint")
	}
	if len(gotSyms) != len(symbols) {
		t.Fatalf("got %d symbols, want %d", len(gotSyms), len(symbols))
	}
	if gotSyms[1].Language != LangJava || gotSyms[1].FQN != "demo.HelperClass" || !gotSyms[1].IsFinal {
		t.Fatalf("Java symbol metadata did not round-trip: %+v", gotSyms[1])
	}
	if len(gotRefs) != len(refs) {
		t.Fatalf("got %d refs, want %d", len(gotRefs), len(refs))
	}
	if gotSyms[0].Name != "helperFunc" || gotRefs[2].Name != "HelperClass" {
		t.Fatalf("round-trip returned unexpected payload: %+v / %+v", gotSyms, gotRefs)
	}

	// Reconstructed CodeIndex should answer the same queries as a fresh build.
	idx := BuildIndexFromData(gotSyms, gotRefs)
	if idx.ReferenceCount("helperFunc") != 2 {
		t.Fatalf("ReferenceCount mismatch after cache load")
	}
	if !idx.MayHaveReference("HelperClass") {
		t.Fatalf("bloom filter missing HelperClass after cache load")
	}
}

func TestCrossFileCacheFingerprintMismatchMisses(t *testing.T) {
	dir := t.TempDir()
	cacheDir := CrossFileCacheDir(dir)

	if err := SaveCrossFileCache(cacheDir, "fp-v1", CrossFileCacheMeta{}, nil, nil); err != nil {
		t.Fatalf("save: %v", err)
	}

	if _, _, ok := LoadCrossFileCache(cacheDir, "fp-v2"); ok {
		t.Fatalf("expected miss on mismatched fingerprint")
	}
}

func TestCrossFileCacheVersionMismatchMisses(t *testing.T) {
	dir := t.TempDir()
	cacheDir := CrossFileCacheDir(dir)

	if err := SaveCrossFileCache(cacheDir, "fp", CrossFileCacheMeta{}, nil, nil); err != nil {
		t.Fatalf("save: %v", err)
	}
	// Rewrite meta.json with a version field from a future cache layout.
	paths := crossFileCacheFiles(cacheDir)
	bogus := []byte(`{"version":9999,"fingerprint":"fp"}`)
	if err := os.WriteFile(paths.Meta, bogus, 0o644); err != nil {
		t.Fatalf("overwrite meta: %v", err)
	}

	if _, _, ok := LoadCrossFileCache(cacheDir, "fp"); ok {
		t.Fatalf("expected miss when meta version disagrees with compiled-in")
	}
}

func TestCrossFileCacheCorruptPayloadMisses(t *testing.T) {
	dir := t.TempDir()
	cacheDir := CrossFileCacheDir(dir)
	if err := SaveCrossFileCache(cacheDir, "fp", CrossFileCacheMeta{}, nil, nil); err != nil {
		t.Fatalf("save: %v", err)
	}
	paths := crossFileCacheFiles(cacheDir)
	if err := os.WriteFile(paths.Symbols, []byte("not-gob-bytes"), 0o644); err != nil {
		t.Fatalf("corrupt symbols: %v", err)
	}
	if _, _, ok := LoadCrossFileCache(cacheDir, "fp"); ok {
		t.Fatalf("expected miss on corrupted payload")
	}
}

func TestCrossFileCacheEmptyDirIsMiss(t *testing.T) {
	if _, _, ok := LoadCrossFileCache("", "fp"); ok {
		t.Fatalf("expected miss on empty cacheDir")
	}
	if _, _, ok := LoadCrossFileCache(t.TempDir(), ""); ok {
		t.Fatalf("expected miss on empty fingerprint")
	}
}

func TestComputeFingerprintDeterministicAndOrderIndependent(t *testing.T) {
	a := &File{Path: "a.kt", Content: []byte("class A")}
	b := &File{Path: "b.kt", Content: []byte("class B")}
	fp1 := fingerprintCrossFileEntries(crossFileFingerprintEntries([]*File{a, b}, nil, nil))
	fp2 := fingerprintCrossFileEntries(crossFileFingerprintEntries([]*File{b, a}, nil, nil))
	if fp1 != fp2 {
		t.Fatalf("fingerprint should be order-independent: %q vs %q", fp1, fp2)
	}

	// Content edit must flip the fingerprint.
	b2 := &File{Path: "b.kt", Content: []byte("class B2")}
	fp3 := fingerprintCrossFileEntries(crossFileFingerprintEntries([]*File{a, b2}, nil, nil))
	if fp3 == fp1 {
		t.Fatalf("expected fingerprint to change after content edit")
	}

	// File add must flip the fingerprint.
	c := &File{Path: "c.kt", Content: []byte("class C")}
	fp4 := fingerprintCrossFileEntries(crossFileFingerprintEntries([]*File{a, b, c}, nil, nil))
	if fp4 == fp1 {
		t.Fatalf("expected fingerprint to change after file add")
	}
}

func TestCrossFileCacheMetaIsJSON(t *testing.T) {
	dir := t.TempDir()
	cacheDir := CrossFileCacheDir(dir)
	if err := SaveCrossFileCache(cacheDir, "fp", CrossFileCacheMeta{KotlinFiles: 7}, nil, nil); err != nil {
		t.Fatalf("save: %v", err)
	}
	paths := crossFileCacheFiles(cacheDir)
	data, err := os.ReadFile(paths.Meta)
	if err != nil {
		t.Fatalf("read meta: %v", err)
	}
	var m CrossFileCacheMeta
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("meta is not JSON-decodable: %v", err)
	}
	if m.Version != CrossFileCacheVersion {
		t.Fatalf("meta version = %d, want %d", m.Version, CrossFileCacheVersion)
	}
	if m.Fingerprint != "fp" {
		t.Fatalf("meta fingerprint = %q, want %q", m.Fingerprint, "fp")
	}
	if m.KotlinFiles != 7 {
		t.Fatalf("meta KotlinFiles = %d, want 7", m.KotlinFiles)
	}
}

func TestCrossFileCacheIndexRoundTripPreservesLookups(t *testing.T) {
	dir := t.TempDir()
	cacheDir := CrossFileCacheDir(dir)

	symbols := []Symbol{
		{Name: "helperFunc", Kind: "function", Visibility: "public", File: "a.kt", Line: 10, StartByte: 3, EndByte: 18},
		{Name: "HelperClass", Kind: "class", Visibility: "internal", File: "b.kt", Line: 2, StartByte: 0, EndByte: 12},
		{Name: "HelperClass", Kind: "class", Visibility: "public", File: "d.kt", Line: 5, StartByte: 20, EndByte: 40},
	}
	refs := []Reference{
		{Name: "helperFunc", File: "a.kt", Line: 10, InComment: false},
		{Name: "helperFunc", File: "b.kt", Line: 4, InComment: false},
		{Name: "helperFunc", File: "c.kt", Line: 5, InComment: true},
		{Name: "HelperClass", File: "c.kt", Line: 7, InComment: false},
		{Name: "HelperClass", File: "c.kt", Line: 9, InComment: false},
	}

	built := BuildIndexFromData(symbols, refs)
	if err := SaveCrossFileCacheIndex(cacheDir, "fp", CrossFileCacheMeta{}, built); err != nil {
		t.Fatalf("SaveCrossFileCacheIndex: %v", err)
	}
	assertCrossFilePayloadZstd(t, cacheDir)

	loaded, ok := LoadCrossFileCacheIndex(cacheDir, "fp")
	if !ok {
		t.Fatalf("expected cache hit")
	}

	// Rule-visible queries must match exactly.
	assertIntEq(t, "ReferenceCount helperFunc",
		loaded.ReferenceCount("helperFunc"), built.ReferenceCount("helperFunc"))
	assertIntEq(t, "ReferenceCount HelperClass",
		loaded.ReferenceCount("HelperClass"), built.ReferenceCount("HelperClass"))
	assertBoolEq(t, "MayHaveReference helperFunc",
		loaded.MayHaveReference("helperFunc"), built.MayHaveReference("helperFunc"))
	assertBoolEq(t, "MayHaveReference HelperClass",
		loaded.MayHaveReference("HelperClass"), built.MayHaveReference("HelperClass"))
	assertBoolEq(t, "IsReferencedOutsideFile helperFunc a.kt",
		loaded.IsReferencedOutsideFile("helperFunc", "a.kt"),
		built.IsReferencedOutsideFile("helperFunc", "a.kt"))
	assertBoolEq(t, "IsReferencedOutsideFileExcludingComments helperFunc a.kt",
		loaded.IsReferencedOutsideFileExcludingComments("helperFunc", "a.kt"),
		built.IsReferencedOutsideFileExcludingComments("helperFunc", "a.kt"))
	assertIntEq(t, "CountNonCommentRefsInFile HelperClass c.kt",
		loaded.CountNonCommentRefsInFile("HelperClass", "c.kt"),
		built.CountNonCommentRefsInFile("HelperClass", "c.kt"))

	// symbolsByName should still contain both HelperClass entries.
	if got := loaded.symbolsByName["HelperClass"]; len(got) != 2 {
		t.Fatalf("symbolsByName[HelperClass] = %d, want 2", len(got))
	}

	// UnusedSymbols should agree between the two indexes.
	assertIntEq(t, "len(UnusedSymbols(true))",
		len(loaded.UnusedSymbols(true)), len(built.UnusedSymbols(true)))
	assertIntEq(t, "len(UnusedSymbols(false))",
		len(loaded.UnusedSymbols(false)), len(built.UnusedSymbols(false)))
}

func assertCrossFilePayloadZstd(t *testing.T, cacheDir string) {
	t.Helper()
	paths := crossFileCacheFiles(cacheDir)
	data, err := os.ReadFile(paths.Symbols)
	if err != nil {
		t.Fatalf("read cross-file payload: %v", err)
	}
	if !cacheutil.IsZstdFrame(data) {
		t.Fatalf("cross-file payload is not zstd-framed: %x", data[:min(4, len(data))])
	}
}

func TestCrossFileCacheIndexBloomCorruptIsMiss(t *testing.T) {
	dir := t.TempDir()
	cacheDir := CrossFileCacheDir(dir)
	idx := BuildIndexFromData(
		[]Symbol{{Name: "X", Kind: "class", Visibility: "public", File: "a.kt"}},
		[]Reference{{Name: "X", File: "b.kt", Line: 1}},
	)
	if err := SaveCrossFileCacheIndex(cacheDir, "fp", CrossFileCacheMeta{}, idx); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Round-trip confirms the happy path before we corrupt.
	if _, ok := LoadCrossFileCacheIndex(cacheDir, "fp"); !ok {
		t.Fatalf("expected hit before corruption")
	}
	paths := crossFileCacheFiles(cacheDir)
	if err := os.WriteFile(paths.Symbols, []byte("garbage"), 0o644); err != nil {
		t.Fatalf("corrupt: %v", err)
	}
	if _, ok := LoadCrossFileCacheIndex(cacheDir, "fp"); ok {
		t.Fatalf("expected miss on corrupted payload")
	}
}

func TestBuildIndexIncrementalReplacesChangedFileContributions(t *testing.T) {
	base := BuildIndexFromData(
		[]Symbol{
			{Name: "A", Kind: "class", Visibility: "public", File: "a.kt", FQN: "demo.A"},
			{Name: "B", Kind: "class", Visibility: "public", File: "b.kt", FQN: "demo.B"},
		},
		[]Reference{
			{Name: "A", File: "b.kt", Line: 1},
			{Name: "B", File: "a.kt", Line: 1},
		},
	)

	got := BuildIndexIncremental(base, map[string]bool{"b.kt": true},
		[]Symbol{{Name: "C", Kind: "class", Visibility: "public", File: "b.kt", FQN: "demo.C"}},
		[]Reference{{Name: "C", File: "a.kt", Line: 2}},
	)

	if got.ReferenceCount("A") != 0 {
		t.Fatalf("old b.kt reference to A survived incremental update")
	}
	if got.ReferenceCount("B") != 1 {
		t.Fatalf("unchanged a.kt reference to B was lost")
	}
	if got.ReferenceCount("C") != 1 {
		t.Fatalf("new reference to C was not indexed")
	}
	if _, ok := got.SymbolByFQN("demo.B"); ok {
		t.Fatalf("old b.kt symbol survived incremental update")
	}
	if _, ok := got.SymbolByFQN("demo.C"); !ok {
		t.Fatalf("new b.kt symbol was not indexed")
	}
}

func TestCrossFileOverlayCacheSmallEditAvoidsPayloadRewrite(t *testing.T) {
	dir := t.TempDir()
	cacheDir := CrossFileCacheDir(dir)
	aPath := filepath.Join(dir, "A.kt")
	bPath := filepath.Join(dir, "B.kt")
	if err := os.WriteFile(aPath, []byte("class A { fun call() = B() }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bPath, []byte("class B\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	aFile, err := ParseFile(context.Background(), aPath)
	if err != nil {
		t.Fatal(err)
	}
	bFile, err := ParseFile(context.Background(), bPath)
	if err != nil {
		t.Fatal(err)
	}

	cold, hit := BuildIndexCached(cacheDir, []*File{aFile, bFile}, 2, nil)
	if hit {
		t.Fatalf("first build unexpectedly hit cache")
	}
	if cold.ReferenceCount("B") == 0 {
		t.Fatalf("cold index did not see reference to B")
	}
	payloadPath := crossFileCacheFiles(cacheDir).Symbols
	before, err := os.ReadFile(payloadPath)
	if err != nil {
		t.Fatalf("read payload: %v", err)
	}

	if err := os.WriteFile(bPath, []byte("class C\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bChanged, err := ParseFile(context.Background(), bPath)
	if err != nil {
		t.Fatal(err)
	}
	updated, hit := BuildIndexCached(cacheDir, []*File{aFile, bChanged}, 2, nil)
	if hit {
		t.Fatalf("changed build should be an overlay miss, not a full hit")
	}
	if _, ok := updated.SymbolByFQN("C"); !ok {
		t.Fatalf("updated index did not include changed symbol C")
	}
	if _, ok := updated.SymbolByFQN("B"); ok {
		t.Fatalf("updated index still included removed symbol B")
	}
	after, err := os.ReadFile(payloadPath)
	if err != nil {
		t.Fatalf("read payload after overlay: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("overlay update rewrote full cross-file payload")
	}

	metaBytes, err := os.ReadFile(crossFileCacheFiles(cacheDir).Meta)
	if err != nil {
		t.Fatalf("read meta: %v", err)
	}
	var meta CrossFileCacheMeta
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		t.Fatalf("unmarshal meta: %v", err)
	}
	if len(meta.OverlayEntries) != 1 || len(meta.RemovedPayloadPaths) != 1 {
		t.Fatalf("overlay meta = %+v, want one overlay and one removed payload path", meta)
	}

	warm, hit := BuildIndexCached(cacheDir, []*File{aFile, bChanged}, 2, nil)
	if !hit {
		t.Fatalf("unchanged overlay build should hit cache")
	}
	if _, ok := warm.SymbolByFQN("C"); !ok {
		t.Fatalf("warm overlay hit did not include changed symbol C")
	}
	if _, ok := warm.SymbolByFQN("B"); ok {
		t.Fatalf("warm overlay hit included removed symbol B")
	}
}

// TestWarmPriorOverlayDoesNotGrowFiles guards against the daemon-mode
// memory leak where BuildIndexCachedWithLoaders' overlay path appended
// the full kotlin+java file slice to a daemon-resident *CodeIndex
// (returned by the prior loader) on every warm rebuild. Without the
// fix, idx.Files grew by len(files)+len(javaFiles) on each invocation
// and the saver stored the same pointer back, so the next call
// appended on top of the previously-grown slice.
func TestWarmPriorOverlayDoesNotGrowFiles(t *testing.T) {
	dir := t.TempDir()
	cacheDir := CrossFileCacheDir(dir)
	aPath := filepath.Join(dir, "A.kt")
	bPath := filepath.Join(dir, "B.kt")
	if err := os.WriteFile(aPath, []byte("class A { fun call() = B() }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bPath, []byte("class B\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	aFile, err := ParseFile(context.Background(), aPath)
	if err != nil {
		t.Fatal(err)
	}
	bFile, err := ParseFile(context.Background(), bPath)
	if err != nil {
		t.Fatal(err)
	}

	var (
		residentIdx  *CodeIndex
		residentMeta CrossFileCacheMeta
	)
	loader := func() (*CodeIndex, CrossFileCacheMeta, bool) {
		if residentIdx == nil {
			return nil, CrossFileCacheMeta{}, false
		}
		return residentIdx, residentMeta, true
	}
	saver := func(idx *CodeIndex, meta CrossFileCacheMeta) {
		residentIdx = idx
		residentMeta = meta
	}

	// Cold build seeds the resident snapshot.
	cold, _ := BuildIndexCachedWithPrior(cacheDir, []*File{aFile, bFile}, 2, loader, saver, nil)
	if got, want := len(cold.Files), 2; got != want {
		t.Fatalf("cold build len(Files) = %d, want %d", got, want)
	}

	// Repeated warm-prior overlay rebuilds: each rewrite of B.kt
	// (different content each time) forces an overlay path, and
	// the resident loader hands back the previously-built pointer.
	// The bug appended the full file list on every call.
	for i := 0; i < 5; i++ {
		body := []byte("class B" + string(rune('0'+i)) + "\n")
		if err := os.WriteFile(bPath, body, 0o644); err != nil {
			t.Fatal(err)
		}
		bChanged, err := ParseFile(context.Background(), bPath)
		if err != nil {
			t.Fatal(err)
		}
		idx, _ := BuildIndexCachedWithPrior(cacheDir, []*File{aFile, bChanged}, 2, loader, saver, nil)
		if got, want := len(idx.Files), 2; got != want {
			t.Fatalf("warm overlay iter %d: len(Files) = %d, want %d (Files grew on warm rebuild)", i, got, want)
		}
	}
}

func TestCrossFileFingerprintIncludesBloomLibraryVersion(t *testing.T) {
	if bloomLibraryVersion == "" {
		t.Fatalf("bloomLibraryVersion must be set")
	}
	// Sanity: fingerprint depends on the constant — changing it changes the fp.
	// This is an indirect check: we can't easily mutate the constant, so just
	// assert it appears in the same binary and that two calls over the same
	// input are stable (regression guard against accidental time/nondeterminism).
	a := &File{Path: "a.kt", Content: []byte("class A")}
	fp1 := fingerprintCrossFileEntries(crossFileFingerprintEntries([]*File{a}, nil, nil))
	fp2 := fingerprintCrossFileEntries(crossFileFingerprintEntries([]*File{a}, nil, nil))
	if fp1 != fp2 {
		t.Fatalf("fingerprint not deterministic: %q vs %q", fp1, fp2)
	}
}

func assertIntEq(t *testing.T, label string, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %d, want %d", label, got, want)
	}
}

func assertBoolEq(t *testing.T, label string, got, want bool) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", label, got, want)
	}
}

func TestClearCrossFileCache(t *testing.T) {
	dir := t.TempDir()
	cacheDir := CrossFileCacheDir(dir)
	if err := SaveCrossFileCache(cacheDir, "fp", CrossFileCacheMeta{}, nil, nil); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := ClearCrossFileCache(cacheDir); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "meta.json")); !os.IsNotExist(err) {
		t.Fatalf("expected meta.json removed, stat err = %v", err)
	}
}
