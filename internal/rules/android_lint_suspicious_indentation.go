package rules

import (
	"bytes"
	"strings"
	"unicode"
	"unicode/utf16"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// SuspiciousIndentationRule mirrors Lint's SuspiciousIndentation: a statement indented as if it belonged to the
// previous statement or to a control statement without braces, or indented with a different whitespace string than
// the line before it. Lint reports at most one problem per block.
type SuspiciousIndentationRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: Lint reads expression types that the syntax tree lacks.
func (r *SuspiciousIndentationRule) Confidence() float64 { return api.ConfidenceMedium }

const (
	siMsgConditional = "Suspicious indentation: This is conditionally executed; expected it to be indented"
	siNotNested      = "nested under"
	siNotContinuing  = "continuing"
)

type siKind uint8

const (
	siOther    siKind = iota
	siIf              // if with or without else
	siLoop            // for and while loops
	siDoWhile         // do-while loops, which Lint does not treat as control statements
	siDecl            // local declarations
	siAssign          // assignments
	siBinary          // binary operations
	siJump            // return, break and continue
	siBreakish        // break and continue, which Lint gives no expression type
)

// siBody describes the unbraced statement a control statement executes.
type siBody struct {
	present bool
	node    uint32
	start   int
	jump    bool
	isIf    bool
}

// siStmt is one statement of a block as Lint's UAST sees it.
type siStmt struct {
	node     uint32
	start    int
	end      int // end of the statement text
	innerEnd int // end of the expression text once parentheses are removed
	kind     siKind
	// curBenign marks statements whose Lint expression type is null, void or an error type, so they never
	// count as suspicious after a declaration or an assignment.
	curBenign bool
	// javaCall is a Java method call whose return type decides whether the statement is suspicious.
	javaCall uint32
	// noSource marks Java declarations, which Lint's UAST gives no source element.
	noSource bool
	body     siBody
}

// siScan holds the state of one block analysis.
type siScan struct {
	file    *scanner.File
	content []byte
	java    bool
}

func (r *SuspiciousIndentationRule) check(ctx *api.Context) {
	file := ctx.File
	s := &siScan{file: file, content: file.Content, java: file.Language == scanner.LangJava}
	var list []siStmt
	switch {
	case s.java && file.FlatType(ctx.Idx) != "statements":
		list = s.javaStatements(ctx.Idx)
	case !s.java && file.FlatType(ctx.Idx) == "statements":
		if s.isBlockExpression(ctx.Idx) {
			list = s.kotlinStatements(ctx.Idx)
		}
	}
	if list == nil {
		return
	}
	s.run(ctx, list)
}

// isBlockExpression reports Kotlin statement lists that Lint visits as a block expression. Script top levels, lambda
// bodies and braced `when` branches are other UAST nodes.
func (s *siScan) isBlockExpression(statements uint32) bool {
	parent, ok := s.file.FlatParent(statements)
	if !ok {
		return false
	}
	switch s.file.FlatType(parent) {
	case "source_file", "lambda_literal":
		return false
	case "control_structure_body":
		grand, ok := s.file.FlatParent(parent)
		return ok && s.file.FlatType(grand) != "when_entry"
	}
	return true
}

// run walks the statements of a block in order, as Lint's visitBlockExpression does.
func (s *siScan) run(ctx *api.Context, list []siStmt) {
	prevIndent, prevLineBegin := -1, -1
	for i := range list {
		st := &list[i]
		if st.noSource {
			prevIndent, prevLineBegin = -1, -1
			continue
		}
		lineBegin := s.lineBeginBackwards(st.start)
		if lineBegin == -1 {
			prevIndent, prevLineBegin = -1, -1
			continue
		}
		indent := st.start - lineBegin
		s.checkBody(ctx, list, i, indent)
		if prevIndent == -1 {
			prevIndent, prevLineBegin = indent, lineBegin
			continue
		}
		if indent > prevIndent+1 && prevIndent != 0 {
			if !s.benignPredecessor(list, i) && !s.conditionCommentedOut(lineBegin, prevIndent) {
				s.reportContinuation(ctx, list, i)
				return
			}
		} else if indent <= prevIndent {
			if delta := s.indentationDelta(prevLineBegin, lineBegin, indent); delta != -1 {
				s.reportIndentString(ctx, st, prevLineBegin, lineBegin, delta)
				return
			}
		}
		prevIndent, prevLineBegin = indent, lineBegin
	}
}

// checkBody reports an unbraced body that starts at the column of its control statement.
func (s *siScan) checkBody(ctx *api.Context, list []siStmt, i, indent int) {
	st := &list[i]
	body := st.body
	if !body.present || (st.kind == siIf && body.isIf) {
		return
	}
	if body.jump && (i >= len(list)-1 || list[i+1].kind == siJump || list[i+1].kind == siBreakish) {
		return
	}
	bodyLineBegin := s.lineBeginBackwards(body.start)
	if bodyLineBegin == -1 {
		return
	}
	if body.start-bodyLineBegin == indent {
		ctx.EmitAt(s.file.FlatRow(body.node)+1, s.file.FlatCol(body.node)+1, siMsgConditional)
	}
}

func (s *siScan) reportContinuation(ctx *api.Context, list []siStmt, i int) {
	st, prev := &list[i], &list[i-1]
	how := siNotContinuing
	if s.isControl(prev.kind) {
		how = siNotNested
	}
	ctx.EmitAt(s.file.FlatRow(st.node)+1, s.file.FlatCol(st.node)+1,
		"Suspicious indentation: This is indented but is not "+how+" the previous expression (`"+s.describeElement(prev)+"`...)")
}

func (s *siScan) reportIndentString(ctx *api.Context, st *siStmt, prevLineBegin, lineBegin, delta int) {
	msg := "The indentation string here is different from on the previous line (`" +
		siDescribeChar(s.content[prevLineBegin+delta]) + "` vs `" + siDescribeChar(s.content[lineBegin+delta]) + "`)"
	ctx.Emit(scanner.Finding{
		Line: s.file.FlatRow(st.node) + 1, Col: delta + 1, Severity: "warning", Message: msg,
	})
}

// isControl reports the statements Lint treats as control statements: if and loops other than do-while.
func (s *siScan) isControl(k siKind) bool { return k == siIf || k == siLoop }

// benignPredecessor ports hasBenignPredecessor: the previous statement explains the extra indentation.
func (s *siScan) benignPredecessor(list []siStmt, i int) bool {
	prev, cur := &list[i-1], &list[i]
	switch {
	case s.isControl(prev.kind):
		return s.closedWithBraces(prev)
	case prev.kind == siDecl || prev.kind == siAssign:
		if cur.kind == siAssign || (cur.curBenign && (cur.javaCall == 0 || !s.javaCallReturnsValue(cur.javaCall))) {
			return true
		}
		return s.closedWithBraces(prev)
	case prev.kind == siBinary:
		return false
	}
	return true
}

func (s *siScan) closedWithBraces(st *siStmt) bool {
	return st.innerEnd > 0 && st.innerEnd <= len(s.content) && s.content[st.innerEnd-1] == '}'
}

// conditionCommentedOut reports a line comment above the statement at the previous statement's indentation.
func (s *siScan) conditionCommentedOut(lineBegin, prevIndent int) bool {
	if lineBegin < 2 {
		return false
	}
	prevStart := bytes.LastIndexByte(s.content[:lineBegin-1], '\n') + 1
	first := s.firstNonSpace(prevStart)
	return first-prevStart == prevIndent && first+2 <= len(s.content) && s.content[first] == '/' && s.content[first+1] == '/'
}

func (s *siScan) firstNonSpace(from int) int {
	i := from
	for i < len(s.content) && siIsSpace(s.content[i]) {
		i++
	}
	return i
}

// lineBeginBackwards returns the offset of the line start when only whitespace precedes the offset on its line, or -1.
func (s *siScan) lineBeginBackwards(offset int) int {
	if offset > len(s.content) {
		return -1
	}
	for i := offset - 1; i >= 0; i-- {
		c := s.content[i]
		if c == '\n' {
			return i + 1
		}
		if !siIsSpace(c) {
			return -1
		}
	}
	return 0
}

// indentationDelta returns the first column where the two indentation strings differ, or -1.
func (s *siScan) indentationDelta(prevLineBegin, lineBegin, indent int) int {
	for k := 0; k < indent; k++ {
		if s.content[prevLineBegin+k] != s.content[lineBegin+k] {
			return k
		}
	}
	return -1
}

func siIsSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}

func siDescribeChar(c byte) string {
	switch c {
	case '\t':
		return `\t`
	case '\r':
		return `\r`
	case ' ':
		return `" "`
	}
	return `\u` + strings.ToUpper(string("0123456789abcdef"[c>>4])+string("0123456789abcdef"[c&15]))
}

// describeElement ports describeElement: the first characters of a statement, cut at an identifier end.
func (s *siScan) describeElement(st *siStmt) string {
	text := utf16.Encode([]rune(string(s.content[st.start:st.end])))
	end := min(20, len(text)-1)
	for end < 20 && end != 10 {
		if end < 0 {
			end = 0
			break
		}
		c := rune(text[end])
		if unicode.IsLetter(c) || unicode.IsDigit(c) || c == '_' || c == '$' {
			end++
			break
		}
		end--
	}
	end = max(0, min(end, len(text)))
	return strings.ReplaceAll(string(utf16.Decode(text[:end])), "\n", " ")
}
