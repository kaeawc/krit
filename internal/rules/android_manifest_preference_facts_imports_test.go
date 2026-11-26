package rules

import (
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func sourceImportSubmatches(content string) [][]string {
	var out [][]string
	for _, m := range sourceImportRe.FindAllStringSubmatch(content, -1) {
		out = append(out, m[1:])
	}
	return out
}

func lintImportSubmatches(content string) [][]string {
	var out [][]string
	for _, m := range lintImportRe.FindAllStringSubmatch(content, -1) {
		out = append(out, m[1:])
	}
	return out
}

func fastLintImportSubmatches(content string) [][]string {
	var out [][]string
	for _, m := range findImportMatches(content, lintImportTailRe) {
		out = append(out, m[1:])
	}
	return out
}

func fastSourceImportSubmatches(content string) [][]string {
	var out [][]string
	for _, m := range findSourceImports(content) {
		out = append(out, m[1:])
	}
	return out
}

func TestFindSourceImportsMatchesRegex(t *testing.T) {
	cases := []string{
		"",
		"import a.B",
		"package p\n\nimport a.B\nimport c.*\nimport d.E as F\n\nclass X",
		"import static a.B.c;\nimport a.b.*;\r\nimport x.Y;",
		"  \t import a.B\n\n\n   import c.D",
		"val s = \"import a.B\"\nimport c.D",
		"fun f() { importFoo() }\n  import a.B",
		"import\na.B\nimport a.B\nas C",
		"import a.B\nas import\nimport c.D",
		"x import a.B\n\fimport c.D",
		"importimport a.B\nimport  a.B  ;  \n",
		"import a.B // trailing\nimport c.D",
	}
	rng := rand.New(rand.NewPCG(7, 11))
	tokens := []string{"import", "import ", " ", "\n", "\t", "\r\n", "\f", "a", "b.", ".", ".*", " as ", "as", ";", "static ", "B", "x", "\"", "//", "_", "1", "(", "@", "é"}
	for i := 0; i < 5000; i++ {
		var b strings.Builder
		for n := rng.IntN(30); n >= 0; n-- {
			b.WriteString(tokens[rng.IntN(len(tokens))])
		}
		cases = append(cases, b.String())
	}
	for _, c := range cases {
		if got, want := fastSourceImportSubmatches(c), sourceImportSubmatches(c); !reflect.DeepEqual(got, want) {
			t.Fatalf("content %q:\n got %q\nwant %q", c, got, want)
		}
		if got, want := fastLintImportSubmatches(c), lintImportSubmatches(c); !reflect.DeepEqual(got, want) {
			t.Fatalf("lint imports, content %q:\n got %q\nwant %q", c, got, want)
		}
	}
}

func TestFindSourceImportsMatchesRegexOnFixtures(t *testing.T) {
	root := filepath.Join("..", "..", "tests", "fixtures")
	n := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if ext := filepath.Ext(path); ext != ".kt" && ext != ".java" && ext != ".kts" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		n++
		if got, want := fastSourceImportSubmatches(string(data)), sourceImportSubmatches(string(data)); !reflect.DeepEqual(got, want) {
			t.Errorf("%s:\n got %q\nwant %q", path, got, want)
		}
		if got, want := fastLintImportSubmatches(string(data)), lintImportSubmatches(string(data)); !reflect.DeepEqual(got, want) {
			t.Errorf("lint imports, %s:\n got %q\nwant %q", path, got, want)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("no fixtures found")
	}
}
