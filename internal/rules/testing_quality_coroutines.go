package rules

import (
	"regexp"
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

// testingQualityRunBlockingWithRxTestScheduler reports a non-suspend test that drives an Rx TestScheduler,
// where runTest would not control the virtual time.
func testingQualityRunBlockingWithRxTestScheduler(file *scanner.File, fn uint32) bool {
	if !fileImportsFQN(file, "io.reactivex.schedulers.TestScheduler") &&
		!fileImportsFQN(file, "io.reactivex.rxjava3.schedulers.TestScheduler") {
		return false
	}
	if mods, ok := file.FlatFindChild(fn, "modifiers"); ok && strings.Contains(file.FlatNodeText(mods), "suspend") {
		return false
	}
	found := false
	file.FlatWalkAllNodes(fn, func(n uint32) {
		if !found && file.FlatType(n) == "simple_identifier" && file.FlatNodeTextEquals(n, "TestScheduler") {
			found = true
		}
	})
	return found
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
		fileImportsFQN(file, "com.github.takahirom.roborazzi.RoborazziRule") ||
		testingQualityIsDeviceJourneyFile(file)
}

// testingQualityIsDeviceJourneyFile reports macrobenchmark, UI Automator and benchmark-source-set instrumentation
// journeys. Their test passes when the flow completes without throwing, so there is no assertion to add.
func testingQualityIsDeviceJourneyFile(file *scanner.File) bool {
	if fileFactsCache().Imports(file).HasAnyPrefix("androidx.test.uiautomator", "androidx.benchmark.macro") {
		return true
	}
	path := scanner.LowerSlashPath(file.Path)
	if !strings.Contains(path, "/benchmark/") && !strings.Contains(path, "/macrobenchmark/") {
		return false
	}
	return strings.Contains(string(file.Content), "@LargeTest") || strings.Contains(string(file.Content), "AndroidJUnit4")
}

func testingQualityIsAndroidInstrumentedTestFile(file *scanner.File) bool {
	if file == nil {
		return false
	}
	path := scanner.LowerPath(file.Path)
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

// testingQualityInlineScopeCalls run their lambda inline, so a `delay` inside one is still in the test body.
var testingQualityInlineScopeCalls = map[string]bool{
	"let": true, "run": true, "apply": true, "also": true, "with": true, "repeat": true,
	"forEach": true, "use": true, "runCatching": true, "assertFailsWith": true, "assertThrows": true,
}

// testingQualityDelayInRunTestBody reports whether idx runs directly in the runTest body. A delay inside
// launch, async, a flow builder or a fake's lambda is a child coroutine, so advanceTimeBy can't replace it.
func testingQualityDelayInRunTestBody(file *scanner.File, idx uint32) bool {
	return testingQualityRunTestLambdaOfDelay(file, idx) != 0
}

// testingQualityRunTestLambdaOfDelay returns the runTest lambda whose own body (through inline scopes) holds idx, or 0.
func testingQualityRunTestLambdaOfDelay(file *scanner.File, idx uint32) uint32 {
	for current, ok := file.FlatParent(idx); ok; current, ok = file.FlatParent(current) {
		switch file.FlatType(current) {
		case "function_declaration", "anonymous_function":
			return 0
		case "lambda_literal":
			owner, found := flatEnclosingAncestor(file, current, "call_expression")
			if !found || !testingQualityLambdaIsCallArgument(file, current, owner) {
				return 0
			}
			name := flatCallNameAny(file, owner)
			if name == "runTest" && testingQualityIsCoroutineRunTestCall(file, owner) {
				return current
			}
			if !testingQualityInlineScopeCalls[name] {
				return 0
			}
		}
	}
	return 0
}

// testingQualityRunTestSchedulesOtherWork reports other coroutine or scheduler calls in the runTest body,
// which make a delay a legitimate way to let them run on the virtual clock.
func testingQualityRunTestSchedulesOtherWork(file *scanner.File, lambda uint32) bool {
	if testingQualityFileSharesTestScheduler(file) || testingQualityRunTestCallsOtherCode(file, lambda) {
		return true
	}
	found := false
	file.FlatWalkNodes(lambda, "call_expression", func(call uint32) {
		switch flatCallNameAny(file, call) {
		case "launch", "async", "produce", "withContext", "runBlocking", "advanceTimeBy", "advanceUntilIdle", "runCurrent",
			"collect", "first", "toList", "backgroundScope", "yield", "join", "await":
			found = true
		}
	})
	return found
}

// testingQualityLambdaIsCallArgument reports a lambda passed straight to the call. A lambda stored in a
// property or assignment (a fake's lambda property) runs later, outside the test body.
func testingQualityLambdaIsCallArgument(file *scanner.File, lambda, call uint32) bool {
	for p, ok := file.FlatParent(lambda); ok && p != call; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "annotated_lambda", "call_suffix", "value_argument", "value_arguments":
		default:
			return false
		}
	}
	return true
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

// testingQualityNonProductionDispatcherConsumers drive the test's own coroutines, so a real dispatcher there is not injected into production code.
var testingQualityNonProductionDispatcherConsumers = map[string]bool{
	"launch": true, "async": true, "withContext": true, "runBlocking": true, "runTest": true,
	"CoroutineScope": true, "flowOn": true, "produce": true, "supervisorScope": true,
	"coroutineScope": true, "withTimeout": true, "withTimeoutOrNull": true, "plus": true,
	"asExecutor": true, "limitedParallelism": true,
}

// testingQualityDispatcherPassedToProductionCode reports whether idx is an argument of a constructor or factory call.
func testingQualityDispatcherPassedToProductionCode(file *scanner.File, idx uint32) bool {
	arg := uint32(0)
	for current, ok := file.FlatParent(idx); ok; current, ok = file.FlatParent(current) {
		switch file.FlatType(current) {
		case "value_argument":
			arg = current
		case "lambda_literal", "function_body", "statements", "function_declaration":
			return false
		}
		if arg != 0 {
			break
		}
	}
	if arg == 0 {
		return false
	}
	call, found := flatEnclosingAncestor(file, arg, "call_expression")
	if !found {
		return false
	}
	return !testingQualityNonProductionDispatcherConsumers[flatCallNameAny(file, call)]
}

// testingQualitySleepInPollingLoop reports whether a Thread.sleep sits in a loop that polls real threads.
func testingQualitySleepInPollingLoop(file *scanner.File, idx uint32) bool {
	for current, ok := file.FlatParent(idx); ok; current, ok = file.FlatParent(current) {
		switch file.FlatType(current) {
		case "while_statement", "do_while_statement", "for_statement":
			return true
		case "function_declaration":
			return false
		}
	}
	return false
}

// testingQualityClassUsesRealDispatchers reports whether the test class deliberately runs on real dispatchers.
// extraMarkers are configured strings (for example a suppression id) whose presence in the class marks it the same way.
func testingQualityClassUsesRealDispatchers(file *scanner.File, fn uint32, extraMarkers []string) bool {
	cls, ok := flatEnclosingAncestor(file, fn, "class_declaration", "object_declaration")
	if !ok {
		return false
	}
	text := file.FlatNodeText(cls)
	return strings.Contains(text, "Dispatchers.Default") || strings.Contains(text, "Dispatchers.IO") ||
		containsAnyNonEmpty(text, extraMarkers)
}

var testingQualityRealConcurrencyPattern = regexp.MustCompile(
	`\b(CountDownLatch|CyclicBarrier|Semaphore|CompletableDeferred|Executors?|newSingleThreadContext|asCoroutineDispatcher)\b|\b\w*[Ll]atch\b|Dispatchers\.(Default|IO)\b|\b[Rr]eal\w*Dispatchers?\b|\b(thread|Thread)\s*[({]`)

// testingQualityTestUsesRealConcurrency reports a test that coordinates real threads or latches. Virtual time
// cannot replace a wait there, because the other side runs on the wall clock.
func testingQualityTestUsesRealConcurrency(file *scanner.File, idx uint32) bool {
	fn, ok := flatEnclosingAncestor(file, idx, "function_declaration")
	if !ok {
		return false
	}
	return testingQualityRealConcurrencyPattern.MatchString(file.FlatNodeText(fn)) || testingQualityClassUsesRealDispatchers(file, fn, nil)
}

var testingQualitySharedSchedulerPattern = regexp.MustCompile(
	`\b(coroutineRule|StandardTestDispatcher|UnconfinedTestDispatcher|TestDispatcher|TestScope|backgroundScope|TestCoroutineScheduler|MainDispatcherRule)\b|\bdispatchers\s*=`)

// testingQualityFileSharesTestScheduler reports a file that builds objects on test dispatchers, so a delay
// in runTest advances the virtual time those objects' work runs on.
func testingQualityFileSharesTestScheduler(file *scanner.File) bool {
	return file != nil && testingQualitySharedSchedulerPattern.Match(file.Content)
}

// testingQualityNeutralCalls are calls that neither start work nor read time-dependent state.
var testingQualityNeutralCalls = map[string]bool{
	"delay": true, "repeat": true, "println": true, "print": true, "check": true, "require": true, "fail": true,
	"listOf": true, "mutableListOf": true, "mapOf": true, "setOf": true, "emptyList": true,
	"let": true, "run": true, "apply": true, "also": true, "with": true, "runTest": true,
	"isEqualTo": true, "isNotEqualTo": true, "isTrue": true, "isFalse": true, "isNull": true, "isNotNull": true,
	"isEmpty": true, "hasSize": true, "contains": true, "containsExactly": true,
}

// testingQualityRunTestCallsOtherCode reports a runTest body that calls code beyond assertions and stdlib
// helpers, or changes state between delays. That code can depend on the elapsed virtual time.
func testingQualityRunTestCallsOtherCode(file *scanner.File, lambda uint32) bool {
	found := false
	file.FlatWalkNodes(lambda, "call_expression", func(call uint32) {
		name := flatCallNameAny(file, call)
		if found || testingQualityNeutralCalls[name] || strings.HasPrefix(name, "assert") || strings.HasPrefix(name, "expect") ||
			testingQualityMockSetupCalls[name] || testingQualityInsideMockSetup(file, call, lambda) {
			return
		}
		found = true
	})
	if found {
		return true
	}
	file.FlatWalkNodes(lambda, "assignment", func(uint32) { found = true })
	for _, kind := range []string{"postfix_expression", "prefix_expression"} {
		file.FlatWalkNodes(lambda, kind, func(n uint32) {
			text := file.FlatNodeText(n)
			if strings.HasSuffix(text, "++") || strings.HasSuffix(text, "--") || strings.HasPrefix(text, "++") || strings.HasPrefix(text, "--") {
				found = true
			}
		})
	}
	return found
}

// testingQualityMockSetupCalls only configure or check test doubles.
var testingQualityMockSetupCalls = map[string]bool{
	"every": true, "coEvery": true, "verify": true, "coVerify": true, "mockk": true, "spyk": true,
	"justRun": true, "coJustRun": true,
}

// testingQualityInsideMockSetup reports a call inside a mock stubbing or verification block.
func testingQualityInsideMockSetup(file *scanner.File, call, lambda uint32) bool {
	for p, ok := file.FlatParent(call); ok && p != lambda; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "call_expression":
			if testingQualityMockSetupCalls[flatCallNameAny(file, p)] {
				return true
			}
		case "infix_expression":
			if testingQualityIsMockKAnswerInfix(file, p) {
				return true
			}
		}
	}
	return false
}

var testingQualityCoroutineTestMarkers = []string{
	"kotlinx.coroutines.test", "runTest", "TestScope", "TestDispatcher", "TestCoroutineScheduler",
	"runCurrent", "advanceUntilIdle", "MainDispatcherRule",
}

// testingQualityFileIsCoroutineTest reports whether the file already uses kotlinx-coroutines-test, so
// runTest is the idiom there.
func testingQualityFileIsCoroutineTest(file *scanner.File) bool {
	content := string(file.Content)
	for _, marker := range testingQualityCoroutineTestMarkers {
		if strings.Contains(content, marker) {
			return true
		}
	}
	return false
}

// testingQualityRunBlockingIsWholeTestBody reports whether the runBlocking call is the entire test body
// (`= runBlocking { }`, `{ runBlocking { } }` or `{ return runBlocking { } }`), where runTest is a drop-in.
func testingQualityRunBlockingIsWholeTestBody(file *scanner.File, call uint32, fn uint32) bool {
	body, ok := file.FlatFindChild(fn, "function_body")
	if !ok || body == 0 {
		return false
	}
	var only uint32
	count := 0
	var visit func(n uint32)
	visit = func(n uint32) {
		for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
			if !file.FlatIsNamed(c) || strings.HasSuffix(file.FlatType(c), "comment") {
				continue
			}
			if file.FlatType(c) == "statements" {
				visit(c)
				continue
			}
			count++
			only = c
		}
	}
	visit(body)
	if count != 1 {
		return false
	}
	if file.FlatType(only) == "jump_expression" {
		return file.FlatNodeText(only) != "" && strings.Contains(file.FlatNodeText(only), file.FlatNodeText(call))
	}
	return only == call
}

var (
	testingQualitySchedulerMarkers = []string{"TestCoroutineScheduler", "testScheduler", "MainCoroutineRule", "TestDispatcher"}
	testingQualitySchedulerSteps   = []string{"runCurrent(", "advanceUntilIdle(", "advanceTimeBy("}
)

// testingQualityFileDrivesOwnScheduler reports a file that owns a test scheduler and steps it by hand.
func testingQualityFileDrivesOwnScheduler(file *scanner.File) bool {
	content := string(file.Content)
	return containsAny(content, testingQualitySchedulerMarkers) && containsAny(content, testingQualitySchedulerSteps)
}

func testingQualityFunctionIsSuspend(file *scanner.File, fn uint32) bool {
	mods, ok := file.FlatFindChild(fn, "modifiers")
	return ok && strings.Contains(file.FlatNodeText(mods), "suspend")
}
