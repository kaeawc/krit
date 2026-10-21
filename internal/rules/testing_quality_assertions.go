package rules

import (
	"path"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

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
