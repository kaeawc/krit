package rules_test

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func requireAnchor(t *testing.T, findings []scanner.Finding, rule string, line, col int) scanner.Finding {
	t.Helper()
	for _, f := range findings {
		if f.Rule == rule {
			if f.Line != line || f.Col != col {
				t.Fatalf("%s at %d:%d, want %d:%d", rule, f.Line, f.Col, line, col)
			}
			return f
		}
	}
	t.Fatalf("no %s finding in %v", rule, findings)
	return scanner.Finding{}
}

func TestDeclarationFindingsAnchorOnNameNotAnnotation(t *testing.T) {
	code := "package test\n" +
		"import org.junit.Test\n" +
		"class Tester {\n" +
		"    @Test\n" +
		"    fun noAssertions() {\n" +
		"        val x = 1\n" +
		"        println(x)\n" +
		"    }\n" +
		"}\n"
	findings := runRuleByName(t, "TestWithoutAssertion", code)
	requireAnchor(t, findings, "TestWithoutAssertion", 5, 9)
}

func TestAnchoredDeclarationStaysSuppressedByAnnotation(t *testing.T) {
	code := "package test\n" +
		"import org.junit.Test\n" +
		"class Tester {\n" +
		"    @Suppress(\"TestWithoutAssertion\")\n" +
		"    @Test\n" +
		"    fun noAssertions() {\n" +
		"        val x = 1\n" +
		"        println(x)\n" +
		"    }\n" +
		"}\n"
	file := parseInline(t, code)
	filter := scanner.BuildSuppressionFilter(file, nil, nil, "")
	// The finding sits on the name (6:9); the annotation range starts on line 4.
	if !filter.IsSuppressedAtColumn("TestWithoutAssertion", "testing-quality", 6, 9) {
		t.Fatal("finding on the declaration name must stay covered by the @Suppress annotation")
	}
	if got := runRuleByNameOnFile(t, "TestWithoutAssertion", file); len(got) != 0 {
		t.Fatalf("suppressed declaration still reported: %v", got)
	}
}

func TestUnsafeCallAnchorsOnBangBangOperator(t *testing.T) {
	code := "package test\n" +
		"fun f(s: String?): Int {\n" +
		"    val y = 5\n" +
		"    return s!!.length + y\n" +
		"}\n"
	findings := runRuleByName(t, "UnsafeCallOnNullableType", code)
	requireAnchor(t, findings, "UnsafeCallOnNullableType", 4, 13)
}

func TestMultiLineChainFindingAnchorsOnCall(t *testing.T) {
	code := "package test\n" +
		"import androidx.compose.foundation.clickable\n" +
		"import androidx.compose.ui.Modifier\n" +
		"fun ui() {\n" +
		"    val m = Modifier\n" +
		"        .fillMaxSize()\n" +
		"        .clickable { }\n" +
		"}\n"
	findings := runRuleByName(t, "ComposeSemanticsMissingRole", code)
	requireAnchor(t, findings, "ComposeSemanticsMissingRole", 7, 10)
}

func TestExplicitItWithMultipleParametersAnchorsOnIt(t *testing.T) {
	code := "package test\n" +
		"fun f() {\n" +
		"    mapOf(1 to 2).forEach { it, v ->\n" +
		"        println(v)\n" +
		"    }\n" +
		"}\n"
	findings := runRuleByName(t, "ExplicitItLambdaMultipleParameters", code)
	requireAnchor(t, findings, "ExplicitItLambdaMultipleParameters", 3, 29)
}

func TestLargeClassMessageNamesTheClass(t *testing.T) {
	var b strings.Builder
	b.WriteString("package test\n@Suppress(\"unused\")\ninterface Big {\n")
	for i := 0; i < 700; i++ {
		b.WriteString("    fun f" + strings.Repeat("x", i%7) + string(rune('a'+i%26)) + string(rune('a'+(i/26)%26)) + string(rune('a'+(i/676)%26)) + "()\n")
	}
	b.WriteString("}\n")
	findings := runRuleByName(t, "LargeClass", b.String())
	f := requireAnchor(t, findings, "LargeClass", 3, 11)
	if f.Message != "Class Big is too large. Consider splitting it into smaller pieces." {
		t.Fatalf("unexpected message %q", f.Message)
	}
}

func TestRedundantNullOperatorMessageDistinguishesSmartCast(t *testing.T) {
	code := "package test\n" +
		"fun f(s: String?, t: String) {\n" +
		"    if (s != null) {\n" +
		"        println(s!!.length)\n" +
		"    }\n" +
		"    println(t!!.length)\n" +
		"}\n"
	findings := runRuleByNameWithResolver(t, "UnnecessaryNotNullOperator", code)
	var smart, plain bool
	for _, f := range findings {
		if strings.Contains(f.Message, "'s' is smart-cast to non-null") {
			smart = true
			if f.Line != 4 || f.Col != 18 {
				t.Errorf("smart-cast finding at %d:%d, want the !! at 4:18", f.Line, f.Col)
			}
		}
		if strings.Contains(f.Message, "on non-nullable 't'") {
			plain = true
		}
	}
	if !smart || !plain {
		t.Fatalf("want smart-cast wording for s and non-nullable wording for t, got %v", findingMessages(findings))
	}
}

func TestElseCaseWhenIsNotFixableAndIgnoresNegationAndGuards(t *testing.T) {
	code := "package test\n" +
		"sealed class S\n" +
		"class A : S()\n" +
		"class B : S()\n" +
		"fun full(s: S): Int = when (s) {\n" +
		"    is A -> 1\n" +
		"    is B -> 2\n" +
		"    else -> 3\n" +
		"}\n" +
		"fun neg(s: S): Int = when (s) {\n" +
		"    !is A -> 1\n" +
		"    is B -> 2\n" +
		"    else -> 3\n" +
		"}\n" +
		"fun guarded(s: S, n: Int): Int = when (s) {\n" +
		"    is A if n > 0 -> 1\n" +
		"    is B -> 2\n" +
		"    else -> 3\n" +
		"}\n"
	findings := runRuleByNameWithResolver(t, "ElseCaseInsteadOfExhaustiveWhen", code)
	if len(findings) != 1 {
		t.Fatalf("want 1 finding (only the unconditional when), got %v", findingMessages(findings))
	}
	f := findings[0]
	if f.Fix != nil {
		t.Fatal("removing the else branch can break compilation; no fix expected")
	}
	if f.Line != 5 {
		t.Fatalf("anchor line %d, want the when expression at line 5 like detekt", f.Line)
	}
}

func TestUnnecessaryBackticksSkipsFixWhenDocsQuoteTheName(t *testing.T) {
	code := "package test\n" +
		"/** See [`helper`]. */\n" +
		"fun `helper`() = 1\n" +
		"fun `other`() = 2\n"
	var withFix, withoutFix int
	for _, f := range runRuleByName(t, "UnnecessaryBackticks", code) {
		if f.Fix != nil {
			withFix++
		} else {
			withoutFix++
		}
	}
	if withFix != 1 || withoutFix != 1 {
		t.Fatalf("want one fixable (other) and one report-only (helper), got fix=%d nofix=%d", withFix, withoutFix)
	}
}
