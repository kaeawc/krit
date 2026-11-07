package rules_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// applyFindingByteFixes applies the byte-mode fixes of findings to src from the end backwards.
func applyFindingByteFixes(src string, findings []scanner.Finding) string {
	var fixes []*scanner.Fix
	for i := range findings {
		if fx := findings[i].Fix; fx != nil && fx.ByteMode {
			fixes = append(fixes, fx)
		}
	}
	sort.Slice(fixes, func(i, j int) bool { return fixes[i].StartByte > fixes[j].StartByte })
	out := []byte(src)
	for _, fx := range fixes {
		out = append(out[:fx.StartByte], append([]byte(fx.Replacement), out[fx.EndByte:]...)...)
	}
	return string(out)
}

func TestLineDeletingFixesKeepLineEndingsConsistent(t *testing.T) {
	cases := []struct {
		name, rule, src, want string
	}{
		{
			name: "unused import",
			rule: "UnusedImport",
			src:  "package x\n\nimport java.util.Date\nimport java.io.File\n\nfun f(): File = File(\"a\")\n",
			want: "package x\n\nimport java.io.File\n\nfun f(): File = File(\"a\")\n",
		},
		{
			name: "equals on signature line",
			rule: "EqualsOnSignatureLine",
			src:  "package x\n\nfun a()\n    = 42\nfun b(): Int  \n    = 100\n",
			want: "package x\n\nfun a() = 42\nfun b(): Int = 100\n",
		},
		{
			name: "equals after signature comment keeps the comment intact",
			rule: "EqualsOnSignatureLine",
			src:  "package x\n\nfun a() // note\n    = 42\n",
			want: "package x\n\nfun a() // note\n    = 42\n",
		},
	}
	for _, tc := range cases {
		for _, eol := range []string{"\n", "\r\n"} {
			name := tc.name + " lf"
			if eol != "\n" {
				name = tc.name + " crlf"
			}
			t.Run(name, func(t *testing.T) {
				src := strings.ReplaceAll(tc.src, "\n", eol)
				got := applyFindingByteFixes(src, runRuleByName(t, tc.rule, src))
				want := strings.ReplaceAll(tc.want, "\n", eol)
				if got != want {
					t.Fatalf("got %q, want %q", got, want)
				}
			})
		}
	}
}

func TestLineRulesTreatCRLFAsLineBreak(t *testing.T) {
	cases := []struct {
		rule, src string
		want      int
	}{
		{"TrailingWhitespace", "package x\n\nval a = 1  \nval b = 2\n", 1},
		{"MergeConflictMarkerLeftover", "package x\n<<<<<<< HEAD\nval a = 1\n=======\nval a = 2\n>>>>>>> other\n", 3},
	}
	for _, tc := range cases {
		for _, eol := range []string{"\n", "\r\n"} {
			src := strings.ReplaceAll(tc.src, "\n", eol)
			if got := len(runRuleByName(t, tc.rule, src)); got != tc.want {
				t.Errorf("%s with %q line endings: got %d findings, want %d", tc.rule, eol, got, tc.want)
			}
		}
	}
	src := "package x\r\n\r\nval a = 1  \r\nval b = 2\r\n"
	got := applyFindingByteFixes(src, runRuleByName(t, "TrailingWhitespace", src))
	if want := "package x\r\n\r\nval a = 1\r\nval b = 2\r\n"; got != want {
		t.Errorf("fix produced %q, want %q", got, want)
	}
}

func TestForbiddenMethodCallFixOnlyRemovesStandaloneStatements(t *testing.T) {
	src := "package x\n\nfun f(x: Int) {\n    println(\"a\")\n    when (x) {\n        1 ->\n            println(\"b\")\n        else -> println(\"c\")\n    }\n    val y = 1; println(y)\n    foo(println(\"d\"))\n}\n\nfun g() = println(\"e\")\n"
	for _, eol := range []string{"\n", "\r\n"} {
		in := strings.ReplaceAll(src, "\n", eol)
		got := applyFindingByteFixes(in, runRuleByName(t, "ForbiddenMethodCall", in))
		want := strings.ReplaceAll(strings.Replace(src, "    println(\"a\")\n", "", 1), "\n", eol)
		if got != want {
			t.Errorf("with %q line endings got %q, want %q", eol, got, want)
		}
	}
}
