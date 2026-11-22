package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func bangBangSourceOperandNonNull(file *scanner.File, operand uint32) bool {
	operand = flatUnwrapParenExpr(file, operand)
	if file.FlatType(operand) != "simple_identifier" {
		return false
	}
	name := file.FlatNodeText(operand)
	decl := bangBangLocalDeclaration(file, operand, name)
	if decl == 0 {
		return false
	}
	typed := decl
	if file.FlatType(decl) == "property_declaration" {
		typed, _ = file.FlatFindChild(decl, "variable_declaration")
	}
	if typed == 0 {
		return false
	}
	for child := file.FlatFirstChild(typed); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "user_type":
			return bangBangDeclaredTypeNonNull(file, operand, child)
		case "nullable_type", "function_type", "parenthesized_type", "not_nullable_type":
			return false
		}
	}
	return false
}

func bangBangLocalDeclaration(file *scanner.File, ref uint32, name string) uint32 {
	for scope, ok := file.FlatParent(ref); ok; scope, ok = file.FlatParent(scope) {
		switch file.FlatType(scope) {
		case "statements":
			var latest uint32
			for child := file.FlatFirstChild(scope); child != 0; child = file.FlatNextSib(child) {
				if file.FlatStartByte(child) >= file.FlatStartByte(ref) {
					break
				}
				if file.FlatType(child) != "property_declaration" {
					continue
				}
				matches := extractIdentifierFlat(file, child) == name
				if multi, found := file.FlatFindChild(child, "multi_variable_declaration"); found {
					for variable := file.FlatFirstChild(multi); variable != 0; variable = file.FlatNextSib(variable) {
						matches = matches || extractIdentifierFlat(file, variable) == name
					}
				}
				if matches {
					if file.FlatEndByte(child) > file.FlatStartByte(ref) {
						return 0
					}
					latest = child
				}
			}
			if latest != 0 {
				return latest
			}
		case "function_declaration", "anonymous_function":
			params, _ := file.FlatFindChild(scope, "function_value_parameters")
			for param := file.FlatFirstChild(params); param != 0; param = file.FlatNextSib(param) {
				if file.FlatType(param) == "parameter" && extractIdentifierFlat(file, param) == name {
					return param
				}
			}
			return 0
		case "lambda_literal", "for_statement", "do_while_statement", "catch_block", "when_subject", "when_expression",
			"multi_variable_declaration", "class_declaration", "object_declaration", "interface_declaration",
			"class_body", "object_literal", "source_file":
			return 0
		}
	}
	return 0
}

func bangBangDeclaredTypeNonNull(file *scanner.File, ref, annotation uint32) bool {
	typeName := file.FlatNodeText(file.FlatNamedChild(annotation, 0))
	if generic, _ := enclosingTypeParameterAllowsNull(file, ref, typeName); generic {
		return false
	}
	uncertain := false
	wildcardImport := false
	declaredClass := false
	file.FlatWalkAllNodes(0, func(node uint32) {
		switch file.FlatType(node) {
		case "class_declaration", "object_declaration":
			if extractIdentifierFlat(file, node) != typeName {
				return
			}
			owner, _ := file.FlatParent(node)
			if file.FlatType(owner) == "statements" && file.FlatStartByte(node) > file.FlatStartByte(ref) {
				return
			}
			for scope, ok := file.FlatParent(ref); ok; scope, ok = file.FlatParent(scope) {
				if scope == owner {
					declaredClass = true
					break
				}
			}
		case "type_alias":
			if extractIdentifierFlat(file, node) == typeName {
				uncertain = true
			}
		case "import_header":
			text := strings.TrimSpace(strings.TrimPrefix(file.FlatNodeText(node), "import"))
			wildcardImport = wildcardImport || strings.HasSuffix(text, ".*")
			if strings.HasSuffix(text, "."+typeName) || strings.HasSuffix(text, " as "+typeName) {
				uncertain = true
			}
		}
	})
	if uncertain || wildcardImport && !declaredClass {
		return false
	}
	_, primitive := typeinfer.PrimitiveTypes[typeName]
	_, standard := typeinfer.KotlinStdlibTypes[typeName]
	return primitive || standard || declaredClass
}
