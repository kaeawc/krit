package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

func cursorPlatformColumn(file *scanner.File, call, expr uint32) bool {
	segments, valid := cursorQualifiedSegments(file, expr)
	if !valid || len(segments) < 2 || !cursorUpperSnake(segments[len(segments)-1]) {
		return false
	}
	qualified := strings.Join(segments, ".")
	platform := cursorPlatformPath(qualified)
	platformImport := ""
	imports := scanner.ParseSourceHeader(file.Content).Imports
	javaExplicitPlatformRoot := false
	javaFullyQualified := file.Language == scanner.LangJava && cursorPlatformPath(qualified)
	if file.Language == scanner.LangJava && !javaFullyQualified {
		for _, imported := range imports {
			if imported.Wildcard {
				continue
			}
			root := imported.Path[strings.LastIndex(imported.Path, ".")+1:]
			if (root == segments[0] || imported.Alias == segments[0]) && cursorPlatformPath(imported.Path) {
				javaExplicitPlatformRoot = true
				break
			}
		}
	}
	for _, imported := range imports {
		if imported.Wildcard {
			if cursorPlatformWildcardBindsRoot(file.Language, imported, javaFullyQualified, javaExplicitPlatformRoot) {
				return false
			}
			continue
		}
		root := imported.Path[strings.LastIndex(imported.Path, ".")+1:]
		if !imported.Wildcard && (root == segments[0] || imported.Alias == segments[0]) {
			if !cursorPlatformPath(imported.Path) || platformImport != "" && platformImport != imported.Path {
				return false
			}
			platformImport = imported.Path
			if imported.Alias == segments[0] || imported.Alias == "" && root == segments[0] {
				platform = true
			}
		}
	}
	return platform && !cursorPlatformSegmentsBound(file, call, segments)
}

func cursorPlatformPath(path string) bool {
	return strings.HasPrefix(path, "android.") || strings.HasPrefix(path, "java.") || strings.HasPrefix(path, "javax.")
}

func cursorUpperSnake(name string) bool {
	if name == "" || name[0] < 'A' || name[0] > 'Z' || name[len(name)-1] == '_' {
		return false
	}
	for index, character := range name {
		if character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' {
			continue
		}
		if character != '_' || index > 0 && name[index-1] == '_' {
			return false
		}
	}
	return true
}

func cursorQualifiedSegments(file *scanner.File, expr uint32) ([]string, bool) {
	switch file.FlatType(expr) {
	case "identifier", "simple_identifier":
		return []string{file.FlatNodeText(expr)}, true
	case "navigation_expression", "navigation_suffix", "field_access":
		var segments []string
		for child := file.FlatFirstChild(expr); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "." {
				continue
			}
			parts, valid := cursorQualifiedSegments(file, child)
			if !valid {
				return nil, false
			}
			segments = append(segments, parts...)
		}
		return segments, len(segments) > 0
	default:
		return nil, false
	}
}

func cursorPlatformSegmentsBound(file *scanner.File, call uint32, segments []string) bool {
	if cursorPlatformReceiverScope(file, call) {
		return true
	}
	fileNames := map[string]bool{}
	file.FlatWalkAllNodes(0, func(node uint32) { cursorPlatformDeclarationNames(file, node, fileNames) })
	for name := range fileNames {
		if strings.Trim(name, "`") == strings.Trim(segments[0], "`") {
			return true
		}
	}
	names := map[string]bool{}
	for scope, ok := file.FlatParent(call); ok; scope, ok = file.FlatParent(scope) {
		switch file.FlatType(scope) {
		case "function_declaration", "method_declaration", "anonymous_function", "lambda_literal", "lambda_expression":
			file.FlatWalkAllNodes(scope, func(node uint32) {
				cursorBindingNames(file, node, names)
				if file.FlatType(node) == "lambda_literal" && !file.FlatHasChildOfType(node, "lambda_parameters") &&
					!file.FlatHasChildOfType(node, "->") {
					names["it"] = true
				}
				if kind := file.FlatType(node); kind == "class_declaration" || kind == "object_declaration" || kind == "type_parameter" {
					if identifier, found := file.FlatFindChild(node, "type_identifier"); found {
						names[file.FlatNodeText(identifier)] = true
					}
				}
			})
		}
	}
	for _, segment := range segments {
		if names[segment] {
			return true
		}
	}
	return false
}

func cursorPlatformDeclarationNames(file *scanner.File, node uint32, names map[string]bool) {
	cursorBindingNames(file, node, names)
	switch file.FlatType(node) {
	case "ERROR":
		previous := ""
		for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
			kind := file.FlatType(child)
			if kind == "simple_identifier" || kind == "identifier" {
				switch previous {
				case "class", "object", "interface", "enum", "typealias", "fun":
					names[file.FlatNodeText(child)] = true
				}
			}
			previous = kind
		}
	case "class_declaration", "object_declaration", "companion_object", "type_parameter", "type_alias",
		"interface_declaration", "annotation_type_declaration", "enum_declaration", "enum_constant",
		"enum_entry", "record_declaration", "method_declaration":
		for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
			if kind := file.FlatType(child); kind == "type_identifier" || kind == "identifier" || kind == "simple_identifier" {
				names[file.FlatNodeText(child)] = true
			}
		}
	}
}

func cursorPlatformReceiverScope(file *scanner.File, call uint32) bool {
	for scope, ok := file.FlatParent(call); ok; scope, ok = file.FlatParent(scope) {
		if flatFunctionDeclarationIsExtension(file, scope) ||
			file.FlatType(scope) == "anonymous_function" && file.FlatHasChildOfType(scope, ".") {
			return true
		}
		if file.FlatType(scope) != "lambda_literal" && file.FlatType(scope) != "lambda_expression" {
			continue
		}
		parent, _ := file.FlatParent(scope)
		switch file.FlatType(parent) {
		case "annotated_lambda":
			parent, _ = file.FlatParent(parent)
		case "value_argument":
			parent, _ = file.FlatParent(parent)
			if file.FlatType(parent) != "value_arguments" {
				return true
			}
			parent, _ = file.FlatParent(parent)
		default:
			return true
		}
		if file.FlatType(parent) != "call_suffix" {
			return true
		}
		lambdaCall, _ := file.FlatParent(parent)
		if file.FlatType(lambdaCall) != "call_expression" {
			return true
		}
		switch flatCallNameAny(file, lambdaCall) {
		case "use", "let", "also", "forEach", "forEachIndexed", "onEach", "map", "mapNotNull", "filter", "takeIf", "takeUnless", "repeat":
		default:
			return true
		}
	}
	return false
}

// cursorPlatformWildcardBindsRoot reports whether a wildcard import could bind the reference's root segment.
func cursorPlatformWildcardBindsRoot(lang scanner.Language, imported scanner.HeaderImport, javaFullyQualified, javaExplicitPlatformRoot bool) bool {
	if lang != scanner.LangJava {
		return !cursorPlatformPath(imported.Path)
	}
	if javaFullyQualified {
		return !cursorPlatformPath(imported.Path)
	}
	return !javaExplicitPlatformRoot || imported.Static
}
