package rules

import (
	"bytes"
	"path"
	"strings"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
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

// testingQualityIsNullableAssertionCall reports whether the call is an assertion-library function that
// accepts the `!!` operand. Project helpers that merely start with "assert" are not: their parameter types are
// unknown, and `assertNotNull` cannot be applied to what they check. A function declared in the file with a
// library name shadows the library.
func testingQualityIsNullableAssertionCall(ctx *api.Context, idx uint32, name string) bool {
	// assertNotNull and assertNull take the nullable value itself, so `!!` there is not fixed by assertNotNull.
	if name == "" || name == "assertNotNull" || name == "assertNull" || !nullableAssertionCallNames[name] {
		return false
	}
	if fileDeclaresFunctionNamed(ctx.File, name) {
		return false
	}
	if resolver, ok := ctx.Resolver.(*oracle.CompositeResolver); ok && resolver.Oracle() != nil {
		if target := oracleLookupCallTargetFlat(resolver.Oracle(), ctx.File, idx); target != "" {
			for _, prefix := range ignoredReturnValueTestLibraryPrefixes {
				if strings.HasPrefix(target, prefix) {
					return true
				}
			}
			// Resolved to a project function that reuses a library name.
			return false
		}
	}
	return true
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

// testingQualityWaitHelpersNeedingEvidence are waits that only count with framework evidence (handled
// separately) or that synchronize without checking anything.
var testingQualityWaitHelpersNeedingEvidence = map[string]bool{
	"waitForIdle": true, "waitForIdleSync": true, "waitUntilAtLeastOneExists": true, "waitUntilExactlyOneExists": true,
	"waitUntilDoesNotExist": true, "waitFor": true,
}

// testingQualityIsWaitOrEnsureHelper matches helpers such as waitForRow or ensureNoEvents, which fail the test
// on timeout or a violated condition. The prefix must end at a word boundary, so `waiter` does not match.
// A function declared in the same file counts only through its own body (see testingQualityAssertionHelperNames).
func testingQualityIsWaitOrEnsureHelper(name string) bool {
	if testingQualityWaitHelpersNeedingEvidence[name] {
		return false
	}
	return testingQualityHasLowerPrefixWord(name, "ensure") ||
		testingQualityHasLowerPrefixWord(name, "waitFor") ||
		testingQualityHasLowerPrefixWord(name, "waitUntil")
}

var verifyCallNames = map[string]bool{
	"verify": true, "coVerify": true,
	"confirmVerified":          true,
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
				(testingQualityIsWaitOrEnsureHelper(name) && !ignoredReturnValueFileDeclaresFunction(file, name)) ||
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

// testingQualityBodyIsEmpty reports a function body that is `= Unit` or a block with no statements.
func testingQualityBodyIsEmpty(file *scanner.File, body uint32) bool {
	text := strings.TrimSpace(file.FlatNodeText(body))
	if strings.TrimSpace(strings.TrimPrefix(text, "=")) == "Unit" && strings.HasPrefix(text, "=") {
		return true
	}
	if !strings.HasPrefix(text, "{") {
		return false
	}
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "statements" && swallowedHasNamedStatement(file, child) {
			return false
		}
	}
	return true
}

// testingQualityBangIsAssertionSubject reports a `!!` that sits in the assertion's own argument, possibly as the
// receiver of a member chain. A `!!` inside a nested call's arguments (a constructor or a lookup) is not the subject.
func testingQualityBangIsAssertionSubject(file *scanner.File, bang, args uint32) bool {
	for current, ok := file.FlatParent(bang); ok && current != args; current, ok = file.FlatParent(current) {
		switch file.FlatType(current) {
		case "value_arguments", "call_suffix":
			return false
		}
	}
	return true
}

// testingQualityBangIsMemberReceiver reports a `x!!` that is the receiver of a member access, as in `x!!.ts`.
func testingQualityBangIsMemberReceiver(file *scanner.File, postfix uint32) bool {
	parent, ok := file.FlatParent(postfix)
	return ok && file.FlatType(parent) == "navigation_expression" && file.FlatNamedChild(parent, 0) == postfix
}

// testingQualityOperandDeclaredNullable reports a `!!` operand whose type is declared nullable. A Java platform
// type (`getText()` of a view) is not nullable in Kotlin, so asserting it non-null is not a nullable assertion.
// Without a resolver there is no type evidence, so the operand is kept.
func testingQualityOperandDeclaredNullable(ctx *api.Context, bang uint32) bool {
	if ctx.Resolver == nil {
		return true
	}
	operand := flatFirstNamedChild(ctx.File, bang)
	if operand == 0 {
		return false
	}
	resolved := ctx.Resolver.ResolveFlatNode(operand, ctx.File)
	return resolved != nil && resolved.IsNullable()
}

// oracleHasDiagnosticAtNode reports a compiler diagnostic with one of the factory names anchored at idx.
func oracleHasDiagnosticAtNode(ctx *api.Context, idx uint32, factoryNames ...string) bool {
	if ctx == nil || ctx.File == nil || ctx.Resolver == nil {
		return false
	}
	resolver, ok := ctx.Resolver.(*oracle.CompositeResolver)
	if !ok || resolver.Oracle() == nil {
		return false
	}
	for _, d := range oracleLookupDiagnosticsForFlatRange(resolver.Oracle(), ctx.File, idx) {
		if diagnosticProjectionMatchesFactory(factoryNames, d.FactoryName) && diagnosticAnchoredAtNode(ctx.File, idx, d) {
			return true
		}
	}
	return false
}

// testingQualityNameDocumentsNoOp reports a test named for a deliberate no-op ("is no-op", "noop"), where
// calling the subject without an error is the whole check.
func testingQualityNameDocumentsNoOp(file *scanner.File, idx uint32) bool {
	name := strings.ToLower(strings.Trim(testingQualityFunctionName(file, idx), "`"))
	for _, sep := range []string{"_", "-", " "} {
		name = strings.ReplaceAll(name, sep, "")
	}
	return strings.Contains(name, "noop")
}

// testingQualityIsConstructionDoesNotThrowTest reports a test whose body is one constructor or factory call
// while a sibling test in the class expects the same kind of call to throw. Reaching the end is the check.
func testingQualityIsConstructionDoesNotThrowTest(file *scanner.File, idx, body uint32) bool {
	class := flatEnclosingClassLike(file, idx)
	if class == 0 {
		return false
	}
	classText := file.FlatNodeBytes(class)
	if !bytes.Contains(classText, []byte("assertFailsWith")) && !bytes.Contains(classText, []byte("assertThrows")) &&
		!bytes.Contains(classText, []byte("assertFails(")) {
		return false
	}
	var statements uint32
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "statements" {
			statements = child
		}
	}
	if statements == 0 {
		return false
	}
	var only uint32
	for child := file.FlatFirstChild(statements); child != 0; child = file.FlatNextSib(child) {
		if isFlatCommentNode(file, child) || !file.FlatIsNamed(child) {
			continue
		}
		if only != 0 {
			return false
		}
		only = child
	}
	if only == 0 || file.FlatType(only) != "call_expression" {
		return false
	}
	name := flatCallExpressionName(file, only)
	if name == "" {
		return false
	}
	first := name[0]
	return (first >= 'A' && first <= 'Z') || strings.HasPrefix(name, "create") || strings.HasPrefix(name, "from") ||
		strings.HasPrefix(name, "parse") || name == "of" || name == "valueOf"
}
