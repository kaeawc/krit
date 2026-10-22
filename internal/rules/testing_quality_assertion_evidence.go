package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	"github.com/kaeawc/krit/internal/scanner"
)

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
