package scanner

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestLoadXMLFilesForCache_StableOrder asserts the slice returned
// from `loadXMLFilesForCache` is in canonical (path-sorted) order
// regardless of which goroutine finished its per-root walk first.
// Regression for #31: the goroutines populated `out` under a mutex
// but in completion order, leaving downstream consumers with a
// shuffled XML file sequence each run.
func TestLoadXMLFilesForCache_StableOrder(t *testing.T) {
	tmp := t.TempDir()

	// Build three independent module-style roots, each with a few
	// reference-candidate XML files at different depths. Goroutine
	// completion order across these is unpredictable — by sorting at
	// the merge seam we get a deterministic returned slice.
	moduleNames := []string{"alpha", "beta", "gamma"}
	for _, m := range moduleNames {
		for _, rel := range []string{
			"src/main/AndroidManifest.xml",
			"src/main/res/layout/main.xml",
			"src/main/res/navigation/nav.xml",
		} {
			full := filepath.Join(tmp, m, rel)
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, []byte(`<root/>`), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}

	// Build ktFiles whose dirname-up-to-`src` discovers each module
	// root. xmlRootsFromKotlinFiles requires "src" to appear in the
	// directory chain.
	var ktFiles []*File
	for _, m := range moduleNames {
		ktPath := filepath.Join(tmp, m, "src", "main", "kotlin", "Foo.kt")
		if err := os.MkdirAll(filepath.Dir(ktPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ktPath, []byte("package x"), 0o644); err != nil {
			t.Fatal(err)
		}
		ktFiles = append(ktFiles, &File{Path: ktPath})
	}

	first := pathsOf(loadXMLFilesForCache(ktFiles))
	if len(first) == 0 {
		t.Fatalf("no xml files discovered")
	}

	// Iterate many times to amplify scheduler-induced order variance.
	for i := 0; i < 100; i++ {
		got := pathsOf(loadXMLFilesForCache(ktFiles))
		if !reflect.DeepEqual(got, first) {
			t.Fatalf("iter %d: order differs\n  first: %v\n  got:   %v", i, first, got)
		}
	}

	// Independent witness: the canonical order must be lexicographic
	// by path (test-only check that asserts the *meaning* of stable,
	// not just the property).
	for k := 1; k < len(first); k++ {
		if first[k-1] >= first[k] {
			t.Fatalf("not sorted: %q >= %q at index %d", first[k-1], first[k], k)
		}
	}
}

// TestSortXMLCacheFiles_TotalOrder pins the comparator's behavior
// for tests and future call sites: ascending path lex order, no
// secondary key needed because Path is unique within a project.
func TestSortXMLCacheFiles_TotalOrder(t *testing.T) {
	in := []*xmlCacheFile{
		{Path: "/c.xml"}, {Path: "/a.xml"}, {Path: "/b.xml"},
	}
	want := []string{"/a.xml", "/b.xml", "/c.xml"}
	sortXMLCacheFiles(in)
	got := pathsOf(in)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func pathsOf(files []*xmlCacheFile) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.Path
	}
	return out
}
