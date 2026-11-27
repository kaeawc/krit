package scanner

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func writeXMLRootsFixture(t *testing.T, files map[string]string) (string, []*File) {
	t.Helper()
	root := t.TempDir()
	var kt []*File
	for rel, body := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if filepath.Ext(path) == ".kt" {
			kt = append(kt, &File{Path: path})
		}
	}
	return root, kt
}

func xmlRelPaths(t *testing.T, root string, files []*XMLCacheFile) []string {
	t.Helper()
	var out []string
	for _, f := range files {
		rel, err := filepath.Rel(root, f.Path)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, filepath.ToSlash(rel))
	}
	return out
}

// A module nested in another module's directory (app/benchmarks inside app)
// is a root of its own, so the outer walk must leave it to that walk. Read
// twice, its XML was recorded twice in the cross-file meta, which then never
// covered the warm run's list, so warm cross findings were never reused on
// such projects (#1917).
func TestLoadXMLFilesForCacheReadsNestedModuleOnce(t *testing.T) {
	root, kt := writeXMLRootsFixture(t, map[string]string{
		"app/src/main/kotlin/A.kt":                         "package a\n",
		"app/src/main/res/layout/a.xml":                    "<LinearLayout/>\n",
		"app/benchmarks/src/main/kotlin/B.kt":              "package b\n",
		"app/benchmarks/src/main/res/layout/b.xml":         "<FrameLayout/>\n",
		"app/benchmarks/deep/src/main/kotlin/C.kt":         "package c\n",
		"app/benchmarks/deep/src/main/AndroidManifest.xml": "<manifest/>\n",
	})
	files := LoadXMLFilesForCache(kt)
	got := xmlRelPaths(t, root, files)
	want := []string{
		"app/benchmarks/deep/src/main/AndroidManifest.xml",
		"app/benchmarks/src/main/res/layout/b.xml",
		"app/src/main/res/layout/a.xml",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("XML files = %q, want each once: %q", got, want)
	}
	meta := CrossFileCacheMeta{Entries: crossFileFingerprintEntries(kt, nil, files)}
	if !meta.CoversXMLFiles(LoadXMLFilesForCache(kt)) {
		t.Fatal("the meta built from the XML list does not cover the same list loaded again")
	}
}

// The outer walk prunes third_party, but a module inside it is still walked
// from its own root, so its XML stays in the list, once.
func TestLoadXMLFilesForCacheKeepsModuleUnderPrunedDir(t *testing.T) {
	root, kt := writeXMLRootsFixture(t, map[string]string{
		"app/src/main/kotlin/A.kt":                          "package a\n",
		"app/third_party/lib/src/main/kotlin/L.kt":          "package l\n",
		"app/third_party/lib/src/main/res/layout/l.xml":     "<LinearLayout/>\n",
		"app/third_party/unbuilt/src/main/res/layout/u.xml": "<LinearLayout/>\n",
	})
	got := xmlRelPaths(t, root, LoadXMLFilesForCache(kt))
	want := []string{"app/third_party/lib/src/main/res/layout/l.xml"}
	if !slices.Equal(got, want) {
		t.Fatalf("XML files = %q, want %q", got, want)
	}
}
