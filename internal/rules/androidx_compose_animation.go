package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Ports of CrossfadeDetector and AnimatedContentDetector from androidx.compose.animation:animation-android 1.13.0,
// and TransitionDetector from androidx.compose.animation:animation-core-android 1.13.0.

const (
	axlAnimationPackage     = "androidx.compose.animation"
	axlAnimationCorePackage = "androidx.compose.animation.core"
	axlTargetStateMessage   = "Target state parameter `%s` is not used"
)

// UnusedCrossfadeTargetStateParameterRule ports UnusedCrossfadeTargetStateParameter: Crossfade's lambda ignores
// the target state it is given, so the same content fades into itself.
type UnusedCrossfadeTargetStateParameterRule struct {
	FlatDispatchBase
	BaseRule
}

// UnusedContentLambdaTargetStateParameterRule ports UnusedContentLambdaTargetStateParameter: AnimatedContent's
// content lambda ignores the target state it is given.
type UnusedContentLambdaTargetStateParameterRule struct {
	FlatDispatchBase
	BaseRule
}

// UnusedTargetStateInContentKeyLambdaRule ports UnusedTargetStateInContentKeyLambda: AnimatedContent's contentKey
// lambda ignores the target state it is given.
type UnusedTargetStateInContentKeyLambdaRule struct {
	FlatDispatchBase
	BaseRule
}

// UnusedTransitionTargetStateParameterRule ports UnusedTransitionTargetStateParameter: a Transition.animate* lambda
// computes its value without the state it is given.
type UnusedTransitionTargetStateParameterRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *UnusedCrossfadeTargetStateParameterRule) fileMentions() []string {
	return []string{axlAnimationPackage}
}
func (r *UnusedContentLambdaTargetStateParameterRule) fileMentions() []string {
	return []string{axlAnimationPackage}
}
func (r *UnusedTargetStateInContentKeyLambdaRule) fileMentions() []string {
	return []string{axlAnimationPackage}
}
func (r *UnusedTransitionTargetStateParameterRule) fileMentions() []string {
	return []string{axlAnimationPackage}
}

var axlTargetStateLimitations = []string{
	"An inner lambda without parameters hides an outer `it` only when it is passed to a known one-parameter " +
		"lambda function (forEach, let, map, LazyListScope.items, ...) or a same-file function whose parameter type " +
		"takes one value; Lint resolves every inner lambda.",
}

func (r *UnusedCrossfadeTargetStateParameterRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UnusedCrossfadeTargetStateParameter", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: axlTargetStateLimitations}
}

func (r *UnusedContentLambdaTargetStateParameterRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UnusedContentLambdaTargetStateParameter", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: append([]string{
			"The content lambda must be passed as the trailing lambda or as `content = { }`.",
		}, axlTargetStateLimitations...)}
}

func (r *UnusedTargetStateInContentKeyLambdaRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UnusedTargetStateInContentKeyLambda", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: append([]string{
			"The contentKey lambda must be passed as `contentKey = { }`.",
		}, axlTargetStateLimitations...)}
}

func (r *UnusedTransitionTargetStateParameterRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UnusedTransitionTargetStateParameter", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: append([]string{
			"The receiver must be proven to be a Transition: a variable declared as one or initialized by " +
				"updateTransition/rememberTransition, a Transition extension's receiver (not through a with/apply/run " +
				"lambda in between), or AnimatedVisibilityScope's `transition` inside an AnimatedVisibility or " +
				"AnimatedContent lambda.",
			"The state lambda must be the trailing lambda or `targetValueByState = { }` (or, for a same-file " +
				"animate* extension, any lambda for a one-parameter function type).",
		}, axlTargetStateLimitations...)}
}

// axlReportUnusedTargetState reports each unreferenced parameter of a one-parameter target-state lambda.
func axlReportUnusedTargetState(ctx *api.Context, lambda uint32) {
	for _, p := range axlUnreferencedLambdaParams(ctx.File, lambda) {
		ctx.EmitAtNode(p.Node, strings.Replace(axlTargetStateMessage, "%s", p.Name, 1))
	}
}

// axlAnimationCallTargets reports whether call is the androidx.compose.animation function name: a simple or fully
// qualified callee, an import alias of it (`import ...Crossfade as Fade`), or an extension call
// (`transition.Crossfade { }`) whose name is imported from the package.
func axlAnimationCallTargets(file *scanner.File, call uint32, name string) bool {
	if acrCallTargets(file, call, axlAnimationPackage, name) {
		return true
	}
	callee, _, hasReceiver := acrCallCallee(file, call)
	if callee == "" || !acrSimpleNameRefersTo(file, callee, axlAnimationPackage, name) {
		return false
	}
	return hasReceiver || (callee != name && !acrFunctionNameShadowed(file, call, callee) &&
		!acrFileDeclaresTopLevel(file, callee))
}

// check mirrors CrossfadeDetector: the first lambda argument of a Crossfade call (content, or contentKey when it
// is passed first) has an unreferenced parameter.
func (r *UnusedCrossfadeTargetStateParameterRule) check(ctx *api.Context) {
	file, lambda := ctx.File, ctx.Idx
	arg, ok := acrLambdaArgumentOf(file, lambda)
	if !ok || !axlIsFirstLambdaArgument(file, arg.Call, lambda) || !axlAnimationCallTargets(file, arg.Call, "Crossfade") {
		return
	}
	axlReportUnusedTargetState(ctx, lambda)
}

// axlIsFirstLambdaArgument reports whether lambda is the first lambda literal among call's value arguments and
// trailing lambda, in source order.
func axlIsFirstLambdaArgument(file *scanner.File, call, lambda uint32) bool {
	for _, arg := range axlCallArgumentExpressions(file, call) {
		if file.FlatType(arg) == "lambda_literal" {
			return arg == lambda
		}
	}
	return false
}

// axlCallArgumentExpressions returns a call's value argument expressions followed by its trailing lambda, looking
// through the inner call of `foo(args) { }`.
func axlCallArgumentExpressions(file *scanner.File, call uint32) []uint32 {
	var out []uint32
	var suffixes []uint32
	for c := call; c != 0 && file.FlatType(c) == "call_expression"; {
		if suffix, ok := file.FlatFindChild(c, "call_suffix"); ok {
			suffixes = append([]uint32{suffix}, suffixes...)
		}
		inner := file.FlatFirstChild(c)
		if inner == 0 || file.FlatType(inner) != "call_expression" || !acrIsTrailingLambdaOnlySuffix(file, file.FlatNextSib(inner)) {
			break
		}
		c = inner
	}
	for _, suffix := range suffixes {
		for child := file.FlatFirstChild(suffix); child != 0; child = file.FlatNextSib(child) {
			switch file.FlatType(child) {
			case "value_arguments":
				for arg := file.FlatFirstChild(child); arg != 0; arg = file.FlatNextSib(arg) {
					if file.FlatType(arg) == "value_argument" {
						if expr := axlUnwrapLambdaPrefix(file, flatValueArgumentExpression(file, arg)); expr != 0 {
							out = append(out, expr)
						}
					}
				}
			case "annotated_lambda":
				if l, ok := file.FlatFindChild(child, "lambda_literal"); ok {
					out = append(out, l)
				}
			}
		}
	}
	return out
}

// axlUnwrapLambdaPrefix returns the lambda literal of a labeled or annotated lambda expression (`label@{ }`), or
// the expression itself.
func axlUnwrapLambdaPrefix(file *scanner.File, expr uint32) uint32 {
	for expr != 0 && file.FlatType(expr) == "prefix_expression" {
		expr = flatLastNamedChild(file, expr)
	}
	return expr
}

// check mirrors AnimatedContentDetector for the content lambda.
func (r *UnusedContentLambdaTargetStateParameterRule) check(ctx *api.Context) {
	file, lambda := ctx.File, ctx.Idx
	call, ok := axlContentLambda(file, lambda, "content", true)
	if !ok || !axlAnimationCallTargets(file, call, "AnimatedContent") {
		return
	}
	axlReportUnusedTargetState(ctx, lambda)
}

// check mirrors AnimatedContentDetector for the contentKey lambda.
func (r *UnusedTargetStateInContentKeyLambdaRule) check(ctx *api.Context) {
	file, lambda := ctx.File, ctx.Idx
	call, ok := axlContentLambda(file, lambda, "contentKey", false)
	if !ok || !axlAnimationCallTargets(file, call, "AnimatedContent") {
		return
	}
	axlReportUnusedTargetState(ctx, lambda)
}

// axlTransitionAnimateFunctions are the Transition.animate* extensions of animation-core and animation, whose
// last parameter, targetValueByState, takes the state; their other lambda, transitionSpec, takes none.
var axlTransitionAnimateFunctions = map[string]bool{
	"animateFloat": true, "animateDp": true, "animateColor": true, "animateValue": true, "animateInt": true,
	"animateIntOffset": true, "animateIntSize": true, "animateOffset": true, "animateRect": true,
	"animateSize": true,
}

// check mirrors TransitionDetector: a lambda of a Transition.animate* call that takes the state (`(S) -> T`)
// never refers to it.
func (r *UnusedTransitionTargetStateParameterRule) check(ctx *api.Context) {
	file, lambda := ctx.File, ctx.Idx
	arg, ok := acrLambdaArgumentOf(file, lambda)
	if !ok {
		return
	}
	name, _, hasReceiver := acrCallCallee(file, arg.Call)
	if !strings.HasPrefix(name, "animate") || !axlTakesTransitionState(file, arg, name) {
		return
	}
	var receiver uint32
	if hasReceiver {
		receiver = file.FlatFirstChild(acrCalleeExpression(file, arg.Call))
	}
	if !axlReceiverIsTransition(file, arg.Call, receiver) {
		return
	}
	axlReportUnusedTargetState(ctx, lambda)
}

// axlTakesTransitionState reports whether the lambda passed as arg is the state lambda of the animate* call: for
// the library functions, the trailing lambda or `targetValueByState = { }`; for a same-file function, a parameter
// whose function type takes one value.
func axlTakesTransitionState(file *scanner.File, arg acrLambdaArgument, name string) bool {
	if decl := acrSameFileFunction(file, name); decl != 0 {
		param := acrParameterForArgument(file, decl, arg)
		return param != 0 && axlFunctionTypeArity(file, param) == 1
	}
	if !axlTransitionAnimateFunctions[name] || acrFileDeclaresAnyFunction(file, name) {
		return false
	}
	return arg.Trailing || arg.Label == "targetValueByState"
}

// axlReceiverIsTransition proves that a call's receiver (or, with no receiver, the implicit one) is an
// androidx.compose.animation.core.Transition. An implicit or `this` receiver is the enclosing Transition extension's,
// unless a with/apply/run lambda in between brings a closer receiver whose members could take the call.
func axlReceiverIsTransition(file *scanner.File, call, receiver uint32) bool {
	if receiver == 0 || file.FlatType(receiver) == "this_expression" {
		fn := axlEnclosingFunction(file, call)
		return fn != 0 && !axlReceiverLambdaBetween(file, call, fn) &&
			axlTypeIsTransition(file, acrFunctionReceiverType(file, fn))
	}
	return axlExprIsTransition(file, receiver, 0)
}

// axlReceiverScopeFunctions are the stdlib scope functions whose lambda has a receiver.
var axlReceiverScopeFunctions = map[string]bool{"with": true, "apply": true, "run": true}

// axlReceiverLambdaBetween reports whether a lambda passed to with/apply/run encloses node inside fn.
func axlReceiverLambdaBetween(file *scanner.File, node, fn uint32) bool {
	for p, ok := file.FlatParent(node); ok && p != fn; p, ok = file.FlatParent(p) {
		if file.FlatType(p) != "lambda_literal" {
			continue
		}
		if arg, ok := acrLambdaArgumentOf(file, p); ok {
			if name, _, _ := acrCallCallee(file, arg.Call); axlReceiverScopeFunctions[name] {
				return true
			}
		}
	}
	return false
}

// axlEnclosingFunction returns the nearest enclosing function declaration, or 0.
func axlEnclosingFunction(file *scanner.File, node uint32) uint32 {
	for p, ok := file.FlatParent(node); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "function_declaration":
			return p
		case "class_body", "source_file":
			return 0
		}
	}
	return 0
}

// axlTypeIsTransition reports whether a type node names androidx.compose.animation.core.Transition.
func axlTypeIsTransition(file *scanner.File, typeNode uint32) bool {
	return typeNode != 0 && acrTypeIs(file, typeNode, axlAnimationCorePackage, "Transition")
}

// axlTransitionFactories are the animation-core functions that return a Transition.
var axlTransitionFactories = []string{"updateTransition", "rememberTransition"}

// axlExprIsTransition proves an expression is a Transition: a call of a Transition factory, a variable declared as
// one or initialized by such an expression, or AnimatedVisibilityScope's `transition` in an AnimatedVisibility or
// AnimatedContent content lambda.
func axlExprIsTransition(file *scanner.File, expr uint32, depth int) bool {
	expr = acrUnwrapParens(file, expr)
	if expr == 0 || depth > 4 {
		return false
	}
	switch file.FlatType(expr) {
	case "call_expression":
		for _, name := range axlTransitionFactories {
			if acrCallTargets(file, expr, axlAnimationCorePackage, name) {
				return true
			}
		}
	case "simple_identifier", "navigation_expression":
		if decl := acrExprDeclaration(file, expr); decl != 0 {
			if t := acrDeclaredType(file, decl); t != 0 {
				return axlTypeIsTransition(file, t)
			}
			return axlExprIsTransition(file, acrInitializer(file, decl), depth+1)
		}
		if file.FlatType(expr) == "simple_identifier" && file.FlatNodeTextEquals(expr, "transition") {
			return axlInAnimatedVisibilityScope(file, expr)
		}
	}
	return false
}

// axlInAnimatedVisibilityScope reports whether node sits in the content lambda of an AnimatedVisibility or
// AnimatedContent call, whose receiver scope has a `transition` property.
func axlInAnimatedVisibilityScope(file *scanner.File, node uint32) bool {
	for p, ok := file.FlatParent(node); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "lambda_literal":
			if arg, ok := acrLambdaArgumentOf(file, p); ok && (arg.Trailing || arg.Label == "content") &&
				(axlAnimationCallTargets(file, arg.Call, "AnimatedVisibility") ||
					axlAnimationCallTargets(file, arg.Call, "AnimatedContent")) {
				return true
			}
		case "function_declaration", "class_body", "source_file":
			return false
		}
	}
	return false
}
