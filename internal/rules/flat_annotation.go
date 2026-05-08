package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// hasAnnotationNamed returns true when the declaration at idx has a
// modifier-list annotation whose final name is exactly `name`. Checks
// both the declaration's `modifiers` child and its immediately preceding
// sibling (some grammar versions emit modifiers as a sibling node).
func hasAnnotationNamed(file *scanner.File, idx uint32, name string) bool {
	if file == nil || idx == 0 {
		return false
	}
	check := func(container uint32) bool {
		if container == 0 {
			return false
		}
		found := false
		file.FlatWalkNodes(container, "annotation", func(ann uint32) {
			if found {
				return
			}
			ctor, _ := file.FlatFindChild(ann, "constructor_invocation")
			if ctor != 0 {
				if annotationConstructorName(file, ctor) == name {
					found = true
				}
				return
			}
			// Marker annotation (no constructor call): `@Foo`
			userType, _ := file.FlatFindChild(ann, "user_type")
			if userType != 0 {
				if ident := flatLastChildOfType(file, userType, "type_identifier"); ident != 0 {
					if file.FlatNodeText(ident) == name {
						found = true
					}
				}
			}
		})
		return found
	}
	if mods, ok := file.FlatFindChild(idx, "modifiers"); ok && check(mods) {
		return true
	}
	if prev, ok := file.FlatPrevSibling(idx); ok && check(prev) {
		return true
	}
	return false
}

// annotationFinalName returns the annotation's final simple name,
// whether the annotation has a constructor call (`@Foo(...)`) or is a
// marker (`@Foo`). Compared to annotationConstructorName, this
// handles both forms transparently.
func annotationFinalName(file *scanner.File, annotation uint32) string {
	if file == nil || annotation == 0 || file.FlatType(annotation) != "annotation" {
		return ""
	}
	if ctor, ok := file.FlatFindChild(annotation, "constructor_invocation"); ok {
		if name := annotationConstructorName(file, ctor); name != "" {
			return name
		}
	}
	userType, _ := file.FlatFindChild(annotation, "user_type")
	if userType == 0 {
		return ""
	}
	if ident := flatLastChildOfType(file, userType, "type_identifier"); ident != 0 {
		return file.FlatNodeText(ident)
	}
	return ""
}

func annotationQualifiedName(file *scanner.File, annotation uint32) string {
	if file == nil || annotation == 0 || file.FlatType(annotation) != "annotation" {
		return ""
	}
	userType := uint32(0)
	if ctor, ok := file.FlatFindChild(annotation, "constructor_invocation"); ok {
		userType, _ = file.FlatFindChild(ctor, "user_type")
	} else {
		userType, _ = file.FlatFindChild(annotation, "user_type")
	}
	if userType == 0 {
		return ""
	}
	return strings.TrimSpace(file.FlatNodeText(userType))
}

// annotationHasClassLiteralArgIn returns true when the annotation has
// an argument of the form `SomeName::class` whose final identifier is
// in `names`. Used by rules like ForbiddenOptIn to match on marker
// class references without string-scanning the annotation text.
func annotationHasClassLiteralArgIn(file *scanner.File, annotation uint32, names map[string]bool) bool {
	ctor, ok := file.FlatFindChild(annotation, "constructor_invocation")
	if !ok {
		return false
	}
	args, _ := file.FlatFindChild(ctor, "value_arguments")
	if args == 0 {
		return false
	}
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		expr := flatValueArgumentExpression(file, arg)
		// `Foo::class` is a class_literal with a child naming the type.
		if expr == 0 || file.FlatType(expr) != "class_literal" {
			continue
		}
		// Last simple_identifier of the class_literal is the class name.
		name := ""
		for c := file.FlatFirstChild(expr); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) == "simple_identifier" {
				name = file.FlatNodeText(c)
			}
			if file.FlatType(c) == "navigation_expression" {
				if n := flatNavigationExpressionLastIdentifier(file, c); n != "" {
					name = n
				}
			}
		}
		if name != "" && names[name] {
			return true
		}
	}
	return false
}

// annotationHasStringArgIn returns true when the annotation has a
// value_argument whose expression is a non-interpolated string_literal
// with content in `names`. Used by rules like ForbiddenSuppress to
// match on `@Suppress("RuleX")` arguments.
func annotationHasStringArgIn(file *scanner.File, annotation uint32, names map[string]bool) bool {
	ctor, ok := file.FlatFindChild(annotation, "constructor_invocation")
	if !ok {
		return false
	}
	args, _ := file.FlatFindChild(ctor, "value_arguments")
	if args == 0 {
		return false
	}
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		expr := flatValueArgumentExpression(file, arg)
		if expr == 0 || file.FlatType(expr) != "string_literal" {
			continue
		}
		if flatContainsStringInterpolation(file, expr) {
			continue
		}
		if names[stringLiteralContent(file, expr)] {
			return true
		}
	}
	return false
}

// annotationConstructorName returns the final identifier of an annotation's
// constructor_invocation. For `@foo.bar.IntDef(...)` this returns "IntDef".
func annotationConstructorName(file *scanner.File, ctor uint32) string {
	if file == nil || ctor == 0 {
		return ""
	}
	userType, _ := file.FlatFindChild(ctor, "user_type")
	if userType == 0 {
		return ""
	}
	ident := flatLastChildOfType(file, userType, "type_identifier")
	if ident == 0 {
		return ""
	}
	return file.FlatNodeText(ident)
}

// annotationConstantKey returns (canonicalKey, display) for a constant
// appearing in an annotation argument. Supports numeric literals (key is
// the literal text), string literals (key is the interpolation-free
// content prefixed with `"s:"`), and simple identifiers / qualified
// navigation expressions (key is the dotted FQN prefixed with `"id:"`).
// Returns ("", "") for expressions we don't recognize as a simple
// constant reference.
func annotationConstantKey(file *scanner.File, expr uint32) (key, display string) {
	if file == nil || expr == 0 {
		return "", ""
	}
	switch file.FlatType(expr) {
	case "integer_literal", "long_literal", "hex_literal", "bin_literal":
		t := file.FlatNodeText(expr)
		return "n:" + t, t
	case "string_literal":
		if flatContainsStringInterpolation(file, expr) {
			return "", ""
		}
		c := stringLiteralContent(file, expr)
		return "s:" + c, `"` + c + `"`
	case "simple_identifier", "navigation_expression":
		t := strings.TrimSpace(file.FlatNodeText(expr))
		return "id:" + t, t
	}
	return "", ""
}
