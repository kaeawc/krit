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

func TestUnusedParameter_StructuralUsage(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		wantHits int
	}{
		{
			name: "substring collision does not count",
			code: `
package test
fun ok(id: String) {
    val guid = "x"
    println(guid)
}
`,
			wantHits: 1,
		},
		{
			name: "comments and strings do not count",
			code: `
package test
fun bad(id: String) {
    // id
    println("id")
}
`,
			wantHits: 1,
		},
		{
			name: "lambda capture counts",
			code: `
package test
fun ok(id: String) {
    listOf(1).map { id.length + it }
}
`,
			wantHits: 0,
		},
		{
			name: "string interpolation counts",
			code: `
package test
fun ok(endpoint: String) {
    println("https://example.com/$endpoint")
}
`,
			wantHits: 0,
		},
		{
			name: "lambda parameter shadows function parameter",
			code: `
package test
fun bad(id: String) {
    listOf("x").forEach { id -> println(id) }
}
`,
			wantHits: 1,
		},
		{
			name: "local destructuring shadows function parameter",
			code: `
package test
data class PairBox(val id: String, val value: String)
fun bad(id: String, box: PairBox) {
    run {
        val (id, value) = box
        println(id + value)
    }
}
`,
			wantHits: 1,
		},
		{
			name: "nested function body does not count",
			code: `
package test
fun bad(id: String) {
    fun nested() {
        println(id)
    }
    nested()
}
`,
			wantHits: 1,
		},
		{
			name: "override is excluded",
			code: `
package test
interface Base {
    fun render(id: String)
}
class Impl : Base {
    override fun render(id: String) {
        println("unused by override")
    }
}
`,
			wantHits: 0,
		},
		{
			name: "entrypoint annotation is excluded",
			code: `
package test
annotation class Subscribe
@Subscribe
fun onEvent(event: String) {
    println("framework")
}
`,
			wantHits: 0,
		},
		{
			name: "allowed name is excluded",
			code: `
package test
fun ok(ignored: String, expected: Int, _: Boolean) {
    println("placeholder")
}
`,
			wantHits: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := runRuleByName(t, "UnusedParameter", tt.code)
			if len(findings) != tt.wantHits {
				t.Fatalf("expected %d findings, got %d: %#v", tt.wantHits, len(findings), findings)
			}
		})
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
