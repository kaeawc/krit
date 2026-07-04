package rules_test

import (
	"strings"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
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

func TestUnusedImport_BacktickQuotedShortName(t *testing.T) {
	// '@' stands in for a backtick — raw strings can't contain backticks.
	bt := func(s string) string { return strings.ReplaceAll(s, "@", "`") }
	cases := []struct {
		name string
		code string
	}{
		{
			name: "both sides backtick-quoted",
			code: bt(`
package test
import com.used.Foo.Bar.@baz@
fun main() { println(@baz@()) }
`),
		},
		{
			name: "backtick import, plain reference",
			code: bt(`
package test
import com.used.Foo.Bar.@baz@
fun main() { println(baz()) }
`),
		},
		{
			name: "plain import, backtick reference",
			code: bt(`
package test
import com.used.Foo.Bar.baz
fun main() { println(@baz@()) }
`),
		},
		{
			name: "backtick alias",
			code: bt(`
package test
import com.used.Foo.Bar.baz as @qux@
fun main() { println(qux()) }
`),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := runRuleByName(t, "UnusedImport", tc.code)
			if len(findings) != 0 {
				t.Fatalf("expected no findings, got %d: %#v", len(findings), findings)
			}
		})
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
			name: "for-loop iterable references function parameter",
			code: `
package test
fun process(params: List<String>) {
    for (param in params) {
        println(param)
    }
}
`,
			wantHits: 0,
		},
		{
			name: "for-loop iterable inside parenthesized expression",
			code: `
package test
fun process(params: List<String>) {
    for (param in (params)) {
        println(param)
    }
}
`,
			wantHits: 0,
		},
		{
			name: "for-loop iterable via member call on parameter",
			code: `
package test
fun process(items: List<String>) {
    for (item in items.asReversed()) {
        println(item)
    }
}
`,
			wantHits: 0,
		},
		{
			name: "for-loop with destructuring iterates over parameter",
			code: `
package test
fun process(entries: Map<String, String>) {
    for ((k, v) in entries) {
        println("$k=$v")
    }
}
`,
			wantHits: 0,
		},
		{
			name: "soft-keyword param used as receiver (annotation)",
			code: `
package test
class KSAnnotation { fun process() {} }
fun foo(annotation: KSAnnotation) {
    annotation.process()
}
`,
			wantHits: 0,
		},
		{
			name: "soft-keyword param used as receiver (field)",
			code: `
package test
class Field { fun read() {} }
fun foo(field: Field) {
    field.read()
}
`,
			wantHits: 0,
		},
		{
			name: "for-loop variable named same as parameter is flagged",
			code: `
package test
fun process(item: String) {
    for (item in listOf("a")) {
        println(item)
    }
}
`,
			wantHits: 1,
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
		{
			// Regression: a lambda-typed parameter forwarded inside an anonymous
			// object whose overridden member shares the parameter's name must not
			// be treated as shadowed by that member. The member is a declaration
			// in a nested object body, not a local function.
			name: "anonymous object override with same name as parameter counts",
			code: `
package test
interface AnimationListener {
    fun onAnimationStart()
    fun onAnimationEnd()
}
fun makeListener(
    onAnimationStart: () -> Unit,
    onAnimationEnd: () -> Unit
): AnimationListener {
    return object : AnimationListener {
        override fun onAnimationStart() {
            onAnimationStart()
        }
        override fun onAnimationEnd() {
            onAnimationEnd()
        }
    }
}
`,
			wantHits: 0,
		},
		{
			// Regression: a parameter referenced only inside a trailing lambda
			// body must count as used.
			name: "trailing lambda body references parameter",
			code: `
package test
fun timer(group: String, durationsByGroup: MutableMap<String, MutableList<Long>>) {
    durationsByGroup.getOrPut(group) { mutableListOf(group.length.toLong()) }
}
`,
			wantHits: 0,
		},
		{
			// Guard: a genuine *local* function whose name shadows the parameter
			// must still mask the parameter (so the parameter is flagged unused).
			name: "local function shadowing parameter name still flags parameter",
			code: `
package test
fun localShadow(handler: () -> Unit) {
    fun handler() {}
    handler()
}
`,
			wantHits: 1,
		},
		{
			// Guard: an inner lambda parameter that re-declares and uses the name
			// must not mask a genuinely unused outer parameter.
			name: "inner lambda parameter shadow does not mask unused outer parameter",
			code: `
package test
fun outer(x: Int) {
    listOf(1, 2).forEach { x -> println(x) }
}
`,
			wantHits: 1,
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

func TestUnusedVariable_ClassMembersAreNotLocalVariables(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test

class Extension(objects: ObjectFactory) {
    public val enabled: Property<Boolean> = objects.property(Boolean::class.java)
    val apiVersion: String = "1.0"
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected class member properties to be ignored, got %d", len(findings))
	}
}

func TestUnusedVariable_CompanionConstantsAreNotLocalVariables(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test

class BluetoothDevice {
    val address: String = "00:11:22:33:44:55"

    companion object {
        const val PROPERTY_NOTIFY = 16
        val SERVICE_UUID: String = "0000180f-0000-1000-8000-00805f9b34fb"
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected companion object constants and class properties to be ignored, got %d: %v", len(findings), findings)
	}
}

func TestUnusedVariable_ClassMethodLocalStillFlagsUnusedLocal(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test

class Worker {
    fun run() {
        val unused = 42
        println("ready")
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected one finding for unused local in class method, got %d: %v", len(findings), findings)
	}
}

func TestUnusedVariable_ClassInitializerLocalStillFlagsUnusedLocal(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test

class Worker {
    init {
        val unused = 42
        println("ready")
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected one finding for unused local in class initializer, got %d: %v", len(findings), findings)
	}
}

func TestUnusedVariable_ClassPropertyInitializerLambdaLocalStillFlagsUnusedLocal(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test

class Holder {
    val value = run {
        val unused = 42
        "ready"
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected one finding for unused local in property initializer lambda, got %d: %v", len(findings), findings)
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

func TestUnusedVariable_ForLoopDestructuring(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantMsgs []string
	}{
		{
			name: "both used",
			body: "println(a); println(b)",
		},
		{
			name:     "none used",
			body:     `println("body")`,
			wantMsgs: []string{"Local variable 'a' is never used.", "Local variable 'b' is never used."},
		},
		{
			name:     "only first used",
			body:     "println(a)",
			wantMsgs: []string{"Local variable 'b' is never used."},
		},
		{
			name: "string interpolation counts as use",
			body: `println("a=$a b=${b}")`,
		},
		{
			name:     "shadowed by inner val before any use",
			body:     "val a = 99; println(a); println(b)",
			wantMsgs: []string{"Local variable 'a' is never used."},
		},
		{
			name: "use before shadow",
			body: "println(a); val a = 99; println(a); println(b)",
		},
		{
			name:     "nested function parameter shadows",
			body:     "fun nested(a: Int) { println(a) }; nested(0); println(b)",
			wantMsgs: []string{"Local variable 'a' is never used."},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code := "package test\nfun main() {\n    val pairs = listOf(1 to 2)\n    for ((a, b) in pairs) {\n        " + c.body + "\n    }\n}\n"
			findings := runRuleByName(t, "UnusedVariable", code)
			if len(findings) != len(c.wantMsgs) {
				t.Fatalf("expected %d findings, got %d: %v", len(c.wantMsgs), len(findings), findings)
			}
			for i, want := range c.wantMsgs {
				if findings[i].Message != want {
					t.Fatalf("finding %d: expected %q, got %q", i, want, findings[i].Message)
				}
			}
		})
	}
}

func TestUnusedVariable_ForLoopDestructuringReferenceOutsideBodyDoesNotCount(t *testing.T) {
	// Destructured bindings are out of scope outside the loop body. The two
	// findings are the destructured 'a' and 'b'; the trailing `val a`/`val b`
	// are used.
	findings := runRuleByName(t, "UnusedVariable", `
package test
fun main() {
    val pairs = listOf(1 to 2)
    for ((a, b) in pairs) {
        println("body")
    }
    val a = 1
    val b = 2
    println(a)
    println(b)
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected two findings for destructured bindings (refs outside loop don't count), got %d: %v", len(findings), findings)
	}
}

func TestUnusedVariable_ForLoopDestructuringUnderscorePlaceholderAllowed(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test
fun main() {
    val pairs = listOf(1 to 2)
    for ((_, b) in pairs) {
        println(b)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected underscore placeholder to be ignored, got %d: %v", len(findings), findings)
	}
}

func TestUnusedVariable_LambdaDestructuringNotFlagged(t *testing.T) {
	// Lambda destructuring `{ (a, b) -> ... }` should not be touched by the
	// UnusedVariable rule — those bindings are lambda parameters handled by
	// UnusedParameter. The bug fix for for-loop destructuring must not extend
	// to lambdas.
	findings := runRuleByName(t, "UnusedVariable", `
package test
fun main() {
    val pairs = listOf(1 to 2)
    pairs.forEach { (a, b) ->
        println("ignore both")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected lambda destructuring bindings to be ignored by UnusedVariable, got %d: %v", len(findings), findings)
	}
}

func TestUnusedVariable_ClassWithSeparateLinePrivateConstructorBodyIsNotLocal(t *testing.T) {
	// tree-sitter mis-parses `class Foo\nprivate constructor(...) ... { ... }`
	// as a class header followed by an expression containing a lambda; class
	// members end up under a lambda_literal that is not actually a lambda
	// expression. They must not be flagged as "local variable".
	findings := runRuleByName(t, "UnusedVariable", `
package test
internal class Parameter
private constructor(
  val kind: Int,
  val name: String,
) : Comparable<Parameter> {
  val typeKey: Int = kind
  val type: Int = kind
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected zero findings (class body mis-parsed as lambda), got %d: %v", len(findings), findings)
	}
}

func TestUnusedVariable_ScopeWithParseErrorsAbstains(t *testing.T) {
	// tree-sitter Kotlin does not support `when` entries with `is X if (cond)`
	// guards yet. References that land in ERROR/orphan subtrees would
	// otherwise be invisible, producing false positives. The rule must
	// abstain when its lookup scope contains parse errors.
	findings := runRuleByName(t, "UnusedVariable", `
package test
fun fn(x: Any): Boolean {
    val isGraph = true
    val isContributedGraph = true
    when (x) {
        is String -> println("s")
        is IntArray if (isGraph && x.size > 0) -> println("a")
        else -> {}
    }
    return isContributedGraph
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected zero findings when scope has parse errors, got %d: %v", len(findings), findings)
	}
}

func TestUnusedVariable_UnaryPlusContinuationAfterWhenBlockCountsAsUse(t *testing.T) {
	// A user-intended unary-plus statement after a multi-line expression is
	// bound by tree-sitter (and Kotlin's grammar) into an additive_expression
	// inside the previous initializer. The rule must recognize that pattern
	// and treat the trailing identifier as a use.
	findings := runRuleByName(t, "UnusedVariable", `
package test
operator fun String.unaryPlus(): Unit = Unit
fun foo(x: Int) {
    val factoryCall = when (x) {
        1 -> "a"
        else -> "b"
    }
    +factoryCall
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected unary-plus continuation to be treated as a use, got %d: %v", len(findings), findings)
	}
}

func TestUnusedVariable_UnaryPlusContinuationOnSameLineDoesNotAffectOtherCases(t *testing.T) {
	// A genuine self-reference on the same line (no leading newline before
	// the `+`) should still be treated as unused — only newline-prefixed
	// unary continuations should be rescued.
	findings := runRuleByName(t, "UnusedVariable", `
package test
fun foo(outer: Int) {
    val self = outer + outer
    println("done")
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected `self` to remain flagged as unused, got %d: %v", len(findings), findings)
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

func TestUnusedPrivateFunction_NegativeNegatedCallBeforeDeclaration(t *testing.T) {
	findings := runRuleByName(t, "UnusedPrivateFunction", `
package test
fun main(): Boolean {
    return !isDisabled()
}

private fun isDisabled() = false
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for private function used by negated call, got %d", len(findings))
	}
}

func TestUnusedPrivateFunction_PositiveCommentOnlyReference(t *testing.T) {
	findings := runRuleByName(t, "UnusedPrivateFunction", `
package test
fun main() {
    // helper()
    val text = "helper()"
}

private fun helper() = 42
`)
	if len(findings) == 0 {
		t.Fatal("expected finding when private function appears only in comments and strings")
	}
}

func TestUnusedPrivateFunction_NegativePreviewWrapperAnnotation(t *testing.T) {
	findings := runRuleByName(t, "UnusedPrivateFunction", `
package test

annotation class ConstellationPreviews
annotation class Composable

@ConstellationPreviews
@Composable
private fun HeaderPreview() {}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for preview wrapper annotation, got %d", len(findings))
	}
}

func TestUnusedPrivateFunction_NegativePreviewAnnotationWithArgumentsAndSuppress(t *testing.T) {
	findings := runRuleByName(t, "UnusedPrivateFunction", `
package test

annotation class Preview(val device: String = "")
annotation class Composable

@Preview(device = "spec:width=211dp,height=891dp")
@Composable
@Suppress("UnusedPrivateMember")
private fun HeaderPreview() {}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for argument-bearing preview annotation, got %d", len(findings))
	}
}

func TestUnusedPrivateFunction_PositivePreviewNameWithoutAnnotation(t *testing.T) {
	findings := runRuleByName(t, "UnusedPrivateFunction", `
package test

private fun HeaderPreview() {}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for preview-named function without preview annotation")
	}
}

func TestUnusedPrivateFunction_NegativeUnusedPrivateMemberSuppressAlias(t *testing.T) {
	findings := runRuleByName(t, "UnusedPrivateFunction", `
package test

@Suppress("UnusedPrivateMember")
private fun helper() {}
`)
	if len(findings) != 0 {
		t.Fatalf("expected UnusedPrivateMember suppression to cover UnusedPrivateFunction, got %d", len(findings))
	}
}

func TestUnusedPrivateFunction_NegativePrivateTimesOperator(t *testing.T) {
	findings := runRuleByName(t, "UnusedPrivateFunction", `
package test

data class Size(val width: Int, val height: Int)

fun scale(size: Size): Size {
    return size * 2
}

private operator fun Size.times(multiplier: Int): Size {
    return Size(width * multiplier, height * multiplier)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for private times operator used by symbol, got %d", len(findings))
	}
}

func TestUnusedPrivateFunction_PositiveUnusedPrivateTimesOperator(t *testing.T) {
	findings := runRuleByName(t, "UnusedPrivateFunction", `
package test

data class Size(val width: Int, val height: Int)

fun scale(size: Size): Size {
    return size
}

private operator fun Size.times(multiplier: Int): Size {
    return Size(width * multiplier, height * multiplier)
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for unused private times operator")
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
	if rule.TypeInfo.PreferBackend != api.PreferAny || rule.TypeInfo.Required {
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
