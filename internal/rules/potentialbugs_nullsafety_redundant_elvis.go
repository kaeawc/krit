package rules

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// UselessElvisOnNonNullRule detects `expr ?: fallback` where `expr` is
// already non-null. Resolves the left operand via ctx.Resolver, which under
// the KAA oracle recovers platform types and substituted generics (e.g.
// getOrDefault) that source-only inference cannot.
type UselessElvisOnNonNullRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *UselessElvisOnNonNullRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *UselessElvisOnNonNullRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	// Projection trusts the compiler's USELESS_ELVIS verdict, so it gates only
	// on the structural left operand — not uselessElvisOperand's resolver
	// heuristic, which refuses e.g. an imported non-local identifier. Gating
	// projection on that heuristic would drop a compiler-confirmed finding.
	if _, ok := uselessElvisLeftOperand(file, idx); ok && projectDiagnostic(ctx, DiagnosticProjection{
		RuleID:       "UselessElvisOnNonNull",
		FactoryNames: []string{"USELESS_ELVIS"},
		Message: func(ctx *api.Context, _ oracle.Diagnostic) string {
			left, _ := uselessElvisLeftOperand(ctx.File, ctx.Idx)
			leftText := strings.TrimSpace(ctx.File.FlatNodeText(left))
			return fmt.Sprintf("Useless elvis (?:) on non-nullable '%s'. The fallback is dead code.", leftText)
		},
		Fix: func(ctx *api.Context, _ oracle.Diagnostic) *scanner.Fix {
			left, _ := uselessElvisLeftOperand(ctx.File, ctx.Idx)
			return &scanner.Fix{
				ByteMode:    true,
				StartByte:   int(ctx.File.FlatEndByte(left)),
				EndByte:     int(ctx.File.FlatEndByte(ctx.Idx)),
				Replacement: "",
			}
		},
		Confidence: api.ConfidenceVeryHigh,
	}) {
		return
	}
	if ctx.Resolver == nil {
		return
	}
	left, operand, ok := uselessElvisOperand(file, idx)
	if !ok {
		return
	}
	resolved, ok := uselessElvisResolvedOperandType(file, ctx.Resolver, operand)
	if !ok || resolved == nil {
		return
	}
	if !resolved.Resolved || resolved.Kind == typeinfer.TypeUnknown || resolved.Kind == typeinfer.TypeGeneric {
		return
	}
	if resolved.IsNullable() {
		return
	}
	leftEnd := int(file.FlatEndByte(left))
	elvisEnd := int(file.FlatEndByte(idx))
	leftText := strings.TrimSpace(file.FlatNodeText(left))
	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		fmt.Sprintf("Useless elvis (?:) on non-nullable '%s'. The fallback is dead code.", leftText))
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   leftEnd,
		EndByte:     elvisEnd,
		Replacement: "",
	}
	ctx.Emit(f)
}

// uselessElvisResolvedOperandType resolves the elvis left operand's type, but
// only when the result is genuinely proven non-null rather than defaulted to
// non-null by source inference.
//
// For qualified calls and member-access navigations it requires a resolved
// same-file / stdlib target (mirroring UnnecessaryNotNullCheckRule's operand
// gate): source inference defaults an unresolved member call or property
// access to a non-null type, which would wrongly flag the elvis fallback as
// dead code (e.g. `external.create(x) ?: continue`, `decl.source ?: return`,
// `obj.missingMember ?: fallback`). Intrinsically non-null operand shapes
// (literals, bare identifiers with a same-file declaration, `this`, `x!!`,
// `x as T`) keep the direct resolver path, where the resolver's nullability
// verdict is trustworthy.
func uselessElvisResolvedOperandType(file *scanner.File, resolver typeinfer.TypeResolver, operand uint32) (*typeinfer.ResolvedType, bool) {
	if file == nil || resolver == nil || operand == 0 {
		return nil, false
	}
	switch file.FlatType(operand) {
	case "call_expression", "navigation_expression":
		return flatResolvedNullCheckOperandType(file, resolver, operand)
	default:
		resolved := resolver.ResolveFlatNode(operand, file)
		if resolved == nil {
			return nil, false
		}
		return resolved, true
	}
}

// ExpressionPositions enumerates left operands the rule wants resolved by
// the KAA pre-pass under --depth=thorough. Selector mirrors check's gates
// so the batched resolveExpressionTypes RPC stays tight.
func (r *UselessElvisOnNonNullRule) ExpressionPositions(file *scanner.File) []uint32 {
	if file == nil {
		return nil
	}
	var out []uint32
	file.FlatWalkNodes(0, "elvis_expression", func(idx uint32) {
		if _, operand, ok := uselessElvisOperand(file, idx); ok {
			out = append(out, operand)
		}
	})
	return out
}

// uselessElvisLeftOperand returns the elvis's left child with only a
// structural check (elvis shape + present left child). It applies none of
// uselessElvisOperand's resolver-heuristic gates (same-file target, skip
// shapes), because the projection path relies on the compiler's authoritative
// USELESS_ELVIS verdict rather than source inference — so it must still emit
// for imported / non-local operands the heuristic would refuse.
func uselessElvisLeftOperand(file *scanner.File, idx uint32) (left uint32, ok bool) {
	if file == nil || file.FlatType(idx) != "elvis_expression" || file.FlatChildCount(idx) < 3 {
		return 0, false
	}
	left = file.FlatChild(idx, 0)
	return left, left != 0
}

// uselessElvisOperand returns (left child, unwrapped operand) when the
// elvis at idx has a resolver-friendly shape and no obvious source-level
// disqualifier. ok=false means the rule should skip this elvis.
func uselessElvisOperand(file *scanner.File, idx uint32) (left, operand uint32, ok bool) {
	if file == nil || file.FlatType(idx) != "elvis_expression" || file.FlatChildCount(idx) < 3 {
		return 0, 0, false
	}
	left = file.FlatChild(idx, 0)
	if left == 0 {
		return 0, 0, false
	}
	operand = flatUnwrapParenExpr(file, left)
	if !uselessElvisOperandResolvable(file, operand) {
		return 0, 0, false
	}
	if uselessElvisOperandShouldSkip(file, operand) {
		return 0, 0, false
	}
	return left, operand, true
}

func uselessElvisOperandResolvable(file *scanner.File, operand uint32) bool {
	if file == nil || operand == 0 {
		return false
	}
	switch file.FlatType(operand) {
	case "string_literal", "line_string_literal", "multi_line_string_literal",
		"integer_literal", "long_literal", "real_literal",
		"boolean_literal", "character_literal":
		return true
	case "simple_identifier":
		return flatReferenceHasSameFileTarget(file, operand)
	case "this_expression":
		return true
	case "navigation_expression":
		return !flatExpressionChainShortCircuits(file, operand)
	case "call_expression":
		return !flatExpressionChainShortCircuits(file, operand)
	case "postfix_expression":
		return flatPostfixHasNotNullAssertion(file, operand)
	case "as_expression":
		// `x as T` is non-null and resolver-trustworthy; `x as? T` (safe cast)
		// is unconditionally nullable, so an elvis on it is never useless —
		// refuse it structurally regardless of what the type resolver says.
		return !flatAsExpressionIsSafeCast(file, operand)
	default:
		return false
	}
}

// flatExpressionIsIndexing reports whether the (paren-unwrapped) node at idx is
// an indexed access `receiver[key]`. For a `Map`/`MutableMap` receiver the
// `get` operator returns a nullable value, so `m[k]!!` / `m[k]?.x` are required,
// not redundant. Source/name-based resolution cannot tell a Map (nullable get)
// from a List/Array/String (non-null get), and the oracle's byte-range lookup
// can mis-attribute an inner sub-expression's type to the whole indexed access,
// so the redundant-null-safety rules skip indexed receivers rather than risk a
// false positive. The rare true positive (`list[i]!!`) is given up deliberately.
func flatExpressionIsIndexing(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	idx = flatUnwrapParenExpr(file, idx)
	return file.FlatType(idx) == "indexing_expression"
}

// flatAsExpressionIsSafeCast reports whether the as_expression at idx uses the
// safe-cast operator `as?` (result type is always nullable) rather than the
// plain `as` operator. The operator surfaces as a dedicated `as?` child node in
// the tree-sitter grammar. Callers pass an already paren-unwrapped node; for
// convenience a parenthesized_expression wrapping a safe cast is also detected.
func flatAsExpressionIsSafeCast(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	idx = flatUnwrapParenExpr(file, idx)
	if file.FlatType(idx) != "as_expression" {
		return false
	}
	for i := 0; i < file.FlatChildCount(idx); i++ {
		if file.FlatType(file.FlatChild(idx, i)) == "as?" {
			return true
		}
	}
	return false
}

// flatExpressionChainShortCircuits reports whether the static result of the
// expression at idx may be null because some `?.` in its receiver/callee
// spine can short-circuit the chain. Walking only the leftmost spine —
// rather than every descendant — avoids treating `?.` inside call argument
// lists (which do not propagate null to the call's return type) as a
// short-circuit. Used by UselessElvisOnNonNull to refuse operands like
// `obj?.foo()` and `obj?.foo().bar()` whose final return-type lookup hides
// an upstream null-propagating step.
//
// Each iteration strictly descends to a child node, so the loop terminates
// naturally at any leaf (default case). Self-loop guards on the descent
// step keep a malformed AST from spinning indefinitely.
func flatExpressionChainShortCircuits(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	for idx != 0 {
		switch file.FlatType(idx) {
		case "parenthesized_expression":
			inner := file.FlatNamedChild(idx, 0)
			if inner == 0 || inner == idx {
				return false
			}
			idx = inner
		case "navigation_expression":
			if flatNavigationSafeCallOperator(file, idx) != 0 {
				return true
			}
			recv := file.FlatNamedChild(idx, 0)
			if recv == 0 || recv == idx {
				return false
			}
			idx = recv
		case "call_expression":
			callee := file.FlatNamedChild(idx, 0)
			if callee == 0 || callee == idx {
				return false
			}
			idx = callee
		case "postfix_expression":
			// `!!` asserts non-null at this level and seals the chain.
			// Other postfix forms (++/--) do not appear in elvis operands.
			return false
		default:
			return false
		}
	}
	return false
}

// uselessElvisOperandShouldSkip guards against false positives the
// resolver alone cannot rule out: mutable `var` (no smart-cast across
// statements), framework-nullable inherited properties the source
// resolver would widen to non-null, and initializers that can produce
// null at runtime.
func uselessElvisOperandShouldSkip(file *scanner.File, operand uint32) bool {
	if file == nil || operand == 0 || file.FlatType(operand) != "simple_identifier" {
		return false
	}
	name := strings.TrimSpace(file.FlatNodeText(operand))
	if name == "" {
		return true
	}
	summary := nullSafetySummaryFor(file)
	return summary.mutableVar[name] ||
		summary.memberAccessInitializer[name] ||
		summary.uncertainCallInitializer[name] ||
		summary.branchNullInitializer[name] ||
		isFrameworkNullableProperty(name)
}
