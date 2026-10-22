package rules

import (
	"bytes"
	"strconv"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// UnnecessaryInitOnArrayRule detects IntArray(n) { 0 }, etc. where init value is the default.
type UnnecessaryInitOnArrayRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. The rule now
// inspects only array-constructor call expressions and single-expression
// init lambdas; confidence remains medium because the constructor target may
// be unresolved in parser-only runs.
func (r *UnnecessaryInitOnArrayRule) Confidence() float64 { return api.ConfidenceMedium }

var arrayDefaultKinds = map[string]string{
	"IntArray":     "zero",
	"UIntArray":    "zero",
	"LongArray":    "zero",
	"ULongArray":   "zero",
	"ShortArray":   "zero",
	"UShortArray":  "zero",
	"ByteArray":    "zero",
	"UByteArray":   "zero",
	"FloatArray":   "zero",
	"DoubleArray":  "zero",
	"BooleanArray": "false",
	"CharArray":    "zero-char",
}

var zeroConversionMethods = map[string]bool{
	"toInt": true, "toUInt": true, "toLong": true, "toULong": true,
	"toShort": true, "toUShort": true, "toByte": true, "toUByte": true,
	"toFloat": true, "toDouble": true, "toChar": true,
}

func unnecessaryInitArrayDefaultLambdaFlat(file *scanner.File, call uint32) (name string, lambda uint32, ok bool) {
	if file == nil || call == 0 || file.FlatType(call) != "call_expression" {
		return "", 0, false
	}
	name = flatCallNameAny(file, call)
	if _, exists := arrayDefaultKinds[name]; !exists {
		return "", 0, false
	}
	lambda = flatCallTrailingLambda(file, call)
	if lambda == 0 {
		return "", 0, false
	}
	expr := singleExpressionLambdaBodyFlat(file, lambda)
	if expr == 0 || !arrayDefaultExpressionFlat(file, name, expr) {
		return "", 0, false
	}
	return name, lambda, true
}

func singleExpressionLambdaBodyFlat(file *scanner.File, lambda uint32) uint32 {
	if file == nil || lambda == 0 {
		return 0
	}
	if file.FlatType(lambda) == "annotated_lambda" {
		lambda, _ = file.FlatFindChild(lambda, "lambda_literal")
	}
	if lambda == 0 || file.FlatType(lambda) != "lambda_literal" {
		return 0
	}
	statements, _ := file.FlatFindChild(lambda, "statements")
	if statements == 0 || file.FlatNamedChildCount(statements) != 1 {
		return 0
	}
	return flatUnwrapParenExpr(file, file.FlatNamedChild(statements, 0))
}

func arrayDefaultExpressionFlat(file *scanner.File, arrayName string, expr uint32) bool {
	switch arrayDefaultKinds[arrayName] {
	case "zero":
		return flatNumericZeroExpression(file, expr)
	case "false":
		return file.FlatType(expr) == "boolean_literal" && file.FlatNodeTextEquals(expr, "false")
	case "zero-char":
		if file.FlatType(expr) == "character_literal" {
			text := file.FlatNodeText(expr)
			return text == `'\\u0000'` || text == `'\u0000'`
		}
		return flatZeroConversionCall(file, expr, "toChar")
	default:
		return false
	}
}

func flatNumericZeroExpression(file *scanner.File, expr uint32) bool {
	expr = flatUnwrapParenExpr(file, expr)
	switch file.FlatType(expr) {
	case "integer_literal", "long_literal", "real_literal", "hex_literal", "bin_literal":
		return numericLiteralIsZero(file.FlatNodeText(expr))
	case "call_expression":
		return flatZeroConversionCall(file, expr, "")
	}
	return false
}

func numericLiteralIsZero(text string) bool {
	clean := strings.ReplaceAll(strings.TrimSpace(text), "_", "")
	clean = strings.TrimRight(clean, "uUlLfFdD")
	if clean == "" {
		return false
	}
	if strings.HasPrefix(clean, "0x") || strings.HasPrefix(clean, "0X") {
		v, err := strconv.ParseUint(clean[2:], 16, 64)
		return err == nil && v == 0
	}
	if strings.HasPrefix(clean, "0b") || strings.HasPrefix(clean, "0B") {
		v, err := strconv.ParseUint(clean[2:], 2, 64)
		return err == nil && v == 0
	}
	v, err := strconv.ParseFloat(clean, 64)
	return err == nil && v == 0
}

func flatZeroConversionCall(file *scanner.File, expr uint32, wantMethod string) bool {
	if file == nil || expr == 0 || file.FlatType(expr) != "call_expression" {
		return false
	}
	method := flatCallExpressionName(file, expr)
	if wantMethod != "" {
		if method != wantMethod {
			return false
		}
	} else if !zeroConversionMethods[method] {
		return false
	}
	nav, _ := flatCallExpressionParts(file, expr)
	if nav == 0 || file.FlatNamedChildCount(nav) == 0 {
		return false
	}
	return flatNumericZeroExpression(file, file.FlatNamedChild(nav, 0))
}

func unnecessaryInitArrayFixFlat(file *scanner.File, lambda uint32) *scanner.Fix {
	if file == nil || lambda == 0 {
		return nil
	}
	start := int(file.FlatStartByte(lambda))
	end := int(file.FlatEndByte(lambda))
	for start > 0 {
		b := file.Content[start-1]
		if b != ' ' && b != '\t' {
			break
		}
		start--
	}
	return &scanner.Fix{
		ByteMode:    true,
		StartByte:   start,
		EndByte:     end,
		Replacement: "",
	}
}

// UnnecessaryPartOfBinaryExpressionRule detects x && true, x || false, etc.
type UnnecessaryPartOfBinaryExpressionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Performance rule. Detection pattern-matches anti-patterns (allocation in
// loops, primitive boxing, collection chains) with optional resolver
// support; fallback is heuristic. Classified per roadmap/17.
func (r *UnnecessaryPartOfBinaryExpressionRule) Confidence() float64 { return api.ConfidenceMedium }

// UnnecessaryTemporaryInstantiationRule detects Integer.valueOf(x).toString() etc.
type UnnecessaryTemporaryInstantiationRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Performance rule. Detection pattern-matches anti-patterns (allocation in
// loops, primitive boxing, collection chains) with optional resolver
// support; fallback is heuristic. Classified per roadmap/17.
func (r *UnnecessaryTemporaryInstantiationRule) Confidence() float64 { return api.ConfidenceMedium }

var tempInstantiationPrefixNeedles = [][]byte{
	[]byte("Integer"), []byte("Long"), []byte("Short"), []byte("Byte"),
	[]byte("Float"), []byte("Double"), []byte("Boolean"), []byte("Character"),
}

var tempInstantiationMethods = map[string]bool{
	"valueOf":     true,
	"parseInt":    true,
	"parseLong":   true,
	"parseFloat":  true,
	"parseDouble": true,
}

var tempInstantiationTypeNames = map[string]bool{
	"Integer":   true,
	"Long":      true,
	"Short":     true,
	"Byte":      true,
	"Float":     true,
	"Double":    true,
	"Boolean":   true,
	"Character": true,
}

func looksLikeTempInstantiation(src []byte) bool {
	if !bytes.Contains(src, []byte("toString")) {
		return false
	}
	if !bytes.Contains(src, []byte("valueOf")) && !bytes.Contains(src, []byte("parse")) {
		return false
	}
	for _, needle := range tempInstantiationPrefixNeedles {
		if bytes.Contains(src, needle) {
			return true
		}
	}
	return false
}

func tempInstantiationReceiverFlat(file *scanner.File, nav uint32) uint32 {
	for i := 0; i < file.FlatChildCount(nav); i++ {
		child := file.FlatChild(nav, i)
		if file.FlatType(child) == "navigation_suffix" || file.FlatType(child) == "." {
			continue
		}
		return child
	}
	return 0
}

func tempInstantiationTypeNameFlat(file *scanner.File, nav uint32) string {
	receiver := tempInstantiationReceiverFlat(file, nav)
	if receiver == 0 {
		return ""
	}
	name := strings.TrimSpace(file.FlatNodeText(receiver))
	if name == "" {
		return ""
	}
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		name = name[idx+1:]
	}
	return name
}

func tempInstantiationFirstArgumentFlat(file *scanner.File, call uint32) string {
	callSuffix, _ := file.FlatFindChild(call, "call_suffix")
	if callSuffix == 0 {
		return ""
	}
	valueArgs, _ := file.FlatFindChild(callSuffix, "value_arguments")
	if valueArgs == 0 {
		return ""
	}
	for i := 0; i < file.FlatChildCount(valueArgs); i++ {
		child := file.FlatChild(valueArgs, i)
		if file.FlatType(child) == "value_argument" {
			return file.FlatNodeText(child)
		}
	}
	return ""
}

// UnnecessaryTypeCastingRule detects casting a value to its own type.
type UnnecessaryTypeCastingRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence — flags casts
// that are no-ops; needs the resolver to confirm the source type matches
// the target, falls back to textual comparison. Classified per roadmap/17.
func (r *UnnecessaryTypeCastingRule) Confidence() float64 { return api.ConfidenceMedium }

func safeCastExpressionParts(file *scanner.File, idx uint32) (source, target uint32, ok bool) {
	if file == nil || idx == 0 || file.FlatType(idx) != "as_expression" {
		return 0, 0, false
	}
	seenSafeAs := false
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "as?":
			seenSafeAs = true
			continue
		}
		if !file.FlatIsNamed(child) {
			continue
		}
		if !seenSafeAs {
			source = child
			continue
		}
		if target == 0 {
			target = child
		}
	}
	return source, target, seenSafeAs && source != 0 && target != 0
}

func safeCastComparedNotNullParts(file *scanner.File, idx uint32) (sourceText, targetText string, ok bool) {
	if file == nil || idx == 0 || file.FlatType(idx) != "equality_expression" {
		return "", "", false
	}
	var left, op, right uint32
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "!=":
			op = child
		default:
			if left == 0 {
				left = child
			} else if right == 0 {
				right = child
			}
		}
	}
	if op == 0 || left == 0 || right == 0 {
		return "", "", false
	}
	left = flatUnwrapParenExpr(file, left)
	right = flatUnwrapParenExpr(file, right)

	var cast uint32
	switch {
	case file.FlatType(left) == "as_expression" && file.FlatType(right) == "null":
		cast = left
	case file.FlatType(left) == "null" && file.FlatType(right) == "as_expression":
		cast = right
	default:
		return "", "", false
	}

	source, target, ok := safeCastExpressionParts(file, cast)
	if !ok {
		return "", "", false
	}
	if file.FlatType(target) == "nullable_type" {
		return "", "", false
	}
	sourceText = strings.TrimSpace(file.FlatNodeText(source))
	targetText = strings.TrimSpace(file.FlatNodeText(target))
	return sourceText, targetText, sourceText != "" && targetText != ""
}
