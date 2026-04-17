package rules_test

import "testing"

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
        log(e)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
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
