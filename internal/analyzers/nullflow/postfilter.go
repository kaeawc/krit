package nullflow

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// IsPostFilterSmartCast reports whether useIdx is inside a transform lambda
// (.map / .forEach / .let / .associate / etc.) preceded in the same chain by
// a `.filter { it.<field> != null }` (or `.filterNotNull()` for the bare `it`
// case). receiverText is the textual receiver under the bang — typically
// "it" or "it.<field>". When the predicate holds, a `!!` on that field is a
// smart-cast workaround rather than an unsafe assertion.
func IsPostFilterSmartCast(file *scanner.File, useIdx uint32, receiverText string) bool {
	base := strings.TrimSuffix(receiverText, ".")
	if !strings.HasPrefix(base, "it.") && base != "it" {
		return false
	}
	field := strings.TrimPrefix(base, "it.")
	lambda := postFilterFindEnclosingLambda(file, useIdx)
	if lambda == 0 {
		return false
	}
	for p, ok := file.FlatParent(lambda); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) != "call_expression" {
			if file.FlatType(p) == "function_declaration" {
				return false
			}
			continue
		}
		if postFilterChainHasFilter(file, p, base, field) {
			return true
		}
	}
	return false
}

func postFilterFindEnclosingLambda(file *scanner.File, idx uint32) uint32 {
	var lambda uint32
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "function_declaration":
			return 0
		case "lambda_literal":
			lambda = p
		}
		if lambda != 0 {
			break
		}
	}
	return lambda
}

func postFilterChainHasFilter(file *scanner.File, callExpr uint32, base, field string) bool {
	chain := callExpr
	for {
		parent, ok := file.FlatParent(chain)
		if !ok {
			break
		}
		pt := file.FlatType(parent)
		if pt != "navigation_expression" && pt != "call_expression" {
			break
		}
		chain = parent
	}
	navExpr, _ := file.FlatFindChild(chain, "navigation_expression")
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
	return postFilterWalkReceiverChain(file, navExpr, base, field)
}

func postFilterWalkReceiverChain(file *scanner.File, navExpr uint32, base, field string) bool {
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
				if last == "filterNotNull" && base == "it" {
					return true
				}
				if last == "filter" || last == "filterKeys" || last == "filterValues" {
					if filterLambdaGuardsField(file, recv, field) {
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

// filterLambdaGuardsField walks the lambda body of a filter/filterKeys/filterValues
// call_expression and returns true if it finds an equality_expression proving
// that <lambdaParam>.<field> != null. Handles both implicit "it" and named params.
//
//nolint:gocyclo // Three sequential walks (locate lambda, resolve param name, scan body) with type-shape branching at each. Extracting helpers re-walks the AST and trades cyclomatic for runtime cost.
func filterLambdaGuardsField(file *scanner.File, filterCall uint32, field string) bool {
	var lambdaLiteral uint32
	file.FlatWalkAllNodes(filterCall, func(candidate uint32) {
		if lambdaLiteral != 0 || candidate == filterCall {
			return
		}
		if file.FlatType(candidate) == "lambda_literal" {
			lambdaLiteral = candidate
		}
	})
	if lambdaLiteral == 0 {
		return false
	}

	paramName := "it"
	for child := file.FlatFirstChild(lambdaLiteral); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "lambda_parameters" {
			continue
		}
		for inner := file.FlatFirstChild(child); inner != 0; inner = file.FlatNextSib(inner) {
			if !file.FlatIsNamed(inner) {
				continue
			}
			name := ""
			if file.FlatType(inner) == "simple_identifier" {
				name = file.FlatNodeText(inner)
			} else {
				for param := file.FlatFirstChild(inner); param != 0; param = file.FlatNextSib(param) {
					if file.FlatType(param) == "simple_identifier" {
						name = file.FlatNodeText(param)
						break
					}
				}
			}
			if name != "" {
				paramName = name
				break
			}
		}
		break
	}

	found := false
	file.FlatWalkAllNodes(lambdaLiteral, func(candidate uint32) {
		if found || file.FlatType(candidate) != "equality_expression" || file.FlatChildCount(candidate) < 3 {
			return
		}
		lhs := flatUnwrapParenExpr(file, file.FlatChild(candidate, 0))
		op := file.FlatChild(candidate, 1)
		rhs := flatUnwrapParenExpr(file, file.FlatChild(candidate, 2))
		if lhs == 0 || op == 0 || rhs == 0 {
			return
		}
		if strings.TrimSpace(file.FlatNodeText(op)) != "!=" || !flatIsNullLiteral(file, rhs) {
			return
		}
		if file.FlatType(lhs) != "navigation_expression" {
			return
		}
		recv := flatNavigationExpressionReceiver(file, lhs)
		sel := flatNavigationExpressionLastIdentifier(file, lhs)
		if recv != 0 && sel == field && strings.TrimSpace(file.FlatNodeText(recv)) == paramName {
			found = true
		}
	})
	return found
}
