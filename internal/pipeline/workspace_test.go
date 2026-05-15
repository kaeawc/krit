package pipeline

import (
	"context"
	"sync"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

const sampleKotlin = `package demo

class Foo {
    fun greet(name: String) = "Hi " + name
}
`

func TestWorkspaceState_NilReceiverDelegates(t *testing.T) {
	var ws *WorkspaceState
	file, err := ws.ParseFile(context.Background(), "Foo.kt", []byte(sampleKotlin))
	if err != nil {
		t.Fatalf("ParseFile on nil ws: %v", err)
	}
	if file == nil {
		t.Fatal("expected parsed file from nil receiver fallback")
	}
}

func TestWorkspaceState_HitsAndMisses(t *testing.T) {
	ws := NewWorkspaceState("/tmp/test")
	ctx := context.Background()
	content := []byte(sampleKotlin)

	first, err := ws.ParseFile(ctx, "Foo.kt", content)
	if err != nil {
		t.Fatalf("first parse: %v", err)
	}
	second, err := ws.ParseFile(ctx, "Foo.kt", content)
	if err != nil {
		t.Fatalf("second parse: %v", err)
	}
	if first != second {
		t.Fatal("expected second ParseFile to return the cached *File pointer")
	}

	stats := ws.Stats()
	if stats.Misses != 1 {
		t.Errorf("misses: got %d, want 1", stats.Misses)
	}
	if stats.Hits != 1 {
		t.Errorf("hits: got %d, want 1", stats.Hits)
	}
	if stats.ParsedEntries != 1 {
		t.Errorf("entries: got %d, want 1", stats.ParsedEntries)
	}
}

func TestWorkspaceState_ContentChangeForcesReparse(t *testing.T) {
	ws := NewWorkspaceState("/tmp/test")
	ctx := context.Background()

	first, err := ws.ParseFile(ctx, "Foo.kt", []byte(sampleKotlin))
	if err != nil {
		t.Fatalf("first parse: %v", err)
	}
	second, err := ws.ParseFile(ctx, "Foo.kt", []byte(sampleKotlin+"\nclass Bar {}"))
	if err != nil {
		t.Fatalf("second parse: %v", err)
	}
	if first == second {
		t.Fatal("expected content change to produce a fresh *File")
	}

	stats := ws.Stats()
	if stats.Misses != 2 {
		t.Errorf("misses: got %d, want 2 (content changed each call)", stats.Misses)
	}
	if stats.Hits != 0 {
		t.Errorf("hits: got %d, want 0", stats.Hits)
	}
}

// TestWorkspaceState_PathNormalization ensures two cosmetically
// different spellings of the same path share a cache entry, so a
// caller passing "/a/./b/Foo.kt" and another passing "/a/b/Foo.kt"
// don't double-parse identical content.
func TestWorkspaceState_PathNormalization(t *testing.T) {
	ws := NewWorkspaceState("/tmp/test")
	ctx := context.Background()
	content := []byte(sampleKotlin)

	if _, err := ws.ParseFile(ctx, "/a/./b/Foo.kt", content); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := ws.ParseFile(ctx, "/a/b/Foo.kt", content); err != nil {
		t.Fatalf("second: %v", err)
	}
	if got := ws.Stats().Hits; got != 1 {
		t.Errorf("expected the second call to hit the cache (paths normalize equal), hits=%d", got)
	}
}

func TestWorkspaceState_DifferentPathsDoNotCollide(t *testing.T) {
	ws := NewWorkspaceState("/tmp/test")
	ctx := context.Background()
	content := []byte(sampleKotlin)

	a, err := ws.ParseFile(ctx, "A.kt", content)
	if err != nil {
		t.Fatalf("A: %v", err)
	}
	b, err := ws.ParseFile(ctx, "B.kt", content)
	if err != nil {
		t.Fatalf("B: %v", err)
	}
	if a == b {
		t.Fatal("identical content under different paths must produce distinct cache entries")
	}
	if got := ws.Stats().ParsedEntries; got != 2 {
		t.Errorf("entries: got %d, want 2", got)
	}
}

func TestWorkspaceState_InvalidateDropsEntry(t *testing.T) {
	ws := NewWorkspaceState("/tmp/test")
	ctx := context.Background()
	content := []byte(sampleKotlin)

	first, _ := ws.ParseFile(ctx, "Foo.kt", content)
	ws.Invalidate("Foo.kt")
	second, _ := ws.ParseFile(ctx, "Foo.kt", content)
	if first == second {
		t.Fatal("expected fresh parse after Invalidate")
	}
	if got := ws.Stats().Hits; got != 0 {
		t.Errorf("hits should remain 0 across invalidate-rebuild, got %d", got)
	}
}

func TestWorkspaceState_InvalidateAllDropsEverything(t *testing.T) {
	ws := NewWorkspaceState("/tmp/test")
	ctx := context.Background()

	for _, name := range []string{"A.kt", "B.kt", "C.kt"} {
		if _, err := ws.ParseFile(ctx, name, []byte(sampleKotlin)); err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
	}
	if got := ws.Stats().ParsedEntries; got != 3 {
		t.Fatalf("setup: got %d entries, want 3", got)
	}
	ws.InvalidateAll()
	if got := ws.Stats().ParsedEntries; got != 0 {
		t.Errorf("after InvalidateAll: got %d entries, want 0", got)
	}
}

func TestWorkspaceState_ConcurrentParseIsSafe(t *testing.T) {
	ws := NewWorkspaceState("/tmp/test")
	ctx := context.Background()
	content := []byte(sampleKotlin)

	const goroutines = 16
	const iterations = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				if _, err := ws.ParseFile(ctx, "Foo.kt", content); err != nil {
					t.Errorf("ParseFile: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()

	stats := ws.Stats()
	total := stats.Hits + stats.Misses
	if total != int64(goroutines*iterations) {
		t.Errorf("hits+misses: got %d, want %d", total, goroutines*iterations)
	}
	if stats.ParsedEntries != 1 {
		t.Errorf("expected 1 entry after concurrent same-content parses, got %d", stats.ParsedEntries)
	}
}

func TestWorkspaceState_LibraryFactsCachesByFingerprint(t *testing.T) {
	ws := NewWorkspaceState("/tmp/test")
	calls := 0
	build := func() *librarymodel.Facts {
		calls++
		return &librarymodel.Facts{}
	}

	first := ws.LibraryFacts("fp-A", build)
	second := ws.LibraryFacts("fp-A", build)
	if first != second {
		t.Error("expected pointer identity for matching fingerprint")
	}
	if calls != 1 {
		t.Errorf("build calls: got %d, want 1", calls)
	}

	third := ws.LibraryFacts("fp-B", build)
	if third == first {
		t.Error("expected fingerprint mismatch to rebuild")
	}
	if calls != 2 {
		t.Errorf("build calls after mismatch: got %d, want 2", calls)
	}

	if !ws.CrossFileStats().HasLibraryFacts {
		t.Error("expected HasLibraryFacts after population")
	}
}

func TestWorkspaceState_CodeIndexCachesByFingerprint(t *testing.T) {
	ws := NewWorkspaceState("/tmp/test")
	calls := 0
	build := func() *scanner.CodeIndex {
		calls++
		return &scanner.CodeIndex{}
	}

	a := ws.CodeIndex("fp-1", build)
	b := ws.CodeIndex("fp-1", build)
	if a != b {
		t.Error("expected pointer identity on cache hit")
	}
	if calls != 1 {
		t.Errorf("build calls: got %d, want 1", calls)
	}

	c := ws.CodeIndex("fp-2", build)
	if c == a {
		t.Error("expected fingerprint change to rebuild")
	}
	if calls != 2 {
		t.Errorf("build calls after rebuild: got %d, want 2", calls)
	}
}

func TestWorkspaceState_LibraryFactsAndCodeIndexAreIndependent(t *testing.T) {
	ws := NewWorkspaceState("/tmp/test")
	lfBuild := 0
	ciBuild := 0
	ws.LibraryFacts("lf", func() *librarymodel.Facts { lfBuild++; return &librarymodel.Facts{} })
	ws.CodeIndex("ci", func() *scanner.CodeIndex { ciBuild++; return &scanner.CodeIndex{} })
	// Replacing one slot must NOT clear the other.
	ws.LibraryFacts("lf2", func() *librarymodel.Facts { lfBuild++; return &librarymodel.Facts{} })

	if !ws.CrossFileStats().HasCodeIndex {
		t.Error("rebuilding LibraryFacts should not clear CodeIndex")
	}
	if lfBuild != 2 || ciBuild != 1 {
		t.Errorf("build counts: lf=%d ci=%d (want 2, 1)", lfBuild, ciBuild)
	}
}

func TestWorkspaceState_CrossFileNilReceiverBuildsEveryCall(t *testing.T) {
	var ws *WorkspaceState
	calls := 0
	build := func() *librarymodel.Facts { calls++; return &librarymodel.Facts{} }
	ws.LibraryFacts("fp", build)
	ws.LibraryFacts("fp", build)
	if calls != 2 {
		t.Errorf("nil receiver should bypass cache, got %d calls (want 2)", calls)
	}
}

func TestWorkspaceState_CrossFileEmptyFingerprintBypassesCache(t *testing.T) {
	ws := NewWorkspaceState("/tmp/test")
	calls := 0
	ws.LibraryFacts("", func() *librarymodel.Facts { calls++; return &librarymodel.Facts{} })
	ws.LibraryFacts("", func() *librarymodel.Facts { calls++; return &librarymodel.Facts{} })
	if calls != 2 {
		t.Errorf("empty fingerprint should bypass cache, got %d calls (want 2)", calls)
	}
	if ws.CrossFileStats().HasLibraryFacts {
		t.Error("empty-fingerprint calls must not populate the cache slot")
	}
}

func TestWorkspaceState_InvalidateLibraryFactsClearsOnlyThatSlot(t *testing.T) {
	ws := NewWorkspaceState("/tmp/test")
	ws.LibraryFacts("lf", func() *librarymodel.Facts { return &librarymodel.Facts{} })
	ws.CodeIndex("ci", func() *scanner.CodeIndex { return &scanner.CodeIndex{} })
	ws.InvalidateLibraryFacts()
	stats := ws.CrossFileStats()
	if stats.HasLibraryFacts {
		t.Error("LibraryFacts slot should be empty after InvalidateLibraryFacts")
	}
	if !stats.HasCodeIndex {
		t.Error("CodeIndex slot should remain populated when only LibraryFacts was invalidated")
	}
}

func TestWorkspaceState_InvalidateCodeIndexClearsOnlyThatSlot(t *testing.T) {
	ws := NewWorkspaceState("/tmp/test")
	ws.LibraryFacts("lf", func() *librarymodel.Facts { return &librarymodel.Facts{} })
	ws.CodeIndex("ci", func() *scanner.CodeIndex { return &scanner.CodeIndex{} })
	ws.InvalidateCodeIndex()
	stats := ws.CrossFileStats()
	if !stats.HasLibraryFacts {
		t.Error("LibraryFacts slot should remain populated when only CodeIndex was invalidated")
	}
	if stats.HasCodeIndex {
		t.Error("CodeIndex slot should be empty after InvalidateCodeIndex")
	}
}

func TestWorkspaceState_InvalidateAllClearsCrossFile(t *testing.T) {
	ws := NewWorkspaceState("/tmp/test")
	ws.LibraryFacts("lf", func() *librarymodel.Facts { return &librarymodel.Facts{} })
	ws.CodeIndex("ci", func() *scanner.CodeIndex { return &scanner.CodeIndex{} })
	ws.Dependents("dep", func() *scanner.DependentsIndex { return &scanner.DependentsIndex{} })
	ws.Resolver("res", func() typeinfer.TypeResolver { return typeinfer.NewResolver() })
	ws.OracleFilter("of", func() *oracle.CallTargetFilterSummary { return &oracle.CallTargetFilterSummary{Enabled: true} })
	ws.InvalidateAll()
	stats := ws.CrossFileStats()
	if stats.HasLibraryFacts || stats.HasCodeIndex || stats.HasDependents ||
		stats.HasResolver || stats.HasOracleFilter {
		t.Errorf("InvalidateAll should clear every cross-file slot, got %+v", stats)
	}
}

func TestWorkspaceState_ConcurrentInvalidateIsSafe(t *testing.T) {
	ws := NewWorkspaceState("/tmp/test")
	ctx := context.Background()
	content := []byte(sampleKotlin)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			_, _ = ws.ParseFile(ctx, "Foo.kt", content)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			ws.Invalidate("Foo.kt")
		}
	}()
	wg.Wait()
	// No assertion beyond "didn't panic / didn't deadlock". The race
	// detector catches data races automatically when -race is set.
}

func TestWorkspaceState_DependentsCachesByFingerprint(t *testing.T) {
	ws := NewWorkspaceState("/tmp/test")
	calls := 0
	build := func() *scanner.DependentsIndex {
		calls++
		return scanner.BuildDependentsIndex(nil)
	}

	a := ws.Dependents("fp-1", build)
	b := ws.Dependents("fp-1", build)
	if a != b {
		t.Error("expected pointer identity on cache hit")
	}
	if calls != 1 {
		t.Errorf("build calls: got %d, want 1", calls)
	}

	c := ws.Dependents("fp-2", build)
	if c == a {
		t.Error("expected fingerprint change to rebuild")
	}
	if calls != 2 {
		t.Errorf("build calls after rebuild: got %d, want 2", calls)
	}
}

func TestWorkspaceState_InvalidateDependentsClearsOnlyThatSlot(t *testing.T) {
	ws := NewWorkspaceState("/tmp/test")
	ws.LibraryFacts("lf", func() *librarymodel.Facts { return &librarymodel.Facts{} })
	ws.CodeIndex("ci", func() *scanner.CodeIndex { return &scanner.CodeIndex{} })
	ws.Dependents("dep", func() *scanner.DependentsIndex { return scanner.BuildDependentsIndex(nil) })

	ws.InvalidateDependents()
	stats := ws.CrossFileStats()
	if !stats.HasLibraryFacts {
		t.Error("InvalidateDependents should not clear LibraryFacts")
	}
	if !stats.HasCodeIndex {
		t.Error("InvalidateDependents should not clear CodeIndex")
	}
	if stats.HasDependents {
		t.Error("InvalidateDependents should clear Dependents")
	}
}

func TestWorkspaceState_InvalidateAllClearsDependents(t *testing.T) {
	ws := NewWorkspaceState("/tmp/test")
	ws.Dependents("dep", func() *scanner.DependentsIndex { return scanner.BuildDependentsIndex(nil) })
	if !ws.CrossFileStats().HasDependents {
		t.Fatal("setup failed: Dependents not cached")
	}
	ws.InvalidateAll()
	if ws.CrossFileStats().HasDependents {
		t.Error("InvalidateAll should clear Dependents")
	}
}

// TestWorkspaceState_LookupAndStoreParsed verifies the path-only fast
// path: StoreParsed populates the cache, LookupParsedByPath returns
// the same *scanner.File pointer, and Invalidate drops it.
func TestWorkspaceState_LookupAndStoreParsed(t *testing.T) {
	w := NewWorkspaceState("")

	if _, ok := w.LookupParsedByPath("/tmp/missing.kt"); ok {
		t.Errorf("LookupParsedByPath on empty cache returned ok=true")
	}

	content := []byte("class Foo {}\n")
	file, err := ParseSingle(context.Background(), "/tmp/foo.kt", content)
	if err != nil {
		t.Fatalf("ParseSingle: %v", err)
	}
	w.StoreParsed("/tmp/foo.kt", content, file)

	got, ok := w.LookupParsedByPath("/tmp/foo.kt")
	if !ok {
		t.Fatalf("LookupParsedByPath after StoreParsed: ok=false")
	}
	if got != file {
		t.Errorf("LookupParsedByPath returned different pointer than StoreParsed")
	}

	w.Invalidate("/tmp/foo.kt")
	if _, ok := w.LookupParsedByPath("/tmp/foo.kt"); ok {
		t.Errorf("LookupParsedByPath after Invalidate: ok=true")
	}
}

// TestWorkspaceState_StoreParsedIdempotent verifies a second
// StoreParsed for the same (path, content hash) does not replace the
// existing entry — callers can race without losing pointer identity.
func TestWorkspaceState_StoreParsedIdempotent(t *testing.T) {
	w := NewWorkspaceState("")
	content := []byte("class Foo {}\n")
	first, err := ParseSingle(context.Background(), "/tmp/foo.kt", content)
	if err != nil {
		t.Fatalf("ParseSingle: %v", err)
	}
	w.StoreParsed("/tmp/foo.kt", content, first)

	// Second store with same hash must keep the first pointer.
	second, err := ParseSingle(context.Background(), "/tmp/foo.kt", content)
	if err != nil {
		t.Fatalf("ParseSingle: %v", err)
	}
	w.StoreParsed("/tmp/foo.kt", content, second)

	got, _ := w.LookupParsedByPath("/tmp/foo.kt")
	if got != first {
		t.Errorf("StoreParsed replaced pointer for same content hash")
	}
}

// TestWorkspaceState_AndroidProjectCachesByFingerprint verifies the
// memoization contract: same fingerprint reuses the cached pointer
// without firing build; different fingerprint forces a rebuild.
func TestWorkspaceState_AndroidProjectCachesByFingerprint(t *testing.T) {
	w := NewWorkspaceState("")
	var builds int
	first := w.AndroidProject("fp-a", func() *android.Project {
		builds++
		return &android.Project{ManifestPaths: []string{"AndroidManifest.xml"}}
	})
	if builds != 1 {
		t.Fatalf("AndroidProject builds = %d, want 1", builds)
	}
	again := w.AndroidProject("fp-a", func() *android.Project {
		builds++
		return &android.Project{}
	})
	if builds != 1 {
		t.Errorf("AndroidProject builds = %d after hit, want 1", builds)
	}
	if again != first {
		t.Errorf("AndroidProject returned different pointer for same fingerprint")
	}
	_ = w.AndroidProject("fp-b", func() *android.Project { builds++; return &android.Project{} })
	if builds != 2 {
		t.Errorf("AndroidProject builds = %d after fp mismatch, want 2", builds)
	}
}

// TestWorkspaceState_InvalidateLibraryFactsDropsAndroid verifies the
// watcher's InvalidateLibraryFacts hook also drops the AndroidProject
// slot — the two share the build.gradle dependency boundary, so a
// build script edit must invalidate both.
func TestWorkspaceState_InvalidateLibraryFactsDropsAndroid(t *testing.T) {
	w := NewWorkspaceState("")
	w.AndroidProject("fp", func() *android.Project { return &android.Project{} })
	w.LibraryFacts("lf", func() *librarymodel.Facts { return &librarymodel.Facts{} })
	if !w.CrossFileStats().HasLibraryFacts {
		t.Fatal("setup: LibraryFacts slot should be populated")
	}

	w.InvalidateLibraryFacts()

	if w.CrossFileStats().HasLibraryFacts {
		t.Errorf("LibraryFacts not invalidated")
	}
	w.xfileMu.Lock()
	if w.androidProject.present {
		t.Errorf("AndroidProject slot survived InvalidateLibraryFacts")
	}
	w.xfileMu.Unlock()
}

// TestWorkspaceState_GradleFindingsMemoizes verifies the per-gradle-
// file findings cache: same key reuses the cached result without
// firing build; different key triggers build; InvalidateLibraryFacts
// drops the whole map.
func TestWorkspaceState_GradleFindingsMemoizes(t *testing.T) {
	w := NewWorkspaceState("")
	var builds int
	first := w.GradleFindings("k1", func() scanner.FindingColumns {
		builds++
		return scanner.FindingColumns{}
	})
	if builds != 1 {
		t.Fatalf("GradleFindings builds = %d after first call, want 1", builds)
	}
	_ = w.GradleFindings("k1", func() scanner.FindingColumns {
		builds++
		return scanner.FindingColumns{}
	})
	if builds != 1 {
		t.Errorf("GradleFindings builds = %d after hit, want 1", builds)
	}
	_ = w.GradleFindings("k2", func() scanner.FindingColumns {
		builds++
		return scanner.FindingColumns{}
	})
	if builds != 2 {
		t.Errorf("GradleFindings builds = %d after distinct key, want 2", builds)
	}
	_ = first
	w.InvalidateLibraryFacts()
	_ = w.GradleFindings("k1", func() scanner.FindingColumns {
		builds++
		return scanner.FindingColumns{}
	})
	if builds != 3 {
		t.Errorf("GradleFindings builds = %d after InvalidateLibraryFacts, want 3", builds)
	}
}
