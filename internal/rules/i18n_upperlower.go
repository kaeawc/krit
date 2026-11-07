package rules

import (
	"fmt"
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// UpperLowerInvariantMisuseRule flags Kotlin 1.5+ `uppercase()` /
// `lowercase()` calls whose result is shown as user-facing text. These
// methods are locale-invariant (they delegate to Locale.ROOT), which is
// correct for tokens and comparisons but wrong for text a user reads, where
// a display locale applies (Turkish dotted i, German sharp s, ...). Other
// uses are not reported; the deprecated default-locale variants
// (`toUpperCase()` / `toLowerCase()`) belong to ImplicitDefaultLocale.
type UpperLowerInvariantMisuseRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence because the rule
// matches on method name without type resolution; same-named local
// helpers will produce false positives.
func (r *UpperLowerInvariantMisuseRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *UpperLowerInvariantMisuseRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File

	navExpr, args := flatCallExpressionParts(file, idx)
	if navExpr == 0 {
		return
	}

	methodName := flatNavigationExpressionLastIdentifier(file, navExpr)
	if methodName != "uppercase" && methodName != "lowercase" {
		return
	}

	_, argCount := flatValueArgumentStats(file, args)
	if argCount != 0 {
		return
	}

	var receiverIdx uint32
	if file.FlatNamedChildCount(navExpr) > 0 {
		receiverIdx = file.FlatNamedChild(navExpr, 0)
	}
	if receiverIdx != 0 {
		recvText := file.FlatNodeText(receiverIdx)
		if containsASCIIInvariantIdentifier(recvText) {
			return
		}
		if upperLowerReceiverIsInvariantToken(ctx, receiverIdx) {
			return
		}
	}
	if upperLowerIsValueOfArgument(ctx, idx) {
		return
	}
	if !upperLowerResultFlowsToUIText(file, idx) {
		return
	}

	ctx.EmitAt(calleeLine(file, idx), calleeCol(file, idx),
		fmt.Sprintf("'%s()' is locale-invariant but its result is shown as user-facing text. Use a display-locale variant such as '%s(Locale.getDefault())'.", methodName, methodName))
}

// upperLowerUISinkCalls are calls whose arguments become text a user reads.
var upperLowerUISinkCalls = map[string]bool{
	"setText": true, "setTitle": true, "setSubtitle": true, "setHint": true,
	"setContentDescription": true, "setMessage": true, "setLabel": true, "setError": true,
	"setSummary": true, "makeText": true, "Text": true, "BasicText": true, "AnnotatedString": true,
}

// upperLowerUISinkProperties are properties whose assigned value becomes text a user reads.
var upperLowerUISinkProperties = map[string]bool{
	"text": true, "title": true, "subtitle": true, "hint": true, "contentDescription": true,
	"label": true, "error": true, "summary": true, "message": true, "headline": true, "placeholder": true,
}

// upperLowerResultFlowsToUIText reports whether the call is, within its statement, an argument of a UI text
// call or the right-hand side of an assignment to a UI text property.
func upperLowerResultFlowsToUIText(file *scanner.File, idx uint32) bool {
	child := idx
	for {
		p, ok := file.FlatParent(child)
		if !ok {
			return false
		}
		switch file.FlatType(p) {
		case "function_declaration", "lambda_literal", "statements", "class_body", "source_file":
			return false
		case "call_expression":
			if file.FlatType(child) == "call_suffix" && upperLowerUISinkCalls[flatCallExpressionName(file, p)] {
				return true
			}
		case "assignment":
			if child != file.FlatFirstChild(p) && upperLowerUISinkProperties[upperLowerAssignedName(file, file.FlatFirstChild(p))] {
				return true
			}
		}
		child = p
	}
}

// upperLowerAssignedName returns the last identifier of an assignment target, such as "text" for `binding.title?.text`.
func upperLowerAssignedName(file *scanner.File, target uint32) string {
	if target == 0 {
		return ""
	}
	text := strings.TrimSpace(file.FlatNodeText(target))
	if i := strings.LastIndex(text, "."); i >= 0 {
		text = text[i+1:]
	}
	return text
}

// upperLowerReceiverIsInvariantToken reports receivers that hold fixed ASCII tokens: string literals,
// enum names and enum toString() values. Only user-facing text needs a display locale.
func upperLowerReceiverIsInvariantToken(ctx *api.Context, receiver uint32) bool {
	file := ctx.File
	switch file.FlatType(receiver) {
	case "string_literal", "line_string_literal", "multi_line_string_literal":
		return !strings.Contains(file.FlatNodeText(receiver), "$")
	case "navigation_expression":
		return flatNavigationExpressionLastIdentifier(file, receiver) == "name"
	case "call_expression":
		navExpr, args := flatCallExpressionParts(file, receiver)
		if _, argCount := flatValueArgumentStats(file, args); argCount != 0 {
			return false
		}
		if navExpr == 0 {
			// Bare toString() inside an enum class.
			owner, ok := flatEnclosingAncestor(file, receiver, "class_declaration")
			return ok && flatCallExpressionName(file, receiver) == "toString" && upperLowerIsEnumClass(file, owner)
		}
		if flatNavigationExpressionLastIdentifier(file, navExpr) != "toString" || file.FlatNamedChildCount(navExpr) == 0 {
			return false
		}
		return upperLowerIsEnumTyped(ctx, file.FlatNamedChild(navExpr, 0))
	}
	return false
}

// upperLowerIsEnumTyped reports whether the expression resolves to an enum declared in the same file.
func upperLowerIsEnumTyped(ctx *api.Context, expr uint32) bool {
	file := ctx.File
	if file.FlatType(expr) == "this_expression" {
		owner, ok := flatEnclosingAncestor(file, expr, "class_declaration")
		return ok && upperLowerIsEnumClass(file, owner)
	}
	if file.FlatType(expr) != "simple_identifier" {
		return false
	}
	// Match a parameter or property declared in scope with an enum type from this file.
	var enums []string
	file.FlatWalkNodes(0, "class_declaration", func(decl uint32) {
		if upperLowerIsEnumClass(file, decl) {
			enums = append(enums, regexp.QuoteMeta(extractIdentifierFlat(file, decl)))
		}
	})
	if len(enums) == 0 {
		return false
	}
	declPattern := regexp.MustCompile(`(?:^|[\s(,])` + regexp.QuoteMeta(file.FlatNodeText(expr)) + `\s*:\s*(?:` + strings.Join(enums, "|") + `)\??\s*(?:$|[=,)])`)
	for p, ok := file.FlatParent(expr); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "function_declaration", "class_declaration", "object_declaration":
			found := false
			for _, declType := range []string{"parameter", "class_parameter", "property_declaration"} {
				file.FlatWalkNodes(p, declType, func(decl uint32) {
					if !found && declPattern.MatchString(file.FlatNodeText(decl)) {
						found = true
					}
				})
			}
			if found {
				return true
			}
		}
	}
	return false
}

// upperLowerIsValueOfArgument reports calls that feed Enum.valueOf-style lookups, where the case matches a fixed constant name.
func upperLowerIsValueOfArgument(ctx *api.Context, idx uint32) bool {
	file := ctx.File
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "call_expression":
			switch flatCallExpressionName(file, p) {
			case "valueOf", "enumValueOf", "valueOfOrNull":
				return true
			}
		case "function_declaration", "lambda_literal", "statements", "class_body":
			return false
		}
	}
	return false
}

func upperLowerIsEnumClass(file *scanner.File, decl uint32) bool {
	for c := file.FlatFirstChild(decl); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "enum" {
			return true
		}
	}
	return false
}
