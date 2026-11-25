package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// Annotation-driven evidence for the Compose runtime Lint ports (RememberInComposition, FrequentlyChangingValue,
// StateFactoryMarker). Lint reads the annotations of the resolved declaration; here a declaration is visible when
// it is in the same file, and library APIs are listed from the annotations in the 1.12.1 (material3 1.5.0) jars.

const acrAnnotationPackage = acrRuntimePackage + ".annotation"

// acrNotRemembered mirrors isNotRemembered(): node runs in composition and its scope is not the calculation lambda
// of androidx.compose.runtime.remember.
func acrNotRemembered(file *scanner.File, node uint32) bool {
	scope := acrCompositionScopeOf(file, node)
	if scope.Node != 0 && file.FlatType(scope.Node) == "lambda_literal" {
		if arg, ok := acrLambdaArgumentOf(file, scope.Node); ok {
			name, _, hasReceiver := acrCallCallee(file, arg.Call)
			if name == "remember" && !hasReceiver && acrCalleeResolvesTo(file, arg.Call, acrRuntimePackage, name) {
				return false
			}
		}
	}
	return scope.Composable
}

// acrAnnotationMatches reports whether an annotation node is pkg.name, optionally with the given use-site target
// ("" accepts an annotation without one).
func acrAnnotationMatches(file *scanner.File, annotation uint32, pkg, name, useSite string) bool {
	target := ""
	var typeNode uint32
	for child := file.FlatFirstChild(annotation); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "use_site_target":
			target = strings.TrimSuffix(strings.TrimSpace(file.FlatNodeText(child)), ":")
		case "user_type":
			typeNode = child
		case "constructor_invocation":
			typeNode, _ = file.FlatFindChild(child, "user_type")
		}
	}
	return target == useSite && typeNode != 0 && acrTypeNameIs(file, typeNode, pkg, name)
}

// acrHasAnnotation reports whether node's modifiers carry pkg.name with the use-site target.
func acrHasAnnotation(file *scanner.File, node uint32, pkg, name, useSite string) bool {
	mods, ok := file.FlatFindChild(node, "modifiers")
	if !ok {
		return false
	}
	for m := file.FlatFirstChild(mods); m != 0; m = file.FlatNextSib(m) {
		if file.FlatType(m) == "annotation" && acrAnnotationMatches(file, m, pkg, name, useSite) {
			return true
		}
	}
	return false
}

// acrPropertyGetterAnnotated reports whether a property's getter carries the annotation: `@get:Name val x` or
// `val x get() = ...` with `@Name` on the accessor.
func acrPropertyGetterAnnotated(file *scanner.File, prop uint32, pkg, name string) bool {
	if file.FlatType(prop) != "property_declaration" {
		return false
	}
	if acrHasAnnotation(file, prop, pkg, name, "get") {
		return true
	}
	if getter, ok := file.FlatFindChild(prop, "getter"); ok && acrHasAnnotation(file, getter, pkg, name, "") {
		return true
	}
	return false
}

// acrClassDeclaresAnnotatedMember reports whether a same-file class, object or interface declares a function (or,
// with property, a property getter) named name carrying the annotation, directly or on the member it overrides in
// a same-file supertype.
func acrClassDeclaresAnnotatedMember(file *scanner.File, class uint32, name string, property bool, pkg, ann string,
	depth int) bool {
	if class == 0 || depth > 4 {
		return false
	}
	member := acrClassMemberDeclaration(file, class, name, property)
	if member != 0 {
		if property && acrPropertyGetterAnnotated(file, member, pkg, ann) ||
			!property && acrHasAnnotation(file, member, pkg, ann, "") {
			return true
		}
		if !acrFunctionHasModifier(file, member, "override") {
			return false
		}
	}
	for child := file.FlatFirstChild(class); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "delegation_specifier" {
			continue
		}
		if super := acrSameFileClass(file, acrDelegationSpecifierType(file, child)); super != 0 && super != class &&
			acrClassDeclaresAnnotatedMember(file, super, name, property, pkg, ann, depth+1) {
			return true
		}
	}
	return false
}

// acrClassMemberDeclaration returns a class body's function or property named name.
func acrClassMemberDeclaration(file *scanner.File, class uint32, name string, property bool) uint32 {
	body, ok := file.FlatFindChild(class, "class_body")
	if !ok {
		return 0
	}
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "function_declaration":
			if !property && acrDeclarationName(file, child) == name {
				return child
			}
		case "property_declaration":
			if property && acrPropertyName(file, child) == name {
				return child
			}
		}
	}
	return 0
}

// acrReceiverClass resolves a navigation receiver to the same-file class its value has: a value declared with a
// same-file class type, a constructor call of a same-file class, `this`, or the name of a same-file object.
func acrReceiverClass(file *scanner.File, recv uint32) uint32 {
	recv = acrUnwrapParens(file, recv)
	switch file.FlatType(recv) {
	case "this_expression":
		return flatEnclosingClassLike(file, recv)
	case "simple_identifier":
		name := file.FlatNodeText(recv)
		if decl := acrValueDeclaration(file, recv, name); decl != 0 {
			if t := acrDeclaredType(file, decl); t != 0 {
				return acrSameFileClass(file, t)
			}
			return acrReceiverClass(file, acrInitializer(file, decl))
		}
		if obj := flatFindSameFileClassLikeDeclaration(file, name); obj != 0 && file.FlatType(obj) == "object_declaration" {
			return obj
		}
	case "call_expression":
		name, _, hasReceiver := acrCallCallee(file, recv)
		if !hasReceiver {
			if class := flatFindSameFileClassLikeDeclaration(file, name); class != 0 && file.FlatType(class) == "class_declaration" {
				return class
			}
		}
	case "navigation_expression":
		if decl := acrExprDeclaration(file, recv); decl != 0 {
			return acrSameFileClass(file, acrDeclaredType(file, decl))
		}
	}
	return 0
}

// acrSameFileCallIsAnnotated reports whether a call resolves to a same-file function or constructor carrying the
// annotation (or overriding a same-file member that does).
func acrSameFileCallIsAnnotated(file *scanner.File, call uint32, pkg, ann string) bool {
	name, _, hasReceiver := acrCallCallee(file, call)
	if name == "" {
		return false
	}
	if hasReceiver {
		recv := file.FlatFirstChild(acrCalleeExpression(file, call))
		return acrClassDeclaresAnnotatedMember(file, acrReceiverClass(file, recv), name, false, pkg, ann, 0)
	}
	if class := flatFindSameFileClassLikeDeclaration(file, name); class != 0 && file.FlatType(class) == "class_declaration" {
		return acrClassConstructorAnnotated(file, class, pkg, ann)
	}
	if fn := acrSameFileFunction(file, name); fn != 0 {
		return acrHasAnnotation(file, fn, pkg, ann, "")
	}
	return false
}

// acrClassConstructorAnnotated reports whether a class's primary or a secondary constructor carries the annotation.
func acrClassConstructorAnnotated(file *scanner.File, class uint32, pkg, ann string) bool {
	if ctor, ok := file.FlatFindChild(class, "primary_constructor"); ok && acrHasAnnotation(file, ctor, pkg, ann, "") {
		return true
	}
	if body, ok := file.FlatFindChild(class, "class_body"); ok {
		for _, ctor := range flatDirectChildrenOfType(file, body, "secondary_constructor") {
			if acrHasAnnotation(file, ctor, pkg, ann, "") {
				return true
			}
		}
	}
	return false
}

// acrSameFilePropertyReadIsAnnotated reports whether a property read (`name` or `receiver.name`) resolves to a
// same-file property whose getter carries the annotation.
func acrSameFilePropertyReadIsAnnotated(file *scanner.File, expr uint32, pkg, ann string) bool {
	switch file.FlatType(expr) {
	case "simple_identifier":
		decl := acrValueDeclaration(file, expr, file.FlatNodeText(expr))
		return decl != 0 && acrPropertyGetterAnnotated(file, decl, pkg, ann)
	case "navigation_expression":
		suffix := flatLastNamedChild(file, expr)
		if suffix == 0 || file.FlatType(suffix) != "navigation_suffix" {
			return false
		}
		member := file.FlatChildTextOrEmpty(suffix, "simple_identifier")
		return member != "" &&
			acrClassDeclaresAnnotatedMember(file, acrReceiverClass(file, file.FlatFirstChild(expr)), member, true, pkg, ann, 0)
	}
	return false
}
