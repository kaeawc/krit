package rules

import (
	"bytes"
	"strings"
	"sync"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
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
func (r *AssertEqualsArgumentOrderRule) Confidence() float64 { return 0.75 }

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

// Confidence reports a tier-2 (medium) base confidence. Testing-quality rule. Detection matches on assertion framework call
// shapes (JUnit, Truth, Kotest, MockK) by name — cross-library identifier
// collisions can produce false positives. Classified per roadmap/17.
func (r *AssertTrueOnComparisonRule) Confidence() float64 { return 0.75 }

// MixedAssertionLibrariesRule detects files that import both JUnit Assert and
// Truth APIs in the import header.
type MixedAssertionLibrariesRule struct {
	LineBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Testing-quality rule. Detection matches on assertion framework call
// shapes (JUnit, Truth, Kotest, MockK) by name — cross-library identifier
// collisions can produce false positives. Classified per roadmap/17.
func (r *MixedAssertionLibrariesRule) Confidence() float64 { return 0.75 }

func (r *MixedAssertionLibrariesRule) check(ctx *v2.Context) {
	file := ctx.File
	var hasJUnitAssertImport bool
	var hasTruthImport bool

	for i, line := range file.Lines {
		trimmed := strings.TrimSpace(line)

		switch {
		case trimmed == "", scanner.IsCommentLine(line):
			continue
		case strings.HasPrefix(trimmed, "@file:"):
			continue
		case strings.HasPrefix(trimmed, "package "):
			continue
		case strings.HasPrefix(trimmed, "import "):
			if strings.HasPrefix(trimmed, "import org.junit.Assert.") {
				hasJUnitAssertImport = true
				if hasTruthImport {
					ctx.Emit(r.Finding(file, i+1, 1,
						"Avoid mixing JUnit Assert and Truth imports in the same file; pick one assertion library."))
					return
				}
			}
			if strings.HasPrefix(trimmed, "import com.google.common.truth.Truth.") {
				hasTruthImport = true
				if hasJUnitAssertImport {
					ctx.Emit(r.Finding(file, i+1, 1,
						"Avoid mixing JUnit Assert and Truth imports in the same file; pick one assertion library."))
					return
				}
			}
			continue
		default:
			return
		}
	}
}

// ---------------------------------------------------------------------------
// AssertNullableWithNotNullAssertionRule
// ---------------------------------------------------------------------------

type AssertNullableWithNotNullAssertionRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *AssertNullableWithNotNullAssertionRule) Confidence() float64 { return 0.75 }

// ---------------------------------------------------------------------------
// MockWithoutVerifyRule
// ---------------------------------------------------------------------------

type MockWithoutVerifyRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *MockWithoutVerifyRule) Confidence() float64 { return 0.75 }

// ---------------------------------------------------------------------------
// RunTestWithDelayRule
// ---------------------------------------------------------------------------

type RunTestWithDelayRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *RunTestWithDelayRule) Confidence() float64 { return 0.75 }

// ---------------------------------------------------------------------------
// RunTestWithThreadSleepRule
// ---------------------------------------------------------------------------

type RunTestWithThreadSleepRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *RunTestWithThreadSleepRule) Confidence() float64 { return 0.75 }

// ---------------------------------------------------------------------------
// RunBlockingInTestRule
// ---------------------------------------------------------------------------

type RunBlockingInTestRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *RunBlockingInTestRule) Confidence() float64 { return 0.75 }

// ---------------------------------------------------------------------------
// TestDispatcherNotInjectedRule
// ---------------------------------------------------------------------------

type TestDispatcherNotInjectedRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *TestDispatcherNotInjectedRule) Confidence() float64 { return 0.75 }

// ---------------------------------------------------------------------------
// TestWithoutAssertionRule
// ---------------------------------------------------------------------------

type TestWithoutAssertionRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *TestWithoutAssertionRule) Confidence() float64 { return 0.75 }

// ---------------------------------------------------------------------------
// TestWithOnlyTodoRule
// ---------------------------------------------------------------------------

type TestWithOnlyTodoRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *TestWithOnlyTodoRule) Confidence() float64 { return 0.75 }

// ---------------------------------------------------------------------------
// TestFunctionReturnValueRule
// ---------------------------------------------------------------------------

type TestFunctionReturnValueRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *TestFunctionReturnValueRule) Confidence() float64 { return 0.75 }

// ---------------------------------------------------------------------------
// TestNameContainsUnderscoreRule
// ---------------------------------------------------------------------------

type TestNameContainsUnderscoreRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *TestNameContainsUnderscoreRule) Confidence() float64 { return 0.6 }

// ---------------------------------------------------------------------------
// SharedMutableStateInObjectRule
// ---------------------------------------------------------------------------

type SharedMutableStateInObjectRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *SharedMutableStateInObjectRule) Confidence() float64 { return 0.75 }

// ---------------------------------------------------------------------------
// TestInheritanceDepthRule
// ---------------------------------------------------------------------------

type TestInheritanceDepthRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *TestInheritanceDepthRule) Confidence() float64 { return 0.6 }

// ---------------------------------------------------------------------------
// RelaxedMockUsedForValueClassRule
// ---------------------------------------------------------------------------

type RelaxedMockUsedForValueClassRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *RelaxedMockUsedForValueClassRule) Confidence() float64 { return 0.75 }

var primitiveTypes = map[string]bool{
	"Int": true, "Long": true, "Float": true, "Double": true,
	"Boolean": true, "String": true, "Byte": true, "Short": true, "Char": true,
}

// ---------------------------------------------------------------------------
// SpyOnDataClassRule
// ---------------------------------------------------------------------------

type SpyOnDataClassRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *SpyOnDataClassRule) Confidence() float64 { return 0.6 }

// ---------------------------------------------------------------------------
// VerifyWithoutMockRule
// ---------------------------------------------------------------------------

type VerifyWithoutMockRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *VerifyWithoutMockRule) Confidence() float64 { return 0.6 }

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

var testingQualityMockHelperCache sync.Map
var testingQualityMockNamesCache sync.Map
var testingQualityAssertionHelperCache sync.Map

type testingQualityMockNamesCacheKey struct {
	file *scanner.File
	fn   uint32
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

func testingQualityDirectVerifyReceivers(file *scanner.File, lambda uint32) []string {
	if file == nil || lambda == 0 {
		return nil
	}
	var receivers []string
	addReceiver := func(stmt uint32) {
		if stmt == 0 || file.FlatType(stmt) != "call_expression" {
			return
		}
		if receiver := flatReceiverNameFromCall(file, stmt); receiver != "" {
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
	return testingQualityCallCreatesMock(file, idx, helperFuncs)
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
		if file.FlatNamedChildCount(expr) > 0 {
			first := file.FlatNamedChild(expr, 0)
			switch file.FlatType(first) {
			case "simple_identifier":
				return file.FlatNodeText(first)
			case "navigation_expression":
				segments := flatNavigationChainIdentifiers(file, first)
				if len(segments) > 0 {
					return segments[0]
				}
			}
		}
	}
	return ""
}

func testingQualityMockHelperFunctions(file *scanner.File) map[string]bool {
	helpers := make(map[string]bool)
	if file == nil {
		return helpers
	}
	if cached, ok := testingQualityMockHelperCache.Load(file); ok {
		return cached.(map[string]bool)
	}
	file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
		name := testingQualityFunctionName(file, fn)
		if name == "" {
			return
		}
		mockVars := make(map[string]bool)
		if body, ok := file.FlatFindChild(fn, "function_body"); ok {
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
	testingQualityMockHelperCache.Store(file, helpers)
	return helpers
}

func testingQualityCollectMockNames(file *scanner.File, fn uint32, helperFuncs map[string]bool) map[string]bool {
	mockVars := make(map[string]bool)
	if file == nil {
		return mockVars
	}
	key := testingQualityMockNamesCacheKey{file: file, fn: fn}
	if cached, ok := testingQualityMockNamesCache.Load(key); ok {
		return cached.(map[string]bool)
	}
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
	testingQualityMockNamesCache.Store(key, mockVars)
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

func testingQualityIsAssertionOrVerify(name string) bool {
	return testingQualityIsAssertionCall(name) ||
		testingQualityIsVerifyCall(name) ||
		strings.HasPrefix(name, "verify") ||
		strings.HasPrefix(name, "validate")
}

var verifyCallNames = map[string]bool{
	"verify": true, "coVerify": true,
	"confirmVerified": true, "every": true,
	"coEvery": true,
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
	return testingQualityBodyHasAssertionOrVerificationWithHelpers(file, body, testingQualityAssertionHelperNames(file))
}

func testingQualityBodyHasAssertionOrVerificationWithHelpers(file *scanner.File, body uint32, helpers map[string]bool) bool {
	if file == nil || body == 0 {
		return false
	}
	if strings.Contains(file.FlatNodeText(body), "AssertionError") {
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
				testingQualityIsHarnessVerificationCall(file, n, name)
		case "infix_expression":
			found = testingQualityIsAssertionOrVerify(testingQualityInfixOperatorName(file, n))
		case "jump_expression":
			found = testingQualityThrowExpressionIsAssertion(file, n)
		}
	})
	return found
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
	default:
		return false
	}
}

var testingQualityTurbineAssertionCalls = map[string]bool{
	"awaitItem":                       true,
	"awaitComplete":                   true,
	"awaitError":                      true,
	"expectMostRecentItem":            true,
	"expectNoEvents":                  true,
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
	if name == "" {
		return false
	}
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
	} {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}

func testingQualityAssertionHelperNames(file *scanner.File) map[string]bool {
	helpers := make(map[string]bool)
	if file == nil {
		return helpers
	}
	if cached, ok := testingQualityAssertionHelperCache.Load(file); ok {
		return cached.(map[string]bool)
	}
	file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
		name := testingQualityFunctionName(file, fn)
		if name == "" || testingQualityIsTestFunction(file, fn) {
			return
		}
		body, _ := file.FlatFindChild(fn, "function_body")
		if body == 0 {
			return
		}
		if testingQualityBodyHasAssertionOrVerificationWithHelpers(file, body, nil) {
			helpers[name] = true
		}
	})
	testingQualityAssertionHelperCache.Store(file, helpers)
	return helpers
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
		if file.FlatType(child) == "simple_identifier" {
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

func testingQualityTypeArgument(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	result := ""
	file.FlatWalkAllNodes(idx, func(n uint32) {
		if result != "" {
			return
		}
		if file.FlatType(n) == "type_arguments" {
			for gc := file.FlatFirstChild(n); gc != 0; gc = file.FlatNextSib(gc) {
				if file.FlatType(gc) == "type_projection" || file.FlatType(gc) == "user_type" || file.FlatType(gc) == "type_identifier" {
					result = strings.TrimSpace(file.FlatNodeText(gc))
					return
				}
			}
		}
	})
	return result
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
