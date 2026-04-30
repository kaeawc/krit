package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
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
	// metadata but never consulted. Default true matches detekt and
	// preserves current behavior (single-param calls don't fire).
	// Setting it to false reports single unnamed-parameter calls.
	var rule *rules.UnnamedParameterUseRule
	for _, candidate := range v2rules.Registry {
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
