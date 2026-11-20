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
}

func hostnameVerifierFileFacts(file *scanner.File) *hostnameVerifierFacts {
	return filefacts.FileFact(fileFactsCache(), file, "hostnameVerifierSource", func() *hostnameVerifierFacts {
		facts := &hostnameVerifierFacts{imports: map[string]string{}, shadows: map[string]int{}, aliases: map[string]uint32{}}
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
			case "class_declaration", "interface_declaration", "object_declaration", "type_alias", "type_parameter", "function_declaration", "method_declaration", "parameter", "formal_parameter", "parameter_with_optional_type", "variable_declarator", "variable_declaration":
				name := hostnameVerifierDeclarationName(file, node)
				facts.shadows[name]++
				if file.FlatType(node) == "type_alias" {
					facts.aliases[name] = node
				}
			}
		})
		for _, imported := range imports {
			if imported.Wildcard {
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
			if file.FlatType(node) == "class_declaration" || file.FlatType(node) == "interface_declaration" || file.FlatType(node) == "type_alias" || file.FlatType(node) == "type_parameter" {
				return strings.Trim(file.FlatNodeText(child), "`")
			}
		}
	}
	return ""
}

func badHostnameVerifierStringType(file *scanner.File, text string) bool {
	text = strings.TrimSuffix(strings.TrimSpace(text), "?")
	if file.Language == scanner.LangKotlin && text == "String" {
		return lintSameFileClass(file, "String") == 0 && lintSameFileTypeAlias(file, "String") == "" &&
			(lintImportsOf(file).bySimple["String"] == "" || lintImportsOf(file).bySimple["String"] == "kotlin.String")
	}
	return javaLangTypeRef(file, text, "String") || text == "kotlin.String"
}

func badHostnameVerifierOwner(file *scanner.File, owner uint32, depth int) bool {
	if depth > 5 {
		return false
	}
	for _, supertype := range lintSupertypeNames(file, owner) {
		if hostnameVerifierSourceMatches(file, owner, supertype, "javax.net.ssl.HostnameVerifier") {
			return true
		}
		facts := hostnameVerifierFileFacts(file)
		if facts.imports[supertype] != "" || facts.shadows[supertype] != 1 {
			continue
		}
		if declared := lintSameFileClass(file, supertype); declared != 0 && declared != owner && badHostnameVerifierClassVisible(file, declared, owner) && badHostnameVerifierOwner(file, declared, depth+1) {
			return true
		}
		if target := lintSameFileTypeAlias(file, supertype); target != "" && badHostnameVerifierClassVisible(file, facts.aliases[supertype], owner) && hostnameVerifierSourceMatches(file, owner, target, "javax.net.ssl.HostnameVerifier") {
			return true
		}
	}
	return false
}

func badHostnameVerifierClassVisible(file *scanner.File, declaration, owner uint32) bool {
	for parent, ok := file.FlatParent(declaration); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "function_declaration", "method_declaration":
			return false
		case "class_declaration", "interface_declaration", "object_declaration", "companion_object", "object_literal":
			for scope, found := file.FlatParent(owner); found; scope, found = file.FlatParent(scope) {
				if scope == parent {
					return true
				}
			}
			return false
		}
	}
	return true
}

func badHostnameVerifierLiteralBody(file *scanner.File, method uint32) bool {
	if _, generic := file.FlatFindChild(method, "type_parameters"); generic {
		return false
	}
	if _, context := file.FlatFindChild(method, scanner.ContextParametersNodeType); context {
		return false
	}
	if file.Language == scanner.LangKotlin && flatFunctionReceiverTypeName(file, method) != "" {
		return false
	}
	body, ok := file.FlatFindChild(method, "function_body")
	if ok {
		if expression := expressionBodyValue(file, body); expression != 0 {
			return isTrueLiteral(file, flatUnwrapParenExpr(file, expression))
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
	if file.FlatType(statement) != "jump_expression" && file.FlatType(statement) != "return_statement" {
		return false
	}
	if !strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(statement)), "return") {
		return false
	}
	for child := file.FlatFirstChild(statement); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) && !strings.Contains(file.FlatType(child), "comment") && isTrueLiteral(file, flatUnwrapParenExpr(file, child)) {
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
