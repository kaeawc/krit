package rules

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func registerTestingQualityRules() {
	registerTestingQualityAssertEqualsArgumentOrder()
	registerTestingQualityAssertTrueOnComparison()
	registerTestingQualityMixedAssertionLibraries()
	registerTestingQualityAssertNullableWithNotNullAssertion()
	registerTestingQualityMockWithoutVerify()
	registerTestingQualityRunTestWithDelay()
	registerTestingQualityRunTestWithThreadSleep()
	registerTestingQualityRunBlockingInTest()
	registerTestingQualityTestDispatcherNotInjected()
	registerTestingQualityTestWithoutAssertion()
	registerTestingQualityTestWithOnlyTodo()
	registerTestingQualityTestFunctionReturnValue()
	registerTestingQualityTestNameContainsUnderscore()
	registerTestingQualitySharedMutableStateInObject()
	registerTestingQualityTestInheritanceDepth()
	registerTestingQualityRelaxedMockUsedForValueClass()
	registerTestingQualitySpyOnDataClass()
	registerTestingQualityVerifyWithoutMock()
	registerTestingQualityUntestedPublicAPI()
}

func registerTestingQualityUntestedPublicAPI() {
	r := &UntestedPublicAPIRule{
		BaseRule: BaseRule{RuleName: "UntestedPublicApi", RuleSetName: testingQualityRuleSet, Sev: "info", Desc: "Detects public Kotlin API declarations that are not referenced from test sources."},
	}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		Needs:      api.NeedsCrossFile | api.NeedsParsedFiles,
		Languages:  []scanner.Language{scanner.LangKotlin},
		Confidence: r.Confidence(), Implementation: r,
		Check: r.check,
	})
}

func registerTestingQualityAssertEqualsArgumentOrder() {
	r := &AssertEqualsArgumentOrderRule{
		BaseRule: BaseRule{RuleName: "AssertEqualsArgumentOrder", RuleSetName: testingQualityRuleSet, Sev: "info", Desc: "Detects assertEquals calls with reversed argument order (actual, expected) instead of (expected, actual)."},
	}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if flatCallExpressionName(file, idx) != "assertEquals" {
				return
			}
			_, args := flatCallExpressionParts(file, idx)
			if args == 0 {
				return
			}
			valueArgs := testingQualityValueArgumentsFlat(file, args)
			// The pair may follow a JUnit message or precede a kotlin.test message or a delta.
			if len(valueArgs) < 2 || len(valueArgs) > 4 {
				return
			}
			reversed := false
			for i := 0; i+1 < len(valueArgs); i++ {
				first := apiNodeNameFlat(file, flatValueArgumentExpression(file, valueArgs[i]))
				second := apiNodeNameFlat(file, flatValueArgumentExpression(file, valueArgs[i+1]))
				if first == "actual" && second == "expected" {
					reversed = true
					break
				}
			}
			if !reversed {
				return
			}
			ctx.EmitAt(callLine(file, idx), callCol(file, idx), "`assertEquals` arguments appear reversed; use (expected, actual).")
		},
	})
}

func registerTestingQualityAssertTrueOnComparison() {
	r := &AssertTrueOnComparisonRule{
		BaseRule: BaseRule{RuleName: "AssertTrueOnComparison", RuleSetName: testingQualityRuleSet, Sev: "info", Desc: "Detects assertTrue(a == b) calls that should use assertEquals for better failure messages."},
	}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if flatCallExpressionName(file, idx) != "assertTrue" {
				return
			}
			if !testingQualityAssertTrueOnComparisonKnownCall(file, idx) {
				return
			}
			_, args := flatCallExpressionParts(file, idx)
			if args == 0 {
				return
			}
			valueArgs := testingQualityValueArgumentsFlat(file, args)
			if len(valueArgs) != 1 {
				return
			}
			condition := flatValueArgumentExpression(file, valueArgs[0])
			if condition == 0 || file.FlatType(condition) != "equality_expression" || file.FlatChildCount(condition) < 3 {
				return
			}
			op := file.FlatChild(condition, 1)
			if op == 0 {
				return
			}
			isEquals := file.FlatNodeTextEquals(op, "==")
			isNotEquals := file.FlatNodeTextEquals(op, "!=")
			if !isEquals && !isNotEquals {
				return
			}
			// A null operand has dedicated assertions with clearer failure messages.
			for _, operand := range []uint32{file.FlatChild(condition, 0), file.FlatChild(condition, 2)} {
				if operand != 0 && file.FlatType(operand) == "null" {
					if isEquals {
						ctx.EmitAt(callLine(file, idx), callCol(file, idx), "Use `assertNull` instead of `assertTrue` for null comparisons.")
					} else {
						ctx.EmitAt(callLine(file, idx), callCol(file, idx), "Use `assertNotNull` instead of `assertTrue` for null comparisons.")
					}
					return
				}
			}
			if !isEquals {
				return
			}
			// Comparing against a Boolean literal is a null-safe idiom, not value equality.
			for _, operand := range []uint32{file.FlatChild(condition, 0), file.FlatChild(condition, 2)} {
				if operand != 0 && file.FlatType(operand) == "boolean_literal" {
					return
				}
			}
			ctx.EmitAt(callLine(file, idx), callCol(file, idx), "Use `assertEquals` instead of `assertTrue` for equality comparisons.")
		},
	})
}

func registerTestingQualityMixedAssertionLibraries() {
	r := &MixedAssertionLibrariesRule{
		BaseRule: BaseRule{RuleName: "MixedAssertionLibraries", RuleSetName: testingQualityRuleSet, Sev: "info", Desc: "Detects files that import both JUnit Assert and Google Truth assertion APIs."},
	}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes:      []string{"source_file"},
		Languages:      []scanner.Language{scanner.LangKotlin},
		Confidence:     r.Confidence(),
		Implementation: r,
		Check:          r.check,
	})
}

func registerTestingQualityAssertNullableWithNotNullAssertion() {
	r := &AssertNullableWithNotNullAssertionRule{
		BaseRule: BaseRule{RuleName: "AssertNullableWithNotNullAssertion", RuleSetName: testingQualityRuleSet, Sev: "warning", Desc: "Detects non-null assertions (!!) inside assertion calls where assertNotNull should be used instead."},
	}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
		// The compiler's unnecessary-assertion diagnostic shows the operand is already non-null.
		Needs: api.NeedsResolver | api.NeedsOracleDiagnostics | api.NeedsOracleCallTargets,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if !testingQualityRuleShouldRunInFile(file) {
				return
			}
			name := flatCallExpressionName(file, idx)
			if !testingQualityIsNullableAssertionCall(ctx, idx, name) {
				return
			}
			_, args := flatCallExpressionParts(file, idx)
			if args == 0 {
				return
			}
			var bangs, postfixes []uint32
			file.FlatWalkAllNodes(args, func(n uint32) {
				if testingQualityInsideNestedExecutableArgument(file, n, args) {
					return
				}
				if file.FlatType(n) == "postfix_expression" {
					// Without a Kotlin contract, `x!!` after an assertion on x is the required idiom.
					if postfixExpressionHasBangBang(file, n) && !testingQualityOperandAssertedNonNull(file, idx, n) &&
						testingQualityBangIsAssertionSubject(file, n, args) &&
						testingQualityOperandDeclaredNullable(ctx, n) &&
						!oracleHasDiagnosticAtNode(ctx, n, "UNNECESSARY_NOT_NULL_ASSERTION") {
						bangs = append(bangs, bangBangNode(file, n))
						postfixes = append(postfixes, n)
					}
				}
			})
			if len(bangs) == 0 {
				return
			}
			// `x!!.ts` inside an argument reads a member; it does not assert the value of x itself.
			scope := "assertion arguments"
			// List the bangs in source order, so chained ones (`a!!.b!!`) read innermost first.
			order := make([]int, len(bangs))
			for i := range order {
				order[i] = i
			}
			sort.Slice(order, func(a, b int) bool { return file.FlatStartByte(bangs[order[a]]) < file.FlatStartByte(bangs[order[b]]) })
			firstBang := bangs[order[0]]
			truth := testingQualityFileUsesTruth(file)
			var checks []string
			for _, i := range order {
				if !testingQualityBangIsMemberReceiver(file, postfixes[i]) {
					scope = "assertions"
				}
				operand := testingQualityShortOperand(strings.ReplaceAll(file.FlatNodeText(flatFirstNamedChild(file, postfixes[i])), "!!", ""))
				if truth {
					checks = append(checks, "`assertThat("+operand+").isNotNull()`")
				} else {
					checks = append(checks, "`assertNotNull("+operand+")`")
				}
			}
			joined := strings.Join(checks, " and ")
			if truth {
				ctx.EmitAtNode(firstBang, "Avoid `!!` in "+scope+"; use "+joined+" first, then use a local val or smart-cast.")
				return
			}
			ctx.EmitAtNode(firstBang, "Avoid `!!` in "+scope+"; use "+joined+" first and smart-cast.")
		},
	})
}

func registerTestingQualityMockWithoutVerify() {
	r := &MockWithoutVerifyRule{
		BaseRule: BaseRule{RuleName: "MockWithoutVerify", RuleSetName: testingQualityRuleSet, Sev: "info", Desc: "Detects mock objects created in test functions that are never verified or stubbed."},
	}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"function_declaration"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if !testingQualityIsTestFunction(file, idx) {
				return
			}
			body, _ := file.FlatFindChild(idx, "function_body")
			if body == 0 {
				return
			}
			var mockNames []string
			var mockRows []int
			var mockCols []int
			mockInitializerStubbed := make(map[string]bool)
			file.FlatWalkAllNodes(body, func(n uint32) {
				if file.FlatType(n) != "property_declaration" {
					return
				}
				varDecl, _ := file.FlatFindChild(n, "variable_declaration")
				if varDecl == 0 {
					return
				}
				ident, _ := file.FlatFindChild(varDecl, "simple_identifier")
				if ident == 0 {
					return
				}
				for rhs := file.FlatFirstChild(n); rhs != 0; rhs = file.FlatNextSib(rhs) {
					if file.FlatType(rhs) != "call_expression" {
						continue
					}
					callName := flatCallNameAny(file, rhs)
					if testingQualityMockCreationCalls[callName] {
						name := file.FlatNodeText(ident)
						mockNames = append(mockNames, name)
						mockRows = append(mockRows, file.FlatRow(n)+1)
						mockCols = append(mockCols, file.FlatCol(n)+1)
						if testingQualityMockCreationCallHasInitializerStubbing(file, rhs) {
							mockInitializerStubbed[name] = true
						}
					}
				}
			})
			if len(mockNames) == 0 {
				return
			}
			used := mockInitializerStubbed
			file.FlatWalkAllNodes(body, func(n uint32) {
				if file.FlatType(n) != "call_expression" {
					return
				}
				callName := flatCallNameAny(file, n)
				if testingQualityMockCreationCalls[callName] {
					return
				}
				for _, name := range mockNames {
					if testingQualityMockUsageCalls[callName] {
						if subtreeHasReferenceName(file, n, name) ||
							testingQualityMockStubbingInfixUsesReference(file, n, name) {
							used[name] = true
							continue
						}
					}
					if testingQualityCallPassesReferenceAsValueArgument(file, n, name) {
						used[name] = true
					}
				}
			})
			for i, name := range mockNames {
				if !used[name] {
					ctx.EmitAt(mockRows[i], mockCols[i], fmt.Sprintf("Mock `%s` is created but never verified or stubbed.", name))
				}
			}
		},
	})
}

func registerTestingQualityRunTestWithDelay() {
	r := &RunTestWithDelayRule{
		BaseRule: BaseRule{RuleName: "RunTestWithDelay", RuleSetName: testingQualityRuleSet, Sev: "info", Desc: "Detects a lone delay() in a runTest body with no other coroutine work, where it only advances virtual time."},
	}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if flatCallExpressionName(file, idx) != "delay" {
				return
			}
			if !testingQualityIsDirectCoroutineDelayCall(file, idx) {
				return
			}
			_, args := flatCallExpressionParts(file, idx)
			if args != 0 {
				valueArgs := testingQualityValueArgumentsFlat(file, args)
				if len(valueArgs) == 1 {
					argText := strings.TrimSpace(file.FlatNodeText(valueArgs[0]))
					if argText == "0" || argText == "0L" {
						return
					}
				}
			}
			runTestLambda := testingQualityRunTestLambdaOfDelay(file, idx)
			if runTestLambda == 0 {
				return
			}
			// delay runs on virtual time here; it is a real way to let launched work run.
			if testingQualityRunTestSchedulesOtherWork(file, runTestLambda) {
				return
			}
			if testingQualityInsideMockKAnswerLambda(file, idx) {
				return
			}
			ctx.EmitAt(callLine(file, idx), callCol(file, idx), "`delay` in `runTest` only advances virtual time and the test body calls no other code; `advanceTimeBy` states the intent, or remove it.")
		},
	})
}

func registerTestingQualityRunTestWithThreadSleep() {
	r := &RunTestWithThreadSleepRule{
		BaseRule: BaseRule{RuleName: "RunTestWithThreadSleep", RuleSetName: testingQualityRuleSet, Sev: "warning", Desc: "Detects Thread.sleep() calls inside runTest blocks where advanceTimeBy should be used instead."},
	}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			name := flatCallExpressionName(file, idx)
			if name != "sleep" {
				return
			}
			navExpr, _ := flatCallExpressionParts(file, idx)
			if navExpr == 0 {
				return
			}
			receiver := file.FlatNamedChild(navExpr, 0)
			if receiver == 0 || !file.FlatNodeTextEquals(receiver, "Thread") {
				return
			}
			if !testingQualityInsideRunTest(file, idx) || !testingQualityDelayInRunTestBody(file, idx) {
				return
			}
			if testingQualitySleepInPollingLoop(file, idx) || testingQualityTestUsesRealConcurrency(file, idx) {
				return
			}
			ctx.EmitAt(callLine(file, idx), callCol(file, idx), "Use `advanceTimeBy` instead of `Thread.sleep` inside `runTest`.")
		},
	})
}

func registerTestingQualityRunBlockingInTest() {
	r := &RunBlockingInTestRule{
		BaseRule: BaseRule{RuleName: "RunBlockingInTest", RuleSetName: testingQualityRuleSet, Sev: "info", Desc: "Detects runBlocking usage in test functions where runTest provides better coroutine test support."},
	}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if flatCallExpressionName(file, idx) != "runBlocking" {
				return
			}
			fn, ok := flatEnclosingFunction(file, idx)
			if !ok || !testingQualityIsTestFunction(file, fn) {
				return
			}
			if testingQualityIsAndroidInstrumentedTestFile(file) {
				return
			}
			if testingQualityInsideAssertionOrCallbackBoundary(file, idx, fn) {
				return
			}
			if testingQualityRunBlockingTestsDispatcherThreadIdentity(file, idx) {
				return
			}
			if testingQualityClassUsesRealDispatchers(file, fn, r.RealDispatcherMarkers) {
				return
			}
			if testingQualityRunBlockingInRxJavaBridgeTest(file, idx, fn) || testingQualityRunBlockingWithRxTestScheduler(file, fn) {
				return
			}
			if testingQualityRunBlockingResultIsAsserted(file, idx, fn) {
				return
			}
			if testingQualityRunBlockingContainsTurbineAssertion(file, idx) {
				return
			}
			if testingQualityRunBlockingHasIntentionalComment(file, idx, fn) {
				return
			}
			// A test that steps its own scheduler only wraps a fake call in runBlocking; runTest would not replace it.
			if !testingQualityInsideRunTest(file, idx) && testingQualityFileDrivesOwnScheduler(file) &&
				!testingQualityFunctionIsSuspend(file, fn) && !testingQualityRunBlockingIsWholeTestBody(file, idx, fn) {
				return
			}
			// A runBlocking inside a larger test that does not use kotlinx-coroutines-test has no
			// drop-in runTest: switching means restructuring the whole test.
			if !testingQualityInsideRunTest(file, idx) && !testingQualityFileIsCoroutineTest(file) &&
				!testingQualityRunBlockingIsWholeTestBody(file, idx, fn) {
				return
			}
			msg := "Use `runTest` instead of `runBlocking` in test functions."
			if testingQualityInsideRunTest(file, idx) {
				msg = "Remove the nested `runBlocking`; the test already runs its body in `runTest`."
			}
			ctx.EmitAt(callLine(file, idx), callCol(file, idx), msg)
		},
	})
}

func registerTestingQualityTestDispatcherNotInjected() {
	r := &TestDispatcherNotInjectedRule{
		BaseRule: BaseRule{RuleName: "TestDispatcherNotInjected", RuleSetName: testingQualityRuleSet, Sev: "info", Desc: "Detects production dispatchers (Dispatchers.IO, Default, Main) used directly in test functions."},
	}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"navigation_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			segments := flatNavigationChainIdentifiers(file, idx)
			if len(segments) != 2 || segments[0] != "Dispatchers" ||
				(segments[1] != "IO" && segments[1] != "Default" && segments[1] != "Main") {
				return
			}
			if testingQualityDispatcherReferenceAllowedInTest(file, idx, segments[1]) {
				return
			}
			if !testingQualityDispatcherPassedToProductionCode(file, idx) {
				return
			}
			fn, ok := flatEnclosingFunction(file, idx)
			if !ok || !testingQualityIsTestFunction(file, fn) {
				return
			}
			ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Avoid production dispatchers in tests; inject a `TestDispatcher`.")
		},
	})
}

func registerTestingQualityTestWithoutAssertion() {
	r := &TestWithoutAssertionRule{
		BaseRule: BaseRule{RuleName: "TestWithoutAssertion", RuleSetName: testingQualityRuleSet, Sev: "warning", Desc: "Detects @Test functions that contain no assertion or verification calls."},
	}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		// Cross-file so assertion helpers declared in other test-support files are visible.
		Needs: api.NeedsCrossFile, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: r.check,
	})
}

// testingQualityBodyIsOnlyTodoOrFail reports a block body whose single statement is a `TODO(...)` call or
// `throw NotImplementedError(...)`. Comments are ignored. Any other statement, or a `fail(...)`/`TODO()` nested
// in a branch or lambda, means the test has real content.
func testingQualityBodyIsOnlyTodoOrFail(file *scanner.File, body uint32) bool {
	var stmt uint32
	count := 0
	var collect func(parent uint32)
	collect = func(parent uint32) {
		for child := file.FlatFirstChild(parent); child != 0; child = file.FlatNextSib(child) {
			if !file.FlatIsNamed(child) {
				continue
			}
			switch file.FlatType(child) {
			case "line_comment", "multiline_comment", "block_comment", "comment":
				continue
			case "statements":
				collect(child)
				continue
			}
			stmt = child
			count++
		}
	}
	collect(body)
	if count != 1 {
		return false
	}
	switch file.FlatType(stmt) {
	case "call_expression":
		return testingQualityIsPlainCall(file, stmt, "TODO")
	case "jump_expression":
		if !strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(stmt)), "throw") {
			return false
		}
		for child := file.FlatFirstChild(stmt); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "call_expression" {
				return testingQualityIsPlainCall(file, child, "NotImplementedError")
			}
		}
	}
	return false
}

// testingQualityIsPlainCall reports `name(...)` with no receiver and no trailing lambda.
func testingQualityIsPlainCall(file *scanner.File, call uint32, name string) bool {
	if flatCallExpressionName(file, call) != name {
		return false
	}
	first := file.FlatFirstChild(call)
	if first == 0 || file.FlatType(first) != "simple_identifier" {
		return false
	}
	return !flatSubtreeHasNodeType(file, call, "lambda_literal")
}

func registerTestingQualityTestWithOnlyTodo() {
	r := &TestWithOnlyTodoRule{
		BaseRule: BaseRule{RuleName: "TestWithOnlyTodo", RuleSetName: testingQualityRuleSet, Sev: "warning", Desc: "Detects @Test functions whose body is only a TODO() call without @Ignore."},
	}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"function_declaration"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if !testingQualityIsTestFunction(file, idx) {
				return
			}
			if testingQualityIsIgnoredTest(file, idx) {
				return
			}
			body, _ := file.FlatFindChild(idx, "function_body")
			if body == 0 {
				return
			}
			if !testingQualityBodyIsOnlyTodoOrFail(file, body) {
				return
			}
			ctx.EmitAt(declLine(file, idx), declCol(file, idx), "Test with only `TODO()`; annotate with `@Ignore` or implement it.")
		},
	})
}

func registerTestingQualityTestFunctionReturnValue() {
	r := &TestFunctionReturnValueRule{
		BaseRule: BaseRule{RuleName: "TestFunctionReturnValue", RuleSetName: testingQualityRuleSet, Sev: "warning", Desc: "Detects @Test functions that return a non-Unit type, since JUnit ignores return values."},
	}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"function_declaration"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if !testingQualityIsTestFunction(file, idx) {
				return
			}
			retType := testingQualityReturnType(file, idx)
			if retType == "" || testingQualityAllowedTestReturnType(retType) {
				return
			}
			ctx.EmitAt(declLine(file, idx), declCol(file, idx), "Test functions should return `Unit`; JUnit ignores return values.")
		},
	})
}

func registerTestingQualityTestNameContainsUnderscore() {
	r := &TestNameContainsUnderscoreRule{
		BaseRule: BaseRule{RuleName: "TestNameContainsUnderscore", RuleSetName: testingQualityRuleSet, Sev: "info", Desc: "Detects test function names using underscores where backtick-quoted names are preferred."},
	}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"function_declaration"}, Confidence: api.ConfidenceMediumLow, Fix: api.FixCosmetic, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if !testingQualityIsTestFunction(file, idx) {
				return
			}
			// Instrumented tests run through dex, where backticked names with spaces are not allowed before API 30.
			if testingQualityIsAndroidInstrumentedTestFile(file) {
				return
			}
			name := testingQualityFunctionName(file, idx)
			if name == "" || !strings.Contains(name, "_") {
				return
			}
			if strings.HasPrefix(name, "`") {
				return
			}
			f := r.Finding(file, declLine(file, idx), declCol(file, idx),
				"Test name uses underscores; consider backtick-quoted names.")
			if fix := testNameUnderscoreFix(file, idx, name); fix != nil {
				f.Fix = fix
			}
			ctx.Emit(f)
		},
	})
}

// testNameUnderscoreFix returns a byte-mode Fix that replaces the
// function's simple_identifier child with the same name rewritten as a
// backtick-quoted identifier with underscores swapped for spaces. Returns
// nil when the identifier byte range cannot be located.
func testNameUnderscoreFix(file *scanner.File, fnIdx uint32, name string) *scanner.Fix {
	for child := file.FlatFirstChild(fnIdx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "simple_identifier", "identifier":
			replacement := "`" + strings.ReplaceAll(name, "_", " ") + "`"
			return &scanner.Fix{
				ByteMode:    true,
				StartByte:   int(file.FlatStartByte(child)),
				EndByte:     int(file.FlatEndByte(child)),
				Replacement: replacement,
			}
		}
	}
	return nil
}

func registerTestingQualitySharedMutableStateInObject() {
	r := &SharedMutableStateInObjectRule{
		BaseRule: BaseRule{RuleName: "SharedMutableStateInObject", RuleSetName: testingQualityRuleSet, Sev: "warning", Desc: "Detects mutable var properties in companion objects or object declarations shared across tests."},
	}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"companion_object", "object_declaration"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if !testingQualityIsTestFile(file) {
				return
			}
			if sharedMutableStateObjectIsTestSupport(file, idx) {
				return
			}
			for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
				if file.FlatType(child) == "class_body" {
					for member := file.FlatFirstChild(child); member != 0; member = file.FlatNextSib(member) {
						if file.FlatType(member) != "property_declaration" {
							continue
						}
						if hasAnnotationFlat(file, member, "VisibleForTesting") {
							continue
						}
						if propertyDeclarationIsVar(file, member) {
							name := extractIdentifierFlat(file, member)
							if mutableObjectVarResetByLifecycle(file, idx, member, name) ||
								(file.FlatHasModifier(member, "private") && mutableObjectVarOnlyWrittenByTestHooks(file, idx, member, name)) {
								continue
							}
							ctx.EmitAtNode(declAnchor(file, member), "Mutable state '"+name+"' in companion/object is shared across tests.")
						}
					}
				}
			}
		},
	})
}

func registerTestingQualityTestInheritanceDepth() {
	r := &TestInheritanceDepthRule{
		BaseRule: BaseRule{RuleName: "TestInheritanceDepth", RuleSetName: testingQualityRuleSet, Sev: "info", Desc: "Detects test class inheritance hierarchies deeper than two levels that should be flattened."},
	}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"class_declaration"}, Confidence: api.ConfidenceMediumLow, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if !testingQualityIsTestFile(file) {
				return
			}
			delegation, _ := file.FlatFindChild(idx, "delegation_specifier")
			if delegation == 0 {
				return
			}
			supertypes := testingQualitySuperclasses(file, idx)
			if len(supertypes) == 0 {
				return
			}
			depth := 1
			for _, st := range supertypes {
				d := testingQualityCountInheritanceInFile(file, st, 1)
				if d > depth {
					depth = d
				}
			}
			if depth < 2 {
				return
			}
			ctx.EmitAt(declLine(file, idx), declCol(file, idx), "Test class inheritance depth exceeds 2; flatten the hierarchy.")
		},
	})
}

// registerTestingQualityRelaxedMockUsedForValueClass lives in testing_quality_relaxed_mock.go.

func registerTestingQualitySpyOnDataClass() {
	r := &SpyOnDataClassRule{
		BaseRule: BaseRule{RuleName: "SpyOnDataClass", RuleSetName: testingQualityRuleSet, Sev: "info", Desc: "Detects spying on data class instances where value-based equality breaks spy semantics."},
	}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMediumLow, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			name := flatCallNameAny(file, idx)
			if name != "spyk" && name != "spy" {
				return
			}
			_, args := flatCallExpressionParts(file, idx)
			if args == 0 {
				return
			}
			valueArgs := testingQualityValueArgumentsFlat(file, args)
			if len(valueArgs) == 0 {
				return
			}
			argExpr := flatValueArgumentExpression(file, valueArgs[0])
			if argExpr == 0 {
				return
			}
			if file.FlatType(argExpr) != "call_expression" {
				return
			}
			ctx.EmitAt(callLine(file, idx), callCol(file, idx), "Avoid spying on data classes; value-based equality breaks spy semantics.")
		},
	})
}

func registerTestingQualityVerifyWithoutMock() {
	r := &VerifyWithoutMockRule{
		BaseRule: BaseRule{RuleName: "VerifyWithoutMock", RuleSetName: testingQualityRuleSet, Sev: "warning", Desc: "Detects verify or coVerify calls on objects that are not declared as mocks in the test."},
	}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMediumLow, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if !testingQualityRuleShouldRunInFile(file) {
				return
			}
			name := flatCallNameAny(file, idx)
			if !testingQualityIsMockKVerifyCall(file, idx, name) {
				return
			}
			lambda := flatCallTrailingLambda(file, idx)
			if lambda == 0 {
				return
			}
			receivers := testingQualityDirectVerifyReceivers(file, lambda)
			if len(receivers) == 0 {
				return
			}
			fn, ok := flatEnclosingFunction(file, idx)
			if !ok {
				return
			}
			if !testingQualityIsTestLikeFunction(file, fn) {
				return
			}
			helperFuncs := testingQualityMockHelperFunctions(file)
			mockVars := testingQualityCollectMockNames(file, fn, helperFuncs)
			for _, recv := range receivers {
				if !testingQualityVerifyReceiverKnownMock(mockVars, recv) {
					ctx.EmitAt(callLine(file, idx), callCol(file, idx), fmt.Sprintf("Calling `verify` on a non-mock object; ensure `%s` is a mock.", recv.display))
				}
			}
		},
	})
}

// testingQualityOperandAssertedNonNull reports an earlier non-null assertion on the operand of the `!!` at bang,
// in the same function: `assertThat(x).isNotNull()`, `assertNotNull(x)` or `requireNotNull(x)`.
func testingQualityOperandAssertedNonNull(file *scanner.File, call, bang uint32) bool {
	operand := flatFirstNamedChild(file, bang)
	if operand == 0 {
		return false
	}
	text := strings.Join(strings.Fields(file.FlatNodeText(operand)), "")
	if text == "" {
		return false
	}
	scopeStart := uint32(0)
	if fn, ok := flatEnclosingFunction(file, call); ok {
		scopeStart = file.FlatStartByte(fn)
	}
	callStart := file.FlatStartByte(call)
	if scopeStart >= callStart || int(callStart) > len(file.Content) {
		return false
	}
	before := strings.Join(strings.Fields(string(file.Content[scopeStart:callStart])), "")
	quoted := regexp.QuoteMeta(text)
	pattern := regexp.MustCompile(`assertThat\(` + quoted + `\)\.isNotNull\(\)|` +
		`(?:assertNotNull|requireNotNull|checkNotNull)\((?:[^,()]*,)?` + quoted + `[,)]|` +
		`assertThat\(` + quoted + `,notNullValue\(\)\)`)
	return pattern.MatchString(before)
}

// testingQualityBodyHasExpectOrAwaitCall reports Turbine and navigator style checks (`expectNoEvents()`,
// `awaitItem()`, `expectPop()`) that fail the test when the expectation is not met.
func testingQualityBodyHasExpectOrAwaitCall(file *scanner.File, body uint32) bool {
	found := false
	file.FlatWalkNodes(body, "call_expression", func(call uint32) {
		if found {
			return
		}
		name := flatCallNameAny(file, call)
		// A function declared in this file is a lookalike unless its body asserts (handled elsewhere).
		if name == "expectError" || ignoredReturnValueFileDeclaresFunction(file, name) {
			return
		}
		found = testingQualityTurbineAssertionCalls[name] ||
			testingQualityHasLowerPrefixWord(name, "expect") ||
			testingQualityHasLowerPrefixWord(name, "await")
	})
	return found
}

func testingQualityHasLowerPrefixWord(name, prefix string) bool {
	if !strings.HasPrefix(name, prefix) {
		return false
	}
	rest := name[len(prefix):]
	return rest == "" || (rest[0] >= 'A' && rest[0] <= 'Z')
}

// sharedMutableStateObjectIsTestSupport reports whether the object is a
// fake, stub or other test-support helper whose mutable state is deliberate.
// Companion objects are judged only by annotation, since the name of the
// enclosing test class says nothing about them.
func sharedMutableStateObjectIsTestSupport(file *scanner.File, idx uint32) bool {
	if hasAnnotationFlat(file, idx, "VisibleForTesting") {
		return true
	}
	if file.FlatType(idx) != "object_declaration" || file.FlatHasModifier(idx, "companion") {
		return false
	}
	name := extractIdentifierFlat(file, idx)
	for _, marker := range []string{"Fake", "Stub", "Dummy", "Fixture"} {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return strings.HasPrefix(name, "Test") || strings.HasSuffix(name, "Test")
}

// testingQualityAllowedTestReturnType reports return types a test runner accepts:
// Unit, and kotlinx-coroutines-test's TestResult that multiplatform runTest tests return.
func testingQualityAllowedTestReturnType(retType string) bool {
	switch retType {
	case "Unit", "kotlin.Unit", "TestResult", "kotlinx.coroutines.test.TestResult":
		return true
	}
	return false
}

// testingQualityCallIsWithin reports whether the nearest enclosing call of n lies inside root.
func testingQualityCallIsWithin(file *scanner.File, n, root uint32) bool {
	outer, ok := flatEnclosingAncestor(file, n, "call_expression")
	return ok && file.FlatStartByte(outer) >= file.FlatStartByte(root)
}

// testingQualityBodyPlantsFailure reports a `throw`, `error(...)` or `fail(...)` anywhere in the body, such as in
// a stub observer or a lazily evaluated lambda. The test passes only when that code is never reached.
func testingQualityBodyPlantsFailure(file *scanner.File, body uint32) bool {
	found := false
	file.FlatWalkAllNodes(body, func(n uint32) {
		if found {
			return
		}
		switch file.FlatType(n) {
		case "jump_expression":
			found = strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(n)), "throw")
		case "call_expression":
			if name := flatCallExpressionName(file, n); name == "error" || name == "fail" {
				found = true
			}
		}
	})
	return found
}
