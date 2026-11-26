package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// implicitIntentActionEval evaluates intent action expressions the way Lint's ConstantEvaluator does for
// UnsafeImplicitIntentLaunch (#1624): string literals and same-file constants, framework action constants such as
// Intent.ACTION_VIEW, and project constants declared in other source files (`const val`, a Kotlin `val` with a
// constant initializer, or a Java `static final` field).
type implicitIntentActionEval struct {
	files []*scanner.File
	// decls indexes the project's property and field declarations by name, built on first use.
	decls map[string][]implicitIntentConstDecl
	// fileNames caches, per file, the names it declares as parameters, properties or variables.
	fileNames map[*scanner.File]map[string]bool
}

type implicitIntentConstDecl struct {
	file *scanner.File
	node uint32
}

func newImplicitIntentActionEval(files []*scanner.File) *implicitIntentActionEval {
	return &implicitIntentActionEval{files: files}
}

// eval returns the constant string value of expr in file.
func (e *implicitIntentActionEval) eval(file *scanner.File, expr uint32) (string, bool) {
	if v, ok := lintEvalString(file, expr); ok {
		return v, true
	}
	expr = lintSkipParensJS(file, expr)
	text := strings.ReplaceAll(strings.Join(strings.Fields(file.FlatNodeText(expr)), ""), "`", "")
	if text == "" || strings.ContainsAny(text, "()[]{}\"+?!:") {
		return "", false
	}
	parts := strings.Split(text, ".")
	name := parts[len(parts)-1]
	if len(parts) == 1 {
		if e.declaresName(file, name) {
			return "", false
		}
		return e.topLevel(file, name)
	}
	qualifier := strings.Join(parts[:len(parts)-1], ".")
	owner := parts[len(parts)-2]
	if v, ok := lintFrameworkActionConstants[owner+"."+name]; ok && implicitIntentFrameworkQualifier(file, qualifier) {
		return v, true
	}
	return e.member(file, qualifier, owner, name)
}

// implicitIntentFrameworkQualifier reports whether qualifier names an android.* framework class: written in full,
// imported, or reached through an android.* wildcard import that no same-file class shadows.
func implicitIntentFrameworkQualifier(file *scanner.File, qualifier string) bool {
	if strings.HasPrefix(qualifier, "android.") {
		return true
	}
	if strings.Contains(qualifier, ".") {
		return false
	}
	imp := lintImportsOf(file)
	if full, ok := imp.bySimple[qualifier]; ok {
		return strings.HasPrefix(full, "android.") && strings.HasSuffix(full, "."+qualifier)
	}
	if lintSameFileClass(file, qualifier) != 0 {
		return false
	}
	for _, star := range imp.stars {
		if strings.HasPrefix(star, "android.") {
			return true
		}
	}
	return false
}

// declsNamed returns the project's property and field declarations named name.
func (e *implicitIntentActionEval) declsNamed(name string) []implicitIntentConstDecl {
	if e.decls == nil {
		e.decls = map[string][]implicitIntentConstDecl{}
		for _, f := range e.files {
			if f == nil || f.FlatTree == nil {
				continue
			}
			for _, kind := range []string{"property_declaration", "field_declaration", "constant_declaration"} {
				f.FlatWalkNodes(0, kind, func(n uint32) {
					if !implicitIntentMemberOrTopLevel(f, n) {
						return
					}
					if name := implicitIntentDeclName(f, n); name != "" {
						e.decls[name] = append(e.decls[name], implicitIntentConstDecl{file: f, node: n})
					}
				})
			}
		}
	}
	return e.decls[name]
}

// declaresName reports whether file declares name as a parameter, property or variable, which would shadow a
// top-level constant of the same name from another file.
func (e *implicitIntentActionEval) declaresName(file *scanner.File, name string) bool {
	if e.fileNames == nil {
		e.fileNames = map[*scanner.File]map[string]bool{}
	}
	names, ok := e.fileNames[file]
	if !ok {
		names = map[string]bool{}
		for _, kind := range []string{"parameter", "class_parameter", "formal_parameter", "variable_declaration", "variable_declarator"} {
			file.FlatWalkNodes(0, kind, func(n uint32) {
				for _, idKind := range []string{"simple_identifier", "identifier"} {
					if id, ok := file.FlatFindChild(n, idKind); ok {
						names[file.FlatNodeText(id)] = true
						return
					}
				}
			})
		}
		e.fileNames[file] = names
	}
	return names[name]
}

// member evaluates `Owner.NAME` (or `pkg.Owner.NAME`): a constant declared in the class, object or companion
// object the qualifier names from file. A same-file owner wins; otherwise the owner must be the explicitly imported
// one, or come from the written package, the file's package or a wildcard import (#1774 review).
func (e *implicitIntentActionEval) member(file *scanner.File, qualifier, owner, name string) (string, bool) {
	sameFileOwner := implicitIntentSameFileOwner(file, owner)
	imp := lintImportsOf(file)
	var pkgs map[string]bool
	switch {
	case strings.Contains(qualifier, "."):
		pkgs = map[string]bool{qualifier[:strings.LastIndexByte(qualifier, '.')]: true}
	case sameFileOwner:
	default:
		pkgs = map[string]bool{}
		if full, ok := imp.bySimple[owner]; ok {
			pkgs[strings.TrimSuffix(full, "."+owner)] = true
		} else {
			pkgs[filePackageName(file)] = true
			for _, star := range imp.stars {
				pkgs[star] = true
			}
		}
	}
	var value string
	found := 0
	for _, d := range e.declsNamed(name) {
		if implicitIntentConstOwnerName(d.file, d.node) != owner {
			continue
		}
		if sameFileOwner && !strings.Contains(qualifier, ".") {
			if d.file != file {
				continue
			}
		} else if !pkgs[filePackageName(d.file)] {
			continue
		}
		v, ok := implicitIntentConstValue(d.file, d.node)
		if !ok {
			return "", false
		}
		value = v
		found++
	}
	return value, found == 1
}

// implicitIntentSameFileOwner reports whether file declares a class, interface or object named owner.
func implicitIntentSameFileOwner(file *scanner.File, owner string) bool {
	found := false
	for _, kind := range []string{"class_declaration", "object_declaration", "interface_declaration"} {
		file.FlatWalkNodes(0, kind, func(n uint32) {
			if !found && implicitIntentDeclTypeName(file, n) == owner {
				found = true
			}
		})
	}
	return found
}

// implicitIntentMemberOrTopLevel reports whether a property or field is declared at top level or directly in a
// class, interface, object or companion body, rather than as a local in a function, lambda or accessor.
func implicitIntentMemberOrTopLevel(file *scanner.File, n uint32) bool {
	parent, ok := file.FlatParent(n)
	if !ok {
		return false
	}
	switch file.FlatType(parent) {
	case "source_file", "class_body", "enum_class_body", "interface_body":
		return true
	}
	return false
}

// topLevel evaluates a bare NAME: an explicitly imported constant, or a top-level Kotlin constant of the file's
// package declared in another file.
func (e *implicitIntentActionEval) topLevel(file *scanner.File, name string) (string, bool) {
	imported := lintImportsOf(file).bySimple[name]
	pkg := filePackageName(file)
	var value string
	found := 0
	for _, d := range e.declsNamed(name) {
		if d.file == file {
			continue
		}
		declPkg := filePackageName(d.file)
		owner := lintDeclarationOwner(d.file, d.node)
		switch {
		case owner == 0 && imported != "" && imported == implicitIntentJoin(declPkg, name):
		case owner == 0 && imported == "" && declPkg == pkg:
		case owner != 0 && imported != "" && imported == implicitIntentJoin(implicitIntentJoin(declPkg, implicitIntentConstOwnerName(d.file, d.node)), name):
			// A static import of a Java field or Kotlin object constant.
		default:
			continue
		}
		if v, ok := implicitIntentConstValue(d.file, d.node); ok {
			value = v
			found++
		}
	}
	return value, found == 1
}

func implicitIntentJoin(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

// implicitIntentDeclName is the name a property_declaration or field_declaration declares.
func implicitIntentDeclName(file *scanner.File, n uint32) string {
	if file.FlatType(n) == "property_declaration" {
		if vd, ok := file.FlatFindChild(n, "variable_declaration"); ok {
			return lintDeclaredName(file, vd)
		}
		return ""
	}
	if vd, ok := file.FlatFindChild(n, "variable_declarator"); ok {
		if id, ok := file.FlatFindChild(vd, "identifier"); ok {
			return file.FlatNodeText(id)
		}
	}
	return ""
}

// implicitIntentConstOwnerName is the class or object a declaration belongs to; a companion object counts as its
// class. "" for a top-level declaration.
func implicitIntentConstOwnerName(file *scanner.File, n uint32) string {
	owner := lintDeclarationOwner(file, n)
	if owner == 0 {
		return ""
	}
	if file.FlatType(owner) == "companion_object" {
		if outer := lintDeclarationOwner(file, owner); outer != 0 {
			owner = outer
		}
	}
	return implicitIntentDeclTypeName(file, owner)
}

// implicitIntentDeclTypeName is the name a class, interface or object declaration declares.
func implicitIntentDeclTypeName(file *scanner.File, decl uint32) string {
	for _, kind := range []string{"type_identifier", "simple_identifier", "identifier"} {
		if id, ok := file.FlatFindChild(decl, kind); ok {
			return file.FlatNodeText(id)
		}
	}
	return ""
}

// implicitIntentConstValue evaluates a constant declaration: a Kotlin `val`/`const val` or a Java `static final`
// field whose initializer is a constant string in its own file.
func implicitIntentConstValue(file *scanner.File, n uint32) (string, bool) {
	var init uint32
	switch file.FlatType(n) {
	case "property_declaration":
		kind, ok := file.FlatFindChild(n, "binding_pattern_kind")
		if !ok || strings.TrimSpace(file.FlatNodeText(kind)) != "val" {
			return "", false
		}
		if _, delegated := file.FlatFindChild(n, "property_delegate"); delegated {
			return "", false
		}
		init = lintInitializerAfterEquals(file, n)
	case "field_declaration", "constant_declaration":
		// Java interface fields (constant_declaration) are implicitly public static final.
		if file.FlatType(n) == "field_declaration" && (!file.FlatHasModifier(n, "static") || !file.FlatHasModifier(n, "final")) {
			return "", false
		}
		if vd, ok := file.FlatFindChild(n, "variable_declarator"); ok {
			init = lintInitializerAfterEquals(file, vd)
		}
	}
	if init == 0 {
		return "", false
	}
	return lintEvalString(file, init)
}
