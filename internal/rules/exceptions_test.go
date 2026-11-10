package rules_test

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
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

func TestExc_ExceptionRaisedInUnexpectedLocation_HonorsMethodNames(t *testing.T) {
	// MethodNames was previously a dead config (the check used a global
	// hardcoded map). Configure a custom list via the rule pointer and
	// verify (a) configured names fire, (b) the previous defaults no
	// longer fire when not in the list.
	var rule *rules.ExceptionRaisedInUnexpectedLocationRule
	for _, candidate := range api.Registry {
		if candidate.ID == "ExceptionRaisedInUnexpectedLocation" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.ExceptionRaisedInUnexpectedLocationRule)
			if !ok {
				t.Fatalf("expected ExceptionRaisedInUnexpectedLocationRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("ExceptionRaisedInUnexpectedLocation rule not registered")
	}
	original := rule.MethodNames
	defer func() { rule.MethodNames = original }()

	rule.MethodNames = []string{"compute"}

	// `compute` is in the list — should fire.
	if findings := runRuleByName(t, "ExceptionRaisedInUnexpectedLocation", `
fun compute(): Int {
    throw IllegalStateException("nope")
}
`); len(findings) == 0 {
		t.Fatal("expected finding for configured method 'compute'")
	}

	// `equals` was a default but is no longer in the configured list — should NOT fire.
	if findings := runRuleByName(t, "ExceptionRaisedInUnexpectedLocation", `
fun equals(other: Any?): Boolean {
    throw IllegalStateException("not comparable")
}
`); len(findings) != 0 {
		t.Fatalf("expected no findings for 'equals' when not in configured methodNames, got %d", len(findings))
	}
}

func TestExc_ExceptionRaisedInUnexpectedLocation_JavaFixtures(t *testing.T) {
	root := fixtureRoot(t)

	t.Run("positive fixture is skipped", func(t *testing.T) {
		file := parseJavaFixture(t, filepath.Join(root, "positive", "exceptions", "ExceptionRaisedInUnexpectedLocation.java"))
		findings := runRuleByNameOnFile(t, "ExceptionRaisedInUnexpectedLocation", file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 Java finding, got %d", len(findings))
		}
	})

	t.Run("negative", func(t *testing.T) {
		file := parseJavaFixture(t, filepath.Join(root, "negative", "exceptions", "ExceptionRaisedInUnexpectedLocation.java"))
		findings := runRuleByNameOnFile(t, "ExceptionRaisedInUnexpectedLocation", file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 Java findings, got %d", len(findings))
		}
	})
}

// --- InstanceOfCheckForException ---

// --- InstanceOfCheckForException: when-dispatch FP-reduction (regression) ---
//
// Regression for isInsideWhenDispatchOnCatchVarFlat: the helper used to
// `return false` on the first when_expression child whose type matched
// when_subject / parenthesized_expression / value_arguments but whose text
// did not equal the caught variable. That early return could miss a later
// sibling that actually carries the caught-variable subject.

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

// Regression: flatCallExpressionName returns the rightmost identifier of a
// navigation chain, so `svc.TODO()` would otherwise be reported as
// `kotlin.TODO`. Only bare TODO() and the explicitly-qualified kotlin.TODO()
// should fire.
func TestExc_NotImplementedDeclaration_IgnoresNavigationReceivers(t *testing.T) {
	findings := runRuleByName(t, "NotImplementedDeclaration", `
class Service {
    fun TODO(): String = "x"
}

class Caller(private val svc: Service) {
    fun a(): String = svc.TODO()
    fun b(): String = this.svc.TODO()
}

object Registry {
    fun TODO(): String = "y"
}

fun obj(): String = Registry.TODO()
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for non-kotlin receivers, got %d:\n%v", len(findings), findings)
	}
}

func TestExc_NotImplementedDeclaration_FiresOnKotlinQualified(t *testing.T) {
	findings := runRuleByName(t, "NotImplementedDeclaration", `
fun a(): Nothing = kotlin.TODO()
fun b(): Nothing = kotlin.TODO("not yet")
`)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings for kotlin.TODO() calls, got %d:\n%v", len(findings), findings)
	}
}

func TestExc_NotImplementedDeclaration_FiresOnBareCall(t *testing.T) {
	findings := runRuleByName(t, "NotImplementedDeclaration", `
fun a(): Nothing = TODO()
fun b(): Nothing = TODO("not yet")
`)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings for bare TODO() calls, got %d:\n%v", len(findings), findings)
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

func TestExc_RethrowCaughtException_SkipsJava(t *testing.T) {
	findings := runRuleByNameOnJava(t, "RethrowCaughtException", `
package test;
class Example {
  void run() throws Exception {
    try {
      work();
    } catch (Exception e) {
      throw e;
    }
  }
  void work() throws Exception {}
}
`)
	if len(findings) != 0 {
		t.Fatalf("detekt does not analyze Java sources; expected no findings, got %d", len(findings))
	}
}

// Regression for #669: a rethrow-only catch followed by another catch of the
// same try deliberately lets that type escape the later handler.
func TestExc_RethrowCaughtException_LaterCatchSuppresses(t *testing.T) {
	findings := runRuleByName(t, "RethrowCaughtException", `
fun test() {
    try {
        doWork()
    } catch (e: CancellationException) {
        throw e
    } catch (e: Exception) {
        log(e)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when a later catch would swallow the exception, got %d", len(findings))
	}
}

func TestExc_RethrowCaughtException_LastCatchStillFlagged(t *testing.T) {
	findings := runRuleByName(t, "RethrowCaughtException", `
fun test() {
    try {
        doWork()
    } catch (e: IOException) {
        log(e)
    } catch (e: Exception) {
        throw e
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for rethrow-only last catch, got %d", len(findings))
	}
	// detekt reports the rethrow itself, not the catch keyword.
	if findings[0].Line != 8 {
		t.Fatalf("expected finding on line 8, got %d", findings[0].Line)
	}
}

// detekt reports every rethrow-only catch that is followed only by other rethrow-only catches, and a
// catch whose first statement is the rethrow even when more statements follow.
func TestExc_RethrowCaughtException_MatchesDetekt(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []int
	}{
		{"all rethrow", "} catch (e: IOException) {\n        throw e\n    } catch (e: IllegalStateException) {\n        throw e\n    }", []int{5, 7}},
		{"handled later", "} catch (e: IOException) {\n        throw e\n    } catch (e: IllegalStateException) {\n        log(e)\n    }", nil},
		{"handled after chain", "} catch (e: IOException) {\n        throw e\n    } catch (e: IllegalStateException) {\n        throw e\n    } catch (e: Exception) {\n        log(e)\n    }", nil},
		{"first statement", "} catch (e: IOException) {\n        throw e\n        log(e)\n    }", []int{5}},
		{"after other statement", "} catch (e: IOException) {\n        log(e)\n        throw e\n    }", nil},
		{"comment first", "} catch (e: IOException) {\n        // keep\n        throw e\n    }", []int{6}},
		{"parenthesized", "} catch (e: IOException) {\n        throw (e)\n    }", nil},
	}
	for _, tc := range cases {
		findings := runRuleByName(t, "RethrowCaughtException", "fun test() {\n    try {\n        doWork()\n    "+tc.body+"\n}\n")
		var got []int
		for _, f := range findings {
			got = append(got, f.Line)
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: lines = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestExc_RethrowCaughtException_JavaFixtures(t *testing.T) {
	root := fixtureRoot(t)

	t.Run("positive fixture is skipped", func(t *testing.T) {
		file := parseJavaFixture(t, filepath.Join(root, "positive", "exceptions", "RethrowCaughtException.java"))
		findings := runRuleByNameOnFile(t, "RethrowCaughtException", file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 Java findings, got %d", len(findings))
		}
	})

	t.Run("negative", func(t *testing.T) {
		file := parseJavaFixture(t, filepath.Join(root, "negative", "exceptions", "RethrowCaughtException.java"))
		findings := runRuleByNameOnFile(t, "RethrowCaughtException", file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 Java findings, got %d", len(findings))
		}
	})
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

func TestExc_ReturnFromFinally_HonorsIgnoreLabeled(t *testing.T) {
	// IgnoreLabeled was previously a dead config — exposed in metadata but
	// never consulted by the check. Configure it via the rule pointer and
	// verify labeled returns (return@something) are excluded while plain
	// `return` inside finally is still flagged.
	var rule *rules.ReturnFromFinallyRule
	for _, candidate := range api.Registry {
		if candidate.ID == "ReturnFromFinally" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.ReturnFromFinallyRule)
			if !ok {
				t.Fatalf("expected ReturnFromFinallyRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("ReturnFromFinally rule not registered")
	}
	original := rule.IgnoreLabeled
	defer func() { rule.IgnoreLabeled = original }()

	rule.IgnoreLabeled = true

	// Labeled return inside finally — under IgnoreLabeled=true, no finding.
	if findings := runRuleByName(t, "ReturnFromFinally", `
fun example() {
    try {
        listOf(1).forEach forEach@{
            try {
                doWork()
            } finally {
                return@forEach
            }
        }
    } finally {}
}
`); len(findings) != 0 {
		t.Fatalf("expected no findings for labeled return when IgnoreLabeled=true, got %d", len(findings))
	}

	// Plain return still fires.
	if findings := runRuleByName(t, "ReturnFromFinally", `
fun example(): Int {
    try {
        return 1
    } finally {
        return 2
    }
}
`); len(findings) == 0 {
		t.Fatal("expected finding for plain return inside finally even with IgnoreLabeled=true")
	}
}

func TestExc_ReturnFromFinally_SkipsJava(t *testing.T) {
	findings := runRuleByNameOnJava(t, "ReturnFromFinally", `
package test;
class Example {
  int run() {
    try {
      return 1;
    } finally {
      return 2;
    }
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("detekt does not analyze Java sources; expected no findings, got %d", len(findings))
	}
}

// --- SwallowedException ---

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

func TestExc_ThrowingExceptionFromFinally_SkipsJava(t *testing.T) {
	findings := runRuleByNameOnJava(t, "ThrowingExceptionFromFinally", `
package test;
class Example {
  void run() {
    try {
      work();
    } finally {
      throw new RuntimeException("masked");
    }
  }
  void work() {}
}
`)
	if len(findings) != 0 {
		t.Fatalf("detekt does not analyze Java sources; expected no findings, got %d", len(findings))
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

func TestExc_ThrowingNewInstanceOfSameException_NegativeCaughtVariableNotPassed(t *testing.T) {
	findings := runRuleByName(t, "ThrowingNewInstanceOfSameException", `
fun test() {
    try {
        doWork()
    } catch (e: IOException) {
        log(e)
        throw IOException()
    }
    try {
        doWork()
    } catch (_: IOException) {
        throw IOException("replaced")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when the caught variable is not wrapped, got %d: %v", len(findings), findings)
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

func TestExc_ThrowingNewInstanceOfSameException_SkipsJava(t *testing.T) {
	findings := runRuleByNameOnJava(t, "ThrowingNewInstanceOfSameException", `
package test;
import java.io.IOException;
class Example {
  void run() throws IOException {
    try {
      work();
    } catch (IOException e) {
      throw new IOException(e);
    }
  }
  void work() throws IOException {}
}
`)
	if len(findings) != 0 {
		t.Fatalf("detekt does not analyze Java sources; expected no findings, got %d", len(findings))
	}
}

func TestExc_ThrowingNewInstanceOfSameException_NegativeStringMentionsCaughtType(t *testing.T) {
	findings := runRuleByName(t, "ThrowingNewInstanceOfSameException", `
fun test() {
    try {
        doWork()
    } catch (e: Exception) {
        throw RuntimeException("about to throw Exception(extra)")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings — the new exception is RuntimeException, the string literal merely mentions Exception(...); got %d:\n%v", len(findings), findings)
	}
}

func TestExc_ThrowingNewInstanceOfSameException_NegativeJavaStringMentionsCaughtType(t *testing.T) {
	findings := runRuleByNameOnJava(t, "ThrowingNewInstanceOfSameException", `
package test;
class Example {
  void run() {
    try {
      work();
    } catch (Exception e) {
      throw new RuntimeException("about to throw Exception(extra)");
    }
  }
  void work() throws Exception {}
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings — the new exception is RuntimeException, the string literal merely mentions Exception(...); got %d:\n%v", len(findings), findings)
	}
}

func TestExc_ThrowingNewInstanceOfSameException_NegativeRethrowWithContext(t *testing.T) {
	findings := runRuleByName(t, "ThrowingNewInstanceOfSameException", `
fun test(path: String) {
    try {
        doWork()
    } catch (e: IOException) {
        throw IOException("failed to read $path", e)
    }
    try {
        doWork()
    } catch (e: IOException) {
        throw IOException("failed to read $path")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for rethrow with a new message, got %d", len(findings))
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

func TestExc_ThrowingExceptionInMain_JavaFixtures(t *testing.T) {
	root := fixtureRoot(t)

	t.Run("positive fixture is skipped", func(t *testing.T) {
		file := parseJavaFixture(t, filepath.Join(root, "positive", "exceptions", "ThrowingExceptionInMain.java"))
		findings := runRuleByNameOnFile(t, "ThrowingExceptionInMain", file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 Java finding, got %d", len(findings))
		}
	})

	t.Run("negative", func(t *testing.T) {
		file := parseJavaFixture(t, filepath.Join(root, "negative", "exceptions", "ThrowingExceptionInMain.java"))
		findings := runRuleByNameOnFile(t, "ThrowingExceptionInMain", file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 Java findings, got %d", len(findings))
		}
	})
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

func TestExc_ErrorUsageWithThrowable_RuntimeExceptionSubtype(t *testing.T) {
	findings := runRuleByName(t, "ErrorUsageWithThrowable", `
fun test(e: RuntimeException) {
    error(e)
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for error(RuntimeException)")
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

func TestExc_ErrorUsageWithThrowable_NegativeStringNamedError(t *testing.T) {
	findings := runRuleByName(t, "ErrorUsageWithThrowable", `
fun test(error: String) {
    error(error)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for String argument named error, got %d", len(findings))
	}
}

func TestExc_ErrorUsageWithThrowable_NegativeLoggerError(t *testing.T) {
	findings := runRuleByName(t, "ErrorUsageWithThrowable", `
class Logger {
    fun error(t: Throwable) {}
}

fun test(logger: Logger, t: Throwable) {
    logger.error(t)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for logger.error(Throwable), got %d", len(findings))
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

func TestExc_ExceptionRaisedInUnexpectedLocation_PositiveToStringWithOtherWork(t *testing.T) {
	findings := runRuleByName(t, "ExceptionRaisedInUnexpectedLocation", `
class Row(val id: Int) {
    override fun toString(): String {
        check(id > 0)
        if (id > 100) throw IllegalStateException("too big")
        return "Row"
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestNotImplementedDeclaration_ThrowOfNotImplementedError(t *testing.T) {
	src := `package test
fun a() { throw NotImplementedError() }
fun b() { throw NotImplementedError("later") }
fun c() { throw kotlin.NotImplementedError() }
fun d() { throw IllegalStateException("x") }
fun e() {
    throw
        NotImplementedError()
}
`
	if got := runRuleByName(t, "NotImplementedDeclaration", src); len(got) != 4 {
		t.Fatalf("expected 4 findings (a, b, c, e), got %d: %v", len(got), got)
	}
}

func TestReturnFromFinally_IgnoresReturnsOfNestedScopes(t *testing.T) {
	clean := map[string]string{
		"local function":  "fun f() { try { g() } finally { fun inner(): Int { return 1 }; inner() } }\nfun g() {}",
		"labeled lambda":  "fun f() { try { g() } finally { listOf(1).forEach { if (it > 0) return@forEach } } }\nfun g() {}",
		"object member":   "fun f() { try { g() } finally { object { fun x(): Int { return 1 } } } }\nfun g() {}",
		"thrown in local": "fun f() { try { g() } finally { fun inner() { throw IllegalStateException(\"x\") } } }\nfun g() {}",
	}
	for name, src := range clean {
		for _, rule := range []string{"ReturnFromFinally", "ThrowingExceptionFromFinally"} {
			if got := runRuleByName(t, rule, src); len(got) != 0 {
				t.Errorf("%s/%s: want no findings, got %d", name, rule, len(got))
			}
		}
	}
	if got := runRuleByName(t, "ReturnFromFinally", "fun f(): Int { try { g() } finally { run { return 1 } } }\nfun g() {}"); len(got) != 1 {
		t.Errorf("a non-local return from an inline lambda must still be reported, got %d", len(got))
	}
}

func TestThrowingExceptionInMain_OnlyProgramEntryPoints(t *testing.T) {
	clean := map[string]string{
		"member named main":   "class K { fun main() { throw IllegalStateException(\"x\") } }",
		"extra parameters":    "fun main(a: Int, b: Int) { throw IllegalStateException(\"x\") }",
		"nonstatic java main": "",
	}
	for name, src := range clean {
		if got := runRuleByName(t, "ThrowingExceptionInMain", src); len(got) != 0 {
			t.Errorf("%s: want no findings, got %d", name, len(got))
		}
	}
	flagged := map[string]string{
		"top level": "fun main() { throw IllegalStateException(\"x\") }",
		"args":      "fun main(args: Array<String>) { throw IllegalStateException(\"x\") }",
		"jvmstatic": "object O { @JvmStatic fun main(args: Array<String>) { throw IllegalStateException(\"x\") } }",
	}
	for name, src := range flagged {
		if got := runRuleByName(t, "ThrowingExceptionInMain", src); len(got) != 1 {
			t.Errorf("%s: want 1 finding, got %d", name, len(got))
		}
	}
	javaMain := "class A { public static void main(String[] args) { throw new IllegalStateException(\"x\"); } }"
	if got := runRuleByNameOnJava(t, "ThrowingExceptionInMain", javaMain); len(got) != 0 {
		t.Errorf("static java main: detekt does not analyze Java, want no findings, got %d", len(got))
	}
	javaInstance := "class A { void main() { throw new IllegalStateException(\"x\"); } }"
	if got := runRuleByNameOnJava(t, "ThrowingExceptionInMain", javaInstance); len(got) != 0 {
		t.Errorf("instance java main: want no findings, got %d", len(got))
	}
}

func TestCatchRethrowFix_SingleLineCatchStartsNewLine(t *testing.T) {
	findings := runRuleByName(t, "SwallowedException", `
package test

fun f() {
    try {
        work()
    } catch (e: Exception) { println("x") }
}
`)
	if len(findings) != 1 || findings[0].Fix == nil {
		t.Fatalf("expected one finding with a fix, got %+v", findings)
	}
	if got := findings[0].Fix.Replacement; got != "\n    "+"    throw e\n    }" {
		t.Errorf("replacement = %q", got)
	}
}
