package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// NotConstructorRule mirrors Lint's NotConstructor: an instance method named like its containing class, which reads
// like a constructor but is a normal method.
type NotConstructorRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-1 (high) base confidence: the check compares two declared names.
func (r *NotConstructorRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *NotConstructorRule) check(ctx *api.Context) {
	file := ctx.File
	name := lintDeclaredMethodName(file, ctx.Idx)
	if name == 0 {
		return
	}
	cls, ok := lintNamedMemberOwner(file, ctx.Idx)
	if !ok || file.FlatHasModifier(ctx.Idx, "static") {
		return
	}
	got := strings.Trim(file.FlatNodeText(name), "`")
	if want := lintOwnerName(file, cls); want == "" || got != want {
		return
	}
	start := lintDeclarationStart(file, ctx.Idx)
	line, col := file.FlatRow(start)+1, file.FlatCol(start)+1
	ctx.Emit(scanner.Finding{
		Line: line, Col: col, StartByte: int(file.FlatStartByte(start)), EndByte: int(file.FlatEndByte(ctx.Idx)),
		Message: "Method " + got + " looks like a constructor but is a normal method",
	})
}

// lintNamedMemberOwner returns the class, interface or enum that directly declares the method. Objects, companion
// objects, anonymous classes and Java records are not owners, and neither is a function nested in another one.
func lintNamedMemberOwner(file *scanner.File, method uint32) (uint32, bool) {
	body, ok := file.FlatParent(method)
	if !ok {
		return 0, false
	}
	owner, ok := file.FlatParent(body)
	if !ok {
		return 0, false
	}
	switch file.FlatType(body) {
	case "class_body", "enum_class_body", "interface_body":
	case "enum_body_declarations":
		if owner, ok = file.FlatParent(owner); !ok {
			return 0, false
		}
	default:
		return 0, false
	}
	switch file.FlatType(owner) {
	case "class_declaration", "interface_declaration", "enum_declaration":
		return owner, true
	}
	return 0, false
}

// lintOwnerName is the declared name of a class-like declaration, without Kotlin backticks.
func lintOwnerName(file *scanner.File, cls uint32) string {
	for child := file.FlatFirstChild(cls); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "type_identifier", "identifier":
			return strings.Trim(file.FlatNodeText(child), "`")
		}
	}
	return ""
}

// lintDeclarationStart is the first node of a declaration as Lint locates it: the doc comment directly above it when
// there is one, else the declaration itself.
func lintDeclarationStart(file *scanner.File, decl uint32) uint32 {
	prev, ok := file.FlatPrevSibling(decl)
	if !ok {
		return decl
	}
	switch file.FlatType(prev) {
	case "multiline_comment", "block_comment":
		if strings.HasPrefix(file.FlatNodeText(prev), "/**") {
			return prev
		}
	}
	return decl
}
