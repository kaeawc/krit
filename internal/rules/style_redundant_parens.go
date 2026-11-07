package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// Expression priorities follow Kotlin's operator precedence (higher binds tighter). Everything that is not an
// operator expression, such as names, literals, lambdas, if and when, has the highest priority.
const (
	parenPrioAtom           = 100
	parenPrioPostfix        = 90
	parenPrioPrefix         = 80
	parenPrioAs             = 70
	parenPrioMultiplicative = 60
	parenPrioAdditive       = 50
	parenPrioRange          = 40
	parenPrioInfix          = 30
	parenPrioElvis          = 25
	parenPrioInOrIs         = 20
	parenPrioComparison     = 15
	parenPrioEquality       = 10
	parenPrioConjunction    = 5
	parenPrioDisjunction    = 4
)

func parenExprPriority(file *scanner.File, n uint32) int {
	// The grammar parses `!a || b` as `!(a || b)`; the real top-level operator is the binary one.
	if file.FlatType(n) == "prefix_expression" {
		operand := uint32(0)
		for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
			if file.FlatIsNamed(c) && !isFlatCommentNode(file, c) {
				operand = c
			}
		}
		if operand != 0 {
			if prio := parenExprPriority(file, operand); prio < parenPrioPrefix {
				return prio
			}
		}
	}
	switch file.FlatType(n) {
	case "postfix_expression", "navigation_expression", "call_expression", "indexing_expression":
		return parenPrioPostfix
	case "prefix_expression":
		return parenPrioPrefix
	case "as_expression":
		return parenPrioAs
	case "multiplicative_expression":
		return parenPrioMultiplicative
	case "additive_expression":
		return parenPrioAdditive
	case "range_expression":
		return parenPrioRange
	case "infix_expression":
		return parenPrioInfix
	case "elvis_expression":
		return parenPrioElvis
	case "check_expression":
		return parenPrioInOrIs
	case "comparison_expression":
		return parenPrioComparison
	case "equality_expression":
		return parenPrioEquality
	case "conjunction_expression":
		return parenPrioConjunction
	case "disjunction_expression":
		return parenPrioDisjunction
	}
	return parenPrioAtom
}

// parenOperandInfo describes the parenthesized expression as an operand of a binary-like parent.
func parenIsFirstOperand(file *scanner.File, parent, paren uint32) bool {
	for c := file.FlatFirstChild(parent); c != 0; c = file.FlatNextSib(c) {
		if t := file.FlatType(c); t == "(" || isFlatCommentNode(file, c) {
			if c == paren {
				return true
			}
			continue
		}
		return c == paren
	}
	return false
}

// parenInnerExpression returns the expression a parenthesized_expression wraps, or 0.
func parenInnerExpression(file *scanner.File, paren uint32) uint32 {
	for c := file.FlatFirstChild(paren); c != 0; c = file.FlatNextSib(c) {
		if t := file.FlatType(c); t == "(" || t == ")" || isFlatCommentNode(file, c) {
			continue
		}
		return c
	}
	return 0
}

// parenthesesNecessary mirrors KtPsiUtil.areParenthesesNecessary, which detekt's UnnecessaryParentheses relies on.
func parenthesesNecessary(file *scanner.File, paren, inner uint32) bool {
	parent, ok := file.FlatParent(paren)
	if !ok {
		return false
	}
	ptype := file.FlatType(parent)
	if ptype == "parenthesized_expression" || file.FlatType(inner) == "parenthesized_expression" {
		return false
	}
	innerPrio := parenExprPriority(file, inner)
	itype := file.FlatType(inner)
	// detekt keeps the parentheses of `return (x@ a)`.
	if ptype == "jump_expression" && itype == "prefix_expression" && file.FlatType(file.FlatFirstChild(inner)) == "label" &&
		strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(parent)), "return") {
		return true
	}
	switch ptype {
	case "multiplicative_expression", "additive_expression", "range_expression", "infix_expression",
		"elvis_expression", "check_expression", "comparison_expression", "equality_expression",
		"conjunction_expression", "disjunction_expression", "as_expression":
		return parenBinaryOperandNecessary(file, parent, paren, inner, innerPrio)
	case "prefix_expression":
		if innerPrio == parenPrioPrefix {
			return parenPrefixSignsClash(file, parent, inner)
		}
		return innerPrio < parenPrioPrefix
	case "postfix_expression":
		return innerPrio < parenPrioPostfix || itype == "if_expression"
	case "navigation_expression", "indexing_expression", "callable_reference":
		if first := parenIsFirstOperand(file, parent, paren); first {
			return innerPrio < parenPrioPostfix || itype == "if_expression"
		}
	case "call_expression":
		if parenIsFirstOperand(file, parent, paren) {
			return !parenCalleeCanLoseParens(file, inner)
		}
	}
	return itype == "if_expression" && parenIfNeedsParens(file, parent)
}

// parenBinaryOperandNecessary decides whether a parenthesized operand of a binary-like parent needs its
// parentheses, comparing the operand's priority with the parent's.
func parenBinaryOperandNecessary(file *scanner.File, parent, paren, inner uint32, innerPrio int) bool {
	ptype := file.FlatType(parent)
	itype := file.FlatType(inner)
	parentPrio := parenExprPriority(file, parent)
	first := parenIsFirstOperand(file, parent, paren)
	// `a < b as T` would read T<... as type arguments.
	if itype == "as_expression" && ptype == "comparison_expression" && file.FlatNodeTextEquals(parenOperator(file, parent), "<") {
		return true
	}
	if itype == "if_expression" && first {
		return true
	}
	// detekt gives `is` and `!is` the highest priority when they are a right operand.
	if itype == "check_expression" && !first && ptype == "elvis_expression" && parenCheckIsTypeTest(file, inner) {
		return false
	}
	if innerPrio == parentPrio {
		// && and || are associative; every other operator needs the parentheses on its right side.
		if !first && (ptype == "conjunction_expression" || ptype == "disjunction_expression") {
			return false
		}
		return !first
	}
	return innerPrio < parentPrio
}

// parenOperator returns the operator token of a binary expression, or 0.
func parenOperator(file *scanner.File, binary uint32) uint32 {
	seen := false
	for c := file.FlatFirstChild(binary); c != 0; c = file.FlatNextSib(c) {
		if !file.FlatIsNamed(c) && !isFlatCommentNode(file, c) {
			return c
		}
		if file.FlatIsNamed(c) {
			if seen {
				break
			}
			seen = true
		}
	}
	return 0
}

// parenCheckIsTypeTest reports an `is` or `!is` check, as opposed to `in` and `!in`.
func parenCheckIsTypeTest(file *scanner.File, check uint32) bool {
	for c := file.FlatFirstChild(check); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "is", "!is":
			return true
		}
	}
	return false
}

// parenIfNeedsParens lists the places where a parenthesized if expression must keep its parentheses because the
// surrounding syntax would otherwise absorb what follows it.
func parenIfNeedsParens(file *scanner.File, parent uint32) bool {
	switch file.FlatType(parent) {
	case "if_expression", "when_condition", "indexing_suffix", "interpolated_expression":
		return true
	case "control_structure_body":
		owner, ok := file.FlatParent(parent)
		if !ok {
			return false
		}
		switch file.FlatType(owner) {
		case "when_entry":
			return true
		case "if_expression":
			// A then-branch followed by else would take that else.
			_, elseBody := ifRawBodies(file, owner)
			return elseBody != 0 && elseBody != parent
		}
	}
	return false
}

// parenPrefixSignsClash reports `-(-a)` and `+(+a)`, where dropping the parentheses would form `--` or `++`.
func parenPrefixSignsClash(file *scanner.File, parent, inner uint32) bool {
	op := strings.TrimSpace(file.FlatNodeText(file.FlatFirstChild(parent)))
	in := strings.TrimSpace(file.FlatNodeText(inner))
	return (op == "-" || op == "+") && strings.HasPrefix(in, op)
}

// parenCalleeCanLoseParens lists the callees that stay valid without parentheses.
func parenCalleeCanLoseParens(file *scanner.File, inner uint32) bool {
	switch file.FlatType(inner) {
	case "simple_identifier", "integer_literal", "real_literal", "string_literal", "this_expression", "null",
		"call_expression", "indexing_expression", "when_expression":
		return true
	case "navigation_expression":
		return !strings.Contains(file.FlatNodeText(inner), "?.")
	}
	return false
}

// parenPrecedenceUnclear mirrors detekt's allowForUnclearPrecedence: parentheses that only spell out a precedence
// readers often misjudge are allowed.
func parenPrecedenceUnclear(file *scanner.File, paren, inner uint32) bool {
	parent, ok := file.FlatParent(paren)
	if !ok {
		return false
	}
	ptype := file.FlatType(parent)
	if ptype == "prefix_expression" {
		return true
	}
	itype := file.FlatType(inner)
	if itype == "real_literal" && strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(inner)), ".") && ptype == "range_expression" {
		return true
	}
	switch ptype {
	case "multiplicative_expression", "additive_expression", "range_expression", "infix_expression",
		"elvis_expression", "check_expression", "comparison_expression", "equality_expression",
		"conjunction_expression", "disjunction_expression":
	default:
		return false
	}
	switch itype {
	case "multiplicative_expression":
		return ptype == "additive_expression" || ptype == "range_expression" || ptype == "elvis_expression"
	case "additive_expression":
		return ptype == "range_expression" || ptype == "elvis_expression"
	case "infix_expression":
		return ptype == "elvis_expression" || ptype == "infix_expression"
	case "elvis_expression":
		return ptype == "equality_expression" || ptype == "comparison_expression" || ptype == "check_expression"
	case "conjunction_expression":
		return ptype == "disjunction_expression"
	}
	return false
}
