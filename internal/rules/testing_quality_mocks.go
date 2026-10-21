package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	"github.com/kaeawc/krit/internal/scanner"
)

var testingQualityMockCreationCalls = map[string]bool{
	"mockk":        true,
	"relaxedMockk": true,
	"mockkClass":   true,
	"spyk":         true,
	"mock":         true,
	"spy":          true,
}

var testingQualityMockUsageCalls = map[string]bool{
	"verify":          true,
	"coVerify":        true,
	"every":           true,
	"coEvery":         true,
	"whenever":        true,
	"when":            true,
	"given":           true,
	"confirmVerified": true,
}

var testingQualityObjectMockCreationCalls = map[string]bool{
	"mockkObject":      true,
	"mockkStatic":      true,
	"mockkConstructor": true,
}

func testingQualityCallPassesReferenceAsValueArgument(file *scanner.File, call uint32, name string) bool {
	if file == nil || call == 0 || name == "" {
		return false
	}
	_, args := flatCallExpressionParts(file, call)
	if args == 0 {
		return false
	}
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		found := false
		file.FlatWalkAllNodes(arg, func(n uint32) {
			if found {
				return
			}
			if file.FlatType(n) == "simple_identifier" && file.FlatNodeTextEquals(n, name) {
				found = true
			}
		})
		if found {
			return true
		}
	}
	return false
}

func testingQualityMockStubbingInfixUsesReference(file *scanner.File, call uint32, name string) bool {
	if file == nil || call == 0 || name == "" {
		return false
	}
	for parent, ok := file.FlatParent(call); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "infix_expression":
			if testingQualityIsMockStubbingInfix(file, parent) && subtreeHasReferenceName(file, parent, name) {
				return true
			}
		case "function_body", "lambda_literal", "class_body", "object_declaration", "function_declaration":
			return false
		}
	}
	return false
}

func testingQualityIsMockStubbingInfix(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "infix_expression" {
		return false
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "simple_identifier" {
			continue
		}
		switch file.FlatNodeText(child) {
		case "returns", "returnsMany", "andThen", "throws", "answers", "coAnswers":
			return true
		}
	}
	return false
}

func testingQualityMockCreationCallHasInitializerStubbing(file *scanner.File, call uint32) bool {
	if file == nil || call == 0 {
		return false
	}
	lambda := flatCallTrailingLambda(file, call)
	if lambda == 0 {
		return false
	}
	found := false
	file.FlatWalkAllNodes(lambda, func(n uint32) {
		if found || file.FlatType(n) != "call_expression" {
			return
		}
		found = testingQualityMockUsageCalls[flatCallNameAny(file, n)]
	})
	return found
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

type testingQualityVerifyReceiver struct {
	display    string
	candidates []string
}

func testingQualityDirectVerifyReceivers(file *scanner.File, lambda uint32) []testingQualityVerifyReceiver {
	if file == nil || lambda == 0 {
		return nil
	}
	var receivers []testingQualityVerifyReceiver
	addReceiver := func(stmt uint32) {
		if stmt == 0 || file.FlatType(stmt) != "call_expression" {
			return
		}
		name := flatCallNameAny(file, stmt)
		if testingQualityIsAssertionCall(name) {
			return
		}
		if testingQualityVerifyWrapperCall(name) {
			if nested := flatCallTrailingLambda(file, stmt); nested != 0 {
				receivers = append(receivers, testingQualityDirectVerifyReceivers(file, nested)...)
			}
			return
		}
		if receiver := testingQualityVerifyReceiverFromCall(file, stmt); receiver.display != "" {
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

func testingQualityVerifyWrapperCall(name string) bool {
	switch name {
	case "forEach", "forEachIndexed", "repeat":
		return true
	default:
		return false
	}
}

func testingQualityVerifyReceiverFromCall(file *scanner.File, call uint32) testingQualityVerifyReceiver {
	leaf := flatReceiverNameFromCall(file, call)
	if leaf == "" {
		return testingQualityVerifyReceiver{}
	}
	receiver := testingQualityVerifyReceiver{
		display:    leaf,
		candidates: []string{leaf},
	}
	nav, _ := flatCallExpressionParts(file, call)
	segments := flatNavigationChainIdentifiers(file, nav)
	if len(segments) > 1 && segments[len(segments)-1] == flatCallNameAny(file, call) {
		segments = segments[:len(segments)-1]
	}
	if len(segments) > 0 && segments[0] != "" && segments[0] != leaf {
		receiver.candidates = append(receiver.candidates, segments[0])
	}
	return receiver
}

func testingQualityVerifyReceiverKnownMock(mockVars map[string]bool, receiver testingQualityVerifyReceiver) bool {
	for _, candidate := range receiver.candidates {
		if mockVars[candidate] || testingQualityNameLooksMockReference(candidate) {
			return true
		}
	}
	return false
}

func testingQualityNameLooksMockReference(name string) bool {
	return testingQualityNameContainsTokenFold(name, "mock") ||
		testingQualityNameContainsTokenFold(name, "spy")
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
	if testingQualityCallCreatesMock(file, idx, helperFuncs) {
		return true
	}
	if !testingQualityScopeFunctionReturnsReceiver(flatCallNameAny(file, idx)) {
		return false
	}
	return testingQualityExprCreatesMock(file, testingQualityCallReceiverExpression(file, idx), helperFuncs)
}

func testingQualityScopeFunctionReturnsReceiver(name string) bool {
	return name == "apply" || name == "also"
}

func testingQualityCallReceiverExpression(file *scanner.File, call uint32) uint32 {
	nav, _ := flatCallExpressionParts(file, call)
	return testingQualityNavigationReceiverExpression(file, nav)
}

func testingQualityNavigationReceiverExpression(file *scanner.File, nav uint32) uint32 {
	if file == nil || nav == 0 || file.FlatType(nav) != "navigation_expression" {
		return 0
	}
	for child := file.FlatFirstChild(nav); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		if file.FlatType(child) == "navigation_expression" {
			return testingQualityNavigationReceiverExpression(file, child)
		}
		return child
	}
	return 0
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
		return testingQualityFirstSimpleIdentifier(file, expr)
	}
	if file.FlatType(expr) != "string_literal" {
		return testingQualityFirstSimpleIdentifier(file, expr)
	}
	return ""
}

func testingQualityFirstSimpleIdentifier(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	found := ""
	file.FlatWalkAllNodes(idx, func(n uint32) {
		if found != "" {
			return
		}
		switch file.FlatType(n) {
		case "simple_identifier", "type_identifier":
			found = file.FlatNodeText(n)
		}
	})
	return found
}

func testingQualityMockHelperFunctions(file *scanner.File) map[string]bool {
	if file == nil {
		return map[string]bool{}
	}
	return filefacts.FileFact(fileFactsCache(), file, slotTestQualityMockHelpers, func() map[string]bool {
		helpers := make(map[string]bool)
		file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
			name := testingQualityFunctionName(file, fn)
			if name == "" {
				return
			}
			mockVars := make(map[string]bool)
			if body, ok := file.FlatFindChild(fn, "function_body"); ok {
				if expr := testingQualityFunctionBodyExpression(file, body); testingQualityExprCreatesMock(file, expr, helpers) {
					helpers[name] = true
					return
				}
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
		return helpers
	})
}

func testingQualityFunctionBodyExpression(file *scanner.File, body uint32) uint32 {
	if file == nil || body == 0 {
		return 0
	}
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		if file.FlatType(child) == "statements" {
			return 0
		}
		return child
	}
	return 0
}

func testingQualityCollectMockNames(file *scanner.File, fn uint32, helperFuncs map[string]bool) map[string]bool {
	if file == nil {
		return map[string]bool{}
	}
	return filefacts.NodeFact(fileFactsCache(), file, fn, slotTestQualityMockNames, func() map[string]bool {
		return testingQualityComputeMockNames(file, fn, helperFuncs)
	})
}

func testingQualityComputeMockNames(file *scanner.File, fn uint32, helperFuncs map[string]bool) map[string]bool {
	mockVars := make(map[string]bool)
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
	return mockVars
}
