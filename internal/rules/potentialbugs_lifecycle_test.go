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

func TestExitOutsideMain_SkipsJava(t *testing.T) {
	findings := runRuleByNameOnJava(t, "ExitOutsideMain", `
package test;
class Example {
  void shutdown() {
    System.exit(1);
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("detekt does not analyze Java sources; expected no findings, got %d", len(findings))
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

func TestExplicitGarbageCollectionCall_SkipsJava(t *testing.T) {
	findings := runRuleByNameOnJava(t, "ExplicitGarbageCollectionCall", `
package test;
class Example {
  void cleanup() {
    System.gc();
    Runtime.getRuntime().gc();
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("detekt does not analyze Java sources; expected no findings, got %d", len(findings))
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

// Regression: when System.gc() appears as a sub-expression (here, an
// argument), the fix must NOT extend back to line start or eat a trailing
// `;`/`\n` — doing so would delete surrounding code on the line. The rule
// should still report a finding, but should not emit a Fix.
// Regression: multiple statements on the same line.
// `foo(); System.gc(); bar();` must not collapse — the fix would otherwise
// destroy `foo()` and `bar()`.
// Regression: `{ System.gc(); }` on a single line must not delete the
// leading brace.
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

func TestIteratorRules_FollowDetektShape(t *testing.T) {
	src := "package test\n" +
		"class Calls : Iterator<Int> {\n" + // line 2
		"    override fun hasNext() = next() > 1\n" +
		"    override fun next(): Int { throw NoSuchElementException() }\n" +
		"}\n" +
		"class Quiet : java.util.Iterator<Int> {\n" + // line 6
		"    override fun hasNext() = true\n" +
		"    override fun next(): Int = 1\n" +
		"}\n" +
		"class Other : ListIterator<Int> {\n" + // supertype name is not exactly Iterator
		"    override fun hasNext() = next() > 1\n" +
		"    override fun next(): Int = 1\n" +
		"}\n" +
		"val anon = object : Iterator<Int> {\n" + // line 14
		"    override fun hasNext() = true\n" +
		"    override fun next(): Int = 1\n" +
		"}\n"
	hasNext := runRuleByName(t, "IteratorHasNextCallsNextMethod", src)
	if len(hasNext) != 1 || hasNext[0].Line != 2 || hasNext[0].Col != 7 {
		t.Errorf("hasNext: want one finding on the class name at 2:7, got %+v", hasNext)
	}
	next := runRuleByName(t, "IteratorNotThrowingNoSuchElementException", src)
	if len(next) != 2 || next[0].Line != 6 || next[1].Line != 14 {
		t.Errorf("next: want findings on lines 6 and 14, got %+v", next)
	}
	if len(next) > 0 && next[0].Col != 7 {
		t.Errorf("next: want the class name column 7, got %d", next[0].Col)
	}
	qualified := "package test\nclass Q : Iterator<Int> {\n    override fun hasNext() = true\n    override fun next(): Int = throw java.util.NoSuchElementException()\n}\n"
	if got := runRuleByName(t, "IteratorNotThrowingNoSuchElementException", qualified); len(got) != 1 {
		t.Errorf("a qualified throw is not detekt's NoSuchElementException call shape: want 1 finding, got %d", len(got))
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

func TestMissingPackageDeclaration_SkipsJava(t *testing.T) {
	findings := runRuleByNameOnJavaPath(t, "MissingPackageDeclaration", "src/main/java/com/example/App.java", `
class App {
}
`)
	if len(findings) != 0 {
		t.Fatalf("detekt does not analyze Java sources; expected no findings, got %d", len(findings))
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

func TestExplicitGarbageCollectionCall_NegativeTestSources(t *testing.T) {
	findings := runRuleByNameOnPath(t, "ExplicitGarbageCollectionCall", "app/src/test/java/test/LeakTest.kt", `
package test
class LeakTest {
    fun forceCollection() {
        System.gc()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings in test sources, got %d", len(findings))
	}
}

func TestInvalidRange_LiteralValuesAndInfixForms(t *testing.T) {
	flagged := map[string]string{
		"underscore literal": "val r = 1000..9_99",
		"long literal":       "val r = 5L..1L",
		"hex literal":        "val r = 0xFF..0x10",
		"downTo ascending":   "val r = 1 downTo 5",
		"until backwards":    "val r = 5 until 1",
		"exclusive range":    "val r = 5..<1",
		"until equal":        "val r = 3 until 3",
		"exclusive equal":    "val r = 3..<3",
	}
	for name, src := range flagged {
		if got := runRuleByName(t, "InvalidRange", "package test\n"+src+"\n"); len(got) != 1 {
			t.Errorf("%s: want 1 finding, got %d", name, len(got))
		}
	}
	clean := map[string]string{
		"leading zeros":     "val r = 0x0001..5",
		"ascending":         "val r = 1..5",
		"downTo descending": "val r = 5 downTo 1",
		"until ascending":   "val r = 1 until 5",
		"exclusive ascend":  "val r = 1..<5",
		"equal inclusive":   "val r = 3..3",
		"hex ascending":     "val r = 0xA..0x10",
	}
	for name, src := range clean {
		if got := runRuleByName(t, "InvalidRange", "package test\n"+src+"\n"); len(got) != 0 {
			t.Errorf("%s: want no findings, got %d", name, len(got))
		}
	}
	got := runRuleByName(t, "InvalidRange", "package test\nval r = 5..<1\n")
	if len(got) != 1 || got[0].Fix != nil {
		t.Errorf("an exclusive range must not get a downTo fix, got %+v", got)
	}
}

func TestExplicitGarbageCollectionCall_FormsAndSpacing(t *testing.T) {
	flagged := []string{
		"System.runFinalization()",
		"java.lang.System.gc()",
		"Runtime.getRuntime()\n        .gc()",
		"Runtime.getRuntime().runFinalization()",
	}
	for _, call := range flagged {
		if got := runRuleByName(t, "ExplicitGarbageCollectionCall", "package test\nfun f() {\n    "+call+"\n}\n"); len(got) != 1 {
			t.Errorf("%q: want 1 finding, got %d", call, len(got))
		}
	}
	if got := runRuleByName(t, "ExplicitGarbageCollectionCall", "package test\nfun f(o: Any) {\n    o.gc()\n    gc()\n}\nfun gc() {}\nfun Any.gc() {}\n"); len(got) != 0 {
		t.Errorf("unrelated gc() calls must not be reported, got %d", len(got))
	}
}

func TestMissingUseCall_ExplicitCloseCountsAsRelease(t *testing.T) {
	closed := "package test\nimport java.io.FileInputStream\nfun f() {\n    val s = FileInputStream(\"x\")\n    try { s.read() } finally { s.close() }\n}\n"
	if got := runRuleByNameWithResolver(t, "MissingUseCall", closed); len(got) != 0 {
		t.Errorf("a stream closed in finally is released, got %v", got)
	}
	leaked := "package test\nimport java.io.FileInputStream\nfun f() {\n    val s = FileInputStream(\"x\")\n    s.read()\n}\n"
	if got := runRuleByNameWithResolver(t, "MissingUseCall", leaked); len(got) != 1 {
		t.Errorf("a stream that is never closed must still be reported, got %d", len(got))
	}
}

func TestLateinitUsage_ReportsOnTheLeadingComment(t *testing.T) {
	findings := runRuleByName(t, "LateinitUsage", "package test\nclass Foo {\n    // set in onCreate\n    lateinit var name: String\n\n    lateinit var other: String\n}\n")
	if len(findings) != 2 || findings[0].Line != 3 || findings[1].Line != 6 {
		t.Fatalf("want findings on the comment line 3 and on line 6, got %+v", findings)
	}
}

func TestExitOutsideMain_FollowsDetektMainFunction(t *testing.T) {
	exempt := map[string]string{
		"top level":       "fun main() { exitProcess(0) }",
		"array parameter": "fun main(args: Array<String>) { exitProcess(0) }",
		"vararg":          "fun main(vararg args: String) { exitProcess(0) }",
		"suspend":         "suspend fun main() { exitProcess(0) }",
		"jvm static":      "object App { @JvmStatic fun main(args: Array<String>) { exitProcess(0) } }",
		"backticked":      "fun `main`() { exitProcess(0) }",
	}
	for name, src := range exempt {
		if got := runRuleByName(t, "ExitOutsideMain", "package test\nimport kotlin.system.exitProcess\n"+src+"\n"); len(got) != 0 {
			t.Errorf("%s: want no findings, got %d", name, len(got))
		}
	}
	reported := map[string]string{
		"member of a class":      "class A { fun main() { exitProcess(0) } }",
		"object without static":  "object A { fun main() { exitProcess(0) } }",
		"extension":              "fun String.main() { exitProcess(0) }",
		"internal":               "internal fun main() { exitProcess(0) }",
		"private":                "private fun main(args: Array<String>) { exitProcess(0) }",
		"other parameter":        "fun main(code: Int) { exitProcess(code) }",
		"array of out String":    "fun main(args: Array<out String>) { exitProcess(0) }",
		"local function":         "fun outer() { fun main() { exitProcess(0) } }",
		"Runtime exit":           "fun stop() { Runtime.getRuntime().exit(0) }",
		"Runtime halt":           "fun stop() { Runtime.getRuntime().halt(0) }",
		"qualified Runtime exit": "fun stop() { java.lang.Runtime.getRuntime().exit(0) }",
	}
	for name, src := range reported {
		if got := runRuleByName(t, "ExitOutsideMain", "package test\nimport kotlin.system.exitProcess\n"+src+"\n"); len(got) != 1 {
			t.Errorf("%s: want 1 finding, got %d", name, len(got))
		}
	}
}
