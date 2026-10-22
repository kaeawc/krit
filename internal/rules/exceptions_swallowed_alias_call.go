package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func swallowedPropertyDeclarationName(file *scanner.File, prop uint32) string {
	if decl, ok := file.FlatFindChild(prop, "variable_declaration"); ok {
		if ident, ok := file.FlatFindChild(decl, "simple_identifier"); ok {
			return file.FlatNodeString(ident, nil)
		}
	}
	return ""
}

func swallowedPropertyInitializer(file *scanner.File, prop uint32) uint32 {
	afterEquals := false
	for child := file.FlatFirstChild(prop); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "=" {
			afterEquals = true
			continue
		}
		if afterEquals && file.FlatIsNamed(child) {
			return child
		}
	}
	return 0
}

func swallowedExpressionAliasKind(file *scanner.File, expr uint32, directAliases, derivedAliases map[string]bool) swallowedAliasKind {
	expr = swallowedUnwrapExpression(file, expr)
	if expr == 0 {
		return swallowedAliasNone
	}
	if file.FlatType(expr) == "simple_identifier" {
		name := file.FlatNodeString(expr, nil)
		if directAliases[name] {
			return swallowedAliasDirect
		}
		if derivedAliases[name] {
			return swallowedAliasDerived
		}
	}
	if swallowedSubtreeReferencesCaught(file, expr, directAliases, derivedAliases) {
		return swallowedAliasDerived
	}
	return swallowedAliasNone
}

// swallowedExpressionIsCauseUnwrap reports whether expr is `<alias>.cause` or
// `<alias>.getCause()` where <alias> is a known direct alias of the caught
// exception. Such an expression yields the original throwable's cause (still a
// Throwable), so a variable bound to it forwards the exception chain.
func swallowedExpressionIsCauseUnwrap(file *scanner.File, expr uint32, directAliases map[string]bool) bool {
	expr = swallowedUnwrapExpression(file, expr)
	if expr == 0 {
		return false
	}
	// `e.getCause()` — a call whose callee is a navigation ending in getCause.
	if file.FlatType(expr) == "call_expression" {
		for child := file.FlatFirstChild(expr); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "navigation_expression" {
				expr = child
				break
			}
		}
	}
	if file.FlatType(expr) != "navigation_expression" {
		return false
	}
	receiver := file.FlatFirstChild(expr)
	if receiver == 0 || file.FlatType(receiver) != "simple_identifier" {
		return false
	}
	if !directAliases[file.FlatNodeString(receiver, nil)] {
		return false
	}
	for child := file.FlatFirstChild(expr); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "navigation_suffix" {
			continue
		}
		for gc := file.FlatFirstChild(child); gc != 0; gc = file.FlatNextSib(gc) {
			if file.FlatType(gc) == "simple_identifier" {
				name := file.FlatNodeString(gc, nil)
				return name == "cause" || name == "getCause"
			}
		}
	}
	return false
}

func swallowedUnwrapExpression(file *scanner.File, expr uint32) uint32 {
	for expr != 0 {
		switch file.FlatType(expr) {
		case "parenthesized_expression":
			if file.FlatNamedChildCount(expr) != 1 {
				return expr
			}
			expr = file.FlatNamedChild(expr, 0)
		default:
			return expr
		}
	}
	return 0
}

func swallowedAnalyzeJump(file *scanner.File, node uint32, directAliases, derivedAliases map[string]bool) swallowedEvidence {
	switch {
	case swallowedIsThrowExpression(file, node):
		return swallowedAnalyzeThrow(file, node, directAliases, derivedAliases)
	case swallowedIsReturnExpression(file, node):
		return swallowedAnalyzeReturn(file, node, directAliases, derivedAliases)
	default:
		return swallowedEvidence{}
	}
}

func swallowedAnalyzeThrow(file *scanner.File, node uint32, directAliases, derivedAliases map[string]bool) swallowedEvidence {
	expr := swallowedJumpExpressionValue(file, node)
	if swallowedExpressionIsDirectAlias(file, expr, directAliases) {
		return swallowedEvidence{handled: true}
	}
	if file.FlatType(expr) == "call_expression" {
		args := flatCallKeyArguments(file, expr)
		argUse := swallowedAnalyzeArguments(file, args, directAliases, derivedAliases)
		if argUse.handled {
			return swallowedEvidence{handled: true}
		}
		if argUse.indirectOnly {
			return swallowedEvidence{indirectOnly: true}
		}
	}
	if swallowedExpressionAliasKind(file, expr, directAliases, derivedAliases) == swallowedAliasDerived {
		return swallowedEvidence{indirectOnly: true}
	}
	return swallowedEvidence{}
}

func swallowedAnalyzeReturn(file *scanner.File, node uint32, directAliases, derivedAliases map[string]bool) swallowedEvidence {
	expr := swallowedJumpExpressionValue(file, node)
	if swallowedExpressionIsDirectAlias(file, expr, directAliases) {
		return swallowedEvidence{handled: true}
	}
	if file.FlatType(expr) == "call_expression" {
		args := flatCallKeyArguments(file, expr)
		argUse := swallowedAnalyzeArguments(file, args, directAliases, derivedAliases)
		if argUse.handled || argUse.indirectOnly {
			return swallowedEvidence{handled: true}
		}
	}
	if swallowedExpressionAliasKind(file, expr, directAliases, derivedAliases) != swallowedAliasNone {
		return swallowedEvidence{handled: true}
	}
	return swallowedEvidence{}
}

func swallowedIsThrowExpression(file *scanner.File, node uint32) bool {
	if file.FlatType(node) != "jump_expression" {
		return false
	}
	for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
		return file.FlatType(child) == "throw"
	}
	return false
}

func swallowedIsReturnExpression(file *scanner.File, node uint32) bool {
	if file.FlatType(node) != "jump_expression" {
		return false
	}
	for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
		return swallowedIsReturnKeyword(file.FlatType(child))
	}
	return false
}

// swallowedIsReturnKeyword matches both bare and labeled return tokens
// (`return` and `return@label`, the latter parsed as a `return@` token).
func swallowedIsReturnKeyword(t string) bool {
	return t == "return" || t == "return@"
}

func swallowedJumpExpressionValue(file *scanner.File, node uint32) uint32 {
	seenKeyword := false
	for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
		if !seenKeyword {
			t := file.FlatType(child)
			seenKeyword = t == "throw" || swallowedIsReturnKeyword(t)
			continue
		}
		// Skip the `label` node that follows a `return@` token; the returned
		// value is the first *other* named child.
		if file.FlatType(child) == "label" {
			continue
		}
		if file.FlatIsNamed(child) {
			return child
		}
	}
	return 0
}

func (r *SwallowedExceptionRule) swallowedAnalyzeCall(ctx *api.Context, catchNode, node uint32, directAliases, derivedAliases map[string]bool) swallowedEvidence {
	file := ctx.File
	kind := swallowedClassifyCall(ctx, catchNode, node)
	if kind == swallowedCallUnknown {
		// Converting the whole caught exception into a value — e.g.
		// `out.append(convertThrowableToString(t))`, `return Result.Failure(e)`,
		// `BackupResponse.ApplicationError(e)` — is meaningful handling: the
		// exception is transformed into something rather than dropped. The
		// exception is treated as handled when EITHER:
		//   * the call's result is consumed (argument, return/throw value,
		//     assignment RHS, navigation receiver, expression-position catch
		//     tail). Uppercase constructor/factory names do not make a discarded
		//     result meaningful on their own.
		// Two shapes are deliberately excluded so genuine swallows still fire:
		//   * a logging-shaped callee on an unrecognized receiver
		//     (`localLog.warn(e)`) — a logger lookalike that drops the cause;
		//   * a bare lowercase call whose result is discarded (`ignored(e)`,
		//     `recover(e)`) — a fire-and-forget callback that may still swallow.
		callee, _ := swallowedCallTarget(file, node)
		if callee != "" && !swallowedLoggingCallee(callee) && swallowedCallResultIsConsumed(file, node) {
			args := flatCallKeyArguments(file, node)
			if swallowedAnalyzeArguments(file, args, directAliases, derivedAliases).handled {
				return swallowedEvidence{handled: true}
			}
		}
		return swallowedEvidence{}
	}
	args := flatCallKeyArguments(file, node)
	argUse := swallowedAnalyzeArguments(file, args, directAliases, derivedAliases)
	if kind == swallowedCallUI {
		if argUse.handled || argUse.indirectOnly {
			return swallowedEvidence{handled: true}
		}
		return swallowedEvidence{}
	}
	if kind == swallowedCallLogging {
		if !r.LoggingCountsAsHandling {
			return swallowedEvidence{}
		}
		if argUse.handled || argUse.indirectOnly || swallowedLoggingCallMentionsCaughtText(file, node, directAliases, derivedAliases) {
			return swallowedEvidence{handled: true}
		}
		return swallowedEvidence{}
	}
	if kind == swallowedCallLocalHandler {
		if argUse.handled {
			return swallowedEvidence{handled: true}
		}
		return swallowedEvidence{}
	}
	return argUse
}

type swallowedCallKind uint8

const (
	swallowedCallUnknown swallowedCallKind = iota
	swallowedCallLogging
	swallowedCallUI
	swallowedCallLocalHandler
)

func swallowedClassifyCall(ctx *api.Context, catchNode, node uint32) swallowedCallKind {
	file := ctx.File
	callee, receivers := swallowedCallTarget(file, node)
	if callee == "" {
		return swallowedCallUnknown
	}
	if swallowedIsQualifiedLoggingCall(file, callee, receivers) || swallowedReceiverTypeIsLogging(file, node) {
		return swallowedCallLogging
	}
	if swallowedIsQualifiedUICall(file, callee, receivers) || swallowedReceiverTypeIsUI(file, node) {
		return swallowedCallUI
	}
	if swallowedIsHandlerName(callee) &&
		(len(receivers) > 0 ||
			swallowedSameOwnerLocalFunctionExists(file, catchNode, callee) ||
			swallowedSameOwnerCallablePropertyExists(file, catchNode, callee)) {
		return swallowedCallLocalHandler
	}
	return swallowedCallUnknown
}

func swallowedCallTarget(file *scanner.File, node uint32) (string, []string) {
	core := node
	if flatCallExpressionName(file, core) == "" {
		for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "call_expression" {
				core = child
				break
			}
		}
	}
	callee := flatCallExpressionName(file, core)
	if callee == "" {
		return "", nil
	}
	navExpr, _ := flatCallExpressionParts(file, core)
	if navExpr == 0 {
		return callee, nil
	}
	parts := swallowedNavigationIdentifiers(file, navExpr)
	if len(parts) <= 1 {
		return callee, nil
	}
	return callee, parts[:len(parts)-1]
}

func swallowedNavigationIdentifiers(file *scanner.File, navExpr uint32) []string {
	var parts []string
	for child := file.FlatFirstChild(navExpr); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "simple_identifier":
			parts = append(parts, file.FlatNodeString(child, nil))
		case "navigation_suffix":
			for gc := file.FlatFirstChild(child); gc != 0; gc = file.FlatNextSib(gc) {
				if file.FlatType(gc) == "simple_identifier" {
					parts = append(parts, file.FlatNodeString(gc, nil))
				}
			}
		case "navigation_expression":
			parts = append(parts, swallowedNavigationIdentifiers(file, child)...)
		}
	}
	return parts
}
