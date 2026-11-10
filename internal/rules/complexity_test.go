package rules_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// --- LongMethod ---

func TestLongMethod_Positive(t *testing.T) {
	// Build a function with 62 lines (line 1 = fun, lines 2-61 = 60 val assignments, line 62 = })
	var b strings.Builder
	b.WriteString("package test\nfun process() {\n")
	for i := 1; i <= 60; i++ {
		b.WriteString("    val x")
		b.WriteString(strings.Repeat("0", 0))
		b.WriteString(strconv.Itoa(i))
		b.WriteString(" = ")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("\n")
	}
	b.WriteString("}\n")
	findings := runRuleByName(t, "LongMethod", b.String())
	if len(findings) == 0 {
		t.Fatal("expected LongMethod finding for 62-line function")
	}
}

func TestLongMethod_Negative(t *testing.T) {
	// Build a function with 59 lines (below threshold of 60)
	var b strings.Builder
	b.WriteString("package test\nfun process() {\n")
	for i := 1; i <= 57; i++ {
		b.WriteString("    val x")
		b.WriteString(strconv.Itoa(i))
		b.WriteString(" = ")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("\n")
	}
	b.WriteString("}\n")
	findings := runRuleByName(t, "LongMethod", b.String())
	if len(findings) != 0 {
		t.Fatalf("expected no LongMethod finding for 59-line function, got %d", len(findings))
	}
}

func TestLongMethod_Suppressed(t *testing.T) {
	var b strings.Builder
	b.WriteString("package test\n")
	b.WriteString("@Suppress(\"LongMethod\")\n")
	b.WriteString("fun process() {\n")
	for i := 1; i <= 60; i++ {
		b.WriteString("    val x")
		b.WriteString(strconv.Itoa(i))
		b.WriteString(" = ")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("\n")
	}
	b.WriteString("}\n")
	findings := runRuleByName(t, "LongMethod", b.String())
	if len(findings) != 0 {
		t.Fatalf("expected suppressed LongMethod finding, got %d", len(findings))
	}
}

// Regression: a `//` line comment containing `"""` must not cause subsequent
// lines to be classified as raw-string content. Prior to the fix in
// countSignificantLines, `strings.Count(line, "\"\"\"")` toggled the
// in-raw-string flag for any line containing the substring `"""`, including
// inside line comments. That caused all following lines to be skipped, so
// long functions could fall under the threshold.
func TestLongMethod_TripleQuoteInLineComment(t *testing.T) {
	var b strings.Builder
	b.WriteString("package test\nfun process() {\n")
	b.WriteString("    // see: \"\"\" in docs\n")
	for i := 1; i <= 70; i++ {
		b.WriteString("    val x")
		b.WriteString(strconv.Itoa(i))
		b.WriteString(" = ")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("\n")
	}
	b.WriteString("}\n")
	findings := runRuleByName(t, "LongMethod", b.String())
	if len(findings) == 0 {
		t.Fatal("expected LongMethod finding despite `\"\"\"` in line comment")
	}
}

// Regression: a `/* ... */` block comment containing `"""` must not toggle
// raw-string state.
func TestLongMethod_TripleQuoteInBlockComment(t *testing.T) {
	var b strings.Builder
	b.WriteString("package test\nfun process() {\n")
	b.WriteString("    /* literal \"\"\" inside block comment */\n")
	for i := 1; i <= 70; i++ {
		b.WriteString("    val x")
		b.WriteString(strconv.Itoa(i))
		b.WriteString(" = ")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("\n")
	}
	b.WriteString("}\n")
	findings := runRuleByName(t, "LongMethod", b.String())
	if len(findings) == 0 {
		t.Fatal("expected LongMethod finding despite `\"\"\"` in block comment")
	}
}

// A multi-line raw string literal counts its non-blank content lines, as detekt does.
func TestLongMethod_RawStringContentCounted(t *testing.T) {
	var b strings.Builder
	b.WriteString("package test\nfun process() {\n")
	b.WriteString("    val s = \"\"\"\n")
	for i := 1; i <= 70; i++ {
		b.WriteString("        line ")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("\n")
	}
	b.WriteString("    \"\"\"\n")
	b.WriteString("    val y = 1\n")
	b.WriteString("}\n")
	findings := runRuleByName(t, "LongMethod", b.String())
	if len(findings) != 1 {
		t.Fatalf("expected a LongMethod finding (raw-string body counts), got %d", len(findings))
	}
}

// --- CyclomaticComplexMethod ---

func TestCyclomaticComplexMethod_Positive(t *testing.T) {
	// Each if adds 1, starting from base complexity of 1.
	// 15 if-expressions => complexity = 16 > 14
	var b strings.Builder
	b.WriteString("package test\nfun complex(x: Int): Int {\n    var r = 0\n")
	for i := 1; i <= 15; i++ {
		b.WriteString("    if (x > ")
		b.WriteString(strconv.Itoa(i))
		b.WriteString(") r += ")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("\n")
	}
	b.WriteString("    return r\n}\n")
	findings := runRuleByName(t, "CyclomaticComplexMethod", b.String())
	if len(findings) == 0 {
		t.Fatal("expected CyclomaticComplexMethod finding")
	}
}

func TestCyclomaticComplexMethod_Negative(t *testing.T) {
	// 13 if-expressions => complexity = 14 (equals threshold, not exceeded)
	var b strings.Builder
	b.WriteString("package test\nfun simple(x: Int): Int {\n    var r = 0\n")
	for i := 1; i <= 13; i++ {
		b.WriteString("    if (x > ")
		b.WriteString(strconv.Itoa(i))
		b.WriteString(") r += ")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("\n")
	}
	b.WriteString("    return r\n}\n")
	findings := runRuleByName(t, "CyclomaticComplexMethod", b.String())
	if len(findings) != 0 {
		t.Fatalf("expected no CyclomaticComplexMethod finding, got %d", len(findings))
	}
}

func TestCyclomaticComplexMethod_CountsElvisButNotSafeCall(t *testing.T) {
	build := func(use func(i int) string) string {
		var b strings.Builder
		b.WriteString("package test\nfun load(a: A?): Int {\n")
		for i := 1; i <= 19; i++ {
			b.WriteString("    val x" + strconv.Itoa(i) + " = " + use(i) + "\n")
		}
		b.WriteString("    return x1\n}\n")
		return b.String()
	}
	elvis := build(func(i int) string { return "a?.v" + strconv.Itoa(i) + " ?: error(\"missing\")" })
	if findings := runRuleByName(t, "CyclomaticComplexMethod", elvis); len(findings) != 1 {
		t.Fatalf("19 elvis operators give complexity 20, which detekt reports, got %d findings", len(findings))
	}
	safe := build(func(i int) string { return "a?.v" + strconv.Itoa(i) })
	if findings := runRuleByName(t, "CyclomaticComplexMethod", safe); len(findings) != 0 {
		t.Fatalf("safe calls are not decision points, got %d findings", len(findings))
	}
}

func TestCyclomaticComplexMethod_CompatibleSimpleWhenEntriesCount(t *testing.T) {
	// ignoreSimpleWhenEntries default is false, so each
	// simple when entry contributes 1 to cyclomatic complexity. A `when`
	// with 16 simple entries (and an else branch) on top of the base 1
	// pushes complexity above the default threshold of 14. Under the
	// previous default (true) the rule would have skipped these entries
	// entirely and produced no finding.
	var b strings.Builder
	b.WriteString("package test\nfun classify(x: Int): String = when (x) {\n")
	for i := 1; i <= 16; i++ {
		b.WriteString("    ")
		b.WriteString(strconv.Itoa(i))
		b.WriteString(" -> \"v")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("\"\n")
	}
	b.WriteString("    else -> \"other\"\n}\n")
	findings := runRuleByName(t, "CyclomaticComplexMethod", b.String())
	if len(findings) == 0 {
		t.Fatal("expected CyclomaticComplexMethod finding for 16 simple when entries")
	}
}

func TestCyclomaticComplexMethod_HonorsIgnoreSingleWhenExpression(t *testing.T) {
	// IgnoreSingleWhenExpression was previously a dead config — exposed
	// in metadata but never consulted. With it set to true, a function
	// whose entire body is a single when-expression is treated as
	// dispatch and skipped, even if it has many branches that would
	// otherwise pop the threshold.
	var rule *rules.CyclomaticComplexMethodRule
	for _, candidate := range api.Registry {
		if candidate.ID == "CyclomaticComplexMethod" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.CyclomaticComplexMethodRule)
			if !ok {
				t.Fatalf("expected CyclomaticComplexMethodRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("CyclomaticComplexMethod rule not registered")
	}
	original := rule.IgnoreSingleWhenExpression
	defer func() { rule.IgnoreSingleWhenExpression = original }()

	// Switch-style dispatch with 16 branches — over the limit by default.
	var b strings.Builder
	b.WriteString("package test\nfun classify(x: Int): String = when (x) {\n")
	for i := 1; i <= 16; i++ {
		b.WriteString("    ")
		b.WriteString(strconv.Itoa(i))
		b.WriteString(" -> \"v")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("\"\n")
	}
	b.WriteString("    else -> \"other\"\n}\n")
	code := b.String()

	rule.IgnoreSingleWhenExpression = true
	if findings := runRuleByName(t, "CyclomaticComplexMethod", code); len(findings) != 0 {
		t.Fatalf("expected no findings under IgnoreSingleWhenExpression=true, got %d", len(findings))
	}

	rule.IgnoreSingleWhenExpression = false
	if findings := runRuleByName(t, "CyclomaticComplexMethod", code); len(findings) == 0 {
		t.Fatal("expected finding for the same when-dispatch under IgnoreSingleWhenExpression=false")
	}

	// A function whose body is more than just a single when (e.g. a
	// guard `if` followed by `when`) is NOT treated as single-when
	// dispatch even with the flag on — the bypass should not apply.
	mixedCode := `package test
fun classify(x: Int): String {
    if (x < 0) return "negative"
    return when (x) {
        1 -> "one"
        2 -> "two"
        3 -> "three"
        4 -> "four"
        5 -> "five"
        6 -> "six"
        7 -> "seven"
        8 -> "eight"
        9 -> "nine"
        10 -> "ten"
        11 -> "eleven"
        12 -> "twelve"
        13 -> "thirteen"
        14 -> "fourteen"
        15 -> "fifteen"
        16 -> "sixteen"
        else -> "other"
    }
}
`
	rule.IgnoreSingleWhenExpression = true
	if findings := runRuleByName(t, "CyclomaticComplexMethod", mixedCode); len(findings) == 0 {
		t.Fatal("expected finding when body has more than just a when, even with IgnoreSingleWhenExpression=true")
	}
}

func TestCyclomaticComplexMethod_NestingFunctionsAddOnePerCallWithLambda(t *testing.T) {
	// detekt counts a call to a nesting function (also, apply, forEach, isNotNull, ifNull, let, run, use,
	// with by default) that has a lambda as +1, and counts the decisions inside every lambda.
	rule := cyclomaticRule(t)
	original := *rule
	defer func() { *rule = original }()

	// 1 + 7 forEach/let/run/with/apply/also/use calls with a lambda = 8, under the threshold of 14 ...
	var b strings.Builder
	b.WriteString("package test\nfun process(items: List<Int>, x: Int?): Int {\n  var r = 0\n")
	for _, name := range []string{"forEach", "let", "run", "with", "apply", "also", "use"} {
		b.WriteString("  items." + name + " { r += 1 }\n")
	}
	b.WriteString("  return r\n}\n")
	code := b.String()
	rule.AllowedComplexity = 7
	if findings := runRuleByName(t, "CyclomaticComplexMethod", code); len(findings) != 1 ||
		!strings.Contains(findings[0].Message, "(complexity: 8)") {
		t.Fatalf("seven nesting-function lambdas give complexity 8, got %+v", findings)
	}
	// ... and ignoreNestingFunctions removes the +1 per call.
	rule.IgnoreNestingFunctions = true
	if findings := runRuleByName(t, "CyclomaticComplexMethod", code); len(findings) != 0 {
		t.Fatalf("expected no findings under IgnoreNestingFunctions=true, got %+v", findings)
	}
	// A narrowed nestingFunctions list counts only the listed names.
	rule.IgnoreNestingFunctions = false
	rule.NestingFunctions = []string{"let"}
	if findings := runRuleByName(t, "CyclomaticComplexMethod", code); len(findings) != 0 {
		t.Fatalf("one listed call gives complexity 2, got %+v", findings)
	}

	// Decisions inside a lambda always count, whatever the callee.
	var d strings.Builder
	d.WriteString("package test\nfun process(items: List<Int>): Int {\n  var r = 0\n  items.map { item ->\n")
	for i := 1; i <= 14; i++ {
		d.WriteString("    if (item > " + strconv.Itoa(i) + ") r += " + strconv.Itoa(i) + "\n")
	}
	d.WriteString("  }\n  return r\n}\n")
	rule.AllowedComplexity = 14
	if findings := runRuleByName(t, "CyclomaticComplexMethod", d.String()); len(findings) != 1 {
		t.Fatalf("14 ifs inside a map lambda give complexity 15, got %d findings", len(findings))
	}
}

func TestCyclomaticComplexMethod_NestedFunctionsCountForTheOuterFunction(t *testing.T) {
	// detekt adds a nested function and its decisions to the outer function, and does not report the nested one.
	var b strings.Builder
	b.WriteString("package test\nfun outer() {\n  fun helper(x: Int): Int {\n    var r = 0\n")
	for i := 1; i <= 14; i++ {
		b.WriteString("    if (x > " + strconv.Itoa(i) + ") r += " + strconv.Itoa(i) + "\n")
	}
	b.WriteString("    return r\n  }\n  helper(1)\n}\n")
	findings := runRuleByName(t, "CyclomaticComplexMethod", b.String())
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "function outer") || !strings.Contains(findings[0].Message, "(complexity: 16)") {
		t.Fatalf("expected only outer with complexity 16, got %+v", findings)
	}
}

func TestCyclomaticComplexMethod_FunctionsInObjectLiteralsAreNotCounted(t *testing.T) {
	var b strings.Builder
	b.WriteString("package test\nfun outer() = object : Runnable {\n  override fun run() {\n")
	for i := 1; i <= 20; i++ {
		b.WriteString("    if (x > " + strconv.Itoa(i) + ") foo()\n")
	}
	b.WriteString("  }\n}\n")
	if findings := runRuleByName(t, "CyclomaticComplexMethod", b.String()); len(findings) != 0 {
		t.Fatalf("detekt skips functions inside object literals, got %+v", findings)
	}
}

// --- LongParameterList ---

func TestLongParameterList_Positive(t *testing.T) {
	code := `package test
fun send(a: Int, b: Int, c: Int, d: Int, e: Int, f: Int) {
}
`
	findings := runRuleByName(t, "LongParameterList", code)
	if len(findings) == 0 {
		t.Fatal("expected LongParameterList finding for 6 params (allowed 5)")
	}
}

func TestLongParameterList_Negative(t *testing.T) {
	code := `package test
fun send(a: Int, b: Int, c: Int, d: Int, e: Int) {
}
`
	findings := runRuleByName(t, "LongParameterList", code)
	if len(findings) != 0 {
		t.Fatalf("expected no LongParameterList finding for 5 params, got %d", len(findings))
	}
}

func TestLongParameterList_CompatibleConstructorBoundary(t *testing.T) {
	// Constructors are allowed 1 more parameter than functions (6 vs 5 by default).
	// Use plain (non-property) ctor params so the all-property short-circuit
	// (which fires when IgnoreDataClasses=true) does not apply.
	allowed := `package test
class Allowed(a: Int, b: Int, c: Int, d: Int, e: Int, f: Int)
`
	if findings := runRuleByName(t, "LongParameterList", allowed); len(findings) != 0 {
		t.Fatalf("expected no LongParameterList finding for 6 ctor params, got %d", len(findings))
	}
	flagged := `package test
class Flagged(a: Int, b: Int, c: Int, d: Int, e: Int, f: Int, g: Int)
`
	if findings := runRuleByName(t, "LongParameterList", flagged); len(findings) == 0 {
		t.Fatal("expected LongParameterList finding for 7 ctor params (allowed 6)")
	}
}

func TestLongParameterList_CompatibleDefaultParametersCount(t *testing.T) {
	// ignoreDefaultParameters=false by default, so parameters
	// with default values still count toward the limit.
	code := `package test
fun send(a: Int = 0, b: Int = 0, c: Int = 0, d: Int = 0, e: Int = 0, f: Int = 0) {
}
`
	findings := runRuleByName(t, "LongParameterList", code)
	if len(findings) == 0 {
		t.Fatal("expected LongParameterList finding for 6 default-valued params (allowed 5)")
	}
}

func TestLongParameterList_HonorsIgnoreAnnotatedParameter(t *testing.T) {
	// When ignoreAnnotatedParameter is configured, parameters carrying any
	// of the listed annotations are not counted toward the limit.
	var rule *rules.LongParameterListRule
	for _, candidate := range api.Registry {
		if candidate.ID == "LongParameterList" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.LongParameterListRule)
			if !ok {
				t.Fatalf("expected LongParameterListRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("LongParameterList rule not registered")
	}
	original := rule.IgnoreAnnotatedParameter
	defer func() { rule.IgnoreAnnotatedParameter = original }()

	rules.ApplyConfig(loadTempConfig(t, `
complexity:
  LongParameterList:
    ignoreAnnotatedParameter:
      - Composable
`))

	if len(rule.IgnoreAnnotatedParameter) == 0 {
		t.Fatalf("expected ApplyConfig to set IgnoreAnnotatedParameter, got %#v", rule.IgnoreAnnotatedParameter)
	}

	// Six params, but five carry @Composable — the count under the
	// configured ignore list is 1, well below the limit.
	allowed := `package test
annotation class Composable
fun screen(@Composable a: Int, @Composable b: Int, @Composable c: Int, @Composable d: Int, @Composable e: Int, f: Int) {
}
`
	if findings := runRuleByName(t, "LongParameterList", allowed); len(findings) != 0 {
		t.Fatalf("expected no findings when 5 of 6 params are annotation-ignored, got %d", len(findings))
	}

	// Same shape without the annotation should still flag.
	flagged := `package test
fun screen(a: Int, b: Int, c: Int, d: Int, e: Int, f: Int) {
}
`
	if findings := runRuleByName(t, "LongParameterList", flagged); len(findings) == 0 {
		t.Fatal("expected LongParameterList finding for 6 unannotated params")
	}
}

// --- TooManyFunctions ---

func TestTooManyFunctions_Positive(t *testing.T) {
	// 12 top-level functions exceeds threshold of 11
	var b strings.Builder
	b.WriteString("package test\n")
	for i := 1; i <= 12; i++ {
		b.WriteString("fun fn")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("() {}\n")
	}
	findings := runRuleByName(t, "TooManyFunctions", b.String())
	if len(findings) == 0 {
		t.Fatal("expected TooManyFunctions finding for 12 functions")
	}
}

func TestTooManyFunctions_Negative(t *testing.T) {
	// 10 top-level functions is below the threshold of 11, which detekt reports at
	var b strings.Builder
	b.WriteString("package test\n")
	for i := 1; i <= 10; i++ {
		b.WriteString("fun fn")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("() {}\n")
	}
	findings := runRuleByName(t, "TooManyFunctions", b.String())
	if len(findings) != 0 {
		t.Fatalf("expected no TooManyFunctions finding for 10 functions, got %d", len(findings))
	}
}

func TestTooManyFunctions_CompatibleOverriddenFunctionsCount(t *testing.T) {
	// ignoreOverridden default is false, so override
	// methods count toward the function total. Build a file with 12
	// top-level functions where every function uses `override` — under
	// the previous default (true) the rule skipped them all and emitted
	// no finding; under the new default it should fire.
	var b strings.Builder
	b.WriteString("package test\n")
	for i := 1; i <= 12; i++ {
		b.WriteString("override fun fn")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("() {}\n")
	}
	findings := runRuleByName(t, "TooManyFunctions", b.String())
	if len(findings) == 0 {
		t.Fatal("expected TooManyFunctions finding for 12 override functions")
	}
}

func TestTooManyFunctions_IgnoresNestedClassFunctionsForOuterClass(t *testing.T) {
	code := `package test
class Outer {
    fun a() {}
    fun b() {}
    class Inner {
        fun c() {}
        fun d() {}
        fun e() {}
        fun f() {}
        fun g() {}
        fun h() {}
        fun i() {}
        fun j() {}
        fun k() {}
        fun l() {}
    }
}
`
	findings := runRuleByName(t, "TooManyFunctions", code)
	if len(findings) != 0 {
		t.Fatalf("expected no TooManyFunctions finding for outer class with 2 direct functions, got %d", len(findings))
	}
}

func TestTooManyFunctions_FunInterfaceMembersNotCountedAsTopLevel(t *testing.T) {
	var b strings.Builder
	b.WriteString("package test\nfun interface Handler {\n    fun a(): Int\n")
	for i := 1; i <= 12; i++ {
		b.WriteString("    fun fn")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("() {}\n")
	}
	b.WriteString("}\n")
	for _, f := range runRuleByName(t, "TooManyFunctions", b.String()) {
		if strings.Contains(f.Message, "top-level") {
			t.Fatalf("unexpected top-level finding: %s", f.Message)
		}
	}
}

func TestTooManyFunctions_ClassMembersAreNotTopLevel(t *testing.T) {
	var b strings.Builder
	b.WriteString("package test\nclass Holder {\n")
	for i := 1; i <= 12; i++ {
		b.WriteString("    fun fn")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("() {}\n")
	}
	b.WriteString("}\n")
	findings := runRuleByName(t, "TooManyFunctions", b.String())
	if len(findings) != 1 || strings.Contains(findings[0].Message, "top-level") {
		t.Fatalf("expected one class-scope finding, got %v", findings)
	}
}

// --- LargeClass ---

func TestLargeClass_Positive(t *testing.T) {
	// Class with 602 lines exceeds 600 threshold
	var b strings.Builder
	b.WriteString("package test\nclass BigService {\n")
	for i := 1; i <= 600; i++ {
		b.WriteString("    val field")
		b.WriteString(strconv.Itoa(i))
		b.WriteString(" = ")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("\n")
	}
	b.WriteString("}\n")
	findings := runRuleByName(t, "LargeClass", b.String())
	if len(findings) == 0 {
		t.Fatal("expected LargeClass finding for 602-line class")
	}
}

func TestLargeClass_Negative(t *testing.T) {
	// Class with 599 lines (below threshold)
	var b strings.Builder
	b.WriteString("package test\nclass SmallService {\n")
	for i := 1; i <= 597; i++ {
		b.WriteString("    val field")
		b.WriteString(strconv.Itoa(i))
		b.WriteString(" = ")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("\n")
	}
	b.WriteString("}\n")
	findings := runRuleByName(t, "LargeClass", b.String())
	if len(findings) != 0 {
		t.Fatalf("expected no LargeClass finding for 599-line class, got %d", len(findings))
	}
}

// --- NestedBlockDepth ---

func TestNestedBlockDepth_Positive(t *testing.T) {
	// 5 levels of nesting exceeds threshold of 4
	code := `package test
fun deepNesting(x: Int) {
    if (x > 0) {
        if (x > 1) {
            if (x > 2) {
                if (x > 3) {
                    if (x > 4) {
                        println(x)
                    }
                }
            }
        }
    }
}
`
	findings := runRuleByName(t, "NestedBlockDepth", code)
	if len(findings) == 0 {
		t.Fatal("expected NestedBlockDepth finding for depth 5")
	}
}

func TestNestedBlockDepth_FindingAnchoredAtFunctionLine(t *testing.T) {
	code := `package test
fun deepNesting(x: Int) {
    if (x > 0) {
        if (x > 1) {
            if (x > 2) {
                if (x > 3) {
                    if (x > 4) {
                        println(x)
                    }
                }
            }
        }
    }
}
`
	findings := runRuleByName(t, "NestedBlockDepth", code)
	if len(findings) != 1 || findings[0].Line != 2 {
		t.Fatalf("expected one finding on the function line (2), got %v", findings)
	}
	suppressed := strings.Replace(code, "fun deepNesting", "@Suppress(\"NestedBlockDepth\")\nfun deepNesting", 1)
	if findings := runRuleByName(t, "NestedBlockDepth", suppressed); len(findings) != 0 {
		t.Fatalf("expected @Suppress on the function to silence the finding, got %d", len(findings))
	}
}

func TestNestedBlockDepth_Negative(t *testing.T) {
	// 3 levels of nesting is below the threshold of 4
	code := `package test
fun shallowNesting(x: Int) {
    if (x > 0) {
        if (x > 1) {
            if (x > 2) {
                println(x)
            }
        }
    }
}
`
	findings := runRuleByName(t, "NestedBlockDepth", code)
	if len(findings) != 0 {
		t.Fatalf("expected no NestedBlockDepth finding for depth 3, got %d", len(findings))
	}
	// detekt reports at depth >= threshold, so 4 levels is reported.
	deeper := strings.Replace(code, "                println(x)\n", "                if (x > 3) {\n                    println(x)\n                }\n", 1)
	if findings := runRuleByName(t, "NestedBlockDepth", deeper); len(findings) != 1 ||
		findings[0].Message != "Function shallowNesting is nested too deeply." {
		t.Fatalf("expected one finding for depth 4, got %+v", findings)
	}
}

func TestNestedBlockDepth_ElseIfChainDoesNotIncreaseDepth(t *testing.T) {
	code := `package test
fun chain(x: Int) {
    if (x > 0) {
        println(x)
    } else if (x < 0) {
        if (x < -1) {
            if (x < -2) {
                println(x)
            }
        }
    }
}
`
	findings := runRuleByName(t, "NestedBlockDepth", code)
	if len(findings) != 0 {
		t.Fatalf("expected no NestedBlockDepth finding for else-if chain depth, got %d", len(findings))
	}
}

func TestNestedBlockDepth_ElseIfChainWithCommentsDoesNotIncreaseDepth(t *testing.T) {
	// Regression: comments and blank lines between `else` and `if` must not
	// defeat else-if-chain detection. The sibling-order check in
	// isElseIfChainNodeFlat handles this; a byte-offset check would too,
	// but extras like comments make offset-based reasoning fragile.
	code := `package test
fun chain(x: Int) {
    if (x > 0) {
        println(x)
    } else
        // explain the next branch
        /* and another note */
        if (x < 0) {
        if (x < -1) {
            if (x < -2) {
                println(x)
            }
        }
    }
}
`
	findings := runRuleByName(t, "NestedBlockDepth", code)
	if len(findings) != 0 {
		t.Fatalf("expected no NestedBlockDepth finding for else-if chain with comments, got %d", len(findings))
	}
}

// --- CognitiveComplexMethod ---

func TestCognitiveComplexMethod_Positive(t *testing.T) {
	// Each top-level if adds 1 (1+0 nesting). 15 top-level ifs => complexity = 15, which reaches the threshold
	var b strings.Builder
	b.WriteString("package test\nfun cognitive(x: Int): Int {\n    var r = 0\n")
	for i := 1; i <= 15; i++ {
		b.WriteString("    if (x > ")
		b.WriteString(strconv.Itoa(i))
		b.WriteString(") r += ")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("\n")
	}
	b.WriteString("    return r\n}\n")
	findings := runRuleByName(t, "CognitiveComplexMethod", b.String())
	if len(findings) == 0 {
		t.Fatal("expected CognitiveComplexMethod finding")
	}
}

func TestCognitiveComplexMethod_Negative(t *testing.T) {
	// 14 top-level ifs => complexity = 14, below the threshold of 15
	var b strings.Builder
	b.WriteString("package test\nfun cognitive(x: Int): Int {\n    var r = 0\n")
	for i := 1; i <= 14; i++ {
		b.WriteString("    if (x > ")
		b.WriteString(strconv.Itoa(i))
		b.WriteString(") r += ")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("\n")
	}
	b.WriteString("    return r\n}\n")
	findings := runRuleByName(t, "CognitiveComplexMethod", b.String())
	if len(findings) != 0 {
		t.Fatalf("expected no CognitiveComplexMethod finding, got %d", len(findings))
	}
}

// --- ComplexCondition ---

func TestComplexCondition_Positive(t *testing.T) {
	code := `package test
fun check(a: Boolean, b: Boolean, c: Boolean, d: Boolean, e: Boolean) {
    if (a && b || c && d || e) {
        println("complex")
    }
}
`
	findings := runRuleByName(t, "ComplexCondition", code)
	if len(findings) == 0 {
		t.Fatal("expected ComplexCondition finding for condition with >3 logical operators")
	}
}

func TestComplexCondition_Negative(t *testing.T) {
	code := `package test
fun check(a: Boolean, b: Boolean, c: Boolean) {
    if (a && b || c) {
        println("simple")
    }
}
`
	findings := runRuleByName(t, "ComplexCondition", code)
	if len(findings) != 0 {
		t.Fatalf("expected no ComplexCondition finding for 3 logical operators, got %d", len(findings))
	}
}

func TestComplexCondition_LambdaOperatorsCountLikeDetekt(t *testing.T) {
	code := `package test
fun check(a: Boolean, items: List<Int>) {
    if (a && items.any { it > 0 && it < 10 && it != 5 && it != 7 || it == 42 }) {
        println("nested lambda operators must not count toward outer condition")
    }
}
`
	findings := runRuleByName(t, "ComplexCondition", code)
	if len(findings) != 1 || findings[0].Message != "This condition is too complex (6). Defined complexity threshold for conditions is set to '4'" {
		t.Fatalf("detekt counts the operators in the text of the lambda, got %v", findings)
	}
	// Three operators make four conditions, which reaches the threshold; two do not.
	if got := runRuleByName(t, "ComplexCondition", "package test\nfun f(a: Boolean, b: Boolean, c: Boolean, d: Boolean) {\n    if (a && b && c && d) { println() }\n}\n"); len(got) != 1 {
		t.Fatalf("a && b && c && d must be reported, got %v", got)
	}
	if got := runRuleByName(t, "ComplexCondition", "package test\nfun f(a: Boolean, b: Boolean, c: Boolean) {\n    if (a && b && c) { println() }\n}\n"); len(got) != 0 {
		t.Fatalf("a && b && c must not be reported, got %v", got)
	}
}

func TestComplexCondition_NestedFunctionOperatorsDoNotInflateOuter(t *testing.T) {
	code := `package test
fun check(a: Boolean, b: Boolean) {
    if (a || b) {
        fun nested(x: Int, y: Int, z: Int): Boolean {
            return x > 0 && y > 0 && z > 0 && x < y && y < z
        }
        println(nested(1, 2, 3))
    }
}
`
	findings := runRuleByName(t, "ComplexCondition", code)
	if len(findings) != 0 {
		t.Fatalf("expected no ComplexCondition finding when operators live in nested function, got %d", len(findings))
	}
}

// --- ComplexInterface ---

func TestComplexInterface_Positive(t *testing.T) {
	var b strings.Builder
	b.WriteString("package test\ninterface BigApi {\n")
	for i := 1; i <= 11; i++ {
		b.WriteString("    fun method")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("()\n")
	}
	b.WriteString("}\n")
	findings := runRuleByName(t, "ComplexInterface", b.String())
	if len(findings) == 0 {
		t.Fatal("expected ComplexInterface finding for interface with 11 methods (allowed 10)")
	}
}

func TestComplexInterface_Negative(t *testing.T) {
	var b strings.Builder
	b.WriteString("package test\ninterface SmallApi {\n")
	for i := 1; i <= 9; i++ {
		b.WriteString("    fun method")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("()\n")
	}
	b.WriteString("}\n")
	findings := runRuleByName(t, "ComplexInterface", b.String())
	if len(findings) != 0 {
		t.Fatalf("expected no ComplexInterface finding for 9 members, got %d", len(findings))
	}
	// detekt reports at members >= threshold (10).
	b.Reset()
	b.WriteString("package test\ninterface BigApi {\n    val p: Int\n")
	for i := 1; i <= 9; i++ {
		b.WriteString("    fun method" + strconv.Itoa(i) + "()\n")
	}
	b.WriteString("}\n")
	findings = runRuleByName(t, "ComplexInterface", b.String())
	if len(findings) != 1 || findings[0].Message != "The interface BigApi is too complex. Consider splitting it up." {
		t.Fatalf("expected one finding for 10 members, got %v", findings)
	}
}

func TestComplexInterface_NestedClassMembersIgnored(t *testing.T) {
	code := `package test
interface Api {
    fun method1()
    fun method2()
    fun method3()
    fun method4()
    fun method5()
    fun method6()
    fun method7()
    fun method8()
    fun method9()

    class Nested {
        fun helper1() {}
        fun helper2() {}
        fun helper3() {}
    }
}
`
	findings := runRuleByName(t, "ComplexInterface", code)
	if len(findings) != 0 {
		t.Fatalf("expected no ComplexInterface finding when only nested class members exceed the threshold, got %d", len(findings))
	}
}

func TestComplexInterface_IgnoresNestedClassMembers(t *testing.T) {
	code := `package test
interface Api {
    fun a()
    fun b()
    fun c()
    fun d()
    fun e()
    fun f()
    fun g()
    fun h()
    fun i()
    class Nested {
        fun nested1() {}
        fun nested2() {}
    }
}
`
	findings := runRuleByName(t, "ComplexInterface", code)
	if len(findings) != 0 {
		t.Fatalf("expected no ComplexInterface finding when only nested class adds members, got %d", len(findings))
	}
}

func TestComplexInterface_HonorsIncludePrivateDeclarations(t *testing.T) {
	// IncludePrivateDeclarations was previously a dead config — exposed
	// in metadata but never consulted by the check. By default (false),
	// private interface members shouldn't count toward the limit. Configure
	// the flag both ways and verify behavior.
	var rule *rules.ComplexInterfaceRule
	for _, candidate := range api.Registry {
		if candidate.ID == "ComplexInterface" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.ComplexInterfaceRule)
			if !ok {
				t.Fatalf("expected ComplexInterfaceRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("ComplexInterface rule not registered")
	}
	original := rule.IncludePrivateDeclarations
	defer func() { rule.IncludePrivateDeclarations = original }()

	// Interface with 5 public + 6 private direct members. Default-active
	// threshold is 10. Under IncludePrivate=false: count is 5 → no finding.
	code := `package test
interface Api {
    fun a()
    fun b()
    fun c()
    fun d()
    fun e()
    private fun helper1() {}
    private fun helper2() {}
    private fun helper3() {}
    private fun helper4() {}
    private fun helper5() {}
    private fun helper6() {}
}
`
	if findings := runRuleByName(t, "ComplexInterface", code); len(findings) != 0 {
		t.Fatalf("expected no findings under IncludePrivateDeclarations=false, got %d", len(findings))
	}

	// Flip to true: count is 11 → finding.
	rule.IncludePrivateDeclarations = true
	if findings := runRuleByName(t, "ComplexInterface", code); len(findings) == 0 {
		t.Fatal("expected finding under IncludePrivateDeclarations=true (11 members > 10 limit)")
	}
}

func TestComplexInterface_HonorsIncludeStaticDeclarations(t *testing.T) {
	// IncludeStaticDeclarations was previously a dead config — when
	// false (default), companion-object members aren't counted; when
	// true, they are.
	var rule *rules.ComplexInterfaceRule
	for _, candidate := range api.Registry {
		if candidate.ID == "ComplexInterface" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.ComplexInterfaceRule)
			if !ok {
				t.Fatalf("expected ComplexInterfaceRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("ComplexInterface rule not registered")
	}
	original := rule.IncludeStaticDeclarations
	defer func() { rule.IncludeStaticDeclarations = original }()

	// Interface with 5 direct + 6 companion members.
	code := `package test
interface Api {
    fun a()
    fun b()
    fun c()
    fun d()
    fun e()
    companion object {
        fun s1() {}
        fun s2() {}
        fun s3() {}
        fun s4() {}
        fun s5() {}
        fun s6() {}
    }
}
`
	// Default (false): 5 direct members, no finding.
	if findings := runRuleByName(t, "ComplexInterface", code); len(findings) != 0 {
		t.Fatalf("expected no findings under IncludeStaticDeclarations=false, got %d", len(findings))
	}

	// Flip to true: 5 + 6 = 11 → finding.
	rule.IncludeStaticDeclarations = true
	if findings := runRuleByName(t, "ComplexInterface", code); len(findings) == 0 {
		t.Fatal("expected finding under IncludeStaticDeclarations=true (5 direct + 6 companion = 11 members > 10)")
	}
}

// --- LabeledExpression ---

func TestLabeledExpression_Positive(t *testing.T) {
	code := `package test
fun process(items: List<List<Int>>) {
    for (list in items) {
        for (item in list) {
            if (item == 0) break@process
        }
    }
}
`
	findings := runRuleByName(t, "LabeledExpression", code)
	if len(findings) == 0 {
		t.Fatal("expected LabeledExpression finding for break@process")
	}
}

func TestLabeledExpression_Negative(t *testing.T) {
	code := `package test
fun process(items: List<Int>) {
    for (item in items) {
        if (item == 0) break
    }
}
`
	findings := runRuleByName(t, "LabeledExpression", code)
	if len(findings) != 0 {
		t.Fatalf("expected no LabeledExpression finding without labels, got %d", len(findings))
	}
}

func TestLabeledExpression_HonorsIgnoredLabels(t *testing.T) {
	// IgnoredLabels was previously a dead config — exposed in metadata but
	// never consulted by the check. Configure it via the rule pointer and
	// verify that labels matching the ignore list produce no findings.
	var rule *rules.LabeledExpressionRule
	for _, candidate := range api.Registry {
		if candidate.ID == "LabeledExpression" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.LabeledExpressionRule)
			if !ok {
				t.Fatalf("expected LabeledExpressionRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("LabeledExpression rule not registered")
	}
	original := rule.IgnoredLabels
	defer func() { rule.IgnoredLabels = original }()

	rule.IgnoredLabels = []string{"forEach"}

	code := `package test
fun process(items: List<Int>) {
    items.forEach forEach@{ item ->
        if (item == 0) return@forEach
    }
}
`
	findings := runRuleByName(t, "LabeledExpression", code)
	if len(findings) != 0 {
		t.Fatalf("expected no LabeledExpression findings when 'forEach' is ignored, got %d", len(findings))
	}

	// A non-ignored label still fires even when other labels are ignored.
	codeOther := `package test
fun process(items: List<List<Int>>) {
    process@ for (list in items) {
        for (item in list) {
            if (item == 0) break@process
        }
    }
}
`
	if got := runRuleByName(t, "LabeledExpression", codeOther); len(got) == 0 {
		t.Fatal("expected LabeledExpression to still fire on non-ignored 'process' label")
	}
}

// --- MethodOverloading ---

func TestMethodOverloading_Positive(t *testing.T) {
	var b strings.Builder
	b.WriteString("package test\n")
	for i := 0; i <= 6; i++ {
		b.WriteString("fun process(")
		for j := 0; j < i; j++ {
			if j > 0 {
				b.WriteString(", ")
			}
			b.WriteString("a")
			b.WriteString(strconv.Itoa(j))
			b.WriteString(": Int")
		}
		b.WriteString(") {}\n")
	}
	findings := runRuleByName(t, "MethodOverloading", b.String())
	if len(findings) == 0 {
		t.Fatal("expected MethodOverloading finding for 7 overloads (allowed 6)")
	}
}

func TestMethodOverloading_Negative(t *testing.T) {
	var b strings.Builder
	b.WriteString("package test\n")
	for i := 0; i < 5; i++ {
		b.WriteString("fun process(")
		for j := 0; j < i; j++ {
			if j > 0 {
				b.WriteString(", ")
			}
			b.WriteString("a")
			b.WriteString(strconv.Itoa(j))
			b.WriteString(": Int")
		}
		b.WriteString(") {}\n")
	}
	findings := runRuleByName(t, "MethodOverloading", b.String())
	if len(findings) != 0 {
		t.Fatalf("expected no MethodOverloading finding for 5 overloads, got %d", len(findings))
	}
	// detekt reports at overloads >= threshold (6), with this message.
	b.WriteString("fun process(z: String) {}\n")
	findings = runRuleByName(t, "MethodOverloading", b.String())
	if len(findings) != 1 || findings[0].Message != "The method 'process' is overloaded 6 times." {
		t.Fatalf("expected one finding for 6 overloads, got %v", findings)
	}
	// Extension functions are keyed by receiver, so they are not overloads of each other.
	ext := "package test\nfun Int.d() {}\nfun String.d() {}\nfun Long.d() {}\nfun Float.d() {}\nfun Double.d() {}\nfun Char.d() {}\n"
	if got := runRuleByName(t, "MethodOverloading", ext); len(got) != 0 {
		t.Fatalf("extensions on different receivers are not overloads, got %v", got)
	}
}

func TestMethodOverloading_NestedClassesAreSeparateScopes(t *testing.T) {
	var b strings.Builder
	b.WriteString("package test\n")
	b.WriteString("class Outer {\n")
	for i := 0; i < 7; i++ {
		b.WriteString("    fun process(")
		for j := 0; j < i; j++ {
			if j > 0 {
				b.WriteString(", ")
			}
			b.WriteString("a")
			b.WriteString(strconv.Itoa(j))
			b.WriteString(": Int")
		}
		b.WriteString(") {}\n")
	}
	b.WriteString("    class Inner {\n")
	for i := 0; i < 7; i++ {
		b.WriteString("        fun process(")
		for j := 0; j < i; j++ {
			if j > 0 {
				b.WriteString(", ")
			}
			b.WriteString("b")
			b.WriteString(strconv.Itoa(j))
			b.WriteString(": Int")
		}
		b.WriteString(") {}\n")
	}
	b.WriteString("    }\n")
	b.WriteString("}\n")
	findings := runRuleByName(t, "MethodOverloading", b.String())
	if len(findings) != 2 {
		t.Fatalf("expected 2 MethodOverloading findings for separate outer and inner class scopes, got %d", len(findings))
	}
}

func TestMethodOverloading_IgnoresNestedClassOverloads(t *testing.T) {
	code := `package test
class Outer {
    fun process() {}
    fun process(a: Int) {}
    class Inner {
        fun process(a: String) {}
        fun process(a: String, b: String) {}
        fun process(a: String, b: String, c: String) {}
        fun process(a: String, b: String, c: String, d: String) {}
        fun process(a: String, b: String, c: String, d: String, e: String) {}
        fun process(a: String, b: String, c: String, d: String, e: String, f: String) {}
    }
}
`
	findings := runRuleByName(t, "MethodOverloading", code)
	if len(findings) != 1 || findings[0].Line != 5 {
		t.Fatalf("expected only Inner (line 5) to be reported, not Outer, got %v", findings)
	}
}

// --- NestedScopeFunctions ---

func TestNestedScopeFunctions_Positive(t *testing.T) {
	code := `package test
fun doStuff(value: String?) {
    value?.let {
        it.run {
            it.also {
                println(this)
            }
        }
    }
}
`
	findings := runRuleByName(t, "NestedScopeFunctions", code)
	if len(findings) == 0 {
		t.Fatal("expected NestedScopeFunctions finding for triple-nested scope functions")
	}
}

func TestNestedScopeFunctions_Negative(t *testing.T) {
	code := `package test
fun doStuff(value: String?) {
    value?.let {
        println(it)
    }
}
`
	findings := runRuleByName(t, "NestedScopeFunctions", code)
	if len(findings) != 0 {
		t.Fatalf("expected no NestedScopeFunctions finding for single scope function, got %d", len(findings))
	}
}

func TestNestedScopeFunctions_HonorsFunctions(t *testing.T) {
	// Functions was previously a dead config — exposed in metadata
	// but never consulted. With it set to a custom list, only the
	// configured callees count toward the depth limit.
	var rule *rules.NestedScopeFunctionsRule
	for _, candidate := range api.Registry {
		if candidate.ID == "NestedScopeFunctions" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.NestedScopeFunctionsRule)
			if !ok {
				t.Fatalf("expected NestedScopeFunctionsRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("NestedScopeFunctions rule not registered")
	}
	original := rule.Functions
	defer func() { rule.Functions = original }()

	// Project-specific helper that nests like apply/run.
	rule.Functions = []string{"configure"}

	customCode := `package test
fun build(value: String?) {
    value?.configure {
        it.configure {
            it.configure {
                println(it)
            }
        }
    }
}
`
	if findings := runRuleByName(t, "NestedScopeFunctions", customCode); len(findings) == 0 {
		t.Fatal("expected finding for custom scope function 'configure' nested 3 deep")
	}

	// With the custom list active, the standard scope functions should
	// no longer be considered nested.
	stdScopeCode := `package test
fun doStuff(value: String?) {
    value?.let {
        it.run {
            it.also {
                println(this)
            }
        }
    }
}
`
	if findings := runRuleByName(t, "NestedScopeFunctions", stdScopeCode); len(findings) != 0 {
		t.Fatalf("expected no findings under custom Functions list with standard scope functions, got %d", len(findings))
	}
}

// --- ReplaceSafeCallChainWithRun ---

func TestReplaceSafeCallChainWithRun_Positive(t *testing.T) {
	code := `package test
fun example(a: A?) {
    val x = a?.b?.c?.d
}
`
	findings := runRuleByName(t, "ReplaceSafeCallChainWithRun", code)
	if len(findings) == 0 {
		t.Fatal("expected ReplaceSafeCallChainWithRun finding for 3 chained safe calls")
	}
}

func TestReplaceSafeCallChainWithRun_Negative(t *testing.T) {
	code := `package test
fun example(a: A?) {
    val x = a?.b?.c
}
`
	findings := runRuleByName(t, "ReplaceSafeCallChainWithRun", code)
	if len(findings) != 0 {
		t.Fatalf("expected no ReplaceSafeCallChainWithRun finding for 2 safe calls, got %d", len(findings))
	}
}

// --- StringLiteralDuplication ---

func TestStringLiteralDuplication_Positive(t *testing.T) {
	code := `package test
fun example() {
    val a = "duplicated string"
    val b = "duplicated string"
    val c = "duplicated string"
}
`
	findings := runRuleByName(t, "StringLiteralDuplication", code)
	if len(findings) == 0 {
		t.Fatal("expected StringLiteralDuplication finding for string repeated 3 times (allowed 2)")
	}
}

func TestStringLiteralDuplication_Negative(t *testing.T) {
	code := `package test
fun example() {
    val a = "some string"
    val b = "some string"
    val c = "other string"
}
`
	findings := runRuleByName(t, "StringLiteralDuplication", code)
	if len(findings) != 0 {
		t.Fatalf("expected no StringLiteralDuplication finding for 2 duplicates, got %d", len(findings))
	}
}

func TestStringLiteralDuplication_HonorsAllowedWithLengthLessThan(t *testing.T) {
	// AllowedWithLengthLessThan was previously a dead config (the check
	// hardcoded a `len(text) <= 3` skip on the quoted text). With the
	// default of 5, short literals are excluded by content length and don't
	// contribute to the duplication count.
	codeShort := `package test
fun example() {
    val a = "x"
    val b = "x"
    val c = "x"
    val d = "x"
}
`
	if findings := runRuleByName(t, "StringLiteralDuplication", codeShort); len(findings) != 0 {
		t.Fatalf("expected no findings for short '\"x\"' literals, got %d", len(findings))
	}

	// Right at the boundary: "abcd" content length 4 — under default of 5,
	// should still be ignored.
	codeBoundary := `package test
fun example() {
    val a = "abcd"
    val b = "abcd"
    val c = "abcd"
}
`
	if findings := runRuleByName(t, "StringLiteralDuplication", codeBoundary); len(findings) != 0 {
		t.Fatalf("expected no findings for 4-char literals under default threshold, got %d", len(findings))
	}

	// One past the boundary: "abcde" content length 5 — counted.
	codeFlagged := `package test
fun example() {
    val a = "abcde"
    val b = "abcde"
    val c = "abcde"
}
`
	if findings := runRuleByName(t, "StringLiteralDuplication", codeFlagged); len(findings) == 0 {
		t.Fatal("expected finding for 5-char literal repeated above threshold")
	}

	// Configured smaller cutoff — short literals now count.
	var rule *rules.StringLiteralDuplicationRule
	for _, candidate := range api.Registry {
		if candidate.ID == "StringLiteralDuplication" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.StringLiteralDuplicationRule)
			if !ok {
				t.Fatalf("expected StringLiteralDuplicationRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("StringLiteralDuplication rule not registered")
	}
	original := rule.AllowedWithLengthLessThan
	defer func() { rule.AllowedWithLengthLessThan = original }()
	rule.AllowedWithLengthLessThan = 0

	if findings := runRuleByName(t, "StringLiteralDuplication", codeShort); len(findings) == 0 {
		t.Fatal("expected finding for short literals when allowedWithLengthLessThan disabled")
	}
}

func TestStringLiteralDuplication_HonorsIgnoreAnnotation(t *testing.T) {
	// IgnoreAnnotation was previously a dead config — exposed in metadata but
	// never consulted by the check. With the wiring, repeat string literals
	// living inside annotation arguments (like @Suppress("UNCHECKED_CAST"))
	// are excluded from the duplication count.
	var rule *rules.StringLiteralDuplicationRule
	for _, candidate := range api.Registry {
		if candidate.ID == "StringLiteralDuplication" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.StringLiteralDuplicationRule)
			if !ok {
				t.Fatalf("expected StringLiteralDuplicationRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("StringLiteralDuplication rule not registered")
	}
	original := rule.IgnoreAnnotation
	defer func() { rule.IgnoreAnnotation = original }()

	rule.IgnoreAnnotation = true

	codeAnnotated := `package test
@Suppress("UNCHECKED_CAST")
class A
@Suppress("UNCHECKED_CAST")
class B
@Suppress("UNCHECKED_CAST")
class C
@Suppress("UNCHECKED_CAST")
class D
`
	if findings := runRuleByName(t, "StringLiteralDuplication", codeAnnotated); len(findings) != 0 {
		t.Fatalf("expected no findings when IgnoreAnnotation=true and duplicates live in annotations, got %d", len(findings))
	}

	// Sanity check: same literal repeated outside annotations still fires.
	codeNonAnnotated := `package test
fun example() {
    val a = "duplicated"
    val b = "duplicated"
    val c = "duplicated"
}
`
	if findings := runRuleByName(t, "StringLiteralDuplication", codeNonAnnotated); len(findings) == 0 {
		t.Fatal("expected finding for non-annotation duplicates even with IgnoreAnnotation=true")
	}
}

func TestStringLiteralDuplication_HonorsIgnoreStringsRegex(t *testing.T) {
	// IgnoreStringsRegex was previously a dead config. Configure it via the
	// rule pointer and verify that string literals matching the regex are
	// excluded from the duplication count.
	var rule *rules.StringLiteralDuplicationRule
	for _, candidate := range api.Registry {
		if candidate.ID == "StringLiteralDuplication" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.StringLiteralDuplicationRule)
			if !ok {
				t.Fatalf("expected StringLiteralDuplicationRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("StringLiteralDuplication rule not registered")
	}
	original := rule.IgnoreStringsRegex
	defer func() { rule.IgnoreStringsRegex = original }()

	rule.IgnoreStringsRegex = `https?://.*` // detekt matches the whole literal

	// URL-shaped literals match the ignore regex and should not contribute
	// to the duplication count, so the file produces no finding even though
	// the URL appears four times.
	codeIgnored := `package test
fun example() {
    val a = "http://example.com/a"
    val b = "http://example.com/a"
    val c = "http://example.com/a"
    val d = "http://example.com/a"
}
`
	if findings := runRuleByName(t, "StringLiteralDuplication", codeIgnored); len(findings) != 0 {
		t.Fatalf("expected no findings when URL literals match ignoreStringsRegex, got %d", len(findings))
	}

	// A non-matching literal repeated past the threshold still fires.
	codeFlagged := `package test
fun example() {
    val a = "duplicated string"
    val b = "duplicated string"
    val c = "duplicated string"
}
`
	if findings := runRuleByName(t, "StringLiteralDuplication", codeFlagged); len(findings) == 0 {
		t.Fatal("expected StringLiteralDuplication to still fire on non-matching duplicate")
	}
}

func parseBenchmarkFile(b *testing.B, code string) *scanner.File {
	b.Helper()
	dir := b.TempDir()
	path := filepath.Join(dir, "bench.kt")
	if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
		b.Fatalf("write benchmark file: %v", err)
	}
	file, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		b.Fatalf("ParseFile(%s): %v", path, err)
	}
	return file
}

func complexityBenchmarkDispatcher() *rules.Dispatcher {
	var selected []*api.Rule
	for _, r := range api.Registry {
		switch r.ID {
		case "LongParameterList", "NestedBlockDepth", "CyclomaticComplexMethod", "CognitiveComplexMethod", "ComplexCondition":
			selected = append(selected, r)
		}
	}
	return rules.NewDispatcher(selected, nil)
}

func BenchmarkComplexityRules_HeavyClassAndFunction(b *testing.B) {
	const code = `package test
class Heavy(
    val p1: Int,
    val p2: Int,
    val p3: Int,
    val p4: Int,
    val p5: Int,
    val p6: Int,
    val p7: Int
) {
    fun complex(x: Int, y: Int): Int {
        var result = 0
        if (x > 0 && y > 0) {
            if (x > 1 || y > 1) {
                if (x > 2 && y > 2) {
                    if (x > 3 || y > 3) {
                        if (x > 4 && y > 4) {
                            if (x > 5 || y > 5) {
                                result += x
                            }
                        }
                    }
                }
            }
        }
        return result
    }
}
`
	file := parseBenchmarkFile(b, code)
	d := complexityBenchmarkDispatcher()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = d.Run(file)
	}
}

func BenchmarkCyclomaticComplexMethod_EarlyExit(b *testing.B) {
	var src strings.Builder
	src.WriteString("package test\nfun complex(x: Int): Int {\n    var r = 0\n")
	for i := 0; i < 300; i++ {
		src.WriteString("    if (x > ")
		src.WriteString(strconv.Itoa(i))
		src.WriteString(") r += ")
		src.WriteString(strconv.Itoa(i))
		src.WriteString("\n")
	}
	src.WriteString("    return r\n}\n")

	file := parseBenchmarkFile(b, src.String())
	var rule *api.Rule
	for _, r := range api.Registry {
		if r.ID == "CyclomaticComplexMethod" {
			rule = r
			break
		}
	}
	if rule == nil {
		b.Fatal("CyclomaticComplexMethod rule not found")
	}
	d := rules.NewDispatcher([]*api.Rule{rule}, nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = d.Run(file)
	}
}

func BenchmarkNestedBlockDepth_EarlyExit(b *testing.B) {
	var src strings.Builder
	src.WriteString("package test\nfun deep(x: Int) {\n")
	for i := 0; i < 200; i++ {
		src.WriteString(strings.Repeat("    ", i+1))
		src.WriteString("if (x > ")
		src.WriteString(strconv.Itoa(i))
		src.WriteString(") {\n")
	}
	src.WriteString(strings.Repeat("    ", 201))
	src.WriteString("println(x)\n")
	for i := 0; i < 200; i++ {
		src.WriteString(strings.Repeat("    ", 200-i))
		src.WriteString("}\n")
	}
	src.WriteString("}\n")

	file := parseBenchmarkFile(b, src.String())
	var rule *api.Rule
	for _, r := range api.Registry {
		if r.ID == "NestedBlockDepth" {
			rule = r
			break
		}
	}
	if rule == nil {
		b.Fatal("NestedBlockDepth rule not found")
	}
	d := rules.NewDispatcher([]*api.Rule{rule}, nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = d.Run(file)
	}
}

func BenchmarkNestedBlockDepth_ElseIfChain(b *testing.B) {
	var src strings.Builder
	src.WriteString("package test\nfun branchy(x: Int) {\n")
	for i := 0; i < 120; i++ {
		if i == 0 {
			src.WriteString("    if (x == ")
		} else {
			src.WriteString("    else if (x == ")
		}
		src.WriteString(strconv.Itoa(i))
		src.WriteString(") { println(x) }\n")
	}
	src.WriteString("    else { println(x) }\n")
	src.WriteString("}\n")

	file := parseBenchmarkFile(b, src.String())
	var rule *api.Rule
	for _, r := range api.Registry {
		if r.ID == "NestedBlockDepth" {
			rule = r
			break
		}
	}
	if rule == nil {
		b.Fatal("NestedBlockDepth rule not found")
	}
	d := rules.NewDispatcher([]*api.Rule{rule}, nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = d.Run(file)
	}
}

func BenchmarkTooManyFunctions_HeavyFile(b *testing.B) {
	var src strings.Builder
	src.WriteString("package test\n")
	for i := 0; i < 80; i++ {
		src.WriteString("fun top")
		src.WriteString(strconv.Itoa(i))
		src.WriteString("() {}\n")
	}
	src.WriteString("class Outer {\n")
	for i := 0; i < 40; i++ {
		src.WriteString("    fun inner")
		src.WriteString(strconv.Itoa(i))
		src.WriteString("() {}\n")
	}
	src.WriteString("    class Nested {\n")
	for i := 0; i < 20; i++ {
		src.WriteString("        fun nested")
		src.WriteString(strconv.Itoa(i))
		src.WriteString("() {}\n")
	}
	src.WriteString("    }\n}\n")

	file := parseBenchmarkFile(b, src.String())
	var rule *api.Rule
	for _, r := range api.Registry {
		if r.ID == "TooManyFunctions" {
			rule = r
			break
		}
	}
	if rule == nil {
		b.Fatal("TooManyFunctions rule not found")
	}
	d := rules.NewDispatcher([]*api.Rule{rule}, nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = d.Run(file)
	}
}

func TestNamedArguments_Positive(t *testing.T) {
	findings := runRuleByName(t, "NamedArguments", `
package test
fun example() {
    createUser("Alice", 30, "admin", "active")
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for 4 unnamed arguments (allowed 3)")
	}
}

func TestNamedArguments_Negative(t *testing.T) {
	findings := runRuleByName(t, "NamedArguments", `
package test
fun example() {
    createUser("Alice", 30, "admin")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for 3 unnamed args (at threshold), got %d", len(findings))
	}
}

func TestNamedArguments_IgnoresGradleAndTestSources(t *testing.T) {
	code := `
package test
fun example() {
    createUser("Alice", 30, "admin", "active")
}
`
	for _, path := range []string{"build.gradle.kts", "src/test/kotlin/FooTest.kt"} {
		findings := runRuleByNameOnPath(t, "NamedArguments", path, code)
		if len(findings) != 0 {
			t.Fatalf("expected no NamedArguments findings for %s, got %d", path, len(findings))
		}
	}
}

func TestNamedArguments_IgnoresForwardingWrappers(t *testing.T) {
	findings := runRuleByName(t, "NamedArguments", `
package test
enum class Priority { DEBUG }
interface Logger {
    fun log(priority: Priority, tag: String, message: String, throwable: Throwable? = null)
}
fun Logger.d(tag: String, message: String, throwable: Throwable? = null) =
    log(Priority.DEBUG, tag, message, throwable)
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for forwarding wrapper call, got %d", len(findings))
	}
}

const longParamList = "a: Int, b: Int, c: Int, d: Int, e: Int, g: Int, h: Int, i: Int"

func TestLongParameterList_ReportsTotalCount(t *testing.T) {
	findings := runRuleByName(t, "LongParameterList", "package test\nfun f("+longParamList+") {}\n")
	want := "The function f(" + longParamList + ") has too many parameters. The current threshold is set to 6."
	if len(findings) != 1 || findings[0].Message != want || findings[0].Col != 6 {
		t.Fatalf("expected one finding at the parameter list with detekt's message, got %v", findings)
	}
}

func TestLongParameterList_SuppressOnIndentedMemberFunction(t *testing.T) {
	code := "package test\nclass Box {\n    @Suppress(\"LongParameterList\")\n    fun f(" + longParamList + ") {}\n}\n"
	if findings := runRuleByName(t, "LongParameterList", code); len(findings) != 0 {
		t.Fatalf("expected suppressed finding, got %v", findings)
	}
}

func TestLongParameterList_SuppressWarningsOnEnclosingClass(t *testing.T) {
	code := "package test\n@SuppressWarnings(\"LongParameterList\")\nclass Box {\n    fun f(" + longParamList + ") {}\n}\n"
	if findings := runRuleByName(t, "LongParameterList", code); len(findings) != 0 {
		t.Fatalf("expected suppressed finding, got %v", findings)
	}
}

func TestLongParameterList_SuppressOnClass(t *testing.T) {
	code := "package test\n@Suppress(\"LongParameterList\")\nclass Box(" + longParamList + ")\n"
	if findings := runRuleByName(t, "LongParameterList", code); len(findings) != 0 {
		t.Fatalf("expected suppressed finding, got %v", findings)
	}
}

// detekt has no DI or Compose exemption: only override functions, data class constructors and parameters
// matching ignoreAnnotatedParameter are skipped.
func TestLongParameterList_OnlyOverridesAndDataClassesAreExempt(t *testing.T) {
	for _, code := range []string{
		"package test\n@Provides\nfun f(" + longParamList + ") = 1\n",
		"package test\nclass Box @Inject constructor(" + longParamList + ")\n",
		"package test\n@Composable\nfun Screen(" + longParamList + ") {}\n",
	} {
		if findings := runRuleByName(t, "LongParameterList", code); len(findings) != 1 {
			t.Fatalf("expected one finding for %q, got %v", code, findings)
		}
	}
	for _, code := range []string{
		"package test\nclass A : B {\n    override fun f(" + longParamList + ") {}\n}\n",
		"package test\ndata class D(" + longParamList + ")\n",
		"package test\nfun outer() {\n    fun inner(" + longParamList + ") {}\n}\n",
	} {
		if findings := runRuleByName(t, "LongParameterList", code); len(findings) != 0 {
			t.Fatalf("expected no findings for %q, got %v", code, findings)
		}
	}
	// 7 constructor parameters reach the constructor threshold of 7.
	seven := "package test\nclass C(a: Int, b: Int, c: Int, d: Int, e: Int, f: Int, g: Int)\n"
	if findings := runRuleByName(t, "LongParameterList", seven); len(findings) != 1 {
		t.Fatalf("7 constructor parameters must be reported, got %v", findings)
	}
}

// kotlinBody returns n statement lines, each `val xN = N`.
func kotlinBody(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		b.WriteString("    val x" + strconv.Itoa(i) + " = " + strconv.Itoa(i) + "\n")
	}
	return b.String()
}

// detekt sizes a function by its body only: the opening brace line to the closing brace line, so the
// annotation and the signature do not count. A body of 60 code lines reaches the default threshold of 60.
func TestLongMethod_BoundaryCountsBodyLinesOnly(t *testing.T) {
	build := func(statements int) string {
		return "package test\n@Suppress(\"unused\")\nfun process(\n    a: Int,\n    b: Int,\n) {\n" + kotlinBody(statements) + "}\n"
	}
	// The `) {` line, the statements and the closing brace.
	if findings := runRuleByName(t, "LongMethod", build(57)); len(findings) != 0 {
		t.Fatalf("59 body lines must not be reported, got %d", len(findings))
	}
	findings := runRuleByName(t, "LongMethod", build(58))
	if len(findings) != 1 {
		t.Fatalf("60 body lines reach the threshold of 60 and must be reported, got %d", len(findings))
	}
	if findings[0].Message != "The function process is too long (60). The maximum length is 60." {
		t.Errorf("unexpected message: %s", findings[0].Message)
	}
}

// An expression body counts from the expression, not from the `=`.
func TestLongMethod_ExpressionBodyCountsExpressionLines(t *testing.T) {
	var b strings.Builder
	b.WriteString("package test\nfun process() =\n    listOf(\n")
	for i := 0; i < 58; i++ {
		b.WriteString("        " + strconv.Itoa(i) + ",\n")
	}
	b.WriteString("    )\n")
	// 1 (listOf) + 58 + 1 = 60 expression lines.
	if findings := runRuleByName(t, "LongMethod", b.String()); len(findings) != 1 {
		t.Fatalf("expected one finding for a 60 line expression body, got %d", len(findings))
	}
}

// Lines of nested functions do not count for the outer function, which keeps only its own lines.
func TestLongMethod_NestedFunctionLinesAreNotCountedForTheOuterFunction(t *testing.T) {
	code := "package test\nfun outer() {\n    fun inner() {\n" + kotlinBody(70) + "    }\n    inner()\n}\n"
	findings := runRuleByName(t, "LongMethod", code)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "The function inner is too long") {
		t.Fatalf("only the nested function should be reported, got %v", findings)
	}
}

// A line with code and a block comment counts, a comment-only line does not.
func TestLongMethod_InlineBlockCommentLineCounts(t *testing.T) {
	var b strings.Builder
	b.WriteString("package test\nfun process() {\n")
	b.WriteString("    /* leading */ val first = 1\n")
	b.WriteString("    val second = 2 /* trailing */\n")
	b.WriteString("    /* only a comment */\n")
	b.WriteString("    /*\n     * many\n     * lines\n     */\n")
	b.WriteString("    // line comment\n")
	b.WriteString(kotlinBody(55))
	b.WriteString("}\n")
	// `{` line + 2 code-and-comment lines + 55 statements + closing brace = 59.
	if findings := runRuleByName(t, "LongMethod", b.String()); len(findings) != 0 {
		t.Fatalf("expected 59 code lines, got a finding: %s", findings[0].Message)
	}
	b.Reset()
	b.WriteString("package test\nfun process() {\n    /* leading */ val first = 1\n")
	b.WriteString(kotlinBody(58))
	b.WriteString("}\n")
	if findings := runRuleByName(t, "LongMethod", b.String()); len(findings) != 1 {
		t.Fatalf("expected a finding for 61 code lines, got %d", len(findings))
	}
}

// LargeClass counts the class's own lines, with every raw string line (blank ones too), in a .main.kts
// script as in a .kt file. A class of 600 code lines reaches the default threshold of 600.
func TestLargeClass_BoundaryInScriptCountsRawStringsAndSkipsNestedClasses(t *testing.T) {
	build := func(statements int) string {
		var b strings.Builder
		b.WriteString("class Big {\n")
		b.WriteString("    val template = \"\"\"\n        first\n\n        second\n    \"\"\"\n")
		b.WriteString("    class Nested {\n" + kotlinBody(30) + "    }\n")
		b.WriteString(kotlinBody(statements))
		b.WriteString("}\n")
		return b.String()
	}
	// class line + 5 raw-string lines (the blank one counts) + statements + closing brace.
	if findings := runRuleByNameOnPath(t, "LargeClass", "scripts/tool.main.kts", build(600-8)); len(findings) != 0 {
		t.Fatalf("599 code lines must not be reported, got %d", len(findings))
	}
	findings := runRuleByNameOnPath(t, "LargeClass", "scripts/tool.main.kts", build(600-7))
	if len(findings) != 1 {
		t.Fatalf("600 code lines reach the threshold of 600 and must be reported, got %d", len(findings))
	}
	if findings[0].Message != "Class Big is too large. Consider splitting it into smaller pieces." {
		t.Errorf("unexpected message: %s", findings[0].Message)
	}
}

func TestCyclomaticComplexMethod_IgnoreSingleWhenExpressionCoversReturnWhen(t *testing.T) {
	var rule *rules.CyclomaticComplexMethodRule
	for _, candidate := range api.Registry {
		if candidate.ID == "CyclomaticComplexMethod" {
			rule, _ = candidate.Implementation.(*rules.CyclomaticComplexMethodRule)
			break
		}
	}
	if rule == nil {
		t.Fatal("CyclomaticComplexMethod rule not registered")
	}
	originalSingle, originalSimple := rule.IgnoreSingleWhenExpression, rule.IgnoreSimpleWhenEntries
	defer func() { rule.IgnoreSingleWhenExpression, rule.IgnoreSimpleWhenEntries = originalSingle, originalSimple }()
	rule.IgnoreSimpleWhenEntries = true

	// A table-like when: braced entries with an if each, wrapped in a block that only returns it.
	var b strings.Builder
	b.WriteString("package test\nenum class Kind { A, B, C, D, E, F, G, H, I, J, K, L, M, N, O, P }\n")
	b.WriteString("fun span(kind: Kind, large: Boolean): Int {\n    return when (kind) {\n")
	for _, name := range []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P"} {
		b.WriteString("        Kind." + name + " -> {\n            if (large) 12 else 6\n        }\n")
	}
	b.WriteString("    }\n}\n")
	code := b.String()

	rule.IgnoreSingleWhenExpression = true
	if findings := runRuleByName(t, "CyclomaticComplexMethod", code); len(findings) != 0 {
		t.Fatalf("expected `return when` to count as a single when expression, got %d findings", len(findings))
	}
	rule.IgnoreSingleWhenExpression = false
	if findings := runRuleByName(t, "CyclomaticComplexMethod", code); len(findings) == 0 {
		t.Fatal("expected a finding for the same function when IgnoreSingleWhenExpression is false")
	}
}

func TestCyclomaticComplexMethod_FlatWhenOfSimpleEntriesCountsOnce(t *testing.T) {
	var rule *rules.CyclomaticComplexMethodRule
	for _, candidate := range api.Registry {
		if candidate.ID == "CyclomaticComplexMethod" {
			rule, _ = candidate.Implementation.(*rules.CyclomaticComplexMethodRule)
			break
		}
	}
	if rule == nil {
		t.Fatal("CyclomaticComplexMethod rule not registered")
	}
	originalSingle, originalSimple := rule.IgnoreSingleWhenExpression, rule.IgnoreSimpleWhenEntries
	defer func() { rule.IgnoreSingleWhenExpression, rule.IgnoreSimpleWhenEntries = originalSingle, originalSimple }()
	rule.IgnoreSingleWhenExpression = false
	rule.IgnoreSimpleWhenEntries = true

	// 30 unbraced entries count as one, so the function stays far below the threshold.
	var b strings.Builder
	b.WriteString("package test\nfun label(x: Int): String {\n    val prefix = \"p\"\n    return prefix + when (x) {\n")
	for i := 1; i <= 30; i++ {
		b.WriteString("        " + strconv.Itoa(i) + " -> \"v" + strconv.Itoa(i) + "\"\n")
	}
	b.WriteString("        else -> \"other\"\n    }\n}\n")
	if findings := runRuleByName(t, "CyclomaticComplexMethod", b.String()); len(findings) != 0 {
		t.Fatalf("expected a flat when of simple entries to count as 1, got %d findings", len(findings))
	}
}

func TestCyclomaticComplexMethod_ReportsLongFunctions(t *testing.T) {
	var b strings.Builder
	b.WriteString("package test\n\nfun big(x: Int): Int {\n    var r = 0\n")
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&b, "    if (x == %d) { r += 1 }\n", i)
	}
	for i := 0; i < 50; i++ {
		fmt.Fprintf(&b, "    r += %d\n", i)
	}
	b.WriteString("    return r\n}\n")
	if got := runRuleByName(t, "CyclomaticComplexMethod", b.String()); len(got) != 1 {
		t.Fatalf("a function longer than 60 lines with complexity 21 must be reported once, got %d", len(got))
	}
}

func TestLongParameterList_ReportsLongFunctions(t *testing.T) {
	var b strings.Builder
	b.WriteString("package test\n\nfun send(a: Int, b: Int, c: Int, d: Int, e: Int, f: Int) {\n    var r = 0\n")
	for i := 0; i < 70; i++ {
		fmt.Fprintf(&b, "    r += %d\n", i)
	}
	b.WriteString("}\n")
	if got := runRuleByName(t, "LongParameterList", b.String()); len(got) != 1 {
		t.Fatalf("a function longer than 60 lines with 6 parameters must be reported once, got %d", len(got))
	}
}

func TestNestedBlockDepth_CountsScopeFunctionLambdas(t *testing.T) {
	code := `package test
fun f(x: Int, y: Int) {
    run {
        if (x > 0) {
            for (i in 0..x) {
                while (i > 0) {
                    if (y > 0) {
                        println()
                    }
                }
            }
        }
    }
}
`
	if got := runRuleByName(t, "NestedBlockDepth", code); len(got) != 1 {
		t.Fatalf("depth 5 inside run { } must be reported once, got %d", len(got))
	}
	plain := `package test
fun f(xs: List<Int>, y: Int) {
    xs.map {
        if (y > 0) {
            println()
        }
    }
}
`
	if got := runRuleByName(t, "NestedBlockDepth", plain); len(got) != 0 {
		t.Fatalf("shallow nesting inside a lambda must not be reported, got %d", len(got))
	}
}

func TestNestedBlockDepth_AlsoLambdaIsNotCounted(t *testing.T) {
	code := `package test
fun f(x: Int, y: Int) {
    if (x > 0) {
        for (i in 0..x) {
            if (y > 0) {
                y.also {
                    println(it)
                }
            }
        }
    }
}
`
	if got := runRuleByName(t, "NestedBlockDepth", code); len(got) != 0 {
		t.Fatalf("depth 3 plus an also lambda must not be reported (detekt does not count also), got %d", len(got))
	}
	counted := strings.Replace(code, "y.also {", "y.let {", 1)
	if got := runRuleByName(t, "NestedBlockDepth", counted); len(got) != 1 {
		t.Fatalf("a let lambda at depth 4 must be reported once, got %d", len(got))
	}
}

func TestNestedBlockDepth_TryCountsAsNesting(t *testing.T) {
	code := `package test
fun f(x: Int, y: Int) {
    try {
        if (x > 0) {
            for (i in 0..x) {
                while (i > 0) {
                    if (y > 0) {
                        println()
                    }
                }
            }
        }
    } catch (e: Exception) {
        throw e
    }
}
`
	if got := runRuleByName(t, "NestedBlockDepth", code); len(got) != 1 {
		t.Fatalf("depth 5 including try must be reported once, got %d", len(got))
	}
}

func TestCognitiveComplexMethod_ElseIfChainIsLinear(t *testing.T) {
	var b strings.Builder
	b.WriteString("package test\nfun f(x: Int) {\n    if (x == 0) { println() }")
	for i := 1; i < 12; i++ {
		fmt.Fprintf(&b, " else if (x == %d) { println() }", i)
	}
	b.WriteString(" else { println() }\n}\n")
	// Sonar scoring: 1 for the if, 1 per else-if, 1 for the else = 13, below the limit of 15.
	if got := runRuleByName(t, "CognitiveComplexMethod", b.String()); len(got) != 0 {
		t.Fatalf("an else-if chain must not be scored quadratically, got %v", got)
	}
}

func TestCognitiveComplexMethod_CountsDecisionsInLambdas(t *testing.T) {
	var b strings.Builder
	b.WriteString("package test\nfun f(xs: List<Int>) {\n    xs.forEach { x ->\n")
	for i := 0; i < 8; i++ {
		fmt.Fprintf(&b, "        if (x == %d) { for (j in 0..x) { if (j > 1) { println() } } }\n", i)
	}
	b.WriteString("    }\n}\n")
	if got := runRuleByName(t, "CognitiveComplexMethod", b.String()); len(got) != 1 {
		t.Fatalf("decisions inside a lambda must count towards cognitive complexity, got %d", len(got))
	}
}

func TestTooManyFunctions_InterfacesObjectsEnumsAndNestedClasses(t *testing.T) {
	funcs := func(n int, body string) string {
		var b strings.Builder
		for i := 0; i < n; i++ {
			fmt.Fprintf(&b, "    fun f%d()%s\n", i, body)
		}
		return b.String()
	}
	cases := map[string]string{
		"interface":    "interface Api {\n" + funcs(12, "") + "}\n",
		"object":       "object Util {\n" + funcs(12, " {}") + "}\n",
		"enum":         "enum class Kind { A;\n" + funcs(12, " {}") + "}\n",
		"nested class": "class Outer {\n    class Inner {\n" + funcs(12, " {}") + "    }\n}\n",
	}
	for name, src := range cases {
		if got := runRuleByName(t, "TooManyFunctions", "package test\n"+src); len(got) != 1 {
			t.Errorf("%s: want 1 finding, got %d", name, len(got))
		}
	}
	got := runRuleByName(t, "TooManyFunctions", "package test\ninterface Api {\n"+funcs(11, "")+"}\n")
	if len(got) != 1 || got[0].Message != "Interface 'Api' with '11' functions detected. Defined threshold inside interfaces is set to '11'" {
		t.Errorf("11 functions reach the threshold of 11, got %v", got)
	}
	if got := runRuleByName(t, "TooManyFunctions", "package test\ninterface Api {\n"+funcs(10, "")+"}\n"); len(got) != 0 {
		t.Errorf("10 functions are allowed, got %d findings", len(got))
	}
}

func TestNamedArguments_SkipsVarargBuilders(t *testing.T) {
	src := "package test\nfun f() {\n    val a = listOf(1, 2, 3, 4, 5)\n    val b = arrayOf(1, 2, 3, 4)\n    val c = String.format(\"%s %s %s %s\", 1, 2, 3, 4)\n}\n"
	if got := runRuleByName(t, "NamedArguments", src); len(got) != 0 {
		t.Errorf("variadic builders cannot take named elements, got %v", got)
	}
	src = "package test\nfun g(a: Int, b: Int, c: Int, d: Int) {}\nfun f() { g(1, 2, 3, 4) }\n"
	if got := runRuleByName(t, "NamedArguments", src); len(got) != 1 {
		t.Errorf("an ordinary call with four positional arguments must be reported, got %d", len(got))
	}
}

func TestComplexCondition_DoWhileCondition(t *testing.T) {
	src := "package test\nfun f(a: Boolean, b: Boolean, c: Boolean, d: Boolean, e: Boolean) {\n    do { println() } while (a && b || c && d || e)\n}\n"
	if got := runRuleByName(t, "ComplexCondition", src); len(got) != 1 {
		t.Fatalf("a complex do-while condition must be reported once, got %d", len(got))
	}
}

func TestNestedScopeFunctions_ChainsAreNotNesting(t *testing.T) {
	chain := "package test\nfun f(x: String) {\n    x.also { println(it) }.also { println(it) }.also { println(it) }.apply { println(this) }\n}\n"
	if got := runRuleByName(t, "NestedScopeFunctions", chain); len(got) != 0 {
		t.Errorf("a chain of scope calls is not nesting, got %v", got)
	}
	nested := "package test\nfun f(x: String) {\n    run { run { run { println(x) } } }\n}\n"
	if got := runRuleByName(t, "NestedScopeFunctions", nested); len(got) != 1 {
		t.Errorf("three nested run blocks must be reported once, got %d", len(got))
	}
}

// detekt's NestedBlockDepth visitor does not descend into a call that is not one of its nesting functions,
// so control flow inside such a call's arguments or lambda is not counted. `with(x) { }` is one call.
func TestNestedBlockDepth_DoesNotDescendIntoOtherCalls(t *testing.T) {
	body := "if (a) { if (b) { if (c) { if (d) { println() } } } }"
	other := "package test\nfun f(a: Boolean, b: Boolean, c: Boolean, d: Boolean) {\n    launch { " + body + " }\n}\n"
	if got := runRuleByName(t, "NestedBlockDepth", other); len(got) != 0 {
		t.Fatalf("control flow inside a non-nesting lambda is not counted, got %d", len(got))
	}
	with := "package test\nfun f(a: Boolean, b: Boolean, c: Boolean, d: Boolean) {\n    with(a) { " + body + " }\n}\n"
	if got := runRuleByName(t, "NestedBlockDepth", with); len(got) != 1 {
		t.Fatalf("with(x) { } nests, so depth 5 is reported, got %d", len(got))
	}
}

// detekt orders logical operators by the operator token, so a parenthesized left operand of the same
// operator is not a new sequence but a parenthesized right operand is.
func TestCognitiveComplexMethod_ParenthesizedOperandOrder(t *testing.T) {
	build := func(expr string) string {
		var b strings.Builder
		b.WriteString("package test\nfun f(x: Int, a: Boolean, b: Boolean, c: Boolean): Boolean {\n")
		for i := 1; i <= 13; i++ {
			fmt.Fprintf(&b, "    if (x > %d) println()\n", i)
		}
		b.WriteString("    return " + expr + "\n}\n")
		return b.String()
	}
	// 13 ifs score 13, so the limit of 15 is reached only when the expression scores 2.
	for _, expr := range []string{"(a && b) && c", "((a || b)) || c", "!(a && b) && c", "a && b && c"} {
		if got := runRuleByName(t, "CognitiveComplexMethod", build(expr)); len(got) != 0 {
			t.Errorf("%s scores 1, got %v", expr, got)
		}
	}
	for _, expr := range []string{"a && (b && c)", "a || (b || c)", "(a || b) && c"} {
		if got := runRuleByName(t, "CognitiveComplexMethod", build(expr)); len(got) != 1 {
			t.Errorf("%s scores 2, got %v", expr, got)
		}
	}
}

// detekt counts named arguments toward the limit and reports a call with any unnamed argument.
func TestNamedArguments_MixedNamedAndPositional(t *testing.T) {
	decl := "package test\nfun g(a: Int, b: Int, c: Int, d: Int) {}\nfun f() {\n    "
	cases := []struct {
		call string
		want int
	}{
		{"g(1, 2, 3, d = 4)", 1},
		{"g(1, b = 2, c = 3, d = 4)", 1},
		{"g(a = 1, b = 2, c = 3, d = 4)", 0},
		{"g(1, 2, c = 3)", 0},
	}
	for _, tc := range cases {
		if got := runRuleByName(t, "NamedArguments", decl+tc.call+"\n}\n"); len(got) != tc.want {
			t.Errorf("%s: got %d findings, want %d", tc.call, len(got), tc.want)
		}
	}
}
