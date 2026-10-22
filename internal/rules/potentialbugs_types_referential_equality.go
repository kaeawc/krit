package rules

import (
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
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
	return looksLikeUppercaseConstantName(last)
}

func looksLikeUppercaseConstantName(text string) bool {
	if text == "" {
		return false
	}
	// Must be all uppercase letters, digits, or underscores
	for _, c := range text {
		if (c >= 'A' && c <= 'Z') || c == '_' || (c >= '0' && c <= '9') {
			continue
		}
		return false
	}
	// Must contain at least one letter
	for _, c := range text {
		if c >= 'A' && c <= 'Z' {
			return true
		}
	}
	return false
}

func looksLikeSentinelObjectRef(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	return file.FlatType(idx) == "simple_identifier" && looksLikeUppercaseConstantName(file.FlatNodeText(idx))
}

func looksLikeSentinelAliasRef(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "simple_identifier" {
		return false
	}
	name := file.FlatNodeText(idx)
	if name == "" || looksLikeUppercaseConstantName(name) {
		return false
	}
	scope := referentialEqualityAliasSearchScope(file, idx)
	if scope == 0 {
		return false
	}
	found := false
	useStart := file.FlatStartByte(idx)
	file.FlatWalkNodes(scope, "property_declaration", func(prop uint32) {
		if found || file.FlatStartByte(prop) >= useStart || propertyDeclarationIsVar(file, prop) {
			return
		}
		if propertyDeclarationName(file, prop) != name {
			return
		}
		init := propertyInitializerExpression(file, prop)
		if init == 0 {
			return
		}
		found = looksLikeSentinelInitializer(file.FlatNodeText(init))
	})
	return found
}

func referentialEqualityAliasSearchScope(file *scanner.File, idx uint32) uint32 {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "function_declaration", "lambda_literal", "source_file":
			return p
		case "class_declaration", "object_declaration":
			return 0
		}
	}
	return 0
}

func looksLikeSentinelInitializer(text string) bool {
	text = strings.TrimSpace(text)
	for strings.HasPrefix(text, "(") && strings.HasSuffix(text, ")") && len(text) > 1 {
		text = strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(text, ")"), "("))
	}
	if looksLikeUppercaseConstantName(text) || looksLikeEnumConstantRef(text) {
		return true
	}
	if dot := strings.LastIndex(text, "."); dot >= 0 {
		return looksLikeUppercaseConstantName(text[dot+1:])
	}
	return false
}

func looksLikeThisFieldIdentityCheck(file *scanner.File, left uint32, right uint32) bool {
	return looksLikeThisFieldIdentityOperand(file.FlatNodeText(left)) ||
		looksLikeThisFieldIdentityOperand(file.FlatNodeText(right))
}

func looksLikeThisFieldIdentityOperand(text string) bool {
	text = strings.TrimSpace(text)
	for strings.HasPrefix(text, "(") && strings.HasSuffix(text, ")") && len(text) > 1 {
		text = strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(text, ")"), "("))
	}
	return strings.HasPrefix(text, "this.")
}

func looksLikeIterationIdentityCheck(file *scanner.File, idx uint32, left uint32, right uint32) bool {
	lambda := uint32(0)
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "lambda_literal":
			lambda = p
		case "function_declaration", "class_declaration", "object_declaration", "source_file":
			return false
		}
		if lambda != 0 {
			break
		}
	}
	if lambda == 0 || !referentialEqualityLambdaBelongsToIterationCall(file, lambda) {
		return false
	}
	params := referentialEqualityLambdaParameterNames(file, lambda)
	if len(params) == 0 {
		params["it"] = true
	}
	return referentialEqualityOperandIsLambdaParameter(file, left, params) ||
		referentialEqualityOperandIsLambdaParameter(file, right, params)
}

func looksLikeArrayElementIdentityCheck(file *scanner.File, left uint32, right uint32) bool {
	return file.FlatType(left) == "indexing_expression" ||
		file.FlatType(right) == "indexing_expression" ||
		looksLikeIndexedOperandText(file.FlatNodeText(left)) ||
		looksLikeIndexedOperandText(file.FlatNodeText(right))
}

func looksLikeIndexedOperandText(text string) bool {
	text = strings.TrimSpace(text)
	return strings.Contains(text, "[") && strings.Contains(text, "]")
}

func looksLikeSingletonTypeIdentityCheck(file *scanner.File, left uint32, right uint32) bool {
	return (file.FlatType(left) == "this_expression" && looksLikeSingletonObjectRefText(file.FlatNodeText(right))) ||
		(file.FlatType(right) == "this_expression" && looksLikeSingletonObjectRefText(file.FlatNodeText(left))) ||
		looksLikeSingletonObjectRefText(file.FlatNodeText(left)) ||
		looksLikeSingletonObjectRefText(file.FlatNodeText(right))
}

func looksLikeSingletonObjectRefText(text string) bool {
	text = strings.TrimSpace(text)
	for strings.HasPrefix(text, "(") && strings.HasSuffix(text, ")") && len(text) > 1 {
		text = strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(text, ")"), "("))
	}
	if text == "" || strings.ContainsAny(text, ".[]() ") {
		if strings.ContainsAny(text, "[]() ") {
			return false
		}
		parts := strings.Split(text, ".")
		if len(parts) < 2 {
			return false
		}
		for _, part := range parts {
			if !looksLikeUpperCamelIdentifier(part) {
				return false
			}
		}
		return true
	}
	return looksLikeUpperCamelIdentifier(text)
}

func looksLikeUpperCamelIdentifier(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" || strings.ContainsAny(text, ".[]() ") {
		return false
	}
	first := text[0]
	return first >= 'A' && first <= 'Z'
}

func looksLikeResourceCleanupIdentityGuard(file *scanner.File, idx uint32, opType string) bool {
	if opType != "!==" {
		return false
	}
	var ifExpr uint32
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "if_expression":
			ifExpr = p
		case "function_declaration", "class_declaration", "object_declaration", "source_file":
			if ifExpr == 0 || !referentialEqualityIsIfCondition(file, idx, ifExpr) {
				return false
			}
			text := file.FlatNodeText(ifExpr)
			return strings.Contains(text, ".recycle()") || strings.Contains(text, ".close()")
		}
		if ifExpr != 0 {
			break
		}
	}
	if ifExpr == 0 || !referentialEqualityIsIfCondition(file, idx, ifExpr) {
		return false
	}
	text := file.FlatNodeText(ifExpr)
	return strings.Contains(text, ".recycle()") || strings.Contains(text, ".close()")
}

func referentialEqualityLambdaBelongsToIterationCall(file *scanner.File, lambda uint32) bool {
	for p, ok := file.FlatParent(lambda); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "call_expression":
			return referentialEqualityIterationCalls[flatCallNameAny(file, p)]
		case "function_declaration", "class_declaration", "object_declaration", "source_file":
			return false
		}
	}
	return false
}

var referentialEqualityIterationCalls = map[string]bool{
	"forEach": true, "forEachIndexed": true,
	"map": true, "mapNotNull": true, "flatMap": true, "flatMapTo": true,
	"filter": true, "filterNot": true, "filterIsInstance": true,
	"collect": true, "collectLatest": true,
	"any": true, "all": true, "none": true, "find": true, "firstOrNull": true,
}

func referentialEqualityLambdaParameterNames(file *scanner.File, lambda uint32) map[string]bool {
	params := make(map[string]bool)
	text := file.FlatNodeText(lambda)
	arrow := strings.Index(text, "->")
	if arrow < 0 {
		return params
	}
	prefix := strings.TrimSpace(strings.TrimPrefix(text[:arrow], "{"))
	prefix = strings.TrimPrefix(prefix, "(")
	prefix = strings.TrimSuffix(prefix, ")")
	for _, part := range strings.Split(prefix, ",") {
		name := strings.TrimSpace(part)
		if colon := strings.Index(name, ":"); colon >= 0 {
			name = strings.TrimSpace(name[:colon])
		}
		if name != "" && name != "_" {
			params[name] = true
		}
	}
	return params
}

func referentialEqualityOperandIsLambdaParameter(file *scanner.File, operand uint32, params map[string]bool) bool {
	if file.FlatType(operand) != "simple_identifier" {
		return false
	}
	return params[file.FlatNodeText(operand)]
}

func isInsideEqualsMethodFlatType(file *scanner.File, idx uint32) bool {
	fn, ok := flatEnclosingFunction(file, idx)
	if !ok {
		return false
	}
	return extractIdentifierFlat(file, fn) == "equals"
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

// referentialEqualityCompileFQNPatterns turns the user-configured glob
// patterns into anchored regexes: `*` -> `.*`, `?` -> `.`, other regex
// specials escaped, and the whole pattern anchored. Invalid patterns are
// skipped silently rather than failing the rule entirely.
func referentialEqualityCompileFQNPatterns(patterns []string) []*regexp.Regexp {
	if len(patterns) == 0 {
		return nil
	}
	out := make([]*regexp.Regexp, 0, len(patterns))
	for _, raw := range patterns {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		var b strings.Builder
		b.WriteByte('^')
		for _, c := range raw {
			switch c {
			case '*':
				b.WriteString(".*")
			case '?':
				b.WriteByte('.')
			case '\\', '.', '+', '(', ')', '[', ']', '{', '}', '|', '^', '$':
				b.WriteByte('\\')
				b.WriteRune(c)
			default:
				b.WriteRune(c)
			}
		}
		b.WriteByte('$')
		re, err := regexp.Compile(b.String())
		if err == nil && re != nil {
			out = append(out, re)
		}
	}
	return out
}

// referentialEqualityResolvedFQN extracts the fully-qualified type name
// from a resolver result, preferring the FQN field but falling back to
// the simple name when no FQN was attached. Returns "" for unknown or
// nil results.
func referentialEqualityResolvedFQN(t *typeinfer.ResolvedType) string {
	if t == nil || t.Kind == typeinfer.TypeUnknown {
		return ""
	}
	if t.FQN != "" {
		return t.FQN
	}
	return t.Name
}

// referentialEqualityFQNMatchesAny reports whether the given fully-
// qualified type name matches any of the compiled patterns.
func referentialEqualityFQNMatchesAny(fqn string, patterns []*regexp.Regexp) bool {
	if fqn == "" || len(patterns) == 0 {
		return false
	}
	for _, p := range patterns {
		if p.MatchString(fqn) {
			return true
		}
	}
	return false
}

// equalsFamilyCallNames is the set of callees whose presence in a
// boolean expression means an accompanying referential compare is
// almost certainly the short-circuit fast-path, not a mistake.
var equalsFamilyCallNames = map[string]bool{
	"equals":            true,
	"hasSameContent":    true,
	"contentEquals":     true,
	"contentDeepEquals": true,
	"sameContentAs":     true,
}

// equalityOperands returns (left, operator, right) of an equality_expression.
// The operator is the token child whose type is one of `==`, `!=`, `===`,
// `!==`. Returns zeros if the expression doesn't have all three parts.
func equalityOperands(file *scanner.File, idx uint32) (left, op, right uint32) {
	if file == nil || file.FlatType(idx) != "equality_expression" {
		return 0, 0, 0
	}
	op = equalityOperatorChild(file, idx)
	if op == 0 {
		return 0, 0, 0
	}
	opStart := file.FlatStartByte(op)
	opEnd := file.FlatEndByte(op)
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		if file.FlatEndByte(child) <= opStart {
			left = child
		} else if right == 0 && file.FlatStartByte(child) >= opEnd {
			right = child
		}
	}
	return left, op, right
}

func equalityOperatorChild(file *scanner.File, idx uint32) uint32 {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "==", "!=", "===", "!==":
			return child
		}
	}
	return 0
}

// enclosingBoolExprHasEqualsCall returns true when the referential-equality
// node sits under a conjunction/disjunction whose other operand is a call
// to one of the named equals-family functions. Structural replacement for
// the old `strings.Contains(parentText, ".equals(")` heuristic — by walking
// the AST we avoid matching `.equals(` that appears in a string literal or
// a comment.
func enclosingBoolExprHasEqualsCall(file *scanner.File, idx uint32, names map[string]bool) bool {
	parent, ok := file.FlatParent(idx)
	if !ok {
		return false
	}
	pt := file.FlatType(parent)
	if pt != "disjunction_expression" && pt != "conjunction_expression" {
		return false
	}
	found := false
	file.FlatWalkAllNodes(parent, func(n uint32) {
		if found || n == idx {
			return
		}
		if file.FlatType(n) == "call_expression" {
			if _, ok := names[flatCallExpressionName(file, n)]; ok {
				found = true
			}
		}
	})
	return found
}

func isComparatorIdentityFastPath(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	var ifExpr uint32
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "if_expression":
			ifExpr = p
		case "function_declaration":
			name := flatFunctionName(file, p)
			if name != "compareTo" && name != "compare" {
				return false
			}
			if ifExpr == 0 || !referentialEqualityIsIfCondition(file, idx, ifExpr) {
				return false
			}
			if !ifExpressionReturnsZero(file, ifExpr) {
				return false
			}
			return ifExpressionIsFirstStatementInFunction(file, ifExpr, p)
		case "source_file", "class_declaration", "object_declaration":
			return false
		}
	}
	return false
}

func referentialEqualityIsIfCondition(file *scanner.File, idx uint32, ifExpr uint32) bool {
	for child := file.FlatFirstChild(ifExpr); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "control_structure_body" {
			return file.FlatEndByte(idx) <= file.FlatStartByte(child)
		}
	}
	return false
}

func ifExpressionReturnsZero(file *scanner.File, ifExpr uint32) bool {
	text := file.FlatNodeText(ifExpr)
	return strings.Contains(text, "return 0") || strings.Contains(text, "-> 0")
}

func ifExpressionIsFirstStatementInFunction(file *scanner.File, ifExpr uint32, fn uint32) bool {
	for p, ok := file.FlatParent(ifExpr); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "statements":
			for child := file.FlatFirstChild(p); child != 0; child = file.FlatNextSib(child) {
				if !file.FlatIsNamed(child) {
					continue
				}
				return child == ifExpr
			}
			return false
		case "function_declaration":
			return p == fn
		}
	}
	return false
}

// Confidence reports a tier-2 (medium) base confidence — flags === / !==
// on value types; needs resolver to confirm operand types, falls back to a
// name-based heuristic. Classified per roadmap/17.
func (r *AvoidReferentialEqualityRule) Confidence() float64 { return api.ConfidenceMedium }
