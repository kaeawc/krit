package rules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

var useIsNullOrEmptyTextRe = regexp.MustCompile(`^(?:([A-Za-z_][A-Za-z0-9_\.]*)==null|null==([A-Za-z_][A-Za-z0-9_\.]*))\|\|([A-Za-z_][A-Za-z0-9_\.]*)(?:\.isEmpty\(\)|\.(?:size|length)==0|\.count\(\)==0|=="")$`)

func flatNonNullCheckText(file *scanner.File, idx uint32, funcName string) (argText string, lambdaText string, ok bool) {
	if file == nil || file.FlatType(idx) != "call_expression" {
		return "", "", false
	}
	if flatCallExpressionName(file, idx) != funcName {
		return "", "", false
	}
	suffix := file.FlatFindChild(idx, "call_suffix")
	if suffix == 0 {
		return "", "", false
	}
	var eq uint32
	file.FlatWalkNodes(suffix, "equality_expression", func(candidate uint32) {
		if eq == 0 {
			eq = candidate
		}
	})
	if eq == 0 || file.FlatType(eq) != "equality_expression" || file.FlatChildCount(eq) < 3 {
		return "", "", false
	}
	left := file.FlatChild(eq, 0)
	op := file.FlatChild(eq, 1)
	right := file.FlatChild(eq, file.FlatChildCount(eq)-1)
	if left == 0 || op == 0 || right == 0 || !file.FlatNodeTextEquals(op, "!=") {
		return "", "", false
	}
	leftText := strings.TrimSpace(file.FlatNodeText(left))
	rightText := strings.TrimSpace(file.FlatNodeText(right))
	if leftText == "null" {
		argText = rightText
	} else if rightText == "null" {
		argText = leftText
	} else {
		return "", "", false
	}
	if argText == "" {
		return "", "", false
	}
	if lambda := flatCallSuffixLambdaNode(file, suffix); lambda != 0 {
		lambdaText = file.FlatNodeText(lambda)
	}
	return argText, lambdaText, true
}

func flatThrowPattern(nodeType, nodeText string, file *scanner.File, idx uint32, exceptionType, replacement string, base BaseRule) []scanner.Finding {
	if file == nil || nodeType != "if_expression" {
		return nil
	}
	if strings.Contains(nodeText, "else") && file.FlatFindChild(idx, "else") != 0 {
		return nil
	}
	var condNode, bodyNode uint32
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		switch file.FlatType(child) {
		case "if", "(", ")", "{", "}":
			continue
		default:
			if condNode == 0 {
				condNode = child
			} else if bodyNode == 0 {
				bodyNode = child
			}
		}
	}
	if condNode == 0 || bodyNode == 0 {
		return nil
	}
	condText := strings.TrimSpace(file.FlatNodeText(condNode))
	isNegated := false
	innerCond := condText
	if strings.HasPrefix(condText, "!") {
		isNegated = true
		innerCond = strings.TrimSpace(condText[1:])
		if strings.HasPrefix(innerCond, "(") && strings.HasSuffix(innerCond, ")") {
			innerCond = innerCond[1 : len(innerCond)-1]
		}
	} else if file.FlatType(condNode) == "prefix_expression" && file.FlatChildCount(condNode) >= 2 {
		opNode := file.FlatChild(condNode, 0)
		if opNode != 0 && file.FlatNodeTextEquals(opNode, "!") {
			isNegated = true
			if argNode := file.FlatChild(condNode, 1); argNode != 0 {
				innerCond = file.FlatNodeText(argNode)
			}
		}
	}
	if !isNegated {
		return nil
	}
	bodyText := strings.TrimSpace(file.FlatNodeText(bodyNode))
	if strings.HasPrefix(bodyText, "{") && strings.HasSuffix(bodyText, "}") {
		bodyText = strings.TrimSpace(bodyText[1 : len(bodyText)-1])
	}
	if !strings.HasPrefix(bodyText, "throw ") {
		return nil
	}
	throwTarget := strings.TrimSpace(bodyText[6:])
	if !strings.HasPrefix(throwTarget, exceptionType) {
		return nil
	}
	f := base.Finding(file, file.FlatRow(idx)+1, 1, fmt.Sprintf("Use '%s()' instead of 'if (...) throw %s'.", replacement, exceptionType))
	if argStart := strings.Index(throwTarget, "("); argStart >= 0 {
		if argEnd := strings.LastIndex(throwTarget, ")"); argEnd > argStart {
			arg := strings.TrimSpace(throwTarget[argStart+1 : argEnd])
			if strings.HasPrefix(arg, "\"") && strings.HasSuffix(arg, "\"") {
				f.Fix = &scanner.Fix{ByteMode: true, StartByte: int(file.FlatStartByte(idx)), EndByte: int(file.FlatEndByte(idx)), Replacement: fmt.Sprintf("%s(%s) { %s }", replacement, innerCond, arg)}
			}
		}
	}
	return []scanner.Finding{f}
}

func flatNullOrEmptyNullCheckedVar(file *scanner.File, node uint32) string {
	node = flatUnwrapParenExpr(file, node)
	if file == nil || node == 0 || file.FlatType(node) != "equality_expression" || file.FlatNamedChildCount(node) < 2 {
		return ""
	}
	left := file.FlatNamedChild(node, 0)
	right := file.FlatNamedChild(node, 1)
	if left == 0 || right == 0 {
		return ""
	}
	if file.FlatChildCount(node) >= 3 {
		op := file.FlatChild(node, 1)
		if op != 0 && !file.FlatNodeTextEquals(op, "==") {
			return ""
		}
	}
	leftText := file.FlatNodeText(left)
	rightText := file.FlatNodeText(right)
	if rightText == "null" {
		return leftText
	}
	if leftText == "null" {
		return rightText
	}
	return ""
}

func flatNullOrEmptyEmptyCheckedVar(file *scanner.File, node uint32) string {
	node = flatUnwrapParenExpr(file, node)
	if file == nil || node == 0 {
		return ""
	}
	switch file.FlatType(node) {
	case "call_expression":
		return flatNullOrEmptyFromCallExpr(file, node)
	case "equality_expression":
		return flatNullOrEmptyFromEqualityExpr(file, node)
	default:
		return ""
	}
}

func flatNullOrEmptyNavSelector(file *scanner.File, nav uint32) string {
	if file == nil || nav == 0 || file.FlatNamedChildCount(nav) < 2 {
		return ""
	}
	lastChild := file.FlatNamedChild(nav, file.FlatNamedChildCount(nav)-1)
	if lastChild == 0 {
		return ""
	}
	if file.FlatType(lastChild) == "navigation_suffix" {
		for i := 0; i < file.FlatNamedChildCount(lastChild); i++ {
			ident := file.FlatNamedChild(lastChild, i)
			if file.FlatType(ident) == "simple_identifier" {
				return file.FlatNodeText(ident)
			}
		}
	}
	return file.FlatNodeText(lastChild)
}

func flatNullOrEmptyNavReceiver(file *scanner.File, nav uint32) string {
	if file == nil || nav == 0 || file.FlatNamedChildCount(nav) < 1 {
		return ""
	}
	receiver := file.FlatNamedChild(nav, 0)
	if receiver == 0 {
		return ""
	}
	return file.FlatNodeText(receiver)
}

func flatNullOrEmptyFromCallExpr(file *scanner.File, node uint32) string {
	if flatCallExpressionName(file, node) != "isEmpty" {
		return ""
	}
	_, args := flatCallExpressionParts(file, node)
	if args != 0 && file.FlatNamedChildCount(args) > 0 {
		return ""
	}
	navExpr, _ := flatCallExpressionParts(file, node)
	if navExpr == 0 {
		return ""
	}
	return flatNullOrEmptyNavReceiver(file, navExpr)
}

func flatNullOrEmptyFromEqualityExpr(file *scanner.File, node uint32) string {
	if file == nil || node == 0 || file.FlatNamedChildCount(node) < 2 {
		return ""
	}
	left := file.FlatNamedChild(node, 0)
	right := file.FlatNamedChild(node, 1)
	if left == 0 || right == 0 {
		return ""
	}
	if file.FlatChildCount(node) >= 3 {
		op := file.FlatChild(node, 1)
		if op != 0 && !file.FlatNodeTextEquals(op, "==") {
			return ""
		}
	}
	leftText := file.FlatNodeText(left)
	rightText := file.FlatNodeText(right)
	if rightText == `""` {
		return leftText
	}
	if leftText == `""` {
		return rightText
	}
	if rightText == "0" {
		return flatNullOrEmptyFromSizeOrCount(file, left)
	}
	if leftText == "0" {
		return flatNullOrEmptyFromSizeOrCount(file, right)
	}
	return ""
}

func flatNullOrEmptyFromSizeOrCount(file *scanner.File, node uint32) string {
	if file == nil || node == 0 {
		return ""
	}
	switch file.FlatType(node) {
	case "call_expression":
		if flatCallExpressionName(file, node) != "count" {
			return ""
		}
		_, args := flatCallExpressionParts(file, node)
		if args != 0 && file.FlatNamedChildCount(args) > 0 {
			return ""
		}
		navExpr, _ := flatCallExpressionParts(file, node)
		if navExpr == 0 {
			return ""
		}
		return flatNullOrEmptyNavReceiver(file, navExpr)
	case "navigation_expression":
		propName := flatNullOrEmptyNavSelector(file, node)
		if !isNullOrEmptySizeProps[propName] {
			return ""
		}
		return flatNullOrEmptyNavReceiver(file, node)
	default:
		return ""
	}
}

func flatIsEmptyRHS(file *scanner.File, node uint32) bool {
	if file == nil || node == 0 {
		return false
	}
	text := strings.TrimSpace(file.FlatNodeText(node))
	if text == `""` || text == `""""""` {
		return true
	}
	switch file.FlatType(node) {
	case "call_expression":
		name := flatCallExpressionName(file, node)
		if useOrEmptyFunctions[name] {
			return true
		}
		if useOrEmptyFactoryFunctions[name] {
			_, args := flatCallExpressionParts(file, node)
			return args == 0 || file.FlatNamedChildCount(args) == 0
		}
	}
	return false
}

// UseCheckNotNullRule detects check(x != null) and suggests checkNotNull(x).
// Uses AST dispatch on call_expression for precise detection, handling both
// `x != null` and `null != x` argument order, nested expressions, and
// optional message lambdas like `check(x != null) { "msg" }`.
type UseCheckNotNullRule struct {
	FlatDispatchBase
	BaseRule
	resolver typeinfer.TypeResolver
}

func (r *UseCheckNotNullRule) SetResolver(res typeinfer.TypeResolver) { r.resolver = res }

// Confidence reports a tier-2 (medium) base confidence — suggests
// checkNotNull over `if (x == null) throw`; pattern-based with resolver
// used to confirm nullability when available. Classified per roadmap/17.
func (r *UseCheckNotNullRule) Confidence() float64 { return 0.75 }

// UseRequireNotNullRule detects require(x != null) and suggests requireNotNull(x).
// Uses AST dispatch on call_expression for precise detection, handling both
// `x != null` and `null != x` argument order, nested expressions, and
// optional message lambdas like `require(x != null) { "msg" }`.
type UseRequireNotNullRule struct {
	FlatDispatchBase
	BaseRule
	resolver typeinfer.TypeResolver
}

func (r *UseRequireNotNullRule) SetResolver(res typeinfer.TypeResolver) { r.resolver = res }

// Confidence reports a tier-2 (medium) base confidence — suggests
// requireNotNull over `if (x == null) throw IAE`; pattern-based with
// resolver confirmation when available. Classified per roadmap/17.
func (r *UseRequireNotNullRule) Confidence() float64 { return 0.75 }

// UseCheckOrErrorRule detects `if (!x) throw IllegalStateException`.
type UseCheckOrErrorRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule. Detection pattern-matches the anti-idiom (if/throw
// blocks, null checks, explicit loops) but whether the suggested
// replacement is actually clearer is context-dependent. Classified per
// roadmap/17.
func (r *UseCheckOrErrorRule) Confidence() float64 { return 0.75 }

// UseRequireRule detects `if (!x) throw IllegalArgumentException`.
type UseRequireRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule. Detection pattern-matches the anti-idiom (if/throw
// blocks, null checks, explicit loops) but whether the suggested
// replacement is actually clearer is context-dependent. Classified per
// roadmap/17.
func (r *UseRequireRule) Confidence() float64 { return 0.75 }

// UseIsNullOrEmptyRule detects `x == null || x.isEmpty()` and related patterns
// such as `x == null || x.count() == 0`, `x == null || x.size == 0`,
// `x == null || x.length == 0`, and `x == null || x == ""`.
// Uses tree-sitter DispatchRule on disjunction_expression for structural accuracy.
type UseIsNullOrEmptyRule struct {
	FlatDispatchBase
	BaseRule
	resolver typeinfer.TypeResolver
}

func (r *UseIsNullOrEmptyRule) SetResolver(res typeinfer.TypeResolver) { r.resolver = res }

// Confidence reports a tier-2 (medium) base confidence — suggests
// isNullOrEmpty() for `x == null || x.isEmpty()`; needs resolver to confirm
// x is String/Collection, falls back to name heuristic. Classified per
// roadmap/17.
func (r *UseIsNullOrEmptyRule) Confidence() float64 { return 0.75 }

// isNullOrEmptySizeProps maps property names that indicate emptiness when == 0.
var isNullOrEmptySizeProps = map[string]bool{
	"size":   true,
	"length": true,
}

// isNullOrEmptyCountFuncs maps function names that indicate emptiness when == 0.
var isNullOrEmptyCountFuncs = map[string]bool{
	"count": true,
}

// isNullOrEmptyEmptyFuncs maps function names that indicate emptiness directly.
var isNullOrEmptyEmptyFuncs = map[string]bool{
	"isEmpty": true,
}

// UseOrEmptyRule detects `x ?: emptyList()` and similar patterns that can use .orEmpty().
// Handles: emptyList/Set/Map/Array/Sequence(), listOf/setOf/mapOf/sequenceOf/arrayOf() with
// no arguments, and empty string literals ("" / """""").
// Uses tree-sitter DispatchRule on elvis_expression for structural accuracy.
type UseOrEmptyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule. Detection pattern-matches the anti-idiom (if/throw
// blocks, null checks, explicit loops) but whether the suggested
// replacement is actually clearer is context-dependent. Classified per
// roadmap/17.
func (r *UseOrEmptyRule) Confidence() float64 { return 0.75 }

// useOrEmptyFunctions maps callee names that represent empty collections/sequences.
var useOrEmptyFunctions = map[string]bool{
	"emptyList":     true,
	"emptySet":      true,
	"emptyMap":      true,
	"emptyArray":    true,
	"emptySequence": true,
}

// useOrEmptyFactoryFunctions maps zero-arg factory calls that produce empty collections.
var useOrEmptyFactoryFunctions = map[string]bool{
	"listOf":     true,
	"setOf":      true,
	"mapOf":      true,
	"arrayOf":    true,
	"sequenceOf": true,
}

func lhsNeedsParensFlat(file *scanner.File, idx uint32) bool {
	switch file.FlatType(idx) {
	case "simple_identifier", "navigation_expression", "call_expression",
		"indexing_expression", "parenthesized_expression":
		return false
	default:
		return true
	}
}

// UseAnyOrNoneInsteadOfFindRule detects `.find {} != null` and `.find {} == null`
// (and also firstOrNull / lastOrNull variants).
// Uses AST dispatch on equality_expression for precise detection.
type UseAnyOrNoneInsteadOfFindRule struct {
	FlatDispatchBase
	BaseRule
}

// anyOrNoneFindFuncs lists the function names that can be replaced.
var anyOrNoneFindFuncs = map[string]bool{
	"find":        true,
	"firstOrNull": true,
	"lastOrNull":  true,
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule. Detection pattern-matches the anti-idiom (if/throw
// blocks, null checks, explicit loops) but whether the suggested
// replacement is actually clearer is context-dependent. Classified per
// roadmap/17.
func (r *UseAnyOrNoneInsteadOfFindRule) Confidence() float64 { return 0.75 }

// UseEmptyCounterpartRule detects `listOf()` etc. with no arguments.
// Uses AST dispatch on call_expression for precise detection, matching
// listOf(), setOf(), mapOf(), arrayOf(), sequenceOf(), and listOfNotNull()
// with zero arguments, and suggesting emptyList(), emptySet(), etc.
type UseEmptyCounterpartRule struct {
	FlatDispatchBase
	BaseRule
}

var emptyCounterparts = map[string]string{
	"listOf":        "emptyList",
	"listOfNotNull": "emptyList",
	"setOf":         "emptySet",
	"mapOf":         "emptyMap",
	"arrayOf":       "emptyArray",
	"sequenceOf":    "emptySequence",
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule. Detection pattern-matches the anti-idiom (if/throw
// blocks, null checks, explicit loops) but whether the suggested
// replacement is actually clearer is context-dependent. Classified per
// roadmap/17.
func (r *UseEmptyCounterpartRule) Confidence() float64 { return 0.75 }


