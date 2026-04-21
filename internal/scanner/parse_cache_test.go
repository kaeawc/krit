package scanner

import (
	"bytes"
	"encoding/gob"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/kaeawc/krit/internal/hashutil"
)

// largeSource returns a Kotlin source blob guaranteed to exceed
// parseCacheMinFileSize so the cache actually engages.
func largeSource() string {
	var b strings.Builder
	b.WriteString("package a\n")
	for i := 0; i < 200; i++ {
		b.WriteString("fun f")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("(): Int = ")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("\n")
	}
	return b.String()
}

func writeKotlin(t *testing.T, dir, name, src string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestParseCache_RoundTrip(t *testing.T) {
	repo := t.TempDir()
	pc, err := NewParseCache(repo)
	if err != nil {
		t.Fatalf("NewParseCache: %v", err)
	}

	src := largeSource()
	path := writeKotlin(t, repo, "Round.kt", src)

	miss, err := ParseKotlinFileCached(path, pc)
	if err != nil {
		t.Fatalf("parse (miss): %v", err)
	}
	if miss.FlatTree == nil {
		t.Fatal("expected FlatTree on miss")
	}

	hit, err := ParseKotlinFileCached(path, pc)
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
		m := miss.FlatTree.Nodes[i]
		h := hit.FlatTree.Nodes[i]
		if m != h {
			t.Fatalf("node %d differs after round-trip:\n  miss=%+v\n  hit =%+v", i, m, h)
		}
	}
}

// TestParseCache_HitInFreshProcess simulates a second process that has
// never seen the file by starting with an empty-ish NodeTypeTable via a
// fresh *ParseCache pointer. The cache payload encodes its own local
// type table so the remap path must reconstruct global indices without
// re-parsing.
func TestParseCache_HitAfterRestart(t *testing.T) {
	repo := t.TempDir()
	src := largeSource()
	path := writeKotlin(t, repo, "Restart.kt", src)

	// Run 1: populate cache.
	pc1, err := NewParseCache(repo)
	if err != nil {
		t.Fatalf("NewParseCache run1: %v", err)
	}
	f1, err := ParseKotlinFileCached(path, pc1)
	if err != nil {
		t.Fatalf("parse run1: %v", err)
	}

	// Run 2: a fresh *ParseCache pointer reads what run 1 wrote. Even
	// though the global NodeTypeTable is already populated in this
	// process, the Load path still has to remap entry-local indices
	// through it — verify the node count and types match run 1.
	pc2, err := NewParseCache(repo)
	if err != nil {
		t.Fatalf("NewParseCache run2: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	tree, ok := pc2.Load("", content)
	if !ok {
		t.Fatal("expected cache hit on run 2")
	}
	if len(tree.Nodes) != len(f1.FlatTree.Nodes) {
		t.Fatalf("node count diverged: want %d got %d",
			len(f1.FlatTree.Nodes), len(tree.Nodes))
	}
	for i := range tree.Nodes {
		if tree.Nodes[i].TypeName() != f1.FlatTree.Nodes[i].TypeName() {
			t.Fatalf("node %d type name differs: want %q got %q",
				i, f1.FlatTree.Nodes[i].TypeName(), tree.Nodes[i].TypeName())
		}
	}
}

func TestParseCache_GrammarVersionMismatch(t *testing.T) {
	repo := t.TempDir()
	pc, err := NewParseCache(repo)
	if err != nil {
		t.Fatalf("NewParseCache: %v", err)
	}
	src := largeSource()
	if _, err := ParseKotlinFileCached(writeKotlin(t, repo, "GV.kt", src), pc); err != nil {
		t.Fatalf("seed parse: %v", err)
	}

	// Mutate the stored entry's GrammarVer to something else and
	// verify the next Load treats it as a miss.
	hash := hashutil.HashHex([]byte(src))
	entryPath := pc.entryPath(hash)

	data, err := os.ReadFile(entryPath)
	if err != nil {
		t.Fatalf("read entry: %v", err)
	}
	var entry parseCacheEntry
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&entry); err != nil {
		t.Fatalf("decode entry: %v", err)
	}
	entry.GrammarVer = "smacker/go-tree-sitter@BOGUS"
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&entry); err != nil {
		t.Fatalf("re-encode entry: %v", err)
	}
	if err := os.WriteFile(entryPath, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("rewrite entry: %v", err)
	}

	if _, ok := pc.Load("", []byte(src)); ok {
		t.Fatal("expected miss after grammar-version mismatch")
	}
}

func TestParseCache_ContentChangeMisses(t *testing.T) {
	repo := t.TempDir()
	pc, err := NewParseCache(repo)
	if err != nil {
		t.Fatalf("NewParseCache: %v", err)
	}
	src := largeSource()
	if _, err := ParseKotlinFileCached(writeKotlin(t, repo, "CC.kt", src), pc); err != nil {
		t.Fatalf("seed parse: %v", err)
	}

	mutated := src + " // one byte change\n"
	if _, ok := pc.Load("", []byte(mutated)); ok {
		t.Fatal("expected miss for mutated content")
	}
	if _, ok := pc.Load("", []byte(src)); !ok {
		t.Fatal("expected hit for original content")
	}
}

func TestParseCache_CorruptEntryTreatedAsMiss(t *testing.T) {
	repo := t.TempDir()
	pc, err := NewParseCache(repo)
	if err != nil {
		t.Fatalf("NewParseCache: %v", err)
	}
	src := largeSource()
	if _, err := ParseKotlinFileCached(writeKotlin(t, repo, "Bad.kt", src), pc); err != nil {
		t.Fatalf("seed parse: %v", err)
	}
	hash := hashutil.HashHex([]byte(src))
	entryPath := pc.entryPath(hash)
	if err := os.WriteFile(entryPath, []byte("not-a-gob-payload"), 0o644); err != nil {
		t.Fatalf("corrupt entry: %v", err)
	}
	if _, ok := pc.Load("", []byte(src)); ok {
		t.Fatal("expected miss on corrupt entry")
	}
	// Corrupt entry should have been removed so the next hit path
	// doesn't redo the Open+Decode dance forever.
	if _, err := os.Stat(entryPath); !os.IsNotExist(err) {
		t.Fatalf("corrupt entry not cleaned up: err=%v", err)
	}
}

func TestParseCache_SmallFileSkipsCache(t *testing.T) {
	repo := t.TempDir()
	pc, err := NewParseCache(repo)
	if err != nil {
		t.Fatalf("NewParseCache: %v", err)
	}
	tiny := "fun x() = 1\n"
	if len(tiny) >= parseCacheMinFileSize {
		t.Fatalf("test assumes tiny source < threshold, got %d", len(tiny))
	}
	path := writeKotlin(t, repo, "Tiny.kt", tiny)
	if _, err := ParseKotlinFileCached(path, pc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	entries := filepath.Join(pc.Dir(), "entries")
	// Walk the shard dir: expect no .gob files.
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

func TestParseCache_ConcurrentWritesSameHash(t *testing.T) {
	repo := t.TempDir()
	pc, err := NewParseCache(repo)
	if err != nil {
		t.Fatalf("NewParseCache: %v", err)
	}
	src := largeSource()
	path := writeKotlin(t, repo, "Conc.kt", src)
	// Seed so we have a real FlatTree to write.
	seed, err := ParseKotlinFileCached(path, nil)
	if err != nil {
		t.Fatalf("seed parse: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = pc.Save("", []byte(src), seed.FlatTree)
		}()
	}
	wg.Wait()

	tree, ok := pc.Load("", []byte(src))
	if !ok {
		t.Fatal("expected hit after concurrent writes")
	}
	if len(tree.Nodes) != len(seed.FlatTree.Nodes) {
		t.Fatalf("node count mismatch: want %d got %d",
			len(seed.FlatTree.Nodes), len(tree.Nodes))
	}
}

func TestParseCache_NilIsSafe(t *testing.T) {
	// Belt-and-braces: the nil receiver is documented as a disabled
	// cache. Exercise the public methods so a future edit that adds an
	// unconditional deref gets caught by CI.
	var pc *ParseCache
	if tree, ok := pc.Load("", []byte("anything")); ok || tree != nil {
		t.Fatal("nil Load should be a miss")
	}
	if err := pc.Save("", []byte("anything"), &FlatTree{}); err != nil {
		t.Fatalf("nil Save: %v", err)
	}
	if err := pc.Clear(); err != nil {
		t.Fatalf("nil Clear: %v", err)
	}
	if pc.Dir() != "" {
		t.Fatalf("nil Dir should be empty")
	}
}

func TestClearParseCache_Removes(t *testing.T) {
	repo := t.TempDir()
	pc, err := NewParseCache(repo)
	if err != nil {
		t.Fatalf("NewParseCache: %v", err)
	}
	src := largeSource()
	if _, err := ParseKotlinFileCached(writeKotlin(t, repo, "Clr.kt", src), pc); err != nil {
		t.Fatalf("seed parse: %v", err)
	}
	if err := ClearParseCache(repo); err != nil {
		t.Fatalf("ClearParseCache: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".krit", parseCacheDirName)); !os.IsNotExist(err) {
		t.Fatalf("parse cache dir still exists after clear: err=%v", err)
	}
}
