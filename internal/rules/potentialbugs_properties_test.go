package rules_test

import "testing"

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
