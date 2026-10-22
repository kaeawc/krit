package rules

import (
	"bytes"

	api "github.com/kaeawc/krit/internal/rules/api"
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
func (r *ComposeColumnRowInScrollableRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *ComposeModifierFillAfterSizeRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *ComposeModifierBackgroundAfterClipRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *ComposeModifierClickableBeforePaddingRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *ComposeModifierPassedThenChainedRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *ComposeDisposableEffectMissingDisposeRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *ComposePreviewWithBackingStateRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *ComposeStatefulDefaultParameterRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *ComposeMutableStateInCompositionRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *ComposeStringResourceInsideLambdaRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *ComposeMutableDefaultArgumentRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *ComposePreviewAnnotationMissingRule) Confidence() float64 { return api.ConfidenceMedium }
