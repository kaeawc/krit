package rules

import (
	"fmt"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// ---------------------------------------------------------------------------
// NullCheckOnMutablePropertyRule detects null check on var property.
// ---------------------------------------------------------------------------
type NullCheckOnMutablePropertyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence — distinguishing
// val vs var requires type resolution of the receiver. Heuristic fallback
// uses declaration patterns. Classified per roadmap/17.
func (r *NullCheckOnMutablePropertyRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *NullCheckOnMutablePropertyRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	operand, _, ok := flatNullComparisonOperand(file, idx)
	if !ok {
		return
	}
	operand = flatUnwrapParenExpr(file, operand)
	if file.FlatType(operand) != "simple_identifier" {
		return
	}
	name := file.FlatNodeString(operand, nil)
	if name == "" || !sameOwnerVarPropertyDeclaration(file, operand, name) {
		return
	}
	if ctx.Resolver != nil {
		resolved := ctx.Resolver.ResolveByNameFlat(name, operand, file)
		if resolved != nil && resolved.Kind != typeinfer.TypeUnknown && !resolved.IsNullable() {
			return
		}
	}
	ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		fmt.Sprintf("Null check on mutable property '%s'. The value may change between the check and the use.", name)))
}

func sameOwnerVarPropertyDeclaration(file *scanner.File, ref uint32, name string) bool {
	if file == nil || ref == 0 || name == "" {
		return false
	}
	refOwner := flatSemanticOwner(file, ref)
	refClass := flatEnclosingClassLike(file, ref)
	found := false
	file.FlatWalkNodes(0, "property_declaration", func(prop uint32) {
		propOwner := flatSemanticOwner(file, prop)
		if found || (propOwner != refOwner && propOwner != refClass) {
			return
		}
		if propertyDeclarationName(file, prop) == name && propertyDeclarationIsVar(file, prop) {
			found = true
		}
	})
	return found
}

// ---------------------------------------------------------------------------
// NullableToStringCallRule detects .toString() on nullable.
// ---------------------------------------------------------------------------
type NullableToStringCallRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence — needs resolver
// to know whether the receiver is nullable; heuristic fallback matches
// common null-returning APIs. Classified per roadmap/17.
func (r *NullableToStringCallRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *NullableToStringCallRule) NodeTypes() []string {
	return []string{"call_expression", "string_literal"}
}

func (r *NullableToStringCallRule) check(ctx *api.Context) {
	if ctx.Resolver == nil {
		return
	}
	switch ctx.File.FlatType(ctx.Idx) {
	case "call_expression":
		r.checkNullableToStringCall(ctx)
	case "string_literal":
		r.checkNullableStringTemplate(ctx)
	}
}

func (r *NullableToStringCallRule) checkNullableToStringCall(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if flatCallExpressionName(file, idx) != "toString" {
		return
	}
	nav, args := flatCallExpressionParts(file, idx)
	if nav == 0 || flatCallHasArguments(file, args) {
		return
	}
	if flatNavigationLastSuffixOperator(file, nav) == "?." {
		return
	}
	receiver := flatUnwrapParenExpr(file, flatNullCheckNavigationReceiver(file, nav))
	receiverType, ok := flatKnownResolvedType(file, ctx.Resolver, receiver)
	if !ok || !receiverType.IsNullable() {
		return
	}
	confidence, ok := flatNullableToStringCallConfidence(ctx, idx, receiver)
	if !ok {
		return
	}
	if flatSameFileNullableToStringExtensionMatches(file, receiverType.Name) {
		return
	}

	finding := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Calling toString() on a nullable receiver may produce the string \"null\". Use a safe call with an explicit fallback instead.")
	finding.Confidence = confidence
	ctx.Emit(finding)
}

func (r *NullableToStringCallRule) checkNullableStringTemplate(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		expr := flatStringTemplateInterpolationExpression(file, child)
		if expr == 0 {
			continue
		}
		if !flatTemplateExpressionIsResolvedNullable(file, ctx.Resolver, expr) {
			continue
		}
		ctx.Emit(r.Finding(file, file.FlatRow(child)+1, file.FlatCol(child)+1,
			"Interpolating a nullable expression may produce the string \"null\". Use an explicit fallback instead."))
	}
}

// ExpressionPositions collects resolver query positions for both rule
// paths in a single FlatWalkAllNodes pass: zero-arg `.toString()` call
// receivers (not safe-call) and string-template interpolation
// expressions. Single walk avoids three sequential AST scans at
// --depth=thorough.
func (r *NullableToStringCallRule) ExpressionPositions(file *scanner.File) []uint32 {
	if file == nil {
		return nil
	}
	var out []uint32
	file.FlatWalkAllNodes(0, func(idx uint32) {
		switch file.FlatType(idx) {
		case "call_expression":
			if flatCallExpressionName(file, idx) != "toString" {
				return
			}
			nav, args := flatCallExpressionParts(file, idx)
			if nav == 0 || flatCallHasArguments(file, args) {
				return
			}
			if flatNavigationLastSuffixOperator(file, nav) == "?." {
				return
			}
			receiver := flatUnwrapParenExpr(file, flatNullCheckNavigationReceiver(file, nav))
			if receiver == 0 {
				return
			}
			switch file.FlatType(receiver) {
			case "simple_identifier", "this_expression", "call_expression", "navigation_expression":
				out = append(out, receiver)
			}
		case "interpolated_identifier":
			out = append(out, idx)
		case "interpolated_expression":
			if file.FlatNamedChildCount(idx) == 0 {
				return
			}
			expr := flatUnwrapParenExpr(file, file.FlatNamedChild(idx, 0))
			if expr != 0 {
				out = append(out, expr)
			}
		}
	})
	return out
}

func flatCallHasArguments(file *scanner.File, args uint32) bool {
	return file != nil && args != 0 && file.FlatNamedChildCount(args) > 0
}

func flatNullableToStringCallConfidence(ctx *api.Context, call uint32, receiver uint32) (float64, bool) {
	target, ok := semantics.ResolveCallTarget(ctx, call)
	if !ok {
		return 0, false
	}
	if target.Resolved {
		if target.QualifiedName == "kotlin.toString" || target.QualifiedName == "kotlin.Any.toString" {
			return 0.95, true
		}
		return 0, false
	}
	if _, hasOracle := ctx.Resolver.(*oracle.CompositeResolver); hasOracle {
		return 0, false
	}
	if !flatNullableToStringHasLocalEvidence(ctx.File, ctx.Resolver, receiver) {
		return 0, false
	}
	return 0.80, true
}

func flatNullableToStringHasLocalEvidence(file *scanner.File, resolver typeinfer.TypeResolver, receiver uint32) bool {
	if file == nil || resolver == nil || receiver == 0 {
		return false
	}
	switch file.FlatType(receiver) {
	case "simple_identifier", "this_expression":
		return flatReferenceHasSameFileTarget(file, receiver)
	case "call_expression":
		return flatCallHasResolvedTarget(file, resolver, receiver)
	case "navigation_expression":
		_, ok := flatNavigationResolvedMemberType(file, resolver, receiver)
		return ok
	default:
		return false
	}
}

func flatNavigationLastSuffixOperator(file *scanner.File, nav uint32) string {
	if file == nil || nav == 0 || file.FlatType(nav) != "navigation_expression" {
		return ""
	}
	var suffix uint32
	for child := file.FlatFirstChild(nav); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "navigation_suffix" {
			suffix = child
		}
	}
	for child := file.FlatFirstChild(suffix); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case ".", "?.":
			return file.FlatType(child)
		}
	}
	return ""
}

func flatSameFileNullableToStringExtensionMatches(file *scanner.File, receiverType string) bool {
	if file == nil || receiverType == "" {
		return false
	}
	matched := false
	file.FlatWalkAllNodes(0, func(candidate uint32) {
		if matched || file.FlatType(candidate) != "function_declaration" || flatFunctionName(file, candidate) != "toString" {
			return
		}
		name, nullable := flatFunctionReceiverTypeInfo(file, candidate)
		matched = nullable && (name == receiverType || name == "Any")
	})
	return matched
}

func flatFunctionReceiverTypeInfo(file *scanner.File, fn uint32) (string, bool) {
	if file == nil || fn == 0 || file.FlatType(fn) != "function_declaration" {
		return "", false
	}
	var name string
	var nullable bool
	for child := file.FlatFirstChild(fn); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "nullable_type":
			nullable = true
			name = flatLastIdentifierInNode(file, child)
		case "user_type", "type_identifier":
			if name == "" {
				name = flatLastIdentifierInNode(file, child)
			}
		case ".":
			return name, nullable
		case "function_value_parameters", "function_body":
			return "", false
		}
	}
	return "", false
}

func flatStringTemplateInterpolationExpression(file *scanner.File, idx uint32) uint32 {
	if file == nil || idx == 0 {
		return 0
	}
	switch file.FlatType(idx) {
	case "interpolated_identifier":
		return idx
	case "interpolated_expression", "line_string_expression", "multi_line_string_expression":
		if file.FlatNamedChildCount(idx) > 0 {
			return file.FlatNamedChild(idx, 0)
		}
	}
	return 0
}

func flatTemplateExpressionIsResolvedNullable(file *scanner.File, resolver typeinfer.TypeResolver, expr uint32) bool {
	if file == nil || resolver == nil || expr == 0 {
		return false
	}
	if file.FlatType(expr) == "interpolated_identifier" {
		resolved := resolver.ResolveByNameFlat(file.FlatNodeString(expr, nil), expr, file)
		return resolved != nil && resolved.Kind != typeinfer.TypeUnknown && resolved.IsNullable()
	}
	resolved, ok := flatKnownResolvedType(file, resolver, flatUnwrapParenExpr(file, expr))
	return ok && resolved.IsNullable()
}
