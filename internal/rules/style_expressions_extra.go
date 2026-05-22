package rules

import (
	"fmt"
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// MultilineLambdaItParameterRule detects 'it' in multiline lambdas.
type MultilineLambdaItParameterRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection uses structural pattern matching on
// expressions; the suggested rewrite's readability is a style call.
// Classified per roadmap/17.
func (r *MultilineLambdaItParameterRule) Confidence() float64 { return api.ConfidenceMedium }

// MultilineRawStringIndentationRule checks raw string indentation.
type MultilineRawStringIndentationRule struct {
	FlatDispatchBase
	BaseRule
	IndentSize      int
	TrimmingMethods []string
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection uses structural pattern matching on
// expressions; the suggested rewrite's readability is a style call.
// Classified per roadmap/17.
func (r *MultilineRawStringIndentationRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *MultilineRawStringIndentationRule) check(ctx *api.Context) {
	if !isUntrimmedMultilineRawString(ctx, r.TrimmingMethods) {
		return
	}
	ctx.Emit(r.Finding(ctx.File, ctx.File.FlatRow(ctx.Idx)+1, ctx.File.FlatCol(ctx.Idx)+1,
		"Multiline raw string should use trimIndent() or trimMargin()."))
}

// TrimMultilineRawStringRule detects raw strings missing trimIndent/trimMargin.
type TrimMultilineRawStringRule struct {
	FlatDispatchBase
	BaseRule
	TrimmingMethods []string
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection uses structural pattern matching on
// expressions; the suggested rewrite's readability is a style call.
// Classified per roadmap/17.
func (r *TrimMultilineRawStringRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *TrimMultilineRawStringRule) check(ctx *api.Context) {
	if !isUntrimmedMultilineRawString(ctx, r.TrimmingMethods) {
		return
	}
	end := int(ctx.File.FlatEndByte(ctx.Idx))
	f := r.Finding(ctx.File, ctx.File.FlatRow(ctx.Idx)+1, ctx.File.FlatCol(ctx.Idx)+1,
		"Multiline raw string should use trimIndent() or trimMargin().")
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   end,
		EndByte:     end,
		Replacement: ".trimIndent()",
	}
	ctx.Emit(f)
}

func isUntrimmedMultilineRawString(ctx *api.Context, trimmingMethods []string) bool {
	if ctx.File == nil || !isRawStringLiteralNode(ctx.File, ctx.Idx) {
		return false
	}
	text := ctx.File.FlatNodeText(ctx.Idx)
	return strings.Contains(text, "\n") && !rawStringHasTrimCall(ctx.File, ctx.Idx, trimmingMethods)
}

func isRawStringLiteralNode(file *scanner.File, idx uint32) bool {
	switch file.FlatType(idx) {
	case "string_literal", "multi_line_string_literal":
		return strings.HasPrefix(file.FlatNodeText(idx), `"""`)
	default:
		return false
	}
}

func rawStringHasTrimCall(file *scanner.File, idx uint32, trimmingMethods []string) bool {
	end := int(file.FlatEndByte(idx))
	if end < 0 || end > len(file.Content) {
		return false
	}
	after := strings.TrimSpace(string(file.Content[end:]))
	for _, method := range rawStringTrimMethods(trimmingMethods) {
		if strings.HasPrefix(after, "."+method+"()") {
			return true
		}
	}
	return false
}

func rawStringTrimMethods(configured []string) []string {
	if len(configured) > 0 {
		return configured
	}
	return []string{"trimIndent", "trimMargin"}
}

// StringShouldBeRawStringRule detects strings with many escape characters.
type StringShouldBeRawStringRule struct {
	FlatDispatchBase
	BaseRule
	MaxEscapes int
}

var escapeCountRe = regexp.MustCompile(`\\[nrt"\\]`)

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection uses structural pattern matching on
// expressions; the suggested rewrite's readability is a style call.
// Classified per roadmap/17.
func (r *StringShouldBeRawStringRule) Confidence() float64 { return api.ConfidenceMedium }

// CanBeNonNullableRule detects nullable types that are never assigned null.
// Handles two cases:
// 1. Properties initialized with non-null values that are never reassigned to null.
// 2. Function parameters declared nullable but only used with !! (non-null assertion).
// Skips override/open/abstract functions, delegated properties, and properties with custom setters.
// Tracks null assignments through if/when branches and lambda bodies.
//
// Known limitations:
//   - Cannot track nullable assignments through function calls (fun setNull(x) { field = null })
//   - Cannot detect properties assigned null via reflection or Java interop
type CanBeNonNullableRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *CanBeNonNullableRule) SetResolver(_ typeinfer.TypeResolver) {}

// Confidence reports a tier-2 (medium) base confidence — detecting which
// nullable properties are never assigned null requires flow analysis; the
// fallback is a conservative heuristic. Classified per roadmap/17.
func (r *CanBeNonNullableRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *CanBeNonNullableRule) checkPropertyFlat(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	text := file.FlatNodeText(idx)
	if !strings.Contains(text, "?") {
		return
	}
	if file.FlatHasChildOfType(idx, "property_delegate") {
		return
	}
	if canBeNonNullableHasSetter(file, idx) {
		return
	}
	if !canBeNonNullableHasNullableType(file, idx) {
		return
	}
	if !file.FlatHasChildOfType(idx, "=") {
		return
	}
	initExpr := canBeNonNullableFindInitExpr(file, idx)
	if initExpr != 0 && strings.TrimSpace(file.FlatNodeText(initExpr)) == "null" {
		return
	}
	propName := canBeNonNullableFindPropName(file, idx)
	isVar := canBeNonNullableIsVar(file, idx)
	if isVar && propName != "" {
		scope, ok := file.FlatParent(idx)
		if ok && r.hasNullAssignmentInScopeFlat(file, scope, idx, propName, ctx.Resolver) {
			return
		}
	}
	ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, 1,
		"Property type can be non-nullable since it is initialized with a non-null value and never assigned null."))
}

func canBeNonNullableHasSetter(file *scanner.File, idx uint32) bool {
	if nextSib, ok := file.FlatNextSibling(idx); ok && file.FlatType(nextSib) == "setter" {
		return true
	}
	if nextSib, ok := file.FlatNextSibling(idx); ok && file.FlatType(nextSib) == "getter" {
		if nextNext, ok := file.FlatNextSibling(nextSib); ok && file.FlatType(nextNext) == "setter" {
			return true
		}
	}
	return false
}

func canBeNonNullableHasNullableType(file *scanner.File, idx uint32) bool {
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if child == 0 {
			continue
		}
		if file.FlatType(child) == "nullable_type" {
			return true
		}
		if file.FlatType(child) == "variable_declaration" {
			for j := 0; j < file.FlatChildCount(child); j++ {
				gc := file.FlatChild(child, j)
				if file.FlatType(gc) == "nullable_type" {
					return true
				}
			}
		}
	}
	return false
}

func canBeNonNullableFindInitExpr(file *scanner.File, idx uint32) uint32 {
	foundEq := false
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) == "=" {
			foundEq = true
			continue
		}
		if foundEq {
			return child
		}
	}
	return 0
}

func canBeNonNullableFindPropName(file *scanner.File, idx uint32) string {
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) == "variable_declaration" {
			return extractIdentifierFlat(file, child)
		}
	}
	return ""
}

func canBeNonNullableIsVar(file *scanner.File, idx uint32) bool {
	for i := 0; i < file.FlatChildCount(idx); i++ {
		if file.FlatNodeText(file.FlatChild(idx, i)) == "var" {
			return true
		}
	}
	return false
}

func (r *CanBeNonNullableRule) hasNullAssignmentInScopeFlat(file *scanner.File, scope, declNode uint32, propName string, resolver typeinfer.TypeResolver) bool {
	assignedNull := false
	file.FlatWalkAllNodes(scope, func(child uint32) {
		if assignedNull || child == declNode {
			return
		}
		if file.FlatType(child) == "assignment" || file.FlatType(child) == "augmented_assignment" {
			if file.FlatChildCount(child) < 2 {
				return
			}
			lhs := file.FlatChild(child, 0)
			lhsText := strings.TrimSpace(file.FlatNodeText(lhs))
			if lhsText != propName && lhsText != "this."+propName {
				return
			}
			rhs := file.FlatChild(child, file.FlatChildCount(child)-1)
			if canBeNonNullableRHSCanBeNullFlat(file, rhs, resolver) {
				assignedNull = true
				return
			}
		}
	})
	return assignedNull
}

func canBeNonNullableRHSCanBeNullFlat(file *scanner.File, idx uint32, resolver typeinfer.TypeResolver) bool {
	if file == nil || idx == 0 {
		return false
	}
	idx = flatUnwrapParenExpr(file, idx)
	switch file.FlatType(idx) {
	case "null", "null_literal":
		return true
	case "simple_identifier":
		if resolver != nil {
			if typ := resolver.ResolveFlatNode(idx, file); typ != nil && typ.IsNullable() {
				return true
			}
		}
		return false
	case "elvis_expression":
		right := canBeNonNullableElvisRightFlat(file, idx)
		return canBeNonNullableRHSCanBeNullFlat(file, right, resolver)
	case "if_expression", "when_expression":
		return canBeNonNullableBranchCanBeNullFlat(file, idx, resolver)
	case "navigation_expression":
		return canBeNonNullableNavigationUsesSafeCallFlat(file, idx) || canBeNonNullableResolvedNullableFlat(file, idx, resolver)
	case "call_expression":
		return canBeNonNullableCallCanReturnNullFlat(file, idx, resolver)
	case "as_expression":
		return canBeNonNullableAsExpressionCanBeNullFlat(file, idx) || canBeNonNullableResolvedNullableFlat(file, idx, resolver)
	default:
		return canBeNonNullableResolvedNullableFlat(file, idx, resolver)
	}
}

func canBeNonNullableResolvedNullableFlat(file *scanner.File, idx uint32, resolver typeinfer.TypeResolver) bool {
	if resolver == nil || idx == 0 {
		return false
	}
	typ := resolver.ResolveFlatNode(idx, file)
	return typ != nil && typ.IsNullable()
}

func canBeNonNullableElvisRightFlat(file *scanner.File, idx uint32) uint32 {
	if file == nil || idx == 0 || file.FlatType(idx) != "elvis_expression" {
		return 0
	}
	seenElvis := false
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "?:" {
			seenElvis = true
			continue
		}
		if seenElvis && (file.FlatIsNamed(child) || file.FlatType(child) == "null") {
			return child
		}
	}
	return 0
}

func canBeNonNullableBranchCanBeNullFlat(file *scanner.File, idx uint32, resolver typeinfer.TypeResolver) bool {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "control_structure_body" {
			continue
		}
		if canBeNonNullableRHSCanBeNullFlat(file, child, resolver) {
			return true
		}
	}
	return false
}

func canBeNonNullableNavigationUsesSafeCallFlat(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	found := false
	file.FlatWalkAllNodes(idx, func(child uint32) {
		if found {
			return
		}
		found = file.FlatType(child) == "?."
	})
	return found
}

func canBeNonNullableCallCanReturnNullFlat(file *scanner.File, idx uint32, resolver typeinfer.TypeResolver) bool {
	if file == nil || idx == 0 {
		return false
	}
	if resolver != nil {
		if typ := resolver.ResolveFlatNode(idx, file); typ != nil && typ.IsNullable() {
			return true
		}
	}
	first := file.FlatFirstChild(idx)
	return first != 0 && file.FlatType(first) == "navigation_expression" && canBeNonNullableNavigationUsesSafeCallFlat(file, first)
}

func canBeNonNullableAsExpressionCanBeNullFlat(file *scanner.File, idx uint32) bool {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "as?":
			return true
		case "nullable_type":
			return true
		}
	}
	return false
}

func (r *CanBeNonNullableRule) checkFunctionParamsFlat(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if file.FlatHasModifier(idx, "override") || file.FlatHasModifier(idx, "open") || file.FlatHasModifier(idx, "abstract") {
		return
	}

	body, _ := file.FlatFindChild(idx, "function_body")
	if body == 0 {
		return
	}

	params, _ := file.FlatFindChild(idx, "function_value_parameters")
	if params == 0 {
		return
	}

	for i := 0; i < file.FlatNamedChildCount(params); i++ {
		param := file.FlatNamedChild(params, i)
		if param == 0 || file.FlatType(param) != "parameter" {
			continue
		}
		if !file.FlatHasChildOfType(param, "nullable_type") {
			continue
		}

		paramName := extractIdentifierFlat(file, param)
		if paramName == "" {
			continue
		}

		allNonNullAsserted := true
		usageCount := 0
		file.FlatWalkNodes(body, "simple_identifier", func(child uint32) {
			if !allNonNullAsserted || !file.FlatNodeTextEquals(child, paramName) {
				return
			}
			if unusedParameterReferenceShadowedFlat(file, body, child, paramName) {
				return
			}
			usageCount++
			parent, ok := file.FlatParent(child)
			if !ok {
				allNonNullAsserted = false
				return
			}
			switch file.FlatType(parent) {
			case "non_null_assertion":
				return
			case "postfix_expression", "postfix_unary_expression":
				if strings.HasSuffix(strings.TrimSpace(file.FlatNodeText(parent)), "!!") {
					return
				}
			}
			allNonNullAsserted = false
		})

		if usageCount > 0 && allNonNullAsserted {
			ctx.Emit(r.Finding(file, file.FlatRow(param)+1, 1,
				fmt.Sprintf("Parameter '%s' can be non-nullable since every usage applies non-null assertion (!!).", paramName)))
		}
	}
}

// DoubleNegativeExpressionRule detects `!isNotEmpty()` etc.
type DoubleNegativeExpressionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence: tier-1 — the detection is purely syntactic. prefix_expression
// with a `!` operator applied to a no-arg call whose callee starts with
// `isNot`/`isNon` is unambiguously a double negative.
func (r *DoubleNegativeExpressionRule) Confidence() float64 { return api.ConfidenceHigher }

// checkDoubleNegativeExpressionFlat runs on a prefix_expression. It fires
// when the shape is `!<expr>.isNot<Suffix>()` or `!isNon<Suffix>()` — a
// unary-bang applied to a zero-argument callable whose name begins with
// `isNot` or `isNon`. Works for qualified (`xs.isNotEmpty()`) and
// unqualified (`isNonBlank()`) callees via flatCallExpressionName.
func (r *DoubleNegativeExpressionRule) checkDoubleNegativeExpressionFlat(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	first := file.FlatFirstChild(idx)
	if first == 0 || file.FlatType(first) != "!" {
		return
	}
	operand := file.FlatNextSib(first)
	for operand != 0 && !file.FlatIsNamed(operand) {
		operand = file.FlatNextSib(operand)
	}
	operand = flatUnwrapParenExpr(file, operand)
	if operand == 0 || file.FlatType(operand) != "call_expression" {
		return
	}
	args := flatCallKeyArguments(file, operand)
	if args != 0 && file.FlatNamedChildCount(args) > 0 {
		return // has arguments — not the zero-arg form we rewrite.
	}
	callee := flatCallExpressionName(file, operand)
	kind, suffix := splitIsNotPrefix(callee)
	if kind == "" {
		return
	}

	receiverPrefix := ""
	for child := file.FlatFirstChild(operand); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "navigation_expression" {
			// everything before the final navigation_suffix is the receiver
			last := flatLastChildOfType(file, child, "navigation_suffix")
			if last != 0 {
				receiverPrefix = strings.TrimSpace(string(file.Content[file.FlatStartByte(child):file.FlatStartByte(last)])) + "."
			}
			break
		}
	}

	var positive string
	switch suffix {
	case "Empty":
		positive = receiverPrefix + "isEmpty()"
	case "Blank":
		positive = receiverPrefix + "isBlank()"
	case "Null":
		positive = receiverPrefix + "isNull()"
	default:
		positive = receiverPrefix + "is" + suffix + "()"
	}
	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Double negative expression. Simplify by using the positive variant.")
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   int(file.FlatStartByte(idx)),
		EndByte:     int(file.FlatEndByte(idx)),
		Replacement: positive,
	}
	ctx.Emit(f)
}

// splitIsNotPrefix returns ("Not"|"Non", suffix) if name starts with
// "isNot<Uppercase...>" or "isNon<Uppercase...>", else ("", "").
func splitIsNotPrefix(name string) (kind, suffix string) {
	for _, prefix := range []string{"isNot", "isNon"} {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		rest := name[len(prefix):]
		if rest == "" {
			return "", ""
		}
		first := rest[0]
		if first < 'A' || first > 'Z' {
			return "", ""
		}
		return prefix[2:], rest
	}
	return "", ""
}

// DoubleNegativeLambdaRule detects `filterNot { !it }`, `none { !it }`.
type DoubleNegativeLambdaRule struct {
	FlatDispatchBase
	BaseRule
	// NegativeFunctions is an additional callee allowlist. Configured
	// names are treated as double-negation candidates alongside the
	// always-on `filterNot` and `none`.
	NegativeFunctions []string
}

// doubleNegativeLambdaCalleeConfigured reports whether the callee name
// appears in the user-configured NegativeFunctions list.
func doubleNegativeLambdaCalleeConfigured(callee string, configured []string) bool {
	if callee == "" || len(configured) == 0 {
		return false
	}
	for _, name := range configured {
		if name == callee {
			return true
		}
	}
	return false
}

// Confidence: tier-1 syntactic — the shape `.filterNot { <negation> }` and
// `.none { <negation> }` where the lambda body is a single prefix-bang
// expression is an unambiguous double negative. We deliberately do NOT
// flag multi-statement lambdas or compound expressions where `!` appears
// inside a larger boolean expression.
func (r *DoubleNegativeLambdaRule) Confidence() float64 { return api.ConfidenceHigher }

// checkDoubleNegativeLambdaFlat runs on a call_expression. Fires when the
// callee is `filterNot` or `none` (qualified or unqualified) and the
// trailing lambda body is a single unary-bang expression. Additional
// callee names can be configured via NegativeFunctions; those receive a
// generic suggestion to invert the lambda body.
func (r *DoubleNegativeLambdaRule) checkDoubleNegativeLambdaFlat(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	callee := flatCallExpressionName(file, idx)
	var msg, replacementCallee string
	switch callee {
	case "filterNot":
		msg = "Double negative in '.filterNot { !... }'. Use '.filter { ... }' instead."
		replacementCallee = "filter"
	case "none":
		msg = "Double negative in '.none { !... }'. Use '.all { ... }' instead."
		replacementCallee = "all"
	default:
		if !doubleNegativeLambdaCalleeConfigured(callee, r.NegativeFunctions) {
			return
		}
		msg = fmt.Sprintf("Double negative in '.%s { !... }'. Invert the lambda body to remove the redundant negation.", callee)
	}
	lambda := flatCallTrailingLambda(file, idx)
	if lambda == 0 {
		return
	}
	stmts, _ := file.FlatFindChild(lambda, "statements")
	if stmts == 0 || file.FlatNamedChildCount(stmts) != 1 {
		return // require a single-statement body
	}
	body := file.FlatNamedChild(stmts, 0)
	body = flatUnwrapParenExpr(file, body)
	if file.FlatType(body) != "prefix_expression" {
		return
	}
	op := file.FlatFirstChild(body)
	if op == 0 || file.FlatType(op) != "!" {
		return
	}

	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, msg)
	// Only the well-known names have a deterministic name swap. For
	// configurable NegativeFunctions the right replacement is the
	// author's call — emit without a fix.
	if replacementCallee != "" {
		f.Fix = buildDoubleNegativeLambdaFix(file, idx, op, callee, replacementCallee)
	}
	ctx.Emit(f)
}

// buildDoubleNegativeLambdaFix rewrites `recv.filterNot { !pred }` to
// `recv.filter { pred }` (and `none`→`all`) by renaming the callee
// identifier and removing the `!` prefix from the lambda body. Returns
// nil when either edit cannot be located.
func buildDoubleNegativeLambdaFix(file *scanner.File, call uint32, bangOp uint32, oldName, newName string) *scanner.Fix {
	var calleeIdent uint32
	for c := file.FlatFirstChild(call); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "simple_identifier":
			if file.FlatNodeTextEquals(c, oldName) {
				calleeIdent = c
			}
		case "navigation_expression":
			calleeIdent = flatNavigationExpressionLastIdentifierNamed(file, c, oldName)
		}
		if calleeIdent != 0 {
			break
		}
	}
	if calleeIdent == 0 || bangOp == 0 {
		return nil
	}

	callStart := int(file.FlatStartByte(call))
	callEnd := int(file.FlatEndByte(call))
	edits := []byteEdit{
		{int(file.FlatStartByte(calleeIdent)), int(file.FlatEndByte(calleeIdent)), newName},
		{int(file.FlatStartByte(bangOp)), int(file.FlatEndByte(bangOp)), ""},
	}
	repl, ok := applyByteEdits(file.Content, callStart, callEnd, edits)
	if !ok {
		return nil
	}
	return &scanner.Fix{
		ByteMode:    true,
		StartByte:   callStart,
		EndByte:     callEnd,
		Replacement: repl,
	}
}

// NullableBooleanCheckRule detects `x == true` on Boolean?.
type NullableBooleanCheckRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection uses structural pattern matching on
// expressions; the suggested rewrite's readability is a style call.
// Classified per roadmap/17.
func (r *NullableBooleanCheckRule) Confidence() float64 { return api.ConfidenceMedium }

// RangeUntilInsteadOfRangeToRule detects `until` usage that could use `..<`.
type RangeUntilInsteadOfRangeToRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection uses structural pattern matching on
// expressions; the suggested rewrite's readability is a style call.
// Classified per roadmap/17.
func (r *RangeUntilInsteadOfRangeToRule) Confidence() float64 { return api.ConfidenceMedium }

// DestructuringDeclarationWithTooManyEntriesRule limits destructuring entries.
type DestructuringDeclarationWithTooManyEntriesRule struct {
	FlatDispatchBase
	BaseRule
	MaxEntries int
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection uses structural pattern matching on
// expressions; the suggested rewrite's readability is a style call.
// Classified per roadmap/17.
func (r *DestructuringDeclarationWithTooManyEntriesRule) Confidence() float64 {
	return api.ConfidenceMedium
}
