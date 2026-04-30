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

// --- ClassNaming ---

func TestClassNaming_FlagsBadName(t *testing.T) {
	findings := runRuleByName(t, "ClassNaming", `
package test
class my_class
`)
	if len(findings) == 0 {
		t.Error("expected ClassNaming to flag lowercase/underscore class name")
	}
}

func TestClassNaming_AcceptsPascalCase(t *testing.T) {
	findings := runRuleByName(t, "ClassNaming", `
package test
class MyClass
`)
	for _, f := range findings {
		if f.Rule == "ClassNaming" {
			t.Errorf("ClassNaming should accept PascalCase name, got: %s", f.Message)
		}
	}
}

func TestClassNaming_SkipsBacktickQuoted(t *testing.T) {
	findings := runRuleByName(t, "ClassNaming", "package test\nclass `my test class`\n")
	for _, f := range findings {
		if f.Rule == "ClassNaming" {
			t.Errorf("ClassNaming should skip backtick-quoted names, got: %s", f.Message)
		}
	}
}

// --- FunctionNaming ---

func TestFunctionNaming_FlagsPascalCase(t *testing.T) {
	findings := runRuleByName(t, "FunctionNaming", `
package test
fun MyFunction() {}
`)
	if len(findings) == 0 {
		t.Error("expected FunctionNaming to flag PascalCase function name")
	}
}

func TestFunctionNaming_AcceptsCamelCase(t *testing.T) {
	findings := runRuleByName(t, "FunctionNaming", `
package test
fun myFunction() {}
`)
	for _, f := range findings {
		if f.Rule == "FunctionNaming" {
			t.Errorf("FunctionNaming should accept camelCase, got: %s", f.Message)
		}
	}
}

func TestFunctionNaming_SkipsComposable(t *testing.T) {
	findings := runRuleByName(t, "FunctionNaming", `
package test
@Composable
fun MyScreen() {}
`)
	for _, f := range findings {
		if f.Rule == "FunctionNaming" {
			t.Errorf("FunctionNaming should skip @Composable functions, got: %s", f.Message)
		}
	}
}

func TestFunctionNaming_SkipsBacktickQuoted(t *testing.T) {
	findings := runRuleByName(t, "FunctionNaming", "package test\nfun `my test function`() {}\n")
	for _, f := range findings {
		if f.Rule == "FunctionNaming" {
			t.Errorf("FunctionNaming should skip backtick-quoted names, got: %s", f.Message)
		}
	}
}

// --- VariableNaming ---

func TestVariableNaming_FlagsUppercaseLocal(t *testing.T) {
	findings := runRuleByName(t, "VariableNaming", `
package test
fun example() {
    val MyVar = 1
}
`)
	if len(findings) == 0 {
		t.Error("expected VariableNaming to flag uppercase local variable")
	}
}

func TestVariableNaming_AcceptsCamelCase(t *testing.T) {
	findings := runRuleByName(t, "VariableNaming", `
package test
fun example() {
    val myVar = 1
}
`)
	for _, f := range findings {
		if f.Rule == "VariableNaming" {
			t.Errorf("VariableNaming should accept camelCase local var, got: %s", f.Message)
		}
	}
}

func TestVariableNaming_HonorsPrivateVariablePattern(t *testing.T) {
	// PrivateVariablePattern was previously a dead config — exposed in
	// zz_meta but never consulted by the check. The Kotlin compiler
	// rejects visibility modifiers on local vars, but tree-sitter parses
	// them leniently — so a half-finished `private val Foo = ...` inside
	// a function still has a "private" modifier on the property_declaration
	// node, and the wired check should validate it against
	// PrivateVariablePattern instead of the public Pattern.
	var rule *rules.VariableNamingRule
	for _, candidate := range v2rules.Registry {
		if candidate.ID == "VariableNaming" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.VariableNamingRule)
			if !ok {
				t.Fatalf("expected VariableNamingRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("VariableNaming rule not registered")
	}
	original := rule.PrivateVariablePattern
	defer func() { rule.PrivateVariablePattern = original }()

	rule.PrivateVariablePattern = registry.CompileAnchoredPattern(
		"VariableNaming", "privateVariablePattern", "_[a-z][A-Za-z0-9]*")
	if rule.PrivateVariablePattern == nil {
		t.Fatal("failed to compile test pattern")
	}

	if findings := runRuleByName(t, "VariableNaming", `
package test
fun example() {
    private val plain = 1
}
`); len(findings) == 0 {
		t.Fatal("expected finding when private local doesn't match PrivateVariablePattern")
	}

	// Permissive private pattern — finding goes away.
	rule.PrivateVariablePattern = registry.CompileAnchoredPattern(
		"VariableNaming", "privateVariablePattern", "_?[a-z][A-Za-z0-9]*")
	if findings := runRuleByName(t, "VariableNaming", `
package test
fun example() {
    private val plain = 1
}
`); len(findings) != 0 {
		t.Fatalf("expected no findings under permissive PrivateVariablePattern, got %d", len(findings))
	}
}

func TestVariableNaming_AcceptsDiscardLocal(t *testing.T) {
	findings := runRuleByName(t, "VariableNaming", `
package test
fun example() {
    val _ = compute()
}
fun compute(): String = "value"
`)
	for _, f := range findings {
		if f.Rule == "VariableNaming" {
			t.Errorf("VariableNaming should accept Kotlin discard local '_', got: %s", f.Message)
		}
	}
}

func TestVariableNaming_SkipsNestedObjectMemberProperties(t *testing.T) {
	findings := runRuleByName(t, "VariableNaming", `
package test
fun install(view: View) {
    view.setOnClickListener(object : Listener {
        private val DEBUG_TAP_TARGET = 8
        override fun onClick(view: View) = Unit
    })
}
class View {
    fun setOnClickListener(listener: Listener) = Unit
}
interface Listener {
    fun onClick(view: View)
}
`)
	for _, f := range findings {
		if f.Rule == "VariableNaming" {
			t.Errorf("VariableNaming should ignore member properties in a nested object, got: %s", f.Message)
		}
	}
}

// --- EnumNaming ---

func TestEnumNaming_FlagsLowercaseEntry(t *testing.T) {
	findings := runRuleByName(t, "EnumNaming", `
package test
enum class Color {
    red,
    green
}
`)
	if len(findings) == 0 {
		t.Error("expected EnumNaming to flag lowercase enum entries")
	}
}

func TestEnumNaming_AcceptsPascalCase(t *testing.T) {
	findings := runRuleByName(t, "EnumNaming", `
package test
enum class Color {
    Red,
    Green
}
`)
	for _, f := range findings {
		if f.Rule == "EnumNaming" {
			t.Errorf("EnumNaming should accept PascalCase entries, got: %s", f.Message)
		}
	}
}

// --- PackageNaming ---

func TestPackageNaming_FlagsUppercase(t *testing.T) {
	findings := runRuleByName(t, "PackageNaming", `
package com.MyApp.feature
class Foo
`)
	if len(findings) == 0 {
		t.Error("expected PackageNaming to flag uppercase package segment")
	}
}

func TestPackageNaming_AcceptsLowercase(t *testing.T) {
	findings := runRuleByName(t, "PackageNaming", `
package com.myapp.feature
class Foo
`)
	for _, f := range findings {
		if f.Rule == "PackageNaming" {
			t.Errorf("PackageNaming should accept lowercase package, got: %s", f.Message)
		}
	}
}

// --- TopLevelPropertyNaming ---

func TestTopLevelPropertyNaming_FlagsBadConst(t *testing.T) {
	findings := runRuleByName(t, "TopLevelPropertyNaming", `
package test
const val myConst = 42
`)
	if len(findings) == 0 {
		t.Error("expected TopLevelPropertyNaming to flag non-SCREAMING_SNAKE const")
	}
}

func TestTopLevelPropertyNaming_AcceptsScreamingSnakeConst(t *testing.T) {
	findings := runRuleByName(t, "TopLevelPropertyNaming", `
package test
const val MY_CONST = 42
`)
	for _, f := range findings {
		if f.Rule == "TopLevelPropertyNaming" {
			t.Errorf("TopLevelPropertyNaming should accept SCREAMING_SNAKE const, got: %s", f.Message)
		}
	}
}

func TestTopLevelPropertyNaming_AcceptsPascalCaseConst(t *testing.T) {
	findings := runRuleByName(t, "TopLevelPropertyNaming", `
package test
const val MillisToNanos = 1_000_000
`)
	for _, f := range findings {
		if f.Rule == "TopLevelPropertyNaming" {
			t.Errorf("TopLevelPropertyNaming should accept initial-uppercase const names, got: %s", f.Message)
		}
	}
}

func TestTopLevelPropertyNaming_AcceptsCamelCaseNonConst(t *testing.T) {
	findings := runRuleByName(t, "TopLevelPropertyNaming", `
package test
val myProperty = "hello"
`)
	for _, f := range findings {
		if f.Rule == "TopLevelPropertyNaming" {
			t.Errorf("TopLevelPropertyNaming should accept camelCase non-const, got: %s", f.Message)
		}
	}
}

func TestTopLevelPropertyNaming_AcceptsPrivateBackingProperty(t *testing.T) {
	findings := runRuleByName(t, "TopLevelPropertyNaming", `
package test
import kotlinx.coroutines.flow.MutableStateFlow

val state = _state
private val _state = MutableStateFlow(0)
`)
	for _, f := range findings {
		if f.Rule == "TopLevelPropertyNaming" {
			t.Errorf("TopLevelPropertyNaming should accept private backing property, got: %s", f.Message)
		}
	}
}

func TestTopLevelPropertyNaming_FlagsPascalCaseNonConst(t *testing.T) {
	findings := runRuleByName(t, "TopLevelPropertyNaming", `
package test
val MyProperty = "hello"
`)
	if len(findings) == 0 {
		t.Error("expected TopLevelPropertyNaming to flag PascalCase non-const top-level property")
	}
}

func TestTopLevelPropertyNaming_HonorsPrivatePropertyPattern(t *testing.T) {
	// PrivatePropertyPattern was previously a dead config field — exposed
	// in zz_meta but never consulted by the check. Configure it via the
	// rule pointer and verify private properties are validated against
	// the configured pattern instead of the public PropertyPattern.
	var rule *rules.TopLevelPropertyNamingRule
	for _, candidate := range v2rules.Registry {
		if candidate.ID == "TopLevelPropertyNaming" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.TopLevelPropertyNamingRule)
			if !ok {
				t.Fatalf("expected TopLevelPropertyNamingRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("TopLevelPropertyNaming rule not registered")
	}
	original := rule.PrivatePropertyPattern
	defer func() { rule.PrivatePropertyPattern = original }()

	// Pattern: must start with `_`, then identifier (allow only this shape
	// for private top-level properties).
	rule.PrivatePropertyPattern = registry.CompileAnchoredPattern(
		"TopLevelPropertyNaming", "privatePropertyPattern", "_[a-z][A-Za-z0-9]*")
	if rule.PrivatePropertyPattern == nil {
		t.Fatal("failed to compile test pattern")
	}

	// Private property starting with `_` is the only shape that matches —
	// but the `_`+private special-case bails before we get to the pattern,
	// so this passes regardless. Construct a private property without `_`
	// prefix to exercise the pattern check.
	if findings := runRuleByName(t, "TopLevelPropertyNaming", `
package test
private val plainPrivate = 1
`); len(findings) == 0 {
		t.Fatal("expected finding when private property doesn't match configured PrivatePropertyPattern")
	}

	// Reset to a permissive pattern that accepts both shapes — no findings.
	rule.PrivatePropertyPattern = registry.CompileAnchoredPattern(
		"TopLevelPropertyNaming", "privatePropertyPattern", "_?[a-z][A-Za-z0-9]*")
	if findings := runRuleByName(t, "TopLevelPropertyNaming", `
package test
private val plainPrivate = 1
`); len(findings) != 0 {
		t.Fatalf("expected no findings under permissive PrivatePropertyPattern, got %d", len(findings))
	}
}

func TestTopLevelPropertyNaming_UsesLocalASTOnly(t *testing.T) {
	rule := buildRuleIndex()["TopLevelPropertyNaming"]
	if rule == nil {
		t.Fatal("TopLevelPropertyNaming rule is not registered")
	}
	if rule.Needs != 0 {
		t.Fatalf("TopLevelPropertyNaming should remain AST-only, got needs %v", rule.Needs)
	}
	if rule.OracleCallTargets != nil || rule.OracleDeclarationNeeds != nil || rule.Oracle != nil {
		t.Fatal("TopLevelPropertyNaming should not declare oracle metadata")
	}
}

// TestNamingMetaRegexDefaultsMatchRegistry guards against documentation
// drift: when the registry initializes a naming rule with a regex, the
// rule's zz_meta_naming_gen.go Default string for that option must compile
// (under the registry's anchoring) to an equivalent regex. Otherwise users
// see one default in the docs / config emitter and another at runtime —
// the precise class of bug this test was written to prevent.
func TestNamingMetaRegexDefaultsMatchRegistry(t *testing.T) {
	cases := []struct {
		ruleID     string
		optionName string
		runtime    string
	}{
		{"ObjectPropertyNaming", "constantPattern", `^[A-Z][_A-Z0-9]*$`},
		{"ObjectPropertyNaming", "propertyPattern", `^[a-z][A-Za-z0-9]*$`},
		{"TopLevelPropertyNaming", "constantPattern", `^[A-Z][_A-Za-z0-9]*$`},
		{"TopLevelPropertyNaming", "propertyPattern", `^[a-z][A-Za-z0-9]*$`},
	}
	for _, tc := range cases {
		t.Run(tc.ruleID+"/"+tc.optionName, func(t *testing.T) {
			rule := buildRuleIndex()[tc.ruleID]
			if rule == nil {
				t.Fatalf("rule %q is not registered", tc.ruleID)
			}
			impl, ok := rule.Implementation.(interface {
				Meta() registry.RuleDescriptor
			})
			if !ok {
				t.Fatalf("rule %q does not expose Meta()", tc.ruleID)
			}
			meta := impl.Meta()
			var optDefault string
			var found bool
			for _, opt := range meta.Options {
				if opt.Name == tc.optionName {
					if s, ok := opt.Default.(string); ok {
						optDefault = s
					}
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("option %q not found on rule %q", tc.optionName, tc.ruleID)
			}
			compiled := registry.CompileAnchoredPattern(tc.ruleID, tc.optionName, optDefault)
			if compiled == nil {
				t.Fatalf("zz_meta default %q failed to compile", optDefault)
			}
			if compiled.String() != tc.runtime {
				t.Fatalf("zz_meta default %q for %s.%s anchors to %q, registry uses %q (drift)",
					optDefault, tc.ruleID, tc.optionName, compiled.String(), tc.runtime)
			}
		})
	}
}

// --- MemberNameEqualsClassName ---

func TestMemberNameEqualsClassName_FlagsSameNameFunction(t *testing.T) {
	findings := runRuleByName(t, "MemberNameEqualsClassName", `
package test
class Foo {
    fun Foo() {}
}
`)
	if len(findings) == 0 {
		t.Error("expected MemberNameEqualsClassName to flag function named same as class")
	}
}

func TestMemberNameEqualsClassName_AcceptsDifferentName(t *testing.T) {
	findings := runRuleByName(t, "MemberNameEqualsClassName", `
package test
class Foo {
    fun bar() {}
}
`)
	for _, f := range findings {
		if f.Rule == "MemberNameEqualsClassName" {
			t.Errorf("MemberNameEqualsClassName should accept different name, got: %s", f.Message)
		}
	}
}

func TestMemberNameEqualsClassName_FlagsSameNameProperty(t *testing.T) {
	findings := runRuleByName(t, "MemberNameEqualsClassName", `
package test
class Foo {
    val Foo = "bar"
}
`)
	if len(findings) == 0 {
		t.Error("expected MemberNameEqualsClassName to flag property named same as class")
	}
}

// --- BooleanPropertyNaming ---

func TestNaming_BooleanProperty_FlagsMissingPrefix(t *testing.T) {
	findings := runRuleByName(t, "BooleanPropertyNaming", `
package test
class Foo {
    val enabled: Boolean = true
}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "BooleanPropertyNaming" {
			found = true
		}
	}
	if !found {
		t.Error("expected BooleanPropertyNaming to flag Boolean property without is/has/are prefix")
	}
}

func TestNaming_BooleanProperty_AcceptsIsPrefix(t *testing.T) {
	findings := runRuleByName(t, "BooleanPropertyNaming", `
package test
class Foo {
    val isEnabled: Boolean = true
}
`)
	for _, f := range findings {
		if f.Rule == "BooleanPropertyNaming" {
			t.Errorf("BooleanPropertyNaming should accept 'is' prefix, got: %s", f.Message)
		}
	}
}

func TestNaming_BooleanProperty_IgnoresTestAndLocalProperties(t *testing.T) {
	testCode := `
package test
class FooTest {
    val enabled: Boolean = true
}
`
	if findings := runRuleByNameOnPath(t, "BooleanPropertyNaming", "src/test/kotlin/FooTest.kt", testCode); len(findings) != 0 {
		t.Fatalf("expected no findings for test boolean properties, got %d", len(findings))
	}
	localCode := `
package test
fun main() {
    val enabled: Boolean = true
}
`
	if findings := runRuleByName(t, "BooleanPropertyNaming", localCode); len(findings) != 0 {
		t.Fatalf("expected no findings for local boolean vals, got %d", len(findings))
	}
}

func TestNaming_BooleanProperty_IgnoresNonBooleanDeclaredType(t *testing.T) {
	findings := runRuleByName(t, "BooleanPropertyNaming", `
package test
class Foo {
    val flag: Any = true
    val marker: Any = false
    val token: Comparable<*> = true
}
`)
	for _, f := range findings {
		if f.Rule == "BooleanPropertyNaming" {
			t.Errorf("BooleanPropertyNaming should not flag property with explicit non-Boolean declared type, got: %s", f.Message)
		}
	}
}

func TestNaming_BooleanProperty_FlagsNullableBoolean(t *testing.T) {
	findings := runRuleByName(t, "BooleanPropertyNaming", `
package test
class Foo {
    val enabled: Boolean? = null
}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "BooleanPropertyNaming" {
			found = true
		}
	}
	if !found {
		t.Error("expected BooleanPropertyNaming to flag Boolean? property without is/has/are prefix")
	}
}

func TestNaming_BooleanProperty_IgnoresNonBooleanInitializerBodies(t *testing.T) {
	findings := runRuleByName(t, "BooleanPropertyNaming", `
package test
class Foo {
    val progress: Flow<State> = upstream.map {
        false
    }
}
class Flow<T> {
    fun <R> map(block: (T) -> R): Flow<R> = Flow()
}
class State
val upstream = Flow<State>()
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for non-Boolean property whose initializer contains false, got %d", len(findings))
	}
}

// --- ConstructorParameterNaming ---

func TestNaming_ConstructorParameter_FlagsBadName(t *testing.T) {
	findings := runRuleByName(t, "ConstructorParameterNaming", `
package test
class Foo(val MyParam: Int)
`)
	found := false
	for _, f := range findings {
		if f.Rule == "ConstructorParameterNaming" {
			found = true
		}
	}
	if !found {
		t.Error("expected ConstructorParameterNaming to flag PascalCase constructor parameter")
	}
}

func TestNaming_ConstructorParameter_AcceptsCamelCase(t *testing.T) {
	findings := runRuleByName(t, "ConstructorParameterNaming", `
package test
class Foo(val myParam: Int)
`)
	for _, f := range findings {
		if f.Rule == "ConstructorParameterNaming" {
			t.Errorf("ConstructorParameterNaming should accept camelCase, got: %s", f.Message)
		}
	}
}

func TestNaming_ConstructorParameter_HonorsPrivateParameterPattern(t *testing.T) {
	// PrivateParameterPattern was previously a dead config — exposed in
	// zz_meta but never consulted by the check. Configure a strict
	// pattern via the rule pointer and verify private constructor
	// parameters are validated against it instead of the public Pattern.
	var rule *rules.ConstructorParameterNamingRule
	for _, candidate := range v2rules.Registry {
		if candidate.ID == "ConstructorParameterNaming" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.ConstructorParameterNamingRule)
			if !ok {
				t.Fatalf("expected ConstructorParameterNamingRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("ConstructorParameterNaming rule not registered")
	}
	original := rule.PrivateParameterPattern
	defer func() { rule.PrivateParameterPattern = original }()

	// Require private params to start with `_`.
	rule.PrivateParameterPattern = registry.CompileAnchoredPattern(
		"ConstructorParameterNaming", "privateParameterPattern", "_[a-z][A-Za-z0-9]*")
	if rule.PrivateParameterPattern == nil {
		t.Fatal("failed to compile test pattern")
	}

	// Private without `_` prefix → flagged under PrivateParameterPattern.
	if findings := runRuleByName(t, "ConstructorParameterNaming", `
package test
class Foo(private val plain: Int)
`); len(findings) == 0 {
		t.Fatal("expected finding when private param doesn't match PrivateParameterPattern")
	}

	// Public param with the same shape passes the public Pattern, so no finding.
	if findings := runRuleByName(t, "ConstructorParameterNaming", `
package test
class Foo(val plain: Int)
`); len(findings) != 0 {
		t.Fatalf("expected no findings for public-passing 'plain', got %d", len(findings))
	}

	// Permissive private pattern — finding goes away.
	rule.PrivateParameterPattern = registry.CompileAnchoredPattern(
		"ConstructorParameterNaming", "privateParameterPattern", "_?[a-z][A-Za-z0-9]*")
	if findings := runRuleByName(t, "ConstructorParameterNaming", `
package test
class Foo(private val plain: Int)
`); len(findings) != 0 {
		t.Fatalf("expected no findings under permissive PrivateParameterPattern, got %d", len(findings))
	}
}

// --- FunctionNameMaxLength ---

func TestNaming_FunctionNameMaxLength_FlagsTooLong(t *testing.T) {
	findings := runRuleByName(t, "FunctionNameMaxLength", `
package test
fun thisIsAnExtremelyLongFunctionNameThatExceedsTheLimit() {}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "FunctionNameMaxLength" {
			found = true
		}
	}
	if !found {
		t.Error("expected FunctionNameMaxLength to flag function name exceeding max length")
	}
}

func TestNaming_FunctionNameMaxLength_AcceptsShortName(t *testing.T) {
	findings := runRuleByName(t, "FunctionNameMaxLength", `
package test
fun doStuff() {}
`)
	for _, f := range findings {
		if f.Rule == "FunctionNameMaxLength" {
			t.Errorf("FunctionNameMaxLength should accept short name, got: %s", f.Message)
		}
	}
}

func TestNaming_FunctionNameMaxLength_IgnoresTestNames(t *testing.T) {
	findings := runRuleByNameOnPath(t, "FunctionNameMaxLength", "src/test/kotlin/FooTest.kt", `
package test

class FooTest {
    @Test
    fun veryLongDescriptiveTestNameThatExplainsBehavior() {}
}
`)
	for _, f := range findings {
		if f.Rule == "FunctionNameMaxLength" {
			t.Errorf("FunctionNameMaxLength should ignore test function names, got: %s", f.Message)
		}
	}
}

// --- FunctionNameMinLength ---

func TestNaming_FunctionNameMinLength_FlagsTooShort(t *testing.T) {
	findings := runRuleByName(t, "FunctionNameMinLength", `
package test
fun x() {}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "FunctionNameMinLength" {
			found = true
		}
	}
	if !found {
		t.Error("expected FunctionNameMinLength to flag single-char function name")
	}
}

func TestNaming_FunctionNameMinLength_AcceptsLongEnough(t *testing.T) {
	findings := runRuleByName(t, "FunctionNameMinLength", `
package test
fun run() {}
`)
	for _, f := range findings {
		if f.Rule == "FunctionNameMinLength" {
			t.Errorf("FunctionNameMinLength should accept 3-char name, got: %s", f.Message)
		}
	}
}

func TestNaming_FunctionNameMinLength_AcceptsLoggerShorthand(t *testing.T) {
	findings := runRuleByName(t, "FunctionNameMinLength", `
package test
interface Logger
fun Logger.d(message: String) {}
fun Logger.e(message: String) {}
fun Logger.i(message: String) {}
fun Logger.v(message: String) {}
fun Logger.w(message: String) {}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for conventional logger shorthand functions, got %d", len(findings))
	}
}

// --- FunctionParameterNaming ---

func TestNaming_FunctionParameter_FlagsBadName(t *testing.T) {
	findings := runRuleByName(t, "FunctionParameterNaming", `
package test
fun doStuff(MyParam: Int) {}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "FunctionParameterNaming" {
			found = true
		}
	}
	if !found {
		t.Error("expected FunctionParameterNaming to flag PascalCase parameter")
	}
}

func TestNaming_FunctionParameter_AcceptsCamelCase(t *testing.T) {
	findings := runRuleByName(t, "FunctionParameterNaming", `
package test
fun doStuff(myParam: Int) {}
`)
	for _, f := range findings {
		if f.Rule == "FunctionParameterNaming" {
			t.Errorf("FunctionParameterNaming should accept camelCase, got: %s", f.Message)
		}
	}
}

func TestNaming_FunctionParameter_SkipsNestedLambdaParameter(t *testing.T) {
	findings := runRuleByName(t, "FunctionParameterNaming", `
package test
fun doStuff(outerParam: Int) {
    listOf(1).forEach { BadName -> println(BadName) }
}
`)
	for _, f := range findings {
		if f.Rule == "FunctionParameterNaming" {
			t.Errorf("FunctionParameterNaming should skip lambda parameters inside a function, got: %s", f.Message)
		}
	}
}

// --- InvalidPackageDeclaration ---

func TestNaming_InvalidPackageDeclaration_FlagsMismatch(t *testing.T) {
	// The file is written to a temp dir that won't match "com.example.feature"
	findings := runRuleByName(t, "InvalidPackageDeclaration", `
package com.example.feature
class Foo
`)
	found := false
	for _, f := range findings {
		if f.Rule == "InvalidPackageDeclaration" {
			found = true
		}
	}
	if !found {
		t.Error("expected InvalidPackageDeclaration to flag package not matching directory")
	}
}

func TestNaming_InvalidPackageDeclaration_AcceptsNoPackage(t *testing.T) {
	// No package declaration at all — nothing to flag
	findings := runRuleByName(t, "InvalidPackageDeclaration", `
class Foo
`)
	for _, f := range findings {
		if f.Rule == "InvalidPackageDeclaration" {
			t.Errorf("InvalidPackageDeclaration should not flag when there is no package, got: %s", f.Message)
		}
	}
}

// --- LambdaParameterNaming ---

func TestNaming_LambdaParameter_FlagsBadName(t *testing.T) {
	findings := runRuleByName(t, "LambdaParameterNaming", `
package test
fun example() {
    val list = listOf(1, 2, 3)
    list.forEach { BadName -> println(BadName) }
}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "LambdaParameterNaming" {
			found = true
		}
	}
	if !found {
		t.Error("expected LambdaParameterNaming to flag PascalCase lambda parameter")
	}
}

func TestNaming_LambdaParameter_AcceptsCamelCase(t *testing.T) {
	findings := runRuleByName(t, "LambdaParameterNaming", `
package test
fun example() {
    val list = listOf(1, 2, 3)
    list.forEach { item -> println(item) }
}
`)
	for _, f := range findings {
		if f.Rule == "LambdaParameterNaming" {
			t.Errorf("LambdaParameterNaming should accept camelCase, got: %s", f.Message)
		}
	}
}

// --- NoNameShadowing ---

func TestNaming_NoNameShadowing_FlagsShadow(t *testing.T) {
	findings := runRuleByName(t, "NoNameShadowing", `
package test
fun example() {
    val name = "outer"
    run {
        val name = "inner"
        println(name)
    }
}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "NoNameShadowing" {
			found = true
		}
	}
	if !found {
		t.Error("expected NoNameShadowing to flag inner 'name' shadowing outer 'name'")
	}
}

func TestNaming_NoNameShadowing_AcceptsDistinctNames(t *testing.T) {
	findings := runRuleByName(t, "NoNameShadowing", `
package test
fun example() {
    val name = "outer"
    run {
        val other = "inner"
        println(other)
    }
}
`)
	for _, f := range findings {
		if f.Rule == "NoNameShadowing" {
			t.Errorf("NoNameShadowing should not flag distinct names, got: %s", f.Message)
		}
	}
}

func TestNaming_NoNameShadowing_SkipsSuppressedAndSelfAlias(t *testing.T) {
	findings := runRuleByName(t, "NoNameShadowing", `
package test

val context = "global"

fun example(options: String, pathSegments: List<String>) {
    @Suppress("NAME_SHADOWING")
    var options = options
    val pathSegments = pathSegments
    println(options)
    println(pathSegments)
}

fun acceptsContext(context: String) {
    println(context)
}

fun catchFallback() {
    try {
        val result = "ok"
        println(result)
    } catch (e: Exception) {
        val result = "fallback"
        println(result)
    }
}
`)
	for _, f := range findings {
		if f.Rule == "NoNameShadowing" {
			t.Fatalf("NoNameShadowing should ignore local suppression and self-alias narrowing, got: %s", f.Message)
		}
	}
}

func TestNaming_NoNameShadowing_SkipsClassPropertyShadowingInMemberFunction(t *testing.T) {
	findings := runRuleByName(t, "NoNameShadowing", `
package test
class Outer(val name: String) {
    inner class Inner {
        fun example(name: String) {
            println(name)
        }
    }
}
`)
	for _, f := range findings {
		if f.Rule == "NoNameShadowing" {
			t.Errorf("NoNameShadowing should ignore member function params that match class constructor params, got: %s", f.Message)
		}
	}
}

func TestNaming_NoNameShadowing_SkipsConstructorBackedClassProperty(t *testing.T) {
	findings := runRuleByName(t, "NoNameShadowing", `
package test
class NameAllocator(allocatedNames: Set<String>) {
    private val allocatedNames =
        mutableMapOf<String, Unit>().apply {
            for (allocated in allocatedNames) {
                put(allocated, Unit)
            }
        }
}
`)
	for _, f := range findings {
		if f.Rule == "NoNameShadowing" {
			t.Fatalf("NoNameShadowing should ignore class properties backed by same-named constructor params, got: %s", f.Message)
		}
	}
}

func TestNaming_NoNameShadowing_SkipsOverrideCallbackParameters(t *testing.T) {
	findings := runRuleByName(t, "NoNameShadowing", `
package test

interface Clicker {
    fun onClick(view: String)
}

class Example(val view: String) {
    val clicker = object : Clicker {
        override fun onClick(view: String) = Unit
    }
}
`)
	for _, f := range findings {
		if f.Rule == "NoNameShadowing" {
			t.Fatalf("NoNameShadowing should ignore override callback params, got: %s", f.Message)
		}
	}
}

func TestNaming_NoNameShadowing_SkipsLambdaDestructuringBindings(t *testing.T) {
	findings := runRuleByName(t, "NoNameShadowing", `
package test

class Recipient

fun consume(recipient: Recipient, pairs: List<Pair<Recipient, Boolean>>) {
    pairs.forEach { (recipient, notAllowed) ->
        println(recipient)
        println(notAllowed)
    }
}
`)
	for _, f := range findings {
		if f.Rule == "NoNameShadowing" {
			t.Fatalf("NoNameShadowing should ignore lambda destructuring bindings, got: %s", f.Message)
		}
	}
}

func TestNaming_NoNameShadowing_SkipsFunctionTypeParameterLabels(t *testing.T) {
	findings := runRuleByName(t, "NoNameShadowing", `
package test

class Canvas
class Drawable

class Wrapper(canvas: Canvas, private val drawFn: (wrapped: Drawable, canvas: Canvas) -> Unit)
`)
	for _, f := range findings {
		if f.Rule == "NoNameShadowing" {
			t.Fatalf("NoNameShadowing should ignore function type parameter labels, got: %s", f.Message)
		}
	}
}

func TestNaming_NoNameShadowing_SkipsOverrideParamsShadowingOuterLocals(t *testing.T) {
	findings := runRuleByName(t, "NoNameShadowing", `
package test

class Animation

interface AnimationListener {
    fun onAnimationStart(animation: Animation?)
    fun onAnimationRepeat(animation: Animation?)
    fun onAnimationEnd(animation: Animation?)
}

class Example {
    fun hide() {
        val animation = Animation()
        val listener = object : AnimationListener {
            override fun onAnimationStart(animation: Animation?) = Unit
            override fun onAnimationRepeat(animation: Animation?) = Unit
            override fun onAnimationEnd(animation: Animation?) = Unit
        }
        println(listener)
        println(animation)
    }
}
`)
	for _, f := range findings {
		if f.Rule == "NoNameShadowing" {
			t.Fatalf("NoNameShadowing should ignore override params shadowing outer locals, got: %s", f.Message)
		}
	}
}

func TestNaming_NoNameShadowing_SkipsOverrideListenerParamsShadowingOuterLocal(t *testing.T) {
	findings := runRuleByName(t, "NoNameShadowing", `
package test

class Animation

object AnimationUtils {
    fun loadAnimation(): Animation = Animation()
}

interface AnimationListener {
    fun onAnimationStart(animation: Animation?)
    fun onAnimationRepeat(animation: Animation?)
    fun onAnimationEnd(animation: Animation?)
}

class AnimationBox {
    fun setAnimationListener(listener: AnimationListener) {}
}

class Example {
    fun hide() {
        val animation = AnimationUtils.loadAnimation()
        val box = AnimationBox()
        box.setAnimationListener(object : AnimationListener {
            override fun onAnimationStart(animation: Animation?) = Unit
            override fun onAnimationRepeat(animation: Animation?) = Unit
            override fun onAnimationEnd(animation: Animation?) = Unit
        })
        println(animation)
    }
}
`)
	for _, f := range findings {
		if f.Rule == "NoNameShadowing" {
			t.Fatalf("NoNameShadowing should ignore override listener params shadowing outer locals, got: %s", f.Message)
		}
	}
}

func TestNaming_NoNameShadowing_SkipsFunctionTypeLabelsBeforeOverrideMethod(t *testing.T) {
	findings := runRuleByName(t, "NoNameShadowing", `
package test

open class Drawable
open class LayerDrawable(layers: Array<Drawable>)
class Canvas

private class CustomDrawWrapper(
  private val wrapped: Drawable,
  private val drawFn: (wrapped: Drawable, canvas: Canvas) -> Unit
) : LayerDrawable(arrayOf(wrapped)) {
  fun draw(canvas: Canvas) {
    drawFn(wrapped, canvas)
  }
}
`)
	for _, f := range findings {
		if f.Rule == "NoNameShadowing" {
			t.Fatalf("NoNameShadowing should ignore function type labels before later method params, got: %s", f.Message)
		}
	}
}

// Regression: the null-narrowing self-shadow detector previously used
// substring scanning over the declaration text, which produced both false
// positives (treating arbitrary code containing "?:" or "?." as the idiom)
// and false negatives (missing the idiom when comments, parentheses, or
// safe-call chains separated the identifier from the operator).
func TestNaming_NoNameShadowing_AcceptsNullNarrowingIdiom(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"basic_elvis", `val name = name ?: return`},
		{"safe_call", `val name = name?.trim()`},
		{"safe_call_then_elvis", `val name = name?.trim() ?: ""`},
		{"safe_call_let_then_elvis", `val name = name?.let { it.trim() } ?: ""`},
		{"parenthesized_elvis", `val name = (name ?: return)`},
		{"comment_between_eq_and_rhs", `val name = /* narrow */ name ?: return`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := "package test\nfun example(name: String?) {\n    " + tc.body + "\n    println(name)\n}\n"
			findings := runRuleByName(t, "NoNameShadowing", src)
			for _, f := range findings {
				if f.Rule == "NoNameShadowing" {
					t.Fatalf("NoNameShadowing should treat %q as null-narrowing self-shadow, got: %s", tc.body, f.Message)
				}
			}
		})
	}
}

// Regression: ensure the AST-based detector does NOT silently allow shadowing
// just because the right-hand side happens to contain "?:" or "?." byte
// sequences that the old substring check could have been confused by.
func TestNaming_NoNameShadowing_FlagsNonNullNarrowingShadows(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		// "?:" appears only inside a string literal — not a real elvis operator.
		{"string_literal_elvis_bytes", `val name = "name ?: ignored"`},
		// "?." appears only in a comment — not a real safe-call operator.
		{"comment_safe_call_bytes", `val name = compute() /* uses ?. once */`},
		// Different identifier on the LHS of the elvis — not self-shadowing.
		{"different_identifier_elvis", `val name = other ?: ""`},
		// Different identifier with safe-call — not self-shadowing.
		{"different_identifier_safe_call", `val name = other?.trim()`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := "package test\nfun compute(): String? = null\nfun example(name: String?, other: String?) {\n    " + tc.body + "\n    println(name)\n}\n"
			findings := runRuleByName(t, "NoNameShadowing", src)
			found := false
			for _, f := range findings {
				if f.Rule == "NoNameShadowing" {
					found = true
				}
			}
			if !found {
				t.Fatalf("NoNameShadowing should flag %q as a real shadow, got no finding", tc.body)
			}
		})
	}
}

func BenchmarkNoNameShadowing_LargeFile(b *testing.B) {
	var src strings.Builder
	src.WriteString("package test\n")
	src.WriteString("class Outer {\n")
	for i := 0; i < 250; i++ {
		src.WriteString("    val shared = 1\n")
		src.WriteString("    fun fn")
		src.WriteString(strings.TrimSpace("1"))
		src.WriteString("() {\n")
		src.WriteString("        val shared = 2\n")
		src.WriteString("        if (shared > 0) {\n")
		src.WriteString("            val inner = shared\n")
		src.WriteString("            println(inner)\n")
		src.WriteString("        }\n")
		src.WriteString("    }\n")
	}
	src.WriteString("}\n")

	dir := b.TempDir()
	path := filepath.Join(dir, "bench.kt")
	if err := os.WriteFile(path, []byte(src.String()), 0o644); err != nil {
		b.Fatal(err)
	}
	file, err := scanner.ParseFile(path)
	if err != nil {
		b.Fatal(err)
	}

	var rule *v2rules.Rule
	for _, r := range v2rules.Registry {
		if r.ID == "NoNameShadowing" {
			rule = r
			break
		}
	}
	if rule == nil {
		b.Fatal("NoNameShadowing rule not found")
	}

	dispatcher := rules.NewDispatcherV2([]*v2rules.Rule{rule})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = dispatcher.Run(file)
	}
}

// --- NonBooleanPropertyPrefixedWithIs ---

func TestNaming_NonBooleanPropertyPrefixedWithIs_Flags(t *testing.T) {
	findings := runRuleByName(t, "NonBooleanPropertyPrefixedWithIs", `
package test
class Foo {
    val isName: String = "hello"
}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "NonBooleanPropertyPrefixedWithIs" {
			found = true
		}
	}
	if !found {
		t.Error("expected NonBooleanPropertyPrefixedWithIs to flag non-Boolean 'is' property")
	}
}

func TestNaming_NonBooleanPropertyPrefixedWithIs_AcceptsBoolean(t *testing.T) {
	findings := runRuleByName(t, "NonBooleanPropertyPrefixedWithIs", `
package test
class Foo {
    val isEnabled: Boolean = true
}
`)
	for _, f := range findings {
		if f.Rule == "NonBooleanPropertyPrefixedWithIs" {
			t.Errorf("NonBooleanPropertyPrefixedWithIs should accept Boolean property, got: %s", f.Message)
		}
	}
}

// --- ObjectPropertyNaming ---

func TestNaming_ObjectProperty_FlagsBadConstName(t *testing.T) {
	findings := runRuleByName(t, "ObjectPropertyNaming", `
package test
object Config {
    const val myConst = 42
}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "ObjectPropertyNaming" {
			found = true
		}
	}
	if !found {
		t.Error("expected ObjectPropertyNaming to flag non-SCREAMING_SNAKE const in object")
	}
}

func TestNaming_ObjectProperty_AcceptsScreamingSnakeConst(t *testing.T) {
	findings := runRuleByName(t, "ObjectPropertyNaming", `
package test
object Config {
    const val MY_CONST = 42
}
`)
	for _, f := range findings {
		if f.Rule == "ObjectPropertyNaming" {
			t.Errorf("ObjectPropertyNaming should accept SCREAMING_SNAKE const, got: %s", f.Message)
		}
	}
}

// --- VariableMaxLength ---

func TestNaming_VariableMaxLength_FlagsTooLong(t *testing.T) {
	findings := runRuleByName(t, "VariableMaxLength", `
package test
fun example() {
    val thisIsAnExtremelyLongVariableNameThatExceedsTheSixtyFourCharacterLimit = 1
}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "VariableMaxLength" {
			found = true
		}
	}
	if !found {
		t.Error("expected VariableMaxLength to flag variable name exceeding 64 chars")
	}
}

func TestNaming_VariableMaxLength_AcceptsShortName(t *testing.T) {
	findings := runRuleByName(t, "VariableMaxLength", `
package test
fun example() {
    val count = 1
}
`)
	for _, f := range findings {
		if f.Rule == "VariableMaxLength" {
			t.Errorf("VariableMaxLength should accept short name, got: %s", f.Message)
		}
	}
}

// --- VariableMinLength ---

func TestNaming_VariableMinLength_FlagsTooShort(t *testing.T) {
	findings := runRuleByName(t, "VariableMinLength", `
package test
fun example() {
    val x = 1
}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "VariableMinLength" {
			found = true
		}
	}
	if !found {
		t.Error("expected VariableMinLength to flag single-char variable name")
	}
}

func TestNaming_VariableMinLength_AcceptsLongEnough(t *testing.T) {
	findings := runRuleByName(t, "VariableMinLength", `
package test
fun example() {
    val count = 1
}
`)
	for _, f := range findings {
		if f.Rule == "VariableMinLength" {
			t.Errorf("VariableMinLength should accept multi-char name, got: %s", f.Message)
		}
	}
}

// --- ForbiddenClassName ---

func TestNaming_ForbiddenClassName_FlagsForbiddenName(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenClassName", `
package test
class Manager
`)
	found := false
	for _, f := range findings {
		if f.Rule == "ForbiddenClassName" {
			found = true
		}
	}
	if !found {
		t.Error("expected ForbiddenClassName to flag class named 'Manager'")
	}
}

func TestNaming_ForbiddenClassName_AcceptsNonForbiddenName(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenClassName", `
package test
class UserService
`)
	for _, f := range findings {
		if f.Rule == "ForbiddenClassName" {
			t.Errorf("ForbiddenClassName should accept non-forbidden name, got: %s", f.Message)
		}
	}
}
