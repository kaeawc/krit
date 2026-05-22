package rules

import (
	"bytes"
	"path"
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
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

var testingQualityMockCreationCalls = map[string]bool{
	"mockk":        true,
	"relaxedMockk": true,
	"mockkClass":   true,
	"spyk":         true,
	"mock":         true,
	"spy":          true,
}

var testingQualityMockUsageCalls = map[string]bool{
	"verify":          true,
	"coVerify":        true,
	"every":           true,
	"coEvery":         true,
	"whenever":        true,
	"when":            true,
	"given":           true,
	"confirmVerified": true,
}

var testingQualityObjectMockCreationCalls = map[string]bool{
	"mockkObject":      true,
	"mockkStatic":      true,
	"mockkConstructor": true,
}

func testingQualityCallPassesReferenceAsValueArgument(file *scanner.File, call uint32, name string) bool {
	if file == nil || call == 0 || name == "" {
		return false
	}
	_, args := flatCallExpressionParts(file, call)
	if args == 0 {
		return false
	}
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		found := false
		file.FlatWalkAllNodes(arg, func(n uint32) {
			if found {
				return
			}
			if file.FlatType(n) == "simple_identifier" && file.FlatNodeTextEquals(n, name) {
				found = true
			}
		})
		if found {
			return true
		}
	}
	return false
}

func testingQualityMockStubbingInfixUsesReference(file *scanner.File, call uint32, name string) bool {
	if file == nil || call == 0 || name == "" {
		return false
	}
	for parent, ok := file.FlatParent(call); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "infix_expression":
			if testingQualityIsMockStubbingInfix(file, parent) && subtreeHasReferenceName(file, parent, name) {
				return true
			}
		case "function_body", "lambda_literal", "class_body", "object_declaration", "function_declaration":
			return false
		}
	}
	return false
}

func testingQualityIsMockStubbingInfix(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "infix_expression" {
		return false
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "simple_identifier" {
			continue
		}
		switch file.FlatNodeText(child) {
		case "returns", "returnsMany", "andThen", "throws", "answers", "coAnswers":
			return true
		}
	}
	return false
}

func testingQualityMockCreationCallHasInitializerStubbing(file *scanner.File, call uint32) bool {
	if file == nil || call == 0 {
		return false
	}
	lambda := flatCallTrailingLambda(file, call)
	if lambda == 0 {
		return false
	}
	found := false
	file.FlatWalkAllNodes(lambda, func(n uint32) {
		if found || file.FlatType(n) != "call_expression" {
			return
		}
		found = testingQualityMockUsageCalls[flatCallNameAny(file, n)]
	})
	return found
}

func testingQualityIsMockKVerifyCall(file *scanner.File, idx uint32, name string) bool {
	if name != "verify" && name != "coVerify" {
		return false
	}
	nav, _ := flatCallExpressionParts(file, idx)
	if nav != 0 {
		segments := flatNavigationChainIdentifiers(file, nav)
		if len(segments) == 3 && segments[0] == "io" && segments[1] == "mockk" && segments[2] == name {
			return true
		}
		return false
	}
	return fileImportsFQN(file, "io.mockk."+name)
}

type testingQualityVerifyReceiver struct {
	display    string
	candidates []string
}

func testingQualityDirectVerifyReceivers(file *scanner.File, lambda uint32) []testingQualityVerifyReceiver {
	if file == nil || lambda == 0 {
		return nil
	}
	var receivers []testingQualityVerifyReceiver
	addReceiver := func(stmt uint32) {
		if stmt == 0 || file.FlatType(stmt) != "call_expression" {
			return
		}
		name := flatCallNameAny(file, stmt)
		if testingQualityIsAssertionCall(name) {
			return
		}
		if testingQualityVerifyWrapperCall(name) {
			if nested := flatCallTrailingLambda(file, stmt); nested != 0 {
				receivers = append(receivers, testingQualityDirectVerifyReceivers(file, nested)...)
			}
			return
		}
		if receiver := testingQualityVerifyReceiverFromCall(file, stmt); receiver.display != "" {
			receivers = append(receivers, receiver)
		}
	}
	if statements, ok := file.FlatFindChild(lambda, "statements"); ok {
		for stmt := file.FlatFirstChild(statements); stmt != 0; stmt = file.FlatNextSib(stmt) {
			if file.FlatIsNamed(stmt) {
				addReceiver(stmt)
			}
		}
		return receivers
	}
	for child := file.FlatFirstChild(lambda); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) {
			addReceiver(child)
		}
	}
	return receivers
}

func testingQualityVerifyWrapperCall(name string) bool {
	switch name {
	case "forEach", "forEachIndexed", "repeat":
		return true
	default:
		return false
	}
}

func testingQualityVerifyReceiverFromCall(file *scanner.File, call uint32) testingQualityVerifyReceiver {
	leaf := flatReceiverNameFromCall(file, call)
	if leaf == "" {
		return testingQualityVerifyReceiver{}
	}
	receiver := testingQualityVerifyReceiver{
		display:    leaf,
		candidates: []string{leaf},
	}
	nav, _ := flatCallExpressionParts(file, call)
	segments := flatNavigationChainIdentifiers(file, nav)
	if len(segments) > 1 && segments[len(segments)-1] == flatCallNameAny(file, call) {
		segments = segments[:len(segments)-1]
	}
	if len(segments) > 0 && segments[0] != "" && segments[0] != leaf {
		receiver.candidates = append(receiver.candidates, segments[0])
	}
	return receiver
}

func testingQualityVerifyReceiverKnownMock(mockVars map[string]bool, receiver testingQualityVerifyReceiver) bool {
	for _, candidate := range receiver.candidates {
		if mockVars[candidate] || testingQualityNameLooksMockReference(candidate) {
			return true
		}
	}
	return false
}

func testingQualityNameLooksMockReference(name string) bool {
	return testingQualityNameContainsTokenFold(name, "mock") ||
		testingQualityNameContainsTokenFold(name, "spy")
}

func testingQualityIdentifierFromDeclaration(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	varDecl, _ := file.FlatFindChild(idx, "variable_declaration")
	if varDecl == 0 {
		return ""
	}
	ident, _ := file.FlatFindChild(varDecl, "simple_identifier")
	if ident == 0 {
		return ""
	}
	return file.FlatNodeText(ident)
}

func testingQualityPropertyInitializer(file *scanner.File, idx uint32) uint32 {
	return propertyInitializerExpression(file, idx)
}

func testingQualityCallCreatesMock(file *scanner.File, idx uint32, helperFuncs map[string]bool) bool {
	name := flatCallNameAny(file, idx)
	return testingQualityMockCreationCalls[name] || helperFuncs[name]
}

func testingQualityExprCreatesMock(file *scanner.File, idx uint32, helperFuncs map[string]bool) bool {
	if file == nil || idx == 0 {
		return false
	}
	idx = flatUnwrapParenExpr(file, idx)
	if file.FlatType(idx) != "call_expression" {
		return false
	}
	if testingQualityCallCreatesMock(file, idx, helperFuncs) {
		return true
	}
	if !testingQualityScopeFunctionReturnsReceiver(flatCallNameAny(file, idx)) {
		return false
	}
	return testingQualityExprCreatesMock(file, testingQualityCallReceiverExpression(file, idx), helperFuncs)
}

func testingQualityScopeFunctionReturnsReceiver(name string) bool {
	return name == "apply" || name == "also"
}

func testingQualityCallReceiverExpression(file *scanner.File, call uint32) uint32 {
	nav, _ := flatCallExpressionParts(file, call)
	return testingQualityNavigationReceiverExpression(file, nav)
}

func testingQualityNavigationReceiverExpression(file *scanner.File, nav uint32) uint32 {
	if file == nil || nav == 0 || file.FlatType(nav) != "navigation_expression" {
		return 0
	}
	for child := file.FlatFirstChild(nav); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		if file.FlatType(child) == "navigation_expression" {
			return testingQualityNavigationReceiverExpression(file, child)
		}
		return child
	}
	return 0
}

func testingQualityAssignmentParts(file *scanner.File, idx uint32) (string, uint32) {
	if file == nil || idx == 0 || file.FlatType(idx) != "assignment" {
		return "", 0
	}
	var lhs uint32
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		lhs = child
		break
	}
	rhs := assignmentRHS(file, idx)
	if lhs == 0 || rhs == 0 {
		return "", 0
	}
	name := ""
	file.FlatWalkAllNodes(lhs, func(n uint32) {
		if name == "" && file.FlatType(n) == "simple_identifier" {
			name = file.FlatNodeText(n)
		}
	})
	return name, rhs
}

func testingQualityMockedObjectName(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 || !testingQualityObjectMockCreationCalls[flatCallNameAny(file, idx)] {
		return ""
	}
	args := flatCallKeyArguments(file, idx)
	if args == 0 {
		return ""
	}
	arg := flatPositionalValueArgument(file, args, 0)
	expr := flatValueArgumentExpression(file, arg)
	if expr == 0 {
		return ""
	}
	switch file.FlatType(expr) {
	case "simple_identifier":
		return file.FlatNodeText(expr)
	case "navigation_expression":
		segments := flatNavigationChainIdentifiers(file, expr)
		if len(segments) > 0 {
			return segments[0]
		}
	case "class_literal":
		return testingQualityFirstSimpleIdentifier(file, expr)
	}
	if file.FlatType(expr) != "string_literal" {
		return testingQualityFirstSimpleIdentifier(file, expr)
	}
	return ""
}

func testingQualityFirstSimpleIdentifier(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	found := ""
	file.FlatWalkAllNodes(idx, func(n uint32) {
		if found != "" {
			return
		}
		switch file.FlatType(n) {
		case "simple_identifier", "type_identifier":
			found = file.FlatNodeText(n)
		}
	})
	return found
}

func testingQualityMockHelperFunctions(file *scanner.File) map[string]bool {
	if file == nil {
		return map[string]bool{}
	}
	return filefacts.FileFact(fileFactsCache(), file, slotTestQualityMockHelpers, func() map[string]bool {
		helpers := make(map[string]bool)
		file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
			name := testingQualityFunctionName(file, fn)
			if name == "" {
				return
			}
			mockVars := make(map[string]bool)
			if body, ok := file.FlatFindChild(fn, "function_body"); ok {
				if expr := testingQualityFunctionBodyExpression(file, body); testingQualityExprCreatesMock(file, expr, helpers) {
					helpers[name] = true
					return
				}
				file.FlatWalkAllNodes(body, func(n uint32) {
					switch file.FlatType(n) {
					case "property_declaration":
						varName := testingQualityIdentifierFromDeclaration(file, n)
						init := testingQualityPropertyInitializer(file, n)
						if varName != "" && testingQualityExprCreatesMock(file, init, helpers) {
							mockVars[varName] = true
						}
					case "jump_expression":
						if !strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(n)), "return") {
							return
						}
						if file.FlatNamedChildCount(n) == 0 {
							return
						}
						expr := file.FlatNamedChild(n, 0)
						if testingQualityExprCreatesMock(file, expr, helpers) {
							helpers[name] = true
							return
						}
						if file.FlatType(expr) == "simple_identifier" && mockVars[file.FlatNodeText(expr)] {
							helpers[name] = true
						}
					}
				})
			}
		})
		return helpers
	})
}

func testingQualityFunctionBodyExpression(file *scanner.File, body uint32) uint32 {
	if file == nil || body == 0 {
		return 0
	}
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		if file.FlatType(child) == "statements" {
			return 0
		}
		return child
	}
	return 0
}

func testingQualityCollectMockNames(file *scanner.File, fn uint32, helperFuncs map[string]bool) map[string]bool {
	if file == nil {
		return map[string]bool{}
	}
	return filefacts.NodeFact(fileFactsCache(), file, fn, slotTestQualityMockNames, func() map[string]bool {
		return testingQualityComputeMockNames(file, fn, helperFuncs)
	})
}

func testingQualityComputeMockNames(file *scanner.File, fn uint32, helperFuncs map[string]bool) map[string]bool {
	mockVars := make(map[string]bool)
	classProps := make(map[string]bool)
	recordProperty := func(prop uint32) {
		name := testingQualityIdentifierFromDeclaration(file, prop)
		if name == "" {
			return
		}
		if flatHasAnnotationNamed(file, prop, "MockK") || flatHasAnnotationNamed(file, prop, "RelaxedMockK") {
			mockVars[name] = true
			return
		}
		if testingQualityExprCreatesMock(file, testingQualityPropertyInitializer(file, prop), helperFuncs) {
			mockVars[name] = true
		}
	}
	recordCall := func(call uint32) {
		if objectName := testingQualityMockedObjectName(file, call); objectName != "" {
			mockVars[objectName] = true
		}
	}
	file.FlatWalkNodes(0, "property_declaration", func(prop uint32) {
		if _, insideFunction := flatEnclosingFunction(file, prop); insideFunction {
			return
		}
		if name := testingQualityIdentifierFromDeclaration(file, prop); name != "" {
			classProps[name] = true
		}
		recordProperty(prop)
	})
	file.FlatWalkNodes(0, "assignment", func(assign uint32) {
		name, rhs := testingQualityAssignmentParts(file, assign)
		if name != "" && classProps[name] && testingQualityExprCreatesMock(file, rhs, helperFuncs) {
			mockVars[name] = true
		}
	})
	file.FlatWalkNodes(0, "call_expression", recordCall)
	if fn != 0 {
		body, _ := file.FlatFindChild(fn, "function_body")
		if body != 0 {
			file.FlatWalkAllNodes(body, func(n uint32) {
				switch file.FlatType(n) {
				case "property_declaration":
					recordProperty(n)
				case "assignment":
					name, rhs := testingQualityAssignmentParts(file, n)
					if name != "" && testingQualityExprCreatesMock(file, rhs, helperFuncs) {
						mockVars[name] = true
					}
				case "call_expression":
					recordCall(n)
				}
			})
		}
	}
	return mockVars
}

// ---------------------------------------------------------------------------
// Testing-quality helpers
// ---------------------------------------------------------------------------

var assertionCallNames = map[string]bool{
	"assertEquals": true, "assertNotEquals": true,
	"assertTrue": true, "assertFalse": true,
	"assertNull": true, "assertNotNull": true,
	"assertSame": true, "assertNotSame": true,
	"assertThat": true, "assertThrows": true,
	"assertFailsWith": true, "assertIs": true,
	"assertIsNot": true, "assertContains": true,
	"assertContentEquals": true, "fail": true,
	"expectThat": true, "expect": true,
	"expectClean": true, "expectErrorCount": true, "expectWarningCount": true,
	"expectFixDiffs": true, "expectFixes": true, "expectMatches": true, "expectContains": true,
	"isEqualTo": true, "isNotEqualTo": true, "isSameInstanceAs": true, "isNotSameInstanceAs": true,
	"isNull": true, "isNotNull": true, "isTrue": true, "isFalse": true,
	"containsExactly": true, "containsAtLeast": true, "containsNoneOf": true,
	"hasSize": true, "isEmpty": true, "isNotEmpty": true,
	"shouldBe": true, "shouldNotBe": true,
	"shouldThrow": true, "shouldNotThrow": true,
	"buildAndAssertThatOutput":        true,
	"buildAndFailAndAssertThatOutput": true,
	"printPluginsAndAssertOutput":     true,
	"check":                           true,
	"snapshot":                        true, "captureRoboImage": true, "captureToImage": true,
	"measureRepeated": true,
	"waitUntil":       true, "testExecute": true, "testEnqueue": true,
	"complete": true,
	"intended": true, "intending": true,
}

func testingQualityIsAssertionCall(name string) bool {
	if assertionCallNames[name] {
		return true
	}
	if strings.HasPrefix(name, "assert") {
		return true
	}
	return testingQualityNameLooksAssertionEquivalent(name)
}

var nullableAssertionCallNames = map[string]bool{
	"assertEquals":        true,
	"assertNotEquals":     true,
	"assertTrue":          true,
	"assertFalse":         true,
	"assertNull":          true,
	"assertNotNull":       true,
	"assertSame":          true,
	"assertNotSame":       true,
	"assertThat":          true,
	"assertThrows":        true,
	"assertFailsWith":     true,
	"assertIs":            true,
	"assertIsNot":         true,
	"assertContains":      true,
	"assertContentEquals": true,
	"expectThat":          true,
	"expect":              true,
	"expectClean":         true,
	"expectErrorCount":    true,
	"expectWarningCount":  true,
	"expectFixDiffs":      true,
	"expectFixes":         true,
	"expectMatches":       true,
	"expectContains":      true,
	"isEqualTo":           true,
	"isNotEqualTo":        true,
	"isSameInstanceAs":    true,
	"isNotSameInstanceAs": true,
	"isNull":              true,
	"isNotNull":           true,
	"isTrue":              true,
	"isFalse":             true,
	"containsExactly":     true,
	"containsAtLeast":     true,
	"containsNoneOf":      true,
	"hasSize":             true,
	"isEmpty":             true,
	"isNotEmpty":          true,
	"shouldBe":            true,
	"shouldNotBe":         true,
	"shouldThrow":         true,
	"shouldNotThrow":      true,
}

func testingQualityIsNullableAssertionCall(name string) bool {
	if name == "" || name == "expectError" {
		return false
	}
	if nullableAssertionCallNames[name] {
		return true
	}
	return strings.HasPrefix(name, "assert") || testingQualityNameContainsTokenFold(name, "expect")
}

func testingQualityInsideNestedExecutableArgument(file *scanner.File, idx, root uint32) bool {
	if file == nil || idx == 0 || root == 0 {
		return false
	}
	for current, ok := file.FlatParent(idx); ok && current != root; current, ok = file.FlatParent(current) {
		switch file.FlatType(current) {
		case "lambda_literal", "function_literal", "anonymous_function", "function_declaration":
			return true
		}
	}
	return false
}

func testingQualityIsAssertionOrVerify(name string) bool {
	return testingQualityIsAssertionCall(name) ||
		testingQualityIsVerifyCall(name) ||
		strings.HasPrefix(name, "verify") ||
		strings.HasPrefix(name, "validate")
}

var verifyCallNames = map[string]bool{
	"verify": true, "coVerify": true,
	"confirmVerified": true, "every": true,
	"coEvery":                  true,
	"verifyNoInteractions":     true,
	"verifyNoMoreInteractions": true,
	"verifyBlocking":           true,
	"verifyOrder":              true,
}

func testingQualityIsVerifyCall(name string) bool {
	return verifyCallNames[name]
}

func testingQualityNameLooksAssertionEquivalent(name string) bool {
	if name == "" || name == "expectError" {
		return false
	}
	if testingQualityTurbineAssertionCalls[name] {
		return false
	}
	for _, token := range []string{"assertion", "assert", "check", "verification", "verify", "expectation", "expect", "snapshot", "await"} {
		if testingQualityNameContainsTokenFold(name, token) {
			return true
		}
	}
	return false
}

func testingQualityNameContainsTokenFold(name, token string) bool {
	lowerName := strings.ToLower(name)
	lowerToken := strings.ToLower(token)
	for start := strings.Index(lowerName, lowerToken); start >= 0; {
		end := start + len(lowerToken)
		if testingQualityIdentifierTokenBoundaryBefore(name, start) &&
			testingQualityIdentifierTokenBoundaryAfter(name, end) {
			return true
		}
		nextStart := end
		if nextStart >= len(lowerName) {
			break
		}
		if next := strings.Index(lowerName[nextStart:], lowerToken); next >= 0 {
			start = nextStart + next
		} else {
			break
		}
	}
	return false
}

func testingQualityIdentifierTokenBoundaryBefore(name string, idx int) bool {
	if idx <= 0 || idx >= len(name) {
		return idx == 0
	}
	prev := name[idx-1]
	cur := name[idx]
	return !testingQualityIsASCIIIdentifierByte(prev) ||
		(testingQualityIsASCIILower(prev) && testingQualityIsASCIIUpper(cur))
}

func testingQualityIdentifierTokenBoundaryAfter(name string, idx int) bool {
	if idx >= len(name) {
		return true
	}
	next := name[idx]
	return !testingQualityIsASCIIIdentifierByte(next) || testingQualityIsASCIIUpper(next)
}

func testingQualityIsASCIIIdentifierByte(b byte) bool {
	return (b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9')
}

func testingQualityIsASCIILower(b byte) bool {
	return b >= 'a' && b <= 'z'
}

func testingQualityIsASCIIUpper(b byte) bool {
	return b >= 'A' && b <= 'Z'
}

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

func testingQualityBodyHasAssertionOrVerification(file *scanner.File, body uint32) bool {
	return testingQualityBodyHasAssertionOrVerificationWithPatterns(file, body, nil)
}

func testingQualityBodyHasAssertionOrVerificationWithPatterns(file *scanner.File, body uint32, patterns []string) bool {
	return testingQualityBodyHasAssertionOrVerificationWithHelpersAndPatterns(file, body, testingQualityAssertionHelperNames(file), patterns)
}

func testingQualityBodyHasAssertionOrVerificationWithHelpers(file *scanner.File, body uint32, helpers map[string]bool) bool {
	return testingQualityBodyHasAssertionOrVerificationWithHelpersAndPatterns(file, body, helpers, nil)
}

func testingQualityBodyHasAssertionOrVerificationWithHelpersAndPatterns(file *scanner.File, body uint32, helpers map[string]bool, patterns []string) bool {
	if file == nil || body == 0 {
		return false
	}
	bodyText := file.FlatNodeText(body)
	if strings.Contains(bodyText, "AssertionError") {
		return true
	}
	if testingQualityTextHasVerifyCall(bodyText) {
		return true
	}
	found := false
	file.FlatWalkAllNodes(body, func(n uint32) {
		if found {
			return
		}
		switch file.FlatType(n) {
		case "call_expression":
			name := flatCallNameAny(file, n)
			found = testingQualityIsAssertionOrVerify(name) ||
				helpers[name] ||
				testingQualityConfiguredAssertionPatternMatches(name, patterns) ||
				testingQualityIsHarnessVerificationCall(file, n, name) ||
				testingQualityIsTurbineAssertionCall(file, name) ||
				testingQualityIsComposeUIAssertionCall(file, n, name) ||
				testingQualityIsEspressoAssertionCall(file, n, name) ||
				testingQualityIsSnapshotVerificationCall(file, name) ||
				testingQualityIsUIVisibilityAssertionCall(file, name) ||
				testingQualityIsExplicitThisOrSuperHelperCall(file, n)
		case "assignment":
			found = testingQualityAssignmentLHSHasVerify(file, n)
		case "infix_expression":
			name := testingQualityInfixOperatorName(file, n)
			found = testingQualityIsAssertionOrVerify(name) ||
				helpers[name] ||
				testingQualityConfiguredAssertionPatternMatches(name, patterns) ||
				testingQualityIsShouldStyleInfixAssertion(name)
		case "jump_expression":
			found = testingQualityThrowExpressionIsAssertion(file, n)
		}
	})
	return found
}

func testingQualityConfiguredAssertionPatternMatches(name string, patterns []string) bool {
	if name == "" || len(patterns) == 0 {
		return false
	}
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if pattern == name {
			return true
		}
		if strings.ContainsAny(pattern, "*?[") {
			matched, err := path.Match(pattern, name)
			if err == nil && matched {
				return true
			}
		}
	}
	return false
}

func testingQualityAssignmentLHSHasVerify(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "assignment" {
		return false
	}
	lhs, _, ok := strings.Cut(file.FlatNodeText(idx), "=")
	if !ok {
		return false
	}
	for name := range verifyCallNames {
		if strings.Contains(lhs, name+"(") {
			return true
		}
	}
	return false
}

func testingQualityTextHasVerifyCall(text string) bool {
	for name := range verifyCallNames {
		if strings.Contains(text, name+"(") {
			return true
		}
	}
	return false
}

func testingQualityIsExplicitThisOrSuperHelperCall(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "call_expression" {
		return false
	}
	nav, _ := flatCallExpressionParts(file, idx)
	if nav == 0 {
		return false
	}
	segments := flatNavigationChainIdentifiers(file, nav)
	if len(segments) >= 2 && (segments[0] == "this" || segments[0] == "super") {
		return true
	}
	text := strings.TrimSpace(file.FlatNodeText(idx))
	return strings.HasPrefix(text, "this.") || strings.HasPrefix(text, "super.")
}

func testingQualityIsHarnessVerificationCall(file *scanner.File, idx uint32, name string) bool {
	if file == nil || idx == 0 || name == "" {
		return false
	}
	switch name {
	case "compile":
		return testingQualityEnclosingClassHasSupertypeSuffix(file, idx, "CompilerTest")
	case "compileKotlinAndFail":
		return testingQualityEnclosingClassHasSupertype(file, idx, "BaseIncrementalCompilationTest")
	case "build":
		return fileImportsFQN(file, "com.autonomousapps.kit.GradleBuilder.build")
	case "waitFor":
		return testingQualityIsPollingCheckWaitFor(file, idx)
	case "expectError":
		return testingQualityIsComposeTestutilsExpectError(file, idx)
	case "test", "testWithLifecycle":
		return testingQualityIsTurbineTestCall(file, idx)
	case "runTest":
		return testingQualityIsPresenterRunTest(file, idx)
	default:
		return false
	}
}

func testingQualityIsTurbineAssertionCall(file *scanner.File, name string) bool {
	if file == nil {
		return false
	}
	if !testingQualityTurbineAssertionCalls[name] {
		return false
	}
	return sourceImportsOrMentions(file, "app.cash.turbine") ||
		strings.Contains(string(file.Content), "Turbine<") ||
		strings.Contains(string(file.Content), "Turbine()")
}

func testingQualityIsPresenterRunTest(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	nav, _ := flatCallExpressionParts(file, idx)
	if nav == 0 {
		return false
	}
	navText := file.FlatNodeText(nav)
	return strings.Contains(navText, "presenter") || strings.Contains(navText, "Presenter")
}

func testingQualityIsComposeUIAssertionCall(file *scanner.File, idx uint32, name string) bool {
	switch name {
	case "isDisplayed", "doesNotExist":
	default:
		return false
	}
	if !testingQualityFileHasComposeUITestEvidence(file) {
		return false
	}
	text := file.FlatNodeText(idx)
	return strings.Contains(text, "onNode") || strings.Contains(text, "onAllNodes")
}

func testingQualityIsSnapshotVerificationCall(file *scanner.File, name string) bool {
	if file == nil {
		return false
	}
	switch name {
	case "gif", "snapshot":
		return sourceImportsOrMentions(file, "SnapshotTestRule")
	case "testContent":
		return strings.Contains(file.Path, "/snapshot/") || sourceImportsOrMentions(file, "SnapshotTestRule")
	default:
		return false
	}
}

func testingQualityIsUIVisibilityAssertionCall(file *scanner.File, name string) bool {
	if file == nil {
		return false
	}
	switch name {
	case "isTextDisplayed", "waitUntilViewIsDisplayed", "waitUntilViewIsNotDisplayed",
		"isNotDisplayed", "waitUntilAtLeastOneExists", "waitUntilExactlyOneExists", "waitUntilDoesNotExist":
	default:
		if !strings.HasPrefix(name, "is") || !strings.HasSuffix(name, "Displayed") {
			return false
		}
	}
	if testingQualityFileHasComposeUITestEvidence(file) || testingQualityFileHasEspressoEvidence(file) {
		return true
	}
	return strings.Contains(string(file.Content), "uitesting")
}

func testingQualityFileHasComposeUITestEvidence(file *scanner.File) bool {
	if file == nil {
		return false
	}
	return sourceImportsOrMentions(file, "androidx.compose.ui.test") ||
		strings.Contains(string(file.Content), "composeTestRule")
}

func testingQualityIsShouldStyleInfixAssertion(name string) bool {
	return strings.HasPrefix(name, "should") || strings.HasPrefix(name, "shouldNot")
}

func testingQualityIsEspressoAssertionCall(file *scanner.File, idx uint32, name string) bool {
	switch name {
	case "matches", "waitForElementDisplayed", "isDisplayed":
	default:
		return false
	}
	if !testingQualityFileHasEspressoEvidence(file) {
		return false
	}
	if name == "waitForElementDisplayed" {
		return true
	}
	text := file.FlatNodeText(idx)
	return strings.Contains(text, "onView") ||
		strings.Contains(text, "withId") ||
		strings.Contains(text, "withText")
}

func testingQualityFileHasEspressoEvidence(file *scanner.File) bool {
	if file == nil {
		return false
	}
	content := string(file.Content)
	return sourceImportsOrMentions(file, "androidx.test.espresso") ||
		strings.Contains(content, "onView(") ||
		strings.Contains(content, "withId(") ||
		strings.Contains(content, "withText(")
}

var testingQualityTurbineAssertionCalls = map[string]bool{
	"awaitItem":                       true,
	"awaitComplete":                   true,
	"awaitError":                      true,
	"expectMostRecentItem":            true,
	"expectNoEvents":                  true,
	"ensureAllEventsConsumed":         true,
	"cancelAndConsumeRemainingEvents": true,
	"cancelAndIgnoreRemainingEvents":  true,
}

func testingQualityIsTurbineTestCall(file *scanner.File, idx uint32) bool {
	name := flatCallNameAny(file, idx)
	if name != "test" && name != "testWithLifecycle" {
		return false
	}
	fqn := "app.cash.turbine." + name
	if !fileImportsFQN(file, fqn) &&
		!sourceImportsOrMentions(file, fqn) {
		return false
	}
	lambda := flatCallTrailingLambda(file, idx)
	if lambda == 0 {
		return false
	}
	found := false
	file.FlatWalkAllNodes(lambda, func(n uint32) {
		if found || file.FlatType(n) != "call_expression" {
			return
		}
		found = testingQualityTurbineAssertionCalls[flatCallNameAny(file, n)]
	})
	return found
}

func testingQualityIsComposeTestutilsExpectError(file *scanner.File, idx uint32) bool {
	if !fileImportsFQN(file, "androidx.compose.testutils.expectError") &&
		!sourceImportsOrMentions(file, "androidx.compose.testutils.expectError") {
		return false
	}
	name := flatCallNameAny(file, idx)
	return name == "expectError"
}

func testingQualityIsPollingCheckWaitFor(file *scanner.File, idx uint32) bool {
	if !fileImportsFQN(file, "androidx.testutils.PollingCheck") &&
		!fileImportsFQN(file, "androidx.testutils.PollingCheck.waitFor") {
		return false
	}
	nav, _ := flatCallExpressionParts(file, idx)
	if nav == 0 {
		return fileImportsFQN(file, "androidx.testutils.PollingCheck.waitFor")
	}
	segments := flatNavigationChainIdentifiers(file, nav)
	return len(segments) == 2 && segments[0] == "PollingCheck" && segments[1] == "waitFor"
}

func testingQualityEnclosingClassHasSupertype(file *scanner.File, idx uint32, supertype string) bool {
	classDecl, ok := flatEnclosingAncestor(file, idx, "class_declaration")
	return ok && classHasSupertypeNamed(file, classDecl, supertype)
}

func testingQualityEnclosingClassHasSupertypeSuffix(file *scanner.File, idx uint32, suffix string) bool {
	classDecl, ok := flatEnclosingAncestor(file, idx, "class_declaration")
	if !ok || suffix == "" {
		return false
	}
	for child := file.FlatFirstChild(classDecl); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "delegation_specifier" {
			continue
		}
		userType, _ := file.FlatFindChild(child, "user_type")
		if userType == 0 {
			if ctor, ok := file.FlatFindChild(child, "constructor_invocation"); ok {
				userType, _ = file.FlatFindChild(ctor, "user_type")
			}
		}
		if userType == 0 {
			continue
		}
		if ident := flatLastChildOfType(file, userType, "type_identifier"); ident != 0 {
			if strings.HasSuffix(file.FlatNodeText(ident), suffix) {
				return true
			}
		}
	}
	return false
}

func testingQualityThrowExpressionIsAssertion(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "jump_expression" {
		return false
	}
	if !strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(idx)), "throw") {
		return false
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		text := file.FlatNodeText(child)
		return strings.Contains(text, "AssertionError") || strings.Contains(text, "AssertionFailedError")
	}
	return false
}

func testingQualityTestNameDocumentsNoException(file *scanner.File, idx uint32) bool {
	name := strings.ToLower(testingQualityFunctionName(file, idx))
	if name != "" && testingQualityTextDocumentsNoException(name) {
		return true
	}
	return testingQualityTextDocumentsNoException(testingQualityFunctionHeaderText(file, idx))
}

func testingQualityTextDocumentsNoException(name string) bool {
	name = strings.Trim(name, "`")
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.Join(strings.Fields(name), " ")
	for _, marker := range []string{
		"no crash",
		"not crash",
		"without crash",
		"without crashing",
		"does not crash",
		"doesn't crash",
		"doesnt crash",
		"no exception",
		"not throw",
		"without throw",
		"without throwing",
		"does not throw",
		"doesn't throw",
		"doesnt throw",
		"completes without",
		"robust",
		"handles any",
		"works for any",
	} {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}

func testingQualityFunctionHeaderText(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	start := file.FlatStartByte(idx)
	end := file.FlatEndByte(idx)
	if body, ok := file.FlatFindChild(idx, "function_body"); ok {
		end = file.FlatStartByte(body)
	}
	if end <= start || int(end) > len(file.Content) {
		return ""
	}
	return strings.ToLower(string(file.Content[start:end]))
}

func testingQualityAssertionHelperNames(file *scanner.File) map[string]bool {
	if file == nil {
		return map[string]bool{}
	}
	return filefacts.FileFact(fileFactsCache(), file, slotTestQualityAssertionHelpers, func() map[string]bool {
		helpers := make(map[string]bool)
		type helperCandidate struct {
			name string
			body uint32
		}
		var candidates []helperCandidate
		file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
			name := testingQualityFunctionName(file, fn)
			if name == "" || testingQualityIsTestFunction(file, fn) {
				return
			}
			body, _ := file.FlatFindChild(fn, "function_body")
			if body == 0 {
				return
			}
			candidates = append(candidates, helperCandidate{name: name, body: body})
		})
		changed := true
		for changed {
			changed = false
			for _, candidate := range candidates {
				if helpers[candidate.name] {
					continue
				}
				if testingQualityBodyHasAssertionOrVerificationWithHelpers(file, candidate.body, helpers) {
					helpers[candidate.name] = true
					changed = true
				}
			}
		}
		return helpers
	})
}

var testingQualityRunBlockingBoundaryCalls = map[string]bool{
	"runOnIdle":     true,
	"runOnUiThread": true,
	"runOnActivity": true,
}

func testingQualityInsideAssertionOrCallbackBoundary(file *scanner.File, idx uint32, fn uint32) bool {
	for current, ok := file.FlatParent(idx); ok && current != fn; current, ok = file.FlatParent(current) {
		if file.FlatType(current) != "call_expression" {
			continue
		}
		name := flatCallNameAny(file, current)
		if testingQualityIsAssertionOrVerify(name) || testingQualityRunBlockingBoundaryCalls[name] {
			return true
		}
	}
	return false
}

func testingQualityRunBlockingTestsDispatcherThreadIdentity(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "call_expression" {
		return false
	}
	bodyText := file.FlatNodeText(idx)
	if strings.Contains(bodyText, "isDispatchNeeded") {
		return true
	}
	hasRealDispatcher := strings.Contains(bodyText, "Dispatchers.Default") ||
		strings.Contains(bodyText, "Dispatchers.IO")
	hasConcurrencyPrimitive := strings.Contains(bodyText, "CountDownLatch") ||
		strings.Contains(bodyText, "Semaphore")
	if hasConcurrencyPrimitive && (hasRealDispatcher || strings.Contains(bodyText, "CoroutineScope(")) {
		return true
	}
	if hasRealDispatcher && strings.Contains(bodyText, "CoroutineScope(") {
		return true
	}
	var hasThreadCurrentThread bool
	var hasThreadIdentityAssertion bool
	file.FlatWalkAllNodes(idx, func(n uint32) {
		if hasThreadCurrentThread && hasThreadIdentityAssertion {
			return
		}
		if file.FlatType(n) != "call_expression" {
			return
		}
		switch flatCallNameAny(file, n) {
		case "currentThread":
			if testingQualityCallReceiverContains(file, n, "Thread") {
				hasThreadCurrentThread = true
			}
		case "isSameInstanceAs", "isNotSameInstanceAs":
			if testingQualityLooksLikeThreadIdentityAssertion(file, n) {
				hasThreadIdentityAssertion = true
			}
		}
	})
	return hasThreadCurrentThread || hasThreadIdentityAssertion
}

func testingQualityRunBlockingInRxJavaBridgeTest(file *scanner.File, idx uint32, fn uint32) bool {
	if file == nil || idx == 0 || fn == 0 || !testingQualityFileImportsRxJavaTestObserver(file) {
		return false
	}
	body, _ := file.FlatFindChild(fn, "function_body")
	if body == 0 {
		return false
	}
	bodyText := strings.TrimSpace(file.FlatNodeText(body))
	if strings.HasPrefix(bodyText, "=") {
		return false
	}
	if !strings.Contains(bodyText, "TestObserver") && !strings.Contains(bodyText, "TestSubscriber") {
		return false
	}
	if !strings.Contains(bodyText, "subscribe(") && !strings.Contains(bodyText, ".subscribe") {
		return false
	}
	return true
}

func testingQualityRunBlockingResultIsAsserted(file *scanner.File, idx uint32, fn uint32) bool {
	if file == nil || idx == 0 || fn == 0 {
		return false
	}
	assignedName := initializerAssignedName(file, idx)
	if assignedName == "" {
		return false
	}
	body, _ := file.FlatFindChild(fn, "function_body")
	if body == 0 {
		return false
	}
	found := false
	file.FlatWalkAllNodes(body, func(n uint32) {
		if found || file.FlatType(n) != "call_expression" {
			return
		}
		name := flatCallNameAny(file, n)
		if !testingQualityIsAssertionOrVerify(name) &&
			!testingQualityIsHarnessVerificationCall(file, n, name) &&
			!testingQualityIsComposeUIAssertionCall(file, n, name) &&
			!testingQualityIsEspressoAssertionCall(file, n, name) {
			return
		}
		found = subtreeHasReferenceName(file, n, assignedName)
	})
	return found
}

func testingQualityRunBlockingContainsTurbineAssertion(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	found := false
	file.FlatWalkAllNodes(idx, func(n uint32) {
		if found || n == idx || file.FlatType(n) != "call_expression" {
			return
		}
		name := flatCallNameAny(file, n)
		found = testingQualityIsTurbineTestCall(file, n) ||
			testingQualityTurbineAssertionCalls[name]
	})
	return found
}

func testingQualityFileImportsRxJavaTestObserver(file *scanner.File) bool {
	return fileImportsFQN(file, "io.reactivex.observers.TestObserver") ||
		fileImportsFQN(file, "io.reactivex.rxjava3.observers.TestObserver") ||
		fileImportsFQN(file, "io.reactivex.subscribers.TestSubscriber") ||
		fileImportsFQN(file, "io.reactivex.rxjava3.subscribers.TestSubscriber")
}

func testingQualityLooksLikeThreadIdentityAssertion(file *scanner.File, call uint32) bool {
	if file == nil || call == 0 {
		return false
	}
	name := flatCallNameAny(file, call)
	if name != "isSameInstanceAs" && name != "isNotSameInstanceAs" {
		return false
	}
	return strings.Contains(strings.ToLower(file.FlatNodeText(call)), "thread")
}

func testingQualityRunBlockingHasIntentionalComment(file *scanner.File, idx uint32, fn uint32) bool {
	if file == nil || idx == 0 || fn == 0 {
		return false
	}
	callLine := file.FlatRow(idx) + 1
	fnStart := file.FlatRow(fn) + 1
	found := false
	file.FlatWalkAllNodes(fn, func(n uint32) {
		if found || !isFlatCommentNode(file, n) {
			return
		}
		commentLine := file.FlatRow(n) + 1
		if commentLine < fnStart || commentLine > callLine {
			return
		}
		found = testingQualityRunBlockingCommentDocumentsIntent(file.FlatNodeText(n))
	})
	return found
}

func testingQualityRunBlockingCommentDocumentsIntent(text string) bool {
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "runblocking") {
		return false
	}
	intent := strings.Contains(lower, "intentional") ||
		strings.Contains(lower, "intentionally") ||
		strings.Contains(lower, "deliberate") ||
		strings.Contains(lower, "deliberately")
	if !intent {
		return false
	}
	return strings.Contains(lower, "dispatcher") ||
		strings.Contains(lower, "thread") ||
		strings.Contains(lower, "confinement") ||
		strings.Contains(lower, "reentrant") ||
		strings.Contains(lower, "locking")
}

func testingQualityCallReceiverContains(file *scanner.File, call uint32, name string) bool {
	if file == nil || call == 0 || name == "" {
		return false
	}
	nav, _ := flatCallExpressionParts(file, call)
	if nav == 0 {
		return false
	}
	found := false
	file.FlatWalkAllNodes(nav, func(n uint32) {
		if found {
			return
		}
		if file.FlatType(n) == "simple_identifier" && file.FlatNodeTextEquals(n, name) {
			found = true
		}
	})
	return found
}

func testingQualityIsBenchmarkOrGoldenFile(file *scanner.File) bool {
	if file == nil {
		return false
	}
	return fileImportsFQN(file, "androidx.benchmark.macro.junit4.MacrobenchmarkRule") ||
		fileImportsFQN(file, "androidx.benchmark.macro.junit4.BaselineProfileRule") ||
		fileImportsFQN(file, "androidx.benchmark.junit4.BenchmarkRule") ||
		fileImportsFQN(file, "androidx.compose.testutils.benchmark.ComposeBenchmarkRule") ||
		fileImportsFQN(file, "com.github.takahirom.roborazzi.RoborazziRule")
}

func testingQualityIsAndroidInstrumentedTestFile(file *scanner.File) bool {
	if file == nil {
		return false
	}
	path := strings.ToLower(file.Path)
	return strings.Contains(path, "/src/androidtest/") ||
		strings.Contains(path, "/src/androidinstrumentedtest/") ||
		strings.Contains(path, "/src/androidcommontest/") ||
		strings.Contains(path, "/src/commonjvmandroidtest/")
}

func testingQualityDispatcherReferenceAllowedInTest(file *scanner.File, idx uint32, dispatcher string) bool {
	if file == nil || idx == 0 {
		return false
	}
	if dispatcher == "Main" {
		for cur := idx; cur != 0; {
			text := file.FlatNodeText(cur)
			if strings.Contains(text, "Dispatchers.Main.immediate") {
				return true
			}
			parent, ok := file.FlatParent(cur)
			if !ok {
				break
			}
			if file.FlatType(parent) != "navigation_expression" {
				break
			}
			cur = parent
		}
	}
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "call_expression":
			if strings.Contains(flatCallNameAny(file, cur), "CoroutineContext") {
				return true
			}
		case "function_body", "statements", "function_declaration", "source_file":
			return false
		}
	}
	return false
}

func testingQualityInsideRunTest(file *scanner.File, idx uint32) bool {
	for current, ok := file.FlatParent(idx); ok; current, ok = file.FlatParent(current) {
		if file.FlatType(current) == "call_expression" {
			name := flatCallNameAny(file, current)
			if name == "runTest" && testingQualityIsCoroutineRunTestCall(file, current) {
				return true
			}
		}
	}
	return false
}

func testingQualityInsideMockKAnswerLambda(file *scanner.File, idx uint32) bool {
	for current, ok := file.FlatParent(idx); ok; current, ok = file.FlatParent(current) {
		switch file.FlatType(current) {
		case "function_declaration":
			return false
		case "lambda_literal":
			if testingQualityLambdaBelongsToMockKAnswerInfix(file, current) {
				return true
			}
		}
	}
	return false
}

func testingQualityLambdaBelongsToMockKAnswerInfix(file *scanner.File, lambda uint32) bool {
	for current, ok := file.FlatParent(lambda); ok; current, ok = file.FlatParent(current) {
		switch file.FlatType(current) {
		case "function_declaration", "statements":
			return false
		case "infix_expression":
			return testingQualityIsMockKAnswerInfix(file, current)
		}
	}
	return false
}

func testingQualityIsMockKAnswerInfix(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "infix_expression" {
		return false
	}
	hasAnswerOperator := false
	hasStubbingCall := false
	file.FlatWalkAllNodes(idx, func(candidate uint32) {
		if file.FlatType(candidate) != "simple_identifier" {
			return
		}
		switch file.FlatNodeText(candidate) {
		case "answers", "coAnswers":
			hasAnswerOperator = true
		case "every", "coEvery":
			hasStubbingCall = true
		}
	})
	return hasAnswerOperator && hasStubbingCall
}

func testingQualityIsDirectCoroutineDelayCall(file *scanner.File, idx uint32) bool {
	navExpr, _ := flatCallExpressionParts(file, idx)
	if navExpr == 0 {
		return fileImportsFQN(file, "kotlinx.coroutines.delay")
	}
	segments := flatNavigationChainIdentifiers(file, navExpr)
	return strings.Join(segments, ".") == "kotlinx.coroutines.delay"
}

func testingQualityIsCoroutineRunTestCall(file *scanner.File, idx uint32) bool {
	navExpr, _ := flatCallExpressionParts(file, idx)
	if navExpr == 0 {
		return fileImportsFQN(file, "kotlinx.coroutines.test.runTest")
	}
	segments := flatNavigationChainIdentifiers(file, navExpr)
	return strings.Join(segments, ".") == "kotlinx.coroutines.test.runTest"
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
