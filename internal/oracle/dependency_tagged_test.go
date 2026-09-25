package oracle

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/kaeawc/krit/internal/store"
)

// taggedDeps builds a krit-types tagged dependency report: every edge in all,
// with the propagating subset in prop.
func taggedDeps(all map[string][]string, prop map[string][]string) *CacheDepsFile {
	files := map[string]*CacheDepsEntry{}
	for path, deps := range all {
		files[path] = &CacheDepsEntry{DepPaths: deps, PropagatingDepPaths: prop[path]}
	}
	return &CacheDepsFile{Version: 1, Approximation: ApproximationKAATaggedReferences, Files: files}
}

func loadClosure(t *testing.T, cacheDir, path string) CacheClosure {
	t.Helper()
	hash, err := ContentHash(path)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := LoadEntry(cacheDir, hash)
	if err != nil || entry == nil {
		t.Fatalf("LoadEntry(%s): entry=%v err=%v", path, entry, err)
	}
	return entry.Closure
}

func TestOracleCacheClosure_TaggedClosureStopsAtBodyOnlyEdges(t *testing.T) {
	dir := t.TempDir()
	cacheDir, err := CacheDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	// D's inferred signature depends on E; only D's explicitly typed body
	// uses G. F calls D.
	e := writeTempFile(t, dir, "E.kt", "package demo\nfun e() = 1\n")
	g := writeTempFile(t, dir, "G.kt", "package demo\nfun g() = 2\n")
	d := writeTempFile(t, dir, "D.kt", "package demo\nfun d() = e()\nfun h(): Int { return g() }\n")
	f := writeTempFile(t, dir, "F.kt", "package demo\nfun f() = d()\n")
	deps := taggedDeps(
		map[string][]string{e: nil, g: nil, d: {e, g}, f: {d}},
		map[string][]string{d: {e}, f: {d}},
	)
	fresh := &Data{Version: 1, Files: map[string]*File{e: {}, g: {}, d: {}, f: {}}}
	if _, err := WriteFreshEntries(cacheDir, fresh, deps); err != nil {
		t.Fatal(err)
	}
	if got, want := loadClosure(t, cacheDir, f).DepPaths, []string{d, e}; !reflect.DeepEqual(got, want) {
		t.Fatalf("F closure = %v, want %v (G is only in D's body)", got, want)
	}
	if got, want := loadClosure(t, cacheDir, d), (CacheClosure{DepPaths: []string{e, g}, PropagatingDepPaths: []string{e}}); !reflect.DeepEqual(got.DepPaths, want.DepPaths) || !reflect.DeepEqual(got.PropagatingDepPaths, want.PropagatingDepPaths) {
		t.Fatalf("D closure = %+v, want direct %v and propagating %v", got, want.DepPaths, want.PropagatingDepPaths)
	}

	if err := os.WriteFile(g, []byte("package demo\nfun g() = 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hits, misses := ClassifyFiles(cacheDir, []string{d, f})
	if len(hits) != 1 || hits[0].FilePath != f || !reflect.DeepEqual(misses, []string{d}) {
		t.Fatalf("G edit must invalidate D only: hits=%v misses=%v", hitPaths(hits), misses)
	}
	if err := os.WriteFile(e, []byte("package demo\nfun e() = \"s\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if hits, misses := ClassifyFiles(cacheDir, []string{f}); len(hits) != 0 || !reflect.DeepEqual(misses, []string{f}) {
		t.Fatalf("E edit must invalidate F through D's signature: hits=%v misses=%v", hitPaths(hits), misses)
	}
}

func TestOracleCacheClosure_PartialTaggedWriteContinuesThroughPersistedPropagatingEdges(t *testing.T) {
	dir := t.TempDir()
	cacheDir, err := CacheDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	e := writeTempFile(t, dir, "E.kt", "package demo\nfun e() = 1\n")
	g := writeTempFile(t, dir, "G.kt", "package demo\nfun g() = 2\n")
	d := writeTempFile(t, dir, "D.kt", "package demo\nfun d() = e()\nfun h(): Int { return g() }\n")
	f := writeTempFile(t, dir, "F.kt", "package demo\nfun f() = d()\n")
	if _, err := WriteFreshEntries(cacheDir,
		&Data{Version: 1, Files: map[string]*File{e: {}, g: {}, d: {}}},
		taggedDeps(map[string][]string{e: nil, g: nil, d: {e, g}}, map[string][]string{d: {e}}),
	); err != nil {
		t.Fatal(err)
	}
	// A later analysis reports only F. D's persisted propagating edges, not
	// its whole closure, must continue the walk.
	if _, err := WriteFreshEntries(cacheDir,
		&Data{Version: 1, Files: map[string]*File{f: {}}},
		taggedDeps(map[string][]string{f: {d}}, map[string][]string{f: {d}}),
	); err != nil {
		t.Fatal(err)
	}
	if got, want := loadClosure(t, cacheDir, f).DepPaths, []string{d, e}; !reflect.DeepEqual(got, want) {
		t.Fatalf("partial F closure = %v, want %v", got, want)
	}
}

func TestOracleCacheClosure_EntryOverTheClosureCapIsNotWritten(t *testing.T) {
	dir := t.TempDir()
	cacheDir, err := CacheDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	deps := make([]string, 0, maxPersistedClosure+1)
	for i := 0; i <= maxPersistedClosure; i++ {
		deps = append(deps, writeTempFile(t, dir, fmt.Sprintf("Dep%d.kt", i), fmt.Sprintf("package demo\nval v%d = %d\n", i, i)))
	}
	owner := writeTempFile(t, dir, "Owner.kt", "package demo\nfun owner() = 0\n")
	small := writeTempFile(t, dir, "Small.kt", "package demo\nfun small() = 0\n")
	if _, err := WriteFreshEntries(cacheDir,
		&Data{Version: 1, Files: map[string]*File{owner: {}, small: {}}},
		taggedDeps(map[string][]string{owner: deps, small: deps[:1]}, nil),
	); err != nil {
		t.Fatal(err)
	}
	hits, misses := ClassifyFiles(cacheDir, []string{owner, small})
	if len(hits) != 1 || hits[0].FilePath != small || !reflect.DeepEqual(misses, []string{owner}) {
		t.Fatalf("over-cap owner must not be cached: hits=%v misses=%v", hitPaths(hits), misses)
	}

	// krit-fir entries are refreshed through the compilation fingerprint;
	// the cap does not apply to them.
	fir := &CacheDepsFile{Version: 1, Approximation: ApproximationFIRWholeCompilation, Files: map[string]*CacheDepsEntry{owner: {DepPaths: deps}}}
	if _, err := WriteFreshEntries(cacheDir, &Data{Version: 1, Files: map[string]*File{owner: {}}}, fir); err != nil {
		t.Fatal(err)
	}
	if hits, _ := ClassifyFilesScopedV3(cacheDir, []string{owner}, "", "", ApproximationFIRWholeCompilation); len(hits) != 1 {
		t.Fatalf("krit-fir entry over the cap must still be cached: hits=%v", hitPaths(hits))
	}
}

func TestClassifyFilesScopedV3_OtherBackendsEntryMisses(t *testing.T) {
	dir := t.TempDir()
	cacheDir, err := CacheDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := writeTempFile(t, dir, "A.kt", "package demo\nfun a() = 1\n")
	if _, err := WriteFreshEntries(cacheDir,
		&Data{Version: 1, Files: map[string]*File{a: {}}},
		&CacheDepsFile{Version: 1, Approximation: ApproximationSymbolResolvedSources, Files: map[string]*CacheDepsEntry{a: {}}},
	); err != nil {
		t.Fatal(err)
	}
	if hits, misses := ClassifyFilesScopedV3(cacheDir, []string{a}, "", "", BackendKAA.CacheApproximation()); len(hits) != 0 || !reflect.DeepEqual(misses, []string{a}) {
		t.Fatalf("old untagged KAA entry must miss new tagged KAA lookup: hits=%v misses=%v", hitPaths(hits), misses)
	}
	if hits, misses := ClassifyFilesScopedV3(cacheDir, []string{a}, "", "", BackendFIR.CacheApproximation()); len(hits) != 0 || !reflect.DeepEqual(misses, []string{a}) {
		t.Fatalf("krit-types entry must not satisfy a krit-fir lookup: hits=%v misses=%v", hitPaths(hits), misses)
	}
}

func TestKAATaggedApproximationInvalidatesBothCacheLayouts(t *testing.T) {
	for _, layout := range []string{"files", "store"} {
		t.Run(layout, func(t *testing.T) {
			dir := t.TempDir()
			cacheDir, err := CacheDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			var fileStore *store.FileStore
			if layout == "store" {
				fileStore = store.New(dir + "/store")
			}
			path := writeTempFile(t, dir, "A.kt", "package demo\nfun a() = 1\n")
			for _, written := range []string{ApproximationSymbolResolvedSources, ApproximationKAATaggedReferences} {
				deps := &CacheDepsFile{Version: 1, Approximation: written, Files: map[string]*CacheDepsEntry{path: {}}}
				if _, err := WriteFreshEntriesToStore(fileStore, cacheDir, &Data{Version: 1, Files: map[string]*File{path: {}}}, deps); err != nil {
					t.Fatal(err)
				}
				for _, requested := range []string{ApproximationSymbolResolvedSources, ApproximationKAATaggedReferences} {
					hits, misses := ClassifyFilesWithStoreScopedV3(fileStore, cacheDir, []string{path}, "", "", requested)
					if same := written == requested; (len(hits) == 1) != same || (len(misses) == 1) == same {
						t.Fatalf("written %q requested %q: hits=%v misses=%v", written, requested, hitPaths(hits), misses)
					}
				}
			}
		})
	}
}

// A krit-types jar built before tagged edges reports no propagating edges;
// stamping its report as tagged would cut every closure at the direct edges.
func TestStampCacheDeps_UntaggedKAAReportStaysLegacy(t *testing.T) {
	legacy := &CacheDepsFile{Approximation: ApproximationSymbolResolvedSources}
	stampCacheDeps(legacy, InvocationOptions{Backend: BackendKAA}, "")
	if legacy.Approximation != ApproximationSymbolResolvedSources || legacy.tagged() {
		t.Fatalf("untagged krit-types report stamped %q", legacy.Approximation)
	}
	tagged := &CacheDepsFile{Approximation: ApproximationKAATaggedReferences}
	stampCacheDeps(tagged, InvocationOptions{Backend: BackendKAA}, "")
	if !tagged.tagged() {
		t.Fatalf("tagged krit-types report stamped %q", tagged.Approximation)
	}
	fir := &CacheDepsFile{Approximation: ApproximationKAATaggedReferences}
	stampCacheDeps(fir, InvocationOptions{Backend: BackendFIR}, "compilation")
	if fir.Approximation != ApproximationFIRWholeCompilation || fir.CompilationFingerprint != "compilation" || fir.tagged() {
		t.Fatalf("krit-fir report stamped %q/%q", fir.Approximation, fir.CompilationFingerprint)
	}
}

func hitPaths(hits []*CacheEntry) []string {
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.FilePath)
	}
	return out
}
