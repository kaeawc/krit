package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// The composition context follows androidx.compose.lint's ComposableBodyVisitor (isInvokedWithinComposable):
// walk up from an expression, past local properties and anonymous objects, to the first function, accessor or
// lambda that is not inlined into its caller; the expression runs in composition when that element is a
// @Composable function or a composable lambda. Lint resolves the call a lambda is passed to; here a lambda is
// composable only when the source proves it (see acrLambdaIsComposable), so unproven lambdas are boundaries that
// read as "not composition".

// acrCompositionScope is the element the composition walk stopped at.
type acrCompositionScope struct {
	// Node is the function, accessor, anonymous function or lambda the walk stopped at, or 0 for a class or file
	// level initializer.
	Node uint32
	// Composable reports whether Node runs in composition.
	Composable bool
}

// acrCompositionScopeOf returns the scope that decides whether node runs in composition.
func acrCompositionScopeOf(file *scanner.File, node uint32) acrCompositionScope {
	for p, ok := file.FlatParent(node); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "function_declaration":
			return acrCompositionScope{Node: p, Composable: acrFunctionIsComposable(file, p)}
		case "getter", "setter":
			return acrCompositionScope{Node: p, Composable: acrHasComposableAnnotation(file, p) ||
				acrPropertyHasComposableAccessorTarget(file, p)}
		case "lambda_literal", "anonymous_function":
			if acrLambdaIsInlinedIntoCaller(file, p) {
				continue
			}
			return acrCompositionScope{Node: p, Composable: acrLambdaIsComposable(file, p)}
		case "property_declaration":
			if acrIsLocalDeclaration(file, p) || acrInsideObjectLiteral(file, p) {
				continue
			}
			return acrCompositionScope{}
		case "function_value_parameters", "class_parameter", "class_declaration", "object_declaration",
			"companion_object", "anonymous_initializer", "secondary_constructor", "source_file":
			return acrCompositionScope{}
		}
	}
	return acrCompositionScope{}
}

// acrInComposition reports whether node is invoked within composition.
func acrInComposition(file *scanner.File, node uint32) bool {
	return acrCompositionScopeOf(file, node).Composable
}

// acrInsideObjectLiteral reports whether a declaration is a member of an anonymous object (`object : T { ... }`).
func acrInsideObjectLiteral(file *scanner.File, decl uint32) bool {
	body, ok := file.FlatParent(decl)
	if !ok || file.FlatType(body) != "class_body" {
		return false
	}
	owner, ok := file.FlatParent(body)
	return ok && file.FlatType(owner) == "object_literal"
}

// acrPropertyHasComposableAccessorTarget reports `@get:Composable` / `@set:Composable` on the property that owns
// an accessor.
func acrPropertyHasComposableAccessorTarget(file *scanner.File, accessor uint32) bool {
	var prop uint32
	if p, ok := file.FlatParent(accessor); ok && file.FlatType(p) == "property_declaration" {
		prop = p
	} else if prev, ok := file.FlatPrevSibling(accessor); ok && file.FlatType(prev) == "property_declaration" {
		// Top-level accessors parse as siblings of their property.
		prop = prev
	} else {
		return false
	}
	mods, ok := file.FlatFindChild(prop, "modifiers")
	if !ok {
		return false
	}
	want := "get"
	if file.FlatType(accessor) == "setter" {
		want = "set"
	}
	for m := file.FlatFirstChild(mods); m != 0; m = file.FlatNextSib(m) {
		if file.FlatType(m) != "annotation" {
			continue
		}
		target, ok := file.FlatFindChild(m, "use_site_target")
		if !ok || strings.TrimSuffix(strings.TrimSpace(file.FlatNodeText(target)), ":") != want {
			continue
		}
		if ut, ok := file.FlatFindChild(m, "user_type"); ok &&
			acrTypeIsRuntime(file, ut, acrComposableShort) {
			return true
		}
	}
	return false
}

// acrLambdaArgument describes how a lambda or anonymous function is passed to a call.
type acrLambdaArgument struct {
	Call     uint32 // the call_expression
	Label    string // the argument name for `name = { }`, else ""
	Index    int    // positional index among unlabeled value arguments, or -1
	Trailing bool   // passed as the trailing lambda
}

// acrLambdaArgumentOf returns how fn (a lambda_literal or anonymous_function) is passed to a call, if it is.
func acrLambdaArgumentOf(file *scanner.File, fn uint32) (acrLambdaArgument, bool) {
	node := fn
	// `label@{ }` and annotated lambdas wrap the literal in a prefix_expression.
	for {
		p, ok := file.FlatParent(node)
		if !ok || file.FlatType(p) != "prefix_expression" {
			break
		}
		node = p
	}
	p, ok := file.FlatParent(node)
	if !ok {
		return acrLambdaArgument{}, false
	}
	switch file.FlatType(p) {
	case "annotated_lambda":
		suffix, ok := file.FlatParent(p)
		if !ok || file.FlatType(suffix) != "call_suffix" {
			return acrLambdaArgument{}, false
		}
		call, ok := file.FlatParent(suffix)
		if !ok || file.FlatType(call) != "call_expression" {
			return acrLambdaArgument{}, false
		}
		return acrLambdaArgument{Call: call, Index: -1, Trailing: true}, true
	case "value_argument":
		if flatValueArgumentExpression(file, p) != node {
			return acrLambdaArgument{}, false
		}
		args, ok := file.FlatParent(p)
		if !ok || file.FlatType(args) != "value_arguments" {
			return acrLambdaArgument{}, false
		}
		suffix, ok := file.FlatParent(args)
		if !ok || file.FlatType(suffix) != "call_suffix" {
			return acrLambdaArgument{}, false
		}
		call, ok := file.FlatParent(suffix)
		if !ok || file.FlatType(call) != "call_expression" {
			return acrLambdaArgument{}, false
		}
		for acrIsInnerCallOfTrailingLambda(file, call) {
			call, _ = file.FlatParent(call)
		}
		arg := acrLambdaArgument{Call: call, Label: flatValueArgumentLabel(file, p), Index: -1}
		if arg.Label == "" {
			index := 0
			for child := file.FlatFirstChild(args); child != 0 && child != p; child = file.FlatNextSib(child) {
				if file.FlatType(child) == "value_argument" {
					if flatValueArgumentLabel(file, child) != "" {
						// A positional argument after a named one cannot be mapped without resolution.
						return acrLambdaArgument{}, false
					}
					index++
				}
			}
			arg.Index = index
		}
		return arg, true
	}
	return acrLambdaArgument{}, false
}

// acrCallCallee returns the callee name of a call_expression and whether it has an explicit receiver. For
// `foo(args) { }`, which parses as a call wrapping `foo(args)`, it reports the inner callee.
func acrCallCallee(file *scanner.File, call uint32) (name string, nameNode uint32, hasReceiver bool) {
	callee := acrCalleeExpression(file, call)
	switch file.FlatType(callee) {
	case "simple_identifier":
		return file.FlatNodeText(callee), callee, false
	case "navigation_expression":
		last := flatLastIdentifierNode(file, callee)
		if last == 0 {
			return "", 0, false
		}
		return file.FlatNodeText(last), last, true
	}
	return "", 0, false
}

// acrCalleeExpression returns the callee expression of a call, looking through the inner call of
// `foo(args) { }`.
func acrCalleeExpression(file *scanner.File, call uint32) uint32 {
	callee := file.FlatFirstChild(call)
	for callee != 0 && file.FlatType(callee) == "call_expression" && acrIsTrailingLambdaOnlySuffix(file, file.FlatNextSib(callee)) {
		callee = file.FlatFirstChild(callee)
	}
	return callee
}

// acrIsTrailingLambdaOnlySuffix reports whether a call_suffix holds only a trailing lambda (no value arguments).
func acrIsTrailingLambdaOnlySuffix(file *scanner.File, suffix uint32) bool {
	if suffix == 0 || file.FlatType(suffix) != "call_suffix" {
		return false
	}
	return file.FlatHasChildOfType(suffix, "annotated_lambda") && !file.FlatHasChildOfType(suffix, "value_arguments")
}

// acrIsInnerCallOfTrailingLambda reports whether call is the `foo(args)` part of `foo(args) { }`, which the outer
// call_expression already represents.
func acrIsInnerCallOfTrailingLambda(file *scanner.File, call uint32) bool {
	parent, ok := file.FlatParent(call)
	return ok && file.FlatType(parent) == "call_expression" && file.FlatFirstChild(parent) == call &&
		acrIsTrailingLambdaOnlySuffix(file, file.FlatNextSib(call))
}

// acrStdlibInlineScopeFunctions are kotlin stdlib inline functions whose lambda parameter is neither noinline nor
// crossinline, so Lint's couldBecomeComposableLambdaViaInlining sees through them.
var acrStdlibInlineScopeFunctions = map[string]bool{
	"let": true, "run": true, "apply": true, "also": true, "with": true, "takeIf": true, "takeUnless": true,
	"repeat": true,
}

// acrLambdaIsInlinedIntoCaller mirrors couldBecomeComposableLambdaViaInlining: the lambda is an argument of a
// call to a non-@Composable inline function whose matching parameter is neither noinline nor crossinline. It is
// proven for a same-file inline function and for the kotlin stdlib scope functions when nothing shadows them.
func acrLambdaIsInlinedIntoCaller(file *scanner.File, fn uint32) bool {
	arg, ok := acrLambdaArgumentOf(file, fn)
	if !ok {
		return false
	}
	name, _, _ := acrCallCallee(file, arg.Call)
	if name == "" {
		return false
	}
	if decl := acrSameFileFunction(file, name); decl != 0 {
		if !acrFunctionHasModifier(file, decl, "inline") || acrFunctionIsComposable(file, decl) {
			return false
		}
		param := acrParameterForArgument(file, decl, arg)
		return param != 0 && !acrParameterHasModifier(file, param, "noinline") &&
			!acrParameterHasModifier(file, param, "crossinline")
	}
	return acrStdlibInlineScopeFunctions[name] && !acrFileDeclaresAnyFunction(file, name) && arg.Trailing
}

// acrLambdaIsComposable mirrors ULambdaExpression.isComposable: a lambda passed to a call is composable when the
// matching parameter's type is @Composable; a lambda that initializes a property is composable when it or the
// property's type is annotated @Composable.
func acrLambdaIsComposable(file *scanner.File, fn uint32) bool {
	if arg, ok := acrLambdaArgumentOf(file, fn); ok {
		return acrCallArgumentIsComposable(file, arg)
	}
	node, annotated := fn, false
	for {
		p, ok := file.FlatParent(node)
		if !ok || file.FlatType(p) != "prefix_expression" {
			break
		}
		if a, ok := file.FlatFindChild(p, "annotation"); ok && acrAnnotationIsComposable(file, a) {
			annotated = true
		}
		node = p
	}
	prop, ok := file.FlatParent(node)
	if !ok || file.FlatType(prop) != "property_declaration" {
		return false
	}
	if annotated {
		return true
	}
	if v, ok := file.FlatFindChild(prop, "variable_declaration"); ok {
		return acrHasComposableAnnotation(file, v)
	}
	return false
}

// acrCallArgumentIsComposable reports whether the parameter a lambda argument binds to is @Composable: from a
// same-file declaration of the callee, or from the known composable-content parameters of Compose library APIs.
func acrCallArgumentIsComposable(file *scanner.File, arg acrLambdaArgument) bool {
	name, _, hasReceiver := acrCallCallee(file, arg.Call)
	if name == "" {
		return false
	}
	if decl := acrSameFileFunction(file, name); decl != 0 {
		param := acrParameterForArgument(file, decl, arg)
		return param != 0 && acrParameterTypeIsComposable(file, param)
	}
	if acrFileDeclaresAnyFunction(file, name) {
		return false
	}
	paramName := arg.Label
	if paramName == "" {
		if !arg.Trailing {
			return false
		}
		paramName = acrTrailingLambdaParameter
	}
	if !hasReceiver {
		for _, api := range acrComposableContentAPIs[name] {
			if api.params[paramName] && acrNameResolvesTo(file, api.pkg, name) {
				return true
			}
		}
	}
	// LazyListScope / LazyGridScope members inside a lazy layout's content lambda.
	if lazy := acrLazyScopeContentParams[name]; lazy[paramName] && !hasReceiver && acrInsideLazyLayoutContent(file, arg.Call) {
		return true
	}
	return false
}

// acrTrailingLambdaParameter is the parameter key used for a trailing lambda in acrComposableContentAPIs: every
// listed API takes its composable content as the last parameter.
const acrTrailingLambdaParameter = "<trailing>"

type acrComposableContentAPI struct {
	pkg    string
	params map[string]bool
}

func acrContentParams(names ...string) map[string]bool {
	out := map[string]bool{acrTrailingLambdaParameter: true}
	for _, n := range names {
		out[n] = true
	}
	return out
}

// acrComposableContentAPIs lists Compose library functions whose composable-lambda parameters are known (from the
// 1.12.1 runtime/foundation and material3 1.5.0 signatures). Each must resolve to the listed package.
var acrComposableContentAPIs = map[string][]acrComposableContentAPI{
	"CompositionLocalProvider": {{acrRuntimePackage, acrContentParams("content")}},
	"key":                      {{acrRuntimePackage, acrContentParams("block")}},
	"ReusableContent":          {{acrRuntimePackage, acrContentParams("content")}},
	"ReusableContentHost":      {{acrRuntimePackage, acrContentParams("content")}},
	"Column":                   {{"androidx.compose.foundation.layout", acrContentParams("content")}},
	"Row":                      {{"androidx.compose.foundation.layout", acrContentParams("content")}},
	"Box":                      {{"androidx.compose.foundation.layout", acrContentParams("content")}},
	"BoxWithConstraints":       {{"androidx.compose.foundation.layout", acrContentParams("content")}},
	"Surface":                  {{"androidx.compose.material3", acrContentParams("content")}, {"androidx.compose.material", acrContentParams("content")}},
	"Card":                     {{"androidx.compose.material3", acrContentParams("content")}, {"androidx.compose.material", acrContentParams("content")}},
	"MaterialTheme":            {{"androidx.compose.material3", acrContentParams("content")}, {"androidx.compose.material", acrContentParams("content")}},
	"Button":                   {{"androidx.compose.material3", acrContentParams("content")}, {"androidx.compose.material", acrContentParams("content")}},
	"TextButton":               {{"androidx.compose.material3", acrContentParams("content")}, {"androidx.compose.material", acrContentParams("content")}},
	"OutlinedButton":           {{"androidx.compose.material3", acrContentParams("content")}, {"androidx.compose.material", acrContentParams("content")}},
	"IconButton":               {{"androidx.compose.material3", acrContentParams("content")}, {"androidx.compose.material", acrContentParams("content")}},
	"Scaffold": {
		{"androidx.compose.material3", acrContentParams("content", "topBar", "bottomBar", "snackbarHost", "floatingActionButton")},
		{"androidx.compose.material", acrContentParams("content", "topBar", "bottomBar", "snackbarHost", "floatingActionButton")},
	},
}

// acrLazyScopeContentParams are the LazyListScope/LazyGridScope members whose content lambda is composable.
var acrLazyScopeContentParams = map[string]map[string]bool{
	"item":          acrContentParams("content"),
	"items":         acrContentParams("itemContent"),
	"itemsIndexed":  acrContentParams("itemContent"),
	"stickyHeader":  acrContentParams("content"),
	"stickyHeaders": acrContentParams("content"),
}

// acrLazyLayouts are the lazy layouts whose trailing `content` lambda has a LazyListScope/LazyGridScope receiver.
var acrLazyLayouts = map[string]string{
	"LazyColumn": "androidx.compose.foundation.lazy", "LazyRow": "androidx.compose.foundation.lazy",
	"LazyVerticalGrid": "androidx.compose.foundation.lazy.grid", "LazyHorizontalGrid": "androidx.compose.foundation.lazy.grid",
}

// acrInsideLazyLayoutContent reports whether call sits directly in the trailing content lambda of a lazy layout,
// where the lazy scope members are the implicit receiver.
func acrInsideLazyLayoutContent(file *scanner.File, call uint32) bool {
	for p, ok := file.FlatParent(call); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "lambda_literal":
			arg, ok := acrLambdaArgumentOf(file, p)
			if !ok || !arg.Trailing {
				return false
			}
			name, _, hasReceiver := acrCallCallee(file, arg.Call)
			pkg, known := acrLazyLayouts[name]
			return known && !hasReceiver && acrNameResolvesTo(file, pkg, name)
		case "function_declaration", "anonymous_function", "class_body", "source_file":
			return false
		}
	}
	return false
}

// acrSameFileFunction returns the declaration a call targets when the file declares exactly one function with that
// name (top-level or member), or 0.
func acrSameFileFunction(file *scanner.File, name string) uint32 {
	if decls := acrFunctionDecls(file, name); len(decls) == 1 {
		return decls[0]
	}
	return 0
}

// acrFileDeclaresAnyFunction reports whether any function in the file (top-level, member or local) has the name.
func acrFileDeclaresAnyFunction(file *scanner.File, name string) bool {
	return len(acrFunctionDecls(file, name)) > 0
}

// acrFunctionParameters returns a function declaration's value parameters in order.
func acrFunctionParameters(file *scanner.File, fn uint32) []uint32 {
	params, ok := file.FlatFindChild(fn, "function_value_parameters")
	if !ok {
		return nil
	}
	return flatDirectChildrenOfType(file, params, "parameter")
}

// acrParameterForArgument maps a lambda argument to the parameter it binds to.
func acrParameterForArgument(file *scanner.File, fn uint32, arg acrLambdaArgument) uint32 {
	params := acrFunctionParameters(file, fn)
	if len(params) == 0 {
		return 0
	}
	switch {
	case arg.Trailing:
		return params[len(params)-1]
	case arg.Label != "":
		for _, p := range params {
			if file.FlatChildTextOrEmpty(p, "simple_identifier") == arg.Label {
				return p
			}
		}
	case arg.Index >= 0 && arg.Index < len(params):
		for _, p := range params[:arg.Index+1] {
			if acrParameterHasModifier(file, p, "vararg") {
				return 0
			}
		}
		return params[arg.Index]
	}
	return 0
}

// acrParameterHasModifier reports whether the parameter_modifiers before a parameter contain the keyword.
func acrParameterHasModifier(file *scanner.File, param uint32, keyword string) bool {
	prev, ok := file.FlatPrevSibling(param)
	if !ok || file.FlatType(prev) != "parameter_modifiers" {
		return false
	}
	found := false
	file.FlatWalkAllNodes(prev, func(n uint32) {
		if !found && file.FlatChildCount(n) == 0 && file.FlatNodeTextEquals(n, keyword) {
			found = true
		}
	})
	return found
}

// acrParameterTypeIsComposable reports whether a parameter's function type carries @Composable.
func acrParameterTypeIsComposable(file *scanner.File, param uint32) bool {
	return acrComposableFunctionType(file, param) != 0
}
