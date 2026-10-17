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
		if name != "" {
			add(name, operator)
			continue
		}
		for _, recovered := range recoveredTopLevelNames(file, child) {
			add(recovered, false)
		}
	}
	sort.Slice(decls, func(i, j int) bool { return decls[i].Name < decls[j].Name })
	header := ParseSourceHeader(file.Content)
	return header.Package, decls
}
