package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
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
fun foo(x: String?) {
    if (x != null) {
        println(x)
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for null check without else")
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

func TestUseDataClass_HonorsAllowVars(t *testing.T) {
	// AllowVars was previously a dead config — exposed in zz_meta but
	// never consulted. The check fired on classes whose primary
	// constructor used `var` parameters. Default behavior (false,
	// matching detekt) now skips those classes.
	var rule *rules.UseDataClassRule
	for _, candidate := range v2rules.Registry {
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
	// Default (false): class with var properties is NOT a data-class
	// candidate.
	if findings := runRuleByName(t, "UseDataClass", codeWithVar); len(findings) != 0 {
		t.Fatalf("expected no findings under default AllowVars=false, got %d", len(findings))
	}

	// Flipping to true brings the class back into scope.
	rule.AllowVars = true
	if findings := runRuleByName(t, "UseDataClass", codeWithVar); len(findings) == 0 {
		t.Fatal("expected finding under AllowVars=true for class with var properties")
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
	// IgnoreWhenContainingVariableDeclaration was previously a dead
	// config — exposed in v2 metadata but never consulted. With it
	// set to true, when-expressions whose branches contain a property
	// declaration are skipped, since they don't translate cleanly to
	// `if` (each branch's local would need a different scope).
	var rule *rules.UseIfInsteadOfWhenRule
	for _, candidate := range v2rules.Registry {
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
fun foo(x: Boolean) {
    when {
        x -> {
            val msg = "yes"
            println(msg)
        }
        else -> println("no")
    }
}
`
	// Default (false): even with a declaration inside, the when fires.
	if findings := runRuleByName(t, "UseIfInsteadOfWhen", codeWithDecl); len(findings) == 0 {
		t.Fatal("expected finding under default IgnoreWhenContainingVariableDeclaration=false")
	}

	rule.IgnoreWhenContainingVariableDeclaration = true

	if findings := runRuleByName(t, "UseIfInsteadOfWhen", codeWithDecl); len(findings) != 0 {
		t.Fatalf("expected no findings under IgnoreWhenContainingVariableDeclaration=true with branch-local val, got %d", len(findings))
	}

	// A simple when with no declarations still fires under the flag.
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
