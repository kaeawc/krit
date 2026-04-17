package rules_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// --- UnsafeCast ---

func TestUnsafeCast_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCast", `
package test
fun process(obj: Any) {
    val str = obj as String
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for unsafe cast 'as String', got none")
	}
}

func runRuleByNameWithResolver(t *testing.T, ruleName string, code string) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	for _, r := range rules.Registry {
		if r.Name() == ruleName {
			d := rules.NewDispatcher([]rules.Rule{r}, resolver)
			return d.Run(file)
		}
	}
	t.Fatalf("rule %q not found in registry", ruleName)
	return nil
}

func TestUnsafeCast_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCast", `
package test
fun process(obj: Any) {
    val str = obj as? String
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for safe cast 'as?', got %d", len(findings))
	}
}

func TestUnsafeCast_IgnoresMultiplatformTestRoots(t *testing.T) {
	for _, root := range []string{"commonJvmTest", "browserCommonTest", "jvmCommonTest"} {
		t.Run(root, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "src", root, "kotlin", "com", "example", "UnsafeCastTest.kt")
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			code := `
package test
fun process(record: Any) {
    val text = (record as String)
}
`
			if err := os.WriteFile(path, []byte(code), 0644); err != nil {
				t.Fatal(err)
			}
			file, err := scanner.ParseFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range rules.Registry {
				if r.Name() != "UnsafeCast" {
					continue
				}
				findings := rules.NewDispatcher([]rules.Rule{r}).Run(file)
				if len(findings) != 0 {
					t.Fatalf("expected no findings for %s source set, got %d", root, len(findings))
				}
				return
			}
			t.Fatal("UnsafeCast rule not found in registry")
		})
	}
}

// --- UnsafeCallOnNullableType ---

func TestUnsafeCallOnNullableType_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test
fun greet(name: String?) {
    val len = name!!.length
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for !! operator, got none")
	}
}

func TestUnsafeCallOnNullableType_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test
fun greet(name: String?) {
    val len = name?.length
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for safe call ?., got %d", len(findings))
	}
}

func TestUnsafeCallOnNullableType_NegativeKspQualifiedName(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test
import com.google.devtools.ksp.symbol.KSClassDeclaration

fun render(clazz: KSClassDeclaration) {
    val fqName = clazz.qualifiedName!!.asString()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for KSP qualifiedName unwrap, got %d", len(findings))
	}
}

func TestUnsafeCallOnNullableType_PositiveQualifiedName(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test
import kotlin.reflect.KClass

fun render(clazz: KClass<*>) {
    val fqName = clazz.qualifiedName!!
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for qualifiedName!! outside KSP, got none")
	}
}

func TestUnsafeCallOnNullableType_NegativeCreatorOrConstructorKsp(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test

import com.google.devtools.ksp.symbol.KSFunctionDeclaration

fun render(creatorOrConstructor: KSFunctionDeclaration?) {
    val name = creatorOrConstructor!!.simpleName.getShortName()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for creatorOrConstructor!! in KSP code, got %d", len(findings))
	}
}

func TestUnsafeCallOnNullableType_PositiveCreatorOrConstructor(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test

fun render(creatorOrConstructor: String?) {
    val name = creatorOrConstructor!!
}
`)
	if len(findings) == 0 {
		t.Fatal("expected ordinary creatorOrConstructor!! to still be flagged, got none")
	}
}

func TestUnsafeCallOnNullableType_CompilerLookupPositive(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test

import org.jetbrains.kotlin.ir.util.referenceClass

fun process(pluginContext: Any, classId: Any) {
    val klass = pluginContext.referenceClass(classId)!!
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected compiler symbol lookup !! to be clean, got %d findings", len(findings))
	}
}

func TestUnsafeCallOnNullableType_CompilerLookupNegative(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test

import org.jetbrains.kotlin.ir.util.referenceClass

fun process(name: String?) {
    val len = name!!
}
`)
	if len(findings) == 0 {
		t.Fatal("expected ordinary !! inside compiler-importing file to still be flagged")
	}
}

func TestUnsafeCallOnNullableType_CompilerSymbolMetadataPositive(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test

import org.jetbrains.kotlin.backend.common.extensions.IrPluginContext

fun process(pluginContext: IrPluginContext, classId: Any) {
    val klass = pluginContext.referenceClass(classId)!!
    val companion = klass.companionObject()!!
    val fqName = companion.classId!!.asString()
    val creator = companion.creatorOrConstructor!!
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected compiler symbol metadata !! to be clean, got %d findings", len(findings))
	}
}

func TestUnsafeCallOnNullableType_CompilerSymbolMetadataNegative(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test

import org.jetbrains.kotlin.backend.common.extensions.IrPluginContext

fun process(pluginContext: IrPluginContext, name: String?) {
    val len = name!!
}
`)
	if len(findings) == 0 {
		t.Fatal("expected ordinary !! in compiler-importing file to still be flagged")
	}
}

func TestUnsafeCallOnNullableType_NegativePostFilterSmartCast(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test

data class State(val inAppPaymentId: String?)

fun ids(states: List<State>): List<String> {
    return states
        .filter { it.inAppPaymentId != null }
        .map { it.inAppPaymentId!! }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for post-filter smart cast, got %d", len(findings))
	}
}

func TestUnsafeCallOnNullableType_NegativePostFilterSmartCastNestedCallArg(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test

data class State(val inAppPaymentId: String?)

class ViewModel(val state: List<State>)

fun consume(any: Any) {}

fun bind(viewModel: ViewModel) {
    consume(viewModel.state.filter { it.inAppPaymentId != null }.map { it.inAppPaymentId!! })
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for nested-call post-filter smart cast, got %d", len(findings))
	}
}

func TestUnsafeCallOnNullableType_NegativeTextUtilsIsEmptyElseBranch(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test

object TextUtils {
    fun isEmpty(value: String?): Boolean = value == null || value.isEmpty()
}

fun normalize(query: String?): String {
    return if (TextUtils.isEmpty(query)) "" else query!!
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for TextUtils.isEmpty else-branch smart cast, got %d", len(findings))
	}
}

// --- NullableToStringCall ---

func TestNullableToStringCall_Positive(t *testing.T) {
	findings := runRuleByName(t, "NullableToStringCall", `
package test
fun display(value: Int?) {
    val text = value?.toString()
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for nullable toString(), got none")
	}
}

func TestNullableToStringCall_Negative(t *testing.T) {
	findings := runRuleByName(t, "NullableToStringCall", `
package test
fun display(value: Int) {
    val text = value.toString()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for non-nullable toString(), got %d", len(findings))
	}
}

// --- NullCheckOnMutableProperty ---

func TestNullCheckOnMutableProperty_Positive(t *testing.T) {
	findings := runRuleByName(t, "NullCheckOnMutableProperty", `
package test
class Foo {
    var name: String? = null
    fun check() {
        if (name != null) {
            println(name)
        }
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for null check on mutable property, got none")
	}
}

func TestNullCheckOnMutableProperty_Negative(t *testing.T) {
	findings := runRuleByName(t, "NullCheckOnMutableProperty", `
package test
class Foo {
    val name: String? = null
    fun check() {
        if (name != null) {
            println(name)
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for null check on immutable val, got %d", len(findings))
	}
}

// --- MapGetWithNotNullAssertionOperator ---

func TestMapGetWithNotNullAssertion_Positive(t *testing.T) {
	findings := runRuleByName(t, "MapGetWithNotNullAssertionOperator", `
package test
fun lookup(map: Map<String, Int>) {
    val value = map["key"]!!
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for map[key]!!, got none")
	}
}

func TestMapGetWithNotNullAssertion_Negative(t *testing.T) {
	findings := runRuleByName(t, "MapGetWithNotNullAssertionOperator", `
package test
fun lookup(map: Map<String, Int>) {
    val value = map.getValue("key")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for getValue(), got %d", len(findings))
	}
}

// --- CastNullableToNonNullableType ---

func TestCastNullableToNonNullableType_Positive(t *testing.T) {
	findings := runRuleByName(t, "CastNullableToNonNullableType", `
package test
fun convert(obj: Any?) {
    val str = obj? as String
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for casting nullable to non-nullable, got none")
	}
}

func TestCastNullableToNonNullableType_Negative(t *testing.T) {
	findings := runRuleByName(t, "CastNullableToNonNullableType", `
package test
fun convert(obj: Any?) {
    val str = obj as? String
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for safe cast as?, got %d", len(findings))
	}
}

// --- CastToNullableType ---

func TestCastToNullableType_Positive(t *testing.T) {
	findings := runRuleByName(t, "CastToNullableType", `
package test
fun convert(obj: Any) {
    val str = obj as String?
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for 'as String?', got none")
	}
}

func TestCastToNullableType_Negative(t *testing.T) {
	findings := runRuleByName(t, "CastToNullableType", `
package test
fun convert(obj: Any) {
    val str = obj as? String
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for safe cast as?, got %d", len(findings))
	}
}

// --- UnnecessaryNotNullCheck ---

func TestUnnecessaryNotNullCheck_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryNotNullCheck", `
package test
fun check() {
    val name: String = "hello"
    if (name != null) {
        println(name)
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for unnecessary null check on non-nullable val, got none")
	}
}

func TestUnnecessaryNotNullCheck_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryNotNullCheck", `
package test
fun check() {
    val name: String? = null
    if (name != null) {
        println(name)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for null check on nullable val, got %d", len(findings))
	}
}

// --- UnnecessarySafeCall ---

func TestUnnecessarySafeCall_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnnecessarySafeCall", `
package test
fun check() {
    val name: String = "hello"
    val len = name?.length
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for unnecessary safe call on non-nullable val, got none")
	}
}

func TestUnnecessarySafeCall_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnnecessarySafeCall", `
package test
fun check() {
    val name: String? = null
    val len = name?.length
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for safe call on nullable val, got %d", len(findings))
	}
}

func TestUnnecessarySafeCall_NegativeNullableExtensionPropertyReceiver(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessarySafeCall", `
package test

class DataMessage(val groupV2: GroupContextV2?)
class GroupContextV2(val masterKey: ByteArray?)

val DataMessage?.hasGroupContext: Boolean
    get() = this?.groupV2?.masterKey.isNotEmpty()
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for nullable extension property receiver, got %d", len(findings))
	}
}

func TestUnnecessarySafeCall_PositiveNonNullCanvasParameter(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessarySafeCall", `
package test

class Canvas {
    fun draw() {}
}

fun onDraw(canvas: Canvas) {
    canvas?.draw()
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for unnecessary safe call on non-null canvas parameter")
	}
}

// --- UnnecessaryNotNullOperator ---

func TestUnnecessaryNotNullOperator_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryNotNullOperator", `
package test
fun check() {
    val name: String = "hello"
    val len = name!!.length
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for unnecessary !! on non-nullable val, got none")
	}
}

func TestUnnecessaryNotNullOperator_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryNotNullOperator", `
package test
fun check() {
    val name: String? = null
    val len = name!!.length
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for !! on nullable val, got %d", len(findings))
	}
}

func TestUnnecessaryNotNullOperator_NegativeNullableReceiverInApply(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessaryNotNullOperator", `
package test

class Typeface(val style: Int)
class TextPaint(var typeface: Typeface?)

fun update(tp: TextPaint?) {
    tp.apply {
        val old = this!!.typeface
        println(old)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for nullable receiver in apply, got %d", len(findings))
	}
}

func TestUnnecessaryNotNullOperator_NegativeNullableGenericDocument(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessaryNotNullOperator", `
package test

interface Document<I> {
    val items: MutableList<I>
}

fun <D : Document<I>?, I> consume(document: D) {
    val iterator = document!!.items.iterator()
    println(iterator)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for nullable generic document, got %d", len(findings))
	}
}

func TestUnnecessaryNotNullOperator_NegativeNullableGenericLocalVal(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessaryNotNullOperator", `
package test

interface Document<I> {
    val items: MutableList<I>
}

fun <D : Document<I>?, I> load(input: D): D = input

fun <D : Document<I>?, I> consume(input: D) {
    val document: D = load(input)
    val iterator = document!!.items.iterator()
    println(iterator)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for nullable generic local val, got %d", len(findings))
	}
}

func TestUnnecessaryNotNullOperator_NegativeNullableGenericLocalValInsideLambda(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessaryNotNullOperator", `
package test

interface Document<I> {
    val items: MutableList<I>
}

class Db

fun <T> withinTransaction(block: (Db) -> T): T {
    throw RuntimeException()
}

fun <D : Document<I>?, I> getDocument(db: Db, clazz: Class<D>): D {
    throw RuntimeException()
}

fun <D : Document<I>?, I> consume(clazz: Class<D>) {
    withinTransaction { db ->
        val document: D = getDocument(db, clazz)
        val iterator = document!!.items.iterator()
        println(iterator)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for nullable generic local val inside lambda, got %d", len(findings))
	}
}

func TestUnnecessaryNotNullOperator_NegativeNullableApplyReceiverThis(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessaryNotNullOperator", `
package test

class Typeface(val style: Int)
class TextPaint(var typeface: Typeface?)

fun <T> T.apply(block: T.() -> Unit): T {
    block()
    return this
}

fun update(tp: TextPaint?) {
    tp.apply {
        val old = this!!.typeface
        println(old)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for nullable apply receiver this!!, got %d", len(findings))
	}
}
