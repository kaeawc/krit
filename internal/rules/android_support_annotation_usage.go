package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// supportAnnotationIntegral lists annotations that only apply to integral values, as AOSP
// SupportAnnotationUsage checks (resource-id annotations, @ColorInt, @Px, @IntRange).
var supportAnnotationIntegral = map[string]bool{
	"IntRange": true, "ColorInt": true, "Px": true, "Dimension": true,
	"AnimRes": true, "AnimatorRes": true, "AnyRes": true, "ArrayRes": true, "AttrRes": true,
	"BoolRes": true, "ColorRes": true, "DimenRes": true, "DrawableRes": true, "FontRes": true,
	"FractionRes": true, "IdRes": true, "IntegerRes": true, "InterpolatorRes": true,
	"LayoutRes": true, "MenuRes": true, "NavigationRes": true, "PluralsRes": true,
	"RawRes": true, "StringRes": true, "StyleRes": true, "StyleableRes": true,
	"TransitionRes": true, "XmlRes": true,
}

// supportAnnotationNotIntegral are type names that can never hold an integral value.
var supportAnnotationNotIntegral = map[string]bool{
	"String": true, "CharSequence": true, "Boolean": true, "Float": true, "Double": true,
	"Char": true, "Unit": true,
}

// supportAnnotationNotFloating are type names that can never hold a floating-point value.
var supportAnnotationNotFloating = map[string]bool{
	"String": true, "CharSequence": true, "Boolean": true, "Int": true, "Long": true,
	"Short": true, "Byte": true, "Char": true, "Unit": true,
	"Integer": true,
}

// checkSupportAnnotationUsage reports androidx range and resource annotations applied to a
// declaration whose explicit type can never satisfy them.
func checkSupportAnnotationUsage(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	var annotations []uint32
	var typeNode uint32
	switch file.FlatType(idx) {
	case "function_declaration":
		annotations = annotationsIn(file, idx, "modifiers")
		seenParams := false
		for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
			switch file.FlatType(c) {
			case "function_value_parameters":
				seenParams = true
			case "function_body":
				seenParams = false
			case "user_type", "nullable_type":
				if seenParams && typeNode == 0 {
					typeNode = c
				}
			}
		}
	case "property_declaration":
		annotations = annotationsIn(file, idx, "modifiers")
		if decl, ok := file.FlatFindChild(idx, "variable_declaration"); ok {
			typeNode = firstTypeChild(file, decl)
		}
	case "class_parameter":
		annotations = annotationsIn(file, idx, "modifiers")
		typeNode = firstTypeChild(file, idx)
	case "parameter":
		if prev, ok := file.FlatPrevSibling(idx); ok && file.FlatType(prev) == "parameter_modifiers" {
			annotations = annotationsIn(file, prev, "")
		}
		typeNode = firstTypeChild(file, idx)
	}
	if typeNode == 0 || len(annotations) == 0 {
		return
	}
	typeName := supportAnnotationBaseTypeName(file, typeNode)
	for _, ann := range annotations {
		name := annotationFinalName(file, ann)
		switch {
		case supportAnnotationIntegral[name] && supportAnnotationNotIntegral[typeName]:
			ctx.EmitAt(file.FlatRow(ann)+1, file.FlatCol(ann)+1,
				"@"+name+" does not apply to type "+typeName+"; expected Int or Long.")
		case name == "FloatRange" && supportAnnotationNotFloating[typeName]:
			ctx.EmitAt(file.FlatRow(ann)+1, file.FlatCol(ann)+1,
				"@FloatRange does not apply to type "+typeName+"; expected Float or Double.")
		}
	}
}

// annotationsIn returns the annotation nodes under the child of the given type, or under idx itself when childType is empty.
func annotationsIn(file *scanner.File, idx uint32, childType string) []uint32 {
	container := idx
	if childType != "" {
		var ok bool
		if container, ok = file.FlatFindChild(idx, childType); !ok {
			return nil
		}
	}
	var out []uint32
	for c := file.FlatFirstChild(container); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "annotation" {
			out = append(out, c)
		}
	}
	return out
}

func firstTypeChild(file *scanner.File, idx uint32) uint32 {
	for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
		if t := file.FlatType(c); t == "user_type" || t == "nullable_type" {
			return c
		}
	}
	return 0
}

// supportAnnotationBaseTypeName returns the simple type name without nullability or type arguments.
func supportAnnotationBaseTypeName(file *scanner.File, typeNode uint32) string {
	if file.FlatType(typeNode) == "nullable_type" {
		if inner := firstTypeChild(file, typeNode); inner != 0 {
			typeNode = inner
		}
	}
	text := strings.TrimSpace(file.FlatNodeText(typeNode))
	if lt := strings.Index(text, "<"); lt >= 0 {
		text = text[:lt]
	}
	if dot := strings.LastIndex(text, "."); dot >= 0 {
		text = text[dot+1:]
	}
	return strings.TrimSuffix(text, "?")
}
