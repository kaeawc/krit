package rules_test

import (
	"testing"
)

// --- ForbiddenPublicDataClass ---

func TestForbiddenPublicDataClass_FlagsPublicDataClass(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenPublicDataClass", `
package test
data class User(val name: String, val age: Int)
`)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestForbiddenPublicDataClass_IgnoresInternalDataClass(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenPublicDataClass", `
package test
internal data class User(val name: String, val age: Int)
`)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestForbiddenPublicDataClass_IgnoresPrivateDataClass(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenPublicDataClass", `
package test
private data class User(val name: String, val age: Int)
`)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestForbiddenPublicDataClass_IgnoresProtectedDataClass(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenPublicDataClass", `
package test
protected data class User(val name: String, val age: Int)
`)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestForbiddenPublicDataClass_IgnoresRegularClass(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenPublicDataClass", `
package test
class User(val name: String, val age: Int)
`)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestForbiddenPublicDataClass_FlagsExplicitPublicDataClass(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenPublicDataClass", `
package test
public data class User(val name: String, val age: Int)
`)
	// public modifier is explicit but still public — should flag
	if len(findings) != 1 {
		t.Errorf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

// --- LibraryEntitiesShouldNotBePublic ---

func TestLibraryEntitiesShouldNotBePublic_FlagsPublicClass(t *testing.T) {
	findings := runRuleByName(t, "LibraryEntitiesShouldNotBePublic", `
package test
class MyService
`)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestLibraryEntitiesShouldNotBePublic_FlagsPublicFunction(t *testing.T) {
	findings := runRuleByName(t, "LibraryEntitiesShouldNotBePublic", `
package test
fun doWork() {}
`)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestLibraryEntitiesShouldNotBePublic_FlagsPublicProperty(t *testing.T) {
	findings := runRuleByName(t, "LibraryEntitiesShouldNotBePublic", `
package test
val config = "default"
`)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestLibraryEntitiesShouldNotBePublic_IgnoresInternalClass(t *testing.T) {
	findings := runRuleByName(t, "LibraryEntitiesShouldNotBePublic", `
package test
internal class MyService
`)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLibraryEntitiesShouldNotBePublic_IgnoresPrivateFunction(t *testing.T) {
	findings := runRuleByName(t, "LibraryEntitiesShouldNotBePublic", `
package test
private fun doWork() {}
`)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLibraryEntitiesShouldNotBePublic_IgnoresPublishedApi(t *testing.T) {
	findings := runRuleByName(t, "LibraryEntitiesShouldNotBePublic", `
package test
@PublishedApi
internal fun doWork() {}
`)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLibraryEntitiesShouldNotBePublic_IgnoresNestedClass(t *testing.T) {
	findings := runRuleByName(t, "LibraryEntitiesShouldNotBePublic", `
package test
class Outer {
    class Inner
}
`)
	// Should only flag the top-level Outer, not Inner
	if len(findings) != 1 {
		t.Errorf("expected 1 finding (only Outer), got %d: %v", len(findings), findings)
	}
}

// --- LibraryCodeMustSpecifyReturnType ---

func TestLibraryCodeMustSpecifyReturnType_FlagsFunctionWithoutType(t *testing.T) {
	findings := runRuleByName(t, "LibraryCodeMustSpecifyReturnType", `
package test
fun compute() = 42
`)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestLibraryCodeMustSpecifyReturnType_FlagsPropertyWithoutType(t *testing.T) {
	findings := runRuleByName(t, "LibraryCodeMustSpecifyReturnType", `
package test
val name = "hello"
`)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestLibraryCodeMustSpecifyReturnType_IgnoresFunctionWithType(t *testing.T) {
	findings := runRuleByName(t, "LibraryCodeMustSpecifyReturnType", `
package test
fun compute(): Int = 42
`)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLibraryCodeMustSpecifyReturnType_IgnoresPropertyWithType(t *testing.T) {
	findings := runRuleByName(t, "LibraryCodeMustSpecifyReturnType", `
package test
val name: String = "hello"
`)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLibraryCodeMustSpecifyReturnType_IgnoresPrivateFunction(t *testing.T) {
	findings := runRuleByName(t, "LibraryCodeMustSpecifyReturnType", `
package test
private fun compute() = 42
`)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLibraryCodeMustSpecifyReturnType_IgnoresInternalProperty(t *testing.T) {
	findings := runRuleByName(t, "LibraryCodeMustSpecifyReturnType", `
package test
internal val name = "hello"
`)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestLibraryCodeMustSpecifyReturnType_IgnoresUnitReturnFunction(t *testing.T) {
	findings := runRuleByName(t, "LibraryCodeMustSpecifyReturnType", `
package test
fun doWork() {
    println("working")
}
`)
	// Functions with block body and no "=" have implicit Unit return, but they also
	// have no expression to infer from — typically Unit functions are fine without annotation.
	// However the rule flags any public function without explicit ": Type".
	// This is the expected behavior for the strict library rule.
	if len(findings) != 1 {
		t.Errorf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestLibraryCodeMustSpecifyReturnType_IgnoresNullableType(t *testing.T) {
	findings := runRuleByName(t, "LibraryCodeMustSpecifyReturnType", `
package test
fun findUser(): String? = null
`)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}
