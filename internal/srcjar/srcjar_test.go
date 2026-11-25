package srcjar

import (
	"archive/zip"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func writeJar(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(entries[name])); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func listFiles(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			rel, _ := filepath.Rel(root, path)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}

func TestExtractWritesOnlySourcesAndSkipsEscapingEntries(t *testing.T) {
	tmp := t.TempDir()
	jar := filepath.Join(tmp, "gen.srcjar")
	writeJar(t, jar, map[string]string{
		"com/example/Foo.kt":     "package com.example\nclass Foo\n",
		"com/example/Bar.java":   "package com.example; public class Bar {}\n",
		"META-INF/MANIFEST.MF":   "Manifest-Version: 1.0\n",
		"com/example/res.txt":    "not a source",
		"../escape/Evil.kt":      "class Evil",
		"/abs/Evil.kt":           "class Evil",
		"com/../../up/Evil.java": "class Evil {}",
	})
	root := filepath.Join(tmp, "root")
	dir, err := Extract(jar, root)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(dir) != root {
		t.Fatalf("source root %s is not under %s", dir, root)
	}
	got := listFiles(t, dir)
	want := []string{"com/example/Bar.java", "com/example/Foo.kt"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("extracted %v, want %v", got, want)
	}
	if _, err := os.Stat(filepath.Join(tmp, "escape")); !os.IsNotExist(err) {
		t.Fatalf("an escaping entry was written outside the root: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "com", "example", "Foo.kt"))
	if err != nil || string(data) != "package com.example\nclass Foo\n" {
		t.Fatalf("Foo.kt = %q, %v", data, err)
	}
}

func TestExtractIsContentAddressed(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "root")
	a := filepath.Join(tmp, "a.srcjar")
	b := filepath.Join(tmp, "copy-of-a.srcjar")
	c := filepath.Join(tmp, "c.srcjar")
	writeJar(t, a, map[string]string{"p/Foo.kt": "package p\nclass Foo\n"})
	writeJar(t, b, map[string]string{"p/Foo.kt": "package p\nclass Foo\n"})
	writeJar(t, c, map[string]string{"p/Foo.kt": "package p\nclass Foo(val x: Int)\n"})
	dirA, err := Extract(a, root)
	if err != nil {
		t.Fatal(err)
	}
	dirB, err := Extract(b, root)
	if err != nil {
		t.Fatal(err)
	}
	dirC, err := Extract(c, root)
	if err != nil {
		t.Fatal(err)
	}
	if dirA != dirB {
		t.Fatalf("identical srcjars map to different roots: %s, %s", dirA, dirB)
	}
	if dirA == dirC {
		t.Fatalf("different srcjars share a root: %s", dirA)
	}
	// A second extraction reuses the published root and leaves no temp dirs.
	again, err := Extract(a, root)
	if err != nil || again != dirA {
		t.Fatalf("re-extract = %s, %v; want %s", again, err, dirA)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Fatalf("temporary directory left behind: %s", e.Name())
		}
	}
	if len(entries) != 2 {
		t.Fatalf("root holds %d entries, want 2", len(entries))
	}
}

func TestExtractAllUsesKritHomeAndReportsBadJars(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("KRIT_HOME", filepath.Join(tmp, "home"))
	good := filepath.Join(tmp, "good.srcjar")
	writeJar(t, good, map[string]string{"p/Foo.kt": "package p\nclass Foo\n"})
	notZip := filepath.Join(tmp, "bad.srcjar")
	if err := os.WriteFile(notZip, []byte("not a zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirs, warnings := ExtractAll([]string{good, notZip, good, filepath.Join(tmp, "missing.srcjar"), ""})
	if len(dirs) != 1 || !strings.HasPrefix(dirs[0], filepath.Join(tmp, "home", "srcjars")+string(filepath.Separator)) {
		t.Fatalf("dirs = %v, want one root under $KRIT_HOME/srcjars", dirs)
	}
	if len(warnings) != 2 {
		t.Fatalf("warnings = %v, want one each for the bad and the missing srcjar", warnings)
	}
	for _, w := range warnings {
		if !strings.Contains(w, "srcjar") {
			t.Fatalf("warning %q does not name the srcjar", w)
		}
	}
}

func TestExtractRejectsDirectories(t *testing.T) {
	tmp := t.TempDir()
	if _, err := Extract(tmp, filepath.Join(tmp, "root")); err == nil {
		t.Fatal("extracting a directory succeeded")
	}
}
