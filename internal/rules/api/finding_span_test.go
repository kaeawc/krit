package api

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func parseSpanFixture(t *testing.T, src string) *scanner.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "T.kt")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestFillSpanOperatorTokenIsExact(t *testing.T) {
	src := "fun f(a: A, s: String?) {\n    val x = a!!.text.length\n    val y = s?.length\n    val z = s ?: \"\"\n}\n"
	file := parseSpanFixture(t, src)
	c := &Context{File: file}
	for _, tok := range []string{"!!", "?.", "?:"} {
		start := strings.Index(src, tok)
		line := strings.Count(src[:start], "\n") + 1
		col := start - strings.LastIndex(src[:start], "\n")
		f := scanner.Finding{File: file.Path, Line: line, Col: col}
		c.fillSpan(&f)
		if f.StartByte != start || f.EndByte != start+2 {
			t.Errorf("%s: span = [%d,%d), want [%d,%d)", tok, f.StartByte, f.EndByte, start, start+2)
		}
	}
}
