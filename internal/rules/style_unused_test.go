package rules_test

import (
	"testing"

	v2rules "github.com/kaeawc/krit/internal/rules/v2"
)

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

func TestUnusedImport_IgnoresOtherImports(t *testing.T) {
	findings := runRuleByName(t, "UnusedImport", `
package test
import a.Foo
import b.Foo as Other
fun main() {
    println("hello")
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for import only referenced by another import")
	}
}

func TestUnusedImport_DoesNotReadFollowingKDocAsImportName(t *testing.T) {
	findings := runRuleByName(t, "UnusedImport", `
package test
import kotlin.math.sqrt

/**
 * Uses sqrt below.
 */
fun main() {
    println(sqrt(4.0))
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %#v", len(findings), findings)
	}
}

func TestUnusedImport_IgnoresImplicitOperatorImports(t *testing.T) {
	findings := runRuleByName(t, "UnusedImport", `
package test
import androidx.compose.runtime.getValue
import androidx.compose.runtime.setValue
import kotlinx.coroutines.channels.plusAssign
import example.component1
fun main() {
    println("implicit")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %#v", len(findings), findings)
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
			name: "nested function capture counts",
			code: `
package test
fun ok(id: String) {
    fun nested() {
        println(id)
    }
    nested()
}
`,
			wantHits: 0,
		},
		{
			name: "nested function parameter shadows function parameter",
			code: `
package test
fun bad(id: String) {
    fun nested(id: String) {
        println(id)
    }
    nested("local")
}
`,
			wantHits: 1,
		},
		{
			name: "anonymous object callback capture counts",
			code: `
package test
abstract class Listener {
    abstract fun onSuccess(result: Boolean?)
}
fun ok(timeRemaining: Long) {
    register(object : Listener() {
        override fun onSuccess(result: Boolean?) {
            println(timeRemaining)
        }
    })
}
fun register(listener: Listener) = listener.onSuccess(true)
`,
			wantHits: 0,
		},
		{
			name: "later default argument counts",
			code: `
package test
fun ok(
    oldState: State,
    value: String = oldState.value
) {
    println(value)
}
class State(val value: String)
`,
			wantHits: 0,
		},
		{
			name: "extension function type receiver call counts",
			code: `
package test
class Config
fun configure(init: Config.() -> Unit): Config {
    val configuration = Config()
    configuration.init()
    return configuration
}
`,
			wantHits: 0,
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

func TestUnusedVariable_IgnoresStringsAndComments(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test
fun main() {
    val user = loadUser()
    // user
    println("user")
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding when only strings and comments mention variable")
	}
}

func TestUnusedVariable_DoesNotMatchIdentifierSubstrings(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test
fun main() {
    val id = 1
    val guid = 2
    println(guid)
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected one finding for id only, got %d", len(findings))
	}
}

func TestUnusedVariable_ShadowingDoesNotCountInnerUsage(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test
fun main() {
    val value = 1
    run {
        val value = 2
        println(value)
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for outer shadowed value")
	}
}

func TestUnusedVariable_LambdaParameterShadowingDoesNotCountInnerUsage(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test
fun main() {
    val value = 1
    listOf(1).forEach { value ->
        println(value)
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for outer value shadowed by lambda parameter")
	}
}

func TestUnusedVariable_ShadowInitializerCanReferenceOuterVariable(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test
fun main() {
    val value = 1
    run {
        val value = value + 1
        println(value)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected initializer reference to count for outer value, got %d", len(findings))
	}
}

func TestUnusedVariable_NestedLambdaCanCapture(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test
fun main() {
    val value = 1
    run {
        println(value)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected captured lambda use to count, got %d", len(findings))
	}
}

func TestUnusedVariable_StringInterpolationCountsAsUse(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test
fun main() {
    val message = "Sync OK"
    println("Result: $message")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected string interpolation use to count, got %d", len(findings))
	}
}

func TestUnusedVariable_BracedStringInterpolationCountsAsUse(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test
fun main() {
    val message = "Sync OK"
    println("Result: ${message}")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected braced string interpolation use to count, got %d", len(findings))
	}
}

func TestUnusedVariable_BacktickIdentifierReferences(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test
fun main() {
    val `+"`actual`"+` = 1
    println(actual)
    val `+"`in`"+` = 2
    println(`+"`in`"+`)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected backtick identifier references to count, got %d", len(findings))
	}
}

func TestUnusedVariable_NestedFunctionCanCapture(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test
fun main() {
    val value = 1
    fun nested() {
        println(value)
    }
    nested()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected nested function capture to count, got %d", len(findings))
	}
}

func TestUnusedVariable_NestedFunctionParameterShadowingDoesNotCountInnerUsage(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test
fun main() {
    val value = 1
    fun nested(value: Int) {
        println(value)
    }
    nested(2)
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected one finding for outer value shadowed by nested function parameter, got %d", len(findings))
	}
}

func TestUnusedVariable_ImplicitItShadowingDoesNotCountInnerUsage(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test
fun main() {
    val it = 1
    listOf(1).forEach {
        println(it)
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected one finding for outer it shadowed by implicit lambda parameter, got %d", len(findings))
	}
}

func TestUnusedVariable_ObjectMembersAreNotLocalVariables(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test
fun main() {
    object Holder {
        val value = 1
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected object member property to be ignored, got %d", len(findings))
	}
}

func TestUnusedVariable_ObjectExpressionOverrideAccessorIsNotLocalVariable(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test

abstract class Launcher {
    abstract val contract: String
}

fun main() {
    val holder = object : Launcher() {
        override val contract: String
            get() = "ready"
    }
    println(holder)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected object expression override accessor to be ignored, got %d", len(findings))
	}
}

func TestUnusedVariable_AnnotatedConstructorClassBodyPropertiesAreNotLocalVariables(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test

annotation class RestrictTo(val value: String)

class BluetoothDevice
@RestrictTo("LIBRARY")
internal constructor(internal val fwkDevice: FwkBluetoothDevice) {
    val id: UUID = deviceId(packageName, fwkDevice)

    val name: String?
        get() = fwkDevice.name

    val bondState: Int
        get() = fwkDevice.bondState

    companion object {
        const val PROPERTY_NOTIFY = 16
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected annotated constructor class body properties to be ignored, got %d: %v", len(findings), findings)
	}
}

func TestUnusedVariable_ConstructorNamedLocalCallbackStillFlagsUnusedLocal(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test

fun constructor(block: () -> Unit) = block()

fun main() {
    constructor {
        val unused = 42
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected constructor-named local callback variable to be checked, got %d: %v", len(findings), findings)
	}
}

func TestUnusedVariable_DestructuringEntries(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test
fun main() {
    val (used, unused) = pair()
    println(used)
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected one finding for unused destructured entry, got %d", len(findings))
	}
}

func TestUnusedVariable_AllowedNames(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test
fun main() {
    val ignored = compute()
    val _ = compute()
    println("done")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected allowed names to be ignored, got %d", len(findings))
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

func TestUnusedPrivateProperty_NegativeStringTemplateReferences(t *testing.T) {
	findings := runRuleByName(t, "UnusedPrivateProperty", `
package test

class BadgeSpriteTransformation {
    private val id = "BadgeSpriteTransformation.$VERSION"

    fun key(): String = id

    companion object {
        private const val VERSION = 3
    }
}

object Fonts {
    private const val BASE_STATIC_BUCKET_URI = "https://cdn.example.test/story-fonts"
    private const val MANIFEST = "manifest.json"

    fun manifestPath(version: String): String = "$BASE_STATIC_BUCKET_URI/$version/$MANIFEST"
}

class GroupsV2StateProcessor(private val groupId: String) {
    private val logPrefix = "[$groupId]"

    fun message(): String = "$logPrefix Local state and server state are equal"
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for properties used in string templates, got %d", len(findings))
	}
}

func TestUnusedPrivateProperty_DoesNotRequireTypeCapabilities(t *testing.T) {
	rule := buildRuleIndex()["UnusedPrivateProperty"]
	if rule == nil {
		t.Fatal("UnusedPrivateProperty rule not registered")
	}
	if rule.Needs != 0 {
		t.Fatalf("UnusedPrivateProperty Needs = %b, want no extra capabilities", rule.Needs)
	}
	if rule.Oracle != nil || rule.OracleCallTargets != nil || rule.OracleDeclarationNeeds != nil {
		t.Fatalf("UnusedPrivateProperty declared oracle metadata: Oracle=%+v CallTargets=%+v Declarations=%+v", rule.Oracle, rule.OracleCallTargets, rule.OracleDeclarationNeeds)
	}
	if rule.TypeInfo.PreferBackend != v2rules.PreferAny || rule.TypeInfo.Required {
		t.Fatalf("UnusedPrivateProperty TypeInfo = %+v, want zero-value PreferAny/Required=false", rule.TypeInfo)
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
