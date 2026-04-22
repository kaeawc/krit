package rules_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// --- ExceptionRaisedInUnexpectedLocation ---

func TestExc_ExceptionRaisedInUnexpectedLocation_Positive(t *testing.T) {
	findings := runRuleByName(t, "ExceptionRaisedInUnexpectedLocation", `
fun equals(other: Any?): Boolean {
    throw IllegalStateException("not comparable")
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for throw inside equals()")
	}
}

func TestExc_ExceptionRaisedInUnexpectedLocation_Negative(t *testing.T) {
	findings := runRuleByName(t, "ExceptionRaisedInUnexpectedLocation", `
fun compute(): Int {
    throw UnsupportedOperationException("nope")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- InstanceOfCheckForException ---

func TestExc_InstanceOfCheckForException_Positive(t *testing.T) {
	findings := runRuleByName(t, "InstanceOfCheckForException", `
fun test() {
    try {
        doWork()
    } catch (e: Exception) {
        if (e is IOException) {
            println("io")
        }
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for is-check inside catch")
	}
}

func TestExc_InstanceOfCheckForException_Negative(t *testing.T) {
	findings := runRuleByName(t, "InstanceOfCheckForException", `
fun test() {
    try {
        doWork()
    } catch (e: IOException) {
        log(e)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- NotImplementedDeclaration ---

func TestExc_NotImplementedDeclaration_Positive(t *testing.T) {
	findings := runRuleByName(t, "NotImplementedDeclaration", `
fun process() {
    TODO("implement later")
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for TODO() call")
	}
}

func TestExc_NotImplementedDeclaration_Negative(t *testing.T) {
	findings := runRuleByName(t, "NotImplementedDeclaration", `
fun process() {
    println("done")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- RethrowCaughtException ---

func TestExc_RethrowCaughtException_Positive(t *testing.T) {
	findings := runRuleByName(t, "RethrowCaughtException", `
fun test() {
    try {
        doWork()
    } catch (e: Exception) {
        throw e
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for rethrow of caught exception")
	}
}

func TestExc_RethrowCaughtException_Negative(t *testing.T) {
	findings := runRuleByName(t, "RethrowCaughtException", `
fun test() {
    try {
        doWork()
    } catch (e: Exception) {
        log(e)
        throw RuntimeException("wrapped", e)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- ReturnFromFinally ---

func TestExc_ReturnFromFinally_Positive(t *testing.T) {
	findings := runRuleByName(t, "ReturnFromFinally", `
fun test(): Int {
    try {
        return 1
    } finally {
        return 2
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for return inside finally")
	}
}

func TestExc_ReturnFromFinally_Negative(t *testing.T) {
	findings := runRuleByName(t, "ReturnFromFinally", `
fun test(): Int {
    try {
        return 1
    } finally {
        cleanup()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- SwallowedException ---

func TestExc_SwallowedException_Positive(t *testing.T) {
	findings := runRuleByName(t, "SwallowedException", `
fun test() {
    try {
        doWork()
    } catch (e: Exception) {
        println("failed")
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for swallowed exception")
	}
}

func TestExc_SwallowedException_Negative(t *testing.T) {
	findings := runRuleByName(t, "SwallowedException", `
fun test() {
    try {
        doWork()
    } catch (e: Exception) {
        Log.e("tag", "failed", e)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestExc_SwallowedException_ASTPositiveCases(t *testing.T) {
	cases := map[string]string{
		"empty catch": `
fun test() {
    try {
        work()
    } catch (e: java.io.IOException) {
    }
}
`,
		"comment only": `
fun test() {
    try {
        work()
    } catch (e: java.io.IOException) {
        // ignored: e throw log handle
    }
}
`,
		"message only throw": `
fun test() {
    try {
        work()
    } catch (e: java.io.IOException) {
        throw IllegalStateException(e.message)
    }
}
`,
		"message alias throw": `
fun test() {
    try {
        work()
    } catch (e: java.io.IOException) {
        val msg = e.message
        throw IllegalStateException(
            msg
        )
    }
}
`,
		"message only logging": `
fun test() {
    try {
        work()
    } catch (e: java.io.IOException) {
        logger.warn(e.message)
    }
}
`,
		"unresolved logger name": `
fun test(logger: Any) {
    try {
        work()
    } catch (e: java.io.IOException) {
        logger.warn("failed", e)
    }
}
`,
		"unresolved same name api": `
class WarningSink {
    fun warn(value: Any?) {}
}
fun test(sink: WarningSink) {
    try {
        work()
    } catch (e: java.io.IOException) {
        sink.warn(e)
    }
}
`,
		"nested lambda ignored": `
fun test() {
    try {
        work()
    } catch (e: java.io.IOException) {
        run {
            logger.warn("failed", e)
        }
    }
}
`,
		"string literals ignored": `
fun test() {
    try {
        work()
    } catch (e: java.io.IOException) {
        println("e throw log handle")
    }
}
`,
		"unknown assignment without exception is not handling": `
fun test() {
    var handled = false
    try {
        work()
    } catch (e: java.io.IOException) {
        handled = true
    }
}
`,
	}
	for name, code := range cases {
		t.Run(name, func(t *testing.T) {
			findings := runRuleByName(t, "SwallowedException", code)
			if len(findings) == 0 {
				t.Fatal("expected finding")
			}
		})
	}
}

func TestExc_SwallowedException_ASTNegativeCases(t *testing.T) {
	cases := map[string]string{
		"direct rethrow": `
fun test() {
    try {
        work()
    } catch (e: java.io.IOException) {
        throw e
    }
}
`,
		"cause forwarding": `
fun test() {
    try {
        work()
    } catch (e: java.io.IOException) {
        throw IllegalStateException("failed", e)
    }
}
`,
		"named cause forwarding": `
fun test() {
    try {
        work()
    } catch (e: java.io.IOException) {
        throw IllegalStateException(
            message = "failed",
            cause = e
        )
    }
}
`,
		"alias forwarding throw": `
fun test() {
    try {
        work()
    } catch (e: java.io.IOException) {
        val cause = e
        throw cause
    }
}
`,
		"alias constructor forwarding": `
fun test() {
    try {
        work()
    } catch (e: java.io.IOException) {
        val cause = e
        throw IllegalStateException(
            "failed",
            cause
        )
    }
}
`,
		"recognized logger": `
fun test() {
    try {
        work()
    } catch (e: java.io.IOException) {
        Log.e("tag", "failed", e)
        recover()
    }
}
`,
	}
	for name, code := range cases {
		t.Run(name, func(t *testing.T) {
			findings := runRuleByNameWithResolver(t, "SwallowedException", code)
			if len(findings) != 0 {
				t.Fatalf("expected no findings, got %d: %#v", len(findings), findings)
			}
		})
	}
}

func TestExc_SwallowedException_ResolvedLoggerCall(t *testing.T) {
	code := `
import java.util.logging.Logger
import java.util.logging.Level

class Test(private val logger: Logger) {
    fun test() {
        try {
            work()
        } catch (e: java.io.IOException) {
            logger.log(Level.WARNING, "failed", e)
        }
    }
}
`
	file := parseInline(t, code)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	fake := oracle.NewFakeOracle()
	fake.CallTargets[file.Path] = map[string]string{}
	file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
		if strings.Contains(file.FlatNodeText(idx), "logger.log") {
			key := fmt.Sprintf("%d:%d", file.FlatRow(idx)+1, file.FlatCol(idx)+1)
			fake.CallTargets[file.Path][key] = "java.util.logging.Logger.log"
		}
	})
	composite := oracle.NewCompositeResolver(fake, resolver)
	for _, r := range v2rules.Registry {
		if r.ID != "SwallowedException" {
			continue
		}
		cols := rules.NewDispatcherV2([]*v2rules.Rule{r}, composite).Run(file)
		findings := cols.Findings()
		if len(findings) != 0 {
			t.Fatalf("expected no findings, got %d: %#v", len(findings), findings)
		}
		return
	}
	t.Fatal("SwallowedException rule not found")
}

func TestExc_SwallowedException_ASTNegativeCasesWithoutResolver(t *testing.T) {
	cases := map[string]string{
		"qualified android log": `
fun test() {
    try {
        work()
    } catch (e: java.io.IOException) {
        Log.e("tag", "failed", e)
    }
}
`,
		"ui handling": `
fun test() {
    try {
        work()
    } catch (e: java.io.IOException) {
        Toast.makeText(context, e.message, Toast.LENGTH_SHORT).show()
    }
}
`,
		"assignment handling": `
class Holder {
    var lastError: Throwable? = null
    fun test() {
        try {
            work()
        } catch (e: java.io.IOException) {
            lastError = e
        }
    }
}
`,
		"same owner local handler": `
fun test() {
    fun handleError(t: Throwable) {}
    try {
        work()
    } catch (e: java.io.IOException) {
        handleError(e)
    }
}
`,
	}
	for name, code := range cases {
		t.Run(name, func(t *testing.T) {
			findings := runRuleByName(t, "SwallowedException", code)
			if len(findings) != 0 {
				t.Fatalf("expected no findings, got %d: %#v", len(findings), findings)
			}
		})
	}
}

// --- ThrowingExceptionFromFinally ---

func TestExc_ThrowingExceptionFromFinally_Positive(t *testing.T) {
	findings := runRuleByName(t, "ThrowingExceptionFromFinally", `
fun test() {
    try {
        doWork()
    } finally {
        throw RuntimeException("oops")
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for throw inside finally")
	}
}

func TestExc_ThrowingExceptionFromFinally_Negative(t *testing.T) {
	findings := runRuleByName(t, "ThrowingExceptionFromFinally", `
fun test() {
    try {
        doWork()
    } finally {
        cleanup()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- ThrowingExceptionsWithoutMessageOrCause ---

func TestExc_ThrowingExceptionsWithoutMessageOrCause_Positive(t *testing.T) {
	findings := runRuleByName(t, "ThrowingExceptionsWithoutMessageOrCause", `
fun test() {
    throw IllegalArgumentException()
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for exception without message")
	}
}

func TestExc_ThrowingExceptionsWithoutMessageOrCause_Negative(t *testing.T) {
	findings := runRuleByName(t, "ThrowingExceptionsWithoutMessageOrCause", `
fun test() {
    throw IllegalArgumentException("value must be positive")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- ThrowingNewInstanceOfSameException ---

func TestExc_ThrowingNewInstanceOfSameException_Positive(t *testing.T) {
	findings := runRuleByName(t, "ThrowingNewInstanceOfSameException", `
fun test() {
    try {
        doWork()
    } catch (e: IOException) {
        throw IOException(e)
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for throwing new instance of same exception")
	}
}

func TestExc_ThrowingNewInstanceOfSameException_Negative(t *testing.T) {
	findings := runRuleByName(t, "ThrowingNewInstanceOfSameException", `
fun test() {
    try {
        doWork()
    } catch (e: IOException) {
        throw RuntimeException("wrapped", e)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- ThrowingExceptionInMain ---

func TestExc_ThrowingExceptionInMain_Positive(t *testing.T) {
	findings := runRuleByName(t, "ThrowingExceptionInMain", `
fun main(args: Array<String>) {
    throw RuntimeException("fatal")
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for throw in main()")
	}
}

func TestExc_ThrowingExceptionInMain_Negative(t *testing.T) {
	findings := runRuleByName(t, "ThrowingExceptionInMain", `
fun main(args: Array<String>) {
    println("hello")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- ErrorUsageWithThrowable ---

func TestExc_ErrorUsageWithThrowable_Positive(t *testing.T) {
	findings := runRuleByName(t, "ErrorUsageWithThrowable", `
fun test() {
    try {
        doWork()
    } catch (e: Exception) {
        error(e)
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for error(throwable)")
	}
}

func TestExc_ErrorUsageWithThrowable_Negative(t *testing.T) {
	findings := runRuleByName(t, "ErrorUsageWithThrowable", `
fun test() {
    error("something went wrong")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- ObjectExtendsThrowable ---

func TestExc_ObjectExtendsThrowable_Positive(t *testing.T) {
	findings := runRuleByName(t, "ObjectExtendsThrowable", `
object MyError : RuntimeException("singleton error")
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for object extending Throwable")
	}
}

func TestExc_ObjectExtendsThrowable_Negative(t *testing.T) {
	findings := runRuleByName(t, "ObjectExtendsThrowable", `
object MyUtils {
    fun doStuff() {}
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}
