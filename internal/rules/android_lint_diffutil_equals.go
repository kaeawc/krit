package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const diffUtilItemCallback = "androidx.recyclerview.widget.DiffUtil.ItemCallback"

// DiffUtilEqualsRule mirrors the checks of Lint's DiffUtilEquals that need no class model: identity comparison
// (`===` in Kotlin, `==` on references in Java) and `equals` or `==` on a type that does not implement equals,
// inside DiffUtil.ItemCallback.areContentsTheSame. A type is known to lack equals only when this file declares it.
type DiffUtilEqualsRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: operand types come from declarations in the file.
func (r *DiffUtilEqualsRule) Confidence() float64 { return api.ConfidenceMedium }

var javaPrimitiveTypes = map[string]bool{
	"int": true, "long": true, "short": true, "byte": true, "char": true, "boolean": true, "float": true, "double": true,
}

func (r *DiffUtilEqualsRule) check(ctx *api.Context) {
	file := ctx.File
	if !diffUtilInContentsMethod(ctx, ctx.Idx) {
		return
	}
	switch file.FlatType(ctx.Idx) {
	case "equality_expression", "binary_expression":
		r.checkComparison(ctx)
	default:
		r.checkEqualsCall(ctx)
	}
}

func (r *DiffUtilEqualsRule) checkComparison(ctx *api.Context) {
	file := ctx.File
	left, op, right := diffUtilOperands(file, ctx.Idx)
	if left == 0 || right == 0 || diffUtilIsNull(file, left) || diffUtilIsNull(file, right) {
		return
	}
	opText := file.FlatNodeText(op)
	java := file.Language == scanner.LangJava
	switch {
	case !java && opText == "===":
		ctx.Emit(lintFindingAtNode(ctx, op, "", "Suspicious equality check: Did you mean `==` instead of `===` ?"))
	case java && opText == "==":
		if diffUtilIsReference(file, left) && diffUtilIsReference(file, right) {
			ctx.Emit(lintFindingAtNode(ctx, op, "", "Suspicious equality check: Did you mean `.equals()` instead of `==` ?"))
		}
	case !java && opText == "==":
		if name, lacks := diffUtilLacksEquals(file, left); lacks {
			ctx.Emit(lintFindingAtNode(ctx, op, "", "Suspicious equality check: `equals()` is not implemented in "+name))
		}
	}
}

func (r *DiffUtilEqualsRule) checkEqualsCall(ctx *api.Context) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || call.Name != "equals" || len(call.Args) != 1 || call.ReceiverNode == 0 {
		return
	}
	name, lacks := diffUtilLacksEquals(file, call.ReceiverNode)
	if !lacks {
		return
	}
	if at := lintCallNameNode(file, ctx.Idx); at != 0 {
		ctx.Emit(lintFindingAtNode(ctx, at, "", "Suspicious equality check: `equals()` is not implemented in "+diffUtilQualified(file, name)))
	}
}

// diffUtilInContentsMethod reports whether idx is inside areContentsTheSame of a DiffUtil.ItemCallback subclass.
func diffUtilInContentsMethod(ctx *api.Context, idx uint32) bool {
	file := ctx.File
	fn, ok := flatEnclosingCallable(file, idx)
	if !ok {
		return false
	}
	name := lintDeclaredMethodName(file, fn)
	if name == 0 || file.FlatNodeText(name) != "areContentsTheSame" || lintDeclaredParamCount(file, fn) != 2 {
		return false
	}
	cls, found := lintDirectClassOf(file, fn)
	return found && lintClassExtends(ctx, cls, diffUtilItemCallback, nil)
}

// diffUtilOperands splits a binary node into its operands and operator token.
func diffUtilOperands(file *scanner.File, idx uint32) (left, op, right uint32) {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch {
		case file.FlatIsNamed(child) && left == 0:
			left = child
		case file.FlatIsNamed(child):
			right = child
		case left != 0 && op == 0:
			op = child
		}
	}
	return left, op, right
}

func diffUtilIsNull(file *scanner.File, idx uint32) bool {
	switch file.FlatType(idx) {
	case "null_literal":
		return true
	case "parenthesized_expression":
		return diffUtilIsNull(file, flatUnwrapParenExpr(file, idx))
	}
	return strings.TrimSpace(file.FlatNodeText(idx)) == "null"
}

// diffUtilIsReference reports whether a Java operand is a plain name declared with a non-primitive type.
func diffUtilIsReference(file *scanner.File, idx uint32) bool {
	if file.FlatType(idx) != "identifier" {
		return false
	}
	typ := lintDeclaredTypeText(file, idx, file.FlatNodeText(idx))
	return typ != "" && !javaPrimitiveTypes[typ]
}

// diffUtilLacksEquals reports whether the operand is a plain name whose declared type is known not to implement
// equals, with the type's simple name.
func diffUtilLacksEquals(file *scanner.File, idx uint32) (string, bool) {
	switch file.FlatType(idx) {
	case "simple_identifier", "identifier":
	default:
		return "", false
	}
	typ := lintDeclaredTypeText(file, idx, file.FlatNodeText(idx))
	if typ == "" {
		return "", false
	}
	simple := typ[strings.LastIndex(typ, ".")+1:]
	if cls := lintSameFileClass(file, simple); cls != 0 {
		return simple, diffUtilClassMissingEquals(file, cls, 0)
	}
	// These interfaces and root classes declare no equals of their own in the Kotlin and Java type models.
	return simple, simple == "Any" || simple == "Object" || simple == "List"
}

// diffUtilClassMissingEquals reports whether a class declared in the file inherits Object.equals. Data, enum, sealed
// and value classes, records, and classes with an unknown superclass count as implementing it.
func diffUtilClassMissingEquals(file *scanner.File, cls uint32, depth int) bool {
	if depth > 5 {
		return false
	}
	for _, mod := range []string{"data", "enum", "sealed", "value", "inline", "annotation", "record"} {
		if file.FlatHasModifier(cls, mod) {
			return false
		}
	}
	if _, isEnum := file.FlatFindChild(cls, "enum_class_body"); isEnum || lintClassDeclaresMethod(file, cls, "equals") {
		return false
	}
	for _, super := range lintSupertypeNames(file, cls) {
		simple := super[strings.LastIndex(super, ".")+1:]
		if simple == "Any" || simple == "Object" {
			continue
		}
		parent := lintSameFileClass(file, simple)
		if parent == 0 || parent == cls || !diffUtilClassMissingEquals(file, parent, depth+1) {
			return false
		}
	}
	return true
}

// diffUtilQualified prefixes a same-file class with the file's package, as Lint prints it in equals messages.
func diffUtilQualified(file *scanner.File, name string) string {
	if pkg := sourcePackageName(file); pkg != "" && lintSameFileClass(file, name) != 0 {
		return pkg + "." + name
	}
	return name
}
