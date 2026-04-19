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

func TestTopLevelPropertyNaming_FlagsPascalCaseNonConst(t *testing.T) {
	findings := runRuleByName(t, "TopLevelPropertyNaming", `
package test
val MyProperty = "hello"
`)
	if len(findings) == 0 {
		t.Error("expected TopLevelPropertyNaming to flag PascalCase non-const top-level property")
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
