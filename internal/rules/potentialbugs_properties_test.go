package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// --- PropertyUsedBeforeDeclaration ---

func TestPropertyUsedBeforeDeclaration_Positive(t *testing.T) {
	findings := runRuleByName(t, "PropertyUsedBeforeDeclaration", `
package test
class Foo {
    val a = b
    val b = 1
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for property used before declaration")
	}
}

func TestPropertyUsedBeforeDeclaration_Negative(t *testing.T) {
	findings := runRuleByName(t, "PropertyUsedBeforeDeclaration", `
package test
class Foo {
    val a = 1
    val b = a
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- UnconditionalJumpStatementInLoop ---

func TestUnconditionalJumpStatementInLoop_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnconditionalJumpStatementInLoop", `
package test
fun main() {
    for (i in 1..10) {
        return
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for unconditional jump in loop")
	}
}

func TestUnconditionalJumpStatementInLoop_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnconditionalJumpStatementInLoop", `
package test
fun main() {
    for (i in 1..10) {
        if (i == 5) return
        println(i)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- UselessPostfixExpression ---

func TestUselessPostfixExpression_Positive(t *testing.T) {
	findings := runRuleByName(t, "UselessPostfixExpression", `
package test
fun inc(x: Int): Int {
    var y = x
    return y++
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for useless postfix expression")
	}
}

func TestUselessPostfixExpression_Negative(t *testing.T) {
	findings := runRuleByName(t, "UselessPostfixExpression", `
package test
fun inc(x: Int): Int {
    var y = x
    y++
    return y
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- UnusedUnaryOperator ---

func TestUnusedUnaryOperator_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnusedUnaryOperator", `
package test
class Foo {
    fun standalone(x: Int) {
        +x
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for unused unary operator")
	}
}

func TestUnusedUnaryOperator_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnusedUnaryOperator", `
package test
fun main() {
    val x = 5
    val y = +x
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestUnusedUnaryOperator_NegativeLambdaAndWhenValues(t *testing.T) {
	findings := runRuleByName(t, "UnusedUnaryOperator", `
package test
fun main(x: Int, items: List<Int>) {
    val flipped = items.map { -it }
    val sign = when (x) {
        1 -> -x
        else -> +x
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- UnnamedParameterUse ---

func TestUnnamedParameterUse_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnnamedParameterUse", `
package test
fun create(a: Int, b: Int, c: Int, d: Int, e: Int) = a + b + c + d + e
fun main() {
    create(1, 2, 3, 4, 5)
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for many unnamed parameters")
	}
}

func TestUnnamedParameterUse_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnnamedParameterUse", `
package test
fun add(a: Int, b: Int) = a + b
fun main() {
    add(1, 2)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestUnnamedParameterUse_IgnoresGradleAndTestSources(t *testing.T) {
	code := `
package test
fun create(a: Int, b: Int, c: Int, d: Int, e: Int) = a + b + c + d + e
fun main() {
    create(1, 2, 3, 4, 5)
}
`
	for _, path := range []string{"build.gradle.kts", "src/test/kotlin/FooTest.kt"} {
		findings := runRuleByNameOnPath(t, "UnnamedParameterUse", path, code)
		if len(findings) != 0 {
			t.Fatalf("expected no UnnamedParameterUse findings for %s, got %d", path, len(findings))
		}
	}
}

func TestUnnamedParameterUse_IgnoresForwardingWrappers(t *testing.T) {
	findings := runRuleByName(t, "UnnamedParameterUse", `
package test
fun target(level: Int, tag: String, message: String, throwable: Throwable? = null, marker: String? = null) = Unit
fun wrapper(tag: String, message: String, throwable: Throwable? = null, marker: String? = null) =
    target(1, tag, message, throwable, marker)
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for forwarding wrapper call, got %d", len(findings))
	}
}

func TestUnnamedParameterUse_HonorsAllowSingleParamUse(t *testing.T) {
	// AllowSingleParamUse was previously a dead config — exposed in
	// metadata but never consulted. Default true preserves current behavior
	// (single-param calls don't fire).
	// Setting it to false reports single unnamed-parameter calls.
	var rule *rules.UnnamedParameterUseRule
	for _, candidate := range api.Registry {
		if candidate.ID == "UnnamedParameterUse" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.UnnamedParameterUseRule)
			if !ok {
				t.Fatalf("expected UnnamedParameterUseRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("UnnamedParameterUse rule not registered")
	}
	original := rule.AllowSingleParamUse
	defer func() { rule.AllowSingleParamUse = original }()

	singleParamCode := `package test
fun greet(name: String) = "hi $name"
fun call() {
    greet("alice")
}
`
	// Default (true): single-param call passes — no finding.
	if findings := runRuleByName(t, "UnnamedParameterUse", singleParamCode); len(findings) != 0 {
		t.Fatalf("expected no findings under default AllowSingleParamUse=true, got %d", len(findings))
	}

	rule.AllowSingleParamUse = false
	// Flag off: single-param call fires.
	if findings := runRuleByName(t, "UnnamedParameterUse", singleParamCode); len(findings) == 0 {
		t.Fatal("expected finding for single-param call under AllowSingleParamUse=false")
	}

	// Forwarding-wrapper exclusion still applies.
	forwarder := `package test
fun target(name: String) = Unit
fun wrapper(name: String) = target(name)
`
	if findings := runRuleByName(t, "UnnamedParameterUse", forwarder); len(findings) != 0 {
		t.Fatalf("expected no findings for single-param forwarder, got %d", len(findings))
	}
}

func TestUselessPostfixExpression_PropertiesAssignmentsAndFix(t *testing.T) {
	property := "package test\nclass K {\n    var f = 0\n    fun m(): Int {\n        return f++\n    }\n}\n"
	if got := runRuleByName(t, "UselessPostfixExpression", property); len(got) != 0 {
		t.Errorf("returning a property's postfix keeps the increment, got %d findings", len(got))
	}
	local := "package test\nfun m(): Int {\n    var a = 1\n    return a++\n}\n"
	got := runRuleByName(t, "UselessPostfixExpression", local)
	if len(got) != 1 || got[0].Fix == nil || got[0].Fix.Replacement != "return a" {
		t.Errorf("a local's returned postfix must be fixed to 'return a', got %+v", got)
	}
	assign := "package test\nfun m() {\n    var a = 1\n    a = a++\n    a = a + 1\n    a = b++\n}\nvar b = 0\n"
	if got := runRuleByName(t, "UselessPostfixExpression", assign); len(got) != 1 {
		t.Errorf("expected only 'a = a++' to be reported, got %d", len(got))
	}
}

func TestUnconditionalJumpStatementInLoop_StructuralBody(t *testing.T) {
	flagged := map[string]string{
		"break after work":                    "fun f(xs: List<Int>) { for (x in xs) { println(x); break } }",
		"bare break":                          "fun f(xs: List<Int>) { for (x in xs) break }",
		"return in while":                     "fun f(): Int { while (true) { return 1 } }",
		"break in do-while":                   "fun f() { do { println(); break } while (true) }",
		"trailing continue":                   "fun f(xs: List<Int>) { for (x in xs) { println(x); continue } }",
		"continue after conditional continue": "fun f(xs: List<Int>) { for (x in xs) { if (x < 0) continue; println(x); break } }",
	}
	for name, src := range flagged {
		if got := runRuleByName(t, "UnconditionalJumpStatementInLoop", "package test\n"+src+"\n"); len(got) != 1 {
			t.Errorf("%s: want 1 finding, got %d", name, len(got))
		}
	}
	clean := map[string]string{
		"conditional break":    "fun f(xs: List<Int>) { for (x in xs) { if (x > 1) break } }",
		"identifier lookalike": "fun f(xs: List<Int>) { var returned = 0; for (x in xs) { returned = x } }",
		"throw is not a jump":  "fun f() { do { throw IllegalStateException() } while (true) }",
		"return after jump":    "fun f(xs: List<Int>) { for (x in xs) { if (x > 1) { break }; println(x); return } }",
		"return before elvis":  "fun f(xs: List<Int?>): Int { for (x in xs) { return x ?: continue }; return 0 }",
		"labeled continue":     "fun f(xs: List<Int>) { outer@ for (x in xs) { for (y in xs) { if (y > 1) continue@outer }; break } }",
		"string mentions if":   "fun f(xs: List<Int>) { for (x in xs) { println(\"if\"); println(x) } }",
	}
	for name, src := range clean {
		if got := runRuleByName(t, "UnconditionalJumpStatementInLoop", "package test\n"+src+"\n"); len(got) != 0 {
			t.Errorf("%s: want no findings, got %d", name, len(got))
		}
	}
}

func TestUnusedUnaryOperator_LeadingOperatorOnNewLine(t *testing.T) {
	flagged := map[string]string{
		"property initializer": "fun f(a: Int, b: Int) {\n    val x = a\n        + b\n    println(x)\n}",
		"return value":         "fun f(a: Int, b: Int): Int {\n    return a\n        - b\n}",
		"statement":            "fun f(a: Int, b: Int) {\n    println(a)\n    +b\n}",
	}
	for name, src := range flagged {
		if got := runRuleByName(t, "UnusedUnaryOperator", "package test\n"+src+"\n"); len(got) != 1 {
			t.Errorf("%s: want 1 finding, got %d", name, len(got))
		}
	}
	clean := map[string]string{
		"trailing operator":  "fun f(a: Int, b: Int) {\n    val x = a +\n        b\n    println(x)\n}",
		"inside parentheses": "fun f(a: Int, b: Int) {\n    val x = (a\n        + b)\n    println(x)\n}",
		"inside call":        "fun f(a: Int, b: Int) {\n    println(a\n        + b)\n}",
		"single line":        "fun f(a: Int, b: Int) {\n    val x = a + b\n    println(x)\n}",
	}
	for name, src := range clean {
		if got := runRuleByName(t, "UnusedUnaryOperator", "package test\n"+src+"\n"); len(got) != 0 {
			t.Errorf("%s: want no findings, got %d", name, len(got))
		}
	}
}

func TestPropertyUsedBeforeDeclaration_LazyAndForeignReferences(t *testing.T) {
	clean := "package test\nclass P {\n    val c get() = d\n    val d = 1\n    val a = other.b\n    val b = 2\n    val e = foo(x = 1)\n    val x = 3\n" +
		"    val h = if (flag()) { val w = 1; w } else 0\n    val w = 3\n    var s: Int = 0\n        set(v) { field = t }\n    val t = 1\n}\n"
	if got := runRuleByName(t, "PropertyUsedBeforeDeclaration", clean); len(got) != 0 {
		t.Errorf("getters, foreign receivers, named arguments and shadowing locals are not early uses, got %v", got)
	}
	flagged := "package test\nclass P {\n    val f = this.g\n    val g = 4\n    init { println(k) }\n    val k = 5\n}\n"
	if got := runRuleByName(t, "PropertyUsedBeforeDeclaration", flagged); len(got) != 2 {
		t.Errorf("expected the this.g initializer and the init block to be reported, got %d", len(got))
	}
}

func TestUnusedUnaryOperator_FollowsDetektLineBreakRule(t *testing.T) {
	clean := map[string]string{
		"same line statements": "fun f(x: Int) { x + 1; +x; -x }",
		"lambda result":        "fun f(): Int = run { 2\n + 3 }",
	}
	for name, src := range clean {
		if got := runRuleByName(t, "UnusedUnaryOperator", "package test\n"+src+"\n"); len(got) != 0 {
			t.Errorf("%s: want no findings, got %d", name, len(got))
		}
	}
	if got := runRuleByName(t, "UnusedUnaryOperator", "package test\nfun f(x: Int) {\n    val a = x\n    +a\n}\n"); len(got) != 1 {
		t.Errorf("a prefix operator starting a line: want 1 finding, got %d", len(got))
	}
}

func TestUselessPostfixExpression_ChecksChildrenOfTheReturnedExpression(t *testing.T) {
	flagged := map[string]string{
		"parenthesized": "fun f(): Int { var i = 0; return (i++) }",
		"binary":        "fun f(j: Int): Int { var i = 0; return i++ + j }",
		"receiver":      "fun f(): Int { var i = 0; return i++.also { } }",
	}
	for name, src := range flagged {
		if got := runRuleByName(t, "UselessPostfixExpression", "package test\n"+src+"\n"); len(got) != 1 {
			t.Errorf("%s: want 1 finding, got %d", name, len(got))
		}
	}
	if got := runRuleByName(t, "UselessPostfixExpression", "package test\nclass A { var n = 0\n fun f(): Int { return (n++) } }\n"); len(got) != 0 {
		t.Errorf("a property keeps the increment: want no findings, got %d", len(got))
	}
}
