package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// perfExpr is a logical view of a Kotlin or Java expression, shaped like the UAST tree Lint's detectors walk.
//
// tree-sitter-kotlin parses a prefix operator over the whole binary expression after it (`!a && b` becomes
// `!(a && b)`), while Kotlin binds the prefix to `a` alone. perfBuild re-associates those operators onto the leftmost
// operand, so operand nesting depths match the compiler's tree.
type perfExpr struct {
	kind  perfKind
	start int    // start byte of the expression's source
	end   int    // end byte of the expression's source
	op    string // operator text for unary, binary and assignment expressions
	name  string // called or referenced name for calls and simple names
	kids  []*perfExpr
	// recv is the receiver of a qualified call or reference, or nil.
	recv *perfExpr
	// node is the flat node the expression was built from (0 for synthesized prefix expressions).
	node uint32
	java bool
}

type perfKind int

const (
	perfOther     perfKind = iota
	perfLiteral            // number, string without templates, char, boolean, null
	perfInstance           // this / super
	perfName               // simple name reference
	perfQualified          // receiver.name property or field access; kids[0] is the selector
	perfCall               // call; kids are the arguments, recv the receiver for a qualified call
	perfUnary              // prefix or postfix operator; kids[0] is the operand
	perfBinary             // binary operator, string template; kids are the operands
	perfAssign             // assignment or compound assignment; kids[0] is the target
	perfParen              // parenthesized expression; kids[0]
	perfIsCheck            // `is` / `instanceof` check
	perfCast               // `as` / cast; kids[0] is the operand
	perfIndex              // indexed access; kids are the indices, recv the receiver
	perfIf                 // if expression / ternary; kids: condition, then, else (nil when absent)
	perfWhen               // when expression
	perfElvis              // `?:`, which UAST models as an expression list
	perfTry                // kids: try block, finally block (nil when absent)
	perfBlock              // block of statements
	perfReturn             // return; kids[0] is the returned value or nil
	perfLoop               // loop; kids[0] is the body
	perfLambda             // lambda or anonymous function
)

func (e *perfExpr) text(file *scanner.File) string {
	if e == nil || e.start < 0 || e.end > len(file.Content) || e.start >= e.end {
		return ""
	}
	return string(file.Content[e.start:e.end])
}

// perfKotlinBinaryKinds are the Kotlin binary expression node types whose operands are re-associated with a
// preceding prefix operator.
var perfKotlinBinaryKinds = map[string]bool{
	"conjunction_expression": true, "disjunction_expression": true, "equality_expression": true,
	"comparison_expression": true, "additive_expression": true, "multiplicative_expression": true,
	"range_expression": true, "infix_expression": true, "elvis_expression": true, "check_expression": true,
	"as_expression": true,
}

// perfBuild builds the logical view of the expression at idx, or nil for 0.
func perfBuild(file *scanner.File, idx uint32) *perfExpr {
	if idx == 0 {
		return nil
	}
	if file.Language == scanner.LangJava {
		return perfBuildJava(file, idx)
	}
	return perfBuildKotlin(file, idx)
}

func perfNode(file *scanner.File, idx uint32, kind perfKind) *perfExpr {
	return &perfExpr{kind: kind, start: int(file.FlatStartByte(idx)), end: int(file.FlatEndByte(idx)), node: idx}
}

func perfNamedChildren(file *scanner.File, idx uint32) []uint32 {
	var out []uint32
	for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
		if file.FlatIsNamed(c) {
			out = append(out, c)
		}
	}
	return out
}

// perfOperands returns the operand nodes and operator text of a binary node; the unnamed `null` keyword counts as
// an operand.
func perfOperands(file *scanner.File, idx uint32) (operands []*perfExpr, op string) {
	for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
		if file.FlatIsNamed(c) {
			operands = append(operands, perfBuild(file, c))
			continue
		}
		switch t := strings.TrimSpace(file.FlatNodeText(c)); t {
		case "null":
			operands = append(operands, perfNode(file, c, perfLiteral))
		default:
			if op == "" {
				op = t
			}
		}
	}
	return operands, op
}

// perfKotlinBuilders builds the logical view of each Kotlin node type; other binary node types are listed in
// perfKotlinBinaryKinds and everything else is perfOther.
var perfKotlinBuilders map[string]func(*scanner.File, uint32) *perfExpr

func init() {
	leaf := func(kind perfKind) func(*scanner.File, uint32) *perfExpr {
		return func(file *scanner.File, idx uint32) *perfExpr { return perfNode(file, idx, kind) }
	}
	perfKotlinBuilders = map[string]func(*scanner.File, uint32) *perfExpr{
		"this_expression":           leaf(perfInstance),
		"super_expression":          leaf(perfInstance),
		"when_expression":           leaf(perfWhen),
		"lambda_literal":            leaf(perfLambda),
		"anonymous_function":        leaf(perfLambda),
		"annotated_lambda":          leaf(perfLambda),
		"string_literal":            perfKotlinString,
		"line_string_literal":       perfKotlinString,
		"multi_line_string_literal": perfKotlinString,
		"simple_identifier":         perfKotlinName,
		"parenthesized_expression":  func(f *scanner.File, i uint32) *perfExpr { return perfWrapFirst(f, i, perfParen) },
		"as_expression":             func(f *scanner.File, i uint32) *perfExpr { return perfWrapFirst(f, i, perfCast) },
		"prefix_expression":         perfKotlinPrefix,
		"postfix_expression":        perfKotlinPostfix,
		"assignment":                perfKotlinAssignment,
		"navigation_expression":     perfNavigationLike,
		"call_expression":           perfKotlinCall,
		"indexing_expression":       perfKotlinIndexing,
		"check_expression":          perfKotlinCheck,
		"elvis_expression":          perfKotlinElvis,
		"if_expression":             perfKotlinIf,
		"try_expression":            perfKotlinTry,
		"for_statement":             perfKotlinLoop,
		"while_statement":           perfKotlinLoop,
		"do_while_statement":        perfKotlinLoop,
		"jump_expression":           perfKotlinJump,
		"statements":                func(f *scanner.File, i uint32) *perfExpr { return perfBlockOf(f, i, i) },
		"control_structure_body":    perfControlBody,
		"annotated_expression":      perfKotlinUnwrapLast,
		"labeled_expression":        perfKotlinUnwrapLast,
	}
	for _, t := range []string{"integer_literal", "long_literal", "hex_literal", "bin_literal", "real_literal",
		"unsigned_literal", "boolean_literal", "character_literal", "null_literal"} {
		perfKotlinBuilders[t] = leaf(perfLiteral)
	}
}

func perfBuildKotlin(file *scanner.File, idx uint32) *perfExpr {
	t := file.FlatType(idx)
	if build := perfKotlinBuilders[t]; build != nil {
		if e := build(file, idx); e != nil {
			return e
		}
		return perfNode(file, idx, perfOther)
	}
	if perfKotlinBinaryKinds[t] {
		e := perfNode(file, idx, perfBinary)
		e.kids, e.op = perfOperands(file, idx)
		return e
	}
	return perfNode(file, idx, perfOther)
}

func perfKotlinName(file *scanner.File, idx uint32) *perfExpr {
	e := perfNode(file, idx, perfName)
	e.name = file.FlatNodeText(idx)
	return e
}

// perfWrapFirst builds a node of kind whose single child is the first named child.
func perfWrapFirst(file *scanner.File, idx uint32, kind perfKind) *perfExpr {
	e := perfNode(file, idx, kind)
	if kids := perfNamedChildren(file, idx); len(kids) > 0 {
		e.kids = []*perfExpr{perfBuild(file, kids[0])}
	} else {
		e.kids = []*perfExpr{nil}
	}
	return e
}

func perfKotlinPostfix(file *scanner.File, idx uint32) *perfExpr {
	kids := perfNamedChildren(file, idx)
	if len(kids) == 0 {
		return nil
	}
	e := perfNode(file, idx, perfUnary)
	e.kids = []*perfExpr{perfBuild(file, kids[0])}
	e.op = strings.TrimSpace(string(file.Content[file.FlatEndByte(kids[0]):file.FlatEndByte(idx)]))
	return e
}

func perfKotlinAssignment(file *scanner.File, idx uint32) *perfExpr {
	e := perfNode(file, idx, perfAssign)
	for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
		switch {
		case file.FlatType(c) == "directly_assignable_expression":
			if inner := perfNamedChildren(file, c); len(inner) == 1 {
				e.kids = append(e.kids, perfBuild(file, inner[0]))
			} else {
				e.kids = append(e.kids, perfNavigationLike(file, c))
			}
		case file.FlatIsNamed(c):
			e.kids = append(e.kids, perfBuild(file, c))
		default:
			e.op = strings.TrimSpace(file.FlatNodeText(c))
		}
	}
	return e
}

func perfKotlinIndexing(file *scanner.File, idx uint32) *perfExpr {
	e := perfNode(file, idx, perfIndex)
	for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "indexing_suffix" {
			for _, i := range perfNamedChildren(file, c) {
				e.kids = append(e.kids, perfBuild(file, i))
			}
		} else if file.FlatIsNamed(c) && e.recv == nil {
			e.recv = perfBuild(file, c)
		}
	}
	return e
}

// perfKotlinCheck builds `x is T` (a type check) or `x in r` (a binary operator).
func perfKotlinCheck(file *scanner.File, idx uint32) *perfExpr {
	if perfHasToken(file, idx, "is", "!is") {
		return perfNode(file, idx, perfIsCheck)
	}
	e := perfNode(file, idx, perfBinary)
	e.kids, e.op = perfOperands(file, idx)
	return e
}

func perfKotlinElvis(file *scanner.File, idx uint32) *perfExpr {
	e := perfNode(file, idx, perfElvis)
	e.kids, e.op = perfOperands(file, idx)
	return e
}

func perfKotlinTry(file *scanner.File, idx uint32) *perfExpr {
	e := perfNode(file, idx, perfTry)
	var try, finally *perfExpr
	for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "statements":
			if try == nil {
				try = perfBlockOf(file, idx, c)
			}
		case "finally_block":
			if s, ok := file.FlatFindChild(c, "statements"); ok {
				finally = perfBlockOf(file, c, s)
			} else {
				finally = perfNode(file, c, perfBlock)
			}
		}
	}
	if try == nil {
		try = &perfExpr{kind: perfBlock, start: int(file.FlatStartByte(idx)), end: int(file.FlatEndByte(idx))}
	}
	e.kids = []*perfExpr{try, finally}
	return e
}

func perfKotlinLoop(file *scanner.File, idx uint32) *perfExpr {
	e := perfNode(file, idx, perfLoop)
	var body *perfExpr
	if b, ok := file.FlatFindChild(idx, "control_structure_body"); ok {
		body = perfControlBody(file, b)
	}
	e.kids = []*perfExpr{body}
	return e
}

// perfKotlinJump builds `return [value]`; break, continue and throw are perfOther.
func perfKotlinJump(file *scanner.File, idx uint32) *perfExpr {
	if !strings.HasPrefix(file.FlatNodeText(idx), "return") {
		return nil
	}
	e := perfNode(file, idx, perfReturn)
	var value *perfExpr
	for _, c := range perfNamedChildren(file, idx) {
		if file.FlatType(c) != "label" {
			value = perfBuild(file, c)
		}
	}
	e.kids = []*perfExpr{value}
	return e
}

// perfKotlinUnwrapLast sees through annotations and labels to the expression they wrap.
func perfKotlinUnwrapLast(file *scanner.File, idx uint32) *perfExpr {
	kids := perfNamedChildren(file, idx)
	if len(kids) == 0 {
		return nil
	}
	return perfBuild(file, kids[len(kids)-1])
}

func perfHasToken(file *scanner.File, idx uint32, tokens ...string) bool {
	for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
		if file.FlatIsNamed(c) {
			continue
		}
		t := strings.TrimSpace(file.FlatNodeText(c))
		for _, want := range tokens {
			if t == want {
				return true
			}
		}
	}
	return false
}

// perfKotlinPrefix builds a prefix expression, moving the operator onto the leftmost operand of a binary operand.
func perfKotlinPrefix(file *scanner.File, idx uint32) *perfExpr {
	kids := perfNamedChildren(file, idx)
	if len(kids) == 0 {
		return perfNode(file, idx, perfOther)
	}
	operandNode := kids[len(kids)-1]
	// The operator is the first token; a comment between it and the operand is not part of it.
	opText := ""
	for c := file.FlatFirstChild(idx); c != 0 && c != operandNode; c = file.FlatNextSib(c) {
		if !file.FlatIsNamed(c) {
			opText = strings.TrimSpace(file.FlatNodeText(c))
			break
		}
	}
	if opText == "" {
		opText = strings.TrimSpace(string(file.Content[file.FlatStartByte(idx):file.FlatStartByte(operandNode)]))
	}
	switch opText {
	case "!", "-", "+", "++", "--":
	default:
		// Annotations and labels (`@Ann x`, `label@ x`) wrap the operand without changing it.
		return perfBuild(file, operandNode)
	}
	return perfApplyPrefix(int(file.FlatStartByte(idx)), opText, perfBuild(file, operandNode))
}

// perfApplyPrefix applies a prefix operator that starts at start to operand, pushing it down the left spine of
// binary, elvis, `is` and cast expressions.
func perfApplyPrefix(start int, op string, operand *perfExpr) *perfExpr {
	if operand == nil {
		return nil
	}
	switch operand.kind {
	case perfBinary, perfElvis, perfCast:
		if len(operand.kids) > 0 && operand.kids[0] != nil {
			out := *operand
			out.start = start
			out.node = 0
			out.kids = append([]*perfExpr{perfApplyPrefix(start, op, operand.kids[0])}, operand.kids[1:]...)
			return &out
		}
	case perfIsCheck:
		out := *operand
		out.start = start
		out.node = 0
		return &out
	}
	return &perfExpr{kind: perfUnary, start: start, end: operand.end, op: op, kids: []*perfExpr{operand}}
}

// perfNavigationLike builds `receiver.name` (or `receiver?.name`) property access.
func perfNavigationLike(file *scanner.File, idx uint32) *perfExpr {
	kids := perfNamedChildren(file, idx)
	if len(kids) < 2 || file.FlatType(kids[len(kids)-1]) != "navigation_suffix" {
		if len(kids) == 1 && file.FlatType(kids[0]) == "simple_identifier" {
			return perfBuild(file, kids[0])
		}
		return perfNode(file, idx, perfOther)
	}
	e := perfNode(file, idx, perfQualified)
	e.recv = perfBuild(file, kids[0])
	suffix := kids[len(kids)-1]
	sel := &perfExpr{kind: perfOther, start: int(file.FlatStartByte(suffix)), end: int(file.FlatEndByte(suffix))}
	if id, ok := file.FlatFindChild(suffix, "simple_identifier"); ok {
		sel = perfNode(file, id, perfName)
		sel.name = file.FlatNodeText(id)
	}
	e.kids = []*perfExpr{sel}
	e.name = sel.name
	return e
}

// perfKotlinCallParts returns the callee and argument nodes of a Kotlin call, joining a trailing lambda (which
// tree-sitter nests as an outer call) with the call it belongs to.
func perfKotlinCallParts(file *scanner.File, idx uint32) (callee uint32, args []uint32) {
	var suffixes []uint32
	for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "call_suffix":
			suffixes = append(suffixes, c)
		default:
			if file.FlatIsNamed(c) && callee == 0 {
				callee = c
			}
		}
	}
	if callee != 0 && file.FlatType(callee) == "call_expression" && len(suffixes) == 1 && perfLambdaOnlySuffix(file, suffixes[0]) {
		innerCallee, innerArgs := perfKotlinCallParts(file, callee)
		callee, args = innerCallee, innerArgs
	}
	for _, s := range suffixes {
		for c := file.FlatFirstChild(s); c != 0; c = file.FlatNextSib(c) {
			switch file.FlatType(c) {
			case "value_arguments":
				for a := file.FlatFirstChild(c); a != 0; a = file.FlatNextSib(a) {
					if file.FlatType(a) == "value_argument" {
						args = append(args, flatValueArgumentExpression(file, a))
					}
				}
			case "annotated_lambda", "lambda_literal":
				args = append(args, c)
			}
		}
	}
	return callee, args
}

func perfLambdaOnlySuffix(file *scanner.File, suffix uint32) bool {
	has := false
	for c := file.FlatFirstChild(suffix); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "annotated_lambda", "lambda_literal":
			has = true
		case "value_arguments", "type_arguments":
			return false
		}
	}
	return has
}

// perfIsTrailingLambdaInner reports whether idx is the inner call of a `f(args) { lambda }` pair, which the outer
// call already describes.
func perfIsTrailingLambdaInner(file *scanner.File, idx uint32) bool {
	parent, ok := file.FlatParent(idx)
	if !ok || file.FlatType(parent) != "call_expression" || file.FlatFirstChild(parent) != idx {
		return false
	}
	for c := file.FlatFirstChild(parent); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "call_suffix" {
			return perfLambdaOnlySuffix(file, c)
		}
	}
	return false
}

func perfKotlinCall(file *scanner.File, idx uint32) *perfExpr {
	callee, args := perfKotlinCallParts(file, idx)
	e := perfNode(file, idx, perfCall)
	for _, a := range args {
		if a == 0 {
			e.kids = append(e.kids, nil)
			continue
		}
		e.kids = append(e.kids, perfBuild(file, a))
	}
	switch file.FlatType(callee) {
	case "simple_identifier":
		e.name = file.FlatNodeText(callee)
	case "navigation_expression":
		nav := perfNavigationLike(file, callee)
		if nav.kind != perfQualified {
			return perfNode(file, idx, perfOther)
		}
		e.name = nav.name
		e.recv = nav.recv
	default:
		return perfNode(file, idx, perfOther)
	}
	return e
}

// perfCallSelectorText is the call's own source text without its receiver (`mkdirs()` for `file.mkdirs()`), which is
// how Lint quotes a call.
func perfCallSelectorText(file *scanner.File, e *perfExpr) string {
	if e.recv == nil || e.node == 0 {
		return e.text(file)
	}
	callee, _ := perfKotlinCallParts(file, e.node)
	if callee == 0 {
		return e.text(file)
	}
	kids := perfNamedChildren(file, callee)
	if len(kids) == 0 {
		return e.text(file)
	}
	suffix := kids[len(kids)-1]
	start := int(file.FlatStartByte(suffix))
	if id, ok := file.FlatFindChild(suffix, "simple_identifier"); ok {
		start = int(file.FlatStartByte(id))
	}
	return string(file.Content[start:e.end])
}

func perfKotlinString(file *scanner.File, idx uint32) *perfExpr {
	var parts []*perfExpr
	interpolated := false
	for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "interpolated_expression":
			interpolated = true
			if kids := perfNamedChildren(file, c); len(kids) > 0 {
				parts = append(parts, perfBuild(file, kids[0]))
			}
		case "interpolated_identifier":
			interpolated = true
			e := perfNode(file, c, perfName)
			e.name = strings.TrimPrefix(file.FlatNodeText(c), "$")
			parts = append(parts, e)
		case "string_content", "escape_sequence":
			parts = append(parts, perfNode(file, c, perfLiteral))
		}
	}
	if !interpolated {
		return perfNode(file, idx, perfLiteral)
	}
	e := perfNode(file, idx, perfBinary)
	e.op = "+"
	e.kids = parts
	return e
}

func perfKotlinIf(file *scanner.File, idx uint32) *perfExpr {
	e := perfNode(file, idx, perfIf)
	var cond, then, els *perfExpr
	for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
		if !file.FlatIsNamed(c) {
			continue
		}
		switch {
		case cond == nil && file.FlatType(c) != "control_structure_body":
			cond = perfBuild(file, c)
		case file.FlatType(c) == "control_structure_body" && then == nil:
			then = perfControlBody(file, c)
		case file.FlatType(c) == "control_structure_body":
			els = perfControlBody(file, c)
		}
	}
	e.kids = []*perfExpr{cond, then, els}
	return e
}

// perfControlBody builds an if/loop body: a braced body is a block, a bare expression stands for itself.
func perfControlBody(file *scanner.File, body uint32) *perfExpr {
	if perfHasToken(file, body, "{") {
		if s, ok := file.FlatFindChild(body, "statements"); ok {
			return perfBlockOf(file, body, s)
		}
		return perfNode(file, body, perfBlock)
	}
	if kids := perfNamedChildren(file, body); len(kids) > 0 {
		if file.FlatType(kids[0]) == "statements" {
			if inner := perfNamedChildren(file, kids[0]); len(inner) == 1 {
				return perfBuild(file, inner[0])
			}
			return perfBlockOf(file, body, kids[0])
		}
		return perfBuild(file, kids[0])
	}
	return nil
}

// perfBlockOf builds a block spanning owner whose statements are the named children of stmts.
func perfBlockOf(file *scanner.File, owner, stmts uint32) *perfExpr {
	e := perfNode(file, owner, perfBlock)
	for _, s := range perfNamedChildren(file, stmts) {
		e.kids = append(e.kids, perfBuild(file, s))
	}
	return e
}

func perfBuildJava(file *scanner.File, idx uint32) *perfExpr {
	e := perfNode(file, idx, perfOther)
	e.java = true
	switch t := file.FlatType(idx); t {
	case "decimal_integer_literal", "hex_integer_literal", "octal_integer_literal", "binary_integer_literal",
		"decimal_floating_point_literal", "hex_floating_point_literal", "true", "false", "null_literal",
		"character_literal", "string_literal", "text_block":
		e.kind = perfLiteral
	case "this", "super":
		e.kind = perfInstance
	case "identifier":
		e.kind = perfName
		e.name = file.FlatNodeText(idx)
	case "parenthesized_expression":
		e.kind = perfParen
		if kids := perfNamedChildren(file, idx); len(kids) > 0 {
			e.kids = []*perfExpr{perfBuild(file, kids[0])}
		}
	case "update_expression":
		e.kind = perfUnary
		if kids := perfNamedChildren(file, idx); len(kids) > 0 {
			e.kids = []*perfExpr{perfBuild(file, kids[0])}
		}
		if perfHasToken(file, idx, "++") {
			e.op = "++"
		} else {
			e.op = "--"
		}
	case "unary_expression":
		e.kind = perfUnary
		if kids := perfNamedChildren(file, idx); len(kids) > 0 {
			e.kids = []*perfExpr{perfBuild(file, kids[0])}
			e.op = strings.TrimSpace(string(file.Content[file.FlatStartByte(idx):file.FlatStartByte(kids[0])]))
		}
	case "binary_expression":
		e.kind = perfBinary
		e.kids, e.op = perfOperands(file, idx)
	case "assignment_expression":
		e.kind = perfAssign
		e.kids, e.op = perfOperands(file, idx)
	case "ternary_expression":
		e.kind = perfIf
		kids := perfNamedChildren(file, idx)
		for i := 0; i < 3; i++ {
			if i < len(kids) {
				e.kids = append(e.kids, perfBuild(file, kids[i]))
			} else {
				e.kids = append(e.kids, nil)
			}
		}
	case "instanceof_expression":
		e.kind = perfIsCheck
	case "cast_expression":
		e.kind = perfCast
		if kids := perfNamedChildren(file, idx); len(kids) > 0 {
			e.kids = []*perfExpr{perfBuild(file, kids[len(kids)-1])}
		}
	case "method_invocation", "object_creation_expression":
		e.kind = perfCall
	case "field_access":
		e.kind = perfQualified
	case "array_access":
		e.kind = perfIndex
	case "lambda_expression":
		e.kind = perfLambda
	}
	return e
}

// perfEnclosingFunctions returns the same-file functions named name that an unqualified call at idx can reach,
// nearest scope first: local functions of enclosing callables, then members (and companion members) of enclosing
// classes, then top-level functions. Only the first non-empty scope is returned, as Kotlin resolves there.
func perfEnclosingFunctions(file *scanner.File, idx uint32, name string, membersOnly bool) []uint32 {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		var found []uint32
		switch file.FlatType(cur) {
		case "statements", "function_body":
			if membersOnly {
				continue
			}
			found = perfDirectFunctions(file, cur, name)
		case "class_body", "enum_class_body":
			found = perfDirectFunctions(file, cur, name)
			for c := file.FlatFirstChild(cur); c != 0 && len(found) == 0; c = file.FlatNextSib(c) {
				if file.FlatType(c) == "companion_object" {
					if body, ok := file.FlatFindChild(c, "class_body"); ok {
						found = perfDirectFunctions(file, body, name)
					}
				}
			}
			if membersOnly && len(found) == 0 {
				return nil
			}
		case "source_file":
			if !membersOnly {
				found = perfDirectFunctions(file, cur, name)
			}
		}
		if len(found) > 0 {
			return found
		}
	}
	return nil
}

func perfDirectFunctions(file *scanner.File, container uint32, name string) []uint32 {
	var out []uint32
	for c := file.FlatFirstChild(container); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) != "function_declaration" {
			continue
		}
		if id, ok := file.FlatFindChild(c, "simple_identifier"); ok && file.FlatNodeText(id) == name {
			out = append(out, c)
		}
	}
	return out
}

// perfResolveFunction picks the one candidate that a call with argCount arguments can target, counting parameters
// with default values as optional, or 0 when none or several are applicable (the argument types would decide).
func perfResolveFunction(file *scanner.File, candidates []uint32, argCount int) uint32 {
	var match uint32
	for _, c := range candidates {
		params, _ := file.FlatFindChild(c, "function_value_parameters")
		total, required, vararg := 0, 0, false
		pendingDefault := false
		for p := file.FlatFirstChild(params); params != 0 && p != 0; p = file.FlatNextSib(p) {
			switch {
			case file.FlatType(p) == "parameter":
				total++
				required++
				pendingDefault = true
			case file.FlatType(p) == "parameter_modifiers" || strings.Contains(file.FlatNodeText(p), "vararg"):
				vararg = vararg || strings.Contains(file.FlatNodeText(p), "vararg")
			case !file.FlatIsNamed(p) && strings.TrimSpace(file.FlatNodeText(p)) == "=" && pendingDefault:
				required--
				pendingDefault = false
			case strings.TrimSpace(file.FlatNodeText(p)) == ",":
				pendingDefault = false
			}
		}
		if argCount < required || argCount > total && !vararg {
			continue
		}
		if match != 0 {
			return 0
		}
		match = c
	}
	return match
}

// perfFunctionBody returns the function's body as a block, with an expression body wrapped in an implicit return the
// way UAST presents it; nil when the function has no body.
func perfFunctionBody(file *scanner.File, fn uint32) *perfExpr {
	body, ok := file.FlatFindChild(fn, "function_body")
	if !ok {
		return nil
	}
	if perfHasToken(file, body, "=") {
		kids := perfNamedChildren(file, body)
		if len(kids) == 0 {
			return nil
		}
		value := perfBuild(file, kids[len(kids)-1])
		ret := &perfExpr{kind: perfReturn, start: value.start, end: value.end, kids: []*perfExpr{value}}
		return &perfExpr{kind: perfBlock, start: int(file.FlatStartByte(body)), end: int(file.FlatEndByte(body)), kids: []*perfExpr{ret}}
	}
	if s, ok := file.FlatFindChild(body, "statements"); ok {
		return perfBlockOf(file, body, s)
	}
	return perfNode(file, body, perfBlock)
}

// perfIsLocalName reports whether e names a local variable or parameter of the callable or lambda around it.
func perfIsLocalName(file *scanner.File, e *perfExpr) bool {
	if e == nil || e.kind != perfName || e.node == 0 {
		return false
	}
	if lintNameIsParameterAt(file, e.node, e.name) {
		return true
	}
	fn, ok := flatEnclosingCallable(file, e.node)
	if !ok {
		return false
	}
	local := false
	file.FlatWalkNodes(fn, "property_declaration", func(decl uint32) {
		if file.FlatStartByte(decl) >= file.FlatStartByte(e.node) {
			return
		}
		vd, ok := file.FlatFindChild(decl, "variable_declaration")
		if !ok || lintDeclaredName(file, vd) != e.name {
			return
		}
		// The declaration's block must enclose the use; a local of an earlier, closed block is out of scope.
		if block, ok := file.FlatParent(decl); ok && lintNodeContains(file, block, e.node) {
			local = true
		}
	})
	return local
}
