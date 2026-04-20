package rules

import (
	"regexp"
	"strings"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
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

// Confidence reports a tier-2 (medium) base confidence. Potential-bugs types rule. Detection pattern-matches type-related
// constructs; resolver usage when available improves precision but
// fallback is heuristic. Classified per roadmap/17.
func (r *WrongEqualsTypeParameterRule) Confidence() float64 { return 0.75 }

var wrongEqualsRe = regexp.MustCompile(`(?:override\s+)?fun\s+equals\s*\(\s*(?:other|obj)\s*:\s*(\w+\??)`)

var wrongEqualsFixRe = regexp.MustCompile(`(fun\s+equals\s*\(\s*(?:other|obj)\s*:\s*)\w+\??`)

// ---------------------------------------------------------------------------
// CharArrayToStringCallRule detects charArray.toString() calls.
// ---------------------------------------------------------------------------
type CharArrayToStringCallRule struct {
	FlatDispatchBase
	BaseRule
}


// Confidence reports a tier-2 (medium) base confidence — flags
// toString() on CharArray receivers; receiver type detection is
// resolver-dependent. Classified per roadmap/17.
func (r *CharArrayToStringCallRule) Confidence() float64 { return 0.75 }

var charArrayToStringRe = regexp.MustCompile(`[Cc]har[Aa]rray[^.]*\.toString\(\)`)
var charArrayToStringFixRe = regexp.MustCompile(`(\w+(?:\.\w+)*)\.toString\(\)`)

func (r *CharArrayToStringCallRule) reportCharArrayFlat(ctx *v2.Context, text string) {
	idx, file := ctx.Idx, ctx.File
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
	ctx.Emit(f)
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

// ---------------------------------------------------------------------------
// ImplicitUnitReturnTypeRule detects functions without explicit return type.
// ---------------------------------------------------------------------------
type ImplicitUnitReturnTypeRule struct {
	FlatDispatchBase
	BaseRule
}


// Confidence reports a tier-2 (medium) base confidence because this
// rule fires on any function_declaration lacking an explicit return
// type when the resolver is absent. That includes @Composable
// functions, which conventionally omit ': Unit' — a convention
// mismatch rather than a bug, but one that produces noise on Compose
// codebases. Medium confidence keeps it off default-strict gates
// without taking it out of the rule set.
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

var whenElseRe = regexp.MustCompile(`(?m)^\s*else\s*->`)
