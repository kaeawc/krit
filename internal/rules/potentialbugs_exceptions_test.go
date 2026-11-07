package rules_test

import (
	"strings"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
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

// detekt has no test-source exemption for TooGenericExceptionThrown in its default config.
func TestTooGenericExceptionThrown_ReportsTestSourcesLikeDetekt(t *testing.T) {
	findings := runRuleByNameOnPath(t, "TooGenericExceptionThrown", "src/test/kotlin/FooTest.kt", `
package test
fun main() {
    throw RuntimeException("boom")
}
`)
	if len(findings) != 1 || findings[0].Message != "RuntimeException is a too generic Exception. Prefer throwing specific exceptions that indicate a specific error case." {
		t.Fatalf("expected one finding with detekt's message for test sources, got %v", findings)
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

func TestUnreachableCatchBlock_MessageNamesTypesBySimpleName(t *testing.T) {
	findings := runRuleByName(t, "UnreachableCatchBlock", `
package test

import java.io.IOException

fun main() {
    try {
        doSomething()
    } catch (e: java.io.IOException) {
        handle(e)
    } catch (e: IOException) {
        handleAgain(e)
    }
}
`)
	for _, f := range findings {
		if strings.Contains(f.Message, "java.io.") {
			t.Fatalf("message must use simple names: %s", f.Message)
		}
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

// java.net.SocketTimeoutException extends InterruptedIOException, not
// SocketException, so a SocketTimeoutException clause after a
// SocketException clause is reachable; after InterruptedIOException it is not.
func TestUnreachableCatchBlock_SocketTimeoutIsAnInterruptedIOException(t *testing.T) {
	for _, run := range []struct {
		name string
		fn   func(*testing.T, string, string) []scanner.Finding
	}{
		{"table", runRuleByName},
		{"resolver", runRuleByNameWithResolver},
	} {
		t.Run(run.name, func(t *testing.T) {
			findings := run.fn(t, "UnreachableCatchBlock", `
package test
import java.io.InterruptedIOException
import java.net.SocketException
import java.net.SocketTimeoutException
fun reachable() {
    try {
        doSomething()
    } catch (e: SocketException) {
        handle(e)
    } catch (e: SocketTimeoutException) {
        handleTimeout(e)
    }
}
fun unreachable() {
    try {
        doSomething()
    } catch (e: InterruptedIOException) {
        handle(e)
    } catch (e: SocketTimeoutException) {
        handleTimeout(e)
    }
}
`)
			if len(findings) != 1 || findings[0].Line != 20 {
				t.Fatalf("want one finding on line 20 (the clause after InterruptedIOException), got %v", findings)
			}
		})
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

func TestTooGenericExceptionCaught_HonoursSuppressOnCatchParameter(t *testing.T) {
	findings := runRuleByName(t, "TooGenericExceptionCaught", `
package test
fun main() {
    try {
        doSomething()
    } catch (@Suppress("TooGenericExceptionCaught") e: Exception) {
        cleanup()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected suppressed catch parameter to produce no findings, got %d", len(findings))
	}
}

func TestTooGenericExceptionCaught_HonoursSuppressOnEnclosingFunction(t *testing.T) {
	findings := runRuleByName(t, "TooGenericExceptionCaught", `
package test
@Suppress("TooGenericExceptionCaught")
fun main() {
    try {
        doSomething()
    } catch (e: Exception) {
        cleanup()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected suppressed function to produce no findings, got %d", len(findings))
	}
}

func TestTooGenericExceptionThrown_IgnoresNonThrowableErrorIdentifier(t *testing.T) {
	findings := runRuleByName(t, "TooGenericExceptionThrown", `
package test
sealed interface UiState {
    class Error(val message: String) : UiState
}
fun render(state: UiState) {
    throw IllegalStateException(UiState.Error("x").message)
}
fun fail(): Nothing = throw wrap(Error("x"))
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestTooGenericExceptionThrown_IgnoresReturnedAndNestedErrorCalls(t *testing.T) {
	findings := runRuleByName(t, "TooGenericExceptionThrown", `
package test
import other.Error
fun a(): Any = run { return@run Error("x") }
fun b(flag: Boolean): Any {
    return Error("x")
}
fun c(flag: Boolean): Nothing = throw if (flag) wrap(Error("x")) else wrap(Error("y"))
fun d(): Nothing = throw wrap(Error("x"))
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %v", findings)
	}
}

func TestPrintStackTrace_NegativeStreamArgument(t *testing.T) {
	findings := runRuleByName(t, "PrintStackTrace", `
package test
import java.io.PrintWriter
import java.io.StringWriter
fun render(e: Exception): String {
    val sink = StringWriter()
    e.printStackTrace(PrintWriter(sink))
    return sink.toString()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for printStackTrace(PrintWriter), got %d", len(findings))
	}
}

func TestPrintStackTrace_NegativeInsideTimberTree(t *testing.T) {
	findings := runRuleByName(t, "PrintStackTrace", `
package test
import timber.log.Timber
class FileTree : Timber.Tree() {
    override fun log(priority: Int, tag: String?, message: String, t: Throwable?) {
        t?.printStackTrace()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings inside a logging sink, got %d", len(findings))
	}
}

func TestTooGenericExceptionThrown_FakeClassIsNotExempt(t *testing.T) {
	findings := runRuleByName(t, "TooGenericExceptionThrown", `
package test
class FakeRepository {
    fun load(): Nothing = throw Exception("simulated failure")
}
class Repository {
    fun load(): Nothing = throw Exception("real failure")
}
`)
	if len(findings) != 2 {
		t.Errorf("detekt flags both classes, got %d", len(findings))
	}
}

func TestUnreachableCatchBlock_CancellationExceptionIsAnIllegalStateException(t *testing.T) {
	cases := []struct {
		name string
		code string
		want int
	}{
		{"cancellation after illegal state", `
package test
import kotlin.coroutines.cancellation.CancellationException
fun f() {
    try { run() } catch (e: IllegalStateException) { } catch (e: CancellationException) { }
}
`, 1},
		{"illegal state after cancellation", `
package test
import kotlin.coroutines.cancellation.CancellationException
fun f() {
    try { run() } catch (e: CancellationException) { } catch (e: IllegalStateException) { }
}
`, 0},
		{"cancellation after unrelated", `
package test
import kotlin.coroutines.cancellation.CancellationException
fun f() {
    try { run() } catch (e: IllegalArgumentException) { } catch (e: CancellationException) { }
}
`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := runRuleByName(t, "UnreachableCatchBlock", tc.code)
			if len(findings) != tc.want {
				t.Fatalf("expected %d findings, got %d: %v", tc.want, len(findings), findings)
			}
		})
	}
}

func TestTooGenericExceptionCaught_ScriptsAreNotExempt(t *testing.T) {
	const code = `
fun run() {
    try {
        work()
    } catch (e: Exception) {
        println("failed: ${e.message}")
    }
}
`
	if got := runRuleByNameOnPath(t, "TooGenericExceptionCaught", "tool.main.kts", code); len(got) != 1 {
		t.Fatalf("expected 1 finding in a .main.kts script, got %d", len(got))
	}
	if got := runRuleByNameOnPath(t, "TooGenericExceptionCaught", "Tool.kt", code); len(got) != 1 {
		t.Fatalf("expected 1 finding in app code, got %d", len(got))
	}
}

func TestPrintStackTrace_FixOnlyRemovesLineHoldingJustTheCall(t *testing.T) {
	alone := runRuleByName(t, "PrintStackTrace", "fun f() {\n    try { g() } catch (e: Exception) {\n        e.printStackTrace()\n    }\n}\nfun g() {}\n")
	if len(alone) != 1 || alone[0].Fix == nil {
		t.Fatalf("expected one fixable finding for a line holding only the call, got %v", alone)
	}
	shared := runRuleByName(t, "PrintStackTrace", "fun f() {\n    try { g() } catch (e: Exception) { e.printStackTrace() }\n}\nfun g() {}\n")
	if len(shared) != 1 {
		t.Fatalf("expected one finding, got %d", len(shared))
	}
	if shared[0].Fix != nil {
		t.Fatalf("the fix must not delete a line that holds other code, got %+v", shared[0].Fix)
	}
}

func TestMissingReturn_Termination(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"try-catch-all-return", "try { return 1 } catch (e: Exception) { throw e }", 0},
		{"finally-only", "try { return 1 } finally { println() }", 0},
		{"catch-falls-through", "try { return 1 } catch (e: Exception) { println() }", 1},
		{"while-true-return", "while (true) { if (x > 1) return 2 }", 0},
		{"while-true-break", "while (true) { if (x > 1) break }", 1},
		{"while-true-nested-break", "while (true) { for (i in 1..2) { break } }", 0},
		{"let-return", "x.let { return 1 }", 0},
		{"for-loop", "for (i in 1..2) { }", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := runRuleByNameWithResolver(t, "MissingReturn", "package test\nfun foo(x: Int): Int {\n    "+tc.body+"\n}\n")
			if len(findings) != tc.want {
				t.Fatalf("want %d findings, got %d", tc.want, len(findings))
			}
		})
	}
}
