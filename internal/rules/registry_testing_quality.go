package rules

import (
	"fmt"
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
			if len(valueArgs) != 2 {
				return
			}
			firstArg := apiNodeNameFlat(file, flatValueArgumentExpression(file, valueArgs[0]))
			secondArg := apiNodeNameFlat(file, flatValueArgumentExpression(file, valueArgs[1]))
			if firstArg != "actual" || secondArg != "expected" {
				return
			}
			ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "`assertEquals` arguments appear reversed; use (expected, actual).")
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
			if op == 0 || !file.FlatNodeTextEquals(op, "==") {
				return
			}
			ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Use `assertEquals` instead of `assertTrue` for equality comparisons.")
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
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if !testingQualityRuleShouldRunInFile(file) {
				return
			}
			name := flatCallExpressionName(file, idx)
			if !testingQualityIsNullableAssertionCall(name) {
				return
			}
			_, args := flatCallExpressionParts(file, idx)
			if args == 0 {
				return
			}
			found := false
			file.FlatWalkAllNodes(args, func(n uint32) {
				if found {
					return
				}
				if testingQualityInsideNestedExecutableArgument(file, n, args) {
					return
				}
				if file.FlatType(n) == "postfix_expression" {
					if postfixExpressionHasBangBang(file, n) {
						found = true
					}
				}
			})
			if !found {
				return
			}
			ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Avoid `!!` in assertions; use `assertNotNull` first.")
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
		BaseRule: BaseRule{RuleName: "RunTestWithDelay", RuleSetName: testingQualityRuleSet, Sev: "warning", Desc: "Detects delay() calls inside runTest blocks where advanceTimeBy should be used instead."},
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
			if !testingQualityInsideRunTest(file, idx) {
				return
			}
			if testingQualityInsideMockKAnswerLambda(file, idx) {
				return
			}
			ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Use `advanceTimeBy` instead of `delay` inside `runTest`.")
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
			if !testingQualityInsideRunTest(file, idx) {
				return
			}
			ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Use `advanceTimeBy` instead of `Thread.sleep` inside `runTest`.")
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
			if testingQualityRunBlockingInRxJavaBridgeTest(file, idx, fn) {
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
			ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Use `runTest` instead of `runBlocking` in test functions.")
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
		NodeTypes: []string{"function_declaration"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if testingQualityIsEditorTemplatePath(file.Path) {
				return
			}
			if !testingQualityIsTestFunction(file, idx) {
				return
			}
			if r.AllowNoAssertionTests {
				return
			}
			if testingQualityTestExpectsException(file, idx) {
				return
			}
			if testingQualityIsIgnoredTest(file, idx) {
				return
			}
			if testingQualityTestAllowsNoCrash(file, idx) {
				return
			}
			if testingQualityIsBenchmarkOrGoldenFile(file) {
				return
			}
			if testingQualityTestNameDocumentsNoException(file, idx) {
				return
			}
			body, _ := file.FlatFindChild(idx, "function_body")
			if body == 0 {
				return
			}
			if testingQualityTextHasVerifyCall(file.FlatNodeText(idx)) {
				return
			}
			if testingQualityBodyHasAssertionOrVerificationWithPatterns(file, body, r.AssertionMethodPatterns) {
				return
			}
			ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Test function has no assertion; add a verification.")
		},
	})
}

func registerTestingQualityTestWithOnlyTodo() {
	r := &TestWithOnlyTodoRule{
		BaseRule: BaseRule{RuleName: "TestWithOnlyTodo", RuleSetName: testingQualityRuleSet, Sev: "warning", Desc: "Detects @Test functions whose body is only a TODO() or fail() call without @Ignore."},
	}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes: []string{"function_declaration"}, Confidence: api.ConfidenceMedium, Implementation: r,
		Check: func(ctx *api.Context) {
			idx, file := ctx.Idx, ctx.File
			if !testingQualityIsTestFunction(file, idx) {
				return
			}
			if flatHasAnnotationNamed(file, idx, "Ignore") || flatHasAnnotationNamed(file, idx, "Disabled") {
				return
			}
			body, _ := file.FlatFindChild(idx, "function_body")
			if body == 0 {
				return
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
				return
			}
			ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Test with only `TODO()`; annotate with `@Ignore` or implement it.")
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
			if retType == "" || retType == "Unit" {
				return
			}
			ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Test functions should return `Unit`; JUnit ignores return values.")
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
			name := testingQualityFunctionName(file, idx)
			if name == "" || !strings.Contains(name, "_") {
				return
			}
			if strings.HasPrefix(name, "`") {
				return
			}
			f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
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
			for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
				if file.FlatType(child) == "class_body" {
					for member := file.FlatFirstChild(child); member != 0; member = file.FlatNextSib(member) {
						if file.FlatType(member) != "property_declaration" {
							continue
						}
						if propertyDeclarationIsVar(file, member) {
							ctx.EmitAt(file.FlatRow(member)+1, file.FlatCol(member)+1, "Mutable state in companion/object is shared across tests.")
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
			supertypes := testingQualitySupertypes(file, idx)
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
			ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Test class inheritance depth exceeds 2; flatten the hierarchy.")
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
			ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Avoid spying on data classes; value-based equality breaks spy semantics.")
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
					ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, fmt.Sprintf("Calling `verify` on a non-mock object; ensure `%s` is a mock.", recv.display))
				}
			}
		},
	})
}
