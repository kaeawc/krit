package rules

import (
	"fmt"
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
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

var causeArgumentRe = regexp.MustCompile(`^(cause\s*=|[\w.]*(Exception|Error|Throwable)\s*\(|[\w.]+\.(message|localizedMessage|cause)$)`)

// flatThrowIfFix returns the autofix that turns `if (cond) throw X("m")` into `check(!cond) { "m" }` (or require),
// or nil when the if is not that exact shape.
func flatThrowIfFix(file *scanner.File, idx uint32, exceptionType, replacement string) *scanner.Fix {
	if file == nil || file.FlatType(idx) != "if_expression" || file.FlatHasChildOfType(idx, "else") {
		return nil
	}
	// An else-if cannot be replaced by a standalone check()/require() call.
	if p, ok := file.FlatParent(idx); ok && file.FlatType(p) == "control_structure_body" {
		if gp, ok := file.FlatParent(p); ok && file.FlatType(gp) == "if_expression" {
			return nil
		}
	}
	condNode, bodyNode := flatThrowExtractCondAndBody(file, idx)
	if condNode == 0 || bodyNode == 0 {
		return nil
	}
	throwNode := flatThrowLocateThrowNode(file, bodyNode)
	if throwNode == 0 {
		return nil
	}
	callNode := flatThrowFindCallNode(file, throwNode)
	if callNode == 0 || flatCallExpressionName(file, callNode) != exceptionType {
		return nil
	}
	_, argsNode := flatCallExpressionParts(file, callNode)
	argCount := 0
	if argsNode != 0 {
		argCount = file.FlatNamedChildCount(argsNode)
	}
	if argCount > 1 {
		return nil
	}
	// The guard passes the negation of the throw condition to check()/require().
	var innerCond string
	if innerCondNode := flatThrowUnwrapNegatedCond(file, condNode); innerCondNode != 0 {
		innerCond = strings.TrimSpace(file.FlatNodeText(innerCondNode))
	} else {
		innerCond = flatThrowNegatedCondText(file, condNode)
	}
	if argCount == 0 {
		return &scanner.Fix{ByteMode: true, StartByte: int(file.FlatStartByte(idx)), EndByte: int(file.FlatEndByte(idx)), Replacement: fmt.Sprintf("%s(%s)", replacement, innerCond)}
	}
	firstArg := file.FlatNamedChild(argsNode, 0)
	if firstArg == 0 {
		return nil
	}
	argText := strings.TrimSpace(file.FlatNodeText(firstArg))
	if !strings.HasPrefix(argText, "\"") || !strings.HasSuffix(argText, "\"") {
		return nil
	}
	return &scanner.Fix{ByteMode: true, StartByte: int(file.FlatStartByte(idx)), EndByte: int(file.FlatEndByte(idx)), Replacement: fmt.Sprintf("%s(%s) { %s }", replacement, innerCond, argText)}
}

// firstCallExpression returns the first call_expression at or below node in source order, or 0.
func firstCallExpression(file *scanner.File, node uint32) uint32 {
	found := uint32(0)
	file.FlatWalkNodes(node, "call_expression", func(n uint32) {
		if found == 0 {
			found = n
		}
	})
	return found
}

// throwCalleeName returns the simple name of the exception constructor a throw calls, qualified or not.
func throwCalleeName(file *scanner.File, call uint32) string {
	callee := file.FlatFirstChild(call)
	if callee == 0 {
		return ""
	}
	if file.FlatType(callee) == "navigation_expression" {
		if suffix, ok := file.FlatFindChild(callee, "navigation_suffix"); ok {
			return strings.TrimLeft(strings.TrimSpace(file.FlatNodeText(suffix)), ".?")
		}
	}
	return strings.TrimSpace(file.FlatNodeText(callee))
}

// throwArgumentsAreEmptyOrString mirrors detekt: no arguments, or one that is a string.
func throwArgumentsAreEmptyOrString(ctx *api.Context, call uint32) bool {
	file := ctx.File
	_, argsNode := flatCallExpressionParts(file, call)
	if argsNode == 0 {
		return true
	}
	var args []uint32
	for c := file.FlatFirstChild(argsNode); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "value_argument" {
			args = append(args, c)
		}
	}
	switch len(args) {
	case 0:
		return true
	case 1:
	default:
		return false
	}
	expr := flatValueArgumentExpression(file, args[0])
	if expr == 0 {
		return false
	}
	if file.FlatType(expr) == "string_literal" {
		return true
	}
	if ctx.Resolver != nil {
		if t := ctx.Resolver.ResolveFlatNode(expr, file); t != nil && t.Kind != typeinfer.TypeUnknown {
			// detekt's KotlinBuiltIns.isString is false for String?.
			return !t.Nullable && (t.Name == "String" || t.FQN == "kotlin.String")
		}
	}
	// Without a type, anything that does not look like an exception is taken for a message.
	if file.FlatType(expr) == "simple_identifier" && throwIsCaughtParameter(file, expr) {
		return false
	}
	return !causeArgumentRe.MatchString(strings.TrimSpace(file.FlatNodeText(args[0])))
}

// throwIsCaughtParameter reports whether the identifier is the parameter of the nearest enclosing catch.
func throwIsCaughtParameter(file *scanner.File, ident uint32) bool {
	catchNode, ok := flatEnclosingAncestor(file, ident, "catch_block", "catch_clause")
	return ok && extractCaughtVarNameFlat(file, catchNode) == file.FlatNodeText(ident)
}

// throwIsDirectControlBody reports whether a throw is the body of an if, else, loop or the only statement of its braces;
// when braced is set, a block with other statements does not count.
func throwIsDirectControlBody(file *scanner.File, throw uint32) bool {
	p, ok := file.FlatParent(throw)
	if !ok {
		return false
	}
	body := p
	if file.FlatType(p) == "statements" {
		namedCount := 0
		for c := file.FlatFirstChild(p); c != 0; c = file.FlatNextSib(c) {
			if file.FlatIsNamed(c) && !isFlatCommentNode(file, c) {
				namedCount++
			}
		}
		if namedCount > 1 {
			return false
		}
		if body, ok = file.FlatParent(p); !ok {
			return false
		}
	}
	if file.FlatType(body) != "control_structure_body" {
		return false
	}
	owner, ok := file.FlatParent(body)
	if !ok {
		return false
	}
	switch file.FlatType(owner) {
	case "if_expression", "for_statement", "while_statement", "do_while_statement":
		return true
	}
	return false
}

// throwIsOnlyStatementInLambda reports `{ throw X }`, which check()/require() cannot replace.
func throwIsOnlyStatementInLambda(file *scanner.File, throw uint32) bool {
	p, ok := file.FlatParent(throw)
	if !ok || file.FlatType(p) != "statements" {
		return false
	}
	gp, ok := file.FlatParent(p)
	if !ok || file.FlatType(gp) != "lambda_literal" {
		return false
	}
	count := 0
	for c := file.FlatFirstChild(p); c != 0; c = file.FlatNextSib(c) {
		if file.FlatIsNamed(c) && !isFlatCommentNode(file, c) {
			count++
		}
	}
	return count == 1
}

// flatThrowPattern reports a throw of exceptionType that check()/require() can replace, like detekt's
// UseCheckOrError (any such throw) and UseRequire (only a throw that is the body of a conditional).
func flatThrowPattern(ctx *api.Context, base BaseRule, exceptionType, replacement, message string, conditionalOnly bool) {
	idx, file := ctx.Idx, ctx.File
	if file == nil || !flatThrowIsThrowJump(file, idx) {
		return
	}
	call := firstCallExpression(file, idx)
	if call == 0 || throwCalleeName(file, call) != exceptionType {
		return
	}
	if conditionalOnly {
		if !throwIsDirectControlBody(file, idx) {
			return
		}
	} else if throwIsOnlyStatementInLambda(file, idx) {
		return
	}
	if !throwArgumentsAreEmptyOrString(ctx, call) {
		return
	}
	f := base.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, message)
	// The fix rewrites the whole `if`, so it is attached only when the throw is that if's whole body.
	if p, ok := file.FlatParent(idx); ok {
		if file.FlatType(p) == "statements" {
			p, _ = file.FlatParent(p)
		}
		if owner, ok := file.FlatParent(p); ok && file.FlatType(p) == "control_structure_body" && file.FlatType(owner) == "if_expression" {
			f.Fix = flatThrowIfFix(file, owner, exceptionType, replacement)
		}
	}
	ctx.Emit(f)
}

// flatThrowNegatedCondText returns the source of the negation of cond, flipping
// ==/!= and is/!is and wrapping everything else in !(...).
func flatThrowNegatedCondText(file *scanner.File, cond uint32) string {
	for file.FlatType(cond) == "parenthesized_expression" && file.FlatNamedChildCount(cond) == 1 {
		cond = file.FlatNamedChild(cond, 0)
	}
	text := strings.TrimSpace(file.FlatNodeText(cond))
	switch file.FlatType(cond) {
	case "equality_expression":
		if file.FlatChildCount(cond) == 3 {
			lhs := strings.TrimSpace(file.FlatNodeText(file.FlatChild(cond, 0)))
			rhs := strings.TrimSpace(file.FlatNodeText(file.FlatChild(cond, 2)))
			switch strings.TrimSpace(file.FlatNodeText(file.FlatChild(cond, 1))) {
			case "==":
				return lhs + " != " + rhs
			case "!=":
				return lhs + " == " + rhs
			}
		}
	case "simple_identifier", "navigation_expression", "call_expression", "boolean_literal":
		return "!" + text
	}
	return "!(" + text + ")"
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
		// `{ throw X }` is a statements node between the braces; comments would be lost by the rewrite.
		if file.FlatType(actualBody) == "statements" && file.FlatNamedChildCount(bodyNode) == 1 && file.FlatNamedChildCount(actualBody) == 1 {
			if child := file.FlatNamedChild(actualBody, 0); flatThrowIsThrowJump(file, child) {
				return child
			}
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

// declaredTypeNameOfIdentifier finds the explicitly declared type of a parameter or local/member
// property named name in the callable or class enclosing node, returning its base type name
// without nullability or type arguments.
func declaredTypeNameOfIdentifier(file *scanner.File, node uint32, name string) (string, bool) {
	// Search the enclosing function first, then the class or file, so a same-named parameter elsewhere is ignored.
	for p, ok := file.FlatParent(node); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "function_declaration", "secondary_constructor", "anonymous_function", "class_declaration", "object_declaration", "source_file":
			if text, found := declaredTypeNameInScope(file, p, name); found {
				return text, true
			}
		}
	}
	return "", false
}

func declaredTypeNameInScope(file *scanner.File, scope uint32, name string) (string, bool) {
	result, found := "", false
	for _, nodeType := range []string{"parameter", "class_parameter", "property_declaration"} {
		file.FlatWalkNodes(scope, nodeType, func(decl uint32) {
			if found || extractIdentifierFlat(file, decl) != name {
				return
			}
			holder := decl
			if nodeType == "property_declaration" {
				if v, ok := file.FlatFindChild(decl, "variable_declaration"); ok {
					holder = v
				}
			}
			for c := file.FlatFirstChild(holder); c != 0; c = file.FlatNextSib(c) {
				t := c
				if file.FlatType(c) == "nullable_type" {
					t, _ = file.FlatFindChild(c, "user_type")
				}
				if t != 0 && file.FlatType(t) == "user_type" {
					text := strings.TrimSpace(file.FlatNodeText(t))
					if lt := strings.IndexByte(text, '<'); lt >= 0 {
						text = text[:lt]
					}
					if dot := strings.LastIndexByte(text, '.'); dot >= 0 {
						text = text[dot+1:]
					}
					result, found = text, true
					return
				}
			}
		})
		if found {
			break
		}
	}
	return result, found
}

// orEmptyReceiverMatches reports whether a receiver of the declared type has an orEmpty() whose
// result type equals the empty value written on the right of the elvis.
func orEmptyReceiverMatches(typeName, rhs string) bool {
	switch {
	case strings.HasPrefix(rhs, `""`):
		return typeName == "String"
	case strings.HasPrefix(rhs, "emptyList(") || strings.HasPrefix(rhs, "listOf("):
		return typeName == "List" || typeName == "Collection"
	case strings.HasPrefix(rhs, "emptySet(") || strings.HasPrefix(rhs, "setOf("):
		return typeName == "Set"
	case strings.HasPrefix(rhs, "emptyMap(") || strings.HasPrefix(rhs, "mapOf("):
		return typeName == "Map"
	case strings.HasPrefix(rhs, "emptySequence(") || strings.HasPrefix(rhs, "sequenceOf("):
		return typeName == "Sequence"
	}
	return true
}
