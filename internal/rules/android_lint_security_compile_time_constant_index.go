package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// CompileTimeConstant callees declared outside the calling file are resolved through the type index
// (typeinfer.ParamAnnotationIndex), which records parameter annotations for every indexed Kotlin source file
// (#1610). A name gate keeps the per-call cost to a map lookup unless some indexed function with an annotated
// parameter has the call's name.

const ctcAnnotation = "CompileTimeConstant"

// ctcOperatorSyntax maps a Kotlin operator function to text its operator-form call sites contain; "" means any
// call `f(x)` can be an invoke.
var ctcOperatorSyntax = map[string]string{
	"get": "[", "set": "[", "invoke": "",
	"plusAssign": "+=", "minusAssign": "-=", "timesAssign": "*=", "divAssign": "/=", "remAssign": "%=",
}

// ctcCompoundOperators maps a compound assignment operator to its operator function.
var ctcCompoundOperators = map[string]string{
	"+=": "plusAssign", "-=": "minusAssign", "*=": "timesAssign", "/=": "divAssign", "%=": "remAssign",
}

// ctcFileGate lets the rule run on files that mention the annotation, and on Kotlin files that mention a function
// declared elsewhere with a @CompileTimeConstant parameter (or the syntax of such an operator).
func ctcFileGate(file *scanner.File, resolver typeinfer.TypeResolver) bool {
	facts := fileFactsCache()
	if facts.Mentions(file, ctcAnnotation) {
		return true
	}
	if file.Language != scanner.LangKotlin {
		return false
	}
	idx := typeinfer.ParamAnnotationsOf(resolver)
	if idx == nil {
		return false
	}
	for _, name := range idx.FunctionNamesWithAnnotatedParam(ctcAnnotation) {
		if facts.Mentions(file, name) {
			return true
		}
		if syntax, ok := ctcOperatorSyntax[name]; ok && (syntax == "" || facts.Mentions(file, syntax)) {
			return true
		}
	}
	return false
}

// ctcIndex returns the parameter-annotation index for Kotlin files, or nil.
func ctcIndex(ctx *api.Context) typeinfer.ParamAnnotationIndex {
	if ctx.Resolver == nil || ctx.File == nil || ctx.File.Language != scanner.LangKotlin {
		return nil
	}
	return typeinfer.ParamAnnotationsOf(ctx.Resolver)
}

// ctcIndexedCallParams resolves a Kotlin call that no same-file declaration matches: a top-level function (explicit
// import, then same package, then wildcard import), a member of the enclosing class's supertypes, a member of the
// receiver's type, or `invoke` on a value of a class type. It returns nil when the call does not resolve to exactly
// one indexed callable with a parameter list.
func ctcIndexedCallParams(ctx *api.Context, idx typeinfer.ParamAnnotationIndex, call lintCall) []ctcParam {
	file := ctx.File
	if call.ReceiverNode == 0 {
		if idx.HasFunctionWithAnnotatedParam(ctcAnnotation, call.Name) {
			if p := ctcPickParamsByArity(ctcTopLevelCandidates(ctx, idx, call), call); p != nil {
				return p
			}
			if owner := lintDeclarationOwner(file, call.Idx); owner != 0 {
				if p := ctcPickParamsByArity(ctcSupertypeMembers(ctx, file, owner, call.Name), call); p != nil {
					return p
				}
			}
		}
		if idx.HasFunctionWithAnnotatedParam(ctcAnnotation, "invoke") {
			if callee := file.FlatFirstChild(call.Idx); callee != 0 && file.FlatType(callee) == "simple_identifier" {
				return ctcPickParamsByArity(ctcMembers(ctx.Resolver, ctx.Resolver.ResolveFlatNode(callee, file), "invoke"), call)
			}
		}
		return nil
	}
	if !idx.HasFunctionWithAnnotatedParam(ctcAnnotation, call.Name) {
		return nil
	}
	return ctcPickParamsByArity(ctcMembers(ctx.Resolver, ctx.Resolver.ResolveFlatNode(call.ReceiverNode, file), call.Name), call)
}

// ctcTopLevelCandidates returns the indexed top-level functions a receiverless call can name, from the highest
// Kotlin import priority that has any.
func ctcTopLevelCandidates(ctx *api.Context, idx typeinfer.ParamAnnotationIndex, call lintCall) [][]typeinfer.ParamInfo {
	file, offset := ctx.File, ctx.File.FlatStartByte(call.Idx)
	var tiers [][]string
	if facts := fileFactsCache().Imports(file); facts != nil {
		if fqn, ok := facts.Aliases[call.Name]; ok {
			tiers = append(tiers, []string{fqn})
		}
		var wildcards []string
		for w := range facts.Wildcards {
			wildcards = append(wildcards, strings.TrimSuffix(w, "*")+call.Name)
		}
		if pkg := filePackageName(file); pkg != "" {
			tiers = append(tiers, []string{pkg + "." + call.Name})
		}
		tiers = append(tiers, wildcards)
	} else if pkg := filePackageName(file); pkg != "" {
		tiers = append(tiers, []string{pkg + "." + call.Name})
	}
	for _, fqns := range tiers {
		var out [][]typeinfer.ParamInfo
		for _, fqn := range fqns {
			out = append(out, idx.TopLevelFunctionParams(fqn, file.Path, offset)...)
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

// ctcMembers returns the parameter lists of the functions named name on typ: those of the nearest class in its
// supertype hierarchy that declares one, so an override hides the method it overrides.
func ctcMembers(resolver typeinfer.TypeResolver, typ *typeinfer.ResolvedType, name string) [][]typeinfer.ParamInfo {
	if resolver == nil || typ == nil || typ.Kind == typeinfer.TypeUnknown {
		return nil
	}
	start := typ.FQN
	if start == "" {
		start = typ.Name
	}
	if start == "" {
		return nil
	}
	return ctcHierarchyMembers(resolver, []string{start}, name)
}

// ctcHierarchyMembers walks classes breadth first from start and returns the members named name of the first
// level that declares any.
func ctcHierarchyMembers(resolver typeinfer.TypeResolver, start []string, name string) [][]typeinfer.ParamInfo {
	seen := map[string]bool{}
	level := start
	for depth := 0; depth < 8 && len(level) > 0; depth++ {
		var next []string
		var out [][]typeinfer.ParamInfo
		for _, typeName := range level {
			if seen[typeName] {
				continue
			}
			seen[typeName] = true
			info := resolver.ClassHierarchy(typeName)
			if info == nil {
				continue
			}
			for _, m := range info.Members {
				if m.Kind == "function" && m.Name == name {
					out = append(out, m.Params)
				}
			}
			next = append(next, info.Supertypes...)
		}
		if len(out) > 0 {
			return out
		}
		level = next
	}
	return nil
}

// ctcSupertypeMembers returns the members named name of the supertypes of the same-file class owner that are
// declared in other files.
func ctcSupertypeMembers(ctx *api.Context, file *scanner.File, owner uint32, name string) [][]typeinfer.ParamInfo {
	var start []string
	for _, super := range lintSupertypeNames(file, owner) {
		simple := super[strings.LastIndex(super, ".")+1:]
		if lintSameFileClassOrObjectJS(file, simple) != 0 {
			continue
		}
		start = append(start, ctcResolveTypeName(ctx, file, super))
	}
	return ctcHierarchyMembers(ctx.Resolver, start, name)
}

// ctcResolveTypeName turns a written type name into the name ClassHierarchy knows: its import, or the file's
// package, or the name as written.
func ctcResolveTypeName(ctx *api.Context, file *scanner.File, name string) string {
	if strings.Contains(name, ".") {
		return name
	}
	if fqn := ctx.Resolver.ResolveImport(name, file); fqn != "" {
		return fqn
	}
	if pkg := filePackageName(file); pkg != "" && ctx.Resolver.ClassHierarchy(pkg+"."+name) != nil {
		return pkg + "." + name
	}
	return name
}

// ctcParamsFromIndex converts indexed parameters to the rule's parameter shape.
func ctcParamsFromIndex(params []typeinfer.ParamInfo) []ctcParam {
	out := make([]ctcParam, len(params))
	for i, p := range params {
		out[i] = ctcParam{name: p.Name, annotated: p.HasAnnotation(ctcAnnotation), vararg: p.Vararg, defaulted: p.HasDefault}
	}
	return out
}

// ctcPickParamsByArity returns the only candidate parameter list that can take the call's arguments.
func ctcPickParamsByArity(candidates [][]typeinfer.ParamInfo, call lintCall) []ctcParam {
	var match []ctcParam
	count := 0
	for _, c := range candidates {
		params := ctcParamsFromIndex(c)
		if !ctcArityFits(params, len(call.Args)) {
			continue
		}
		match = params
		count++
	}
	if count != 1 {
		return nil
	}
	return match
}

// ctcArityFits reports whether a parameter list can take n arguments.
func ctcArityFits(params []ctcParam, n int) bool {
	required, vararg := 0, false
	for _, p := range params {
		if p.vararg {
			vararg = true
		} else if !p.defaulted {
			required++
		}
	}
	return n >= required && (vararg || n <= len(params))
}

// ctcCheckArguments reports the non-constant arguments the call passes to @CompileTimeConstant parameters.
func ctcCheckArguments(ctx *api.Context, call lintCall, params []ctcParam) {
	for i, arg := range call.Args {
		if arg == 0 {
			continue
		}
		p, ok := ctcParamForArgument(params, call, i)
		if !ok || !p.annotated {
			continue
		}
		if ctcConstness(ctx.File, arg, ctx.Idx, 0) == ctcNotConstant {
			ctx.Emit(lintFindingAtNode(ctx, lintSkipParensJS(ctx.File, arg), "", ctcArgumentMessage))
		}
	}
}

// checkOperatorCall checks Kotlin operator calls: `a[i]` (get), `a[i] = v` (set) and `a += v` (plusAssign and the
// other compound assignments), resolved to the receiver type's operator function through the type index.
func (r *CompileTimeConstantRule) checkOperatorCall(ctx *api.Context) {
	idx := ctcIndex(ctx)
	if idx == nil {
		return
	}
	file, n := ctx.File, ctx.Idx
	var call lintCall
	switch file.FlatType(n) {
	case "indexing_expression":
		if parent, ok := file.FlatParent(n); ok && file.FlatType(parent) == "directly_assignable_expression" {
			return
		}
		recv, args := ctcIndexingParts(file, n)
		call = lintCall{File: file, Idx: n, Name: "get", ReceiverNode: recv, Args: args}
	case "assignment":
		target := file.FlatNamedChild(n, 0)
		if target == 0 || file.FlatType(target) != "directly_assignable_expression" {
			return
		}
		op := ""
		var value uint32
		for c := file.FlatNextSib(target); c != 0; c = file.FlatNextSib(c) {
			if !file.FlatIsNamed(c) {
				op = strings.TrimSpace(file.FlatNodeText(c))
				continue
			}
			value = c
			break
		}
		if value == 0 {
			return
		}
		if op == "=" {
			recv, args := ctcIndexingParts(file, target)
			if recv == 0 {
				return
			}
			call = lintCall{File: file, Idx: n, Name: "set", ReceiverNode: recv, Args: append(args, value)}
		} else if name, ok := ctcCompoundOperators[op]; ok {
			recv := file.FlatNamedChild(target, 0)
			if recv == 0 || file.FlatNamedChildCount(target) != 1 {
				return
			}
			call = lintCall{File: file, Idx: n, Name: name, ReceiverNode: recv, Args: []uint32{value}}
		} else {
			return
		}
	default:
		return
	}
	if call.ReceiverNode == 0 || !idx.HasFunctionWithAnnotatedParam(ctcAnnotation, call.Name) {
		return
	}
	call.Labels = make([]string, len(call.Args))
	if params := ctcPickParamsByArity(ctcMembers(ctx.Resolver, ctx.Resolver.ResolveFlatNode(call.ReceiverNode, file), call.Name), call); params != nil {
		ctcCheckArguments(ctx, call, params)
	}
}

// ctcIndexingParts splits `recv[a, b]` (an indexing_expression, or the directly_assignable_expression of an
// indexed assignment) into the receiver and the index expressions; recv is 0 for other shapes.
func ctcIndexingParts(file *scanner.File, n uint32) (recv uint32, args []uint32) {
	suffix, ok := file.FlatFindChild(n, "indexing_suffix")
	if !ok {
		return 0, nil
	}
	recv = file.FlatNamedChild(n, 0)
	if recv == suffix {
		return 0, nil
	}
	for c := file.FlatFirstChild(suffix); c != 0; c = file.FlatNextSib(c) {
		if file.FlatIsNamed(c) {
			args = append(args, c)
		}
	}
	return recv, args
}

// ctcIndexedOverriddenParams returns the parameter lists of the methods named name with arity params that fn
// overrides in supertypes declared in other files.
func ctcIndexedOverriddenParams(ctx *api.Context, fn uint32, arity int) [][]ctcParam {
	if ctcIndex(ctx) == nil {
		return nil
	}
	owner := lintDeclarationOwner(ctx.File, fn)
	if owner == 0 {
		return nil
	}
	var out [][]ctcParam
	for _, params := range ctcSupertypeMembers(ctx, ctx.File, owner, javascriptInterfaceMemberName(ctx.File, fn)) {
		if len(params) == arity {
			out = append(out, ctcParamsFromIndex(params))
		}
	}
	return out
}
