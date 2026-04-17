package rules

import (
	"bytes"
	"fmt"
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

func (r *BitmapDecodeWithoutOptionsRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *BitmapDecodeWithoutOptionsRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	navExpr, args := flatCallExpressionParts(file, idx)
	if navExpr == 0 || args == 0 {
		return nil
	}

	methodName := flatNavigationExpressionLastIdentifier(file, navExpr)
	if !bitmapDecodeMethods[methodName] {
		return nil
	}

	if file.FlatNamedChildCount(navExpr) == 0 {
		return nil
	}
	receiver := file.FlatNamedChild(navExpr, 0)
	receiverText := strings.TrimSpace(file.FlatNodeText(receiver))
	if idx := strings.LastIndex(receiverText, "."); idx >= 0 {
		receiverText = receiverText[idx+1:]
	}
	if receiverText != "BitmapFactory" {
		return nil
	}

	argCount := 0
	for i := 0; i < file.FlatChildCount(args); i++ {
		if file.FlatType(file.FlatChild(args, i)) == "value_argument" {
			argCount++
		}
	}
	if argCount != 1 {
		return nil
	}

	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		fmt.Sprintf("BitmapFactory.%s without BitmapFactory.Options may decode a full-size bitmap. Pass BitmapFactory.Options to control memory usage.", methodName))}
}

// ArrayPrimitiveRule detects Array<Int> etc. instead of IntArray.
// With type inference: verifies the type argument resolves to a primitive type
// via ResolveImport, catching aliased or re-imported primitives.
type ArrayPrimitiveRule struct {
	FlatDispatchBase
	BaseRule
	resolver typeinfer.TypeResolver
}

func (r *ArrayPrimitiveRule) SetResolver(res typeinfer.TypeResolver) { r.resolver = res }

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

func (r *ArrayPrimitiveRule) NodeTypes() []string { return []string{"user_type"} }

func (r *ArrayPrimitiveRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	ident := file.FlatFindChild(idx, "type_identifier")
	typeArgs := file.FlatFindChild(idx, "type_arguments")
	if ident == 0 || typeArgs == 0 {
		return nil
	}
	if !file.FlatNodeTextEquals(ident, "Array") {
		return nil
	}
	text := file.FlatNodeText(typeArgs)

	// With type inference: resolve the type argument to verify it is a primitive
	if r.resolver != nil {
		argName := simpleTypeReferenceName(text)
		fqn := r.resolver.ResolveImport(argName, file)
		if fqn != "" {
			if replacement, ok := primitiveFQNToReplacement[fqn]; ok {
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					fmt.Sprintf("Use '%s' instead of 'Array<%s>' for better performance.", replacement, argName))
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(idx)),
					EndByte:     int(file.FlatEndByte(idx)),
					Replacement: replacement,
				}
				return []scanner.Finding{f}
			}
			// Resolved to a non-primitive FQN — not a match
			return nil
		}
	}

	primitive, replacement, ok := primitiveArrayReplacementForTypeRef(text)
	if !ok {
		return nil
	}
	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		fmt.Sprintf("Use '%s' instead of 'Array<%s>' for better performance.", replacement, primitive))
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   int(file.FlatStartByte(idx)),
		EndByte:     int(file.FlatEndByte(idx)),
		Replacement: replacement,
	}
	return []scanner.Finding{f}
}

// CouldBeSequenceRule detects collection operation chains that could be sequences.
type CouldBeSequenceRule struct {
	FlatDispatchBase
	BaseRule
	AllowedOperations int
	resolver          typeinfer.TypeResolver
}

func (r *CouldBeSequenceRule) SetResolver(res typeinfer.TypeResolver) { r.resolver = res }

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

func (r *CouldBeSequenceRule) NodeTypes() []string { return []string{"call_expression"} }

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

func (r *CouldBeSequenceRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Count chained collection operations starting from this node
	count := 0
	current := idx
	for current != 0 {
		name := flatCallExpressionName(file, current)
		if collectionOps[name] {
			count++
		} else {
			break
		}
		// Walk up through navigation_expression -> call_expression chain
		if parent, ok := file.FlatParent(current); ok && file.FlatType(parent) == "navigation_expression" {
			if gp, ok := file.FlatParent(parent); ok && file.FlatType(gp) == "call_expression" {
				current = gp
				continue
			}
		}
		break
	}
	if count <= r.AllowedOperations {
		return nil
	}

	rootReceiver := collectionChainRootFlat(file, idx)
	if rootReceiver == 0 {
		return nil
	}

	// With resolver: verify the chain starts on a collection, not a Sequence/Flow.
	if r.resolver != nil {
		resolved := flatResolveByName(file, r.resolver, rootReceiver)
		if resolved != nil && resolved.Kind != typeinfer.TypeUnknown {
			if resolvedTypeMatches(resolved, sequenceExcludedTypes) {
				return nil // Already a Sequence or Flow.
			}
			if resolvedTypeMatches(resolved, sequenceCandidateTypes) {
				return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
					fmt.Sprintf("Chain of %d collection operations. Consider using 'asSequence()' for better performance.", count))}
			}
			return nil
		}
	}

	name := flatCallExpressionName(file, rootReceiver)
	if obviousSequenceSourceCalls[name] {
		return nil
	}
	if !obviousCollectionSourceCalls[name] {
		if file.FlatType(rootReceiver) == "simple_identifier" {
			if fn := enclosingFunctionDeclarationFlat(file, idx); fn != 0 {
				if hasCollectionTypeAnnotation(file.FlatNodeText(fn), file.FlatNodeText(rootReceiver)) {
					return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
						fmt.Sprintf("Chain of %d collection operations. Consider using 'asSequence()' for better performance.", count))}
				}
			}
		}
		return nil
	}

	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
		fmt.Sprintf("Chain of %d collection operations. Consider using 'asSequence()' for better performance.", count))}
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

func (r *ForEachOnRangeRule) NodeTypes() []string { return []string{"call_expression"} }

// rangeInfixOps are the infix operators that create ranges in Kotlin.
var rangeInfixOps = map[string]bool{
	"rangeTo": true, "downTo": true, "until": true,
}

func (r *ForEachOnRangeRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// We want call_expression nodes where the called function is "forEach".
	name := flatCallExpressionName(file, idx)
	if name != "forEach" {
		return nil
	}

	// Get the receiver: the navigation_expression's first child is the receiver.
	navExpr, _ := flatCallExpressionParts(file, idx)
	if navExpr == 0 {
		return nil
	}
	if file.FlatNamedChildCount(navExpr) == 0 {
		return nil
	}
	receiver := file.FlatNamedChild(navExpr, 0)

	// Walk through the receiver chain to see if a range expression is at its root.
	if !containsRangeExpressionFlat(file, receiver) {
		return nil
	}

	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Use a regular 'for' loop instead of '(range).forEach' for better performance.")

	// Auto-fix: for simple cases like (rangeExpr).forEach { body }, rewrite to for loop.
	f.Fix = forEachOnRangeFixFlat(file, idx, receiver)

	return []scanner.Finding{f}
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

	callSuffix := file.FlatFindChild(callNode, "call_suffix")
	if callSuffix == 0 {
		return nil
	}
	lambdaNode := file.FlatFindChild(callSuffix, "annotated_lambda")
	if lambdaNode == 0 {
		lambdaNode = file.FlatFindChild(callSuffix, "lambda_literal")
	}
	if lambdaNode == 0 {
		return nil
	}
	ll := lambdaNode
	if file.FlatType(ll) == "annotated_lambda" {
		ll = file.FlatFindChild(ll, "lambda_literal")
	}
	if ll == 0 {
		return nil
	}

	iterVar := "i"
	params := file.FlatFindChild(ll, "lambda_parameters")
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
	statements := file.FlatFindChild(ll, "statements")
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

func (r *SpreadOperatorRule) NodeTypes() []string { return []string{"spread_expression"} }

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

func (r *SpreadOperatorRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Skip .gradle.kts files — vararg forwarding is the only way to pass
	// collections to many Gradle DSL APIs.
	if strings.HasSuffix(file.Path, ".gradle.kts") {
		return nil
	}
	// The child of spread_expression is the expression being spread.
	if file.FlatChildCount(idx) > 0 {
		child := file.FlatChild(idx, file.FlatChildCount(idx)-1)
		// *arrayOf(...) — child is a call_expression whose first child is the function name.
		if file.FlatType(child) == "call_expression" {
			fnName := flatCallExpressionName(file, child)
			if arrayConstructors[fnName] {
				return nil
			}
			// Any other call_expression result is a computed array being
			// forwarded to a vararg API site. The author cannot avoid the
			// spread without rewriting the callee.
			//   .request(*PermissionCompat.forImagesAndVideos())
			//   ByteString.of(*metadata.sourceServiceId.toByteArray())
			return nil
		}
		// If spreading a simple identifier that matches a vararg parameter
		// of the enclosing function, Kotlin REQUIRES the spread operator —
		// this isn't an optional style choice.
		if file.FlatType(child) == "simple_identifier" {
			name := file.FlatNodeText(child)
			if isEnclosingVarargParamFlat(file, idx, name) {
				return nil
			}
		}
		// If spreading into a database fluent builder call like `.select(...)`
		// or `.where(...)`, the vararg is the API shape — forwarding a
		// `Array<String>` constant projection is idiomatic and has no
		// wrapper-free alternative.
		if isSpreadIntoSqlBuilderFlat(file, idx) {
			return nil
		}
	}
	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Spread operator used. This creates a copy of the array.")}
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
		params := file.FlatFindChild(p, "function_value_parameters")
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

func (r *UnnecessaryInitOnArrayRule) NodeTypes() []string { return []string{"call_expression"} }

var defaultZeroArrayRe = regexp.MustCompile(`(IntArray|LongArray|ShortArray|ByteArray|FloatArray|DoubleArray)\s*\([^)]+\)\s*\{\s*0+\.?0*\s*\}`)
var defaultFalseArrayRe = regexp.MustCompile(`BooleanArray\s*\([^)]+\)\s*\{\s*false\s*\}`)
var defaultCharArrayRe = regexp.MustCompile(`CharArray\s*\([^)]+\)\s*\{\s*'\\u0000'\s*\}`)

var defaultInitRemoveRe = regexp.MustCompile(`((IntArray|LongArray|ShortArray|ByteArray|FloatArray|DoubleArray)\s*\([^)]+\))\s*\{\s*0+\.?0*\s*\}`)
var defaultFalseRemoveRe = regexp.MustCompile(`(BooleanArray\s*\([^)]+\))\s*\{\s*false\s*\}`)
var defaultCharRemoveRe = regexp.MustCompile(`(CharArray\s*\([^)]+\))\s*\{\s*'\\u0000'\s*\}`)

func (r *UnnecessaryInitOnArrayRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	if !defaultZeroArrayRe.MatchString(text) && !defaultFalseArrayRe.MatchString(text) && !defaultCharArrayRe.MatchString(text) {
		return nil
	}
	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Unnecessary initialization. The default value is already the array's default.")
	startByte := int(file.FlatStartByte(idx))
	// Remove the { ... } initializer
	var m []int
	if m = defaultInitRemoveRe.FindStringSubmatchIndex(text); m != nil {
		keep := text[m[2]:m[3]]
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   startByte + m[0],
			EndByte:     startByte + m[1],
			Replacement: keep,
		}
	} else if m = defaultFalseRemoveRe.FindStringSubmatchIndex(text); m != nil {
		keep := text[m[2]:m[3]]
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   startByte + m[0],
			EndByte:     startByte + m[1],
			Replacement: keep,
		}
	} else if m = defaultCharRemoveRe.FindStringSubmatchIndex(text); m != nil {
		keep := text[m[2]:m[3]]
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   startByte + m[0],
			EndByte:     startByte + m[1],
			Replacement: keep,
		}
	}
	return []scanner.Finding{f}
}

// UnnecessaryPartOfBinaryExpressionRule detects x && true, x || false, etc.
type UnnecessaryPartOfBinaryExpressionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Performance rule. Detection pattern-matches anti-patterns (allocation in
// loops, primitive boxing, collection chains) with optional resolver
// support; fallback is heuristic. Classified per roadmap/17.
func (r *UnnecessaryPartOfBinaryExpressionRule) Confidence() float64 { return 0.75 }

func (r *UnnecessaryPartOfBinaryExpressionRule) NodeTypes() []string {
	return []string{"conjunction_expression", "disjunction_expression"}
}

func (r *UnnecessaryPartOfBinaryExpressionRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// conjunction_expression: left && right
	// disjunction_expression: left || right
	if file.FlatNamedChildCount(idx) < 2 {
		return nil
	}
	left := file.FlatNamedChild(idx, 0)
	right := file.FlatNamedChild(idx, file.FlatNamedChildCount(idx)-1)
	leftText := file.FlatNodeText(left)
	rightText := file.FlatNodeText(right)
	isConjunction := file.FlatType(idx) == "conjunction_expression"

	// Check for redundant literals:
	// x && true -> x, true && x -> x
	// x || false -> x, false || x -> x
	var redundant bool
	var keepNode uint32
	if isConjunction {
		if rightText == "true" {
			redundant = true
			keepNode = left
		} else if leftText == "true" {
			redundant = true
			keepNode = right
		}
	} else {
		if rightText == "false" {
			redundant = true
			keepNode = left
		} else if leftText == "false" {
			redundant = true
			keepNode = right
		}
	}
	if !redundant {
		return nil
	}
	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Unnecessary part of binary expression. 'true' or 'false' literal in logical expression is redundant.")
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   int(file.FlatStartByte(idx)),
		EndByte:     int(file.FlatEndByte(idx)),
		Replacement: file.FlatNodeText(keepNode),
	}
	return []scanner.Finding{f}
}

// UnnecessaryTemporaryInstantiationRule detects Integer.valueOf(x).toString() etc.
type UnnecessaryTemporaryInstantiationRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Performance rule. Detection pattern-matches anti-patterns (allocation in
// loops, primitive boxing, collection chains) with optional resolver
// support; fallback is heuristic. Classified per roadmap/17.
func (r *UnnecessaryTemporaryInstantiationRule) Confidence() float64 { return 0.75 }

func (r *UnnecessaryTemporaryInstantiationRule) NodeTypes() []string {
	return []string{"call_expression"}
}

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

func (r *UnnecessaryTemporaryInstantiationRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	src := file.FlatNodeBytes(idx)
	if !looksLikeTempInstantiation(src) {
		return nil
	}
	if flatCallExpressionName(file, idx) != "toString" {
		return nil
	}
	nav := file.FlatFindChild(idx, "navigation_expression")
	if nav == 0 {
		return nil
	}
	innerCall := tempInstantiationReceiverFlat(file, nav)
	if innerCall == 0 || file.FlatType(innerCall) != "call_expression" {
		return nil
	}
	method := flatCallExpressionName(file, innerCall)
	if !tempInstantiationMethods[method] {
		return nil
	}
	innerNav := file.FlatFindChild(innerCall, "navigation_expression")
	if innerNav == 0 {
		return nil
	}
	typeName := tempInstantiationTypeNameFlat(file, innerNav)
	if !tempInstantiationTypeNames[typeName] {
		return nil
	}
	arg := tempInstantiationFirstArgumentFlat(file, innerCall)
	if arg == "" {
		return nil
	}
	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Unnecessary temporary instantiation. Use the type's toString() or conversion method directly.")
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   int(file.FlatStartByte(idx)),
		EndByte:     int(file.FlatEndByte(idx)),
		Replacement: arg + ".toString()",
	}
	return []scanner.Finding{f}
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
	callSuffix := file.FlatFindChild(call, "call_suffix")
	if callSuffix == 0 {
		return ""
	}
	valueArgs := file.FlatFindChild(callSuffix, "value_arguments")
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
	resolver typeinfer.TypeResolver
}

func (r *UnnecessaryTypeCastingRule) SetResolver(res typeinfer.TypeResolver) { r.resolver = res }

// Confidence reports a tier-2 (medium) base confidence — flags casts
// that are no-ops; needs the resolver to confirm the source type matches
// the target, falls back to textual comparison. Classified per roadmap/17.
func (r *UnnecessaryTypeCastingRule) Confidence() float64 { return 0.75 }

func (r *UnnecessaryTypeCastingRule) NodeTypes() []string { return []string{"as_expression"} }

func (r *UnnecessaryTypeCastingRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// as_expression has the expression and the target type
	text := file.FlatNodeText(idx)
	parts := strings.Split(text, " as ")
	if len(parts) != 2 {
		return nil
	}
	target := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(parts[1]), "?"))
	expr := strings.TrimSpace(parts[0])

	// Type-aware check: resolve source and target FQNs
	if r.resolver != nil && file.FlatChildCount(idx) >= 2 {
		exprType := flatResolveByName(file, r.resolver, file.FlatChild(idx, 0))
		targetType := flatResolveByName(file, r.resolver, file.FlatChild(idx, file.FlatChildCount(idx)-1))
		if exprType != nil && targetType != nil &&
			exprType.Kind != typeinfer.TypeUnknown && targetType.Kind != typeinfer.TypeUnknown {
			if exprType.FQN == targetType.FQN {
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					fmt.Sprintf("Unnecessary cast to '%s'. The expression is already of this type.", target))
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(idx)),
					EndByte:     int(file.FlatEndByte(idx)),
					Replacement: expr,
				}
				return []scanner.Finding{f}
			}
			// Types differ — cast is needed
			return nil
		}
	}

	// Fallback: text-based heuristics
	matched := false
	// Simple heuristic 1: check if the expression ends with : Type matching the cast
	if strings.HasSuffix(expr, ": "+target) || strings.HasSuffix(expr, ":"+target) {
		matched = true
	}
	// Heuristic 2: check if the parent is a property_declaration with a matching type annotation
	if !matched {
		for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
			if file.FlatType(parent) == "property_declaration" || file.FlatType(parent) == "function_declaration" {
				parentText := file.FlatNodeText(parent)
				// Look for `: Type =` pattern before the as expression
				declType := ""
				if idx := strings.Index(parentText, ":"); idx >= 0 {
					rest := parentText[idx+1:]
					if eqIdx := strings.Index(rest, "="); eqIdx >= 0 {
						declType = strings.TrimSpace(rest[:eqIdx])
					}
				}
				if declType == target {
					matched = true
				}
				break
			}
		}
	}

	if matched {
		f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
			fmt.Sprintf("Unnecessary cast to '%s'. The expression is already of this type.", target))
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   int(file.FlatStartByte(idx)),
			EndByte:     int(file.FlatEndByte(idx)),
			Replacement: expr,
		}
		return []scanner.Finding{f}
	}
	return nil
}
