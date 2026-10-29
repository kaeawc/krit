package oracle

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/kaeawc/krit/internal/gradlemodel"
	"github.com/kaeawc/krit/internal/store"
)

// TestExpandStaleOraclePaths_PullsInReverseDependents verifies that a changed
// file drags every file whose stored dependency closure contains it into the
// stale set, so the daemon re-analyzes dependents rather than merging their
// stale facts.
func TestExpandStaleOraclePaths_PullsInReverseDependents(t *testing.T) {
	dir := t.TempDir()
	cacheDir, err := CacheDir(dir)
	if err != nil {
		t.Fatalf("CacheDir: %v", err)
	}
	a := writeTempFile(t, dir, "A.kt", "package demo\nfun a() = \"value\"\n")
	b := writeTempFile(t, dir, "B.kt", "package demo\nfun b() = a()\n")
	if _, err := WriteFreshEntries(cacheDir,
		&Data{Version: 1, Files: map[string]*File{a: {}, b: {}}},
		&CacheDepsFile{Version: 1, Files: map[string]*CacheDepsEntry{
			a: {DepPaths: nil},
			b: {DepPaths: []string{a}},
		}},
	); err != nil {
		t.Fatalf("WriteFreshEntries: %v", err)
	}

	got := ExpandStaleOraclePaths(nil, cacheDir, []string{a, b}, []string{a})
	if want := []string{a, b}; !reflect.DeepEqual(sortedCopy(got), want) {
		t.Fatalf("A changed must pull in dependent B: got %v, want %v", got, want)
	}
}

// TestExpandStaleOraclePaths_SeedsUnreadableEntry pins the review fix: a file
// whose cache entry cannot be loaded contributes no reverse edge, so a change
// to something it depends on could not otherwise reach it. It must be seeded
// into the work set directly so its stale facts are recomputed rather than
// silently merged forward.
func TestExpandStaleOraclePaths_SeedsUnreadableEntry(t *testing.T) {
	dir := t.TempDir()
	cacheDir, err := CacheDir(dir)
	if err != nil {
		t.Fatalf("CacheDir: %v", err)
	}
	a := writeTempFile(t, dir, "A.kt", "package demo\nfun a() = \"value\"\n")
	// B exists on disk and depends on A, but has no cache entry (never
	// written / evicted / corrupted), so its reverse edge to A is unknown.
	b := writeTempFile(t, dir, "B.kt", "package demo\nfun b() = a()\n")
	if _, err := WriteFreshEntries(cacheDir,
		&Data{Version: 1, Files: map[string]*File{a: {}}},
		&CacheDepsFile{Version: 1, Files: map[string]*CacheDepsEntry{a: {DepPaths: nil}}},
	); err != nil {
		t.Fatalf("WriteFreshEntries: %v", err)
	}

	got := ExpandStaleOraclePaths(nil, cacheDir, []string{a, b}, []string{a})
	if want := []string{a, b}; !reflect.DeepEqual(sortedCopy(got), want) {
		t.Fatalf("unreadable-entry file B must be seeded: got %v, want %v", got, want)
	}
}

// TestExpandStaleOraclePaths_TwoFileCycle verifies the BFS terminates and
// returns both members when two files depend on each other.
func TestExpandStaleOraclePaths_TwoFileCycle(t *testing.T) {
	dir := t.TempDir()
	cacheDir, err := CacheDir(dir)
	if err != nil {
		t.Fatalf("CacheDir: %v", err)
	}
	a := writeTempFile(t, dir, "A.kt", "package demo\nfun a() = b()\n")
	b := writeTempFile(t, dir, "B.kt", "package demo\nfun b() = a()\n")
	if _, err := WriteFreshEntries(cacheDir,
		&Data{Version: 1, Files: map[string]*File{a: {}, b: {}}},
		&CacheDepsFile{Version: 1, Files: map[string]*CacheDepsEntry{
			a: {DepPaths: []string{b}},
			b: {DepPaths: []string{a}},
		}},
	); err != nil {
		t.Fatalf("WriteFreshEntries: %v", err)
	}

	got := ExpandStaleOraclePaths(nil, cacheDir, []string{a, b}, []string{a})
	if want := []string{a, b}; !reflect.DeepEqual(sortedCopy(got), want) {
		t.Fatalf("mutually dependent files: got %v, want %v", got, want)
	}
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

// The freshness gate's expansion must read exactly the entries the oracle
// invocation writes. With Kotlin settings those live in the settings-scoped
// legacy directory; reading the unscoped one finds nothing, seeds every file as
// unresolved and forces a full re-analysis on every incremental run.
func TestExpandStaleOraclePaths_ReadsTheSettingsScopedLegacyDir(t *testing.T) {
	dir := t.TempDir()
	cacheDir, err := CacheDir(dir)
	if err != nil {
		t.Fatalf("CacheDir: %v", err)
	}
	kt := gradlemodel.KotlinSettings{LanguageVersion: "2.1", APIVersion: "2.1"}
	a := writeTempFile(t, dir, "A.kt", "package demo\nfun a() = 1\n")
	b := writeTempFile(t, dir, "B.kt", "package demo\nfun b() = a()\n")
	c := writeTempFile(t, dir, "C.kt", "package demo\nfun c() = 3\n")
	// Written where InvokeCached writes for these settings.
	if _, err := WriteFreshEntries(prepareLegacyEntryDir(cacheDir, kt, nil),
		&Data{Version: 1, Files: map[string]*File{a: {}, b: {}, c: {}}},
		&CacheDepsFile{Version: 1, Files: map[string]*CacheDepsEntry{
			a: {DepPaths: nil}, b: {DepPaths: []string{a}}, c: {DepPaths: nil},
		}},
	); err != nil {
		t.Fatalf("WriteFreshEntries: %v", err)
	}
	got := ExpandStaleOraclePaths(nil, cacheDir, []string{a, b, c}, []string{a}, ExpansionScope{Kotlin: kt})
	if want := []string{a, b}; !reflect.DeepEqual(sortedCopy(got), want) {
		t.Fatalf("expansion must find B as A's dependent without seeding C: got %v, want %v", got, want)
	}
	// Another scope's entries are not this run's: every file is unresolved.
	other := ExpandStaleOraclePaths(nil, cacheDir, []string{a, b, c}, []string{a})
	if len(other) != 3 {
		t.Fatalf("unscoped expansion must not see scoped entries: %v", other)
	}
}

// The same for the store layout: expansion reads under the run's full scope
// (jar, classpath, JVM target, settings).
func TestExpandStaleOraclePaths_ReadsTheRunsStoreScope(t *testing.T) {
	dir := t.TempDir()
	jar := filepath.Join(dir, "krit-fir.jar")
	if err := os.WriteFile(jar, []byte("jar"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KRIT_FIR_JAR", jar)
	cacheDir, err := CacheDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := store.New(filepath.Join(dir, "store"))
	kt := gradlemodel.KotlinSettings{LanguageVersion: "2.1", APIVersion: "2.1"}
	classpath := []string{jar}
	es := ExpansionScope{Kotlin: kt, Classpath: classpath}
	a := writeTempFile(t, dir, "A.kt", "package demo\nfun a() = 1\n")
	b := writeTempFile(t, dir, "B.kt", "package demo\nfun b() = a()\n")
	c := writeTempFile(t, dir, "C.kt", "package demo\nfun c() = 3\n")
	release := BindStoreScope(s, NewStoreScopeWithSettings(BackendFIR, jar, classpath, "", kt))
	for path, deps := range map[string][]string{a: nil, b: {a}, c: nil} {
		hash, _ := ContentHash(path)
		fp, _ := closureFingerprint(deps, nil)
		if err := WriteEntryToStore(s, &CacheEntry{
			ContentHash: hash, FilePath: path, FileResult: &File{},
			Closure: CacheClosure{DepPaths: deps, Fingerprint: fp}, Approximation: ApproximationFIRWholeCompilation,
		}); err != nil {
			t.Fatal(err)
		}
	}
	release()
	got := ExpandStaleOraclePaths(s, cacheDir, []string{a, b, c}, []string{a}, es)
	if want := []string{a, b}; !reflect.DeepEqual(sortedCopy(got), want) {
		t.Fatalf("store expansion under the run's scope: got %v, want %v", got, want)
	}
	// Without the run's classpath/settings the scope differs and nothing resolves.
	if len(ExpandStaleOraclePaths(s, cacheDir, []string{a, b, c}, []string{a})) != 3 {
		t.Fatal("expansion under another scope must not read these entries")
	}
}

// Call-site test: InvokeCachedWithOptions must use LegacyEntryDir. With every
// file's entry present only in the settings-scoped directory, the run is served
// entirely from cache; reading the unscoped directory would miss and need a JVM.
func TestInvokeCachedUsesTheSettingsScopedLegacyDir(t *testing.T) {
	repo := t.TempDir()
	srcDir := filepath.Join(repo, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	a := writeTempFile(t, srcDir, "A.kt", "package demo\nfun a() = 1\n")
	cacheDir, err := CacheDir(repo)
	if err != nil {
		t.Fatal(err)
	}
	kt := gradlemodel.KotlinSettings{LanguageVersion: "2.1", APIVersion: "2.1"}
	opts := InvocationOptions{Backend: BackendFIR, Kotlin: kt}
	// Stamped with the compilation the run will compute, so a hit needs no
	// whole-compilation refresh.
	compilation := wholeCompilationFingerprint(opts, []string{srcDir}, "", opts.tracker())
	if _, err := WriteFreshEntries(prepareLegacyEntryDir(cacheDir, kt, nil),
		&Data{Version: 1, Files: map[string]*File{a: {}}},
		&CacheDepsFile{Version: 1, Approximation: ApproximationFIRWholeCompilation, CompilationFingerprint: compilation, Files: map[string]*CacheDepsEntry{a: {DepPaths: nil}}},
	); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(repo, "types.json")
	if _, err := InvokeCachedWithOptions("", []string{srcDir}, repo, out, "", false, nil, opts); err != nil {
		t.Fatalf("a run whose entries are all in the scoped dir must be served from cache: %v", err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("no output assembled from the scoped entries: %v", err)
	}
	// Without settings the same repo has no entries in the base dir: it must try the JVM.
	if _, err := InvokeCachedWithOptions("", []string{srcDir}, repo, filepath.Join(repo, "other.json"), "", false, nil, InvocationOptions{Backend: BackendFIR}); err == nil {
		t.Fatal("settings-free run found the settings-scoped entries")
	}
}
