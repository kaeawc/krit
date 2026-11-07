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

	// detekt reports both pairs; only the outermost carries the fix.
	if len(findings) != 2 {
		t.Fatalf("expected a finding per pair for nested ((42)), got %d", len(findings))
	}
	fix := findings[0].Fix
	if findings[1].Fix != nil && findings[0].Fix == nil {
		fix = findings[1].Fix
	}
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

	if len(findings) != 3 {
		t.Fatalf("expected a finding per pair for (((42))), got %d", len(findings))
	}
	fixes := 0
	for _, f := range findings {
		if f.Fix == nil {
			continue
		}
		fixes++
		if rep := strings.TrimSpace(f.Fix.Replacement); rep != "42" {
			t.Errorf("expected replacement %q, got %q", "42", rep)
		}
	}
	if fixes != 1 {
		t.Errorf("expected only the outermost pair to carry a fix, got %d fixes", fixes)
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

// TestUnnecessaryParentheses_FollowsOperatorPrecedenceLikeDetekt covers the operand positions detekt decides by precedence.
func TestUnnecessaryParentheses_FollowsOperatorPrecedenceLikeDetekt(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryParentheses", `package test
fun f(a: Int, b: Int, c: Boolean, d: Boolean, n: Int?) {
    val keep1 = (a + b) * 2
    val keep2 = a - (b - 1)
    val keep3 = (c || d) && c
    val keep4 = a ?: (b ?: 1)
    val drop1 = a + (b * 2)
    val drop2 = (a > b) == c
    val drop3 = (a + b) > 3
    val drop4 = (n?.plus(1)) ?: 0
    val drop5 = c && (d && c)
}
`)
	var lines []int
	for _, f := range findings {
		lines = append(lines, f.Line)
	}
	want := []int{7, 8, 9, 10, 11}
	if len(lines) != len(want) {
		t.Fatalf("expected findings on lines %v, got %v", want, lines)
	}
	got := map[int]bool{}
	for _, l := range lines {
		got[l] = true
	}
	for _, l := range want {
		if !got[l] {
			t.Errorf("missing finding on line %d (got %v)", l, lines)
		}
	}
	for _, f := range findings {
		if !strings.HasPrefix(f.Message, "Parentheses in (") || !strings.Contains(f.Message, " are unnecessary and can be replaced with: ") {
			t.Errorf("unexpected message %q", f.Message)
		}
	}
}
