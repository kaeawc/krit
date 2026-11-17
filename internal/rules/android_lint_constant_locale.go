package rules

import (
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const javaUtilLocale = "java.util.Locale"

// ConstantLocaleRule mirrors Lint's ConstantLocale: Locale.getDefault() evaluated in the initializer of a final
// static field, which freezes the locale at class load.
type ConstantLocaleRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: the receiver class is matched by name and imports.
func (r *ConstantLocaleRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *ConstantLocaleRule) check(ctx *api.Context) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || call.Name != "getDefault" || len(call.Args) > 1 || call.ReceiverNode == 0 {
		return
	}
	if !lintTypeRefMatches(file, call.ReceiverText, javaUtilLocale) || !lintInConstantInitializer(file, ctx.Idx) {
		return
	}
	ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "",
		"Assigning `Locale.getDefault()` to a final static field is suspicious; this code will not work correctly if the user changes locale while the app is running"))
}

// lintInConstantInitializer reports whether idx is evaluated while a final static field is initialized. Lambdas,
// anonymous class bodies, accessors, delegates and any function body run later, so they end the search.
func lintInConstantInitializer(file *scanner.File, idx uint32) bool {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "lambda_literal", "anonymous_function", "lambda_expression", "function_declaration", "method_declaration",
			"constructor_declaration", "secondary_constructor", "property_delegate", "getter", "setter", "block",
			"anonymous_initializer", "static_initializer", "class_body", "object_literal", "object_creation_expression":
			return false
		case "property_declaration":
			return lintKotlinConstantProperty(file, cur)
		case "variable_declarator":
			decl, ok := file.FlatParent(cur)
			return ok && lintJavaConstantField(file, decl)
		}
	}
	return false
}

// lintKotlinConstantProperty reports whether a Kotlin property is a `val` stored in a static position: top level, or
// inside an object or companion object.
func lintKotlinConstantProperty(file *scanner.File, prop uint32) bool {
	kind, ok := file.FlatFindChild(prop, "binding_pattern_kind")
	if !ok || file.FlatNodeText(kind) != "val" {
		return false
	}
	parent, ok := file.FlatParent(prop)
	if !ok {
		return false
	}
	switch file.FlatType(parent) {
	case "source_file":
		return true
	case "class_body":
		owner, ok := file.FlatParent(parent)
		if !ok {
			return false
		}
		switch file.FlatType(owner) {
		case "object_declaration", "companion_object":
			return true
		}
	}
	return false
}

// lintJavaConstantField reports whether a Java field declaration is final and static, which an interface constant
// is implicitly.
func lintJavaConstantField(file *scanner.File, decl uint32) bool {
	switch file.FlatType(decl) {
	case "constant_declaration":
		return true
	case "field_declaration":
		return file.FlatHasModifier(decl, "static") && file.FlatHasModifier(decl, "final")
	}
	return false
}
