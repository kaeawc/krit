package rules

import (
	"fmt"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func flatNonNullCheckText(file *scanner.File, idx uint32, funcName string) (argText string, lambdaText string, ok bool) {
	if file == nil || file.FlatType(idx) != "call_expression" {
		return "", "", false
	}
	if flatCallExpressionName(file, idx) != funcName {
		return "", "", false
	}
	suffix, _ := file.FlatFindChild(idx, "call_suffix")
	if suffix == 0 {
		return "", "", false
	}
	args := flatCallSuffixValueArgs(file, suffix)
	if args == 0 {
		return "", "", false
	}
	var arg uint32
	for child := file.FlatFirstChild(args); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "value_argument" {
			continue
		}
		if arg != 0 {
			return "", "", false
		}
		arg = child
	}
	if arg == 0 {
		return "", "", false
	}
	eq := flatUnwrapParenExpr(file, flatValueArgumentExpression(file, arg))
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

func flatNavigationReceiver(file *scanner.File, nav uint32) uint32 {
	if file == nil || nav == 0 || file.FlatType(nav) != "navigation_expression" || file.FlatNamedChildCount(nav) == 0 {
		return 0
	}
	return file.FlatNamedChild(nav, 0)
}

func flatThrowPattern(ctx *api.Context, nodeType, nodeText string, exceptionType, replacement string, base BaseRule) {
	idx, file := ctx.Idx, ctx.File
	if file == nil || nodeType != "if_expression" {
		return
	}
	if strings.Contains(nodeText, "else") && file.FlatHasChildOfType(idx, "else") {
		return
	}
	condNode, bodyNode := flatThrowExtractCondAndBody(file, idx)
	if condNode == 0 || bodyNode == 0 {
		return
	}
	innerCondNode := flatThrowUnwrapNegatedCond(file, condNode)
	if innerCondNode == 0 {
		return
	}
	throwNode := flatThrowLocateThrowNode(file, bodyNode)
	if throwNode == 0 {
		return
	}
	callNode := flatThrowFindCallNode(file, throwNode)
	if callNode == 0 {
		return
	}
	if flatCallExpressionName(file, callNode) != exceptionType {
		return
	}
	innerCond := strings.TrimSpace(file.FlatNodeText(innerCondNode))
	f := base.Finding(file, file.FlatRow(idx)+1, 1, fmt.Sprintf("Use '%s()' instead of 'if (...) throw %s'.", replacement, exceptionType))
	_, argsNode := flatCallExpressionParts(file, callNode)
	if argsNode != 0 && file.FlatNamedChildCount(argsNode) > 0 {
		firstArg := file.FlatNamedChild(argsNode, 0)
		if firstArg != 0 {
			argText := strings.TrimSpace(file.FlatNodeText(firstArg))
			if strings.HasPrefix(argText, "\"") && strings.HasSuffix(argText, "\"") {
				f.Fix = &scanner.Fix{ByteMode: true, StartByte: int(file.FlatStartByte(idx)), EndByte: int(file.FlatEndByte(idx)), Replacement: fmt.Sprintf("%s(%s) { %s }", replacement, innerCond, argText)}
			}
		}
	}
	ctx.Emit(f)
}

func flatThrowExtractCondAndBody(file *scanner.File, idx uint32) (condNode, bodyNode uint32) {
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
	return condNode, bodyNode
}

func flatThrowUnwrapNegatedCond(file *scanner.File, condNode uint32) uint32 {
	if file.FlatType(condNode) != "prefix_expression" || file.FlatChildCount(condNode) < 2 {
		return 0
	}
	opNode := file.FlatChild(condNode, 0)
	if opNode == 0 || !file.FlatNodeTextEquals(opNode, "!") {
		return 0
	}
	innerCondNode := file.FlatChild(condNode, 1)
	if innerCondNode == 0 {
		return 0
	}
	if file.FlatType(innerCondNode) == "parenthesized_expression" {
		if inner := file.FlatNamedChild(innerCondNode, 0); inner != 0 {
			innerCondNode = inner
		}
	}
	return innerCondNode
}

func flatThrowIsThrowJump(file *scanner.File, node uint32) bool {
	if file.FlatType(node) != "jump_expression" {
		return false
	}
	first := file.FlatFirstChild(node)
	return first != 0 && file.FlatNodeTextEquals(first, "throw")
}

func flatThrowLocateThrowNode(file *scanner.File, bodyNode uint32) uint32 {
	actualBody := bodyNode
	if file.FlatType(bodyNode) == "control_structure_body" {
		if inner := file.FlatNamedChild(bodyNode, 0); inner != 0 {
			actualBody = inner
		}
	}
	switch file.FlatType(actualBody) {
	case "jump_expression":
		if flatThrowIsThrowJump(file, actualBody) {
			return actualBody
		}
	case "block":
		if file.FlatNamedChildCount(actualBody) == 1 {
			child := file.FlatNamedChild(actualBody, 0)
			if flatThrowIsThrowJump(file, child) {
				return child
			}
		}
	}
	return 0
}

func flatThrowFindCallNode(file *scanner.File, throwNode uint32) uint32 {
	for child := file.FlatFirstChild(throwNode); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "call_expression" {
			return child
		}
	}
	return 0
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
// `x != null` and `null != x` argument order as the full condition, plus
// optional message lambdas like `check(x != null) { "msg" }`.
type UseCheckNotNullRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence — suggests
// checkNotNull over `if (x == null) throw`; pattern-based with resolver
// used to confirm nullability when available. Classified per roadmap/17.
func (r *UseCheckNotNullRule) Confidence() float64 { return api.ConfidenceMedium }

// UseRequireNotNullRule detects require(x != null) and suggests requireNotNull(x).
// Uses AST dispatch on call_expression for precise detection, handling both
// `x != null` and `null != x` argument order as the full condition, plus
// optional message lambdas like `require(x != null) { "msg" }`.
type UseRequireNotNullRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence — suggests
// requireNotNull over `if (x == null) throw IAE`; pattern-based with
// resolver confirmation when available. Classified per roadmap/17.
func (r *UseRequireNotNullRule) Confidence() float64 { return api.ConfidenceMedium }

// UseCheckOrErrorRule detects `if (!x) throw IllegalStateException`.
type UseCheckOrErrorRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule. Detection pattern-matches the anti-idiom (if/throw
// blocks, null checks, explicit loops) but whether the suggested
// replacement is actually clearer is context-dependent. Classified per
// roadmap/17.
func (r *UseCheckOrErrorRule) Confidence() float64 { return api.ConfidenceMedium }

// UseRequireRule detects `if (!x) throw IllegalArgumentException`.
type UseRequireRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule. Detection pattern-matches the anti-idiom (if/throw
// blocks, null checks, explicit loops) but whether the suggested
// replacement is actually clearer is context-dependent. Classified per
// roadmap/17.
func (r *UseRequireRule) Confidence() float64 { return api.ConfidenceMedium }

// UseIsNullOrEmptyRule detects `x == null || x.isEmpty()` and related patterns
// such as `x == null || x.count() == 0`, `x == null || x.size == 0`,
// `x == null || x.length == 0`, and `x == null || x == ""`.
// Uses tree-sitter DispatchRule on disjunction_expression for structural accuracy.
type UseIsNullOrEmptyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence — suggests
// isNullOrEmpty() for `x == null || x.isEmpty()` after semantic receiver
// checks, with lower-confidence same-file declaration fallback. Classified per
// roadmap/17.
func (r *UseIsNullOrEmptyRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *UseOrEmptyRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *UseAnyOrNoneInsteadOfFindRule) Confidence() float64 { return api.ConfidenceMedium }

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
func (r *UseEmptyCounterpartRule) Confidence() float64 { return api.ConfidenceMedium }
