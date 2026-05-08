package nullflow

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// IsGuardedNonNull reports whether useIdx sits in a control-flow region where
// the receiver expression is provably non-null via an enclosing if/when guard.
// Walks outward from useIdx, looking at each control_structure_body or
// when_entry to see if its condition proves the receiver non-null.
//
//nolint:gocyclo // Two parent walks (if/when control bodies, then bare when_entries) plus three condition shapes (if-then, if-else, when-condition). Splitting hurts readability without reducing essential complexity.
func IsGuardedNonNull(file *scanner.File, useIdx uint32, receiver uint32) bool {
	if receiver == 0 {
		return false
	}
	for current, ok := file.FlatParent(useIdx); ok; current, ok = file.FlatParent(current) {
		t := file.FlatType(current)
		if t == "function_declaration" || t == "lambda_literal" {
			return false
		}
		if t != "control_structure_body" {
			continue
		}
		parent, ok := file.FlatParent(current)
		if !ok {
			continue
		}
		if file.FlatType(parent) == "when_entry" && whenEntryConditionsProveNonNull(file, parent, receiver, useIdx) {
			return true
		}
		if file.FlatType(parent) != "if_expression" {
			continue
		}
		cond, thenBody, elseBody := ifConditionThenElseBodies(file, parent)
		if cond == 0 {
			continue
		}
		if thenBody == current && ConditionTrueProvesNonNull(file, cond, receiver, useIdx) {
			return true
		}
		if elseBody == current && ConditionFalseProvesNonNull(file, cond, receiver, useIdx) {
			return true
		}
	}
	for current, ok := file.FlatParent(useIdx); ok; current, ok = file.FlatParent(current) {
		t := file.FlatType(current)
		if t == "function_declaration" || t == "lambda_literal" {
			return false
		}
		if t != "when_entry" {
			continue
		}
		body := whenEntryBody(file, current)
		if body != 0 && flatNodeWithin(file, body, useIdx) &&
			whenEntryConditionsProveNonNull(file, current, receiver, useIdx) {
			return true
		}
	}
	return false
}

func whenEntryBody(file *scanner.File, entry uint32) uint32 {
	if file == nil || entry == 0 || file.FlatType(entry) != "when_entry" {
		return 0
	}
	if body, ok := file.FlatFindChild(entry, "control_structure_body"); ok {
		return body
	}
	for i := file.FlatNamedChildCount(entry) - 1; i >= 0; i-- {
		child := file.FlatNamedChild(entry, i)
		if child == 0 {
			continue
		}
		switch file.FlatType(child) {
		case "when_condition", "else":
			continue
		default:
			return child
		}
	}
	return 0
}

func whenEntryConditionsProveNonNull(file *scanner.File, entry, receiver, useIdx uint32) bool {
	if file == nil || entry == 0 || file.FlatType(entry) != "when_entry" {
		return false
	}
	found := false
	for child := file.FlatFirstChild(entry); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "else":
			return false
		case "when_condition":
			found = true
			if !ConditionTrueProvesNonNull(file, child, receiver, useIdx) {
				return false
			}
		}
	}
	return found
}

// IsEarlyReturnGuarded reports whether an `if (x == null) return` style early
// exit appears earlier in the same statements scope as useIdx, proving the
// receiver non-null. The bodyExits callback decides whether a control body
// always transfers control out of the function (return/throw/break/continue
// or a fully-covering nested if-else); nullflow does not own that CFG logic.
func IsEarlyReturnGuarded(
	file *scanner.File,
	useIdx uint32,
	receiver uint32,
	bodyExits func(file *scanner.File, body uint32) bool,
) bool {
	if receiver == 0 || bodyExits == nil {
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
	for i := 0; i < file.FlatNamedChildCount(statements); i++ {
		stmt := file.FlatNamedChild(statements, i)
		if stmt == 0 {
			continue
		}
		if stmt == anchor || file.FlatStartByte(stmt) >= file.FlatStartByte(anchor) {
			break
		}
		if file.FlatType(stmt) != "if_expression" {
			continue
		}
		cond, thenBody, elseBody := ifConditionThenElseBodies(file, stmt)
		if cond == 0 || thenBody == 0 || elseBody != 0 {
			continue
		}
		if !bodyExits(file, thenBody) {
			continue
		}
		if ConditionFalseProvesNonNull(file, cond, receiver, useIdx) {
			return true
		}
	}
	return false
}

// IsShortCircuitGuardedNonNull reports whether the receiver is proven non-null
// by a preceding operand of an enclosing conjunction_expression. This catches
// `x != null && x!!.y` patterns where the right-hand bang relies on the
// short-circuit evaluation of the left side.
func IsShortCircuitGuardedNonNull(file *scanner.File, useIdx uint32, receiver uint32) bool {
	if file == nil || useIdx == 0 || receiver == 0 {
		return false
	}
	for current, ok := file.FlatParent(useIdx); ok; current, ok = file.FlatParent(current) {
		switch file.FlatType(current) {
		case "function_declaration", "lambda_literal", "if_expression", "when_expression", "try_expression":
			return false
		case "conjunction_expression":
			return conjunctionPrecedingOperandsProveNonNull(file, current, useIdx, receiver)
		}
	}
	return false
}

func conjunctionPrecedingOperandsProveNonNull(file *scanner.File, conjunction, useIdx, receiver uint32) bool {
	for child := file.FlatFirstChild(conjunction); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		if flatNodeWithin(file, child, useIdx) {
			return false
		}
		if ConditionTrueProvesNonNull(file, child, receiver, useIdx) {
			return true
		}
	}
	return false
}

// IsSameBlockAssignedNonNullBeforeUse reports whether a simple-name receiver
// is provably non-null at useIdx because of an assignment earlier in the same
// statements block. Handles direct `x = nonNullValue` and
// `if (x == null) x = create()` shapes. The resolver, when supplied, is
// consulted to resolve the assignment RHS type; nil callers fall back to a
// purely structural heuristic.
func IsSameBlockAssignedNonNullBeforeUse(file *scanner.File, useIdx uint32, receiver uint32, resolver typeinfer.TypeResolver) bool {
	name := simpleReceiverName(file, receiver)
	if name == "" {
		return false
	}
	statements, statement := enclosingStatementInStatements(file, useIdx)
	if statements == 0 || statement == 0 {
		return false
	}
	proven := false
	for child := file.FlatFirstChild(statements); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		if child == statement || flatNodeWithin(file, child, useIdx) {
			return proven
		}
		if statementDominatesNonNullAssignment(file, child, name, receiver, useIdx, resolver) {
			proven = true
			continue
		}
		if statementMayWriteSimpleName(file, child, name) {
			proven = false
		}
	}
	return proven
}

func simpleReceiverName(file *scanner.File, receiver uint32) string {
	if file == nil || receiver == 0 {
		return ""
	}
	receiver = flatUnwrapParenExpr(file, receiver)
	text := strings.TrimPrefix(strings.TrimSpace(file.FlatNodeText(receiver)), "this.")
	if text == "" {
		return ""
	}
	for _, c := range text {
		if c == '_' || (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			continue
		}
		return ""
	}
	return text
}

func enclosingStatementInStatements(file *scanner.File, idx uint32) (statements uint32, statement uint32) {
	if file == nil || idx == 0 {
		return 0, 0
	}
	child := idx
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		if file.FlatType(parent) == "function_declaration" || file.FlatType(parent) == "lambda_literal" {
			return 0, 0
		}
		if file.FlatType(parent) == "statements" {
			return parent, child
		}
		child = parent
	}
	return 0, 0
}

func statementDominatesNonNullAssignment(file *scanner.File, stmt uint32, name string, receiver, useIdx uint32, resolver typeinfer.TypeResolver) bool {
	switch file.FlatType(stmt) {
	case "assignment":
		return assignmentWritesSimpleName(file, stmt, name) && assignmentRHSIsNonNull(file, stmt, resolver)
	case "if_expression":
		condition, thenBody, elseBody := ifConditionThenElseBodies(file, stmt)
		if condition == 0 || thenBody == 0 || elseBody != 0 {
			return false
		}
		return ConditionTrueProvesNull(file, condition, receiver, useIdx) &&
			nodeAssignsSimpleNameNonNull(file, thenBody, name, resolver)
	default:
		return false
	}
}

func nodeAssignsSimpleNameNonNull(file *scanner.File, node uint32, name string, resolver typeinfer.TypeResolver) bool {
	if file == nil || node == 0 {
		return false
	}
	switch file.FlatType(node) {
	case "control_structure_body":
		if stmts, ok := file.FlatFindChild(node, "statements"); ok {
			return nodeAssignsSimpleNameNonNull(file, stmts, name, resolver)
		}
		if file.FlatNamedChildCount(node) == 1 {
			return nodeAssignsSimpleNameNonNull(file, file.FlatNamedChild(node, 0), name, resolver)
		}
	case "statements":
		for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
			if !file.FlatIsNamed(child) {
				continue
			}
			return file.FlatType(child) == "assignment" &&
				assignmentWritesSimpleName(file, child, name) &&
				assignmentRHSIsNonNull(file, child, resolver)
		}
	case "assignment":
		return assignmentWritesSimpleName(file, node, name) && assignmentRHSIsNonNull(file, node, resolver)
	}
	return false
}

func statementMayWriteSimpleName(file *scanner.File, stmt uint32, name string) bool {
	if file == nil || stmt == 0 || name == "" {
		return false
	}
	writes := false
	file.FlatWalkNodes(stmt, "assignment", func(assign uint32) {
		if writes {
			return
		}
		writes = assignmentWritesSimpleName(file, assign, name)
	})
	return writes
}

func assignmentWritesSimpleName(file *scanner.File, assignment uint32, name string) bool {
	if file == nil || assignment == 0 || file.FlatType(assignment) != "assignment" {
		return false
	}
	left := firstNamedChildBeforeToken(file, assignment, "=")
	if left == 0 {
		return false
	}
	return finalSimpleIdentifier(file, left) == name
}

func assignmentRHSIsNonNull(file *scanner.File, assignment uint32, resolver typeinfer.TypeResolver) bool {
	if file == nil || assignment == 0 || file.FlatType(assignment) != "assignment" {
		return false
	}
	rhs := firstNamedChildAfterToken(file, assignment, "=")
	rhs = flatUnwrapParenExpr(file, rhs)
	if rhs == 0 || flatIsNullLiteral(file, rhs) {
		return false
	}
	if resolver != nil {
		if nullable := resolver.IsNullableFlat(rhs, file); nullable != nil {
			return !*nullable
		}
	}
	switch file.FlatType(rhs) {
	case "string_literal", "integer_literal", "real_literal", "boolean_literal", "collection_literal", "object_literal":
		return true
	case "call_expression":
		name := flatCallExpressionName(file, rhs)
		if name == "" {
			name = flatCallNameAny(file, rhs)
		}
		if name != "" {
			first := name[0]
			return first >= 'A' && first <= 'Z'
		}
	}
	return false
}

func firstNamedChildBeforeToken(file *scanner.File, parent uint32, token string) uint32 {
	var last uint32
	for child := file.FlatFirstChild(parent); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == token {
			return last
		}
		if file.FlatIsNamed(child) {
			last = child
		}
	}
	return 0
}

func firstNamedChildAfterToken(file *scanner.File, parent uint32, token string) uint32 {
	seen := false
	for child := file.FlatFirstChild(parent); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == token {
			seen = true
			continue
		}
		if seen && file.FlatIsNamed(child) {
			return child
		}
	}
	return 0
}
