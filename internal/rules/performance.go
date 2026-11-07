package rules

import (
	"path/filepath"
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

var bitmapSmallImageNames = []string{"thumb", "tiny", "icon", "avatar"}

// bitmapDecodeInSmallImageContext reports a decode inside a file, class or function named for a small image.
func bitmapDecodeInSmallImageContext(file *scanner.File, idx uint32) bool {
	names := []string{filepath.Base(file.Path)}
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "function_declaration":
			names = append(names, flatFunctionName(file, cur))
		case "class_declaration", "object_declaration":
			names = append(names, extractIdentifierFlat(file, cur))
		}
	}
	for _, name := range names {
		lower := strings.ToLower(name)
		for _, small := range bitmapSmallImageNames {
			if strings.Contains(lower, small) {
				return true
			}
		}
	}
	return false
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

// arrayTypeArgIsNullable reports whether the type argument text such as `<Int?>` is nullable.
func arrayTypeArgIsNullable(text string) bool {
	return strings.HasSuffix(strings.TrimSpace(strings.Trim(strings.TrimSpace(text), "<>")), "?")
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
	// detekt looks at the root of the receiver chain: a parenthesized range, or a rangeTo, downTo or until
	// call, however the chain continues after it.
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
			if child := file.FlatChild(idx, i); file.FlatType(child) != "simple_identifier" && containsRangeExpressionFlat(file, child) {
				return true
			}
		}
	case "navigation_expression":
		return containsRangeExpressionFlat(file, file.FlatFirstChild(idx))
	case "call_expression":
		if rangeInfixOps[flatCallExpressionName(file, idx)] {
			return true
		}
		return containsRangeExpressionFlat(file, file.FlatFirstChild(idx))
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

	// `return@forEach` would have to become `continue`, so leave those loops to the user.
	if strings.Contains(file.FlatNodeText(ll), "@forEach") {
		return nil
	}

	iterVar := "it"
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
	// Comment text with an unbalanced `/*` re-lexes ` * foo` lines as spreads outside any call.
	if parent, ok := file.FlatParent(idx); !ok || file.FlatType(parent) != "value_argument" {
		return false
	}
	// detekt skips forwarding the enclosing function's own vararg parameter.
	if file.FlatChildCount(idx) > 0 {
		if child := file.FlatChild(idx, file.FlatChildCount(idx)-1); file.FlatType(child) == "simple_identifier" {
			if isEnclosingVarargParamFlat(file, idx, file.FlatNodeText(child)) {
				return false
			}
		}
	}
	return true
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

// arrayPrimitiveInParameterizedContext reports contexts that need Array<T>: JUnit parameter providers, vararg and generic type arguments.
func arrayPrimitiveInParameterizedContext(file *scanner.File, idx uint32) bool {
	inBody := false
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "function_body":
			inBody = true
		case "type_arguments", "type_projection":
			return true
		case "parameter", "class_parameter":
			if strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(cur)), "vararg") || file.FlatHasModifier(cur, "vararg") {
				return true
			}
		case "function_declaration":
			// An override signature must keep the supertype's parameter and return types.
			if !inBody && file.FlatHasModifier(cur, "override") {
				return true
			}
			name := flatFunctionName(file, cur)
			if strings.Contains(name, "Parameters") || strings.Contains(strings.ToLower(name), "cartesianproduct") {
				return true
			}
			if mods, found := file.FlatFindChild(cur, "modifiers"); found && strings.Contains(file.FlatNodeText(mods), "Parameters") {
				return true
			}
		case "class_declaration":
			if mods, found := file.FlatFindChild(cur, "modifiers"); found && strings.Contains(file.FlatNodeText(mods), "Parameterized") {
				return true
			}
		}
	}
	return false
}
