package rules_test

import "testing"

// ---------- AbstractClassCanBeConcreteClass ----------

func TestAbstractClassCanBeConcreteClass_Positive(t *testing.T) {
	findings := runRuleByName(t, "AbstractClassCanBeConcreteClass", `
package test
abstract class Foo {
    fun bar() {}
}`)
	if len(findings) == 0 {
		t.Error("expected finding for abstract class with no abstract members")
	}
}

func TestAbstractClassCanBeConcreteClass_Negative(t *testing.T) {
	findings := runRuleByName(t, "AbstractClassCanBeConcreteClass", `
package test
abstract class Foo {
    abstract fun bar()
}`)
	if len(findings) != 0 {
		t.Errorf("unexpected findings: %v", findings)
	}
}

// ---------- AbstractClassCanBeInterface ----------

func TestAbstractClassCanBeInterface_Positive(t *testing.T) {
	findings := runRuleByName(t, "AbstractClassCanBeInterface", `
package test
abstract class Foo {
    abstract fun bar()
}`)
	if len(findings) == 0 {
		t.Error("expected finding for abstract class with no state")
	}
}

func TestAbstractClassCanBeInterface_Negative(t *testing.T) {
	findings := runRuleByName(t, "AbstractClassCanBeInterface", `
package test
abstract class Foo(val x: Int) {
    abstract fun bar()
}`)
	if len(findings) != 0 {
		t.Errorf("unexpected findings: %v", findings)
	}
}

// ---------- DataClassShouldBeImmutable ----------

func TestDataClassShouldBeImmutable_Positive(t *testing.T) {
	findings := runRuleByName(t, "DataClassShouldBeImmutable", `
package test
data class Foo(var name: String)`)
	if len(findings) == 0 {
		t.Error("expected finding for var in data class")
	}
}

func TestDataClassShouldBeImmutable_Negative(t *testing.T) {
	findings := runRuleByName(t, "DataClassShouldBeImmutable", `
package test
data class Foo(val name: String)`)
	if len(findings) != 0 {
		t.Errorf("unexpected findings: %v", findings)
	}
}

// ---------- DataClassContainsFunctions ----------

func TestDataClassContainsFunctions_Positive(t *testing.T) {
	findings := runRuleByName(t, "DataClassContainsFunctions", `
package test
data class Foo(val name: String) {
    fun greet() = "Hello $name"
}`)
	if len(findings) == 0 {
		t.Error("expected finding for data class with functions")
	}
}

func TestDataClassContainsFunctions_Negative(t *testing.T) {
	findings := runRuleByName(t, "DataClassContainsFunctions", `
package test
data class Foo(val name: String)`)
	if len(findings) != 0 {
		t.Errorf("unexpected findings: %v", findings)
	}
}

// ---------- ProtectedMemberInFinalClass ----------

func TestProtectedMemberInFinalClass_Positive(t *testing.T) {
	findings := runRuleByName(t, "ProtectedMemberInFinalClass", `
package test
class Foo {
    protected fun bar() {}
}`)
	if len(findings) == 0 {
		t.Error("expected finding for protected member in final class")
	}
}

func TestProtectedMemberInFinalClass_Negative(t *testing.T) {
	findings := runRuleByName(t, "ProtectedMemberInFinalClass", `
package test
open class Foo {
    protected fun bar() {}
}`)
	if len(findings) != 0 {
		t.Errorf("unexpected findings: %v", findings)
	}
}

// ---------- NestedClassesVisibility ----------

func TestNestedClassesVisibility_Positive(t *testing.T) {
	findings := runRuleByName(t, "NestedClassesVisibility", `
package test
internal class Foo {
    public class Bar
}`)
	if len(findings) == 0 {
		t.Error("expected finding for public nested class in internal class")
	}
}

func TestNestedClassesVisibility_Negative(t *testing.T) {
	findings := runRuleByName(t, "NestedClassesVisibility", `
package test
internal class Foo {
    class Bar
}`)
	if len(findings) != 0 {
		t.Errorf("unexpected findings: %v", findings)
	}
}

// ---------- UtilityClassWithPublicConstructor ----------

func TestUtilityClassWithPublicConstructor_Positive(t *testing.T) {
	findings := runRuleByName(t, "UtilityClassWithPublicConstructor", `
package test
class Util {
    companion object {
        fun doStuff() {}
    }
}`)
	if len(findings) == 0 {
		t.Error("expected finding for utility class with public constructor")
	}
}

func TestUtilityClassWithPublicConstructor_Negative(t *testing.T) {
	findings := runRuleByName(t, "UtilityClassWithPublicConstructor", `
package test
class Util private constructor() {
    companion object {
        fun doStuff() {}
    }
}`)
	if len(findings) != 0 {
		t.Errorf("unexpected findings: %v", findings)
	}
}

func TestUtilityClassWithPublicConstructor_FixInsertsConstructor(t *testing.T) {
	src := `package test
class Util {
    companion object {
        fun doStuff() {}
    }
}`
	findings := runRuleByName(t, "UtilityClassWithPublicConstructor", src)
	if len(findings) == 0 {
		t.Fatal("expected finding")
	}
	fix := findings[0].Fix
	if fix == nil {
		t.Fatal("expected fix to be present for no-constructor case")
	}
	if !fix.ByteMode {
		t.Error("expected byte-mode fix")
	}
	if fix.StartByte != fix.EndByte {
		t.Errorf("expected zero-width insertion, got range [%d,%d]", fix.StartByte, fix.EndByte)
	}
	if fix.Replacement != " private constructor()" {
		t.Errorf("unexpected replacement %q", fix.Replacement)
	}
	// Verify the resulting content is syntactically valid.
	result := src[:fix.StartByte] + fix.Replacement + src[fix.EndByte:]
	want := `package test
class Util private constructor() {
    companion object {
        fun doStuff() {}
    }
}`
	if result != want {
		t.Errorf("fix produced wrong content:\nwant: %q\ngot:  %q", want, result)
	}
}

func TestUtilityClassWithPublicConstructor_FixReplacesExplicitPublic(t *testing.T) {
	src := `package test
class Util public constructor() {
    companion object {
        fun doStuff() {}
    }
}`
	findings := runRuleByName(t, "UtilityClassWithPublicConstructor", src)
	if len(findings) == 0 {
		t.Fatal("expected finding for explicit public constructor")
	}
	fix := findings[0].Fix
	if fix == nil {
		t.Fatal("expected fix to be present for explicit-public case")
	}
	if fix.Replacement != "private" {
		t.Errorf("expected modifier swap to 'private', got %q", fix.Replacement)
	}
	result := src[:fix.StartByte] + fix.Replacement + src[fix.EndByte:]
	want := `package test
class Util private constructor() {
    companion object {
        fun doStuff() {}
    }
}`
	if result != want {
		t.Errorf("fix produced wrong content:\nwant: %q\ngot:  %q", want, result)
	}
}

// ---------- OptionalAbstractKeyword ----------

func TestOptionalAbstractKeyword_Positive(t *testing.T) {
	findings := runRuleByName(t, "OptionalAbstractKeyword", `
package test
interface Foo {
    abstract fun bar()
}`)
	if len(findings) == 0 {
		t.Error("expected finding for abstract keyword on interface member")
	}
}

func TestOptionalAbstractKeyword_Negative(t *testing.T) {
	findings := runRuleByName(t, "OptionalAbstractKeyword", `
package test
interface Foo {
    fun bar()
}`)
	if len(findings) != 0 {
		t.Errorf("unexpected findings: %v", findings)
	}
}

func TestOptionalAbstractKeyword_NestedAbstractClass_Negative(t *testing.T) {
	findings := runRuleByName(t, "OptionalAbstractKeyword", `
package test
interface Foo {
    abstract class Nested {
        abstract fun bar()
    }
}`)
	if len(findings) != 0 {
		t.Errorf("unexpected findings for nested abstract class members: %v", findings)
	}
}

func TestOptionalAbstractKeyword_DaggerStyleNestedBuilder_Negative(t *testing.T) {
	findings := runRuleByName(t, "OptionalAbstractKeyword", `
package test
interface Parent {
    fun s(): String

    abstract class SharedBuilder<B, C> {
        abstract fun build(): C
        abstract fun setValue(value: String): B
    }

    abstract class Builder : SharedBuilder<Builder, Parent>() {
        abstract override fun setValue(value: String): Builder
    }
}`)
	if len(findings) != 0 {
		t.Errorf("unexpected findings for recovered nested builder members: %v", findings)
	}
}

// ---------- ClassOrdering ----------

func TestClassOrdering_Positive(t *testing.T) {
	findings := runRuleByName(t, "ClassOrdering", `
package test
class Foo {
    fun bar() {}
    val x = 1
}`)
	if len(findings) == 0 {
		t.Error("expected finding for out-of-order class members")
	}
}

func TestClassOrdering_Negative(t *testing.T) {
	findings := runRuleByName(t, "ClassOrdering", `
package test
class Foo {
    val x = 1
    fun bar() {}
}`)
	if len(findings) != 0 {
		t.Errorf("unexpected findings: %v", findings)
	}
}

// ---------- ObjectLiteralToLambda ----------

func TestObjectLiteralToLambda_Positive(t *testing.T) {
	findings := runRuleByName(t, "ObjectLiteralToLambda", `
package test
val r = object : Runnable {
    override fun run() {
        println("hello")
    }
}`)
	if len(findings) == 0 {
		t.Error("expected finding for object literal convertible to lambda")
	}
}

func TestObjectLiteralToLambda_Negative(t *testing.T) {
	findings := runRuleByName(t, "ObjectLiteralToLambda", `
package test
val r = object : Runnable {
    override fun run() {
        println("hello")
    }
    fun extra() {}
}`)
	if len(findings) != 0 {
		t.Errorf("unexpected findings: %v", findings)
	}
}

// ---------- SerialVersionUIDInSerializableClass ----------

func TestSerialVersionUIDInSerializableClass_Positive(t *testing.T) {
	findings := runRuleByName(t, "SerialVersionUIDInSerializableClass", `
package test
class Foo : java.io.Serializable {
    val name = "test"
}`)
	if len(findings) == 0 {
		t.Error("expected finding for missing serialVersionUID")
	}
}

func TestSerialVersionUIDInSerializableClass_Negative(t *testing.T) {
	findings := runRuleByName(t, "SerialVersionUIDInSerializableClass", `
package test
class Foo : java.io.Serializable {
    companion object {
        const val serialVersionUID = 1L
    }
}`)
	if len(findings) != 0 {
		t.Errorf("unexpected findings: %v", findings)
	}
}
