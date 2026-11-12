package rules_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// --- style_idiomatic.go rules ---

func TestUseCheckNotNull_Positive(t *testing.T) {
	findings := runRuleByName(t, "UseCheckNotNull", `
package test
fun foo(x: String?) {
    check(x != null)
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for check(x != null)")
	}
}

func TestUseCheckNotNull_Negative(t *testing.T) {
	findings := runRuleByName(t, "UseCheckNotNull", `
package test
fun foo(x: String?) {
    checkNotNull(x)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestUseRequireNotNull_Positive(t *testing.T) {
	findings := runRuleByName(t, "UseRequireNotNull", `
package test
fun foo(x: String?) {
    require(x != null)
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for require(x != null)")
	}
}

func TestUseRequireNotNull_Negative(t *testing.T) {
	findings := runRuleByName(t, "UseRequireNotNull", `
package test
fun foo(x: String?) {
    requireNotNull(x)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestUseRequireNotNull_DoesNotFlagComplexRequireConditions(t *testing.T) {
	findings := runRuleByName(t, "UseRequireNotNull", `
package test
fun foo(x: String?, y: String?) {
    require(x != null && x.isNotBlank())
    require(x != null || y != null) { "one value is required" }
    require(null != x && y != null)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for complex require conditions, got %d", len(findings))
	}
}

func TestUseRequireNotNull_FlagsParenthesizedDirectNullCheck(t *testing.T) {
	findings := runRuleByName(t, "UseRequireNotNull", `
package test
fun foo(x: String?) {
    require((x != null))
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for parenthesized require(x != null)")
	}
}

func TestUseCheckOrError_Positive(t *testing.T) {
	findings := runRuleByName(t, "UseCheckOrError", `
package test
fun foo(valid: Boolean) {
    if (!valid) throw IllegalStateException("invalid")
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for if (!x) throw IllegalStateException")
	}
}

func TestUseCheckOrError_Negative(t *testing.T) {
	findings := runRuleByName(t, "UseCheckOrError", `
package test
fun foo(valid: Boolean) {
    check(valid) { "invalid" }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// detekt takes the first call in source order, so a chain on the exception constructor still counts.
func TestUseCheckOrError_ReportsConstructorWithChainedCall(t *testing.T) {
	findings := runRuleByName(t, "UseCheckOrError", `
package test
fun foo(x: Int) {
    if (x > 1) throw IllegalStateException("a").also { println(it) }
    if (x > 2) throw IllegalStateException("b")?.also { println(it) }
    if (x > 3) throw IllegalStateException("c").let { it }
    if (x > 4) throw wrap(IllegalStateException("d"))
    if (x > 5) throw make().also { }
}
`)
	if len(findings) != 3 || findings[0].Line != 4 || findings[1].Line != 5 || findings[2].Line != 6 {
		t.Fatalf("expected findings on lines 4, 5 and 6, got %v", findings)
	}
}

func TestUseRequire_ReportsConstructorWithChainedCall(t *testing.T) {
	findings := runRuleByName(t, "UseRequire", `
package test
fun foo(x: Int) {
    if (x > 1) throw IllegalArgumentException("a").also { println(it) }
    if (x > 2) throw wrap(IllegalArgumentException("b"))
}
`)
	if len(findings) != 1 || findings[0].Line != 4 {
		t.Fatalf("expected one finding on line 4, got %v", findings)
	}
}

func TestUseRequire_Positive(t *testing.T) {
	findings := runRuleByName(t, "UseRequire", `
package test
fun foo(x: Int) {
    if (!x.isValid()) throw IllegalArgumentException("bad arg")
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for if (!x) throw IllegalArgumentException")
	}
}

func TestUseRequire_Negative(t *testing.T) {
	findings := runRuleByName(t, "UseRequire", `
package test
fun foo(x: Int) {
    require(x > 0) { "bad arg" }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestUseIsNullOrEmpty_Positive(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UseIsNullOrEmpty", `
package test
fun foo(x: String?) {
    if (x == null || x.isEmpty()) println("empty")
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for x == null || x.isEmpty()")
	}
}

func TestUseIsNullOrEmpty_PositiveSizeCheck(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UseIsNullOrEmpty", `
package test
fun foo(value: Collection<String>?) {
    if (value == null || value.size == 0) println("empty")
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for value == null || value.size == 0")
	}
}

func TestUseIsNullOrEmpty_PositiveStructuralVariants(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UseIsNullOrEmpty", `
package test
class Holder(val text: String?) {
    fun stringEmpty() {
        if ((text) == null || (text) == "") println("empty")
    }
    fun listCount(items: List<String>?) {
        if (items == null ||
            items.count() == 0
        ) println("empty")
    }
    fun stringLength(name: String?) {
        if (null == name || name.length == 0) println("empty")
    }
    fun thisProperty() {
        if (this.text == null || this.text.isEmpty()) println("empty")
    }
}
`)
	if len(findings) != 4 {
		t.Fatalf("expected 4 findings for structural variants, got %d", len(findings))
	}
}

func TestUseIsNullOrEmpty_Negative(t *testing.T) {
	findings := runRuleByName(t, "UseIsNullOrEmpty", `
package test
fun foo(x: String?) {
    if (x.isNullOrEmpty()) println("empty")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestUseIsNullOrEmpty_NegativeSemanticGuards(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UseIsNullOrEmpty", `
package test
class Box { fun isEmpty() = true }
class Holder(val text: String?) {
    fun differentVariables(a: String?, b: String?) {
        if (a == null || b.isEmpty()) println("empty")
    }
    fun customType(box: Box?) {
        if (box == null || box.isEmpty()) println("empty")
    }
    fun shadowed(text: String?) {
        if (this.text == null || text.isEmpty()) println("empty")
    }
    fun unresolved(value: MissingType?) {
        if (value == null || value.isEmpty()) println("empty")
    }
    fun primitiveArray(values: IntArray?) {
        if (values == null || values.isEmpty()) println("empty")
        if (values == null || values.size == 0) println("empty")
    }
    fun sequence(values: Sequence<String>?) {
        if (values == null || values.count() == 0) println("empty")
    }
    fun commentsAndStrings(x: String?) {
        // x == null || x.isEmpty()
        val s = "x == null || x.isEmpty()"
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestUseOrEmpty_Positive(t *testing.T) {
	findings := runRuleByName(t, "UseOrEmpty", `
package test
fun foo(x: List<String>?) {
    val result = x ?: emptyList()
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for x ?: emptyList()")
	}
}

func TestUseOrEmpty_Negative(t *testing.T) {
	findings := runRuleByName(t, "UseOrEmpty", `
package test
fun foo(x: List<String>?) {
    val result = x.orEmpty()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestUseOrEmpty_NegativeStringTemplateElvis(t *testing.T) {
	findings := runRuleByName(t, "UseOrEmpty", `
package test
fun foo(value: Throwable?) {
    val result = "error ${value ?: ""}"
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestUseAnyOrNoneInsteadOfFind_Positive(t *testing.T) {
	findings := runRuleByName(t, "UseAnyOrNoneInsteadOfFind", `
package test
fun foo(list: List<Int>) {
    val found = list.find { it > 0 } != null
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for .find {} != null")
	}
}

func TestUseAnyOrNoneInsteadOfFind_Negative(t *testing.T) {
	findings := runRuleByName(t, "UseAnyOrNoneInsteadOfFind", `
package test
fun foo(list: List<Int>) {
    val found = list.any { it > 0 }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestUseEmptyCounterpart_Positive(t *testing.T) {
	findings := runRuleByName(t, "UseEmptyCounterpart", `
package test
fun foo() {
    val x = listOf()
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for listOf() with no args")
	}
}

func TestUseEmptyCounterpart_Negative(t *testing.T) {
	findings := runRuleByName(t, "UseEmptyCounterpart", `
package test
fun foo() {
    val x = emptyList()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- style_idiomatic_data.go rules ---

func TestUseArrayLiteralsInAnnotations_Positive(t *testing.T) {
	findings := runRuleByName(t, "UseArrayLiteralsInAnnotations", `
package test
@Target(arrayOf(AnnotationTarget.CLASS))
annotation class MyAnnotation
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for arrayOf() in annotation")
	}
}

func TestUseArrayLiteralsInAnnotations_Negative(t *testing.T) {
	findings := runRuleByName(t, "UseArrayLiteralsInAnnotations", `
package test
@Target([AnnotationTarget.CLASS])
annotation class MyAnnotation
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestUseSumOfInsteadOfFlatMapSize_Positive(t *testing.T) {
	findings := runRuleByName(t, "UseSumOfInsteadOfFlatMapSize", `
package test
fun foo(lists: List<List<Int>>) {
    val total = lists.flatMap { it }.size
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for flatMap{}.size")
	}
}

func TestUseSumOfInsteadOfFlatMapSize_Negative(t *testing.T) {
	findings := runRuleByName(t, "UseSumOfInsteadOfFlatMapSize", `
package test
fun foo(lists: List<List<Int>>) {
    val total = lists.sumOf { it.size }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestUseLet_Positive(t *testing.T) {
	findings := runRuleByName(t, "UseLet", `
package test
fun foo(x: String?): Int? {
    return if (x != null) x.length else null
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected finding for a null check with a null else, got %d", len(findings))
	}
	want := "Use `?.let {}` instead of if/else with a null block when checking for nullable values"
	if findings[0].Message != want {
		t.Errorf("message = %q, want %q", findings[0].Message, want)
	}
}

func TestUseLet_Negative(t *testing.T) {
	findings := runRuleByName(t, "UseLet", `
package test
fun foo(x: String?) {
    x?.let { println(it) }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestUseDataClass_Positive(t *testing.T) {
	findings := runRuleByName(t, "UseDataClass", `
package test
class Person(val name: String, val age: Int)
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for class that could be data class")
	}
}

func TestUseDataClass_Negative(t *testing.T) {
	findings := runRuleByName(t, "UseDataClass", `
package test
data class Person(val name: String, val age: Int)
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestUseDataClass_FollowsDetektConditions(t *testing.T) {
	findings := runRuleByName(t, "UseDataClass", `package test
interface Marker
class Plain(val x: Int)
class WithMarker(val x: Int) : Marker
class WithBase(val x: Int) : Base()
class WithBodyProperty(val x: Int) {
    val y = 1
}
class WithFunction(val x: Int) {
    fun doIt() {}
}
class WithToString(val x: Int) {
    override fun toString() = "x"
}
class NoConstructor {
    val x = 1
}
class PlainParameter(val x: Int, y: Int)
enum class E(val x: Int) { A(1) }
`)
	got := map[int]string{}
	for _, f := range findings {
		got[f.Line] = f.Message
	}
	for _, line := range []int{3, 4, 6, 12} {
		if got[line] == "" {
			t.Errorf("expected a finding on line %d, got %v", line, got)
		}
	}
	if len(got) != 4 {
		t.Errorf("expected exactly 4 findings, got %v", got)
	}
	want := "The class Plain defines no functionality and only holds data. Consider converting it to a data class."
	if got[3] != want {
		t.Errorf("message = %q, want %q", got[3], want)
	}
}

func TestUseDataClass_HonorsAllowVars(t *testing.T) {
	// AllowVars was previously a dead config — exposed in metadata but
	// never consulted. The check fired on classes whose primary
	// constructor used `var` parameters. Default behavior (false) now skips
	// those classes.
	var rule *rules.UseDataClassRule
	for _, candidate := range api.Registry {
		if candidate.ID == "UseDataClass" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.UseDataClassRule)
			if !ok {
				t.Fatalf("expected UseDataClassRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("UseDataClass rule not registered")
	}
	original := rule.AllowVars
	defer func() { rule.AllowVars = original }()

	codeWithVar := `package test
class Person(var name: String, var age: Int)
`
	// Default (false): like detekt, a class with var properties is still a data-class candidate.
	if findings := runRuleByName(t, "UseDataClass", codeWithVar); len(findings) != 1 {
		t.Fatalf("expected a finding under default AllowVars=false, got %d", len(findings))
	}

	// Flipping to true relaxes the rule for classes that have a var.
	rule.AllowVars = true
	if findings := runRuleByName(t, "UseDataClass", codeWithVar); len(findings) != 0 {
		t.Fatal("expected no finding under AllowVars=true for class with var properties")
	}

	// All-val classes still fire under either setting (existing behavior).
	codeAllVal := `package test
class Person(val name: String, val age: Int)
`
	rule.AllowVars = false
	if findings := runRuleByName(t, "UseDataClass", codeAllVal); len(findings) == 0 {
		t.Fatal("expected finding for all-val candidate class regardless of AllowVars")
	}
}

func TestUseIfInsteadOfWhen_Positive(t *testing.T) {
	findings := runRuleByName(t, "UseIfInsteadOfWhen", `
package test
fun foo(x: Boolean) {
    when {
        x -> println("yes")
        else -> println("no")
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for when with two branches")
	}
}

func TestUseIfInsteadOfWhen_FixCondElse(t *testing.T) {
	findings := runRuleByName(t, "UseIfInsteadOfWhen", `
package test
fun foo(c: Boolean): Int = when {
    c -> 1
    else -> 2
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Fix == nil {
		t.Fatal("expected fix to be populated")
	}
	if !strings.Contains(findings[0].Fix.Replacement, "if (c) 1 else 2") {
		t.Errorf("got %q", findings[0].Fix.Replacement)
	}
}

func TestUseIfInsteadOfWhen_RequiresTwoEntriesWithAnElse(t *testing.T) {
	// detekt reports only a binary when: one condition plus an else.
	findings := runRuleByName(t, "UseIfInsteadOfWhen", `
package test
fun foo(c: Boolean, d: Boolean) {
    when {
        c -> 1
        d -> 2
    }
    when { c -> println("yes") }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings without an else, got %v", findings)
	}
}

func TestUseIfInsteadOfWhen_NoFixForSubjectForm(t *testing.T) {
	// Subject-form when (`when (x) { 1 -> ... }`) needs synthetic
	// `==`/`is`/`in` operators to preserve semantics — the fix should
	// bail and let the author choose.
	findings := runRuleByName(t, "UseIfInsteadOfWhen", `
package test
fun foo(x: Int): Int = when (x) {
    1 -> 10
    else -> 20
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Fix != nil {
		t.Errorf("expected no fix for subject form, got %q", findings[0].Fix.Replacement)
	}
}

func TestUseIfInsteadOfWhen_IgnoresMultiConditionEntries(t *testing.T) {
	findings := runRuleByName(t, "UseIfInsteadOfWhen", `
package test
fun foo(c: Boolean, d: Boolean) = when {
    c, d -> 1
    else -> 2
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for a comma-separated condition, got %v", findings)
	}
}

func TestUseIfInsteadOfWhen_Negative(t *testing.T) {
	findings := runRuleByName(t, "UseIfInsteadOfWhen", `
package test
fun foo(x: Int) {
    when (x) {
        1 -> println("one")
        2 -> println("two")
        3 -> println("three")
        else -> println("other")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestUseIfInsteadOfWhen_HonorsIgnoreWhenContainingVariableDeclaration(t *testing.T) {
	// With the option set, a `when` whose subject declares a variable is skipped, like detekt.
	var rule *rules.UseIfInsteadOfWhenRule
	for _, candidate := range api.Registry {
		if candidate.ID == "UseIfInsteadOfWhen" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.UseIfInsteadOfWhenRule)
			if !ok {
				t.Fatalf("expected UseIfInsteadOfWhenRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("UseIfInsteadOfWhen rule not registered")
	}
	original := rule.IgnoreWhenContainingVariableDeclaration
	defer func() { rule.IgnoreWhenContainingVariableDeclaration = original }()

	codeWithDecl := `package test
fun foo(x: Int) {
    when (val y = x) {
        1 -> println(y)
        else -> println("no")
    }
}
`
	if findings := runRuleByName(t, "UseIfInsteadOfWhen", codeWithDecl); len(findings) == 0 {
		t.Fatal("expected finding under default IgnoreWhenContainingVariableDeclaration=false")
	}

	rule.IgnoreWhenContainingVariableDeclaration = true

	if findings := runRuleByName(t, "UseIfInsteadOfWhen", codeWithDecl); len(findings) != 0 {
		t.Fatalf("expected no findings under IgnoreWhenContainingVariableDeclaration=true with a subject variable, got %d", len(findings))
	}

	codeNoDecl := `package test
fun foo(x: Boolean) {
    when {
        x -> println("yes")
        else -> println("no")
    }
}
`
	if findings := runRuleByName(t, "UseIfInsteadOfWhen", codeNoDecl); len(findings) == 0 {
		t.Fatal("expected finding for when without declarations even with flag=true")
	}
}

func TestUseIfEmptyOrIfBlank_Positive(t *testing.T) {
	findings := runRuleByName(t, "UseIfEmptyOrIfBlank", `
package test
fun foo(s: String): String {
    return if (s.isEmpty()) { "default" } else { s }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for manual isEmpty check")
	}
}

func TestUseIfEmptyOrIfBlank_Negative(t *testing.T) {
	findings := runRuleByName(t, "UseIfEmptyOrIfBlank", `
package test
fun foo(s: String): String {
    return s.ifEmpty { "default" }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestExplicitCollectionElementAccessMethod_Positive(t *testing.T) {
	findings := runRuleByName(t, "ExplicitCollectionElementAccessMethod", `
package test
fun foo(map: Map<String, Int>) {
    val v = map.get("key")
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for explicit .get() call")
	}
}

func TestExplicitCollectionElementAccessMethod_SetPositive(t *testing.T) {
	findings := runRuleByName(t, "ExplicitCollectionElementAccessMethod", `
package test
fun foo(list: MutableList<String>) {
    list.set(0, "value")
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for explicit .set() call on mutable list")
	}
}

func TestExplicitCollectionElementAccessMethod_InferredMapPositive(t *testing.T) {
	findings := runRuleByName(t, "ExplicitCollectionElementAccessMethod", `
package test
fun foo() {
    val map = mapOf("key" to 1)
    val v = map.get("key")
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for inferred map get()")
	}
}

func TestExplicitCollectionElementAccessMethod_Negative(t *testing.T) {
	findings := runRuleByName(t, "ExplicitCollectionElementAccessMethod", `
package test
fun foo(map: Map<String, Int>) {
    val v = map["key"]
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestExplicitCollectionElementAccessMethod_DoesNotFlagNonCollectionGet(t *testing.T) {
	findings := runRuleByName(t, "ExplicitCollectionElementAccessMethod", `
package test
class Cache {
    fun get(key: String): Int = 0
}

fun foo(cache: Cache) {
    val v = cache.get("key")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for non-collection get(), got %d", len(findings))
	}
}

func TestExplicitCollectionElementAccessMethod_DoesNotUseNestedCollectionInitializer(t *testing.T) {
	findings := runRuleByName(t, "ExplicitCollectionElementAccessMethod", `
package test
class Cache(private val keys: List<String>) {
    fun get(key: String): Int = 0
}

fun foo() {
    val cache = Cache(listOf("key"))
    val v = cache.get("key")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for non-collection initialized with nested list, got %d", len(findings))
	}
}

func TestExplicitCollectionElementAccessMethod_DoesNotMatchStringSuffix(t *testing.T) {
	findings := runRuleByName(t, "ExplicitCollectionElementAccessMethod", `
package test
fun foo(map: Map<String, Int>) {
    val v = map.target("other.get")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for string suffix mention, got %d", len(findings))
	}
}

func TestExplicitCollectionElementAccessMethod_ImportBackedImmutableList(t *testing.T) {
	findings := runRuleByName(t, "ExplicitCollectionElementAccessMethod", `
package test
import kotlinx.collections.immutable.ImmutableList

fun foo(items: ImmutableList<String>) {
    val first = items.get(0)
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for imported immutable list get()")
	}
}

func TestExplicitCollectionElementAccessMethod_DoesNotSetImportedImmutableList(t *testing.T) {
	findings := runRuleByName(t, "ExplicitCollectionElementAccessMethod", `
package test
import kotlinx.collections.immutable.ImmutableList

fun foo(items: ImmutableList<String>) {
    items.set(0, "value")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for set() on imported immutable list, got %d", len(findings))
	}
}

func TestAlsoCouldBeApply_Positive(t *testing.T) {
	findings := runRuleByName(t, "AlsoCouldBeApply", `
package test
fun foo() {
    val x = StringBuilder().also {
        it.append("hello")
        it.append(" world")
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for also with multiple it. references")
	}
}

func TestAlsoCouldBeApply_SingleStatementUsesDetektMessage(t *testing.T) {
	findings := runRuleByName(t, "AlsoCouldBeApply", `
package test
fun foo(b: StringBuilder) {
    b.also { it.append("a") }
    b.also({ it.append("b") })
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected both single-statement also calls, got %d", len(findings))
	}
	want := "When an `also` block contains only `it`-started expressions, simplify it to the `apply` block."
	if findings[0].Message != want {
		t.Errorf("message = %q, want %q", findings[0].Message, want)
	}
}

func TestAlsoCouldBeApply_PositiveAssignments(t *testing.T) {
	findings := runRuleByName(t, "AlsoCouldBeApply", `
package test
fun foo(person: Person) {
    person.also {
        it.name = "Bob"
        it.age = 25
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for also with multiple it receiver assignments")
	}
}

func TestAlsoCouldBeApply_FixSimpleCalls(t *testing.T) {
	findings := runRuleByName(t, "AlsoCouldBeApply", `
package test
fun foo(b: StringBuilder) {
    b.also {
        it.append("a")
        it.append("b")
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Fix == nil {
		t.Fatal("expected fix to be populated")
	}
	repl := findings[0].Fix.Replacement
	if !strings.Contains(repl, ".apply {") {
		t.Errorf("expected .apply rewrite, got %q", repl)
	}
	if strings.Contains(repl, "it.append") {
		t.Errorf("expected it. prefix to be stripped, got %q", repl)
	}
	if !strings.Contains(repl, "append(\"a\")") || !strings.Contains(repl, "append(\"b\")") {
		t.Errorf("expected calls preserved, got %q", repl)
	}
}

func TestAlsoCouldBeApply_FixAssignmentsAndCall(t *testing.T) {
	findings := runRuleByName(t, "AlsoCouldBeApply", `
package test
class Cfg { var name = ""; var size = 0; fun init() {} }
fun build(): Cfg = Cfg().also {
    it.name = "x"
    it.size = 1
    it.init()
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Fix == nil {
		t.Fatal("expected fix to be populated")
	}
	repl := findings[0].Fix.Replacement
	if strings.Contains(repl, "it.") {
		t.Errorf("expected all it. prefixes stripped, got %q", repl)
	}
	if !strings.Contains(repl, ".apply {") {
		t.Errorf("expected apply rewrite, got %q", repl)
	}
}

func TestAlsoCouldBeApply_FixSkipsWhenItUsedInArg(t *testing.T) {
	// `it` appears in an argument position — the fix must bail because
	// switching to `apply` would leave the inner `it` unbound.
	findings := runRuleByName(t, "AlsoCouldBeApply", `
package test
fun foo(b: StringBuilder, log: (Any) -> Unit) {
    b.also {
        it.append("a")
        it.append(it)
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Fix != nil {
		t.Errorf("expected fix to be skipped when it appears in arg position, got %q", findings[0].Fix.Replacement)
	}
}

func TestAlsoCouldBeApply_Negative(t *testing.T) {
	findings := runRuleByName(t, "AlsoCouldBeApply", `
package test
fun foo() {
    val x = StringBuilder().apply {
        append("hello")
        append(" world")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestAlsoCouldBeApply_NegativeMixedLambdaBody(t *testing.T) {
	findings := runRuleByName(t, "AlsoCouldBeApply", `
package test
fun foo(builder: StringBuilder) {
    builder.also {
        it.append("hello")
        log(it.length)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for mixed also body, got %d", len(findings))
	}
}

func TestAlsoCouldBeApply_NegativeTextLookalikes(t *testing.T) {
	findings := runRuleByName(t, "AlsoCouldBeApply", `
package test
fun foo() {
    // StringBuilder().also { it.append("hello"); it.append("world") }
    val code = "StringBuilder().also { it.append(\"hello\"); it.append(\"world\") }"
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for text lookalikes, got %d", len(findings))
	}
}

func TestEqualsNullCall_Positive(t *testing.T) {
	findings := runRuleByName(t, "EqualsNullCall", `
package test
fun foo(x: String?) {
    if (x.equals(null)) println("null")
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for .equals(null)")
	}
}

func TestEqualsNullCall_IgnoresStaticTwoArgHelpers(t *testing.T) {
	findings := runRuleByName(t, "EqualsNullCall", `
package test
fun foo(a: String?, b: String?) {
    println(Objects.equals(a, null))
    println(Utils.equals(a, b))
    println(a.equals(b))
    println(a.equals("null"))
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestEqualsNullCall_Negative(t *testing.T) {
	findings := runRuleByName(t, "EqualsNullCall", `
package test
fun foo(x: String?) {
    if (x == null) println("null")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestUseRequire_FlagsNonNegatedGuardAndBuildsNegatedFix(t *testing.T) {
	src := `package test
fun foo(x: Int, y: Int?) {
    if (x > 3) throw IllegalArgumentException("too big")
    if (y == null) throw IllegalArgumentException()
}
`
	findings := runRuleByName(t, "UseRequire", src)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}
	apply := func(f scanner.Finding) string {
		return src[:f.Fix.StartByte] + f.Fix.Replacement + src[f.Fix.EndByte:]
	}
	if got := apply(findings[0]); !strings.Contains(got, "require(!(x > 3)) { \"too big\" }") {
		t.Errorf("unexpected fix: %s", got)
	}
	if got := apply(findings[1]); !strings.Contains(got, "require(y != null)\n") {
		t.Errorf("unexpected fix: %s", got)
	}
}

// TestUseCheckOrError_ReportsElseIfAndElseThrowsWithoutAFix follows detekt, which reports every throw of an
// IllegalStateException with no argument or a string message. Only a lone `if` guard can be rewritten.
func TestUseCheckOrError_ReportsElseIfAndElseThrowsWithoutAFix(t *testing.T) {
	findings := runRuleByName(t, "UseCheckOrError", `
package test
fun foo(x: Int, e: Exception) {
    if (x > 3) throw IllegalStateException("with cause", e)
    if (x > 4) println() else if (x > 5) throw IllegalStateException("else-if")
    if (x > 6) throw IllegalStateException("a") else println()
}
`)
	if len(findings) != 2 || findings[0].Line != 5 || findings[1].Line != 6 {
		t.Fatalf("expected findings on lines 5 and 6, got %v", findings)
	}
	for _, f := range findings {
		if f.Fix != nil {
			t.Errorf("line %d: an if with an else cannot become check(), got fix %v", f.Line, f.Fix)
		}
	}
}

func TestUseLet_OnlyReportsIfElseWithANullBranch(t *testing.T) {
	findings := runRuleByName(t, "UseLet", `
package test
fun foo(a: String?, b: String?): Any? {
    if (a != null) println(a)
    if (a != null) { println(a) } else println()
    val c = if (a == null) null else a.length
    val d = if (a == null) { null } else { a.length }
    val e = if (a != null) 1 else 0
    val f = if (null == b) null else b.length
    return if (a != null && b != null) 1 else null
}
`)
	if len(findings) != 3 || findings[0].Line != 6 || findings[1].Line != 7 || findings[2].Line != 9 {
		t.Fatalf("expected findings on lines 6, 7 and 9, got %v", findings)
	}
}

func TestUseDataClass_SkipsPrivateConstructorAndInner(t *testing.T) {
	findings := runRuleByName(t, "UseDataClass", `
package test
class Plain(val a: Int)
class Hidden private constructor(val a: Int)
class Outer {
    inner class In(val a: Int)
}
@JvmInline value class V(val a: Int)
`)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "Plain") {
		t.Fatalf("expected only Plain, got %v", findings)
	}
}

func TestUseOrEmpty_SkipsReceiversWhoseDeclaredTypeDiffers(t *testing.T) {
	findings := runRuleByName(t, "UseOrEmpty", `
package test
class A(val set: Set<Int>?, val anyVal: Any?) {
    fun f(str: String?, list: List<Int>?, any: Any?, other: Set<Int>?) {
        val a = str ?: ""
        val b = list ?: emptyList()
        val c = any ?: ""
        val d = other ?: emptyList()
        val e = set ?: emptySet()
        val g = anyVal ?: ""
    }
}
`)
	if len(findings) != 3 {
		t.Fatalf("expected str, list and set to be reported, got %d: %v", len(findings), findings)
	}
}

func TestEqualsNullCall_FixKeepsPrecedenceAndSkipsSafeCalls(t *testing.T) {
	src := `package test
fun f(a: Any?, b: Any?) {
    if (a.equals(null)) {}
    val c = !a.equals(null)
    val d = b?.equals(null)
}
`
	findings := runRuleByName(t, "EqualsNullCall", src)
	if len(findings) != 3 {
		t.Fatalf("expected three findings, got %d", len(findings))
	}
	apply := func(f scanner.Finding) string { return src[:f.Fix.StartByte] + f.Fix.Replacement + src[f.Fix.EndByte:] }
	if got := apply(findings[0]); !strings.Contains(got, "if (a == null) {}") {
		t.Errorf("unexpected fix: %s", got)
	}
	if got := apply(findings[1]); !strings.Contains(got, "val c = !(a == null)") {
		t.Errorf("unexpected fix: %s", got)
	}
	if findings[2].Fix != nil {
		t.Error("b?.equals(null) has no plain == null equivalent")
	}
}

func TestExplicitCollectionElementAccessMethod_SetOnlyAsStatementAndNamedArgs(t *testing.T) {
	src := `package test
fun f(l: MutableList<Int>, m: Map<String, Int>) {
    l.set(0, 1)
    val old = l.set(1, 2)
    m.get(key = "a")
}
`
	findings := runRuleByNameWithResolver(t, "ExplicitCollectionElementAccessMethod", src)
	for _, f := range findings {
		if strings.Contains(f.Message, "'set'") && f.Line == 4 {
			t.Errorf("set() used as a value must not be reported")
		}
		if f.Fix != nil && strings.Contains(f.Fix.Replacement, "key =") {
			t.Errorf("index operands cannot be named: %s", f.Fix.Replacement)
		}
	}
}

func TestUseAnyOrNoneInsteadOfFind_SafeCallFixKeepsNullSemantics(t *testing.T) {
	src := `package test
fun f(l: List<Int>?) {
    val a = l?.find { it > 1 } != null
    val b = l?.find { it > 1 } == null
}
`
	findings := runRuleByName(t, "UseAnyOrNoneInsteadOfFind", src)
	if len(findings) != 2 {
		t.Fatalf("expected two findings, got %d", len(findings))
	}
	apply := func(f scanner.Finding) string { return src[:f.Fix.StartByte] + f.Fix.Replacement + src[f.Fix.EndByte:] }
	if got := apply(findings[0]); !strings.Contains(got, "l?.any { it > 1 } == true") {
		t.Errorf("unexpected fix: %s", got)
	}
	if got := apply(findings[1]); !strings.Contains(got, "l?.none { it > 1 } ?: true") {
		t.Errorf("unexpected fix: %s", got)
	}
}

func TestUseEmptyCounterpart_FixKeepsTypeArguments(t *testing.T) {
	src := `package test
val a = listOf<String>()
`
	findings := runRuleByName(t, "UseEmptyCounterpart", src)
	if len(findings) != 1 {
		t.Fatalf("expected one finding, got %d", len(findings))
	}
	f := findings[0]
	if got := src[:f.Fix.StartByte] + f.Fix.Replacement + src[f.Fix.EndByte:]; !strings.Contains(got, "emptyList<String>()") {
		t.Errorf("unexpected fix: %s", got)
	}
}

func TestUseCheckOrError_BlockBodyAndCauseArgument(t *testing.T) {
	src := `package test
fun f(a: Int, msg: String) {
    if (a > 1) { throw IllegalStateException("m") }
    if (a > 2) throw IllegalStateException(RuntimeException())
    if (a > 3) throw IllegalStateException(msg)
    if (a > 4) { println(); throw IllegalStateException("m") }
}
`
	findings := runRuleByName(t, "UseCheckOrError", src)
	if len(findings) != 3 || findings[0].Line != 3 || findings[1].Line != 5 || findings[2].Line != 6 {
		t.Fatalf("expected findings on lines 3, 5 and 6, got %v", findings)
	}
	if findings[2].Fix != nil {
		t.Errorf("a throw after another statement cannot be rewritten, got fix %v", findings[2].Fix)
	}
	f := findings[0]
	if got := src[:f.Fix.StartByte] + f.Fix.Replacement + src[f.Fix.EndByte:]; !strings.Contains(got, `check(a <= 1) { "m" }`) && !strings.Contains(got, `check(!(a > 1)) { "m" }`) {
		t.Errorf("unexpected fix: %s", got)
	}
}

func TestUseIfInsteadOfWhen_FollowsDetektShape(t *testing.T) {
	flagged := map[string]string{
		"subject with else":     "fun f(x: Int) = when (x) { 1 -> 1; else -> 2 }",
		"subjectless with else": "fun f(x: Int) = when { x > 1 -> 1; else -> 2 }",
	}
	for name, src := range flagged {
		if got := runRuleByName(t, "UseIfInsteadOfWhen", "package test\n"+src+"\n"); len(got) != 1 {
			t.Errorf("%s: want 1 finding, got %d", name, len(got))
		}
	}
	clean := map[string]string{
		"two entries without else": "fun f(x: Int) = when (x) { 1 -> 1; 2 -> 2 }",
		"one entry":                "fun f(x: Int) { when (x) { 1 -> println() } }",
		"three entries":            "fun f(x: Int) = when (x) { 1 -> 1; 2 -> 2; else -> 3 }",
		"several conditions":       "fun f(x: Int) = when (x) { 1, 2 -> 1; else -> 3 }",
	}
	for name, src := range clean {
		if got := runRuleByName(t, "UseIfInsteadOfWhen", "package test\n"+src+"\n"); len(got) != 0 {
			t.Errorf("%s: want no findings, got %d", name, len(got))
		}
	}
}

func TestPreferToOverPairSyntax_Positive(t *testing.T) {
	findings := runRuleByName(t, "PreferToOverPairSyntax", `
package test
fun foo(a: Int, b: String) {
    val p = Pair(a + 1, b)
    val q = kotlin.Pair<Int, String>(a, b)
    val r = Pair(first = a, second = b)
    val s = Pair(Pair(1, 2), 3)
    val t = Pair(a) { b }
}
`)
	want := []string{
		"`a + 1 to b`",
		"`a to b`",
		"`first = a to second = b`",
		"`Pair(1, 2) to 3`",
		"`1 to 2`",
		"`a to { b }`",
	}
	if len(findings) != len(want) {
		t.Fatalf("expected %d findings, got %d: %v", len(want), len(findings), findings)
	}
	for _, w := range want {
		found := false
		for _, f := range findings {
			if strings.Contains(f.Message, w) {
				found = true
			}
		}
		if !found {
			t.Errorf("no finding with replacement %s", w)
		}
	}
}

func TestPreferToOverPairSyntax_Negative(t *testing.T) {
	findings := runRuleByName(t, "PreferToOverPairSyntax", `
package test
import android.util.Pair as AndroidPair
class Factory { fun Pair(a: Int, b: Int) = a + b }
fun foo(a: Int, f: Factory) {
    val infix = a to 1
    val member = f.Pair(1, 2)
    val ref = ::Pair
    val android = AndroidPair(1, 2)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %v", findings)
	}
}

func TestPreferToOverPairSyntax_LookalikeDeclarations(t *testing.T) {
	for name, src := range map[string]string{
		"own class":       "package test\nclass Pair<A, B>(val a: A, val b: B)\nfun foo() = Pair(1, 2)\n",
		"own function":    "package test\nfun Pair(a: Int, b: Int) = a + b\nfun foo() = Pair(1, 2)\n",
		"local class":     "package test\nfun foo() {\n    class Pair(val a: Int, val b: Int)\n    Pair(1, 2)\n}\n",
		"explicit import": "package test\nimport android.util.Pair\nfun foo() = Pair(1, 2)\n",
		"star import":     "package test\nimport androidx.core.util.*\nfun foo() = Pair(1, 2)\n",
	} {
		if findings := runRuleByName(t, "PreferToOverPairSyntax", src); len(findings) != 0 {
			t.Errorf("%s: expected no findings, got %v", name, findings)
		}
	}
}

func TestPreferToOverPairSyntax_ImportAliasAndKotlinImport(t *testing.T) {
	findings := runRuleByName(t, "PreferToOverPairSyntax", `
package test
import kotlin.Pair
import kotlin.Pair as KPair
fun foo() {
    val a = Pair(1, 2)
    val b = KPair(3, 4)
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %v", findings)
	}
}

func TestPreferToOverPairSyntax_NoFix(t *testing.T) {
	findings := runRuleByName(t, "PreferToOverPairSyntax", "package test\nfun foo() = Pair(1, 2)\n")
	if len(findings) != 1 || findings[0].Fix != nil {
		t.Fatalf("expected one finding without a fix, got %v", findings)
	}
}

// detekt's isString is false for a nullable String and for the caught exception itself, so throws that pass
// them are not reported.
func TestUseCheckOrError_SkipsNullableMessageAndCaughtException(t *testing.T) {
	findings := runRuleByName(t, "UseCheckOrError", `package test
fun f() {
    try {
        println()
    } catch (e: Exception) {
        throw IllegalStateException(e.message)
    }
    try {
        println()
    } catch (e: Exception) {
        throw IllegalStateException(e)
    }
    try {
        println()
    } catch (e: Exception) {
        throw IllegalStateException(e.toString())
    }
}
fun g(t: String) {
    throw IllegalStateException(t)
}
`)
	var lines []int
	for _, f := range findings {
		lines = append(lines, f.Line)
	}
	if want := []int{16, 20}; !slices.Equal(lines, want) {
		t.Fatalf("lines = %v, want %v", lines, want)
	}
}
