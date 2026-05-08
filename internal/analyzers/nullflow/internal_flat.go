package nullflow

import (
	"strings"

	"github.com/kaeawc/krit/internal/analyzers/astflat"
	"github.com/kaeawc/krit/internal/scanner"
)

// Thin lowercase aliases to keep the existing call sites in this package
// readable. The actual implementation lives in internal/analyzers/astflat,
// which is the foundation tier shared across analyzer packages.
//
// Helpers that are heuristics (not pure AST navigation), like
// stableRepeatedNullCheckReceiverText and isDottedFieldChain, stay
// nullflow-internal because they are nullability-specific judgments.

func flatCallExpressionParts(file *scanner.File, idx uint32) (uint32, uint32) {
	return astflat.CallExpressionParts(file, idx)
}

func flatNavigationExpressionLastIdentifier(file *scanner.File, idx uint32) string {
	return astflat.NavigationExpressionLastIdentifier(file, idx)
}

func flatNavigationExpressionReceiver(file *scanner.File, nav uint32) uint32 {
	return astflat.NavigationExpressionReceiver(file, nav)
}

func flatNavigationLastSuffixHasSafeAccess(file *scanner.File, nav uint32) bool {
	return astflat.NavigationLastSuffixHasSafeAccess(file, nav)
}

func flatValueArgumentExpression(file *scanner.File, arg uint32) uint32 {
	return astflat.ValueArgumentExpression(file, arg)
}

func flatLastNamedChild(file *scanner.File, idx uint32) uint32 {
	return astflat.LastNamedChild(file, idx)
}

func flatUnwrapParenExpr(file *scanner.File, idx uint32) uint32 {
	return astflat.UnwrapParenExpr(file, idx)
}

func flatSingleValueArgumentExpression(file *scanner.File, args uint32) (uint32, bool) {
	return astflat.SingleValueArgumentExpression(file, args)
}

func flatPositionalValueArgument(file *scanner.File, args uint32, index int) uint32 {
	return astflat.PositionalValueArgument(file, args, index)
}

func flatCallSuffixValueArgs(file *scanner.File, suffix uint32) uint32 {
	return astflat.CallSuffixValueArgs(file, suffix)
}

func flatCallExpressionName(file *scanner.File, idx uint32) string {
	return astflat.CallExpressionName(file, idx)
}

func flatCallNameAny(file *scanner.File, idx uint32) string {
	return astflat.CallNameAny(file, idx)
}

func extractIdentifier(file *scanner.File, idx uint32) string {
	return astflat.ExtractIdentifier(file, idx)
}

func flatNodeWithin(file *scanner.File, container, node uint32) bool {
	return astflat.NodeWithin(file, container, node)
}

func flatEnclosingAncestor(file *scanner.File, idx uint32, types ...string) (uint32, bool) {
	return astflat.EnclosingAncestor(file, idx, types...)
}

func ifConditionThenElseBodies(file *scanner.File, node uint32) (uint32, uint32, uint32) {
	return astflat.IfConditionThenElseBodies(file, node)
}

func finalSimpleIdentifier(file *scanner.File, idx uint32) string {
	return astflat.FinalSimpleIdentifier(file, idx)
}

func prefixExpressionIsNot(file *scanner.File, idx uint32) bool {
	return astflat.PrefixExpressionIsNot(file, idx)
}

func anyConditionOperand(file *scanner.File, idx uint32, predicate func(uint32) bool) bool {
	return astflat.AnyConditionOperand(file, idx, predicate)
}

func allConditionOperands(file *scanner.File, idx uint32, predicate func(uint32) bool) bool {
	return astflat.AllConditionOperands(file, idx, predicate)
}

func flatIsNullLiteral(file *scanner.File, idx uint32) bool {
	return astflat.IsNullLiteral(file, idx)
}

// Heuristic helpers that interpret receiver text — these stay in nullflow
// because they encode nullability-rule policy, not AST primitives.

func isDottedFieldChain(s string) bool {
	if !strings.Contains(s, ".") {
		return false
	}
	if strings.ContainsAny(s, "()[]") {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '.' || c == '_' ||
			(c >= '0' && c <= '9') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= 'a' && c <= 'z') {
			continue
		}
		return false
	}
	return true
}

func stableRepeatedNullCheckReceiverText(text string) bool {
	if text == "" {
		return false
	}
	simple := true
	for _, c := range text {
		if c == '_' || (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			continue
		}
		simple = false
		break
	}
	return simple || isDottedFieldChain(text) || strings.Contains(text, ".group(")
}
