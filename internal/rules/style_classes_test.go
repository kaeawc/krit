package rules_test

import (
	"strings"
	"testing"
)

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
    abstract fun onDismissTapped()
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

interface PlaylistContract {
    abstract class Controller :
        ViewModel(), @Suppress("DEPRECATION") BasePresenter<View> {
        abstract fun shuffle(enabled: Boolean)
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
abstract class BillingModule {
    @Binds
    abstract fun bindLedger(ledger: DiskLedger): Ledger
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
abstract class BillingModule {
    @dagger.Binds
    abstract fun bindLedger(ledger: DiskLedger): Ledger
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
    abstract fun create(parent: ViewGroup): OrderRowHolder
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for assisted factory abstract class, got: %v", findings)
	}
}

func TestAbstractClassCanBeInterface_NegativeProtectedConcreteHelper(t *testing.T) {
	findings := runRuleByName(t, "AbstractClassCanBeInterface", `
package test

interface Scorer

abstract class BaseRanker : Scorer {
    protected fun describe(entry: Entry?): String {
        return entry?.title.orEmpty()
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

abstract class ImageFilter {
    abstract fun apply(input: Image): Image

    fun label(): String {
        return this::class.java.simpleName
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

interface Job<T>
class Receipt

abstract class ReceiptJob : Job<Receipt> {
    abstract fun key(): String
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for abstract class with only generic interface supertype")
	}
}

func TestAbstractClassCanBeInterface_NegativeAbstractProperty(t *testing.T) {
	findings := runRuleByName(t, "AbstractClassCanBeInterface", `
package test

interface Job<T>
class Receipt

abstract class ReceiptJob : Job<Receipt> {
    abstract val receiptNumber: Long
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

func TestDataClassContainsFunctions_ReportsEachOffendingFunctionByName(t *testing.T) {
	findings := runRuleByName(t, "DataClassContainsFunctions", `package test
data class Foo(val name: String) {
    fun greet() = "hi"
    fun wave() = "wave"
    fun toJson() = "{}"
    override fun toString() = name
    operator fun plus(other: Foo) = other
}
`)
	if len(findings) != 3 {
		t.Fatalf("expected greet, wave and plus to be reported, got %d", len(findings))
	}
	want := "The data class Foo contains functions which are not registered conversion functions. The offending method is called "
	for _, f := range findings {
		if !strings.HasPrefix(f.Message, want) {
			t.Errorf("unexpected message %q", f.Message)
		}
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

func TestProtectedMemberInFinalClass_NegativeFixedVisibility(t *testing.T) {
	findings := runRuleByName(t, "ProtectedMemberInFinalClass", `
package test
class Foo : Base() {
    override fun onStart() {}
    @Implementation protected fun hook() {}
    companion object {
        @JvmStatic protected fun entry() {}
    }
}`)
	if len(findings) != 0 {
		t.Errorf("unexpected findings: %v", findings)
	}
}

func TestProtectedMemberInFinalClass_NegativeShadowClass(t *testing.T) {
	findings := runRuleByName(t, "ProtectedMemberInFinalClass", `
package test
@Implements(Widget::class)
class ShadowWidget {
    protected fun helper() {}
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

func TestOptionalAbstractKeyword_FunInterface_Positive(t *testing.T) {
	findings := runRuleByName(t, "OptionalAbstractKeyword", `
package test
fun interface Fn {
    abstract fun run()
}

fun interface Mapper<T> : Base {
    abstract fun map(value: T)
}`)
	if len(findings) != 2 {
		t.Errorf("expected 2 findings for abstract on fun interface members, got %d", len(findings))
	}
}

func TestOptionalAbstractKeyword_FunInterface_Negative(t *testing.T) {
	findings := runRuleByName(t, "OptionalAbstractKeyword", `
package test
fun interface Fn {
    fun run()
}

fun render(block: () -> Unit) {
    block()
}

abstract class Base {
    abstract fun run()
}`)
	if len(findings) != 0 {
		t.Errorf("unexpected findings: %v", findings)
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

func TestClassOrdering_ReportsEachMisplacedMemberWithDetektWording(t *testing.T) {
	findings := runRuleByName(t, "ClassOrdering", `package test
class Foo {
    constructor(x: Int)
    val p = 1
    fun f() {}
    companion object {}
    val q = 2
}
`)
	want := map[string]bool{
		"property `p` should be declared before secondary constructors.": true,
		"property `q` should be declared before secondary constructors.": true,
	}
	if len(findings) != len(want) {
		t.Fatalf("expected %d findings, got %d: %v", len(want), len(findings), findings)
	}
	for _, f := range findings {
		if !want[f.Message] {
			t.Errorf("unexpected message %q", f.Message)
		}
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

func TestObjectLiteralToLambda_NegativeKotlinInterfaceInFile(t *testing.T) {
	findings := runRuleByName(t, "ObjectLiteralToLambda", `
package test
interface Callback {
    fun onDone()
}
val c = object : Callback {
    override fun onDone() {
        println("done")
    }
}`)
	if len(findings) != 0 {
		t.Errorf("unexpected findings for a non-fun Kotlin interface: %v", findings)
	}
}

func TestObjectLiteralToLambda_PositiveFunInterfaceInFile(t *testing.T) {
	findings := runRuleByName(t, "ObjectLiteralToLambda", `
package test
fun interface Callback {
    fun onDone()
}
val c = object : Callback {
    override fun onDone() {
        println("done")
    }
}`)
	if len(findings) == 0 {
		t.Error("expected finding for a fun interface")
	}
}

func TestObjectLiteralToLambda_NegativeUnresolvedSupertype(t *testing.T) {
	findings := runRuleByName(t, "ObjectLiteralToLambda", `
package test
val c = object : some.pkg.Unknown {
    override fun onDone() {
        println("done")
    }
}`)
	if len(findings) != 0 {
		t.Errorf("unexpected findings for an unresolved supertype: %v", findings)
	}
}

// ---------- SerialVersionUIDInSerializableClass ----------

func TestSerialVersionUIDInSerializableClass_Positive(t *testing.T) {
	findings := runRuleByName(t, "SerialVersionUIDInSerializableClass", `
package test
class Foo : Serializable {
    val name = "test"
}`)
	if len(findings) == 0 {
		t.Error("expected finding for missing serialVersionUID")
	}
}

func TestSerialVersionUIDInSerializableClass_FixNoCompanion(t *testing.T) {
	findings := runRuleByName(t, "SerialVersionUIDInSerializableClass", `
package test
class Foo : Serializable {
    val name = "test"
}`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Fix == nil {
		t.Fatal("expected Fix to be populated")
	}
	if !strings.Contains(findings[0].Fix.Replacement, "companion object") {
		t.Errorf("expected fix to inject companion object, got %q", findings[0].Fix.Replacement)
	}
	if !strings.Contains(findings[0].Fix.Replacement, "private const val serialVersionUID: Long = 1L") {
		t.Errorf("expected fix to inject serialVersionUID property, got %q", findings[0].Fix.Replacement)
	}
}

func TestSerialVersionUIDInSerializableClass_FixExistingCompanion(t *testing.T) {
	findings := runRuleByName(t, "SerialVersionUIDInSerializableClass", `
package test
class Foo : Serializable {
    companion object {
        const val FOO = 1
    }
}`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Fix == nil {
		t.Fatal("expected Fix to be populated")
	}
	repl := findings[0].Fix.Replacement
	if strings.Contains(repl, "companion object {") {
		t.Errorf("fix should not declare a new companion object when one exists, got %q", repl)
	}
	if !strings.Contains(repl, "private const val serialVersionUID: Long = 1L") {
		t.Errorf("expected fix to inject serialVersionUID property, got %q", repl)
	}
}

func TestSerialVersionUIDInSerializableClass_Negative(t *testing.T) {
	findings := runRuleByName(t, "SerialVersionUIDInSerializableClass", `
package test
class Foo : Serializable {
    companion object {
        private const val serialVersionUID = 1L
    }
}`)
	if len(findings) != 0 {
		t.Errorf("unexpected findings: %v", findings)
	}
}

func TestSerialVersionUIDInSerializableClass_FollowsDetektSignatureAndPlacement(t *testing.T) {
	findings := runRuleByName(t, "SerialVersionUIDInSerializableClass", `package test
import java.io.Serializable
class WrongSignature : Serializable {
    companion object {
        const val serialVersionUID = 1L
    }
}
class WrongPlace : Serializable {
    private val serialVersionUID = 1L
}
object Single : Serializable
`)
	messages := map[int]string{}
	for _, f := range findings {
		messages[f.Line] = f.Message
	}
	if len(findings) != 3 {
		t.Fatalf("expected three findings, got %v", messages)
	}
	if messages[5] != "The property `serialVersionUID` signature is not correct. `serialVersionUID` should be `private` and `constant` and its type should be `Long`" {
		t.Errorf("line 5: %q", messages[5])
	}
	if messages[8] != "The class WrongPlace implements the `Serializable` interface and should thus define a `serialVersionUID`." {
		t.Errorf("line 8: %q", messages[8])
	}
	if messages[11] != "The object Single implements the `Serializable` interface and should thus define a `serialVersionUID`." {
		t.Errorf("line 11: %q", messages[11])
	}
}

func TestUtilityClassWithPublicConstructor_SkipsAbstractAndFrameworkClasses(t *testing.T) {
	for name, code := range map[string]string{
		"abstract":    "package test\nabstract class Helpers {\n    companion object { fun make() = 1 }\n}\n",
		"module":      "package test\n@Module\nclass Wiring {\n    companion object { fun make() = 1 }\n}\n",
		"implements":  "package test\n@Implements(Target::class)\nclass Shadow {\n    companion object { fun make() = 1 }\n}\n",
		"component":   "package test\n@Component\nclass Graph {\n    companion object { fun make() = 1 }\n}\n",
		"inject":      "package test\nclass Service @Inject constructor() {\n    companion object { fun make() = 1 }\n}\n",
		"assisted":    "package test\nclass Worker @AssistedInject constructor() {\n    companion object { fun make() = 1 }\n}\n",
		"supertype":   "package test\nclass Child : Base() {\n    companion object { fun make() = 1 }\n}\n",
		"interfaceOf": "package test\ninterface Api {\n    companion object { fun make() = 1 }\n}\n",
	} {
		if findings := runRuleByName(t, "UtilityClassWithPublicConstructor", code); len(findings) != 0 {
			t.Errorf("%s: expected no findings, got %v", name, findings)
		}
	}
}

func TestUtilityClassWithPublicConstructor_StillFlagsPlainUtilityClass(t *testing.T) {
	findings := runRuleByName(t, "UtilityClassWithPublicConstructor", "package test\nclass Helpers {\n    companion object { fun make() = 1 }\n}\n")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %v", findings)
	}
}

func TestOptionalAbstractKeyword_NestedAbstractClassInInterfaceKeepsAbstract(t *testing.T) {
	findings := runRuleByName(t, "OptionalAbstractKeyword", `
package test

interface Outer {
    fun run()

    abstract class Base(val x: Int) {
        abstract fun step()
        abstract val label: String
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for members of an abstract class nested in an interface, got %d", len(findings))
	}
}

func TestOptionalAbstractKeyword_FlagsAbstractMemberOfNestedInterface(t *testing.T) {
	findings := runRuleByName(t, "OptionalAbstractKeyword", `
package test

abstract class Outer {
    interface Inner {
        abstract fun run()
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestAbstractClassCanBeConcreteClass_SkipsModuleAndTestBase(t *testing.T) {
	findings := runRuleByName(t, "AbstractClassCanBeConcreteClass", `
package test

@Module
abstract class NetworkModule {
    companion object {
        @Provides fun client(): Client = Client()
    }
}

abstract class BaseScreenTest {
    @get:Rule val rule = createRule()

    @Test fun smoke() {}
}

@RunWith(Runner::class)
abstract class BaseRunnerTest
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestAbstractClassCanBeConcreteClass_StillFlagsPlainAbstractClass(t *testing.T) {
	findings := runRuleByName(t, "AbstractClassCanBeConcreteClass", `
package test

abstract class Helper {
    fun help() {}
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestAbstractClassCanBeInterface_SkipsStateDelegatesAndModules(t *testing.T) {
	findings := runRuleByName(t, "AbstractClassCanBeInterface", `
package test

@Module
abstract class BindingsModule {
    @Binds abstract fun bind(impl: Impl): Api
}

abstract class Lazy {
    val cache by lazy { build() }
    abstract fun build(): Int
}

abstract class WithInit {
    init { register() }
    abstract fun register()
}

abstract class WithCtor(seed: Int) {
    abstract fun next(): Int
}

abstract class WithLateinit {
    lateinit var name: String
    abstract fun run()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestAbstractClassCanBeInterface_StillFlagsPureAbstractClass(t *testing.T) {
	findings := runRuleByName(t, "AbstractClassCanBeInterface", `
package test

abstract class Listener {
    abstract fun onEvent(id: Int)
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestOptionalAbstractKeyword_FunInterface_NestedAndModified(t *testing.T) {
	findings := runRuleByName(t, "OptionalAbstractKeyword", `
package test
public fun interface Fn {
    abstract fun run()
}

class Holder {
    fun interface Inner {
        abstract fun call()
    }
}`)
	if len(findings) != 2 {
		t.Errorf("expected 2 findings, got %d", len(findings))
	}
}

func TestOptionalAbstractKeyword_NestedAbstractClassWithAnnotatedSupertypeIgnored(t *testing.T) {
	findings := runRuleByName(t, "OptionalAbstractKeyword", `
package test
interface Contract {
    interface View {
        fun show()
    }

    abstract class Presenter :
        Base(),
        @Suppress("DEPRECATION")
        some.pkg.BasePresenter<View>,
        Listener {

        /** Doc. */
        abstract fun setChannelId(channelId: String)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for members of the nested abstract class, got %d", len(findings))
	}
}

func TestUtilityClassWithPublicConstructor_NamespaceHolderOfNestedTypes(t *testing.T) {
	findings := runRuleByName(t, "UtilityClassWithPublicConstructor", `
package test
class Holder {
    class First(val id: String)
    interface Second
    companion object {
        const val NAME = "x"
    }
}`)
	if len(findings) != 0 {
		t.Errorf("a class that nests types is a namespace, got %v", findings)
	}
}

func TestUtilityClassWithPublicConstructor_EmptyCompanionIsNotStatic(t *testing.T) {
	findings := runRuleByName(t, "UtilityClassWithPublicConstructor", `
package test
class Holder {
    companion object
}`)
	if len(findings) != 0 {
		t.Errorf("an empty companion adds no static member, got %v", findings)
	}
}

func TestDataClassShouldBeImmutable_NoFixForOverrideOrSetter(t *testing.T) {
	findings := runRuleByName(t, "DataClassShouldBeImmutable", `
package test

data class State(
    override var title: String,
    var count: Int,
) : Titled {
    var cached: Int = 0
        set(value) { field = value }
    var plain: Int = 0
}
`)
	fixes := map[int]bool{}
	for _, f := range findings {
		fixes[f.Line] = f.Fix != nil
	}
	if len(findings) != 4 {
		t.Fatalf("expected 4 findings, got %d", len(findings))
	}
	if fixes[5] || fixes[8] {
		t.Errorf("override and setter properties must not get a val fix: %v", fixes)
	}
	if !fixes[6] || !fixes[10] {
		t.Errorf("plain var properties keep the fix: %v", fixes)
	}
}

func TestDataClassContainsFunctions_OnlyDirectNonOverrideMembers(t *testing.T) {
	findings := runRuleByName(t, "DataClassContainsFunctions", `
package test

data class WithCompanion(val a: Int) {
    companion object {
        fun make() = WithCompanion(1)
    }
    class Nested {
        fun helper() {}
    }
    override fun toString() = "x"
    fun toDto() = a
    val lazyValue: Int get() { fun local() = 1; return local() }
}

data class WithFunction(val a: Int) {
    fun doubled() = a * 2
}
`)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "WithFunction") {
		t.Fatalf("expected only WithFunction to be reported, got %v", findings)
	}
}

func TestSerialVersionUID_IgnoresInterfaces(t *testing.T) {
	findings := runRuleByName(t, "SerialVersionUIDInSerializableClass", `
package test
import java.io.Serializable
interface Marker : Serializable
class Impl : Serializable
`)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "Impl") {
		t.Fatalf("expected only Impl, got %v", findings)
	}
}

func TestUtilityClassWithPublicConstructor_SkipsOpenInitAndMetadataNames(t *testing.T) {
	findings := runRuleByName(t, "UtilityClassWithPublicConstructor", `
package test
open class Opener {
    companion object { fun a() = 1 }
}
class WithInit {
    init { println() }
    companion object { fun a() = 1 }
}
annotation class Anno {
    companion object { fun a() = 1 }
}
class Real {
    companion object {
        // metadata in the header text must not hide the class
        fun metadata() = 1
    }
}
`)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "Real") {
		t.Fatalf("expected only Real, got %v", findings)
	}
}

func TestObjectLiteralToLambda_SkipsRecursiveAndGenericMethods(t *testing.T) {
	src := `package test
val a = object : Runnable { override fun run() { println() } }
val b = object : Runnable { override fun run() { run() } }
val c = object : java.util.Comparator<Int> { override fun compare(a: Int, b: Int) = a - b }
`
	findings := runRuleByNameWithResolver(t, "ObjectLiteralToLambda", src)
	for _, f := range findings {
		if f.Line == 3 {
			t.Errorf("a recursive run() call would change meaning inside a lambda: %v", f)
		}
	}
	if len(findings) != 2 {
		t.Fatalf("expected findings for lines 2 and 4, got %v", findings)
	}
}

func TestClassOrdering_InitBlocksMixWithProperties(t *testing.T) {
	findings := runRuleByName(t, "ClassOrdering", `
package test
class A {
    init { println() }
    val a = 1
    init { println() }
    val b = 2
    constructor(x: Int)
    fun f() {}
}
`)
	if len(findings) != 0 {
		t.Fatalf("properties and init blocks may interleave, got %v", findings)
	}
}

func TestSerialVersionUIDInSerializableClass_OnlyDeclaredSerializableSupertype(t *testing.T) {
	findings := runRuleByName(t, "SerialVersionUIDInSerializableClass", `
package test

import java.util.concurrent.atomic.AtomicReference

interface Closeable2

class Holder<T> : AtomicReference<T>(), Closeable2

class Qualified : java.io.Serializable

class Ext : java.io.Externalizable
`)
	if len(findings) != 0 {
		t.Errorf("detekt only matches a declared `Serializable` entry, got %v", findings)
	}
	findings = runRuleByName(t, "SerialVersionUIDInSerializableClass", `
package test

class Direct : Closeable2(), Serializable
`)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding for a declared Serializable supertype, got %d", len(findings))
	}
}
