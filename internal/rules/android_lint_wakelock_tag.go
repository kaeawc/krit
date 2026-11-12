package rules

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const powerManagerClass = "android.os.PowerManager"

// InvalidWakeLockTagRule mirrors Lint's InvalidWakeLockTag: a constant tag passed to PowerManager.newWakeLock that is
// empty, reserved (`*name*`), or lacks a `prefix:` colon.
type InvalidWakeLockTagRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: the receiver type is inferred from declarations.
func (r *InvalidWakeLockTagRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *InvalidWakeLockTagRule) check(ctx *api.Context) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || call.Name != "newWakeLock" || len(call.Args) < 2 || call.ReceiverNode == 0 || !isPowerManagerReceiver(ctx, call) {
		return
	}
	tagNode := call.Arg(1, "tag")
	tag, known := lintEvalStringStrict(file, tagNode)
	if !known {
		return
	}
	if msg := wakeLockTagError(tag); msg != "" {
		ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", msg))
	}
}

// wakeLockTagError returns Lint's message for an invalid tag, or "" when the tag is acceptable.
func wakeLockTagError(tag string) string {
	switch {
	case tag == "":
		return "Tag name should not be empty to make wake lock problems easier to debug"
	case len(tag) >= 2 && tag[0] == '*' && strings.IndexByte(tag[1:], '*') >= 0:
		return fmt.Sprintf("`%s` is a reserved platform tag name and cannot be used", tag)
	case !strings.Contains(tag, ":"):
		return fmt.Sprintf("Tag name should use a unique prefix followed by a colon (found %s). For instance `myapp:mywakelocktag`. This will help with debugging", tag)
	}
	return ""
}

// isPowerManagerReceiver reports whether the call's receiver is a PowerManager, by resolved type, declared type, or
// an initializer whose cast or `getSystemService` type argument resolves to it.
func isPowerManagerReceiver(ctx *api.Context, call lintCall) bool {
	if lintReceiverIsType(ctx, call, powerManagerClass) {
		return true
	}
	file := ctx.File
	recv := flatUnwrapParenExpr(file, call.ReceiverNode)
	switch file.FlatType(recv) {
	case "simple_identifier", "identifier":
		init := lintInitializerOf(file, call.Idx, file.FlatNodeText(recv), true)
		return init != 0 && lintExprNamesType(file, init, powerManagerClass)
	}
	return lintExprNamesType(file, recv, powerManagerClass)
}

// lintExprNamesType reports whether expr is a cast to fqn (`x as T`, `x as? T`, `(T) x`) or a `getSystemService`
// call whose class or type argument is fqn (`getSystemService<T>()`, `getSystemService(T::class.java)`,
// `getSystemService(T.class)`). Type names resolve through the file's imports.
func lintExprNamesType(file *scanner.File, expr uint32, fqn string) bool {
	expr = flatUnwrapParenExpr(file, expr)
	switch file.FlatType(expr) {
	case "as_expression":
		typ := flatLastNamedChild(file, expr)
		return typ != 0 && lintTypeRefMatches(file, strings.TrimSuffix(file.FlatNodeText(typ), "?"), fqn)
	case "cast_expression":
		typ := file.FlatNamedChild(expr, 0)
		return typ != 0 && lintTypeRefMatches(file, file.FlatNodeText(typ), fqn)
	case "call_expression", "method_invocation":
		call, ok := lintCallAt(file, expr)
		if !ok || call.Name != "getSystemService" {
			return false
		}
		if typeArgs := callExpressionTypeArgumentsFlat(file, expr); typeArgs != 0 {
			proj := file.FlatNamedChild(typeArgs, 0)
			return proj != 0 && lintTypeRefMatches(file, file.FlatNodeText(proj), fqn)
		}
		if len(call.Args) != 1 {
			return false
		}
		return lintTypeRefMatches(file, classLiteralTypeText(file, call.Args[0]), fqn)
	}
	return false
}

// classLiteralTypeText returns the class named by `T::class.java`, `T::class`, or Java's `T.class`, or "".
func classLiteralTypeText(file *scanner.File, idx uint32) string {
	idx = flatUnwrapParenExpr(file, idx)
	switch file.FlatType(idx) {
	case "navigation_expression":
		if flatNavigationExpressionLastIdentifier(file, idx) != "java" {
			return ""
		}
		return classLiteralTypeText(file, file.FlatNamedChild(idx, 0))
	case "callable_reference":
		text := file.FlatNodeText(idx)
		if typ, ok := strings.CutSuffix(text, "::class"); ok {
			return typ
		}
	case "class_literal":
		if typ := file.FlatNamedChild(idx, 0); typ != 0 {
			return file.FlatNodeText(typ)
		}
	}
	return ""
}

// lintEvalStringStrict folds a string made only of constants: literals without interpolation, `+` concatenation, and
// declarations in this file that nothing reassigns. It reports false when any part is unknown.
func lintEvalStringStrict(file *scanner.File, idx uint32) (string, bool) {
	return lintEvalStringStrictDepth(file, idx, idx, 0)
}

func lintEvalStringStrictDepth(file *scanner.File, idx, scope uint32, depth int) (string, bool) {
	if idx == 0 || depth > 6 {
		return "", false
	}
	switch file.FlatType(idx) {
	case "string_literal":
		if flatContainsStringInterpolation(file, idx) {
			return "", false
		}
		return lintStringLiteralValue(file, idx), true
	case "parenthesized_expression":
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) {
				return lintEvalStringStrictDepth(file, child, scope, depth)
			}
		}
	case "additive_expression", "binary_expression":
		var sb strings.Builder
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			if !file.FlatIsNamed(child) {
				if file.FlatNodeText(child) != "+" {
					return "", false
				}
				continue
			}
			part, ok := lintEvalStringStrictDepth(file, child, scope, depth)
			if !ok {
				return "", false
			}
			sb.WriteString(part)
		}
		return sb.String(), true
	case "simple_identifier", "identifier":
		if init := lintInitializerOf(file, scope, file.FlatNodeText(idx), true); init != 0 && !lintNameAssignedInFile(file, file.FlatNodeText(idx)) {
			return lintEvalStringStrictDepth(file, init, init, depth+1)
		}
	}
	return "", false
}

// lintNameAssignedInFile reports whether any assignment or increment in the file targets name.
func lintNameAssignedInFile(file *scanner.File, name string) bool {
	return lintNameReassigned(file, 0, name)
}
