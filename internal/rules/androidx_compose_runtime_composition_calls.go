package rules

import (
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const acrCoroutinesPackage = "kotlinx.coroutines"

// CoroutineCreationDuringCompositionRule ports runtime-lint's CoroutineCreationDuringComposition: `launch`,
// `async` and `launchIn` from kotlinx.coroutines called within composition.
type CoroutineCreationDuringCompositionRule struct {
	FlatDispatchBase
	BaseRule
}

// FlowOperatorInvokedInCompositionRule ports runtime-lint's FlowOperatorInvokedInComposition: a non-composable
// extension on Flow that returns a Flow, called within composition.
type FlowOperatorInvokedInCompositionRule struct {
	FlatDispatchBase
	BaseRule
}

// StateFlowValueCalledInCompositionRule ports runtime-lint's StateFlowValueCalledInComposition: StateFlow.value
// accessed within composition.
type StateFlowValueCalledInCompositionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports medium confidence: the receiver and composition proofs are source-level.
func (r *CoroutineCreationDuringCompositionRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports medium confidence: the receiver and composition proofs are source-level.
func (r *FlowOperatorInvokedInCompositionRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports medium confidence: the receiver and composition proofs are source-level.
func (r *StateFlowValueCalledInCompositionRule) Confidence() float64 { return api.ConfidenceMedium }

// fileMentions gates the rule to files that can reference kotlinx.coroutines and the Compose runtime.
func (r *CoroutineCreationDuringCompositionRule) fileMentions() []string {
	return []string{acrCoroutinesPackage, "androidx.compose"}
}

// fileMentions gates the rule to files that can reference kotlinx.coroutines.flow and the Compose runtime.
func (r *FlowOperatorInvokedInCompositionRule) fileMentions() []string {
	return []string{acrFlowPackage, "androidx.compose"}
}

// fileMentions gates the rule to files that can reference kotlinx.coroutines.flow and the Compose runtime.
func (r *StateFlowValueCalledInCompositionRule) fileMentions() []string {
	return []string{acrFlowPackage, "androidx.compose"}
}

var acrCompositionLimitations = []string{
	"A lambda counts as composition only when the file proves its parameter is @Composable: a same-file declaration, a @Composable-annotated or -typed variable, or a known Compose content API (Column, Box, Scaffold, CompositionLocalProvider, lazy item content, ...).",
	"Receivers must be proven by a same-file declaration or initializer; values typed only by inference across files are not checked.",
}

func (r *CoroutineCreationDuringCompositionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "CoroutineCreationDuringComposition", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: acrCompositionLimitations}
}

func (r *FlowOperatorInvokedInCompositionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "FlowOperatorInvokedInComposition", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: acrCompositionLimitations}
}

func (r *StateFlowValueCalledInCompositionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "StateFlowValueCalledInComposition", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: acrCompositionLimitations}
}

func registerAndroidxComposeRuntimeCompositionCallRules() {
	kotlin := []scanner.Language{scanner.LangKotlin}
	coroutine := &CoroutineCreationDuringCompositionRule{BaseRule: BaseRule{RuleName: "CoroutineCreationDuringComposition",
		RuleSetName: "compose", Sev: "error",
		Desc: "Detects launch, async and launchIn calls made directly in composition instead of inside a LaunchedEffect."}}
	registerAcrRule(coroutine.BaseRule, coroutine, []string{"call_expression"}, kotlin, coroutine.check)

	flow := &FlowOperatorInvokedInCompositionRule{BaseRule: BaseRule{RuleName: "FlowOperatorInvokedInComposition",
		RuleSetName: "compose", Sev: "error",
		Desc: "Detects Flow operators such as map or filter invoked in composition, which creates a new Flow on every recomposition."}}
	registerAcrRule(flow.BaseRule, flow, []string{"call_expression"}, kotlin, flow.check)

	value := &StateFlowValueCalledInCompositionRule{BaseRule: BaseRule{RuleName: "StateFlowValueCalledInComposition",
		RuleSetName: "compose", Sev: "error",
		Desc: "Detects StateFlow.value read in composition, which does not observe changes; use collectAsState()."}}
	registerAcrRule(value.BaseRule, value, []string{"navigation_expression", "directly_assignable_expression"}, kotlin,
		value.check)
}

func (r *CoroutineCreationDuringCompositionRule) check(ctx *api.Context) {
	file, call := ctx.File, ctx.Idx
	if acrIsInnerCallOfTrailingLambda(file, call) {
		return
	}
	name, nameNode, hasReceiver := acrCallCallee(file, call)
	switch name {
	case "launchIn":
		if !hasReceiver || !acrCalleeResolvesTo(file, call, acrFlowPackage, name) {
			return
		}
	case "launch", "async":
		if !hasReceiver || !acrCalleeResolvesTo(file, call, acrCoroutinesPackage, name) ||
			!acrExprIsCoroutineScope(file, file.FlatFirstChild(acrCalleeExpression(file, call)), 0) {
			return
		}
	default:
		return
	}
	if !acrInComposition(file, call) {
		return
	}
	ctx.EmitAtNode(nameNode, "Calls to "+name+" should happen inside a LaunchedEffect and not composition")
}

// acrExprIsCoroutineScope proves that a call receiver is a kotlinx.coroutines CoroutineScope, so `launch`/`async`
// are the kotlinx extensions rather than lookalike members (ActivityResultLauncher.launch and the like).
func acrExprIsCoroutineScope(file *scanner.File, expr uint32, depth int) bool {
	expr = acrUnwrapParens(file, expr)
	if expr == 0 || depth > 4 {
		return false
	}
	switch file.FlatType(expr) {
	case "simple_identifier":
		name := file.FlatNodeText(expr)
		if decl := acrValueDeclaration(file, expr, name); decl != 0 {
			if t := acrDeclaredType(file, decl); t != 0 {
				return acrTypeIs(file, t, acrCoroutinesPackage, "CoroutineScope")
			}
			return acrExprIsCoroutineScope(file, acrInitializer(file, decl), depth+1)
		}
		if recv := acrScopeLambdaReceiver(file, expr); recv != 0 {
			return acrExprIsCoroutineScope(file, recv, depth+1)
		}
		return name == "GlobalScope" && acrNameResolvesTo(file, acrCoroutinesPackage, name)
	case "call_expression":
		return acrCallTargets(file, expr, acrRuntimePackage, "rememberCoroutineScope") ||
			acrCallTargets(file, expr, acrCoroutinesPackage, "CoroutineScope") ||
			acrCallTargets(file, expr, acrCoroutinesPackage, "MainScope")
	case "navigation_expression":
		if decl := acrExprDeclaration(file, expr); decl != 0 {
			if t := acrDeclaredType(file, decl); t != 0 {
				return acrTypeIs(file, t, acrCoroutinesPackage, "CoroutineScope")
			}
			return acrExprIsCoroutineScope(file, acrInitializer(file, decl), depth+1)
		}
	}
	return false
}

// acrFlowKind classifies an expression's proven Flow type.
type acrFlowKind int

const (
	acrNotFlow acrFlowKind = iota
	acrFlow
	acrStateFlow
)

// acrFlowOperators are the kotlinx.coroutines.flow extension functions on Flow that return a Flow, the shape
// Lint's isFlowOperator matches. stateIn and asStateFlow return a StateFlow.
var acrFlowOperators = map[string]acrFlowKind{
	"map": acrFlow, "mapNotNull": acrFlow, "mapLatest": acrFlow, "filter": acrFlow, "filterNot": acrFlow,
	"filterNotNull": acrFlow, "filterIsInstance": acrFlow, "onEach": acrFlow, "onStart": acrFlow,
	"onCompletion": acrFlow, "onEmpty": acrFlow, "catch": acrFlow, "retry": acrFlow, "retryWhen": acrFlow,
	"take": acrFlow, "takeWhile": acrFlow, "drop": acrFlow, "dropWhile": acrFlow, "distinctUntilChanged": acrFlow,
	"distinctUntilChangedBy": acrFlow, "debounce": acrFlow, "sample": acrFlow, "conflate": acrFlow, "buffer": acrFlow,
	"flowOn": acrFlow, "flatMapConcat": acrFlow, "flatMapMerge": acrFlow, "flatMapLatest": acrFlow,
	"flattenConcat": acrFlow, "flattenMerge": acrFlow, "transform": acrFlow, "transformLatest": acrFlow,
	"transformWhile": acrFlow, "scan": acrFlow, "runningFold": acrFlow, "runningReduce": acrFlow, "combine": acrFlow,
	"combineTransform": acrFlow, "zip": acrFlow, "withIndex": acrFlow, "cancellable": acrFlow, "timeout": acrFlow,
	"chunked": acrFlow, "shareIn": acrFlow, "asSharedFlow": acrFlow, "onSubscription": acrFlow,
	"stateIn": acrStateFlow, "asStateFlow": acrStateFlow,
}

// acrFlowFactories are kotlinx.coroutines.flow functions that create a Flow.
var acrFlowFactories = map[string]acrFlowKind{
	"flowOf": acrFlow, "emptyFlow": acrFlow, "flow": acrFlow, "channelFlow": acrFlow, "callbackFlow": acrFlow,
	"MutableSharedFlow": acrFlow, "MutableStateFlow": acrStateFlow,
}

// acrFlowTypes maps kotlinx.coroutines.flow type names to their kind.
var acrFlowTypes = map[string]acrFlowKind{
	"Flow": acrFlow, "SharedFlow": acrFlow, "MutableSharedFlow": acrFlow,
	"StateFlow": acrStateFlow, "MutableStateFlow": acrStateFlow,
}

// acrTypeFlowKind classifies a declared type node.
func acrTypeFlowKind(file *scanner.File, typeNode uint32) acrFlowKind {
	if typeNode == 0 {
		return acrNotFlow
	}
	for name, kind := range acrFlowTypes {
		if acrTypeNameIs(file, typeNode, acrFlowPackage, name) {
			return kind
		}
	}
	return acrClassFlowKind(file, acrSameFileClass(file, typeNode))
}

// acrClassFlowKind classifies a same-file class, object or object literal by its supertypes.
func acrClassFlowKind(file *scanner.File, class uint32) acrFlowKind {
	switch {
	case class == 0:
		return acrNotFlow
	case acrClassSupertypeIs(file, class, 0, acrFlowPackage, "StateFlow", "MutableStateFlow"):
		return acrStateFlow
	case acrClassSupertypeIs(file, class, 0, acrFlowPackage, "Flow", "SharedFlow", "MutableSharedFlow"):
		return acrFlow
	}
	return acrNotFlow
}

// acrExprFlowKind proves an expression's Flow type from same-file declarations, flow factories and flow operator
// chains.
func acrExprFlowKind(file *scanner.File, expr uint32, depth int) acrFlowKind {
	expr = acrUnwrapParens(file, expr)
	if expr == 0 || depth > 6 {
		return acrNotFlow
	}
	switch file.FlatType(expr) {
	case "simple_identifier", "navigation_expression":
		if decl := acrExprDeclaration(file, expr); decl != 0 {
			return acrDeclFlowKind(file, decl, depth)
		}
		if file.FlatType(expr) == "simple_identifier" {
			if recv := acrScopeLambdaReceiver(file, expr); recv != 0 {
				return acrExprFlowKind(file, recv, depth+1)
			}
		}
	case "object_literal":
		return acrClassFlowKind(file, expr)
	case "call_expression":
		name, _, hasReceiver := acrCallCallee(file, expr)
		if !hasReceiver {
			if kind, ok := acrFlowFactories[name]; ok && acrCalleeResolvesTo(file, expr, acrFlowPackage, name) {
				return kind
			}
			if class := flatFindSameFileClassLikeDeclaration(file, name); class != 0 {
				return acrClassFlowKind(file, class)
			}
			return acrNotFlow
		}
		if kind, ok := acrFlowOperatorCallAt(file, expr, depth+1); ok {
			return kind
		}
		if name == "asFlow" && acrCalleeResolvesTo(file, expr, acrFlowPackage, name) {
			return acrFlow
		}
	}
	return acrNotFlow
}

// acrDeclFlowKind classifies a declaration by its declared type, or by its initializer when it has none.
func acrDeclFlowKind(file *scanner.File, decl uint32, depth int) acrFlowKind {
	if t := acrDeclaredType(file, decl); t != 0 {
		return acrTypeFlowKind(file, t)
	}
	return acrExprFlowKind(file, acrInitializer(file, decl), depth+1)
}

// acrFlowOperatorCall reports whether call is `receiver.op(...)` with op a kotlinx.coroutines.flow operator (or a
// same-file non-composable Flow extension returning a Flow) on a proven Flow receiver, and the kind it returns.
func acrFlowOperatorCall(file *scanner.File, call uint32) (acrFlowKind, bool) {
	return acrFlowOperatorCallAt(file, call, 0)
}

func acrFlowOperatorCallAt(file *scanner.File, call uint32, depth int) (acrFlowKind, bool) {
	name, _, hasReceiver := acrCallCallee(file, call)
	if !hasReceiver || name == "" {
		return acrNotFlow, false
	}
	// Cheap name and declaration checks first; proving the receiver is a Flow walks declarations.
	var kind acrFlowKind
	if decl := acrSameFileFlowOperator(file, name); decl != 0 {
		if acrFunctionIsComposable(file, decl) {
			return acrNotFlow, false
		}
		kind = acrTypeFlowKind(file, acrFunctionReturnType(file, decl))
	} else if k, ok := acrFlowOperators[name]; ok && !acrFileDeclaresAnyFunction(file, name) &&
		acrNameResolvesTo(file, acrFlowPackage, name) {
		kind = k
	} else {
		return acrNotFlow, false
	}
	receiver := file.FlatFirstChild(acrCalleeExpression(file, call))
	if acrExprFlowKind(file, receiver, depth) == acrNotFlow {
		return acrNotFlow, false
	}
	return kind, true
}

// acrSameFileFlowOperator returns the single same-file extension function with the name whose receiver and return
// types are Flow types, or 0.
func acrSameFileFlowOperator(file *scanner.File, name string) uint32 {
	decl := acrSameFileFunction(file, name)
	if decl == 0 || !flatFunctionDeclarationIsExtension(file, decl) {
		return 0
	}
	if acrTypeFlowKind(file, acrFunctionReceiverType(file, decl)) == acrNotFlow ||
		acrTypeFlowKind(file, acrFunctionReturnType(file, decl)) == acrNotFlow {
		return 0
	}
	return decl
}

// acrFunctionReceiverType returns the receiver type node of an extension function declaration.
func acrFunctionReceiverType(file *scanner.File, fn uint32) uint32 {
	for child := file.FlatFirstChild(fn); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "user_type", "nullable_type":
			return child
		case "simple_identifier", "function_value_parameters":
			return 0
		}
	}
	return 0
}

// acrFunctionReturnType returns the declared return type node of a function declaration, or 0.
func acrFunctionReturnType(file *scanner.File, fn uint32) uint32 {
	afterParams, sawColon := false, false
	for child := file.FlatFirstChild(fn); child != 0; child = file.FlatNextSib(child) {
		switch t := file.FlatType(child); {
		case t == "function_value_parameters":
			afterParams = true
		case afterParams && t == ":":
			sawColon = true
		case sawColon && (t == "user_type" || t == "nullable_type"):
			return child
		case t == "function_body":
			return 0
		}
	}
	return 0
}

func (r *FlowOperatorInvokedInCompositionRule) check(ctx *api.Context) {
	file, call := ctx.File, ctx.Idx
	if acrIsInnerCallOfTrailingLambda(file, call) {
		return
	}
	if _, ok := acrFlowOperatorCall(file, call); !ok || !acrInComposition(file, call) {
		return
	}
	_, nameNode, _ := acrCallCallee(file, call)
	ctx.EmitAtNode(nameNode, "Flow operator functions should not be invoked within composition")
}

func (r *StateFlowValueCalledInCompositionRule) check(ctx *api.Context) {
	file, node := ctx.File, ctx.Idx
	receiver := file.FlatFirstChild(node)
	suffix := flatLastNamedChild(file, node)
	if receiver == 0 || suffix == 0 || file.FlatType(suffix) != "navigation_suffix" {
		return
	}
	valueNode, _ := file.FlatFindChild(suffix, "simple_identifier")
	if valueNode == 0 || !file.FlatNodeTextEquals(valueNode, "value") {
		return
	}
	kind := acrNotFlow
	if file.FlatType(node) == "directly_assignable_expression" {
		// `a.b.value = x` keeps every suffix flat under the assignable expression.
		if decl := acrAssignableReceiverDeclaration(file, node, suffix); decl != 0 {
			kind = acrDeclFlowKind(file, decl, 0)
		}
	} else {
		kind = acrExprFlowKind(file, receiver, 0)
	}
	if kind != acrStateFlow || !acrInComposition(file, node) {
		return
	}
	ctx.EmitAtNode(valueNode, "StateFlow.value should not be called within composition")
}
