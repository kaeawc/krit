package rules_test

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
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

func TestUndocumentedPublicClass_HonorsSearchInFlags(t *testing.T) {
	// Four flags were previously dead config — the rule fired on every
	// undocumented public class regardless of nesting context. Wired
	// flags now let users opt out of searching specific container types.
	var rule *rules.UndocumentedPublicClassRule
	for _, candidate := range api.Registry {
		if candidate.ID == "UndocumentedPublicClass" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.UndocumentedPublicClassRule)
			if !ok {
				t.Fatalf("expected UndocumentedPublicClassRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("UndocumentedPublicClass rule not registered")
	}
	originals := struct {
		nested, inner, object, iface bool
	}{
		nested: rule.SearchInNestedClass,
		inner:  rule.SearchInInnerClass,
		object: rule.SearchInInnerObject,
		iface:  rule.SearchInInnerInterface,
	}
	defer func() {
		rule.SearchInNestedClass = originals.nested
		rule.SearchInInnerClass = originals.inner
		rule.SearchInInnerObject = originals.object
		rule.SearchInInnerInterface = originals.iface
	}()

	cases := []struct {
		name      string
		code      string
		flagOff   func()
		expectOn  bool
		expectOff bool
	}{
		{
			name: "nested",
			code: `package test
/** outer doc */
class Outer {
    class Nested
}
`,
			flagOff:   func() { rule.SearchInNestedClass = false },
			expectOn:  true,
			expectOff: false,
		},
		{
			name: "inner",
			code: `package test
/** outer doc */
class Outer {
    inner class Inner
}
`,
			flagOff:   func() { rule.SearchInInnerClass = false },
			expectOn:  true,
			expectOff: false,
		},
		{
			name: "inside-object",
			code: `package test
/** obj doc */
object Obj {
    class InsideObj
}
`,
			flagOff:   func() { rule.SearchInInnerObject = false },
			expectOn:  true,
			expectOff: false,
		},
		{
			name: "inside-interface",
			code: `package test
/** iface doc */
interface IF {
    class InsideIF
}
`,
			flagOff:   func() { rule.SearchInInnerInterface = false },
			expectOn:  true,
			expectOff: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rule.SearchInNestedClass = true
			rule.SearchInInnerClass = true
			rule.SearchInInnerObject = true
			rule.SearchInInnerInterface = true
			gotOn := len(runRuleByName(t, "UndocumentedPublicClass", tc.code)) > 0
			if gotOn != tc.expectOn {
				t.Fatalf("with all flags on, expected finding=%v, got=%v", tc.expectOn, gotOn)
			}
			tc.flagOff()
			gotOff := len(runRuleByName(t, "UndocumentedPublicClass", tc.code)) > 0
			if gotOff != tc.expectOff {
				t.Fatalf("with %s flag off, expected finding=%v, got=%v", tc.name, tc.expectOff, gotOff)
			}
		})
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
	for _, candidate := range api.Registry {
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
		t.Fatalf("expected exactly one `endOfSentenceFormat` option in metadata, got %d", count)
	}
}

// TestEndOfSentenceFormat_HonorsConfiguredPattern verifies the regex
// option is wired: with a pattern that requires a question mark only
// (no period), a KDoc ending in `.` is flagged as not matching.
func TestEndOfSentenceFormat_HonorsConfiguredPattern(t *testing.T) {
	var rule *rules.EndOfSentenceFormatRule
	for _, candidate := range api.Registry {
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

func TestOutdatedDocumentation_HonorsMatchTypeParameters(t *testing.T) {
	// MatchTypeParameters was previously a dead config — exposed but
	// never consulted. Default true means @param tags can document
	// type parameters without being flagged as orphans.
	docCode := `package test

fun anchor() {}

/**
 * Maps a value.
 * @param T input type
 * @param R output type
 * @param value the input
 * @param transform the mapping
 */
fun <T, R> map(value: T, transform: (T) -> R): R = transform(value)
`
	if findings := runRuleByName(t, "OutdatedDocumentation", docCode); len(findings) != 0 {
		t.Fatalf("expected no findings under default MatchTypeParameters=true (type-param tags T, R recognized), got %d", len(findings))
	}

	// Flip MatchTypeParameters off via the rule pointer and verify the
	// type-param tags now read as orphans.
	var rule *rules.OutdatedDocumentationRule
	for _, candidate := range api.Registry {
		if candidate.ID == "OutdatedDocumentation" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.OutdatedDocumentationRule)
			if !ok {
				t.Fatalf("expected OutdatedDocumentationRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("OutdatedDocumentation rule not registered")
	}
	original := rule.MatchTypeParameters
	defer func() { rule.MatchTypeParameters = original }()
	rule.MatchTypeParameters = false

	if findings := runRuleByName(t, "OutdatedDocumentation", docCode); len(findings) == 0 {
		t.Fatal("expected findings under MatchTypeParameters=false for @param T / @param R")
	}
}

func TestOutdatedDocumentation_HonorsMatchDeclarationsOrder(t *testing.T) {
	// MatchDeclarationsOrder was previously a dead config. Default
	// true: @param tags must appear in the same order as the function's
	// parameters. Out-of-order tags fire.
	swappedOrder := `package test

fun anchor() {}

/**
 * Combines two values.
 * @param b the second value
 * @param a the first value
 */
fun combine(a: Int, b: Int): Int = a + b
`
	if findings := runRuleByName(t, "OutdatedDocumentation", swappedOrder); len(findings) == 0 {
		t.Fatal("expected finding for out-of-order @param tags under default MatchDeclarationsOrder=true")
	}

	// Flip the flag off and verify the same swapped order doesn't fire.
	var rule *rules.OutdatedDocumentationRule
	for _, candidate := range api.Registry {
		if candidate.ID == "OutdatedDocumentation" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.OutdatedDocumentationRule)
			if !ok {
				t.Fatalf("expected OutdatedDocumentationRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("OutdatedDocumentation rule not registered")
	}
	original := rule.MatchDeclarationsOrder
	defer func() { rule.MatchDeclarationsOrder = original }()
	rule.MatchDeclarationsOrder = false

	if findings := runRuleByName(t, "OutdatedDocumentation", swappedOrder); len(findings) != 0 {
		t.Fatalf("expected no findings under MatchDeclarationsOrder=false for swapped order, got %d", len(findings))
	}
}

// --- KDocReferencesNonPublicProperty ---

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

// --- SampleAnnotationFreshness ---

func TestSampleAnnotationFreshness_Positive(t *testing.T) {
	findings := runSampleAnnotationFreshnessFixture(t, map[string]string{
		"src/main/kotlin/comments/Api.kt": `package comments

/**
 * References a sample that no longer exists.
 * @sample comments.removedSample
 */
fun documentedApi(): String = "api"
`,
	})
	if len(findings) != 1 {
		t.Fatalf("expected one finding for missing @sample target, got %d: %+v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "comments.removedSample") {
		t.Fatalf("finding message did not name missing sample: %q", findings[0].Message)
	}
}

func TestSampleAnnotationFreshness_NegativeValidSample(t *testing.T) {
	findings := runSampleAnnotationFreshnessFixture(t, map[string]string{
		"src/main/kotlin/comments/Samples.kt": `package comments

fun validSample(): String = "ok"
`,
		"src/main/kotlin/comments/Api.kt": `package comments

/**
 * References a sample that still exists.
 * @sample comments.validSample
 */
fun documentedApi(): String = validSample()
`,
	})
	if len(findings) != 0 {
		t.Fatalf("expected no findings for valid @sample target, got %d: %+v", len(findings), findings)
	}
}

func TestSampleAnnotationFreshness_NegativeSuppressedDeclaration(t *testing.T) {
	findings := runSampleAnnotationFreshnessFixture(t, map[string]string{
		"src/main/kotlin/comments/Api.kt": `package comments

/**
 * References a sample that is intentionally provided out of tree.
 * @sample comments.externalSample
 */
@Suppress("SampleAnnotationFreshness")
fun documentedApi(): String = "api"
`,
	})
	if len(findings) != 0 {
		t.Fatalf("expected no findings for suppressed declaration, got %d: %+v", len(findings), findings)
	}
}

// --- KdocLinkValidation ---

func TestKdocLinkValidation_Positive(t *testing.T) {
	findings := runCommentsCrossFileFixture(t, "KdocLinkValidation", map[string]string{
		"src/main/kotlin/comments/Api.kt": `package comments

/**
 * Uses [MissingType] and [comments.removed.helper].
 */
fun documentedApi(): String = "api"
`,
	})
	if len(findings) != 2 {
		t.Fatalf("expected two findings for missing KDoc links, got %d: %+v", len(findings), findings)
	}
}

func TestKdocLinkValidation_NegativeResolvableAndExternalLinks(t *testing.T) {
	findings := runCommentsCrossFileFixture(t, "KdocLinkValidation", map[string]string{
		"src/main/kotlin/comments/Symbols.kt": `package comments

class ExistingType

fun existingHelper(): String = "ok"
`,
		"src/main/kotlin/comments/Api.kt": `package comments

/**
 * Uses [ExistingType], [comments.existingHelper], [String], and [kotlin.collections.List].
 * Markdown [external](https://example.com) links are not KDoc symbols.
 */
fun documentedApi(): ExistingType = ExistingType()
`,
	})
	if len(findings) != 0 {
		t.Fatalf("expected no findings for resolvable or external KDoc links, got %d: %+v", len(findings), findings)
	}
}

func TestKdocLinkValidation_NegativeSuppressedDeclaration(t *testing.T) {
	findings := runCommentsCrossFileFixture(t, "KdocLinkValidation", map[string]string{
		"src/main/kotlin/comments/Api.kt": `package comments

/**
 * Uses [MissingType].
 */
@Suppress("KdocLinkValidation")
fun documentedApi(): String = "api"
`,
	})
	if len(findings) != 0 {
		t.Fatalf("expected no findings for suppressed declaration, got %d: %+v", len(findings), findings)
	}
}

func TestKdocLinkValidation_NegativeScopeImportsAndQualifiedMembers(t *testing.T) {
	findings := runCommentsCrossFileFixture(t, "KdocLinkValidation", map[string]string{
		"src/main/kotlin/comments/Api.kt": `package comments

import android.content.Context

class Target {
    fun member() {}
}

/**
 * Uses [Context], [Target.member], [value], [T], [android.view.View].
 */
fun <T> documentedApi(value: Int, other: T) {}

/** Holds [name] and [secret]. */
class Holder(val name: String, private val secret: String)
`,
	})
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %+v", len(findings), findings)
	}
	findings = runCommentsCrossFileFixture(t, "KdocLinkValidation", map[string]string{
		"src/main/kotlin/comments/Api.kt": `package comments

class Target {
    fun member() {}
}

/** Uses [Target.missing] and [Gone.member]. */
fun documentedApi() {}
`,
	})
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d: %+v", len(findings), findings)
	}
}

func runSampleAnnotationFreshnessFixture(t *testing.T, files map[string]string) []scanner.Finding {
	return runCommentsCrossFileFixture(t, "SampleAnnotationFreshness", files)
}

func runCommentsCrossFileFixture(t *testing.T, ruleName string, files map[string]string) []scanner.Finding {
	t.Helper()
	root := t.TempDir()
	parsed := make([]*scanner.File, 0, len(files))
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		file, err := scanner.ParseFile(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		file.Suppression = scanner.BuildSuppressionFilter(file, nil, nil, "")
		file.SuppressionIdx = file.Suppression.Annotations()
		parsed = append(parsed, file)
	}
	index := scanner.BuildIndex(parsed, 1)
	rule := commentsCrossFileRuleForTest(t, ruleName)
	collector := scanner.NewFindingCollector(0)
	rule.Check(&api.Context{
		CodeIndex:   index,
		ParsedFiles: parsed,
		Collector:   collector,
		Rule:        rule,
	})
	return collector.Columns().Findings()
}

func commentsCrossFileRuleForTest(t *testing.T, ruleName string) *api.Rule {
	t.Helper()
	for _, rule := range api.Registry {
		if rule.ID == ruleName {
			return rule
		}
	}
	t.Fatalf("%s rule not registered", ruleName)
	return nil
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

func TestDeprecatedBlockTag_NegativeLongerTagName(t *testing.T) {
	// `@deprecatedSince` is not the `@deprecated` block tag — it is a
	// different (hypothetical) tag whose name begins with the same
	// letters. The substring scan previously matched it anyway.
	findings := runRuleByName(t, "DeprecatedBlockTag", `
package test

/**
 * Description.
 * @deprecatedSince 1.4 used by custom tooling.
 */
fun helper() {}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for @deprecatedSince custom tag, got %d", len(findings))
	}
}

func TestDeprecatedBlockTag_NegativeInlineCodeSpan(t *testing.T) {
	// `@deprecated` mentioned inside a Markdown code span in a KDoc
	// description is documentation about the tag, not the tag itself.
	findings := runRuleByName(t, "DeprecatedBlockTag", `
package test

/**
 * Returns the legacy result. To mark a function as removed in a future
 * release, prefer the [`+"`@Deprecated`"+`] annotation over a free-form
 * `+"`@deprecated`"+` block tag in KDoc.
 */
fun legacy() {}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when @deprecated appears only inside a code span, got %d", len(findings))
	}
}

func TestUndocumentedPublicFunction_MembersOfNonPublicClassesAreSkipped(t *testing.T) {
	findings := runRuleByName(t, "UndocumentedPublicFunction", `
package test

private class Hidden {
    fun a() {}
}

internal class Shared {
    fun b() {}
}

class Visible {
    private class Inner {
        fun c() {}
    }
    fun d() {}
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected only Visible.d to be reported, got %d: %v", len(findings), findings)
	}
}

func TestUndocumentedPublicClass_ReportsObjects(t *testing.T) {
	findings := runRuleByName(t, "UndocumentedPublicClass", `
package test

object Registry

/** Documented. */
object Documented

private object Hidden
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for Registry, got %d: %v", len(findings), findings)
	}
}

func TestEndOfSentenceFormat_WrappedSentenceAndColon(t *testing.T) {
	findings := runRuleByName(t, "EndOfSentenceFormat", `
package test

/**
 * This summary wraps
 * onto a second line.
 */
fun a() {}

/** Lists the options: */
fun b() {}

/**
 * This summary wraps
 * and never ends
 */
fun c() {}
`)
	if len(findings) != 1 {
		t.Fatalf("expected only the unterminated wrapped summary, got %d: %v", len(findings), findings)
	}
	fix := findings[0].Fix
	if fix == nil || fix.Replacement != "." {
		t.Fatalf("expected a period-insertion fix, got %+v", fix)
	}
}

func TestStripDeprecatedKDocTag_MessageIsNormalizedAndEscaped(t *testing.T) {
	file := parseInline(t, `package test

/**
 * Old.
 * @deprecated use $NEW
 *   instead
 */
fun old() {}
`)
	findings := runRuleByNameOnFile(t, "DeprecatedBlockTag", file)
	if len(findings) != 1 || findings[0].Fix == nil {
		t.Fatalf("expected one finding with a fix, got %+v", findings)
	}
	want := `@Deprecated("use \$NEW instead")`
	if !strings.Contains(findings[0].Fix.Replacement, want) {
		t.Fatalf("fix %q does not contain %q", findings[0].Fix.Replacement, want)
	}
}

func TestDocumentationOverPrivateFunction_FixRemovesIndentation(t *testing.T) {
	code := "package test\n\nclass C {\n    /** doc. */\n    private fun a() {}\n}\n"
	findings := runRuleByName(t, "DocumentationOverPrivateFunction", code)
	if len(findings) != 1 || findings[0].Fix == nil {
		t.Fatalf("expected one finding with a fix, got %+v", findings)
	}
	fix := findings[0].Fix
	got := code[:fix.StartByte] + fix.Replacement + code[fix.EndByte:]
	want := "package test\n\nclass C {\n    private fun a() {}\n}\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestEndOfSentenceFormat_WrappedSentenceAndMidLineTerminator(t *testing.T) {
	findings := runRuleByName(t, "EndOfSentenceFormat", `
package test

/**
 * Wrapped first sentence that continues
 * onto the next line and ends there.
 */
class Wrapped

/**
 * First sentence. Second sentence that has no terminator
 */
class MidLine

/**
 * Intro to a list:
 * - one
 */
class ListIntro

/**
 * No terminator
 * on any line
 */
class Missing
`)
	if len(findings) != 1 {
		t.Fatalf("expected only the paragraph without any terminator, got %d findings", len(findings))
	}
	if findings[0].Fix == nil || findings[0].Fix.Replacement != "." {
		t.Fatalf("expected a period fix, got %+v", findings[0].Fix)
	}
}

func TestDocumentationOverPrivateFunction_FixRemovesWholeKdocLines(t *testing.T) {
	findings := runRuleByName(t, "DocumentationOverPrivateFunction", `
package test

class A {
    /** Explains the helper. */
    private fun helper() {}

    /**
     * Multi-line
     * explanation.
     */
    private fun other() {}
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}
	src := "\npackage test\n\nclass A {\n    /** Explains the helper. */\n    private fun helper() {}\n"
	for _, f := range findings[:1] {
		if f.Fix == nil {
			t.Fatal("expected a fix")
		}
		start, end := f.Fix.StartByte, f.Fix.EndByte
		if got := src[:start] + src[end:]; got != "\npackage test\n\nclass A {\n    private fun helper() {}\n" {
			t.Errorf("fix left a doubled indent or blank line: %q", got)
		}
	}
}

// Expected results come from running detekt 1.23.8 on the same sources: the finding is on the private or
// internal body property that the class KDoc references; member and top-level KDoc are not looked at.
func TestKDocReferencesNonPublicProperty_MatchesDetekt(t *testing.T) {
	findings := runRuleByName(t, "KDocReferencesNonPublicProperty", `
package test

/** Uses [priv], [pub], [intl], [prot] and [ctorPriv]. */
class A(private val ctorPriv: Int) {
    private val priv = 1
    val pub = 2
    internal val intl = 3
    protected val prot = 4

    /** Member doc [priv] */
    fun f() {}
}

/** Top-level doc [topPriv] */
fun g() {}
private val topPriv = 1
`)
	if len(findings) != 2 || findings[0].Line != 6 || findings[1].Line != 8 ||
		findings[0].Message != "The property priv is non-public and should not be referenced from KDoc comments." {
		t.Fatalf("expected findings on priv (line 6) and intl (line 8), got %v", findings)
	}
}
