package nullflow

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// IsMapContainsKeyGuarded reports whether useIdx sits inside a control flow
// branch dominated by a `receiver.containsKey(key)` check (or its negation in
// the else branch). The receiver and key nodes are matched structurally; both
// must be named flat-AST nodes from the same file.
func IsMapContainsKeyGuarded(file *scanner.File, useIdx uint32, receiver, key uint32) bool {
	for p, ok := file.FlatParent(useIdx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "function_declaration" || file.FlatType(p) == "lambda_literal" {
			break
		}
		if file.FlatType(p) != "control_structure_body" {
			continue
		}
		parent, ok := file.FlatParent(p)
		if !ok || file.FlatType(parent) != "if_expression" {
			continue
		}
		cond, thenBody, elseBody := flatIfConditionBodies(file, parent)
		if cond == 0 {
			continue
		}
		if thenBody == p && mapContainsKeyConditionProves(file, cond, receiver, key, true) {
			return true
		}
		if elseBody == p && mapContainsKeyConditionProves(file, cond, receiver, key, false) {
			return true
		}
	}
	return false
}

// IsEarlyReturnMapContainsKeyGuarded reports whether useIdx is preceded in the
// same statements scope by an `if (!receiver.containsKey(key)) return` style
// guard. The bodyExits parameter is the caller-supplied predicate that decides
// whether a control_structure_body always transfers control out of the
// enclosing function (return, throw, break, continue, or a fully-covering
// nested if/else chain). nullflow does not own that decision because it lives
// outside null reasoning.
func IsEarlyReturnMapContainsKeyGuarded(
	file *scanner.File,
	useIdx uint32,
	receiver, key uint32,
	bodyExits func(file *scanner.File, body uint32) bool,
) bool {
	if bodyExits == nil {
		return false
	}
	var anchor uint32
	var statements uint32
	child := useIdx
	for p, ok := file.FlatParent(useIdx); ok; p, ok = file.FlatParent(p) {
		t := file.FlatType(p)
		if t == "function_declaration" || t == "lambda_literal" {
			break
		}
		if t == "statements" {
			statements = p
			anchor = child
			break
		}
		child = p
	}
	if statements == 0 || anchor == 0 {
		return false
	}
	for stmt := file.FlatFirstChild(statements); stmt != 0; stmt = file.FlatNextSib(stmt) {
		if !file.FlatIsNamed(stmt) {
			continue
		}
		if stmt == anchor || file.FlatStartByte(stmt) >= file.FlatStartByte(anchor) {
			break
		}
		if file.FlatType(stmt) != "if_expression" {
			continue
		}
		cond, thenBody, elseBody := flatIfConditionBodies(file, stmt)
		if cond == 0 || thenBody == 0 || elseBody != 0 {
			continue
		}
		if !bodyExits(file, thenBody) {
			continue
		}
		if mapContainsKeyConditionProves(file, cond, receiver, key, false) {
			return true
		}
	}
	return false
}

// IsInsideContainsKeyFilterChain reports whether useIdx is inside a transform
// lambda (.map / .forEach / .let / etc.) whose receiver chain has a preceding
// `.filter { receiver.containsKey(...) }` step. In that case any subsequent
// access on the same receiver inside the transform is provably non-null.
//
//nolint:gocyclo // Walks lambda → transform-call → receiver chain (each with several shape branches) without auxiliary state to thread through helpers. Splitting raises argument-passing cost without reducing essential branch count.
func IsInsideContainsKeyFilterChain(file *scanner.File, useIdx uint32, receiver uint32) bool {
	var lambda uint32
	for p, ok := file.FlatParent(useIdx); ok; p, ok = file.FlatParent(p) {
		t := file.FlatType(p)
		if t == "lambda_literal" {
			lambda = p
			break
		}
		if t == "function_declaration" || t == "source_file" {
			return false
		}
	}
	if lambda == 0 {
		return false
	}
	var transformCall uint32
	for p, ok := file.FlatParent(lambda); ok; p, ok = file.FlatParent(p) {
		t := file.FlatType(p)
		if t == "call_expression" {
			transformCall = p
			break
		}
		if t == "function_declaration" || t == "source_file" {
			return false
		}
	}
	if transformCall == 0 {
		return false
	}
	navExpr, _ := file.FlatFindChild(transformCall, "navigation_expression")
	if navExpr == 0 {
		return false
	}
	callee := flatNavigationExpressionLastIdentifier(file, navExpr)
	switch callee {
	case "map", "mapNotNull", "mapIndexed", "flatMap", "forEach",
		"forEachIndexed", "associate", "associateBy", "associateWith",
		"sortedBy", "sortedByDescending", "groupBy", "onEach", "let":
	default:
		return false
	}
	cur := navExpr
	for i := 0; i < 8; i++ {
		if cur == 0 || file.FlatNamedChildCount(cur) == 0 {
			return false
		}
		recv := file.FlatNamedChild(cur, 0)
		if recv == 0 {
			return false
		}
		if file.FlatType(recv) == "call_expression" {
			recvCallee, _ := file.FlatFindChild(recv, "navigation_expression")
			if recvCallee != 0 {
				last := flatNavigationExpressionLastIdentifier(file, recvCallee)
				if last == "filter" || last == "filterKeys" || last == "filterValues" {
					if flatSubtreeHasContainsKeyForReceiver(file, recv, receiver) {
						return true
					}
				}
				cur = recvCallee
				continue
			}
		}
		if file.FlatType(recv) == "navigation_expression" {
			cur = recv
			continue
		}
		return false
	}
	return false
}

func flatIfConditionBodies(file *scanner.File, ifExpr uint32) (cond, thenBody, elseBody uint32) {
	foundElse := false
	for child := file.FlatFirstChild(ifExpr); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "else":
			foundElse = true
		case "control_structure_body":
			if !foundElse && thenBody == 0 {
				thenBody = child
			} else if foundElse && elseBody == 0 {
				elseBody = child
			}
		default:
			if cond == 0 && file.FlatIsNamed(child) && file.FlatType(child) != "control_structure_body" {
				cond = child
			}
		}
	}
	return cond, thenBody, elseBody
}

func mapContainsKeyConditionProves(file *scanner.File, cond, receiver, key uint32, truth bool) bool {
	proves := false
	file.FlatWalkAllNodes(cond, func(candidate uint32) {
		if proves || file.FlatType(candidate) != "call_expression" {
			return
		}
		if !mapContainsKeyCallMatches(file, candidate, receiver, key) {
			return
		}
		negated := flatCallNegatedWithin(file, candidate, cond)
		if truth {
			if !negated && !flatHasAncestorBetween(file, candidate, cond, "disjunction_expression") {
				proves = true
			}
			return
		}
		if negated && !flatHasAncestorBetween(file, candidate, cond, "conjunction_expression") {
			proves = true
		}
	})
	return proves
}

func mapContainsKeyCallMatches(file *scanner.File, call, receiver, key uint32) bool {
	nav, args := flatCallExpressionParts(file, call)
	if nav == 0 || args == 0 || flatNavigationExpressionLastIdentifier(file, nav) != "containsKey" {
		return false
	}
	if flatNavigationLastSuffixHasSafeAccess(file, nav) {
		return false
	}
	callReceiver := flatNavigationExpressionReceiver(file, nav)
	if callReceiver == 0 || !flatExpressionsEquivalent(file, callReceiver, receiver) {
		return false
	}
	arg, ok := flatSingleValueArgumentExpression(file, args)
	return ok && flatExpressionsEquivalent(file, arg, key)
}

func flatSubtreeHasContainsKeyForReceiver(file *scanner.File, root, receiver uint32) bool {
	found := false
	file.FlatWalkAllNodes(root, func(candidate uint32) {
		if found || file.FlatType(candidate) != "call_expression" {
			return
		}
		nav, _ := flatCallExpressionParts(file, candidate)
		if nav == 0 || flatNavigationExpressionLastIdentifier(file, nav) != "containsKey" {
			return
		}
		callReceiver := flatNavigationExpressionReceiver(file, nav)
		if callReceiver != 0 && flatExpressionsEquivalent(file, callReceiver, receiver) {
			found = true
		}
	})
	return found
}

func flatCallNegatedWithin(file *scanner.File, call, root uint32) bool {
	negated := false
	for p, ok := file.FlatParent(call); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "prefix_expression" && flatPrefixExpressionIsBang(file, p) {
			negated = !negated
		}
		if p == root {
			break
		}
	}
	return negated
}

func flatPrefixExpressionIsBang(file *scanner.File, idx uint32) bool {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) && file.FlatType(child) == "!" {
			return true
		}
	}
	return false
}

func flatHasAncestorBetween(file *scanner.File, idx, root uint32, nodeType string) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if p == root {
			return false
		}
		if file.FlatType(p) == nodeType {
			return true
		}
	}
	return false
}

func flatExpressionsEquivalent(file *scanner.File, a, b uint32) bool {
	a = flatUnwrapParenExpr(file, a)
	b = flatUnwrapParenExpr(file, b)
	if a == 0 || b == 0 {
		return false
	}
	if a == b {
		return true
	}
	if file.FlatType(a) != file.FlatType(b) {
		return false
	}
	return strings.TrimSpace(file.FlatNodeText(a)) == strings.TrimSpace(file.FlatNodeText(b))
}
