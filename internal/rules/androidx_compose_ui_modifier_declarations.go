package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Ports of ModifierDeclarationDetector (ModifierFactoryExtensionFunction, ModifierFactoryReturnType,
// ModifierFactoryUnreferencedReceiver) and ModifierParameterDetector (ModifierParameter) from
// androidx.compose.ui:ui-android 1.12.1. Lint resolves the declared or inferred type of every declaration; these
// ports only act on types the file proves: a declared type that names androidx.compose.ui.Modifier or a known
// Modifier subtype (imported, or a same-file class), or an expression body whose type the source shows.

const acuiUIPackage = "androidx.compose.ui"

// ModifierFactoryExtensionFunctionRule ports ModifierFactoryExtensionFunction: a function returning a Modifier
// should be an extension on Modifier so it can be chained.
type ModifierFactoryExtensionFunctionRule struct {
	FlatDispatchBase
	BaseRule
}

// ModifierFactoryReturnTypeRule ports ModifierFactoryReturnType: a Modifier factory should return Modifier, not a
// subtype such as Modifier.Element.
type ModifierFactoryReturnTypeRule struct {
	FlatDispatchBase
	BaseRule
}

// ModifierFactoryUnreferencedReceiverRule ports ModifierFactoryUnreferencedReceiver: a Modifier extension factory
// must use its receiver, or the chain built before it is dropped.
type ModifierFactoryUnreferencedReceiverRule struct {
	FlatDispatchBase
	BaseRule
}

// ModifierParameterRule ports ModifierParameter: the Modifier parameter of a Unit @Composable function should be
// named `modifier`, typed `Modifier`, default to `Modifier` and be the first optional parameter.
type ModifierParameterRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ModifierFactoryExtensionFunctionRule) fileMentions() []string {
	return []string{acuiUIPackage}
}
func (r *ModifierFactoryReturnTypeRule) fileMentions() []string { return []string{acuiUIPackage} }
func (r *ModifierFactoryUnreferencedReceiverRule) fileMentions() []string {
	return []string{acuiUIPackage}
}
func (r *ModifierParameterRule) fileMentions() []string { return []string{acuiUIPackage} }

var acuiModifierFactoryLimitations = []string{
	"A declaration is only checked when its declared type, or the expression body, proves it returns a Modifier.",
	"Only Kotlin declarations are checked, as Lint only reports Kotlin sources.",
}

func (r *ModifierFactoryExtensionFunctionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ModifierFactoryExtensionFunction", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: acuiModifierFactoryLimitations}
}

func (r *ModifierFactoryReturnTypeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ModifierFactoryReturnType", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: acuiModifierFactoryLimitations}
}

func (r *ModifierFactoryUnreferencedReceiverRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ModifierFactoryUnreferencedReceiver", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: append(append([]string(nil), acuiModifierFactoryLimitations...),
			"An unqualified call that may bind the receiver implicitly (any lowercase function not known to be "+
				"top-level) counts as a use, so receivers used only through such calls are not reported.")}
}

func (r *ModifierParameterRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ModifierParameter", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: []string{
			"Expression-bodied @Composable functions without a declared return type are only checked when the body " +
				"is Unit or a same-file function call: otherwise their return type needs type inference.",
		}}
}

func registerAndroidxComposeUIModifierDeclarationRules() {
	kotlin := []scanner.Language{scanner.LangKotlin}
	nodes := []string{"function_declaration", "property_declaration"}
	ext := &ModifierFactoryExtensionFunctionRule{BaseRule: BaseRule{RuleName: "ModifierFactoryExtensionFunction",
		RuleSetName: "compose", Sev: "warning",
		Desc: "Detects functions returning a Modifier that are not extensions on Modifier, so they cannot be chained."}}
	registerAcuiRule(ext.BaseRule, ext, nodes, kotlin, api.FixNone, func(ctx *api.Context) {
		if f, ok := acuiModifierFactoryAt(ctx.File, ctx.Idx); ok && f.receiver != acuiReceiverModifier &&
			f.receiver != acuiReceiverUnsupported {
			ctx.EmitAtNode(f.name, "Modifier factory functions should be extensions on Modifier")
		}
	})

	ret := &ModifierFactoryReturnTypeRule{BaseRule: BaseRule{RuleName: "ModifierFactoryReturnType", RuleSetName: "compose",
		Sev: "warning", Desc: "Detects Modifier factory functions that return a Modifier subtype instead of Modifier."}}
	registerAcuiRule(ret.BaseRule, ret, nodes, kotlin, api.FixNone, func(ctx *api.Context) {
		if f, ok := acuiModifierFactoryAt(ctx.File, ctx.Idx); ok && f.kind == acuiModifierSubtype {
			ctx.EmitAtNode(f.name, "Modifier factory functions should have a return type of Modifier")
		}
	})

	unref := &ModifierFactoryUnreferencedReceiverRule{BaseRule: BaseRule{RuleName: "ModifierFactoryUnreferencedReceiver",
		RuleSetName: "compose", Sev: "error",
		Desc: "Detects Modifier extension factories whose body never uses the receiver Modifier."}}
	registerAcuiRule(unref.BaseRule, unref, nodes, kotlin, api.FixNone, func(ctx *api.Context) {
		f, ok := acuiModifierFactoryAt(ctx.File, ctx.Idx)
		if !ok || f.receiver != acuiReceiverModifier || f.body == 0 || acuiBodyMayUseReceiver(ctx.File, f) {
			return
		}
		ctx.EmitAtNode(f.name, "Modifier factory functions must use the receiver Modifier instance")
	})

	param := &ModifierParameterRule{BaseRule: BaseRule{RuleName: "ModifierParameter", RuleSetName: "compose",
		Sev: "warning", Desc: "Detects Modifier parameters of @Composable functions that are not named modifier, " +
			"typed Modifier, defaulted to Modifier, or the first optional parameter."}}
	registerAcuiRule(param.BaseRule, param, []string{"function_declaration"}, kotlin, api.FixNone, param.check)
}

// acuiModifierKind classifies a type: not a Modifier, exactly androidx.compose.ui.Modifier, a Modifier subtype, or
// a ParentDataModifier (which Lint exempts from the factory checks).
type acuiModifierKind int

const (
	acuiNotModifier acuiModifierKind = iota
	acuiModifierExact
	acuiModifierSubtype
	acuiModifierParentData
)

// acuiReceiverKind classifies a factory's extension receiver.
type acuiReceiverKind int

const (
	acuiReceiverNone acuiReceiverKind = iota
	acuiReceiverModifier
	acuiReceiverOther
	// acuiReceiverUnsupported is a receiver Lint does not inspect (a function or parenthesized type).
	acuiReceiverUnsupported
)

// acuiKnownModifierSubtypes lists public androidx.compose.ui interfaces and classes that are Modifier subtypes, by
// simple name and package.
var acuiKnownModifierSubtypes = map[string]string{
	"LayoutModifier":               "androidx.compose.ui.layout",
	"OnGloballyPositionedModifier": "androidx.compose.ui.layout",
	"OnPlacedModifier":             "androidx.compose.ui.layout",
	"OnRemeasuredModifier":         "androidx.compose.ui.layout",
	"DrawModifier":                 "androidx.compose.ui.draw",
	"DrawCacheModifier":            "androidx.compose.ui.draw",
	"PointerInputModifier":         "androidx.compose.ui.input.pointer",
	"SemanticsModifier":            "androidx.compose.ui.semantics",
	"ModifierLocalConsumer":        "androidx.compose.ui.modifier",
	"ModifierLocalProvider":        "androidx.compose.ui.modifier",
	"FocusEventModifier":           "androidx.compose.ui.focus",
	"FocusOrderModifier":           "androidx.compose.ui.focus",
	"ModifierNodeElement":          "androidx.compose.ui.node",
}

// acuiModifierTypeKind classifies a type node (user_type or nullable_type).
func acuiModifierTypeKind(file *scanner.File, typeNode uint32) acuiModifierKind {
	return acuiModifierTypeKindDepth(file, typeNode, 0)
}

func acuiModifierTypeKindDepth(file *scanner.File, typeNode uint32, depth int) acuiModifierKind {
	if typeNode == 0 || depth > 4 {
		return acuiNotModifier
	}
	if file.FlatType(typeNode) == "nullable_type" {
		typeNode = file.FlatNamedChild(typeNode, 0)
	}
	if file.FlatType(typeNode) != "user_type" {
		return acuiNotModifier
	}
	text := strings.Join(strings.Fields(file.FlatNodeText(typeNode)), "")
	if lt := strings.IndexByte(text, '<'); lt >= 0 {
		text = text[:lt]
	}
	return acuiModifierTypeNameKind(file, text, depth)
}

// acuiModifierTypeNameKind classifies a type written as text (type arguments stripped).
func acuiModifierTypeNameKind(file *scanner.File, text string, depth int) acuiModifierKind {
	const fqn = acuiUIPackage + ".Modifier"
	switch text {
	case fqn:
		return acuiModifierExact
	case fqn + ".Element":
		return acuiModifierSubtype
	case "androidx.compose.ui.layout.ParentDataModifier":
		return acuiModifierParentData
	}
	if head, rest, dotted := strings.Cut(text, "."); dotted {
		if rest == "Element" && acrSimpleNameRefersTo(file, head, acuiUIPackage, "Modifier") {
			return acuiModifierSubtype
		}
		if pkg, ok := acuiKnownModifierSubtypes[text[strings.LastIndexByte(text, '.')+1:]]; ok &&
			text == pkg+"."+text[strings.LastIndexByte(text, '.')+1:] {
			return acuiModifierSubtype
		}
		return acuiNotModifier
	}
	if acrSimpleNameRefersTo(file, text, acuiUIPackage, "Modifier") {
		return acuiModifierExact
	}
	if acrSimpleNameRefersTo(file, text, "androidx.compose.ui.layout", "ParentDataModifier") {
		return acuiModifierParentData
	}
	for name, pkg := range acuiKnownModifierSubtypes {
		if acrSimpleNameRefersTo(file, text, pkg, name) {
			return acuiModifierSubtype
		}
	}
	return acuiSameFileClassModifierKind(file, text, depth)
}

// acuiSameFileClassModifierKind classifies a class, object or interface declared in the file by its supertypes.
func acuiSameFileClassModifierKind(file *scanner.File, name string, depth int) acuiModifierKind {
	if depth > 4 || !acrFileDeclaresTopLevel(file, name) {
		return acuiNotModifier
	}
	kind := acuiNotModifier
	file.FlatWalkNodes(0, "class_declaration", func(decl uint32) {
		if kind == acuiNotModifier && acrDeclarationName(file, decl) == name {
			kind = acuiSupertypesModifierKind(file, decl, depth+1)
		}
	})
	file.FlatWalkNodes(0, "object_declaration", func(decl uint32) {
		if kind == acuiNotModifier && acrDeclarationName(file, decl) == name {
			kind = acuiSupertypesModifierKind(file, decl, depth+1)
		}
	})
	return kind
}

// acuiSupertypesModifierKind returns the strongest Modifier kind among the declared supertypes of a class or object
// declaration or object literal: ParentDataModifier wins, then any Modifier subtype.
func acuiSupertypesModifierKind(file *scanner.File, decl uint32, depth int) acuiModifierKind {
	kind := acuiNotModifier
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
		switch k := acuiModifierTypeKindDepth(file, typeNode, depth); k {
		case acuiModifierParentData:
			return k
		case acuiModifierExact, acuiModifierSubtype:
			kind = acuiModifierSubtype
		}
	}
	return kind
}

// acuiModifierFactory describes a function or top-level property getter that returns a Modifier.
type acuiModifierFactory struct {
	decl     uint32
	name     uint32
	kind     acuiModifierKind
	receiver acuiReceiverKind
	// body is the function body or getter body, or 0 for an abstract or interface function.
	body uint32
}

// acuiModifierFactoryAt mirrors ModifierDeclarationDetector's selection: UMethods (functions, including members,
// overrides and interface functions, and the getters of top-level `val`s with a getter) whose return type is a
// Modifier but not a ParentDataModifier. Local functions and properties without a getter are not UMethods.
func acuiModifierFactoryAt(file *scanner.File, decl uint32) (acuiModifierFactory, bool) {
	f := acuiModifierFactory{decl: decl}
	var receiverType, returnType uint32
	switch file.FlatType(decl) {
	case "function_declaration":
		if acrIsLocalDeclaration(file, decl) {
			return f, false
		}
		f.name = acrNameNode(file, decl)
		receiverType, returnType = acuiFunctionReceiverAndReturnType(file, decl)
		if body, ok := file.FlatFindChild(decl, "function_body"); ok {
			f.body = body
		}
	case "property_declaration":
		if acrIsLocalDeclaration(file, decl) || acuiInsideClassBody(file, decl) {
			return f, false
		}
		kind, _ := file.FlatFindChild(decl, "binding_pattern_kind")
		if strings.TrimSpace(file.FlatNodeText(kind)) != "val" {
			return f, false
		}
		getter, ok := file.FlatFindChild(decl, "getter")
		if !ok {
			return f, false
		}
		body, _ := file.FlatFindChild(getter, "function_body")
		if body == 0 {
			return f, false
		}
		f.body = body
		vd, _ := file.FlatFindChild(decl, "variable_declaration")
		f.name, _ = file.FlatFindChild(vd, "simple_identifier")
		returnType = acuiTypeChild(file, vd)
		if gt := acuiTypeChild(file, getter); gt != 0 {
			returnType = gt
		}
		receiverType = acuiReceiverTypeBefore(file, decl, vd)
	default:
		return f, false
	}
	if f.name == 0 {
		return f, false
	}
	if returnType != 0 {
		f.kind = acuiModifierTypeKind(file, returnType)
	} else if f.body != 0 {
		f.kind = acuiExpressionBodyModifierKind(file, f.body)
	}
	if f.kind != acuiModifierExact && f.kind != acuiModifierSubtype {
		return f, false
	}
	f.receiver = acuiReceiverKindOf(file, receiverType)
	return f, true
}

// acuiFunctionReceiverAndReturnType returns the receiver type (before the name) and the declared return type
// (after the parameters) of a function declaration.
func acuiFunctionReceiverAndReturnType(file *scanner.File, fn uint32) (receiver, ret uint32) {
	afterName, afterParams, sawColon := false, false, false
	for child := file.FlatFirstChild(fn); child != 0; child = file.FlatNextSib(child) {
		t := file.FlatType(child)
		switch {
		case t == "simple_identifier" && !afterName:
			afterName = true
		case !afterName && acuiIsTypeNode(t):
			receiver = child
		case t == "function_value_parameters":
			afterParams = true
		case afterParams && t == ":":
			sawColon = true
		case afterParams && sawColon && acuiIsTypeNode(t):
			return receiver, child
		}
	}
	return receiver, 0
}

// acuiReceiverTypeBefore returns the receiver type of an extension property: the type node before its variable
// declaration.
func acuiReceiverTypeBefore(file *scanner.File, decl, vd uint32) uint32 {
	var receiver uint32
	for child := file.FlatFirstChild(decl); child != 0 && child != vd; child = file.FlatNextSib(child) {
		if acuiIsTypeNode(file.FlatType(child)) {
			receiver = child
		}
	}
	return receiver
}

// acuiTypeChild returns the type node following a `:` among node's children.
func acuiTypeChild(file *scanner.File, node uint32) uint32 {
	sawColon := false
	for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
		t := file.FlatType(child)
		if t == ":" {
			sawColon = true
		} else if sawColon && acuiIsTypeNode(t) {
			return child
		}
	}
	return 0
}

func acuiIsTypeNode(t string) bool {
	switch t {
	case "user_type", "nullable_type", "function_type", "parenthesized_type", "dynamic_type":
		return true
	}
	return false
}

// acuiInsideClassBody reports whether a declaration is a member of a class, object or interface.
func acuiInsideClassBody(file *scanner.File, decl uint32) bool {
	p, ok := file.FlatParent(decl)
	return ok && (file.FlatType(p) == "class_body" || file.FlatType(p) == "enum_class_body")
}

// acuiReceiverKindOf mirrors checkReceiver: no receiver; a (nullable) user type that names
// androidx.compose.ui.Modifier; another user type; or a function or parenthesized type, which Lint skips.
func acuiReceiverKindOf(file *scanner.File, receiver uint32) acuiReceiverKind {
	if receiver == 0 {
		return acuiReceiverNone
	}
	if file.FlatType(receiver) == "nullable_type" {
		receiver = file.FlatNamedChild(receiver, 0)
	}
	if file.FlatType(receiver) != "user_type" {
		return acuiReceiverUnsupported
	}
	text := strings.Join(strings.Fields(file.FlatNodeText(receiver)), "")
	if lt := strings.IndexByte(text, '<'); lt >= 0 {
		text = text[:lt]
	}
	if text == acuiUIPackage+".Modifier" {
		return acuiReceiverModifier
	}
	if strings.Contains(text, ".") {
		return acuiReceiverOther
	}
	// Lint compares the resolved class; when the name does not resolve it compares the simple name, so a
	// `Modifier` receiver only fails when the file binds that name to another class.
	if text == "Modifier" {
		if facts := fileFactsCache().Imports(file); facts != nil {
			if target, ok := facts.Aliases[text]; ok && target != acuiUIPackage+".Modifier" {
				return acuiReceiverOther
			}
		}
		if acrFileDeclaresTopLevel(file, text) && filePackageName(file) != acuiUIPackage {
			return acuiReceiverOther
		}
		return acuiReceiverModifier
	}
	if acrSimpleNameRefersTo(file, text, acuiUIPackage, "Modifier") {
		return acuiReceiverModifier
	}
	return acuiReceiverOther
}

// acuiExpressionBodyModifierKind infers the type of an untyped expression body when the source shows it: the
// Modifier companion, a Modifier chain ending in a known Modifier factory, an object literal or a same-file class
// constructor whose supertypes are Modifiers.
func acuiExpressionBodyModifierKind(file *scanner.File, body uint32) acuiModifierKind {
	first := file.FlatFirstChild(body)
	if first == 0 || file.FlatType(first) != "=" {
		return acuiNotModifier
	}
	return acuiExpressionModifierKind(file, flatLastNamedChild(file, body))
}

func acuiExpressionModifierKind(file *scanner.File, expr uint32) acuiModifierKind {
	expr = acuiSkipParens(file, expr)
	switch file.FlatType(expr) {
	case "simple_identifier":
		if file.FlatNodeTextEquals(expr, "Modifier") && acrSimpleNameRefersTo(file, "Modifier", acuiUIPackage, "Modifier") {
			return acuiModifierExact
		}
	case "object_literal":
		return acuiSupertypesModifierKind(file, expr, 0)
	case "call_expression":
		call, ok := lintCallAt(file, expr)
		if !ok {
			return acuiNotModifier
		}
		if call.ReceiverNode == 0 {
			if acrFirstRuneIsUpper(call.Name) {
				return acuiSameFileClassModifierKind(file, call.Name, 0)
			}
			return acuiNotModifier
		}
		if acuiIsModifierChain(file, call.ReceiverNode) && acuiReturnsModifier(file, call.Name) {
			return acuiModifierExact
		}
	}
	return acuiNotModifier
}

// acuiIsModifierChain reports whether expr is the Modifier companion or a chain of Modifier factory calls on it.
func acuiIsModifierChain(file *scanner.File, expr uint32) bool {
	for depth := 0; depth < 32; depth++ {
		expr = acuiSkipParens(file, expr)
		switch file.FlatType(expr) {
		case "simple_identifier":
			return file.FlatNodeTextEquals(expr, "Modifier") && acrSimpleNameRefersTo(file, "Modifier", acuiUIPackage, "Modifier")
		case "call_expression":
			call, ok := lintCallAt(file, expr)
			if !ok || call.ReceiverNode == 0 || !acuiReturnsModifier(file, call.Name) {
				return false
			}
			expr = call.ReceiverNode
		default:
			return false
		}
	}
	return false
}

// acuiReturnsModifier reports whether a Modifier extension of this name returns Modifier: Modifier.then, a known
// Compose Modifier factory, or a same-file `fun Modifier.name(...): Modifier`.
func acuiReturnsModifier(file *scanner.File, name string) bool {
	if acuiModifierFactoryNames[name] {
		return true
	}
	for _, fn := range acrFunctionDecls(file, name) {
		receiver, ret := acuiFunctionReceiverAndReturnType(file, fn)
		if acuiReceiverKindOf(file, receiver) == acuiReceiverModifier && acuiModifierTypeKind(file, ret) == acuiModifierExact {
			return true
		}
	}
	return false
}

// acuiModifierFactoryNames are Modifier members and Compose Modifier factories (androidx.compose.ui and foundation)
// that return Modifier.
var acuiModifierFactoryNames = map[string]bool{
	"then": true, "composed": true, "padding": true, "paddingFrom": true, "absolutePadding": true, "size": true,
	"width": true, "height": true, "requiredSize": true, "requiredWidth": true, "requiredHeight": true,
	"sizeIn": true, "widthIn": true, "heightIn": true, "defaultMinSize": true, "fillMaxSize": true,
	"fillMaxWidth": true, "fillMaxHeight": true, "wrapContentSize": true, "wrapContentWidth": true,
	"wrapContentHeight": true, "aspectRatio": true, "offset": true, "absoluteOffset": true, "background": true,
	"border": true, "clip": true, "clipToBounds": true, "alpha": true, "rotate": true, "scale": true,
	"shadow": true, "graphicsLayer": true, "zIndex": true, "clickable": true, "combinedClickable": true,
	"selectable": true, "toggleable": true, "focusable": true, "focusRequester": true, "onFocusChanged": true,
	"semantics": true, "clearAndSetSemantics": true, "testTag": true, "layoutId": true, "layout": true,
	"drawBehind": true, "drawWithContent": true, "drawWithCache": true, "pointerInput": true,
	"onGloballyPositioned": true, "onSizeChanged": true, "onPlaced": true, "verticalScroll": true,
	"horizontalScroll": true, "scrollable": true, "draggable": true, "nestedScroll": true, "paint": true,
	"blur": true, "onKeyEvent": true, "onPreviewKeyEvent": true, "focusProperties": true, "focusGroup": true,
	"indication": true, "hoverable": true, "statusBarsPadding": true, "navigationBarsPadding": true,
	"systemBarsPadding": true, "imePadding": true, "safeDrawingPadding": true, "windowInsetsPadding": true,
	"animateContentSize": true,
}

// acuiNonReceiverFunctions are Kotlin top-level functions an unqualified call can name without binding an
// implicit receiver.
var acuiNonReceiverFunctions = map[string]bool{
	"TODO": true, "error": true, "require": true, "requireNotNull": true, "check": true, "checkNotNull": true,
	"println": true, "print": true, "lazy": true, "lazyOf": true, "listOf": true, "listOfNotNull": true,
	"setOf": true, "mapOf": true, "mutableListOf": true, "mutableSetOf": true, "mutableMapOf": true,
	"emptyList": true, "emptySet": true, "emptyMap": true, "arrayOf": true, "sequenceOf": true,
	"buildList": true, "buildString": true, "buildMap": true, "buildSet": true, "maxOf": true, "minOf": true,
	"repeat": true, "with": true, "synchronized": true,
}

// acuiBodyMayUseReceiver reports whether a factory body could reference its receiver: any `this`, or an unqualified
// call that may bind it implicitly. Unqualified calls to capitalized names (constructors), known top-level Kotlin
// functions, function-typed parameters and locals, and same-file non-extension functions do not.
func acuiBodyMayUseReceiver(file *scanner.File, f acuiModifierFactory) bool {
	used := false
	file.FlatWalkAllNodes(f.body, func(n uint32) {
		if used {
			return
		}
		switch file.FlatType(n) {
		case "this_expression":
			used = true
		case "call_expression":
			callee := file.FlatFirstChild(n)
			if file.FlatType(callee) != "simple_identifier" {
				return
			}
			name := strings.Trim(file.FlatNodeText(callee), "`")
			if acrFirstRuneIsUpper(name) || acuiNonReceiverFunctions[name] || acuiNameIsLocalValue(file, f.decl, n, name) {
				return
			}
			if decls := acrFunctionDecls(file, name); len(decls) > 0 {
				for _, fn := range decls {
					if receiver, _ := acuiFunctionReceiverAndReturnType(file, fn); receiver != 0 || acuiInsideClassBody(file, fn) {
						used = true
						return
					}
				}
				return
			}
			used = true
		}
	})
	return used
}

// acuiNameIsLocalValue reports whether name is a parameter of the factory or a local property declared in its body
// before the use, so `name(...)` invokes a value rather than a receiver member.
func acuiNameIsLocalValue(file *scanner.File, decl, use uint32, name string) bool {
	if params, ok := file.FlatFindChild(decl, "function_value_parameters"); ok {
		for p := file.FlatFirstChild(params); p != 0; p = file.FlatNextSib(p) {
			if file.FlatType(p) == "parameter" && file.FlatChildTextOrEmpty(p, "simple_identifier") == name {
				return true
			}
		}
	}
	found := false
	file.FlatWalkNodes(decl, "property_declaration", func(prop uint32) {
		if found || file.FlatStartByte(prop) >= file.FlatStartByte(use) {
			return
		}
		if vd, ok := file.FlatFindChild(prop, "variable_declaration"); ok && file.FlatChildTextOrEmpty(vd, "simple_identifier") == name {
			found = true
		}
	})
	return found
}

// check mirrors ModifierParameterDetector on a Unit @Composable function: its first Modifier-typed parameter.
func (r *ModifierParameterRule) check(ctx *api.Context) {
	file, fn := ctx.File, ctx.Idx
	if !acrFunctionIsComposable(file, fn) || acrIsLocalDeclaration(file, fn) || acrFunctionReturnKind(file, fn) != acrReturnUnit {
		return
	}
	params, ok := file.FlatFindChild(fn, "function_value_parameters")
	if !ok {
		return
	}
	var all []uint32
	modifierIndex, firstDefault := -1, -1
	var modifierType uint32
	for p := file.FlatFirstChild(params); p != 0; p = file.FlatNextSib(p) {
		if file.FlatType(p) != "parameter" {
			continue
		}
		idx := len(all)
		all = append(all, p)
		if firstDefault < 0 && acuiParameterDefault(file, p) != 0 {
			firstDefault = idx
		}
		if modifierIndex < 0 {
			if t := acuiTypeChild(file, p); t != 0 {
				if k := acuiModifierTypeKind(file, t); k != acuiNotModifier {
					modifierIndex, modifierType = idx, t
				}
			}
		}
	}
	if modifierIndex < 0 {
		return
	}
	param := all[modifierIndex]
	nameNode, _ := file.FlatFindChild(param, "simple_identifier")
	if nameNode == 0 {
		return
	}
	if strings.Trim(file.FlatNodeText(nameNode), "`") != "modifier" {
		ctx.EmitAtNode(nameNode, "Modifier parameter should be named modifier")
	}
	if acuiModifierTypeKind(file, modifierType) != acuiModifierExact {
		ctx.EmitAtNode(nameNode, "Modifier parameter should have a type of Modifier")
	}
	if def := acuiParameterDefault(file, param); def != 0 {
		if file.FlatType(def) != "simple_identifier" || !file.FlatNodeTextEquals(def, "Modifier") {
			ctx.EmitAtNode(nameNode, "Optional Modifier parameter should have a default value of `Modifier`")
		}
		if modifierIndex != firstDefault {
			ctx.EmitAtNode(nameNode, "Modifier parameter should be the first optional parameter")
		}
	}
}

// acuiParameterDefault returns the default value expression of a function parameter, or 0. tree-sitter places the
// `= value` after the parameter node, inside function_value_parameters.
func acuiParameterDefault(file *scanner.File, param uint32) uint32 {
	next := file.FlatNextSib(param)
	if next == 0 || file.FlatType(next) != "=" {
		return 0
	}
	// Keyword literals such as `null` are anonymous nodes, so take the next sibling of any kind.
	if v := file.FlatNextSib(next); v != 0 && file.FlatType(v) != "," && file.FlatType(v) != ")" {
		return v
	}
	return 0
}
