package scanner

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/hashutil"
)

// largeJavaSource returns a Java source blob guaranteed to exceed
// parseCacheMinFileSize so the cache actually engages.
func largeJavaSource() string {
	var b strings.Builder
	b.WriteString("package a;\n\npublic class Gen {\n")
	for i := 0; i < 120; i++ {
		b.WriteString("  public int f")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("() { return ")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("; }\n")
	}
	b.WriteString("}\n")
	return b.String()
}

func writeJava(t *testing.T, dir, name, src string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestParseCache_Java_RoundTrip(t *testing.T) {
	repo := t.TempDir()
	pc, err := NewParseCache(repo)
	if err != nil {
		t.Fatalf("NewParseCache: %v", err)
	}

	src := largeJavaSource()
	path := writeJava(t, repo, "Round.java", src)

	miss, err := ParseJavaFileCached(path, pc)
	if err != nil {
		t.Fatalf("parse (miss): %v", err)
	}
	if miss.FlatTree == nil {
		t.Fatal("expected FlatTree on miss")
	}

	hit, err := ParseJavaFileCached(path, pc)
	if err != nil {
		t.Fatalf("parse (hit): %v", err)
	}
	if hit.FlatTree == nil {
		t.Fatal("expected FlatTree on hit")
	}

	if len(hit.FlatTree.Nodes) != len(miss.FlatTree.Nodes) {
		t.Fatalf("node count differs: miss=%d hit=%d",
			len(miss.FlatTree.Nodes), len(hit.FlatTree.Nodes))
	}
	for i := range miss.FlatTree.Nodes {
		if miss.FlatTree.Nodes[i].TypeName() != hit.FlatTree.Nodes[i].TypeName() {
			t.Fatalf("node %d type name differs: want %q got %q",
				i, miss.FlatTree.Nodes[i].TypeName(), hit.FlatTree.Nodes[i].TypeName())
		}
	}
}

func TestParseJavaFileCachedForIndex_RecordsMissPerfAndPrecomputesRefs(t *testing.T) {
	repo := t.TempDir()
	src := "package a;\npublic class Foo { Helper h; void m() { new Helper().go(); } }\nclass Helper { void go() {} }\n"
	path := writeJava(t, repo, "Foo.java", src)

	stats := &JavaIndexPerf{}
	file, err := ParseJavaFileCachedForIndex(path, nil, stats)
	if err != nil {
		t.Fatalf("parse index java: %v", err)
	}
	if len(file.Lines) != 0 {
		t.Fatalf("index-only Java parse built %d lines, want 0", len(file.Lines))
	}
	if !file.ReferencesPrecomputed {
		t.Fatal("expected Java references to be precomputed on cache miss")
	}
	if !hasReferenceName(file.PrecomputedReferences, "Helper") {
		t.Fatalf("expected precomputed Helper reference, got %#v", file.PrecomputedReferences)
	}

	// Prove the collector reuses the precomputed references instead of
	// requiring the flattened AST to still be present.
	file.FlatTree = nil
	var refs []Reference
	collectJavaReferencesFlat(file, &refs)
	if !hasReferenceName(refs, "Helper") {
		t.Fatalf("expected reused Helper reference, got %#v", refs)
	}

	snap := stats.Snapshot()
	if snap.Files != 1 || snap.Bytes != int64(len(src)) {
		t.Fatalf("summary = files:%d bytes:%d, want files:1 bytes:%d", snap.Files, snap.Bytes, len(src))
	}
	if snap.CacheHits != 0 || snap.CacheMisses != 1 {
		t.Fatalf("cache summary = hits:%d misses:%d, want hits:0 misses:1", snap.CacheHits, snap.CacheMisses)
	}
	if snap.FileReadNs <= 0 || snap.TreeSitterParseNs <= 0 || snap.FlattenTreeNs <= 0 || snap.ReferenceExtractionNs <= 0 {
		t.Fatalf("expected positive read/parse/flatten/reference timings, got %#v", snap)
	}
}

func TestParseJavaFileCachedForIndex_RecordsCacheHitPerf(t *testing.T) {
	repo := t.TempDir()
	pc, err := NewParseCache(repo)
	if err != nil {
		t.Fatalf("NewParseCache: %v", err)
	}

	src := largeJavaSource()
	path := writeJava(t, repo, "Hit.java", src)
	seed, err := ParseJavaFileCached(path, nil)
	if err != nil {
		t.Fatalf("seed parse: %v", err)
	}
	if err := pc.SaveJava(path, []byte(src), seed.FlatTree); err != nil {
		t.Fatalf("SaveJava: %v", err)
	}

	stats := &JavaIndexPerf{}
	hit, err := ParseJavaFileCachedForIndex(path, pc, stats)
	if err != nil {
		t.Fatalf("parse index java hit: %v", err)
	}
	if len(hit.Lines) != 0 {
		t.Fatalf("index-only Java cache hit built %d lines, want 0", len(hit.Lines))
	}
	if hit.ReferencesPrecomputed {
		t.Fatal("cache-hit Java parse should not precompute references before index cache lookup")
	}

	snap := stats.Snapshot()
	if snap.CacheHits != 1 || snap.CacheMisses != 0 {
		t.Fatalf("cache summary = hits:%d misses:%d, want hits:1 misses:0", snap.CacheHits, snap.CacheMisses)
	}
	if snap.TreeSitterParseNs != 0 || snap.FlattenTreeNs != 0 || snap.ReferenceExtractionNs != 0 {
		t.Fatalf("cache hit should skip parse/flatten/reference extraction, got %#v", snap)
	}
}

func hasReferenceName(refs []Reference, name string) bool {
	for _, ref := range refs {
		if ref.Name == name {
			return true
		}
	}
	return false
}

// TestParseCache_Java_CrossLanguageIsolation asserts that Java and
// Kotlin entries share no shard. A byte-identical blob parsed as Java
// and Kotlin must hit the Java shard only when read as Java, and vice
// versa — otherwise a FlatTree built under one grammar could be handed
// back for a request under the other.
func TestParseCache_Java_CrossLanguageIsolation(t *testing.T) {
	repo := t.TempDir()
	pc, err := NewParseCache(repo)
	if err != nil {
		t.Fatalf("NewParseCache: %v", err)
	}
	// Use identical-but-large content. The grammar-version token and
	// per-language subdir both isolate; test guards against regression
	// on either axis.
	src := largeJavaSource()
	path := writeJava(t, repo, "Iso.java", src)
	if _, err := ParseJavaFileCached(path, pc); err != nil {
		t.Fatalf("seed java: %v", err)
	}
	// Kotlin Load on the same bytes must miss — the Kotlin shard never
	// saw a write for this hash even though the Java shard did.
	if _, ok := pc.Load("", []byte(src)); ok {
		t.Fatal("expected miss on Kotlin Load after Java-only write")
	}
	// Java LoadJava on the same bytes must hit.
	if _, ok := pc.LoadJava("", []byte(src)); !ok {
		t.Fatal("expected hit on LoadJava after Java write")
	}
}

func TestParseCache_Java_GrammarVersionMismatch(t *testing.T) {
	repo := t.TempDir()
	pc, err := NewParseCache(repo)
	if err != nil {
		t.Fatalf("NewParseCache: %v", err)
	}
	src := largeJavaSource()
	if _, err := ParseJavaFileCached(writeJava(t, repo, "GV.java", src), pc); err != nil {
		t.Fatalf("seed parse: %v", err)
	}

	hash := hashutil.HashHex([]byte(src))
	data, packPath := packedBlobForHash(t, pc.java, hash)
	if !cacheutil.IsZstdFrame(data) {
		t.Fatalf("java parse cache entry is not zstd-framed: %x", data[:min(4, len(data))])
	}
	if err := os.WriteFile(filepath.Join(pc.JavaDir(), "grammar-version"), []byte("smacker/go-tree-sitter@BOGUS#java:1"), 0o644); err != nil {
		t.Fatalf("rewrite grammar-version sidecar: %v", err)
	}

	pc2, err := NewParseCache(repo)
	if err != nil {
		t.Fatalf("NewParseCache after sidecar edit: %v", err)
	}
	if _, ok := pc2.LoadJava("", []byte(src)); ok {
		t.Fatal("expected miss after Java grammar-version mismatch")
	}
	if _, err := os.Stat(packPath); !os.IsNotExist(err) {
		t.Fatalf("expected stale Java pack removed after sidecar mismatch, stat err=%v", err)
	}
}

func TestParseCache_Java_ContentChangeMisses(t *testing.T) {
	repo := t.TempDir()
	pc, err := NewParseCache(repo)
	if err != nil {
		t.Fatalf("NewParseCache: %v", err)
	}
	src := largeJavaSource()
	if _, err := ParseJavaFileCached(writeJava(t, repo, "CC.java", src), pc); err != nil {
		t.Fatalf("seed parse: %v", err)
	}

	mutated := src + "// one byte change\n"
	if _, ok := pc.LoadJava("", []byte(mutated)); ok {
		t.Fatal("expected miss for mutated content")
	}
	if _, ok := pc.LoadJava("", []byte(src)); !ok {
		t.Fatal("expected hit for original content")
	}
}

func TestParseCache_Java_SmallFileSkipsCache(t *testing.T) {
	repo := t.TempDir()
	pc, err := NewParseCache(repo)
	if err != nil {
		t.Fatalf("NewParseCache: %v", err)
	}
	tiny := "class X {}\n"
	if len(tiny) >= parseCacheMinFileSize {
		t.Fatalf("test assumes tiny source < threshold, got %d", len(tiny))
	}
	path := writeJava(t, repo, "Tiny.java", tiny)
	if _, err := ParseJavaFileCached(path, pc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	entries := filepath.Join(pc.JavaDir(), "entries")
	_ = filepath.Walk(entries, func(p string, info os.FileInfo, werr error) error {
		if werr != nil || info == nil || info.IsDir() {
			return nil
		}
		if filepath.Ext(p) == ".gob" {
			t.Fatalf("unexpected cache entry written for small file: %s", p)
		}
		return nil
	})
}

func TestParseCache_Java_ConcurrentWritesSameHash(t *testing.T) {
	repo := t.TempDir()
	pc, err := NewParseCache(repo)
	if err != nil {
		t.Fatalf("NewParseCache: %v", err)
	}
	src := largeJavaSource()
	path := writeJava(t, repo, "Conc.java", src)
	seed, err := ParseJavaFileCached(path, nil)
	if err != nil {
		t.Fatalf("seed parse: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = pc.SaveJava("", []byte(src), seed.FlatTree)
		}()
	}
	wg.Wait()

	tree, ok := pc.LoadJava("", []byte(src))
	if !ok {
		t.Fatal("expected hit after concurrent writes")
	}
	if len(tree.Nodes) != len(seed.FlatTree.Nodes) {
		t.Fatalf("node count mismatch: want %d got %d",
			len(seed.FlatTree.Nodes), len(tree.Nodes))
	}
}

func TestParseCache_Java_NilIsSafe(t *testing.T) {
	var pc *ParseCache
	if tree, ok := pc.LoadJava("", []byte("anything")); ok || tree != nil {
		t.Fatal("nil LoadJava should be a miss")
	}
	if err := pc.SaveJava("", []byte("anything"), &FlatTree{}); err != nil {
		t.Fatalf("nil SaveJava: %v", err)
	}
	if pc.JavaDir() != "" {
		t.Fatalf("nil JavaDir should be empty")
	}
}

func TestParseCache_ClearRemovesBothLanguages(t *testing.T) {
	repo := t.TempDir()
	pc, err := NewParseCache(repo)
	if err != nil {
		t.Fatalf("NewParseCache: %v", err)
	}
	// Seed one entry per language.
	ktSrc := largeSource()
	jSrc := largeJavaSource()
	if _, err := ParseKotlinFileCached(writeKotlin(t, repo, "K.kt", ktSrc), pc); err != nil {
		t.Fatalf("kotlin seed: %v", err)
	}
	if _, err := ParseJavaFileCached(writeJava(t, repo, "J.java", jSrc), pc); err != nil {
		t.Fatalf("java seed: %v", err)
	}
	if err := pc.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if _, ok := pc.Load("", []byte(ktSrc)); ok {
		t.Fatal("expected miss on Kotlin after Clear")
	}
	if _, ok := pc.LoadJava("", []byte(jSrc)); ok {
		t.Fatal("expected miss on Java after Clear")
	}
}

func TestParseCache_GrammarVersions_Distinct(t *testing.T) {
	if KotlinGrammarVersion() == JavaGrammarVersion() {
		t.Fatalf("Kotlin and Java grammar versions must differ: %q",
			KotlinGrammarVersion())
	}
}
