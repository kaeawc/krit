package rules_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func runPerformanceRuleWithResolver(t *testing.T, ruleName string, code string, resolver typeinfer.TypeResolver) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	for _, r := range v2rules.Registry {
		if r.ID == ruleName {
			dispatcher := rules.NewDispatcherV2([]*v2rules.Rule{r}, resolver)
			cols := dispatcher.Run(file)
			return cols.Findings()
		}
	}
	t.Fatalf("rule %q not found in registry", ruleName)
	return nil
}

func runCouldBeSequenceWithCallTargets(t *testing.T, code string, targetsByName map[string]string) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	fake := oracle.NewFakeOracle()
	fake.CallTargets[file.Path] = map[string]string{}
	file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
		if target := targetsByName[strings.TrimSpace(flatCallNameForTest(file, idx))]; target != "" {
			key := fmt.Sprintf("%d:%d", file.FlatRow(idx)+1, file.FlatCol(idx)+1)
			fake.CallTargets[file.Path][key] = target
		}
	})
	composite := oracle.NewCompositeResolver(fake, resolver)
	for _, r := range v2rules.Registry {
		if r.ID == "CouldBeSequence" {
			dispatcher := rules.NewDispatcherV2([]*v2rules.Rule{r}, composite)
			cols := dispatcher.Run(file)
			return cols.Findings()
		}
	}
	t.Fatal("rule \"CouldBeSequence\" not found in registry")
	return nil
}

func flatCallNameForTest(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 || file.FlatType(idx) != "call_expression" || file.FlatNamedChildCount(idx) == 0 {
		return ""
	}
	first := file.FlatNamedChild(idx, 0)
	text := file.FlatNodeText(first)
	text = strings.TrimSpace(text)
	if dot := strings.LastIndex(text, "."); dot >= 0 {
		text = text[dot+1:]
	}
	if paren := strings.Index(text, "("); paren >= 0 {
		text = text[:paren]
	}
	return strings.TrimSpace(text)
}

// --- SpreadOperator ---

func TestSpreadOperator_Positive(t *testing.T) {
	findings := runRuleByName(t, "SpreadOperator", `
package test
fun foo(vararg items: String) {}
fun bar() {
    val arr = arrayOf("a", "b")
    foo(*arr)
}`)
	if len(findings) == 0 {
		t.Error("SpreadOperator should flag *arr in function call")
	}
}

func TestSpreadOperator_Negative(t *testing.T) {
	findings := runRuleByName(t, "SpreadOperator", `
package test
fun foo(vararg items: String) {}
fun bar() {
    foo(*arrayOf("a", "b"))
}`)
	if len(findings) != 0 {
		t.Errorf("SpreadOperator should not flag *arrayOf(...), got %d findings", len(findings))
	}
}

func TestSpreadOperator_NegativeArrayFactories(t *testing.T) {
	cases := []string{
		`foo(*arrayOf("a", "b"))`,
		`takeInts(*intArrayOf(1, 2))`,
		`foo(*arrayOfNulls<String>(2))`,
		`foo(*emptyArray<String>())`,
	}
	for _, expr := range cases {
		findings := runRuleByName(t, "SpreadOperator", `
package test
fun foo(vararg items: String) {}
fun takeInts(vararg items: Int) {}
fun bar() {
    `+expr+`
}`)
		if len(findings) != 0 {
			t.Errorf("SpreadOperator should not flag %s, got %d findings", expr, len(findings))
		}
	}
}

func TestSpreadOperator_NegativeForwardedVarargParameter(t *testing.T) {
	findings := runRuleByName(t, "SpreadOperator", `
package test
fun sink(vararg items: String) {}
fun forward(vararg items: String) {
    sink(*items)
}`)
	if len(findings) != 0 {
		t.Errorf("SpreadOperator should not flag forwarded vararg parameter, got %d findings", len(findings))
	}
}

func TestSpreadOperator_NegativeComputedCallResult(t *testing.T) {
	findings := runRuleByName(t, "SpreadOperator", `
package test
fun foo(vararg items: String) {}
fun buildItems(): Array<String> = arrayOf("a", "b")
fun bar() {
    foo(*buildItems())
}`)
	if len(findings) != 0 {
		t.Errorf("SpreadOperator should not flag computed call results, got %d findings", len(findings))
	}
}

func TestSpreadOperator_NegativeResolvedArrayFactoryCallTarget(t *testing.T) {
	code := `
package test
import kotlin.arrayOf as makeArray
fun foo(vararg items: String) {}
fun bar() {
    foo(*makeArray("a", "b"))
}`
	file := parseInline(t, code)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	fake := oracle.NewFakeOracle()
	fake.CallTargets[file.Path] = map[string]string{}
	file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
		if strings.Contains(file.FlatNodeText(idx), "makeArray") {
			key := fmt.Sprintf("%d:%d", file.FlatRow(idx)+1, file.FlatCol(idx)+1)
			fake.CallTargets[file.Path][key] = "kotlin.arrayOf"
		}
	})
	var findings []scanner.Finding
	for _, r := range v2rules.Registry {
		if r.ID == "SpreadOperator" {
			dispatcher := rules.NewDispatcherV2([]*v2rules.Rule{r}, oracle.NewCompositeResolver(fake, resolver))
			cols := dispatcher.Run(file)
			findings = cols.Findings()
			break
		}
	}
	if len(findings) != 0 {
		t.Errorf("SpreadOperator should not flag resolved kotlin.arrayOf alias, got %d findings", len(findings))
	}
}

// --- UnnecessaryTemporaryInstantiation ---

func TestUnnecessaryTemporaryInstantiation_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryTemporaryInstantiation", `
package test
fun bar() {
    val s = Integer.valueOf(42).toString()
}`)
	if len(findings) == 0 {
		t.Error("UnnecessaryTemporaryInstantiation should flag Integer.valueOf(x).toString()")
	}
}

func TestUnnecessaryTemporaryInstantiation_QualifiedPositive(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryTemporaryInstantiation", `
package test
fun bar() {
    val s = java.lang.Integer.parseInt("42").toString()
}`)
	if len(findings) == 0 {
		t.Error("UnnecessaryTemporaryInstantiation should flag qualified wrapper conversions")
	}
}

func TestUnnecessaryTemporaryInstantiation_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryTemporaryInstantiation", `
package test
fun bar() {
    val s = 42.toString()
}`)
	if len(findings) != 0 {
		t.Errorf("UnnecessaryTemporaryInstantiation should not flag direct toString(), got %d findings", len(findings))
	}
}

func BenchmarkUnnecessaryTemporaryInstantiation_NoMatch(b *testing.B) {
	var src strings.Builder
	src.WriteString("package test\nfun bar() {\n")
	for i := 0; i < 2000; i++ {
		src.WriteString("    val s = value.toString()\n")
	}
	src.WriteString("}\n")

	dir := b.TempDir()
	path := filepath.Join(dir, "bench.kt")
	if err := os.WriteFile(path, []byte(src.String()), 0644); err != nil {
		b.Fatal(err)
	}
	file, err := scanner.ParseFile(path)
	if err != nil {
		b.Fatal(err)
	}

	var rule *v2rules.Rule
	for _, r := range v2rules.Registry {
		if r.ID == "UnnecessaryTemporaryInstantiation" {
			rule = r
			break
		}
	}
	if rule == nil {
		b.Fatal("rule not found")
	}

	dispatcher := rules.NewDispatcherV2([]*v2rules.Rule{rule})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = dispatcher.Run(file)
	}
}

// --- ArrayPrimitive ---

func TestArrayPrimitive_Positive(t *testing.T) {
	findings := runRuleByName(t, "ArrayPrimitive", `
package test
fun bar() {
    val arr: Array<Int> = arrayOf(1, 2, 3)
}`)
	if len(findings) == 0 {
		t.Error("ArrayPrimitive should flag Array<Int>")
	}
}

func TestArrayPrimitive_FactoryCallPositive(t *testing.T) {
	findings := runRuleByName(t, "ArrayPrimitive", `
package test
fun bar() {
    val arr = arrayOf<Int>(1, 2, 3)
    val empty = emptyArray<Boolean>()
}`)
	if len(findings) != 2 {
		t.Fatalf("ArrayPrimitive should flag primitive array factory calls, got %d", len(findings))
	}
}

func TestArrayPrimitive_Negative(t *testing.T) {
	findings := runRuleByName(t, "ArrayPrimitive", `
package test
fun bar() {
    val arr: IntArray = intArrayOf(1, 2, 3)
}`)
	if len(findings) != 0 {
		t.Errorf("ArrayPrimitive should not flag IntArray, got %d findings", len(findings))
	}
}

func TestArrayPrimitive_NegativeSubstringWrapperName(t *testing.T) {
	findings := runRuleByName(t, "ArrayPrimitive", `
package test
fun bar() {
    val arr: Array<MyIntWrapper> = arrayOf(MyIntWrapper())
}`)
	if len(findings) != 0 {
		t.Errorf("ArrayPrimitive should not flag Array<MyIntWrapper>, got %d findings", len(findings))
	}
}

// --- BitmapDecodeWithoutOptions ---

func TestBitmapDecodeWithoutOptions_Positive(t *testing.T) {
	findings := runRuleByName(t, "BitmapDecodeWithoutOptions", `
package test
import android.graphics.BitmapFactory

fun load(path: String) {
    val bitmap = BitmapFactory.decodeFile(path)
    println(bitmap)
}`)
	if len(findings) == 0 {
		t.Error("BitmapDecodeWithoutOptions should flag BitmapFactory.decodeFile(path)")
	}
}

func TestBitmapDecodeWithoutOptions_Negative(t *testing.T) {
	findings := runRuleByName(t, "BitmapDecodeWithoutOptions", `
package test
import android.graphics.BitmapFactory

fun load(path: String) {
    val options = BitmapFactory.Options().apply { inSampleSize = 2 }
    val bitmap = BitmapFactory.decodeFile(path, options)
    println(bitmap)
}`)
	if len(findings) != 0 {
		t.Errorf("BitmapDecodeWithoutOptions should not flag decodeFile(path, options), got %d findings", len(findings))
	}
}

// --- ForEachOnRange ---

func TestForEachOnRange_Positive(t *testing.T) {
	findings := runRuleByName(t, "ForEachOnRange", `
package test
fun bar() {
    (1..10).forEach { println(it) }
}`)
	if len(findings) == 0 {
		t.Error("ForEachOnRange should flag (1..10).forEach")
	}
}

func TestForEachOnRange_Negative(t *testing.T) {
	findings := runRuleByName(t, "ForEachOnRange", `
package test
fun bar() {
    for (i in 1..10) { println(i) }
}`)
	if len(findings) != 0 {
		t.Errorf("ForEachOnRange should not flag for loop on range, got %d findings", len(findings))
	}
}

// --- CouldBeSequence ---

func TestCouldBeSequence_Positive(t *testing.T) {
	findings := runRuleByName(t, "CouldBeSequence", `
package test
fun bar() {
    val result = listOf(1, 2, 3).filter { it > 1 }.map { it * 2 }.sorted()
}`)
	if len(findings) != 1 {
		t.Errorf("CouldBeSequence should flag chain of 3 collection operations once, got %d", len(findings))
	}
}

func TestCouldBeSequence_PositiveSetSource(t *testing.T) {
	findings := runRuleByName(t, "CouldBeSequence", `
package test
fun bar() {
    val result = setOf(1, 2, 3).filter { it > 1 }.map { it * 2 }.distinct()
}`)
	if len(findings) == 0 {
		t.Error("CouldBeSequence should flag obvious Set chains")
	}
}

func TestCouldBeSequence_Negative(t *testing.T) {
	findings := runRuleByName(t, "CouldBeSequence", `
package test
fun bar() {
    val result = listOf(1, 2, 3).filter { it > 1 }
}`)
	if len(findings) != 0 {
		t.Errorf("CouldBeSequence should not flag single collection operation, got %d findings", len(findings))
	}
}

func TestCouldBeSequence_NegativeAsSequence(t *testing.T) {
	findings := runRuleByName(t, "CouldBeSequence", `
package test
fun bar(items: List<Int>) {
    val result = items.asSequence()
        .filter { it > 1 }
        .map { it * 2 }
        .distinct()
        .map { it.toString() }
        .toList()
}`)
	if len(findings) != 0 {
		t.Errorf("CouldBeSequence should not flag existing sequence chains, got %d findings", len(findings))
	}
}

func TestCouldBeSequence_NegativeMapSource(t *testing.T) {
	findings := runRuleByName(t, "CouldBeSequence", `
package test
fun bar() {
    val result = mapOf("a" to 1, "b" to 2).filter { it.value > 1 }.map { it.key }.sorted()
}`)
	if len(findings) != 0 {
		t.Errorf("CouldBeSequence should not flag Map chains in no-type fallback, got %d findings", len(findings))
	}
}

func TestCouldBeSequence_NegativeCustomFluentApi(t *testing.T) {
	findings := runRuleByName(t, "CouldBeSequence", `
package test

class Pipeline {
    fun filter(block: (Int) -> Boolean) = this
    fun map(block: (Int) -> Int) = this
    fun sorted() = this
}

fun bar() {
    val result = Pipeline().filter { true }.map { it }.sorted()
}`)
	if len(findings) != 0 {
		t.Errorf("CouldBeSequence should not flag custom fluent APIs, got %d findings", len(findings))
	}
}

func TestCouldBeSequence_NegativeResolvedFlow(t *testing.T) {
	resolver := typeinfer.NewFakeResolver()
	resolver.NameTypes["items"] = &typeinfer.ResolvedType{
		Name: "Flow",
		FQN:  "kotlinx.coroutines.flow.Flow",
		Kind: typeinfer.TypeClass,
	}
	findings := runPerformanceRuleWithResolver(t, "CouldBeSequence", `
package test
import kotlinx.coroutines.flow.Flow
fun bar(items: Flow<Int>) {
    val result = items.filter { it > 1 }.map { it * 2 }.take(10)
}`, resolver)
	if len(findings) != 0 {
		t.Errorf("CouldBeSequence should not flag Flow chains, got %d findings", len(findings))
	}
}

func TestCouldBeSequence_WithResolver(t *testing.T) {
	resolver := typeinfer.NewFakeResolver()
	resolver.NameTypes["items"] = &typeinfer.ResolvedType{
		Name: "List",
		FQN:  "kotlin.collections.List",
		Kind: typeinfer.TypeClass,
	}
	findings := runPerformanceRuleWithResolver(t, "CouldBeSequence", `
package test
fun bar(items: List<Int>) {
    val result = items.filter { it > 1 }.map { it * 2 }.sorted()
}`, resolver)
	if len(findings) == 0 {
		t.Error("CouldBeSequence should flag resolved List receiver chains")
	}
}

func TestCouldBeSequence_WithResolverSet(t *testing.T) {
	resolver := typeinfer.NewFakeResolver()
	resolver.NameTypes["items"] = &typeinfer.ResolvedType{
		Name: "Set",
		FQN:  "kotlin.collections.Set",
		Kind: typeinfer.TypeClass,
	}
	findings := runPerformanceRuleWithResolver(t, "CouldBeSequence", `
package test
fun bar(items: Set<Int>) {
    val result = items.filter { it > 1 }.map { it * 2 }.take(10)
}`, resolver)
	if len(findings) == 0 {
		t.Error("CouldBeSequence should flag resolved Set receiver chains")
	}
}

func TestCouldBeSequence_WithOracleCollectionCallTargets(t *testing.T) {
	findings := runCouldBeSequenceWithCallTargets(t, `
package test
fun bar(items: List<Int>) {
    val result = items.filter { it > 1 }.map { it * 2 }.take(10)
}`, map[string]string{
		"filter": "kotlin.collections.filter",
		"map":    "kotlin.collections.map",
		"take":   "kotlin.collections.take",
	})
	if len(findings) == 0 {
		t.Error("CouldBeSequence should flag KAA-confirmed collection chains")
	}
}

func TestCouldBeSequence_WithOracleCustomCallTargets(t *testing.T) {
	findings := runCouldBeSequenceWithCallTargets(t, `
package test
class Query {
    fun filter(block: (Int) -> Boolean) = this
    fun map(block: (Int) -> Int) = this
    fun take(count: Int) = this
}
fun bar(query: Query) {
    val result = query.filter { true }.map { it }.take(10)
}`, map[string]string{
		"filter": "test.Query.filter",
		"map":    "test.Query.map",
		"take":   "test.Query.take",
	})
	if len(findings) != 0 {
		t.Errorf("CouldBeSequence should not flag oracle-confirmed custom fluent APIs, got %d findings", len(findings))
	}
}

// --- UnnecessaryInitOnArray ---

func TestUnnecessaryInitOnArray_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryInitOnArray", `
package test
fun bar() {
    val arr = IntArray(10) { 0 }
}`)
	if len(findings) == 0 {
		t.Error("UnnecessaryInitOnArray should flag IntArray(n) { 0 }")
	}
}

func TestUnnecessaryInitOnArray_BooleanPositive(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryInitOnArray", `
package test
fun bar() {
    val arr = BooleanArray(5) { false }
}`)
	if len(findings) == 0 {
		t.Error("UnnecessaryInitOnArray should flag BooleanArray(n) { false }")
	}
}

func TestUnnecessaryInitOnArray_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryInitOnArray", `
package test
fun bar() {
    val arr = IntArray(10) { it * 2 }
}`)
	if len(findings) != 0 {
		t.Errorf("UnnecessaryInitOnArray should not flag non-default init, got %d findings", len(findings))
	}
}

// --- UnnecessaryPartOfBinaryExpression ---

func TestUnnecessaryPartOfBinaryExpression_AndTrue(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryPartOfBinaryExpression", `
package test
fun bar() {
    val x = someCondition() && true
}`)
	if len(findings) == 0 {
		t.Error("UnnecessaryPartOfBinaryExpression should flag x && true")
	}
}

func TestUnnecessaryPartOfBinaryExpression_OrFalse(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryPartOfBinaryExpression", `
package test
fun bar() {
    val x = someCondition() || false
}`)
	if len(findings) == 0 {
		t.Error("UnnecessaryPartOfBinaryExpression should flag x || false")
	}
}

func TestUnnecessaryPartOfBinaryExpression_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryPartOfBinaryExpression", `
package test
fun bar() {
    val x = a && b
}`)
	if len(findings) != 0 {
		t.Errorf("UnnecessaryPartOfBinaryExpression should not flag a && b, got %d findings", len(findings))
	}
}

// --- UnnecessaryTypeCasting ---

func TestUnnecessaryTypeCasting_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryTypeCasting", `
package test
fun bar() {
    val x: String = "hello"
    val y: String = x as String
}`)
	if len(findings) == 0 {
		t.Error("UnnecessaryTypeCasting should flag casting String to String")
	}
}

func TestUnnecessaryTypeCasting_SafeCastNullCheckPositive(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryTypeCasting", `
package test
class Foo
fun f(value: Any) {
    if (value as? String != null) {
        println(value)
    }
    if (null != value as? Foo) {
        println(value)
    }
}`)
	if len(findings) != 2 {
		t.Fatalf("UnnecessaryTypeCasting should flag both safe-cast null checks, got %d findings", len(findings))
	}
	for _, finding := range findings {
		if finding.Fix == nil {
			t.Fatalf("expected safe-cast null check finding to include a fix: %#v", finding)
		}
	}
}

func TestUnnecessaryTypeCasting_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryTypeCasting", `
package test
fun bar() {
    val x: Any = "hello"
    val y = x as String
}`)
	if len(findings) != 0 {
		t.Errorf("UnnecessaryTypeCasting should not flag casting Any to String, got %d findings", len(findings))
	}
}

func TestUnnecessaryTypeCasting_SafeCastStandaloneNegative(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryTypeCasting", `
package test
fun f(value: Any) {
    val maybe = value as? String
    if (maybe != null) {
        println(maybe)
    }
}`)
	if len(findings) != 0 {
		t.Errorf("UnnecessaryTypeCasting should not flag standalone safe casts, got %d findings", len(findings))
	}
}

func TestUnnecessaryTypeCasting_NullableSafeCastTargetNegative(t *testing.T) {
	findings := runRuleByName(t, "UnnecessaryTypeCasting", `
package test
fun f(value: Any?) {
    if (value as? String? != null) {
        println(value)
    }
}`)
	if len(findings) != 0 {
		t.Errorf("UnnecessaryTypeCasting should not rewrite nullable safe-cast targets, got %d findings", len(findings))
	}
}
