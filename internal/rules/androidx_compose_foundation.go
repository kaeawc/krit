package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Ports of NonLambdaOffsetModifierDetector, BoxWithConstraintsDetector and TextFieldBufferAppendDetector from
// androidx.compose.foundation:foundation-android 1.13.0.

const (
	axlFoundationLayoutPackage = "androidx.compose.foundation.layout"
	axlTextInputPackage        = "androidx.compose.foundation.text.input"
)

// UseOfNonLambdaOffsetOverloadRule ports UseOfNonLambdaOffsetOverload: Modifier.offset(x, y) with State-backed
// arguments, which recomposes on every change where offset { } only relayouts.
type UseOfNonLambdaOffsetOverloadRule struct {
	FlatDispatchBase
	BaseRule
}

// UnusedBoxWithConstraintsScopeRule ports UnusedBoxWithConstraintsScope: a BoxWithConstraints whose content never
// reads its constraints, paying for subcomposition a Box would not need.
type UnusedBoxWithConstraintsScopeRule struct {
	FlatDispatchBase
	BaseRule
}

// TextFieldBufferInternalAppendRule ports TextFieldBufferInternalAppend: TextFieldBuffer.append in
// androidx.compose.foundation's own code, which drops hardware-source tracking.
type TextFieldBufferInternalAppendRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *UseOfNonLambdaOffsetOverloadRule) fileMentions() []string {
	return []string{axlFoundationLayoutPackage}
}
func (r *UnusedBoxWithConstraintsScopeRule) fileMentions() []string {
	return []string{"BoxWithConstraints"}
}
func (r *TextFieldBufferInternalAppendRule) fileMentions() []string {
	return []string{"package androidx.compose.foundation"}
}

// Confidence is medium: State-backed arguments are proven from same-file declarations.
func (r *UseOfNonLambdaOffsetOverloadRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *UseOfNonLambdaOffsetOverloadRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UseOfNonLambdaOffsetOverload", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: []string{
			"An argument is State-backed when it reads a value declared or initialized as a State, MutableState or " +
				"Animatable (mutableStateOf, remember { ... }, animate*AsState, derivedStateOf, collectAsState, " +
				"Animatable(...)), the .value of one, or a local delegated `by` one; values typed only in other " +
				"files are not proven.",
		}}
}

func (r *UnusedBoxWithConstraintsScopeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UnusedBoxWithConstraintsScope", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: []string{
			"Scope use is a `this`, a read of maxWidth, maxHeight, minWidth, minHeight or constraints, a call of a " +
				"same-file BoxWithConstraintsScope extension, or an invocation of a value typed with a " +
				"BoxWithConstraintsScope receiver; such extensions declared in other files are not seen.",
		}}
}

func (r *TextFieldBufferInternalAppendRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "TextFieldBufferInternalAppend", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: []string{
			"Only applies to code in androidx.compose.foundation packages, as in Lint.",
		}}
}

// check mirrors NonLambdaOffsetModifierDetector: Modifier.offset(x, y) or absoluteOffset(x, y) from
// androidx.compose.foundation.layout (the Dp overload) with an argument that reads State, at the call name.
func (r *UseOfNonLambdaOffsetOverloadRule) check(ctx *api.Context) {
	file, node := ctx.File, ctx.Idx
	// The callee name rejects almost every call, so test it before anything that walks the tree.
	name := flatCallExpressionName(file, node)
	if name == "" {
		return
	}
	target := name
	if name != "offset" && name != "absoluteOffset" {
		// An import alias (`import ...offset as off`) still targets the foundation function.
		alias := fileFactsCache().Imports(file).Aliases[name]
		if alias != axlFoundationLayoutPackage+".offset" && alias != axlFoundationLayoutPackage+".absoluteOffset" {
			return
		}
		target = alias[strings.LastIndex(alias, ".")+1:]
	}
	if acrIsInnerCallOfTrailingLambda(file, node) || acrHasTrailingLambda(file, node) {
		return
	}
	name, nameNode, hasReceiver := acrCallCallee(file, node)
	if name == "" || len(acrFunctionDecls(file, name)) > 0 ||
		!acrSimpleNameRefersTo(file, name, axlFoundationLayoutPackage, target) {
		return
	}
	if hasReceiver && axlReceiverProvenNotModifier(file, file.FlatFirstChild(acrCalleeExpression(file, node))) {
		return
	}
	call, ok := lintCallAt(file, node)
	if !ok || len(call.Args) == 0 || len(call.Args) > 2 {
		return
	}
	for i, label := range call.Labels {
		if label != "" && label != "x" && label != "y" {
			return
		}
		if label == "" && i > 1 {
			return
		}
	}
	for _, arg := range call.Args {
		if axlReadsState(file, arg) {
			ctx.EmitAtNode(nameNode, "State backed values should use the lambda overload of Modifier.offset")
			return
		}
	}
}

// axlReceiverProvenNotModifier reports whether a call receiver is a value whose same-file declaration gives it a
// type other than androidx.compose.ui.Modifier (android.graphics.Rect.offset and the like).
func axlReceiverProvenNotModifier(file *scanner.File, receiver uint32) bool {
	receiver = acrUnwrapParens(file, receiver)
	if receiver == 0 || file.FlatType(receiver) != "simple_identifier" {
		return false
	}
	decl := acrValueDeclaration(file, receiver, file.FlatNodeText(receiver))
	if decl == 0 {
		return false
	}
	t := acrDeclaredType(file, decl)
	return t != 0 && !acrTypeIs(file, t, "androidx.compose.ui", "Modifier")
}

// axlReadsState reports whether any name in expr refers to a State-backed declaration, or reads `.value` of one.
func axlReadsState(file *scanner.File, expr uint32) bool {
	found := false
	file.FlatWalkAllNodes(expr, func(n uint32) {
		if found || file.FlatType(n) != "simple_identifier" {
			return
		}
		if p, ok := file.FlatParent(n); ok && file.FlatType(p) == "navigation_suffix" {
			return
		}
		decl := acrValueDeclaration(file, n, file.FlatNodeText(n))
		found = decl != 0 && axlDeclIsStateBacked(file, decl)
	})
	return found
}

// axlStateTypes are the runtime and animation types whose reads Lint treats as composition-aware.
var axlStateTypes = map[string][]string{
	"androidx.compose.runtime": {"State", "MutableState", "IntState", "MutableIntState", "FloatState",
		"MutableFloatState", "LongState", "MutableLongState", "DoubleState", "MutableDoubleState"},
	"androidx.compose.animation.core": {"Animatable"},
}

// axlStateFactories are the calls that return a State or Animatable.
var axlStateFactories = map[string]string{
	"mutableStateOf": "androidx.compose.runtime", "mutableIntStateOf": "androidx.compose.runtime",
	"mutableFloatStateOf": "androidx.compose.runtime", "mutableLongStateOf": "androidx.compose.runtime",
	"mutableDoubleStateOf": "androidx.compose.runtime", "derivedStateOf": "androidx.compose.runtime",
	"produceState": "androidx.compose.runtime", "collectAsState": "androidx.compose.runtime",
	"Animatable": "androidx.compose.animation.core", "animateDpAsState": "androidx.compose.animation.core",
	"animateFloatAsState": "androidx.compose.animation.core", "animateIntAsState": "androidx.compose.animation.core",
	"animateOffsetAsState":        "androidx.compose.animation.core",
	"animateIntOffsetAsState":     "androidx.compose.animation.core",
	"animateValueAsState":         "androidx.compose.animation.core",
	"collectAsStateWithLifecycle": "androidx.lifecycle.compose",
	"rememberUpdatedState":        "androidx.compose.runtime",
}

// axlDeclIsStateBacked reports whether a property or parameter is a State or Animatable (by declared type or
// initializer), or a local delegated `by` one.
func axlDeclIsStateBacked(file *scanner.File, decl uint32) bool {
	if t := acrDeclaredType(file, decl); t != 0 && !file.FlatHasChildOfType(decl, "property_delegate") {
		for pkg, names := range axlStateTypes {
			if acrTypeIs(file, t, pkg, names...) {
				return true
			}
		}
		return false
	}
	if file.FlatType(decl) != "property_declaration" {
		return false
	}
	if delegate, ok := file.FlatFindChild(decl, "property_delegate"); ok {
		return axlExprIsState(file, flatLastNamedChild(file, delegate), 0)
	}
	return axlExprIsState(file, acrInitializer(file, decl), 0)
}

// axlExprIsState proves an expression produces a State or Animatable: a factory call, `remember { factory }`, or a
// name declared as one.
func axlExprIsState(file *scanner.File, expr uint32, depth int) bool {
	expr = acrUnwrapParens(file, expr)
	if expr == 0 || depth > 4 {
		return false
	}
	switch file.FlatType(expr) {
	case "call_expression":
		name, _, _ := acrCallCallee(file, expr)
		if pkg, ok := axlStateFactories[name]; ok && acrNameResolvesTo(file, pkg, name) &&
			len(acrFunctionDecls(file, name)) == 0 {
			return true
		}
		if name == "remember" && acrNameResolvesTo(file, "androidx.compose.runtime", name) {
			return axlExprIsState(file, acrTrailingLambdaResult(file, expr), depth+1)
		}
		if name == "rememberSaveable" && acrNameResolvesTo(file, "androidx.compose.runtime.saveable", name) {
			return axlExprIsState(file, acrTrailingLambdaResult(file, expr), depth+1)
		}
	case "simple_identifier":
		if decl := acrValueDeclaration(file, expr, file.FlatNodeText(expr)); decl != 0 {
			return axlDeclIsStateBacked(file, decl)
		}
	}
	return false
}

// axlBoxWithConstraintsScopeMembers are the properties of BoxWithConstraintsScope.
var axlBoxWithConstraintsScopeMembers = map[string]bool{
	"maxWidth": true, "maxHeight": true, "minWidth": true, "minHeight": true, "constraints": true,
}

// check mirrors BoxWithConstraintsDetector: the content lambda of a BoxWithConstraints from
// androidx.compose.foundation.layout that never uses its BoxWithConstraintsScope, at the lambda.
func (r *UnusedBoxWithConstraintsScopeRule) check(ctx *api.Context) {
	file, lambda := ctx.File, ctx.Idx
	call, ok := axlContentLambda(file, lambda, "content", true)
	if !ok || !acrCallTargets(file, call, axlFoundationLayoutPackage, "BoxWithConstraints") {
		return
	}
	if axlUsesBoxWithConstraintsScope(file, lambda) {
		return
	}
	ctx.EmitAtNode(lambda, "BoxWithConstraints scope is not used")
}

// axlUsesBoxWithConstraintsScope reports whether the lambda uses its scope: `this`, a scope property not shadowed
// by a declaration, a call of a same-file BoxWithConstraintsScope extension, or an invocation of a value whose
// type has a BoxWithConstraintsScope receiver.
func axlUsesBoxWithConstraintsScope(file *scanner.File, lambda uint32) bool {
	used := false
	file.FlatWalkAllNodes(lambda, func(n uint32) {
		if used {
			return
		}
		switch file.FlatType(n) {
		case "this_expression":
			used = true
		case "simple_identifier", "interpolated_identifier":
			name := file.FlatNodeText(n)
			if p, ok := file.FlatParent(n); ok && (file.FlatType(p) == "navigation_suffix" ||
				(file.FlatType(p) == "value_argument" && file.FlatNextSib(n) != 0 && file.FlatType(file.FlatNextSib(n)) == "=")) {
				// `x.maxWidth` reads another value's property; `minWidth = 48.dp` is an argument label.
				return
			}
			if axlBoxWithConstraintsScopeMembers[name] {
				used = acrValueDeclaration(file, n, name) == 0
				return
			}
			if axlSameFileBoxScopeExtension(file, name) {
				used = true
				return
			}
			if !axlBoxScopeReceiverValueNames(file)[name] {
				return
			}
			if decl := acrValueDeclaration(file, n, name); decl != 0 {
				if t := acrDeclaredType(file, decl); t != 0 && axlIsBoxScopeReceiverType(file, t) {
					used = true
				}
			}
		}
	})
	return used
}

// axlSameFileBoxScopeExtension reports whether the file declares a function or property with that name whose
// receiver type is BoxWithConstraintsScope. The names are collected once per file.
func axlSameFileBoxScopeExtension(file *scanner.File, name string) bool {
	names := filefacts.FileFact(fileFactsCache(), file, slotAxlBoxScopeExtensions, func() map[string]bool {
		out := map[string]bool{}
		file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
			if t := acrFunctionReceiverType(file, fn); t != 0 && acrTypeNodeName(file, t) == "BoxWithConstraintsScope" {
				out[acrDeclarationName(file, fn)] = true
			}
		})
		file.FlatWalkNodes(0, "property_declaration", func(p uint32) {
			if t, ok := file.FlatFindChild(p, "user_type"); ok && acrTypeNodeName(file, t) == "BoxWithConstraintsScope" {
				out[acrPropertyName(file, p)] = true
			}
		})
		return out
	})
	return names[name]
}

const (
	slotAxlBoxScopeExtensions     = "axlBoxScopeExtensions"
	slotAxlBoxScopeReceiverValues = "axlBoxScopeReceiverValues"
)

// axlBoxScopeReceiverValueNames returns, once per file, the names of parameters and properties whose declared type
// is a function type with a BoxWithConstraintsScope receiver; only those can be invoked to use the scope.
func axlBoxScopeReceiverValueNames(file *scanner.File) map[string]bool {
	return filefacts.FileFact(fileFactsCache(), file, slotAxlBoxScopeReceiverValues, func() map[string]bool {
		out := map[string]bool{}
		if !fileFactsCache().Mentions(file, "BoxWithConstraintsScope") {
			return out
		}
		file.FlatWalkNodes(0, "parameter", func(p uint32) {
			if t := acrParameterType(file, p); t != 0 && axlIsBoxScopeReceiverType(file, t) {
				out[file.FlatChildTextOrEmpty(p, "simple_identifier")] = true
			}
		})
		file.FlatWalkNodes(0, "property_declaration", func(p uint32) {
			if t := acrDeclaredType(file, p); t != 0 && axlIsBoxScopeReceiverType(file, t) {
				out[acrPropertyName(file, p)] = true
			}
		})
		return out
	})
}

// axlIsBoxScopeReceiverType reports whether a type is a function type with a BoxWithConstraintsScope receiver.
func axlIsBoxScopeReceiverType(file *scanner.File, t uint32) bool {
	return strings.Contains(strings.Join(strings.Fields(file.FlatNodeText(t)), ""), "BoxWithConstraintsScope.(")
}

// check mirrors TextFieldBufferAppendDetector: append on a TextFieldBuffer receiver in a file whose package is
// androidx.compose.foundation or below, at the call.
func (r *TextFieldBufferInternalAppendRule) check(ctx *api.Context) {
	file, node := ctx.File, ctx.Idx
	if pkg := filePackageName(file); pkg != "androidx.compose.foundation" &&
		!strings.HasPrefix(pkg, "androidx.compose.foundation.") {
		return
	}
	if acrIsInnerCallOfTrailingLambda(file, node) {
		return
	}
	name, nameNode, hasReceiver := acrCallCallee(file, node)
	if name != "append" {
		return
	}
	if hasReceiver {
		if !axlExprIsTextFieldBuffer(file, file.FlatFirstChild(acrCalleeExpression(file, node)), 0) {
			return
		}
		ctx.EmitAtNode(node, axlTextFieldBufferMessage)
		return
	}
	if !axlImplicitTextFieldBuffer(file, node) {
		return
	}
	ctx.EmitAtNode(nameNode, axlTextFieldBufferMessage)
}

// axlNonReceiverLambdaFunctions are stdlib functions whose lambda has no receiver, so the implicit receiver inside it
// is the enclosing one.
var axlNonReceiverLambdaFunctions = map[string]bool{
	"let": true, "also": true, "takeIf": true, "takeUnless": true, "forEach": true, "forEachIndexed": true,
	"map": true, "mapNotNull": true, "filter": true, "onEach": true, "repeat": true, "any": true, "all": true,
}

const axlTextFieldBufferMessage = "Do not use `append` on TextFieldBuffer internally as it swallows the hardware " +
	"source tracking information. Use the internal `replace` overload instead."

// axlExprIsTextFieldBuffer proves an expression is a TextFieldBuffer by its same-file declaration.
func axlExprIsTextFieldBuffer(file *scanner.File, expr uint32, depth int) bool {
	expr = acrUnwrapParens(file, expr)
	if expr == 0 || depth > 4 {
		return false
	}
	switch file.FlatType(expr) {
	case "simple_identifier", "navigation_expression":
		decl := acrExprDeclaration(file, expr)
		if decl == 0 {
			return false
		}
		if t := acrDeclaredType(file, decl); t != 0 {
			return acrTypeIs(file, t, axlTextInputPackage, "TextFieldBuffer")
		}
		return axlExprIsTextFieldBuffer(file, acrInitializer(file, decl), depth+1)
	case "this_expression":
		return axlImplicitTextFieldBuffer(file, expr)
	}
	return false
}

// axlImplicitTextFieldBuffer reports whether the implicit receiver at node is a TextFieldBuffer: the subject of an
// enclosing with(buffer) { }, or the receiver of the enclosing TextFieldBuffer extension.
func axlImplicitTextFieldBuffer(file *scanner.File, node uint32) bool {
	for p, ok := file.FlatParent(node); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "lambda_literal":
			arg, ok := acrLambdaArgumentOf(file, p)
			if !ok {
				continue
			}
			name, _, hasReceiver := acrCallCallee(file, arg.Call)
			switch {
			case name == "with" && !hasReceiver:
				return axlExprIsTextFieldBuffer(file, acrCallValueArgument(file, arg.Call, 0, "receiver"), 0)
			case (name == "apply" || name == "run") && hasReceiver:
				return axlExprIsTextFieldBuffer(file, file.FlatFirstChild(acrCalleeExpression(file, arg.Call)), 0)
			case name == "edit" && hasReceiver:
				return true
			case axlNonReceiverLambdaFunctions[name]:
				// The lambda has no receiver of its own; the implicit receiver comes from further out.
				continue
			}
			// Any other lambda (buildString { }, buildAnnotatedString { }, ...) may bring its own receiver.
			return false
		case "function_declaration":
			t := acrFunctionReceiverType(file, p)
			return t != 0 && acrTypeIs(file, t, axlTextInputPackage, "TextFieldBuffer")
		case "class_body", "source_file":
			return false
		}
	}
	return false
}
