package scanner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCrossFileCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cacheDir := CrossFileCacheDir(dir)

	symbols := []Symbol{
		{Name: "helperFunc", Kind: "function", Visibility: "public", File: "a.kt", Line: 10, StartByte: 3, EndByte: 18},
		{Name: "HelperClass", Kind: "class", Visibility: "internal", File: "b.kt", Line: 2, StartByte: 0, EndByte: 12},
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

	gotSyms, gotRefs, ok := LoadCrossFileCache(cacheDir, fp)
	if !ok {
		t.Fatalf("expected cache hit on matching fingerprint")
	}
	if len(gotSyms) != len(symbols) {
		t.Fatalf("got %d symbols, want %d", len(gotSyms), len(symbols))
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
	fp1, _ := computeCrossFileFingerprint([]*File{a, b}, nil, nil)
	fp2, _ := computeCrossFileFingerprint([]*File{b, a}, nil, nil)
	if fp1 != fp2 {
		t.Fatalf("fingerprint should be order-independent: %q vs %q", fp1, fp2)
	}

	// Content edit must flip the fingerprint.
	b2 := &File{Path: "b.kt", Content: []byte("class B2")}
	fp3, _ := computeCrossFileFingerprint([]*File{a, b2}, nil, nil)
	if fp3 == fp1 {
		t.Fatalf("expected fingerprint to change after content edit")
	}

	// File add must flip the fingerprint.
	c := &File{Path: "c.kt", Content: []byte("class C")}
	fp4, _ := computeCrossFileFingerprint([]*File{a, b, c}, nil, nil)
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
