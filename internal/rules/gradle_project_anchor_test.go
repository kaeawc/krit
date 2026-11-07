package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// anchorVariant rewrites every Gradle/catalog file of a fixture project.
type anchorVariant struct {
	name string
	// lineShift is how many lines the rewrite adds before the original first line.
	lineShift int
	rewrite   func(path, text string) string
}

func anchorVariants() []anchorVariant {
	return []anchorVariant{
		{"crlf", 0, func(_, s string) string { return strings.ReplaceAll(s, "\n", "\r\n") }},
		{"tabs", 0, func(_, s string) string { return strings.ReplaceAll(s, "    ", "\t") }},
		{"header-comment", 4, func(p, s string) string {
			header := "// header comment mentioning implementation(\"com.fake:dep:1.0\")\n/*\n implementation(\"com.fake:other:2.0\")\n*/\n"
			if strings.HasSuffix(p, ".toml") {
				header = "# header\n# okhttp = \"9.9.9\"\n\n# end\n"
			}
			return header + s
		}},
		{"crlf-tabs-header", 4, func(p, s string) string {
			header := "// header\n// header\n\n// end\n"
			if strings.HasSuffix(p, ".toml") {
				header = "# header\n# header\n\n# end\n"
			}
			return strings.ReplaceAll(header+strings.ReplaceAll(s, "    ", "\t"), "\n", "\r\n")
		}},
	}
}

func copyProject(t *testing.T, src string, rewrite func(path, text string) string) string {
	t.Helper()
	dst := t.TempDir()
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		text := string(data)
		if strings.HasSuffix(p, ".kts") || strings.HasSuffix(p, ".toml") {
			text = rewrite(p, text)
		}
		out := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		return os.WriteFile(out, []byte(text), 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

// anchor is a finding position plus the source text it points at.
type anchor struct {
	file      string
	line, col int
	msg       string
	token     string
}

// tokenAt returns the text starting at the 1-based line and byte column of a
// file, up to 8 bytes, so an anchor can be compared across layouts.
func tokenAt(path string, line, col int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "<unreadable>"
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if line < 1 || line > len(lines) || col < 1 || col-1 > len(lines[line-1]) {
		return fmt.Sprintf("<out of range %d:%d>", line, col)
	}
	text := lines[line-1][col-1:]
	if len(text) > 8 {
		text = text[:8]
	}
	return text
}

func anchors(root string, findings []scanner.Finding) []anchor {
	var out []anchor
	for _, f := range findings {
		rel, err := filepath.Rel(root, f.File)
		if err != nil {
			rel = f.File
		}
		out = append(out, anchor{filepath.ToSlash(rel), f.Line, f.Col, f.Message, tokenAt(f.File, f.Line, f.Col)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].msg+out[i].file < out[j].msg+out[j].file })
	return out
}

// sameAnchors reports whether got matches want after lineShift lines were
// added. Columns may differ (tabs are one byte wide) but must point at the same text.
func sameAnchors(got, want []anchor, lineShift int) string {
	if len(got) != len(want) {
		return fmt.Sprintf("got %d findings, want %d", len(got), len(want))
	}
	for i := range want {
		g, w := got[i], want[i]
		if g.file != w.file || g.msg != w.msg || g.line != w.line+lineShift || g.token != w.token {
			return fmt.Sprintf("finding %d: got %+v, want line %d and %+v", i, g, w.line+lineShift, w)
		}
	}
	return ""
}

// Project rules that read build scripts and version catalogs must anchor on
// the same token whatever the line endings, indent or leading comments.
func TestGradleProjectRuleAnchorsSurviveFileLayout(t *testing.T) {
	root := internalFixtureRoot(t)
	runners := map[string]func(*testing.T, string) []scanner.Finding{
		"version-catalog-raw-version-in-build": runVersionCatalogRawVersionInBuild,
		"version-catalog-unused": func(t *testing.T, dir string) []scanner.Finding {
			return runVersionCatalogUnused(t, dir, nil, true)
		},
		"version-catalog-duplicate-version": runVersionCatalogDuplicateVersion,
	}
	for fixture, run := range runners {
		src := filepath.Join(root, "positive", "supply-chain", fixture)
		baseDir := copyProject(t, src, func(_, s string) string { return s })
		want := anchors(baseDir, run(t, baseDir))
		if len(want) == 0 {
			t.Fatalf("%s: baseline produced no findings", fixture)
		}
		for _, v := range anchorVariants() {
			t.Run(fixture+"/"+v.name, func(t *testing.T) {
				dir := copyProject(t, src, v.rewrite)
				got := anchors(dir, run(t, dir))
				if diff := sameAnchors(got, want, v.lineShift); diff != "" {
					t.Fatalf("anchors drifted: %s", diff)
				}
			})
		}
	}
}
