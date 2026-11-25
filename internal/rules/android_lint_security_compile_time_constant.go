package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// CompileTimeConstantRule mirrors Lint's CompileTimeConstant (CompileTimeConstantDetector) for declarations in the
// same file: a non-constant argument for a @CompileTimeConstant parameter, a non-constant assignment to a
// @CompileTimeConstant variable, and the declaration shapes Lint rejects (non-final fields, public properties,
// setters, receivers, overrides that add the annotation, function references, ++ and --).
type CompileTimeConstantRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: callees and referenced variables are resolved from
// declarations in the file, and anything resolved elsewhere is not reported.
func (r *CompileTimeConstantRule) Confidence() float64 { return api.ConfidenceMedium }

var compileTimeConstantNodeTypes = []string{
	"call_expression", "method_invocation", "object_creation_expression", "callable_reference",
	"assignment", "assignment_expression", "postfix_expression", "prefix_expression", "update_expression",
	"property_declaration", "field_declaration", "class_parameter", "function_declaration", "method_declaration",
	"setter",
}

const (
	ctcArgumentMessage   = "Non-compile-time constant expression passed to parameter with `@CompileTimeConstant` annotation"
	ctcAssignmentMessage = "Non-compile-time constant expression assigned to variable annotated with `@CompileTimeConstant`"
	ctcIncDecMessage     = "Cannot apply increment or decrement operators to variable annotated with `@CompileTimeConstant`"
	ctcPrivateMessage    = "Property must be declared `private` or use `@JvmField` to allow `@CompileTimeConstant`"
	ctcFieldParamMessage = "Annotating backing field of constructor property with `@field:CompileTimeConstant`\n" +
		"is unsafe and unnecessary. Annotate as `@CompileTimeConstant` only."
	ctcSetterMessage = "`@CompileTimeConstant` parameters are not supported in Kotlin property setters.\n" +
		"Use a `fun` with a parameter annotated `@CompileTimeConstant`."
	ctcReceiverMessage = "Annotating extension function receivers with `@receiver:CompileTimeConstant` is not\n" +
		"supported. Only regular function parameters can be annotated with\n`@CompileTimeConstant`."
	ctcOverrideMessage  = "Method with `@CompileTimeConstant` parameter cannot override method without it"
	ctcReferenceMessage = "Method with `@CompileTimeConstant` parameter cannot be used as a Kotlin function type"
)

func (r *CompileTimeConstantRule) check(ctx *api.Context) {
	file, n := ctx.File, ctx.Idx
	switch file.FlatType(n) {
	case "call_expression", "method_invocation", "object_creation_expression":
		r.checkCall(ctx)
	case "callable_reference":
		if fn := ctcReferencedFunction(file, n); fn != 0 && len(ctcAnnotatedParams(file, fn)) > 0 {
			ctx.Emit(lintFindingAtNode(ctx, n, "", ctcReferenceMessage))
		}
	case "assignment", "assignment_expression":
		r.checkAssignment(ctx)
	case "postfix_expression", "prefix_expression", "update_expression":
		text := strings.TrimSpace(file.FlatNodeText(n))
		if !strings.HasPrefix(text, "++") && !strings.HasPrefix(text, "--") && !strings.HasSuffix(text, "++") &&
			!strings.HasSuffix(text, "--") {
			return
		}
		operand := strings.Trim(text, "+- \t")
		if lintIdentOnly(operand) && ctcVariableAnnotated(file, n, operand) {
			ctx.Emit(lintFindingAtNode(ctx, n, "", ctcIncDecMessage))
		}
	case "field_declaration":
		if ctcAnnotated(file, n, false) && !file.FlatHasModifier(n, "final") {
			ctx.Emit(lintFindingAtNode(ctx, n, "", "`@CompileTimeConstant` found on non-final field. Fields "+
				"annotated with `@CompileTimeConstant` must be declared `final`."))
		}
	case "property_declaration":
		r.checkProperty(ctx)
	case "class_parameter":
		if !ctcHasAnnotation(file, n, "field") {
			return
		}
		ctx.Emit(lintFindingAtNode(ctx, n, "", ctcFieldParamMessage))
		if !ctcIsPrivate(file, n) && !ctcHasJvmField(file, n) {
			ctx.Emit(lintFindingAtNode(ctx, n, "", ctcPrivateMessage))
		}
	case "setter":
		if params, ok := file.FlatFindChild(n, "function_value_parameters"); ok && ctcAnyParamAnnotated(file, params) {
			ctx.Emit(lintFindingAtNode(ctx, n, "", ctcSetterMessage))
		} else if param, ok := file.FlatFindChild(n, "parameter_with_optional_type"); ok && ctcAnnotated(file, param, false) {
			ctx.Emit(lintFindingAtNode(ctx, n, "", ctcSetterMessage))
		} else if ctcSetterParamAnnotated(file, n) {
			ctx.Emit(lintFindingAtNode(ctx, n, "", ctcSetterMessage))
		}
	case "function_declaration", "method_declaration":
		r.checkFunction(ctx)
	}
}

// ctcAnnotationNodes lists the annotations of a declaration, with their use-site target ("" when there is none).
func ctcAnnotationNodes(file *scanner.File, decl uint32) []uint32 {
	var out []uint32
	collect := func(container uint32) {
		for child := file.FlatFirstChild(container); child != 0; child = file.FlatNextSib(child) {
			switch file.FlatType(child) {
			case "annotation", "marker_annotation":
				out = append(out, child)
			case "parameter_modifiers", "modifiers":
				for inner := file.FlatFirstChild(child); inner != 0; inner = file.FlatNextSib(inner) {
					if t := file.FlatType(inner); t == "annotation" || t == "marker_annotation" {
						out = append(out, inner)
					}
				}
			}
		}
	}
	if mods, ok := file.FlatFindChild(decl, "modifiers"); ok {
		collect(mods)
	}
	if mods, ok := file.FlatFindChild(decl, "parameter_modifiers"); ok {
		collect(mods)
	}
	// A Kotlin function parameter's annotations sit in a parameter_modifiers node just before it.
	if file.FlatType(decl) == "parameter" {
		if prev, ok := file.FlatPrevSibling(decl); ok && file.FlatType(prev) == "parameter_modifiers" {
			collect(prev)
		}
	}
	return out
}

// ctcAnnotationTarget returns an annotation's name and use-site target, e.g. ("CompileTimeConstant", "field").
func ctcAnnotationTarget(file *scanner.File, ann uint32) (name, target string) {
	text := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(file.FlatNodeText(ann)), "@"))
	if i := strings.Index(text, "("); i >= 0 {
		text = text[:i]
	}
	if t, rest, ok := strings.Cut(text, ":"); ok && lintIdentOnly(strings.TrimSpace(t)) {
		target, text = strings.TrimSpace(t), strings.TrimSpace(rest)
	}
	name = text[strings.LastIndex(text, ".")+1:]
	return strings.TrimSpace(name), target
}

// ctcHasAnnotation reports whether the declaration has @CompileTimeConstant with the given use-site target. Lint
// matches the annotation by simple name, in any package.
func ctcHasAnnotation(file *scanner.File, decl uint32, target string) bool {
	for _, ann := range ctcAnnotationNodes(file, decl) {
		if name, t := ctcAnnotationTarget(file, ann); name == "CompileTimeConstant" && t == target {
			return true
		}
	}
	return false
}

// ctcAnnotated reports whether the declaration carries @CompileTimeConstant, without a use-site target or (when
// allowField) with @field:.
func ctcAnnotated(file *scanner.File, decl uint32, allowField bool) bool {
	return ctcHasAnnotation(file, decl, "") || (allowField && ctcHasAnnotation(file, decl, "field"))
}

func ctcHasJvmField(file *scanner.File, decl uint32) bool {
	for _, ann := range ctcAnnotationNodes(file, decl) {
		if name, _ := ctcAnnotationTarget(file, ann); name == "JvmField" {
			return true
		}
	}
	return false
}

func ctcIsPrivate(file *scanner.File, decl uint32) bool {
	mods, ok := file.FlatFindChild(decl, "modifiers")
	return ok && strings.Contains(" "+strings.Join(strings.Fields(file.FlatNodeText(mods)), " ")+" ", " private ")
}

func (r *CompileTimeConstantRule) checkProperty(ctx *api.Context) {
	file, n := ctx.File, ctx.Idx
	if !ctcAnnotated(file, n, true) {
		return
	}
	if _, local := flatEnclosingCallable(file, n); local {
		return
	}
	kind, _ := file.FlatFindChild(n, "binding_pattern_kind")
	if strings.TrimSpace(file.FlatNodeText(kind)) == "var" {
		ctx.Emit(lintFindingAtNode(ctx, n, "", "`@CompileTimeConstant` found on non-final field. Fields annotated "+
			"with `@CompileTimeConstant` must be declared `val`."))
	}
	if !ctcIsPrivate(file, n) && !ctcHasJvmField(file, n) && !file.FlatHasModifier(n, "const") {
		ctx.Emit(lintFindingAtNode(ctx, n, "", ctcPrivateMessage))
	}
}

// ctcParam is one declared parameter of a function, method or constructor.
type ctcParam struct {
	node      uint32
	name      string
	annotated bool
	vararg    bool
	defaulted bool
}

// ctcParams lists the parameters of a callable declaration (or a class's primary constructor).
func ctcParams(file *scanner.File, decl uint32) []ctcParam {
	var params uint32
	for _, kind := range []string{"function_value_parameters", "formal_parameters", "class_parameters"} {
		if p, ok := file.FlatFindChild(decl, kind); ok {
			params = p
			break
		}
	}
	if params == 0 {
		if ctor, ok := file.FlatFindChild(decl, "primary_constructor"); ok {
			params, _ = file.FlatFindChild(ctor, "class_parameters")
		}
	}
	if params == 0 {
		return nil
	}
	var out []ctcParam
	var pendingMods uint32
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "parameter_modifiers":
			pendingMods = child
		case "parameter", "class_parameter", "formal_parameter", "spread_parameter":
			p := ctcParam{node: child, annotated: ctcAnnotated(file, child, false)}
			if pendingMods != 0 {
				for inner := file.FlatFirstChild(pendingMods); inner != 0; inner = file.FlatNextSib(inner) {
					if file.FlatType(inner) == "annotation" {
						if name, t := ctcAnnotationTarget(file, inner); name == "CompileTimeConstant" && t == "" {
							p.annotated = true
						}
					}
					if strings.TrimSpace(file.FlatNodeText(inner)) == "vararg" {
						p.vararg = true
					}
				}
				pendingMods = 0
			}
			p.name = lintDeclaredName(file, child)
			if p.name == "" {
				if id, ok := file.FlatFindChild(child, "identifier"); ok {
					p.name = file.FlatNodeText(id)
				}
			}
			p.vararg = p.vararg || file.FlatType(child) == "spread_parameter" || strings.Contains(file.FlatNodeText(child), "...")
			p.defaulted = strings.Contains(file.FlatNodeText(child), "=")
			out = append(out, p)
		}
	}
	return out
}

func ctcAnnotatedParams(file *scanner.File, decl uint32) []ctcParam {
	var out []ctcParam
	for _, p := range ctcParams(file, decl) {
		if p.annotated {
			out = append(out, p)
		}
	}
	return out
}

func ctcAnyParamAnnotated(file *scanner.File, params uint32) bool {
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		if ctcAnnotated(file, child, false) {
			return true
		}
		if file.FlatType(child) == "parameter_modifiers" || file.FlatType(child) == "annotation" {
			if strings.Contains(file.FlatNodeText(child), "CompileTimeConstant") {
				return true
			}
		}
	}
	return false
}

// ctcSetterParamAnnotated reads the setter parameter's annotations from its source text, for grammar versions
// that do not wrap the parameter in its own node.
func ctcSetterParamAnnotated(file *scanner.File, setter uint32) bool {
	text := file.FlatNodeText(setter)
	open := strings.Index(text, "(")
	closeIdx := strings.Index(text, ")")
	return open >= 0 && closeIdx > open && strings.Contains(text[open:closeIdx], "@CompileTimeConstant")
}

func (r *CompileTimeConstantRule) checkFunction(ctx *api.Context) {
	file, fn := ctx.File, ctx.Idx
	// `fun @receiver:CompileTimeConstant T.f()`.
	for child := file.FlatFirstChild(fn); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "function_value_parameters" {
			break
		}
		file.FlatWalkNodes(child, "annotation", func(ann uint32) {
			if name, t := ctcAnnotationTarget(file, ann); name == "CompileTimeConstant" && t == "receiver" {
				ctx.Emit(lintFindingAtNode(ctx, ann, "", ctcReceiverMessage))
			}
		})
	}
	params := ctcParams(file, fn)
	var annotated []int
	for i, p := range params {
		if p.annotated {
			annotated = append(annotated, i)
		}
	}
	if len(annotated) == 0 {
		return
	}
	// An override that adds the annotation to a parameter of a same-file supertype method.
	for _, super := range ctcOverriddenMethods(file, fn) {
		superParams := ctcParams(file, super)
		if len(superParams) != len(params) {
			continue
		}
		for _, i := range annotated {
			if !superParams[i].annotated {
				ctx.Emit(lintFindingAtNode(ctx, fn, "", ctcOverrideMessage))
				return
			}
		}
	}
}

// ctcOverriddenMethods returns the same-file methods with the same name and arity in the supertypes of the class
// declaring fn, when fn is an override.
func ctcOverriddenMethods(file *scanner.File, fn uint32) []uint32 {
	isOverride := file.FlatHasModifier(fn, "override")
	if !isOverride {
		for _, ann := range ctcAnnotationNodes(file, fn) {
			if name, _ := ctcAnnotationTarget(file, ann); name == "Override" {
				isOverride = true
			}
		}
	}
	if !isOverride {
		return nil
	}
	owner := lintDeclarationOwner(file, fn)
	if owner == 0 {
		return nil
	}
	name := javascriptInterfaceMemberName(file, fn)
	arity := len(ctcParams(file, fn))
	var out []uint32
	seen := map[uint32]bool{owner: true}
	var visit func(cls uint32, depth int)
	visit = func(cls uint32, depth int) {
		if depth > 8 {
			return
		}
		for _, super := range lintSupertypeNames(file, cls) {
			decl := lintSameFileClassOrObjectJS(file, super[strings.LastIndex(super, ".")+1:])
			if decl == 0 || seen[decl] {
				continue
			}
			seen[decl] = true
			javascriptInterfaceEachMember(file, decl, func(member uint32) {
				if javascriptInterfaceMemberName(file, member) == name && len(ctcParams(file, member)) == arity {
					out = append(out, member)
				}
			})
			visit(decl, depth+1)
		}
	}
	visit(owner, 0)
	return out
}

func (r *CompileTimeConstantRule) checkCall(ctx *api.Context) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok {
		return
	}
	callee := ctcResolveCallee(file, call)
	if callee == 0 {
		return
	}
	params := ctcParams(file, callee)
	if len(params) == 0 {
		return
	}
	for i, arg := range call.Args {
		if arg == 0 {
			continue
		}
		p, ok := ctcParamForArgument(params, call, i)
		if !ok || !p.annotated {
			continue
		}
		if ctcConstness(file, arg, ctx.Idx, 0) == ctcNotConstant {
			ctx.Emit(lintFindingAtNode(ctx, lintSkipParensJS(file, arg), "", ctcArgumentMessage))
		}
	}
}

// ctcParamForArgument maps argument index of the call to its declared parameter.
func ctcParamForArgument(params []ctcParam, call lintCall, index int) (ctcParam, bool) {
	if index < len(call.Labels) && call.Labels[index] != "" {
		for _, p := range params {
			if p.name == call.Labels[index] {
				return p, true
			}
		}
		return ctcParam{}, false
	}
	pos := unsafeIntentPositionalCTC(call, index)
	if pos < len(params) {
		return params[pos], true
	}
	if n := len(params); n > 0 && params[n-1].vararg {
		return params[n-1], true
	}
	return ctcParam{}, false
}

// unsafeIntentPositionalCTC is the positional index of argument index among the unlabeled arguments.
func unsafeIntentPositionalCTC(call lintCall, index int) int {
	pos := 0
	for i := 0; i < index && i < len(call.Labels); i++ {
		if call.Labels[i] == "" {
			pos++
		}
	}
	return pos
}

// ctcResolveCallee finds the same-file declaration a call reaches: a function or method by name and arity
// (unqualified, on `this`, or on a same-file class or object), or the constructor of a same-file class. It returns 0
// when the call does not resolve to exactly one declaration in the file.
func ctcResolveCallee(file *scanner.File, call lintCall) uint32 {
	recv := strings.TrimSpace(call.ReceiverText)
	var owner uint32
	switch {
	case file.FlatType(call.Idx) == "object_creation_expression":
		if strings.Contains(call.TypeText, ".") {
			return 0
		}
		cls := lintSameFileClass(file, call.TypeText)
		if cls == 0 {
			return 0
		}
		return ctcPickByArity(file, ctcConstructors(file, cls), call)
	case call.ReceiverNode == 0 || recv == "this":
		if file.Language != scanner.LangJava && call.Name != "" && call.Name[0] >= 'A' && call.Name[0] <= 'Z' {
			if cls := lintSameFileClass(file, call.Name); cls != 0 {
				return ctcPickByArity(file, ctcConstructors(file, cls), call)
			}
		}
	case lintIdentOnly(recv):
		owner = lintSameFileClassOrObjectJS(file, recv)
		if owner == 0 {
			return 0
		}
	default:
		return 0
	}
	var candidates []uint32
	for _, kind := range []string{"function_declaration", "method_declaration"} {
		file.FlatWalkNodes(owner, kind, func(fn uint32) {
			if javascriptInterfaceMemberName(file, fn) != call.Name {
				return
			}
			if _, local := flatEnclosingCallable(file, fn); local {
				return
			}
			if owner != 0 {
				declOwner := lintDeclarationOwner(file, fn)
				if declOwner != owner && (file.FlatType(declOwner) != "companion_object" || lintDeclarationOwner(file, declOwner) != owner) {
					return
				}
			}
			candidates = append(candidates, fn)
		})
	}
	return ctcPickByArity(file, candidates, call)
}

// ctcConstructors returns the primary and secondary constructors of a class.
func ctcConstructors(file *scanner.File, cls uint32) []uint32 {
	out := []uint32{cls}
	body := javascriptInterfaceBody(file, cls)
	if body != 0 {
		for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
			if t := file.FlatType(child); t == "secondary_constructor" || t == "constructor_declaration" {
				out = append(out, child)
			}
		}
	}
	return out
}

// ctcPickByArity returns the only candidate that can take the call's arguments.
func ctcPickByArity(file *scanner.File, candidates []uint32, call lintCall) uint32 {
	var match uint32
	count := 0
	for _, c := range candidates {
		params := ctcParams(file, c)
		if file.FlatType(c) == "class_declaration" && len(params) == 0 {
			continue
		}
		required, vararg := 0, false
		for _, p := range params {
			if p.vararg {
				vararg = true
			} else if !p.defaulted {
				required++
			}
		}
		if len(call.Args) < required || (!vararg && len(call.Args) > len(params)) {
			continue
		}
		match = c
		count++
	}
	if count != 1 {
		return 0
	}
	return match
}

func (r *CompileTimeConstantRule) checkAssignment(ctx *api.Context) {
	file, n := ctx.File, ctx.Idx
	lhs := file.FlatNamedChild(n, 0)
	name := strings.TrimSpace(file.FlatNodeText(lhs))
	if !lintIdentOnly(name) || !ctcVariableAnnotated(file, n, name) {
		return
	}
	op := ""
	for child := file.FlatFirstChild(n); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			op = strings.TrimSpace(file.FlatNodeText(child))
		}
	}
	rhs := file.FlatNamedChild(n, file.FlatNamedChildCount(n)-1)
	if op != "=" || ctcConstness(file, rhs, n, 0) == ctcNotConstant {
		ctx.Emit(lintFindingAtNode(ctx, n, "", ctcAssignmentMessage))
	}
}

// ctcVariableAnnotated reports whether name, seen from scope, resolves in the file to a variable annotated with
// @CompileTimeConstant: a parameter, a local, or a field or property.
func ctcVariableAnnotated(file *scanner.File, scope uint32, name string) bool {
	decl, _ := ctcResolveVariable(file, scope, name)
	return decl != 0 && ctcAnnotated(file, decl, true)
}

// ctcResolveVariable finds the declaration name refers to from scope, and what kind it is: "param", "local" or
// "field". It returns 0 when the name does not resolve in the file.
func ctcResolveVariable(file *scanner.File, scope uint32, name string) (uint32, string) {
	for cur, ok := file.FlatParent(scope); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "function_declaration", "method_declaration", "constructor_declaration", "secondary_constructor", "setter":
			for _, p := range ctcParams(file, cur) {
				if p.name == name {
					return p.node, "param"
				}
			}
		case "lambda_literal":
			if params, ok := file.FlatFindChild(cur, "lambda_parameters"); ok && lintParamsDeclare(file, params, name) {
				return 0, ""
			}
		}
	}
	if local := lintLocalDeclarationBeforeCTC(file, scope, name); local != 0 {
		return local, "local"
	}
	for cur, ok := file.FlatParent(scope); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "class_declaration", "object_declaration", "companion_object":
			if decl := ctcMemberField(file, cur, name); decl != 0 {
				return decl, "field"
			}
			for _, p := range ctcParams(file, cur) {
				if p.name == name && strings.Contains(" "+file.FlatNodeText(p.node), "val ") {
					return p.node, "field"
				}
			}
		}
	}
	if decl := ctcMemberField(file, 0, name); decl != 0 {
		return decl, "field"
	}
	return 0, ""
}

// ctcMemberField returns the property or field declared directly in owner (0: the top level) named name.
func ctcMemberField(file *scanner.File, owner uint32, name string) uint32 {
	var found uint32
	file.FlatWalkNodes(owner, "property_declaration", func(decl uint32) {
		if found != 0 {
			return
		}
		if vd, ok := file.FlatFindChild(decl, "variable_declaration"); ok && lintDeclaredName(file, vd) == name {
			if _, local := flatEnclosingCallable(file, decl); !local && lintDeclarationOwner(file, decl) == owner {
				found = decl
			}
		}
	})
	file.FlatWalkNodes(owner, "field_declaration", func(decl uint32) {
		if found != 0 || lintDeclarationOwner(file, decl) != owner {
			return
		}
		for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "variable_declarator" {
				if id, ok := file.FlatFindChild(child, "identifier"); ok && file.FlatNodeText(id) == name {
					found = decl
				}
			}
		}
	})
	return found
}

// lintLocalDeclarationBeforeCTC returns the local variable declaration named name in the function around use,
// declared before it.
func lintLocalDeclarationBeforeCTC(file *scanner.File, use uint32, name string) uint32 {
	fn, ok := flatEnclosingCallable(file, use)
	if !ok {
		return 0
	}
	var best uint32
	consider := func(decl uint32) {
		if file.FlatStartByte(decl) < file.FlatStartByte(use) && (best == 0 || file.FlatStartByte(decl) > file.FlatStartByte(best)) {
			best = decl
		}
	}
	file.FlatWalkNodes(fn, "property_declaration", func(decl uint32) {
		if vd, ok := file.FlatFindChild(decl, "variable_declaration"); ok && lintDeclaredName(file, vd) == name {
			consider(decl)
		}
	})
	file.FlatWalkNodes(fn, "local_variable_declaration", func(decl uint32) {
		for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "variable_declarator" {
				if id, ok := file.FlatFindChild(child, "identifier"); ok && file.FlatNodeText(id) == name {
					consider(decl)
				}
			}
		}
	})
	return best
}

// ctcConstnessKind is what the rule can tell about an expression.
type ctcConstnessKind int

const (
	ctcUnknown ctcConstnessKind = iota
	ctcConstant
	ctcNotConstant
)

// ctcCombine folds the constness of operands: any non-constant operand makes the whole non-constant.
func ctcCombine(kinds ...ctcConstnessKind) ctcConstnessKind {
	out := ctcConstant
	for _, k := range kinds {
		switch k {
		case ctcNotConstant:
			return ctcNotConstant
		case ctcUnknown:
			out = ctcUnknown
		}
	}
	return out
}

// ctcConstness ports CompileTimeConstantDetector.isConstant for what the file can prove. References that resolve
// outside the file are unknown, and unknown is never reported.
func ctcConstness(file *scanner.File, expr, scope uint32, depth int) ctcConstnessKind {
	expr = lintSkipParensJS(file, expr)
	if expr == 0 || depth > 8 {
		return ctcUnknown
	}
	switch file.FlatType(expr) {
	case "string_literal":
		var parts []ctcConstnessKind
		for child := file.FlatFirstChild(expr); child != 0; child = file.FlatNextSib(child) {
			switch file.FlatType(child) {
			case "interpolated_identifier":
				parts = append(parts, ctcIdentifierConstness(file, child, scope, depth))
			case "interpolated_expression":
				parts = append(parts, ctcConstness(file, file.FlatNamedChild(child, 0), scope, depth+1))
			}
		}
		return ctcCombine(parts...)
	case "integer_literal", "hex_literal", "bin_literal", "real_literal", "long_literal", "boolean_literal",
		"character_literal", "decimal_integer_literal", "hex_integer_literal", "octal_integer_literal",
		"binary_integer_literal", "decimal_floating_point_literal", "true", "false", "null_literal", "text_block":
		return ctcConstant
	case "if_expression", "ternary_expression":
		var branches []uint32
		for child := file.FlatFirstChild(expr); child != 0; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) {
				branches = append(branches, child)
			}
		}
		if len(branches) < 3 {
			return ctcNotConstant
		}
		return ctcCombine(ctcConstness(file, ctcBranchValue(file, branches[1]), scope, depth+1),
			ctcConstness(file, ctcBranchValue(file, branches[2]), scope, depth+1))
	case "additive_expression", "multiplicative_expression", "binary_expression":
		var parts []ctcConstnessKind
		for child := file.FlatFirstChild(expr); child != 0; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) {
				parts = append(parts, ctcConstness(file, child, scope, depth+1))
			}
		}
		return ctcCombine(parts...)
	case "simple_identifier", "identifier":
		return ctcIdentifierConstness(file, expr, scope, depth)
	case "navigation_expression", "field_access":
		return ctcQualifiedConstness(file, expr, depth)
	case "call_expression", "method_invocation":
		call, ok := lintCallAt(file, expr)
		if !ok {
			return ctcUnknown
		}
		if ctcConstantCollectionCall(file, call) {
			parts := make([]ctcConstnessKind, 0, len(call.Args))
			for _, arg := range call.Args {
				parts = append(parts, ctcConstness(file, arg, scope, depth+1))
			}
			return ctcCombine(parts...)
		}
		return ctcNotConstant
	case "object_creation_expression", "lambda_literal", "lambda_expression", "anonymous_function",
		"postfix_expression", "as_expression", "cast_expression", "indexing_expression", "array_access",
		"elvis_expression":
		if text := strings.TrimSpace(file.FlatNodeText(expr)); file.FlatType(expr) == "postfix_expression" && !strings.HasSuffix(text, "!!") {
			return ctcUnknown
		}
		return ctcNotConstant
	}
	if strings.TrimSpace(file.FlatNodeText(expr)) == "null" {
		return ctcConstant
	}
	return ctcUnknown
}

// ctcBranchValue unwraps a Kotlin if branch to its value expression.
func ctcBranchValue(file *scanner.File, branch uint32) uint32 {
	if file.FlatType(branch) == "control_structure_body" {
		if inner := file.FlatNamedChild(branch, 0); inner != 0 {
			return inner
		}
	}
	return branch
}

// ctcConstantCollectionCall reports whether the call is kotlin.collections listOf, setOf, emptyList or emptySet.
func ctcConstantCollectionCall(file *scanner.File, call lintCall) bool {
	switch call.Name {
	case "listOf", "setOf", "emptyList", "emptySet":
	default:
		return false
	}
	if call.ReceiverNode != 0 {
		return strings.TrimSpace(call.ReceiverText) == "kotlin.collections"
	}
	return file.Language != scanner.LangJava && lintSameFileFunctionNamed(file, call.Name) == 0
}

// lintSameFileFunctionNamed returns a function declared in the file with the name, or 0.
func lintSameFileFunctionNamed(file *scanner.File, name string) uint32 {
	var found uint32
	file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
		if found == 0 && javascriptInterfaceMemberName(file, fn) == name {
			found = fn
		}
	})
	return found
}

// ctcIdentifierConstness classifies a name: an annotated (and, in Java, effectively final) parameter, a final local
// with a constant initializer, or a constant field or property.
func ctcIdentifierConstness(file *scanner.File, id, scope uint32, depth int) ctcConstnessKind {
	name := strings.TrimSpace(file.FlatNodeText(id))
	decl, kind := ctcResolveVariable(file, scope, name)
	switch kind {
	case "param":
		if !ctcAnnotated(file, decl, false) {
			return ctcNotConstant
		}
		if file.Language == scanner.LangJava {
			if fn, ok := flatEnclosingCallable(file, decl); ok && lintNameReassigned(file, fn, name) {
				return ctcNotConstant
			}
		}
		return ctcConstant
	case "local":
		if !ctcDeclaredFinal(file, decl) {
			return ctcNotConstant
		}
		return ctcConstness(file, ctcInitializer(file, decl, name), decl, depth+1)
	case "field":
		return ctcFieldConstness(file, decl, name, depth)
	}
	return ctcUnknown
}

// ctcQualifiedConstness classifies `Owner.NAME` for an owner declared in the file; other owners are unknown.
func ctcQualifiedConstness(file *scanner.File, expr uint32, depth int) ctcConstnessKind {
	var named []uint32
	for child := file.FlatFirstChild(expr); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) {
			named = append(named, child)
		}
	}
	if len(named) != 2 {
		return ctcUnknown
	}
	owner := strings.TrimSpace(file.FlatNodeText(named[0]))
	name := strings.TrimPrefix(strings.TrimSpace(file.FlatNodeText(named[1])), ".")
	if !lintIdentOnly(owner) || !lintIdentOnly(name) {
		return ctcUnknown
	}
	cls := lintSameFileClassOrObjectJS(file, owner)
	if cls == 0 {
		return ctcUnknown
	}
	decl := ctcMemberField(file, cls, name)
	if decl == 0 {
		file.FlatWalkNodes(cls, "companion_object", func(c uint32) {
			if decl == 0 && lintDeclarationOwner(file, c) == cls {
				decl = ctcMemberField(file, c, name)
			}
		})
	}
	if decl == 0 {
		return ctcUnknown
	}
	return ctcFieldConstness(file, decl, name, depth)
}

// ctcFieldConstness classifies a field or property: a Kotlin const val, a Java final field with a constant
// initializer, or a final field annotated @CompileTimeConstant.
func ctcFieldConstness(file *scanner.File, decl uint32, name string, depth int) ctcConstnessKind {
	switch file.FlatType(decl) {
	case "property_declaration":
		if file.FlatHasModifier(decl, "const") {
			return ctcConstant
		}
		kind, _ := file.FlatFindChild(decl, "binding_pattern_kind")
		if strings.TrimSpace(file.FlatNodeText(kind)) == "val" && ctcAnnotated(file, decl, true) {
			return ctcConstant
		}
		return ctcNotConstant
	case "field_declaration":
		if !file.FlatHasModifier(decl, "final") {
			return ctcNotConstant
		}
		if ctcAnnotated(file, decl, false) {
			return ctcConstant
		}
		if ctcConstness(file, ctcInitializer(file, decl, name), decl, depth+1) == ctcConstant {
			return ctcConstant
		}
		return ctcNotConstant
	case "class_parameter":
		return ctcNotConstant
	}
	return ctcUnknown
}

// ctcDeclaredFinal reports whether a local is a Kotlin val or a Java final local.
func ctcDeclaredFinal(file *scanner.File, decl uint32) bool {
	if file.FlatType(decl) == "property_declaration" {
		kind, _ := file.FlatFindChild(decl, "binding_pattern_kind")
		return strings.TrimSpace(file.FlatNodeText(kind)) == "val"
	}
	return file.FlatHasModifier(decl, "final")
}

// ctcInitializer returns the initializer of a declaration (for Java, of the declarator named name).
func ctcInitializer(file *scanner.File, decl uint32, name string) uint32 {
	if file.FlatType(decl) == "property_declaration" {
		return lintInitializerAfterEquals(file, decl)
	}
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "variable_declarator" {
			if id, ok := file.FlatFindChild(child, "identifier"); ok && file.FlatNodeText(id) == name {
				return lintInitializerAfterEquals(file, child)
			}
		}
	}
	return 0
}

// ctcReferencedFunction resolves a Kotlin function reference (`Owner::name` or `::name`) to a same-file function.
func ctcReferencedFunction(file *scanner.File, ref uint32) uint32 {
	text := strings.TrimSpace(file.FlatNodeText(ref))
	owner, name, ok := strings.Cut(text, "::")
	if !ok || !lintIdentOnly(name) {
		return 0
	}
	call := lintCall{File: file, Idx: ref, Name: name}
	var candidates []uint32
	var ownerDecl uint32
	if owner != "" {
		ownerDecl = lintSameFileClassOrObjectJS(file, strings.TrimSpace(owner))
		if ownerDecl == 0 {
			return 0
		}
	}
	file.FlatWalkNodes(ownerDecl, "function_declaration", func(fn uint32) {
		if javascriptInterfaceMemberName(file, fn) == name {
			candidates = append(candidates, fn)
		}
	})
	_ = call
	if len(candidates) != 1 {
		return 0
	}
	return candidates[0]
}
