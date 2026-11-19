package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// runPairRuleWithProject runs PreferToOverPairSyntax on code with a resolver that also indexes the other files.
func runPairRuleWithProject(t *testing.T, code string, others ...string) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	files := []*scanner.File{file}
	for _, o := range others {
		files = append(files, parseInline(t, o))
	}
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel(files, 1)
	return runRuleOnFileWithResolver(t, "PreferToOverPairSyntax", file, resolver)
}

const otherPackagePair = "package other\n\nclass Pair<A, B>(val a: A, val b: B)\n"

// A Pair class in another package is not in scope without an import, so kotlin.Pair still wins.
func TestPreferToOverPairSyntax_ProjectPairElsewhereDoesNotShadow(t *testing.T) {
	findings := runPairRuleWithProject(t, "package test\n\nfun foo() = Pair(1, 2)\n", otherPackagePair)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %v", findings)
	}
}

func TestPreferToOverPairSyntax_ProjectPairInScopeShadows(t *testing.T) {
	for name, code := range map[string]string{
		"same package": "package other\n\nfun foo() = Pair(1, 2)\n",
		"star import":  "package test\n\nimport other.*\n\nfun foo() = Pair(1, 2)\n",
	} {
		if findings := runPairRuleWithProject(t, code, otherPackagePair); len(findings) != 0 {
			t.Errorf("%s: expected no findings, got %v", name, findings)
		}
	}
}

// An import alias of kotlin.Pair removes the default import, so a bare `Pair` does not resolve to it.
func TestPreferToOverPairSyntax_AliasHidesBareName(t *testing.T) {
	findings := runRuleByName(t, "PreferToOverPairSyntax", `
package test
import kotlin.Pair as KPair
fun aliased() = KPair(1, 2)
fun bare() = Pair(3, 4)
`)
	if len(findings) != 1 || findings[0].Line != 4 {
		t.Fatalf("expected one finding on line 4, got %v", findings)
	}
}
