package scanner

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/kaeawc/krit/internal/hashutil"
)

// Cache writes skip fsync, so after an OS crash or power loss an entry can be
// empty, cut short, zero-filled, or hold unrelated bytes. Every such entry must
// load as a miss: no error surfaced to the scan, no panic, no wrong findings.

// tornVariants returns damaged copies of a valid entry.
func tornVariants(valid []byte) map[string][]byte {
	variants := map[string][]byte{
		"empty":       {},
		"zero-filled": make([]byte, len(valid)),
		"garbage":     bytes.Repeat([]byte("garbage!"), len(valid)/8+1),
	}
	for _, n := range []int{1, 4, len(valid) / 4, len(valid) / 2, len(valid) - 1} {
		if n > 0 && n < len(valid) {
			variants["truncated-"+strconv.Itoa(n)] = append([]byte(nil), valid[:n]...)
		}
	}
	return variants
}

// assertTornEntriesMiss overwrites path with each damaged variant of its
// current content and asserts load reports a miss.
func assertTornEntriesMiss(t *testing.T, path string, load func() bool) {
	t.Helper()
	valid, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read valid entry: %v", err)
	}
	if !load() {
		t.Fatal("valid entry did not load")
	}
	for name, data := range tornVariants(valid) {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatal(err)
			}
			if load() {
				t.Fatalf("damaged entry (%d of %d bytes) loaded as a hit", len(data), len(valid))
			}
		})
	}
}

// withHugeLength rewrites the uint64 payload length that precedes the
// compressed body (header layout: magic u32, version u32, keyLen u16, key,
// crc u32, length u64) so the header stays well formed but claims far more
// bytes than the file holds.
func withHugeLength(t *testing.T, valid []byte) []byte {
	t.Helper()
	keyLen := int(binary.BigEndian.Uint16(valid[8:10]))
	off := 10 + keyLen + 4
	if off+8 > len(valid) {
		t.Fatalf("entry too short for header: %d bytes", len(valid))
	}
	out := append([]byte(nil), valid...)
	binary.BigEndian.PutUint64(out[off:], 1<<62)
	return out
}

func tornTestFindings() FindingColumns {
	return CollectFindings([]Finding{{
		File: "/repo/A.kt", Line: 1, Col: 2, Rule: "RuleA", Severity: "warning", Message: "message",
	}})
}

func TestFindingsBundleStore_TornEntryIsMiss(t *testing.T) {
	root := t.TempDir()
	fp := RunFingerprint{Version: "v1", Rules: "rules", Config: "config", SourceSet: "src"}
	cols := tornTestFindings()
	store := DiskFindingsBundleStore{}
	if err := store.Save(root, fp, &cols); err != nil {
		t.Fatalf("Save: %v", err)
	}
	path := findingsBundleEntryPath(root, FindingsBundleKey(fp))
	valid, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	load := func() bool {
		_, ok := store.Load(root, fp)
		return ok
	}
	assertTornEntriesMiss(t, path, load)

	if err := os.WriteFile(path, withHugeLength(t, valid), 0o644); err != nil {
		t.Fatal(err)
	}
	if load() {
		t.Fatal("entry with an oversized length loaded as a hit")
	}
}

func TestFindingsBundleManifest_TornEntryIsMiss(t *testing.T) {
	repo := t.TempDir()
	key := FindingsBundleManifestKey(repo, []string{repo})
	if err := SaveFindingsBundleManifest(repo, key, FindingsBundleManifest{
		BundleKey:     "bundle",
		ContentHashes: map[string]string{filepath.Join(repo, "A.kt"): "hash"},
	}); err != nil {
		t.Fatalf("SaveFindingsBundleManifest: %v", err)
	}
	assertTornEntriesMiss(t, findingsBundleManifestPath(repo, key), func() bool {
		_, ok := LoadFindingsBundleManifest(repo, key)
		return ok
	})
}

func TestCrossFindings_TornEntryIsMiss(t *testing.T) {
	dir := t.TempDir()
	key := CrossFindingsKey("index-fp", "rules")
	if err := SaveCrossFindings(dir, key, tornTestFindings()); err != nil {
		t.Fatalf("SaveCrossFindings: %v", err)
	}
	path := filepath.Join(dir, crossFindingsCacheFile)
	valid, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	load := func() bool {
		_, ok := LoadCrossFindings(dir, key)
		return ok
	}
	assertTornEntriesMiss(t, path, load)

	if err := os.WriteFile(path, withHugeLength(t, valid), 0o644); err != nil {
		t.Fatal(err)
	}
	if load() {
		t.Fatal("entry with an oversized length loaded as a hit")
	}
}

func TestAndroidFindings_TornEntryIsMiss(t *testing.T) {
	dir := t.TempDir()
	key := AndroidFindingsKey(AndroidFindingsKeyInputs{Kind: AndroidFindingsKindManifest, InputFP: "abc"})
	if err := SaveAndroidFindings(dir, key, sampleAndroidFindings()); err != nil {
		t.Fatalf("SaveAndroidFindings: %v", err)
	}
	path := androidEntryPath(dir, key)
	valid, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	load := func() bool {
		_, ok := LoadAndroidFindings(dir, key)
		return ok
	}
	assertTornEntriesMiss(t, path, load)

	if err := os.WriteFile(path, withHugeLength(t, valid), 0o644); err != nil {
		t.Fatal(err)
	}
	if load() {
		t.Fatal("entry with an oversized length loaded as a hit")
	}
}

func TestCrossFileCache_TornMetaAndPayloadAreMisses(t *testing.T) {
	cacheDir := CrossFileCacheDir(t.TempDir())
	idx := BuildIndexFromData(
		[]Symbol{{Name: "helper", Kind: "function", Visibility: "public", File: "a.kt", Line: 1, Language: LangKotlin}},
		[]Reference{{Name: "helper", File: "b.kt", Line: 2}},
	)
	if err := SaveCrossFileCacheIndex(cacheDir, "fp", CrossFileCacheMeta{}, idx); err != nil {
		t.Fatalf("SaveCrossFileCacheIndex: %v", err)
	}
	paths := crossFileCacheFiles(cacheDir)
	load := func() bool {
		_, ok := LoadCrossFileCacheIndex(cacheDir, "fp")
		return ok
	}
	t.Run("payload", func(t *testing.T) { assertTornEntriesMiss(t, paths.Symbols, load) })
	if err := SaveCrossFileCacheIndex(cacheDir, "fp", CrossFileCacheMeta{}, idx); err != nil {
		t.Fatalf("SaveCrossFileCacheIndex: %v", err)
	}
	t.Run("meta", func(t *testing.T) { assertTornEntriesMiss(t, paths.Meta, load) })
}

// TestCrossFileCache_StalePayloadIsMiss covers the crash case unique to the
// two-file cross-file cache: the new meta survives but the payload rename
// written just before it does not, leaving an earlier, intact payload. The
// meta's payload checksum must turn that pair into a miss.
func TestCrossFileCache_StalePayloadIsMiss(t *testing.T) {
	cacheDir := CrossFileCacheDir(t.TempDir())
	paths := crossFileCacheFiles(cacheDir)
	oldIdx := BuildIndexFromData(
		[]Symbol{{Name: "old", Kind: "function", Visibility: "public", File: "a.kt", Line: 1, Language: LangKotlin}},
		nil,
	)
	if err := SaveCrossFileCacheIndex(cacheDir, "fp-old", CrossFileCacheMeta{}, oldIdx); err != nil {
		t.Fatalf("save old: %v", err)
	}
	oldPayload, err := os.ReadFile(paths.Symbols)
	if err != nil {
		t.Fatal(err)
	}
	newIdx := BuildIndexFromData(
		[]Symbol{{Name: "new", Kind: "function", Visibility: "public", File: "a.kt", Line: 1, Language: LangKotlin}},
		[]Reference{{Name: "new", File: "b.kt", Line: 3}},
	)
	if err := SaveCrossFileCacheIndex(cacheDir, "fp-new", CrossFileCacheMeta{}, newIdx); err != nil {
		t.Fatalf("save new: %v", err)
	}
	if _, ok := LoadCrossFileCacheIndex(cacheDir, "fp-new"); !ok {
		t.Fatal("fresh pair did not load")
	}
	if err := os.WriteFile(paths.Symbols, oldPayload, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := LoadCrossFileCacheIndex(cacheDir, "fp-new"); ok {
		t.Fatal("new meta with the previous payload loaded as a hit")
	}
	if _, _, ok := LoadCurrentCrossFileCacheIndex(cacheDir); ok {
		t.Fatal("LoadCurrentCrossFileCacheIndex accepted the previous payload")
	}
	if _, _, ok := LoadCrossFileCache(cacheDir, "fp-new"); ok {
		t.Fatal("LoadCrossFileCache accepted the previous payload")
	}
}

// TestCrossFileCacheOverlay_InheritsPayloadChecksum asserts a meta-only
// overlay save keeps describing the payload already on disk.
func TestCrossFileCacheOverlay_InheritsPayloadChecksum(t *testing.T) {
	cacheDir := CrossFileCacheDir(t.TempDir())
	idx := BuildIndexFromData(
		[]Symbol{{Name: "helper", Kind: "function", Visibility: "public", File: "a.kt", Line: 1, Language: LangKotlin}},
		nil,
	)
	entries := []fingerprintEntry{{Path: "a.kt", Hash: "h1"}}
	if err := SaveCrossFileCacheIndex(cacheDir, "fp", CrossFileCacheMeta{Entries: entries}, idx); err != nil {
		t.Fatalf("SaveCrossFileCacheIndex: %v", err)
	}
	base, ok := LoadCurrentCrossFileCacheMeta(cacheDir)
	if !ok || base.PayloadCRC == 0 {
		t.Fatalf("base meta missing payload checksum: ok=%v crc=%d", ok, base.PayloadCRC)
	}
	// A caller-built meta without the checksum (for example from a daemon's
	// resident copy) still inherits it from disk.
	if err := SaveCrossFileCacheOverlay(cacheDir, "fp2", CrossFileCacheMeta{Entries: entries, PayloadEntries: entries}, idx); err != nil {
		t.Fatalf("SaveCrossFileCacheOverlay: %v", err)
	}
	got, ok := LoadCurrentCrossFileCacheMeta(cacheDir)
	if !ok || got.PayloadCRC != base.PayloadCRC {
		t.Fatalf("overlay payload checksum = %d (ok=%v), want %d", got.PayloadCRC, ok, base.PayloadCRC)
	}
	if _, ok := LoadCrossFileCacheIndex(cacheDir, "fp2"); !ok {
		t.Fatal("overlay meta did not load with its inherited checksum")
	}
}

func TestParsePack_TornPackIsMiss(t *testing.T) {
	repo := t.TempDir()
	pc, err := NewParseCache(repo)
	if err != nil {
		t.Fatalf("NewParseCache: %v", err)
	}
	src := largeSource()
	if _, err := ParseKotlinFileCached(context.Background(), writeKotlin(t, repo, "Torn.kt", src), pc); err != nil {
		t.Fatalf("seed parse: %v", err)
	}
	_, packPath := packedBlobForHash(t, pc.kotlin, hashutil.HashHex([]byte(src)))
	assertTornEntriesMiss(t, packPath, func() bool {
		pc.kotlin.pack = newParsePackStore(pc.kotlin.dir)
		_, ok := pc.Load("", []byte(src))
		return ok
	})
}
