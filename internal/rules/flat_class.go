package rules

import (
	"github.com/kaeawc/krit/internal/scanner"
)

// flatFunctionParameterNames returns the simple_identifier names of every
// parameter in a function_declaration's function_value_parameters block.
func flatFunctionParameterNames(file *scanner.File, funcDecl uint32) []string {
	if file == nil || file.FlatType(funcDecl) != "function_declaration" {
		return nil
	}
	params, _ := file.FlatFindChild(funcDecl, "function_value_parameters")
	if params == 0 {
		return nil
	}
	var names []string
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "parameter" {
			continue
		}
		if ident, ok := file.FlatFindChild(child, "simple_identifier"); ok {
			names = append(names, file.FlatNodeString(ident, nil))
		}
	}
	return names
}

// classOverriddenFunctions returns the set of function names declared
// with the `override` modifier at class-top-level. Used by rules that
// need to answer "did this subclass override method X?" without scanning
// source text for `override fun X(`.
func classOverriddenFunctions(file *scanner.File, classIdx uint32) map[string]bool {
	out := map[string]bool{}
	if file == nil || classIdx == 0 {
		return out
	}
	body, _ := file.FlatFindChild(classIdx, "class_body")
	if body == 0 {
		return out
	}
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "function_declaration" {
			continue
		}
		if !file.FlatHasModifier(child, "override") {
			continue
		}
		ident, _ := file.FlatFindChild(child, "simple_identifier")
		if ident == 0 {
			continue
		}
		out[file.FlatNodeText(ident)] = true
	}
	return out
}

// classHasSupertypeNamed returns true when the class_declaration at idx
// lists a supertype whose final type_identifier equals `name`. Covers
// both interface form (`: Foo`, delegation_specifier→user_type) and
// class form with a constructor call (`: Foo()`,
// delegation_specifier→constructor_invocation→user_type). Works for
// qualified receivers like `: pkg.sub.Foo[()]`.
func classHasSupertypeNamed(file *scanner.File, idx uint32, name string) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "class_declaration" {
		return false
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "delegation_specifier" {
			continue
		}
		userType, _ := file.FlatFindChild(child, "user_type")
		if userType == 0 {
			// `: Foo(args)` form — user_type lives under constructor_invocation.
			if ctor, ok := file.FlatFindChild(child, "constructor_invocation"); ok {
				userType, _ = file.FlatFindChild(ctor, "user_type")
			}
		}
		if userType == 0 {
			continue
		}
		if ident := flatLastChildOfType(file, userType, "type_identifier"); ident != 0 {
			if file.FlatNodeText(ident) == name {
				return true
			}
		}
	}
	return false
}

// classBodyOf returns the class_body (or enum_class_body) of the class at idx, or 0.
func classBodyOf(file *scanner.File, idx uint32) uint32 {
	if body, ok := file.FlatFindChild(idx, "class_body"); ok {
		return body
	}
	body, _ := file.FlatFindChild(idx, "enum_class_body")
	return body
}

// classBodyProperty returns the property_declaration named `name` directly in body, or 0.
func classBodyProperty(file *scanner.File, body uint32, name string) uint32 {
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "property_declaration" && propertyDeclarationName(file, child) == name {
			return child
		}
	}
	return 0
}

// propertyDeclarationName returns the identifier name of a property_declaration,
// or "" if the node isn't a property_declaration or has no variable_declaration.
func propertyDeclarationName(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 || file.FlatType(idx) != "property_declaration" {
		return ""
	}
	varDecl, _ := file.FlatFindChild(idx, "variable_declaration")
	if varDecl == 0 {
		return ""
	}
	ident, _ := file.FlatFindChild(varDecl, "simple_identifier")
	if ident == 0 {
		return ""
	}
	return file.FlatNodeText(ident)
}

// propertyInitializerExpression returns the initializer expression node
// of a property_declaration — the first named child after the `=`
// token — or 0 when the property has no initializer.
func propertyInitializerExpression(file *scanner.File, idx uint32) uint32 {
	if file == nil || idx == 0 || file.FlatType(idx) != "property_declaration" {
		return 0
	}
	seenEquals := false
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "=" {
			seenEquals = true
			continue
		}
		if seenEquals && file.FlatIsNamed(child) {
			return child
		}
	}
	return 0
}

// propertyDeclarationIsVar returns true when the property_declaration's
// binding_pattern_kind child carries the `var` keyword. `val` returns
// false. Used instead of `strings.Contains(propText, "var ")`, which
// false-positives on property types or initializers that contain the
// substring "var " (e.g. `val x = "the var keyword"` or
// `val foo: MutableList<Bar> = ...` containing "var" in a word).
func propertyDeclarationIsVar(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "property_declaration" {
		return false
	}
	bpk, _ := file.FlatFindChild(idx, "binding_pattern_kind")
	if bpk == 0 {
		return false
	}
	for c := file.FlatFirstChild(bpk); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "var" {
			return true
		}
	}
	return false
}

// propertyInitializerCallCalleeName returns the callee name of the
// property's initializer expression when the initializer is a
// call_expression, otherwise "". For `val foo = Channel<Int>()` this
// returns "Channel"; for `val foo = listOf(1,2,3)` it returns "listOf".
func propertyInitializerCallCalleeName(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 || file.FlatType(idx) != "property_declaration" {
		return ""
	}
	seenEquals := false
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "=" {
			seenEquals = true
			continue
		}
		if !seenEquals || !file.FlatIsNamed(child) {
			continue
		}
		if file.FlatType(child) == "call_expression" {
			return flatCallExpressionName(file, child)
		}
		return ""
	}
	return ""
}

// initializerAssignedName returns the local property name whose initializer
// contains idx. It covers Kotlin `val name = expr` and `var name = expr`.
func initializerAssignedName(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	for current, ok := file.FlatParent(idx); ok; current, ok = file.FlatParent(current) {
		switch file.FlatType(current) {
		case "property_declaration":
			init := propertyInitializerExpression(file, current)
			if init == 0 {
				return ""
			}
			for n := idx; n != 0; {
				if n == init {
					return propertyDeclarationName(file, current)
				}
				parent, ok := file.FlatParent(n)
				if !ok || parent == current {
					break
				}
				n = parent
			}
			return ""
		case "function_declaration", "class_declaration", "object_declaration", "source_file":
			return ""
		}
	}
	return ""
}

// assignmentRHS returns the expression on the right of `=` in an assignment.
func assignmentRHS(file *scanner.File, idx uint32) uint32 {
	if file == nil || idx == 0 || file.FlatType(idx) != "assignment" {
		return 0
	}
	seenEquals := false
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "=" {
			seenEquals = true
			continue
		}
		if seenEquals && file.FlatIsNamed(child) {
			return child
		}
	}
	return 0
}

// parcelCreatorMember finds how the class at idx declares CREATOR: a property in the class body or in
// any companion (prop != 0), or a companion object named CREATOR (found with prop == 0).
func parcelCreatorMember(file *scanner.File, idx uint32) (prop uint32, found bool) {
	body := classBodyOf(file, idx)
	if body == 0 {
		return 0, false
	}
	if p := classBodyProperty(file, body, "CREATOR"); p != 0 {
		return p, true
	}
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "companion_object" {
			continue
		}
		if name, ok := file.FlatFindChild(child, "type_identifier"); ok && file.FlatNodeText(name) == "CREATOR" {
			return 0, true
		}
		if inner, ok := file.FlatFindChild(child, "class_body"); ok {
			if p := classBodyProperty(file, inner, "CREATOR"); p != 0 {
				return p, true
			}
		}
	}
	return 0, false
}
