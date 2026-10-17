package scanner

import (
	"sort"
	"strings"
)

// TopLevelDecl is a top-level declaration other files can see.
type TopLevelDecl struct {
	Name string
	// Operator reports whether the declaration carries the `operator`
	// modifier, so calls through operator syntax can resolve to it.
	Operator bool
}

// TopLevelDeclaredNames returns the package of a parsed Kotlin file and its
// top-level declarations that other files can see: functions (extensions
// included), properties, classes, interfaces, objects, and typealiases that
// are not private. These are the names whose arrival in a package can change
// how a call or type reference in another file resolves.
//
// Only direct children of the file are reported, so members of classes and
// objects, and declarations nested in lambdas, never leak. A `fun interface`
// declaration, which the grammar parses as an error node followed by a lambda
// holding the body, is recognized by its tokens and yields the interface
// name rather than its members.
func TopLevelDeclaredNames(file *File) (pkg string, decls []TopLevelDecl) {
	if file == nil || file.FlatTree == nil || file.FlatTree.Len() == 0 {
		return "", nil
	}
	byName := map[string]int{}
	add := func(name string, operator bool) {
		name = strings.Trim(name, "`")
		if name == "" || name == "_" {
			return
		}
		if i, ok := byName[name]; ok {
			decls[i].Operator = decls[i].Operator || operator
			return
		}
		byName[name] = len(decls)
		decls = append(decls, TopLevelDecl{Name: name, Operator: operator})
	}
	for child := file.FlatFirstChild(0); child != 0; child = file.FlatNextSib(child) {
		name, operator := visibleTopLevelName(file, child)
		add(name, operator)
	}
	sort.Slice(decls, func(i, j int) bool { return decls[i].Name < decls[j].Name })
	header := ParseSourceHeader(file.Content)
	return header.Package, decls
}

// visibleTopLevelName returns the declared name of a direct child of the
// file when it is a non-private declaration, or "".
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
	case "ERROR":
		if name, private := funInterfaceName(file, child); !private {
			return name, false
		}
	}
	return "", false
}

// funInterfaceName recognizes the error node the grammar produces for
// `fun interface Name`: the `fun` keyword, `interface` parsed as a user type,
// then the name as a simple identifier.
func funInterfaceName(file *File, errNode uint32) (name string, private bool) {
	sawFun, sawInterface := false, false
	for c := file.FlatFirstChild(errNode); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "fun":
			sawFun = true
		case "user_type":
			if sawFun && file.FlatNodeText(c) == "interface" {
				sawInterface = true
			}
		case "simple_identifier":
			if sawInterface {
				text := file.FlatNodeText(errNode)
				prefix := text[:strings.Index(text, "fun")]
				return file.FlatNodeText(c), strings.Contains(prefix, "private")
			}
		}
	}
	return "", false
}
