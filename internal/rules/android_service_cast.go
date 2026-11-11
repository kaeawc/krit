package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// serviceCastCheck reports a Kotlin `as` or Java cast of a getSystemService(X_SERVICE) result to a type other than the manager X_SERVICE returns.
func serviceCastCheck(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	call, castType := serviceCastOperands(file, idx)
	if call == 0 || castType == "" {
		return
	}
	svcConst := serviceCastConstant(file, call)
	expectedType, ok := serviceCastMap[svcConst]
	if !ok || castType == expectedType {
		return
	}
	// Gate on resolved call-target FQN; falls back to AST evidence only without an oracle.
	var oracleLookup oracle.Lookup
	if cr, ok := ctx.Resolver.(*oracle.CompositeResolver); ok {
		oracleLookup = cr.Oracle()
	}
	if !serviceCastOracleConfirmed(oracleLookup, file, call) {
		return
	}
	// Lint prints the JVM name of Kotlin's Any.
	if castType == "Any" {
		castType = "Object"
	}
	ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Suspicious cast to `"+castType+"` for a `"+svcConst+"`: expected `"+expectedType+"`")
}

// serviceCastOperands returns the getSystemService call being cast and the simple name of the cast type, or 0 and "".
func serviceCastOperands(file *scanner.File, idx uint32) (call uint32, castType string) {
	if file.FlatType(idx) == "cast_expression" {
		return serviceCastJavaOperands(file, idx)
	}
	// The first named child is the source expression; parentheses around it do not change the cast.
	call = file.FlatFirstChild(idx)
	for call != 0 && !file.FlatIsNamed(call) {
		call = file.FlatNextSib(call)
	}
	call = flatUnwrapParenExpr(file, call)
	if call == 0 || file.FlatType(call) != "call_expression" || flatCallExpressionName(file, call) != "getSystemService" {
		return 0, ""
	}
	// Target type of the cast is the last type_identifier of user_type.
	if userType, ok := file.FlatFindChild(idx, "user_type"); ok {
		if ident := flatLastChildOfType(file, userType, "type_identifier"); ident != 0 {
			castType = file.FlatNodeText(ident)
		}
	}
	return call, castType
}

func serviceCastJavaOperands(file *scanner.File, idx uint32) (call uint32, castType string) {
	count := file.FlatNamedChildCount(idx)
	if count < 2 {
		return 0, ""
	}
	typeNode := file.FlatNamedChild(idx, 0)
	switch file.FlatType(typeNode) {
	case "type_identifier", "scoped_type_identifier":
		text := strings.TrimSpace(file.FlatNodeText(typeNode))
		castType = text[strings.LastIndex(text, ".")+1:]
	default:
		return 0, ""
	}
	call = flatUnwrapParenExpr(file, file.FlatNamedChild(idx, count-1))
	if file.FlatType(call) != "method_invocation" || javaMethodInvocationName(file, call) != "getSystemService" {
		return 0, ""
	}
	return call, castType
}

// serviceCastConstant returns the final identifier of the first argument, such as WIFI_SERVICE for Context.WIFI_SERVICE.
func serviceCastConstant(file *scanner.File, call uint32) string {
	if file.FlatType(call) == "method_invocation" {
		args := javaArgumentExpressions(file, call)
		if len(args) == 0 {
			return ""
		}
		text := strings.TrimSpace(file.FlatNodeText(args[0]))
		return text[strings.LastIndex(text, ".")+1:]
	}
	args := flatCallKeyArguments(file, call)
	expr := flatValueArgumentExpression(file, flatPositionalValueArgument(file, args, 0))
	switch file.FlatType(expr) {
	case "simple_identifier":
		return file.FlatNodeText(expr)
	case "navigation_expression":
		return flatNavigationExpressionLastIdentifier(file, expr)
	}
	return ""
}
