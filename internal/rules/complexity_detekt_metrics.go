package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// The functions in this file port the metrics of detekt 1.23.8 (detekt-metrics and the complexity rules) over
// the tree-sitter tree, quirks included.

// detektNestingFunctions are the scope functions detekt's NestedBlockDepth treats as block nesting.
var detektNestingFunctions = map[string]bool{
	"apply": true, "forEach": true, "let": true, "run": true, "use": true, "with": true,
}

// isPsiBinaryExpression reports a node that is a KtBinaryExpression in the Kotlin PSI: every infix operator
// including `in`, elvis, ranges and assignments, but not `is`, `as` or `?.`.
func isPsiBinaryExpression(file *scanner.File, n uint32) bool {
	switch file.FlatType(n) {
	case "conjunction_expression", "disjunction_expression", "comparison_expression", "equality_expression",
		"additive_expression", "multiplicative_expression", "infix_expression", "elvis_expression",
		"range_expression", "assignment":
		return true
	case "check_expression":
		for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
			switch file.FlatType(c) {
			case "in", "!in":
				return true
			}
		}
	}
	return false
}

// callTrailingLambda returns the trailing lambda of a call_expression, or 0.
func callTrailingLambda(file *scanner.File, call uint32) uint32 {
	suffix, ok := file.FlatFindChild(call, "call_suffix")
	if !ok {
		return 0
	}
	for c := file.FlatFirstChild(suffix); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "annotated_lambda":
			if l, ok := file.FlatFindChild(c, "lambda_literal"); ok {
				return l
			}
			return c
		case "lambda_literal":
			return c
		}
	}
	return 0
}

// callValueArgumentCount counts the arguments in parentheses and the trailing lambda of a call.
func callValueArgumentCount(file *scanner.File, call uint32) (parens, lambdas int) {
	suffix, ok := file.FlatFindChild(call, "call_suffix")
	if !ok {
		return 0, 0
	}
	for c := file.FlatFirstChild(suffix); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "value_arguments":
			for a := file.FlatFirstChild(c); a != 0; a = file.FlatNextSib(a) {
				if file.FlatType(a) == "value_argument" {
					parens++
				}
			}
		case "annotated_lambda", "lambda_literal":
			lambdas++
		}
	}
	return parens, lambdas
}

// isInsideObjectLiteral reports a declaration nested in an `object : T { ... }` expression.
func isInsideObjectLiteral(file *scanner.File, n uint32) bool {
	for p, ok := file.FlatParent(n); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "object_literal" {
			return true
		}
	}
	return false
}

// ifBodyRole classifies a control_structure_body that is a branch of an if_expression: "then", "else", or ""
// for any other body.
func ifBodyRole(file *scanner.File, body uint32) string {
	if file.FlatType(body) != "control_structure_body" {
		return ""
	}
	p, ok := file.FlatParent(body)
	if !ok || file.FlatType(p) != "if_expression" {
		return ""
	}
	sawElse := false
	for c := file.FlatFirstChild(p); c != 0; c = file.FlatNextSib(c) {
		if c == body {
			if sawElse {
				return "else"
			}
			return "then"
		}
		if file.FlatType(c) == "else" {
			sawElse = true
		}
	}
	return ""
}

// ifIsElseIf reports an if_expression that is the whole else branch of another if.
func ifIsElseIf(file *scanner.File, ifExpr uint32) bool {
	p, ok := file.FlatParent(ifExpr)
	return ok && ifBodyRole(file, p) == "else"
}

// splitPsiCall splits a tree-sitter call_expression into the parts the Kotlin PSI keeps apart: the receiver
// expressions (separate qualified expressions in the PSI), the callee name, and the argument and trailing
// lambda suffixes (the single KtCallExpression). `f(x) { }` is one PSI call spread over two nested nodes.
func splitPsiCall(file *scanner.File, call uint32) (receivers, suffixes []uint32, name string) {
	cur := call
	for {
		for c := file.FlatFirstChild(cur); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) == "call_suffix" {
				suffixes = append(suffixes, c)
			}
		}
		first := file.FlatFirstChild(cur)
		switch file.FlatType(first) {
		case "call_expression":
			cur = first
			continue
		case "navigation_expression":
			receivers = append(receivers, file.FlatFirstChild(first))
			for c := file.FlatFirstChild(first); c != 0; c = file.FlatNextSib(c) {
				if file.FlatType(c) == "navigation_suffix" {
					for g := file.FlatFirstChild(c); g != 0; g = file.FlatNextSib(g) {
						if file.FlatType(g) == "simple_identifier" {
							name = file.FlatNodeText(g)
						}
					}
				}
			}
		case "simple_identifier":
			name = file.FlatNodeText(first)
		default:
			receivers = append(receivers, first)
		}
		return receivers, suffixes, name
	}
}

// hasTrailingLambdaSuffix reports a call_suffix that holds a lambda.
func hasTrailingLambdaSuffix(file *scanner.File, suffix uint32) bool {
	for c := file.FlatFirstChild(suffix); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "annotated_lambda", "lambda_literal":
			return true
		}
	}
	return false
}

// detektNestedBlockDepth returns the deepest nesting that detekt's NestedBlockDepth visitor reaches inside fn.
// As in detekt, the visitor does not descend into the arguments and lambda of a call that is not one of the
// nesting functions, so control flow inside such a lambda is not counted.
func detektNestedBlockDepth(file *scanner.File, fn uint32) int {
	depth, maxDepth := 0, 0
	var walk func(n uint32)
	walkChildren := func(n uint32) {
		for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
			walk(c)
		}
	}
	nested := func(walkBody func()) {
		depth++
		if depth > maxDepth {
			maxDepth = depth
		}
		walkBody()
		depth--
	}
	walk = func(n uint32) {
		switch file.FlatType(n) {
		case "if_expression":
			// An if that is the direct body of another if or loop (`else if`) does not nest.
			if p, ok := file.FlatParent(n); ok && file.FlatType(p) == "control_structure_body" {
				if gp, ok := file.FlatParent(p); ok {
					switch file.FlatType(gp) {
					case "if_expression", "for_statement", "while_statement", "do_while_statement":
						walkChildren(n)
						return
					}
				}
			}
			nested(func() { walkChildren(n) })
		case "for_statement", "while_statement", "do_while_statement", "when_expression", "try_expression":
			nested(func() { walkChildren(n) })
		case "call_expression":
			receivers, suffixes, name := splitPsiCall(file, n)
			for _, r := range receivers {
				walk(r)
			}
			if !detektNestingFunctions[name] {
				return
			}
			lambda := false
			for _, s := range suffixes {
				lambda = lambda || hasTrailingLambdaSuffix(file, s)
			}
			visit := func() {
				for _, s := range suffixes {
					walk(s)
				}
			}
			if lambda {
				nested(visit)
			} else {
				visit()
			}
		default:
			walkChildren(n)
		}
	}
	walkChildren(fn)
	return maxDepth
}

// cyclomaticOptions are the CyclomaticComplexMethod settings that change how decisions are counted.
type cyclomaticOptions struct {
	ignoreSimpleWhenEntries bool
	ignoreNestingFunctions  bool
	nestingFunctions        map[string]bool
}

var defaultNestingFunctionSet = map[string]bool{
	"also": true, "apply": true, "forEach": true, "isNotNull": true, "ifNull": true,
	"let": true, "run": true, "use": true, "with": true,
}

func defaultCyclomaticOptions() cyclomaticOptions {
	return cyclomaticOptions{nestingFunctions: defaultNestingFunctionSet}
}

// detektCyclomaticComplexity is detekt's CyclomaticComplexity of fn: one per function (nested ones included,
// except in object literals), per branch, loop, `when` entry (the `else` entry too), catch clause, `&&`, `||`,
// `?:`, `break` and `continue`, and per nesting-function call that has a lambda.
func detektCyclomaticComplexity(file *scanner.File, fn uint32, opts cyclomaticOptions) int {
	total := 0
	var walk func(n uint32)
	walk = func(n uint32) {
		switch file.FlatType(n) {
		case "function_declaration", "anonymous_function":
			if isInsideObjectLiteral(file, n) {
				return
			}
			total++
		case "conjunction_expression", "disjunction_expression", "elvis_expression",
			"if_expression", "for_statement", "while_statement", "do_while_statement":
			total++
		case "jump_expression":
			if first := file.FlatFirstChild(n); first != 0 {
				switch file.FlatType(first) {
				case "break", "break@", "continue", "continue@":
					total++
				}
			}
		case "when_expression":
			total += whenCyclomaticCountFlat(file, n, opts.ignoreSimpleWhenEntries)
		case "try_expression":
			for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
				if file.FlatType(c) == "catch_block" {
					total++
				}
			}
		case "call_expression":
			if !opts.ignoreNestingFunctions && opts.nestingFunctions[extractCallNameFlat(file, n)] && callTrailingLambda(file, n) != 0 {
				total++
			}
		}
		for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
			walk(c)
		}
	}
	walk(fn)
	return total
}

// whenEntryHasBlockBody reports a `when` entry whose body is a braced block, which is what detekt calls a
// non-simple entry.
func whenEntryHasBlockBody(file *scanner.File, entry uint32) bool {
	for c := file.FlatFirstChild(entry); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "control_structure_body" {
			first := file.FlatFirstChild(c)
			return first != 0 && file.FlatType(first) == "{"
		}
	}
	return false
}

// whenCyclomaticCountFlat is detekt's contribution of one `when`: every entry (the `else` entry included),
// only the braced ones when simple entries are ignored, and 1 when that leaves none.
func whenCyclomaticCountFlat(file *scanner.File, when uint32, ignoreSimpleWhenEntries bool) int {
	counted := 0
	for entry := file.FlatFirstChild(when); entry != 0; entry = file.FlatNextSib(entry) {
		if file.FlatType(entry) != "when_entry" {
			continue
		}
		if ignoreSimpleWhenEntries && !whenEntryHasBlockBody(file, entry) {
			continue
		}
		counted++
	}
	// An entry's body followed by a line that starts with `(` parses as a call, which turns the next
	// entries into nested ERROR nodes holding their `->`. Each one is an entry of unknown body.
	if !ignoreSimpleWhenEntries {
		counted += lostWhenEntriesFlat(file, when)
	}
	if ignoreSimpleWhenEntries && counted == 0 {
		return 1
	}
	return counted
}

// detektCognitiveComplexity is detekt's CognitiveComplexity of fn.
func detektCognitiveComplexity(file *scanner.File, fn uint32) int {
	complexity, nesting := 0, 0
	fnName := extractIdentifierFlat(file, fn)
	paramCount := 0
	if params, ok := file.FlatFindChild(fn, "function_value_parameters"); ok {
		for c := file.FlatFirstChild(params); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) == "parameter" {
				paramCount++
			}
		}
	}
	add := func() { complexity += 1 + nesting }
	var walk func(n uint32)
	walkChildren := func(n uint32) {
		for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
			walk(c)
		}
	}
	nest := func(n uint32) {
		nesting++
		walkChildren(n)
		nesting--
	}
	logical := func(n uint32) {
		// The outermost binary expression scores every change of &&/|| across all its descendants.
		var ops []uint32
		var collect func(x uint32)
		collect = func(x uint32) {
			if t := file.FlatType(x); t == "conjunction_expression" || t == "disjunction_expression" {
				ops = append(ops, x)
			}
			for c := file.FlatFirstChild(x); c != 0; c = file.FlatNextSib(c) {
				collect(c)
			}
		}
		collect(n)
		// Stable order by start offset, parents before children on ties (the visit order).
		for i := 1; i < len(ops); i++ {
			for j := i; j > 0 && file.FlatStartByte(ops[j]) < file.FlatStartByte(ops[j-1]); j-- {
				ops[j], ops[j-1] = ops[j-1], ops[j]
			}
		}
		prev := ""
		for i, op := range ops {
			t := file.FlatType(op)
			enclosed := false
			if p, ok := file.FlatParent(op); ok && file.FlatType(p) == "parenthesized_expression" {
				enclosed = true
			}
			if i == 0 || t != prev || enclosed {
				complexity++
			}
			prev = t
		}
	}
	inBinary := false
	walk = func(n uint32) {
		t := file.FlatType(n)
		if isPsiBinaryExpression(file, n) && !inBinary {
			inBinary = true
			logical(n)
			walkChildren(n)
			inBinary = false
			return
		}
		switch t {
		case "when_expression", "for_statement", "while_statement", "do_while_statement", "catch_block":
			add()
			nest(n)
		case "control_structure_body":
			switch ifBodyRole(file, n) {
			case "then":
				p, _ := file.FlatParent(n)
				if ifIsElseIf(file, p) {
					complexity++
				} else {
					add()
				}
				nest(n)
			case "else":
				if inner, ok := file.FlatFindChild(n, "if_expression"); ok && onlyNamedChild(file, n, inner) {
					walkChildren(n)
				} else {
					complexity++
					nest(n)
				}
			default:
				walkChildren(n)
			}
		case "lambda_literal":
			nest(n)
		case "function_declaration", "anonymous_function":
			if n == fn {
				walkChildren(n)
			} else {
				nest(n)
			}
		case "jump_expression":
			if first := file.FlatFirstChild(n); first != 0 && strings.HasSuffix(file.FlatType(first), "@") && file.FlatType(first) != "throw" {
				complexity++
			}
			walkChildren(n)
		case "call_expression":
			if isRecursiveCall(file, n, fnName, paramCount) {
				complexity++
			}
			walkChildren(n)
		default:
			walkChildren(n)
		}
	}
	walkChildren(fn)
	return complexity
}

// onlyNamedChild reports that child is the only named node in parent, so the body is exactly that node.
func onlyNamedChild(file *scanner.File, parent, child uint32) bool {
	for c := file.FlatFirstChild(parent); c != 0; c = file.FlatNextSib(c) {
		if c != child && file.FlatIsNamed(c) {
			return false
		}
	}
	return true
}

// isRecursiveCall reports an unqualified (or `this.`) call to the function being measured with as many
// arguments as it has parameters. As in detekt, a trailing lambda counts twice.
func isRecursiveCall(file *scanner.File, call uint32, fnName string, paramCount int) bool {
	if fnName == "" {
		return false
	}
	parens, lambdas := callValueArgumentCount(file, call)
	if parens+2*lambdas != paramCount {
		return false
	}
	callee := file.FlatFirstChild(call)
	switch file.FlatType(callee) {
	case "simple_identifier":
		// A call that is the receiver of `?.` or `.` has a qualified parent in the PSI, which is not recursion.
		if p, ok := file.FlatParent(call); ok && file.FlatType(p) == "navigation_expression" {
			return false
		}
		return file.FlatNodeText(callee) == fnName
	case "navigation_expression":
		recv := file.FlatFirstChild(callee)
		return file.FlatType(recv) == "this_expression" && extractCallNameFlat(file, call) == fnName
	}
	return false
}

// hasEnclosingFunction reports a node inside the body of a function. Several detekt rules do not visit those
// nodes because their function visitors never descend into the function they handle.
func hasEnclosingFunction(file *scanner.File, n uint32) bool {
	for p, ok := file.FlatParent(n); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "function_declaration", "anonymous_function":
			return true
		}
	}
	return false
}

// conditionExpressionOf returns the parenthesised condition of an if, while or do-while expression.
func conditionExpressionOf(file *scanner.File, n uint32) uint32 {
	open := false
	for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
		t := file.FlatType(c)
		if t == "(" {
			open = true
			continue
		}
		if open && file.FlatIsNamed(c) {
			return c
		}
		if t == "control_structure_body" && file.FlatType(n) != "do_while_statement" {
			break
		}
	}
	return 0
}

// calleeNameNode returns the identifier that names the callee of a call_expression: the last name of a
// qualified callee, or the call itself when it has none.
func calleeNameNode(file *scanner.File, call uint32) uint32 {
	cur := call
	for file.FlatType(file.FlatFirstChild(cur)) == "call_expression" {
		cur = file.FlatFirstChild(cur)
	}
	first := file.FlatFirstChild(cur)
	switch file.FlatType(first) {
	case "simple_identifier":
		return first
	case "navigation_expression":
		var last uint32
		for c := file.FlatFirstChild(first); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) == "navigation_suffix" {
				for g := file.FlatFirstChild(c); g != 0; g = file.FlatNextSib(g) {
					if file.FlatType(g) == "simple_identifier" {
						last = g
					}
				}
			}
		}
		if last != 0 {
			return last
		}
	}
	return call
}

// hasAncestorOfType reports whether any ancestor of n has the given node type.
func hasAncestorOfType(file *scanner.File, n uint32, typ string) bool {
	for p, ok := file.FlatParent(n); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == typ {
			return true
		}
	}
	return false
}
