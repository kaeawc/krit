package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// castTypePredicates maps target types to boolean predicates that, when
// proven true in an enclosing if/&& chain, guarantee the cast is safe.
// These are hierarchy invariants rather than smart-cast semantics.
var castTypePredicates = map[string][]string{
	"MmsMessageRecord": {
		"isMms", "isMediaMessage", "hasSharedContact", "hasLinkPreview",
		"hasSticker", "hasLocation", "hasAudio", "hasThumbnail",
		"isStoryReaction", "isStory", "hasGiftBadge", "isViewOnceMessage",
		"hasAttachment", "hasMediaMessage", "isMediaPendingMessage",
	},
	"MmsSmsDatabase": {"isMms"},
	// Recipient / group hierarchy
	"GroupRecord":               {"isGroup", "isPushGroup"},
	"GroupId.V2":                {"isV2"},
	"GroupId.V1":                {"isV1"},
	"GroupId.Mms":               {"isMms"},
	"V2":                        {"isV2"},
	"V1":                        {"isV1"},
	"GroupReply":                {"isGroupReply"},
	"RotatableGradientDrawable": {"isGradient"},
	"StoryTextPostModel":        {"isTextStory"},
	"TextStory":                 {"isTextStory"},
}

// unsafeCastIsCheckPartsFlat parses a check_expression node into its components.
func unsafeCastIsCheckPartsFlat(file *scanner.File, node uint32) (expr uint32, typeName string, positive bool, ok bool) {
	if file.FlatType(node) != "check_expression" {
		return 0, "", false, false
	}
	positive = true
	seenOp := false
	var typeNode uint32
	for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "!is":
			positive = false
			seenOp = true
		case "is":
			positive = true
			seenOp = true
		default:
			if !file.FlatIsNamed(child) {
				continue
			}
			if !seenOp && expr == 0 {
				expr = child
			} else if seenOp && typeNode == 0 {
				typeNode = child
			}
		}
	}
	if !seenOp || expr == 0 || typeNode == 0 {
		return 0, "", false, false
	}
	typeName = unsafeCastTypeNameFlat(file, typeNode)
	return expr, typeName, positive, typeName != ""
}

// unsafeCastConditionHasIsCheckFlat walks the condition subtree looking for a
// check_expression that matches castVar is/!is castTarget. Uses AST nodes only,
// so it never matches text inside comments or string literals.
func unsafeCastConditionHasIsCheckFlat(file *scanner.File, cond uint32, castVar, castTarget string, wantPositive bool) bool {
	found := false
	file.FlatWalkAllNodes(cond, func(n uint32) {
		if found {
			return
		}
		expr, typeName, positive, ok := unsafeCastIsCheckPartsFlat(file, n)
		if !ok || positive != wantPositive || typeName != castTarget {
			return
		}
		if strings.TrimSpace(file.FlatNodeText(expr)) == castVar {
			found = true
		}
	})
	return found
}

// unsafeCastConditionHasPredicateCallFlat walks the condition subtree looking
// for a call_expression whose callee name is in predicates. Only matches real
// call nodes, never text inside comments or string literals.
func unsafeCastConditionHasPredicateCallFlat(file *scanner.File, cond uint32, predicates []string) bool {
	found := false
	file.FlatWalkAllNodes(cond, func(n uint32) {
		if found || file.FlatType(n) != "call_expression" {
			return
		}
		callee := flatCallExpressionName(file, n)
		for _, pred := range predicates {
			if callee == pred {
				found = true
				return
			}
		}
	})
	return found
}

// unsafeCastExtractIfPartsFlat returns the condition node, then-body, and
// else-body of an if_expression using the AST structure.
func unsafeCastExtractIfPartsFlat(file *scanner.File, ifNode uint32) (cond, thenBody, elseBody uint32) {
	foundElse := false
	for child := file.FlatFirstChild(ifNode); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "control_structure_body":
			if !foundElse && thenBody == 0 {
				thenBody = child
			} else if foundElse && elseBody == 0 {
				elseBody = child
			}
		case "else":
			foundElse = true
		default:
			if cond == 0 && file.FlatIsNamed(child) {
				cond = child
			}
		}
	}
	return
}

// unsafeCastFindIfExprFlat extracts an if_expression from a statement node,
// handling the expression_statement wrapper.
func unsafeCastFindIfExprFlat(file *scanner.File, node uint32) uint32 {
	switch file.FlatType(node) {
	case "if_expression":
		return node
	case "expression_statement":
		for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "if_expression" {
				return child
			}
		}
	}
	return 0
}

// unsafeCastStatementAlwaysExitsFlat reports whether a single AST statement
// unconditionally transfers control (return, throw, continue, break, or an
// if/else where both branches do the same).
func unsafeCastStatementAlwaysExitsFlat(file *scanner.File, node uint32) bool {
	if node == 0 {
		return false
	}
	switch file.FlatType(node) {
	case "jump_expression":
		return true
	case "expression_statement":
		for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) {
				return unsafeCastStatementAlwaysExitsFlat(file, child)
			}
		}
	case "if_expression":
		_, thenBody, elseBody := unsafeCastExtractIfPartsFlat(file, node)
		if elseBody == 0 {
			return false
		}
		return bodyAlwaysExitsFlat(file, thenBody) && bodyAlwaysExitsFlat(file, elseBody)
	}
	return false
}

func isCastGuardedByTypePredicateFlat(file *scanner.File, idx uint32, _ string, castTarget string) bool {
	predicates, ok := castTypePredicates[castTarget]
	if !ok {
		return false
	}
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "function_declaration", "lambda_literal":
			return false
		case "conjunction_expression":
			if unsafeCastConditionHasPredicateCallFlat(file, cur, predicates) {
				return true
			}
		case "if_expression":
			cond, thenBody, _ := unsafeCastExtractIfPartsFlat(file, cur)
			if cond != 0 && thenBody != 0 && isDescendantFlat(file, idx, thenBody) {
				if unsafeCastConditionHasPredicateCallFlat(file, cond, predicates) {
					return true
				}
			}
		case "when_entry":
			if unsafeCastConditionHasPredicateCallFlat(file, cur, predicates) {
				return true
			}
		}
	}
	return false
}

// unsafeCastGuardedByIsCheck walks up the AST from an as_expression to check
// whether an enclosing if-expression or when-entry has an is-check that proves
// the cast is safe. This handles the common pattern:
//
//	if (x is Foo) { val f = x as Foo }       — safe, is-check guards the cast
//	when (x) { is Foo -> x as Foo }           — safe, when branch guards the cast
//
// It also handles early-return patterns like:
//
//	if (x !is Foo) return; val f = x as Foo   — safe, negative is-check + early exit
func unsafeCastGuardedByIsCheckFlat(file *scanner.File, idx uint32, varName, targetType string) bool {
	for current, ok := file.FlatParent(idx); ok; current, ok = file.FlatParent(current) {
		switch file.FlatType(current) {
		case "if_expression":
			if unsafeCastIfGuardsFlat(file, current, idx, varName, targetType) {
				return true
			}
		case "when_entry":
			if unsafeCastWhenEntryGuardsFlat(file, current, varName, targetType) {
				return true
			}
		case "statements", "function_body", "class_body":
			if unsafeCastPrecedingNegativeIsCheckFlat(file, current, idx, varName, targetType) {
				return true
			}
		case "function_declaration", "lambda_literal", "anonymous_function":
			return false
		}
	}
	return false
}

func unsafeCastPrecedingNegativeIsCheckFlat(file *scanner.File, statementsNode, asNode uint32, varName, targetType string) bool {
	asStart := file.FlatStartByte(asNode)
	for i := 0; i < file.FlatChildCount(statementsNode); i++ {
		child := file.FlatChild(statementsNode, i)
		if child == 0 {
			continue
		}
		if file.FlatStartByte(child) >= asStart {
			break
		}
		ifNode := unsafeCastFindIfExprFlat(file, child)
		if ifNode == 0 {
			continue
		}
		cond, thenBody, _ := unsafeCastExtractIfPartsFlat(file, ifNode)
		if cond == 0 || thenBody == 0 {
			continue
		}
		if unsafeCastConditionHasIsCheckFlat(file, cond, varName, targetType, false) &&
			bodyAlwaysExitsFlat(file, thenBody) {
			return true
		}
	}
	return false
}

func unsafeCastIfGuardsFlat(file *scanner.File, ifNode, asNode uint32, varName, targetType string) bool {
	cond, thenBody, elseBody := unsafeCastExtractIfPartsFlat(file, ifNode)
	if cond == 0 {
		return false
	}

	// if (x is T) { ... x as T ... }
	if unsafeCastConditionHasIsCheckFlat(file, cond, varName, targetType, true) {
		if thenBody != 0 && isDescendantFlat(file, asNode, thenBody) {
			return true
		}
	}

	// if (x !is T) { return/throw } then x as T (in else or after the if)
	if unsafeCastConditionHasIsCheckFlat(file, cond, varName, targetType, false) {
		if thenBody != 0 && bodyAlwaysExitsFlat(file, thenBody) {
			if elseBody != 0 && isDescendantFlat(file, asNode, elseBody) {
				return true
			}
			if !isDescendantFlat(file, asNode, ifNode) {
				return true
			}
		}
	}

	return false
}

func unsafeCastWhenEntryGuardsFlat(file *scanner.File, whenEntry uint32, varName, targetType string) bool {
	whenExpr, ok := file.FlatParent(whenEntry)
	if !ok || file.FlatType(whenExpr) != "when_expression" {
		return false
	}
	whenSubject := ""
	for i := 0; i < file.FlatChildCount(whenExpr); i++ {
		child := file.FlatChild(whenExpr, i)
		if file.FlatType(child) == "when_subject" {
			subjectText := strings.TrimSpace(file.FlatNodeText(child))
			if strings.HasPrefix(subjectText, "(") && strings.HasSuffix(subjectText, ")") {
				subjectText = strings.TrimSpace(subjectText[1 : len(subjectText)-1])
			}
			if strings.HasPrefix(subjectText, "val ") {
				if eqIdx := strings.Index(subjectText, "="); eqIdx >= 0 {
					subjectText = strings.TrimSpace(subjectText[eqIdx+1:])
				}
			}
			whenSubject = subjectText
			break
		}
	}
	if whenSubject != varName {
		return false
	}
	for i := 0; i < file.FlatChildCount(whenEntry); i++ {
		child := file.FlatChild(whenEntry, i)
		switch file.FlatType(child) {
		case "when_condition", "type_test":
			condText := strings.TrimSpace(file.FlatNodeText(child))
			if condText == "is "+targetType {
				return true
			}
		}
	}
	return false
}
