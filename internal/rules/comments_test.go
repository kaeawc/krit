package rules_test

import (
	"regexp"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
)

// --- UndocumentedPublicClass ---

func TestUndocumentedPublicClass_Positive(t *testing.T) {
	findings := runRuleByName(t, "UndocumentedPublicClass", `
package test

class Foo {
    fun bar() {}
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for undocumented public class, got none")
	}
}

func TestUndocumentedPublicClass_Negative(t *testing.T) {
	findings := runRuleByName(t, "UndocumentedPublicClass", `
package test

/** This class does things. */
class Foo {
    fun bar() {}
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for documented public class, got %d", len(findings))
	}
}

// --- UndocumentedPublicFunction ---

func TestUndocumentedPublicFunction_Positive(t *testing.T) {
	findings := runRuleByName(t, "UndocumentedPublicFunction", `
package test

fun doSomething() {}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for undocumented public function, got none")
	}
}

func TestUndocumentedPublicFunction_Negative(t *testing.T) {
	findings := runRuleByName(t, "UndocumentedPublicFunction", `
package test

/** Does something. */
fun doSomething() {}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for documented public function, got %d", len(findings))
	}
}

// --- UndocumentedPublicProperty ---

func TestUndocumentedPublicProperty_Positive(t *testing.T) {
	findings := runRuleByName(t, "UndocumentedPublicProperty", `
package test

val myProp: String = "hello"
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for undocumented public property, got none")
	}
}

func TestUndocumentedPublicProperty_IgnoresLocalVals(t *testing.T) {
	findings := runRuleByName(t, "UndocumentedPublicProperty", `
package test

fun render() {
    val scrollState = rememberState()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for local vals, got %d", len(findings))
	}
}

func TestUndocumentedPublicRules_IgnoreTestSources(t *testing.T) {
	code := `
package test

class Fixture {
    val subject = Any()
    fun setUp() {}
}
`
	for _, ruleName := range []string{
		"UndocumentedPublicClass",
		"UndocumentedPublicFunction",
		"UndocumentedPublicProperty",
	} {
		findings := runRuleByNameOnPath(t, ruleName, "src/test/kotlin/Fixture.kt", code)
		if len(findings) != 0 {
			t.Fatalf("%s should ignore test sources, got %d findings", ruleName, len(findings))
		}
	}
}

func TestUndocumentedPublicProperty_Negative(t *testing.T) {
	findings := runRuleByName(t, "UndocumentedPublicProperty", `
package test

/** The greeting string. */
val myProp: String = "hello"
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for documented public property, got %d", len(findings))
	}
}

func TestUndocumentedPublicRules_IgnoreGradleBuildScripts(t *testing.T) {
	code := `
plugins {
    id("com.android.application")
}

val customLintConfig = "lint.xml"

fun helperTaskName() = "lint"
`
	for _, ruleName := range []string{
		"UndocumentedPublicFunction",
		"UndocumentedPublicProperty",
	} {
		findings := runRuleByNameOnPath(t, ruleName, "build.gradle.kts", code)
		if len(findings) != 0 {
			t.Fatalf("%s should ignore Gradle build scripts, got %d findings", ruleName, len(findings))
		}
	}
}

func TestUndocumentedPublicRules_IgnoreGeneratedDIDeclarations(t *testing.T) {
	code := `
package test

import dev.zacsweers.metro.ContributesTo
import dev.zacsweers.metro.DependencyGraph
import dev.zacsweers.metro.Inject
import dev.zacsweers.metro.Provides

@DependencyGraph(AppScope::class)
interface AppGraph {
    fun inject(target: App)
    val application: App
}

@ContributesTo(AppScope::class)
interface ApplicationModule {
    @Provides
    fun provideClock(): Clock = Clock()
}

/** Application target. */
class App

/** Clock dependency. */
class Clock
`
	for _, ruleName := range []string{
		"UndocumentedPublicClass",
		"UndocumentedPublicFunction",
		"UndocumentedPublicProperty",
	} {
		findings := runRuleByName(t, ruleName, code)
		if len(findings) != 0 {
			t.Fatalf("%s should ignore DI declarations consumed by generated code, got %d findings: %+v", ruleName, len(findings), findings)
		}
	}
}

// --- DocumentationOverPrivateFunction ---

func TestDocumentationOverPrivateFunction_Positive(t *testing.T) {
	findings := runRuleByName(t, "DocumentationOverPrivateFunction", `package test

fun anchor() {}

/** This is unnecessary. */
private fun helper() {}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for KDoc on private function, got none")
	}
}

func TestDocumentationOverPrivateFunction_Negative(t *testing.T) {
	findings := runRuleByName(t, "DocumentationOverPrivateFunction", `
package test

private fun helper() {}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for private function without KDoc, got %d", len(findings))
	}
}

// --- DocumentationOverPrivateProperty ---

func TestDocumentationOverPrivateProperty_Positive(t *testing.T) {
	findings := runRuleByName(t, "DocumentationOverPrivateProperty", `package test

fun anchor() {}

/** This is unnecessary. */
private val secret = 42
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for KDoc on private property, got none")
	}
}

func TestDocumentationOverPrivateProperty_Negative(t *testing.T) {
	findings := runRuleByName(t, "DocumentationOverPrivateProperty", `
package test

private val secret = 42
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for private property without KDoc, got %d", len(findings))
	}
}

// --- EndOfSentenceFormat ---

func TestEndOfSentenceFormat_Positive(t *testing.T) {
	findings := runRuleByName(t, "EndOfSentenceFormat", `
package test

/** This has no ending punctuation */
fun foo() {}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for KDoc without ending punctuation, got none")
	}
}

func TestEndOfSentenceFormat_Negative(t *testing.T) {
	findings := runRuleByName(t, "EndOfSentenceFormat", `
package test

/** This has proper ending punctuation. */
fun foo() {}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for properly punctuated KDoc, got %d", len(findings))
	}
}

// TestEndOfSentenceFormat_MetaSingleEndOfSentenceFormatOption guards
// against re-introducing the duplicate `endOfSentenceFormat` option that
// previously appeared twice — once as OptString writing to a dead field
// and once as OptRegex writing to the live Pattern field — which would
// silently apply twice when users set the option in YAML.
func TestEndOfSentenceFormat_MetaSingleEndOfSentenceFormatOption(t *testing.T) {
	impl := (*rules.EndOfSentenceFormatRule)(nil)
	for _, candidate := range v2rules.Registry {
		if candidate.ID == "EndOfSentenceFormat" {
			var ok bool
			impl, ok = candidate.Implementation.(*rules.EndOfSentenceFormatRule)
			if !ok {
				t.Fatalf("expected *EndOfSentenceFormatRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if impl == nil {
		t.Fatal("EndOfSentenceFormat rule not registered")
	}
	desc := impl.Meta()
	count := 0
	for _, opt := range desc.Options {
		if opt.Name == "endOfSentenceFormat" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly one `endOfSentenceFormat` option in zz_meta, got %d", count)
	}
}

// TestEndOfSentenceFormat_HonorsConfiguredPattern verifies the regex
// option is wired: with a pattern that requires a question mark only
// (no period), a KDoc ending in `.` is flagged as not matching.
func TestEndOfSentenceFormat_HonorsConfiguredPattern(t *testing.T) {
	var rule *rules.EndOfSentenceFormatRule
	for _, candidate := range v2rules.Registry {
		if candidate.ID == "EndOfSentenceFormat" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.EndOfSentenceFormatRule)
			if !ok {
				t.Fatalf("expected EndOfSentenceFormatRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("EndOfSentenceFormat rule not registered")
	}
	original := rule.Pattern
	defer func() { rule.Pattern = original }()

	// Configure a stricter pattern that only accepts `?` as a terminator.
	stricter, err := regexp.Compile(`\?$|\? `)
	if err != nil {
		t.Fatalf("compile pattern: %v", err)
	}
	rule.Pattern = stricter

	if findings := runRuleByName(t, "EndOfSentenceFormat", `
package test

/** Ends with period. */
fun foo() {}
`); len(findings) == 0 {
		t.Fatal("expected finding when pattern only accepts '?' but KDoc ends with '.'")
	}
	if findings := runRuleByName(t, "EndOfSentenceFormat", `
package test

/** Ends with question? */
fun foo() {}
`); len(findings) != 0 {
		t.Fatalf("expected no findings when KDoc matches configured pattern, got %d", len(findings))
	}
}

func TestEndOfSentenceFormat_IgnoresGradleAndTestSources(t *testing.T) {
	code := `
package test

/** Fixture documentation without punctuation */
fun foo() {}
`
	for _, path := range []string{"build.gradle.kts", "src/test/kotlin/FooTest.kt"} {
		findings := runRuleByNameOnPath(t, "EndOfSentenceFormat", path, code)
		if len(findings) != 0 {
			t.Fatalf("expected no EndOfSentenceFormat findings for %s, got %d", path, len(findings))
		}
	}
}

// --- OutdatedDocumentation ---

func TestOutdatedDocumentation_Positive(t *testing.T) {
	findings := runRuleByName(t, "OutdatedDocumentation", `package test

fun anchor() {}

/**
 * Does something.
 * @param x the value
 * @param y the other value
 */
fun doSomething(x: Int) {}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for @param that does not match actual parameter, got none")
	}
}

func TestOutdatedDocumentation_Negative(t *testing.T) {
	findings := runRuleByName(t, "OutdatedDocumentation", `package test

fun anchor() {}

/**
 * Does something.
 * @param x the value
 */
fun doSomething(x: Int) {}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for matching @param tags, got %d", len(findings))
	}
}

// --- KDocReferencesNonPublicProperty ---

func TestKDocReferencesNonPublicProperty_Positive(t *testing.T) {
	findings := runRuleByName(t, "KDocReferencesNonPublicProperty", `
package test

private val secret = 42

/** See [secret] for details. */
fun foo() {}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for KDoc referencing non-public property, got none")
	}
}

func TestKDocReferencesNonPublicProperty_Negative(t *testing.T) {
	findings := runRuleByName(t, "KDocReferencesNonPublicProperty", `
package test

val visible = 42

/** See [visible] for details. */
fun foo() {}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for KDoc referencing public property, got %d", len(findings))
	}
}

// --- AbsentOrWrongFileLicense ---

func TestAbsentOrWrongFileLicense_Positive(t *testing.T) {
	findings := runRuleByName(t, "AbsentOrWrongFileLicense", `
package test

fun foo() {}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for missing license header, got none")
	}
}

func TestAbsentOrWrongFileLicense_Negative(t *testing.T) {
	findings := runRuleByName(t, "AbsentOrWrongFileLicense", `/* Copyright 2024 Acme Corp */
package test

fun foo() {}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for file with license header, got %d", len(findings))
	}
}

// --- DeprecatedBlockTag ---

func TestDeprecatedBlockTag_Positive(t *testing.T) {
	findings := runRuleByName(t, "DeprecatedBlockTag", `
package test

/**
 * Old method.
 * @deprecated Use newMethod instead.
 */
fun oldMethod() {}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for @deprecated KDoc tag, got none")
	}
}

func TestDeprecatedBlockTag_Negative(t *testing.T) {
	findings := runRuleByName(t, "DeprecatedBlockTag", `
package test

/**
 * Old method.
 */
@Deprecated("Use newMethod instead.")
fun oldMethod() {}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when using @Deprecated annotation, got %d", len(findings))
	}
}
