package rules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/rules/registry"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

// --- ExpressionBodySyntax ---

func TestExpressionBodySyntax_Positive(t *testing.T) {
	findings := runRuleByName(t, "ExpressionBodySyntax", `
package test
fun double(x: Int): Int {
    return x * 2
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for single-return body convertible to expression syntax")
	}
}

func TestExpressionBodySyntax_Negative(t *testing.T) {
	findings := runRuleByName(t, "ExpressionBodySyntax", `
package test
fun double(x: Int): Int = x * 2
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for expression body, got %d", len(findings))
	}
}

func TestExpressionBodySyntax_HonorsIncludeLineWrapping(t *testing.T) {
	// IncludeLineWrapping was previously a dead config — exposed in
	// zz_meta but never consulted. Configure it via the rule pointer
	// and verify multi-line single-return bodies are flagged only when
	// the option is on.
	var rule *rules.ExpressionBodySyntaxRule
	for _, candidate := range v2rules.Registry {
		if candidate.ID == "ExpressionBodySyntax" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.ExpressionBodySyntaxRule)
			if !ok {
				t.Fatalf("expected ExpressionBodySyntaxRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("ExpressionBodySyntax rule not registered")
	}
	original := rule.IncludeLineWrapping
	defer func() { rule.IncludeLineWrapping = original }()

	multiLineCode := `package test
fun render(name: String): String {
    return name
        .trim()
        .lowercase()
}
`
	// Default (false): multi-line return body NOT flagged.
	if findings := runRuleByName(t, "ExpressionBodySyntax", multiLineCode); len(findings) != 0 {
		t.Fatalf("expected no findings for multi-line body under IncludeLineWrapping=false, got %d", len(findings))
	}

	rule.IncludeLineWrapping = true

	// IncludeLineWrapping=true: same multi-line body is flagged.
	if findings := runRuleByName(t, "ExpressionBodySyntax", multiLineCode); len(findings) == 0 {
		t.Fatal("expected finding for multi-line body when IncludeLineWrapping=true")
	}

	// A non-return-only multi-line body still doesn't fire — IncludeLineWrapping
	// only affects the single-return-statement case.
	multiStatement := `package test
fun work() {
    println("a")
    println("b")
}
`
	if findings := runRuleByName(t, "ExpressionBodySyntax", multiStatement); len(findings) != 0 {
		t.Fatalf("expected no findings for multi-statement body even with IncludeLineWrapping=true, got %d", len(findings))
	}
}

func TestExpressionBodySyntax_IgnoresTestSources(t *testing.T) {
	findings := runRuleByNameOnPath(t, "ExpressionBodySyntax", "src/androidTest/kotlin/TestRunner.kt", `
package test
fun runner(): Runner {
    return Runner()
}
class Runner
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for test sources, got %d", len(findings))
	}
}

// --- ReturnCount ---

func TestReturnCount_Positive(t *testing.T) {
	findings := runRuleByName(t, "ReturnCount", `
package test
fun classify(x: Int, y: Int): String {
    println("starting classify")
    val result = if (x > y) {
        if (x > 100) {
            return "big"
        }
        println("x bigger")
        "x"
    } else {
        if (y < 0) {
            return "negative"
        }
        if (y == 0) {
            return "zero"
        }
        if (y > 1000) {
            return "huge"
        }
        println("y bigger")
        "y"
    }
    return result
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for function with many non-guard returns")
	}
}

func TestReturnCount_Negative(t *testing.T) {
	findings := runRuleByName(t, "ReturnCount", `
package test
fun isPositive(x: Int): Boolean {
    if (x > 0) return true
    return false
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for 2 returns (within limit), got %d", len(findings))
	}
}

func TestReturnCount_DefaultCountsGuardClauseReturns(t *testing.T) {
	findings := runRuleByName(t, "ReturnCount", `
package test
fun classify(x: Int): String {
    if (x < 0) return "negative"
    if (x == 0) return "zero"
    return "positive"
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for 3 returns including guard clauses")
	}
}

// --- ThrowsCount ---

func TestThrowsCount_Positive(t *testing.T) {
	findings := runRuleByName(t, "ThrowsCount", `
package test
fun validate(x: Int) {
    if (x < 0) throw IllegalArgumentException("negative")
    if (x == 0) throw IllegalStateException("zero")
    if (x > 100) throw RuntimeException("too big")
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for function with 3 throw statements (max 2)")
	}
}

func TestThrowsCount_Negative(t *testing.T) {
	findings := runRuleByName(t, "ThrowsCount", `
package test
fun validate(x: Int) {
    if (x < 0) throw IllegalArgumentException("negative")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for 1 throw (within limit), got %d", len(findings))
	}
}

// --- CollapsibleIfStatements ---

func TestCollapsibleIfStatements_Positive(t *testing.T) {
	findings := runRuleByName(t, "CollapsibleIfStatements", `
package test
fun check(a: Boolean, b: Boolean) {
    if (a) {
        if (b) {
            println("both")
        }
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for collapsible nested ifs")
	}
}

func TestCollapsibleIfStatements_Negative(t *testing.T) {
	findings := runRuleByName(t, "CollapsibleIfStatements", `
package test
fun check(a: Boolean, b: Boolean) {
    if (a && b) {
        println("both")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for already-merged condition, got %d", len(findings))
	}
}

// --- SafeCast ---

func TestSafeCast_Positive(t *testing.T) {
	findings := runRuleByName(t, "SafeCast", `
package test
fun process(obj: Any) {
    if (obj is String) {
        val s = obj as String
        println(s)
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for is-check followed by unsafe cast")
	}
}

func TestSafeCast_Negative(t *testing.T) {
	findings := runRuleByName(t, "SafeCast", `
package test
fun process(obj: Any) {
    val s = obj as? String
    println(s)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for safe cast, got %d", len(findings))
	}
}

// --- VarCouldBeVal ---

func TestVarCouldBeVal_Positive(t *testing.T) {
	findings := runRuleByName(t, "VarCouldBeVal", `
package test
fun example() {
    var x = 42
    println(x)
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for var that is never reassigned")
	}
}

func TestVarCouldBeVal_Negative(t *testing.T) {
	findings := runRuleByName(t, "VarCouldBeVal", `
package test
fun example() {
    var x = 0
    x = 42
    println(x)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for var that is reassigned, got %d", len(findings))
	}
}

func TestVarCouldBeVal_DoesNotTreatOtherReceiverAssignmentAsReassignment(t *testing.T) {
	findings := runRuleByName(t, "VarCouldBeVal", `
package test
class Box(var x: Int)
fun example(other: Box) {
    var x = 0
    other.x = 42
    println(x)
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for local var when only another receiver's property is reassigned")
	}
}

func TestVarCouldBeVal_TreatsThisReceiverAssignmentAsReassignment(t *testing.T) {
	findings := runRuleByName(t, "VarCouldBeVal", `
package test
class SendButton {
    private var scheduledSendListener: (() -> Unit)? = null

    fun setScheduledSendListener(listener: (() -> Unit)?) {
        this.scheduledSendListener = listener
    }

    fun fire() = scheduledSendListener?.invoke()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for private property assigned through this receiver, got %d", len(findings))
	}
}

func TestVarCouldBeVal_DoesNotRequireTypeContext(t *testing.T) {
	for _, rule := range v2rules.Registry {
		if rule.ID == "VarCouldBeVal" {
			if rule.Needs.Has(v2rules.NeedsResolver) || rule.Needs.Has(v2rules.NeedsOracle) ||
				rule.Needs.Has(v2rules.NeedsParsedFiles) || rule.Needs.Has(v2rules.NeedsCrossFile) {
				t.Fatalf("VarCouldBeVal should stay AST-only; got needs %v", rule.Needs)
			}
			return
		}
	}
	t.Fatal("VarCouldBeVal rule not found")
}

func BenchmarkVarCouldBeValSharedScope(b *testing.B) {
	var src strings.Builder
	src.WriteString("package test\nfun example() {\n")
	for i := 0; i < 400; i++ {
		src.WriteString("    var value")
		src.WriteString(string(rune('A' + (i % 26))))
		src.WriteString("_")
		src.WriteString(strings.Repeat("x", i/26))
		src.WriteString(" = ")
		src.WriteString("0\n")
	}
	for i := 0; i < 100; i++ {
		src.WriteString("    value")
		src.WriteString(string(rune('A' + (i % 26))))
		src.WriteString("_")
		src.WriteString(strings.Repeat("x", i/26))
		src.WriteString(" = ")
		src.WriteString("1\n")
	}
	src.WriteString("}\n")

	path := filepath.Join(b.TempDir(), "bench.kt")
	if err := os.WriteFile(path, []byte(src.String()), 0644); err != nil {
		b.Fatal(err)
	}
	file, err := scanner.ParseFile(path)
	if err != nil {
		b.Fatal(err)
	}

	var target *v2rules.Rule
	for _, r := range v2rules.Registry {
		if r.ID == "VarCouldBeVal" {
			target = r
			break
		}
	}
	if target == nil {
		b.Fatal("VarCouldBeVal rule not found")
	}

	dispatcher := rules.NewDispatcherV2([]*v2rules.Rule{target})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = dispatcher.Run(file)
	}
}

// --- MayBeConstant ---

func TestMayBeConstant_Positive(t *testing.T) {
	findings := runRuleByName(t, "MayBeConstant", `
package test
val MAX_COUNT = 100
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for top-level val with constant initializer")
	}
}

func TestMayBeConstant_Negative(t *testing.T) {
	findings := runRuleByName(t, "MayBeConstant", `
package test
const val MAX_COUNT = 100
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for const val, got %d", len(findings))
	}
}

func TestMayBeConstant_SameFileReferenceAndBinaryPositive(t *testing.T) {
	findings := runRuleByName(t, "MayBeConstant", `
package test
val BASE = 40
val MAX_COUNT = BASE + 2
`)
	if len(findings) < 2 {
		t.Fatalf("expected findings for literal and same-file constant expression, got %d", len(findings))
	}
}

func TestMayBeConstant_NullNegative(t *testing.T) {
	findings := runRuleByName(t, "MayBeConstant", `
package test
val NOTHING = null
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for null initializer, got %d", len(findings))
	}
}

// --- ModifierOrder ---

func TestModifierOrder_Positive(t *testing.T) {
	findings := runRuleByName(t, "ModifierOrder", `
package test
override public fun toString(): String = "test"
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for modifiers in wrong order (override before public)")
	}
}

func TestModifierOrder_Negative(t *testing.T) {
	findings := runRuleByName(t, "ModifierOrder", `
package test
public override fun toString(): String = "test"
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for correctly ordered modifiers, got %d", len(findings))
	}
}

// --- FunctionOnlyReturningConstant ---

func TestFunctionOnlyReturningConstant_Positive(t *testing.T) {
	findings := runRuleByName(t, "FunctionOnlyReturningConstant", `
package test
fun getAnswer(): Int {
    return 42
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for function only returning a constant")
	}
}

func TestFunctionOnlyReturningConstant_Negative(t *testing.T) {
	findings := runRuleByName(t, "FunctionOnlyReturningConstant", `
package test
fun compute(x: Int): Int {
    return x * 2
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for function returning computed value, got %d", len(findings))
	}
}

// --- LoopWithTooManyJumpStatements ---

func TestLoopWithTooManyJumpStatements_Positive(t *testing.T) {
	findings := runRuleByName(t, "LoopWithTooManyJumpStatements", `
package test
fun process(items: List<Int>) {
    for (item in items) {
        if (item < 0) continue
        if (item > 100) break
        println(item)
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for loop with 2 jump statements (max 1)")
	}
}

func TestLoopWithTooManyJumpStatements_Negative(t *testing.T) {
	findings := runRuleByName(t, "LoopWithTooManyJumpStatements", `
package test
fun process(items: List<Int>) {
    for (item in items) {
        if (item < 0) continue
        println(item)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for loop with 1 jump (within limit), got %d", len(findings))
	}
}

func TestLoopWithTooManyJumpStatementsDefaultsMatchDetekt(t *testing.T) {
	var rule *rules.LoopWithTooManyJumpStatementsRule
	var meta registry.RuleDescriptor
	for _, candidate := range v2rules.Registry {
		if candidate.ID != "LoopWithTooManyJumpStatements" {
			continue
		}
		var ok bool
		rule, ok = candidate.Implementation.(*rules.LoopWithTooManyJumpStatementsRule)
		if !ok {
			t.Fatalf("expected LoopWithTooManyJumpStatementsRule, got %T", candidate.Implementation)
		}
		metaProvider, ok := candidate.Implementation.(registry.MetaProvider)
		if !ok {
			t.Fatal("expected LoopWithTooManyJumpStatementsRule to provide metadata")
		}
		meta = metaProvider.Meta()
		break
	}
	if rule == nil {
		t.Fatal("LoopWithTooManyJumpStatements rule not registered")
	}
	if rule.MaxJumps != 1 {
		t.Fatalf("expected MaxJumps default 1, got %d", rule.MaxJumps)
	}
	for _, opt := range meta.Options {
		if opt.Name == "maxJumpCount" {
			if opt.Default != 1 {
				t.Fatalf("expected maxJumpCount metadata default 1, got %v", opt.Default)
			}
			return
		}
	}
	t.Fatal("maxJumpCount option not found")
}

// --- ExplicitItLambdaParameter ---

func TestExplicitItLambdaParameter_Positive(t *testing.T) {
	findings := runRuleByName(t, "ExplicitItLambdaParameter", `
package test
fun example() {
    listOf(1, 2, 3).map { it -> it * 2 }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for explicit 'it' lambda parameter")
	}
}

func TestExplicitItLambdaParameter_Negative(t *testing.T) {
	findings := runRuleByName(t, "ExplicitItLambdaParameter", `
package test
fun example() {
    listOf(1, 2, 3).map { it * 2 }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for implicit 'it', got %d", len(findings))
	}
}

// --- ExplicitItLambdaMultipleParameters ---

func TestExplicitItLambdaMultipleParameters_Positive(t *testing.T) {
	findings := runRuleByName(t, "ExplicitItLambdaMultipleParameters", `
package test
fun example() {
    mapOf(1 to "a").forEach { it, value -> println(value) }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for 'it' used as parameter name in multi-param lambda")
	}
}

func TestExplicitItLambdaMultipleParameters_Negative(t *testing.T) {
	findings := runRuleByName(t, "ExplicitItLambdaMultipleParameters", `
package test
fun example() {
    mapOf(1 to "a").forEach { key, value -> println(value) }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for properly named multi-param lambda, got %d", len(findings))
	}
}

// --- MagicNumber (from style_forbidden.go) ---

func TestMagicNumber_Expr_Positive(t *testing.T) {
	findings := runRuleByName(t, "MagicNumber", `
package test
fun compute(x: Int): Int {
    return x * 42
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for magic number 42")
	}
}

func TestMagicNumber_Expr_Negative(t *testing.T) {
	findings := runRuleByName(t, "MagicNumber", `
package test
fun compute(x: Int): Int {
    return x * 1
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for allowed number 1, got %d", len(findings))
	}
}

// --- WildcardImport (from style_forbidden.go) ---

func TestWildcard_Positive(t *testing.T) {
	findings := runRuleByName(t, "WildcardImport", `
package test
import kotlin.collections.*
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for wildcard import")
	}
}

func TestWildcard_Negative(t *testing.T) {
	findings := runRuleByName(t, "WildcardImport", `
package test
import kotlin.collections.List
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for explicit import, got %d", len(findings))
	}
}

// --- UnusedParameter (from style_unused.go) ---

func TestUnusedParam_Positive(t *testing.T) {
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

func TestUnusedParam_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnusedParameter", `
package test
fun greet(name: String) {
    println(name)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when all parameters are used, got %d", len(findings))
	}
}

// --- MaxLineLength (from style_format.go) ---

func TestMaxLine_Positive(t *testing.T) {
	// 121+ character line (excluding import/package which are skipped by default)
	findings := runRuleByName(t, "MaxLineLength", `package test
val result = aaaaaaaaa + bbbbbbbbb + ccccccccc + ddddddddd + eeeeeeeee + fffffffff + ggggggggg + hhhhhhhhh + iiiiiiiii + jjjjjjjjj
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for line exceeding 120 characters")
	}
}

func TestMaxLine_Negative(t *testing.T) {
	findings := runRuleByName(t, "MaxLineLength", `package test
val x = "short line"
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for short lines, got %d", len(findings))
	}
}
