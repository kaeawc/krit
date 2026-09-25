package oracle

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestCompilationFingerprint_ChangesWithEveryInput(t *testing.T) {
	dir := t.TempDir()
	a := writeTempFile(t, dir, "A.kt", "package demo\nfun a() = 1\n")
	b := writeTempFile(t, dir, "B.kt", "package demo\nfun b() = 2\n")
	jar := writeTempFile(t, dir, "backend.jar", "jar")
	lib := writeTempFile(t, dir, "lib.jar", "lib")
	fp := func(files, classpath []string) string {
		return CompilationFingerprint(files, classpath, jar)
	}

	base := fp([]string{a, b}, []string{lib})
	if again := fp([]string{b, a}, []string{lib}); again != base {
		t.Fatal("fingerprint must not depend on file order")
	}
	if fp([]string{a}, []string{lib}) == base {
		t.Fatal("removing a file must change the fingerprint")
	}
	if fp([]string{a, b}, nil) == base {
		t.Fatal("dropping a classpath entry must change the fingerprint")
	}
	if err := os.WriteFile(b, []byte("package demo\nfun b() = 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	edited := fp([]string{a, b}, []string{lib})
	if edited == base {
		t.Fatal("editing a file must change the fingerprint")
	}
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(lib, later, later); err != nil {
		t.Fatal(err)
	}
	if fp([]string{a, b}, []string{lib}) == edited {
		t.Fatal("a rewritten classpath jar must change the fingerprint")
	}
	gone := filepath.Join(dir, "Gone.kt")
	first := CompilationFingerprint([]string{a, gone}, nil, jar)
	if second := CompilationFingerprint([]string{a, gone}, nil, jar); first != second {
		t.Fatal("an unreadable source must give a stable fingerprint, not defeat the cache")
	}
	if CompilationFingerprint([]string{a, gone}, nil, jar) == CompilationFingerprint([]string{a}, nil, jar) {
		t.Fatal("an unreadable source must still count as part of the compilation")
	}
}

func TestCompilationSources_IncludesWhatCollectKtFilesPrunes(t *testing.T) {
	dir := t.TempDir()
	src := writeTempFile(t, dir, "Use.kt", "package demo\nfun use() = fixture()\n")
	if err := os.MkdirAll(filepath.Join(dir, "testData"), 0o755); err != nil {
		t.Fatal(err)
	}
	pruned := writeTempFile(t, filepath.Join(dir, "testData"), "Fixture.kt", "package demo\nfun fixture() = 1\n")
	writeTempFile(t, dir, "build.gradle.kts", "plugins {}\n")

	collected, err := CollectKtFiles([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range collected {
		if p == pruned {
			t.Fatal("CollectKtFiles no longer prunes testData/; pick another pruned path for this test")
		}
	}
	// krit-fir compiles every .kt it finds, pruned or not, and no .kts.
	if got, want := CompilationSources([]string{dir}), []string{src, pruned}; !reflect.DeepEqual(got, want) {
		t.Fatalf("CompilationSources = %v, want every .kt krit-fir compiles %v", got, want)
	}
}
