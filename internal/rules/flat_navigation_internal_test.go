package rules

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// referenceNavLastIdentifier is the original walk: it records the text of
// every simple_identifier it visits and keeps the last one.
func referenceNavLastIdentifier(file *scanner.File, idx uint32) string {
	last := ""
	var walk func(uint32)
	walk = func(n uint32) {
		switch file.FlatType(n) {
		case "simple_identifier":
			last = file.FlatNodeString(n, nil)
			return
		case "value_arguments", "value_argument", "call_suffix", "lambda_literal", "string_literal":
			return
		}
		for child := file.FlatFirstChild(n); child != 0; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) {
				walk(child)
			}
		}
	}
	walk(idx)
	return last
}

func TestFlatNavigationExpressionLastIdentifierMatchesReferenceWalk(t *testing.T) {
	file := parseInlineKt(t, `
fun f(a: A) {
    a.b.c.d()
    a?.b()?.c(1) { it.x }.d
    a.b("s").c[0].e
    a::b.c
    (a ?: b).c.d<T>()
    this.a.b
    super.foo().bar
    a.b { c.d }.e(f.g)
    "x".length
    listOf(1).map { it * 2 }.filter { it > 1 }.size
}
`)
	n := 0
	file.FlatWalkNodes(0, "navigation_expression", func(idx uint32) {
		n++
		if got, want := flatNavigationExpressionLastIdentifier(file, idx), referenceNavLastIdentifier(file, idx); got != want {
			t.Errorf("%q: got %q, want %q", file.FlatNodeText(idx), got, want)
		}
	})
	if n < 10 {
		t.Fatalf("only %d navigation expressions parsed", n)
	}
}
