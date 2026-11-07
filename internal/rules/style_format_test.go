package rules_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// --- TrailingWhitespace ---

func TestTrailingWhitespace_Positive(t *testing.T) {
	findings := runRuleByName(t, "TrailingWhitespace", "package test\nfun main() {   \n    println(\"hi\")\n}\n")
	if len(findings) == 0 {
		t.Fatal("expected finding for trailing whitespace")
	}
}

func TestTrailingWhitespace_Negative(t *testing.T) {
	findings := runRuleByName(t, "TrailingWhitespace", "package test\nfun main() {\n    println(\"hi\")\n}\n")
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- NoTabs ---

func TestNoTabs_Positive(t *testing.T) {
	findings := runRuleByName(t, "NoTabs", "package test\nfun main() {\n\tprintln(\"hi\")\n}\n")
	if len(findings) == 0 {
		t.Fatal("expected finding for tab character")
	}
}

func TestNoTabs_Negative(t *testing.T) {
	findings := runRuleByName(t, "NoTabs", "package test\nfun main() {\n    println(\"hi\")\n}\n")
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestNoTabs_FixUsesDefaultFourSpaces(t *testing.T) {
	findings := runRuleByName(t, "NoTabs", "package test\nfun main() {\n\tprintln(\"hi\")\n}\n")
	if len(findings) == 0 || findings[0].Fix == nil {
		t.Fatal("expected a fix")
	}
	got := findings[0].Fix.Replacement
	want := "    println(\"hi\")"
	if got != want {
		t.Fatalf("default fix replacement = %q, want %q", got, want)
	}
}

// --- MaxLineLength ---

func TestMaxLineLength_Positive(t *testing.T) {
	longLine := "val result = aaaaaaaa + bbbbbbbb + cccccccc + dddddddd + eeeeeeee + ffffffff + gggggggg + hhhhhhhh + iiiiiiii + jjjjjjjj"
	code := "package test\nfun main() {\n    " + longLine + "\n}\n"
	findings := runRuleByName(t, "MaxLineLength", code)
	if len(findings) == 0 {
		t.Fatal("expected finding for line exceeding max length")
	}
}

func TestMaxLineLength_Negative(t *testing.T) {
	findings := runRuleByName(t, "MaxLineLength", "package test\nfun main() {\n    val x = 1\n}\n")
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestMaxLineLength_UrlTailsAndMessageFollowDetekt(t *testing.T) {
	long := strings.Repeat("x", 130)
	code := "package test\n/**\n * " + long + " https://example.com/a\n * " + long + " [text](https://example.com/a)\n * " + long + "\n */\nfun f() {}\n"
	findings := runRuleByName(t, "MaxLineLength", code)
	if len(findings) != 1 || findings[0].Line != 5 {
		t.Fatalf("expected only the line without a URL or link tail, got %v", findings)
	}
	if findings[0].Message != "Line detected, which is longer than the defined maximum line length in the code style." {
		t.Errorf("unexpected message %q", findings[0].Message)
	}
}

func TestMaxLineLength_SkipsFullyQualifiedLogCall(t *testing.T) {
	// A fully-qualified `android.util.Log.d(...)` call whose overflow
	// position lands on a concatenated-identifier argument (not inside a
	// string literal) used to be flagged because the prefix check only
	// matched lines starting with `Log.`. The same call without the
	// qualifier was already skipped — this locks the qualified form to
	// the same treatment.
	long := `android.util.Log.d(TAGTAGTAGTAGTAGTAGTAGTAG, msgMsgMsgMsgMsgMsgMsgMsgMsg + extraExtraExtraExtraExtraExtraExtraExtraExtraExtraExtra)`
	code := "package test\nfun main() {\n    " + long + "\n}\n"
	findings := runRuleByName(t, "MaxLineLength", code)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for fully-qualified Log call, got %d: %v", len(findings), findings)
	}
}

func TestMaxLineLength_SkipsFullyQualifiedTimberCall(t *testing.T) {
	long := `timber.log.Timber.d(TAGTAGTAGTAGTAGTAGTAGTAG, msgMsgMsgMsgMsgMsgMsgMsgMsg + extraExtraExtraExtraExtraExtraExtraExtraExtraExtraExtra)`
	code := "package test\nfun main() {\n    " + long + "\n}\n"
	findings := runRuleByName(t, "MaxLineLength", code)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for fully-qualified Timber call, got %d: %v", len(findings), findings)
	}
}

func TestMaxLineLength_SkipsJava(t *testing.T) {
	longLine := "String result = aaaaaaaa + bbbbbbbb + cccccccc + dddddddd + eeeeeeee + ffffffff + gggggggg + hhhhhhhh + iiiiiiii + jjjjjjjj;"
	code := "package test;\nclass Example {\n  void run() {\n    " + longLine + "\n  }\n}\n"
	findings := runRuleByNameOnJava(t, "MaxLineLength", code)
	if len(findings) != 0 {
		t.Fatalf("detekt does not analyze Java sources; expected no findings, got %d", len(findings))
	}
}

func TestMaxLineLength_JavaExcludesPackageAndImportLines(t *testing.T) {
	findings := runRuleByNameOnJava(t, "MaxLineLength", `
package com.example.this.is.a.very.long.package.name.that.would.otherwise.exceed.the.configured.maximum.line.length.for.source.files;
import com.example.this.is.a.very.long.import.name.that.would.otherwise.exceed.the.configured.maximum.line.length.ForSourceFiles;
class Example {}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no Java max-line-length findings for excluded package/import lines, got %d", len(findings))
	}
}

func TestMaxLineLength_CompatibleDefaults(t *testing.T) {
	var rule *rules.MaxLineLengthRule
	var meta api.RuleDescriptor
	for _, candidate := range api.Registry {
		if candidate.ID != "MaxLineLength" {
			continue
		}
		var ok bool
		rule, ok = candidate.Implementation.(*rules.MaxLineLengthRule)
		if !ok {
			t.Fatalf("expected MaxLineLengthRule, got %T", candidate.Implementation)
		}
		metaProvider, ok := candidate.Implementation.(api.MetaProvider)
		if !ok {
			t.Fatalf("expected MaxLineLengthRule to provide metadata")
		}
		meta = metaProvider.Meta()
		break
	}
	if rule == nil {
		t.Fatal("MaxLineLength rule not registered")
	}
	if !rule.ExcludePackageStatements {
		t.Fatal("expected package statements to be excluded by default")
	}
	if !rule.ExcludeImportStatements {
		t.Fatal("expected import statements to be excluded by default")
	}
	if !rule.ExcludeRawStrings {
		t.Fatal("expected raw strings to be excluded by default")
	}
	defaults := map[string]interface{}{}
	for _, opt := range meta.Options {
		defaults[opt.Name] = opt.Default
	}
	for _, name := range []string{"excludePackageStatements", "excludeImportStatements", "excludeRawStrings"} {
		if defaults[name] != true {
			t.Fatalf("expected %s metadata default true, got %v", name, defaults[name])
		}
	}
}

// --- NewLineAtEndOfFile ---

func TestNewLineAtEndOfFile_Positive(t *testing.T) {
	findings := runRuleByName(t, "NewLineAtEndOfFile", "package test\nfun main() {\n}")
	if len(findings) == 0 {
		t.Fatal("expected finding for missing newline at end of file")
	}
}

func TestNewLineAtEndOfFile_Negative(t *testing.T) {
	findings := runRuleByName(t, "NewLineAtEndOfFile", "package test\nfun main() {\n}\n")
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- SpacingAfterPackageAndImports ---

func TestSpacingAfterPackageAndImports_Positive(t *testing.T) {
	findings := runRuleByName(t, "SpacingAfterPackageAndImports", "package test\nimport a.B\nclass X\n")
	if len(findings) != 2 {
		t.Fatalf("expected a finding after the package and one after the imports, got %v", findings)
	}
	if findings[0].Message != "There should be exactly one empty line in between the package declaration and the list of imports." ||
		findings[1].Message != "There should be exactly one empty line in between the list of imports and the declaration of X." {
		t.Errorf("unexpected messages: %q, %q", findings[0].Message, findings[1].Message)
	}
}

// detekt only checks the spacing when a file has imports; a package followed by a class is left alone.
func TestSpacingAfterPackageAndImports_IgnoresFilesWithoutImports(t *testing.T) {
	findings := runRuleByName(t, "SpacingAfterPackageAndImports", "package test\nfun main() {\n}\n")
	if len(findings) != 0 {
		t.Fatalf("expected no findings without imports, got %v", findings)
	}
}

func TestSpacingAfterPackageAndImports_Negative(t *testing.T) {
	findings := runRuleByName(t, "SpacingAfterPackageAndImports", "package test\n\nfun main() {\n}\n")
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// Import-shaped text inside a block comment must not anchor the finding.
func TestSpacingAfterPackageAndImports_IgnoresImportTextInBlockComment(t *testing.T) {
	code := "package style\n\n" +
		"import com.example.Foo\n\n" +
		"/*\nimport faux.Bar\n*/\n" +
		"class MyClass\n"
	findings := runRuleByName(t, "SpacingAfterPackageAndImports", code)
	if len(findings) != 0 {
		t.Fatalf("expected no findings; got %d: %+v", len(findings), findings)
	}
}

// Import-shaped text inside a raw string must not anchor the finding.
func TestSpacingAfterPackageAndImports_IgnoresImportTextInRawString(t *testing.T) {
	code := "package style\n\n" +
		"import com.example.Foo\n\n" +
		"val s = \"\"\"\nimport faux.Bar\n\"\"\"\n\n" +
		"class MyClass\n"
	findings := runRuleByName(t, "SpacingAfterPackageAndImports", code)
	if len(findings) != 0 {
		t.Fatalf("expected no findings; got %d: %+v", len(findings), findings)
	}
}

// Java: import-shaped text inside a block comment must not anchor the finding.
func TestSpacingAfterPackageAndImports_JavaIgnoresImportTextInBlockComment(t *testing.T) {
	code := "package style;\n\n" +
		"import java.util.List;\n\n" +
		"/*\nimport faux.Bar;\n*/\n" +
		"class C {}\n"
	findings := runRuleByNameOnJava(t, "SpacingAfterPackageAndImports", code)
	if len(findings) != 0 {
		t.Fatalf("expected no findings; got %d: %+v", len(findings), findings)
	}
}

// Java: only the LAST import anchors; expect one finding at the class line.
func TestSpacingAfterPackageAndImports_JavaMultipleImports(t *testing.T) {
	code := "package style;\n" +
		"import java.util.List;\n" +
		"import java.util.Map;\n" +
		"class C {}\n"
	findings := runRuleByNameOnJava(t, "SpacingAfterPackageAndImports", code)
	if len(findings) != 1 {
		t.Fatalf("expected exactly 1 finding; got %d: %+v", len(findings), findings)
	}
	if findings[0].Line != 4 {
		t.Fatalf("expected finding at line 4 (class line); got line %d", findings[0].Line)
	}
}

// --- MaxChainedCallsOnSameLine ---

func TestMaxChainedCallsOnSameLine_Positive(t *testing.T) {
	findings := runRuleByName(t, "MaxChainedCallsOnSameLine", `
package test

fun main() {
    val x = a.b.c.d.e.f.g()
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for too many chained calls")
	}
}

func TestMaxChainedCallsOnSameLine_CountsTheReceiverAndEachWrappedRun(t *testing.T) {
	findings := runRuleByName(t, "MaxChainedCallsOnSameLine", `package test
fun f(s: String) {
    s.trim().trim().trim().trim().trim()
    s.trim().trim().trim()
        .trim().trim().trim()
    s.trim().trim().trim().trim().trim().trim()
        .trim()
}
`)
	if len(findings) != 2 || findings[0].Line != 3 || findings[1].Line != 6 {
		t.Fatalf("expected the two chains with a same-line run of five or more, got %v", findings)
	}
	want := "6 chained calls on a single line; more than 5 calls should be wrapped to a new line."
	if findings[0].Message != want {
		t.Errorf("message = %q, want %q", findings[0].Message, want)
	}
}

func TestMaxChainedCallsOnSameLine_Negative(t *testing.T) {
	findings := runRuleByName(t, "MaxChainedCallsOnSameLine", `
package test

fun main() {
    val x = a.b.c()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestMaxChainedCallsOnSameLine_SkipsJava(t *testing.T) {
	findings := runRuleByNameOnJava(t, "MaxChainedCallsOnSameLine", `
package test;

class Example {
  void run() {
    Object x = a.b().c().d().e().f().g();
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("detekt does not analyze Java sources; expected no findings, got %d", len(findings))
	}
}

func TestMaxChainedCallsOnSameLine_JavaNegative(t *testing.T) {
	findings := runRuleByNameOnJava(t, "MaxChainedCallsOnSameLine", `
package test;

class Example {
  void run() {
    Object x = a.b().c();
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no Java chained-call findings, got %d", len(findings))
	}
}

func TestMaxChainedCallsOnSameLine_IgnoresGradleAndTestSources(t *testing.T) {
	code := `
package test
fun main() {
    val x = a.b.c.d.e.f.g()
}
`
	for _, path := range []string{"build.gradle.kts", "src/test/kotlin/FooTest.kt"} {
		findings := runRuleByNameOnPath(t, "MaxChainedCallsOnSameLine", path, code)
		if len(findings) != 0 {
			t.Fatalf("expected no MaxChainedCallsOnSameLine findings for %s, got %d", path, len(findings))
		}
	}
}

func TestMaxChainedCallsOnSameLine_IgnoresStringLiteralDots(t *testing.T) {
	findings := runRuleByName(t, "MaxChainedCallsOnSameLine", `
package test

fun main() {
    val s = "a.b.c.d.e.f.g"
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for dots inside a string literal, got %d", len(findings))
	}
}

func TestMaxChainedCallsOnSameLine_IgnoresTrailingCommentDots(t *testing.T) {
	findings := runRuleByName(t, "MaxChainedCallsOnSameLine", `
package test

fun main() {
    val x = a.b // see x.y.z.w.q.r.s for context
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for dots in a trailing comment, got %d", len(findings))
	}
}

func TestMaxChainedCallsOnSameLine_IgnoresDecimalLiteralDots(t *testing.T) {
	findings := runRuleByName(t, "MaxChainedCallsOnSameLine", `
package test

fun main() {
    val sum = 1.0 + 2.0 + 3.0 + 4.0 + 5.0 + 6.0
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for decimal-literal dots, got %d", len(findings))
	}
}

func TestMaxChainedCallsOnSameLine_IgnoresImportDots(t *testing.T) {
	findings := runRuleByName(t, "MaxChainedCallsOnSameLine", `
package test

import com.example.foo.bar.baz.qux.Quux

fun main() {
    val q = Quux
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for dots inside an import, got %d", len(findings))
	}
}

func TestMaxChainedCallsOnSameLine_IgnoresRawStringDots(t *testing.T) {
	findings := runRuleByName(t, "MaxChainedCallsOnSameLine", `
package test

fun main() {
    val raw = """x.y.z.w.q.r.s.t"""
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for dots inside a raw string, got %d", len(findings))
	}
}

func TestMaxChainedCallsOnSameLine_IgnoresWrappedChain(t *testing.T) {
	findings := runRuleByName(t, "MaxChainedCallsOnSameLine", `
package test

fun main() {
    val result = listOf(1, 2, 3)
        .filter { true }
        .map { it }
        .flatMap { listOf(it) }
        .sorted()
        .take(1)
        .first()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for an already-wrapped chain, got %d", len(findings))
	}
}

// --- UnderscoresInNumericLiterals ---

func TestUnderscoresInNumericLiterals_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnderscoresInNumericLiterals", `
package test

fun main() {
    val x = 1000000
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for large number without underscores")
	}
}

func TestUnderscoresInNumericLiterals_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnderscoresInNumericLiterals", `
package test

fun main() {
    val x = 1_000_000
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestUnderscoresInNumericLiterals_SkipsJava(t *testing.T) {
	findings := runRuleByNameOnJava(t, "UnderscoresInNumericLiterals", `
package test;
class Example {
  int count = 1000000;
  long timeoutMs = 2500000L;
}
`)
	if len(findings) != 0 {
		t.Fatalf("detekt does not analyze Java sources; expected no findings, got %d", len(findings))
	}
}

func TestUnderscoresInNumericLiterals_JavaNegative(t *testing.T) {
	findings := runRuleByNameOnJava(t, "UnderscoresInNumericLiterals", `
package test;
class Example {
  int count = 1_000_000;
  int mask = 0xFF00FF;
  int bits = 0b10101010;
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no Java numeric-underscore findings, got %d", len(findings))
	}
}

func TestUnderscoresInNumericLiterals_CompatibleDefaults(t *testing.T) {
	var rule *rules.UnderscoresInNumericLiteralsRule
	var meta api.RuleDescriptor
	for _, candidate := range api.Registry {
		if candidate.ID != "UnderscoresInNumericLiterals" {
			continue
		}
		var ok bool
		rule, ok = candidate.Implementation.(*rules.UnderscoresInNumericLiteralsRule)
		if !ok {
			t.Fatalf("expected UnderscoresInNumericLiteralsRule, got %T", candidate.Implementation)
		}
		metaProvider, ok := candidate.Implementation.(api.MetaProvider)
		if !ok {
			t.Fatal("expected UnderscoresInNumericLiteralsRule to provide metadata")
		}
		meta = metaProvider.Meta()
		break
	}
	if rule == nil {
		t.Fatal("UnderscoresInNumericLiterals rule not registered")
	}
	if rule.AcceptableLength != 4 {
		t.Fatalf("expected AcceptableLength default 4, got %d", rule.AcceptableLength)
	}
	if rule.AllowNonStandardGrouping {
		t.Fatal("expected AllowNonStandardGrouping default false")
	}
	defaults := map[string]interface{}{}
	for _, opt := range meta.Options {
		defaults[opt.Name] = opt.Default
	}
	if defaults["acceptableLength"] != 4 {
		t.Fatalf("expected acceptableLength metadata default 4, got %v", defaults["acceptableLength"])
	}
	if defaults["allowNonStandardGrouping"] != false {
		t.Fatalf("expected allowNonStandardGrouping metadata default false, got %v", defaults["allowNonStandardGrouping"])
	}
	if _, ok := defaults["threshold"]; ok {
		t.Fatal("did not expect alternate threshold metadata option")
	}
}

func TestUnderscoresInNumericLiteralsAllowsConfiguredNonStandardGrouping(t *testing.T) {
	var rule *rules.UnderscoresInNumericLiteralsRule
	for _, candidate := range api.Registry {
		if candidate.ID == "UnderscoresInNumericLiterals" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.UnderscoresInNumericLiteralsRule)
			if !ok {
				t.Fatalf("expected UnderscoresInNumericLiteralsRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("UnderscoresInNumericLiterals rule not registered")
	}
	originalAcceptableLength := rule.AcceptableLength
	originalAllowNonStandardGrouping := rule.AllowNonStandardGrouping
	defer func() {
		rule.AcceptableLength = originalAcceptableLength
		rule.AllowNonStandardGrouping = originalAllowNonStandardGrouping
	}()

	rules.ApplyConfig(loadTempConfig(t, `
style:
  UnderscoresInNumericLiterals:
    allowNonStandardGrouping: true
`))

	findings := runRuleByName(t, "UnderscoresInNumericLiterals", `
package test

fun main() {
    val x = 10_00
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when non-standard grouping is configured as allowed, got %d", len(findings))
	}
}

// --- EqualsOnSignatureLine ---

func TestEqualsOnSignatureLine_Positive(t *testing.T) {
	findings := runRuleByName(t, "EqualsOnSignatureLine", `
package test

fun add(a: Int, b: Int): Int
    = a + b
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for equals on next line")
	}
}

func TestEqualsOnSignatureLine_Negative(t *testing.T) {
	findings := runRuleByName(t, "EqualsOnSignatureLine", `
package test

fun add(a: Int, b: Int): Int = a + b
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- CascadingCallWrapping ---

func TestCascadingCallWrapping_Positive(t *testing.T) {
	findings := runRuleByName(t, "CascadingCallWrapping", `
package test

fun main() {
    val x = list
        .filter { it > 0 }.map { it * 2 }
    val y = list
        .filter { it > 0 }
        .map { it }.first()
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings for calls left on a line after a wrapped call, got %d", len(findings))
	}
}

func TestCascadingCallWrapping_Negative(t *testing.T) {
	findings := runRuleByName(t, "CascadingCallWrapping", `
package test

fun main() {
    val a = list
        .filter { it > 0 }
        .map { it * 2 }
    val b = list.filter { it > 0 }.map { it * 2 }
    val c = list.filter { it > 0 }
        .map { it * 2 }
    val d = foo(list
        .filter { it > 0 })
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestCascadingCallWrapping_IgnoresRawString(t *testing.T) {
	findings := runRuleByName(t, "CascadingCallWrapping", `
package test

fun main() {
    val sql = """
        SELECT foo.bar
        .baz
        FROM t
    """.trimIndent()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings inside raw string, got %d", len(findings))
	}
}

func TestCascadingCallWrapping_IgnoresBlockComment(t *testing.T) {
	findings := runRuleByName(t, "CascadingCallWrapping", `
package test

/*
val x = foo.bar
.baz
*/
fun main() {}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings inside block comment, got %d", len(findings))
	}
}

func TestCascadingCallWrapping_HonorsIncludeElvis(t *testing.T) {
	// IncludeElvis was previously a dead config — exposed in metadata but
	// never consulted. Configure it via the rule pointer and verify
	// elvis-chain continuations are checked under the same indentation
	// rule as dotted continuations.
	var rule *rules.CascadingCallWrappingRule
	for _, candidate := range api.Registry {
		if candidate.ID == "CascadingCallWrapping" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.CascadingCallWrappingRule)
			if !ok {
				t.Fatalf("expected CascadingCallWrappingRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("CascadingCallWrapping rule not registered")
	}
	original := rule.IncludeElvis
	defer func() { rule.IncludeElvis = original }()

	src := `
package test
fun main() {
    val x = list
        .filter { it > 0 } ?: baz
    val y = list
        .filter { it > 0 }
        ?: baz
    val z = bar ?: baz
}
`
	// Default (IncludeElvis=false): elvis operators are not checked.
	if findings := runRuleByName(t, "CascadingCallWrapping", src); len(findings) != 0 {
		t.Fatalf("expected no findings under IncludeElvis=false, got %d", len(findings))
	}

	rule.IncludeElvis = true
	if findings := runRuleByName(t, "CascadingCallWrapping", src); len(findings) != 1 {
		t.Fatalf("expected exactly the unwrapped elvis to be flagged under IncludeElvis=true, got %d", len(findings))
	}
}

func TestMaxLineLength_CountsCharactersNotBytes(t *testing.T) {
	// 70 two-byte characters plus code stays under 120 characters but exceeds 120 bytes.
	line := "val s = aaa + " + strings.Repeat("é", 70)
	code := "package test\nfun main() {\n    " + line + "\n}\n"
	if findings := runRuleByName(t, "MaxLineLength", code); len(findings) != 0 {
		t.Fatalf("line is under the limit in characters, got %v", findings)
	}
}

func TestMaxLineLength_FlagsLongCommentsUnlessExcluded(t *testing.T) {
	long := "// " + strings.Repeat("word ", 30)
	code := "package test\n" + long + "\nfun main() {}\n"
	if findings := runRuleByName(t, "MaxLineLength", code); len(findings) != 1 {
		t.Fatalf("expected the long comment to be reported, got %v", findings)
	}
}

func TestUnderscoresInNumericLiterals_SuffixesAndSingleReport(t *testing.T) {
	src := "package test\nval a = 1234567L\nval b = 123456u\nval c = 1234567UL\nval d = 123456789\n"
	findings := runRuleByName(t, "UnderscoresInNumericLiterals", src)
	// detekt does not check unsigned literals, so only the long and the plain literal are reported.
	if len(findings) != 2 {
		t.Fatalf("expected one finding per checked literal, got %d: %v", len(findings), findings)
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].Fix.StartByte < findings[j].Fix.StartByte })
	// The node covers the digits and the L suffix of a long literal.
	want := []string{"1_234_567L", "123_456_789"}
	for i, f := range findings {
		if f.Fix == nil || f.Fix.Replacement != want[i] {
			t.Errorf("finding %d: expected fix %q, got %+v", i, want[i], f.Fix)
		}
	}
}

func TestEqualsOnSignatureLine_FixJoinsWithoutGluingNextLine(t *testing.T) {
	src := "package test\n\nclass A {\n    fun f(a: Int): Int\n        = a + 1\n\n    fun g() = 2\n\n    val s = \"\"\"\n    fun h()\n    = 3\n    \"\"\"\n}\n"
	findings := runRuleByName(t, "EqualsOnSignatureLine", src)
	if len(findings) != 1 || findings[0].Fix == nil {
		t.Fatalf("expected one fixable finding, got %v", findings)
	}
	fix := findings[0].Fix
	got := src[:fix.StartByte] + fix.Replacement + src[fix.EndByte:]
	want := strings.Replace(src, "Int\n        = a + 1", "Int = a + 1", 1)
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestTrailingWhitespace_CRLFAndRawStrings(t *testing.T) {
	src := "package test\r\nclass A {  \r\n    val raw = \"\"\"\r\n  keep  \r\n    \"\"\"\r\n}\r\n"
	findings := runRuleByName(t, "TrailingWhitespace", src)
	if len(findings) != 2 {
		t.Fatalf("expected CRLF line and raw string line to be reported, got %v", findings)
	}
	if findings[0].Fix == nil {
		t.Fatal("code line should be fixable")
	}
	fix := findings[0].Fix
	if got := src[:fix.StartByte] + fix.Replacement + src[fix.EndByte:]; !strings.Contains(got, "class A {\r\n") {
		t.Errorf("fix should keep the CRLF terminator, got %q", got)
	}
	if findings[1].Fix != nil {
		t.Error("whitespace inside a raw string must not be auto-removed")
	}
}

func TestNoTabs_RawStringTabsAreNotRewritten(t *testing.T) {
	findings := runRuleByName(t, "NoTabs", "package test\nval raw = \"\"\"\n\tx\n\"\"\"\nfun f() {\n\tprintln()\n}\n")
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}
	if findings[0].Fix != nil {
		t.Error("tab inside a raw string must not be auto-replaced")
	}
	if findings[1].Fix == nil {
		t.Error("tab in code should be fixable")
	}
}

func TestMaxLineLength_CRLFDoesNotCountCarriageReturn(t *testing.T) {
	line := "    val s = a + " + strings.Repeat("b", 104) // exactly 120 characters
	code := "package test\r\nfun main() {\r\n" + line + "\r\n}\r\n"
	if findings := runRuleByName(t, "MaxLineLength", code); len(findings) != 0 {
		t.Fatalf("a 120 character CRLF line is within the limit, got %v", findings)
	}
}
