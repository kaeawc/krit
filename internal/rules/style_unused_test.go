package rules_test

import "testing"

// --- UnusedImport ---

func TestUnusedImport_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnusedImport", `
package test
import kotlin.math.sqrt
fun main() {
    println("hello")
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for unused import")
	}
}

func TestUnusedImport_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnusedImport", `
package test
import kotlin.math.sqrt
fun main() {
    println(sqrt(4.0))
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- UnusedParameter ---

func TestUnusedParameter_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnusedParameter", `
package test
fun greet(name: String, unused: Int) {
    println(name)
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for unused parameter")
	}
}

func TestUnusedParameter_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnusedParameter", `
package test
fun greet(name: String) {
    println(name)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- UnusedVariable ---

func TestUnusedVariable_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test
fun main() {
    val unused = 42
    println("hello")
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for unused variable")
	}
}

func TestUnusedVariable_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test
fun main() {
    val x = 42
    println(x)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- UnusedPrivateClass ---

func TestUnusedPrivateClass_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnusedPrivateClass", `
package test
private class Unused
fun main() {
    println("hello")
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for unused private class")
	}
}

func TestUnusedPrivateClass_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnusedPrivateClass", `
package test
private class Helper
fun main() {
    val h = Helper()
    println(h)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- UnusedPrivateFunction ---

func TestUnusedPrivateFunction_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnusedPrivateFunction", `
package test
private fun unused() {}
fun main() {
    println("hello")
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for unused private function")
	}
}

func TestUnusedPrivateFunction_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnusedPrivateFunction", `
package test
private fun helper() = 42
fun main() {
    println(helper())
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- UnusedPrivateProperty ---

func TestUnusedPrivateProperty_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnusedPrivateProperty", `
package test
private val unused = 42
fun main() {
    println("hello")
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for unused private property")
	}
}

func TestUnusedPrivateProperty_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnusedPrivateProperty", `
package test
private val secret = 42
fun main() {
    println(secret)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- UnusedPrivateMember ---

func TestUnusedPrivateMember_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnusedPrivateMember", `
package test
class Foo {
    private fun unused() {}
    fun bar() {
        println("hello")
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for unused private member")
	}
}

func TestUnusedPrivateMember_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnusedPrivateMember", `
package test
class Foo {
    private fun helper() = 42
    fun bar() {
        println(helper())
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}
