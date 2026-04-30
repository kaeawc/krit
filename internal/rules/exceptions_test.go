package rules_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/experiment"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func experimentSnapshot() []string     { return experiment.Current().Names() }
func experimentRestore(names []string) { experiment.SetCurrent(names) }
func enableExperiment(name string) {
	cur := experiment.Current().Names()
	for _, n := range cur {
		if n == name {
			return
		}
	}
	experiment.SetCurrent(append(cur, name))
}

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
	for _, candidate := range v2rules.Registry {
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

	t.Run("positive", func(t *testing.T) {
		file := parseJavaFixture(t, filepath.Join(root, "positive", "exceptions", "ExceptionRaisedInUnexpectedLocation.java"))
		findings := runRuleByNameOnFile(t, "ExceptionRaisedInUnexpectedLocation", file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 Java finding, got %d", len(findings))
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

// --- InstanceOfCheckForException: when-dispatch FP-reduction (regression) ---
//
// Regression for isInsideWhenDispatchOnCatchVarFlat: the helper used to
// `return false` on the first when_expression child whose type matched
// when_subject / parenthesized_expression / value_arguments but whose text
// did not equal the caught variable. That early return could miss a later
// sibling that actually carries the caught-variable subject.

func TestExc_InstanceOfCheckForException_SkipsWhenDispatchOnCaughtVar(t *testing.T) {
	prev := experimentSnapshot()
	defer experimentRestore(prev)
	enableExperiment("instance-of-check-skip-when-dispatch")

	findings := runRuleByName(t, "InstanceOfCheckForException", `
fun test() {
    try {
        doWork()
    } catch (e: Exception) {
        when (e) {
            is IOException -> println("io")
            is RuntimeException -> println("rt")
            else -> throw e
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings (when-dispatch on caught var should be skipped), got %d", len(findings))
	}
}

func TestExc_InstanceOfCheckForException_FiresWhenDispatchOnOtherVar(t *testing.T) {
	prev := experimentSnapshot()
	defer experimentRestore(prev)
	enableExperiment("instance-of-check-skip-when-dispatch")

	findings := runRuleByName(t, "InstanceOfCheckForException", `
fun test(other: Any) {
    try {
        doWork()
    } catch (e: Exception) {
        when (other) {
            is IOException -> println("io")
        }
        if (e is IOException) println("io2")
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding: when-dispatch is on `other`, not the caught var, so the is-check on `e` must still fire")
	}
}

func TestExc_InstanceOfCheckForException_FiresOnSubjectlessWhen(t *testing.T) {
	prev := experimentSnapshot()
	defer experimentRestore(prev)
	enableExperiment("instance-of-check-skip-when-dispatch")

	findings := runRuleByName(t, "InstanceOfCheckForException", `
fun test() {
    try {
        doWork()
    } catch (e: Exception) {
        when {
            e is IOException -> println("io")
        }
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding: subjectless when is not a dispatch, so the is-check on caught var must fire")
	}
}

func TestExc_InstanceOfCheckForException_NegativeNonExceptionTypeWithExceptionPrefix(t *testing.T) {
	// `is ExceptionHandler` (a non-exception type that happens to start with
	// "Exception") must not trigger the rule. Regression for the missing
	// trailing word boundary in isExceptionRe.
	findings := runRuleByName(t, "InstanceOfCheckForException", `
fun test() {
    try {
        doWork()
    } catch (e: Throwable) {
        if (e is ExceptionHandler) {
            handle(e)
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for is-check on non-exception type, got %d: %+v", len(findings), findings)
	}
}

func TestExc_InstanceOfCheckForException_NegativeIdentifierContainingException(t *testing.T) {
	// `is Foo` where Foo is unrelated must not match — regression guard.
	findings := runRuleByName(t, "InstanceOfCheckForException", `
fun test() {
    try {
        doWork()
    } catch (e: Throwable) {
        if (e is ExceptionalCircumstance) {
            handle(e)
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for ExceptionalCircumstance, got %d", len(findings))
	}
}

func TestExc_InstanceOfCheckForException_Java(t *testing.T) {
	findings := runRuleByNameOnJava(t, "InstanceOfCheckForException", `
package test;
class Example {
  void run() {
    try {
      work();
    } catch (Exception e) {
      if (e instanceof java.io.IOException) {
        work();
      }
    }
  }
  void work() {}
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 Java instanceof finding, got %d", len(findings))
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

func TestExc_RethrowCaughtException_Java(t *testing.T) {
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
	if len(findings) != 1 {
		t.Fatalf("expected 1 Java rethrow finding, got %d", len(findings))
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

func TestExc_ReturnFromFinally_HonorsIgnoreLabeled(t *testing.T) {
	// IgnoreLabeled was previously a dead config — exposed in metadata but
	// never consulted by the check. Configure it via the rule pointer and
	// verify labeled returns (return@something) are excluded while plain
	// `return` inside finally is still flagged.
	var rule *rules.ReturnFromFinallyRule
	for _, candidate := range v2rules.Registry {
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

func TestExc_ReturnFromFinally_Java(t *testing.T) {
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
	if len(findings) != 1 {
		t.Fatalf("expected 1 Java return-from-finally finding, got %d", len(findings))
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
import org.signal.core.util.logging.Log

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

func TestExc_SwallowedException_NegativeCallbackHandler(t *testing.T) {
	findings := runRuleByName(t, "SwallowedException", `
import java.io.IOException

class FaultHidingSink(
    private val onException: (IOException) -> Unit,
) {
    fun flush() {
        try {
            delegateFlush()
        } catch (e: IOException) {
            onException(e)
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected callback handler to count as handling, got %d", len(findings))
	}
}

func TestExc_SwallowedException_PositiveUnknownCallbackName(t *testing.T) {
	findings := runRuleByName(t, "SwallowedException", `
import java.io.IOException

class FaultHidingSink(
    private val ignored: (IOException) -> Unit,
) {
    fun flush() {
        try {
            delegateFlush()
        } catch (e: IOException) {
            ignored(e)
        }
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected unknown callback name to remain suspicious")
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
		"local log lookalike": `
object Log {
    fun e(tag: String, message: String, error: Throwable) {}
}
fun test() {
    try {
        work()
    } catch (e: java.io.IOException) {
        Log.e("tag", "failed", e)
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
import org.signal.core.util.logging.Log

fun test() {
    try {
        work()
    } catch (e: java.io.IOException) {
        Log.e("tag", "failed", e)
        recover()
    }
}
`,
		"java util logger parameter": `
import java.util.logging.Logger
import java.util.logging.Level

fun test(logger: Logger) {
    try {
        work()
    } catch (e: java.io.IOException) {
        logger.log(Level.WARNING, "failed", e)
    }
}
`,
		"returned domain failure": `
sealed class ChangeNumberResult {
    data class SvrWrongPin(val cause: Throwable): ChangeNumberResult()
}
fun test(): ChangeNumberResult {
    try {
        work()
    } catch (e: java.io.IOException) {
        return ChangeNumberResult.SvrWrongPin(e)
    }
    return ChangeNumberResult.SvrWrongPin(RuntimeException())
}
`,
		"callback load failure": `
interface Callback {
    fun onLoadFailed(error: Throwable)
}
fun test(callback: Callback) {
    try {
        work()
    } catch (e: java.io.IOException) {
        callback.onLoadFailed(e)
    }
}
`,
		"signal logger with tag helper": `
import org.signal.core.util.logging.Log

/**
 * Import header regression: tree-sitter may include this comment in the
 * previous import_header node.
 */

object Migration {
    private val TAG = Log.tag(Migration::class.java)
    fun migrate() {
        try {
            work()
        } catch (t: Throwable) {
            Log.e(TAG, "Failed to perform migration!", t)
        }
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
import android.util.Log

fun test() {
    try {
        work()
    } catch (e: java.io.IOException) {
        Log.e("tag", "failed", e)
    }
}
`,
		"qualified timber log": `
import timber.log.Timber

fun test() {
    try {
        work()
    } catch (e: java.io.IOException) {
        Timber.e(e)
    }
}
`,
		"aliased timber log": `
import timber.log.Timber as Logcat

fun test() {
    try {
        work()
    } catch (exception: java.io.IOException) {
        Logcat.e(exception)
    }
}
`,
		"member timber log import": `
import timber.log.Timber.e

fun test() {
    try {
        work()
    } catch (exception: java.io.IOException) {
        e(exception)
    }
}
`,
		"ui handling": `
import android.widget.Toast

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

func TestExc_ThrowingExceptionFromFinally_Java(t *testing.T) {
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
	if len(findings) != 1 {
		t.Fatalf("expected 1 Java throw-from-finally finding, got %d", len(findings))
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

func TestExc_ThrowingNewInstanceOfSameException_Java(t *testing.T) {
	findings := runRuleByNameOnJava(t, "ThrowingNewInstanceOfSameException", `
package test;
import java.io.IOException;
class Example {
  void run() throws IOException {
    try {
      work();
    } catch (IOException e) {
      throw new IOException();
    }
  }
  void work() throws IOException {}
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 Java same-exception finding, got %d", len(findings))
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

	t.Run("positive", func(t *testing.T) {
		file := parseJavaFixture(t, filepath.Join(root, "positive", "exceptions", "ThrowingExceptionInMain.java"))
		findings := runRuleByNameOnFile(t, "ThrowingExceptionInMain", file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 Java finding, got %d", len(findings))
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
