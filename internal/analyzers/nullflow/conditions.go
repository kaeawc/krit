package nullflow

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// ConditionTrueProvesNonNull reports whether asserting that `cond` is true
// proves that `receiver` is non-null at useIdx. Handles equality checks
// (x != null), conjunction/disjunction normal forms, and `!`-negated forms.
// when_condition operands are recursed into.
func ConditionTrueProvesNonNull(file *scanner.File, cond, receiver, useIdx uint32) bool {
	cond = flatUnwrapParenExpr(file, cond)
	switch file.FlatType(cond) {
	case "when_condition":
		for i := 0; i < file.FlatNamedChildCount(cond); i++ {
			if ConditionTrueProvesNonNull(file, file.FlatNamedChild(cond, i), receiver, useIdx) {
				return true
			}
		}
	case "equality_expression":
		nonNull, _, ok := equalityNullFacts(file, cond, receiver, useIdx)
		return ok && nonNull
	case "conjunction_expression":
		return anyConditionOperand(file, cond, func(child uint32) bool {
			return ConditionTrueProvesNonNull(file, child, receiver, useIdx)
		})
	case "disjunction_expression":
		return allConditionOperands(file, cond, func(child uint32) bool {
			return ConditionTrueProvesNonNull(file, child, receiver, useIdx)
		})
	case "prefix_expression":
		if prefixExpressionIsNot(file, cond) {
			return ConditionFalseProvesNonNull(file, flatLastNamedChild(file, cond), receiver, useIdx)
		}
	}
	return false
}

// ConditionTrueProvesNull is the dual of ConditionTrueProvesNonNull.
func ConditionTrueProvesNull(file *scanner.File, cond, receiver, useIdx uint32) bool {
	cond = flatUnwrapParenExpr(file, cond)
	switch file.FlatType(cond) {
	case "equality_expression":
		_, isNull, ok := equalityNullFacts(file, cond, receiver, useIdx)
		return ok && isNull
	case "conjunction_expression":
		return anyConditionOperand(file, cond, func(child uint32) bool {
			return ConditionTrueProvesNull(file, child, receiver, useIdx)
		})
	case "disjunction_expression":
		return allConditionOperands(file, cond, func(child uint32) bool {
			return ConditionTrueProvesNull(file, child, receiver, useIdx)
		})
	case "prefix_expression":
		if prefixExpressionIsNot(file, cond) {
			return ConditionFalseProvesNull(file, flatLastNamedChild(file, cond), receiver, useIdx)
		}
	}
	return false
}

// ConditionFalseProvesNonNull reports whether asserting that `cond` is false
// proves that `receiver` is non-null at useIdx. Recognizes call-form null
// predicates like `isNullOrEmpty()` / `isNullOrBlank()` / `TextUtils.isEmpty()`.
func ConditionFalseProvesNonNull(file *scanner.File, cond, receiver, useIdx uint32) bool {
	cond = flatUnwrapParenExpr(file, cond)
	switch file.FlatType(cond) {
	case "equality_expression":
		_, isNull, ok := equalityNullFacts(file, cond, receiver, useIdx)
		return ok && isNull
	case "call_expression":
		return nullPredicateCallFalseProvesNonNull(file, cond, receiver, useIdx)
	case "disjunction_expression":
		return anyConditionOperand(file, cond, func(child uint32) bool {
			return ConditionFalseProvesNonNull(file, child, receiver, useIdx)
		})
	case "conjunction_expression":
		return allConditionOperands(file, cond, func(child uint32) bool {
			return ConditionFalseProvesNonNull(file, child, receiver, useIdx)
		})
	case "prefix_expression":
		if prefixExpressionIsNot(file, cond) {
			return ConditionTrueProvesNonNull(file, flatLastNamedChild(file, cond), receiver, useIdx)
		}
	}
	return false
}

// ConditionFalseProvesNull is the dual of ConditionFalseProvesNonNull.
func ConditionFalseProvesNull(file *scanner.File, cond, receiver, useIdx uint32) bool {
	cond = flatUnwrapParenExpr(file, cond)
	switch file.FlatType(cond) {
	case "equality_expression":
		nonNull, _, ok := equalityNullFacts(file, cond, receiver, useIdx)
		return ok && nonNull
	case "conjunction_expression":
		return allConditionOperands(file, cond, func(child uint32) bool {
			return ConditionFalseProvesNull(file, child, receiver, useIdx)
		})
	case "disjunction_expression":
		return anyConditionOperand(file, cond, func(child uint32) bool {
			return ConditionFalseProvesNull(file, child, receiver, useIdx)
		})
	case "prefix_expression":
		if prefixExpressionIsNot(file, cond) {
			return ConditionTrueProvesNull(file, flatLastNamedChild(file, cond), receiver, useIdx)
		}
	}
	return false
}

func equalityNullFacts(file *scanner.File, expr, receiver, useIdx uint32) (nonNull bool, isNull bool, ok bool) {
	if file == nil || expr == 0 || file.FlatType(expr) != "equality_expression" || file.FlatChildCount(expr) < 3 {
		return false, false, false
	}
	left := flatUnwrapParenExpr(file, file.FlatChild(expr, 0))
	op := file.FlatChild(expr, 1)
	right := flatUnwrapParenExpr(file, file.FlatChild(expr, 2))
	if left == 0 || op == 0 || right == 0 {
		return false, false, false
	}

	var candidate uint32
	switch {
	case flatIsNullLiteral(file, right):
		candidate = left
	case flatIsNullLiteral(file, left):
		candidate = right
	default:
		return false, false, false
	}
	if !conditionReferenceMatchesReceiver(file, candidate, receiver, useIdx) {
		return false, false, false
	}

	switch strings.TrimSpace(file.FlatNodeText(op)) {
	case "!=":
		return true, false, true
	case "==":
		return false, true, true
	default:
		return false, false, false
	}
}

func nullPredicateCallFalseProvesNonNull(file *scanner.File, call, receiver, useIdx uint32) bool {
	navExpr, args := flatCallExpressionParts(file, call)
	if navExpr == 0 {
		return false
	}
	path, ok := FlatReferencePathFromExpr(file, navExpr)
	if !ok || len(path.Parts) == 0 {
		return false
	}
	callee := path.Parts[len(path.Parts)-1]
	switch callee {
	case "isNullOrEmpty", "isNullOrBlank":
		if len(path.Parts) < 2 {
			return false
		}
		receiverExpr := file.FlatNamedChild(navExpr, 0)
		return conditionReferenceMatchesReceiver(file, receiverExpr, receiver, useIdx)
	case "isEmpty":
		if len(path.Parts) != 2 || path.Parts[0] != "TextUtils" || args == 0 {
			return false
		}
		firstArg := flatPositionalValueArgument(file, args, 0)
		if firstArg == 0 {
			return false
		}
		return conditionReferenceMatchesReceiver(file, flatValueArgumentExpression(file, firstArg), receiver, useIdx)
	default:
		return false
	}
}

func conditionReferenceMatchesReceiver(file *scanner.File, candidate, receiver, useIdx uint32) bool {
	candidate = flatUnwrapParenExpr(file, candidate)
	receiver = flatUnwrapParenExpr(file, receiver)
	if file.FlatNodeTextEquals(candidate, file.FlatNodeText(receiver)) &&
		stableRepeatedNullCheckReceiverText(file.FlatNodeText(receiver)) {
		return true
	}
	candPath, candOK := FlatReferencePathFromExpr(file, candidate)
	recvPath, recvOK := FlatReferencePathFromExpr(file, receiver)
	if !candOK || !recvOK {
		return false
	}
	if referencePathsMatchReceiver(file, candPath, recvPath, useIdx) {
		return true
	}
	candTrimmed, candHadThis := trimLeadingThisPath(candPath)
	recvTrimmed, recvHadThis := trimLeadingThisPath(recvPath)
	if !candHadThis && !recvHadThis {
		return false
	}
	return referencePathsMatchReceiver(file, candTrimmed, recvTrimmed, useIdx) &&
		sameExplicitThisReferenceTarget(file, candPath, recvPath, useIdx)
}
