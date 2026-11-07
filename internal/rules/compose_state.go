package rules

import (
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// ComposeDerivedStateMisuseRule detects derivedStateOf around direct boolean
// comparisons of Compose state reads. Those reads already trigger
// recomposition, so wrapping them in derivedStateOf usually adds overhead
// without reducing updates.
type ComposeDerivedStateMisuseRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Compose rule. Detection relies on call-name token matching (e.g.
// 'verticalScroll', 'LazyColumn', 'rememberSaveable') rather than type
// resolution, so any project symbol with a matching name or token can
// produce a false match. Classified per roadmap/17.
func (r *ComposeDerivedStateMisuseRule) Confidence() float64 { return api.ConfidenceMedium }

func composeDerivedStateBodyFlat(file *scanner.File, idx uint32) uint32 {
	callSuffix, _ := file.FlatFindChild(idx, "call_suffix")
	if callSuffix == 0 {
		return 0
	}
	annotatedLambda, _ := file.FlatFindChild(callSuffix, "annotated_lambda")
	if annotatedLambda == 0 {
		return 0
	}
	lambda, _ := file.FlatFindChild(annotatedLambda, "lambda_literal")
	if lambda == 0 {
		return 0
	}
	statements, _ := file.FlatFindChild(lambda, "statements")
	if statements == 0 || file.FlatNamedChildCount(statements) != 1 {
		return 0
	}
	return file.FlatNamedChild(statements, 0)
}

func composeStateBindingsInScopeFlat(file *scanner.File, idx uint32) (map[string]bool, map[string]bool) {
	currentFn := composeNearestAncestorFlat(file, idx, "function_declaration")
	delegatedReads := make(map[string]bool)
	stateHolders := make(map[string]bool)

	if currentFn != 0 {
		params, _ := file.FlatFindChild(currentFn, "function_value_parameters")
		if params != 0 {
			for param := file.FlatFirstChild(params); param != 0; param = file.FlatNextSib(param) {
				if !file.FlatIsNamed(param) || file.FlatType(param) != "parameter" {
					continue
				}
				if name := composeParameterNameFlat(file, param); name != "" && composeParameterHasStateTypeFlat(file, param) {
					stateHolders[name] = true
				}
			}
		}
	}

	file.FlatWalkNodes(0, "property_declaration", func(prop uint32) {
		if prop == 0 || file.FlatStartByte(prop) >= file.FlatStartByte(idx) {
			return
		}
		if composeNearestAncestorFlat(file, prop, "function_declaration") != currentFn {
			return
		}

		name := composePropertyNameFlat(file, prop)
		if name == "" {
			return
		}

		hasStateFactory := composeContainsStateFactoryFlat(file, prop)
		if composePropertyHasStateTypeFlat(file, prop) || hasStateFactory {
			stateHolders[name] = true
		}
		if _, hasDelegate := file.FlatFindChild(prop, "property_delegate"); hasDelegate && hasStateFactory {
			delegatedReads[name] = true
		}
	})

	return delegatedReads, stateHolders
}

func composeIsDirectStateComparisonFlat(file *scanner.File, expr uint32, delegatedReads, stateHolders map[string]bool) bool {
	if expr == 0 || file.FlatNamedChildCount(expr) != 2 {
		return false
	}

	stateReads := 0
	for child := file.FlatFirstChild(expr); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		if composeIsStateReadOperandFlat(file, child, delegatedReads, stateHolders) {
			stateReads++
		}
	}
	return stateReads == 1
}

// composeBooleanStateDeclRe matches declarations of Boolean-valued state.
var composeBooleanStateDeclRe = regexp.MustCompile(`mutableStateOf\(\s*(true|false)\b|\bBoolean\b`)

// composeStateIsBooleanFlat reports whether the local state binding name holds a Boolean.
func composeStateIsBooleanFlat(file *scanner.File, idx uint32, name string) bool {
	return composeBooleanStateDeclRe.MatchString(composeStateDeclarationTextFlat(file, idx, name))
}

// composeRememberKeyedOnFlat reports whether an enclosing remember call is keyed on the value of stateName.
// A key that is only the State holder never changes, so it does not make derivedStateOf redundant.
func composeRememberKeyedOnFlat(file *scanner.File, idx uint32, stateName string, delegatedReads map[string]bool) bool {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		if file.FlatType(cur) != "call_expression" {
			continue
		}
		// With a trailing lambda the callee sits on the nested call.
		name := flatCallExpressionName(file, cur)
		if inner, ok := file.FlatFindChild(cur, "call_expression"); name == "" && ok {
			name = flatCallExpressionName(file, inner)
		}
		if name != "remember" {
			continue
		}
		args := flatCallKeyArguments(file, cur)
		if args == 0 {
			continue
		}
		found := false
		file.FlatWalkNodes(args, "simple_identifier", func(ident uint32) {
			if file.FlatNodeTextEquals(ident, stateName) && (delegatedReads[stateName] || composeIdentifierIsValueRead(file, ident)) {
				found = true
			}
		})
		if found {
			return true
		}
	}
	return false
}

// composeIdentifierIsValueRead reports whether ident is the receiver of a `.value` read.
func composeIdentifierIsValueRead(file *scanner.File, ident uint32) bool {
	parent, ok := file.FlatParent(ident)
	if !ok || file.FlatType(parent) != "navigation_expression" {
		return false
	}
	suffix, ok := file.FlatFindChild(parent, "navigation_suffix")
	return ok && strings.TrimSpace(strings.TrimPrefix(file.FlatNodeText(suffix), ".")) == "value"
}

// composeComparedStateName returns the state binding read by a direct comparison.
func composeComparedStateName(file *scanner.File, expr uint32, delegatedReads, stateHolders map[string]bool) string {
	for child := file.FlatFirstChild(expr); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) || !composeIsStateReadOperandFlat(file, child, delegatedReads, stateHolders) {
			continue
		}
		node := composeUnwrapParenthesizedFlat(file, child)
		if file.FlatType(node) == "navigation_expression" {
			return composeNavigationReceiverFlat(file, node)
		}
		return file.FlatNodeText(node)
	}
	return ""
}

// composeStateDeclarationTextFlat returns the source of the local property or parameter that declares name.
func composeStateDeclarationTextFlat(file *scanner.File, idx uint32, name string) string {
	currentFn := composeNearestAncestorFlat(file, idx, "function_declaration")
	text := ""
	if currentFn != 0 {
		if params, _ := file.FlatFindChild(currentFn, "function_value_parameters"); params != 0 {
			for param := file.FlatFirstChild(params); param != 0; param = file.FlatNextSib(param) {
				if file.FlatType(param) == "parameter" && composeParameterNameFlat(file, param) == name {
					text = file.FlatNodeText(param)
				}
			}
		}
	}
	file.FlatWalkNodes(0, "property_declaration", func(prop uint32) {
		if file.FlatStartByte(prop) < file.FlatStartByte(idx) && composeNearestAncestorFlat(file, prop, "function_declaration") == currentFn &&
			composePropertyNameFlat(file, prop) == name {
			text = file.FlatNodeText(prop)
		}
	})
	return text
}

func composeIsStateReadOperandFlat(file *scanner.File, node uint32, delegatedReads, stateHolders map[string]bool) bool {
	node = composeUnwrapParenthesizedFlat(file, node)
	if node == 0 {
		return false
	}

	switch file.FlatType(node) {
	case "simple_identifier":
		return delegatedReads[file.FlatNodeText(node)]
	case "navigation_expression":
		if composeNavigationExpressionLastIdentifierFlat(file, node) != "value" {
			return false
		}
		receiver := composeNavigationReceiverFlat(file, node)
		return receiver != "" && stateHolders[receiver]
	default:
		return false
	}
}

func composeUnwrapParenthesizedFlat(file *scanner.File, node uint32) uint32 {
	for node != 0 && file.FlatType(node) == "parenthesized_expression" && file.FlatNamedChildCount(node) == 1 {
		node = file.FlatNamedChild(node, 0)
	}
	return node
}

func composeNavigationReceiverFlat(file *scanner.File, node uint32) string {
	if node == 0 || file.FlatNamedChildCount(node) == 0 {
		return ""
	}
	first := file.FlatNamedChild(node, 0)
	if file.FlatType(first) != "simple_identifier" {
		return ""
	}
	return file.FlatNodeText(first)
}

func composePropertyNameFlat(file *scanner.File, node uint32) string {
	varDecl, _ := file.FlatFindChild(node, "variable_declaration")
	if varDecl == 0 {
		return ""
	}
	ident, _ := file.FlatFindChild(varDecl, "simple_identifier")
	if ident == 0 {
		return ""
	}
	return file.FlatNodeText(ident)
}

func composeParameterNameFlat(file *scanner.File, node uint32) string {
	ident, _ := file.FlatFindChild(node, "simple_identifier")
	if ident == 0 {
		return ""
	}
	return file.FlatNodeText(ident)
}

func composePropertyHasStateTypeFlat(file *scanner.File, node uint32) bool {
	varDecl, _ := file.FlatFindChild(node, "variable_declaration")
	if varDecl == 0 {
		return false
	}
	return composeNodeHasStateTypeFlat(file, varDecl)
}

func composeParameterHasStateTypeFlat(file *scanner.File, node uint32) bool {
	return composeNodeHasStateTypeFlat(file, node)
}

func composeNodeHasStateTypeFlat(file *scanner.File, node uint32) bool {
	userType, _ := file.FlatFindChild(node, "user_type")
	if userType == 0 {
		return false
	}
	return composeIsStateType(file.FlatNodeText(userType))
}

func composeIsStateType(text string) bool {
	typeName := text
	if idx := strings.LastIndex(typeName, "."); idx >= 0 {
		typeName = typeName[idx+1:]
	}
	if idx := strings.Index(typeName, "<"); idx >= 0 {
		typeName = typeName[:idx]
	}

	switch typeName {
	case "State", "MutableState", "IntState", "LongState", "FloatState", "DoubleState":
		return true
	default:
		return false
	}
}

func composeContainsStateFactoryFlat(file *scanner.File, node uint32) bool {
	found := false
	file.FlatWalkNodes(node, "call_expression", func(call uint32) {
		if found {
			return
		}
		switch flatCallExpressionName(file, call) {
		case "mutableStateOf", "mutableIntStateOf", "mutableLongStateOf", "mutableFloatStateOf", "mutableDoubleStateOf":
			found = true
		}
	})
	return found
}

// ComposeLambdaCapturesUnstableStateRule flags inline onClick lambdas in lazy
// item builders when they capture the current item directly instead of
// hoisting the callback through remember(item) { ... }.
type ComposeLambdaCapturesUnstableStateRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Compose rule. Detection relies on call-name token matching (e.g.
// 'verticalScroll', 'LazyColumn', 'rememberSaveable') rather than type
// resolution, so any project symbol with a matching name or token can
// produce a false match. Classified per roadmap/17.
func (r *ComposeLambdaCapturesUnstableStateRule) Confidence() float64 { return api.ConfidenceMedium }

func composeIsNamedArgumentLambdaFlat(file *scanner.File, node uint32, argName string) bool {
	if file == nil || node == 0 || file.FlatType(node) != "lambda_literal" {
		return false
	}

	parent, ok := file.FlatParent(node)
	if !ok || file.FlatType(parent) != "value_argument" || file.FlatNamedChildCount(parent) < 2 {
		return false
	}

	name := file.FlatNamedChild(parent, 0)
	return name != 0 && file.FlatType(name) == "simple_identifier" && file.FlatNodeTextEquals(name, argName)
}

func composeLambdaParameterNamesFlat(file *scanner.File, node uint32) []string {
	params, _ := file.FlatFindChild(node, "lambda_parameters")
	if params == 0 {
		return nil
	}

	names := make([]string, 0, file.FlatNamedChildCount(params))
	for param := file.FlatFirstChild(params); param != 0; param = file.FlatNextSib(param) {
		if !file.FlatIsNamed(param) || file.FlatType(param) != "variable_declaration" {
			continue
		}
		name, _ := file.FlatFindChild(param, "simple_identifier")
		if name != 0 {
			names = append(names, file.FlatNodeString(name, nil))
		}
	}
	return names
}

func composeLambdaReferencesIdentifierFlat(file *scanner.File, node uint32, target string) bool {
	found := false
	file.FlatWalkAllNodes(node, func(ident uint32) {
		if found || file.FlatType(ident) != "simple_identifier" || !file.FlatNodeTextEquals(ident, target) || !composeIdentifierIsDirectCaptureFlat(file, ident) {
			return
		}
		found = true
	})
	return found
}

func composeIdentifierIsDirectCaptureFlat(file *scanner.File, node uint32) bool {
	parent, ok := file.FlatParent(node)
	if !ok {
		return false
	}

	switch file.FlatType(parent) {
	case "variable_declaration", "parameter", "type_identifier", "user_type", "navigation_suffix":
		return false
	case "navigation_expression":
		return false
	case "call_expression":
		return file.FlatNamedChildCount(parent) == 0 || file.FlatNamedChild(parent, 0) != node
	case "value_argument":
		return file.FlatNamedChildCount(parent) < 2 || file.FlatNamedChild(parent, 0) != node
	default:
		return true
	}
}
