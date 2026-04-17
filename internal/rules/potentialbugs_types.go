package rules

import (
	"fmt"
	"regexp"
	"strings"

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
	resolver              typeinfer.TypeResolver
	ForbiddenTypePatterns []string
}

func (r *AvoidReferentialEqualityRule) SetResolver(res typeinfer.TypeResolver) { r.resolver = res }

// Confidence reports a tier-2 (medium) base confidence — flags === / !==
// on value types; needs resolver to confirm operand types, falls back to a
// name-based heuristic. Classified per roadmap/17.
func (r *AvoidReferentialEqualityRule) Confidence() float64 { return 0.75 }

func (r *AvoidReferentialEqualityRule) NodeTypes() []string { return []string{"equality_expression"} }

func (r *AvoidReferentialEqualityRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	if !strings.Contains(text, "===") && !strings.Contains(text, "!==") {
		return nil
	}
	// Skip comparisons against the `null` literal. In Kotlin,
	// `x === null` and `x == null` produce identical bytecode because
	// `==` against `null` is already a reference check. The referential
	// form is NOT an anti-pattern — it's a stylistic variation and
	// offers no semantic difference. Firebase Data Connect alone has
	// ~230 findings of this shape.
	if file.FlatChildCount(idx) >= 3 {
		left := strings.TrimSpace(file.FlatNodeText(file.FlatChild(idx, 0)))
		right := strings.TrimSpace(file.FlatNodeText(file.FlatChild(idx, file.FlatChildCount(idx)-1)))
		if left == "null" || right == "null" {
			return nil
		}
	}
	// Skip `this === other` inside an `equals(other: Any?)` override —
	// this is the canonical identity fast-path required by the equals
	// contract, not a mistake.
	trimmed := strings.TrimSpace(text)
	if (strings.HasPrefix(trimmed, "this === ") || strings.HasPrefix(trimmed, "this !== ")) &&
		isInsideEqualsMethodFlatType(file, idx) {
		return nil
	}
	// Skip when === / !== is used as a fast-path short-circuit before a
	// structural equality check: `a === b || a.equals(b)` or
	// `a === b || a.hasSameContent(b)`. This is an idiomatic performance
	// optimization, not a bug.
	if parent, ok := file.FlatParent(idx); ok {
		if pt := file.FlatType(parent); pt == "disjunction_expression" || pt == "conjunction_expression" {
			parentText := file.FlatNodeText(parent)
			if strings.Contains(parentText, ".equals(") ||
				strings.Contains(parentText, ".hasSameContent(") ||
				strings.Contains(parentText, ".contentEquals(") ||
				strings.Contains(parentText, ".contentDeepEquals(") ||
				strings.Contains(parentText, ".sameContentAs(") {
				return nil
			}
		}
	}
	// Skip when comparing against an enum constant — enums are singletons,
	// so `===` is semantically equivalent to `==` and has no downside.
	// Heuristic: if either operand is a qualified reference ending in an
	// ALL_CAPS identifier (e.g., `Type.CONSTANT` / `STATE.READY`), treat
	// as enum comparison.
	if file.FlatChildCount(idx) >= 3 {
		leftText := strings.TrimSpace(file.FlatNodeText(file.FlatChild(idx, 0)))
		rightText := strings.TrimSpace(file.FlatNodeText(file.FlatChild(idx, file.FlatChildCount(idx)-1)))
		if looksLikeEnumConstantRef(leftText) || looksLikeEnumConstantRef(rightText) {
			return nil
		}
	}

	// With type inference we can be more precise: only flag referential equality
	// on known value types (String, Int, Long, etc.) where structural equality
	// should be used. For enums, objects, etc. referential equality may be
	// intentional.
	if r.resolver != nil && file.FlatChildCount(idx) >= 3 {
		leftIdx := file.FlatChild(idx, 0)
		rightIdx := file.FlatChild(idx, file.FlatChildCount(idx)-1)
		if leftIdx != 0 && rightIdx != 0 {
			leftType := r.resolver.ResolveFlatNode(leftIdx, file)
			rightType := r.resolver.ResolveFlatNode(rightIdx, file)
			leftKnown := leftType != nil && leftType.Kind != typeinfer.TypeUnknown
			rightKnown := rightType != nil && rightType.Kind != typeinfer.TypeUnknown
			if leftKnown || rightKnown {
				isValueType := typeinfer.IsKnownValueType(leftType) || typeinfer.IsKnownValueType(rightType)
				if !isValueType {
					return nil // non-value-type referential equality — may be intentional
				}
			}
		}
	}

	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Referential equality (===, !==) should be avoided. Use structural equality (==, !=) instead.")
	// Pin the replacement to the operator child only, so `===` / `!==`
	// occurrences inside adjacent string literals or templates can't
	// accidentally be rewritten. equality_expression children: left, op, right.
	if file.FlatChildCount(idx) >= 3 {
		op := file.FlatChild(idx, 1)
		if op != 0 {
			opText := file.FlatNodeText(op)
			var repl string
			switch opText {
			case "===":
				repl = "=="
			case "!==":
				repl = "!="
			}
			if repl != "" {
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(op)),
					EndByte:     int(file.FlatEndByte(op)),
					Replacement: repl,
				}
			}
		}
	}
	return []scanner.Finding{f}
}

// ---------------------------------------------------------------------------
// DoubleMutabilityForCollectionRule detects var with mutable collection type.
// ---------------------------------------------------------------------------
type DoubleMutabilityForCollectionRule struct {
	FlatDispatchBase
	BaseRule
	resolver     typeinfer.TypeResolver
	MutableTypes []string
}

func (r *DoubleMutabilityForCollectionRule) SetResolver(res typeinfer.TypeResolver) {
	r.resolver = res
}

// Confidence reports a tier-2 (medium) base confidence. The rule
// uses a fixed allow-list of mutable-collection type names and
// factory function names; it correctly flags the common case (`var
// xs = mutableListOf<Foo>()`) but cannot distinguish a custom
// mutable-collection type without type resolution, and won't detect
// the pattern when the collection is returned from a wrapper
// function. Medium confidence matches the known scope-analysis gap.
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

func (r *DoubleMutabilityForCollectionRule) NodeTypes() []string {
	return []string{"property_declaration"}
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

func firstExplicitMutableTypeText(text string, mutableTypes map[string]bool) string {
	colon := strings.Index(text, ":")
	if colon < 0 {
		return ""
	}
	typeText := strings.TrimSpace(text[colon+1:])
	if eq := strings.Index(typeText, "="); eq >= 0 {
		typeText = strings.TrimSpace(typeText[:eq])
	}
	if typeText == "" {
		return ""
	}
	simple := simpleTypeName(typeText)
	if mutableTypes[simple] {
		return simple
	}
	return ""
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

func (r *DoubleMutabilityForCollectionRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if isTestFile(file.Path) {
		return nil
	}
	text := file.FlatNodeText(idx)
	if !strings.Contains(text, "var ") {
		return nil
	}
	mutableTypes := r.configuredMutableTypes()
	varDecl := file.FlatFindChild(idx, "variable_declaration")
	if varDecl == 0 {
		return nil
	}

	// If we have a type resolver, use it for precise mutable collection detection
	if r.resolver != nil {
		// Try to resolve the type of the variable
		for j := 0; j < file.FlatChildCount(varDecl); j++ {
			gc := file.FlatChild(varDecl, j)
			if gc == 0 {
				continue
			}
			if file.FlatType(gc) == "user_type" || file.FlatType(gc) == "nullable_type" {
				resolved := r.resolver.ResolveFlatNode(gc, file)
				if resolved.Kind != typeinfer.TypeUnknown && resolved.IsMutable() {
					f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
						"Variable with mutable collection type creates double mutability. Use val with a mutable collection or var with an immutable collection.")
					// Find the "var" keyword child node in the AST
					var varKeyword uint32
					file.FlatForEachChild(idx, func(ch uint32) {
						if file.FlatNodeTextEquals(ch, "var") {
							varKeyword = ch
						}
					})
					if varKeyword != 0 {
						f.Fix = &scanner.Fix{
							ByteMode:    true,
							StartByte:   int(file.FlatStartByte(varKeyword)),
							EndByte:     int(file.FlatEndByte(varKeyword)),
							Replacement: "val",
						}
					}
					return []scanner.Finding{f}
				}
			}
		}
	}

	if firstExplicitMutableTypeText(text, mutableTypes) == "" {
		if !initializerLooksLikeMutableFactory(text) {
			return nil
		}
	}

	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Variable with mutable collection type creates double mutability. Use val with a mutable collection or var with an immutable collection.")
	// Find the "var" keyword child node in the AST
	var varKeyword uint32
	file.FlatForEachChild(idx, func(ch uint32) {
		if file.FlatNodeTextEquals(ch, "var") {
			varKeyword = ch
		}
	})
	if varKeyword != 0 {
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   int(file.FlatStartByte(varKeyword)),
			EndByte:     int(file.FlatEndByte(varKeyword)),
			Replacement: "val",
		}
	}
	return []scanner.Finding{f}
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

func (r *EqualsAlwaysReturnsTrueOrFalseRule) NodeTypes() []string {
	return []string{"function_declaration"}
}

func (r *EqualsAlwaysReturnsTrueOrFalseRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Must be named "equals"
	name := extractIdentifierFlat(file, idx)
	if name != "equals" {
		return nil
	}
	// Must have override modifier
	if !file.FlatHasModifier(idx, "override") {
		return nil
	}
	body := file.FlatFindChild(idx, "function_body")
	if body == 0 {
		return nil
	}
	bodyText := file.FlatNodeText(body)
	// Expression body: = true or = false
	trimmed := strings.TrimSpace(bodyText)
	if trimmed == "= true" || trimmed == "= false" {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			"equals() always returns the same value. This is likely a bug.")}
	}
	// Block body: check if ALL return statements return the same literal
	allTrue := true
	allFalse := true
	returnCount := 0
	file.FlatWalkNodes(body, "jump_expression", func(jmp uint32) {
		jmpText := strings.TrimSpace(file.FlatNodeText(jmp))
		if !strings.HasPrefix(jmpText, "return") {
			return
		}
		returnCount++
		val := strings.TrimSpace(strings.TrimPrefix(jmpText, "return"))
		if val != "true" {
			allTrue = false
		}
		if val != "false" {
			allFalse = false
		}
	})
	if returnCount > 0 && (allTrue || allFalse) {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			"equals() always returns the same value. This is likely a bug.")}
	}
	return nil
}

func (r *EqualsAlwaysReturnsTrueOrFalseRule) Check(file *scanner.File) []scanner.Finding {
	return nil
}

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

func (r *EqualsWithHashCodeExistRule) NodeTypes() []string { return []string{"class_declaration"} }

func (r *EqualsWithHashCodeExistRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	body := file.FlatFindChild(idx, "class_body")
	if body == 0 {
		return nil
	}
	hasEquals := false
	hasHashCode := false
	walkFlatClassMembers(file, body, func(child uint32) {
		if hasEquals && hasHashCode {
			return
		}
		if file.FlatType(child) != "function_declaration" {
			return
		}
		if !file.FlatHasModifier(child, "override") {
			return
		}
		name := extractIdentifierFlat(file, child)
		if name == "" {
			return
		}
		switch name {
		case "equals":
			hasEquals = true
		case "hashCode":
			hasHashCode = true
		}
	})
	if hasEquals && !hasHashCode {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			"Class overrides equals() but not hashCode().")}
	} else if !hasEquals && hasHashCode {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			"Class overrides hashCode() but not equals().")}
	}
	return nil
}

// ---------------------------------------------------------------------------
// WrongEqualsTypeParameterRule detects equals(other: String) instead of Any?.
// ---------------------------------------------------------------------------
type WrongEqualsTypeParameterRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Potential-bugs types rule. Detection pattern-matches type-related
// constructs; resolver usage when available improves precision but
// fallback is heuristic. Classified per roadmap/17.
func (r *WrongEqualsTypeParameterRule) Confidence() float64 { return 0.75 }

func (r *WrongEqualsTypeParameterRule) NodeTypes() []string { return []string{"function_declaration"} }

var wrongEqualsRe = regexp.MustCompile(`(?:override\s+)?fun\s+equals\s*\(\s*(?:other|obj)\s*:\s*(\w+\??)`)

var wrongEqualsFixRe = regexp.MustCompile(`(fun\s+equals\s*\(\s*(?:other|obj)\s*:\s*)\w+\??`)

func (r *WrongEqualsTypeParameterRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Only check override fun equals(...)
	if !file.FlatHasModifier(idx, "override") {
		return nil
	}
	if extractIdentifierFlat(file, idx) != "equals" {
		return nil
	}
	text := file.FlatNodeText(idx)
	m := wrongEqualsRe.FindStringSubmatch(text)
	if m == nil || m[1] == "Any?" {
		return nil
	}
	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		fmt.Sprintf("equals() parameter type is '%s' instead of 'Any?'. This does not properly override Any.equals().", m[1]))
	startByte := int(file.FlatStartByte(idx))
	if loc := wrongEqualsFixRe.FindStringSubmatchIndex(text); loc != nil {
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   startByte + loc[0],
			EndByte:     startByte + loc[1],
			Replacement: text[loc[2]:loc[3]] + "Any?",
		}
	}
	return []scanner.Finding{f}
}

// ---------------------------------------------------------------------------
// CharArrayToStringCallRule detects charArray.toString() calls.
// ---------------------------------------------------------------------------
type CharArrayToStringCallRule struct {
	FlatDispatchBase
	BaseRule
	resolver typeinfer.TypeResolver
}

func (r *CharArrayToStringCallRule) SetResolver(res typeinfer.TypeResolver) { r.resolver = res }

// Confidence reports a tier-2 (medium) base confidence — flags
// toString() on CharArray receivers; receiver type detection is
// resolver-dependent. Classified per roadmap/17.
func (r *CharArrayToStringCallRule) Confidence() float64 { return 0.75 }

func (r *CharArrayToStringCallRule) NodeTypes() []string { return []string{"call_expression"} }

var charArrayToStringRe = regexp.MustCompile(`[Cc]har[Aa]rray[^.]*\.toString\(\)`)
var charArrayToStringFixRe = regexp.MustCompile(`(\w+(?:\.\w+)*)\.toString\(\)`)

func (r *CharArrayToStringCallRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	if !strings.HasSuffix(text, ".toString()") && !strings.Contains(text, ".toString()") {
		return nil
	}
	// With resolver: check if the receiver's navigation_expression target is CharArray
	if r.resolver != nil {
		navExpr := file.FlatFindChild(idx, "navigation_expression")
		if navExpr != 0 && file.FlatChildCount(navExpr) >= 1 {
			receiver := file.FlatChild(navExpr, 0)
			if receiver != 0 {
				receiverText := file.FlatNodeText(receiver)
				// Get the simple name (last dotted segment)
				simpleName := receiverText
				if dotIdx := strings.LastIndex(simpleName, "."); dotIdx >= 0 {
					simpleName = simpleName[dotIdx+1:]
				}
				resolved := r.resolver.ResolveByNameFlat(simpleName, idx, file)
				if resolved != nil && resolved.Kind != typeinfer.TypeUnknown {
					if resolved.Name == "CharArray" || resolved.FQN == "kotlin.CharArray" {
						return r.reportCharArrayFlat(idx, text, file)
					}
					return nil // resolved to non-CharArray
				}
			}
		}
	}
	// Heuristic fallback: match variable names containing charArray
	if charArrayToStringRe.MatchString(text) {
		return r.reportCharArrayFlat(idx, text, file)
	}
	return nil
}

func (r *CharArrayToStringCallRule) reportCharArrayFlat(idx uint32, text string, file *scanner.File) []scanner.Finding {
	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Calling toString() on a CharArray does not return the string representation. Use String(charArray) instead.")
	startByte := int(file.FlatStartByte(idx))
	if loc := charArrayToStringFixRe.FindStringSubmatchIndex(text); loc != nil {
		receiver := text[loc[2]:loc[3]]
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   startByte + loc[0],
			EndByte:     startByte + loc[1],
			Replacement: "String(" + receiver + ")",
		}
	}
	return []scanner.Finding{f}
}

// ---------------------------------------------------------------------------
// DontDowncastCollectionTypesRule detects `as MutableList`, `as MutableMap`, etc.
// With type inference: uses ClassHierarchy on source and target of a cast to verify
// the source is a supertype of the target in the collection hierarchy.
// ---------------------------------------------------------------------------
type DontDowncastCollectionTypesRule struct {
	FlatDispatchBase
	BaseRule
	resolver typeinfer.TypeResolver
}

func (r *DontDowncastCollectionTypesRule) SetResolver(res typeinfer.TypeResolver) {
	r.resolver = res
}

// Confidence reports a tier-2 (medium) base confidence — flags downcasts
// like List -> MutableList; source type determination requires the
// resolver. Classified per roadmap/17.
func (r *DontDowncastCollectionTypesRule) Confidence() float64 { return 0.75 }

var mutableCollectionCastRe = regexp.MustCompile(`\bas\s+(Mutable(?:List|Set|Map|Collection|Iterator|ListIterator|Iterable))\b`)

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

func (r *DontDowncastCollectionTypesRule) NodeTypes() []string { return []string{"as_expression"} }

func (r *DontDowncastCollectionTypesRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	m := mutableCollectionCastRe.FindStringSubmatch(text)
	if m == nil {
		return nil
	}

	targetType := m[1] // e.g., "MutableList"

	// With resolver: verify the source expression's type is indeed an immutable
	// collection supertype of the target mutable type.
	if r.resolver != nil {
		sourceIdx := file.FlatChild(idx, 0)
		if sourceIdx != 0 {
			sourceType := r.resolver.ResolveFlatNode(sourceIdx, file)
			if sourceType.Kind != typeinfer.TypeUnknown {
				// Check if the source type is an immutable collection that maps to this mutable target
				expectedImmutable := ""
				for immutable, mutable := range immutableToMutableMap {
					if mutable == targetType {
						expectedImmutable = immutable
						break
					}
				}
				if expectedImmutable != "" && sourceType.Name != expectedImmutable {
					// Source is not the expected immutable counterpart — also check hierarchy
					info := r.resolver.ClassHierarchy(sourceType.Name)
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
							return nil
						}
					}
				}
			}
		}
	}

	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		fmt.Sprintf("Don't downcast collection type to '%s'. This can lead to unexpected mutations.", targetType))
	// Fix: replace "expr as MutableList<T>" with "expr.toMutableList()"
	parts := strings.SplitN(text, " as ", 2)
	if len(parts) == 2 {
		expr := strings.TrimSpace(parts[0])
		if method, ok := mutableCollectionToMethodMap[targetType]; ok {
			f.Fix = &scanner.Fix{
				ByteMode:    true,
				StartByte:   int(file.FlatStartByte(idx)),
				EndByte:     int(file.FlatEndByte(idx)),
				Replacement: expr + "." + method,
			}
		}
	}
	return []scanner.Finding{f}
}

// ---------------------------------------------------------------------------
// ImplicitUnitReturnTypeRule detects functions without explicit return type.
// ---------------------------------------------------------------------------
type ImplicitUnitReturnTypeRule struct {
	FlatDispatchBase
	BaseRule
	resolver typeinfer.TypeResolver
}

func (r *ImplicitUnitReturnTypeRule) SetResolver(res typeinfer.TypeResolver) { r.resolver = res }

// Confidence reports a tier-2 (medium) base confidence because this
// rule fires on any function_declaration lacking an explicit return
// type when the resolver is absent. That includes @Composable
// functions, which conventionally omit ': Unit' — a convention
// mismatch rather than a bug, but one that produces noise on Compose
// codebases. Medium confidence keeps it off default-strict gates
// without taking it out of the rule set.
func (r *ImplicitUnitReturnTypeRule) Confidence() float64 { return 0.75 }

func (r *ImplicitUnitReturnTypeRule) NodeTypes() []string { return []string{"function_declaration"} }

func (r *ImplicitUnitReturnTypeRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Must have a function_body with a block (not expression body with =)
	body := file.FlatFindChild(idx, "function_body")
	if body == 0 {
		return nil
	}
	// Expression bodies (= ...) always have a return type or it's inferred from expression
	// We only care about block bodies { ... }
	if file.FlatChildCount(body) == 0 || file.FlatType(file.FlatChild(body, 0)) != "statements" {
		// Check if the body starts with "{"
		bodyText := file.FlatNodeText(body)
		if !strings.HasPrefix(strings.TrimSpace(bodyText), "{") {
			return nil
		}
	}
	// Check if there's already an explicit return type (a ":" after the parameter list)
	// Look for a user_type or nullable_type child that represents the return type
	hasReturnType := false
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		ct := file.FlatType(child)
		if ct == "user_type" || ct == "nullable_type" || ct == "type_identifier" {
			hasReturnType = true
			break
		}
		// Also check for ":" token before the body
		if file.FlatNodeTextEquals(child, ":") {
			hasReturnType = true
			break
		}
	}
	if hasReturnType {
		return nil
	}
	// Get the function name
	funcName := extractIdentifierFlat(file, idx)
	if funcName == "" {
		return nil
	}

	// If resolver is available, check if the function's return type resolves to Unit
	if r.resolver != nil {
		resolved := r.resolver.ResolveByNameFlat(funcName, idx, file)
		if resolved != nil && resolved.Kind != typeinfer.TypeUnknown &&
			(resolved.Kind == typeinfer.TypeUnit || resolved.Name == "Unit" || resolved.FQN == "kotlin.Unit") {
			return nil
		}
	}

	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"Function without explicit return type. Consider adding ': Unit' or the appropriate return type.")
	// Try to insert ": Unit " before the opening "{"
	if body != 0 {
		insertAt := int(file.FlatStartByte(body))
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   insertAt,
			EndByte:     insertAt,
			Replacement: ": Unit ",
		}
	}
	return []scanner.Finding{f}
}

// ---------------------------------------------------------------------------
// ElseCaseInsteadOfExhaustiveWhenRule detects when with else on enum/sealed.
// With type inference: checks if ALL sealed/enum variants are covered in the
// when branches. If they are, the else is truly unnecessary. Without resolver,
// falls back to the heuristic (flags any when-with-else that uses `is` checks).
// ---------------------------------------------------------------------------
type ElseCaseInsteadOfExhaustiveWhenRule struct {
	FlatDispatchBase
	BaseRule
	resolver typeinfer.TypeResolver
}

func (r *ElseCaseInsteadOfExhaustiveWhenRule) SetResolver(res typeinfer.TypeResolver) {
	r.resolver = res
}

// Confidence reports a tier-2 (medium) base confidence — with the
// resolver it checks if sealed/enum variants are fully covered; fallback
// flags any when-with-else-using-is, which is noisier. Classified per
// roadmap/17.
func (r *ElseCaseInsteadOfExhaustiveWhenRule) Confidence() float64 { return 0.75 }

var whenElseRe = regexp.MustCompile(`(?m)^\s*else\s*->`)

func (r *ElseCaseInsteadOfExhaustiveWhenRule) NodeTypes() []string {
	return []string{"when_expression"}
}

func (r *ElseCaseInsteadOfExhaustiveWhenRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	if !whenElseRe.MatchString(text) {
		return nil
	}

	// With type inference we can verify whether the else is truly unnecessary
	// by checking if all sealed/enum variants are already covered.
	if r.resolver != nil {
		// Collect the type names used in `is` checks across when entries
		coveredTypes := make(map[string]bool)
		var subjectTypeName string

		file.FlatForEachChild(idx, func(entry uint32) {
			if file.FlatType(entry) != "when_entry" {
				return
			}
			// Look for `is TypeName` conditions inside when_condition nodes
			file.FlatForEachChild(entry, func(cond uint32) {
				condText := strings.TrimSpace(file.FlatNodeText(cond))
				if strings.HasPrefix(strings.TrimSpace(condText), "is ") {
					typeName := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(condText), "is "))
					coveredTypes[typeName] = true
				}
			})
		})

		if len(coveredTypes) > 0 {
			// Try to find the subject type: first covered type may share a sealed parent
			for typeName := range coveredTypes {
				info := r.resolver.ClassHierarchy(typeName)
				if info != nil && len(info.Supertypes) > 0 {
					// Use the first supertype as the sealed parent candidate
					for _, st := range info.Supertypes {
						// Extract simple name from FQN
						parts := strings.Split(st, ".")
						simpleName := parts[len(parts)-1]
						variants := r.resolver.SealedVariants(simpleName)
						if len(variants) == 0 {
							variants = r.resolver.SealedVariants(st)
						}
						if len(variants) > 0 {
							subjectTypeName = simpleName
							// Check if all variants are covered
							allCovered := true
							for _, v := range variants {
								vParts := strings.Split(v, ".")
								vSimple := vParts[len(vParts)-1]
								if !coveredTypes[vSimple] && !coveredTypes[v] {
									allCovered = false
									break
								}
							}
							if !allCovered {
								// Not all variants covered — else might be needed
								return nil
							}
							break
						}
					}
					if subjectTypeName != "" {
						break
					}
				}
			}

			// If we resolved a sealed type and all variants are covered, flag it
			if subjectTypeName != "" {
				return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					fmt.Sprintf("When expression on sealed type '%s' uses 'else' but all variants are covered. Remove the else branch.", subjectTypeName))}
			}
		}
		// Resolver available but couldn't determine sealed variants — don't flag
		return nil
	}

	// Without type resolution we cannot determine whether the when subject is
	// an enum or sealed class, so skip to avoid false positives.
	return nil
}

func (r *ElseCaseInsteadOfExhaustiveWhenRule) Check(file *scanner.File) []scanner.Finding {
	return nil
}
