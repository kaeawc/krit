package rules_test

import (
	"strings"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
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
	if rule.Needs.Has(api.NeedsResolver) || rule.Needs.Has(api.NeedsOracle) ||
		rule.Needs.Has(api.NeedsParsedFiles) || rule.Needs.Has(api.NeedsCrossFile) {
		t.Fatalf("TooGenericExceptionCaught should stay AST/import-only; got Needs=%b", rule.Needs)
	}
	if rule.TypeInfo != (api.TypeInfoHint{}) {
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

func TestUnreachableCode_NegativeMultilineReturnGenericCast(t *testing.T) {
	findings := runRuleByName(t, "UnreachableCode", `
package test

class Flow<T>

fun <U> getResult(): Flow<Map<String, List<U>>> = Flow()

fun <U> create(): Flow<Map<String, List<U>>> {
    return getResult()
        as Flow<Map<String, List<U>>>
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for multiline generic return cast, got %d: %#v", len(findings), findings)
	}
}

func TestUnreachableCode_NegativeMultilineReturnCallCast(t *testing.T) {
	findings := runRuleByName(t, "UnreachableCode", `
package test

class Presenter(val value: String)

fun <T> create(value: String): T {
    return Presenter(
        value = value,
    )
        as T
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for parenthesized multiline return call cast, got %d: %#v", len(findings), findings)
	}
}

func TestUnreachableCode_NegativeMultilineReturnObjectCast(t *testing.T) {
	findings := runRuleByName(t, "UnreachableCode", `
package test

interface Accessor {
    fun scheduler(): String
}

fun <T> create(fakeScheduler: String): T {
    return object : Accessor {
        override fun scheduler() = fakeScheduler
    }
        as T
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for multiline object-expression return cast, got %d: %#v", len(findings), findings)
	}
}

func TestUnreachableCode_NegativeLongMultilineReturnObjectCast(t *testing.T) {
	findings := runRuleByName(t, "UnreachableCode", `
package test

interface Accessor {
    fun scheduler(): String
}

fun <T> create(fakeScheduler: String): T {
    return object : Accessor {
        override fun scheduler() = fakeScheduler
`+strings.Repeat("        fun extra() = Unit\n", 45)+`
    }
        as T
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for long multiline object-expression return cast, got %d: %#v", len(findings), findings)
	}
}

func TestUnreachableCode_NegativeAfterLoggerError(t *testing.T) {
	findings := runRuleByName(t, "UnreachableCode", `
package test

class Logger {
    fun error(message: String) {}
}

fun main(logger: Logger) {
    logger.error("failed")
    println("still reachable")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings after logger.error(), got %d", len(findings))
	}
}

func TestUnreachableCode_AfterExitProcess_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnreachableCode", `
package test
fun main() {
    exitProcess(0)
    println("unreachable")
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for unreachable code after exitProcess()")
	}
}

func TestUnreachableCode_AfterQualifiedExitProcess_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnreachableCode", `
package test
fun main() {
    kotlin.system.exitProcess(0)
    println("unreachable")
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for unreachable code after kotlin.system.exitProcess()")
	}
}

func TestUnreachableCode_AfterWorkspaceNothingFunc_Positive(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnreachableCode", `
package test

private fun failHard(msg: String): Nothing = throw IllegalStateException(msg)

fun main() {
    failHard("boom")
    println("unreachable")
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for unreachable code after workspace Nothing-returning function")
	}
}

func TestUnreachableCode_NegativeUserExitProcessOnReceiver(t *testing.T) {
	// A user-defined `exitProcess` method on an arbitrary receiver does not
	// match the strict qualifier allow-list, and the resolver cannot
	// confirm a Nothing return type for an unknown method. The rule must
	// not fire.
	findings := runRuleByName(t, "UnreachableCode", `
package test

class Watchdog {
    fun exitProcess(code: Int) {}
}

fun main(w: Watchdog) {
    w.exitProcess(0)
    println("still reachable")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings after Watchdog.exitProcess(), got %d", len(findings))
	}
}

// --- MissingReturn ---

func TestMissingReturn_FallThrough_Positive(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "MissingReturn", `
package test
fun foo(x: Int): Int {
    println(x)
}
`)
	if len(findings) == 0 {
		t.Fatal("expected MissingReturn finding for fall-through body")
	}
}

func TestMissingReturn_NormalReturn_Negative(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "MissingReturn", `
package test
fun foo(x: Int): Int {
    return x + 1
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestMissingReturn_ExpressionBody_Negative(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "MissingReturn", `
package test
fun foo(x: Int): Int = x + 1
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings on expression body, got %d", len(findings))
	}
}

func TestMissingReturn_UnitReturnType_Negative(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "MissingReturn", `
package test
fun foo(x: Int): Unit {
    println(x)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for explicit Unit return, got %d", len(findings))
	}
}

func TestMissingReturn_ImplicitUnit_Negative(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "MissingReturn", `
package test
fun foo(x: Int) {
    println(x)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for implicit Unit, got %d", len(findings))
	}
}

func TestMissingReturn_NothingReturnType_Negative(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "MissingReturn", `
package test
fun crash(): Nothing {
    throw IllegalStateException("boom")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for Nothing return type, got %d", len(findings))
	}
}

func TestMissingReturn_ExhaustiveIfReturns_Negative(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "MissingReturn", `
package test
fun foo(x: Int): Int {
    if (x > 0) {
        return x
    } else {
        return -x
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for exhaustive-if returns, got %d", len(findings))
	}
}

func TestMissingReturn_NothingCallTerminates_Negative(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "MissingReturn", `
package test
fun foo(x: Int): String {
    TODO("not yet")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when TODO terminates, got %d", len(findings))
	}
}

func TestMissingReturn_AbstractFunction_Negative(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "MissingReturn", `
package test
abstract class Holder {
    abstract fun get(): Int
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings on abstract function, got %d", len(findings))
	}
}
