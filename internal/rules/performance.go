package rules

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
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
func (r *BitmapDecodeWithoutOptionsRule) Confidence() float64 { return 0.75 }


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
func (r *ArrayPrimitiveRule) Confidence() float64 { return 0.75 }

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


// CouldBeSequenceRule detects collection operation chains that could be sequences.
type CouldBeSequenceRule struct {
	FlatDispatchBase
	BaseRule
	AllowedOperations int
}


// Confidence reports a tier-2 (medium) base confidence — detects long
// collection chains that should use asSequence(); chain-length heuristic is
// conservative but the threshold is a style call. Classified per
// roadmap/17.
func (r *CouldBeSequenceRule) Confidence() float64 { return 0.75 }

// collectionTypes that benefit from asSequence() conversion.
var sequenceCandidateTypes = map[string]bool{
	"List": true, "MutableList": true, "Collection": true,
	"Iterable": true, "Set": true, "MutableSet": true,
}

// Types that should NOT be suggested for asSequence().
var sequenceExcludedTypes = map[string]bool{
	"Sequence": true, "Flow": true, "StateFlow": true,
	"SharedFlow": true, "MutableStateFlow": true, "MutableSharedFlow": true,
}

var obviousSequenceSourceCalls = map[string]bool{
	"sequenceOf":       true,
	"generateSequence": true,
	"emptySequence":    true,
	"asSequence":       true,
}

var obviousCollectionSourceCalls = map[string]bool{
	"listOf":        true,
	"mutableListOf": true,
	"arrayListOf":   true,
	"setOf":         true,
	"mutableSetOf":  true,
	"mapOf":         true,
	"mutableMapOf":  true,
	"emptyList":     true,
	"emptySet":      true,
	"emptyMap":      true,
	"buildList":     true,
	"buildSet":      true,
	"buildMap":      true,
}

var collectionAnnotationTypeNames = []string{
	"List", "MutableList",
	"Set", "MutableSet",
	"Map", "MutableMap",
	"Collection", "Iterable",
}

var collectionOps = map[string]bool{
	"map": true, "filter": true, "flatMap": true, "sorted": true,
	"sortedBy": true, "sortedWith": true, "sortedDescending": true,
	"sortedByDescending": true, "reversed": true, "distinct": true,
	"distinctBy": true, "drop": true, "dropWhile": true,
	"take": true, "takeWhile": true, "zip": true,
}


func collectionChainRootFlat(file *scanner.File, idx uint32) uint32 {
	current := idx
	for current != 0 && file.FlatType(current) == "call_expression" {
		navExpr, _ := flatCallExpressionParts(file, current)
		if navExpr == 0 || file.FlatNamedChildCount(navExpr) == 0 {
			return current
		}
		receiver := file.FlatNamedChild(navExpr, 0)
		if receiver == 0 || file.FlatType(receiver) != "call_expression" {
			return receiver
		}
		current = receiver
	}
	return current
}

func matchesKnownTypeName(name string, candidates map[string]bool) bool {
	name = normalizeTypeReference(name)
	if name == "" {
		return false
	}
	if candidates[name] {
		return true
	}
	if idx := strings.LastIndex(name, "."); idx >= 0 && candidates[name[idx+1:]] {
		return true
	}
	return false
}

func resolvedTypeMatches(resolved *typeinfer.ResolvedType, candidates map[string]bool) bool {
	if resolved == nil || resolved.Kind == typeinfer.TypeUnknown {
		return false
	}
	if matchesKnownTypeName(resolved.Name, candidates) || matchesKnownTypeName(resolved.FQN, candidates) {
		return true
	}
	for _, st := range resolved.Supertypes {
		if matchesKnownTypeName(st, candidates) {
			return true
		}
	}
	return false
}

func flatResolveByName(file *scanner.File, resolver typeinfer.TypeResolver, idx uint32) *typeinfer.ResolvedType {
	if resolver == nil || file == nil || idx == 0 {
		return nil
	}
	var name string
	switch file.FlatType(idx) {
	case "simple_identifier", "type_identifier":
		name = file.FlatNodeText(idx)
	case "navigation_expression":
		name = flatNavigationExpressionLastIdentifier(file, idx)
	case "user_type", "nullable_type":
		name = simpleTypeReferenceName(file.FlatNodeText(idx))
	case "call_expression":
		name = flatCallExpressionName(file, idx)
	default:
		name = strings.TrimSpace(file.FlatNodeText(idx))
	}
	if name == "" {
		return nil
	}
	return resolver.ResolveByNameFlat(name, idx, file)
}

func hasCollectionTypeAnnotation(functionText, name string) bool {
	for _, typeName := range collectionAnnotationTypeNames {
		if strings.Contains(functionText, name+": "+typeName) ||
			strings.Contains(functionText, name+": kotlin.collections."+typeName) {
			return true
		}
	}
	return false
}

func enclosingFunctionDeclarationFlat(file *scanner.File, idx uint32) uint32 {
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		if file.FlatType(parent) == "function_declaration" {
			return parent
		}
	}
	return 0
}

// ForEachOnRangeRule detects (range).forEach pattern using AST dispatch.
// Matches detekt's ForEachOnRange: catches .., rangeTo, until, downTo, ..<
// on any literal type (int, long, char, unsigned) and through chained calls
// like .reversed(), .step().
type ForEachOnRangeRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Performance rule. Detection pattern-matches anti-patterns (allocation in
// loops, primitive boxing, collection chains) with optional resolver
// support; fallback is heuristic. Classified per roadmap/17.
func (r *ForEachOnRangeRule) Confidence() float64 { return 0.75 }


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
		if file.FlatChildCount(idx) > 0 {
			return containsRangeExpressionFlat(file, file.FlatChild(idx, 0))
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
func (r *SpreadOperatorRule) Confidence() float64 { return 0.75 }


// arrayConstructors lists functions where the Kotlin compiler skips the array copy.
var arrayConstructors = map[string]bool{
	"arrayOf":        true,
	"intArrayOf":     true,
	"longArrayOf":    true,
	"shortArrayOf":   true,
	"byteArrayOf":    true,
	"floatArrayOf":   true,
	"doubleArrayOf":  true,
	"charArrayOf":    true,
	"booleanArrayOf": true,
	"arrayOfNulls":   true,
	"emptyArray":     true,
	"Array":          true,
	"IntArray":       true,
	"LongArray":      true,
	"ShortArray":     true,
	"ByteArray":      true,
	"FloatArray":     true,
	"DoubleArray":    true,
	"CharArray":      true,
	"BooleanArray":   true,
}

func isSpreadIntoSqlBuilderFlat(file *scanner.File, idx uint32) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "call_expression" {
			switch flatCallExpressionName(file, p) {
			case "select", "where", "columns", "projection",
				"buildArgs", "selectColumns":
				return true
			}
			return false
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
		prevWasVararg := false
		for i := 0; i < file.FlatChildCount(params); i++ {
			child := file.FlatChild(params, i)
			switch file.FlatType(child) {
			case "parameter_modifiers":
				if strings.Contains(file.FlatNodeText(child), "vararg") {
					prevWasVararg = true
				}
			case "parameter":
				if prevWasVararg && extractIdentifierFlat(file, child) == name {
					return true
				}
				prevWasVararg = false
			case ",", "(", ")":
				// separators — keep prevWasVararg flag
			default:
				prevWasVararg = false
			}
		}
		return false
	}
	return false
}

// UnnecessaryInitOnArrayRule detects IntArray(n) { 0 }, etc. where init value is the default.
type UnnecessaryInitOnArrayRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Performance rule. Detection pattern-matches anti-patterns (allocation in
// loops, primitive boxing, collection chains) with optional resolver
// support; fallback is heuristic. Classified per roadmap/17.
func (r *UnnecessaryInitOnArrayRule) Confidence() float64 { return 0.75 }


var defaultZeroArrayRe = regexp.MustCompile(`(IntArray|LongArray|ShortArray|ByteArray|FloatArray|DoubleArray)\s*\([^)]+\)\s*\{\s*0+\.?0*\s*\}`)
var defaultFalseArrayRe = regexp.MustCompile(`BooleanArray\s*\([^)]+\)\s*\{\s*false\s*\}`)
var defaultCharArrayRe = regexp.MustCompile(`CharArray\s*\([^)]+\)\s*\{\s*'\\u0000'\s*\}`)

var defaultInitRemoveRe = regexp.MustCompile(`((IntArray|LongArray|ShortArray|ByteArray|FloatArray|DoubleArray)\s*\([^)]+\))\s*\{\s*0+\.?0*\s*\}`)
var defaultFalseRemoveRe = regexp.MustCompile(`(BooleanArray\s*\([^)]+\))\s*\{\s*false\s*\}`)
var defaultCharRemoveRe = regexp.MustCompile(`(CharArray\s*\([^)]+\))\s*\{\s*'\\u0000'\s*\}`)

// UnnecessaryPartOfBinaryExpressionRule detects x && true, x || false, etc.
type UnnecessaryPartOfBinaryExpressionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Performance rule. Detection pattern-matches anti-patterns (allocation in
// loops, primitive boxing, collection chains) with optional resolver
// support; fallback is heuristic. Classified per roadmap/17.
func (r *UnnecessaryPartOfBinaryExpressionRule) Confidence() float64 { return 0.75 }


// UnnecessaryTemporaryInstantiationRule detects Integer.valueOf(x).toString() etc.
type UnnecessaryTemporaryInstantiationRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Performance rule. Detection pattern-matches anti-patterns (allocation in
// loops, primitive boxing, collection chains) with optional resolver
// support; fallback is heuristic. Classified per roadmap/17.
func (r *UnnecessaryTemporaryInstantiationRule) Confidence() float64 { return 0.75 }


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
func (r *UnnecessaryTypeCastingRule) Confidence() float64 { return 0.75 }

