package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// --- ExitOutsideMain ---

func TestExitOutsideMain_Positive(t *testing.T) {
	findings := runRuleByName(t, "ExitOutsideMain", `
package test
fun shutdown() {
    exitProcess(1)
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for exitProcess outside main")
	}
}

func TestExitOutsideMain_SystemExitPositive(t *testing.T) {
	findings := runRuleByName(t, "ExitOutsideMain", `
package test
fun shutdown() {
    System.exit(1)
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for System.exit outside main")
	}
}

func TestExitOutsideMain_Negative(t *testing.T) {
	findings := runRuleByName(t, "ExitOutsideMain", `
package test
fun main() {
    exitProcess(0)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestExitOutsideMain_IgnoresNonSystemExitReceiver(t *testing.T) {
	findings := runRuleByName(t, "ExitOutsideMain", `
package test
class Process {
    fun exit(code: Int) {}
}

fun shutdown(process: Process) {
    process.exit(1)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for non-System exit receiver, got %d", len(findings))
	}
}

func TestExitOutsideMain_IgnoresStringMentions(t *testing.T) {
	findings := runRuleByName(t, "ExitOutsideMain", `
package test
fun shutdown() {
    val msg = "exitProcess(1) and System.exit(1)"
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for string mentions, got %d", len(findings))
	}
}

func TestExitOutsideMain_Java(t *testing.T) {
	findings := runRuleByNameOnJava(t, "ExitOutsideMain", `
package test;
class Example {
  void shutdown() {
    System.exit(1);
  }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 Java System.exit finding, got %d", len(findings))
	}
}

func TestExitOutsideMain_JavaIgnoresMain(t *testing.T) {
	findings := runRuleByNameOnJava(t, "ExitOutsideMain", `
package test;
class Example {
  public static void main(String[] args) {
    System.exit(0);
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no Java System.exit findings in main, got %d", len(findings))
	}
}

func TestExitOutsideMain_JavaIgnoresLocalSystemLookalike(t *testing.T) {
	findings := runRuleByNameOnJava(t, "ExitOutsideMain", `
package test;
class System {
  static void exit(int code) {}
}
class Example {
  void shutdown() {
    System.exit(1);
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no Java System.exit findings for local System lookalike, got %d", len(findings))
	}
}

func TestExitOutsideMain_JavaIgnoresImportedSystemLookalike(t *testing.T) {
	findings := runRuleByNameOnJava(t, "ExitOutsideMain", `
package test;
import com.example.System;
class Example {
  void shutdown() {
    System.exit(1);
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no Java System.exit findings for imported System lookalike, got %d", len(findings))
	}
}

// --- ExplicitGarbageCollectionCall ---

func TestExplicitGarbageCollectionCall_Positive(t *testing.T) {
	findings := runRuleByName(t, "ExplicitGarbageCollectionCall", `
package test
fun cleanup() {
    System.gc()
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for System.gc()")
	}
}

func TestExplicitGarbageCollectionCall_Negative(t *testing.T) {
	findings := runRuleByName(t, "ExplicitGarbageCollectionCall", `
package test
fun cleanup() {
    System.out.println("done")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestExplicitGarbageCollectionCall_Java(t *testing.T) {
	findings := runRuleByNameOnJava(t, "ExplicitGarbageCollectionCall", `
package test;
class Example {
  void cleanup() {
    System.gc();
    Runtime.getRuntime().gc();
  }
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected 2 Java explicit-GC findings, got %d", len(findings))
	}
}

func TestExplicitGarbageCollectionCall_JavaIgnoresLocalLookalikes(t *testing.T) {
	findings := runRuleByNameOnJava(t, "ExplicitGarbageCollectionCall", `
package test;
class System {
  static void gc() {}
}
class Runtime {
  static Runtime getRuntime() { return new Runtime(); }
  void gc() {}
}
class Example {
  void cleanup() {
    System.gc();
    Runtime.getRuntime().gc();
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no Java explicit-GC findings for local lookalikes, got %d", len(findings))
	}
}

func TestExplicitGarbageCollectionCall_JavaIgnoresImportedLookalikes(t *testing.T) {
	findings := runRuleByNameOnJava(t, "ExplicitGarbageCollectionCall", `
package test;
import com.example.System;
import com.example.Runtime;
class Example {
  void cleanup() {
    System.gc();
    Runtime.getRuntime().gc();
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no Java explicit-GC findings for imported lookalikes, got %d", len(findings))
	}
}

func TestExplicitGarbageCollectionCall_JavaFixIsLineDeletion(t *testing.T) {
	findings := runRuleByNameOnJava(t, "ExplicitGarbageCollectionCall", `package test;
class Example {
  void cleanup() {
    System.gc();
  }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0].Fix
	if f == nil || !f.ByteMode {
		t.Fatalf("expected byte-mode Fix, got %#v", f)
	}
	if f.Replacement != "" {
		t.Fatalf("expected empty replacement, got %q", f.Replacement)
	}
}

// Regression: when System.gc() appears as a sub-expression (here, an
// argument), the fix must NOT extend back to line start or eat a trailing
// `;`/`\n` — doing so would delete surrounding code on the line. The rule
// should still report a finding, but should not emit a Fix.
func TestExplicitGarbageCollectionCall_NoFixForInExpressionPosition(t *testing.T) {
	findings := runRuleByNameOnJava(t, "ExplicitGarbageCollectionCall", `package test;
class Example {
  void foo(Object x) {}
  void cleanup() {
    foo(System.gc());
  }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Fix != nil {
		t.Fatalf("expected no Fix for in-expression System.gc(), got %#v", findings[0].Fix)
	}
}

// Regression: multiple statements on the same line.
// `foo(); System.gc(); bar();` must not collapse — the fix would otherwise
// destroy `foo()` and `bar()`.
func TestExplicitGarbageCollectionCall_NoFixWithMultipleStatementsOnLine(t *testing.T) {
	findings := runRuleByNameOnJava(t, "ExplicitGarbageCollectionCall", `package test;
class Example {
  void foo() {}
  void bar() {}
  void cleanup() {
    foo(); System.gc(); bar();
  }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Fix != nil {
		t.Fatalf("expected no Fix when other statements share the line, got %#v", findings[0].Fix)
	}
}

// Regression: `{ System.gc(); }` on a single line must not delete the
// leading brace.
func TestExplicitGarbageCollectionCall_NoFixForSingleLineBlock(t *testing.T) {
	findings := runRuleByNameOnJava(t, "ExplicitGarbageCollectionCall", `package test;
class Example {
  void cleanup() { System.gc(); }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Fix != nil {
		t.Fatalf("expected no Fix for single-line block, got %#v", findings[0].Fix)
	}
}

// Regression: Kotlin `val x = System.gc()` has no trailing `;`, so the
// pre-fix logic would eat the newline and join with the next line. The
// guard must reject this position (prefix is not whitespace-only).
func TestExplicitGarbageCollectionCall_NoFixForKotlinAssignment(t *testing.T) {
	findings := runRuleByName(t, "ExplicitGarbageCollectionCall", `
package test
fun cleanup() {
    val x = System.gc()
    println(x)
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Fix != nil {
		t.Fatalf("expected no Fix for Kotlin assignment of System.gc(), got %#v", findings[0].Fix)
	}
}

// Regression: when System.gc() shares a line with preceding code (e.g.
// `if (cond) System.gc();`), the fix must not eat the leading code.
func TestExplicitGarbageCollectionCall_NoFixWhenSharingLineWithLeadingCode(t *testing.T) {
	findings := runRuleByNameOnJava(t, "ExplicitGarbageCollectionCall", `package test;
class Example {
  void cleanup(boolean cond) {
    if (cond) System.gc();
  }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Fix != nil {
		t.Fatalf("expected no Fix when call shares a line with leading code, got %#v", findings[0].Fix)
	}
}

// --- InvalidRange ---

func TestInvalidRange_Positive(t *testing.T) {
	findings := runRuleByName(t, "InvalidRange", `
package test
fun main() {
    for (i in 10..1) {
        println(i)
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for backwards range 10..1")
	}
}

func TestInvalidRange_Negative(t *testing.T) {
	findings := runRuleByName(t, "InvalidRange", `
package test
fun main() {
    for (i in 1..10) {
        println(i)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestInvalidRange_DownToCommentDoesNotSuppressBackwardsRange(t *testing.T) {
	findings := runRuleByName(t, "InvalidRange", `
package test
fun main() {
    for (i in 5..1) { // use downTo here
        println(i)
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for backwards range even when downTo appears in a comment")
	}
}

// --- IteratorHasNextCallsNextMethod ---

func TestIteratorHasNextCallsNextMethod_Positive(t *testing.T) {
	findings := runRuleByName(t, "IteratorHasNextCallsNextMethod", `
package test
class MyIterator : Iterator<Int> {
    override fun hasNext(): Boolean {
        next()
        return true
    }
    override fun next(): Int = 1
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for hasNext() calling next()")
	}
}

func TestIteratorHasNextCallsNextMethod_Negative(t *testing.T) {
	findings := runRuleByName(t, "IteratorHasNextCallsNextMethod", `
package test
class MyIterator : Iterator<Int> {
    override fun hasNext(): Boolean {
        return index < size
    }
    override fun next(): Int = 1
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- IteratorNotThrowingNoSuchElementException ---

func TestIteratorNotThrowingNoSuchElementException_Positive(t *testing.T) {
	findings := runRuleByName(t, "IteratorNotThrowingNoSuchElementException", `
package test
class MyIterator : Iterator<Int> {
    override fun hasNext(): Boolean = index < size
    override fun next(): Int {
        return items[index++]
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for next() without NoSuchElementException")
	}
}

func TestIteratorNotThrowingNoSuchElementException_Negative(t *testing.T) {
	findings := runRuleByName(t, "IteratorNotThrowingNoSuchElementException", `
package test
class MyIterator : Iterator<Int> {
    override fun hasNext(): Boolean = index < size
    override fun next(): Int {
        if (!hasNext()) throw NoSuchElementException()
        return items[index++]
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- LateinitUsage ---

func TestLateinitUsage_Positive(t *testing.T) {
	findings := runRuleByName(t, "LateinitUsage", `
package test
class Foo {
    lateinit var name: String
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for lateinit var")
	}
}

func TestLateinitUsage_Negative(t *testing.T) {
	findings := runRuleByName(t, "LateinitUsage", `
package test
class Foo {
    var name: String = ""
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestLateinitUsage_IgnoresTestFixtures(t *testing.T) {
	findings := runRuleByNameOnPath(t, "LateinitUsage", "src/test/kotlin/FooTest.kt", `
package test
class FooTest {
    lateinit var subject: Foo
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for test lateinit properties, got %d", len(findings))
	}
}

func TestLateinitUsage_IgnoresInjectedProperties(t *testing.T) {
	findings := runRuleByName(t, "LateinitUsage", `
package test
import javax.inject.Inject
class Foo {
    @Inject lateinit var service: Service
}
class Service
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for injected lateinit properties, got %d", len(findings))
	}
}

func TestLateinitUsage_HonorsIgnoreOnClassesPattern(t *testing.T) {
	// IgnoreOnClassesPattern was previously a dead config — exposed in
	// metadata but never consulted. Configure it via the rule pointer
	// and verify lateinit declarations inside matching classes are
	// skipped while non-matching ones still fire.
	var rule *rules.LateinitUsageRule
	for _, candidate := range api.Registry {
		if candidate.ID == "LateinitUsage" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.LateinitUsageRule)
			if !ok {
				t.Fatalf("expected LateinitUsageRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("LateinitUsage rule not registered")
	}
	original := rule.IgnoreOnClassesPattern
	defer func() { rule.IgnoreOnClassesPattern = original }()

	rule.IgnoreOnClassesPattern = api.CompileAnchoredPattern(
		"LateinitUsage", "ignoreOnClassesPattern", ".*Spec")

	if findings := runRuleByName(t, "LateinitUsage", `
package test
class FooSpec {
    lateinit var subject: String
}
`); len(findings) != 0 {
		t.Fatalf("expected no findings inside FooSpec when IgnoreOnClassesPattern matches, got %d", len(findings))
	}

	// A non-matching class still fires.
	if findings := runRuleByName(t, "LateinitUsage", `
package test
class FooImpl {
    lateinit var subject: String
}
`); len(findings) == 0 {
		t.Fatal("expected finding inside FooImpl (does not match Spec pattern)")
	}
}

// --- MissingPackageDeclaration ---

func TestMissingPackageDeclaration_Positive(t *testing.T) {
	findings := runRuleByName(t, "MissingPackageDeclaration", `
fun main() {
    println("hello")
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for missing package declaration")
	}
}

func TestMissingPackageDeclaration_Negative(t *testing.T) {
	findings := runRuleByName(t, "MissingPackageDeclaration", `
package test
fun main() {
    println("hello")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestMissingPackageDeclaration_Java(t *testing.T) {
	findings := runRuleByNameOnJavaPath(t, "MissingPackageDeclaration", "src/main/java/com/example/App.java", `
class App {
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 Java missing-package finding, got %d", len(findings))
	}
	if findings[0].Fix == nil || findings[0].Fix.Replacement != "package com.example\n\n" {
		t.Fatalf("expected Java package fix for com.example, got %#v", findings[0].Fix)
	}
}

func TestMissingPackageDeclaration_JavaNegative(t *testing.T) {
	findings := runRuleByNameOnJava(t, "MissingPackageDeclaration", `
package test;
class App {
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no Java missing-package findings, got %d", len(findings))
	}
}

// --- MissingSuperCall ---

func TestMissingSuperCall_Positive(t *testing.T) {
	findings := runRuleByName(t, "MissingSuperCall", `
package test

import android.app.Activity

class Child : Activity() {
    override fun onCreate() {
        println("child")
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for missing super.onCreate()")
	}
}

func TestMissingSuperCall_Negative(t *testing.T) {
	findings := runRuleByName(t, "MissingSuperCall", `
package test

import android.app.Activity

class Child : Activity() {
    override fun onCreate() {
        super.onCreate()
        println("child")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestMissingSuperCall_NegativeOrdinaryInterfaceOverride(t *testing.T) {
	findings := runRuleByName(t, "MissingSuperCall", `
package test
interface Logger {
    fun log(message: String)
}
class AndroidLogger : Logger {
    override fun log(message: String) {
        println(message)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for ordinary interface override, got %d", len(findings))
	}
}

func TestMissingSuperCall_NegativeLocalLifecycleLookalike(t *testing.T) {
	findings := runRuleByName(t, "MissingSuperCall", `
package test
open class Base {
    open fun onCreate() {}
}
class Child : Base() {
    override fun onCreate() {
        println("child")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for local onCreate lookalike, got %d", len(findings))
	}
}

func TestMissingSuperCall_HonorsMustInvokeSuperAnnotations(t *testing.T) {
	// MustInvokeSuperAnnotations was previously a dead config — exposed
	// in metadata but never consulted. Wired with same-file lookup: when
	// the parent's overridden method (declared in the same source file)
	// carries one of the configured annotations, the override is treated
	// as required-super even when the parent isn't an Android lifecycle
	// owner. Cross-file annotation lookup is intentionally out of scope.
	annotated := `package test
annotation class CallSuper
open class Base {
    @CallSuper
    open fun onAttach() {
        println("base")
    }
}
class Child : Base() {
    override fun onAttach() {
        println("child")
    }
}
`
	if findings := runRuleByName(t, "MissingSuperCall", annotated); len(findings) == 0 {
		t.Fatal("expected finding when parent's same-file method is annotated @CallSuper")
	}

	// Same shape, but Child calls super — no finding.
	withSuper := `package test
annotation class CallSuper
open class Base {
    @CallSuper
    open fun onAttach() {
        println("base")
    }
}
class Child : Base() {
    override fun onAttach() {
        super.onAttach()
        println("child")
    }
}
`
	if findings := runRuleByName(t, "MissingSuperCall", withSuper); len(findings) != 0 {
		t.Fatalf("expected no findings when override calls super.onAttach(), got %d", len(findings))
	}

	// Without the annotation, the override is just an ordinary one
	// against a non-lifecycle base — still no finding.
	noAnnotation := `package test
open class Base {
    open fun onAttach() {
        println("base")
    }
}
class Child : Base() {
    override fun onAttach() {
        println("child")
    }
}
`
	if findings := runRuleByName(t, "MissingSuperCall", noAnnotation); len(findings) != 0 {
		t.Fatalf("expected no findings for ordinary override without annotation, got %d", len(findings))
	}
}

// --- MissingUseCall ---

func TestMissingUseCall_Positive(t *testing.T) {
	findings := runRuleByName(t, "MissingUseCall", `
package test
fun readFile() {
    val stream = FileInputStream("file.txt")
    stream.read()
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for FileInputStream without .use {}")
	}
}

func TestMissingUseCall_Negative(t *testing.T) {
	findings := runRuleByName(t, "MissingUseCall", `
package test
fun readFile() {
    FileInputStream("file.txt").use { stream ->
        stream.read()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}
