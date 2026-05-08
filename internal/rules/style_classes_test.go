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

func TestAbstractClassCanBeInterface_NegativeInheritedStateSuperclass(t *testing.T) {
	findings := runRuleByName(t, "AbstractClassCanBeInterface", `
package test

abstract class BasePresenter {
    val scope = Any()
}

abstract class Presenter : BasePresenter {
    abstract fun render()
}`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for abstract class inheriting superclass state, got: %v", findings)
	}
}

func TestAbstractClassCanBeInterface_NegativeViewModelConstructorSuperclass(t *testing.T) {
	findings := runRuleByName(t, "AbstractClassCanBeInterface", `
package test

open class ViewModel
interface BasePresenter<T>
class View

abstract class Presenter : ViewModel(), BasePresenter<View> {
    abstract fun onCloseButtonClicked()
}`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for abstract class extending concrete ViewModel superclass, got: %v", findings)
	}
}

func TestAbstractClassCanBeInterface_NegativeNestedViewModelConstructorSuperclass(t *testing.T) {
	findings := runRuleByName(t, "AbstractClassCanBeInterface", `
package test

open class ViewModel
interface BasePresenter<T>
class View

interface MediaCaptureContract {
    abstract class Presenter :
        ViewModel(), @Suppress("DEPRECATION") BasePresenter<View> {
        abstract fun setMode(mode: String)
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for nested abstract presenter extending ViewModel, got: %v", findings)
	}
}

func TestAbstractClassCanBeInterface_NegativeDaggerModule(t *testing.T) {
	findings := runRuleByName(t, "AbstractClassCanBeInterface", `
package test

annotation class Module
annotation class Binds

@Module
abstract class ConnectHubModule {
    @Binds
    abstract fun bindResolver(resolver: RealResolver): FragmentResolver
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for Dagger abstract module, got: %v", findings)
	}
}

func TestAbstractClassCanBeInterface_NegativeFqnDaggerModule(t *testing.T) {
	findings := runRuleByName(t, "AbstractClassCanBeInterface", `
package test

@dagger.Module
abstract class ConnectHubModule {
    @dagger.Binds
    abstract fun bindResolver(resolver: RealResolver): FragmentResolver
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for FQN Dagger abstract module, got: %v", findings)
	}
}

func TestAbstractClassCanBeInterface_NegativeAssistedFactory(t *testing.T) {
	findings := runRuleByName(t, "AbstractClassCanBeInterface", `
package test

annotation class AssistedFactory

@AssistedFactory
abstract class Factory {
    abstract fun create(parent: ViewGroup): TabStickersViewHolder
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for assisted factory abstract class, got: %v", findings)
	}
}

func TestAbstractClassCanBeInterface_NegativeProtectedConcreteHelper(t *testing.T) {
	findings := runRuleByName(t, "AbstractClassCanBeInterface", `
package test

interface BonusPointScorer

abstract class BaseUniversalResultMatcher : BonusPointScorer {
    protected fun unwrapUniversalResult(item: HasId?): Any? {
        return item
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for protected concrete helper method, got: %v", findings)
	}
}

func TestAbstractClassCanBeInterface_NegativePublicConcreteHelper(t *testing.T) {
	findings := runRuleByName(t, "AbstractClassCanBeInterface", `
package test

abstract class BitmapTransformer {
    abstract fun transform(input: Bitmap): Bitmap

    fun cacheKey(): String {
        return this::class.java.name
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for concrete helper method, got: %v", findings)
	}
}

func TestAbstractClassCanBeInterface_PositiveGenericInterfaceSupertype(t *testing.T) {
	findings := runRuleByName(t, "AbstractClassCanBeInterface", `
package test

interface PendingAction<T>
class Draft

abstract class DraftPendingAction : PendingAction<Draft> {
    abstract fun id(): String
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for abstract class with only generic interface supertype")
	}
}

func TestAbstractClassCanBeInterface_NegativeAbstractProperty(t *testing.T) {
	findings := runRuleByName(t, "AbstractClassCanBeInterface", `
package test

interface PendingAction<T>
class Draft

abstract class DraftPendingAction : PendingAction<Draft> {
    abstract val draftLocalId: Long
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for abstract class carrying abstract property contract, got: %v", findings)
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

func TestDataClassShouldBeImmutable_BodyVarPositive(t *testing.T) {
	findings := runRuleByName(t, "DataClassShouldBeImmutable", `
package test
data class Foo(val name: String) {
    var age: Int = 0
}`)
	if len(findings) == 0 {
		t.Error("expected finding for var body property in data class")
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

func TestDataClassShouldBeImmutable_NegativeStringVarMention(t *testing.T) {
	findings := runRuleByName(t, "DataClassShouldBeImmutable", `
package test
data class Foo(val name: String) {
    val sample = "var should not matter"
}`)
	if len(findings) != 0 {
		t.Errorf("unexpected findings for string var mention: %v", findings)
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

func TestDataClassContainsFunctions_HonorsConversionFunctionPrefix(t *testing.T) {
	// ConversionFunctionPrefix was previously a dead config. With the
	// default of ["to"], a data class containing only conversion functions
	// like toJson()/toDto() should not fire.
	if findings := runRuleByName(t, "DataClassContainsFunctions", `
package test
data class Foo(val name: String) {
    fun toJson(): String = "{name=$name}"
    fun toDto(): Dto = Dto(name)
}
class Dto(val name: String)
`); len(findings) != 0 {
		t.Fatalf("expected no findings under default ConversionFunctionPrefix=[to], got %d", len(findings))
	}

	// A non-conversion function still fires alongside conversion ones.
	if findings := runRuleByName(t, "DataClassContainsFunctions", `
package test
data class Foo(val name: String) {
    fun toJson(): String = "{name=$name}"
    fun greet(): String = "hi $name"
}
`); len(findings) == 0 {
		t.Fatal("expected finding when data class has non-conversion fun greet() alongside conversion ones")
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
