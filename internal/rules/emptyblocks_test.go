package rules_test

import "testing"

// --- EmptyFunctionBlock ---

func TestEmptyFunctionBlock_Positive(t *testing.T) {
	findings := runRuleByName(t, "EmptyFunctionBlock", `
fun foo() {}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for empty function block")
	}
}

func TestEmptyFunctionBlock_Negative(t *testing.T) {
	findings := runRuleByName(t, "EmptyFunctionBlock", `
fun foo() {
    println("hello")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- EmptyClassBlock ---

func TestEmptyClassBlock_Positive(t *testing.T) {
	findings := runRuleByName(t, "EmptyClassBlock", `
class Foo {}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for empty class body")
	}
}

func TestEmptyClassBlock_Negative(t *testing.T) {
	findings := runRuleByName(t, "EmptyClassBlock", `
class Foo {
    val x = 1
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- EmptyCatchBlock ---

func TestEmptyCatchBlock_Positive(t *testing.T) {
	findings := runRuleByName(t, "EmptyCatchBlock", `
fun foo() {
    try {
        doSomething()
    } catch (e: Exception) {}
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for empty catch block")
	}
}

func TestEmptyCatchBlock_Negative(t *testing.T) {
	findings := runRuleByName(t, "EmptyCatchBlock", `
fun foo() {
    try {
        doSomething()
    } catch (e: Exception) {
        e.printStackTrace()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- EmptyIfBlock ---

func TestEmptyIfBlock_Positive(t *testing.T) {
	findings := runRuleByName(t, "EmptyIfBlock", `
fun foo() {
    val x = true
    if (x) {}
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for empty if block")
	}
}

func TestEmptyIfBlock_Negative(t *testing.T) {
	findings := runRuleByName(t, "EmptyIfBlock", `
fun foo() {
    val x = true
    if (x) {
        println("yes")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- EmptyElseBlock ---

func TestEmptyElseBlock_Positive(t *testing.T) {
	findings := runRuleByName(t, "EmptyElseBlock", `
fun foo() {
    val x = true
    if (x) {
        doSomething()
    } else {}
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for empty else block")
	}
}

func TestEmptyElseBlock_Negative(t *testing.T) {
	findings := runRuleByName(t, "EmptyElseBlock", `
fun foo() {
    val x = true
    if (x) {
        doSomething()
    } else {
        doOther()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- EmptyWhenBlock ---

func TestEmptyWhenBlock_Positive(t *testing.T) {
	findings := runRuleByName(t, "EmptyWhenBlock", `
fun foo() {
    val x = 1
    when (x) {}
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for empty when block")
	}
}

func TestEmptyWhenBlock_Negative(t *testing.T) {
	findings := runRuleByName(t, "EmptyWhenBlock", `
fun foo() {
    val x = 1
    when (x) {
        1 -> println("one")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- EmptyForBlock ---

func TestEmptyForBlock_Positive(t *testing.T) {
	findings := runRuleByName(t, "EmptyForBlock", `
fun foo() {
    val list = listOf(1, 2, 3)
    for (i in list) {}
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for empty for block")
	}
}

func TestEmptyForBlock_Negative(t *testing.T) {
	findings := runRuleByName(t, "EmptyForBlock", `
fun foo() {
    val list = listOf(1, 2, 3)
    for (i in list) {
        println(i)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- EmptyWhileBlock ---

func TestEmptyWhileBlock_Positive(t *testing.T) {
	findings := runRuleByName(t, "EmptyWhileBlock", `
fun foo() {
    while (true) {}
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for empty while block")
	}
}

func TestEmptyWhileBlock_Negative(t *testing.T) {
	findings := runRuleByName(t, "EmptyWhileBlock", `
fun foo() {
    while (true) {
        Thread.sleep(100)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- EmptyTryBlock ---

func TestEmptyTryBlock_Positive(t *testing.T) {
	findings := runRuleByName(t, "EmptyTryBlock", `
fun foo() {
    try {} catch (e: Exception) {
        handle()
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for empty try block")
	}
}

func TestEmptyTryBlock_Negative(t *testing.T) {
	findings := runRuleByName(t, "EmptyTryBlock", `
fun foo() {
    try {
        doSomething()
    } catch (e: Exception) {
        handle()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- EmptyFinallyBlock ---

func TestEmptyFinallyBlock_Positive(t *testing.T) {
	findings := runRuleByName(t, "EmptyFinallyBlock", `
fun foo() {
    try {
        doSomething()
    } finally {}
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for empty finally block")
	}
}

func TestEmptyFinallyBlock_Negative(t *testing.T) {
	findings := runRuleByName(t, "EmptyFinallyBlock", `
fun foo() {
    try {
        doSomething()
    } finally {
        cleanup()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- EmptyInitBlock ---

func TestEmptyInitBlock_Positive(t *testing.T) {
	findings := runRuleByName(t, "EmptyInitBlock", `
class Foo {
    init {}
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for empty init block")
	}
}

func TestEmptyInitBlock_Negative(t *testing.T) {
	findings := runRuleByName(t, "EmptyInitBlock", `
class Foo {
    init {
        println("initialized")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- EmptyDoWhileBlock ---

func TestEmptyDoWhileBlock_Positive(t *testing.T) {
	findings := runRuleByName(t, "EmptyDoWhileBlock", `
fun foo() {
    do {} while (true)
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for empty do-while block")
	}
}

func TestEmptyDoWhileBlock_Negative(t *testing.T) {
	findings := runRuleByName(t, "EmptyDoWhileBlock", `
fun foo() {
    do {
        Thread.sleep(100)
    } while (true)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- EmptyDefaultConstructor ---

func TestEmptyDefaultConstructor_Positive(t *testing.T) {
	findings := runRuleByName(t, "EmptyDefaultConstructor", `
class Foo constructor() {
    val x = 1
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for empty default constructor")
	}
}

func TestEmptyDefaultConstructor_Negative(t *testing.T) {
	findings := runRuleByName(t, "EmptyDefaultConstructor", `
class Foo(val name: String) {
    val x = 1
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestEmptyDefaultConstructor_IgnoresDIAnnotatedConstructors(t *testing.T) {
	findings := runRuleByName(t, "EmptyDefaultConstructor", `
package test

import dev.zacsweers.metro.ContributesBinding
import dev.zacsweers.metro.Inject

interface Service

@ContributesBinding(AppScope::class)
@Inject
class RealService() : Service

object AppScope
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for DI annotated empty constructor, got %d", len(findings))
	}
}

func TestEmptyDefaultConstructor_UsesLocalASTOnly(t *testing.T) {
	rule := buildRuleIndex()["EmptyDefaultConstructor"]
	if rule == nil {
		t.Fatal("EmptyDefaultConstructor rule is not registered")
	}
	if rule.Needs != 0 {
		t.Fatalf("EmptyDefaultConstructor should remain AST-only, got needs %v", rule.Needs)
	}
	if rule.OracleCallTargets != nil || rule.OracleDeclarationNeeds != nil || rule.Oracle != nil {
		t.Fatal("EmptyDefaultConstructor should not declare oracle metadata")
	}
}

// --- EmptyKotlinFile ---

func TestEmptyKotlinFile_Positive(t *testing.T) {
	findings := runRuleByName(t, "EmptyKotlinFile", `
package test
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for empty Kotlin file")
	}
}

func TestEmptyKotlinFile_Negative(t *testing.T) {
	findings := runRuleByName(t, "EmptyKotlinFile", `
package test
fun hello() {}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- EmptySecondaryConstructor ---

func TestEmptySecondaryConstructor_Positive(t *testing.T) {
	findings := runRuleByName(t, "EmptySecondaryConstructor", `
class Foo(val x: Int) {
    constructor(x: Int, y: Int) : this(x) {}
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for empty secondary constructor")
	}
}

func TestEmptySecondaryConstructor_Negative(t *testing.T) {
	findings := runRuleByName(t, "EmptySecondaryConstructor", `
class Foo(val x: Int) {
    constructor(x: Int, y: Int) : this(x) {
        println("initialized with y=$y")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}
