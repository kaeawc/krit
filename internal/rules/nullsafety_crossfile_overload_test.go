package rules

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// runNullSafetyRuleAcrossFiles indexes the named sources in one directory and
// returns how many findings ruleID reports on target.
func runNullSafetyRuleAcrossFiles(t *testing.T, ruleID, target string, sources map[string]string) int {
	t.Helper()
	dir := t.TempDir()
	var files []*scanner.File
	var targetFile *scanner.File
	for name, code := range sources {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
			t.Fatal(err)
		}
		f, err := scanner.ParseFile(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
		if name == target {
			targetFile = f
		}
	}
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel(files, 2)
	for _, r := range api.Registry {
		if r.ID == ruleID {
			cols := NewDispatcher([]*api.Rule{r}, resolver).Run(targetFile)
			return cols.Len()
		}
	}
	t.Fatalf("%s is not registered", ruleID)
	return 0
}

const crossFileNullableCaller = `package p

fun maker(x: String): String? = if (x.isEmpty()) null else x

fun use(x: String): String {
    val n = maker(x) ?: return ""
    if (maker(x) != null) println(n)
    return n
}
`

// A private non-null function with the same name in another file must not
// decide the type of a call that can only reach the nullable one.
func TestNullSafetyRulesIgnorePrivateSameNameFunctionInOtherFile(t *testing.T) {
	for _, other := range []string{"a_other.kt", "z_other.kt"} {
		sources := map[string]string{
			"m_caller.kt": crossFileNullableCaller,
			other:         "package p\n\nprivate fun maker(x: Int): String = \"v$x\"\n",
		}
		for _, id := range []string{"UselessElvisOnNonNull", "UnnecessaryNotNullCheck"} {
			if n := runNullSafetyRuleAcrossFiles(t, id, "m_caller.kt", sources); n != 0 {
				t.Errorf("%s with %s: got %d finding(s), want none", id, other, n)
			}
		}
	}
}

// Visible same-name candidates that disagree on nullability make the rule decline.
func TestUselessElvisOnNonNullDeclinesAmbiguousSameFileOverloads(t *testing.T) {
	sources := map[string]string{"a.kt": `package p

private fun maker(x: Int): String = "v$x"
private fun maker(x: String): String? = null

fun use(x: String): String {
    val n = maker(x) ?: return ""
    return n
}
`}
	if n := runNullSafetyRuleAcrossFiles(t, "UselessElvisOnNonNull", "a.kt", sources); n != 0 {
		t.Fatalf("got %d finding(s), want none", n)
	}
}

// A same-file private non-null function still proves the elvis useless even
// when another file declares a private nullable function of the same name.
func TestUselessElvisOnNonNullKeepsSameFilePrivateFunction(t *testing.T) {
	sources := map[string]string{
		"a.kt": `package p

private fun maker(x: Int): String = "v$x"

fun use(): String {
    val n = maker(1) ?: return ""
    return n
}
`,
		"z.kt": "package p\n\nprivate fun maker(x: Int): String? = null\n",
	}
	if n := runNullSafetyRuleAcrossFiles(t, "UselessElvisOnNonNull", "a.kt", sources); n != 1 {
		t.Fatalf("got %d finding(s), want 1", n)
	}
}
