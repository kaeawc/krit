package rules

import "github.com/kaeawc/krit/internal/scanner"

func cursorBindingNames(file *scanner.File, decl uint32, names map[string]bool) {
	switch file.FlatType(decl) {
	case "variable_declarator", "variable_declaration", "parameter", "class_parameter",
		"formal_parameter", "spread_parameter", "catch_formal_parameter", "resource",
		"enhanced_for_statement", "inferred_parameters", "lambda_expression", "catch_block",
		"parameter_with_optional_type", "instanceof_expression", "type_pattern", "record_pattern_component",
		"function_declaration", "class_declaration", "object_declaration":
		for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
			if kind := file.FlatType(child); kind == "identifier" || kind == "simple_identifier" {
				names[file.FlatNodeText(child)] = true
			}
		}
	}
}

func cursorReceiverInvariant(file *scanner.File, receiver, call, body uint32, locals map[string]bool) bool {
	if receiver == 0 || !cursorDirectLoopCall(file, call, body) {
		return false
	}
	if cursorLookupUsesLocal(file, receiver, locals) {
		return false
	}
	if kind := file.FlatType(receiver); kind == "simple_identifier" || kind == "identifier" {
		return cursorNameBoundOutside(file, call, body, file.FlatNodeText(receiver))
	}
	return false
}

func cursorDirectLoopCall(file *scanner.File, call, body uint32) bool {
	for scope, ok := file.FlatParent(call); ok; scope, ok = file.FlatParent(scope) {
		if scope == body {
			return true
		}
		if cursorExecutionBoundary(file.FlatType(scope)) {
			return false
		}
	}
	return false
}

func cursorExecutionBoundary(kind string) bool {
	switch kind {
	case "function_declaration", "method_declaration", "constructor_declaration", "secondary_constructor",
		"lambda_literal", "lambda_expression", "anonymous_function", "object_literal", "class_body",
		"class_declaration", "object_declaration", "getter", "setter":
		return true
	}
	return false
}

func cursorNameBoundOutside(file *scanner.File, call, body uint32, name string) bool {
	return cursorBindingInvariant(file, call, body, name, false)
}

func cursorBindingInvariant(file *scanner.File, call, body uint32, name string, column bool) bool {
	at := file.FlatStartByte(body)
	for scope, ok := file.FlatParent(call); ok; scope, ok = file.FlatParent(scope) {
		if file.FlatStartByte(scope) >= at {
			continue
		}
		names := map[string]bool{}
		if file.FlatType(scope) != "function_declaration" {
			cursorBindingNames(file, scope, names)
		}
		if names[name] {
			return false
		}
		bound := false
		for child := file.FlatFirstChild(scope); child != 0; child = file.FlatNextSib(child) {
			proven, blocked := cursorChildBinding(file, scope, child, at, name, column)
			if blocked {
				return false
			}
			bound = bound || proven
		}
		if file.FlatType(scope) == "lambda_literal" && name == "it" {
			if _, explicit := file.FlatFindChild(scope, "lambda_parameters"); !explicit && !file.FlatHasChildOfType(scope, "->") {
				return false
			}
		}
		if bound {
			return !cursorNameWritten(file, scope, name)
		}
		if cursorExecutionBoundary(file.FlatType(scope)) {
			return false
		}
	}
	return false
}

func cursorChildBinding(file *scanner.File, scope, child, at uint32, name string, column bool) (bool, bool) {
	names := map[string]bool{}
	switch file.FlatType(child) {
	case "function_value_parameters", "formal_parameters":
		cursorDeclarationNames(file, child, names)
		kind := file.FlatType(scope)
		accepted := kind == "function_declaration" || kind == "method_declaration" || kind == "anonymous_function"
		return names[name] && accepted, names[name] && !accepted
	case "inferred_parameters", "lambda_parameters", "resource_specification", "variable_declaration",
		"multi_variable_declaration", "parameter_with_optional_type":
		cursorDeclarationNames(file, child, names)
		if file.FlatType(child) == "lambda_parameters" && !column && names[name] {
			accepted := cursorDirectEnclosingLambda(file, scope, at)
			return accepted, !accepted
		}
	case "condition", "parenthesized_expression", "switch_label", "catch_formal_parameter", "when_subject":
		file.FlatWalkAllNodes(child, func(node uint32) { cursorBindingNames(file, node, names) })
	case "property_declaration", "local_variable_declaration", "field_declaration":
		return cursorLocalBinding(file, scope, child, at, name, column)
	}
	return false, names[name]
}

func cursorLocalBinding(file *scanner.File, scope, child, at uint32, name string, column bool) (bool, bool) {
	names := map[string]bool{}
	cursorDeclarationNames(file, child, names)
	kind := file.FlatType(scope)
	local := kind == "statements" || kind == "block"
	if !names[name] || (local && file.FlatStartByte(child) >= at) {
		return false, false
	}
	accepted := local && file.FlatEndByte(child) <= at &&
		!file.FlatHasChildOfType(child, "property_delegate") && (!column || !declarationIsVarFlat(file, child))
	return accepted, !accepted
}

func cursorDeclarationNames(file *scanner.File, node uint32, names map[string]bool) {
	if file.FlatType(node) == "class_parameter" &&
		!file.FlatHasChildOfType(node, "binding_pattern_kind") &&
		!file.FlatHasChildOfType(node, "val") && !file.FlatHasChildOfType(node, "var") {
		return
	}
	cursorBindingNames(file, node, names)
	for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "variable_declarator", "variable_declaration", "multi_variable_declaration",
			"parameter", "class_parameter", "formal_parameter", "spread_parameter", "inferred_parameters",
			"class_parameters", "function_value_parameters", "formal_parameters", "lambda_parameters",
			"resource", "resource_specification", "type_pattern", "record_pattern_component", "parameter_with_optional_type":
			cursorDeclarationNames(file, child, names)
		}
	}
}

func cursorNameWritten(file *scanner.File, scope uint32, name string) bool {
	found := false
	file.FlatWalkAllNodes(scope, func(node uint32) {
		switch file.FlatType(node) {
		case "assignment", "assignment_expression", "update_expression":
			found = found || cursorLookupUsesLocal(file, file.FlatNamedChild(node, 0), map[string]bool{name: true})
		}
	})
	return found || lintNameReassigned(file, scope, name)
}
