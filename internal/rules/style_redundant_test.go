package rules_test

import (
	"strings"
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

func TestRedundantConstructorKeyword_FixRemovesKeyword(t *testing.T) {
	findings := runRuleByName(t, "RedundantConstructorKeyword", `
package test
class Foo constructor(val x: Int)
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Fix == nil {
		t.Fatal("expected fix populated (Kotlin tree-sitter emits class_parameter children directly, not a class_parameters wrapper)")
	}
	// The fix replaces the leading whitespace + `constructor` keyword
	// with empty bytes, ending exactly at the `(` of the parameter list.
	if findings[0].Fix.Replacement != "" {
		t.Errorf("expected empty replacement, got %q", findings[0].Fix.Replacement)
	}
	if findings[0].Fix.EndByte <= findings[0].Fix.StartByte {
		t.Errorf("expected non-empty cut range, got [%d,%d)", findings[0].Fix.StartByte, findings[0].Fix.EndByte)
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
fun f() {
    val x: String = "hello"
}
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

func TestRedundantUnnecessaryInnerClass_OuterMemberUseKeepsInner(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryInnerClass", `
package test
class Outer<T>(private val ctorProp: Int) {
    val field = 1
    fun method() = 2
    inner class UsesField { fun f() = field }
    inner class UsesMethod { fun f() = method() }
    inner class UsesCtor { fun f() = ctorProp }
    inner class UsesTypeParam { var t: T? = null }
    inner class UsesSibling { fun f() = Sibling() }
    inner class Sibling { fun g() = field }
    inner class Unneeded { fun f() = 1 }
}
`)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "Unneeded") {
		t.Fatalf("expected only Unneeded, got %v", findings)
	}
}

func TestRedundantUnnecessaryInnerClass_OuterWithSupertypeIsSkipped(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryInnerClass", `
package test
class Outer : Base() {
    inner class Inner { fun f() = inheritedMember() }
}
`)
	if len(findings) != 0 {
		t.Fatalf("inherited members cannot be ruled out, got %v", findings)
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

func TestUnnecessaryBackticks_CommentsAndKDocNotInspected(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryBackticks", "package a\n/** Uses `backtick` in docs and [`foo`]. */\nfun f() {}\n// comment `ticks`\nfun g() {\n  /* `a` */\n  val s = \"`b`\"\n}\n")
	if len(findings) != 0 {
		t.Fatalf("expected no findings in comments, KDoc or strings, got %d", len(findings))
	}
}

func TestRedundantUnnecessaryBackticks_SkipsKDocReadAsCodeAfterParseError(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryBackticks", "package test\n\nclass A {\n  fun f() {}\n  }\n}\n\n/**\n * Mirrors the legacy `LegacyBinder`, see \"the docs\".\n */\nfun g() {}\n")
	if len(findings) != 0 {
		t.Fatalf("backticks inside a comment are code spans, got %d", len(findings))
	}
}

func TestRedundantUnnecessaryBackticks_StillReportsBackticksAfterCommentsAndStrings(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryBackticks", "package test\n// a `note`\nval s = \"/* not a comment\"\nval `myVar` = s\n")
	if len(findings) != 1 {
		t.Fatalf("expected the code identifier to be reported, got %d", len(findings))
	}
}

func TestRedundantConstructorKeyword_FixJoinsLineBreak(t *testing.T) {
	findings := runRuleByName(t, "RedundantConstructorKeyword", "\npackage test\n\nclass Foo\nconstructor(\n    val a: Int,\n) {\n}\n")
	if len(findings) != 1 || findings[0].Fix == nil {
		t.Fatalf("expected one finding with fix, got %+v", findings)
	}
	src := "\npackage test\n\nclass Foo\nconstructor(\n    val a: Int,\n) {\n}\n"
	fix := findings[0].Fix
	if got := src[:fix.StartByte] + fix.Replacement + src[fix.EndByte:]; got != "\npackage test\n\nclass Foo(\n    val a: Int,\n) {\n}\n" {
		t.Errorf("expected the constructor parameters to follow the class name, got %q", got)
	}
}

// detekt checks local variables only and looks at literals, constructor calls and object references, so neither
// property of the class nor a factory-function call is reported.
func TestRedundantExplicitType_OnlyLocalLiteralsConstructorsAndObjects(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "RedundantExplicitType", `
package test

class Holder {
    val a: Int = 1
    fun f() {
        val b: MutableList<String> = mutableListOf()
        val c: Holder = Holder()
        val d: Int = 2
    }
}
`)
	if len(findings) != 2 || findings[0].Line != 8 || findings[1].Line != 9 {
		t.Fatalf("expected the constructor call and the literal local, got %+v", findings)
	}
}

func TestUnnecessaryParentheses_KeepsParensThatChangeMeaning(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryParentheses", `
package test

class A {
    var retain = false
    fun onCreate(a: Boolean) {
        @Suppress("DEPRECATION")
        retain = true
        val s1 = (if (a) "x" else "yy").length
        val s2 = (if (a) 1 else 2) + 3
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %+v", findings)
	}
}

func TestUnnecessaryInnerClass_KeepsInnerWhenOuterMembersAreUsed(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryInnerClass", `
package test

class Outer<T>(val q: Int) {
    val x = 1
    inner class UsesMember {
        fun foo() = x + 1
    }
    inner class UsesCtorProperty {
        fun foo() = q
    }
    inner class UsesTypeParameter {
        fun foo(value: T): T = value
    }
    inner class Standalone(val p: Int) {
        fun foo(a: Int): Int {
            val l = a + p
            return l
        }
    }
}
`)
	if len(findings) != 1 || findings[0].Line != 15 {
		t.Fatalf("expected only the standalone inner class, got %+v", findings)
	}
	if findings[0].Fix == nil || findings[0].Fix.Replacement != "" {
		t.Fatalf("expected a deletion fix, got %+v", findings[0].Fix)
	}
}

func TestUselessCallOnNotNull_ListOfNotNullFixKeepsArguments(t *testing.T) {
	src := `package test
val a = listOfNotNull("a", "b")
`
	findings := runRuleByNameWithResolver(t, "UselessCallOnNotNull", src)
	if len(findings) != 1 || findings[0].Fix == nil {
		t.Fatalf("expected one fixable finding, got %v", findings)
	}
	f := findings[0]
	if got := src[:f.Fix.StartByte] + f.Fix.Replacement + src[f.Fix.EndByte:]; !strings.Contains(got, `listOf("a", "b")`) {
		t.Errorf("unexpected fix: %s", got)
	}
}

func TestRedundantExplicitType_GenericTypeArgumentsAreNotInferable(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "RedundantExplicitType", `
package test
fun f() {
    val a: List<Int> = emptyList()
    val b: MutableList<String> = mutableListOf()
    val c: List<Any> = listOf("a")
    val d: Int = 1
}
`)
	if len(findings) != 1 || findings[0].Line != 7 {
		t.Fatalf("only the plain Int declaration is redundant, got %v", findings)
	}
}

func TestUselessCallOnNotNull_ListOfNotNullWithReceiverIsNotStdlib(t *testing.T) {
	src := `package test
fun f(l: Helper) {
    val a = l.listOfNotNull("a")
}
`
	if got := runRuleByNameWithResolver(t, "UselessCallOnNotNull", src); len(got) != 0 {
		t.Fatalf("a member or extension call is not the stdlib function, got %v", got)
	}
}

func TestUnnecessaryParentheses_KeepsIfExpressionOperands(t *testing.T) {
	src := `package test
fun f(flag: Boolean) {
    val a = (if (flag) 1 else 2) + 1
    val b = (if (flag) 1 else 2).toString()
    val c = (if (flag) 1 else 2)
}
`
	findings := runRuleByName(t, "UnnecessaryParentheses", src)
	if len(findings) != 1 || findings[0].Line != 5 {
		t.Fatalf("only the whole right-hand side is redundant, got %v", findings)
	}
}

func TestUselessCallOnNotNull_MessagesMatchDetekt(t *testing.T) {
	src := `package test
val a = "hello".orEmpty()
val b = "hello".isNullOrEmpty()
val c = "hello".isNullOrBlank()
val d = listOfNotNull("a", "b")
`
	findings := runRuleByNameWithResolver(t, "UselessCallOnNotNull", src)
	want := []string{
		"Remove redundant call to orEmpty",
		"Replace isNullOrEmpty with isEmpty",
		"Replace isNullOrBlank with isBlank",
		"Replace listOfNotNull with listOf",
	}
	if len(findings) != len(want) {
		t.Fatalf("expected %d findings, got %v", len(want), findings)
	}
	for i, f := range findings {
		if f.Message != want[i] {
			t.Errorf("finding %d message = %q, want %q", i, f.Message, want[i])
		}
	}
}
