package rules

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Ports of ComposedModifierDetector (UnnecessaryComposedModifier), SuspiciousModifierThenDetector,
// ModifierNodeInspectablePropertiesDetector and SuspiciousCompositionLocalModifierReadDetector from
// androidx.compose.ui:ui-android 1.12.1.

const (
	acuiNodePackage     = "androidx.compose.ui.node"
	acuiPlatformPackage = "androidx.compose.ui.platform"
)

// UnnecessaryComposedModifierRule ports UnnecessaryComposedModifier: Modifier.composed whose factory calls no
// @Composable function only adds cost.
type UnnecessaryComposedModifierRule struct {
	FlatDispatchBase
	BaseRule
}

// SuspiciousModifierThenRule ports SuspiciousModifierThen: `then(factory())` where the factory chains on the implicit
// receiver adds the receiver to the chain twice.
type SuspiciousModifierThenRule struct {
	FlatDispatchBase
	BaseRule
}

// ModifierNodeInspectablePropertiesRule ports ModifierNodeInspectableProperties: a ModifierNodeElement without an
// inspectableProperties() override is shown reflectively in the layout inspector.
type ModifierNodeInspectablePropertiesRule struct {
	FlatDispatchBase
	BaseRule
}

// SuspiciousCompositionLocalModifierReadRule ports SuspiciousCompositionLocalModifierRead: currentValueOf read in
// onAttach/onDetach, a constructor or a lazy block is not updated when the CompositionLocal changes.
type SuspiciousCompositionLocalModifierReadRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a medium base confidence: whether the factory calls a @Composable function is decided from
// same-file declarations and a list of known non-composable callees.
func (r *UnnecessaryComposedModifierRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports a medium base confidence: a Modifier extension callee is proven from same-file declarations
// and known Compose Modifier factories.
func (r *SuspiciousModifierThenRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *UnnecessaryComposedModifierRule) fileMentions() []string {
	return []string{"androidx.compose.ui"}
}
func (r *SuspiciousModifierThenRule) fileMentions() []string { return []string{"androidx.compose.ui"} }
func (r *ModifierNodeInspectablePropertiesRule) fileMentions() []string {
	return []string{"ModifierNodeElement"}
}
func (r *SuspiciousCompositionLocalModifierReadRule) fileMentions() []string {
	return []string{acuiNodePackage}
}

func (r *UnnecessaryComposedModifierRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UnnecessaryComposedModifier", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: []string{
			"A factory that calls a function declared in another file (other than known Compose Modifier factories " +
				"and Kotlin stdlib functions), reads `.current`, or reads a property of an object other than Modifier, " +
				"Color, Alignment or Arrangement is assumed to call a @Composable function, so it is not reported.",
		}}
}

func (r *SuspiciousModifierThenRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "SuspiciousModifierThen", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: []string{
			"Only implicit-receiver factories declared in the same file or known Compose Modifier factories are " +
				"recognized; calls inside a lambda in the then() argument are not checked.",
		}}
}

func (r *ModifierNodeInspectablePropertiesRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ModifierNodeInspectableProperties", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: []string{
			"A class whose superclass chain leaves the file before reaching ModifierNodeElement is not checked.",
		}}
}

func (r *SuspiciousCompositionLocalModifierReadRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "SuspiciousCompositionLocalModifierRead", RuleSet: "compose", DefaultActive: true}
}

func registerAndroidxComposeUIModifierUsageRules() {
	kotlin := []scanner.Language{scanner.LangKotlin}
	composed := &UnnecessaryComposedModifierRule{BaseRule: BaseRule{RuleName: "UnnecessaryComposedModifier",
		RuleSetName: "compose", Sev: "warning",
		Desc: "Detects Modifier.composed factories that call no @Composable function, which only adds cost."}}
	registerAcuiRule(composed.BaseRule, composed, []string{"call_expression"}, kotlin, api.FixNone, composed.check)

	then := &SuspiciousModifierThenRule{BaseRule: BaseRule{RuleName: "SuspiciousModifierThen", RuleSetName: "compose",
		Sev: "error", Desc: "Detects Modifier.then(factory()) where the factory chains on the implicit receiver, adding it twice."}}
	registerAcuiRule(then.BaseRule, then, []string{"call_expression"}, kotlin, api.FixNone, then.check)

	inspectable := &ModifierNodeInspectablePropertiesRule{BaseRule: BaseRule{RuleName: "ModifierNodeInspectableProperties",
		RuleSetName: "compose", Sev: "info",
		Desc: "Detects ModifierNodeElement classes that do not override inspectableProperties() for the layout inspector."}}
	registerAcuiRule(inspectable.BaseRule, inspectable,
		[]string{"class_declaration", "object_declaration", "object_literal"},
		[]scanner.Language{scanner.LangKotlin, scanner.LangJava}, api.FixNone, inspectable.check)

	local := &SuspiciousCompositionLocalModifierReadRule{BaseRule: BaseRule{RuleName: "SuspiciousCompositionLocalModifierRead",
		RuleSetName: "compose", Sev: "error",
		Desc: "Detects CompositionLocal reads in Modifier.Node onAttach/onDetach, constructors or lazy blocks."}}
	registerAcuiRule(local.BaseRule, local, []string{"call_expression"}, kotlin, api.FixNone, local.check)
}

// acuiCallee returns the callee name, its name node and whether it has an explicit receiver, looking through the
// inner call of `foo(args) { }`. It reports false for the inner call itself, which the outer node represents.
func acuiCallee(file *scanner.File, call uint32) (name string, nameNode uint32, qualified, ok bool) {
	if acrIsInnerCallOfTrailingLambda(file, call) {
		return "", 0, false, false
	}
	name, nameNode, qualified = acrCallCallee(file, call)
	return strings.Trim(name, "`"), nameNode, qualified, name != ""
}

// acuiExtensionCallTargets reports whether a call named name targets the top-level (extension) function pkg.name:
// the file binds the name to it and declares no function of that name.
func acuiExtensionCallTargets(file *scanner.File, call uint32, name, pkg string) bool {
	return len(acrFunctionDecls(file, name)) == 0 && !acrFunctionNameShadowed(file, call, name) &&
		acrNameResolvesTo(file, pkg, name)
}

// acuiCallSuffixes returns the call_suffix nodes of a call, including the inner call of `foo(args) { }`.
func acuiCallSuffixes(file *scanner.File, call uint32) []uint32 {
	var out []uint32
	for c := call; c != 0; {
		var inner uint32
		for child := file.FlatFirstChild(c); child != 0; child = file.FlatNextSib(child) {
			switch {
			case file.FlatType(child) == "call_suffix":
				out = append(out, child)
			case file.FlatType(child) == "call_expression" && acrIsInnerCallOfTrailingLambda(file, child):
				inner = child
			}
		}
		c = inner
	}
	return out
}

// acuiCallArguments returns a call's value arguments (with labels) and its trailing lambda, if any.
func acuiCallArguments(file *scanner.File, call uint32) (args []uint32, labels []string, trailing uint32) {
	suffixes := acuiCallSuffixes(file, call)
	for i := len(suffixes) - 1; i >= 0; i-- {
		for child := file.FlatFirstChild(suffixes[i]); child != 0; child = file.FlatNextSib(child) {
			switch file.FlatType(child) {
			case "value_arguments":
				for arg := file.FlatFirstChild(child); arg != 0; arg = file.FlatNextSib(arg) {
					if file.FlatType(arg) == "value_argument" {
						args = append(args, flatValueArgumentExpression(file, arg))
						labels = append(labels, flatValueArgumentLabel(file, arg))
					}
				}
			case "annotated_lambda":
				if l, ok := file.FlatFindChild(child, "lambda_literal"); ok {
					trailing = l
				}
			}
		}
	}
	return args, labels, trailing
}

// ---------------------------------------------------------------------------
// UnnecessaryComposedModifier
// ---------------------------------------------------------------------------

func (r *UnnecessaryComposedModifierRule) check(ctx *api.Context) {
	file, call := ctx.File, ctx.Idx
	name, nameNode, _, ok := acuiCallee(file, call)
	if !ok || name != "composed" || !acuiExtensionCallTargets(file, call, name, acuiUIPackage) {
		return
	}
	factory := acuiComposedFactory(file, call)
	if factory == 0 || acuiMayCallComposable(file, factory) {
		return
	}
	ctx.EmitAtNode(nameNode, "Unnecessary use of Modifier.composed")
}

// acuiComposedFactory returns the argument bound to composed's `factory` parameter: the label, the trailing
// lambda, or the second positional argument (the first is inspectorInfo).
func acuiComposedFactory(file *scanner.File, call uint32) uint32 {
	args, labels, trailing := acuiCallArguments(file, call)
	positional := 0
	for i, label := range labels {
		switch label {
		case "factory":
			return args[i]
		case "":
			positional++
			if positional == 2 {
				return args[i]
			}
		}
	}
	return trailing
}

// acuiNonComposableCallees are callees that are never @Composable: Kotlin stdlib functions and Compose Modifier
// factories.
var acuiNonComposableCallees = map[string]bool{
	"let": true, "run": true, "apply": true, "also": true, "takeIf": true, "takeUnless": true, "map": true,
	"forEach": true, "filter": true, "fold": true, "toFloat": true, "toInt": true, "toDp": true, "toPx": true,
	"roundToPx": true, "coerceAtLeast": true, "coerceAtMost": true, "coerceIn": true, "copy": true,
}

// acuiSafePropertyOwners are capitalized receivers whose properties are not @Composable getters.
var acuiSafePropertyOwners = map[string]bool{
	"Modifier": true, "Color": true, "Alignment": true, "Arrangement": true, "Dp": true, "TextUnit": true,
	"Offset": true, "Size": true, "IntOffset": true, "IntSize": true,
}

// acuiMayCallComposable reports whether a lambda may invoke a @Composable function or getter. A call proves nothing
// when it names a same-file function that is not @Composable, a known Modifier factory, a stdlib function or a
// same-file class constructor; anything else, a `.current` read, or a property read on another object, may be
// composable.
func acuiMayCallComposable(file *scanner.File, lambda uint32) bool {
	may := false
	file.FlatWalkAllNodes(lambda, func(n uint32) {
		if may {
			return
		}
		switch file.FlatType(n) {
		case "annotation":
			if acrAnnotationIsComposable(file, n) {
				may = true
			}
		case "call_expression":
			name, _, _, ok := acuiCallee(file, n)
			if !ok {
				return
			}
			may = acuiCalleeMayBeComposable(file, name)
		case "navigation_expression":
			if parent, ok := file.FlatParent(n); ok && file.FlatType(parent) == "call_expression" && file.FlatFirstChild(parent) == n {
				return
			}
			suffix := flatLastIdentifierNode(file, n)
			if suffix != 0 && file.FlatNodeTextEquals(suffix, "current") {
				may = true
				return
			}
			recv := file.FlatFirstChild(n)
			if file.FlatType(recv) == "simple_identifier" {
				text := file.FlatNodeText(recv)
				if acrFirstRuneIsUpper(text) && !acuiSafePropertyOwners[text] {
					may = true
				}
			}
		}
	})
	return may
}

func acuiCalleeMayBeComposable(file *scanner.File, name string) bool {
	if decls := acrFunctionDecls(file, name); len(decls) > 0 {
		for _, fn := range decls {
			if acrFunctionIsComposable(file, fn) {
				return true
			}
		}
		return false
	}
	if acuiModifierFactoryNames[name] || acuiNonReceiverFunctions[name] || acuiNonComposableCallees[name] {
		return false
	}
	if acrFirstRuneIsUpper(name) && acrFileDeclaresTopLevel(file, name) {
		return false
	}
	return true
}

// ---------------------------------------------------------------------------
// SuspiciousModifierThen
// ---------------------------------------------------------------------------

// check mirrors SuspiciousModifierThenDetector: in Modifier.then(arg), every call in arg that has no explicit
// receiver and resolves to a Modifier extension bound to a receiver from outside arg is reported.
func (r *SuspiciousModifierThenRule) check(ctx *api.Context) {
	file, call := ctx.File, ctx.Idx
	name, _, qualified, ok := acuiCallee(file, call)
	if !ok || name != "then" || len(acrFunctionDecls(file, "then")) > 0 || acrFunctionNameShadowed(file, call, "then") {
		return
	}
	if !acuiThenOnModifier(file, call, qualified) {
		return
	}
	args, _, trailing := acuiCallArguments(file, call)
	arg := trailing
	if len(args) > 0 {
		arg = args[0]
	}
	if arg == 0 {
		return
	}
	acuiWalkOutsideLambdas(file, arg, func(n uint32) {
		if file.FlatType(n) != "call_expression" {
			return
		}
		inner, nameNode, innerQualified, ok := acuiCallee(file, n)
		if !ok || innerQualified || !acuiIsModifierExtension(file, n, inner) {
			return
		}
		ctx.EmitAtNode(nameNode, "Using Modifier.then with a Modifier factory function with an implicit receiver")
	})
}

// acuiThenOnModifier reports whether a then() call's receiver is a Modifier: `this` or an unqualified call inside a
// Modifier extension or composed factory, the Modifier companion or a chain on it, or a parameter typed Modifier.
func acuiThenOnModifier(file *scanner.File, call uint32, qualified bool) bool {
	if !qualified {
		return acuiInModifierReceiverScope(file, call)
	}
	c, ok := lintCallAt(file, acrCalleeCall(file, call))
	if !ok || c.ReceiverNode == 0 {
		return false
	}
	recv := acuiSkipParens(file, c.ReceiverNode)
	switch file.FlatType(recv) {
	case "this_expression":
		return acuiInModifierReceiverScope(file, call)
	case "simple_identifier":
		if acuiIsModifierChain(file, recv) {
			return true
		}
		return acuiNameIsModifierParameter(file, call, file.FlatNodeText(recv))
	}
	return acuiIsModifierChain(file, recv)
}

// acrCalleeCall returns the call node that holds the callee of call: the inner call of `foo(args) { }`.
func acrCalleeCall(file *scanner.File, call uint32) uint32 {
	for {
		first := file.FlatFirstChild(call)
		if first == 0 || file.FlatType(first) != "call_expression" || !acrIsInnerCallOfTrailingLambda(file, first) {
			return call
		}
		call = first
	}
}

// acuiInModifierReceiverScope reports whether the nearest receiver scope around node is a Modifier: an extension
// function or property on Modifier, or the factory lambda of composed.
func acuiInModifierReceiverScope(file *scanner.File, node uint32) bool {
	for p, ok := file.FlatParent(node); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "lambda_literal":
			outer, ok := file.FlatParent(p)
			if ok && file.FlatType(outer) == "annotated_lambda" {
				if suffix, ok := file.FlatParent(outer); ok {
					if c, ok := file.FlatParent(suffix); ok && file.FlatType(c) == "call_expression" {
						if name, _, _, ok := acuiCallee(file, c); ok && name == "composed" {
							return true
						}
					}
				}
			}
			// Another lambda may bring its own receiver.
			return false
		case "function_declaration":
			receiver, _ := acuiFunctionReceiverAndReturnType(file, p)
			return acuiReceiverKindOf(file, receiver) == acuiReceiverModifier
		case "property_declaration":
			vd, _ := file.FlatFindChild(p, "variable_declaration")
			return acuiReceiverKindOf(file, acuiReceiverTypeBefore(file, p, vd)) == acuiReceiverModifier
		case "class_declaration", "object_declaration", "object_literal", "source_file":
			return false
		}
	}
	return false
}

// acuiNameIsModifierParameter reports whether name is a parameter of an enclosing function declared as Modifier.
func acuiNameIsModifierParameter(file *scanner.File, node uint32, name string) bool {
	for p, ok := file.FlatParent(node); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) != "function_declaration" {
			continue
		}
		params, ok := file.FlatFindChild(p, "function_value_parameters")
		if !ok {
			continue
		}
		for param := file.FlatFirstChild(params); param != 0; param = file.FlatNextSib(param) {
			if file.FlatType(param) == "parameter" && file.FlatChildTextOrEmpty(param, "simple_identifier") == name {
				return acuiModifierTypeKind(file, acuiTypeChild(file, param)) == acuiModifierExact
			}
		}
	}
	return false
}

// acuiIsModifierExtension reports whether an unqualified call named name resolves to a Modifier extension: a
// same-file `fun Modifier.name`, or a known Compose Modifier factory the file does not redeclare or shadow.
func acuiIsModifierExtension(file *scanner.File, call uint32, name string) bool {
	if decls := acrFunctionDecls(file, name); len(decls) > 0 {
		for _, fn := range decls {
			if receiver, _ := acuiFunctionReceiverAndReturnType(file, fn); acuiReceiverKindOf(file, receiver) == acuiReceiverModifier {
				return true
			}
		}
		return false
	}
	return acuiModifierFactoryNames[name] && !acrFunctionNameShadowed(file, call, name)
}

// acuiWalkOutsideLambdas visits root's subtree without entering lambdas or declarations, whose receivers are
// below root and so do not count for SuspiciousModifierThen.
func acuiWalkOutsideLambdas(file *scanner.File, root uint32, visit func(uint32)) {
	var walk func(n uint32)
	walk = func(n uint32) {
		switch file.FlatType(n) {
		case "lambda_literal", "anonymous_function", "object_literal", "function_declaration", "class_declaration":
			return
		}
		visit(n)
		for child := file.FlatFirstChild(n); child != 0; child = file.FlatNextSib(child) {
			walk(child)
		}
	}
	walk(root)
}

// ---------------------------------------------------------------------------
// ModifierNodeInspectableProperties
// ---------------------------------------------------------------------------

func (r *ModifierNodeInspectablePropertiesRule) check(ctx *api.Context) {
	file, decl := ctx.File, ctx.Idx
	if file.Language == scanner.LangJava {
		r.checkJava(ctx)
		return
	}
	if !acuiKotlinExtendsNodeElement(file, decl, 0) {
		return
	}
	if overrides, known := acuiKotlinOverridesInspectable(file, decl, 0); overrides || !known {
		return
	}
	name, nameNode := "null", uint32(0)
	switch file.FlatType(decl) {
	case "object_literal":
		nameNode = file.FlatFirstChild(decl)
	default:
		for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
			if t := file.FlatType(child); t == "type_identifier" || t == "simple_identifier" {
				nameNode = child
				name = file.FlatNodeText(child)
				break
			}
		}
	}
	if nameNode == 0 {
		return
	}
	ctx.EmitAtNode(nameNode, acuiInspectableMessage(name))
}

func acuiInspectableMessage(name string) string {
	return fmt.Sprintf("%s does not override inspectableProperties(). The layout inspector will use the default "+
		"implementation of this function, which will attempt to read %s's properties reflectively. Override "+
		"inspectableProperties() if you'd like to customize this modifier's presentation in the layout inspector.",
		name, name)
}

// acuiSupertypeNames returns the simple or qualified type names (type arguments stripped) a Kotlin class, object or
// object literal lists as supertypes.
func acuiSupertypeNames(file *scanner.File, decl uint32) []string {
	var out []string
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "delegation_specifier" {
			continue
		}
		var typeNode uint32
		file.FlatWalkAllNodes(child, func(n uint32) {
			if typeNode == 0 && file.FlatType(n) == "user_type" {
				typeNode = n
			}
		})
		if typeNode == 0 {
			continue
		}
		text := strings.Join(strings.Fields(file.FlatNodeText(typeNode)), "")
		if lt := strings.IndexByte(text, '<'); lt >= 0 {
			text = text[:lt]
		}
		out = append(out, text)
	}
	return out
}

// acuiTypeNameIs reports whether a type name written in file refers to pkg.name.
func acuiTypeNameIs(file *scanner.File, text, pkg, name string) bool {
	return text == pkg+"."+name || acrSimpleNameRefersTo(file, text, pkg, name)
}

// acuiSameFileClass returns the top-level or nested Kotlin class declaration with the given simple name.
func acuiSameFileClass(file *scanner.File, name string) uint32 {
	var found uint32
	file.FlatWalkNodes(0, "class_declaration", func(decl uint32) {
		if found == 0 && acrDeclarationName(file, decl) == name {
			found = decl
		}
	})
	return found
}

// acuiKotlinExtendsNodeElement reports whether a declaration's superclass chain, followed through same-file
// classes, reaches androidx.compose.ui.node.ModifierNodeElement.
func acuiKotlinExtendsNodeElement(file *scanner.File, decl uint32, depth int) bool {
	if depth > 8 {
		return false
	}
	for _, super := range acuiSupertypeNames(file, decl) {
		if acuiTypeNameIs(file, super, acuiNodePackage, "ModifierNodeElement") {
			return true
		}
		if !strings.Contains(super, ".") {
			if parent := acuiSameFileClass(file, super); parent != 0 && acuiKotlinExtendsNodeElement(file, parent, depth+1) {
				return true
			}
		}
	}
	return false
}

// acuiKotlinOverridesInspectable reports whether the declaration, or a same-file superclass before
// ModifierNodeElement, declares `fun InspectorInfo.inspectableProperties()`. known is false when the chain leaves
// the file before reaching ModifierNodeElement.
func acuiKotlinOverridesInspectable(file *scanner.File, decl uint32, depth int) (overrides, known bool) {
	if depth > 8 {
		return false, false
	}
	if body, ok := file.FlatFindChild(decl, "class_body"); ok {
		for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "function_declaration" && acuiIsInspectablePropertiesOverride(file, child) {
				return true, true
			}
		}
	}
	for _, super := range acuiSupertypeNames(file, decl) {
		if acuiTypeNameIs(file, super, acuiNodePackage, "ModifierNodeElement") {
			return false, true
		}
		if strings.Contains(super, ".") {
			continue
		}
		if parent := acuiSameFileClass(file, super); parent != 0 && acuiKotlinExtendsNodeElement(file, parent, 0) {
			return acuiKotlinOverridesInspectable(file, parent, depth+1)
		}
	}
	return false, false
}

// acuiIsInspectablePropertiesOverride matches Lint's signature: named inspectableProperties, receiver
// androidx.compose.ui.platform.InspectorInfo, and no value parameters.
func acuiIsInspectablePropertiesOverride(file *scanner.File, fn uint32) bool {
	if acrDeclarationName(file, fn) != "inspectableProperties" {
		return false
	}
	if params, ok := file.FlatFindChild(fn, "function_value_parameters"); ok && file.FlatHasChildOfType(params, "parameter") {
		return false
	}
	receiver, _ := acuiFunctionReceiverAndReturnType(file, fn)
	if receiver == 0 || file.FlatType(receiver) != "user_type" {
		return false
	}
	return acuiTypeNameIs(file, strings.Join(strings.Fields(file.FlatNodeText(receiver)), ""), acuiPlatformPackage, "InspectorInfo")
}

// checkJava reports a Java class extending ModifierNodeElement: Java cannot declare the extension override Lint
// looks for, so Lint always reports it.
func (r *ModifierNodeInspectablePropertiesRule) checkJava(ctx *api.Context) {
	file, decl := ctx.File, ctx.Idx
	super, ok := file.FlatFindChild(decl, "superclass")
	if !ok {
		return
	}
	var typeText string
	for child := file.FlatFirstChild(super); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "type_identifier", "scoped_type_identifier", "generic_type":
			typeText = strings.Join(strings.Fields(file.FlatNodeText(child)), "")
		}
	}
	if lt := strings.IndexByte(typeText, '<'); lt >= 0 {
		typeText = typeText[:lt]
	}
	if typeText != acuiNodePackage+".ModifierNodeElement" &&
		(typeText != "ModifierNodeElement" || !acrJavaImports(file, acuiNodePackage, "ModifierNodeElement")) {
		return
	}
	nameNode, _ := file.FlatFindChild(decl, "identifier")
	if nameNode == 0 {
		return
	}
	ctx.EmitAtNode(nameNode, acuiInspectableMessage(file.FlatNodeText(nameNode)))
}

// ---------------------------------------------------------------------------
// SuspiciousCompositionLocalModifierRead
// ---------------------------------------------------------------------------

func (r *SuspiciousCompositionLocalModifierReadRule) check(ctx *api.Context) {
	file, call := ctx.File, ctx.Idx
	name, _, _, ok := acuiCallee(file, call)
	if !ok || name != "currentValueOf" || !acuiExtensionCallTargets(file, call, name, acuiNodePackage) {
		return
	}
	local := "a composition local"
	if args, _, _ := acuiCallArguments(file, call); len(args) > 0 && args[0] != 0 {
		local = file.FlatNodeText(args[0])
	}
	if msg := acuiNodeLifecycleReadMessage(file, call, local); msg != "" {
		ctx.EmitAtNode(call, msg)
	}
	if acuiInsideLazy(file, call) {
		ctx.EmitAtNode(call, fmt.Sprintf("Reading %s lazily will only access the CompositionLocal's value once. To be "+
			"notified of the latest value of the CompositionLocal, read the value in one of the modifier's callbacks.", local))
	}
}

// acuiNodeLifecycleReadMessage mirrors reportIfInNodeLifecycleCallback: walking up from the read, stopping at a
// lambda body, the first method decides. In a class that directly implements CompositionLocalConsumerModifierNode,
// onAttach/onDetach and constructors (init blocks and secondary constructors) are reported.
func acuiNodeLifecycleReadMessage(file *scanner.File, call uint32, local string) string {
	for p, ok := file.FlatParent(call); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "lambda_literal", "anonymous_function":
			return ""
		case "function_declaration":
			if acrIsLocalDeclaration(file, p) || !acuiDirectlyImplementsConsumerNode(file, acuiEnclosingClass(file, p)) {
				return ""
			}
			switch method := acrDeclarationName(file, p); method {
			case "onAttach", "onDetach":
				event := strings.ToLower(strings.TrimPrefix(method, "on"))
				return fmt.Sprintf("Reading %s in %s will only access the CompositionLocal's value when the modifier "+
					"is %sed. To be notified of the latest value of the CompositionLocal, read the value in one of "+
					"the modifier's other callbacks.", local, method, event)
			}
			return ""
		case "anonymous_initializer", "secondary_constructor":
			if acuiDirectlyImplementsConsumerNode(file, acuiEnclosingClass(file, p)) {
				return "CompositionLocals cannot be read in modifiers before the node is attached."
			}
			return ""
		case "getter", "setter":
			return ""
		}
	}
	return ""
}

// acuiEnclosingClass returns the class, object or object literal whose body holds node.
func acuiEnclosingClass(file *scanner.File, node uint32) uint32 {
	for p, ok := file.FlatParent(node); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "class_declaration", "object_declaration", "object_literal":
			return p
		}
	}
	return 0
}

// acuiDirectlyImplementsConsumerNode mirrors isClConsumerNode: the class's own supertypes (not inherited ones)
// include androidx.compose.ui.node.CompositionLocalConsumerModifierNode.
func acuiDirectlyImplementsConsumerNode(file *scanner.File, decl uint32) bool {
	if decl == 0 {
		return false
	}
	for _, super := range acuiSupertypeNames(file, decl) {
		if acuiTypeNameIs(file, super, acuiNodePackage, "CompositionLocalConsumerModifierNode") {
			return true
		}
	}
	return false
}

// acuiInsideLazy mirrors reportIfInLazyBlock: any enclosing call to kotlin's lazy.
func acuiInsideLazy(file *scanner.File, call uint32) bool {
	for p, ok := file.FlatParent(call); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) != "call_expression" {
			continue
		}
		if name, _, qualified, ok := acuiCallee(file, p); ok && name == "lazy" && !qualified &&
			len(acrFunctionDecls(file, "lazy")) == 0 && !acrFileDeclaresTopLevel(file, "lazy") {
			return true
		}
	}
	return false
}
