package rules

import (
	"bytes"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

var (
	composeVerticalScrollToken   = []byte("verticalScroll")
	composeHorizontalScrollToken = []byte("horizontalScroll")
	composeLazyColumnToken       = []byte("LazyColumn")
	composeLazyRowToken          = []byte("LazyRow")
)

// ComposeColumnRowInScrollableRule flags nested same-axis Compose scroll
// containers such as Column(verticalScroll) containing LazyColumn.
type ComposeColumnRowInScrollableRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Compose rule. Detection relies on call-name token matching (e.g.
// 'verticalScroll', 'LazyColumn', 'rememberSaveable') rather than type
// resolution, so any project symbol with a matching name or token can
// produce a false match. Classified per roadmap/17.
func (r *ComposeColumnRowInScrollableRule) Confidence() float64 { return 0.75 }

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
func (r *ComposeDerivedStateMisuseRule) Confidence() float64 { return 0.75 }

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

func composeNearestAncestorFlat(file *scanner.File, idx uint32, nodeType string) uint32 {
	for current, ok := file.FlatParent(idx); ok; current, ok = file.FlatParent(current) {
		if file.FlatType(current) == nodeType {
			return current
		}
	}
	return 0
}

func composeNavigationExpressionLastIdentifierFlat(file *scanner.File, node uint32) string {
	if node == 0 || file.FlatType(node) != "navigation_expression" {
		return ""
	}
	last := ""
	for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		switch file.FlatType(child) {
		case "navigation_suffix":
			for gc := file.FlatFirstChild(child); gc != 0; gc = file.FlatNextSib(gc) {
				if file.FlatIsNamed(gc) && file.FlatType(gc) == "simple_identifier" {
					last = file.FlatNodeText(gc)
				}
			}
		case "simple_identifier":
			last = file.FlatNodeText(child)
		}
	}
	return last
}

func composeFlatCallContainsCall(file *scanner.File, root uint32, target string) bool {
	found := false
	file.FlatWalkNodes(root, "call_expression", func(idx uint32) {
		if !found && flatCallExpressionName(file, idx) == target {
			found = true
		}
	})
	return found
}

func composeFlatNodeMayContain(file *scanner.File, idx uint32, token []byte) bool {
	if file == nil || len(token) == 0 {
		return false
	}
	start := file.FlatStartByte(idx)
	end := file.FlatEndByte(idx)
	if end <= start || int(end) > len(file.Content) {
		return false
	}
	return bytes.Contains(file.Content[start:end], token)
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
func (r *ComposeLambdaCapturesUnstableStateRule) Confidence() float64 { return 0.75 }

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

func composeLambdaBelongsToCallFlat(file *scanner.File, node uint32, callNames ...string) bool {
	call := composeNearestAncestorFlat(file, node, "call_expression")
	if call == 0 {
		return false
	}

	name := flatCallExpressionName(file, call)
	if name == "" && file.FlatNamedChildCount(call) > 0 {
		if inner := file.FlatNamedChild(call, 0); inner != 0 && file.FlatType(inner) == "call_expression" {
			name = flatCallExpressionName(file, inner)
		}
	}
	for _, candidate := range callNames {
		if name == candidate {
			return true
		}
	}
	return false
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

// composeChainedCallPrev returns the index of the call_expression that
// immediately precedes `outer` in a Modifier-style chain, or 0 if there is
// no such previous call. For `Modifier.size(48.dp).fillMaxWidth()`, calling
// this with `outer` pointing at `fillMaxWidth()` returns the index of
// `size(48.dp)`.
//
// Compose modifier-ordering rules use this to check the immediate
// previous call in a left-to-right chain without walking deeper.
func composeChainedCallPrev(file *scanner.File, outer uint32) uint32 {
	navExpr, _ := flatCallExpressionParts(file, outer)
	if navExpr == 0 {
		return 0
	}
	for child := file.FlatFirstChild(navExpr); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "call_expression" {
			return child
		}
	}
	return 0
}

// ComposeModifierFillAfterSizeRule flags chained Compose Modifier calls where
// a `fillMaxWidth` / `fillMaxHeight` / `fillMaxSize` call immediately follows
// `size(...)`. The fillMax call overrides the explicit size on one or both
// axes, which is almost always an author mistake.
type ComposeModifierFillAfterSizeRule struct {
	FlatDispatchBase
	BaseRule
}

var composeFillMaxAxisNames = map[string]struct{}{
	"fillMaxWidth":  {},
	"fillMaxHeight": {},
	"fillMaxSize":   {},
}

// Confidence reports a tier-2 (medium) base confidence. Compose rule. Detection relies on call-name token matching (e.g.
// 'verticalScroll', 'LazyColumn', 'rememberSaveable') rather than type
// resolution, so any project symbol with a matching name or token can
// produce a false match. Classified per roadmap/17.
func (r *ComposeModifierFillAfterSizeRule) Confidence() float64 { return 0.75 }

// ComposeModifierBackgroundAfterClipRule flags `Modifier.background(...).clip(...)`
// where the background is drawn in the un-clipped rectangular region instead
// of the clipped shape the author almost certainly intended.
type ComposeModifierBackgroundAfterClipRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Compose rule. Detection relies on call-name token matching (e.g.
// 'verticalScroll', 'LazyColumn', 'rememberSaveable') rather than type
// resolution, so any project symbol with a matching name or token can
// produce a false match. Classified per roadmap/17.
func (r *ComposeModifierBackgroundAfterClipRule) Confidence() float64 { return 0.75 }

// ComposeModifierClickableBeforePaddingRule flags
// `Modifier.clickable { }.padding(...)` — the click area excludes the padding
// region, which is almost never what the author wants.
type ComposeModifierClickableBeforePaddingRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Compose rule. Detection relies on call-name token matching (e.g.
// 'verticalScroll', 'LazyColumn', 'rememberSaveable') rather than type
// resolution, so any project symbol with a matching name or token can
// produce a false match. Classified per roadmap/17.
func (r *ComposeModifierClickableBeforePaddingRule) Confidence() float64 { return 0.75 }

// composeEnclosingLambdaArgumentName walks up from idx looking for the
// nearest lambda_literal ancestor that is itself a named value_argument, and
// returns the argument label (e.g. "onClick", "onValueChange"). Returns ""
// if no such enclosing lambda exists, or if the lambda is not a named
// argument. This powers callback/handler detection without needing type
// info about Compose itself.
func composeEnclosingLambdaArgumentName(file *scanner.File, idx uint32) string {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		if file.FlatType(cur) != "lambda_literal" {
			continue
		}
		parent, pok := file.FlatParent(cur)
		if !pok {
			return ""
		}
		// Lambdas passed as trailing lambdas are wrapped in annotated_lambda
		// / lambda_argument, not value_argument — skip those.
		if file.FlatType(parent) != "value_argument" {
			return ""
		}
		if file.FlatNamedChildCount(parent) < 2 {
			return ""
		}
		label := file.FlatNamedChild(parent, 0)
		if label == 0 || file.FlatType(label) != "simple_identifier" {
			return ""
		}
		return file.FlatNodeText(label)
	}
	return ""
}

// composeLambdaReferencesEnclosingParam reports whether the lambda body at
// lambdaIdx references any simple_identifier that matches a parameter of
// the nearest enclosing function_declaration. Used to detect
// `remember { f(param) }` / `LaunchedEffect(Unit) { f(param) }` patterns
// where the lambda captures mutable outer state without listing it as a key.
func composeLambdaReferencesEnclosingParam(file *scanner.File, lambdaIdx uint32) (string, bool) {
	fn, ok := flatEnclosingFunction(file, lambdaIdx)
	if !ok {
		return "", false
	}
	paramNames := flatFunctionParameterNames(file, fn)
	if len(paramNames) == 0 {
		return "", false
	}
	paramSet := make(map[string]struct{}, len(paramNames))
	for _, n := range paramNames {
		paramSet[n] = struct{}{}
	}

	var hit string
	file.FlatWalkAllNodes(lambdaIdx, func(idx uint32) {
		if hit != "" {
			return
		}
		if file.FlatType(idx) != "simple_identifier" {
			return
		}
		text := file.FlatNodeText(idx)
		if _, match := paramSet[text]; !match {
			return
		}
		// Filter out identifiers that appear as a label/name (declaration or
		// named-argument label), not as a reference — those don't count as a
		// capture.
		parent, pok := file.FlatParent(idx)
		if !pok {
			return
		}
		switch file.FlatType(parent) {
		case "variable_declaration", "parameter", "navigation_suffix":
			return
		case "value_argument":
			// The first named child of a value_argument whose count is >= 2
			// is the argument label (e.g. `onClick = { ... }`), not a reference.
			if file.FlatNamedChildCount(parent) >= 2 && file.FlatNamedChild(parent, 0) == idx {
				return
			}
		}
		hit = text
	})
	return hit, hit != ""
}

// ComposeRememberWithoutKeyRule flags `remember { f(param) }` where the
// lambda body references an enclosing function parameter but the remember
// call has no key arguments. The cached value won't update when the
// parameter changes across recomposition.
type ComposeRememberWithoutKeyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Compose rule. Detection relies on call-name token matching (e.g.
// 'verticalScroll', 'LazyColumn', 'rememberSaveable') rather than type
// resolution, so any project symbol with a matching name or token can
// produce a false match. Classified per roadmap/17.
func (r *ComposeRememberWithoutKeyRule) Confidence() float64 { return 0.75 }

// ComposeLaunchedEffectWithoutKeysRule flags `LaunchedEffect(Unit) { f(param) }`
// where the effect body references an enclosing parameter but the keys are
// constant (Unit / true / false / null). The effect won't re-run when the
// parameter changes.
type ComposeLaunchedEffectWithoutKeysRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Compose rule. Detection relies on call-name token matching (e.g.
// 'verticalScroll', 'LazyColumn', 'rememberSaveable') rather than type
// resolution, so any project symbol with a matching name or token can
// produce a false match. Classified per roadmap/17.
func (r *ComposeLaunchedEffectWithoutKeysRule) Confidence() float64 { return 0.75 }


var composeConstantKeyTexts = map[string]struct{}{
	"Unit":  {},
	"true":  {},
	"false": {},
	"null":  {},
}
// ComposeUnstableParameterRule flags `@Composable fun X(users: List<User>)`
// whose parameters use the mutable Kotlin collection types (`List`, `Map`,
// `Set`, `MutableList`, etc.) directly. Compose considers these unstable
// for recomposition-skipping unless the author wraps them in a stable type
// (`@Immutable data class`, `ImmutableList` / `PersistentList` from
// kotlinx.collections.immutable).
type ComposeUnstableParameterRule struct {
	FlatDispatchBase
	BaseRule
}

var composeUnstableCollectionTypes = map[string]struct{}{
	"List":        {},
	"Map":         {},
	"Set":         {},
	"MutableList": {},
	"MutableMap":  {},
	"MutableSet":  {},
	"Collection":  {},
	"Iterable":    {},
}

// Confidence reports a tier-2 (medium) base confidence. Compose rule. Detection relies on call-name token matching (e.g.
// 'verticalScroll', 'LazyColumn', 'rememberSaveable') rather than type
// resolution, so any project symbol with a matching name or token can
// produce a false match. Classified per roadmap/17.
func (r *ComposeUnstableParameterRule) Confidence() float64 { return 0.75 }

// ComposeRememberSaveableNonParcelableRule flags `rememberSaveable {
// SomeType() }` where SomeType() is a constructor call to a plain class
// with no `saver` / `stateSaver` argument passed. Without a saver,
// rememberSaveable falls back to AutoSaver which only handles primitives,
// Parcelables, and a few stdlib types.
//
// Since this rule is syntactic it can't verify Parcelable status — it
// warns on any constructor-looking call in the trailing lambda whose name
// doesn't match a well-known stdlib type. Users can suppress the finding
// locally when they know the type is Parcelable/Serializable.
type ComposeRememberSaveableNonParcelableRule struct {
	FlatDispatchBase
	BaseRule
}

var composeSaverSafeBuiltins = map[string]struct{}{
	"Int":     {},
	"Long":    {},
	"Float":   {},
	"Double":  {},
	"Boolean": {},
	"Char":    {},
	"String":  {},
	"Unit":    {},
	"Short":   {},
	"Byte":    {},
	"BigInt":  {},
	"Bundle":  {},
}

// Confidence reports a tier-2 (medium) base confidence. Compose rule. Detection relies on call-name token matching (e.g.
// 'verticalScroll', 'LazyColumn', 'rememberSaveable') rather than type
// resolution, so any project symbol with a matching name or token can
// produce a false match. Classified per roadmap/17.
func (r *ComposeRememberSaveableNonParcelableRule) Confidence() float64 { return 0.75 }

// ComposeSideEffectInCompositionRule flags direct assignments inside a
// @Composable function body that aren't wrapped in a recognized effect
// block (LaunchedEffect / SideEffect / DisposableEffect). Assignments
// during composition run on every recomposition and cause unpredictable
// behavior — mutations belong in an effect scope.
type ComposeSideEffectInCompositionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Compose rule. Detection relies on call-name token matching (e.g.
// 'verticalScroll', 'LazyColumn', 'rememberSaveable') rather than type
// resolution, so any project symbol with a matching name or token can
// produce a false match. Classified per roadmap/17.
func (r *ComposeSideEffectInCompositionRule) Confidence() float64 { return 0.75 }


var composeEffectBlockCalls = map[string]struct{}{
	"LaunchedEffect":   {},
	"SideEffect":       {},
	"DisposableEffect": {},
}
// composeLambdaOwningCall returns the call_expression that a lambda_literal
// is passed as a trailing lambda to, if any.
func composeLambdaOwningCall(file *scanner.File, lambdaIdx uint32) (uint32, bool) {
	parent, ok := file.FlatParent(lambdaIdx)
	if !ok {
		return 0, false
	}
	// annotated_lambda wraps the lambda_literal.
	if file.FlatType(parent) == "annotated_lambda" {
		parent, ok = file.FlatParent(parent)
		if !ok {
			return 0, false
		}
	}
	// call_suffix wraps the annotated_lambda.
	if file.FlatType(parent) != "call_suffix" {
		return 0, false
	}
	callExpr, ok := file.FlatParent(parent)
	if !ok || file.FlatType(callExpr) != "call_expression" {
		return 0, false
	}
	return callExpr, true
}

// ComposeModifierPassedThenChainedRule flags a @Composable function that
// declares a `modifier: Modifier` parameter but never actually uses it —
// the body starts fresh `Modifier.X()` chains, dropping anything the
// caller passed in.
type ComposeModifierPassedThenChainedRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Compose rule. Detection relies on call-name token matching (e.g.
// 'verticalScroll', 'LazyColumn', 'rememberSaveable') rather than type
// resolution, so any project symbol with a matching name or token can
// produce a false match. Classified per roadmap/17.
func (r *ComposeModifierPassedThenChainedRule) Confidence() float64 { return 0.75 }

// composeHasModifierParameter reports whether the given function_declaration
// declares a parameter named exactly `modifier` of type `Modifier`.
func composeHasModifierParameter(file *scanner.File, funcDecl uint32) bool {
	params, _ := file.FlatFindChild(funcDecl, "function_value_parameters")
	if params == 0 {
		return false
	}
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "parameter" {
			continue
		}
		nameNode, _ := file.FlatFindChild(child, "simple_identifier")
		if nameNode == 0 || !file.FlatNodeTextEquals(nameNode, "modifier") {
			continue
		}
		typeNode, _ := file.FlatFindChild(child, "user_type")
		if typeNode == 0 {
			continue
		}
		if typeIdent, ok := file.FlatFindChild(typeNode, "type_identifier"); ok &&
			file.FlatNodeTextEquals(typeIdent, "Modifier") {
			return true
		}
	}
	return false
}

// ComposeDisposableEffectMissingDisposeRule flags `DisposableEffect(key) {
// ... }` whose trailing lambda does not end with an `onDispose { }` call.
// Without `onDispose`, the registered resource leaks when the composable
// leaves the composition.
type ComposeDisposableEffectMissingDisposeRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Compose rule. Detection relies on call-name token matching (e.g.
// 'verticalScroll', 'LazyColumn', 'rememberSaveable') rather than type
// resolution, so any project symbol with a matching name or token can
// produce a false match. Classified per roadmap/17.
func (r *ComposeDisposableEffectMissingDisposeRule) Confidence() float64 { return 0.75 }

// ComposePreviewWithBackingStateRule flags `@Preview @Composable fun
// FooPreview()` whose body calls a runtime state holder like
// `hiltViewModel()`, `viewModel()`, or `collectAsState*()` — those won't
// work in the Android Studio preview renderer and will either crash or
// render a placeholder. Previews should use fake/injected data.
type ComposePreviewWithBackingStateRule struct {
	FlatDispatchBase
	BaseRule
}

var composeRuntimeStateHolderCalls = map[string]struct{}{
	"hiltViewModel":               {},
	"viewModel":                   {},
	"collectAsState":              {},
	"collectAsStateWithLifecycle": {},
}

// Confidence reports a tier-2 (medium) base confidence. Compose rule. Detection relies on call-name token matching (e.g.
// 'verticalScroll', 'LazyColumn', 'rememberSaveable') rather than type
// resolution, so any project symbol with a matching name or token can
// produce a false match. Classified per roadmap/17.
func (r *ComposePreviewWithBackingStateRule) Confidence() float64 { return 0.75 }

// ComposeStatefulDefaultParameterRule flags `@Composable fun Foo(state:
// MyState = MyState())` — the `MyState()` default allocates a fresh instance
// on every recomposition, breaking state hoisting and silently dropping
// accumulated state. The safe pattern is `= rememberMyState()` or
// `= remember { MyState() }`, both of which return the same instance across
// recompositions.
type ComposeStatefulDefaultParameterRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Compose rule. Detection relies on call-name token matching (e.g.
// 'verticalScroll', 'LazyColumn', 'rememberSaveable') rather than type
// resolution, so any project symbol with a matching name or token can
// produce a false match. Classified per roadmap/17.
func (r *ComposeStatefulDefaultParameterRule) Confidence() float64 { return 0.75 }

// ComposeMutableStateInCompositionRule flags `val count = mutableStateOf(0)`
// as a local property inside a @Composable function. Without a surrounding
// `remember { }`, the state is reconstructed on every recomposition and
// silently discards any updates.
type ComposeMutableStateInCompositionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Compose rule. Detection relies on call-name token matching (e.g.
// 'verticalScroll', 'LazyColumn', 'rememberSaveable') rather than type
// resolution, so any project symbol with a matching name or token can
// produce a false match. Classified per roadmap/17.
func (r *ComposeMutableStateInCompositionRule) Confidence() float64 { return 0.75 }

// ComposeStringResourceInsideLambdaRule flags `stringResource(...)` calls
// nested inside a callback-style named-argument lambda (e.g. `onClick = {
// ... stringResource(R.string.X) ... }`). `stringResource` is composition-
// only and will crash when invoked from a non-composable callback lambda;
// the safe pattern is to hoist the resource lookup above the lambda.
type ComposeStringResourceInsideLambdaRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Compose rule. Detection relies on call-name token matching (e.g.
// 'verticalScroll', 'LazyColumn', 'rememberSaveable') rather than type
// resolution, so any project symbol with a matching name or token can
// produce a false match. Classified per roadmap/17.
func (r *ComposeStringResourceInsideLambdaRule) Confidence() float64 { return 0.75 }

// ComposeMutableDefaultArgumentRule flags `@Composable fun Foo(items:
// MutableList<X> = mutableListOf())` — the mutable default evaluates on each
// recomposition, creating a fresh collection that silently breaks identity
// checks and can leak state across composes.
type ComposeMutableDefaultArgumentRule struct {
	FlatDispatchBase
	BaseRule
}

var composeMutableCollectionFactories = map[string]struct{}{
	"mutableListOf": {},
	"mutableSetOf":  {},
	"mutableMapOf":  {},
	"arrayListOf":   {},
	"hashMapOf":     {},
	"hashSetOf":     {},
	"linkedMapOf":   {},
	"linkedSetOf":   {},
}

// Confidence reports a tier-2 (medium) base confidence. Compose rule. Detection relies on call-name token matching (e.g.
// 'verticalScroll', 'LazyColumn', 'rememberSaveable') rather than type
// resolution, so any project symbol with a matching name or token can
// produce a false match. Classified per roadmap/17.
func (r *ComposeMutableDefaultArgumentRule) Confidence() float64 { return 0.75 }

// ComposePreviewAnnotationMissingRule flags `@Composable fun FooPreview()`
// whose name ends in "Preview" but which is missing the `@Preview`
// annotation. Such functions almost always intend to render a preview in
// Android Studio; without `@Preview` they're dead code at preview time.
type ComposePreviewAnnotationMissingRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Compose rule. Detection relies on call-name token matching (e.g.
// 'verticalScroll', 'LazyColumn', 'rememberSaveable') rather than type
// resolution, so any project symbol with a matching name or token can
// produce a false match. Classified per roadmap/17.
func (r *ComposePreviewAnnotationMissingRule) Confidence() float64 { return 0.75 }
