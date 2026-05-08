package astflat

import "github.com/kaeawc/krit/internal/scanner"

// ExtractIdentifier returns the identifier of a declaration node — function,
// class, parameter, property, or variable_declaration — by finding its first
// named simple_identifier / type_identifier child. Returns "" when no such
// child exists.
func ExtractIdentifier(file *scanner.File, idx uint32) string {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		switch file.FlatType(child) {
		case "simple_identifier", "type_identifier", "identifier":
			return file.FlatNodeString(child, nil)
		case "variable_declaration":
			for gc := file.FlatFirstChild(child); gc != 0; gc = file.FlatNextSib(gc) {
				if file.FlatIsNamed(gc) && file.FlatType(gc) == "simple_identifier" {
					return file.FlatNodeString(gc, nil)
				}
			}
		}
	}
	return ""
}

// NodeWithin reports whether `node` is a descendant of `container` (or the
// same node).
func NodeWithin(file *scanner.File, container, node uint32) bool {
	if file == nil || container == 0 || node == 0 {
		return false
	}
	if container == node {
		return true
	}
	for current, ok := file.FlatParent(node); ok; current, ok = file.FlatParent(current) {
		if current == container {
			return true
		}
	}
	return false
}

// EnclosingAncestor returns the closest ancestor of idx whose type matches
// any of the given types, or (0, false) if none exists.
func EnclosingAncestor(file *scanner.File, idx uint32, types ...string) (uint32, bool) {
	if file == nil || len(types) == 0 {
		return 0, false
	}
	wants := make(map[string]struct{}, len(types))
	for _, t := range types {
		wants[t] = struct{}{}
	}
	for current, ok := file.FlatParent(idx); ok; current, ok = file.FlatParent(current) {
		if _, ok := wants[file.FlatType(current)]; ok {
			return current, true
		}
	}
	return 0, false
}

// IfConditionThenElseBodies decomposes an if_expression into its condition,
// then-body, and (optional) else-body nodes. Tolerates the various brace /
// keyword shapes the tree-sitter Kotlin grammar emits.
func IfConditionThenElseBodies(file *scanner.File, node uint32) (condition, thenBody, elseBody uint32) {
	if file == nil || node == 0 || file.FlatType(node) != "if_expression" {
		return 0, 0, 0
	}
	sawElse := false
	for i := 0; i < file.FlatChildCount(node); i++ {
		child := file.FlatChild(node, i)
		if child == 0 {
			continue
		}
		switch file.FlatType(child) {
		case "if", "(", ")", "{", "}":
			continue
		case "else":
			sawElse = true
			continue
		default:
			if condition == 0 {
				condition = child
				continue
			}
			if !sawElse && thenBody == 0 {
				thenBody = child
				continue
			}
			if sawElse && elseBody == 0 {
				elseBody = child
				return condition, thenBody, elseBody
			}
		}
	}
	return condition, thenBody, elseBody
}

// FinalSimpleIdentifier returns the identifier text at the end of a
// navigation chain or directly_assignable_expression — the rightmost
// simple_identifier reachable by walking named children. For
// `w.settings.javaScriptEnabled` this returns `javaScriptEnabled`.
func FinalSimpleIdentifier(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	last := ""
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "simple_identifier":
			last = file.FlatNodeText(child)
		case "navigation_suffix":
			if inner, ok := file.FlatFindChild(child, "simple_identifier"); ok {
				last = file.FlatNodeText(inner)
			}
		case "navigation_expression", "directly_assignable_expression":
			if nested := FinalSimpleIdentifier(file, child); nested != "" {
				last = nested
			}
		}
	}
	return last
}

// LastNamedChild returns the last named child of idx, or 0.
func LastNamedChild(file *scanner.File, idx uint32) uint32 {
	if file == nil || idx == 0 || file.FlatNamedChildCount(idx) == 0 {
		return 0
	}
	return file.FlatNamedChild(idx, file.FlatNamedChildCount(idx)-1)
}

// UnwrapParenExpr peels parenthesized_expression layers off idx, returning
// the innermost wrapped expression (or idx itself when not parenthesized).
func UnwrapParenExpr(file *scanner.File, idx uint32) uint32 {
	for idx != 0 && file.FlatType(idx) == "parenthesized_expression" && file.FlatNamedChildCount(idx) > 0 {
		idx = file.FlatNamedChild(idx, 0)
	}
	return idx
}

// PrefixExpressionIsNot reports whether a prefix_expression has the `!`
// operator (logical not).
func PrefixExpressionIsNot(file *scanner.File, idx uint32) bool {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) && file.FlatNodeTextEquals(child, "!") {
			return true
		}
	}
	return false
}

// AnyConditionOperand returns true if any named child of idx satisfies the
// predicate.
func AnyConditionOperand(file *scanner.File, idx uint32, predicate func(uint32) bool) bool {
	for i := 0; i < file.FlatNamedChildCount(idx); i++ {
		child := file.FlatNamedChild(idx, i)
		if child != 0 && predicate(child) {
			return true
		}
	}
	return false
}

// AllConditionOperands returns true if at least one named child exists and
// every named child satisfies the predicate.
func AllConditionOperands(file *scanner.File, idx uint32, predicate func(uint32) bool) bool {
	seen := false
	for i := 0; i < file.FlatNamedChildCount(idx); i++ {
		child := file.FlatNamedChild(idx, i)
		if child == 0 {
			continue
		}
		seen = true
		if !predicate(child) {
			return false
		}
	}
	return seen
}

// IsNullLiteral reports whether the (parenthesis-unwrapped) idx is the
// `null` literal.
func IsNullLiteral(file *scanner.File, idx uint32) bool {
	idx = UnwrapParenExpr(file, idx)
	return idx != 0 && (file.FlatType(idx) == "null" || file.FlatNodeTextEquals(idx, "null"))
}
