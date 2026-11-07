package api

import (
	"os"
	"path/filepath"
	"testing"
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
