package rules

import "github.com/kaeawc/krit/internal/scanner"

func cursorDirectEnclosingLambda(file *scanner.File, lambda, bodyStart uint32) bool {
	if file.FlatType(lambda) != "lambda_literal" || !cursorLambdaCallArgument(file, lambda) {
		return false
	}
	statements, _ := file.FlatFindChild(lambda, "statements")
	for loop := file.FlatFirstChild(statements); loop != 0; loop = file.FlatNextSib(loop) {
		if file.FlatType(loop) != "while_statement" {
			continue
		}
		body, _ := file.FlatFindChild(loop, "control_structure_body")
		if body != 0 && file.FlatStartByte(body) == bodyStart {
			return true
		}
	}
	return false
}

func cursorLambdaCallArgument(file *scanner.File, lambda uint32) bool {
	parent, _ := file.FlatParent(lambda)
	switch file.FlatType(parent) {
	case "annotated_lambda":
		parent, _ = file.FlatParent(parent)
	case "value_argument":
		arguments, _ := file.FlatParent(parent)
		if file.FlatType(arguments) != "value_arguments" || file.FlatNamedChildCount(arguments) != 1 {
			return false
		}
		parent, _ = file.FlatParent(arguments)
	default:
		return false
	}
	if file.FlatType(parent) != "call_suffix" {
		return false
	}
	call, _ := file.FlatParent(parent)
	return file.FlatType(call) == "call_expression"
}
