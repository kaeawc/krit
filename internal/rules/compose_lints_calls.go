package rules

import (
	"bytes"
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Messages of compose-lint-checks 1.6.1's call-site detectors, as Lint reports them (its text format drops the
// explanation's code backticks).
const (
	composeM2ApiMessage = "Compose Material 2 (M2) is succeeded by Material 3 (M3). Please use M3 APIs. See " +
		"https://slackhq.github.io/compose-lints/rules/#use-material-3 for more information."
	composeItemKeyHashCodeMessage = "Item keys in Lazy*/Pager/etc. layouts must be unique, but hashCode() is " +
		"never guaranteed to be unique. Lazy* layouts throw at runtime when they encounter duplicate keys, so a " +
		"hashCode-based key can crash as soon as data with a colliding hash comes along. Use a genuinely unique, " +
		"stable identifier instead (e.g. a server-provided id). See " +
		"https://slackhq.github.io/compose-lints/rules/#dont-use-hashcode-as-a-key for more information."
	composeModifierComposedMessage = "Modifier.composed { ... } is no longer recommended due to performance issues. " +
		"You should use the Modifier.Node API instead, as it was designed from the ground up to be far more " +
		"performant than composed modifiers. See https://slackhq.github.io/compose-lints/rules/#migrate-to-modifiernode " +
		"for more information."
)

const (
	composeM2Package = "androidx.compose.material"
	composeUIPackage = "androidx.compose.ui"
)

// composeM2ThemeMembers are MaterialTheme's properties, all of an M2 type, so a member read on them is M2 too.
var composeM2ThemeMembers = map[string]bool{"colors": true, "typography": true, "shapes": true}

// ComposeM2ApiRule ports compose-lints' ComposeM2Api: a call or qualified reference that resolves into
// androidx.compose.material (Material 2). Like Lint, a qualified chain is reported once, by its outermost
// expression, when its last selector is an M2 member.
//
// It runs over the cross-file index: a name a star import binds can be taken by a same-package declaration in
// another file.
type ComposeM2ApiRule struct {
	BaseRule
	// AllowedM2Apis lists M2 API names that are not reported (compose-lints `allowed-m2-apis`).
	AllowedM2Apis []string
}

// ComposeItemKeyHashCodeRule ports compose-lints' ComposeItemKeyHashCode: a `hashCode()` call inside the `key`
// argument of a call in a @Composable function body.
type ComposeItemKeyHashCodeRule struct {
	FlatDispatchBase
	BaseRule
}

// ComposeModifierComposedRule ports compose-lints' ComposeModifierComposed: a call to androidx.compose.ui's
// Modifier.composed { }.
type ComposeModifierComposedRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ComposeM2ApiRule) check(ctx *api.Context) {
	if ctx.CodeIndex == nil {
		return
	}
	for _, file := range ctx.CodeIndex.Files {
		if file == nil || file.FlatTree == nil || file.Language != scanner.LangKotlin ||
			!bytes.Contains(file.Content, []byte(composeM2Package+".")) {
			continue
		}
		pkg := filePackageName(file)
		// Kotlin binds same-package declarations, in any file, before star imports.
		samePackage := func(name string) bool {
			for _, sym := range ctx.CodeIndex.SymbolsNamed(name) {
				if sym.Package == pkg && sym.Language == scanner.LangKotlin {
					return true
				}
			}
			return false
		}
		file.FlatWalkAllNodes(0, func(node uint32) {
			if t := file.FlatType(node); t == "call_expression" || t == "navigation_expression" {
				r.checkNode(ctx, file, node, samePackage)
			}
		})
	}
}

func (r *ComposeM2ApiRule) checkNode(ctx *api.Context, file *scanner.File, node uint32, samePackage func(string) bool) {
	var name string
	var ok bool
	switch file.FlatType(node) {
	case "call_expression":
		callee := file.FlatFirstChild(node)
		switch file.FlatType(callee) {
		case "simple_identifier":
			name = file.FlatNodeText(callee)
			ok = composeM2NameResolves(file, node, name, samePackage)
		case "navigation_expression":
			name, ok = r.qualified(file, node, callee, samePackage)
		}
	case "navigation_expression":
		if parent, has := file.FlatParent(node); has && file.FlatType(parent) == "call_expression" &&
			file.FlatFirstChild(parent) == node {
			return // reported as the call
		}
		name, ok = r.qualified(file, node, node, samePackage)
	}
	if !ok {
		return
	}
	for _, allowed := range r.AllowedM2Apis {
		if name == allowed {
			return
		}
	}
	ctx.Emit(r.Finding(file, file.FlatRow(node)+1, file.FlatCol(node)+1, composeM2ApiMessage))
}

// composeM2NameResolves reports whether a simple name used at node binds to an androidx.compose.material
// top-level declaration: an explicit import, or the M2 wildcard import for a name M2 declares, with no enclosing
// or same-package declaration (in any file) taking it first.
func composeM2NameResolves(file *scanner.File, node uint32, name string, samePackage func(string) bool) bool {
	facts := fileFactsCache().Imports(file)
	if facts == nil {
		return false
	}
	if target, ok := facts.Aliases[name]; ok {
		return strings.HasPrefix(target, composeM2Package+".") &&
			!strings.Contains(strings.TrimPrefix(target, composeM2Package+"."), ".") &&
			!acrFunctionNameShadowed(file, node, name)
	}
	if _, star := facts.Wildcards[composeM2Package+".*"]; !star || !composeM2TopLevelNames[name] &&
		!composeM2TopLevelProperties[name] {
		return false
	}
	return !acrFileDeclaresTopLevel(file, name) && !samePackage(name) && !acrFunctionNameShadowed(file, node, name)
}

// qualified decides whether the qualified expression `unit` (a navigation expression, or a call on one) is the
// outermost of its chain and ends in an M2 member, returning the last selector's name. nav is the navigation
// expression whose suffix is that selector.
func (r *ComposeM2ApiRule) qualified(file *scanner.File, unit, nav uint32, samePackage func(string) bool) (string, bool) {
	if parent, ok := file.FlatParent(unit); ok && file.FlatType(parent) == "navigation_expression" &&
		file.FlatFirstChild(parent) == unit {
		return "", false // a receiver: Lint reports the longer chain instead
	}
	root, selectors := composeLintsChain(file, nav)
	if root == 0 || len(selectors) == 0 {
		return "", false
	}
	last := selectors[len(selectors)-1]
	if file.FlatType(root) != "simple_identifier" {
		return "", false
	}
	rootName := file.FlatNodeText(root)
	// An extension imported from M2, such as Modifier.swipeable(). Extensions are lowercase; an uppercase import
	// such as Text is a composable or class, which a qualified chain cannot select.
	if facts := fileFactsCache().Imports(file); facts != nil && rootName != "androidx" && !acrFirstRuneIsUpper(last) &&
		facts.Aliases[last] == composeM2Package+"."+last {
		return last, true
	}
	// A fully qualified reference: androidx.compose.material.X or androidx.compose.material.X.member.
	if rootName == "androidx" {
		path := strings.Join(selectors, ".")
		rest, ok := strings.CutPrefix(path, "compose.material.")
		if !ok {
			return "", false
		}
		parts := strings.Split(rest, ".")
		if len(parts) == 1 || len(parts) == 2 && acrFirstRuneIsUpper(parts[0]) && !composeM2TopLevelProperties[parts[0]] {
			return last, true
		}
		return "", false
	}
	if !acrFirstRuneIsUpper(rootName) || composeM2TopLevelProperties[rootName] || !composeM2NameResolves(file, root, rootName, samePackage) {
		return "", false
	}
	switch {
	case len(selectors) == 1:
		return last, true
	case len(selectors) == 2 && composeM2RootIsTheme(file, rootName) && composeM2ThemeMembers[selectors[0]]:
		return last, true
	}
	return "", false
}

// composeM2RootIsTheme reports whether rootName names M2's MaterialTheme (directly or through an import alias).
func composeM2RootIsTheme(file *scanner.File, rootName string) bool {
	if facts := fileFactsCache().Imports(file); facts != nil {
		if target, ok := facts.Aliases[rootName]; ok {
			return target == composeM2Package+".MaterialTheme"
		}
	}
	return rootName == "MaterialTheme"
}

// composeLintsChain flattens a navigation expression into its root expression and selector names, looking
// through calls in receiver position (`A.b().c` gives A and [b, c]).
func composeLintsChain(file *scanner.File, nav uint32) (uint32, []string) {
	var selectors []string
	node := nav
	for {
		switch file.FlatType(node) {
		case "navigation_expression":
			suffix, ok := file.FlatFindChild(node, "navigation_suffix")
			if !ok {
				return 0, nil
			}
			name := file.FlatChildTextOrEmpty(suffix, "simple_identifier")
			if name == "" {
				return 0, nil
			}
			selectors = append(selectors, name)
			node = file.FlatFirstChild(node)
		case "call_expression":
			node = file.FlatFirstChild(node)
		default:
			for i, j := 0, len(selectors)-1; i < j; i, j = i+1, j-1 {
				selectors[i], selectors[j] = selectors[j], selectors[i]
			}
			return node, selectors
		}
	}
}

func (r *ComposeItemKeyHashCodeRule) check(ctx *api.Context) {
	file, arg := ctx.File, ctx.Idx
	name := file.FlatFirstChild(arg)
	if name == 0 || file.FlatType(name) != "simple_identifier" || !file.FlatNodeTextEquals(name, "key") {
		return
	}
	eq := file.FlatNextSib(name)
	if eq == 0 || file.FlatType(eq) != "=" {
		return
	}
	value := file.FlatNextSib(eq)
	if value == 0 || !composeLintsArgumentOfCall(file, arg) || !composeLintsInComposableBody(file, arg) {
		return
	}
	file.FlatWalkNodes(value, "call_expression", func(call uint32) {
		if composeLintsIsZeroArgCall(file, call, "hashCode") {
			ctx.EmitAtNode(call, composeItemKeyHashCodeMessage)
		}
	})
}

// composeLintsArgumentOfCall reports whether a value_argument belongs to a call expression (not an annotation or
// a supertype constructor call).
func composeLintsArgumentOfCall(file *scanner.File, arg uint32) bool {
	args, ok := file.FlatParent(arg)
	if !ok || file.FlatType(args) != "value_arguments" {
		return false
	}
	suffix, ok := file.FlatParent(args)
	if !ok || file.FlatType(suffix) != "call_suffix" {
		return false
	}
	call, ok := file.FlatParent(suffix)
	return ok && file.FlatType(call) == "call_expression"
}

// composeLintsInComposableBody reports whether node sits in the body of a @Composable member or top-level
// function (or getter). Lambdas and local functions are looked through, as compose-lints walks the whole body
// of the enclosing composable method; classes and objects end the search.
func composeLintsInComposableBody(file *scanner.File, node uint32) bool {
	for p, ok := file.FlatParent(node); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "function_declaration":
			if acrIsLocalDeclaration(file, p) {
				continue
			}
			return acrFunctionIsComposable(file, p)
		case "getter", "setter":
			return acrHasComposableAnnotation(file, p)
		case "class_body", "object_literal", "source_file", "anonymous_initializer":
			return false
		}
	}
	return false
}

// composeLintsIsZeroArgCall reports whether call invokes `name` (unqualified or as the last selector) with no
// arguments and no trailing lambda.
func composeLintsIsZeroArgCall(file *scanner.File, call uint32, name string) bool {
	callee := file.FlatFirstChild(call)
	switch file.FlatType(callee) {
	case "simple_identifier":
		if !file.FlatNodeTextEquals(callee, name) {
			return false
		}
	case "navigation_expression":
		suffix, ok := file.FlatFindChild(callee, "navigation_suffix")
		if !ok || file.FlatChildTextOrEmpty(suffix, "simple_identifier") != name {
			return false
		}
	default:
		return false
	}
	suffix, ok := file.FlatFindChild(call, "call_suffix")
	if !ok || file.FlatHasChildOfType(suffix, "annotated_lambda") {
		return false
	}
	args, ok := file.FlatFindChild(suffix, "value_arguments")
	return ok && !file.FlatHasChildOfType(args, "value_argument")
}

func (r *ComposeModifierComposedRule) check(ctx *api.Context) {
	file, call := ctx.File, ctx.Idx
	callee := file.FlatFirstChild(call)
	var receiver uint32
	switch file.FlatType(callee) {
	case "simple_identifier":
		if !file.FlatNodeTextEquals(callee, "composed") {
			return
		}
	case "navigation_expression":
		suffix, ok := file.FlatFindChild(callee, "navigation_suffix")
		if !ok || file.FlatChildTextOrEmpty(suffix, "simple_identifier") != "composed" {
			return
		}
		receiver = file.FlatFirstChild(callee)
	default:
		return
	}
	if !acrCalleeResolvesTo(file, call, composeUIPackage, "composed") {
		return
	}
	if receiver == 0 {
		if !composeLintsInModifierExtension(file, call) {
			return
		}
	} else if !composeLintsIsModifierExpression(file, receiver) {
		return
	}
	ctx.EmitAtNode(call, composeModifierComposedMessage)
}

// composeLintsIsModifierExpression reports whether an expression is provably a Modifier: `Modifier`, `this` in a
// Modifier extension, a parameter or property declared as Modifier, or a call chain rooted in one of those or in
// a call on a Modifier extension's implicit receiver.
func composeLintsIsModifierExpression(file *scanner.File, expr uint32) bool {
	root, _ := composeLintsChain(file, expr)
	if root == 0 {
		return false
	}
	switch file.FlatType(root) {
	case "this_expression":
		return composeLintsInModifierExtension(file, root)
	case "simple_identifier":
		name := file.FlatNodeText(root)
		if name == "Modifier" {
			return acrNameResolvesTo(file, composeUIPackage, "Modifier")
		}
		if parent, ok := file.FlatParent(root); ok && file.FlatType(parent) == "call_expression" &&
			file.FlatFirstChild(parent) == root {
			// padding(1.dp).composed { } in a Modifier extension: a modifier call on the implicit receiver.
			return composeLintsInModifierExtension(file, root)
		}
		return composeLintsNameDeclaredAsModifier(file, root, name)
	}
	return false
}

// composeLintsInModifierExtension reports whether node is inside a function or property whose extension receiver
// is androidx.compose.ui.Modifier (the nearest non-local declaration).
func composeLintsInModifierExtension(file *scanner.File, node uint32) bool {
	for p, ok := file.FlatParent(node); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "function_declaration":
			if acrIsLocalDeclaration(file, p) {
				continue
			}
			return composeLintsReceiverIsModifier(file, p)
		case "property_declaration":
			if !acrIsLocalDeclaration(file, p) {
				return composeLintsReceiverIsModifier(file, p)
			}
		case "class_body", "object_literal", "source_file":
			return false
		}
	}
	return false
}

// composeLintsReceiverIsModifier reports whether a function or property declaration's receiver type is Modifier.
func composeLintsReceiverIsModifier(file *scanner.File, decl uint32) bool {
	t := composeLintsReceiverType(file, decl)
	return t != 0 && composeLintsTypeIsModifier(file, t)
}

// composeLintsReceiverType returns the type node of a function or property declaration's extension receiver, or 0.
func composeLintsReceiverType(file *scanner.File, decl uint32) uint32 {
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "user_type", "nullable_type", "function_type", "parenthesized_type":
			if next := file.FlatNextSib(child); next != 0 && file.FlatType(next) == "." {
				return child
			}
			return 0
		case "simple_identifier", "variable_declaration", "function_value_parameters", "function_body":
			return 0
		}
	}
	return 0
}

// composeLintsTypeIsModifier reports whether a (nullable) user_type names androidx.compose.ui.Modifier.
func composeLintsTypeIsModifier(file *scanner.File, typeNode uint32) bool {
	if file.FlatType(typeNode) == "nullable_type" {
		typeNode, _ = file.FlatFindChild(typeNode, "user_type")
	}
	if typeNode == 0 || file.FlatType(typeNode) != "user_type" {
		return false
	}
	text := composeLintsTypeText(file, typeNode)
	if text == composeUIPackage+".Modifier" {
		return true
	}
	if text == "Modifier" {
		return acrNameResolvesTo(file, composeUIPackage, "Modifier")
	}
	facts := fileFactsCache().Imports(file)
	return facts != nil && facts.Aliases[text] == composeUIPackage+".Modifier"
}

// composeLintsNameDeclaredAsModifier reports whether the nearest enclosing function declares a parameter `name`
// typed Modifier, or an enclosing block declares a local property `name` typed Modifier, before node.
func composeLintsNameDeclaredAsModifier(file *scanner.File, node uint32, name string) bool {
	for p, ok := file.FlatParent(node); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "statements":
			for child := file.FlatFirstChild(p); child != 0 && file.FlatStartByte(child) < file.FlatStartByte(node); child = file.FlatNextSib(child) {
				if file.FlatType(child) != "property_declaration" {
					continue
				}
				v, _ := file.FlatFindChild(child, "variable_declaration")
				if v != 0 && file.FlatChildTextOrEmpty(v, "simple_identifier") == name {
					t := composeLintsDeclaredType(file, v)
					return t != 0 && composeLintsTypeIsModifier(file, t)
				}
			}
		case "function_declaration":
			params, _ := file.FlatFindChild(p, "function_value_parameters")
			for param := file.FlatFirstChild(params); params != 0 && param != 0; param = file.FlatNextSib(param) {
				if file.FlatType(param) == "parameter" && file.FlatChildTextOrEmpty(param, "simple_identifier") == name {
					t := composeLintsDeclaredType(file, param)
					return t != 0 && composeLintsTypeIsModifier(file, t)
				}
			}
		case "class_body":
			return composeLintsClassMemberIsModifier(file, p, name)
		case "source_file":
			return false
		}
	}
	return false
}

// composeLintsClassMemberIsModifier reports whether the class owning body declares a property or a primary
// constructor `val`/`var` named name with a Modifier type.
func composeLintsClassMemberIsModifier(file *scanner.File, body uint32, name string) bool {
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "property_declaration" {
			continue
		}
		if v, ok := file.FlatFindChild(child, "variable_declaration"); ok && file.FlatChildTextOrEmpty(v, "simple_identifier") == name {
			t := composeLintsDeclaredType(file, v)
			return t != 0 && composeLintsTypeIsModifier(file, t)
		}
	}
	decl, ok := file.FlatParent(body)
	if !ok {
		return false
	}
	ctor, ok := file.FlatFindChild(decl, "primary_constructor")
	if !ok {
		return false
	}
	for param := file.FlatFirstChild(ctor); param != 0; param = file.FlatNextSib(param) {
		if file.FlatType(param) != "class_parameter" || file.FlatChildTextOrEmpty(param, "simple_identifier") != name ||
			!file.FlatHasChildOfType(param, "binding_pattern_kind") {
			continue
		}
		t := composeLintsDeclaredType(file, param)
		return t != 0 && composeLintsTypeIsModifier(file, t)
	}
	return false
}

// composeLintsDeclaredType returns the user_type or nullable_type child of a parameter or variable declaration.
func composeLintsDeclaredType(file *scanner.File, decl uint32) uint32 {
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		if t := file.FlatType(child); t == "user_type" || t == "nullable_type" {
			return child
		}
	}
	return 0
}
