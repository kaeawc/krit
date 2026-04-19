package rules

import (
	"strings"

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
