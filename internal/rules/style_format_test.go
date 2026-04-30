package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
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

func TestMaxLineLength_Java(t *testing.T) {
	longLine := "String result = aaaaaaaa + bbbbbbbb + cccccccc + dddddddd + eeeeeeee + ffffffff + gggggggg + hhhhhhhh + iiiiiiii + jjjjjjjj;"
	code := "package test;\nclass Example {\n  void run() {\n    " + longLine + "\n  }\n}\n"
	findings := runRuleByNameOnJava(t, "MaxLineLength", code)
	if len(findings) == 0 {
		t.Fatal("expected Java finding for line exceeding max length")
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

func TestMaxLineLengthDefaultsMatchDetekt(t *testing.T) {
	var rule *rules.MaxLineLengthRule
	var meta v2rules.RuleDescriptor
	for _, candidate := range v2rules.Registry {
		if candidate.ID != "MaxLineLength" {
			continue
		}
		var ok bool
		rule, ok = candidate.Implementation.(*rules.MaxLineLengthRule)
		if !ok {
			t.Fatalf("expected MaxLineLengthRule, got %T", candidate.Implementation)
		}
		metaProvider, ok := candidate.Implementation.(v2rules.MetaProvider)
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
	findings := runRuleByName(t, "SpacingAfterPackageAndImports", "package test\nfun main() {\n}\n")
	if len(findings) == 0 {
		t.Fatal("expected finding for missing blank line after package")
	}
}

func TestSpacingAfterPackageAndImports_Negative(t *testing.T) {
	findings := runRuleByName(t, "SpacingAfterPackageAndImports", "package test\n\nfun main() {\n}\n")
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
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

func TestMaxChainedCallsOnSameLine_Java(t *testing.T) {
	findings := runRuleByNameOnJava(t, "MaxChainedCallsOnSameLine", `
package test;

class Example {
  void run() {
    Object x = a.b().c().d().e().f().g();
  }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected Java finding for too many chained calls")
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

func TestUnderscoresInNumericLiterals_Java(t *testing.T) {
	findings := runRuleByNameOnJava(t, "UnderscoresInNumericLiterals", `
package test;
class Example {
  int count = 1000000;
  long timeoutMs = 2500000L;
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected 2 Java numeric-underscore findings, got %d", len(findings))
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

func TestUnderscoresInNumericLiteralsDefaultsMatchDetekt(t *testing.T) {
	var rule *rules.UnderscoresInNumericLiteralsRule
	var meta v2rules.RuleDescriptor
	for _, candidate := range v2rules.Registry {
		if candidate.ID != "UnderscoresInNumericLiterals" {
			continue
		}
		var ok bool
		rule, ok = candidate.Implementation.(*rules.UnderscoresInNumericLiteralsRule)
		if !ok {
			t.Fatalf("expected UnderscoresInNumericLiteralsRule, got %T", candidate.Implementation)
		}
		metaProvider, ok := candidate.Implementation.(v2rules.MetaProvider)
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
		t.Fatal("did not expect non-detekt threshold metadata option")
	}
}

func TestUnderscoresInNumericLiteralsAllowsConfiguredNonStandardGrouping(t *testing.T) {
	var rule *rules.UnderscoresInNumericLiteralsRule
	for _, candidate := range v2rules.Registry {
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
    val x = list.filter { it > 0 }
    .map { it * 2 }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for cascading call with bad indentation")
	}
}

func TestCascadingCallWrapping_Negative(t *testing.T) {
	findings := runRuleByName(t, "CascadingCallWrapping", `
package test

fun main() {
    val x = list
        .filter { it > 0 }
        .map { it * 2 }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestCascadingCallWrapping_HonorsIncludeElvis(t *testing.T) {
	// IncludeElvis was previously a dead config — exposed in zz_meta but
	// never consulted. Configure it via the rule pointer and verify
	// elvis-chain continuations are checked under the same indentation
	// rule as dotted continuations.
	var rule *rules.CascadingCallWrappingRule
	for _, candidate := range v2rules.Registry {
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

	// Default (IncludeElvis=false): elvis continuations are not checked.
	if findings := runRuleByName(t, "CascadingCallWrapping", `
package test
fun main() {
    val x = bar
    ?: baz
}
`); len(findings) != 0 {
		t.Fatalf("expected no findings under IncludeElvis=false, got %d", len(findings))
	}

	rule.IncludeElvis = true

	// Now the misindented elvis continuation should fire.
	if findings := runRuleByName(t, "CascadingCallWrapping", `
package test
fun main() {
    val x = bar
    ?: baz
}
`); len(findings) == 0 {
		t.Fatal("expected finding for misindented elvis continuation under IncludeElvis=true")
	}

	// Properly indented elvis chain should not fire.
	if findings := runRuleByName(t, "CascadingCallWrapping", `
package test
fun main() {
    val x = bar
        ?: baz
        ?: qux
}
`); len(findings) != 0 {
		t.Fatalf("expected no findings for properly indented elvis chain under IncludeElvis=true, got %d", len(findings))
	}
}
