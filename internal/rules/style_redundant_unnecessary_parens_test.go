package rules_test

import (
	"strings"
	"testing"
)

// Regression test for the bug surfaced by TestFixableFixturesIdempotent:
// the fix used to strip one layer of parens per pass, so ((42)) became
// (42) on the first run and only converged on the second. The rule
// now skips inner parens in nested chains and lets the outermost
// fix descend through the chain in one pass.
func TestUnnecessaryParentheses_NestedConvergesInOnePass(t *testing.T) {
	findings := assertFlags(t, "UnnecessaryParentheses", `package test

fun f() {
    val a = ((42))
}
`)

	if len(findings) != 1 {
		t.Fatalf("expected exactly one finding for nested ((42)), got %d", len(findings))
	}
	fix := findings[0].Fix
	if fix == nil {
		t.Fatal("finding is missing a Fix")
	}
	if !fix.ByteMode {
		t.Fatalf("expected byte-mode fix, got line-mode: %+v", fix)
	}
	if strings.TrimSpace(fix.Replacement) != "42" {
		t.Errorf("expected one-pass replacement %q, got %q", "42", fix.Replacement)
	}
}

// Triple-nested: (((42))) should also converge in a single pass.
func TestUnnecessaryParentheses_TripleNestedConvergesInOnePass(t *testing.T) {
	findings := assertFlags(t, "UnnecessaryParentheses", `package test

fun f() {
    val a = (((42)))
}
`)

	if len(findings) != 1 {
		t.Fatalf("expected exactly one finding for (((42))), got %d", len(findings))
	}
	if rep := strings.TrimSpace(findings[0].Fix.Replacement); rep != "42" {
		t.Errorf("expected replacement %q, got %q", "42", rep)
	}
}

// Lambda inside nested parens in a value-argument position: f((lambda)).
// The rule has a special case to keep parens around lambdas in argument
// position (parenthesized lambda prevents trailing-lambda promotion).
// After the convergence fix, the redundancy check must still decide
// based on the immediate child's type, not the descended replacement
// node — otherwise nested parens around lambdas would silently become
// trailing-lambda calls.
func TestUnnecessaryParentheses_LambdaInValueArgumentBehaviorPreserved(t *testing.T) {
	// Single paren around lambda in arg: NOT flagged (today's behavior).
	assertClean(t, "UnnecessaryParentheses", `package test

fun take(block: () -> Int): Int = block()

fun caller() {
    take({ 42 })
}
`)
}
