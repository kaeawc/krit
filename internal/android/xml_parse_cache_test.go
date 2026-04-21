package android

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/hashutil"
)

// largeManifest returns an AndroidManifest.xml large enough to clear the
// min-file-size floor so the cache actually engages.
func largeManifest() []byte {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	sb.WriteString(`<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example.app">` + "\n")
	sb.WriteString(`  <application android:name=".App">` + "\n")
	// Repeat to push the serialised payload comfortably past the 512-byte floor.
	for i := 0; i < 40; i++ {
		sb.WriteString(`    <activity android:name=".Activity`)
		sb.WriteByte(byte('0' + (i % 10)))
		sb.WriteString(`" android:exported="true" />` + "\n")
	}
	sb.WriteString(`  </application>` + "\n")
	sb.WriteString(`</manifest>` + "\n")
	return []byte(sb.String())
}

func saveActiveXMLParseCache() *XMLParseCache {
	return ActiveXMLParseCache()
}

func restoreActiveXMLParseCache(t *testing.T, prev *XMLParseCache) {
	t.Helper()
	SetActiveXMLParseCache(prev)
}

func TestXMLParseCache_RoundTrip(t *testing.T) {
	repo := t.TempDir()
	pc, err := NewXMLParseCache(repo)
	if err != nil {
		t.Fatalf("NewXMLParseCache: %v", err)
	}
	defer pc.Close()

	prev := saveActiveXMLParseCache()
	defer restoreActiveXMLParseCache(t, prev)
	SetActiveXMLParseCache(pc)

	data := largeManifest()

	// Cold parse populates the cache.
	freshRoot, err := ParseXMLAST(data)
	if err != nil {
		t.Fatalf("fresh ParseXMLAST: %v", err)
	}
	if pc.Stats().Hits != 0 {
		t.Fatalf("expected 0 hits on cold parse, got %d", pc.Stats().Hits)
	}
	if err := pc.Close(); err != nil {
		t.Fatalf("Close after cold parse: %v", err)
	}

	// Warm parse should hit.
	cachedRoot, err := ParseXMLAST(data)
	if err != nil {
		t.Fatalf("warm ParseXMLAST: %v", err)
	}
	if pc.Stats().Hits != 1 {
		t.Fatalf("expected 1 hit after warm parse, got %d", pc.Stats().Hits)
	}

	if !xmlNodesEqual(freshRoot, cachedRoot) {
		t.Fatalf("cached AST differs from fresh AST")
	}
}

// xmlNodesEqual does a structural node-by-node comparison. reflect.DeepEqual
// would also work but this makes failures easier to localise.
func xmlNodesEqual(a, b *XMLNode) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Tag != b.Tag || a.Line != b.Line || a.Col != b.Col ||
		a.StartByte != b.StartByte || a.EndByte != b.EndByte ||
		a.Text != b.Text {
		return false
	}
	if !reflect.DeepEqual(a.Attrs, b.Attrs) {
		return false
	}
	if len(a.Children) != len(b.Children) {
		return false
	}
	for i := range a.Children {
		if !xmlNodesEqual(a.Children[i], b.Children[i]) {
			return false
		}
	}
	return true
}

func TestXMLParseCache_SkipsSmallFiles(t *testing.T) {
	repo := t.TempDir()
	pc, err := NewXMLParseCache(repo)
	if err != nil {
		t.Fatalf("NewXMLParseCache: %v", err)
	}
	defer pc.Close()

	prev := saveActiveXMLParseCache()
	defer restoreActiveXMLParseCache(t, prev)
	SetActiveXMLParseCache(pc)

	small := []byte(`<root><a/></root>`)
	if _, err := ParseXMLAST(small); err != nil {
		t.Fatalf("ParseXMLAST: %v", err)
	}
	if _, err := ParseXMLAST(small); err != nil {
		t.Fatalf("ParseXMLAST: %v", err)
	}

	stats := pc.Stats()
	if stats.Hits != 0 || stats.Misses != 0 {
		t.Fatalf("small file should not touch cache, got %+v", stats)
	}
}

func TestXMLParseCache_SaveAsyncFlushesClonedTree(t *testing.T) {
	repo := t.TempDir()
	pc, err := NewXMLParseCache(repo)
	if err != nil {
		t.Fatalf("NewXMLParseCache: %v", err)
	}

	data := largeManifest()
	root := &XMLNode{
		Tag: "manifest",
		Attrs: []XMLAttribute{
			{Name: "package", Value: "com.example"},
		},
		Children: []*XMLNode{
			{Tag: "application", Attrs: []XMLAttribute{{Name: "android:name", Value: ".App"}}},
		},
	}
	if err := pc.SaveAsync(data, root); err != nil {
		t.Fatalf("SaveAsync: %v", err)
	}
	root.Tag = "mutated"
	root.Children[0].Tag = "mutated-child"
	if err := pc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got, ok := pc.Load(data)
	if !ok {
		t.Fatal("expected flushed async cache hit")
	}
	if got.Tag != "manifest" || got.Children[0].Tag != "application" {
		t.Fatalf("cached tree was not cloned before async write: %#v", got)
	}
}

func TestXMLParseCache_GrammarMismatchForcesMiss(t *testing.T) {
	repo := t.TempDir()
	pc, err := NewXMLParseCache(repo)
	if err != nil {
		t.Fatalf("NewXMLParseCache: %v", err)
	}
	prev := saveActiveXMLParseCache()
	defer restoreActiveXMLParseCache(t, prev)
	SetActiveXMLParseCache(pc)

	data := largeManifest()
	if _, err := ParseXMLAST(data); err != nil {
		t.Fatalf("ParseXMLAST: %v", err)
	}
	if err := pc.Close(); err != nil {
		t.Fatalf("Close after parse: %v", err)
	}

	hash := hashutil.Default().HashContent("", data)
	entryPath := pc.entryPath(hash)
	if _, err := os.Stat(entryPath); err != nil {
		t.Fatalf("expected cache entry at %s: %v", entryPath, err)
	}

	// Simulate a grammar-version mismatch by rewriting the sidecar. A
	// fresh cache open must clear the stale entry before serving it.
	dataOnDisk, err := os.ReadFile(entryPath)
	if err != nil {
		t.Fatalf("read entry: %v", err)
	}
	if !cacheutil.IsZstdFrame(dataOnDisk) {
		t.Fatalf("xml parse cache entry is not zstd-framed: %x", dataOnDisk[:min(4, len(dataOnDisk))])
	}
	if err := os.WriteFile(filepath.Join(pc.Dir(), "grammar-version"), []byte("bogus#xml:0"), 0o644); err != nil {
		t.Fatalf("rewrite grammar-version sidecar: %v", err)
	}

	// Reset counters by installing a fresh cache pointing at the same
	// dir with the real grammar version.
	pc2, err := NewXMLParseCache(repo)
	if err != nil {
		t.Fatalf("NewXMLParseCache: %v", err)
	}
	defer pc2.Close()
	SetActiveXMLParseCache(pc2)

	if _, ok := pc2.Load(data); ok {
		t.Fatal("expected grammar mismatch to miss")
	}
	if pc2.Stats().Misses != 1 {
		t.Fatalf("expected 1 miss, got %+v", pc2.Stats())
	}
	if _, err := os.Stat(entryPath); !os.IsNotExist(err) {
		t.Fatalf("expected stale XML entry removed after sidecar mismatch, stat err=%v", err)
	}
}

func TestXMLParseCache_NilRootPayloadMisses(t *testing.T) {
	repo := t.TempDir()
	pc, err := NewXMLParseCache(repo)
	if err != nil {
		t.Fatalf("NewXMLParseCache: %v", err)
	}
	defer pc.Close()

	data := largeManifest()

	// Force-save a structurally invalid but decodable entry. The load
	// path must refuse it instead of returning (nil, true).
	hash := hashutil.Default().HashContent("", data)
	entry := xmlParseCacheEntry{}
	entryPath := pc.entryPath(hash)
	if err := os.MkdirAll(filepath.Dir(entryPath), 0o755); err != nil {
		t.Fatalf("mkdir shard: %v", err)
	}
	blob, err := cacheutil.EncodeZstdGob(entry)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if err := os.WriteFile(entryPath, blob, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, ok := pc.Load(data); ok {
		t.Fatal("expected nil-root payload to miss")
	}
}

func TestXMLParseCache_ClearViaRegistry(t *testing.T) {
	repo := t.TempDir()
	pc, err := NewXMLParseCache(repo)
	if err != nil {
		t.Fatalf("NewXMLParseCache: %v", err)
	}
	prev := saveActiveXMLParseCache()
	defer restoreActiveXMLParseCache(t, prev)
	SetActiveXMLParseCache(pc)

	data := largeManifest()
	if _, err := ParseXMLAST(data); err != nil {
		t.Fatalf("ParseXMLAST: %v", err)
	}
	if err := pc.Close(); err != nil {
		t.Fatalf("Close after parse: %v", err)
	}

	hash := hashutil.Default().HashContent("", data)
	entryPath := pc.entryPath(hash)
	if _, err := os.Stat(entryPath); err != nil {
		t.Fatalf("expected cache entry at %s: %v", entryPath, err)
	}

	if err := cacheutil.ClearAll(cacheutil.ClearContext{RepoDir: repo}); err != nil {
		t.Fatalf("ClearAll: %v", err)
	}
	if _, err := os.Stat(entryPath); !os.IsNotExist(err) {
		t.Fatalf("expected entry removed after ClearAll, stat err = %v", err)
	}
}

func TestXMLParseCache_NilSafe(t *testing.T) {
	var pc *XMLParseCache
	if got, ok := pc.Load(largeManifest()); ok || got != nil {
		t.Fatal("nil cache must miss")
	}
	if err := pc.Save(largeManifest(), &XMLNode{Tag: "x"}); err != nil {
		t.Fatalf("nil Save should be no-op, got %v", err)
	}
	if err := pc.Close(); err != nil {
		t.Fatalf("nil Close should be no-op, got %v", err)
	}
	if err := pc.Clear(); err != nil {
		t.Fatalf("nil Clear should be no-op, got %v", err)
	}
	if st := pc.Stats(); st != (cacheutil.CacheStats{}) {
		t.Fatalf("nil Stats = %+v", st)
	}
}
