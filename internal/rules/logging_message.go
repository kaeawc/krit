package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// UnstructuredErrorLogRule detects logger.error/logger.warn calls that embed a
// Throwable-looking value in the message instead of passing it as a structured
// throwable argument.
type UnstructuredErrorLogRule struct {
	FlatDispatchBase
	BaseRule

	Methods []string
}

func (r *UnstructuredErrorLogRule) Confidence() float64 { return api.ConfidenceMediumLow }

func (r *UnstructuredErrorLogRule) methodEnabled(method string) bool {
	if len(r.Methods) == 0 {
		return method == "error" || method == "warn" || method == "warning"
	}
	for _, configured := range r.Methods {
		if strings.TrimSpace(configured) == method {
			return true
		}
	}
	return false
}

func unstructuredErrorLogMessageArgFlat(file *scanner.File, call uint32) uint32 {
	_, args := flatCallExpressionParts(file, call)
	if args == 0 || flatValueArgumentCount(args, file) != 1 {
		return 0
	}
	arg := flatPositionalValueArgument(file, args, 0)
	if arg == 0 {
		arg = flatNamedValueArgument(file, args, "message")
	}
	if arg == 0 {
		return 0
	}
	expr := flatValueArgumentExpression(file, arg)
	if expr == 0 {
		return 0
	}
	if flatContainsStringInterpolation(file, expr) && throwableLikeInterpolation(file.FlatNodeText(expr)) {
		return arg
	}
	if concatHasThrowableLikeOperand(file, expr) {
		return arg
	}
	if stringFormatHasThrowableLikeArg(file, expr) {
		return arg
	}
	return 0
}

func flatValueArgumentCount(args uint32, file *scanner.File) int {
	if file == nil || args == 0 {
		return 0
	}
	count := 0
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) == "value_argument" {
			count++
		}
	}
	return count
}

func concatHasThrowableLikeOperand(file *scanner.File, expr uint32) bool {
	expr = flatUnwrapParenExpr(file, expr)
	if file == nil || expr == 0 || file.FlatType(expr) != "additive_expression" {
		return false
	}
	var operands []uint32
	if !collectStringConcatOperands(file, expr, &operands) {
		return false
	}
	for _, op := range operands {
		op = flatUnwrapParenExpr(file, op)
		switch file.FlatType(op) {
		case "string_literal", "line_string_literal", "multi_line_string_literal":
			continue
		default:
			if throwableLikeIdentifierExpr(file, op) {
				return true
			}
		}
	}
	return false
}

func stringFormatHasThrowableLikeArg(file *scanner.File, expr uint32) bool {
	expr = flatUnwrapParenExpr(file, expr)
	if file == nil || expr == 0 || file.FlatType(expr) != "call_expression" || flatCallExpressionName(file, expr) != "format" {
		return false
	}
	_, args := flatCallExpressionParts(file, expr)
	if args == 0 {
		return false
	}
	index := 0
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		expr := flatValueArgumentExpression(file, arg)
		if index > 0 && throwableLikeIdentifierExpr(file, expr) {
			return true
		}
		index++
	}
	return false
}

func throwableLikeIdentifierExpr(file *scanner.File, expr uint32) bool {
	expr = flatUnwrapParenExpr(file, expr)
	if file == nil || expr == 0 {
		return false
	}
	switch file.FlatType(expr) {
	case "simple_identifier":
		return throwableLikeIdentifierRe.MatchString(file.FlatNodeText(expr))
	default:
		return false
	}
}

// TraceIDLoggedAsPlainMessageRule detects trace/span/request/correlation IDs
// embedded directly in log message text instead of being carried in MDC or a
// structured logging field.
type TraceIDLoggedAsPlainMessageRule struct {
	FlatDispatchBase
	BaseRule

	Identifiers []string
}

func (r *TraceIDLoggedAsPlainMessageRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *TraceIDLoggedAsPlainMessageRule) identifierSet() map[string]bool {
	defaults := []string{"traceId", "trace_id", "spanId", "span_id", "requestId", "request_id", "correlationId", "correlation_id"}
	values := r.Identifiers
	if len(values) == 0 {
		values = defaults
	}
	out := make(map[string]bool, len(values))
	for _, value := range values {
		if normalized := normalizeObservabilityIdentifier(value); normalized != "" {
			out[normalized] = true
		}
	}
	return out
}

func traceIDPlainMessageArgFlat(file *scanner.File, call uint32, identifiers map[string]bool) uint32 {
	_, args := flatCallExpressionParts(file, call)
	if args == 0 {
		return 0
	}
	arg := flatNamedValueArgument(file, args, "message")
	if arg == 0 {
		arg = flatPositionalValueArgument(file, args, 0)
	}
	if arg == 0 {
		return 0
	}
	expr := flatValueArgumentExpression(file, arg)
	if expr == 0 {
		return 0
	}
	if flatContainsStringInterpolation(file, expr) && interpolationHasObservedIdentifier(file.FlatNodeText(expr), identifiers) {
		return arg
	}
	if concatHasObservedIdentifier(file, expr, identifiers) {
		return arg
	}
	if stringFormatHasObservedIdentifier(file, expr, identifiers) {
		return arg
	}
	return 0
}

func concatHasObservedIdentifier(file *scanner.File, expr uint32, identifiers map[string]bool) bool {
	expr = flatUnwrapParenExpr(file, expr)
	if file == nil || expr == 0 || file.FlatType(expr) != "additive_expression" {
		return false
	}
	var operands []uint32
	if !collectStringConcatOperands(file, expr, &operands) {
		return false
	}
	for _, op := range operands {
		if observedIdentifierExpr(file, op, identifiers) {
			return true
		}
	}
	return false
}

func stringFormatHasObservedIdentifier(file *scanner.File, expr uint32, identifiers map[string]bool) bool {
	expr = flatUnwrapParenExpr(file, expr)
	if file == nil || expr == 0 || file.FlatType(expr) != "call_expression" || flatCallExpressionName(file, expr) != "format" {
		return false
	}
	_, args := flatCallExpressionParts(file, expr)
	index := 0
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		if index > 0 && observedIdentifierExpr(file, flatValueArgumentExpression(file, arg), identifiers) {
			return true
		}
		index++
	}
	return false
}

func observedIdentifierExpr(file *scanner.File, expr uint32, identifiers map[string]bool) bool {
	expr = flatUnwrapParenExpr(file, expr)
	if file == nil || expr == 0 || file.FlatType(expr) != "simple_identifier" {
		return false
	}
	return identifiers[normalizeObservabilityIdentifier(file.FlatNodeText(expr))]
}

// StructuredLogKeyMixedCaseRule detects files that mostly use one structured
// logging key convention but contain minority keys using the other convention.
type StructuredLogKeyMixedCaseRule struct {
	FlatDispatchBase
	BaseRule

	MinKeys          int
	ThresholdPercent int
}

func (r *StructuredLogKeyMixedCaseRule) Confidence() float64 { return api.ConfidenceMedium }

// structuredLogKeyDecision carries the per-file majority/minority verdict.
// Empty strings mean "no finding for this file".
type structuredLogKeyDecision struct {
	minority string
	majority string
}

// structuredLogKeyAtCall returns the static structured-log key written at
// `call` when it is an `addKeyValue(...)` or `MDC.put(...)` invocation whose
// first positional argument is a non-interpolated string literal. The
// receiver check rejects `someMap.put("k", v)` look-alikes.
func structuredLogKeyAtCall(file *scanner.File, call uint32) (string, bool) {
	switch flatCallExpressionName(file, call) {
	case "addKeyValue":
	case "put":
		if flatReceiverNameFromCall(file, call) != "MDC" {
			return "", false
		}
	default:
		return "", false
	}
	return mdcStaticKeyFlat(file, call)
}

func (r *StructuredLogKeyMixedCaseRule) decisionFor(ctx *api.Context) structuredLogKeyDecision {
	file := ctx.File
	minKeys := r.MinKeys
	if minKeys == 0 {
		minKeys = 3
	}
	threshold := r.ThresholdPercent
	if threshold == 0 {
		threshold = 70
	}
	return filefacts.FileFact(ctx.Facts, file, "structuredLogKeyDecision", func() structuredLogKeyDecision {
		snake, camel := 0, 0
		tree := file.FlatTree
		callTypeID, ok := scanner.LookupFlatNodeType("call_expression")
		if !ok || tree == nil {
			return structuredLogKeyDecision{}
		}
		for _, flatIdx := range tree.NodesOfType(callTypeID) {
			key, ok := structuredLogKeyAtCall(file, flatIdx)
			if !ok {
				continue
			}
			switch structuredLogKeyConvention(key) {
			case "snake_case":
				snake++
			case "camelCase":
				camel++
			}
		}
		total := snake + camel
		if total < minKeys || snake == 0 || camel == 0 {
			return structuredLogKeyDecision{}
		}
		switch {
		case snake*100 >= total*threshold:
			return structuredLogKeyDecision{minority: "camelCase", majority: "snake_case"}
		case camel*100 >= total*threshold:
			return structuredLogKeyDecision{minority: "snake_case", majority: "camelCase"}
		}
		return structuredLogKeyDecision{}
	})
}

func (r *StructuredLogKeyMixedCaseRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if file == nil || file.FlatTree == nil {
		return
	}
	key, ok := structuredLogKeyAtCall(file, idx)
	if !ok {
		return
	}
	convention := structuredLogKeyConvention(key)
	if convention == "" {
		return
	}
	decision := r.decisionFor(ctx)
	if decision.minority == "" || convention != decision.minority {
		return
	}
	ctx.EmitAt(file.FlatRow(idx)+1, 1,
		"Structured log key "+key+" uses "+decision.minority+" in a file that mostly uses "+decision.majority+". Keep structured log keys consistent within a file.")
}

// LoggerStringConcatRule detects SLF4J/Logback/log4j-style logger calls whose
// message argument is built with `+` string concatenation. Like string
// interpolation, this evaluates the concatenation eagerly even when the level
// is disabled. The fix is the same parameterized form
// (`logger.info("value={}", value)`).
type LoggerStringConcatRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Detection
// pattern-matches logger call shapes without confirming the receiver type.
func (r *LoggerStringConcatRule) Confidence() float64 { return api.ConfidenceMedium }

// loggerStringConcatMessageArgFlat returns the message argument of `call` when
// it is a positional or named `+` concatenation that includes a string literal
// operand. Pure numeric or non-string concatenation is ignored.
func loggerStringConcatMessageArgFlat(file *scanner.File, call uint32) uint32 {
	_, args := flatCallExpressionParts(file, call)
	if args == 0 {
		return 0
	}

	if arg := flatNamedValueArgument(file, args, "message"); arg != 0 {
		if argIsLoggerStringConcat(file, arg) {
			return arg
		}
		return 0
	}

	first := flatPositionalValueArgument(file, args, 0)
	if first == 0 {
		return 0
	}
	if argIsLoggerStringConcat(file, first) {
		return first
	}
	return 0
}

func argIsLoggerStringConcat(file *scanner.File, arg uint32) bool {
	expr := flatValueArgumentExpression(file, arg)
	if expr == 0 {
		return false
	}
	expr = flatUnwrapParenExpr(file, expr)
	if file.FlatType(expr) != "additive_expression" {
		return false
	}
	var operands []uint32
	if !collectStringConcatOperands(file, expr, &operands) {
		return false
	}
	if len(operands) < 2 {
		return false
	}
	for _, op := range operands {
		op = flatUnwrapParenExpr(file, op)
		switch file.FlatType(op) {
		case "string_literal", "line_string_literal", "multi_line_string_literal":
			return true
		}
	}
	return false
}
