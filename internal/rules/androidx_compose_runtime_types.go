package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	"github.com/kaeawc/krit/internal/scanner"
)

// Source-visible type evidence for the Compose runtime Lint ports. Lint asks the compiler for an expression's
// type; these helpers only answer when a same-file declaration (its declared type, or an initializer whose type
// is known) proves it, and report "unknown" otherwise.

const acrFlowPackage = "kotlinx.coroutines.flow"

// acrValueDeclaration finds the same-file declaration a simple name refers to at ref: a local property declared
// before ref in an enclosing block, a parameter of an enclosing function, a property or constructor property of an
// enclosing class, or a top-level property. It returns 0 when a lambda parameter, destructuring or anything else
// it cannot see might bind the name.
func acrValueDeclaration(file *scanner.File, ref uint32, name string) uint32 {
	for p, ok := file.FlatParent(ref); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "statements":
			var found uint32
			for _, child := range acrBlockProperties(file, p)[name] {
				if file.FlatStartByte(child) >= file.FlatStartByte(ref) {
					break
				}
				// A declaration does not see itself: `val items = items.map { }` refers to an outer `items`.
				if !flatNodeContains(file, child, ref) {
					found = child
				}
			}
			if found != 0 {
				return found
			}
		case "lambda_literal":
			if acrLambdaDeclaresParameter(file, p, name) {
				return 0
			}
		case "function_declaration", "anonymous_function":
			for _, param := range acrFunctionParameters(file, p) {
				if file.FlatChildTextOrEmpty(param, "simple_identifier") == name {
					return param
				}
			}
		case "class_body":
			for child := file.FlatFirstChild(p); child != 0; child = file.FlatNextSib(child) {
				if file.FlatType(child) == "property_declaration" && acrPropertyName(file, child) == name &&
					!flatNodeContains(file, child, ref) {
					return child
				}
			}
		case "class_declaration":
			for _, cp := range acrClassParameters(file, p) {
				if file.FlatChildTextOrEmpty(cp, "simple_identifier") == name {
					return cp
				}
			}
		case "source_file":
			for child := file.FlatFirstChild(p); child != 0; child = file.FlatNextSib(child) {
				if file.FlatType(child) == "property_declaration" && acrPropertyName(file, child) == name &&
					!flatNodeContains(file, child, ref) {
					return child
				}
			}
			return 0
		}
	}
	return 0
}

// acrBlockProperties indexes a statements block's property declarations by name, in source order, once per file,
// so resolving many references in a long block does not rescan it for each one.
func acrBlockProperties(file *scanner.File, block uint32) map[string][]uint32 {
	return filefacts.NodeFact(fileFactsCache(), file, block, slotAcrBlockProperties, func() map[string][]uint32 {
		out := map[string][]uint32{}
		for child := file.FlatFirstChild(block); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "property_declaration" {
				if name := acrPropertyName(file, child); name != "" {
					out[name] = append(out[name], child)
				}
			}
		}
		return out
	})
}

// acrPropertyName returns a property declaration's simple name ("" for destructuring).
func acrPropertyName(file *scanner.File, prop uint32) string {
	v, ok := file.FlatFindChild(prop, "variable_declaration")
	if !ok {
		return ""
	}
	return strings.Trim(file.FlatChildTextOrEmpty(v, "simple_identifier"), "`")
}

// acrLambdaDeclaresParameter reports whether a lambda declares the name as a parameter (or uses the implicit `it`).
func acrLambdaDeclaresParameter(file *scanner.File, lambda uint32, name string) bool {
	params, ok := file.FlatFindChild(lambda, "lambda_parameters")
	if !ok {
		return name == "it"
	}
	found := false
	file.FlatWalkAllNodes(params, func(n uint32) {
		if !found && file.FlatType(n) == "simple_identifier" && file.FlatNodeTextEquals(n, name) {
			found = true
		}
	})
	return found
}

// acrDeclaredType returns the declared type node of a property, parameter or class parameter, or 0.
func acrDeclaredType(file *scanner.File, decl uint32) uint32 {
	holder := decl
	if file.FlatType(decl) == "property_declaration" {
		v, ok := file.FlatFindChild(decl, "variable_declaration")
		if !ok {
			return 0
		}
		holder = v
	}
	for child := file.FlatFirstChild(holder); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "user_type", "nullable_type", "function_type", "parenthesized_type":
			return child
		}
	}
	return 0
}

// acrInitializer returns the initializer expression of a property declaration (not a delegate), or 0.
func acrInitializer(file *scanner.File, prop uint32) uint32 {
	if file.FlatType(prop) != "property_declaration" {
		return 0
	}
	sawEquals := false
	for child := file.FlatFirstChild(prop); child != 0; child = file.FlatNextSib(child) {
		switch t := file.FlatType(child); {
		case t == "=":
			sawEquals = true
		case t == "property_delegate" || t == "getter" || t == "setter":
			return 0
		case sawEquals && file.FlatIsNamed(child):
			return child
		}
	}
	return 0
}

// acrUnwrapParens strips parentheses around an expression.
func acrUnwrapParens(file *scanner.File, expr uint32) uint32 {
	for expr != 0 && file.FlatType(expr) == "parenthesized_expression" {
		expr = flatFirstNamedChild(file, expr)
	}
	return expr
}

// acrTypeNodeName returns the base name of a type node without type arguments or nullability ("Flow" for
// `Flow<Int>?`, "kotlinx.coroutines.flow.Flow" when qualified).
func acrTypeNodeName(file *scanner.File, typeNode uint32) string {
	if typeNode == 0 {
		return ""
	}
	if file.FlatType(typeNode) == "nullable_type" {
		inner := flatFirstNamedChild(file, typeNode)
		if inner == 0 {
			return ""
		}
		typeNode = inner
	}
	if file.FlatType(typeNode) != "user_type" {
		return ""
	}
	text := file.FlatNodeText(typeNode)
	if lt := strings.IndexByte(text, '<'); lt >= 0 {
		text = text[:lt]
	}
	return strings.Join(strings.Fields(text), "")
}

// acrTypeNameIs reports whether a type node names one of pkg's classes, qualified or through the file's imports.
func acrTypeNameIs(file *scanner.File, typeNode uint32, pkg string, names ...string) bool {
	text := acrTypeNodeName(file, typeNode)
	if text == "" {
		return false
	}
	for _, name := range names {
		if text == pkg+"."+name || acrSimpleNameRefersTo(file, text, pkg, name) {
			return true
		}
	}
	return false
}

// acrSameFileClass returns the class or object declaration named by a type node when the file declares it.
func acrSameFileClass(file *scanner.File, typeNode uint32) uint32 {
	text := acrTypeNodeName(file, typeNode)
	if text == "" || strings.Contains(text, ".") {
		return 0
	}
	return flatFindSameFileClassLikeDeclaration(file, text)
}

// acrClassSupertypeIs reports whether a same-file class or object lists one of pkg's classes as a direct
// supertype, or extends a same-file class that does (depth-limited).
func acrClassSupertypeIs(file *scanner.File, class uint32, depth int, pkg string, names ...string) bool {
	if class == 0 || depth > 4 {
		return false
	}
	for child := file.FlatFirstChild(class); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "delegation_specifier" {
			continue
		}
		typeNode := acrDelegationSpecifierType(file, child)
		if acrTypeNameIs(file, typeNode, pkg, names...) {
			return true
		}
		if super := acrSameFileClass(file, typeNode); super != 0 && super != class &&
			acrClassSupertypeIs(file, super, depth+1, pkg, names...) {
			return true
		}
	}
	return false
}

// acrDelegationSpecifierType returns the user_type of a supertype entry (`Foo`, `Foo()`, `Foo by x`).
func acrDelegationSpecifierType(file *scanner.File, spec uint32) uint32 {
	var found uint32
	file.FlatWalkAllNodes(spec, func(n uint32) {
		if found == 0 && file.FlatType(n) == "user_type" {
			found = n
		}
	})
	return found
}

// acrTypeIs reports whether a type node is one of pkg's named classes or a same-file class extending one.
func acrTypeIs(file *scanner.File, typeNode uint32, pkg string, names ...string) bool {
	if acrTypeNameIs(file, typeNode, pkg, names...) {
		return true
	}
	return acrClassSupertypeIs(file, acrSameFileClass(file, typeNode), 0, pkg, names...)
}

// acrExprDeclaration resolves an identifier, `this.name` or `receiver.name` expression to its same-file
// declaration. For `receiver.name` the receiver must resolve to a declaration whose type is a same-file class
// declaring the member.
func acrExprDeclaration(file *scanner.File, expr uint32) uint32 {
	expr = acrUnwrapParens(file, expr)
	switch file.FlatType(expr) {
	case "simple_identifier":
		return acrValueDeclaration(file, expr, file.FlatNodeText(expr))
	case "navigation_expression":
		recv := acrUnwrapParens(file, file.FlatFirstChild(expr))
		suffix := flatLastNamedChild(file, expr)
		if suffix == 0 || file.FlatType(suffix) != "navigation_suffix" {
			return 0
		}
		member := file.FlatChildTextOrEmpty(suffix, "simple_identifier")
		if member == "" {
			return 0
		}
		var class uint32
		if file.FlatType(recv) == "this_expression" {
			class = flatEnclosingClassLike(file, expr)
		} else if decl := acrExprDeclaration(file, recv); decl != 0 {
			class = acrSameFileClass(file, acrDeclaredType(file, decl))
		}
		if class == 0 {
			return 0
		}
		return acrClassMember(file, class, member)
	}
	return 0
}

// acrClassMember returns a property or constructor property declared directly by a same-file class.
func acrClassMember(file *scanner.File, class uint32, name string) uint32 {
	if body, ok := file.FlatFindChild(class, "class_body"); ok {
		for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "property_declaration" && acrPropertyName(file, child) == name {
				return child
			}
		}
	}
	for _, cp := range acrClassParameters(file, class) {
		if file.FlatChildTextOrEmpty(cp, "simple_identifier") == name && file.FlatHasChildOfType(cp, "binding_pattern_kind") {
			return cp
		}
	}
	return 0
}

// acrClassParameters returns a class's primary-constructor parameters. tree-sitter-kotlin puts them directly under
// primary_constructor (older grammars wrap them in class_parameters).
func acrClassParameters(file *scanner.File, class uint32) []uint32 {
	ctor, ok := file.FlatFindChild(class, "primary_constructor")
	if !ok {
		return nil
	}
	if params, ok := file.FlatFindChild(ctor, "class_parameters"); ok {
		ctor = params
	}
	return flatDirectChildrenOfType(file, ctor, "class_parameter")
}

// acrCallTargets reports whether call is a call of pkg.name: a simple callee resolving to it, or a fully
// qualified `pkg.name(...)`.
func acrCallTargets(file *scanner.File, call uint32, pkg, name string) bool {
	if file.FlatType(call) != "call_expression" {
		return false
	}
	callee := acrCalleeExpression(file, call)
	switch file.FlatType(callee) {
	case "simple_identifier":
		return file.FlatNodeTextEquals(callee, name) && acrCalleeResolvesTo(file, call, pkg, name)
	case "navigation_expression":
		return strings.Join(strings.Fields(file.FlatNodeText(callee)), "") == pkg+"."+name
	}
	return false
}

// acrAssignableReceiverDeclaration resolves the receiver of the final suffix of a directly_assignable_expression
// (`a.b.value`, flattened as a, .b, .value) to its same-file declaration, following members of same-file classes.
func acrAssignableReceiverDeclaration(file *scanner.File, node, last uint32) uint32 {
	first := file.FlatFirstChild(node)
	if first == 0 || file.FlatType(first) != "simple_identifier" {
		return 0
	}
	decl := acrValueDeclaration(file, first, file.FlatNodeText(first))
	for child := file.FlatNextSib(first); child != 0 && child != last; child = file.FlatNextSib(child) {
		if decl == 0 || file.FlatType(child) != "navigation_suffix" {
			return 0
		}
		class := acrSameFileClass(file, acrDeclaredType(file, decl))
		if class == 0 {
			return 0
		}
		decl = acrClassMember(file, class, file.FlatChildTextOrEmpty(child, "simple_identifier"))
	}
	return decl
}

// acrScopeLambdaReceivers are the stdlib scope functions whose lambda parameter is the call's receiver.
var acrScopeLambdaReceivers = map[string]bool{"let": true, "also": true, "takeIf": true, "takeUnless": true}

// acrScopeLambdaReceiver returns the receiver expression bound to the name when it is the (implicit or single)
// parameter of a `receiver.let { }`-style trailing lambda, or 0.
func acrScopeLambdaReceiver(file *scanner.File, ident uint32) uint32 {
	name := file.FlatNodeText(ident)
	for p, ok := file.FlatParent(ident); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "lambda_literal":
			if !acrLambdaDeclaresParameter(file, p, name) {
				continue
			}
			if params, ok := file.FlatFindChild(p, "lambda_parameters"); ok && file.FlatNamedChildCount(params) != 1 {
				return 0
			}
			arg, ok := acrLambdaArgumentOf(file, p)
			if !ok || !arg.Trailing {
				return 0
			}
			callee, _, hasReceiver := acrCallCallee(file, arg.Call)
			if !hasReceiver || !acrScopeLambdaReceivers[callee] || acrFileDeclaresAnyFunction(file, callee) {
				return 0
			}
			nav := acrCalleeExpression(file, arg.Call)
			if flatNavigationHasSafeCall(file, nav) {
				return 0
			}
			return file.FlatFirstChild(nav)
		case "function_declaration", "class_body", "source_file":
			return 0
		}
	}
	return 0
}
