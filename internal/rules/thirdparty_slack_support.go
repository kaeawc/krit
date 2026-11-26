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
