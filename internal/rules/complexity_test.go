package rules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
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
		b.WriteString(itoa(i))
		b.WriteString(" = ")
		b.WriteString(itoa(i))
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
		b.WriteString(itoa(i))
		b.WriteString(" = ")
		b.WriteString(itoa(i))
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
		b.WriteString(itoa(i))
		b.WriteString(" = ")
		b.WriteString(itoa(i))
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
		b.WriteString(itoa(i))
		b.WriteString(" = ")
		b.WriteString(itoa(i))
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
		b.WriteString(itoa(i))
		b.WriteString(" = ")
		b.WriteString(itoa(i))
		b.WriteString("\n")
	}
	b.WriteString("}\n")
	findings := runRuleByName(t, "LongMethod", b.String())
	if len(findings) == 0 {
		t.Fatal("expected LongMethod finding despite `\"\"\"` in block comment")
	}
}

// Sanity: a multi-line raw string literal still suppresses its content lines.
func TestLongMethod_RawStringContentSkipped(t *testing.T) {
	var b strings.Builder
	b.WriteString("package test\nfun process() {\n")
	b.WriteString("    val s = \"\"\"\n")
	for i := 1; i <= 70; i++ {
		b.WriteString("        line ")
		b.WriteString(itoa(i))
		b.WriteString("\n")
	}
	b.WriteString("    \"\"\"\n")
	b.WriteString("    val y = 1\n")
	b.WriteString("}\n")
	findings := runRuleByName(t, "LongMethod", b.String())
	if len(findings) != 0 {
		t.Fatalf("expected no LongMethod finding (raw-string body should not count), got %d", len(findings))
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
		b.WriteString(itoa(i))
		b.WriteString(") r += ")
		b.WriteString(itoa(i))
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
		b.WriteString(itoa(i))
		b.WriteString(") r += ")
		b.WriteString(itoa(i))
		b.WriteString("\n")
	}
	b.WriteString("    return r\n}\n")
	findings := runRuleByName(t, "CyclomaticComplexMethod", b.String())
	if len(findings) != 0 {
		t.Fatalf("expected no CyclomaticComplexMethod finding, got %d", len(findings))
	}
}

func TestCyclomaticComplexMethod_DetektParitySimpleWhenEntriesCount(t *testing.T) {
	// detekt parity: ignoreSimpleWhenEntries default is false, so each
	// simple when entry contributes 1 to cyclomatic complexity. A `when`
	// with 16 simple entries (and an else branch) on top of the base 1
	// pushes complexity above the default threshold of 14. Under the
	// previous default (true) the rule would have skipped these entries
	// entirely and produced no finding.
	var b strings.Builder
	b.WriteString("package test\nfun classify(x: Int): String = when (x) {\n")
	for i := 1; i <= 16; i++ {
		b.WriteString("    ")
		b.WriteString(itoa(i))
		b.WriteString(" -> \"v")
		b.WriteString(itoa(i))
		b.WriteString("\"\n")
	}
	b.WriteString("    else -> \"other\"\n}\n")
	findings := runRuleByName(t, "CyclomaticComplexMethod", b.String())
	if len(findings) == 0 {
		t.Fatal("expected CyclomaticComplexMethod finding for 16 simple when entries")
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

func TestLongParameterList_DetektParityConstructorBoundary(t *testing.T) {
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

func TestLongParameterList_DetektParityDefaultParametersCount(t *testing.T) {
	// Detekt parity: ignoreDefaultParameters=false by default, so parameters
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

// --- TooManyFunctions ---

func TestTooManyFunctions_Positive(t *testing.T) {
	// 12 top-level functions exceeds threshold of 11
	var b strings.Builder
	b.WriteString("package test\n")
	for i := 1; i <= 12; i++ {
		b.WriteString("fun fn")
		b.WriteString(itoa(i))
		b.WriteString("() {}\n")
	}
	findings := runRuleByName(t, "TooManyFunctions", b.String())
	if len(findings) == 0 {
		t.Fatal("expected TooManyFunctions finding for 12 functions")
	}
}

func TestTooManyFunctions_Negative(t *testing.T) {
	// 11 top-level functions equals threshold (not exceeded)
	var b strings.Builder
	b.WriteString("package test\n")
	for i := 1; i <= 11; i++ {
		b.WriteString("fun fn")
		b.WriteString(itoa(i))
		b.WriteString("() {}\n")
	}
	findings := runRuleByName(t, "TooManyFunctions", b.String())
	if len(findings) != 0 {
		t.Fatalf("expected no TooManyFunctions finding for 11 functions, got %d", len(findings))
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

// --- LargeClass ---

func TestLargeClass_Positive(t *testing.T) {
	// Class with 602 lines exceeds 600 threshold
	var b strings.Builder
	b.WriteString("package test\nclass BigService {\n")
	for i := 1; i <= 600; i++ {
		b.WriteString("    val field")
		b.WriteString(itoa(i))
		b.WriteString(" = ")
		b.WriteString(itoa(i))
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
		b.WriteString(itoa(i))
		b.WriteString(" = ")
		b.WriteString(itoa(i))
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

func TestNestedBlockDepth_Negative(t *testing.T) {
	// 4 levels of nesting equals threshold (not exceeded)
	code := `package test
fun shallowNesting(x: Int) {
    if (x > 0) {
        if (x > 1) {
            if (x > 2) {
                if (x > 3) {
                    println(x)
                }
            }
        }
    }
}
`
	findings := runRuleByName(t, "NestedBlockDepth", code)
	if len(findings) != 0 {
		t.Fatalf("expected no NestedBlockDepth finding for depth 4, got %d", len(findings))
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
                if (x < -3) {
                    println(x)
                }
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
                if (x < -3) {
                    println(x)
                }
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
	// Each top-level if adds 1 (1+0 nesting). 16 top-level ifs => complexity = 16 > 15
	var b strings.Builder
	b.WriteString("package test\nfun cognitive(x: Int): Int {\n    var r = 0\n")
	for i := 1; i <= 16; i++ {
		b.WriteString("    if (x > ")
		b.WriteString(itoa(i))
		b.WriteString(") r += ")
		b.WriteString(itoa(i))
		b.WriteString("\n")
	}
	b.WriteString("    return r\n}\n")
	findings := runRuleByName(t, "CognitiveComplexMethod", b.String())
	if len(findings) == 0 {
		t.Fatal("expected CognitiveComplexMethod finding")
	}
}

func TestCognitiveComplexMethod_Negative(t *testing.T) {
	// 15 top-level ifs => complexity = 15 (equals threshold, not exceeded)
	var b strings.Builder
	b.WriteString("package test\nfun cognitive(x: Int): Int {\n    var r = 0\n")
	for i := 1; i <= 15; i++ {
		b.WriteString("    if (x > ")
		b.WriteString(itoa(i))
		b.WriteString(") r += ")
		b.WriteString(itoa(i))
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

func TestComplexCondition_LambdaOperatorsDoNotInflateOuter(t *testing.T) {
	code := `package test
fun check(a: Boolean, items: List<Int>) {
    if (a && items.any { it > 0 && it < 10 && it != 5 && it != 7 || it == 42 }) {
        println("nested lambda operators must not count toward outer condition")
    }
}
`
	findings := runRuleByName(t, "ComplexCondition", code)
	if len(findings) != 0 {
		t.Fatalf("expected no ComplexCondition finding when operators live in nested lambda, got %d", len(findings))
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
		b.WriteString(itoa(i))
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
	for i := 1; i <= 10; i++ {
		b.WriteString("    fun method")
		b.WriteString(itoa(i))
		b.WriteString("()\n")
	}
	b.WriteString("}\n")
	findings := runRuleByName(t, "ComplexInterface", b.String())
	if len(findings) != 0 {
		t.Fatalf("expected no ComplexInterface finding for 10 methods, got %d", len(findings))
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
    fun method10()

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
    fun j()
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
			b.WriteString(itoa(j))
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
	for i := 0; i < 6; i++ {
		b.WriteString("fun process(")
		for j := 0; j < i; j++ {
			if j > 0 {
				b.WriteString(", ")
			}
			b.WriteString("a")
			b.WriteString(itoa(j))
			b.WriteString(": Int")
		}
		b.WriteString(") {}\n")
	}
	findings := runRuleByName(t, "MethodOverloading", b.String())
	if len(findings) != 0 {
		t.Fatalf("expected no MethodOverloading finding for 6 overloads, got %d", len(findings))
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
			b.WriteString(itoa(j))
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
			b.WriteString(itoa(j))
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
	if len(findings) != 0 {
		t.Fatalf("expected no MethodOverloading finding when overloads are split across nested classes, got %d", len(findings))
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

func parseBenchmarkFile(b *testing.B, code string) *scanner.File {
	b.Helper()
	dir := b.TempDir()
	path := filepath.Join(dir, "bench.kt")
	if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
		b.Fatalf("write benchmark file: %v", err)
	}
	file, err := scanner.ParseFile(path)
	if err != nil {
		b.Fatalf("ParseFile(%s): %v", path, err)
	}
	return file
}

func complexityBenchmarkDispatcher() *rules.Dispatcher {
	var selected []*v2rules.Rule
	for _, r := range v2rules.Registry {
		switch r.ID {
		case "LongParameterList", "NestedBlockDepth", "CyclomaticComplexMethod", "CognitiveComplexMethod", "ComplexCondition":
			selected = append(selected, r)
		}
	}
	return rules.NewDispatcherV2(selected)
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
		src.WriteString(itoa(i))
		src.WriteString(") r += ")
		src.WriteString(itoa(i))
		src.WriteString("\n")
	}
	src.WriteString("    return r\n}\n")

	file := parseBenchmarkFile(b, src.String())
	var rule *v2rules.Rule
	for _, r := range v2rules.Registry {
		if r.ID == "CyclomaticComplexMethod" {
			rule = r
			break
		}
	}
	if rule == nil {
		b.Fatal("CyclomaticComplexMethod rule not found")
	}
	d := rules.NewDispatcherV2([]*v2rules.Rule{rule})
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
		src.WriteString(itoa(i))
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
	var rule *v2rules.Rule
	for _, r := range v2rules.Registry {
		if r.ID == "NestedBlockDepth" {
			rule = r
			break
		}
	}
	if rule == nil {
		b.Fatal("NestedBlockDepth rule not found")
	}
	d := rules.NewDispatcherV2([]*v2rules.Rule{rule})
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
		src.WriteString(itoa(i))
		src.WriteString(") { println(x) }\n")
	}
	src.WriteString("    else { println(x) }\n")
	src.WriteString("}\n")

	file := parseBenchmarkFile(b, src.String())
	var rule *v2rules.Rule
	for _, r := range v2rules.Registry {
		if r.ID == "NestedBlockDepth" {
			rule = r
			break
		}
	}
	if rule == nil {
		b.Fatal("NestedBlockDepth rule not found")
	}
	d := rules.NewDispatcherV2([]*v2rules.Rule{rule})
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
		src.WriteString(itoa(i))
		src.WriteString("() {}\n")
	}
	src.WriteString("class Outer {\n")
	for i := 0; i < 40; i++ {
		src.WriteString("    fun inner")
		src.WriteString(itoa(i))
		src.WriteString("() {}\n")
	}
	src.WriteString("    class Nested {\n")
	for i := 0; i < 20; i++ {
		src.WriteString("        fun nested")
		src.WriteString(itoa(i))
		src.WriteString("() {}\n")
	}
	src.WriteString("    }\n}\n")

	file := parseBenchmarkFile(b, src.String())
	var rule *v2rules.Rule
	for _, r := range v2rules.Registry {
		if r.ID == "TooManyFunctions" {
			rule = r
			break
		}
	}
	if rule == nil {
		b.Fatal("TooManyFunctions rule not found")
	}
	d := rules.NewDispatcherV2([]*v2rules.Rule{rule})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = d.Run(file)
	}
}

// itoa is a simple int-to-string helper to avoid importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
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
