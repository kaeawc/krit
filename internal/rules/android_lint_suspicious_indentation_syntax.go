package rules

import (
	"strings"
)

// siSkippable reports the children of a statement list that are not statements.
func (s *siScan) siSkippable(n uint32) bool {
	switch s.file.FlatType(n) {
	case "line_comment", "multiline_comment", "block_comment", "comment", ";", "{", "}", "ERROR":
		return true
	}
	return false
}

func (s *siScan) startOf(n uint32) int { return int(s.file.FlatStartByte(n)) }
func (s *siScan) endOf(n uint32) int   { return int(s.file.FlatEndByte(n)) }

// children lists the nodes of a statement list that are statements.
func (s *siScan) children(block uint32) (out []uint32, hasError bool) {
	for c := s.file.FlatFirstChild(block); c != 0; c = s.file.FlatNextSib(c) {
		if s.file.FlatType(c) == "ERROR" {
			return nil, true
		}
		if !s.siSkippable(c) {
			out = append(out, c)
		}
	}
	return out, false
}

// namedChildren lists the non-comment named children of a node.
func (s *siScan) namedChildren(n uint32) []uint32 {
	var out []uint32
	for c := s.file.FlatFirstChild(n); c != 0; c = s.file.FlatNextSib(c) {
		if s.file.FlatIsNamed(c) && !s.siSkippable(c) {
			out = append(out, c)
		}
	}
	return out
}

// kotlinStatements lists the statements of a Kotlin `statements` node.
func (s *siScan) kotlinStatements(block uint32) []siStmt {
	nodes, bad := s.children(block)
	if bad {
		return nil
	}
	out := make([]siStmt, 0, len(nodes))
	for i := 0; i < len(nodes); i++ {
		st := s.kotlinStmt(nodes[i])
		// The grammar leaves the newline-separated body of a for loop as the next statement.
		if s.file.FlatType(nodes[i]) == "for_statement" && !s.file.FlatHasChildOfType(nodes[i], "control_structure_body") && i+1 < len(nodes) {
			next := nodes[i+1]
			st.end, st.innerEnd = s.endOf(next), s.endOf(next)
			st.body = s.kotlinBody(next)
			i++
		}
		out = append(out, st)
	}
	return out
}

func (s *siScan) kotlinStmt(n uint32) siStmt {
	inner := s.unparenthesized(n)
	st := siStmt{node: n, start: s.startOf(n), end: s.endOf(n), innerEnd: s.endOf(inner), kind: s.kotlinKind(inner)}
	st.curBenign = s.kotlinBenignAsCurrent(n, st.kind)
	switch s.file.FlatType(n) {
	case "if_expression", "for_statement", "while_statement", "do_while_statement":
		st.body = s.kotlinControlBody(n)
	}
	return st
}

// unparenthesized strips parentheses, as skipParenthesizedExprDown does.
func (s *siScan) unparenthesized(n uint32) uint32 {
	for s.file.FlatType(n) == "parenthesized_expression" {
		kids := s.namedChildren(n)
		if len(kids) != 1 {
			return n
		}
		n = kids[0]
	}
	return n
}

func (s *siScan) kotlinKind(n uint32) siKind {
	switch s.file.FlatType(n) {
	case "if_expression":
		return siIf
	case "for_statement", "while_statement":
		return siLoop
	case "do_while_statement":
		return siDoWhile
	case "property_declaration", "function_declaration", "class_declaration", "object_declaration", "typealias_declaration":
		return siDecl
	case "assignment":
		return siAssign
	case "additive_expression", "multiplicative_expression", "comparison_expression", "equality_expression",
		"conjunction_expression", "disjunction_expression", "range_expression", "infix_expression":
		return siBinary
	case "check_expression":
		if s.hasToken(n, "in", "!in") {
			return siBinary
		}
	case "jump_expression":
		return s.kotlinJumpKind(n)
	}
	return siOther
}

// hasToken reports a direct anonymous child whose type is one of the given operators.
func (s *siScan) hasToken(n uint32, ops ...string) bool {
	for c := s.file.FlatFirstChild(n); c != 0; c = s.file.FlatNextSib(c) {
		t := s.file.FlatType(c)
		for _, op := range ops {
			if t == op {
				return true
			}
		}
	}
	return false
}

func (s *siScan) kotlinJumpKind(n uint32) siKind {
	text := s.file.FlatNodeText(n)
	switch {
	case strings.HasPrefix(text, "return"):
		return siJump
	case strings.HasPrefix(text, "break"), strings.HasPrefix(text, "continue"):
		return siBreakish
	}
	return siOther
}

// kotlinBenignAsCurrent marks statements whose Lint expression type is null or an assignment-like statement.
func (s *siScan) kotlinBenignAsCurrent(n uint32, k siKind) bool {
	switch k {
	case siAssign, siDecl, siLoop, siDoWhile, siBreakish:
		return true
	}
	if s.file.FlatType(n) == "prefix_expression" {
		first := s.file.FlatFirstChild(n)
		t := s.file.FlatType(first)
		return t == "++" || t == "--" || t == "annotation"
	}
	return false
}

// kotlinControlBody finds the unbraced statement an if or loop executes.
func (s *siScan) kotlinControlBody(n uint32) siBody {
	var then, otherwise uint32
	sawElse := false
	for c := s.file.FlatFirstChild(n); c != 0; c = s.file.FlatNextSib(c) {
		switch s.file.FlatType(c) {
		case "else":
			sawElse = true
		case "control_structure_body":
			if sawElse {
				otherwise = c
			} else if then == 0 {
				then = c
			}
		}
	}
	cb := then
	if otherwise != 0 {
		cb = otherwise
	}
	if cb == 0 {
		return siBody{}
	}
	for c := s.file.FlatFirstChild(cb); c != 0; c = s.file.FlatNextSib(c) {
		if s.file.FlatType(c) == "{" {
			return siBody{}
		}
		if !s.siSkippable(c) {
			return s.kotlinBody(c)
		}
	}
	return siBody{}
}

// kotlinBody describes a body statement.
func (s *siScan) kotlinBody(n uint32) siBody {
	return siBody{present: true, node: n, start: s.startOf(n), jump: s.kotlinKind(n) == siJump || s.kotlinKind(n) == siBreakish,
		isIf: s.file.FlatType(n) == "if_expression"}
}

// javaStatements lists the statements of a Java block.
func (s *siScan) javaStatements(block uint32) []siStmt {
	nodes, bad := s.children(block)
	if bad {
		return nil
	}
	out := make([]siStmt, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, s.javaStmt(n))
	}
	return out
}

func (s *siScan) javaStmt(n uint32) siStmt {
	st := siStmt{node: n, start: s.startOf(n), end: s.endOf(n), innerEnd: s.endOf(n), kind: siOther, curBenign: true}
	switch s.file.FlatType(n) {
	case "expression_statement":
		s.javaExpressionStmt(&st, n)
	case "local_variable_declaration", "class_declaration", "interface_declaration", "enum_declaration",
		"record_declaration", "empty_statement":
		st.noSource, st.kind = true, siDecl
	case "if_statement":
		st.kind, st.body = siIf, s.javaIfBody(n)
	case "for_statement", "enhanced_for_statement", "while_statement":
		st.kind, st.body = siLoop, s.javaBody(s.lastStatement(n))
	case "do_statement":
		st.kind, st.body = siDoWhile, s.javaBody(s.doBody(n))
	case "return_statement":
		st.kind = siJump
	case "break_statement", "continue_statement":
		st.kind = siBreakish
	}
	return st
}

// javaExpressionStmt classifies an expression statement; Lint's UAST drops the trailing semicolon.
func (s *siScan) javaExpressionStmt(st *siStmt, n uint32) {
	kids := s.namedChildren(n)
	if len(kids) == 0 {
		return
	}
	expr := kids[0]
	st.end, st.innerEnd = s.endOf(expr), s.endOf(expr)
	switch s.file.FlatType(expr) {
	case "assignment_expression":
		st.kind = siAssign
	case "update_expression":
		first := s.file.FlatType(s.file.FlatFirstChild(expr))
		st.curBenign = first == "++" || first == "--"
	case "object_creation_expression", "field_access":
		st.curBenign = false
	case "method_invocation":
		st.javaCall = expr
	}
}

// javaCallReturnsValue reports an unqualified call whose same-file declarations all return a value. Other calls
// have return types the syntax tree cannot give, and Lint ignores void ones.
func (s *siScan) javaCallReturnsValue(call uint32) bool {
	first := s.file.FlatFirstChild(call)
	if s.file.FlatType(first) != "identifier" {
		return false
	}
	name := s.file.FlatNodeText(first)
	found, allValues := false, true
	s.file.FlatWalkNodes(0, "method_declaration", func(decl uint32) {
		if id, ok := s.file.FlatFindChild(decl, "identifier"); !ok || s.file.FlatNodeText(id) != name {
			return
		}
		found = true
		if _, void := s.file.FlatFindChild(decl, "void_type"); void {
			allValues = false
		}
	})
	return found && allValues
}

func (s *siScan) javaIfBody(n uint32) siBody {
	var then, otherwise uint32
	sawElse := false
	for c := s.file.FlatFirstChild(n); c != 0; c = s.file.FlatNextSib(c) {
		t := s.file.FlatType(c)
		switch {
		case t == "else":
			sawElse = true
		case t == "if" || t == "parenthesized_expression" || s.siSkippable(c):
		case sawElse:
			otherwise = c
		default:
			then = c
		}
	}
	if otherwise != 0 {
		return s.javaBody(otherwise)
	}
	return s.javaBody(then)
}

// lastStatement returns the last non-token child, the body of a Java loop.
func (s *siScan) lastStatement(n uint32) uint32 {
	var last uint32
	for c := s.file.FlatFirstChild(n); c != 0; c = s.file.FlatNextSib(c) {
		if s.file.FlatIsNamed(c) && !s.siSkippable(c) {
			last = c
		}
	}
	return last
}

// doBody returns the statement between `do` and `while`.
func (s *siScan) doBody(n uint32) uint32 {
	sawDo := false
	for c := s.file.FlatFirstChild(n); c != 0; c = s.file.FlatNextSib(c) {
		switch {
		case s.file.FlatType(c) == "do":
			sawDo = true
		case sawDo && s.file.FlatIsNamed(c) && !s.siSkippable(c):
			return c
		}
	}
	return 0
}

// javaBody describes an unbraced Java body statement.
func (s *siScan) javaBody(n uint32) siBody {
	if n == 0 {
		return siBody{}
	}
	switch s.file.FlatType(n) {
	case "block", "empty_statement":
		return siBody{}
	}
	t := s.file.FlatType(n)
	return siBody{present: true, node: n, start: s.startOf(n), jump: t == "return_statement" || t == "break_statement" || t == "continue_statement",
		isIf: t == "if_statement"}
}
