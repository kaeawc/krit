package oracle

import (
	"testing"

	"github.com/kaeawc/krit/internal/store"
)

// removedKAAApproximation is the approximation the removed krit-types backend
// stamped on its entries. Such entries may still sit in a user's cache.
const removedKAAApproximation = "kaa-tagged-references"

// An entry written by the removed krit-types backend (or any non-FIR
// approximation) must never satisfy a krit-fir lookup, in either cache layout.
func TestClassifyFilesWithApproximation_ForeignApproximationEntryMisses(t *testing.T) {
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
			for _, written := range []string{removedKAAApproximation, ApproximationFIRWholeCompilation} {
				deps := &CacheDepsFile{Version: 1, Approximation: written, Files: map[string]*CacheDepsEntry{path: {}}}
				if _, err := WriteFreshEntriesToStore(fileStore, cacheDir, &Data{Version: 1, Files: map[string]*File{path: {}}}, deps); err != nil {
					t.Fatal(err)
				}
				hits, misses := ClassifyFilesWithStoreApproximation(fileStore, cacheDir, []string{path}, BackendFIR.CacheApproximation())
				if same := written == ApproximationFIRWholeCompilation; (len(hits) == 1) != same || (len(misses) == 1) == same {
					t.Fatalf("written %q: hits=%v misses=%v", written, hitPaths(hits), misses)
				}
			}
		})
	}
}

func TestStampCacheDeps_FIRReport(t *testing.T) {
	fir := &CacheDepsFile{Approximation: removedKAAApproximation}
	stampCacheDeps(fir, InvocationOptions{Backend: BackendFIR}, "compilation")
	if fir.Approximation != ApproximationFIRWholeCompilation || fir.CompilationFingerprint != "compilation" {
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
