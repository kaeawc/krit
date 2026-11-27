package api

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAnchorXMLColumn(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "a.xml")
	if err := os.WriteFile(xmlPath, []byte("<a>\n    <b/>\n</a>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ktPath := filepath.Join(dir, "a.kt")
	if err := os.WriteFile(ktPath, []byte("fun f() {\n    g()\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name      string
		file      string
		line, col int
		want      int
	}{
		{"indented element", xmlPath, 2, 1, 5},
		{"explicit column kept", xmlPath, 2, 3, 3},
		{"unindented line", xmlPath, 1, 1, 1},
		{"other languages untouched", ktPath, 2, 1, 1},
		{"missing file", filepath.Join(dir, "none.xml"), 2, 1, 1},
	}
	for _, c := range cases {
		if got := anchorXMLColumn(c.file, c.line, c.col); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}

// The line cache outlives one scan in the daemon, LSP and MCP, so an edited
// XML file must be read again: a new indentation moves the anchored column,
// and a new line count moves every span (#1852).
func TestXMLLinesFollowEdits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "strings.xml")
	write := func(content string, mtime time.Time) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-time.Hour)
	write("<resources>\n    <string name=\"a\">A</string>\n</resources>\n", old)
	if got := anchorXMLColumn(path, 2, 1); got != 5 {
		t.Fatalf("first read: column %d, want 5", got)
	}

	write("<resources>\n  <string name=\"a\">A</string>\n</resources>\n", old.Add(time.Minute))
	if got := anchorXMLColumn(path, 2, 1); got != 3 {
		t.Fatalf("after re-indenting: column %d, want 3", got)
	}
	write("<?xml version=\"1.0\"?>\n<resources>\n  <string name=\"a\">A</string>\n</resources>\n", old.Add(2*time.Minute))
	if start, end := xmlSpan(path, 3, 3); start != 36 || end != 43 {
		t.Fatalf("after adding a line: span %d-%d, want 36-43", start, end)
	}

	// A file written within the filesystem's mtime granularity can change
	// again without its size or mtime changing, so it is never served from
	// the cache.
	now := time.Now()
	write("<resources>\n  <string name=\"a\">A</string>\n</resources>\n", now)
	if got := anchorXMLColumn(path, 2, 1); got != 3 {
		t.Fatalf("recent file: column %d, want 3", got)
	}
	write("<resources>\n<string   name=\"a\">A</string>\n</resources>\n", now)
	if got := anchorXMLColumn(path, 2, 1); got != 1 {
		t.Fatalf("same size and mtime, no indent: column %d, want 1", got)
	}
}
