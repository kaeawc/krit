package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
)

// --- MultilineLambdaItParameter ---

func TestMultilineLambdaItParameter_Positive(t *testing.T) {
	findings := runRuleByName(t, "MultilineLambdaItParameter", `
package test
fun main() {
    listOf(1, 2, 3).forEach {
        println(it.toString())
        println(it)
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for multiline lambda using 'it'")
	}
}

func TestMultilineLambdaItParameter_Negative(t *testing.T) {
	findings := runRuleByName(t, "MultilineLambdaItParameter", `
package test
fun main() {
    listOf(1, 2, 3).forEach { item ->
        println(item.toString())
        println(item)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestMultilineLambdaItParameter_IgnoresCommentsAndNonProductionSources(t *testing.T) {
	commentOnly := `
package test
fun main() {
    initialize {
        // Initialize it here before first use.
        Dispatchers.Main
    }
}
`
	if findings := runRuleByName(t, "MultilineLambdaItParameter", commentOnly); len(findings) != 0 {
		t.Fatalf("expected no findings when only comments mention it, got %d", len(findings))
	}
	for _, path := range []string{"build.gradle.kts", "src/test/kotlin/FooTest.kt"} {
		findings := runRuleByNameOnPath(t, "MultilineLambdaItParameter", path, `
package test
fun main() {
    listOf(1).forEach {
        println(it)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected no findings for %s, got %d", path, len(findings))
		}
	}
}

// --- MultilineRawStringIndentation ---

func TestMultilineRawStringIndentation_Positive(t *testing.T) {
	findings := runRuleByName(t, "MultilineRawStringIndentation", `
package test
fun main() {
    val s = """
        hello
        world
    """
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for multiline raw string without trimIndent/trimMargin")
	}
}

func TestMultilineRawStringIndentation_Negative(t *testing.T) {
	findings := runRuleByName(t, "MultilineRawStringIndentation", `
package test
fun main() {
    val s = "hello world"
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestMultilineRawStringIndentation_IgnoresTripleQuotesOutsideRawStringNodes(t *testing.T) {
	findings := runRuleByName(t, "MultilineRawStringIndentation", `
package test
fun main() {
    /*
     * Example raw string delimiter: """
     */
    val regular = "foo \"\"\" bar"
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for triple quotes in comments or regular strings, got %d", len(findings))
	}
}

// --- TrimMultilineRawString ---

func TestTrimMultilineRawString_Positive(t *testing.T) {
	findings := runRuleByName(t, "TrimMultilineRawString", `
package test
fun main() {
    val s = """
        hello
        world
    """
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for multiline raw string without trimIndent/trimMargin")
	}
	if findings[0].Fix == nil {
		t.Fatal("expected trim rule to provide a fix")
	}
}

func TestTrimMultilineRawString_Negative(t *testing.T) {
	findings := runRuleByName(t, "TrimMultilineRawString", `
package test
fun main() {
    val s = "hello"
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestTrimMultilineRawString_IgnoresTripleQuotesOutsideRawStringNodes(t *testing.T) {
	findings := runRuleByName(t, "TrimMultilineRawString", `
package test
fun main() {
    /*
     * Example raw string delimiter: """
     */
    val regular = "foo \"\"\" bar"
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for triple quotes in comments or regular strings, got %d", len(findings))
	}
}

// --- StringShouldBeRawString ---

func TestStringShouldBeRawString_Positive(t *testing.T) {
	findings := runRuleByName(t, "StringShouldBeRawString", `
package test
fun main() {
    val s = "line1\nline2\nline3\t"
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for string with many escapes")
	}
}

func TestStringShouldBeRawString_Negative(t *testing.T) {
	findings := runRuleByName(t, "StringShouldBeRawString", `
package test
fun main() {
    val s = "hello world"
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- CanBeNonNullable ---

func TestCanBeNonNullable_NullableParameterOnlyUsedWithBangBang(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "CanBeNonNullable", `
package test
fun process(x: String?) {
    println(x!!)
    val length = x!!.length
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for nullable parameter only used with non-null assertions")
	}
}

func TestCanBeNonNullable_NullableParameterNameOnlyAppearsAsSubstring(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "CanBeNonNullable", `
package test
fun process(i: String?) {
    print("index")
    val minimum = minOf(1, 2)
    this.toString()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when parameter name only appears as a substring, got %d", len(findings))
	}
}

func TestCanBeNonNullable_IgnoresShadowedNestedParameter(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "CanBeNonNullable", `
package test
fun process(x: String?) {
    listOf("value").forEach { x ->
        println(x!!)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when only a shadowed lambda parameter is asserted, got %d", len(findings))
	}
}

func TestCanBeNonNullable_PositiveVarAssignedElvisWithNonNullFallback(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "CanBeNonNullable", `
package test
class Example {
    var prop: String? = ""
    fun update(candidate: String?, fallback: String) {
        prop = candidate ?: fallback
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding when nullable property is only assigned an Elvis expression with a non-null fallback")
	}
}

func TestCanBeNonNullable_NegativeVarAssignedNullLiteral(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "CanBeNonNullable", `
package test
class Example {
    var prop: String? = ""
    fun clear() {
        prop = null
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when nullable property is assigned null, got %d", len(findings))
	}
}

func TestCanBeNonNullable_NegativeVarAssignedNullableSafeCall(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "CanBeNonNullable", `
package test
class Example {
    var prop: String? = ""
    fun update(candidate: String?) {
        prop = candidate?.trim()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when nullable property is assigned a safe-call result, got %d", len(findings))
	}
}

func TestCanBeNonNullable_NegativeVarAssignedNullableIdentifier(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "CanBeNonNullable", `
package test
class Example {
    var prop: String? = ""
    fun update(candidate: String?) {
        prop = candidate
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when nullable property is assigned a nullable identifier, got %d", len(findings))
	}
}

// --- DoubleNegativeExpression ---

func TestDoubleNegativeExpression_Positive(t *testing.T) {
	findings := runRuleByName(t, "DoubleNegativeExpression", `
package test
fun main() {
    val x = !list.isNotEmpty()
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for double negative expression")
	}
}

func TestDoubleNegativeExpression_Negative(t *testing.T) {
	findings := runRuleByName(t, "DoubleNegativeExpression", `
package test
fun main() {
    val x = list.isEmpty()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- DoubleNegativeLambda ---

func TestDoubleNegativeLambda_Positive(t *testing.T) {
	findings := runRuleByName(t, "DoubleNegativeLambda", `
package test
fun main() {
    val result = list.filterNot { !it.isValid() }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for double negative lambda")
	}
}

func TestDoubleNegativeLambda_Negative(t *testing.T) {
	findings := runRuleByName(t, "DoubleNegativeLambda", `
package test
fun main() {
    val result = list.filter { it.isValid() }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestDoubleNegativeLambda_HonorsNegativeFunctions(t *testing.T) {
	// NegativeFunctions was previously a dead config — exposed in zz_meta
	// but never consulted. Configure it via the rule pointer and verify
	// custom callee names are flagged when the lambda body is a single
	// `!` prefix expression.
	var rule *rules.DoubleNegativeLambdaRule
	for _, candidate := range v2rules.Registry {
		if candidate.ID == "DoubleNegativeLambda" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.DoubleNegativeLambdaRule)
			if !ok {
				t.Fatalf("expected DoubleNegativeLambdaRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("DoubleNegativeLambda rule not registered")
	}
	original := rule.NegativeFunctions
	defer func() { rule.NegativeFunctions = original }()

	rule.NegativeFunctions = []string{"rejectAll"}

	// Custom callee with `!`-only body — fires under the new config.
	if findings := runRuleByName(t, "DoubleNegativeLambda", `
package test
fun main(list: List<String>) {
    list.rejectAll { !it.isEmpty() }
}
`); len(findings) == 0 {
		t.Fatal("expected finding for configured negative-function callee 'rejectAll'")
	}

	// A non-configured custom callee still doesn't fire.
	if findings := runRuleByName(t, "DoubleNegativeLambda", `
package test
fun main(list: List<String>) {
    list.allowAll { !it.isEmpty() }
}
`); len(findings) != 0 {
		t.Fatalf("expected no findings for un-configured callee, got %d", len(findings))
	}
}

// --- NullableBooleanCheck ---

func TestNullableBooleanCheck_Positive(t *testing.T) {
	findings := runRuleByName(t, "NullableBooleanCheck", `
package test
fun main() {
    val flag: Boolean? = true
    if (flag == true) {
        println("yes")
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for nullable boolean check")
	}
}

func TestNullableBooleanCheck_Negative(t *testing.T) {
	findings := runRuleByName(t, "NullableBooleanCheck", `
package test
fun main() {
    val flag = true
    if (flag) {
        println("yes")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- RangeUntilInsteadOfRangeTo ---

func TestRangeUntilInsteadOfRangeTo_Positive(t *testing.T) {
	findings := runRuleByName(t, "RangeUntilInsteadOfRangeTo", `
package test
fun main() {
    for (i in 0 until 10) {
        println(i)
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for 'until' usage")
	}
}

func TestRangeUntilInsteadOfRangeTo_Negative(t *testing.T) {
	findings := runRuleByName(t, "RangeUntilInsteadOfRangeTo", `
package test
fun main() {
    for (i in 0..<10) {
        println(i)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- DestructuringDeclarationWithTooManyEntries ---

func TestDestructuringDeclarationWithTooManyEntries_Positive(t *testing.T) {
	findings := runRuleByName(t, "DestructuringDeclarationWithTooManyEntries", `
package test
fun main() {
    val (a, b, c, d) = listOf(1, 2, 3, 4)
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for destructuring with too many entries")
	}
}

func TestDestructuringDeclarationWithTooManyEntries_Negative(t *testing.T) {
	findings := runRuleByName(t, "DestructuringDeclarationWithTooManyEntries", `
package test
fun main() {
    val (a, b) = Pair(1, 2)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}
