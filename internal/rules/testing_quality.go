package rules

import (
	"fmt"
	"strings"

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

func (r *AssertEqualsArgumentOrderRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *AssertEqualsArgumentOrderRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if flatCallExpressionName(file, idx) != "assertEquals" {
		return nil
	}

	_, args := flatCallExpressionParts(file, idx)
	if args == 0 {
		return nil
	}

	valueArgs := testingQualityValueArgumentsFlat(file, args)
	if len(valueArgs) != 2 {
		return nil
	}

	firstArg := strings.TrimSpace(file.FlatNodeText(valueArgs[0]))
	secondArg := strings.TrimSpace(file.FlatNodeText(valueArgs[1]))
	if firstArg != "actual" || secondArg != "expected" {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"`assertEquals` arguments appear reversed; use (expected, actual).",
	)}
}

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

func (r *AssertTrueOnComparisonRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *AssertTrueOnComparisonRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if flatCallExpressionName(file, idx) != "assertTrue" {
		return nil
	}

	_, args := flatCallExpressionParts(file, idx)
	if args == 0 {
		return nil
	}

	valueArgs := testingQualityValueArgumentsFlat(file, args)
	if len(valueArgs) != 1 {
		return nil
	}

	condition := flatValueArgumentExpression(file, valueArgs[0])
	if condition == 0 || file.FlatType(condition) != "equality_expression" || file.FlatChildCount(condition) < 3 {
		return nil
	}

	op := file.FlatChild(condition, 1)
	if op == 0 || !file.FlatNodeTextEquals(op, "==") {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"Use `assertEquals` instead of `assertTrue` for equality comparisons.",
	)}
}

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

func (r *MixedAssertionLibrariesRule) CheckLines(file *scanner.File) []scanner.Finding {
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
					return []scanner.Finding{r.Finding(file, i+1, 1,
						"Avoid mixing JUnit Assert and Truth imports in the same file; pick one assertion library.")}
				}
			}
			if strings.HasPrefix(trimmed, "import com.google.common.truth.Truth.") {
				hasTruthImport = true
				if hasJUnitAssertImport {
					return []scanner.Finding{r.Finding(file, i+1, 1,
						"Avoid mixing JUnit Assert and Truth imports in the same file; pick one assertion library.")}
				}
			}
			continue
		default:
			return nil
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// AssertNullableWithNotNullAssertionRule
// ---------------------------------------------------------------------------

type AssertNullableWithNotNullAssertionRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *AssertNullableWithNotNullAssertionRule) Confidence() float64 { return 0.75 }
func (r *AssertNullableWithNotNullAssertionRule) NodeTypes() []string {
	return []string{"call_expression"}
}

func (r *AssertNullableWithNotNullAssertionRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := flatCallExpressionName(file, idx)
	if !testingQualityIsAssertionCall(name) {
		return nil
	}

	_, args := flatCallExpressionParts(file, idx)
	if args == 0 {
		return nil
	}

	found := false
	file.FlatWalkAllNodes(args, func(n uint32) {
		if found {
			return
		}
		if file.FlatType(n) == "postfix_expression" {
			text := file.FlatNodeText(n)
			if strings.HasSuffix(text, "!!") {
				found = true
			}
		}
	})
	if !found {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"Avoid `!!` in assertions; use `assertNotNull` first.",
	)}
}

// ---------------------------------------------------------------------------
// MockWithoutVerifyRule
// ---------------------------------------------------------------------------

type MockWithoutVerifyRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *MockWithoutVerifyRule) Confidence() float64 { return 0.75 }
func (r *MockWithoutVerifyRule) NodeTypes() []string { return []string{"function_declaration"} }

func (r *MockWithoutVerifyRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !testingQualityIsTestFunction(file, idx) {
		return nil
	}

	body := file.FlatFindChild(idx, "function_body")
	if body == 0 {
		return nil
	}

	var mockNames []string
	var mockRows []int
	var mockCols []int

	file.FlatWalkAllNodes(body, func(n uint32) {
		if file.FlatType(n) != "property_declaration" {
			return
		}
		varDecl := file.FlatFindChild(n, "variable_declaration")
		if varDecl == 0 {
			return
		}
		ident := file.FlatFindChild(varDecl, "simple_identifier")
		if ident == 0 {
			return
		}

		for rhs := file.FlatFirstChild(n); rhs != 0; rhs = file.FlatNextSib(rhs) {
			if file.FlatType(rhs) != "call_expression" {
				continue
			}
			callName := flatCallNameAny(file, rhs)
			if callName == "mockk" || callName == "mock" || callName == "spyk" || callName == "spy" {
				mockNames = append(mockNames, file.FlatNodeText(ident))
				mockRows = append(mockRows, file.FlatRow(n)+1)
				mockCols = append(mockCols, file.FlatCol(n)+1)
			}
		}
	})

	if len(mockNames) == 0 {
		return nil
	}

	used := make(map[string]bool)
	file.FlatWalkAllNodes(body, func(n uint32) {
		if file.FlatType(n) != "call_expression" {
			return
		}
		callName := flatCallNameAny(file, n)
		switch callName {
		case "verify", "coVerify", "every", "coEvery", "confirmVerified":
		default:
			return
		}

		nodeText := file.FlatNodeText(n)
		for _, name := range mockNames {
			if strings.Contains(nodeText, name) {
				used[name] = true
			}
		}
	})

	var findings []scanner.Finding
	for i, name := range mockNames {
		if !used[name] {
			findings = append(findings, r.Finding(
				file,
				mockRows[i],
				mockCols[i],
				fmt.Sprintf("Mock `%s` is created but never verified or stubbed.", name),
			))
		}
	}
	return findings
}

// ---------------------------------------------------------------------------
// RunTestWithDelayRule
// ---------------------------------------------------------------------------

type RunTestWithDelayRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *RunTestWithDelayRule) Confidence() float64 { return 0.75 }
func (r *RunTestWithDelayRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *RunTestWithDelayRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if flatCallExpressionName(file, idx) != "delay" {
		return nil
	}

	_, args := flatCallExpressionParts(file, idx)
	if args != 0 {
		valueArgs := testingQualityValueArgumentsFlat(file, args)
		if len(valueArgs) == 1 {
			argText := strings.TrimSpace(file.FlatNodeText(valueArgs[0]))
			if argText == "0" || argText == "0L" {
				return nil
			}
		}
	}

	if !testingQualityInsideRunTest(file, idx) {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"Use `advanceTimeBy` instead of `delay` inside `runTest`.",
	)}
}

// ---------------------------------------------------------------------------
// RunTestWithThreadSleepRule
// ---------------------------------------------------------------------------

type RunTestWithThreadSleepRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *RunTestWithThreadSleepRule) Confidence() float64 { return 0.75 }
func (r *RunTestWithThreadSleepRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *RunTestWithThreadSleepRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := flatCallExpressionName(file, idx)
	if name != "sleep" {
		return nil
	}

	navExpr, _ := flatCallExpressionParts(file, idx)
	if navExpr == 0 {
		return nil
	}
	receiver := file.FlatNamedChild(navExpr, 0)
	if receiver == 0 || !file.FlatNodeTextEquals(receiver, "Thread") {
		return nil
	}

	if !testingQualityInsideRunTest(file, idx) {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"Use `advanceTimeBy` instead of `Thread.sleep` inside `runTest`.",
	)}
}

// ---------------------------------------------------------------------------
// RunBlockingInTestRule
// ---------------------------------------------------------------------------

type RunBlockingInTestRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *RunBlockingInTestRule) Confidence() float64 { return 0.75 }
func (r *RunBlockingInTestRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *RunBlockingInTestRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if flatCallExpressionName(file, idx) != "runBlocking" {
		return nil
	}

	fn, ok := flatEnclosingFunction(file, idx)
	if !ok || !testingQualityIsTestFunction(file, fn) {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"Use `runTest` instead of `runBlocking` in test functions.",
	)}
}

// ---------------------------------------------------------------------------
// TestDispatcherNotInjectedRule
// ---------------------------------------------------------------------------

type TestDispatcherNotInjectedRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *TestDispatcherNotInjectedRule) Confidence() float64 { return 0.75 }
func (r *TestDispatcherNotInjectedRule) NodeTypes() []string {
	return []string{"navigation_expression"}
}

func (r *TestDispatcherNotInjectedRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	if text != "Dispatchers.IO" && text != "Dispatchers.Default" && text != "Dispatchers.Main" {
		return nil
	}

	fn, ok := flatEnclosingFunction(file, idx)
	if !ok || !testingQualityIsTestFunction(file, fn) {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"Avoid production dispatchers in tests; inject a `TestDispatcher`.",
	)}
}

// ---------------------------------------------------------------------------
// TestWithoutAssertionRule
// ---------------------------------------------------------------------------

type TestWithoutAssertionRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *TestWithoutAssertionRule) Confidence() float64 { return 0.75 }
func (r *TestWithoutAssertionRule) NodeTypes() []string { return []string{"function_declaration"} }

func (r *TestWithoutAssertionRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !testingQualityIsTestFunction(file, idx) {
		return nil
	}

	body := file.FlatFindChild(idx, "function_body")
	if body == 0 {
		return nil
	}

	found := false
	file.FlatWalkAllNodes(body, func(n uint32) {
		if found {
			return
		}
		if file.FlatType(n) != "call_expression" {
			return
		}
		name := flatCallNameAny(file, n)
		if testingQualityIsAssertionCall(name) || testingQualityIsVerifyCall(name) {
			found = true
		}
	})
	if found {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"Test function has no assertion; add a verification.",
	)}
}

// ---------------------------------------------------------------------------
// TestWithOnlyTodoRule
// ---------------------------------------------------------------------------

type TestWithOnlyTodoRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *TestWithOnlyTodoRule) Confidence() float64 { return 0.75 }
func (r *TestWithOnlyTodoRule) NodeTypes() []string { return []string{"function_declaration"} }

func (r *TestWithOnlyTodoRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !testingQualityIsTestFunction(file, idx) {
		return nil
	}

	if flatHasAnnotationNamed(file, idx, "Ignore") || flatHasAnnotationNamed(file, idx, "Disabled") {
		return nil
	}

	body := file.FlatFindChild(idx, "function_body")
	if body == 0 {
		return nil
	}

	stmtCount := 0
	isTodoOrFail := false
	file.FlatWalkAllNodes(body, func(n uint32) {
		if file.FlatType(n) == "call_expression" {
			name := flatCallNameAny(file, n)
			if name == "TODO" || name == "fail" {
				isTodoOrFail = true
			}
			stmtCount++
		}
	})

	if !isTodoOrFail || stmtCount != 1 {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"Test with only `TODO()`; annotate with `@Ignore` or implement it.",
	)}
}

// ---------------------------------------------------------------------------
// TestFunctionReturnValueRule
// ---------------------------------------------------------------------------

type TestFunctionReturnValueRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *TestFunctionReturnValueRule) Confidence() float64 { return 0.75 }
func (r *TestFunctionReturnValueRule) NodeTypes() []string { return []string{"function_declaration"} }

func (r *TestFunctionReturnValueRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !testingQualityIsTestFunction(file, idx) {
		return nil
	}

	retType := testingQualityReturnType(file, idx)
	if retType == "" || retType == "Unit" {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"Test functions should return `Unit`; JUnit ignores return values.",
	)}
}

// ---------------------------------------------------------------------------
// TestNameContainsUnderscoreRule
// ---------------------------------------------------------------------------

type TestNameContainsUnderscoreRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *TestNameContainsUnderscoreRule) Confidence() float64 { return 0.6 }
func (r *TestNameContainsUnderscoreRule) NodeTypes() []string {
	return []string{"function_declaration"}
}

func (r *TestNameContainsUnderscoreRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !testingQualityIsTestFunction(file, idx) {
		return nil
	}

	name := testingQualityFunctionName(file, idx)
	if name == "" || !strings.Contains(name, "_") {
		return nil
	}

	if strings.HasPrefix(name, "`") {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"Test name uses underscores; consider backtick-quoted names.",
	)}
}

// ---------------------------------------------------------------------------
// SharedMutableStateInObjectRule
// ---------------------------------------------------------------------------

type SharedMutableStateInObjectRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *SharedMutableStateInObjectRule) Confidence() float64 { return 0.75 }
func (r *SharedMutableStateInObjectRule) NodeTypes() []string {
	return []string{"companion_object", "object_declaration"}
}

func (r *SharedMutableStateInObjectRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !testingQualityIsTestFile(file) {
		return nil
	}

	var findings []scanner.Finding
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "class_body" {
			for member := file.FlatFirstChild(child); member != 0; member = file.FlatNextSib(member) {
				if file.FlatType(member) != "property_declaration" {
					continue
				}
				text := file.FlatNodeText(member)
				if strings.HasPrefix(strings.TrimSpace(text), "var ") {
					findings = append(findings, r.Finding(
						file,
						file.FlatRow(member)+1,
						file.FlatCol(member)+1,
						"Mutable state in companion/object is shared across tests.",
					))
				}
			}
		}
	}
	return findings
}

// ---------------------------------------------------------------------------
// TestInheritanceDepthRule
// ---------------------------------------------------------------------------

type TestInheritanceDepthRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *TestInheritanceDepthRule) Confidence() float64 { return 0.6 }
func (r *TestInheritanceDepthRule) NodeTypes() []string { return []string{"class_declaration"} }

func (r *TestInheritanceDepthRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !testingQualityIsTestFile(file) {
		return nil
	}

	delegation := file.FlatFindChild(idx, "delegation_specifier")
	if delegation == 0 {
		return nil
	}

	supertypes := testingQualitySupertypes(file, idx)
	if len(supertypes) == 0 {
		return nil
	}

	depth := 1
	for _, st := range supertypes {
		d := testingQualityCountInheritanceInFile(file, st, 1)
		if d > depth {
			depth = d
		}
	}

	if depth < 2 {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"Test class inheritance depth exceeds 2; flatten the hierarchy.",
	)}
}

// ---------------------------------------------------------------------------
// RelaxedMockUsedForValueClassRule
// ---------------------------------------------------------------------------

type RelaxedMockUsedForValueClassRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *RelaxedMockUsedForValueClassRule) Confidence() float64 { return 0.75 }
func (r *RelaxedMockUsedForValueClassRule) NodeTypes() []string {
	return []string{"call_expression"}
}

var primitiveTypes = map[string]bool{
	"Int": true, "Long": true, "Float": true, "Double": true,
	"Boolean": true, "String": true, "Byte": true, "Short": true, "Char": true,
}

func (r *RelaxedMockUsedForValueClassRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if flatCallNameAny(file, idx) != "mockk" {
		return nil
	}

	nodeText := file.FlatNodeText(idx)
	if !strings.Contains(nodeText, "relaxed") {
		return nil
	}

	typeArg := testingQualityTypeArgument(file, idx)
	if typeArg == "" {
		return nil
	}

	if !primitiveTypes[typeArg] {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"Don't mock primitives/value types; use literal values.",
	)}
}

// ---------------------------------------------------------------------------
// SpyOnDataClassRule
// ---------------------------------------------------------------------------

type SpyOnDataClassRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *SpyOnDataClassRule) Confidence() float64 { return 0.6 }
func (r *SpyOnDataClassRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *SpyOnDataClassRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := flatCallNameAny(file, idx)
	if name != "spyk" && name != "spy" {
		return nil
	}

	_, args := flatCallExpressionParts(file, idx)
	if args == 0 {
		return nil
	}

	valueArgs := testingQualityValueArgumentsFlat(file, args)
	if len(valueArgs) == 0 {
		return nil
	}

	argExpr := flatValueArgumentExpression(file, valueArgs[0])
	if argExpr == 0 {
		return nil
	}

	if file.FlatType(argExpr) != "call_expression" {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"Avoid spying on data classes; value-based equality breaks spy semantics.",
	)}
}

// ---------------------------------------------------------------------------
// VerifyWithoutMockRule
// ---------------------------------------------------------------------------

type VerifyWithoutMockRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *VerifyWithoutMockRule) Confidence() float64 { return 0.6 }
func (r *VerifyWithoutMockRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *VerifyWithoutMockRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := flatCallNameAny(file, idx)
	if name != "verify" && name != "coVerify" {
		return nil
	}

	lambda := flatCallTrailingLambda(file, idx)
	if lambda == 0 {
		return nil
	}

	var receivers []string
	file.FlatWalkAllNodes(lambda, func(n uint32) {
		if file.FlatType(n) == "navigation_expression" {
			first := file.FlatNamedChild(n, 0)
			if first != 0 && file.FlatType(first) == "simple_identifier" {
				receivers = append(receivers, file.FlatNodeText(first))
			}
		}
	})

	if len(receivers) == 0 {
		return nil
	}

	fn, ok := flatEnclosingFunction(file, idx)
	if !ok {
		return nil
	}

	body := file.FlatFindChild(fn, "function_body")
	if body == 0 {
		return nil
	}

	mockVars := make(map[string]bool)
	file.FlatWalkAllNodes(body, func(n uint32) {
		if file.FlatType(n) != "property_declaration" {
			return
		}
		varDecl := file.FlatFindChild(n, "variable_declaration")
		if varDecl == 0 {
			return
		}
		ident := file.FlatFindChild(varDecl, "simple_identifier")
		if ident == 0 {
			return
		}
		for rhs := file.FlatFirstChild(n); rhs != 0; rhs = file.FlatNextSib(rhs) {
			if file.FlatType(rhs) == "call_expression" {
				cn := flatCallNameAny(file, rhs)
				if cn == "mockk" || cn == "mock" || cn == "spyk" || cn == "spy" {
					mockVars[file.FlatNodeText(ident)] = true
				}
			}
		}
	})

	var findings []scanner.Finding
	for _, recv := range receivers {
		if !mockVars[recv] {
			findings = append(findings, r.Finding(
				file,
				file.FlatRow(idx)+1,
				file.FlatCol(idx)+1,
				fmt.Sprintf("Calling `verify` on a non-mock object; ensure `%s` is a mock.", recv),
			))
		}
	}
	return findings
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
	"check": true,
}

func testingQualityIsAssertionCall(name string) bool {
	if assertionCallNames[name] {
		return true
	}
	return strings.HasPrefix(name, "assert")
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

func testingQualityInsideRunTest(file *scanner.File, idx uint32) bool {
	for current, ok := file.FlatParent(idx); ok; current, ok = file.FlatParent(current) {
		if file.FlatType(current) == "call_expression" {
			name := flatCallNameAny(file, current)
			if name == "runTest" {
				return true
			}
		}
	}
	return false
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
