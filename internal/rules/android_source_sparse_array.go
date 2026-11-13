package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// sparseArrayAlternative maps the single type argument of a SparseArray constructor to the primitive
// specialization Android Lint suggests, or "" when none applies. A nullable Kotlin type counts as its box.
func sparseArrayAlternative(typeArg string) string {
	typeArg = strings.TrimSuffix(strings.Join(strings.Fields(typeArg), ""), "?")
	switch typeArg {
	case "Int", "Integer", "kotlin.Int", "java.lang.Integer":
		return "SparseIntArray"
	case "Boolean", "kotlin.Boolean", "java.lang.Boolean":
		return "SparseBooleanArray"
	}
	return ""
}

// sparseArrayImported reports whether a bare SparseArray in the file names android.util.SparseArray.
func sparseArrayImported(file *scanner.File) bool {
	for _, line := range file.Lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "import ") {
			continue
		}
		path := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(line, "import ")), ";"))
		if path == "android.util.SparseArray" || path == "android.util.*" {
			return true
		}
	}
	return false
}

// sparseArrayCheck reports SparseArray<Int|Boolean> constructor calls in Kotlin and Java.
func sparseArrayCheck(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	var typeArg string
	var name string
	if file.FlatType(idx) == "call_expression" {
		name, typeArg = sparseArrayKotlinCall(file, idx)
	} else {
		name, typeArg = sparseArrayJavaNew(file, idx)
	}
	alt := sparseArrayAlternative(typeArg)
	if alt == "" || !sparseArrayCallNamesFramework(file, name) {
		return
	}
	line, col := file.FlatRow(idx)+1, file.FlatCol(idx)+1
	if file.FlatType(idx) == "object_creation_expression" {
		ctx.EmitAt(line, col, "Use `new "+alt+"(...)` instead for better performance")
		return
	}
	ctx.EmitAt(line, col, "Use `"+alt+"(...)` instead for better performance")
}

// sparseArrayCallNamesFramework reports whether the written constructor name is android.util.SparseArray.
func sparseArrayCallNamesFramework(file *scanner.File, name string) bool {
	switch name {
	case "SparseArray":
		return sparseArrayImported(file)
	case "android.util.SparseArray":
		return true
	}
	return false
}

// sparseArrayKotlinCall returns the callee text and the only explicit type argument of `Callee<T>(...)`.
func sparseArrayKotlinCall(file *scanner.File, call uint32) (name, typeArg string) {
	if flatCallExpressionName(file, call) != "SparseArray" {
		return "", ""
	}
	suffix, _ := file.FlatFindChild(call, "call_suffix")
	if suffix == 0 {
		return "", ""
	}
	typeArgs, _ := file.FlatFindChild(suffix, "type_arguments")
	if typeArgs == 0 || file.FlatNamedChildCount(typeArgs) != 1 {
		return "", ""
	}
	callee := file.FlatFirstChild(call)
	return strings.Join(strings.Fields(file.FlatNodeText(callee)), ""), file.FlatNodeText(file.FlatNamedChild(typeArgs, 0))
}

// sparseArrayJavaNew returns the type name and only type argument of `new Type<T>(...)`. A diamond takes
// its argument from the declared type of the variable it initializes.
func sparseArrayJavaNew(file *scanner.File, newExpr uint32) (name, typeArg string) {
	generic, _ := file.FlatFindChild(newExpr, "generic_type")
	if generic == 0 {
		return "", ""
	}
	typeName := ""
	args := uint32(0)
	for child := file.FlatFirstChild(generic); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "type_identifier", "scoped_type_identifier":
			typeName = strings.Join(strings.Fields(file.FlatNodeText(child)), "")
		case "type_arguments":
			args = child
		}
	}
	if args == 0 || (typeName != "SparseArray" && typeName != "android.util.SparseArray") {
		return "", ""
	}
	switch file.FlatNamedChildCount(args) {
	case 1:
		return typeName, file.FlatNodeText(file.FlatNamedChild(args, 0))
	case 0:
		return typeName, sparseArrayDeclaredArgument(file, newExpr)
	}
	return "", ""
}

// sparseArrayDeclaredArgument reads T from `SparseArray<T> v = new SparseArray<>()`.
func sparseArrayDeclaredArgument(file *scanner.File, newExpr uint32) string {
	declarator, ok := file.FlatParent(newExpr)
	if !ok || file.FlatType(declarator) != "variable_declarator" {
		return ""
	}
	decl, ok := file.FlatParent(declarator)
	if !ok {
		return ""
	}
	generic, _ := file.FlatFindChild(decl, "generic_type")
	if generic == 0 {
		return ""
	}
	args, _ := file.FlatFindChild(generic, "type_arguments")
	if args == 0 || file.FlatNamedChildCount(args) != 1 {
		return ""
	}
	return file.FlatNodeText(file.FlatNamedChild(args, 0))
}
