package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// LabeledExpressionRule detects return@label, break@label, continue@label.
type LabeledExpressionRule struct {
	FlatDispatchBase
	BaseRule
	IgnoredLabels []string
}

// Confidence reports a tier-2 (medium) base confidence. This rule
// uses a threshold-based metric (line count, nesting depth, branch
// count, parameter count, etc.) against a configurable limit. The
// counting is structurally precise but the threshold is a style
// preference that varies by codebase — a given value may be
// conservative in some contexts and lax in others. Classified per
// roadmap/17.
func (r *LabeledExpressionRule) Confidence() float64 { return api.ConfidenceMedium }

// labeledExpressionLabelIgnored reports whether the label-node text matches
// any name in the ignored list. Tree-sitter Kotlin emits a `label` node for
// the trailing-`@` form (`process@`) used at the definition site; the
// comparison strips the `@` so users configure plain identifiers like
// `forEach` or `process`.
func labeledExpressionLabelIgnored(labelText string, ignored []string) bool {
	if len(ignored) == 0 {
		return false
	}
	name := strings.TrimSuffix(strings.TrimPrefix(labelText, "@"), "@")
	if name == "" {
		return false
	}
	for _, candidate := range ignored {
		if candidate == name {
			return true
		}
	}
	return false
}

// NamedArgumentsRule detects function calls with too many unnamed arguments.
type NamedArgumentsRule struct {
	FlatDispatchBase
	BaseRule
	AllowedArguments int
}

// Confidence reports a tier-2 (medium) base confidence. This rule
// uses a threshold-based metric (line count, nesting depth, branch
// count, parameter count, etc.) against a configurable limit. The
// counting is structurally precise but the threshold is a style
// preference that varies by codebase — a given value may be
// conservative in some contexts and lax in others. Classified per
// roadmap/17.
func (r *NamedArgumentsRule) Confidence() float64 { return api.ConfidenceMedium }

// NestedScopeFunctionsRule detects nested scope functions (apply, also, let, run, with).
type NestedScopeFunctionsRule struct {
	FlatDispatchBase
	BaseRule
	AllowedDepth int
	Functions    []string
}

var scopeFunctions = map[string]bool{
	"apply": true, "also": true, "let": true, "run": true, "with": true,
}

// nestedScopeFunctionsActiveSet returns the set of callee names treated
// as scope functions for NestedScopeFunctionsRule. When the configured
// list is empty (the registry default), the always-on set covers the
// five Kotlin standard scope functions. A non-empty configured list
// fully replaces the defaults so users can include project-specific
// scope-style helpers (or narrow the set to exclude one of the five).
func nestedScopeFunctionsActiveSet(configured []string) map[string]bool {
	if len(configured) == 0 {
		return scopeFunctions
	}
	out := make(map[string]bool, len(configured))
	for _, name := range configured {
		name = strings.TrimSpace(name)
		if name != "" {
			out[name] = true
		}
	}
	return out
}

// Confidence reports a tier-2 (medium) base confidence. This rule
// uses a threshold-based metric (line count, nesting depth, branch
// count, parameter count, etc.) against a configurable limit. The
// counting is structurally precise but the threshold is a style
// preference that varies by codebase — a given value may be
// conservative in some contexts and lax in others. Classified per
// roadmap/17.
func (r *NestedScopeFunctionsRule) Confidence() float64 { return api.ConfidenceMedium }

func extractCallNameFlat(file *scanner.File, idx uint32) string {
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		switch file.FlatType(child) {
		case "simple_identifier":
			return file.FlatNodeText(child)
		case "navigation_expression":
			for j := file.FlatChildCount(child) - 1; j >= 0; j-- {
				gc := file.FlatChild(child, j)
				if file.FlatType(gc) == "navigation_suffix" {
					for k := 0; k < file.FlatChildCount(gc); k++ {
						ggc := file.FlatChild(gc, k)
						if file.FlatType(ggc) == "simple_identifier" {
							return file.FlatNodeText(ggc)
						}
					}
				}
				if file.FlatType(gc) == "simple_identifier" {
					return file.FlatNodeText(gc)
				}
			}
			return ""
		}
	}
	return ""
}

// ReplaceSafeCallChainWithRunRule detects 3+ chained safe calls (?.).
type ReplaceSafeCallChainWithRunRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. This rule
// uses a threshold-based metric (line count, nesting depth, branch
// count, parameter count, etc.) against a configurable limit. The
// counting is structurally precise but the threshold is a style
// preference that varies by codebase — a given value may be
// conservative in some contexts and lax in others. Classified per
// roadmap/17.
func (r *ReplaceSafeCallChainWithRunRule) Confidence() float64 { return api.ConfidenceMedium }

func countSafeCallsInChainFlat(file *scanner.File, idx uint32) int {
	if file.FlatType(idx) != "navigation_expression" {
		return 0
	}
	count := 0
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) == "navigation_suffix" && strings.HasPrefix(file.FlatNodeText(child), "?.") {
			count++
		}
	}
	if file.FlatChildCount(idx) == 0 {
		return count
	}
	receiver := file.FlatChild(idx, 0)
	switch file.FlatType(receiver) {
	case "navigation_expression":
		count += countSafeCallsInChainFlat(file, receiver)
	case "call_expression":
		if file.FlatChildCount(receiver) > 0 {
			inner := file.FlatChild(receiver, 0)
			if file.FlatType(inner) == "navigation_expression" {
				count += countSafeCallsInChainFlat(file, inner)
			}
		}
	}
	return count
}

// StringLiteralDuplicationRule detects duplicate string literals in a file.
type StringLiteralDuplicationRule struct {
	FlatDispatchBase
	BaseRule
	AllowedDuplications       int
	AllowedWithLengthLessThan int
	IgnoreAnnotation          bool
	IgnoreStringsRegex        string
}

// Confidence reports a tier-2 (medium) base confidence. This rule
// uses a threshold-based metric (line count, nesting depth, branch
// count, parameter count, etc.) against a configurable limit. The
// counting is structurally precise but the threshold is a style
// preference that varies by codebase — a given value may be
// conservative in some contexts and lax in others. Classified per
// roadmap/17.
func (r *StringLiteralDuplicationRule) Confidence() float64 { return api.ConfidenceMedium }

// stringLiteralUnquote strips the surrounding quote delimiters from a
// Kotlin string literal's raw text so that user-supplied
// IgnoreStringsRegex patterns match against the contents rather than
// the quoted form. Both regular ("...") and triple-quoted ("""...""")
// shapes are handled.
func stringLiteralUnquote(text string) string {
	if strings.HasPrefix(text, `"""`) && strings.HasSuffix(text, `"""`) && len(text) >= 6 {
		return text[3 : len(text)-3]
	}
	if strings.HasPrefix(text, `"`) && strings.HasSuffix(text, `"`) && len(text) >= 2 {
		return text[1 : len(text)-1]
	}
	return text
}

// stringLiteralInsideAnnotationFlat reports whether the given
// string_literal node is positioned inside any enclosing `annotation`
// AST node. Used by StringLiteralDuplicationRule.IgnoreAnnotation to
// skip strings that live in annotation arguments such as
// `@Suppress("UNCHECKED_CAST")`. The walk stops at the nearest
// declaration boundary so it doesn't scan the whole file.
func stringLiteralInsideAnnotationFlat(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	cur := idx
	for {
		parent, ok := file.FlatParent(cur)
		if !ok {
			return false
		}
		t := file.FlatType(parent)
		if t == "annotation" {
			return true
		}
		switch t {
		case "function_declaration", "class_declaration", "object_declaration",
			"property_declaration", "function_body", "class_body", "source_file":
			return false
		}
		cur = parent
	}
}
