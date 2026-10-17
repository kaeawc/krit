package oracle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/perf"
)

// writeOversized writes a source file just over the Analysis API size limit.
func writeOversized(t *testing.T, dir, name string) string {
	t.Helper()
	p := writeTempFile(t, dir, name, "class Big\n")
	if err := os.Truncate(p, maxJVMSourceBytes+1); err != nil {
		t.Fatal(err)
	}
	return p
}

// Two oversized sources the JVM never answers for are an expected skip, not a
// request mismatch: both keep their markers, so warm runs do not relaunch the
// JVM, and no warning is printed.
func TestWriteSkippedPoisonEntries_OversizedMissesAlwaysMarked(t *testing.T) {
	tmp := t.TempDir()
	cacheDir, err := CacheDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	a := writeOversized(t, tmp, "A.kt")
	b := writeOversized(t, tmp, "B.kt")
	warnings := captureWarnings(t)

	writeSkippedPoisonEntries(nil, cacheDir, []string{tmp}, []string{a, b}, &Data{Files: map[string]*File{}}, &CacheDepsFile{}, "", "", perf.New(false), false)

	hits, misses := ClassifyFiles(cacheDir, []string{a, b})
	if len(hits) != 2 || len(misses) != 0 {
		t.Fatalf("oversized misses must be marked: hits=%d misses=%v", len(hits), misses)
	}
	if warnings.Len() != 0 {
		t.Fatalf("unexpected warning: %q", warnings.String())
	}
}

// With a mismatch among ordinary files, an oversized file in the same batch
// still gets its marker while the unexplained files do not.
func TestWriteSkippedPoisonEntries_MismatchStillMarksOversized(t *testing.T) {
	tmp := t.TempDir()
	cacheDir, err := CacheDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	small1 := writeTempFile(t, tmp, "S1.kt", "class S1\n")
	small2 := writeTempFile(t, tmp, "S2.kt", "class S2\n")
	big := writeOversized(t, tmp, "Big.kt")
	warnings := captureWarnings(t)

	writeSkippedPoisonEntries(nil, cacheDir, []string{tmp}, []string{small1, small2, big}, &Data{Files: map[string]*File{}}, &CacheDepsFile{}, "", "", perf.New(false), false)

	hits, misses := ClassifyFiles(cacheDir, []string{small1, small2, big})
	if len(hits) != 1 || hits[0].FilePath != big || len(misses) != 2 {
		t.Fatalf("want only the oversized file marked: hits=%v misses=%v", hits, misses)
	}
	if !strings.Contains(warnings.String(), "0 of 3") {
		t.Fatalf("expected a mismatch warning, got %q", warnings.String())
	}
}

// Files outside every source root are never in the JVM's source module, so
// leaving them unanswered is expected too.
func TestWriteSkippedPoisonEntries_FilesOutsideSourceRootsAreExpected(t *testing.T) {
	tmp := t.TempDir()
	cacheDir, err := CacheDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(tmp, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	a := writeTempFile(t, tmp, "A.kt", "class A\n")
	b := writeTempFile(t, tmp, "B.kt", "class B\n")
	captureWarnings(t)

	writeSkippedPoisonEntries(nil, cacheDir, []string{root}, []string{a, b}, &Data{Files: map[string]*File{}}, &CacheDepsFile{}, "", "", perf.New(false), false)

	if hits, _ := ClassifyFiles(cacheDir, []string{a, b}); len(hits) != 2 {
		t.Fatalf("files outside the source roots should be marked, hits=%d", len(hits))
	}
}

// The forced-miss hint and the scan can spell one file differently; the
// match is by absolute path and the result keeps the scan's spelling.
func TestSplitForcedMisses_MatchesAcrossSpellings(t *testing.T) {
	chdirProject(t, map[string]string{"x.kt": "class X\n", "y.kt": "class Y\n"})
	classify, forced := splitForcedMisses([]string{"x.kt", "y.kt"}, []string{"./x.kt"})
	if len(forced) != 1 || forced[0] != "x.kt" {
		t.Fatalf("forced = %v, want [x.kt] (the scan's spelling)", forced)
	}
	if len(classify) != 1 || classify[0] != "y.kt" {
		t.Fatalf("classify = %v, want [y.kt]", classify)
	}
	abs, _ := filepath.Abs("y.kt")
	_, forced = splitForcedMisses([]string{"x.kt", "y.kt"}, []string{abs})
	if len(forced) != 1 || forced[0] != "y.kt" {
		t.Fatalf("absolute hint: forced = %v, want [y.kt]", forced)
	}
}

// krit-types compares --files entries by exact equality with its virtual
// file paths, which use forward slashes on every platform.
func TestMissListLines_AbsoluteForwardSlash(t *testing.T) {
	dir := chdirProject(t, map[string]string{"src/A.kt": "class A\n"})
	got := missListLines([]string{filepath.Join("src", "A.kt")})
	want := filepath.ToSlash(filepath.Join(dir, "src", "A.kt"))
	if len(got) != 1 || got[0] != want {
		t.Fatalf("missListLines = %v, want [%q]", got, want)
	}
	if filepath.Separator != '/' && strings.ContainsRune(got[0], filepath.Separator) {
		t.Fatalf("%q still contains native separators", got[0])
	}
}
