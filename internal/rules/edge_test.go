package rules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
	sitter "github.com/smacker/go-tree-sitter"
)

// parseInline creates a temp .kt file, parses it, and returns the File.
func parseInline(t *testing.T, code string) *scanner.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.kt")
	if err := os.WriteFile(path, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	f, err := scanner.ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// runRuleByName finds a rule by name and runs it on the given code.
func runRuleByName(t *testing.T, ruleName string, code string) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	return runRuleByNameOnFile(t, ruleName, file)
}

func runRuleByNameOnJava(t *testing.T, ruleName string, code string) []scanner.Finding {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.java")
	if err := os.WriteFile(path, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseJavaFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return runRuleByNameOnFile(t, ruleName, file)
}

func runRuleByNameOnJavaPath(t *testing.T, ruleName, filename, code string) []scanner.Finding {
	t.Helper()
	path := filepath.Join(t.TempDir(), filename)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseJavaFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return runRuleByNameOnFile(t, ruleName, file)
}

func parseJavaFixture(t *testing.T, path string) *scanner.File {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseJavaFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

func runRuleByNameOnFile(t *testing.T, ruleName string, file *scanner.File) []scanner.Finding {
	t.Helper()
	for _, r := range v2rules.Registry {
		if r.ID == ruleName {
			d := rules.NewDispatcherV2([]*v2rules.Rule{r})
			cols := d.Run(file)
			return cols.Findings()
		}
	}
	t.Fatalf("rule %q not found in registry", ruleName)
	return nil
}

// --- MagicNumber edge cases ---

func TestMagicNumber_IgnoresCompanionObjectConst(t *testing.T) {
	findings := runRuleByName(t, "MagicNumber", `
package test
class Foo {
    companion object {
        const val MAX = 100
    }
}`)
	for _, f := range findings {
		if strings.Contains(f.Message, "100") {
			t.Error("MagicNumber should not flag const val in companion object")
		}
	}
}

func TestMagicNumber_IgnoresDefaultNumbers(t *testing.T) {
	findings := runRuleByName(t, "MagicNumber", `
package test
fun example() {
    val a = -1
    val b = 0
    val c = 1
    val d = 2
}`)
	for _, f := range findings {
		if f.Rule == "MagicNumber" {
			t.Errorf("MagicNumber should ignore default numbers (-1,0,1,2), got: %s", f.Message)
		}
	}
}

func TestMagicNumber_FlagsNonDefault(t *testing.T) {
	findings := runRuleByName(t, "MagicNumber", `
package test
fun example() {
    val timeout = 5000
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "MagicNumber" && strings.Contains(f.Message, "5000") {
			found = true
		}
	}
	if !found {
		t.Error("MagicNumber should flag 5000")
	}
}

func TestMagicNumber_IgnoresDpUnits(t *testing.T) {
	findings := runRuleByName(t, "MagicNumber", `
package test
fun example() {
    val padding = 16.dp
    val margin = 8.sp
}`)
	for _, f := range findings {
		if f.Rule == "MagicNumber" {
			t.Errorf("MagicNumber should ignore .dp/.sp units, got: %s", f.Message)
		}
	}
}

func TestMagicNumber_IgnoresColorLiterals(t *testing.T) {
	findings := runRuleByName(t, "MagicNumber", `
package test
val bg = Color(0xFF000000)
val fg = Color(0xFFFFFFFF)
`)
	for _, f := range findings {
		if f.Rule == "MagicNumber" {
			t.Errorf("MagicNumber should ignore color literals, got: %s", f.Message)
		}
	}
}

func TestMagicNumber_IgnoresNamedArguments(t *testing.T) {
	findings := runRuleByName(t, "MagicNumber", `
package test
fun example() {
    call(timeout = 5000)
    create(width = 100, height = 200)
}`)
	for _, f := range findings {
		if f.Rule == "MagicNumber" {
			t.Errorf("MagicNumber should ignore named arguments, got: %s", f.Message)
		}
	}
}

func TestMagicNumber_IgnoresExtensionFunctions(t *testing.T) {
	findings := runRuleByName(t, "MagicNumber", `
package test
fun example() {
    val x = 100.toLong()
    val y = 24.hours
}`)
	for _, f := range findings {
		if f.Rule == "MagicNumber" {
			t.Errorf("MagicNumber should ignore extension function receivers, got: %s", f.Message)
		}
	}
}

func TestMagicNumber_IgnoresDefaultParameterValues(t *testing.T) {
	findings := runRuleByName(t, "MagicNumber", `
package test
fun example(timeout: Int = 5000, retries: Int = 3) {}
class Foo(val x: Int = 42)
`)
	for _, f := range findings {
		if f.Rule == "MagicNumber" {
			t.Errorf("MagicNumber should ignore default parameter values, got: %s", f.Message)
		}
	}
}

func TestMagicNumber_IgnoresFunctionReturnConstants(t *testing.T) {
	findings := runRuleByName(t, "MagicNumber", `
package test
fun maxSize() = 100
fun getDefault(): Int {
    return 42
}`)
	for _, f := range findings {
		if f.Rule == "MagicNumber" {
			t.Errorf("MagicNumber should ignore function return constants, got: %s", f.Message)
		}
	}
}

func TestMagicNumber_IgnoresDurationCallsWithImportedTimeUnit(t *testing.T) {
	findings := runRuleByName(t, "MagicNumber", `
package test
import java.util.concurrent.TimeUnit

fun example() {
    Observable.interval(0, 5, TimeUnit.SECONDS)
    events.throttleLatest(500, TimeUnit.MILLISECONDS)
    completable.timeout(10, TimeUnit.SECONDS, fallback)
}
`)
	for _, f := range findings {
		if f.Rule == "MagicNumber" {
			t.Errorf("MagicNumber should ignore literals paired with imported TimeUnit, got: %s at line %d", f.Message, f.Line)
		}
	}
}

func TestMagicNumber_IgnoresAndroidApiLevelEvidence(t *testing.T) {
	findings := runRuleByName(t, "MagicNumber", `
package test

annotation class RequiresApi(val value: Int)

fun minSdkLessThan(value: Int): Boolean = false
fun checkApi(value: Int, message: String) = Unit

@RequiresApi(34)
fun load() {
    if (minSdkLessThan(23)) return
    checkApi(33, "needs Tiramisu")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected Android API-level literals to be ignored, got %d", len(findings))
	}
}

func TestMagicNumber_IgnoresHTTPStatusRange(t *testing.T) {
	findings := runRuleByName(t, "MagicNumber", `
package test

fun isSuccessful(response: Response): Boolean {
    return response.code !in 200 until 300 && response.code != HTTP_RESPONSE_NOT_MODIFIED
}

fun isAlsoSuccessful(response: Response): Boolean {
    return response.code() >= 200 && response.code() < 300
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected HTTP status range literals to be ignored, got %d", len(findings))
	}
}

func TestMagicNumber_FlagsHTTPStatusLookalikeWithoutStatusEvidence(t *testing.T) {
	findings := runRuleByName(t, "MagicNumber", `
package test

fun makeList(): List<Int> = List(200) { it }
`)
	found := false
	for _, f := range findings {
		if f.Rule == "MagicNumber" && strings.Contains(f.Message, "'200'") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected non-status 200 literal to remain a finding")
	}
}

func TestMagicNumber_IgnoresHalfRatioLiteral(t *testing.T) {
	findings := runRuleByName(t, "MagicNumber", `
package test

fun choose(newRatio: Float, storedRatio: Float): Boolean {
    return newRatio >= .5 || newRatio >= storedRatio
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected half-ratio literal to be ignored, got %d", len(findings))
	}
}

func TestMagicNumber_IgnoresTestSupportSource(t *testing.T) {
	file := parseInline(t, `
package test

fun timeoutMillis(): Int = 3000
`)
	file.Path = "/repo/camera/camera-testing/src/main/kotlin/androidx/camera/testing/Fake.kt"
	findings := runRuleByNameOnFile(t, "MagicNumber", file)
	if len(findings) != 0 {
		t.Fatalf("expected test-support source to be ignored, got %d", len(findings))
	}
}

func TestMagicNumber_IgnoresJvmAndroidTestSource(t *testing.T) {
	file := parseInline(t, `
package test

fun offset(): Float = 1.99999f
`)
	file.Path = "/repo/ink/ink-geometry/src/jvmAndroidTest/kotlin/androidx/ink/geometry/BoxTest.kt"
	findings := runRuleByNameOnFile(t, "MagicNumber", file)
	if len(findings) != 0 {
		t.Fatalf("expected jvmAndroidTest source to be ignored, got %d", len(findings))
	}
}

func TestMagicNumber_FlagsDurationCallWithLocalTimeUnitLookalike(t *testing.T) {
	findings := runRuleByName(t, "MagicNumber", `
package test

object TimeUnit {
    const val SECONDS = "seconds"
}

fun example() {
    pollEvery(5, TimeUnit.SECONDS)
}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "MagicNumber" && strings.Contains(f.Message, "'5'") {
			found = true
		}
	}
	if !found {
		t.Error("MagicNumber should flag duration-looking calls when TimeUnit is only a local lookalike")
	}
}

func TestMagicNumberDoesNotContributeToOracle(t *testing.T) {
	for _, r := range v2rules.Registry {
		if r.ID != "MagicNumber" {
			continue
		}
		if r.Needs != 0 || rules.RuleNeedsKotlinOracle(r) {
			t.Fatalf("MagicNumber should remain AST-only, got Needs=%b Oracle=%+v OracleCallTargets=%+v OracleDeclarationNeeds=%+v",
				r.Needs, r.Oracle, r.OracleCallTargets, r.OracleDeclarationNeeds)
		}
		return
	}
	t.Fatal("MagicNumber rule not found in registry")
}

func TestMagicNumber_CompanionObjectRespectsConfig(t *testing.T) {
	findings := runRuleByName(t, "MagicNumber", `
package test
class Foo {
    companion object {
        val TIMEOUT = 5000
    }
}`)
	for _, f := range findings {
		if f.Rule == "MagicNumber" {
			t.Errorf("MagicNumber should ignore companion object properties by default, got: %s", f.Message)
		}
	}
}

// --- EmptyFunctionBlock edge cases ---

func TestEmptyFunctionBlock_IgnoresExpressionBody(t *testing.T) {
	findings := runRuleByName(t, "EmptyFunctionBlock", `
package test
fun getValue() = 42
fun getName() = "hello"
`)
	for _, f := range findings {
		if f.Rule == "EmptyFunctionBlock" {
			t.Errorf("Should not flag expression-body functions, got: %s at line %d", f.Message, f.Line)
		}
	}
}

func TestEmptyFunctionBlock_FlagsEmptyBraceBody(t *testing.T) {
	findings := runRuleByName(t, "EmptyFunctionBlock", `
package test
fun doNothing() { }
`)
	found := false
	for _, f := range findings {
		if f.Rule == "EmptyFunctionBlock" {
			found = true
		}
	}
	if !found {
		t.Error("Should flag fun doNothing() { }")
	}
}

// --- UnsafeCast edge cases ---

func TestUnsafeCast_DoesNotFlagBareAnyAs(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnsafeCast", `
package test
fun example(x: Any) {
    val s = x as String
}`)
	for _, f := range findings {
		if f.Rule == "UnsafeCast" {
			t.Error("Should not flag broad Any-to-String casts without never-succeeds proof")
		}
	}
}

func TestUnsafeCast_IgnoresSafeCast(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnsafeCast", `
package test
fun example(x: Any) {
    val s = x as? String
}`)
	for _, f := range findings {
		if f.Rule == "UnsafeCast" {
			t.Error("Should not flag safe cast 'as?'")
		}
	}
}

func TestUnsafeCast_SuppressedByIsCheck(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnsafeCast", `
package test
fun example(x: Any) {
    if (x is String) {
        val s = x as String
    }
}`)
	for _, f := range findings {
		if f.Rule == "UnsafeCast" {
			t.Error("Should not flag cast guarded by is-check")
		}
	}
}

func TestUnsafeCast_SuppressedByNegativeIsCheckEarlyReturn(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnsafeCast", `
package test
fun example(x: Any): String {
    if (x !is String) return ""
    return x as String
}`)
	for _, f := range findings {
		if f.Rule == "UnsafeCast" {
			t.Error("Should not flag cast after negative is-check with early return")
		}
	}
}

func TestUnsafeCast_SuppressedByWhenIsCheck(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnsafeCast", `
package test
fun example(x: Any): String {
    return when (x) {
        is String -> x as String
        else -> ""
    }
}`)
	for _, f := range findings {
		if f.Rule == "UnsafeCast" {
			t.Error("Should not flag cast in when branch with is-check")
		}
	}
}

func TestUnsafeCast_StillFlagsUnguardedCast(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnsafeCast", `
package test
fun example(x: Any) {
    if (x is Int) {
        // The is-check is for Int, but the cast is to String — NOT guarded
        val s = x as String
    }
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "UnsafeCast" {
			found = true
		}
	}
	if !found {
		t.Error("Should flag cast to different type than is-check")
	}
}

func TestUnsafeCast_ConjunctionIsCheck(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "UnsafeCast", `
package test
fun example(x: Any, y: Any) {
    if (x != null && x is String) {
        val s = x as String
    }
}`)
	for _, f := range findings {
		if f.Rule == "UnsafeCast" {
			t.Error("Should not flag cast guarded by is-check in conjunction")
		}
	}
}

// --- WildcardImport edge cases ---

func TestWildcardImport_FlagsWildcard(t *testing.T) {
	findings := runRuleByName(t, "WildcardImport", `
package test
import kotlin.collections.*
class Foo
`)
	found := false
	for _, f := range findings {
		if f.Rule == "WildcardImport" {
			found = true
		}
	}
	if !found {
		t.Error("Should flag wildcard import")
	}
}

func TestWildcardImport_SkipsJavaUtil(t *testing.T) {
	findings := runRuleByName(t, "WildcardImport", `
package test
import java.util.*
class Foo
`)
	for _, f := range findings {
		if f.Rule == "WildcardImport" && strings.Contains(f.Message, "java.util") {
			t.Error("Should skip java.util.* (default exclude)")
		}
	}
}

func TestWildcardImport_SkipsKotlinNativeInterop(t *testing.T) {
	findings := runRuleByName(t, "WildcardImport", `
package test
import kotlinx.cinterop.*
import platform.AVFoundation.*
class Foo
`)
	for _, f := range findings {
		if f.Rule == "WildcardImport" {
			t.Errorf("Should skip Kotlin/Native interop wildcard imports, got: %s", f.Message)
		}
	}
}

func TestWildcardImport_DefaultConfigSkipsKotlinNativeInterop(t *testing.T) {
	cfg, err := config.LoadAndMerge("", filepath.Join("..", "..", "config", "default-krit.yml"))
	if err != nil {
		t.Fatalf("load default config: %v", err)
	}
	excludes := cfg.GetStringList("style", "WildcardImport", "excludeImports")
	for _, want := range []string{"java.util.*", "platform.**", "kotlinx.cinterop.*"} {
		found := false
		for _, got := range excludes {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("default WildcardImport excludeImports missing %q in %v", want, excludes)
		}
	}
}

func TestWildcardImport_FlagsJavaUtilSubpackage(t *testing.T) {
	findings := runRuleByName(t, "WildcardImport", `
package test
import java.util.concurrent.*
class Foo
`)
	found := false
	for _, f := range findings {
		if f.Rule == "WildcardImport" {
			found = true
		}
	}
	if !found {
		t.Error("Should flag java.util subpackage wildcard import")
	}
}

// --- ForbiddenComment edge cases ---

func TestForbiddenComment_InStringLiteral(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenComment", `
package test
fun example() {
    val msg = "TODO: implement this"
}`)
	for _, f := range findings {
		if f.Rule == "ForbiddenComment" {
			t.Error("Should NOT flag TODO inside a string literal")
		}
	}
}

func TestForbiddenComment_InActualComment(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenComment", `
package test
// TODO: fix this
fun example() {}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "ForbiddenComment" {
			found = true
		}
	}
	if !found {
		t.Error("Should flag TODO in a real comment")
	}
}

func TestForbiddenComment_IgnoresMarkerInsideQuotedCommentedCode(t *testing.T) {
	findings := runRuleByName(t, "ForbiddenComment", `
package test
// extraMessage.set("TODO: Once docs exist")
fun example() {}
`)
	for _, f := range findings {
		if f.Rule == "ForbiddenComment" {
			t.Fatalf("ForbiddenComment should ignore TODO inside quoted commented-out code, got: %s", f.Message)
		}
	}
}

// --- Suppress integration ---

func TestSuppress_SuppressesSpecificRule(t *testing.T) {
	findings := runRuleByName(t, "MagicNumber", `
package test
@Suppress("MagicNumber")
fun example() {
    val x = 42
}`)
	for _, f := range findings {
		if f.Rule == "MagicNumber" {
			t.Error("@Suppress('MagicNumber') should suppress MagicNumber findings")
		}
	}
}

func TestSuppress_AllSuppressesEverything(t *testing.T) {
	file := parseInline(t, `
package test
@Suppress("all")
class Foo {
    fun bar() {
        val x = 42
        val y: String? = null
    }
}`)
	var allRules []*v2rules.Rule
	for _, r := range v2rules.Registry {
		if rules.IsDefaultActive(r.ID) {
			allRules = append(allRules, r)
		}
	}
	d := rules.NewDispatcherV2(allRules)
	findingCols := d.Run(file)
	findings := findingCols.Findings()

	for _, f := range findings {
		if f.Line >= 3 { // inside the @Suppress("all") class
			t.Errorf("@Suppress('all') should suppress all findings, got: [%s:%s] %s at line %d",
				f.RuleSet, f.Rule, f.Message, f.Line)
		}
	}
}

// --- Suppress edge cases ---

func TestSuppress_MultipleRulesInAnnotation(t *testing.T) {
	// @Suppress with two rule names should suppress both
	findings := runRuleByName(t, "MagicNumber", `
package test
@Suppress("MagicNumber", "UnusedVariable")
fun example() {
    val x = 42
}`)
	for _, f := range findings {
		if f.Rule == "MagicNumber" {
			t.Error("@Suppress('MagicNumber', 'UnusedVariable') should suppress MagicNumber")
		}
	}
}

func TestSuppress_NestedClassDoesNotAffectOuter(t *testing.T) {
	// Suppress on inner class should not suppress findings in the outer class
	findings := runRuleByName(t, "MagicNumber", `
package test
class Outer {
    val outerVal = 42

    @Suppress("MagicNumber")
    class Inner {
        val innerVal = 42
    }
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "MagicNumber" && f.Line == 4 {
			found = true
		}
		if f.Rule == "MagicNumber" && f.Line == 8 {
			t.Error("inner class member should be suppressed")
		}
	}
	if !found {
		t.Error("outer class member (line 4) should NOT be suppressed")
	}
}

func TestSuppress_SuppressWarningsJavaStyle(t *testing.T) {
	// Java-style @SuppressWarnings should work
	findings := runRuleByName(t, "MagicNumber", `
package test
@SuppressWarnings("MagicNumber")
fun example() {
    val x = 42
}`)
	for _, f := range findings {
		if f.Rule == "MagicNumber" {
			t.Error("@SuppressWarnings('MagicNumber') should suppress MagicNumber")
		}
	}
}

func TestSuppress_DetektPrefix(t *testing.T) {
	// detekt: prefix should work
	findings := runRuleByName(t, "MagicNumber", `
package test
@Suppress("detekt:MagicNumber")
fun example() {
    val x = 42
}`)
	for _, f := range findings {
		if f.Rule == "MagicNumber" {
			t.Error("@Suppress('detekt:MagicNumber') should suppress MagicNumber")
		}
	}
}

func TestSuppress_ScopeBoundarySiblingFunctions(t *testing.T) {
	// Suppress on one function should not affect a sibling function
	findings := runRuleByName(t, "MagicNumber", `
package test
@Suppress("MagicNumber")
fun suppressed() {
    val x = 42
}

fun notSuppressed() {
    val y = 42
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "MagicNumber" && f.Line == 5 {
			t.Error("first function should be suppressed")
		}
		if f.Rule == "MagicNumber" && f.Line == 9 {
			found = true
		}
	}
	if !found {
		t.Error("second function (line 9) should NOT be suppressed")
	}
}

func TestSuppress_ClassLevelCoversAllMembers(t *testing.T) {
	// Suppress on class should cover all members inside it
	findings := runRuleByName(t, "MagicNumber", `
package test
@Suppress("MagicNumber")
class Foo {
    val a = 42
    fun bar() {
        val b = 99
    }
    companion object {
        val c = 123
    }
}`)
	for _, f := range findings {
		if f.Rule == "MagicNumber" {
			t.Errorf("class-level @Suppress should cover all members, got finding at line %d: %s", f.Line, f.Message)
		}
	}
}

func TestSuppress_ExpressionLevelOnlyCoversDeclaration(t *testing.T) {
	// Suppress on val should cover only that declaration, not others in the same function
	findings := runRuleByName(t, "MagicNumber", `
package test
fun example() {
    @Suppress("MagicNumber")
    val x = 42
    val y = 42
}`)
	suppressedFound := false
	unsuppressedFound := false
	for _, f := range findings {
		if f.Rule == "MagicNumber" && f.Line == 5 {
			suppressedFound = true
		}
		if f.Rule == "MagicNumber" && f.Line == 6 {
			unsuppressedFound = true
		}
	}
	if suppressedFound {
		t.Error("val x (line 5) should be suppressed by expression-level @Suppress")
	}
	if !unsuppressedFound {
		t.Error("val y (line 6) should NOT be suppressed — it's outside the expression-level scope")
	}
}

// parseInlineWithName creates a temp .kt file with a specific filename, parses it, and returns the File.
func parseInlineWithName(t *testing.T, filename string, code string) *scanner.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), filename)
	if err := os.WriteFile(path, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	f, err := scanner.ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// runMatchingDeclarationName runs the MatchingDeclarationName rule on code in a file with the given name.
func runMatchingDeclarationName(t *testing.T, filename string, code string) []scanner.Finding {
	t.Helper()
	file := parseInlineWithName(t, filename, code)
	for _, r := range v2rules.Registry {
		if r.ID == "MatchingDeclarationName" {
			d := rules.NewDispatcherV2([]*v2rules.Rule{r})
			cols := d.Run(file)
			return cols.Findings()
		}
	}
	t.Fatal("MatchingDeclarationName rule not found in registry")
	return nil
}

// --- MatchingDeclarationName edge cases ---

func TestMatchingDeclarationName_SingleClassMatches(t *testing.T) {
	findings := runMatchingDeclarationName(t, "Foo.kt", `class Foo`)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestMatchingDeclarationName_SingleClassMismatch(t *testing.T) {
	findings := runMatchingDeclarationName(t, "Bar.kt", `class Foo`)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(findings))
	}
}

func TestMatchingDeclarationName_SingleClassPlusExtensions(t *testing.T) {
	// A single non-private class + extension functions should still flag if mismatched
	findings := runMatchingDeclarationName(t, "Foo.kt", `
class Foo
fun Foo.bar() = 5
fun helper() = 42
`)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for matching class+extensions, got %d", len(findings))
	}
}

func TestMatchingDeclarationName_SingleClassPlusHelpersShouldFlag(t *testing.T) {
	// File named Utils.kt with class Foo + helpers should flag
	findings := runMatchingDeclarationName(t, "Utils.kt", `
class Foo
fun helper() = 42
`)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(findings))
	}
}

func TestMatchingDeclarationName_PrivateClassIgnored(t *testing.T) {
	// Private class + top-level function: no non-private class, so no finding
	findings := runMatchingDeclarationName(t, "Utils.kt", `
private class Helper
fun doStuff() = 42
`)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for private class, got %d: %v", len(findings), findings)
	}
}

func TestMatchingDeclarationName_TwoPublicClassesSkipped(t *testing.T) {
	// Multiple non-private classes: rule should not flag
	findings := runMatchingDeclarationName(t, "Stuff.kt", `
class Foo
class Bar
`)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for multiple classes, got %d", len(findings))
	}
}

func TestMatchingDeclarationName_TypealiasMatchesFilename(t *testing.T) {
	// typealias Foo matches filename, so class FooImpl should not be flagged
	findings := runMatchingDeclarationName(t, "Foo.kt", `
typealias Foo = FooImpl
class FooImpl
`)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when typealias matches filename, got %d: %v", len(findings), findings)
	}
}

func TestMatchingDeclarationName_TypealiasMismatch(t *testing.T) {
	// typealias Bar does NOT match filename Foo.kt, and class FooImpl != Foo
	findings := runMatchingDeclarationName(t, "Foo.kt", `
class FooImpl
typealias Bar = FooImpl
`)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(findings))
	}
}

func TestMatchingDeclarationName_MustBeFirst(t *testing.T) {
	// With mustBeFirst=true (default), class not first -> no finding
	findings := runMatchingDeclarationName(t, "Classes.kt", `
fun a() = 5
fun C.b() = 5
class C
`)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when class is not first (mustBeFirst=true), got %d", len(findings))
	}
}

func TestMatchingDeclarationName_ObjectDeclaration(t *testing.T) {
	findings := runMatchingDeclarationName(t, "Objects.kt", `object O`)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding for object name mismatch, got %d", len(findings))
	}
}

func TestMatchingDeclarationName_MultiplatformSuffix(t *testing.T) {
	findings := runMatchingDeclarationName(t, "Foo.android.kt", `actual class Foo`)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for multiplatform suffix, got %d: %v", len(findings), findings)
	}
}

// --- UnderscoresInNumericLiterals acceptableLength tests ---

func TestUnderscoresInNumericLiterals_SkipsShortNumbers(t *testing.T) {
	// 4-digit numbers should not be flagged (acceptableLength default = 4)
	findings := runRuleByName(t, "UnderscoresInNumericLiterals", `
package test
fun example() {
    val a = 1000
    val b = 9999
    val c = 100
    val d = 42
}`)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for short numeric literals, got %d", len(findings))
		for _, f := range findings {
			t.Logf("  %s", f.Message)
		}
	}
}

func TestUnderscoresInNumericLiterals_FlagsLongNumbers(t *testing.T) {
	// 5+ consecutive digits without underscores should be flagged
	findings := runRuleByName(t, "UnderscoresInNumericLiterals", `
package test
fun example() {
    val a = 10000
    val b = 1000000
}`)
	if len(findings) != 2 {
		t.Errorf("expected 2 findings for long numeric literals, got %d", len(findings))
		for _, f := range findings {
			t.Logf("  %s", f.Message)
		}
	}
}

func TestUnderscoresInNumericLiterals_SkipsHexAndBinary(t *testing.T) {
	findings := runRuleByName(t, "UnderscoresInNumericLiterals", `
package test
fun example() {
    val hex = 0xFF
    val bin = 0b1010
}`)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for hex/binary literals, got %d", len(findings))
	}
}

func TestUnderscoresInNumericLiterals_SkipsAlreadyFormatted(t *testing.T) {
	findings := runRuleByName(t, "UnderscoresInNumericLiterals", `
package test
fun example() {
    val x = 1_000_000
    val y = 10_000L
}`)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for already-formatted literals, got %d", len(findings))
	}
}

func TestUnderscoresInNumericLiterals_FlagsNonStandardGrouping(t *testing.T) {
	findings := runRuleByName(t, "UnderscoresInNumericLiterals", `
package test
fun example() {
    val x = 10_00
}`)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding for non-standard underscore grouping, got %d", len(findings))
	}
}

// --- ElseCaseInsteadOfExhaustiveWhen without resolver tests ---

func TestElseCaseInsteadOfExhaustiveWhen_NoResolverNoFindings(t *testing.T) {
	// Without a type resolver the rule should not fire to avoid false positives
	findings := runRuleByName(t, "ElseCaseInsteadOfExhaustiveWhen", `
package test

fun describe(x: Int): String {
    return when (x) {
        1 -> "one"
        2 -> "two"
        else -> "other"
    }
}`)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings without resolver, got %d", len(findings))
		for _, f := range findings {
			t.Logf("  %s", f.Message)
		}
	}
}

func TestElseCaseInsteadOfExhaustiveWhen_NoResolverSealedClass(t *testing.T) {
	// Even with sealed class in same file, without resolver we don't fire
	findings := runRuleByName(t, "ElseCaseInsteadOfExhaustiveWhen", `
package test

sealed class Shape
class Circle : Shape()
class Square : Shape()

fun describe(shape: Shape): String {
    return when (shape) {
        is Circle -> "circle"
        else -> "unknown"
    }
}`)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings without resolver even for sealed class, got %d", len(findings))
		for _, f := range findings {
			t.Logf("  %s", f.Message)
		}
	}
}

// ---------------------------------------------------------------------------
// UnreachableCode with oracle diagnostics
// ---------------------------------------------------------------------------

// stubResolver is a minimal TypeResolver that returns zero values.
type stubResolver struct{}

func (s *stubResolver) ResolveNode(_ *sitter.Node, _ *scanner.File) *typeinfer.ResolvedType {
	return typeinfer.UnknownType()
}
func (s *stubResolver) ResolveFlatNode(_ uint32, _ *scanner.File) *typeinfer.ResolvedType {
	return typeinfer.UnknownType()
}
func (s *stubResolver) ResolveByName(_ string, _ *sitter.Node, _ *scanner.File) *typeinfer.ResolvedType {
	return nil
}
func (s *stubResolver) ResolveByNameFlat(_ string, _ uint32, _ *scanner.File) *typeinfer.ResolvedType {
	return nil
}
func (s *stubResolver) ResolveImport(_ string, _ *scanner.File) string   { return "" }
func (s *stubResolver) IsNullable(_ *sitter.Node, _ *scanner.File) *bool { return nil }
func (s *stubResolver) IsNullableFlat(_ uint32, _ *scanner.File) *bool   { return nil }
func (s *stubResolver) ClassHierarchy(_ string) *typeinfer.ClassInfo     { return nil }
func (s *stubResolver) SealedVariants(_ string) []string                 { return nil }
func (s *stubResolver) EnumEntries(_ string) []string                    { return nil }
func (s *stubResolver) AnnotationValue(_ *sitter.Node, _ *scanner.File, _, _ string) string {
	return ""
}
func (s *stubResolver) AnnotationValueFlat(_ uint32, _ *scanner.File, _, _ string) string {
	return ""
}
func (s *stubResolver) IsExceptionSubtype(_, _ string) bool { return false }

// runRuleWithOracle runs the UnreachableCode rule with a FakeOracle providing diagnostics.
func runRuleWithOracle(t *testing.T, code string, filePath string, diags []oracle.OracleDiagnostic) []scanner.Finding {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.kt")
	if err := os.WriteFile(path, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	f, err := scanner.ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Override the file path to match oracle diagnostics lookup.
	f.Path = filePath

	fake := oracle.NewFakeOracle()
	if len(diags) > 0 {
		fake.Diagnostics[filePath] = diags
	}
	composite := oracle.NewCompositeResolver(fake, &stubResolver{})

	var rule *v2rules.Rule
	for _, r := range v2rules.Registry {
		if r.ID == "UnreachableCode" {
			rule = r
			break
		}
	}
	if rule == nil {
		t.Fatal("UnreachableCode rule not found")
	}

	d := rules.NewDispatcherV2([]*v2rules.Rule{rule}, composite)
	cols := d.Run(f)
	return cols.Findings()
}

func TestUnreachableCode_OracleDiagnostic_UNREACHABLE_CODE(t *testing.T) {
	code := `package test
fun example() {
    return
    println("dead")
}`
	diags := []oracle.OracleDiagnostic{
		{FactoryName: "UNREACHABLE_CODE", Severity: "WARNING", Message: "Unreachable code", Line: 4, Col: 5},
	}
	findings := runRuleWithOracle(t, code, "test.kt", diags)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding from oracle diagnostic, got %d", len(findings))
	}
	if findings[0].Line != 4 {
		t.Errorf("expected finding at line 4, got %d", findings[0].Line)
	}
	if !strings.Contains(findings[0].Message, "Unreachable code") {
		t.Errorf("expected message about unreachable code, got %q", findings[0].Message)
	}
}

func TestUnreachableCode_OracleDiagnostic_USELESS_ELVIS(t *testing.T) {
	code := `package test
fun example() {
    val name: String = "hello"
    val result = name ?: "default"
}`
	diags := []oracle.OracleDiagnostic{
		{FactoryName: "USELESS_ELVIS", Severity: "WARNING", Message: "Elvis operator (?:) always returns the left operand of non-nullable type String", Line: 4, Col: 18},
	}
	findings := runRuleWithOracle(t, code, "test.kt", diags)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding from USELESS_ELVIS diagnostic, got %d", len(findings))
	}
	if findings[0].Line != 4 {
		t.Errorf("expected finding at line 4, got %d", findings[0].Line)
	}
}

func TestUnreachableCode_OracleNoDiagnostics_FallsBackToHeuristic(t *testing.T) {
	code := `package test
fun example() {
    return
    println("dead")
}`
	// No diagnostics in oracle — should fall back to heuristic analysis.
	findings := runRuleWithOracle(t, code, "test.kt", nil)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding from heuristic fallback, got %d", len(findings))
	}
	if !strings.Contains(findings[0].Message, "jump statement") {
		t.Errorf("expected heuristic message about jump statement, got %q", findings[0].Message)
	}
}

func TestUnreachableCode_OracleIgnoresIrrelevantDiagnostics(t *testing.T) {
	code := `package test
fun example() {
    val x = 1
    println(x)
}`
	// A diagnostic with a factory name we don't care about should be ignored.
	diags := []oracle.OracleDiagnostic{
		{FactoryName: "UNUSED_VARIABLE", Severity: "WARNING", Message: "Variable 'x' is never used", Line: 3, Col: 9},
	}
	findings := runRuleWithOracle(t, code, "test.kt", diags)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for irrelevant diagnostic, got %d", len(findings))
	}
}

func TestUnreachableCode_OracleDiagnosticOutsideNodeRange(t *testing.T) {
	code := `package test
fun example() {
    val x = 1
}
fun other() {
    return
    println("dead")
}`
	// Diagnostic at line 7 is in the second function; first function should not pick it up.
	diags := []oracle.OracleDiagnostic{
		{FactoryName: "UNREACHABLE_CODE", Severity: "WARNING", Message: "Unreachable code", Line: 7, Col: 5},
	}
	findings := runRuleWithOracle(t, code, "test.kt", diags)
	// The diagnostic should only be reported once (by the statements node that contains line 7).
	unreachableCount := 0
	for _, f := range findings {
		if f.Rule == "UnreachableCode" {
			unreachableCount++
		}
	}
	if unreachableCount != 1 {
		t.Errorf("expected 1 finding for diagnostic at line 7, got %d", unreachableCount)
	}
}
