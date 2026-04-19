package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// RedundantVisibilityModifierRule detects explicit `public` keyword.
type RedundantVisibilityModifierRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/redundant rule. Detection flags visible but arguably-redundant
// modifiers, types, or keywords. Whether removal improves readability is
// context-dependent. Classified per roadmap/17.
func (r *RedundantVisibilityModifierRule) Confidence() float64 { return 0.75 }

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
func (r *RedundantConstructorKeywordRule) Confidence() float64 { return 0.75 }

// RedundantExplicitTypeRule detects type annotations where the type is obvious.
// With type inference: uses ResolveNode on both the declared type and the initializer
// expression. If both resolve to the same FQN, the explicit type is redundant.
type RedundantExplicitTypeRule struct {
	FlatDispatchBase
	BaseRule
	resolver typeinfer.TypeResolver
}

func (r *RedundantExplicitTypeRule) SetResolver(res typeinfer.TypeResolver) {
	r.resolver = res
}

// Confidence reports a tier-2 (medium) base confidence — inferring
// whether the type annotation is necessary requires the resolver; fallback
// is conservative. Classified per roadmap/17.
func (r *RedundantExplicitTypeRule) Confidence() float64 { return 0.75 }

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

// UnnecessaryParenthesesRule detects unnecessary parentheses around expressions.
// Matches detekt's UnnecessaryParentheses: flags parens around return values,
// if/when conditions, assignments, double-wrapped parens, and other contexts
// where the parentheses add no value.
type UnnecessaryParenthesesRule struct {
	FlatDispatchBase
	BaseRule
	AllowForUnclearPrecedence bool // if true, allow parens that clarify operator precedence
}

// Confidence reports a tier-2 (medium) base confidence. Style/redundant rule. Detection flags visible but arguably-redundant
// modifiers, types, or keywords. Whether removal improves readability is
// context-dependent. Classified per roadmap/17.
func (r *UnnecessaryParenthesesRule) Confidence() float64 { return 0.75 }

func unnParensIsIfConditionFlat(file *scanner.File, node, parent uint32) bool {
	for i := 0; i < file.FlatChildCount(parent); i++ {
		child := file.FlatChild(parent, i)
		if child == 0 {
			continue
		}
		if t := file.FlatType(child); t == "control_structure_body" || t == "{" {
			return false
		}
		if child == node {
			return true
		}
	}
	return false
}

func unnParensIsWhenSubjectFlat(file *scanner.File, node, parent uint32) bool {
	for i := 0; i < file.FlatChildCount(parent); i++ {
		child := file.FlatChild(parent, i)
		if child == 0 {
			continue
		}
		if t := file.FlatType(child); t == "when_entry" || t == "{" {
			return false
		}
		if child == node {
			return true
		}
	}
	return false
}

func unnParensInnerIsSafeFlat(file *scanner.File, inner uint32) bool {
	switch file.FlatType(inner) {
	case "simple_identifier", "integer_literal", "long_literal",
		"real_literal", "boolean_literal", "character_literal",
		"string_literal", "null_literal", "this_expression",
		"super_expression", "call_expression", "navigation_expression",
		"indexing_expression", "parenthesized_expression",
		"object_literal", "lambda_literal", "when_expression",
		"if_expression", "try_expression", "collection_literal":
		return true
	}
	return false
}

// Binary operator node types in tree-sitter Kotlin.
var unnParensBinaryExprTypes = map[string]bool{
	"multiplicative_expression": true,
	"additive_expression":       true,
	"range_expression":          true,
	"infix_expression":          true,
	"elvis_expression":          true,
	"check_expression":          true,
	"comparison_expression":     true,
	"equality_expression":       true,
	"conjunction_expression":    true,
	"disjunction_expression":    true,
}

// Precedence rank (higher = binds tighter).
var unnParensBinaryPrecedence = map[string]int{
	"disjunction_expression":    1,
	"conjunction_expression":    2,
	"equality_expression":       3,
	"comparison_expression":     4,
	"check_expression":          5,
	"range_expression":          6,
	"additive_expression":       7,
	"multiplicative_expression": 8,
	"infix_expression":          9,
	"elvis_expression":          10,
}

func unnParensClarifyPrecedenceFlat(file *scanner.File, parenNode, inner uint32) bool {
	if !unnParensBinaryExprTypes[file.FlatType(inner)] {
		if file.FlatType(inner) == "prefix_expression" {
			outerParent, ok := file.FlatParent(parenNode)
			if ok && unnParensBinaryExprTypes[file.FlatType(outerParent)] {
				return true
			}
		}
		return false
	}
	outerParent, ok := file.FlatParent(parenNode)
	for ok && file.FlatType(outerParent) == "parenthesized_expression" {
		outerParent, ok = file.FlatParent(outerParent)
	}
	if !ok || !unnParensBinaryExprTypes[file.FlatType(outerParent)] {
		return false
	}
	innerPrec := unnParensBinaryPrecedence[file.FlatType(inner)]
	outerPrec := unnParensBinaryPrecedence[file.FlatType(outerParent)]
	return innerPrec != outerPrec
}

// UnnecessaryInheritanceRule detects `: Any()`.
type UnnecessaryInheritanceRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/redundant rule. Detection flags visible but arguably-redundant
// modifiers, types, or keywords. Whether removal improves readability is
// context-dependent. Classified per roadmap/17.
func (r *UnnecessaryInheritanceRule) Confidence() float64 { return 0.75 }

// UnnecessaryInnerClassRule detects inner classes that don't use the outer reference.
type UnnecessaryInnerClassRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/redundant rule. Detection flags visible but arguably-redundant
// modifiers, types, or keywords. Whether removal improves readability is
// context-dependent. Classified per roadmap/17.
func (r *UnnecessaryInnerClassRule) Confidence() float64 { return 0.75 }

// OptionalUnitRule detects explicit `: Unit` return types on functions
// and redundant `return Unit` statements inside function bodies.
type OptionalUnitRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/redundant rule. Detection flags visible but arguably-redundant
// modifiers, types, or keywords. Whether removal improves readability is
// context-dependent. Classified per roadmap/17.
func (r *OptionalUnitRule) Confidence() float64 { return 0.75 }

// UnnecessaryBackticksRule detects unnecessary backtick-quoted identifiers.
type UnnecessaryBackticksRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/redundant rule. Detection flags visible but arguably-redundant
// modifiers, types, or keywords. Whether removal improves readability is
// context-dependent. Classified per roadmap/17.
func (r *UnnecessaryBackticksRule) Confidence() float64 { return 0.75 }

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
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_') {
				return false
			}
		} else {
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_') {
				return false
			}
		}
	}
	return true
}

func isIdentChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_'
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
// non-null arguments. Mirrors detekt's UselessCallOnNotNull.
type UselessCallOnNotNullRule struct {
	FlatDispatchBase
	BaseRule
	resolver typeinfer.TypeResolver
}

func (r *UselessCallOnNotNullRule) SetResolver(res typeinfer.TypeResolver) { r.resolver = res }

// Confidence reports a tier-2 (medium) base confidence — flags ?-safe
// calls on provably non-null receivers; resolver-dependent. Classified per
// roadmap/17.
func (r *UselessCallOnNotNullRule) Confidence() float64 { return 0.75 }

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
