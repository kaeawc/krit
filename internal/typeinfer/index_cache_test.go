package typeinfer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/buildid"
	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestIndexFilesParallelCachedWithTrackerReusesUnchangedFiles(t *testing.T) {
	dir := t.TempDir()
	cacheDir := TypeIndexCacheDir(dir)
	aPath := filepath.Join(dir, "A.kt")
	if err := os.WriteFile(aPath, []byte("package demo\nclass A { val name: String = \"a\" }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseFile(context.Background(), aPath)
	if err != nil {
		t.Fatal(err)
	}

	first := NewResolver()
	hits, misses := first.IndexFilesParallelCachedWithTracker([]*scanner.File{file}, 1, cacheDir, nil)
	if hits != 0 || misses != 1 {
		t.Fatalf("first cache summary = hits %d misses %d, want 0/1", hits, misses)
	}
	if got := first.ResolveImport("String", file); got != "kotlin.String" {
		t.Fatalf("first resolver import String = %q", got)
	}

	second := NewResolver()
	hits, misses = second.IndexFilesParallelCachedWithTracker([]*scanner.File{file}, 1, cacheDir, nil)
	if hits != 1 || misses != 0 {
		t.Fatalf("second cache summary = hits %d misses %d, want 1/0", hits, misses)
	}
	if got := second.ResolveImport("String", file); got != "kotlin.String" {
		t.Fatalf("cached resolver import String = %q", got)
	}
	info := second.ClassHierarchy("A")
	if info == nil || len(info.Members) != 1 || info.Members[0].Type == nil || !info.Members[0].Type.Resolved {
		t.Fatalf("cached member type lost resolved evidence: %#v", info)
	}
}

func TestIndexFilesParallelCachedRetainsDistinctClassScopes(t *testing.T) {
	dir := t.TempDir()
	cacheDir := TypeIndexCacheDir(dir)
	path := filepath.Join(dir, "Scopes.kt")
	source := []byte(`package example
class A {
    class Context
    fun service(context: Context) = context.getSystemService("service")
}
class B {
    class Context
}
`)
	if err := os.WriteFile(path, source, 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}

	first := NewResolver()
	first.IndexFilesParallelCachedWithTracker([]*scanner.File{file}, 1, cacheDir, nil)
	second := NewResolver()
	hits, misses := second.IndexFilesParallelCachedWithTracker([]*scanner.File{file}, 1, cacheDir, nil)
	if hits != 1 || misses != 0 {
		t.Fatalf("cached scope summary = hits %d misses %d, want 1/0", hits, misses)
	}
	if got := second.classFQNScopes["example.Context"]; len(got) != 2 {
		t.Fatalf("cached declarations for example.Context = %d, want both scoped declarations", len(got))
	}
	call := flatFirstOfTypeWithText(file, "call_expression", `context.getSystemService("service")`)
	if call == 0 {
		t.Fatal("missing Context receiver call")
	}
	if got := second.ResolveFlatNode(call, file); got.Kind != TypeUnknown {
		t.Fatalf("visible A.Context call = %+v, want unknown local method result", got)
	}
}

func TestIndexFilesParallelCachedInvalidatesChangedFileOnly(t *testing.T) {
	dir := t.TempDir()
	cacheDir := TypeIndexCacheDir(dir)
	aPath := filepath.Join(dir, "A.kt")
	bPath := filepath.Join(dir, "B.kt")
	if err := os.WriteFile(aPath, []byte("class A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bPath, []byte("class B\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	aFile, err := scanner.ParseFile(context.Background(), aPath)
	if err != nil {
		t.Fatal(err)
	}
	bFile, err := scanner.ParseFile(context.Background(), bPath)
	if err != nil {
		t.Fatal(err)
	}

	initial := NewResolver()
	initial.IndexFilesParallelCachedWithTracker([]*scanner.File{aFile, bFile}, 2, cacheDir, nil)

	if err := os.WriteFile(bPath, []byte("class C\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The second index stands in for a later krit run, which starts with an
	// empty content-hash memo (runner_state resets it). Without the reset, the
	// same-size rewrite above can land in the same coarse mtime tick and reuse
	// the hash memoized for the old content.
	hashutil.ResetDefault()
	bChanged, err := scanner.ParseFile(context.Background(), bPath)
	if err != nil {
		t.Fatal(err)
	}
	updated := NewResolver()
	hits, misses := updated.IndexFilesParallelCachedWithTracker([]*scanner.File{aFile, bChanged}, 2, cacheDir, nil)
	if hits != 1 || misses != 1 {
		t.Fatalf("changed cache summary = hits %d misses %d, want 1/1", hits, misses)
	}
	if updated.ClassHierarchy("A") == nil {
		t.Fatalf("unchanged file contribution was not reused")
	}
	if updated.ClassHierarchy("B") != nil {
		t.Fatalf("changed file retained stale class B")
	}
	if updated.ClassHierarchy("C") == nil {
		t.Fatalf("changed file did not index class C")
	}
}

func TestTypeIndexCacheRejectsPreAnnotationOffsetVersion(t *testing.T) {
	old := packedFileTypeInfo{
		Version: 4,
		Path:    "Old.kt",
		RootScope: packedScope{
			Children: []packedScope{{StartByte: 24, EndByte: 40}},
		},
	}
	if _, ok := unpackFileTypeInfo(old); ok {
		t.Fatal("accepted version 4 type index with declaration offsets predating annotation normalization")
	}
}

func TestTypeIndexCacheRejectsUnscopedAliasVersion(t *testing.T) {
	old := packedFileTypeInfo{
		Version: 10,
		Path:    "a/Aliases.kt",
		TypeAliases: map[string]*ResolvedType{
			"Registry": {Name: "MutableMap", FQN: "kotlin.collections.MutableMap", Resolved: true},
		},
	}
	if _, ok := unpackFileTypeInfo(old); ok {
		t.Fatal("accepted version 10 type index with a package alias under a bare-name key")
	}
}

func TestTypeIndexCacheRejectsUnscopedClassNames(t *testing.T) {
	old := packedFileTypeInfo{Version: 11, Path: "Screen.kt"}
	if _, ok := unpackFileTypeInfo(old); ok {
		t.Fatal("accepted class names without declaration scope offsets")
	}
}

func TestTypeIndexCacheRejectsTypesWithoutProjectLookalike(t *testing.T) {
	old := packedFileTypeInfo{Version: 12, Path: "Screen.kt"}
	if _, ok := unpackFileTypeInfo(old); ok {
		t.Fatal("accepted types without same-file project lookalike provenance")
	}
}

func TestTypeIndexCacheRejectsTypesWithPrematureImportResolution(t *testing.T) {
	old := packedFileTypeInfo{Version: 13, Path: "Analytics.kt"}
	if _, ok := unpackFileTypeInfo(old); ok {
		t.Fatal("accepted types resolved with imports exposed during declaration indexing")
	}
}

func TestTypeIndexCacheMissesAcrossBuilds(t *testing.T) {
	dir := t.TempDir()
	cacheDir := TypeIndexCacheDir(dir)
	aPath := filepath.Join(dir, "A.kt")
	if err := os.WriteFile(aPath, []byte("package demo\nclass A { val name: String = \"a\" }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseFile(context.Background(), aPath)
	if err != nil {
		t.Fatal(err)
	}
	index := func() (int, int) {
		return NewResolver().IndexFilesParallelCachedWithTracker([]*scanner.File{file}, 1, cacheDir, nil)
	}

	buildid.OverrideTokenForTest(t, "rev:build-a")
	if hits, misses := index(); hits != 0 || misses != 1 {
		t.Fatalf("cold scan = hits %d misses %d, want 0/1", hits, misses)
	}
	if hits, misses := index(); hits != 1 || misses != 0 {
		t.Fatalf("same build rescan = hits %d misses %d, want 1/0", hits, misses)
	}

	// Type inference code differs between builds, so another build must not reuse the entry.
	buildid.OverrideTokenForTest(t, "rev:build-b")
	if hits, misses := index(); hits != 0 || misses != 1 {
		t.Fatalf("different build = hits %d misses %d, want 0/1", hits, misses)
	}
	if hits, misses := index(); hits != 1 || misses != 0 {
		t.Fatalf("different build rescan = hits %d misses %d, want 1/0", hits, misses)
	}
}

// TestTypeIndexCacheTornEntryIsMiss covers entries damaged by a crash: cache
// writes skip fsync, so an entry can come back empty, cut short, or garbage.
// Each must be a miss that re-indexes the file, with the same resolution.
func TestTypeIndexCacheTornEntryIsMiss(t *testing.T) {
	dir := t.TempDir()
	cacheDir := TypeIndexCacheDir(dir)
	aPath := filepath.Join(dir, "A.kt")
	if err := os.WriteFile(aPath, []byte("package demo\nclass A { val name: String = \"a\" }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseFile(context.Background(), aPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, misses := NewResolver().IndexFilesParallelCachedWithTracker([]*scanner.File{file}, 1, cacheDir, nil); misses != 1 {
		t.Fatalf("cold scan misses = %d, want 1", misses)
	}
	entry := typeIndexCachePath(cacheDir, typeIndexCacheKey(file))
	valid, err := os.ReadFile(entry)
	if err != nil {
		t.Fatalf("read entry: %v", err)
	}
	damaged := map[string][]byte{
		"empty":       {},
		"truncated":   valid[:len(valid)/2],
		"zero-filled": make([]byte, len(valid)),
		"garbage":     []byte("not a zstd gob payload"),
	}
	for name, data := range damaged {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(entry, data, 0o644); err != nil {
				t.Fatal(err)
			}
			r := NewResolver()
			hits, misses := r.IndexFilesParallelCachedWithTracker([]*scanner.File{file}, 1, cacheDir, nil)
			if hits != 0 || misses != 1 {
				t.Fatalf("damaged entry = hits %d misses %d, want 0/1", hits, misses)
			}
			if got := r.ResolveImport("String", file); got != "kotlin.String" {
				t.Fatalf("resolver import String after miss = %q", got)
			}
		})
	}
}
