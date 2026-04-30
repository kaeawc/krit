package rules_test

import (
	"testing"

	v2rules "github.com/kaeawc/krit/internal/rules/v2"
)

// --- PrintStackTrace ---

func TestPrintStackTrace_Positive(t *testing.T) {
	findings := runRuleByName(t, "PrintStackTrace", `
package test
fun main() {
    try {
        doSomething()
    } catch (e: Exception) {
        e.printStackTrace()
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for printStackTrace()")
	}
}

func TestPrintStackTrace_Negative(t *testing.T) {
	findings := runRuleByName(t, "PrintStackTrace", `
package test
fun main() {
    try {
        doSomething()
    } catch (e: Exception) {
        logger.error("Error", e)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestPrintStackTrace_NegativeLocalLookalike(t *testing.T) {
	findings := runRuleByName(t, "PrintStackTrace", `
package test

class Printer {
    fun printStackTrace() {}
}

fun main(printer: Printer) {
    printer.printStackTrace()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for local printStackTrace lookalike, got %d", len(findings))
	}
}

func TestPrintStackTrace_NegativeNonCaughtReceiver(t *testing.T) {
	findings := runRuleByName(t, "PrintStackTrace", `
package test
fun main(other: Throwable) {
    try {
        doSomething()
    } catch (e: Exception) {
        other.printStackTrace()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when printStackTrace receiver is not the caught exception, got %d", len(findings))
	}
}

// --- TooGenericExceptionCaught ---

func TestTooGenericExceptionCaught_Positive(t *testing.T) {
	findings := runRuleByName(t, "TooGenericExceptionCaught", `
package test
fun main() {
    try {
        doSomething()
    } catch (e: Exception) {
        println("caught")
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for catching Exception")
	}
}

func TestTooGenericExceptionCaught_Negative(t *testing.T) {
	findings := runRuleByName(t, "TooGenericExceptionCaught", `
package test
fun main() {
    try {
        doSomething()
    } catch (e: IllegalArgumentException) {
        handle(e)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestTooGenericExceptionCaught_DoesNotRequireTypeContext(t *testing.T) {
	rule := buildRuleIndex()["TooGenericExceptionCaught"]
	if rule == nil {
		t.Fatal("TooGenericExceptionCaught rule not found")
	}
	if rule.Needs.Has(v2rules.NeedsResolver) || rule.Needs.Has(v2rules.NeedsOracle) ||
		rule.Needs.Has(v2rules.NeedsParsedFiles) || rule.Needs.Has(v2rules.NeedsCrossFile) {
		t.Fatalf("TooGenericExceptionCaught should stay AST/import-only; got Needs=%b", rule.Needs)
	}
	if rule.TypeInfo != (v2rules.TypeInfoHint{}) {
		t.Fatalf("TooGenericExceptionCaught TypeInfo=%+v, want zero value", rule.TypeInfo)
	}
}

// --- TooGenericExceptionThrown ---

func TestTooGenericExceptionThrown_Positive(t *testing.T) {
	findings := runRuleByName(t, "TooGenericExceptionThrown", `
package test
fun main() {
    throw Exception("something went wrong")
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for throwing Exception")
	}
}

func TestTooGenericExceptionThrown_Negative(t *testing.T) {
	findings := runRuleByName(t, "TooGenericExceptionThrown", `
package test
fun main() {
    throw IllegalArgumentException("bad arg")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestTooGenericExceptionThrown_IgnoresTestSources(t *testing.T) {
	findings := runRuleByNameOnPath(t, "TooGenericExceptionThrown", "src/test/kotlin/FooTest.kt", `
package test
fun main() {
    throw RuntimeException("boom")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for test sources, got %d", len(findings))
	}
}

// --- UnreachableCatchBlock ---

func TestUnreachableCatchBlock_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnreachableCatchBlock", `
package test
fun main() {
    try {
        doSomething()
    } catch (e: Exception) {
        handle(e)
    } catch (e: Exception) {
        handleAgain(e)
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for duplicate catch block")
	}
}

func TestUnreachableCatchBlock_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnreachableCatchBlock", `
package test
fun main() {
    try {
        doSomething()
    } catch (e: IllegalArgumentException) {
        handle(e)
    } catch (e: IllegalStateException) {
        handleOther(e)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- UnreachableCode ---

func TestUnreachableCode_AfterReturn_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnreachableCode", `
package test
fun main() {
    return
    println("unreachable")
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for unreachable code after return")
	}
}

func TestUnreachableCode_AfterReturn_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnreachableCode", `
package test
fun main() {
    println("reachable")
    return
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestUnreachableCode_NegativeMultilineReturnCast(t *testing.T) {
	findings := runRuleByName(t, "UnreachableCode", `
package test

open class View
class CircuitScreenComposeView : View()

fun showInflatedView(): View = CircuitScreenComposeView()

fun create(): CircuitScreenComposeView {
    return showInflatedView()
        as CircuitScreenComposeView
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for multiline return expression cast, got %d", len(findings))
	}
}
