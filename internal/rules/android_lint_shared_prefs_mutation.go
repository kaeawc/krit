package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const mutatingSharedPrefsMessage = "Do not modify the set returned by `SharedPreferences.getStringSet()``"

// MutatingSharedPrefsRule mirrors Lint's MutatingSharedPrefs: a mutating call on the set that
// SharedPreferences.getStringSet returned, seen through the same method's local variables, casts and scope functions.
type MutatingSharedPrefsRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: the value flow is followed by name within one method.
func (r *MutatingSharedPrefsRule) Confidence() float64 { return api.ConfidenceMedium }

// prefsSetMutator reports whether the method name is one Lint treats as modifying a set.
func prefsSetMutator(name string) bool {
	return strings.HasPrefix(name, "add") || strings.HasPrefix(name, "remove") || name == "retainAll" || name == "clear"
}

func (r *MutatingSharedPrefsRule) check(ctx *api.Context) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || !prefsSetMutator(call.Name) || !fileFactsCache().Mentions(file, "getStringSet") {
		return
	}
	recv := call.ReceiverNode
	if recv == 0 {
		binding, ok := lintThisBinding(file, ctx.Idx)
		if !ok {
			return
		}
		recv = binding
	}
	if prefsSetTracked(ctx, recv, 0) {
		ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", mutatingSharedPrefsMessage))
	}
}

// prefsSetTracked reports whether the expression evaluates to the set from getStringSet.
func prefsSetTracked(ctx *api.Context, idx uint32, depth int) bool {
	file := ctx.File
	if idx == 0 || depth > 10 {
		return false
	}
	switch file.FlatType(idx) {
	case "parenthesized_expression", "postfix_expression", "as_expression":
		return prefsSetTracked(ctx, file.FlatNamedChild(idx, 0), depth+1)
	case "cast_expression":
		return prefsSetTracked(ctx, flatLastNamedChild(file, idx), depth+1)
	case "elvis_expression":
		return prefsAnyChildTracked(ctx, idx, 0, depth)
	case "ternary_expression":
		return prefsAnyChildTracked(ctx, idx, 1, depth)
	case "if_expression":
		return prefsIfBranchTracked(ctx, idx, depth)
	case "call_expression", "method_invocation":
		return prefsCallTracked(ctx, idx, depth)
	case "simple_identifier", "identifier":
		return prefsNameTracked(ctx, idx, depth)
	case "this_expression":
		binding, ok := lintThisBinding(file, idx)
		return ok && prefsSetTracked(ctx, binding, depth+1)
	}
	return false
}

// prefsAnyChildTracked reports whether any named child from index first on is tracked.
func prefsAnyChildTracked(ctx *api.Context, idx uint32, first, depth int) bool {
	file := ctx.File
	for i := first; i < file.FlatNamedChildCount(idx); i++ {
		if prefsSetTracked(ctx, file.FlatNamedChild(idx, i), depth+1) {
			return true
		}
	}
	return false
}

// prefsIfBranchTracked reports whether the value of either branch of a Kotlin if-expression is tracked.
func prefsIfBranchTracked(ctx *api.Context, idx uint32, depth int) bool {
	file := ctx.File
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "control_structure_body" && prefsSetTracked(ctx, flatLastCodeNamedChild(file, child), depth+1) {
			return true
		}
	}
	return false
}

// prefsCallTracked handles the getStringSet call itself and the calls that return their receiver or lambda result.
func prefsCallTracked(ctx *api.Context, idx uint32, depth int) bool {
	file := ctx.File
	call, ok := lintCallAt(file, idx)
	if !ok {
		return false
	}
	switch call.Name {
	case "getStringSet":
		return prefsIsStringSetCall(ctx, call)
	case "also", "apply", "takeIf", "takeUnless":
		return call.ReceiverNode != 0 && flatCallTrailingLambda(file, idx) != 0 && prefsSetTracked(ctx, call.ReceiverNode, depth+1)
	case "let", "run":
		lit := flatCallTrailingLambda(file, idx)
		if call.ReceiverNode == 0 || lit == 0 {
			return false
		}
		stmts, ok := file.FlatFindChild(lit, "statements")
		return ok && prefsSetTracked(ctx, flatLastCodeNamedChild(file, stmts), depth+1)
	}
	return false
}

// prefsIsStringSetCall confirms a getStringSet(key, default) call is SharedPreferences.getStringSet.
func prefsIsStringSetCall(ctx *api.Context, call lintCall) bool {
	file := ctx.File
	if call.ReceiverNode == 0 || lintValueArgumentCount(file, call) != 2 {
		return false
	}
	if verdict, resolved := lintResolvedOwnerExtends(ctx, call, sharedPreferencesClass); resolved {
		return verdict
	}
	return lintReceiverIsType(ctx, call, sharedPreferencesClass) || prefsReceiverCallReturnsPrefs(file, call)
}

// prefsReceiverCallReturnsPrefs recognizes a receiver that is a call returning SharedPreferences: the framework
// getters or a function of this file that declares the type.
func prefsReceiverCallReturnsPrefs(file *scanner.File, call lintCall) bool {
	inner, ok := lintCallAt(file, lintUnwrapReceiver(file, call.ReceiverNode))
	if !ok {
		return false
	}
	if strings.HasSuffix(inner.Name, "SharedPreferences") || inner.Name == "getPreferences" {
		return true
	}
	found := false
	for _, kind := range []string{"function_declaration", "method_declaration"} {
		file.FlatWalkNodes(0, kind, func(fn uint32) {
			if name := lintDeclaredMethodName(file, fn); name != 0 && file.FlatNodeText(name) == inner.Name &&
				lintChildTypeText(file, fn) == "SharedPreferences" {
				found = true
			}
		})
	}
	return found
}

// lintOutermostFunction returns the outermost function or method around idx, whose body Lint's flow analysis covers.
func lintOutermostFunction(file *scanner.File, idx uint32) uint32 {
	fn := uint32(0)
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "function_declaration", "method_declaration":
			fn = cur
		}
	}
	return fn
}

// prefsDef is one place that gives a local name a value: a declaration, an assignment or a parameter.
type prefsDef struct {
	pos   uint32
	value uint32
	// param is a lambda parameter, whose value is the receiver of its scope function.
	param uint32
}

// prefsNameTracked follows a local name back to its latest definition before the use.
func prefsNameTracked(ctx *api.Context, use uint32, depth int) bool {
	file := ctx.File
	name := file.FlatNodeText(use)
	fn := lintOutermostFunction(file, use)
	if fn == 0 {
		return false
	}
	defs, declared := prefsDefinitions(file, fn, use, name)
	if !declared {
		if name == "it" {
			if binding, ok := lintItBinding(file, use); ok {
				return prefsSetTracked(ctx, binding, depth+1)
			}
		}
		return false
	}
	var latest *prefsDef
	for i := range defs {
		if latest == nil || defs[i].pos > latest.pos {
			latest = &defs[i]
		}
	}
	if latest == nil {
		return false
	}
	if latest.param != 0 {
		binding, ok := lintParamBinding(file, latest.param)
		return ok && prefsSetTracked(ctx, binding, depth+1)
	}
	return prefsSetTracked(ctx, latest.value, depth+1)
}

// prefsDefinitions collects the definitions of name in fn that precede the use. declared is false when no local
// declaration, parameter or lambda parameter named so is visible, so assignments to a field are not followed.
func prefsDefinitions(file *scanner.File, fn, use uint32, name string) (defs []prefsDef, declared bool) {
	useStart := file.FlatStartByte(use)
	visible := func(scope uint32) bool {
		return file.FlatStartByte(scope) <= useStart && useStart <= file.FlatEndByte(scope)
	}
	addDecl := func(decl, scope, value, param uint32) {
		if file.FlatStartByte(decl) < useStart && visible(scope) {
			declared = true
			defs = append(defs, prefsDef{pos: file.FlatStartByte(decl), value: value, param: param})
		}
	}
	file.FlatWalkNodes(fn, "property_declaration", func(decl uint32) {
		if vd, ok := file.FlatFindChild(decl, "variable_declaration"); ok && lintDeclaredName(file, vd) == name {
			if scope, ok := file.FlatParent(decl); ok {
				addDecl(decl, scope, lintInitializerAfterEquals(file, decl), 0)
			}
		}
	})
	file.FlatWalkNodes(fn, "variable_declarator", func(decl uint32) {
		if id, ok := file.FlatFindChild(decl, "identifier"); ok && file.FlatNodeText(id) == name {
			if stmt, ok := file.FlatParent(decl); ok {
				if scope, ok := file.FlatParent(stmt); ok {
					addDecl(decl, scope, lintInitializerAfterEquals(file, decl), 0)
				}
			}
		}
	})
	prefsParameterDefs(file, fn, name, addDecl)
	file.FlatWalkNodes(fn, "assignment", func(as uint32) { prefsAssignmentDef(file, as, name, useStart, declared, &defs) })
	file.FlatWalkNodes(fn, "assignment_expression", func(as uint32) { prefsAssignmentDef(file, as, name, useStart, declared, &defs) })
	return defs, declared
}

// prefsParameterDefs registers function and lambda parameters named name; they hide earlier values.
func prefsParameterDefs(file *scanner.File, fn uint32, name string, add func(decl, scope, value, param uint32)) {
	for _, kind := range []string{"parameter", "formal_parameter", "catch_formal_parameter", "enhanced_for_statement"} {
		file.FlatWalkNodes(fn, kind, func(p uint32) {
			if id, ok := file.FlatFindChild(p, "simple_identifier"); ok && file.FlatNodeText(id) == name {
				prefsAddParam(file, p, add)
			} else if id, ok := file.FlatFindChild(p, "identifier"); ok && file.FlatNodeText(id) == name {
				prefsAddParam(file, p, add)
			}
		})
	}
	file.FlatWalkNodes(fn, "lambda_parameters", func(params uint32) {
		for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "variable_declaration" && lintDeclaredName(file, child) == name {
				if lit, ok := file.FlatParent(params); ok {
					add(child, lit, 0, lit)
				}
			}
		}
	})
}

func prefsAddParam(file *scanner.File, p uint32, add func(decl, scope, value, param uint32)) {
	if scope, ok := file.FlatParent(p); ok {
		if outer, ok := file.FlatParent(scope); ok {
			scope = outer
		}
		add(p, scope, 0, 0)
	}
}

// prefsAssignmentDef records `name = value` when it precedes the use and a local named so is declared.
func prefsAssignmentDef(file *scanner.File, as uint32, name string, useStart uint32, declared bool, defs *[]prefsDef) {
	if !declared || file.FlatStartByte(as) >= useStart {
		return
	}
	target := file.FlatNamedChild(as, 0)
	if file.FlatType(target) == "directly_assignable_expression" {
		target = file.FlatNamedChild(target, 0)
	}
	if target == 0 || file.FlatNodeText(target) != name || file.FlatNamedChildCount(as) < 2 || !prefsPlainAssignment(file, as) {
		return
	}
	*defs = append(*defs, prefsDef{pos: file.FlatStartByte(as), value: flatLastNamedChild(file, as)})
}

// prefsPlainAssignment reports whether the assignment operator is `=`.
func prefsPlainAssignment(file *scanner.File, as uint32) bool {
	for child := file.FlatFirstChild(as); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			return file.FlatType(child) == "="
		}
	}
	return false
}

// lintScopeLambdaOwner returns the call that a lambda literal is the trailing lambda of, or false.
func lintScopeLambdaOwner(file *scanner.File, lit uint32) (lintCall, bool) {
	al, ok := file.FlatParent(lit)
	if !ok || file.FlatType(al) != "annotated_lambda" {
		return lintCall{}, false
	}
	suffix, ok := file.FlatParent(al)
	if !ok {
		return lintCall{}, false
	}
	owner, ok := file.FlatParent(suffix)
	if !ok || file.FlatType(owner) != "call_expression" {
		return lintCall{}, false
	}
	// `with(x) { ... }` nests the call that holds the arguments inside the call that holds the lambda.
	if inner, nested := file.FlatFindChild(owner, "call_expression"); nested {
		owner = inner
	}
	return lintCallAt(file, owner)
}

// lintThisBinding finds the receiver bound as `this` for idx by the nearest apply, run or with lambda around it.
func lintThisBinding(file *scanner.File, idx uint32) (uint32, bool) {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "function_declaration", "class_declaration", "object_declaration":
			return 0, false
		case "lambda_literal":
			if owner, ok := lintScopeLambdaOwner(file, cur); ok {
				switch {
				case (owner.Name == "apply" || owner.Name == "run") && owner.ReceiverNode != 0:
					return owner.ReceiverNode, true
				case owner.Name == "with" && len(owner.Args) == 1 && owner.Args[0] != 0:
					return owner.Args[0], true
				}
			}
		}
	}
	return 0, false
}

// lintItBinding finds the receiver bound as the implicit `it` of the nearest let, also, takeIf or takeUnless lambda.
func lintItBinding(file *scanner.File, idx uint32) (uint32, bool) {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "function_declaration", "class_declaration", "object_declaration":
			return 0, false
		case "lambda_literal":
			if _, explicit := file.FlatFindChild(cur, "lambda_parameters"); explicit {
				return 0, false
			}
			return lintParamBinding(file, cur)
		}
	}
	return 0, false
}

// lintParamBinding returns the receiver passed as the parameter of a let, also, takeIf or takeUnless lambda.
func lintParamBinding(file *scanner.File, lit uint32) (uint32, bool) {
	owner, ok := lintScopeLambdaOwner(file, lit)
	if !ok || owner.ReceiverNode == 0 {
		return 0, false
	}
	switch owner.Name {
	case "let", "also", "takeIf", "takeUnless":
		return owner.ReceiverNode, true
	}
	return 0, false
}
