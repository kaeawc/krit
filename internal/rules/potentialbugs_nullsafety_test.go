package rules_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
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
	for _, r := range v2rules.Registry {
		if r.ID == ruleName {
			d := rules.NewDispatcherV2([]*v2rules.Rule{r}, resolver)
			cols := d.Run(file)
			return cols.Findings()
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

func TestUnsafeCast_DoesNotSuppressSubstringCallee(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCast", `
package test
fun findViewByIdButNotReally(): Any = ""
fun process() {
    val str = findViewByIdButNotReally() as String
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for local callee whose name merely contains findViewById")
	}
}

func TestUnsafeCast_SuppressesResolvedGetSystemServiceCast(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnsafeCast", `
package test
import android.app.AlarmManager
import android.content.Context

fun process(context: Context) {
    val manager = context.getSystemService(Context.ALARM_SERVICE) as AlarmManager
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no finding for Context.ALARM_SERVICE cast to AlarmManager, got %d", len(findings))
	}
}

func TestUnsafeCast_FlagsMismatchedGetSystemServiceCast(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnsafeCast", `
package test
import android.content.Context
import android.os.PowerManager

fun process(context: Context) {
    val manager = context.getSystemService(Context.ALARM_SERVICE) as PowerManager
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for Context.ALARM_SERVICE cast to PowerManager")
	}
}

func TestUnsafeCast_SuppressesUnqualifiedGetSystemServiceInContextOwner(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnsafeCast", `
package test
import android.app.Service

class SyncService : Service() {
    fun process() {
        val manager = getSystemService(POWER_SERVICE) as android.os.PowerManager
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no finding for POWER_SERVICE cast inside Service owner, got %d", len(findings))
	}
}

func TestUnsafeCast_FallsBackAfterLexicalOracleCallTarget(t *testing.T) {
	code := `
package test
import android.app.Service

class SyncService : Service() {
    fun process() {
        val manager = getSystemService(POWER_SERVICE) as android.os.PowerManager
    }
}
`
	file := parseInline(t, code)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	fake := oracle.NewFakeOracle()
	fake.CallTargets[file.Path] = map[string]string{}
	file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
		if strings.Contains(file.FlatNodeText(idx), "getSystemService") {
			key := fmt.Sprintf("%d:%d", file.FlatRow(idx)+1, file.FlatCol(idx)+1)
			fake.CallTargets[file.Path][key] = "getSystemService"
		}
	})
	composite := oracle.NewCompositeResolver(fake, resolver)

	for _, r := range v2rules.Registry {
		if r.ID != "UnsafeCast" {
			continue
		}
		cols := rules.NewDispatcherV2([]*v2rules.Rule{r}, composite).Run(file)
		findings := cols.Findings()
		if len(findings) != 0 {
			t.Fatalf("expected lexical oracle call target to fall back to resolver checks, got %d findings", len(findings))
		}
		return
	}
	t.Fatal("UnsafeCast rule not found")
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
			for _, r := range v2rules.Registry {
				if r.ID != "UnsafeCast" {
					continue
				}
				findingCols := rules.NewDispatcherV2([]*v2rules.Rule{r}).Run(file)
				if findingCols.Len() != 0 {
					t.Fatalf("expected no findings for %s source set, got %d", root, findingCols.Len())
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

func TestUnsafeCallOnNullableType_NegativeNonNullIfGuard(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test
fun greet(name: String?) {
    if (name != null) {
        val len = name!!.length
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for structurally guarded local parameter, got %d", len(findings))
	}
}

func TestUnsafeCallOnNullableType_NegativeEarlyReturnNullGuard(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test
fun greet(name: String?) {
    if (name == null) return
    val len = name!!.length
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for early-return null guard, got %d", len(findings))
	}
}

func TestUnsafeCallOnNullableType_NegativeThisQualifiedPropertyGuard(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test
class Greeter(private val name: String?) {
    fun greet() {
        if (this.name != null) {
            val len = name!!.length
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for this-qualified property guard, got %d", len(findings))
	}
}

func TestUnsafeCallOnNullableType_PositivePrefixOnlyGuard(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test
data class User(val name: String?)
fun greet(user: User?) {
    if (user != null) {
        val len = user.name!!.length
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding when only receiver prefix is guarded")
	}
}

func TestUnsafeCallOnNullableType_PositiveFunctionCallGuard(t *testing.T) {
	findings := runRuleByName(t, "UnsafeCallOnNullableType", `
package test
fun nextName(): String? = null
fun greet() {
    if (nextName() != null) {
        val len = nextName()!!.length
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for repeated function call null guard")
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
	findings := runRuleByNameWithResolver(t, "MapGetWithNotNullAssertionOperator", `
package test
fun lookup(map: Map<String, Int>) {
    val value = map["key"]!!
    val value2 = map.get("key")!!
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings for map access with !!, got %d", len(findings))
	}
}

func TestMapGetWithNotNullAssertion_Negative(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "MapGetWithNotNullAssertionOperator", `
package test
class Box { operator fun get(key: String): String? = null }
operator fun Map<*, *>.get(one: Int): Int? = null
fun lookup(map: Map<String, Int>) {
    val value = map.getValue("key")
    if (map.containsKey("other")) {
        val guarded = map["other"]!!
    }
    val extensionGet = map[0]!!
}
fun ok(box: Box) {
    val value = box["x"]!!
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for getValue(), guarded access, or non-map indexing, got %d", len(findings))
	}
}

func TestMapGetWithNotNullAssertion_NestedReceiver(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "MapGetWithNotNullAssertionOperator", `
package test
class Holder(val maps: Maps)
class Maps(val current: Map<String, Int>)
fun lookup(holder: Holder, key: String) {
    val value = holder.maps.current[key]!!
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected finding for nested map receiver, got %d", len(findings))
	}
}

func TestMapGetWithNotNullAssertion_DoesNotMatchUnrelatedNestedTerminalName(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "MapGetWithNotNullAssertionOperator", `
package test
class Maps(val current: Map<String, Int>)
class Other(val current: Box)
class Box { operator fun get(key: String): String? = null }
fun lookup(other: Other, key: String) {
    val value = other.current[key]!!
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no finding for unrelated nested terminal name, got %d", len(findings))
	}
}

// --- CastNullableToNonNullableType ---

func TestCastNullableToNonNullableType_Positive(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "CastNullableToNonNullableType", `
package test
fun convert(obj: String?) {
    val str = obj as String
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for casting nullable to non-nullable, got none")
	}
}

func TestCastNullableToNonNullableType_InferredNullablePositive(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "CastNullableToNonNullableType", `
package test
fun convert(flag: Boolean): String {
    val value = if (flag) "ok" else null
    return value as String
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for inferred nullable value cast to non-nullable, got none")
	}
}

func TestCastNullableToNonNullableType_MultilineGenericPositive(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "CastNullableToNonNullableType", `
package test
fun convert(values: List<String>?) {
    val cast =
        (values) as
            List<String>
    println(cast)
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for multiline nullable generic cast, got none")
	}
	if findings[0].Fix == nil {
		t.Fatal("expected fix for multiline nullable generic cast")
	}
	if findings[0].Fix.Replacement != "as?" {
		t.Fatalf("expected fix to replace only operator with as?, got %q", findings[0].Fix.Replacement)
	}
}

func TestCastNullableToNonNullableType_Negative(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "CastNullableToNonNullableType", `
package test
fun convert(obj: Any?) {
    val str = obj as? String
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for safe cast as?, got %d", len(findings))
	}
}

func TestCastNullableToNonNullableType_NegativeNullableTarget(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "CastNullableToNonNullableType", `
package test
fun convert(obj: String?) {
    val str = obj as String?
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for nullable target cast, got %d", len(findings))
	}
}

func TestCastNullableToNonNullableType_NegativeNullLiteral(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "CastNullableToNonNullableType", `
package test
fun convert() {
    val str = null as String
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for null literal cast, got %d", len(findings))
	}
}

func TestCastNullableToNonNullableType_NegativeUnresolvedSource(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "CastNullableToNonNullableType", `
package test
fun convert() {
    val str = missing as String
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for unresolved source nullability, got %d", len(findings))
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
	findings := runRuleByNameWithResolver(t, "UnnecessaryNotNullCheck", `
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
	findings := runRuleByNameWithResolver(t, "UnnecessaryNotNullCheck", `
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

func TestUnnecessaryNotNullCheck_PositiveResolvedCall(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessaryNotNullCheck", `
package test
fun name(): String = "hello"
fun check() {
    if (name() == null) {
        println("impossible")
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for null check on non-nullable call result, got none")
	}
}

func TestUnnecessaryNotNullCheck_NegativeUnresolvedCall(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessaryNotNullCheck", `
package test
fun check() {
    if (missing() != null) {
        println("unknown")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for unresolved call target, got %d", len(findings))
	}
}

func TestUnnecessaryNotNullCheck_NegativeArbitraryExpression(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessaryNotNullCheck", `
package test
fun check(name: String) {
    if (name + "!" != null) {
        println(name)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for arbitrary non-reference expression, got %d", len(findings))
	}
}

func TestUnnecessaryNotNullCheck_PositiveQualifiedSameFileMember(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessaryNotNullCheck", `
package test
class User {
    val name: String = "Ada"
}
fun check(user: User) {
    if (user.name != null) {
        println(user.name)
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for non-nullable same-file member null check, got none")
	}
}

func TestUnnecessaryNotNullCheck_NegativeNullableSameFileMember(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessaryNotNullCheck", `
package test
class User {
    val name: String? = null
}
fun check(user: User) {
    if (user.name != null) {
        println(user.name)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for nullable same-file member null check, got %d", len(findings))
	}
}

func TestUnnecessaryNotNullCheck_NegativeUnresolvedConstructorCall(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessaryNotNullCheck", `
package test
fun check() {
    if (MissingType() != null) {
        println("unknown")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for unresolved constructor-like call, got %d", len(findings))
	}
}

func TestUnnecessaryNotNullCheck_NegativeQualifiedCallDoesNotMatchTopLevelFunction(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnnecessaryNotNullCheck", `
package test
class Api
fun fetch(): String = "local"
fun check(api: Api) {
    if (api.fetch() != null) {
        println("unknown receiver call")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for qualified call whose receiver target is unresolved, got %d", len(findings))
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
