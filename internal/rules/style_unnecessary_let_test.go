package rules_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The testdata corpora are synthetic sources; the .expected files hold the
// findings detekt 1.23.8 reports for them with type resolution, one
// `line:column: message` per line.
func TestUnnecessaryLet_MatchesDetektOnCorpus(t *testing.T) {
	sources, err := filepath.Glob("testdata/unnecessary_let/*.kt")
	if err != nil || len(sources) == 0 {
		t.Fatalf("no corpus found: %v", err)
	}
	for _, src := range sources {
		src := src
		t.Run(filepath.Base(src), func(t *testing.T) {
			code, err := os.ReadFile(src)
			if err != nil {
				t.Fatal(err)
			}
			want := readUnnecessaryLetExpected(t, strings.TrimSuffix(src, ".kt")+".expected")
			var got []string
			for _, f := range runRuleByName(t, "UnnecessaryLet", string(code)) {
				got = append(got, fmt.Sprintf("%d:%d: %s", f.Line, f.Col, f.Message))
			}
			sort.Strings(got)
			sort.Strings(want)
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("findings differ from detekt\ngot:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
		})
	}
}

func readUnnecessaryLetExpected(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

func TestUnnecessaryLet_FixKeepsTheValue(t *testing.T) {
	src := `package test
class Q { fun f(): Int = 1; val a = 1 }
fun t(q: Q?, r: Q) {
    val a = r.let { it.f() }
    val b = q?.let { it.a.toString() }
    val c = r.let { it }
    val d = r.let { 1 }
}
`
	findings := runRuleByName(t, "UnnecessaryLet", src)
	if len(findings) != 4 {
		t.Fatalf("expected 4 findings, got %v", findings)
	}
	apply := func(i int) string {
		f := findings[i]
		if f.Fix == nil {
			return ""
		}
		return src[:f.Fix.StartByte] + f.Fix.Replacement + src[f.Fix.EndByte:]
	}
	for i, want := range []string{"val a = r.f()", "val b = q?.a.toString()", "val c = r\n", ""} {
		got := apply(i)
		if want == "" {
			if got != "" {
				t.Errorf("finding %d: expected no fix, got %q", i, got)
			}
			continue
		}
		if !strings.Contains(got, want) {
			t.Errorf("finding %d: fix result does not contain %q:\n%s", i, want, got)
		}
	}
}

func TestUnnecessaryLet_ReportsOnTheLetLineOfAChain(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryLet", `package test
class Q { fun f(): Int = 1; fun me(): Q = this }
fun t(q: Q) {
    val a = q
        .me()
        .let { it.f() }
}
`)
	if len(findings) != 1 || findings[0].Line != 6 {
		t.Fatalf("expected one finding on line 6, got %+v", findings)
	}
}

func TestUnnecessaryLet_SkipsAFileThatDeclaresItsOwnLet(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryLet", `package test
class Q { fun f(): Int = 1 }
fun <T, R> T.let(block: (T) -> R): R = block(this)
fun t(q: Q) = q.let { it.f() }
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %+v", findings)
	}
}
