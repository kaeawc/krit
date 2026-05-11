package rules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// ---------------------------------------------------------------------------
// looksLikeEnumConstantRef returns true if the text looks like a qualified
// reference to an enum constant: `Type.CONSTANT` or `pkg.Type.CONSTANT`,
// where the last segment is ALL_CAPS.
func looksLikeEnumConstantRef(text string) bool {
	if !strings.Contains(text, ".") {
		return false
	}
	dotIdx := strings.LastIndex(text, ".")
	last := text[dotIdx+1:]
	return looksLikeUppercaseConstantName(last)
}

func looksLikeUppercaseConstantName(text string) bool {
	if text == "" {
		return false
	}
	// Must be all uppercase letters, digits, or underscores
	for _, c := range text {
		if (c >= 'A' && c <= 'Z') || c == '_' || (c >= '0' && c <= '9') {
			continue
		}
		return false
	}
	// Must contain at least one letter
	for _, c := range text {
		if c >= 'A' && c <= 'Z' {
			return true
		}
	}
	return false
}

func looksLikeSentinelObjectRef(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	return file.FlatType(idx) == "simple_identifier" && looksLikeUppercaseConstantName(file.FlatNodeText(idx))
}

func looksLikeSentinelAliasRef(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "simple_identifier" {
		return false
	}
	name := file.FlatNodeText(idx)
	if name == "" || looksLikeUppercaseConstantName(name) {
		return false
	}
	scope := referentialEqualityAliasSearchScope(file, idx)
	if scope == 0 {
		return false
	}
	found := false
	useStart := file.FlatStartByte(idx)
	file.FlatWalkNodes(scope, "property_declaration", func(prop uint32) {
		if found || file.FlatStartByte(prop) >= useStart || propertyDeclarationIsVar(file, prop) {
			return
		}
		if propertyDeclarationName(file, prop) != name {
			return
		}
		init := propertyInitializerExpression(file, prop)
		if init == 0 {
			return
		}
		found = looksLikeSentinelInitializer(file.FlatNodeText(init))
	})
	return found
}

func referentialEqualityAliasSearchScope(file *scanner.File, idx uint32) uint32 {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "function_declaration", "lambda_literal", "source_file":
			return p
		case "class_declaration", "object_declaration":
			return 0
		}
	}
	return 0
}

func looksLikeSentinelInitializer(text string) bool {
	text = strings.TrimSpace(text)
	for strings.HasPrefix(text, "(") && strings.HasSuffix(text, ")") && len(text) > 1 {
		text = strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(text, ")"), "("))
	}
	if looksLikeUppercaseConstantName(text) || looksLikeEnumConstantRef(text) {
		return true
	}
	if dot := strings.LastIndex(text, "."); dot >= 0 {
		return looksLikeUppercaseConstantName(text[dot+1:])
	}
	return false
}

func looksLikeThisFieldIdentityCheck(file *scanner.File, left uint32, right uint32) bool {
	return looksLikeThisFieldIdentityOperand(file.FlatNodeText(left)) ||
		looksLikeThisFieldIdentityOperand(file.FlatNodeText(right))
}

func looksLikeThisFieldIdentityOperand(text string) bool {
	text = strings.TrimSpace(text)
	for strings.HasPrefix(text, "(") && strings.HasSuffix(text, ")") && len(text) > 1 {
		text = strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(text, ")"), "("))
	}
	return strings.HasPrefix(text, "this.")
}

func looksLikeIterationIdentityCheck(file *scanner.File, idx uint32, left uint32, right uint32) bool {
	lambda := uint32(0)
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "lambda_literal":
			lambda = p
		case "function_declaration", "class_declaration", "object_declaration", "source_file":
			return false
		}
		if lambda != 0 {
			break
		}
	}
	if lambda == 0 || !referentialEqualityLambdaBelongsToIterationCall(file, lambda) {
		return false
	}
	params := referentialEqualityLambdaParameterNames(file, lambda)
	if len(params) == 0 {
		params["it"] = true
	}
	return referentialEqualityOperandIsLambdaParameter(file, left, params) ||
		referentialEqualityOperandIsLambdaParameter(file, right, params)
}

func looksLikeArrayElementIdentityCheck(file *scanner.File, left uint32, right uint32) bool {
	return file.FlatType(left) == "indexing_expression" ||
		file.FlatType(right) == "indexing_expression" ||
		looksLikeIndexedOperandText(file.FlatNodeText(left)) ||
		looksLikeIndexedOperandText(file.FlatNodeText(right))
}

func looksLikeIndexedOperandText(text string) bool {
	text = strings.TrimSpace(text)
	return strings.Contains(text, "[") && strings.Contains(text, "]")
}

func looksLikeSingletonTypeIdentityCheck(file *scanner.File, left uint32, right uint32) bool {
	return (file.FlatType(left) == "this_expression" && looksLikeSingletonObjectRefText(file.FlatNodeText(right))) ||
		(file.FlatType(right) == "this_expression" && looksLikeSingletonObjectRefText(file.FlatNodeText(left))) ||
		looksLikeSingletonObjectRefText(file.FlatNodeText(left)) ||
		looksLikeSingletonObjectRefText(file.FlatNodeText(right))
}

func looksLikeSingletonObjectRefText(text string) bool {
	text = strings.TrimSpace(text)
	for strings.HasPrefix(text, "(") && strings.HasSuffix(text, ")") && len(text) > 1 {
		text = strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(text, ")"), "("))
	}
	if text == "" || strings.ContainsAny(text, ".[]() ") {
		if strings.ContainsAny(text, "[]() ") {
			return false
		}
		parts := strings.Split(text, ".")
		if len(parts) < 2 {
			return false
		}
		for _, part := range parts {
			if !looksLikeUpperCamelIdentifier(part) {
				return false
			}
		}
		return true
	}
	return looksLikeUpperCamelIdentifier(text)
}

func looksLikeUpperCamelIdentifier(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" || strings.ContainsAny(text, ".[]() ") {
		return false
	}
	first := text[0]
	return first >= 'A' && first <= 'Z'
}

func looksLikeResourceCleanupIdentityGuard(file *scanner.File, idx uint32, opType string) bool {
	if opType != "!==" {
		return false
	}
	var ifExpr uint32
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "if_expression":
			ifExpr = p
		case "function_declaration", "class_declaration", "object_declaration", "source_file":
			if ifExpr == 0 || !referentialEqualityIsIfCondition(file, idx, ifExpr) {
				return false
			}
			text := file.FlatNodeText(ifExpr)
			return strings.Contains(text, ".recycle()") || strings.Contains(text, ".close()")
		}
		if ifExpr != 0 {
			break
		}
	}
	if ifExpr == 0 || !referentialEqualityIsIfCondition(file, idx, ifExpr) {
		return false
	}
	text := file.FlatNodeText(ifExpr)
	return strings.Contains(text, ".recycle()") || strings.Contains(text, ".close()")
}

func referentialEqualityLambdaBelongsToIterationCall(file *scanner.File, lambda uint32) bool {
	for p, ok := file.FlatParent(lambda); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "call_expression":
			return referentialEqualityIterationCalls[flatCallNameAny(file, p)]
		case "function_declaration", "class_declaration", "object_declaration", "source_file":
			return false
		}
	}
	return false
}

var referentialEqualityIterationCalls = map[string]bool{
	"forEach": true, "forEachIndexed": true,
	"map": true, "mapNotNull": true, "flatMap": true, "flatMapTo": true,
	"filter": true, "filterNot": true, "filterIsInstance": true,
	"collect": true, "collectLatest": true,
	"any": true, "all": true, "none": true, "find": true, "firstOrNull": true,
}

func referentialEqualityLambdaParameterNames(file *scanner.File, lambda uint32) map[string]bool {
	params := make(map[string]bool)
	text := file.FlatNodeText(lambda)
	arrow := strings.Index(text, "->")
	if arrow < 0 {
		return params
	}
	prefix := strings.TrimSpace(strings.TrimPrefix(text[:arrow], "{"))
	prefix = strings.TrimPrefix(prefix, "(")
	prefix = strings.TrimSuffix(prefix, ")")
	for _, part := range strings.Split(prefix, ",") {
		name := strings.TrimSpace(part)
		if colon := strings.Index(name, ":"); colon >= 0 {
			name = strings.TrimSpace(name[:colon])
		}
		if name != "" && name != "_" {
			params[name] = true
		}
	}
	return params
}

func referentialEqualityOperandIsLambdaParameter(file *scanner.File, operand uint32, params map[string]bool) bool {
	if file.FlatType(operand) != "simple_identifier" {
		return false
	}
	return params[file.FlatNodeText(operand)]
}

func isInsideEqualsMethodFlatType(file *scanner.File, idx uint32) bool {
	fn, ok := flatEnclosingFunction(file, idx)
	if !ok {
		return false
	}
	return extractIdentifierFlat(file, fn) == "equals"
}

func walkFlatClassMembers(file *scanner.File, parent uint32, fn func(uint32)) {
	if file == nil || parent == 0 || fn == nil {
		return
	}
	file.FlatForEachChild(parent, func(child uint32) {
		switch file.FlatType(child) {
		case "function_declaration":
			fn(child)
		case "class_member_declarations":
			walkFlatClassMembers(file, child, fn)
		}
	})
}

// AvoidReferentialEqualityRule detects === or !== usage.
// With type inference: only flags when operands are String (where referential
// equality is almost certainly a bug). Without resolver, flags all === / !==.
// ---------------------------------------------------------------------------
type AvoidReferentialEqualityRule struct {
	FlatDispatchBase
	BaseRule
	ForbiddenTypePatterns []string
}

// referentialEqualityCompileFQNPatterns turns the user-configured glob
// patterns into anchored regexes: `*` -> `.*`, `?` -> `.`, other regex
// specials escaped, and the whole pattern anchored. Invalid patterns are
// skipped silently rather than failing the rule entirely.
func referentialEqualityCompileFQNPatterns(patterns []string) []*regexp.Regexp {
	if len(patterns) == 0 {
		return nil
	}
	out := make([]*regexp.Regexp, 0, len(patterns))
	for _, raw := range patterns {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		var b strings.Builder
		b.WriteByte('^')
		for _, c := range raw {
			switch c {
			case '*':
				b.WriteString(".*")
			case '?':
				b.WriteByte('.')
			case '\\', '.', '+', '(', ')', '[', ']', '{', '}', '|', '^', '$':
				b.WriteByte('\\')
				b.WriteRune(c)
			default:
				b.WriteRune(c)
			}
		}
		b.WriteByte('$')
		re, err := regexp.Compile(b.String())
		if err == nil && re != nil {
			out = append(out, re)
		}
	}
	return out
}

// referentialEqualityResolvedFQN extracts the fully-qualified type name
// from a resolver result, preferring the FQN field but falling back to
// the simple name when no FQN was attached. Returns "" for unknown or
// nil results.
func referentialEqualityResolvedFQN(t *typeinfer.ResolvedType) string {
	if t == nil || t.Kind == typeinfer.TypeUnknown {
		return ""
	}
	if t.FQN != "" {
		return t.FQN
	}
	return t.Name
}

// referentialEqualityFQNMatchesAny reports whether the given fully-
// qualified type name matches any of the compiled patterns.
func referentialEqualityFQNMatchesAny(fqn string, patterns []*regexp.Regexp) bool {
	if fqn == "" || len(patterns) == 0 {
		return false
	}
	for _, p := range patterns {
		if p.MatchString(fqn) {
			return true
		}
	}
	return false
}

// equalsFamilyCallNames is the set of callees whose presence in a
// boolean expression means an accompanying referential compare is
// almost certainly the short-circuit fast-path, not a mistake.
var equalsFamilyCallNames = map[string]bool{
	"equals":            true,
	"hasSameContent":    true,
	"contentEquals":     true,
	"contentDeepEquals": true,
	"sameContentAs":     true,
}

// equalityOperands returns (left, operator, right) of an equality_expression.
// The operator is the token child whose type is one of `==`, `!=`, `===`,
// `!==`. Returns zeros if the expression doesn't have all three parts.
func equalityOperands(file *scanner.File, idx uint32) (left, op, right uint32) {
	if file == nil || file.FlatType(idx) != "equality_expression" {
		return 0, 0, 0
	}
	op = equalityOperatorChild(file, idx)
	if op == 0 {
		return 0, 0, 0
	}
	opStart := file.FlatStartByte(op)
	opEnd := file.FlatEndByte(op)
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		if file.FlatEndByte(child) <= opStart {
			left = child
		} else if right == 0 && file.FlatStartByte(child) >= opEnd {
			right = child
		}
	}
	return left, op, right
}

func equalityOperatorChild(file *scanner.File, idx uint32) uint32 {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "==", "!=", "===", "!==":
			return child
		}
	}
	return 0
}

// enclosingBoolExprHasEqualsCall returns true when the referential-equality
// node sits under a conjunction/disjunction whose other operand is a call
// to one of the named equals-family functions. Structural replacement for
// the old `strings.Contains(parentText, ".equals(")` heuristic — by walking
// the AST we avoid matching `.equals(` that appears in a string literal or
// a comment.
func enclosingBoolExprHasEqualsCall(file *scanner.File, idx uint32, names map[string]bool) bool {
	parent, ok := file.FlatParent(idx)
	if !ok {
		return false
	}
	pt := file.FlatType(parent)
	if pt != "disjunction_expression" && pt != "conjunction_expression" {
		return false
	}
	found := false
	file.FlatWalkAllNodes(parent, func(n uint32) {
		if found || n == idx {
			return
		}
		if file.FlatType(n) == "call_expression" {
			if _, ok := names[flatCallExpressionName(file, n)]; ok {
				found = true
			}
		}
	})
	return found
}

func isComparatorIdentityFastPath(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	var ifExpr uint32
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "if_expression":
			ifExpr = p
		case "function_declaration":
			name := flatFunctionName(file, p)
			if name != "compareTo" && name != "compare" {
				return false
			}
			if ifExpr == 0 || !referentialEqualityIsIfCondition(file, idx, ifExpr) {
				return false
			}
			if !ifExpressionReturnsZero(file, ifExpr) {
				return false
			}
			return ifExpressionIsFirstStatementInFunction(file, ifExpr, p)
		case "source_file", "class_declaration", "object_declaration":
			return false
		}
	}
	return false
}

func referentialEqualityIsIfCondition(file *scanner.File, idx uint32, ifExpr uint32) bool {
	for child := file.FlatFirstChild(ifExpr); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "control_structure_body" {
			return file.FlatEndByte(idx) <= file.FlatStartByte(child)
		}
	}
	return false
}

func ifExpressionReturnsZero(file *scanner.File, ifExpr uint32) bool {
	text := file.FlatNodeText(ifExpr)
	return strings.Contains(text, "return 0") || strings.Contains(text, "-> 0")
}

func ifExpressionIsFirstStatementInFunction(file *scanner.File, ifExpr uint32, fn uint32) bool {
	for p, ok := file.FlatParent(ifExpr); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "statements":
			for child := file.FlatFirstChild(p); child != 0; child = file.FlatNextSib(child) {
				if !file.FlatIsNamed(child) {
					continue
				}
				return child == ifExpr
			}
			return false
		case "function_declaration":
			return p == fn
		}
	}
	return false
}

// Confidence reports a tier-2 (medium) base confidence — flags === / !==
// on value types; needs resolver to confirm operand types, falls back to a
// name-based heuristic. Classified per roadmap/17.
func (r *AvoidReferentialEqualityRule) Confidence() float64 { return 0.75 }

// ---------------------------------------------------------------------------
// DoubleMutabilityForCollectionRule detects var with mutable collection type.
// ---------------------------------------------------------------------------
type DoubleMutabilityForCollectionRule struct {
	FlatDispatchBase
	BaseRule
	MutableTypes []string
}

// Confidence reports a tier-2 (medium) base confidence. The rule uses a fixed
// allow-list of mutable-collection type names and factory function names; it
// correctly flags the common case (`var xs = mutableListOf<Foo>()`) with local
// AST/import evidence, but won't detect the pattern when the collection is
// returned from a wrapper function. Medium confidence matches the known
// scope-analysis gap.
func (r *DoubleMutabilityForCollectionRule) Confidence() float64 { return 0.75 }

var defaultDoubleMutableTypes = []string{
	"MutableList", "MutableSet", "MutableMap", "MutableCollection",
	"ArrayList", "HashMap", "HashSet", "LinkedHashMap", "LinkedHashSet",
}

var mutableCollectionFactories = map[string]bool{
	"mutableListOf": true,
	"mutableSetOf":  true,
	"mutableMapOf":  true,
	"arrayListOf":   true,
	"hashMapOf":     true,
	"hashSetOf":     true,
	"linkedMapOf":   true,
	"linkedSetOf":   true,
	"ArrayList":     true,
	"HashMap":       true,
	"HashSet":       true,
	"LinkedHashMap": true,
	"LinkedHashSet": true,
}

func simpleTypeName(text string) string {
	text = strings.TrimSpace(text)
	text = strings.TrimSuffix(text, "?")
	if idx := strings.Index(text, "<"); idx >= 0 {
		text = text[:idx]
	}
	if idx := strings.LastIndex(text, "."); idx >= 0 {
		text = text[idx+1:]
	}
	return strings.TrimSpace(text)
}

func (r *DoubleMutabilityForCollectionRule) configuredMutableTypes() map[string]bool {
	types := r.MutableTypes
	if len(types) == 0 {
		types = defaultDoubleMutableTypes
	}
	out := make(map[string]bool, len(types))
	for _, t := range types {
		if t == "" {
			continue
		}
		out[simpleTypeName(t)] = true
	}
	return out
}

func doubleMutabilityHasExplicitMutableType(file *scanner.File, varDecl uint32, mutableTypes map[string]bool) bool {
	if file == nil || varDecl == 0 {
		return false
	}
	for child := file.FlatFirstChild(varDecl); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "user_type", "nullable_type":
			if doubleMutabilityTypeNodeHasMutableEvidence(file, child, mutableTypes) {
				return true
			}
		}
	}
	return false
}

func doubleMutabilityTypeNodeHasMutableEvidence(file *scanner.File, typeNode uint32, mutableTypes map[string]bool) bool {
	typeText := file.FlatNodeText(typeNode)
	return doubleMutabilityTypeTextHasMutableEvidence(file, typeText, mutableTypes)
}

func doubleMutabilityTypeTextHasMutableEvidence(file *scanner.File, typeText string, mutableTypes map[string]bool) bool {
	typeName := strings.TrimSpace(typeText)
	typeName = strings.TrimSuffix(typeName, "?")
	if idx := strings.Index(typeName, "<"); idx >= 0 {
		typeName = strings.TrimSpace(typeName[:idx])
	}
	simple := simpleTypeName(typeName)
	if simple == "" || !mutableTypes[simple] {
		return false
	}
	if strings.Contains(typeName, ".") {
		return knownMutableCollectionFQN(typeName) || !knownDefaultMutableCollectionName(simple)
	}
	if !knownDefaultMutableCollectionName(simple) {
		return true
	}
	return !doubleMutabilitySimpleTypeShadowed(file, simple)
}

func knownDefaultMutableCollectionName(name string) bool {
	for _, typ := range defaultDoubleMutableTypes {
		if typ == name {
			return true
		}
	}
	return false
}

func knownMutableCollectionFQN(fqn string) bool {
	switch fqn {
	case "kotlin.collections.MutableList",
		"kotlin.collections.MutableSet",
		"kotlin.collections.MutableMap",
		"kotlin.collections.MutableCollection",
		"kotlin.collections.ArrayList",
		"kotlin.collections.HashMap",
		"kotlin.collections.HashSet",
		"kotlin.collections.LinkedHashMap",
		"kotlin.collections.LinkedHashSet",
		"java.util.ArrayList",
		"java.util.HashMap",
		"java.util.HashSet",
		"java.util.LinkedHashMap",
		"java.util.LinkedHashSet":
		return true
	default:
		return false
	}
}

// doubleMutabilityShadowSet holds the per-file set of simple type names
// that are shadowed by a local declaration or non-collection import,
// plus a sentinel indicating an unrelated wildcard import (which makes
// every name potentially shadowed).
type doubleMutabilityShadowSet struct {
	declared       map[string]struct{}
	importedShadow map[string]struct{}
	wildcardShadow bool
}

func doubleMutabilityShadows(file *scanner.File) *doubleMutabilityShadowSet {
	return filefacts.FileFact(fileFactsCache(), file, slotDoubleMutabilityShadows, func() *doubleMutabilityShadowSet {
		set := &doubleMutabilityShadowSet{
			declared:       map[string]struct{}{},
			importedShadow: map[string]struct{}{},
		}
		for _, nodeType := range []string{"class_declaration", "object_declaration", "interface_declaration", "type_alias"} {
			file.FlatWalkNodes(0, nodeType, func(node uint32) {
				if name := extractIdentifierFlat(file, node); name != "" {
					set.declared[name] = struct{}{}
				}
			})
		}
		file.FlatWalkNodes(0, "import_header", func(node uint32) {
			text := cleanImportHeaderTextWithAlias(file.FlatNodeText(node))
			if alias := strings.Index(text, " as "); alias >= 0 {
				fqn := strings.TrimSpace(text[:alias])
				asName := strings.TrimSpace(text[alias+4:])
				if asName != "" && !knownMutableCollectionFQN(fqn) {
					set.importedShadow[asName] = struct{}{}
				}
				return
			}
			if strings.HasSuffix(text, ".*") {
				pkg := strings.TrimSuffix(text, ".*")
				if pkg != "kotlin.collections" && pkg != "java.util" {
					set.wildcardShadow = true
				}
				return
			}
			if dot := strings.LastIndex(text, "."); dot >= 0 && !knownMutableCollectionFQN(text) {
				set.importedShadow[text[dot+1:]] = struct{}{}
			}
		})
		return set
	})
}

func doubleMutabilitySimpleTypeShadowed(file *scanner.File, simple string) bool {
	if file == nil || simple == "" {
		return false
	}
	set := doubleMutabilityShadows(file)
	if _, ok := set.declared[simple]; ok {
		return true
	}
	if _, ok := set.importedShadow[simple]; ok {
		return true
	}
	return set.wildcardShadow
}

func doubleMutabilityInitializerLooksLikeMutableFactory(file *scanner.File, prop uint32) bool {
	init := propertyInitializerExpression(file, prop)
	if init == 0 || file.FlatType(init) != "call_expression" {
		return false
	}
	return mutableCollectionFactories[flatCallExpressionName(file, init)]
}

// ---------------------------------------------------------------------------
// EqualsAlwaysReturnsTrueOrFalseRule detects equals() that always returns true/false.
// ---------------------------------------------------------------------------
type EqualsAlwaysReturnsTrueOrFalseRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Potential-bugs types rule. Detection pattern-matches type-related
// constructs; resolver usage when available improves precision but
// fallback is heuristic. Classified per roadmap/17.
func (r *EqualsAlwaysReturnsTrueOrFalseRule) Confidence() float64 { return 0.75 }

// ---------------------------------------------------------------------------
// EqualsWithHashCodeExistRule detects equals without hashCode or vice versa.
// ---------------------------------------------------------------------------
type EqualsWithHashCodeExistRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Potential-bugs types rule. Detection pattern-matches type-related
// constructs; resolver usage when available improves precision but
// fallback is heuristic. Classified per roadmap/17.
func (r *EqualsWithHashCodeExistRule) Confidence() float64 { return 0.75 }

// ---------------------------------------------------------------------------
// WrongEqualsTypeParameterRule detects equals(other: String) instead of Any?.
// ---------------------------------------------------------------------------
type WrongEqualsTypeParameterRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *WrongEqualsTypeParameterRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if !file.FlatHasModifier(idx, "override") {
		return
	}
	if extractIdentifierFlat(file, idx) != "equals" {
		return
	}
	params, ok := file.FlatFindChild(idx, "function_value_parameters")
	if !ok {
		return
	}
	var firstParam uint32
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "parameter" {
			firstParam = child
			break
		}
	}
	if firstParam == 0 {
		return
	}
	var typeNode uint32
	isNullable := false
	for child := file.FlatFirstChild(firstParam); child != 0; child = file.FlatNextSib(child) {
		ct := file.FlatType(child)
		if ct == "nullable_type" {
			typeNode = child
			isNullable = true
			break
		}
		if ct == "user_type" {
			typeNode = child
			break
		}
	}
	if typeNode == 0 {
		return
	}
	typeText := file.FlatNodeText(typeNode)
	typeName := typeText
	if i := strings.Index(typeName, "?"); i >= 0 {
		typeName = typeName[:i]
	}
	if i := strings.Index(typeName, "<"); i >= 0 {
		typeName = typeName[:i]
	}
	typeName = strings.TrimSpace(typeName)
	if isNullable && typeName == "Any" {
		return
	}
	f := r.Finding(file, file.FlatRow(firstParam)+1, file.FlatCol(firstParam)+1,
		fmt.Sprintf("equals() parameter type is '%s' instead of 'Any?'. This does not properly override Any.equals().", typeText))
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   int(file.FlatStartByte(typeNode)),
		EndByte:     int(file.FlatEndByte(typeNode)),
		Replacement: "Any?",
	}
	ctx.Emit(f)
}

// ---------------------------------------------------------------------------
// CharArrayToStringCallRule detects charArray.toString() calls.
// ---------------------------------------------------------------------------
type CharArrayToStringCallRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *CharArrayToStringCallRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if flatCallExpressionName(file, idx) != "toString" {
		return
	}
	navExpr, args := flatCallExpressionParts(file, idx)
	if navExpr == 0 {
		return
	}
	// Verify 0 arguments
	if args != 0 {
		for child := file.FlatFirstChild(args); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "value_argument" {
				return
			}
		}
	}
	// Get receiver: first named non-navigation_suffix child of navigation_expression
	var receiver uint32
	for child := file.FlatFirstChild(navExpr); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		if file.FlatType(child) != "navigation_suffix" {
			receiver = child
			break
		}
	}
	if receiver == 0 {
		return
	}
	if ctx.Resolver != nil {
		receiverText := file.FlatNodeText(receiver)
		simpleName := receiverText
		if dotIdx := strings.LastIndex(simpleName, "."); dotIdx >= 0 {
			simpleName = simpleName[dotIdx+1:]
		}
		resolved := ctx.Resolver.ResolveByNameFlat(simpleName, idx, file)
		if resolved != nil && resolved.Kind != typeinfer.TypeUnknown {
			if resolved.Name == "CharArray" || resolved.FQN == "kotlin.CharArray" {
				// Resolver confirmed CharArray type — high confidence.
				r.emitCharArrayFinding(ctx, idx, file, receiver, 0.9)
			}
			return
		}
	}
	// No resolver: heuristic fallback on variable name / declaration.
	if charArrayReceiverFlat(file, receiver) {
		r.emitCharArrayFinding(ctx, idx, file, receiver, 0.8)
	}
}

func (r *CharArrayToStringCallRule) emitCharArrayFinding(ctx *api.Context, callIdx uint32, file *scanner.File, receiver uint32, confidence float64) {
	receiverText := file.FlatNodeText(receiver)
	f := r.Finding(file, file.FlatRow(callIdx)+1, file.FlatCol(callIdx)+1,
		"Calling toString() on a CharArray does not return the string representation. Use String(charArray) instead.")
	f.Confidence = confidence
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   int(file.FlatStartByte(callIdx)),
		EndByte:     int(file.FlatEndByte(callIdx)),
		Replacement: "String(" + receiverText + ")",
	}
	ctx.Emit(f)
}

// charArrayReceiverFlat returns true when receiver appears to be a CharArray:
// either a direct charArrayOf() call, or a same-file identifier whose declaration
// has a CharArray type annotation or charArrayOf() initializer.
func charArrayReceiverFlat(file *scanner.File, receiver uint32) bool {
	rt := file.FlatType(receiver)

	// Direct charArrayOf() call expression
	if rt == "call_expression" {
		return flatCallExpressionName(file, receiver) == "charArrayOf"
	}
	if rt != "simple_identifier" {
		return false
	}
	receiverName := file.FlatNodeText(receiver)

	found := false
	// Check function parameters with CharArray type annotation
	file.FlatWalkNodes(0, "parameter", func(pIdx uint32) {
		if found {
			return
		}
		id, ok := file.FlatFindChild(pIdx, "simple_identifier")
		if !ok || file.FlatNodeText(id) != receiverName {
			return
		}
		for child := file.FlatFirstChild(pIdx); child != 0; child = file.FlatNextSib(child) {
			if charArrayTypeNode(file, child) {
				found = true
				return
			}
		}
	})
	if found {
		return true
	}
	// Check property/local variable declarations
	file.FlatWalkNodes(0, "property_declaration", func(pIdx uint32) {
		if found {
			return
		}
		varDecl, ok := file.FlatFindChild(pIdx, "variable_declaration")
		if !ok {
			return
		}
		id, ok := file.FlatFindChild(varDecl, "simple_identifier")
		if !ok || file.FlatNodeText(id) != receiverName {
			return
		}
		// Explicit CharArray type annotation
		for child := file.FlatFirstChild(varDecl); child != 0; child = file.FlatNextSib(child) {
			if charArrayTypeNode(file, child) {
				found = true
				return
			}
		}
		// charArrayOf() initializer
		if init := propertyInitializerExpression(file, pIdx); expressionContainsCallNamed(file, init, "charArrayOf") {
			found = true
		}
	})
	return found
}

func charArrayTypeNode(file *scanner.File, idx uint32) bool {
	switch file.FlatType(idx) {
	case "user_type", "nullable_type":
		return simpleTypeName(file.FlatNodeText(idx)) == "CharArray"
	default:
		return false
	}
}

func expressionContainsCallNamed(file *scanner.File, idx uint32, name string) bool {
	if file == nil || idx == 0 || name == "" {
		return false
	}
	if file.FlatType(idx) == "call_expression" && flatCallExpressionName(file, idx) == name {
		return true
	}
	found := false
	file.FlatWalkNodes(idx, "call_expression", func(call uint32) {
		if !found && flatCallExpressionName(file, call) == name {
			found = true
		}
	})
	return found
}

// ---------------------------------------------------------------------------
// DontDowncastCollectionTypesRule detects `as MutableList`, `as MutableMap`, etc.
// With type inference: uses ClassHierarchy on source and target of a cast to verify
// the source is a supertype of the target in the collection hierarchy.
// ---------------------------------------------------------------------------
type DontDowncastCollectionTypesRule struct {
	FlatDispatchBase
	BaseRule
}

var mutableCollectionToMethodMap = map[string]string{
	"MutableList":         "toMutableList()",
	"MutableSet":          "toMutableSet()",
	"MutableMap":          "toMutableMap()",
	"MutableCollection":   "toMutableList()",
	"MutableIterator":     "toMutableList().iterator()",
	"MutableListIterator": "toMutableList().listIterator()",
	"MutableIterable":     "toMutableList()",
}

// immutableToMutableMap maps immutable collection types to their mutable counterparts.
var immutableToMutableMap = map[string]string{
	"List": "MutableList", "Set": "MutableSet", "Map": "MutableMap",
	"Collection": "MutableCollection", "Iterable": "MutableIterable",
	"Iterator": "MutableIterator", "ListIterator": "MutableListIterator",
}

func (r *DontDowncastCollectionTypesRule) check(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	if file == nil || idx == 0 {
		return
	}

	// Walk as_expression children: find source expression (before 'as') and type node (after 'as').
	var sourceIdx, typeNode uint32
	seenAs := false
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		ct := file.FlatType(child)
		if ct == "as" || ct == "as?" {
			seenAs = true
			continue
		}
		if !file.FlatIsNamed(child) {
			continue
		}
		if !seenAs {
			sourceIdx = child
		} else if typeNode == 0 {
			typeNode = child
		}
	}
	if typeNode == 0 {
		return
	}

	// Resolve the base user_type (unwrap nullable_type if needed).
	baseType := typeNode
	if file.FlatType(typeNode) == "nullable_type" {
		inner, ok := file.FlatFindChild(typeNode, "user_type")
		if !ok {
			return
		}
		baseType = inner
	}
	if file.FlatType(baseType) != "user_type" {
		return
	}
	typeIdent, ok := file.FlatFindChild(baseType, "type_identifier")
	if !ok {
		return
	}
	targetType := file.FlatNodeText(typeIdent)

	if _, ok := mutableCollectionToMethodMap[targetType]; !ok {
		return
	}

	skip, resolverConfirmed := dontDowncastCheckResolver(ctx, file, sourceIdx, targetType)
	if skip {
		return
	}

	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		fmt.Sprintf("Don't downcast collection type to '%s'. This can lead to unexpected mutations.", targetType))
	if resolverConfirmed {
		f.Confidence = 0.9
	} else {
		f.Confidence = 0.8
	}

	if sourceIdx != 0 {
		expr := file.FlatNodeText(sourceIdx)
		if method, ok := mutableCollectionToMethodMap[targetType]; ok {
			f.Fix = &scanner.Fix{
				ByteMode:    true,
				StartByte:   int(file.FlatStartByte(idx)),
				EndByte:     int(file.FlatEndByte(idx)),
				Replacement: expr + "." + method,
			}
		}
	}

	ctx.Emit(f)
}

func dontDowncastCheckResolver(ctx *api.Context, file *scanner.File, sourceIdx uint32, targetType string) (skip bool, confirmed bool) {
	if ctx.Resolver == nil || sourceIdx == 0 {
		return false, false
	}
	sourceType := ctx.Resolver.ResolveFlatNode(sourceIdx, file)
	if sourceType.Kind == typeinfer.TypeUnknown {
		return false, false
	}
	if _, alreadyMutable := mutableCollectionToMethodMap[sourceType.Name]; alreadyMutable {
		return true, false
	}
	expectedImmutable := ""
	for immutable, mutable := range immutableToMutableMap {
		if mutable == targetType {
			expectedImmutable = immutable
			break
		}
	}
	if expectedImmutable != "" && sourceType.Name != expectedImmutable {
		info := ctx.Resolver.ClassHierarchy(sourceType.Name)
		if info != nil {
			for _, st := range info.Supertypes {
				parts := strings.Split(st, ".")
				if parts[len(parts)-1] == expectedImmutable {
					return false, true
				}
			}
			return true, false
		}
	}
	return false, true
}

// ---------------------------------------------------------------------------
// ImplicitUnitReturnTypeRule detects block-body functions without explicit Unit return type.
// ---------------------------------------------------------------------------
type ImplicitUnitReturnTypeRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence because this
// rule fires on block-body function_declarations lacking an explicit
// return type. That includes @Composable functions, which conventionally
// omit ': Unit' — a convention mismatch rather than a bug, but one that
// produces noise on Compose codebases. Medium confidence keeps it off
// default-strict gates without taking it out of the rule set.
func (r *ImplicitUnitReturnTypeRule) Confidence() float64 { return 0.75 }

// ---------------------------------------------------------------------------
// ElseCaseInsteadOfExhaustiveWhenRule detects when with else on enum/sealed.
// With type inference: checks if ALL sealed/enum variants are covered in the
// when branches. If they are, the else is truly unnecessary. Without resolver,
// falls back to the heuristic (flags any when-with-else that uses `is` checks).
// ---------------------------------------------------------------------------
type ElseCaseInsteadOfExhaustiveWhenRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence — with the
// resolver it checks if sealed/enum variants are fully covered; fallback
// flags any when-with-else-using-is, which is noisier. Classified per
// roadmap/17.
func (r *ElseCaseInsteadOfExhaustiveWhenRule) Confidence() float64 { return 0.75 }

// whenElseBranchDeletionFix returns a byte-mode Fix that removes the
// `else -> ...` when_entry from a when_expression along with surrounding
// whitespace. Returns nil when no else entry is found. The fix preserves
// the surrounding closing brace and indentation by trimming the trailing
// newline only when present.
func whenElseBranchDeletionFix(file *scanner.File, idx uint32) *scanner.Fix {
	for entry := file.FlatFirstChild(idx); entry != 0; entry = file.FlatNextSib(entry) {
		if file.FlatType(entry) != "when_entry" {
			continue
		}
		isElse := false
		for child := file.FlatFirstChild(entry); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "else" {
				isElse = true
				break
			}
		}
		if !isElse {
			continue
		}
		start := int(file.FlatStartByte(entry))
		end := int(file.FlatEndByte(entry))
		// Pull leading indentation back to (but not past) the previous
		// newline so the entry's leading whitespace is removed.
		for start > 0 {
			c := file.Content[start-1]
			if c == ' ' || c == '\t' {
				start--
				continue
			}
			break
		}
		// Consume the trailing newline so the line collapses entirely.
		if end < len(file.Content) && file.Content[end] == '\n' {
			end++
		}
		return &scanner.Fix{
			ByteMode:    true,
			StartByte:   start,
			EndByte:     end,
			Replacement: "",
		}
	}
	return nil
}

// whenHasElseBranchFlat returns true when the when_expression at idx has
// an `else ->` branch. A when_entry for the else branch has an `else`
// token child in place of a when_condition.
func whenHasElseBranchFlat(file *scanner.File, idx uint32) bool {
	if file == nil || file.FlatType(idx) != "when_expression" {
		return false
	}
	for entry := file.FlatFirstChild(idx); entry != 0; entry = file.FlatNextSib(entry) {
		if file.FlatType(entry) != "when_entry" {
			continue
		}
		for child := file.FlatFirstChild(entry); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "else" {
				return true
			}
		}
	}
	return false
}

func whenElseBranchTerminatesFlat(file *scanner.File, idx uint32) bool {
	if file == nil || file.FlatType(idx) != "when_expression" {
		return false
	}
	for entry := file.FlatFirstChild(idx); entry != 0; entry = file.FlatNextSib(entry) {
		if file.FlatType(entry) != "when_entry" {
			continue
		}
		isElse := false
		for child := file.FlatFirstChild(entry); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "else" {
				isElse = true
				break
			}
		}
		if !isElse {
			continue
		}
		if body, ok := file.FlatFindChild(entry, "control_structure_body"); ok {
			return blockTerminatesFlat(file, body)
		}
		return blockTerminatesFlat(file, entry)
	}
	return false
}

// whenSubjectExhaustiveKindFlat is the kind-aware variant of
// whenSubjectExhaustiveVariantsFlat: it returns whether the subject is a
// sealed hierarchy or an enum so callers can match variants by `is`
// type-test or by entry value. kind is "sealed" or "enum"; empty when
// the subject doesn't resolve to either.
func whenSubjectExhaustiveKindFlat(file *scanner.File, idx uint32, resolver typeinfer.TypeResolver) (kind, subjectName string, variants []string) {
	if file == nil || idx == 0 || resolver == nil || file.FlatType(idx) != "when_expression" {
		return "", "", nil
	}
	subject := whenSubjectExpressionFlat(file, idx)
	if subject == 0 {
		return "", "", nil
	}
	candidates := whenSubjectTypeCandidatesFlat(file, subject, resolver)
	for _, name := range candidates {
		if vs := resolver.SealedVariants(name); len(vs) > 0 {
			return "sealed", simpleTypeName(name), vs
		}
		if vs := resolver.EnumEntries(name); len(vs) > 0 {
			return "enum", simpleTypeName(name), vs
		}
		if info := resolver.ClassHierarchy(name); info != nil {
			for _, infoName := range []string{info.Name, info.FQN} {
				if infoName == "" || infoName == name {
					continue
				}
				if vs := resolver.SealedVariants(infoName); len(vs) > 0 {
					return "sealed", simpleTypeName(infoName), vs
				}
				if vs := resolver.EnumEntries(infoName); len(vs) > 0 {
					return "enum", simpleTypeName(infoName), vs
				}
			}
		}
	}
	return "", "", nil
}

func whenSubjectExhaustiveVariantsFlat(file *scanner.File, idx uint32, resolver typeinfer.TypeResolver) (string, []string) {
	if file == nil || idx == 0 || resolver == nil || file.FlatType(idx) != "when_expression" {
		return "", nil
	}
	subject := whenSubjectExpressionFlat(file, idx)
	if subject == 0 {
		return "", nil
	}
	candidates := whenSubjectTypeCandidatesFlat(file, subject, resolver)
	for _, name := range candidates {
		if variants := resolver.SealedVariants(name); len(variants) > 0 {
			return simpleTypeName(name), variants
		}
		if variants := resolver.EnumEntries(name); len(variants) > 0 {
			return simpleTypeName(name), variants
		}
		if info := resolver.ClassHierarchy(name); info != nil {
			for _, infoName := range []string{info.Name, info.FQN} {
				if infoName == "" || infoName == name {
					continue
				}
				if variants := resolver.SealedVariants(infoName); len(variants) > 0 {
					return simpleTypeName(infoName), variants
				}
				if variants := resolver.EnumEntries(infoName); len(variants) > 0 {
					return simpleTypeName(infoName), variants
				}
			}
		}
	}
	return "", nil
}

func whenSubjectExpressionFlat(file *scanner.File, idx uint32) uint32 {
	subject, ok := file.FlatFindChild(idx, "when_subject")
	if !ok || subject == 0 {
		return 0
	}
	for child := file.FlatFirstChild(subject); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		switch file.FlatType(child) {
		case "variable_declaration", "property_declaration":
			if init := declInitializerFlat(file, child); init != 0 {
				return init
			}
		default:
			return child
		}
	}
	return 0
}

func whenSubjectTypeCandidatesFlat(file *scanner.File, subject uint32, resolver typeinfer.TypeResolver) []string {
	seen := make(map[string]bool, 4)
	var out []string
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		out = append(out, name)
	}
	addType := func(t *typeinfer.ResolvedType) {
		if t == nil || t.Kind == typeinfer.TypeUnknown {
			return
		}
		add(t.Name)
		add(t.FQN)
		if imported := resolver.ResolveImport(t.Name, file); imported != "" {
			add(imported)
		}
	}
	addType(resolver.ResolveFlatNode(subject, file))
	if file.FlatType(subject) == "simple_identifier" {
		name := file.FlatNodeText(subject)
		addType(resolver.ResolveByNameFlat(name, subject, file))
		if imported := resolver.ResolveImport(name, file); imported != "" {
			add(imported)
		}
	}
	return out
}

func whenVariantsCoveredFlat(coveredTypes map[string]bool, variants []string) bool {
	for _, v := range variants {
		if v == "" {
			continue
		}
		if coveredTypes[v] || coveredTypes[simpleTypeName(v)] {
			continue
		}
		return false
	}
	return len(variants) > 0
}

// whenConditionTypeTestName returns the type identifier from an
// `is TypeName` when_condition, or "" if the condition is not a
// type_test. Works on the condition node, not the whole entry.
func whenConditionTypeTestName(file *scanner.File, condition uint32) string {
	if file == nil || file.FlatType(condition) != "when_condition" {
		return ""
	}
	typeTest, ok := file.FlatFindChild(condition, "type_test")
	if !ok {
		return ""
	}
	userType, ok := file.FlatFindChild(typeTest, "user_type")
	if !ok {
		return ""
	}
	return strings.TrimSpace(file.FlatNodeText(userType))
}

// ---------------------------------------------------------------------------
// NoElseInWhenSealedRule — the inverse of ElseCaseInsteadOfExhaustiveWhen.
// Flags `when` expressions whose subject is a sealed type or enum, that
// have no `else` branch, and that are missing one or more variants. Mimics
// kotlinc's NO_ELSE_IN_WHEN / NON_EXHAUSTIVE_WHEN_STATEMENT diagnostics for
// the cases the resolver can prove from source — sealed hierarchies and
// enums declared in the workspace.
//
// Without a resolver the rule cannot identify variants and stays silent.
// Library-defined sealed types (e.g. kotlin.Result) require classpath
// metadata; until that lands the rule simply doesn't fire on them.
// ---------------------------------------------------------------------------
type NoElseInWhenSealedRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *NoElseInWhenSealedRule) Confidence() float64 { return 0.9 }

// whenConditionEnumEntryName extracts the entry name from a value-style
// when_condition like `Color.RED` (navigation_expression) or a bare
// `RED` (simple_identifier). Returns "" when the condition isn't a
// recognizable enum-entry match.
func whenConditionEnumEntryName(file *scanner.File, condition uint32) string {
	if file == nil || file.FlatType(condition) != "when_condition" {
		return ""
	}
	for child := file.FlatFirstChild(condition); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		switch file.FlatType(child) {
		case "navigation_expression":
			if name := flatNavigationExpressionLastIdentifier(file, child); name != "" {
				return name
			}
		case "simple_identifier":
			return strings.TrimSpace(file.FlatNodeText(child))
		}
	}
	return ""
}

func collectWhenCoveredVariants(file *scanner.File, idx uint32) (typeNames map[string]bool, entryNames map[string]bool) {
	typeNames = map[string]bool{}
	entryNames = map[string]bool{}
	file.FlatForEachChild(idx, func(entry uint32) {
		if file.FlatType(entry) != "when_entry" {
			return
		}
		file.FlatForEachChild(entry, func(cond uint32) {
			if file.FlatType(cond) != "when_condition" {
				return
			}
			if name := whenConditionTypeTestName(file, cond); name != "" {
				typeNames[name] = true
			}
			if name := whenConditionEnumEntryName(file, cond); name != "" {
				entryNames[name] = true
			}
		})
	})
	return typeNames, entryNames
}

func missingSealedVariants(covered map[string]bool, variants []string) []string {
	// Normalize covered names to also include their simple-name form so
	// `is Result.Loading` matches a variant indexed as `Loading`.
	coveredSet := make(map[string]bool, len(covered)*2)
	for name := range covered {
		coveredSet[name] = true
		coveredSet[simpleTypeName(name)] = true
	}
	seen := map[string]bool{}
	var missing []string
	for _, v := range variants {
		if v == "" {
			continue
		}
		simple := simpleTypeName(v)
		if seen[simple] {
			continue
		}
		seen[simple] = true
		if coveredSet[v] || coveredSet[simple] {
			continue
		}
		missing = append(missing, simple)
	}
	return missing
}

func missingEnumEntries(covered map[string]bool, entries []string) []string {
	seen := map[string]bool{}
	var missing []string
	for _, e := range entries {
		if e == "" {
			continue
		}
		if seen[e] {
			continue
		}
		seen[e] = true
		if covered[e] {
			continue
		}
		missing = append(missing, e)
	}
	return missing
}
