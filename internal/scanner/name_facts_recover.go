package scanner

import (
	"regexp"
	"strings"
)

// visibleTopLevelName returns the declared name of a direct child of the
// file when it is a cleanly parsed, non-private declaration, or "".
func visibleTopLevelName(file *File, child uint32) (name string, operator bool) {
	switch file.FlatType(child) {
	case "function_declaration":
		if file.FlatHasModifier(child, "private") {
			return "", false
		}
		return file.FlatChildTextOrEmpty(child, "simple_identifier"), file.FlatHasModifier(child, "operator")
	case "property_declaration":
		if file.FlatHasModifier(child, "private") {
			return "", false
		}
		name = file.FlatChildTextOrEmpty(child, "simple_identifier")
		if name == "" {
			if varDecl, ok := file.FlatFindChild(child, "variable_declaration"); ok {
				name = file.FlatChildTextOrEmpty(varDecl, "simple_identifier")
			}
		}
		return name, file.FlatHasModifier(child, "operator")
	case "class_declaration", "object_declaration", "type_alias":
		if file.FlatHasModifier(child, "private") {
			return "", false
		}
		name = file.FlatChildTextOrEmpty(child, "type_identifier")
		if name == "" {
			name = file.FlatChildTextOrEmpty(child, "simple_identifier")
		}
		return name, false
	}
	return "", false
}

// cleanTopLevelTypes are root children the grammar parsed as ordinary
// declarations or as non-declaration header content; they are never
// recovered from.
var cleanTopLevelTypes = map[string]bool{
	"function_declaration": true, "property_declaration": true,
	"class_declaration": true, "object_declaration": true, "type_alias": true,
	"package_header": true, "import_list": true, "import_header": true,
	"line_comment": true, "multiline_comment": true,
	"file_annotation": true, "annotation": true, "shebang_line": true,
	// A bare lambda at the root is the body of a `fun interface`, whose
	// members must not be reported.
	"lambda_literal": true,
}

// recoveredTopLevelNames handles root children the grammar failed to parse
// as a declaration: error nodes and the expression shapes error recovery
// produces for `fun interface`, annotated, or context-receiver declarations.
// A `fun interface Name` yields just Name. Anything else contributes every
// identifier in it except keywords and parameter, type-argument, and
// argument lists. Over-collecting is safe: a hit still has to use the name.
func recoveredTopLevelNames(file *File, child uint32) []string {
	if cleanTopLevelTypes[file.FlatType(child)] {
		return nil
	}
	if name, private := funInterfaceName(file, child); name != "" {
		if private {
			return nil
		}
		return []string{name}
	}
	var names []string
	collectIdentifiers(file, child, &names)
	return names
}

var recoveryKeywords = map[string]bool{
	"fun": true, "interface": true, "class": true, "object": true, "val": true,
	"var": true, "typealias": true, "public": true, "private": true,
	"internal": true, "protected": true, "abstract": true, "open": true,
	"final": true, "sealed": true, "data": true, "enum": true, "annotation": true,
	"inline": true, "value": true, "suspend": true, "operator": true,
	"infix": true, "tailrec": true, "external": true, "const": true,
	"lateinit": true, "override": true, "expect": true, "actual": true,
	"vararg": true, "crossinline": true, "noinline": true, "reified": true,
	"companion": true, "context": true,
}

var recoverySkipSubtrees = map[string]bool{
	"lambda_literal": true, "function_value_parameters": true,
	"type_parameters": true, "type_arguments": true, "value_arguments": true,
	"class_body": true,
}

func collectIdentifiers(file *File, idx uint32, out *[]string) {
	switch file.FlatType(idx) {
	case "simple_identifier", "type_identifier":
		if text := file.FlatNodeText(idx); text != "" && !recoveryKeywords[text] {
			*out = append(*out, text)
		}
		return
	}
	if recoverySkipSubtrees[file.FlatType(idx)] {
		return
	}
	for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
		collectIdentifiers(file, c, out)
	}
}

type funInterfaceScan struct {
	funStart             uint32
	sawFun, sawInterface bool
	name                 string
}

var quotedString = regexp.MustCompile(`"[^"]*"`)

// funInterfaceName recognizes `fun interface Name` in the nodes the grammar
// produces for it: the `fun` keyword, `interface` parsed as a user type or
// identifier, then the name as a simple identifier. It scans the subtree in
// source order, so type parameters and supertypes after the name, and
// modifiers or annotations before `fun`, do not matter. private reports a
// `private` modifier before the `fun` keyword.
func funInterfaceName(file *File, root uint32) (name string, private bool) {
	scan := funInterfaceScan{}
	scanFunInterface(file, root, &scan)
	if scan.name == "" {
		return "", false
	}
	start := file.FlatStartByte(root)
	prefix := ""
	if scan.funStart > start {
		prefix = quotedString.ReplaceAllString(string(file.Content[start:scan.funStart]), `""`)
	}
	for _, word := range strings.Fields(strings.NewReplacer("(", " ", ")", " ", "@", " ").Replace(prefix)) {
		if word == "private" {
			return scan.name, true
		}
	}
	return scan.name, false
}

func scanFunInterface(file *File, idx uint32, scan *funInterfaceScan) {
	if scan.name != "" {
		return
	}
	switch file.FlatType(idx) {
	case "fun":
		if !scan.sawFun {
			scan.sawFun, scan.funStart = true, file.FlatStartByte(idx)
		}
		return
	case "simple_identifier", "user_type", "type_identifier":
		text := file.FlatNodeText(idx)
		switch {
		case text == "fun" && !scan.sawFun:
			scan.sawFun, scan.funStart = true, file.FlatStartByte(idx)
		case text == "interface" && scan.sawFun:
			scan.sawInterface = true
		case scan.sawInterface && file.FlatType(idx) == "simple_identifier":
			scan.name = text
		}
		return
	case "lambda_literal":
		return
	}
	for c := file.FlatFirstChild(idx); c != 0 && scan.name == ""; c = file.FlatNextSib(c) {
		scanFunInterface(file, c, scan)
	}
}
