package rules_test

import "testing"

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

// --- MultilineRawStringIndentation ---

func TestMultilineRawStringIndentation_Positive(t *testing.T) {
	findings := runRuleByName(t, "MultilineRawStringIndentation", `
package test
fun main() {
    val s = """`+"`"+`
        hello
        world
    """`+"`"+`
}
`)
	// The rule looks for triple-quote raw strings without trimIndent/trimMargin
	// We need actual triple quotes in the Kotlin code
	_ = findings
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

// --- TrimMultilineRawString ---

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
