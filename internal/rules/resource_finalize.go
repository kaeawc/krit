package rules

import "github.com/kaeawc/krit/internal/scanner"

// maxFinalizeAliasDepth bounds how many `val b = a` hops a tracked local is followed through.
const maxFinalizeAliasDepth = 3

// scopeLambdaFinalizes reports whether call is an apply/run/also/let call whose trailing lambda
// finalizes its receiver: it calls one of targets on the implicit `this` (apply/run) or on the lambda
// parameter (also/let), or hands the parameter to another owner. Lint tracks the receiver through these
// lambdas. Calls inside nested lambdas do not count because they may never run.
func scopeLambdaFinalizes(file *scanner.File, call uint32, targets map[string]bool, accept func(*scanner.File, uint32) bool) bool {
	scope := flatCallExpressionName(file, call)
	implicitThis := scope == "apply" || scope == "run"
	if !implicitThis && scope != "also" && scope != "let" {
		return false
	}
	lambda := flatCallTrailingLambda(file, call)
	if lambda == 0 {
		return false
	}
	param := lambdaFirstParamName(file, lambda)
	if !implicitThis && variableEscapes(file, lambda, param) {
		return true
	}
	found := false
	file.FlatWalkNodes(lambda, "call_expression", func(inner uint32) {
		if found || !targets[flatCallExpressionName(file, inner)] || (accept != nil && !accept(file, inner)) {
			return
		}
		if owner, ok := flatEnclosingAncestor(file, inner, "lambda_literal"); !ok || owner != lambda {
			return
		}
		if implicitThis {
			found = callIsOnImplicitThis(file, inner)
		} else {
			found = flatReceiverNameFromCall(file, inner) == param
		}
	})
	return found
}

// lambdaFirstParamName returns the declared first parameter of a lambda, or `it`.
func lambdaFirstParamName(file *scanner.File, lambda uint32) string {
	params, ok := file.FlatFindChild(lambda, "lambda_parameters")
	if !ok {
		return "it"
	}
	for i := 0; i < file.FlatNamedChildCount(params); i++ {
		param := file.FlatNamedChild(params, i)
		if t := file.FlatType(param); t == "variable_declaration" || t == "simple_identifier" {
			return file.FlatNodeText(param)
		}
	}
	return "it"
}

// callIsOnImplicitThis reports whether call has no receiver or an explicit `this` receiver.
func callIsOnImplicitThis(file *scanner.File, call uint32) bool {
	navExpr, _ := flatCallExpressionParts(file, call)
	if navExpr == 0 {
		return true
	}
	return file.FlatType(file.FlatNamedChild(navExpr, 0)) == "this_expression"
}

// ancestorScopeLambdaFinalizes reports whether idx is the receiver of an enclosing scope-function call
// whose lambda finalizes it, such as `prefs.edit().also { it.apply() }`.
func ancestorScopeLambdaFinalizes(file *scanner.File, idx uint32, targets map[string]bool, accept func(*scanner.File, uint32) bool) bool {
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "call_expression":
			lambda := flatCallTrailingLambda(file, parent)
			if lambda != 0 && file.FlatStartByte(lambda) > file.FlatEndByte(idx) && scopeLambdaFinalizes(file, parent, targets, accept) {
				return true
			}
		case "function_declaration", "source_file":
			return false
		}
	}
	return false
}

// referenceFinalizes reports whether the local reference ref is the receiver of a call to one of targets,
// looking through `!!`, or of a scope function whose lambda finalizes it.
func referenceFinalizes(file *scanner.File, ref uint32, targets map[string]bool, accept func(*scanner.File, uint32) bool) bool {
	recv := ref
	if parent, ok := file.FlatParent(recv); ok && file.FlatType(parent) == "postfix_expression" {
		recv = parent
	}
	nav, ok := file.FlatParent(recv)
	if !ok || file.FlatType(nav) != "navigation_expression" || file.FlatNamedChild(nav, 0) != recv {
		return false
	}
	call, ok := file.FlatParent(nav)
	if !ok || file.FlatType(call) != "call_expression" {
		return false
	}
	if navExpr, _ := flatCallExpressionParts(file, call); navExpr != nav {
		return false
	}
	if targets[flatCallExpressionName(file, call)] && (accept == nil || accept(file, call)) {
		return true
	}
	return scopeLambdaFinalizes(file, call, targets, accept)
}

// localVarFinalized reports whether the local name, initialized by the expression at call inside fn, is
// later finalized, escapes to another owner, or is aliased by a local that is finalized.
func localVarFinalized(file *scanner.File, fn, call uint32, name string, targets map[string]bool, accept func(*scanner.File, uint32) bool, depth int) bool {
	if name == "" {
		return false
	}
	if variableEscapes(file, fn, name) {
		return true
	}
	callEnd := file.FlatEndByte(call)
	done := false
	file.FlatWalkNodes(fn, "simple_identifier", func(id uint32) {
		if done || file.FlatStartByte(id) < callEnd || !file.FlatNodeTextEquals(id, name) {
			return
		}
		if referenceFinalizes(file, id, targets, accept) {
			done = true
			return
		}
		if depth < maxFinalizeAliasDepth {
			if alias := aliasDeclaredFrom(file, id); alias != "" {
				done = localVarFinalized(file, fn, id, alias, targets, accept, depth+1)
			}
		}
	})
	return done
}

// aliasDeclaredFrom returns the local name declared as `val alias = ref`, or "" when ref is not a bare initializer.
func aliasDeclaredFrom(file *scanner.File, ref uint32) string {
	parent, ok := file.FlatParent(ref)
	if !ok || file.FlatType(parent) != "property_declaration" || propertyInitializerExpression(file, parent) != ref {
		return ""
	}
	return propertyDeclarationName(file, parent)
}
