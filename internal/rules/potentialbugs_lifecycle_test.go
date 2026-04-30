package rules_test

import "testing"

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
