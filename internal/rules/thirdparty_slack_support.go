package rules

// Small AST helpers shared by the slack-lints ports.

import "github.com/kaeawc/krit/internal/scanner"

// slackCalleeNameNode returns the method-name node of a Kotlin call_expression or Java method_invocation, where
// Lint's name location (and its call location without the receiver) starts.
func slackCalleeNameNode(file *scanner.File, call uint32) uint32 {
	if file.FlatType(call) == "method_invocation" {
		var name uint32
		for child := file.FlatFirstChild(call); child != 0 && file.FlatType(child) != "argument_list"; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "identifier" {
				name = child
			}
		}
		return name
	}
	callee := file.FlatFirstChild(call)
	switch file.FlatType(callee) {
	case "simple_identifier":
		return callee
	case "navigation_expression":
		var last uint32
		for child := file.FlatFirstChild(callee); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "navigation_suffix" {
				last = child
			}
		}
		if id, ok := file.FlatFindChild(last, "simple_identifier"); ok {
			return id
		}
	}
	return 0
}

// slackLastLambdaArgument returns the lambda_literal passed as a Kotlin call's last argument, trailing or in
// parentheses, or 0.
func slackLastLambdaArgument(file *scanner.File, call uint32) uint32 {
	suffix, ok := file.FlatFindChild(call, "call_suffix")
	if !ok {
		return 0
	}
	if trailing, ok := file.FlatFindChild(suffix, "annotated_lambda"); ok {
		lambda, _ := file.FlatFindChild(trailing, "lambda_literal")
		return lambda
	}
	args, ok := file.FlatFindChild(suffix, "value_arguments")
	if !ok {
		return 0
	}
	var last uint32
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) == "value_argument" {
			last = flatValueArgumentExpression(file, arg)
		}
	}
	if file.FlatType(last) == "lambda_literal" {
		return last
	}
	return 0
}

// slackDeclaredTypeAfterColon returns the type a Kotlin variable_declaration, parameter or class_parameter
// declares (a leading `suspend` modifier included), or 0 when it has none.
func slackDeclaredTypeAfterColon(file *scanner.File, decl uint32) uint32 {
	afterColon := false
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == ":" {
			afterColon = true
			continue
		}
		if afterColon && file.FlatIsNamed(child) {
			return child
		}
	}
	return 0
}

// slackFunctionShadows reports whether something called name declared in the file is in scope at idx ahead of an
// imported function, with Kotlin's priorities: a local function, a local variable or parameter of a function type
// (invoked as name(...)), or a member of an enclosing class always wins; a top-level function of the file's package
// wins only over a default or star import (explicitlyImported false), not over an explicit import.
func slackFunctionShadows(file *scanner.File, idx uint32, name string, explicitlyImported bool) bool {
	return slackScopeShadows(file, idx, name, !explicitlyImported)
}

// slackLocalFunctionShadows is slackFunctionShadows without the file's top-level functions: what wins even over an
// implicit receiver's member (#1940).
func slackLocalFunctionShadows(file *scanner.File, idx uint32, name string) bool {
	return slackScopeShadows(file, idx, name, false)
}

func slackScopeShadows(file *scanner.File, idx uint32, name string, topLevel bool) bool {
	child := idx
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "statements":
			for sib, found := file.FlatPrevSibling(child); found; sib, found = file.FlatPrevSibling(sib) {
				switch file.FlatType(sib) {
				case "function_declaration":
					if extractIdentifierFlat(file, sib) == name {
						return true
					}
				case "property_declaration":
					if v, ok := file.FlatFindChild(sib, "variable_declaration"); ok && extractIdentifierFlat(file, v) == name {
						return true
					}
				}
			}
		case "function_declaration", "anonymous_function", "secondary_constructor":
			if params, ok := file.FlatFindChild(parent, "function_value_parameters"); ok && lintParamsDeclare(file, params, name) {
				return true
			}
		case "lambda_literal":
			if params, ok := file.FlatFindChild(parent, "lambda_parameters"); ok && lintParamsDeclare(file, params, name) {
				return true
			}
		case "class_body":
			for member := file.FlatFirstChild(parent); member != 0; member = file.FlatNextSib(member) {
				if file.FlatType(member) == "function_declaration" && extractIdentifierFlat(file, member) == name {
					return true
				}
			}
		case "source_file":
			if !topLevel {
				return false
			}
			for member := file.FlatFirstChild(parent); member != 0; member = file.FlatNextSib(member) {
				if file.FlatType(member) == "function_declaration" && extractIdentifierFlat(file, member) == name {
					return true
				}
			}
		}
		child = parent
	}
	return false
}
