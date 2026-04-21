package rules

import (
	"fmt"
	"regexp"
	"strings"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// MultilineLambdaItParameterRule detects 'it' in multiline lambdas.
type MultilineLambdaItParameterRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection uses structural pattern matching on
// expressions; the suggested rewrite's readability is a style call.
// Classified per roadmap/17.
func (r *MultilineLambdaItParameterRule) Confidence() float64 { return 0.75 }

// MultilineRawStringIndentationRule checks raw string indentation.
type MultilineRawStringIndentationRule struct {
	LineBase
	BaseRule
	IndentSize      int
	TrimmingMethods []string
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection uses structural pattern matching on
// expressions; the suggested rewrite's readability is a style call.
// Classified per roadmap/17.
func (r *MultilineRawStringIndentationRule) Confidence() float64 { return 0.75 }

func (r *MultilineRawStringIndentationRule) check(ctx *v2.Context) {
	file := ctx.File
	content := string(file.Content)
	idx := 0
	for {
		start := strings.Index(content[idx:], "\"\"\"")
		if start < 0 {
			break
		}
		start += idx
		end := strings.Index(content[start+3:], "\"\"\"")
		if end < 0 {
			break
		}
		end += start + 3
		raw := content[start : end+3]
		// Also check for trimIndent()/trimMargin() chained after the closing """
		afterEnd := end + 3
		trailing := ""
		if afterEnd < len(content) {
			trailEnd := afterEnd + 40
			if trailEnd > len(content) {
				trailEnd = len(content)
			}
			trailing = content[afterEnd:trailEnd]
		}
		if strings.Contains(raw, "\n") && !strings.Contains(raw, "trimIndent()") && !strings.Contains(raw, "trimMargin()") && !strings.Contains(trailing, "trimIndent()") && !strings.Contains(trailing, "trimMargin()") {
			line := strings.Count(content[:start], "\n") + 1
			ctx.Emit(r.Finding(file, line, 1,
				"Multiline raw string should use trimIndent() or trimMargin()."))
		}
		idx = end + 3
	}
}

// TrimMultilineRawStringRule detects raw strings missing trimIndent/trimMargin.
type TrimMultilineRawStringRule struct {
	LineBase
	BaseRule
	TrimmingMethods []string
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection uses structural pattern matching on
// expressions; the suggested rewrite's readability is a style call.
// Classified per roadmap/17.
func (r *TrimMultilineRawStringRule) Confidence() float64 { return 0.75 }

func (r *TrimMultilineRawStringRule) check(ctx *v2.Context) {
	file := ctx.File
	content := string(file.Content)
	idx := 0
	for {
		start := strings.Index(content[idx:], "\"\"\"")
		if start < 0 {
			break
		}
		start += idx
		end := strings.Index(content[start+3:], "\"\"\"")
		if end < 0 {
			break
		}
		end += start + 3
		raw := content[start : end+3]
		afterEnd := end + 3
		rest := ""
		if afterEnd < len(content) {
			rest = content[afterEnd:]
		}
		if strings.Contains(raw, "\n") &&
			!strings.HasPrefix(strings.TrimSpace(rest), ".trimIndent()") &&
			!strings.HasPrefix(strings.TrimSpace(rest), ".trimMargin()") {
			line := strings.Count(content[:start], "\n") + 1
			f := r.Finding(file, line, 1,
				"Multiline raw string should use trimIndent() or trimMargin().")
			f.Fix = &scanner.Fix{
				ByteMode:    true,
				StartByte:   end + 3,
				EndByte:     end + 3,
				Replacement: ".trimIndent()",
			}
			ctx.Emit(f)
		}
		idx = end + 3
	}
}

// StringShouldBeRawStringRule detects strings with many escape characters.
type StringShouldBeRawStringRule struct {
	FlatDispatchBase
	BaseRule
	MaxEscapes int
}

var escapeCountRe = regexp.MustCompile(`\\[nrt"\\]`)

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection uses structural pattern matching on
// expressions; the suggested rewrite's readability is a style call.
// Classified per roadmap/17.
func (r *StringShouldBeRawStringRule) Confidence() float64 { return 0.75 }

// CanBeNonNullableRule detects nullable types that are never assigned null.
// Handles two cases:
// 1. Properties initialized with non-null values that are never reassigned to null.
// 2. Function parameters declared nullable but only used with !! (non-null assertion).
// Skips override/open/abstract functions, delegated properties, and properties with custom setters.
// Tracks null assignments through if/when branches and lambda bodies.
//
// Limitations vs detekt (which uses full data-flow analysis, 609 lines):
//   - Cannot track nullable assignments through function calls (fun setNull(x) { field = null })
//   - Cannot detect properties assigned null via reflection or Java interop
type CanBeNonNullableRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *CanBeNonNullableRule) SetResolver(res typeinfer.TypeResolver) {}

// Confidence reports a tier-2 (medium) base confidence — detecting which
// nullable properties are never assigned null requires flow analysis; the
// fallback is a conservative heuristic. Classified per roadmap/17.
func (r *CanBeNonNullableRule) Confidence() float64 { return 0.75 }

func (r *CanBeNonNullableRule) checkPropertyFlat(ctx *v2.Context) {
	idx, file := ctx.Idx, ctx.File
	text := file.FlatNodeText(idx)
	if !strings.Contains(text, "?") {
		return
	}

	if file.FlatHasChildOfType(idx, "property_delegate") {
		return
	}

	if nextSib, ok := file.FlatNextSibling(idx); ok && file.FlatType(nextSib) == "setter" {
		return
	}
	if nextSib, ok := file.FlatNextSibling(idx); ok && file.FlatType(nextSib) == "getter" {
		if nextNext, ok := file.FlatNextSibling(nextSib); ok && file.FlatType(nextNext) == "setter" {
			return
		}
	}

	hasNullable := false
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if child == 0 {
			continue
		}
		if file.FlatType(child) == "nullable_type" {
			hasNullable = true
			break
		}
		if file.FlatType(child) == "variable_declaration" {
			for j := 0; j < file.FlatChildCount(child); j++ {
				gc := file.FlatChild(child, j)
				if file.FlatType(gc) == "nullable_type" {
					hasNullable = true
					break
				}
			}
		}
	}
	if !hasNullable {
		return
	}

	if !file.FlatHasChildOfType(idx, "=") {
		return
	}

	var initExpr uint32
	foundEq := false
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) == "=" {
			foundEq = true
			continue
		}
		if foundEq {
			initExpr = child
			break
		}
	}
	if initExpr != 0 {
		if strings.TrimSpace(file.FlatNodeText(initExpr)) == "null" {
			return
		}
	}

	propName := ""
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) == "variable_declaration" {
			propName = extractIdentifierFlat(file, child)
			break
		}
	}

	isVar := false
	for i := 0; i < file.FlatChildCount(idx); i++ {
		if file.FlatNodeText(file.FlatChild(idx, i)) == "var" {
			isVar = true
			break
		}
	}

	if isVar && propName != "" {
		scope, ok := file.FlatParent(idx)
		if ok && r.hasNullAssignmentInScopeFlat(file, scope, idx, propName) {
			return
		}
	}

	ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, 1,
		"Property type can be non-nullable since it is initialized with a non-null value and never assigned null."))
}

func (r *CanBeNonNullableRule) hasNullAssignmentInScopeFlat(file *scanner.File, scope, declNode uint32, propName string) bool {
	assignedNull := false
	file.FlatWalkAllNodes(scope, func(child uint32) {
		if assignedNull || child == declNode {
			return
		}
		if file.FlatType(child) == "assignment" || file.FlatType(child) == "augmented_assignment" {
			if file.FlatChildCount(child) < 2 {
				return
			}
			lhs := file.FlatChild(child, 0)
			lhsText := strings.TrimSpace(file.FlatNodeText(lhs))
			if lhsText != propName && lhsText != "this."+propName {
				return
			}
			rhs := file.FlatChild(child, file.FlatChildCount(child)-1)
			rhsText := strings.TrimSpace(file.FlatNodeText(rhs))
			if rhsText == "null" || strings.Contains(rhsText, "?") {
				assignedNull = true
				return
			}
		}
	})
	return assignedNull
}

func (r *CanBeNonNullableRule) checkFunctionParamsFlat(ctx *v2.Context) {
	idx, file := ctx.Idx, ctx.File
	if file.FlatHasModifier(idx, "override") || file.FlatHasModifier(idx, "open") || file.FlatHasModifier(idx, "abstract") {
		return
	}

	body, _ := file.FlatFindChild(idx, "function_body")
	if body == 0 {
		return
	}
	bodyText := file.FlatNodeText(body)

	params, _ := file.FlatFindChild(idx, "function_value_parameters")
	if params == 0 {
		return
	}

	for i := 0; i < file.FlatNamedChildCount(params); i++ {
		param := file.FlatNamedChild(params, i)
		if param == 0 || file.FlatType(param) != "parameter" {
			continue
		}
		if !file.FlatHasChildOfType(param, "nullable_type") {
			continue
		}

		paramName := extractIdentifierFlat(file, param)
		if paramName == "" || !strings.Contains(bodyText, paramName) {
			continue
		}

		allNonNullAsserted := true
		usageCount := 0
		file.FlatWalkAllNodes(body, func(child uint32) {
			if !allNonNullAsserted || file.FlatType(child) != "simple_identifier" || !file.FlatNodeTextEquals(child, paramName) {
				return
			}
			usageCount++
			parent, ok := file.FlatParent(child)
			if !ok {
				allNonNullAsserted = false
				return
			}
			switch file.FlatType(parent) {
			case "non_null_assertion":
				return
			case "postfix_unary_expression":
				if strings.HasSuffix(strings.TrimSpace(file.FlatNodeText(parent)), "!!") {
					return
				}
			}
			allNonNullAsserted = false
		})

		if usageCount > 0 && allNonNullAsserted {
			ctx.Emit(r.Finding(file, file.FlatRow(param)+1, 1,
				fmt.Sprintf("Parameter '%s' can be non-nullable since every usage applies non-null assertion (!!).", paramName)))
		}
	}
}

// DoubleNegativeExpressionRule detects `!isNotEmpty()` etc.
type DoubleNegativeExpressionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection uses structural pattern matching on
// expressions; the suggested rewrite's readability is a style call.
// Classified per roadmap/17.
func (r *DoubleNegativeExpressionRule) Confidence() float64 { return 0.75 }

var doubleNegFixRe = regexp.MustCompile(`!(\w*\.\s*)is(Not|Non)(\w+)\(\)`)

// DoubleNegativeLambdaRule detects `filterNot { !it }`, `none { }`, `takeUnless { }`.
type DoubleNegativeLambdaRule struct {
	FlatDispatchBase
	BaseRule
	NegativeFunctions []string
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection uses structural pattern matching on
// expressions; the suggested rewrite's readability is a style call.
// Classified per roadmap/17.
func (r *DoubleNegativeLambdaRule) Confidence() float64 { return 0.75 }

var filterNotNegRe = regexp.MustCompile(`\.filterNot\s*\{[^}]*![^}]*\}`)
var noneNegRe = regexp.MustCompile(`\.none\s*\{[^}]*![^}]*\}`)

// NullableBooleanCheckRule detects `x == true` on Boolean?.
type NullableBooleanCheckRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection uses structural pattern matching on
// expressions; the suggested rewrite's readability is a style call.
// Classified per roadmap/17.
func (r *NullableBooleanCheckRule) Confidence() float64 { return 0.75 }

// RangeUntilInsteadOfRangeToRule detects `until` usage that could use `..<`.
type RangeUntilInsteadOfRangeToRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection uses structural pattern matching on
// expressions; the suggested rewrite's readability is a style call.
// Classified per roadmap/17.
func (r *RangeUntilInsteadOfRangeToRule) Confidence() float64 { return 0.75 }

// DestructuringDeclarationWithTooManyEntriesRule limits destructuring entries.
type DestructuringDeclarationWithTooManyEntriesRule struct {
	FlatDispatchBase
	BaseRule
	MaxEntries int
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection uses structural pattern matching on
// expressions; the suggested rewrite's readability is a style call.
// Classified per roadmap/17.
func (r *DestructuringDeclarationWithTooManyEntriesRule) Confidence() float64 { return 0.75 }

