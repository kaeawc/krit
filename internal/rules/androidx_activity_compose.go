package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Ports of ActivityResultLaunchDetector, CollectProgressDetector and LocalContextCastIssueDetector from
// androidx.activity:activity-compose 1.13.0.

const axlActivityComposePackage = "androidx.activity.compose"

// LaunchDuringCompositionRule ports LaunchDuringComposition: ManagedActivityResultLauncher.launch called while
// composing, which launches again on every recomposition.
type LaunchDuringCompositionRule struct {
	FlatDispatchBase
	BaseRule
}

// NoCollectCallFoundRule ports NoCollectCallFound: a PredictiveBackHandler onBack lambda that never collects the
// progress Flow, so it runs as soon as the gesture starts.
type NoCollectCallFoundRule struct {
	FlatDispatchBase
	BaseRule
}

// ContextCastToActivityRule ports ContextCastToActivity: LocalContext.current cast to an Activity, which a Context
// is not always.
type ContextCastToActivityRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *LaunchDuringCompositionRule) fileMentions() []string {
	return []string{axlActivityComposePackage}
}
func (r *NoCollectCallFoundRule) fileMentions() []string    { return []string{"PredictiveBackHandler"} }
func (r *ContextCastToActivityRule) fileMentions() []string { return []string{"LocalContext"} }

// Confidence is medium: the receiver and composition proofs are source-level.
func (r *LaunchDuringCompositionRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *LaunchDuringCompositionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "LaunchDuringComposition", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: append([]string{
			"The launcher must be proven to be a ManagedActivityResultLauncher: a variable declared as one or " +
				"initialized by rememberLauncherForActivityResult.",
			"As in Lint, only launch(input, options) is ManagedActivityResultLauncher's own method; launch(input) " +
				"is ActivityResultLauncher's and is not reported.",
		}, acrCompositionLimitations...)}
}

func (r *NoCollectCallFoundRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "NoCollectCallFound", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: []string{
			"As in Lint, any name collect, collectLatest or collectIndexed in the onBack lambda counts as collecting.",
		}}
}

func (r *ContextCastToActivityRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ContextCastToActivity", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: []string{
			"The cast type counts as an Activity when it is a known Activity class (android.app.Activity, " +
				"ComponentActivity, FragmentActivity, AppCompatActivity), a same-file subclass of one, or another " +
				"class whose name ends in Activity.",
		}}
}

// check mirrors ActivityResultLaunchDetector: launch(input, options) on a ManagedActivityResultLauncher within
// composition, at the call name.
func (r *LaunchDuringCompositionRule) check(ctx *api.Context) {
	file, node := ctx.File, ctx.Idx
	if acrIsInnerCallOfTrailingLambda(file, node) {
		return
	}
	name, nameNode, hasReceiver := acrCallCallee(file, node)
	if name != "launch" || !hasReceiver {
		return
	}
	call, ok := lintCallAt(file, node)
	if !ok || len(call.Args) != 2 || acrHasTrailingLambda(file, node) {
		return
	}
	receiver := file.FlatFirstChild(acrCalleeExpression(file, node))
	if !axlExprIsManagedLauncher(file, receiver, 0) || !acrInComposition(file, node) {
		return
	}
	ctx.EmitAtNode(nameNode, "Calls to `launch` should happen inside of a SideEffect and not during composition")
}

// axlActivityCallTargets reports whether call is the androidx.activity.compose function name, directly, fully
// qualified, or through an import alias (`import ...rememberLauncherForActivityResult as rl`).
func axlActivityCallTargets(file *scanner.File, call uint32, name string) bool {
	if acrCallTargets(file, call, axlActivityComposePackage, name) {
		return true
	}
	callee, _, hasReceiver := acrCallCallee(file, call)
	return !hasReceiver && callee != "" && callee != name &&
		acrSimpleNameRefersTo(file, callee, axlActivityComposePackage, name) &&
		!acrFunctionNameShadowed(file, call, callee) && !acrFileDeclaresTopLevel(file, callee)
}

// axlExprIsManagedLauncher proves an expression is an androidx.activity.compose.ManagedActivityResultLauncher.
func axlExprIsManagedLauncher(file *scanner.File, expr uint32, depth int) bool {
	expr = acrUnwrapParens(file, expr)
	if expr == 0 || depth > 4 {
		return false
	}
	switch file.FlatType(expr) {
	case "call_expression":
		return axlActivityCallTargets(file, expr, "rememberLauncherForActivityResult")
	case "simple_identifier", "navigation_expression":
		decl := acrExprDeclaration(file, expr)
		if decl == 0 {
			return false
		}
		if t := acrDeclaredType(file, decl); t != 0 {
			return acrTypeIs(file, t, axlActivityComposePackage, "ManagedActivityResultLauncher")
		}
		return axlExprIsManagedLauncher(file, acrInitializer(file, decl), depth+1)
	}
	return false
}

// check mirrors CollectProgressDetector on PredictiveBackHandler's onBack lambda: an unreferenced progress
// parameter is reported at the parameter (or the lambda, for `it`); otherwise, when no name in the lambda is
// collect, collectLatest or collectIndexed, at the lambda's first name reference with Lint's "Flow null" message.
func (r *NoCollectCallFoundRule) check(ctx *api.Context) {
	file, lambda := ctx.File, ctx.Idx
	arg, ok := acrLambdaArgumentOf(file, lambda)
	// onBack is PredictiveBackHandler(enabled, onBack)'s second parameter: trailing, named or second positional.
	isOnBack := arg.Trailing || arg.Label == "onBack" || (arg.Label == "" && arg.Index == 1)
	if !ok || !isOnBack || !axlActivityCallTargets(file, arg.Call, "PredictiveBackHandler") {
		return
	}
	if unused := axlUnreferencedLambdaParams(file, lambda, 1); len(unused) > 0 {
		ctx.EmitAtNode(unused[0].Node, "You must call collect() on Flow "+unused[0].Name)
		return
	}
	first, collects := axlLambdaNameReferences(file, lambda, func(name string) bool {
		return name == "collect" || name == "collectLatest" || name == "collectIndexed"
	})
	if collects {
		return
	}
	at := lambda
	if first != 0 {
		at = first
	}
	ctx.EmitAtNode(at, "You must call collect() on Flow null")
}

// axlLambdaNameReferences walks the name references of a lambda in source order (as collecting its
// KtSimpleNameExpressions does: names in expressions and types, not declared parameter names) and returns the
// first one and whether any matches.
func axlLambdaNameReferences(file *scanner.File, lambda uint32, match func(string) bool) (first uint32, found bool) {
	var walk func(n uint32)
	walk = func(n uint32) {
		for child := file.FlatFirstChild(n); child != 0 && !found; child = file.FlatNextSib(child) {
			switch file.FlatType(child) {
			case "variable_declaration":
				// The declared name is not a reference; its type is.
				for c := file.FlatFirstChild(child); c != 0; c = file.FlatNextSib(c) {
					if file.FlatType(c) != "simple_identifier" {
						walk(c)
					}
				}
				continue
			case "simple_identifier", "type_identifier", "interpolated_identifier":
				if first == 0 {
					first = child
				}
				if match(strings.Trim(file.FlatNodeText(child), "`")) {
					found = true
				}
				continue
			}
			walk(child)
		}
	}
	walk(lambda)
	return first, found
}

// axlActivityClasses are the library Activity classes a cast type can name.
var axlActivityClasses = map[string]string{
	"Activity": "android.app", "ComponentActivity": "androidx.activity", "FragmentActivity": "androidx.fragment.app",
	"AppCompatActivity": "androidx.appcompat.app",
}

// check mirrors LocalContextCastIssueDetector: `LocalContext.current as Activity` (or `as?`), with LocalContext
// from androidx.compose.ui.platform and an Activity type, at the cast expression.
func (r *ContextCastToActivityRule) check(ctx *api.Context) {
	file, node := ctx.File, ctx.Idx
	operand := file.FlatFirstChild(node)
	var typeNode uint32
	for child := file.FlatNextSib(operand); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) {
			typeNode = child
		}
	}
	if operand == 0 || typeNode == 0 || !axlIsLocalContextCurrent(file, operand) || !axlTypeIsActivity(file, typeNode) {
		return
	}
	ctx.EmitAtNode(node, "LocalContext should not be cast to Activity, use LocalActivity instead")
}

// axlIsLocalContextCurrent reports whether expr is `LocalContext.current`, with LocalContext resolving to
// androidx.compose.ui.platform. A parenthesized receiver is not unwrapped, as in Lint.
func axlIsLocalContextCurrent(file *scanner.File, expr uint32) bool {
	if file.FlatType(expr) != "navigation_expression" {
		return false
	}
	suffix := flatLastNamedChild(file, expr)
	if suffix == 0 || file.FlatType(suffix) != "navigation_suffix" ||
		!file.FlatNodeTextEquals(file.FlatNamedChild(suffix, 0), "current") {
		return false
	}
	receiver := file.FlatFirstChild(expr)
	text := strings.Join(strings.Fields(file.FlatNodeText(receiver)), "")
	if text == "androidx.compose.ui.platform.LocalContext" {
		return true
	}
	return file.FlatType(receiver) == "simple_identifier" &&
		acrSimpleNameRefersTo(file, text, "androidx.compose.ui.platform", "LocalContext") &&
		acrValueDeclaration(file, receiver, text) == 0
}

// axlTypeIsActivity reports whether a cast type is an Activity: a library Activity class, a same-file class
// extending one, or a class named *Activity.
func axlTypeIsActivity(file *scanner.File, typeNode uint32) bool {
	name := acrTypeNodeName(file, typeNode)
	simple := name[strings.LastIndex(name, ".")+1:]
	if simple == "" {
		return false
	}
	for cls, pkg := range axlActivityClasses {
		if acrTypeNameIs(file, typeNode, pkg, cls) {
			return true
		}
	}
	if class := acrSameFileClass(file, typeNode); class != 0 {
		for cls, pkg := range axlActivityClasses {
			if acrClassSupertypeIs(file, class, 0, pkg, cls) {
				return true
			}
		}
		return false
	}
	return strings.HasSuffix(simple, "Activity")
}
