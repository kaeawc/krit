package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// RedundantVisibilityModifierRule detects explicit `public` keyword.
type RedundantVisibilityModifierRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/redundant rule. Detection flags visible but arguably-redundant
// modifiers, types, or keywords. Whether removal improves readability is
// context-dependent. Classified per roadmap/17.
func (r *RedundantVisibilityModifierRule) Confidence() float64 { return api.ConfidenceMedium }

// RedundantConstructorKeywordRule detects unnecessary `constructor` keyword.
// Flags primary constructors that use the explicit `constructor` keyword when
// there are no annotations or visibility modifiers on the constructor itself.
type RedundantConstructorKeywordRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/redundant rule. Detection flags visible but arguably-redundant
// modifiers, types, or keywords. Whether removal improves readability is
// context-dependent. Classified per roadmap/17.
func (r *RedundantConstructorKeywordRule) Confidence() float64 { return api.ConfidenceMedium }

// RedundantExplicitTypeRule detects type annotations where the type is obvious.
// With type inference: uses ResolveNode on both the declared type and the initializer
// expression. If both resolve to the same FQN, the explicit type is redundant.
type RedundantExplicitTypeRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence — inferring
// whether the type annotation is necessary requires the resolver; fallback
// is conservative. Classified per roadmap/17.
func (r *RedundantExplicitTypeRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *RedundantExplicitTypeRule) buildFixFlat(typeNode uint32, file *scanner.File) *scanner.Fix {
	typeStart := int(file.FlatStartByte(typeNode))
	typeEnd := int(file.FlatEndByte(typeNode))
	colonPos := typeStart - 1
	for colonPos >= 0 && (file.Content[colonPos] == ' ' || file.Content[colonPos] == '\t') {
		colonPos--
	}
	if colonPos >= 0 && file.Content[colonPos] == ':' {
		startRemove := colonPos
		for startRemove > 0 && (file.Content[startRemove-1] == ' ' || file.Content[startRemove-1] == '\t') {
			startRemove--
		}
		return &scanner.Fix{
			ByteMode:    true,
			StartByte:   startRemove,
			EndByte:     typeEnd,
			Replacement: "",
		}
	}
	return nil
}

// UnnecessaryParenthesesRule detects unnecessary parentheses around expressions:
// return values, if/when conditions, assignments, double-wrapped parens, and
// other contexts where the parentheses add no value.
type UnnecessaryParenthesesRule struct {
	FlatDispatchBase
	BaseRule
	AllowForUnclearPrecedence bool // if true, allow parens that clarify operator precedence
}

// Confidence reports a tier-2 (medium) base confidence. Style/redundant rule. Detection flags visible but arguably-redundant
// modifiers, types, or keywords. Whether removal improves readability is
// context-dependent. Classified per roadmap/17.
func (r *UnnecessaryParenthesesRule) Confidence() float64 { return api.ConfidenceMedium }

// UnnecessaryInheritanceRule detects `: Any()`.
type UnnecessaryInheritanceRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/redundant rule. Detection flags visible but arguably-redundant
// modifiers, types, or keywords. Whether removal improves readability is
// context-dependent. Classified per roadmap/17.
func (r *UnnecessaryInheritanceRule) Confidence() float64 { return api.ConfidenceMedium }

// UnnecessaryInnerClassRule detects inner classes that don't use the outer reference.
type UnnecessaryInnerClassRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/redundant rule. Detection flags visible but arguably-redundant
// modifiers, types, or keywords. Whether removal improves readability is
// context-dependent. Classified per roadmap/17.
func (r *UnnecessaryInnerClassRule) Confidence() float64 { return api.ConfidenceMedium }

// OptionalUnitRule detects explicit `: Unit` return types on functions
// and redundant `return Unit` statements inside function bodies.
type OptionalUnitRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/redundant rule. Detection flags visible but arguably-redundant
// modifiers, types, or keywords. Whether removal improves readability is
// context-dependent. Classified per roadmap/17.
func (r *OptionalUnitRule) Confidence() float64 { return api.ConfidenceMedium }

// UnnecessaryBackticksRule detects unnecessary backtick-quoted identifiers.
type UnnecessaryBackticksRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/redundant rule. Detection flags visible but arguably-redundant
// modifiers, types, or keywords. Whether removal improves readability is
// context-dependent. Classified per roadmap/17.
func (r *UnnecessaryBackticksRule) Confidence() float64 { return api.ConfidenceMedium }

func isAllUnderscores(s string) bool {
	for _, ch := range s {
		if ch != '_' {
			return false
		}
	}
	return len(s) > 0
}

func isValidKotlinIdentifier(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i, ch := range s {
		if i == 0 {
			if (ch < 'a' || ch > 'z') && (ch < 'A' || ch > 'Z') && ch != '_' {
				return false
			}
		} else {
			if (ch < 'a' || ch > 'z') && (ch < 'A' || ch > 'Z') && (ch < '0' || ch > '9') && ch != '_' {
				return false
			}
		}
	}
	return true
}

func isIdentChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_'
}

// offsetInsideSourceComment reports whether a byte offset lies in a `//` or `/* */` comment. The parser
// can lose comment context after earlier recovery and read KDoc text as code, so rules that look at
// identifiers use the source text to tell.
func offsetInsideSourceComment(file *scanner.File, offset uint32) bool {
	ranges := filefacts.FileFact(fileFactsCache(), file, slotSourceCommentRanges, func() [][2]uint32 {
		return sourceCommentRanges(file.Content)
	})
	for _, r := range ranges {
		if offset >= r[0] && offset < r[1] {
			return true
		}
	}
	return false
}

// sourceCommentRanges lexes Kotlin text just enough to find comment spans, skipping strings and chars.
func sourceCommentRanges(src []byte) [][2]uint32 {
	l := &commentRangeLexer{src: src}
	l.scan(0, false)
	return l.ranges
}

// commentRangeLexer collects comment spans; string templates recurse into scan for their `${...}` code.
type commentRangeLexer struct {
	src    []byte
	ranges [][2]uint32
}

// scan lexes code from i and returns the offset after it: the end of src, or after the closing `}` of a template
// expression when untilBrace is set.
func (l *commentRangeLexer) scan(i int, untilBrace bool) int {
	src := l.src
	depth := 0
	for i < len(src) {
		c := src[i]
		switch {
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			start := i
			i = lineCommentEnd(src, i)
			l.ranges = append(l.ranges, [2]uint32{uint32(start), uint32(i)})
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			start := i
			i = blockCommentEnd(src, i)
			l.ranges = append(l.ranges, [2]uint32{uint32(start), uint32(i)})
		case c == '"' && i+2 < len(src) && src[i+1] == '"' && src[i+2] == '"':
			i = l.scanString(i+3, true)
		case c == '"':
			i = l.scanString(i+1, false)
		case c == '\'':
			i = charLiteralEnd(src, i)
		case untilBrace && c == '{':
			depth++
			i++
		case untilBrace && c == '}':
			if depth == 0 {
				return i + 1
			}
			depth--
			i++
		default:
			i++
		}
	}
	return i
}

// scanString skips a string body starting at i (raw for triple-quoted strings) and returns the offset after it.
func (l *commentRangeLexer) scanString(i int, raw bool) int {
	src := l.src
	for i < len(src) {
		switch {
		case !raw && src[i] == '\\':
			i += 2
		case raw && src[i] == '"' && i+2 < len(src) && src[i+1] == '"' && src[i+2] == '"':
			return i + 3
		case !raw && (src[i] == '"' || src[i] == '\n'):
			return i + 1
		case src[i] == '$' && i+1 < len(src) && src[i+1] == '{':
			i = l.scan(i+2, true)
		default:
			i++
		}
	}
	return i
}

// lineCommentEnd returns the offset of the line break (or end of src) that ends the `//` comment at i.
func lineCommentEnd(src []byte, i int) int {
	for i < len(src) && src[i] != '\n' {
		i++
	}
	return i
}

// blockCommentEnd returns the offset after the `*/` that closes the (possibly nested) block comment at i.
func blockCommentEnd(src []byte, i int) int {
	nest := 0
	for i < len(src) {
		if src[i] == '/' && i+1 < len(src) && src[i+1] == '*' {
			nest++
			i += 2
		} else if src[i] == '*' && i+1 < len(src) && src[i+1] == '/' {
			nest--
			i += 2
			if nest == 0 {
				break
			}
		} else {
			i++
		}
	}
	return i
}

// charLiteralEnd returns the offset after the char literal whose opening quote is at i.
func charLiteralEnd(src []byte, i int) int {
	i++
	for i < len(src) && src[i] != '\'' && src[i] != '\n' {
		if src[i] == '\\' {
			i++
		}
		i++
	}
	return i + 1
}

func isInsideStringTemplateFlat(file *scanner.File, idx uint32) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		t := file.FlatType(p)
		if t == "line_string_literal" || t == "multi_line_string_literal" {
			return true
		}
	}
	return false
}

func isKotlinKeyword(s string) bool {
	switch s {
	case "as", "break", "class", "continue", "do", "else", "false", "for", "fun",
		"if", "in", "interface", "is", "null", "object", "package", "return",
		"super", "this", "throw", "true", "try", "typealias", "typeof", "val",
		"var", "when", "while":
		return true
	}
	return false
}

// UselessCallOnNotNullRule detects `.orEmpty()`, `.isNullOrEmpty()`, `.isNullOrBlank()`
// on definitely non-null receivers, and `listOfNotNull()`/`setOfNotNull()` with all
// non-null arguments.
type UselessCallOnNotNullRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence — flags ?-safe
// calls on provably non-null receivers; resolver-dependent. Classified per
// roadmap/17.
func (r *UselessCallOnNotNullRule) Confidence() float64 { return api.ConfidenceMedium }

// uselessNullCalls maps method names that are useless on non-null receivers to
// their replacement. Empty string means "remove the call entirely".
var uselessNullCalls = map[string]string{
	"orEmpty":       "",
	"isNullOrEmpty": "isEmpty",
	"isNullOrBlank": "isBlank",
}

// orEmptyValidTypes lists types that actually define .orEmpty().
var orEmptyValidTypes = map[string]bool{
	"String": true, "List": true, "Set": true, "Map": true,
	"Sequence":    true,
	"MutableList": true, "MutableSet": true, "MutableMap": true,
}

// isNullOrValidTypes lists types that define .isNullOrEmpty() / .isNullOrBlank().
var isNullOrValidTypes = map[string]bool{
	"String": true, "CharSequence": true,
	"List": true, "Set": true, "Map": true, "Collection": true,
	"MutableList": true, "MutableSet": true, "MutableMap": true,
}

// ofNotNullReplacements maps factory functions to their non-null equivalents.
var ofNotNullReplacements = map[string]string{
	"listOfNotNull": "listOf",
	"setOfNotNull":  "setOf",
}

// nullableStdlibCallMarkers are substrings that, when present in an
// argument expression, signal that the argument's value is nullable
// because it's the result of a stdlib function known to return T?.
// Used by UselessCallOnNotNull's *OfNotNull check to avoid false
// positives when the resolver doesn't encode the nullable return type.
var nullableStdlibCallMarkers = []string{
	".takeIf", ".takeUnless",
	".firstOrNull", ".lastOrNull", ".singleOrNull",
	".findOrNull", ".maxOrNull", ".minOrNull",
	".getOrNull", ".randomOrNull",
	".maxByOrNull", ".minByOrNull", ".maxWithOrNull", ".minWithOrNull",
	".toIntOrNull", ".toLongOrNull", ".toDoubleOrNull",
	".toFloatOrNull", ".toByteOrNull", ".toShortOrNull",
	".toBigIntegerOrNull", ".toBigDecimalOrNull",
	".toBooleanStrictOrNull", ".toUIntOrNull", ".toULongOrNull",
}

// containsNullableStdlibCall reports whether the expression text
// contains any call to a stdlib function that returns T?.
func containsNullableStdlibCall(text string) bool {
	for _, m := range nullableStdlibCallMarkers {
		if strings.Contains(text, m) {
			return true
		}
	}
	return false
}

func flatCallExpressionMethodSpan(file *scanner.File, idx uint32, methodName string) (int, int, bool) {
	if file == nil || idx == 0 || methodName == "" {
		return 0, 0, false
	}
	text := file.FlatNodeText(idx)
	needle := "." + methodName
	pos := strings.LastIndex(text, needle)
	if pos < 0 {
		return 0, 0, false
	}
	start := int(file.FlatStartByte(idx)) + pos + 1
	return start, start + len(methodName), true
}

// nonNullFactoryCalls are call prefixes that always produce non-null collections/strings.
var nonNullFactoryCalls = []string{
	"listOf(", "setOf(", "mapOf(",
	"mutableListOf(", "mutableSetOf(", "mutableMapOf(",
	"sequenceOf(", "emptyList(", "emptySet(", "emptyMap(",
}

// innerClassMayUseOuterInstance reports whether an inner class could need its
// outer instance. Without type information it is conservative: any `this@X`
// label, any mention of an outer member, sibling inner class or type
// parameter name, and any outer class with supertypes (inherited members are
// invisible here) count as a use.
func innerClassMayUseOuterInstance(file *scanner.File, inner uint32) bool {
	outer, ok := file.FlatParent(inner)
	if !ok {
		return true
	}
	outerBody := outer
	outer, ok = file.FlatParent(outerBody)
	if !ok || file.FlatType(outerBody) != "class_body" || file.FlatType(outer) != "class_declaration" {
		return true
	}
	for child := file.FlatFirstChild(outer); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "delegation_specifier", "delegation_specifiers", ":":
			return true
		}
	}
	names := make(map[string]bool)
	addName := func(n string) {
		if n != "" {
			names[strings.Trim(n, "`")] = true
		}
	}
	forEachDirectClassMemberFlat(file, outerBody, func(member uint32) {
		switch file.FlatType(member) {
		case "property_declaration":
			file.FlatWalkNodes(member, "variable_declaration", func(v uint32) {
				addName(extractIdentifierFlat(file, v))
			})
			addName(propertyDeclarationNameFlat(file, member))
		case "function_declaration":
			addName(extractIdentifierFlat(file, member))
		case "class_declaration":
			if member != inner && file.FlatHasModifier(member, "inner") {
				addName(extractIdentifierFlat(file, member))
			}
		}
	})
	file.FlatWalkAllNodes(outer, func(n uint32) {
		if p, ok := file.FlatParent(n); !ok || (p != outer && file.FlatType(p) != "primary_constructor" &&
			file.FlatType(p) != "class_parameter" && file.FlatType(p) != "type_parameters" && file.FlatType(p) != "type_parameter") {
			return
		}
		if t := file.FlatType(n); t == "simple_identifier" || t == "type_identifier" {
			// Parameters and type parameters of nested classes belong to those classes, not to the outer one.
			if owner, ok := flatEnclosingAncestor(file, n, "class_declaration"); ok && owner != outer {
				return
			}
			addName(file.FlatNodeText(n))
		}
	})
	used := false
	file.FlatWalkAllNodes(inner, func(n uint32) {
		if used {
			return
		}
		switch file.FlatType(n) {
		case "simple_identifier", "type_identifier":
			if n != inner && names[strings.Trim(file.FlatNodeText(n), "`")] {
				used = true
			}
		case "this_expression", "super_expression":
			if strings.Contains(file.FlatNodeText(n), "@") {
				used = true
			}
		}
	})
	return used
}
