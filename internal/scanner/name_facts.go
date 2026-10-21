package scanner

import (
	"bytes"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/hashutil"
)

// TopLevelDecl is a top-level declaration other files can see.
type TopLevelDecl struct {
	Name string
	// Operator reports whether the declaration carries the `operator`
	// modifier, so calls through operator syntax can resolve to it.
	Operator bool
	// Sig fingerprints what other files can observe of the declaration: for
	// a function with a block body and no inline or contract semantics, the
	// text up to the body; otherwise the whole declaration. It is stable
	// across edits that cannot change how a call resolves, so a changed Sig
	// is what makes a same-named declaration a new collision. Overloads that
	// share a name in one file are folded into one Sig.
	Sig string
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
	var sigs [][][]byte
	add := func(name string, operator bool, sig []byte) {
		name = strings.Trim(name, "`")
		if name == "" || name == "_" {
			return
		}
		if i, ok := byName[name]; ok {
			decls[i].Operator = decls[i].Operator || operator
			sigs[i] = append(sigs[i], sig)
			return
		}
		byName[name] = len(decls)
		decls = append(decls, TopLevelDecl{Name: name, Operator: operator})
		sigs = append(sigs, [][]byte{sig})
	}
	for child := file.FlatFirstChild(0); child != 0; child = file.FlatNextSib(child) {
		name, operator := visibleTopLevelName(file, child)
		if name != "" {
			add(name, operator, declarationSignature(file, child))
			continue
		}
		whole := file.Content[file.FlatStartByte(child):file.FlatEndByte(child)]
		for _, recovered := range recoveredTopLevelNames(file, child) {
			add(recovered, false, whole)
		}
	}
	foldSignatures(decls, sigs)
	sort.Slice(decls, func(i, j int) bool { return decls[i].Name < decls[j].Name })
	header := ParseSourceHeader(file.Content)
	return header.Package, decls
}

// foldSignatures sets each decl's Sig from the signature texts of the
// declarations that share its name. The texts are sorted first so reordering
// overloads in a file does not change the result.
func foldSignatures(decls []TopLevelDecl, sigs [][][]byte) {
	for i := range decls {
		parts := sigs[i]
		sort.Slice(parts, func(a, b int) bool { return bytes.Compare(parts[a], parts[b]) < 0 })
		decls[i].Sig = hashutil.HashHex(bytes.Join(parts, []byte{0}))[:16]
	}
}

// declarationSignature returns the text of a top-level declaration that other
// files can observe. A function with a block body has its body dropped when
// the body cannot leak into callers: an expression body may fix an inferred
// return type, an inline function is expanded into callers, and a contract
// declares facts about them. Everything else (properties, which may be const
// or have inferred types, and classes, whose members take part in
// constructor and invoke resolution) keeps its whole text.
func declarationSignature(file *File, decl uint32) []byte {
	start, end := file.FlatStartByte(decl), file.FlatEndByte(decl)
	whole := file.Content[start:end]
	if file.FlatType(decl) != "function_declaration" || file.FlatHasModifier(decl, "inline") {
		return whole
	}
	body, ok := file.FlatFindChild(decl, "function_body")
	if !ok {
		return whole
	}
	first := file.FlatFirstChild(body)
	if first == 0 || file.FlatType(first) != "{" {
		return whole
	}
	bodyStart := file.FlatStartByte(body)
	if bytes.Contains(file.Content[bodyStart:end], []byte("contract")) {
		return whole
	}
	return file.Content[start:bodyStart]
}

// TopLevelJavaTypeNames returns the package of a parsed Java file and the
// top-level types it declares: classes, interfaces, enums, records, and
// annotation types. Each Sig covers the whole declaration, because a Java
// type's constructors and members all take part in how Kotlin callers
// resolve a name.
func TopLevelJavaTypeNames(file *File) (pkg string, decls []TopLevelDecl) {
	if file == nil || file.FlatTree == nil || file.FlatTree.Len() == 0 {
		return "", nil
	}
	byName := map[string]int{}
	var sigs [][][]byte
	for child := file.FlatFirstChild(0); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "class_declaration", "interface_declaration", "enum_declaration", "record_declaration", "annotation_type_declaration":
		default:
			continue
		}
		name := file.FlatChildTextOrEmpty(child, "identifier")
		if name == "" {
			continue
		}
		whole := file.Content[file.FlatStartByte(child):file.FlatEndByte(child)]
		if i, ok := byName[name]; ok {
			sigs[i] = append(sigs[i], whole)
			continue
		}
		byName[name] = len(decls)
		decls = append(decls, TopLevelDecl{Name: name})
		sigs = append(sigs, [][]byte{whole})
	}
	foldSignatures(decls, sigs)
	sort.Slice(decls, func(i, j int) bool { return decls[i].Name < decls[j].Name })
	return javaPackageName(file), decls
}
