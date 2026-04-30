package rules

import (
	"fmt"
	"strings"
	"sync"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
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
	if last == "" {
		return false
	}
	// Must be all uppercase letters, digits, or underscores
	for _, c := range last {
		if (c >= 'A' && c <= 'Z') || c == '_' || (c >= '0' && c <= '9') {
			continue
		}
		return false
	}
	// Must contain at least one letter
	for _, c := range last {
		if c >= 'A' && c <= 'Z' {
			return true
		}
	}
	return false
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
// The operator is the unnamed token child whose type is one of `==`, `!=`,
// `===`, `!==`. Returns zeros if the expression doesn't have all three parts.
func equalityOperands(file *scanner.File, idx uint32) (left, op, right uint32) {
	if file == nil || file.FlatType(idx) != "equality_expression" {
		return 0, 0, 0
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		t := file.FlatType(child)
		switch t {
		case "==", "!=", "===", "!==":
			op = child
		default:
			if file.FlatIsNamed(child) {
				if left == 0 {
					left = child
				} else {
					right = child
				}
			}
		}
	}
	return left, op, right
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

type doubleMutabilityShadowCacheKey struct {
	file   *scanner.File
	simple string
}

var doubleMutabilityShadowCache sync.Map

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

func doubleMutabilitySimpleTypeShadowed(file *scanner.File, simple string) bool {
	if file == nil || simple == "" {
		return false
	}
	key := doubleMutabilityShadowCacheKey{file: file, simple: simple}
	if cached, ok := doubleMutabilityShadowCache.Load(key); ok {
		return cached.(bool)
	}
	shadowed := false
	for _, nodeType := range []string{"class_declaration", "object_declaration", "interface_declaration", "type_alias"} {
		file.FlatWalkNodes(0, nodeType, func(node uint32) {
			if shadowed {
				return
			}
			if extractIdentifierFlat(file, node) == simple {
				shadowed = true
			}
		})
	}
	if shadowed {
		doubleMutabilityShadowCache.Store(key, true)
		return true
	}
	file.FlatWalkNodes(0, "import_header", func(node uint32) {
		if shadowed {
			return
		}
		text := cleanImportHeaderTextWithAlias(file.FlatNodeText(node))
		if alias := strings.Index(text, " as "); alias >= 0 {
			fqn := strings.TrimSpace(text[:alias])
			asName := strings.TrimSpace(text[alias+4:])
			if asName == simple && !knownMutableCollectionFQN(fqn) {
				shadowed = true
			}
			return
		}
		if strings.HasSuffix(text, ".*") {
			pkg := strings.TrimSuffix(text, ".*")
			if pkg != "kotlin.collections" && pkg != "java.util" {
				shadowed = true
			}
			return
		}
		if strings.HasSuffix(text, "."+simple) && !knownMutableCollectionFQN(text) {
			shadowed = true
		}
	})
	doubleMutabilityShadowCache.Store(key, shadowed)
	return shadowed
}

func initializerLooksLikeMutableFactory(text string) bool {
	eq := strings.Index(text, "=")
	if eq < 0 {
		return false
	}
	rhs := strings.TrimSpace(text[eq+1:])
	if rhs == "" {
		return false
	}
	for name := range mutableCollectionFactories {
		if strings.HasPrefix(rhs, name+"(") || strings.HasPrefix(rhs, name+"<") {
			return true
		}
	}
	return false
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

func (r *WrongEqualsTypeParameterRule) check(ctx *v2.Context) {
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

func (r *CharArrayToStringCallRule) check(ctx *v2.Context) {
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

func (r *CharArrayToStringCallRule) emitCharArrayFinding(ctx *v2.Context, callIdx uint32, file *scanner.File, receiver uint32, confidence float64) {
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
	receiverText := file.FlatNodeText(receiver)

	// Direct charArrayOf() call expression
	if rt == "call_expression" && strings.HasPrefix(receiverText, "charArrayOf") {
		return true
	}
	if rt != "simple_identifier" {
		return false
	}

	found := false
	// Check function parameters with CharArray type annotation
	file.FlatWalkNodes(0, "parameter", func(pIdx uint32) {
		if found {
			return
		}
		id, ok := file.FlatFindChild(pIdx, "simple_identifier")
		if !ok || file.FlatNodeText(id) != receiverText {
			return
		}
		for child := file.FlatFirstChild(pIdx); child != 0; child = file.FlatNextSib(child) {
			ct := file.FlatType(child)
			if ct == "user_type" || ct == "nullable_type" {
				if strings.TrimSuffix(file.FlatNodeText(child), "?") == "CharArray" {
					found = true
					return
				}
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
		if !ok || file.FlatNodeText(id) != receiverText {
			return
		}
		// Explicit CharArray type annotation
		for child := file.FlatFirstChild(varDecl); child != 0; child = file.FlatNextSib(child) {
			ct := file.FlatType(child)
			if ct == "user_type" || ct == "nullable_type" {
				if strings.TrimSuffix(file.FlatNodeText(child), "?") == "CharArray" {
					found = true
					return
				}
			}
		}
		// charArrayOf() initializer
		if strings.Contains(file.FlatNodeText(pIdx), "charArrayOf") {
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

func (r *DontDowncastCollectionTypesRule) check(ctx *v2.Context) {
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

	// If KAA available: only emit when source is the corresponding immutable interface.
	resolverConfirmed := false
	if ctx.Resolver != nil && sourceIdx != 0 {
		sourceType := ctx.Resolver.ResolveFlatNode(sourceIdx, file)
		if sourceType.Kind != typeinfer.TypeUnknown {
			// Source is already a mutable collection type — no unsafe downcast.
			if _, alreadyMutable := mutableCollectionToMethodMap[sourceType.Name]; alreadyMutable {
				return
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
					isCollectionSupertype := false
					for _, st := range info.Supertypes {
						parts := strings.Split(st, ".")
						stName := parts[len(parts)-1]
						if stName == expectedImmutable {
							isCollectionSupertype = true
							break
						}
					}
					if !isCollectionSupertype {
						return
					}
				}
			}
			resolverConfirmed = true
		}
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
