package firchecks

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kaeawc/krit/internal/output"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestCoverageGroupsUnresolvedReferencesByImportPrefix(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	a := write("A.kt", "/* header */\n@file:Suppress(\"x\")\npackage app\n\nimport androidx.compose.ui.Modifier\nimport kotlinx.coroutines.flow.Flow\n\nfun m(): Modifier = Modifier\n")
	b := write("B.kt", "package app\nimport androidx.compose.runtime.Composable\nimport androidx.compose.runtime.remember\n@Composable fun B() {}\n")
	c := write("C.kt", "package app\nfun c() = missingLocal()\n")
	d := write("D.kt", "package app\nfun d() {}\n")
	e := write("E.kt", "package app\nfun e() {}\n")
	// B is parsed; its content comes from the parsed file, not disk.
	parsed := map[string]*scanner.File{b: {Path: b, Content: []byte("package app\nimport androidx.compose.runtime.Composable\n")}}
	stats := VerdictStats{
		AuthoritativeFiles: 7,
		ExcludedFiles:      2,
		GatedFiles: map[string]string{
			a: "Unresolved reference 'compose'.\nmore detail",
			b: "Unresolved reference: 'compose'",
			c: "Unresolved reference 'missingLocal'.",
			d: "Argument type mismatch: actual type is 'Int', but 'String' was expected.",
			e: "crashed: java.lang.OutOfMemoryError",
		},
	}
	got := Coverage(stats, parsed)
	want := output.FirCoverage{Authoritative: 7, Gated: 5, Excluded: 2, TopCauses: []output.FirGatingCause{
		{Cause: "Unresolved reference 'androidx.compose'", Count: 2},
		{Cause: "Argument type mismatch: actual type is '…', but '…' was expected.", Count: 1},
		{Cause: "Unresolved reference 'missingLocal'", Count: 1},
		{Cause: "compiler crash", Count: 1},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Coverage =\n%+v\nwant\n%+v", got, want)
	}
}

func TestCoverageKeepsTheTopFiveCauses(t *testing.T) {
	gated := map[string]string{}
	for i, name := range []string{"a", "b", "c", "d", "e", "f", "g"} {
		for j := 0; j <= i; j++ {
			gated[filepath.Join("/nowhere", name, string(rune('0'+j))+".kt")] = "Unresolved reference '" + name + "'."
		}
	}
	got := Coverage(VerdictStats{GatedFiles: gated}, nil)
	if len(got.TopCauses) != maxCoverageCauses || got.TopCauses[0].Cause != "Unresolved reference 'g'" || got.TopCauses[0].Count != 7 {
		t.Fatalf("TopCauses = %+v", got.TopCauses)
	}
}
