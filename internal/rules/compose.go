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

func (r *ComposeColumnRowInScrollableRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *ComposeColumnRowInScrollableRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	call := file.FlatChild(idx, 0)
	if call == 0 || file.FlatType(call) != "call_expression" {
		return nil
	}
	nameNode := file.FlatFindChild(call, "simple_identifier")
	if nameNode == 0 {
		return nil
	}
	callSuffix := file.FlatFindChild(idx, "call_suffix")
	if callSuffix == 0 {
		return nil
	}
	annotatedLambda := file.FlatFindChild(callSuffix, "annotated_lambda")
	if annotatedLambda == 0 {
		return nil
	}
	lambda := file.FlatFindChild(annotatedLambda, "lambda_literal")
	if lambda == 0 {
		return nil
	}

	if file.FlatNodeTextEquals(nameNode, "Column") {
		if !composeFlatNodeMayContain(file, call, composeVerticalScrollToken) ||
			!composeFlatNodeMayContain(file, lambda, composeLazyColumnToken) ||
			!composeFlatCallContainsCall(file, call, "verticalScroll") {
			return nil
		}
		if !composeFlatCallContainsCall(file, lambda, "LazyColumn") {
			return nil
		}
		return []scanner.Finding{r.Finding(
			file,
			file.FlatRow(idx)+1,
			file.FlatCol(idx)+1,
			"Column with verticalScroll should not contain LazyColumn; use a single LazyColumn for the scrollable content.",
		)}
	} else if file.FlatNodeTextEquals(nameNode, "Row") {
		if !composeFlatNodeMayContain(file, call, composeHorizontalScrollToken) ||
			!composeFlatNodeMayContain(file, lambda, composeLazyRowToken) ||
			!composeFlatCallContainsCall(file, call, "horizontalScroll") {
			return nil
		}
		if !composeFlatCallContainsCall(file, lambda, "LazyRow") {
			return nil
		}
		return []scanner.Finding{r.Finding(
			file,
			file.FlatRow(idx)+1,
			file.FlatCol(idx)+1,
			"Row with horizontalScroll should not contain LazyRow; use a single LazyRow for the scrollable content.",
		)}
	}
	return nil
}

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

func (r *ComposeDerivedStateMisuseRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *ComposeDerivedStateMisuseRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if flatCallExpressionName(file, idx) != "derivedStateOf" {
		return nil
	}

	body := composeDerivedStateBodyFlat(file, idx)
	if body == 0 {
		return nil
	}
	if t := file.FlatType(body); t != "comparison_expression" && t != "equality_expression" {
		return nil
	}

	delegatedReads, stateHolders := composeStateBindingsInScopeFlat(file, idx)
	if !composeIsDirectStateComparisonFlat(file, body, delegatedReads, stateHolders) {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"derivedStateOf around a direct state comparison is unnecessary; the state read already drives recomposition.",
	)}
}

func composeDerivedStateBodyFlat(file *scanner.File, idx uint32) uint32 {
	callSuffix := file.FlatFindChild(idx, "call_suffix")
	if callSuffix == 0 {
		return 0
	}
	annotatedLambda := file.FlatFindChild(callSuffix, "annotated_lambda")
	if annotatedLambda == 0 {
		return 0
	}
	lambda := file.FlatFindChild(annotatedLambda, "lambda_literal")
	if lambda == 0 {
		return 0
	}
	statements := file.FlatFindChild(lambda, "statements")
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
		params := file.FlatFindChild(currentFn, "function_value_parameters")
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
		if file.FlatFindChild(prop, "property_delegate") != 0 && hasStateFactory {
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
	varDecl := file.FlatFindChild(node, "variable_declaration")
	if varDecl == 0 {
		return ""
	}
	ident := file.FlatFindChild(varDecl, "simple_identifier")
	if ident == 0 {
		return ""
	}
	return file.FlatNodeText(ident)
}

func composeParameterNameFlat(file *scanner.File, node uint32) string {
	ident := file.FlatFindChild(node, "simple_identifier")
	if ident == 0 {
		return ""
	}
	return file.FlatNodeText(ident)
}

func composePropertyHasStateTypeFlat(file *scanner.File, node uint32) bool {
	varDecl := file.FlatFindChild(node, "variable_declaration")
	if varDecl == 0 {
		return false
	}
	return composeNodeHasStateTypeFlat(file, varDecl)
}

func composeParameterHasStateTypeFlat(file *scanner.File, node uint32) bool {
	return composeNodeHasStateTypeFlat(file, node)
}

func composeNodeHasStateTypeFlat(file *scanner.File, node uint32) bool {
	userType := file.FlatFindChild(node, "user_type")
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

func (r *ComposeLambdaCapturesUnstableStateRule) NodeTypes() []string {
	return []string{"lambda_literal"}
}

func (r *ComposeLambdaCapturesUnstableStateRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !composeIsNamedArgumentLambdaFlat(file, idx, "onClick") {
		return nil
	}

	itemsLambda := composeNearestAncestorFlat(file, idx, "lambda_literal")
	if itemsLambda == 0 || !composeLambdaBelongsToCallFlat(file, itemsLambda, "items", "itemsIndexed") {
		return nil
	}

	for _, paramName := range composeLambdaParameterNamesFlat(file, itemsLambda) {
		if composeLambdaReferencesIdentifierFlat(file, idx, paramName) {
			return []scanner.Finding{r.Finding(
				file,
				file.FlatRow(idx)+1,
				file.FlatCol(idx)+1,
				"inline Compose callback captures the current lazy item; hoist it behind remember(item) before passing it to onClick.",
			)}
		}
	}

	return nil
}

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
	params := file.FlatFindChild(node, "lambda_parameters")
	if params == 0 {
		return nil
	}

	names := make([]string, 0, file.FlatNamedChildCount(params))
	for param := file.FlatFirstChild(params); param != 0; param = file.FlatNextSib(param) {
		if !file.FlatIsNamed(param) || file.FlatType(param) != "variable_declaration" {
			continue
		}
		name := file.FlatFindChild(param, "simple_identifier")
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

func (r *ComposeModifierFillAfterSizeRule) NodeTypes() []string {
	return []string{"call_expression"}
}

func (r *ComposeModifierFillAfterSizeRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if _, ok := composeFillMaxAxisNames[flatCallExpressionName(file, idx)]; !ok {
		return nil
	}
	prev := composeChainedCallPrev(file, idx)
	if prev == 0 || flatCallExpressionName(file, prev) != "size" {
		return nil
	}
	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"fillMax* after size() overrides the explicit size on one or both axes; drop the size() or swap the order so fillMax*() is applied first.",
	)}
}

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

func (r *ComposeModifierBackgroundAfterClipRule) NodeTypes() []string {
	return []string{"call_expression"}
}

func (r *ComposeModifierBackgroundAfterClipRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if flatCallExpressionName(file, idx) != "clip" {
		return nil
	}
	prev := composeChainedCallPrev(file, idx)
	if prev == 0 || flatCallExpressionName(file, prev) != "background" {
		return nil
	}
	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"background() applied before clip() paints the un-clipped rectangle; put .clip(...) before .background(...) so the background respects the clip shape.",
	)}
}

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

func (r *ComposeModifierClickableBeforePaddingRule) NodeTypes() []string {
	return []string{"call_expression"}
}

func (r *ComposeModifierClickableBeforePaddingRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if flatCallExpressionName(file, idx) != "padding" {
		return nil
	}
	prev := composeChainedCallPrev(file, idx)
	if prev == 0 || flatCallExpressionName(file, prev) != "clickable" {
		return nil
	}
	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"clickable { } before padding() makes the click area exclude the padding region; put .padding(...) first so the click hit test covers the padded bounds.",
	)}
}

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

func (r *ComposeRememberWithoutKeyRule) NodeTypes() []string {
	return []string{"call_expression"}
}

func (r *ComposeRememberWithoutKeyRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if flatCallNameAny(file, idx) != "remember" {
		return nil
	}
	if flatCallKeyArguments(file, idx) != 0 {
		return nil
	}
	lambda := flatCallTrailingLambda(file, idx)
	if lambda == 0 {
		return nil
	}
	paramName, ok := composeLambdaReferencesEnclosingParam(file, lambda)
	if !ok {
		return nil
	}
	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"remember { } reads enclosing parameter "+paramName+" but has no keys; the cached value won't update when "+paramName+" changes. Pass remember("+paramName+") { ... }.",
	)}
}

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

func (r *ComposeLaunchedEffectWithoutKeysRule) NodeTypes() []string {
	return []string{"call_expression"}
}

var composeConstantKeyTexts = map[string]struct{}{
	"Unit":  {},
	"true":  {},
	"false": {},
	"null":  {},
}

func (r *ComposeLaunchedEffectWithoutKeysRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if flatCallNameAny(file, idx) != "LaunchedEffect" {
		return nil
	}
	lambda := flatCallTrailingLambda(file, idx)
	if lambda == 0 {
		return nil
	}
	args := flatCallKeyArguments(file, idx)
	if args == 0 {
		return nil
	}
	// Every value_argument must be a constant key (Unit / true / false /
	// null) for the "effectively no keys" check to fire. If the author
	// passes any non-constant, assume they thought about keying.
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		text := file.FlatNodeText(arg)
		if _, ok := composeConstantKeyTexts[text]; !ok {
			return nil
		}
	}
	paramName, ok := composeLambdaReferencesEnclosingParam(file, lambda)
	if !ok {
		return nil
	}
	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"LaunchedEffect body reads enclosing parameter "+paramName+" but is keyed only on constants; the effect won't re-run when "+paramName+" changes. Pass LaunchedEffect("+paramName+") { ... }.",
	)}
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

func (r *ComposeUnstableParameterRule) NodeTypes() []string {
	return []string{"function_declaration"}
}

func (r *ComposeUnstableParameterRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !flatHasAnnotationNamed(file, idx, "Composable") {
		return nil
	}
	params := file.FlatFindChild(idx, "function_value_parameters")
	if params == 0 {
		return nil
	}
	var findings []scanner.Finding
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "parameter" {
			continue
		}
		typeNode := file.FlatFindChild(child, "user_type")
		if typeNode == 0 {
			continue
		}
		typeIdent := file.FlatFindChild(typeNode, "type_identifier")
		if typeIdent == 0 {
			continue
		}
		name := file.FlatNodeText(typeIdent)
		if _, unstable := composeUnstableCollectionTypes[name]; !unstable {
			continue
		}
		paramName := ""
		if idNode := file.FlatFindChild(child, "simple_identifier"); idNode != 0 {
			paramName = file.FlatNodeText(idNode)
		}
		findings = append(findings, r.Finding(
			file,
			file.FlatRow(child)+1,
			file.FlatCol(child)+1,
			"@Composable parameter "+paramName+": "+name+"<...> is an unstable type for recomposition skipping; use an ImmutableList/PersistentList from kotlinx.collections.immutable or wrap the collection in an @Immutable data class.",
		))
	}
	return findings
}

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

func (r *ComposeRememberSaveableNonParcelableRule) NodeTypes() []string {
	return []string{"call_expression"}
}

func (r *ComposeRememberSaveableNonParcelableRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if flatCallNameAny(file, idx) != "rememberSaveable" {
		return nil
	}
	// If the user passed a saver / stateSaver argument, assume they know
	// what they're doing.
	if args := flatCallKeyArguments(file, idx); args != 0 {
		for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
			if file.FlatType(arg) != "value_argument" {
				continue
			}
			if file.FlatNamedChildCount(arg) < 2 {
				continue
			}
			label := file.FlatNamedChild(arg, 0)
			if label == 0 || file.FlatType(label) != "simple_identifier" {
				continue
			}
			text := file.FlatNodeText(label)
			if text == "saver" || text == "stateSaver" {
				return nil
			}
		}
	}
	lambda := flatCallTrailingLambda(file, idx)
	if lambda == 0 {
		return nil
	}
	stmts := file.FlatFindChild(lambda, "statements")
	if stmts == 0 {
		return nil
	}
	// The last statement in the lambda is the returned value. If it's a
	// constructor-looking call_expression (uppercase name) and not a
	// saver-safe builtin, report.
	var last uint32
	for child := file.FlatFirstChild(stmts); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) {
			last = child
		}
	}
	if last == 0 || file.FlatType(last) != "call_expression" {
		return nil
	}
	name := flatCallExpressionName(file, last)
	if name == "" {
		return nil
	}
	first := name[0]
	if first < 'A' || first > 'Z' {
		return nil
	}
	if _, safe := composeSaverSafeBuiltins[name]; safe {
		return nil
	}
	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"rememberSaveable { "+name+"() } with no `saver`/`stateSaver` falls back to AutoSaver, which only handles primitives and Parcelable/Serializable. If "+name+" isn't @Parcelize, pass an explicit saver; otherwise suppress this warning.",
	)}
}

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

func (r *ComposeSideEffectInCompositionRule) NodeTypes() []string {
	return []string{"assignment"}
}

var composeEffectBlockCalls = map[string]struct{}{
	"LaunchedEffect":   {},
	"SideEffect":       {},
	"DisposableEffect": {},
}

func (r *ComposeSideEffectInCompositionRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	fn, ok := flatEnclosingFunction(file, idx)
	if !ok || !flatHasAnnotationNamed(file, fn, "Composable") {
		return nil
	}
	// Walk up from the assignment. If any enclosing lambda_literal is the
	// trailing lambda of a recognized effect block call, the assignment is
	// legal.
	for cur, ok := file.FlatParent(idx); ok && cur != fn; cur, ok = file.FlatParent(cur) {
		if file.FlatType(cur) != "lambda_literal" {
			continue
		}
		// Is this lambda the trailing lambda of a call_expression named
		// LaunchedEffect/SideEffect/DisposableEffect?
		// Walk up from the lambda: lambda_literal -> annotated_lambda ->
		// call_suffix -> call_expression (the outer call).
		if owningCall, ok := composeLambdaOwningCall(file, cur); ok {
			if _, effect := composeEffectBlockCalls[flatCallNameAny(file, owningCall)]; effect {
				return nil
			}
		}
	}
	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"assignment inside a @Composable function body runs on every recomposition; wrap the mutation in LaunchedEffect / SideEffect / DisposableEffect so it only runs when intended.",
	)}
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

func (r *ComposeModifierPassedThenChainedRule) NodeTypes() []string {
	return []string{"function_declaration"}
}

func (r *ComposeModifierPassedThenChainedRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !flatHasAnnotationNamed(file, idx, "Composable") {
		return nil
	}
	if !composeHasModifierParameter(file, idx) {
		return nil
	}
	body := file.FlatFindChild(idx, "function_body")
	if body == 0 {
		return nil
	}

	// Body references to `modifier` (the parameter) as a simple_identifier.
	// If the author uses it anywhere we don't fire, even if they also start
	// fresh chains elsewhere — that's a coverage tradeoff.
	referenced := false
	file.FlatWalkAllNodes(body, func(n uint32) {
		if referenced {
			return
		}
		if file.FlatType(n) != "simple_identifier" {
			return
		}
		if !file.FlatNodeTextEquals(n, "modifier") {
			return
		}
		// Skip navigation_suffix children — they'd be `.modifier` accessors,
		// not references to the local.
		parent, ok := file.FlatParent(n)
		if !ok {
			return
		}
		if file.FlatType(parent) == "navigation_suffix" {
			return
		}
		referenced = true
	})
	if referenced {
		return nil
	}

	// Find all fresh `Modifier.X(...)` chains (navigation_expression whose
	// first named child is a simple_identifier "Modifier") and report them.
	var findings []scanner.Finding
	file.FlatWalkAllNodes(body, func(n uint32) {
		if file.FlatType(n) != "navigation_expression" {
			return
		}
		if file.FlatNamedChildCount(n) == 0 {
			return
		}
		first := file.FlatNamedChild(n, 0)
		if file.FlatType(first) != "simple_identifier" {
			return
		}
		if !file.FlatNodeTextEquals(first, "Modifier") {
			return
		}
		findings = append(findings, r.Finding(
			file,
			file.FlatRow(n)+1,
			file.FlatCol(n)+1,
			"this @Composable declares a `modifier: Modifier` parameter but the body starts a fresh `Modifier.X()` chain and never uses it; chain off the passed-in `modifier` instead so the caller's modifiers aren't dropped.",
		))
	})
	return findings
}

// composeHasModifierParameter reports whether the given function_declaration
// declares a parameter named exactly `modifier` of type `Modifier`.
func composeHasModifierParameter(file *scanner.File, funcDecl uint32) bool {
	params := file.FlatFindChild(funcDecl, "function_value_parameters")
	if params == 0 {
		return false
	}
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "parameter" {
			continue
		}
		nameNode := file.FlatFindChild(child, "simple_identifier")
		if nameNode == 0 || !file.FlatNodeTextEquals(nameNode, "modifier") {
			continue
		}
		typeNode := file.FlatFindChild(child, "user_type")
		if typeNode == 0 {
			continue
		}
		if typeIdent := file.FlatFindChild(typeNode, "type_identifier"); typeIdent != 0 &&
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

func (r *ComposeDisposableEffectMissingDisposeRule) NodeTypes() []string {
	return []string{"call_expression"}
}

func (r *ComposeDisposableEffectMissingDisposeRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if flatCallNameAny(file, idx) != "DisposableEffect" {
		return nil
	}
	lambda := flatCallTrailingLambda(file, idx)
	if lambda == 0 {
		return nil
	}
	stmts := file.FlatFindChild(lambda, "statements")
	if stmts == 0 {
		return nil
	}
	// Find the last named child of the statements block. If it's a
	// call_expression to onDispose, we're good; otherwise report.
	var last uint32
	for child := file.FlatFirstChild(stmts); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) {
			last = child
		}
	}
	if last == 0 {
		return nil
	}
	if file.FlatType(last) == "call_expression" && flatCallNameAny(file, last) == "onDispose" {
		return nil
	}
	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"DisposableEffect body must end with onDispose { ... }; without it the resource registered in the block leaks when the composable leaves composition.",
	)}
}

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

func (r *ComposePreviewWithBackingStateRule) NodeTypes() []string {
	return []string{"function_declaration"}
}

func (r *ComposePreviewWithBackingStateRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !flatHasAnnotationNamed(file, idx, "Preview") {
		return nil
	}
	if !flatHasAnnotationNamed(file, idx, "Composable") {
		return nil
	}
	body := file.FlatFindChild(idx, "function_body")
	if body == 0 {
		return nil
	}
	var findings []scanner.Finding
	file.FlatWalkAllNodes(body, func(n uint32) {
		if file.FlatType(n) != "call_expression" {
			return
		}
		name := flatCallExpressionName(file, n)
		if _, ok := composeRuntimeStateHolderCalls[name]; !ok {
			return
		}
		findings = append(findings, r.Finding(
			file,
			file.FlatRow(n)+1,
			file.FlatCol(n)+1,
			"@Preview body calls "+name+"() which depends on a runtime state holder; previews should render with fake/injected data so the Studio preview renderer can show them.",
		))
	})
	return findings
}

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

func (r *ComposeStatefulDefaultParameterRule) NodeTypes() []string {
	return []string{"function_declaration"}
}

func (r *ComposeStatefulDefaultParameterRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !flatHasAnnotationNamed(file, idx, "Composable") {
		return nil
	}
	params := file.FlatFindChild(idx, "function_value_parameters")
	if params == 0 {
		return nil
	}
	var findings []scanner.Finding
	sawEquals := false
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		typ := file.FlatType(child)
		switch typ {
		case "=":
			sawEquals = true
			continue
		case ",", "parameter", "(", ")":
			sawEquals = false
			continue
		}
		if !sawEquals {
			continue
		}
		sawEquals = false
		if typ != "call_expression" {
			continue
		}
		name := flatCallExpressionName(file, child)
		if name == "" {
			continue
		}
		// Constructor-looking call: starts with an uppercase letter. Factories
		// and `remember*` helpers start lowercase and are therefore excluded.
		first := name[0]
		if first < 'A' || first > 'Z' {
			continue
		}
		findings = append(findings, r.Finding(
			file,
			file.FlatRow(child)+1,
			file.FlatCol(child)+1,
			"default `"+name+"()` on a @Composable parameter allocates a fresh instance every recomposition; use `remember { "+name+"() }` or a `remember"+name+"()` helper so the state persists across composes.",
		))
	}
	return findings
}

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

func (r *ComposeMutableStateInCompositionRule) NodeTypes() []string {
	return []string{"property_declaration"}
}

func (r *ComposeMutableStateInCompositionRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Must live inside a @Composable function.
	fn, ok := flatEnclosingFunction(file, idx)
	if !ok || !flatHasAnnotationNamed(file, fn, "Composable") {
		return nil
	}
	// Walk the property_declaration children looking for the pattern
	// `= mutableStateOf(...)`. A `by` delegate yields a `property_delegate`
	// child instead and is explicitly allowed.
	sawEquals := false
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "=":
			sawEquals = true
			continue
		case "property_delegate":
			return nil
		}
		if !sawEquals {
			continue
		}
		if file.FlatType(child) != "call_expression" {
			continue
		}
		if flatCallExpressionName(file, child) != "mutableStateOf" {
			return nil
		}
		return []scanner.Finding{r.Finding(
			file,
			file.FlatRow(child)+1,
			file.FlatCol(child)+1,
			"mutableStateOf() as a plain local in a @Composable creates a fresh state on every recomposition; wrap it in remember { mutableStateOf(...) } or use `by remember { mutableStateOf(...) }`.",
		)}
	}
	return nil
}

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

func (r *ComposeStringResourceInsideLambdaRule) NodeTypes() []string {
	return []string{"call_expression"}
}

func (r *ComposeStringResourceInsideLambdaRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if flatCallExpressionName(file, idx) != "stringResource" {
		return nil
	}
	label := composeEnclosingLambdaArgumentName(file, idx)
	if label == "" {
		return nil
	}
	// Treat any `on*` handler argument as a callback context. Compose
	// composable lambdas (content parameters) are passed either positionally
	// or via the trailing-lambda syntax, which the enclosing-lambda walker
	// explicitly skips — so this heuristic only fires on named callback
	// arguments.
	if len(label) < 3 || label[0] != 'o' || label[1] != 'n' || label[2] < 'A' || label[2] > 'Z' {
		return nil
	}
	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"stringResource() is composition-only and will crash when invoked from a "+label+" callback; hoist the resource lookup above the lambda.",
	)}
}

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

func (r *ComposeMutableDefaultArgumentRule) NodeTypes() []string {
	return []string{"function_declaration"}
}

func (r *ComposeMutableDefaultArgumentRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !flatHasAnnotationNamed(file, idx, "Composable") {
		return nil
	}
	params := file.FlatFindChild(idx, "function_value_parameters")
	if params == 0 {
		return nil
	}

	// In the tree-sitter Kotlin grammar, `function_value_parameters` children
	// are a flat sequence: `parameter`, `=`, `<default_expr>`, `,`,
	// `parameter`, `=`, `<default_expr>`, ... — the default value is a
	// sibling of its parameter, not a child. Walk the sequence and flag any
	// `call_expression` that follows an `=` and names a mutable collection
	// factory.
	var findings []scanner.Finding
	sawEquals := false
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		typ := file.FlatType(child)
		switch typ {
		case "=":
			sawEquals = true
			continue
		case ",", "parameter", "(", ")":
			sawEquals = false
			continue
		}
		if !sawEquals {
			continue
		}
		sawEquals = false
		if typ != "call_expression" {
			continue
		}
		name := flatCallExpressionName(file, child)
		if _, ok := composeMutableCollectionFactories[name]; !ok {
			continue
		}
		findings = append(findings, r.Finding(
			file,
			file.FlatRow(child)+1,
			file.FlatCol(child)+1,
			"mutable collection default ("+name+"()) on a @Composable parameter re-evaluates each recomposition; use an immutable default like emptyList()/emptyMap() or hoist the collection to state.",
		))
	}
	return findings
}

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

func (r *ComposePreviewAnnotationMissingRule) NodeTypes() []string {
	return []string{"function_declaration"}
}

func (r *ComposePreviewAnnotationMissingRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !flatHasAnnotationNamed(file, idx, "Composable") {
		return nil
	}
	if flatHasAnnotationNamed(file, idx, "Preview") {
		return nil
	}
	nameNode := file.FlatFindChild(idx, "simple_identifier")
	if nameNode == 0 {
		return nil
	}
	name := file.FlatNodeText(nameNode)
	if !strings.HasSuffix(name, "Preview") {
		return nil
	}
	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"@Composable function named "+name+" is missing @Preview; add @Preview so Android Studio renders it in the preview pane.",
	)}
}
