package rules

import (
	"bytes"
	"path"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const testingQualityRuleSet = "testing-quality"

// AssertEqualsArgumentOrderRule detects assertEquals(actual, expected)
// when the arguments are simple identifiers named exactly "actual" and
// "expected". This keeps the first iteration narrow and low-noise.
type AssertEqualsArgumentOrderRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Testing-quality rule. Detection matches on assertion framework call
// shapes (JUnit, Truth, Kotest, MockK) by name — cross-library identifier
// collisions can produce false positives. Classified per roadmap/17.
func (r *AssertEqualsArgumentOrderRule) Confidence() float64 { return api.ConfidenceMedium }

func testingQualityValueArgumentsFlat(file *scanner.File, args uint32) []uint32 {
	var valueArgs []uint32
	for i := 0; i < file.FlatNamedChildCount(args); i++ {
		child := file.FlatNamedChild(args, i)
		if child != 0 && file.FlatType(child) == "value_argument" {
			valueArgs = append(valueArgs, child)
		}
	}
	return valueArgs
}

// AssertTrueOnComparisonRule detects assertTrue(actual == expected)
// and recommends assertEquals for better failure messages.
type AssertTrueOnComparisonRule struct {
	FlatDispatchBase
	BaseRule
}

func testingQualityAssertTrueOnComparisonKnownCall(file *scanner.File, idx uint32) bool {
	navExpr, _ := flatCallExpressionParts(file, idx)
	if navExpr == 0 {
		return true
	}
	segments := flatNavigationChainIdentifiers(file, navExpr)
	if len(segments) <= 1 || segments[len(segments)-1] != "assertTrue" {
		return true
	}
	receiver := strings.Join(segments[:len(segments)-1], ".")
	switch receiver {
	case "Assert", "org.junit.Assert", "junit.framework.Assert",
		"TestCase", "junit.framework.TestCase",
		"Assertions", "org.junit.jupiter.api.Assertions",
		"kotlin.test":
		return true
	default:
		return false
	}
}

// Confidence reports a tier-2 (medium) base confidence. Testing-quality rule. Detection matches
// receiverless assertTrue calls and known JUnit receiver qualifiers by AST shape. Classified per roadmap/17.
func (r *AssertTrueOnComparisonRule) Confidence() float64 { return api.ConfidenceMedium }

// MixedAssertionLibrariesRule detects files that import both JUnit Assert and
// Truth APIs in the import header.
type MixedAssertionLibrariesRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Testing-quality rule. Detection matches on assertion framework call
// shapes (JUnit, Truth, Kotest, MockK) by name — cross-library identifier
// collisions can produce false positives. Classified per roadmap/17.
func (r *MixedAssertionLibrariesRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *MixedAssertionLibrariesRule) check(ctx *api.Context) {
	file := ctx.File
	imports := fileFactsCache().Imports(file)
	if !imports.HasAnyPrefix("org.junit.Assert.") || !imports.HasAnyPrefix("com.google.common.truth.Truth.") {
		return
	}
	ctx.Emit(r.Finding(file, file.FlatRow(ctx.Idx)+1, 1,
		"Avoid mixing JUnit Assert and Truth imports in the same file; pick one assertion library."))
}

// ---------------------------------------------------------------------------
// AssertNullableWithNotNullAssertionRule
// ---------------------------------------------------------------------------

type AssertNullableWithNotNullAssertionRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *AssertNullableWithNotNullAssertionRule) Confidence() float64 { return api.ConfidenceMedium }

// ---------------------------------------------------------------------------
// MockWithoutVerifyRule
// ---------------------------------------------------------------------------

type MockWithoutVerifyRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *MockWithoutVerifyRule) Confidence() float64 { return api.ConfidenceMedium }

// ---------------------------------------------------------------------------
// RunTestWithDelayRule
// ---------------------------------------------------------------------------

type RunTestWithDelayRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *RunTestWithDelayRule) Confidence() float64 { return api.ConfidenceMedium }

// ---------------------------------------------------------------------------
// RunTestWithThreadSleepRule
// ---------------------------------------------------------------------------

type RunTestWithThreadSleepRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *RunTestWithThreadSleepRule) Confidence() float64 { return api.ConfidenceMedium }

// ---------------------------------------------------------------------------
// RunBlockingInTestRule
// ---------------------------------------------------------------------------

type RunBlockingInTestRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *RunBlockingInTestRule) Confidence() float64 { return api.ConfidenceMedium }

// ---------------------------------------------------------------------------
// TestDispatcherNotInjectedRule
// ---------------------------------------------------------------------------

type TestDispatcherNotInjectedRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *TestDispatcherNotInjectedRule) Confidence() float64 { return api.ConfidenceMedium }

// ---------------------------------------------------------------------------
// TestWithoutAssertionRule
// ---------------------------------------------------------------------------

type TestWithoutAssertionRule struct {
	FlatDispatchBase
	BaseRule

	AllowNoAssertionTests   bool
	AssertionMethodPatterns []string
}

func (r *TestWithoutAssertionRule) Confidence() float64 { return api.ConfidenceMedium }

func testingQualityIsEditorTemplatePath(filePath string) bool {
	filePath = strings.ReplaceAll(filePath, "\\", "/")
	clean := strings.TrimPrefix(path.Clean(filePath), "./")
	return clean == ".idea" ||
		strings.HasPrefix(clean, ".idea/") ||
		strings.Contains(clean, "/.idea/")
}

// ---------------------------------------------------------------------------
// TestWithOnlyTodoRule
// ---------------------------------------------------------------------------

type TestWithOnlyTodoRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *TestWithOnlyTodoRule) Confidence() float64 { return api.ConfidenceMedium }

// ---------------------------------------------------------------------------
// TestFunctionReturnValueRule
// ---------------------------------------------------------------------------

type TestFunctionReturnValueRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *TestFunctionReturnValueRule) Confidence() float64 { return api.ConfidenceMedium }

// ---------------------------------------------------------------------------
// TestNameContainsUnderscoreRule
// ---------------------------------------------------------------------------

type TestNameContainsUnderscoreRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *TestNameContainsUnderscoreRule) Confidence() float64 { return api.ConfidenceMediumLow }

// ---------------------------------------------------------------------------
// SharedMutableStateInObjectRule
// ---------------------------------------------------------------------------

type SharedMutableStateInObjectRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *SharedMutableStateInObjectRule) Confidence() float64 { return api.ConfidenceMedium }

// ---------------------------------------------------------------------------
// TestInheritanceDepthRule
// ---------------------------------------------------------------------------

type TestInheritanceDepthRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *TestInheritanceDepthRule) Confidence() float64 { return api.ConfidenceMediumLow }

// RelaxedMockUsedForValueClassRule lives in testing_quality_relaxed_mock.go.

// ---------------------------------------------------------------------------
// SpyOnDataClassRule
// ---------------------------------------------------------------------------

type SpyOnDataClassRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *SpyOnDataClassRule) Confidence() float64 { return api.ConfidenceMediumLow }

// ---------------------------------------------------------------------------
// VerifyWithoutMockRule
// ---------------------------------------------------------------------------

type VerifyWithoutMockRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *VerifyWithoutMockRule) Confidence() float64 { return api.ConfidenceMediumLow }

func testingQualityIsTestFile(file *scanner.File) bool {
	if file == nil {
		return false
	}
	return strings.Contains(file.Path, "/test") ||
		strings.Contains(file.Path, "Test.kt") ||
		strings.Contains(file.Path, "/testing-quality/")
}

func testingQualityRuleShouldRunInFile(file *scanner.File) bool {
	if file == nil {
		return false
	}
	lower := strings.ToLower(file.Path)
	return isTestSupportFile(file.Path) ||
		strings.HasSuffix(lower, "test.kt") ||
		(strings.Contains(lower, "/tests/fixtures/") && strings.Contains(lower, "/testing-quality/"))
}

func testingQualityIsTestFunction(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "function_declaration" {
		return false
	}
	return flatHasAnnotationNamed(file, idx, "Test")
}

func testingQualityIsTestLikeFunction(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "function_declaration" {
		return false
	}
	for _, annotation := range []string{"Test", "ParameterizedTest", "TestFactory", "TestTemplate"} {
		if flatHasAnnotationNamed(file, idx, annotation) {
			return true
		}
	}
	name := strings.Trim(testingQualityFunctionName(file, idx), "`")
	return strings.HasPrefix(name, "test")
}

func testingQualityTestExpectsException(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	if mods, ok := file.FlatFindChild(idx, "modifiers"); ok {
		for child := file.FlatFirstChild(mods); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) != "annotation" && file.FlatType(child) != "modifier" {
				continue
			}
			text := file.FlatNodeText(child)
			if strings.Contains(text, "Test") && strings.Contains(text, "expected") {
				return true
			}
		}
	}
	return false
}

func testingQualityTestAllowsNoCrash(file *scanner.File, idx uint32) bool {
	name := strings.ToLower(strings.Trim(testingQualityFunctionName(file, idx), "`"))
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")
	if strings.Contains(name, "no crash") ||
		strings.Contains(name, "does not crash") ||
		strings.Contains(name, "doesn't crash") {
		return true
	}
	body, _ := file.FlatFindChild(idx, "function_body")
	if body == 0 {
		return false
	}
	return testingQualityBodyDocumentsSmokeTest(file, body)
}

func testingQualityBodyDocumentsSmokeTest(file *scanner.File, body uint32) bool {
	start, end := file.FlatStartByte(body), file.FlatEndByte(body)
	if end <= start || int(end) > len(file.Content) {
		return false
	}
	source := file.Content[start:end]
	if !bytes.Contains(source, []byte("crash")) &&
		!bytes.Contains(source, []byte("Crash")) &&
		!bytes.Contains(source, []byte("throw")) &&
		!bytes.Contains(source, []byte("Throw")) &&
		!bytes.Contains(source, []byte("fail")) &&
		!bytes.Contains(source, []byte("Fail")) &&
		!bytes.Contains(source, []byte("exception")) &&
		!bytes.Contains(source, []byte("Exception")) {
		return false
	}
	return testingQualityTextDocumentsSmokeTest(string(source))
}

func testingQualityTextDocumentsSmokeTest(text string) bool {
	text = strings.ToLower(text)
	text = strings.ReplaceAll(text, "’", "'")
	smokePhrases := []string{
		"shouldn't crash",
		"should not crash",
		"doesn't crash",
		"does not crash",
		"shouldn't throw",
		"should not throw",
		"doesn't throw",
		"does not throw",
		"does not cause an exception",
		"doesn't cause an exception",
		"shouldn't fail",
		"should not fail",
		"doesn't fail",
		"does not fail",
	}
	for _, phrase := range smokePhrases {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return false
}

func testingQualityIsIgnoredTest(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	if flatHasAnnotationNamed(file, idx, "Ignore") || flatHasAnnotationNamed(file, idx, "Disabled") {
		return true
	}
	if owner, ok := flatEnclosingAncestor(file, idx, "class_declaration", "object_declaration"); ok {
		return flatHasAnnotationNamed(file, owner, "Ignore") || flatHasAnnotationNamed(file, owner, "Disabled")
	}
	return false
}

func testingQualityInfixOperatorName(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 || file.FlatType(idx) != "infix_expression" {
		return ""
	}
	seenLeft := false
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		if !seenLeft {
			seenLeft = true
			continue
		}
		if file.FlatType(child) == "simple_identifier" {
			return file.FlatNodeString(child, nil)
		}
	}
	return ""
}

func testingQualityFunctionName(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "simple_identifier", "identifier":
			return file.FlatNodeText(child)
		}
	}
	return ""
}

func testingQualityReturnType(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "user_type" || file.FlatType(child) == "type_identifier" {
			return strings.TrimSpace(file.FlatNodeText(child))
		}
	}
	return ""
}

func testingQualitySupertypes(file *scanner.File, classDecl uint32) []string {
	if file == nil || classDecl == 0 {
		return nil
	}
	var names []string
	for child := file.FlatFirstChild(classDecl); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "delegation_specifier" {
			continue
		}
		text := strings.TrimSpace(file.FlatNodeText(child))
		if paren := strings.IndexByte(text, '('); paren > 0 {
			text = text[:paren]
		}
		names = append(names, text)
	}
	return names
}

func testingQualityCountInheritanceInFile(file *scanner.File, className string, depth int) int {
	if depth > 10 || file == nil {
		return depth
	}

	file.FlatWalkAllNodes(0, func(n uint32) {
		if file.FlatType(n) != "class_declaration" {
			return
		}
		name := ""
		for child := file.FlatFirstChild(n); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "type_identifier" {
				name = file.FlatNodeText(child)
				break
			}
		}
		if name != className {
			return
		}
		supers := testingQualitySupertypes(file, n)
		for _, s := range supers {
			d := testingQualityCountInheritanceInFile(file, s, depth+1)
			if d > depth {
				depth = d
			}
		}
	})

	return depth
}
