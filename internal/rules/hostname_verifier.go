package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	"github.com/kaeawc/krit/internal/scanner"
)

type hostnameVerifierFacts struct {
	imports map[string]string
	shadows map[string]int
	aliases map[string]uint32
	stars   map[string]bool
}

func hostnameVerifierFileFacts(file *scanner.File) *hostnameVerifierFacts {
	return filefacts.FileFact(fileFactsCache(), file, "hostnameVerifierSource", func() *hostnameVerifierFacts {
		facts := &hostnameVerifierFacts{imports: map[string]string{}, shadows: map[string]int{}, aliases: map[string]uint32{}, stars: map[string]bool{}}
		var imports []scanner.HeaderImport
		if file.Language == scanner.LangKotlin {
			imports = scanner.ParseSourceHeader(file.Content).Imports
		}
		file.FlatWalkAllNodes(0, func(node uint32) {
			switch file.FlatType(node) {
			case "import_declaration":
				for _, match := range lintImportRe.FindAllStringSubmatch(stripLineAndBlockComments(file.FlatNodeText(node)), -1) {
					imports = append(imports, scanner.HeaderImport{Path: match[2], Wildcard: match[3] != ""})
				}
			case "class_declaration", "interface_declaration", "enum_declaration", "enum_entry", "record_declaration", "object_declaration", "companion_object", "type_alias", "type_parameter", "function_declaration", "method_declaration", "parameter", "formal_parameter", "catch_formal_parameter", "catch_block", "class_parameter", "parameter_with_optional_type", "variable_declarator", "variable_declaration":
				name := hostnameVerifierDeclarationName(file, node)
				facts.shadows[name]++
				if file.FlatType(node) == "type_alias" {
					facts.aliases[name] = node
				}
			case "lambda_expression", "inferred_parameters":
				for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
					if file.FlatType(child) == "identifier" {
						facts.shadows[file.FlatNodeText(child)]++
					}
				}
			case "object", "class", "interface", "enum", "record":
				parent, ok := file.FlatParent(node)
				if ok && file.FlatType(parent) == "ERROR" {
					name := file.FlatNextSib(node)
					if file.FlatType(name) == "simple_identifier" || file.FlatType(name) == "identifier" || file.FlatType(name) == "type_identifier" {
						facts.shadows[strings.Trim(file.FlatNodeText(name), "`")]++
					}
				}
			}
		})
		for _, imported := range imports {
			if imported.Wildcard {
				facts.stars[imported.Path] = true
				continue
			}
			name := imported.Alias
			if name == "" {
				name = imported.Path[strings.LastIndex(imported.Path, ".")+1:]
			}
			facts.imports[name] = imported.Path
		}
		return facts
	})
}

func hostnameVerifierSourceMatches(file *scanner.File, _ uint32, text, fqn string) bool {
	text = strings.ReplaceAll(strings.TrimSpace(text), "`", "")
	if text == "" {
		return false
	}
	head, _, _ := strings.Cut(text, ".")
	matched := text == fqn
	facts := hostnameVerifierFileFacts(file)
	if strings.HasPrefix(fqn, "javax.net.ssl.") && facts.stars["javax.net.ssl"] {
		matched = lintTypeRefMatches(file, text, fqn)
	}
	if imported, ok := facts.imports[head]; ok {
		matched = imported+strings.TrimPrefix(text, head) == fqn
	}
	return matched && facts.shadows[head] == 0
}

func hostnameVerifierDeclarationName(file *scanner.File, node uint32) string {
	for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "simple_identifier", "identifier":
			return strings.Trim(file.FlatNodeText(child), "`")
		case "type_identifier":
			switch file.FlatType(node) {
			case "class_declaration", "interface_declaration", "enum_declaration", "record_declaration", "object_declaration", "companion_object", "type_alias", "type_parameter":
				return strings.Trim(file.FlatNodeText(child), "`")
			}
		}
	}
	return ""
}

func badHostnameVerifierAdditionalMethod(file *scanner.File, owner, method uint32) bool {
	var direct bool
	for _, supertype := range lintSupertypeNames(file, owner) {
		direct = direct || lintTypeRefMatches(file, supertype, "javax.net.ssl.HostnameVerifier")
	}
	if !direct {
		return false
	}
	for parent, ok := file.FlatParent(method); ok; parent, ok = file.FlatParent(parent) {
		if parent == owner {
			break
		}
		switch file.FlatType(parent) {
		case "function_declaration", "anonymous_function", "method_declaration", "lambda_literal", "lambda_expression", "anonymous_initializer", "static_initializer", "secondary_constructor", "constructor_declaration", "getter", "setter", "class_declaration", "interface_declaration", "enum_declaration", "record_declaration", "object_literal", "object_declaration", "object_creation_expression":
			return false
		}
	}
	if _, generic := file.FlatFindChild(method, "type_parameters"); generic {
		return false
	}
	if file.Language == scanner.LangKotlin && flatFunctionReceiverTypeName(file, method) != "" {
		return false
	}
	var vararg bool
	file.FlatWalkAllNodes(method, func(node uint32) {
		vararg = vararg || file.FlatType(node) == "vararg"
	})
	if vararg {
		return false
	}
	body, ok := file.FlatFindChild(method, "function_body")
	if ok {
		if expression := expressionBodyValue(file, body); expression != 0 {
			return isTrueLiteral(file, expression)
		}
		body, _ = file.FlatFindChild(body, "statements")
	} else {
		body, _ = file.FlatFindChild(method, "block")
	}
	return badHostnameVerifierSingleTrueStatement(file, body, false)
}

func badHostnameVerifierSingleTrueStatement(file *scanner.File, body uint32, implicitReturn bool) bool {
	var statement uint32
	for child := file.FlatFirstChild(body); body != 0 && child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) || strings.Contains(file.FlatType(child), "comment") {
			continue
		}
		if statement != 0 {
			return false
		}
		statement = child
	}
	if statement == 0 {
		return false
	}
	if implicitReturn && isTrueLiteral(file, flatUnwrapParenExpr(file, statement)) {
		return true
	}
	return badHostnameVerifierTrueReturn(file, statement, implicitReturn)
}

func badHostnameVerifierTrueReturn(file *scanner.File, statement uint32, allowParentheses bool) bool {
	if file.FlatType(statement) != "jump_expression" && file.FlatType(statement) != "return_statement" {
		return false
	}
	if !strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(statement)), "return") {
		return false
	}
	for child := file.FlatFirstChild(statement); child != 0; child = file.FlatNextSib(child) {
		value := child
		if allowParentheses {
			value = flatUnwrapParenExpr(file, value)
		}
		if file.FlatIsNamed(child) && !strings.Contains(file.FlatType(child), "comment") && isTrueLiteral(file, value) {
			return true
		}
	}
	return false
}

func badHostnameVerifierSamLambda(file *scanner.File, call uint32) bool {
	callee, ok := lintCallAt(file, call)
	if !ok || !hostnameVerifierSourceMatches(file, call, callee.TypeText, "javax.net.ssl.HostnameVerifier") {
		return false
	}
	lambda := flatCallTrailingLambda(file, call)
	if lambda != 0 && len(callee.Args) != 0 {
		return false
	}
	if lambda == 0 && len(callee.Args) == 1 {
		lambda = flatUnwrapParenExpr(file, callee.Args[0])
		if file.FlatType(lambda) != "lambda_literal" {
			return false
		}
	}
	if lambda == 0 {
		return false
	}
	body, _ := file.FlatFindChild(lambda, "statements")
	return badHostnameVerifierSingleTrueStatement(file, body, true)
}
