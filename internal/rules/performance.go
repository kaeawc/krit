package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// BitmapDecodeWithoutOptionsRule detects BitmapFactory.decode* calls that omit
// BitmapFactory.Options, which can lead to decoding full-size bitmaps.
type BitmapDecodeWithoutOptionsRule struct {
	FlatDispatchBase
	BaseRule
}

var bitmapDecodeMethods = map[string]bool{
	"decodeFile":     true,
	"decodeResource": true,
	"decodeStream":   true,
}

// Confidence reports a tier-2 (medium) base confidence. Performance rule. Detection pattern-matches anti-patterns (allocation in
// loops, primitive boxing, collection chains) with optional resolver
// support; fallback is heuristic. Classified per roadmap/17.
func (r *BitmapDecodeWithoutOptionsRule) Confidence() float64 { return api.ConfidenceMedium }

// ArrayPrimitiveRule detects Array<Int> etc. instead of IntArray.
// With type inference: verifies the type argument resolves to a primitive type
// via ResolveImport, catching aliased or re-imported primitives.
type ArrayPrimitiveRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence — detects
// Array<Int>/Array<Long> that should be IntArray/LongArray; needs resolver
// for generic receivers, falls back to text match. Classified per
// roadmap/17.
func (r *ArrayPrimitiveRule) Confidence() float64 { return api.ConfidenceMedium }

var primitiveArrayTypes = map[string]string{
	"Int":     "IntArray",
	"Long":    "LongArray",
	"Short":   "ShortArray",
	"Byte":    "ByteArray",
	"Char":    "CharArray",
	"Float":   "FloatArray",
	"Double":  "DoubleArray",
	"Boolean": "BooleanArray",
}

// primitiveFQNToReplacement maps FQNs to their specialized array type.
var primitiveFQNToReplacement = map[string]string{
	"kotlin.Int":     "IntArray",
	"kotlin.Long":    "LongArray",
	"kotlin.Short":   "ShortArray",
	"kotlin.Byte":    "ByteArray",
	"kotlin.Char":    "CharArray",
	"kotlin.Float":   "FloatArray",
	"kotlin.Double":  "DoubleArray",
	"kotlin.Boolean": "BooleanArray",
}

func normalizeTypeReference(text string) string {
	text = strings.TrimSpace(text)
	text = strings.Trim(text, "<>")
	text = strings.TrimSpace(text)
	text = strings.TrimSuffix(text, "?")
	text = strings.TrimSpace(text)
	return text
}

func simpleTypeReferenceName(text string) string {
	text = normalizeTypeReference(text)
	if idx := strings.LastIndex(text, "."); idx >= 0 {
		return text[idx+1:]
	}
	return text
}

func primitiveArrayReplacementForTypeRef(typeRef string) (primitive string, replacement string, ok bool) {
	typeRef = normalizeTypeReference(typeRef)
	if typeRef == "" {
		return "", "", false
	}

	if replacement, ok := primitiveFQNToReplacement[typeRef]; ok {
		return simpleTypeReferenceName(typeRef), replacement, true
	}

	simple := simpleTypeReferenceName(typeRef)
	if replacement, ok := primitiveArrayTypes[simple]; ok {
		return simple, replacement, true
	}

	if replacement, ok := primitiveFQNToReplacement["kotlin."+simple]; ok {
		return simple, replacement, true
	}

	return "", "", false
}

func callExpressionTypeArgumentsFlat(file *scanner.File, call uint32) uint32 {
	if file == nil || call == 0 || file.FlatType(call) != "call_expression" {
		return 0
	}
	if typeArgs, ok := file.FlatFindChild(call, "type_arguments"); ok {
		return typeArgs
	}
	suffix, _ := file.FlatFindChild(call, "call_suffix")
	if suffix == 0 {
		return 0
	}
	typeArgs, _ := file.FlatFindChild(suffix, "type_arguments")
	return typeArgs
}

// ForEachOnRangeRule detects (range).forEach patterns using AST dispatch:
// .., rangeTo, until, downTo, ..< on any literal type (int, long, char,
// unsigned) and through chained calls like .reversed(), .step().
type ForEachOnRangeRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Performance rule. Detection pattern-matches anti-patterns (allocation in
// loops, primitive boxing, collection chains) with optional resolver
// support; fallback is heuristic. Classified per roadmap/17.
func (r *ForEachOnRangeRule) Confidence() float64 { return api.ConfidenceMedium }

// rangeInfixOps are the infix operators that create ranges in Kotlin.
var rangeInfixOps = map[string]bool{
	"rangeTo": true, "downTo": true, "until": true,
}

func containsRangeExpressionFlat(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	switch file.FlatType(idx) {
	case "range_expression":
		return true
	case "parenthesized_expression":
		for i := 0; i < file.FlatChildCount(idx); i++ {
			child := file.FlatChild(idx, i)
			if file.FlatType(child) != "(" && file.FlatType(child) != ")" {
				return containsRangeExpressionFlat(file, child)
			}
		}
	case "infix_expression":
		for i := 0; i < file.FlatChildCount(idx); i++ {
			child := file.FlatChild(idx, i)
			if file.FlatType(child) == "simple_identifier" && rangeInfixOps[file.FlatNodeText(child)] {
				return true
			}
		}
		for i := 0; i < file.FlatChildCount(idx); i++ {
			child := file.FlatChild(idx, i)
			if file.FlatType(child) == "simple_identifier" {
				continue
			}
			if containsRangeExpressionFlat(file, child) {
				return true
			}
		}
	case "call_expression":
		if rangeInfixOps[flatCallExpressionName(file, idx)] {
			return true
		}
		navExpr, _ := flatCallExpressionParts(file, idx)
		if navExpr != 0 && file.FlatNamedChildCount(navExpr) > 0 {
			return containsRangeExpressionFlat(file, file.FlatNamedChild(navExpr, 0))
		}
	case "navigation_expression":
		if file.FlatNamedChildCount(idx) > 0 {
			return containsRangeExpressionFlat(file, file.FlatNamedChild(idx, 0))
		}
	}
	return false
}

// forEachOnRangeFixFlat builds an auto-fix for the flat-tree path.
func forEachOnRangeFixFlat(file *scanner.File, callNode, receiver uint32) *scanner.Fix {
	if file == nil || callNode == 0 || receiver == 0 {
		return nil
	}

	// Only fix simple cases: (rangeExpr).forEach { [param ->] body }
	unwrapped := receiver
	if file.FlatType(unwrapped) == "parenthesized_expression" {
		for i := 0; i < file.FlatChildCount(unwrapped); i++ {
			child := file.FlatChild(unwrapped, i)
			if file.FlatType(child) != "(" && file.FlatType(child) != ")" {
				unwrapped = child
				break
			}
		}
	}

	switch file.FlatType(unwrapped) {
	case "range_expression", "infix_expression":
	default:
		return nil
	}

	rangeText := file.FlatNodeText(unwrapped)

	callSuffix, _ := file.FlatFindChild(callNode, "call_suffix")
	if callSuffix == 0 {
		return nil
	}
	lambdaNode, _ := file.FlatFindChild(callSuffix, "annotated_lambda")
	if lambdaNode == 0 {
		lambdaNode, _ = file.FlatFindChild(callSuffix, "lambda_literal")
	}
	if lambdaNode == 0 {
		return nil
	}
	ll := lambdaNode
	if file.FlatType(ll) == "annotated_lambda" {
		ll, _ = file.FlatFindChild(ll, "lambda_literal")
	}
	if ll == 0 {
		return nil
	}

	iterVar := "i"
	params, _ := file.FlatFindChild(ll, "lambda_parameters")
	if params != 0 {
		for i := 0; i < file.FlatNamedChildCount(params); i++ {
			param := file.FlatNamedChild(params, i)
			if param == 0 {
				continue
			}
			switch file.FlatType(param) {
			case "variable_declaration", "simple_identifier":
				iterVar = file.FlatNodeText(param)
				goto body
			}
		}
	}

body:
	bodyText := ""
	statements, _ := file.FlatFindChild(ll, "statements")
	if statements != 0 {
		bodyText = file.FlatNodeText(statements)
	}

	replacement := "for (" + iterVar + " in " + rangeText + ") { " + bodyText + " }"
	return &scanner.Fix{
		ByteMode:    true,
		StartByte:   int(file.FlatStartByte(callNode)),
		EndByte:     int(file.FlatEndByte(callNode)),
		Replacement: replacement,
	}
}

// SpreadOperatorRule detects *array in function calls where an array copy is created.
// Excludes cases where the Kotlin compiler (1.1.60+) can skip the copy:
// - *arrayOf(...), *intArrayOf(...), etc. (array constructor calls)
// - *arrayOfNulls(...), *emptyArray()
type SpreadOperatorRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Performance rule. Detection pattern-matches anti-patterns (allocation in
// loops, primitive boxing, collection chains) with optional resolver
// support; fallback is heuristic. Classified per roadmap/17.
func (r *SpreadOperatorRule) Confidence() float64 { return api.ConfidenceMedium }

func spreadOperatorShouldReportFlat(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	if strings.HasSuffix(file.Path, ".gradle.kts") {
		return false
	}
	if file.FlatChildCount(idx) > 0 {
		child := file.FlatChild(idx, file.FlatChildCount(idx)-1)
		if file.FlatType(child) == "call_expression" {
			// Preserve the historical fast-path behavior: computed call
			// results, including array factory calls, are not classified
			// as guaranteed array-copy sites.
			return false
		}
		if file.FlatType(child) == "simple_identifier" {
			name := file.FlatNodeText(child)
			if isEnclosingVarargParamFlat(file, idx, name) {
				return false
			}
		}
		if isSpreadIntoSQLBuilderFlat(file, idx) {
			return false
		}
	}
	return true
}

func isSpreadIntoSQLBuilderFlat(file *scanner.File, idx uint32) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "call_expression" {
			switch flatCallExpressionName(file, p) {
			case "select", "where", "columns", "projection",
				"buildArgs", "selectColumns":
				return true
			}
			// Continue walking: a wrapping helper call may itself be
			// nested inside a SQL builder. Stopping here would cause
			// nested builder calls to slip through.
			continue
		}
		if file.FlatType(p) == "function_declaration" {
			return false
		}
	}
	return false
}

func isEnclosingVarargParamFlat(file *scanner.File, idx uint32, name string) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) != "function_declaration" {
			continue
		}
		params, _ := file.FlatFindChild(p, "function_value_parameters")
		if params == 0 {
			return false
		}
		for i := 0; i < file.FlatChildCount(params); i++ {
			child := file.FlatChild(params, i)
			if file.FlatType(child) != "parameter" || extractIdentifierFlat(file, child) != name {
				continue
			}
			if strings.Contains(file.FlatNodeText(child), "vararg") {
				return true
			}
			if modifiers, ok := file.FlatFindChild(child, "parameter_modifiers"); ok &&
				strings.Contains(file.FlatNodeText(modifiers), "vararg") {
				return true
			}
		}
		for _, paramText := range strings.Split(file.FlatNodeText(params), ",") {
			if strings.Contains(paramText, "vararg") && spreadParameterTextContainsName(paramText, name) {
				return true
			}
		}
		return false
	}
	return false
}

func spreadParameterTextContainsName(paramText, name string) bool {
	for _, part := range strings.FieldsFunc(paramText, func(r rune) bool {
		return r != '_' && r != '$' && (r < '0' || r > '9') && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z')
	}) {
		if part == name {
			return true
		}
	}
	return false
}
