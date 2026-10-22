package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

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
