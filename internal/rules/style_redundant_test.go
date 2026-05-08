package rules_test

import (
	"testing"
)

// --- RedundantVisibilityModifier ---

func TestRedundantVisibilityModifier_Positive(t *testing.T) {
	findings := runRuleByName(t, "RedundantVisibilityModifier", `
package test
public class Foo {
    public fun bar() {}
}`)
	if len(findings) == 0 {
		t.Error("expected findings for explicit 'public' modifier")
	}
}

func TestRedundantVisibilityModifier_Negative(t *testing.T) {
	findings := runRuleByName(t, "RedundantVisibilityModifier", `
package test
class Foo {
    fun bar() {}
}`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// --- RedundantConstructorKeyword ---

func TestRedundantConstructorKeyword_Positive(t *testing.T) {
	findings := runRuleByName(t, "RedundantConstructorKeyword", `
package test
class Foo constructor(val x: Int)
`)
	if len(findings) == 0 {
		t.Error("expected findings for redundant 'constructor' keyword")
	}
}

func TestRedundantConstructorKeyword_Negative(t *testing.T) {
	findings := runRuleByName(t, "RedundantConstructorKeyword", `
package test
class Foo(val x: Int)
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// --- RedundantExplicitType ---

func TestRedundantExplicitType_Positive(t *testing.T) {
	findings := runRuleByName(t, "RedundantExplicitType", `
package test
val x: String = "hello"
`)
	if len(findings) == 0 {
		t.Error("expected findings for redundant explicit type on string literal")
	}
}

func TestRedundantExplicitType_Negative(t *testing.T) {
	findings := runRuleByName(t, "RedundantExplicitType", `
package test
val x = "hello"
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// --- UnnecessaryParentheses ---

func TestRedundantUnnecessaryParentheses_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryParentheses", `
package test
fun foo(): Int {
    return (42)
}
`)
	if len(findings) == 0 {
		t.Error("expected findings for unnecessary parentheses around return value")
	}
}

func TestRedundantUnnecessaryParentheses_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryParentheses", `
package test
fun foo(): Int {
    return 42
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// --- UnnecessaryInheritance ---

func TestRedundantUnnecessaryInheritance_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryInheritance", `
package test
class Foo : Any()
`)
	if len(findings) == 0 {
		t.Error("expected findings for unnecessary inheritance from Any")
	}
}

func TestRedundantUnnecessaryInheritance_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryInheritance", `
package test
class Foo
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// --- UnnecessaryInnerClass ---

func TestRedundantUnnecessaryInnerClass_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryInnerClass", `
package test
class Outer {
    inner class Inner {
        fun doStuff() = 1
    }
}
`)
	if len(findings) == 0 {
		t.Error("expected findings for inner class not using outer reference")
	}
}

func TestRedundantUnnecessaryInnerClass_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryInnerClass", `
package test
class Outer {
    val value = 10
    inner class Inner {
        fun doStuff() = this@Outer.value
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// --- OptionalUnit ---

func TestRedundantOptionalUnit_Positive(t *testing.T) {
	findings := runRuleByName(t, "OptionalUnit", `
package test
fun doWork(): Unit {
    println("work")
}
`)
	if len(findings) == 0 {
		t.Error("expected findings for explicit Unit return type")
	}
}

func TestRedundantOptionalUnit_Negative(t *testing.T) {
	findings := runRuleByName(t, "OptionalUnit", `
package test
fun doWork() {
    println("work")
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// --- UnnecessaryBackticks ---

func TestRedundantUnnecessaryBackticks_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryBackticks", "package test\nval `myVar` = 42\n")
	if len(findings) == 0 {
		t.Error("expected findings for unnecessary backticks around valid identifier")
	}
}

func TestRedundantUnnecessaryBackticks_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryBackticks", "package test\nval `class` = 42\n")
	if len(findings) != 0 {
		t.Errorf("expected no findings for backticks around keyword, got %d", len(findings))
	}
}

// --- UselessCallOnNotNull ---

func TestRedundantUselessCallOnNotNull_Positive(t *testing.T) {
	findings := runRuleByName(t, "UselessCallOnNotNull", `
package test
fun foo() {
    val x = "hello".orEmpty()
}
`)
	if len(findings) == 0 {
		t.Error("expected findings for useless orEmpty() on string literal")
	}
}

func TestRedundantUselessCallOnNotNull_Negative(t *testing.T) {
	findings := runRuleByName(t, "UselessCallOnNotNull", `
package test
fun foo(x: String?) {
    val y = x?.orEmpty()
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}
