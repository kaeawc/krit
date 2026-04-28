package rules

import (
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

var testingQualityObjectMockCreationCalls = map[string]bool{
	"mockkObject":      true,
	"mockkStatic":      true,
	"mockkConstructor": true,
}

var testingQualityMockHelperCache sync.Map
var testingQualityMockNamesCache sync.Map

type testingQualityMockNamesCacheKey struct {
	file *scanner.File
	fn   uint32
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
	"shouldBe": true, "shouldNotBe": true,
	"shouldThrow": true, "shouldNotThrow": true,
	"check":    true,
	"snapshot": true, "captureRoboImage": true, "captureToImage": true,
	"measureRepeated": true,
	"waitUntil":       true, "testExecute": true, "testEnqueue": true,
	"complete": true,
}

func testingQualityIsAssertionCall(name string) bool {
	if assertionCallNames[name] {
		return true
	}
	return strings.HasPrefix(name, "assert")
}

func testingQualityIsAssertionOrVerify(name string) bool {
	return testingQualityIsAssertionCall(name) || testingQualityIsVerifyCall(name)
}

var verifyCallNames = map[string]bool{
	"verify": true, "coVerify": true,
	"confirmVerified": true, "every": true,
	"coEvery": true,
}

func testingQualityIsVerifyCall(name string) bool {
	return verifyCallNames[name]
}

func testingQualityIsTestFile(file *scanner.File) bool {
	if file == nil {
		return false
	}
	return strings.Contains(file.Path, "/test") ||
		strings.Contains(file.Path, "Test.kt") ||
		strings.Contains(file.Path, "/testing-quality/")
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
	if file == nil || body == 0 {
		return false
	}
	found := false
	file.FlatWalkAllNodes(body, func(n uint32) {
		if found {
			return
		}
		switch file.FlatType(n) {
		case "call_expression":
			found = testingQualityIsAssertionOrVerify(flatCallNameAny(file, n))
		case "infix_expression":
			found = testingQualityIsAssertionOrVerify(testingQualityInfixOperatorName(file, n))
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
		fileImportsFQN(file, "com.github.takahirom.roborazzi.RoborazziRule")
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
