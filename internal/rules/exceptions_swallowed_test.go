package rules_test

import (
	"fmt"
	"testing"
)

// The expected lines were produced by running detekt 1.23.8 SwallowedException on the same source.
func TestSwallowedException_MatchesDetekt(t *testing.T) {
	cases := []struct {
		name  string
		catch string
		want  bool
	}{
		{"empty body", `catch (e: Exception) { }`, true},
		{"unused", `catch (e: Exception) { println("x") }`, true},
		{"passed to println", `catch (e: Exception) { println(e) }`, false},
		{"message only", `catch (e: Exception) { println(e.message) }`, false},
		{"throw without cause", `catch (e: Exception) { throw RuntimeException("x") }`, true},
		{"throw with cause", `catch (e: Exception) { throw RuntimeException("x", e) }`, false},
		{"throw with message only", `catch (e: Exception) { throw RuntimeException(e.message) }`, true},
		{"throw with exception", `catch (e: Exception) { throw RuntimeException(e) }`, false},
		{"rethrow", `catch (e: Exception) { throw e }`, false},
		{"underscore name", `catch (_: Exception) { println() }`, false},
		{"ignored name prefix", `catch (ignored: Exception) { println() }`, false},
		{"expected name prefix", `catch (expectedFailure: Exception) { println() }`, false},
		{"ignored type", `catch (e: NumberFormatException) { println() }`, false},
		{"ignored type empty", `catch (e: InterruptedException) { }`, false},
		{"returns", `catch (e: Exception) { return }`, true},
		{"assignment", `catch (e: Exception) { x = 1 }`, true},
		{"comment only", `catch (e: Exception) { /* ignored */ }`, true},
		{"logger without exception", `catch (e: Exception) { log.error("x") }`, true},
		{"logger with exception", `catch (e: Exception) { log.error("x", e) }`, false},
		{"callee named like the exception", `catch (e: Exception) { Timber.e("fail") }`, false},
		{"cause member through a variable", `catch (e: Exception) { val c = e.cause; throw RuntimeException(c) }`, true},
		{"message through a variable", `catch (e: Exception) { val m = e.message; throw RuntimeException(m) }`, true},
		{"toString only", `catch (e: Exception) { throw RuntimeException(e.toString()) }`, true},
		{"string template", `catch (e: Exception) { println("failed: $e") }`, false},
		{"class reference", `catch (e: Exception) { println(e::class.java.simpleName) }`, false},
	}
	for _, tc := range cases {
		code := fmt.Sprintf("package test\nfun f() {\n    try { g() } %s\n}\n", tc.catch)
		got := runRuleByName(t, "SwallowedException", code)
		if (len(got) == 1) != tc.want || len(got) > 1 {
			t.Errorf("%s: want report=%v, got %v", tc.name, tc.want, got)
		}
	}
	// A catch used as an expression is reported too, and test sources are not exempt.
	expr := "package test\nfun f(): Int? = try { g() } catch (e: Exception) { null }\n"
	if got := runRuleByName(t, "SwallowedException", expr); len(got) != 1 || got[0].Message != "The caught exception is swallowed. The original exception could be lost." {
		t.Errorf("try used as an expression must be reported with detekt's message, got %v", got)
	}
}

// Expected results come from running detekt 1.23.8 on the same sources.
func TestExceptionRulesMatchDetektOnPathsAndShapes(t *testing.T) {
	// ExceptionRaisedInUnexpectedLocation reports a function once, even a throw-only toString, in test sources too.
	src := "package test\nclass A {\n    override fun toString(): String { throw Exception(\"x\") }\n    override fun hashCode(): Int { if (true) throw IllegalStateException(\"a\"); throw IllegalStateException(\"b\") }\n}\n"
	got := runRuleByNameOnPath(t, "ExceptionRaisedInUnexpectedLocation", "src/test/kotlin/ATest.kt", src)
	if len(got) != 2 || got[0].Message != "This method is not expected to throw exceptions. This can cause weird behavior." || got[0].Col != 18 {
		t.Errorf("want one finding per function at its name, got %v", got)
	}
	// PrintStackTrace reports <caught>.printStackTrace() in its catch and Thread.dumpStack(), anywhere.
	ps := "package test\nfun f(e2: Exception) {\n    try { g() } catch (e: Exception) { e.printStackTrace() }\n    e2.printStackTrace(System.out)\n    Thread.dumpStack()\n}\n"
	got = runRuleByNameOnPath(t, "PrintStackTrace", "tools/build/Tool.kt", ps)
	if len(got) != 2 || got[0].Line != 3 || got[1].Line != 5 {
		t.Errorf("want findings on lines 3 and 5, got %v", got)
	}
	// ThrowingExceptionsWithoutMessageOrCause looks at every call, qualified or not, not only throws.
	tw := "package test\nfun f() {\n    val a = IllegalStateException()\n    throw java.lang.IllegalStateException()\n}\nfun g() = IllegalStateException(\"x\")\nfun h() = UnsupportedOperationException()\n"
	got = runRuleByName(t, "ThrowingExceptionsWithoutMessageOrCause", tw)
	if len(got) != 2 || got[1].Line != 4 || got[1].Col != 21 {
		t.Errorf("want findings at lines 3 and 4 (callee name), got %v", got)
	}
}

// Expected results come from running detekt 1.23.8 on the same sources.
func TestInstanceOfCheckForException_MatchesDetekt(t *testing.T) {
	cases := []struct {
		name  string
		catch string
		want  int
	}{
		{"is on the caught value", `catch (e: Exception) { if (e is IllegalStateException) { g() } }`, 1},
		{"not is", `catch (e: Exception) { if (e !is IllegalStateException) { g() } }`, 1},
		{"unsafe as", `catch (e: Exception) { val x = e as IllegalStateException }`, 1},
		{"safe as", `catch (e: Exception) { val x = e as? IllegalStateException }`, 0},
		{"when dispatch", `catch (e: Exception) { when (e) { is IllegalStateException -> g() else -> h() } }`, 0},
		{"local from the cause", `catch (e: Exception) { val c = e.cause; if (c is IllegalStateException) { g() } }`, 0},
		{"another value", `catch (e: Exception) { if (other is IllegalStateException) { g() } }`, 0},
		{"the cause", `catch (e: Exception) { if (e.cause is IllegalStateException) { g() } }`, 0},
		{"any type", `catch (e: Throwable) { if (e is Exception) { g() } }`, 1},
	}
	for _, tc := range cases {
		code := fmt.Sprintf("package test\nfun f() {\n    try { g() } %s\n}\n", tc.catch)
		got := runRuleByName(t, "InstanceOfCheckForException", code)
		if len(got) != tc.want {
			t.Errorf("%s: want %d findings, got %v", tc.name, tc.want, got)
		}
	}
}
