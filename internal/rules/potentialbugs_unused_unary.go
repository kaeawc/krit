package rules

import (
	"bytes"
	"fmt"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// unaryBinaryTypes are the grammar nodes the Kotlin PSI models as a binary expression. Assignments are
// included because the PSI treats `a = b` as one.
var unaryBinaryTypes = map[string]bool{
	"additive_expression": true, "multiplicative_expression": true, "comparison_expression": true,
	"equality_expression": true, "conjunction_expression": true, "disjunction_expression": true,
	"elvis_expression": true, "infix_expression": true, "range_expression": true, "assignment": true,
}

// unaryUnitLambdaCalls take a lambda whose result is Unit, so its last statement is not a value.
var unaryUnitLambdaCalls = map[string]bool{
	"forEach": true, "forEachIndexed": true, "onEach": true, "onEachIndexed": true, "also": true,
	"apply": true, "repeat": true, "launch": true,
}

// checkUnusedUnaryOperator follows detekt's rule: a `+` or `-` prefix expression that starts a line and
// whose enclosing binary expression is never used as a value. The scanner ends the statement above
// such a line, so the tree has the shape Kotlin reads. The grammar nests the whole rest of the
// expression under the prefix, so the operand's left spine is walked to find what the prefix binds to.
//
// detekt reports only operators that resolve to the stdlib. Without types this also reports an unused
// user-defined `unaryMinus` or `unaryPlus`, whose discarded result is still a bug.
func checkUnusedUnaryOperator(ctx *api.Context, r *UnusedUnaryOperatorRule) {
	idx, file := ctx.Idx, ctx.File
	op := file.FlatFirstChild(idx)
	if op == 0 {
		return
	}
	if t := file.FlatType(op); t != "+" && t != "-" {
		return
	}
	operand := file.FlatNextSib(op)
	if operand == 0 || !unaryStartsAfterLineBreak(file, idx) {
		return
	}

	// `-x as T` and `-x is T` use the prefix as an operand; `-x + 1` and `-x in r` do not.
	leaf := operand
	var spine []uint32
	for unaryIsSpineNode(file, leaf) {
		spine = append(spine, leaf)
		leaf = file.FlatFirstChild(leaf)
		if leaf == 0 {
			return
		}
	}
	for _, n := range spine {
		if !unaryIsBinary(file, n) {
			return
		}
	}
	switch file.FlatType(leaf) {
	case "callable_reference":
		return
	}

	top := idx
	for {
		parent, ok := file.FlatParent(top)
		if !ok || !(unaryBinaryTypes[file.FlatType(parent)] || file.FlatType(parent) == "check_expression" && unaryIsBinary(file, parent)) {
			break
		}
		// detekt does not report `a += -b` on a var: the compiler rewrites it to an assignment it uses.
		// An indexed target such as `a[i] += -b` is reported.
		if unaryIsAugmentedAssignment(file, parent) {
			return
		}
		top = parent
	}
	if unaryUsedAsExpression(file, top) {
		return
	}

	text := string(file.Content[file.FlatStartByte(top):file.FlatEndByte(top)])
	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, fmt.Sprintf("This '%s' is not used", text))
	if top == idx {
		if start, end, ok := unaryRemovableLines(file, idx); ok {
			f.Fix = &scanner.Fix{ByteMode: true, StartByte: start, EndByte: end}
		}
	}
	ctx.Emit(f)
}

// unaryIsSpineNode reports a node the grammar nests an operand's left side under.
func unaryIsSpineNode(file *scanner.File, n uint32) bool {
	switch t := file.FlatType(n); t {
	case "as_expression":
		return true
	case "check_expression":
		return true
	default:
		return unaryBinaryTypes[t] && t != "assignment"
	}
}

// unaryIsBinary reports whether the Kotlin PSI models n as a binary expression. `as` and `is` are not.
func unaryIsBinary(file *scanner.File, n uint32) bool {
	switch file.FlatType(n) {
	case "as_expression":
		return false
	case "check_expression":
		for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
			switch file.FlatType(c) {
			case "in", "!in":
				return true
			case "is", "!is":
				return false
			}
		}
		return false
	}
	return true
}

// unaryStartsAfterLineBreak reports whether only blanks and comments separate the node from a line break.
func unaryStartsAfterLineBreak(file *scanner.File, n uint32) bool {
	c := file.Content
	i := int(file.FlatStartByte(n))
	for i > 0 {
		switch c[i-1] {
		case ' ', '\t', '\r':
			i--
		case '\n':
			return true
		case '/':
			if i < 2 || c[i-2] != '*' {
				return false
			}
			open := bytes.LastIndex(c[:i-2], []byte("/*"))
			if open < 0 {
				return false
			}
			i = open
		default:
			return false
		}
	}
	return false
}

// unaryUsedAsExpression reports whether the value of expression n is consumed.
func unaryUsedAsExpression(file *scanner.File, n uint32) bool {
	parent, ok := file.FlatParent(n)
	if !ok {
		return false
	}
	switch file.FlatType(parent) {
	case "statements":
		for s := file.FlatNextSib(n); s != 0; s = file.FlatNextSib(s) {
			switch file.FlatType(s) {
			case ";", "line_comment", "multiline_comment":
			default:
				return false
			}
		}
		return unaryBlockValueUsed(file, parent)
	case "control_structure_body":
		return unaryBodyValueUsed(file, parent)
	case "parenthesized_expression":
		return unaryUsedAsExpression(file, parent)
	case "source_file":
		return false
	}
	return true
}

// unaryBlockValueUsed reports whether the value of the last statement of a statements node is consumed.
func unaryBlockValueUsed(file *scanner.File, stmts uint32) bool {
	parent, ok := file.FlatParent(stmts)
	if !ok {
		return true
	}
	switch file.FlatType(parent) {
	case "function_body", "anonymous_initializer", "secondary_constructor", "finally_block", "source_file":
		return false
	case "control_structure_body":
		return unaryBodyValueUsed(file, parent)
	case "try_expression", "catch_block":
		try := parent
		if file.FlatType(parent) == "catch_block" {
			try, ok = file.FlatParent(parent)
			if !ok {
				return true
			}
		}
		return unaryUsedAsExpression(file, try)
	case "lambda_literal":
		return !unaryLambdaReturnsUnit(file, parent)
	}
	return true
}

// unaryBodyValueUsed reports whether the value of a branch or loop body is consumed.
func unaryBodyValueUsed(file *scanner.File, body uint32) bool {
	owner, ok := file.FlatParent(body)
	if !ok {
		return true
	}
	switch file.FlatType(owner) {
	case "if_expression":
		return unaryUsedAsExpression(file, owner)
	case "when_entry":
		when, ok := file.FlatParent(owner)
		return !ok || unaryUsedAsExpression(file, when)
	case "for_statement", "while_statement", "do_while_statement":
		return false
	}
	return true
}

// unaryLambdaReturnsUnit reports a lambda passed to a call known to take a Unit lambda.
func unaryLambdaReturnsUnit(file *scanner.File, lambda uint32) bool {
	n, ok := file.FlatParent(lambda)
	for ok {
		switch file.FlatType(n) {
		case "annotated_lambda", "call_suffix", "value_arguments", "value_argument":
			n, ok = file.FlatParent(n)
			continue
		case "call_expression":
			callee := file.FlatFirstChild(n)
			for callee != 0 && file.FlatType(callee) == "call_expression" {
				callee = file.FlatFirstChild(callee)
			}
			if callee != 0 && file.FlatType(callee) == "navigation_expression" {
				callee = unaryLastChild(file, unaryLastChild(file, callee))
			}
			return callee != 0 && unaryUnitLambdaCalls[file.FlatNodeText(callee)]
		}
		return false
	}
	return false
}

// unaryRemovableLines returns the lines of a prefix expression that can be deleted without changing
// behavior: it sits alone on its lines and its operand has no calls or assignments.
func unaryRemovableLines(file *scanner.File, idx uint32) (int, int, bool) {
	for _, t := range []string{"call_expression", "lambda_literal", "assignment", "anonymous_function", "postfix_expression", "object_literal"} {
		found := false
		file.FlatWalkNodes(idx, t, func(uint32) { found = true })
		if found {
			return 0, 0, false
		}
	}
	start, end := nodeLineRange(file.Content, int(file.FlatStartByte(idx)), int(file.FlatEndByte(idx)))
	rest := file.Content[file.FlatEndByte(idx):]
	if nl := bytes.IndexByte(rest, '\n'); nl >= 0 {
		rest = rest[:nl]
	}
	if len(bytes.TrimSpace(rest)) != 0 {
		return 0, 0, false
	}
	return start, end, true
}

func unaryLastChild(file *scanner.File, n uint32) uint32 {
	var last uint32
	if n != 0 {
		for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
			last = c
		}
	}
	return last
}

func unaryIsAugmentedAssignment(file *scanner.File, n uint32) bool {
	if file.FlatType(n) != "assignment" {
		return false
	}
	target := file.FlatFirstChild(n)
	if target != 0 && bytes.HasSuffix(file.Content[file.FlatStartByte(target):file.FlatEndByte(target)], []byte("]")) {
		return false
	}
	for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "+=", "-=", "*=", "/=", "%=":
			return true
		}
	}
	return false
}
