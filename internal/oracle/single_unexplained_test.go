package oracle

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/perf"
)

// krit-types answers for every in-root file within the size limit, so a lone
// unanswered one is unexplained: it must not be cached as skipped, or later
// runs would serve the marker as a hit and drop the file.
func TestWriteSkippedPoisonEntries_LoneUnexplainedFileIsNotMarked(t *testing.T) {
	tmp := t.TempDir()
	cacheDir, err := CacheDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	a := writeTempFile(t, tmp, "A.kt", "package app\n")
	warnings := captureWarnings(t)

	writeSkippedPoisonEntries(nil, cacheDir, []string{tmp}, []string{a}, &Data{Files: map[string]*File{}}, &CacheDepsFile{}, "", "", perf.New(false), false)

	if hits, misses := ClassifyFiles(cacheDir, []string{a}); len(hits) != 0 || len(misses) != 1 {
		t.Fatalf("lone unanswered in-root file must stay a miss: hits=%d misses=%v", len(hits), misses)
	}
	if !strings.Contains(warnings.String(), "0 of 1") {
		t.Fatalf("expected a mismatch warning, got %q", warnings.String())
	}
}

// Facts are keyed by clean paths; a spelling such as ./src/A.kt (a single-file
// scan argument) must reach the facts stored under src/A.kt.
func TestOracleLookups_UncleanRelativeSpellingReachesCleanKeys(t *testing.T) {
	dir := chdirProject(t, map[string]string{"src/A.kt": "class A\n"})
	_ = dir
	rel := filepath.Join("src", "A.kt")
	o, err := LoadFromData(&Data{Version: 1, Files: map[string]*File{rel: {Package: "p"}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, spelling := range []string{rel, "./" + filepath.ToSlash(rel), filepath.Join("src", "..", "src", "A.kt")} {
		if o.fileKey(spelling) != rel {
			t.Errorf("fileKey(%q) = %q, want %q", spelling, o.fileKey(spelling), rel)
		}
	}
	if KeyBaseFor([]string{"src"}) != KeyBaseFor([]string{"./src"}) {
		t.Error("KeyBaseFor must not depend on the spelling of a relative scan path")
	}
}
