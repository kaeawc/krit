package rules_test

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
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

func TestMultilineLambdaItParameter_DoesNotMisparseEmbeddedQuotesInRawString(t *testing.T) {
	// The raw-string body contains an embedded `"hi"` and a `//keep` that
	// previously confused the comment-stripper's state machine, leaking
	// raw-string content out and risking spurious `it.` matches once a
	// future maintainer adds substring checks. This locks the parser
	// behavior so the rule's downstream substring scan sees a coherent
	// view of the lambda body even when raw strings hide special chars.
	src := `
package test
fun main(items: List<String>) {
    items.forEach { item ->
        val s = """he said "hi"//keep me
trailing"""
        println(s + item)
    }
}
`
	if findings := runRuleByName(t, "MultilineLambdaItParameter", src); len(findings) != 0 {
		t.Fatalf("expected no findings for explicit-parameter lambda with embedded-quote raw string, got %d", len(findings))
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
        """.trimIndent()
    val untrimmed = """
      hello
    """
}
`)
	if len(findings) != 2 || findings[0].Line != 6 || findings[1].Line != 7 {
		t.Fatalf("expected the misaligned line and closing quotes of the trimmed string only, got %v", findings)
	}
}

func TestMultilineRawStringIndentation_AlignedStringsAreCompliant(t *testing.T) {
	findings := runRuleByName(t, "MultilineRawStringIndentation", `
package test
fun main() {
    val a = """
        hello

        world
    """.trimIndent()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %v", findings)
	}
}

func TestMultilineRawStringIndentation_RequiresBreaksAroundTheContent(t *testing.T) {
	findings := runRuleByName(t, "MultilineRawStringIndentation", `
package test
fun main() {
    val b = """
        |hello
        |world""".trimMargin()
}
`)
	if len(findings) != 1 || findings[0].Message != "A multiline raw string should start with a break line and should end with another" {
		t.Fatalf("expected one missing-break finding, got %v", findings)
	}
}

func TestMultilineRawStringIndentation_DeeperContentIsReportedOnceAtItsFirstLine(t *testing.T) {
	findings := runRuleByName(t, "MultilineRawStringIndentation", `
package test
fun main() {
    val a = """
            first
              nested
            last
    """.trimIndent()
}
`)
	if len(findings) != 1 || findings[0].Line != 5 || findings[0].Message != "The indentation should be 8 but it is 12." {
		t.Fatalf("expected one finding on the first content line, got %v", findings)
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
    private var prop: String? = ""
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

func TestDoubleNegativeLambda_FixFilterNotToFilter(t *testing.T) {
	findings := runRuleByName(t, "DoubleNegativeLambda", `
package test
fun main(list: List<Int>) {
    val result = list.filterNot { !it.isValid() }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Fix == nil {
		t.Fatal("expected fix to be populated")
	}
	repl := findings[0].Fix.Replacement
	if !strings.Contains(repl, ".filter {") {
		t.Errorf("expected callee renamed to filter, got %q", repl)
	}
	if strings.Contains(repl, "filterNot") {
		t.Errorf("expected filterNot removed, got %q", repl)
	}
	if strings.Contains(repl, "!it.isValid") {
		t.Errorf("expected ! removed from lambda body, got %q", repl)
	}
}

func TestDoubleNegativeLambda_FixNoneToAll(t *testing.T) {
	findings := runRuleByName(t, "DoubleNegativeLambda", `
package test
fun main(list: List<Int>) {
    val r = list.none { !it.isValid() }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Fix == nil {
		t.Fatal("expected fix to be populated")
	}
	repl := findings[0].Fix.Replacement
	if !strings.Contains(repl, ".all {") {
		t.Errorf("expected callee renamed to all, got %q", repl)
	}
}

func TestDoubleNegativeLambda_NoFixForConfiguredCallee(t *testing.T) {
	var rule *rules.DoubleNegativeLambdaRule
	for _, candidate := range api.Registry {
		if candidate.ID == "DoubleNegativeLambda" {
			rule = candidate.Implementation.(*rules.DoubleNegativeLambdaRule)
			break
		}
	}
	if rule == nil {
		t.Fatal("rule not registered")
	}
	original := rule.NegativeFunctions
	defer func() { rule.NegativeFunctions = original }()
	rule.NegativeFunctions = []string{"rejectAll"}

	findings := runRuleByName(t, "DoubleNegativeLambda", `
package test
fun main(list: List<Int>) {
    val r = list.rejectAll { !it.isValid() }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Fix != nil {
		t.Errorf("expected no fix for configured callee (no canonical inverse), got %q", findings[0].Fix.Replacement)
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
	// NegativeFunctions was previously a dead config — exposed in metadata
	// but never consulted. Configure it via the rule pointer and verify
	// custom callee names are flagged when the lambda body is a single
	// `!` prefix expression.
	var rule *rules.DoubleNegativeLambdaRule
	for _, candidate := range api.Registry {
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
	findings := runRuleByNameWithResolver(t, "NullableBooleanCheck", `
package test
fun main(flag: Boolean?) {
    if (flag ?: false) {
        println("yes")
    }
    val enabled = flag ?: true
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings for elvis boolean checks, got %d", len(findings))
	}
	if got := findings[0].Fix.Replacement; got != "flag == true" {
		t.Errorf("`?: false` fix = %q, want `flag == true`", got)
	}
	if got := findings[1].Fix.Replacement; got != "flag != false" {
		t.Errorf("`?: true` fix = %q, want `flag != false`", got)
	}
}

func TestNullableBooleanCheck_Negative(t *testing.T) {
	findings := runRuleByName(t, "NullableBooleanCheck", `
package test
fun main(flag: Boolean?, other: Int?) {
    if (flag == true) {
        println("yes")
    }
    if (flag != false) {
        println("no")
    }
    val n = other ?: 0
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestNullableBooleanCheck_FixParenthesizesElvisChainOperand(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "NullableBooleanCheck", `
package test
fun main(a: Boolean?, b: Boolean?) {
    val x = a ?: b ?: false
}
`)
	if len(findings) != 1 || findings[0].Fix == nil {
		t.Fatalf("expected one finding with a fix, got %v", findings)
	}
	if got := findings[0].Fix.Replacement; got != "(a ?: b) == true" {
		t.Errorf("the elvis chain operand must be parenthesized, got %q", got)
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

func TestDestructuringDeclarationWithTooManyEntries_ForLoopAndLambdaPositive(t *testing.T) {
	findings := runRuleByName(t, "DestructuringDeclarationWithTooManyEntries", `
package test
fun main(items: List<Quad>) {
    for ((a, b, c, d) in items) {}
    items.forEach { (a, b, c, d) -> println(a) }
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}
}

func TestDestructuringDeclarationWithTooManyEntries_SecondaryConstructorNotFlagged(t *testing.T) {
	findings := runRuleByName(t, "DestructuringDeclarationWithTooManyEntries", `
package test
class Widget(a: Int) {
    constructor(a: Int, b: Int, c: Int, d: Int) : this(a)
    @JvmOverloads constructor(a: Int, b: Int, c: Int, d: Int, e: Int = 0) : this(a)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestDestructuringDeclarationWithTooManyEntries_ParameterListsAndWithinLimitNotFlagged(t *testing.T) {
	findings := runRuleByName(t, "DestructuringDeclarationWithTooManyEntries", `
package test
class Widget(val a: Int, val b: Int, val c: Int, val d: Int) {
    constructor(a: Int, b: Int, c: Int, d: Int, e: Int) : this(a, b, c, d)
    fun place(left: Int, top: Int, right: Int, bottom: Int) {}
}
fun main(pair: Pair<Int, Int>, triple: Triple<Int, Int, Int>) {
    val (a, b) = pair
    val (x, y, z) = triple
    for ((k, v) in mapOf(1 to 2)) {}
    listOf(pair).forEach { (first, second) -> println(first + second) }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestNullableBooleanCheck_ElvisOnNullableBoolean(t *testing.T) {
	src := `package test
fun main(plain: Boolean, maybe: Boolean?, s: String?, other: Boolean) {
    if (plain == true) {}
    if (maybe ?: false) {}
    if (maybe ?: true) {}
    if (s?.isEmpty() ?: false) {}
    if (maybe == true) {}
    val x = other == maybe ?: false
}
`
	findings := runRuleByNameWithResolver(t, "NullableBooleanCheck", src)
	if len(findings) != 4 {
		t.Fatalf("expected the four elvis comparisons on nullable Booleans, got %d: %v", len(findings), findings)
	}
	apply := func(f scanner.Finding) string { return src[:f.Fix.StartByte] + f.Fix.Replacement + src[f.Fix.EndByte:] }
	for i, want := range []string{
		"if (maybe == true) {}",
		"if (maybe != false) {}",
		"if (s?.isEmpty() == true) {}",
		"val x = other == (maybe == true)",
	} {
		if got := apply(findings[i]); !strings.Contains(got, want) {
			t.Errorf("fix %d: want %q in %s", i, want, got)
		}
	}
}

func TestTrimMultilineRawString_ArgumentsConstAndAnnotations(t *testing.T) {
	findings := runRuleByName(t, "TrimMultilineRawString", `
package test
const val CONST_SQL = """
    select 1
"""
@Doc("""
    text
""")
fun a() {}
fun b() = """
    text
""".trimMargin("|")
fun c() = """
    text
""".trimIndent()
fun d() = """
    text
"""
`)
	if len(findings) != 1 {
		t.Fatalf("expected only the untrimmed raw string in d(), got %d: %v", len(findings), findings)
	}
}

func TestMultilineLambdaItParameter_UsesAstNotTextHeuristics(t *testing.T) {
	findings := runRuleByName(t, "MultilineLambdaItParameter", `
package test
fun f(l: List<Int>) {
    l.forEach {
        when (it) {
            1 -> println()
            else -> println()
        }
    }
    l.forEach {
        println("it is a word in a string")
        println()
    }
    l.forEach {
        l.map { println() }
        println()
    }
    l.forEach { x ->
        println(it)
        println(x)
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected only the first lambda (when inside it), got %d: %v", len(findings), findings)
	}
}

func TestCanBeNonNullable_PublicMembersNullDefaultsAndActuals(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "CanBeNonNullable", `
package test
class A {
    var pub: String? = ""
    private var priv: String? = ""
    open var opened: String? = ""
}
fun withDefault(p: String? = null) { println(p!!) }
actual fun act(p: String?) { println(p!!) }
fun plain(p: String?) { println(p!!) }
`)
	lines := map[int]bool{}
	for _, f := range findings {
		lines[f.Line] = true
	}
	if len(findings) != 2 || !lines[5] || !lines[10] {
		t.Fatalf("expected only the private property and the plain parameter, got %v", findings)
	}
}

// detekt reports any negating operator or negatively named call inside the lambda, not only a whole-body negation.
func TestDoubleNegativeLambda_ReportsAnyNegationInsideTheLambda(t *testing.T) {
	findings := runRuleByName(t, "DoubleNegativeLambda", `
package test
fun f(l: List<String>) {
    l.none { !it.isEmpty() }
    l.none { !it.isEmpty() || it.isBlank() }
    l.filterNot { !it.isEmpty() && true }
    l.none { it.isNotEmpty() }
    l.none { it.isEmpty() }
}
`)
	if len(findings) != 4 {
		t.Fatalf("expected 4 findings, got %v", findings)
	}
	want := "Double negative through using `isNotEmpty()` inside a `none` lambda. Use `all` instead."
	if findings[3].Message != want {
		t.Errorf("message = %q, want %q", findings[3].Message, want)
	}
}

func TestUntilInsteadOfRangeTo_ReportsRangeAndRangeToWithMinusOne(t *testing.T) {
	findings := runRuleByName(t, "UntilInsteadOfRangeTo", `package test
fun f(n: Int) {
    val a = 0..n - 1
    val b = 0.rangeTo(n - 1)
    val c = 0..n
    val d = 0..n - 2
    val e = 0..(n - 1)
    val g = 0..<n
}
`)
	messages := map[int]string{}
	for _, f := range findings {
		messages[f.Line] = f.Message
	}
	if len(findings) != 2 || messages[3] != "'..' call can be replaced with 'until'" || messages[4] != "'rangeTo' call can be replaced with 'until'" {
		t.Fatalf("expected the '..' on line 3 and the rangeTo on line 4, got %v", messages)
	}
}
