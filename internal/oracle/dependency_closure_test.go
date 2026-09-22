package oracle

import (
	"os"
	"reflect"
	"testing"
)

func TestOracleCacheClosure_TransitiveDependencyChangeMisses(t *testing.T) {
	dir := t.TempDir()
	cacheDir, err := CacheDir(dir)
	if err != nil {
		t.Fatalf("CacheDir: %v", err)
	}
	e := writeTempFile(t, dir, "E.kt", "package demo\nfun e() = \"value\"\n")
	d := writeTempFile(t, dir, "D.kt", "package demo\nfun d() = e()\n")
	f := writeTempFile(t, dir, "F.kt", "package demo\nfun f() = d().length\n")
	fresh := &Data{Version: 1, Files: map[string]*File{
		e: {Package: "demo"},
		d: {Package: "demo"},
		f: {Package: "demo"},
	}}
	deps := &CacheDepsFile{Version: 1, Files: map[string]*CacheDepsEntry{
		e: {DepPaths: nil},
		d: {DepPaths: []string{e}},
		f: {DepPaths: []string{d}},
	}}
	if _, err := WriteFreshEntries(cacheDir, fresh, deps); err != nil {
		t.Fatalf("WriteFreshEntries: %v", err)
	}

	fHash, err := ContentHash(f)
	if err != nil {
		t.Fatalf("ContentHash(F): %v", err)
	}
	fEntry, err := LoadEntry(cacheDir, fHash)
	if err != nil || fEntry == nil {
		t.Fatalf("LoadEntry(F): entry=%v err=%v", fEntry, err)
	}
	if got, want := fEntry.Closure.DepPaths, []string{d, e}; !reflect.DeepEqual(got, want) {
		t.Fatalf("F closure = %v, want transitive %v", got, want)
	}

	if err := os.WriteFile(e, []byte("package demo\nfun e() = null\n"), 0o644); err != nil {
		t.Fatalf("rewrite E: %v", err)
	}
	hits, misses := ClassifyFiles(cacheDir, []string{f})
	if len(hits) != 0 || !reflect.DeepEqual(misses, []string{f}) {
		t.Fatalf("E semantic change must invalidate F through D: hits=%d misses=%v", len(hits), misses)
	}
}

func TestOracleCacheClosure_PartialWriteUsesPersistedDependencyClosure(t *testing.T) {
	dir := t.TempDir()
	cacheDir, err := CacheDir(dir)
	if err != nil {
		t.Fatalf("CacheDir: %v", err)
	}
	e := writeTempFile(t, dir, "E.kt", "package demo\nfun e() = \"value\"\n")
	d := writeTempFile(t, dir, "D.kt", "package demo\nfun d() = e()\n")
	f := writeTempFile(t, dir, "F.kt", "package demo\nfun f() = d().length\n")
	if _, err := WriteFreshEntries(cacheDir,
		&Data{Version: 1, Files: map[string]*File{e: {}, d: {}}},
		&CacheDepsFile{Version: 1, Files: map[string]*CacheDepsEntry{
			e: {DepPaths: nil},
			d: {DepPaths: []string{e}},
		}},
	); err != nil {
		t.Fatalf("write E/D: %v", err)
	}
	// Simulate a later partial analysis whose dependency report includes F
	// but not the unchanged D entry. D's persisted closure must supply E.
	if _, err := WriteFreshEntries(cacheDir,
		&Data{Version: 1, Files: map[string]*File{f: {}}},
		&CacheDepsFile{Version: 1, Files: map[string]*CacheDepsEntry{
			f: {DepPaths: []string{d}},
		}},
	); err != nil {
		t.Fatalf("write F: %v", err)
	}
	fHash, err := ContentHash(f)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := LoadEntry(cacheDir, fHash)
	if err != nil || entry == nil {
		t.Fatalf("LoadEntry(F): entry=%v err=%v", entry, err)
	}
	if got, want := entry.Closure.DepPaths, []string{d, e}; !reflect.DeepEqual(got, want) {
		t.Fatalf("partial F closure = %v, want persisted transitive %v", got, want)
	}
}
