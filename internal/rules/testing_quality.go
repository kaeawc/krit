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
	ctx.Emit(r.Finding(file, file.FlatRow(ctx.Idx)+1, file.FlatCol(ctx.Idx)+1,
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
	// RealDispatcherMarkers are strings that mark a test class as deliberately running on real dispatchers.
	RealDispatcherMarkers []string
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
	lower := scanner.LowerPath(file.Path)
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
	// camelCase names (`doesNotCrash`) have no spaces once lower-cased.
	squashed := strings.ReplaceAll(name, " ", "")
	if strings.Contains(name, "no crash") ||
		strings.Contains(name, "does not crash") ||
		strings.Contains(name, "doesn't crash") ||
		strings.Contains(squashed, "smoke") ||
		strings.Contains(squashed, "nocrash") ||
		strings.Contains(squashed, "doesnotcrash") ||
		strings.Contains(squashed, "doesnotthrow") {
		return true
	}
	body, _ := file.FlatFindChild(idx, "function_body")
	if body == 0 {
		return false
	}
	return testingQualityBodyDocumentsSmokeTest(file, body) || testingQualityComposeSmokeWithComment(file, body)
}

// composeSmokeCommentWords mark a comment that explains why reaching the end of a Compose test is the check.
var composeSmokeCommentWords = []string{
	"without", "reaching here", "reach here", "completes", "completed", "crash", "throw", "exception",
	"survive", "measure", "layout", "no error",
}

// testingQualityComposeSmokeWithComment reports a Compose UI test that sets content and then waits for idle,
// with a comment saying that finishing without an exception is the expectation.
func testingQualityComposeSmokeWithComment(file *scanner.File, body uint32) bool {
	hasSetContent, hasIdle := false, false
	file.FlatWalkNodes(body, "call_expression", func(call uint32) {
		switch flatCallNameAny(file, call) {
		case "setContent":
			hasSetContent = true
		case "waitForIdle":
			hasIdle = true
		}
	})
	if !hasSetContent || !hasIdle {
		return false
	}
	found := false
	file.FlatWalkAllNodes(body, func(n uint32) {
		if found || !isFlatCommentNode(file, n) {
			return
		}
		text := strings.ToLower(file.FlatNodeText(n))
		for _, word := range composeSmokeCommentWords {
			if strings.Contains(text, word) {
				found = true
				return
			}
		}
	})
	return found
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
		"no exception",
		"without throwing",
		"without crashing",
		"without crash",
		"no crash",
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

// testingQualitySuperclasses returns the simple names of the superclasses a class constructs.
// Interfaces have no constructor call and add no inheritance depth, so they are left out.
func testingQualitySuperclasses(file *scanner.File, classDecl uint32) []string {
	if file == nil || classDecl == 0 {
		return nil
	}
	var names []string
	for child := file.FlatFirstChild(classDecl); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "delegation_specifier" {
			continue
		}
		if _, ok := file.FlatFindChild(child, "constructor_invocation"); !ok {
			continue
		}
		text := strings.TrimSpace(file.FlatNodeText(child))
		if cut := strings.IndexAny(text, "(<"); cut > 0 {
			text = text[:cut]
		}
		if dot := strings.LastIndexByte(text, '.'); dot >= 0 {
			text = text[dot+1:]
		}
		names = append(names, strings.TrimSpace(text))
	}
	return names
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
		supers := testingQualitySuperclasses(file, n)
		for _, s := range supers {
			d := testingQualityCountInheritanceInFile(file, s, depth+1)
			if d > depth {
				depth = d
			}
		}
	})

	return depth
}

// testingQualityBodyEndsWithCast reports a test whose last statement is a throwing check: a cast, `!!`, or a check/require call.
func testingQualityBodyEndsWithCast(file *scanner.File, body uint32) bool {
	last := testingQualityLastStatement(file, body)
	// `runTest { ...; x as T }` runs the lambda in place, so its last statement is the test's last statement.
	for depth := 0; last != 0 && depth < 3 && file.FlatType(last) == "call_expression" && testingQualityRunnerCalls[flatCallNameAny(file, last)]; depth++ {
		lambda := flatCallTrailingLambda(file, last)
		if lambda == 0 {
			return false
		}
		last = testingQualityLastStatement(file, lambda)
	}
	return last != 0 && testingQualityIsThrowingCheck(file, last)
}

// testingQualityRunnerCalls run their trailing lambda exactly once, in place.
var testingQualityRunnerCalls = map[string]bool{"runTest": true, "runBlocking": true, "runBlockingTest": true}

// testingQualityLastStatement returns the last non-comment named child of a function body or lambda.
func testingQualityLastStatement(file *scanner.File, parent uint32) uint32 {
	if statements, ok := file.FlatFindChild(parent, "statements"); ok {
		parent = statements
	}
	last := uint32(0)
	file.FlatForEachChild(parent, func(child uint32) {
		if file.FlatIsNamed(child) && !isFlatCommentNode(file, child) {
			last = child
		}
	})
	return last
}

// testingQualityIsSafeCast reports whether an as_expression uses the `as?` operator token.
func testingQualityIsSafeCast(file *scanner.File, node uint32) bool {
	safe := false
	file.FlatForEachChild(node, func(child uint32) {
		if !file.FlatIsNamed(child) && file.FlatNodeText(child) == "as?" {
			safe = true
		}
	})
	return safe
}

var testingQualityThrowingCheckCalls = map[string]bool{
	"checkNotNull": true, "requireNotNull": true, "assertIs": true, "assertIsNot": true, "check": true, "require": true,
}

// testingQualityIsThrowingCheck reports a cast, `!!`, or a stdlib/kotlin.test check call that throws on failure.
func testingQualityIsThrowingCheck(file *scanner.File, node uint32) bool {
	switch file.FlatType(node) {
	case "as_expression":
		// A safe cast yields null on a wrong type instead of throwing.
		return !testingQualityIsSafeCast(file, node)
	case "postfix_expression":
		return strings.HasSuffix(strings.TrimSpace(file.FlatNodeText(node)), "!!")
	case "call_expression":
		return testingQualityThrowingCheckCalls[flatCallNameAny(file, node)]
	case "property_declaration":
		last := uint32(0)
		file.FlatForEachChild(node, func(child uint32) {
			if file.FlatIsNamed(child) && !isFlatCommentNode(file, child) {
				last = child
			}
		})
		return last != 0 && last != node && file.FlatType(last) != "property_declaration" && testingQualityIsThrowingCheck(file, last)
	}
	return false
}

const testingQualityOperandMaxLen = 60

// testingQualityShortOperand collapses an expression to one line and cuts it to a short length for messages.
func testingQualityShortOperand(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	if len(runes) <= testingQualityOperandMaxLen {
		return text
	}
	return string(runes[:testingQualityOperandMaxLen-3]) + "..."
}

// testingQualityFileUsesTruth reports a file that imports Google Truth.
func testingQualityFileUsesTruth(file *scanner.File) bool {
	return fileFactsCache().Imports(file).HasAnyPrefix("com.google.common.truth.")
}
