package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// isStatementPositionExpr reports whether the given expression node appears
// in statement position. Kotlin's grammar uses if_expression / when_expression
// for both statements (`if (x) foo()`, `when (x) { ... }`) and expression-
// position uses (`val r = if (...) y else z`, `val r = when (x) { ... }`,
// `return if (...) ...`, arguments, etc.). Wrapping the body of an
// expression-position if/when in braces is syntactically valid but visually
// broken and is never what the brace-wrap rules intend.
//
// An expression is in statement position when its parent is a `statements`
// block. Statement-position control constructs may nest (else-if chains
// encoded as control_structure_body, or a when inside an if body); walk up
// through those wrappers and check the outermost parent.
func isStatementPositionExpr(file *scanner.File, idx uint32) bool {
	cur := idx
	for {
		p, ok := file.FlatParent(cur)
		if !ok {
			return false
		}
		switch file.FlatType(p) {
		case "statements":
			return true
		case "control_structure_body":
			gp, ok2 := file.FlatParent(p)
			if !ok2 {
				return false
			}
			switch file.FlatType(gp) {
			case "if_expression", "when_expression",
				"for_statement", "while_statement", "do_while_statement":
				cur = gp
			default:
				return false
			}
		default:
			return false
		}
	}
}

// controlBodyHasBraces returns true when the control_structure_body node
// at body has an opening brace token as its first child — i.e. the body is
// a `{ ... }` block. A body without a brace child is a single expression
// form (`if (x) foo()`), regardless of whitespace or comments.
func controlBodyHasBraces(file *scanner.File, body uint32) bool {
	first := file.FlatFirstChild(body)
	return first != 0 && file.FlatType(first) == "{"
}

// buildBraceWrapFix wraps a single-statement control body in braces while
// preserving the column the body already sits at (or one step deeper than
// the header for inline forms) so the result stays ktfmt-compatible.
// Returns nil when the control header is not the first non-whitespace on
// its line (e.g. inline RHS `val r = if (x) y else z`): the surrounding
// indentation is not strong enough to derive a safe placement for the
// closing brace, so we emit only the finding without a fix.
func buildBraceWrapFix(file *scanner.File, control, body uint32) *scanner.Fix {
	controlStart := int(file.FlatStartByte(control))
	if !isFirstNonWSOnLine(file.Content, controlStart) {
		return nil
	}
	bodyTrimmed := strings.TrimSpace(file.FlatNodeText(body))

	parentIndent := detectIndent(file.Content, controlStart)
	bodyIndent := parentIndent + indentStep(parentIndent)
	// A body that starts its own line keeps its indent when that is deeper than the header's;
	// otherwise (the body follows `->` or `)` on a line of the header) it nests one step in.
	if bodyStart := int(file.FlatStartByte(body)); isFirstNonWSOnLine(file.Content, bodyStart) {
		if indent := detectIndent(file.Content, bodyStart); len(indent) > len(parentIndent) {
			bodyIndent = indent
		}
	}

	// Extend the replacement range back from body's start byte over whitespace
	// and any comments between the header and the body, so the new `{` lands
	// on the header line and never inside a `//` comment. The comments are
	// kept: they are re-emitted between the `{` and the body.
	startByte := int(file.FlatStartByte(body))
	bodyStart := startByte
	comments := braceWrapCommentsBefore(file, control, startByte)
	for {
		for startByte > 0 && isFixWhitespace(file.Content[startByte-1]) {
			startByte--
		}
		c, ok := comments[startByte]
		if !ok {
			break
		}
		startByte = c
	}
	between := ""
	if startByte < bodyStart {
		between = string(file.Content[startByte:bodyStart])
	}
	if strings.Contains(between, "//") || strings.Contains(between, "/*") {
		// Keep the original layout of the comments, then the body on its own line.
		return &scanner.Fix{
			ByteMode:    true,
			StartByte:   startByte,
			EndByte:     int(file.FlatEndByte(body)),
			Replacement: " {" + between + bodyTrimmed + "\n" + parentIndent + "}",
		}
	}

	return &scanner.Fix{
		ByteMode:    true,
		StartByte:   startByte,
		EndByte:     int(file.FlatEndByte(body)),
		Replacement: " {\n" + bodyIndent + bodyTrimmed + "\n" + parentIndent + "}",
	}
}

func isFixWhitespace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

// braceWrapCommentsBefore maps the end byte of each comment inside control that
// ends at or before limit to the comment's start byte.
func braceWrapCommentsBefore(file *scanner.File, control uint32, limit int) map[int]int {
	var out map[int]int
	file.FlatWalkAllNodes(control, func(n uint32) {
		if !isFlatCommentNode(file, n) || int(file.FlatEndByte(n)) > limit {
			return
		}
		if out == nil {
			out = map[int]int{}
		}
		out[int(file.FlatEndByte(n))] = int(file.FlatStartByte(n))
	})
	return out
}

// isFirstNonWSOnLine distinguishes statement-position control flow from
// inline RHS forms where wrapping with our derived indent would misalign.
func isFirstNonWSOnLine(content []byte, byteOffset int) bool {
	if byteOffset < 0 || byteOffset >= len(content) {
		return false
	}
	for i := byteOffset - 1; i >= 0; i-- {
		c := content[i]
		if c == '\n' {
			return true
		}
		if c != ' ' && c != '\t' {
			return false
		}
	}
	return true
}

// indentStep returns the indentation unit (tab or 4 spaces) to add for one
// nesting level. Uses tab when the parent indent itself is tab-based so we
// stay consistent with the file's indentation style; otherwise defaults to
// 4 spaces (ktfmt-kotlinlang default).
func indentStep(parentIndent string) string {
	if strings.ContainsRune(parentIndent, '\t') {
		return "\t"
	}
	return "    "
}

// bracedBodyRemovableStatement returns the lone statement of a braced body
// whose braces can be dropped (`{ foo() }`), or 0 when the braces are needed
// or carry comments.
func bracedBodyRemovableStatement(file *scanner.File, body uint32) uint32 {
	if !controlBodyHasBraces(file, body) {
		return 0
	}
	hasComment := false
	file.FlatWalkAllNodes(body, func(n uint32) {
		if t := file.FlatType(n); t == "line_comment" || t == "multiline_comment" {
			hasComment = true
		}
	})
	if hasComment {
		return 0
	}
	stmts, ok := file.FlatFindChild(body, "statements")
	if !ok || stmts == 0 || file.FlatNamedChildCount(stmts) != 1 {
		return 0
	}
	stmt := file.FlatNamedChild(stmts, 0)
	switch file.FlatType(stmt) {
	case "property_declaration", "function_declaration", "class_declaration",
		"object_declaration", "type_alias", "multi_variable_declaration":
		return 0
	}
	return stmt
}

// bracedBodyUnwrapFix replaces `{ stmt }` with `stmt` for single-line statements.
func bracedBodyUnwrapFix(file *scanner.File, body, stmt uint32) *scanner.Fix {
	text := file.FlatNodeText(stmt)
	if strings.ContainsAny(text, "\r\n") {
		return nil
	}
	return &scanner.Fix{
		ByteMode:    true,
		StartByte:   int(file.FlatStartByte(body)),
		EndByte:     int(file.FlatEndByte(body)),
		Replacement: text,
	}
}

// ifHasElse reports whether the if_expression has an else branch.
func ifHasElse(file *scanner.File, idx uint32) bool {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "else" {
			return true
		}
	}
	return false
}

// BracesOnIfStatementsRule enforces braces on if statements.
type BracesOnIfStatementsRule struct {
	FlatDispatchBase
	BaseRule
	SingleLine string
	MultiLine  string
}

// Confidence reports a tier-2 (medium) base confidence. Style/braces rule. Detection checks AST shape for if/when/else brace
// presence; the preferred form is a style preference. Classified per
// roadmap/17.
func (r *BracesOnIfStatementsRule) Confidence() float64 { return api.ConfidenceMedium }

// ifBranch is one branch of an if/else-if chain: the control body and the keyword detekt reports on.
type ifBranch struct {
	body   uint32
	anchor uint32 // the `if` expression for a then branch, the `else` token for an else branch
	isElse bool
}

// bracesPolicyOrDefault returns mode, or the detekt default when mode is empty.
func bracesPolicyOrDefault(mode string, multiLine bool, fallbackSingle, fallbackMulti string) string {
	if mode != "" {
		return mode
	}
	if multiLine {
		return fallbackMulti
	}
	return fallbackSingle
}

// isElseIfOfParent reports whether the if_expression sits in the else branch of another if.
func isElseIfOfParent(file *scanner.File, idx uint32) bool {
	p, ok := file.FlatParent(idx)
	if !ok || file.FlatType(p) != "control_structure_body" {
		return false
	}
	gp, ok := file.FlatParent(p)
	if !ok || file.FlatType(gp) != "if_expression" {
		return false
	}
	_, elseBody := ifRawBodies(file, gp)
	return elseBody == p
}

// ifRawBodies returns the then and else control bodies, keeping an else-if wrapper as the else body.
func ifRawBodies(file *scanner.File, idx uint32) (thenBody, elseBody uint32) {
	sawElse := false
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "else":
			sawElse = true
		case "control_structure_body":
			if sawElse {
				elseBody = child
			} else if thenBody == 0 {
				thenBody = child
			}
		}
	}
	return thenBody, elseBody
}

// ifElseToken returns the `else` keyword child of an if_expression.
func ifElseToken(file *scanner.File, idx uint32) uint32 {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "else" {
			return child
		}
	}
	return 0
}

// bodyExprIsQualifiedOrBinary mirrors detekt's walk, which ignores an else branch that is a
// qualified call or a binary expression.
func bodyExprIsQualifiedOrBinary(file *scanner.File, body uint32) bool {
	stmt := file.FlatFirstChild(body)
	for stmt != 0 && isFlatCommentNode(file, stmt) {
		stmt = file.FlatNextSib(stmt)
	}
	if stmt == 0 {
		return false
	}
	switch file.FlatType(stmt) {
	case "navigation_expression", "additive_expression", "multiplicative_expression",
		"comparison_expression", "equality_expression", "conjunction_expression",
		"disjunction_expression", "elvis_expression", "infix_expression", "range_expression",
		"assignment", "in_expression":
		return true
	case "call_expression":
		first := file.FlatFirstChild(stmt)
		return first != 0 && file.FlatType(first) == "navigation_expression"
	}
	return false
}

// ifBranches lists the branches detekt validates for the if chain rooted at idx.
func ifBranches(file *scanner.File, idx uint32) []ifBranch {
	var out []ifBranch
	for cur := idx; cur != 0 && file.FlatType(cur) == "if_expression"; {
		thenBody, elseBody := ifRawBodies(file, cur)
		if thenBody != 0 {
			out = append(out, ifBranch{body: thenBody, anchor: cur})
		}
		if elseBody == 0 {
			break
		}
		inner := file.FlatFirstChild(elseBody)
		if inner != 0 && file.FlatType(inner) == "if_expression" {
			cur = inner
			continue
		}
		if !bodyExprIsQualifiedOrBinary(file, elseBody) {
			out = append(out, ifBranch{body: elseBody, anchor: ifElseToken(file, cur), isElse: true})
		}
		break
	}
	return out
}

// blockStatementCount counts the statements of a braced control body.
func blockStatementCount(file *scanner.File, body uint32) int {
	stmts, ok := file.FlatFindChild(body, "statements")
	if !ok || stmts == 0 {
		return 0
	}
	return file.FlatNamedChildCount(stmts)
}

func (r *BracesOnIfStatementsRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if scanner.IsTestFile(file.Path) || isGradleBuildScript(file.Path) {
		return
	}
	if isElseIfOfParent(file, idx) {
		return
	}
	branches := ifBranches(file, idx)
	if len(branches) == 0 {
		return
	}

	isSingleLine := !strings.Contains(file.FlatNodeText(idx), "\n")
	mode := bracesPolicyOrDefault(r.SingleLine, !isSingleLine, "never", "always")
	if !isSingleLine {
		mode = bracesPolicyOrDefault(r.MultiLine, true, "never", "always")
	}

	var violators []ifBranch
	var message string
	switch mode {
	case "always":
		message = "Missing braces on this branch, add them."
		for _, b := range branches {
			if !controlBodyHasBraces(file, b.body) {
				violators = append(violators, b)
			}
		}
	case "necessary":
		message = "Extra braces exist on this branch, remove them (ignore multi-statement)."
		for _, b := range branches {
			if controlBodyHasBraces(file, b.body) && blockStatementCount(file, b.body) <= 1 {
				violators = append(violators, b)
			}
		}
	case "never":
		message = "Extra braces exist on this branch, remove them."
		for _, b := range branches {
			if controlBodyHasBraces(file, b.body) {
				violators = append(violators, b)
			}
		}
	case "consistent":
		message = "Inconsistent braces, make sure all branches either have or don't have braces."
		braced := 0
		for _, b := range branches {
			if controlBodyHasBraces(file, b.body) {
				braced++
			}
		}
		if braced > 0 && braced < len(branches) {
			violators = branches[:1]
		}
	default:
		return
	}

	for _, b := range violators {
		if b.anchor == 0 {
			continue
		}
		f := r.Finding(file, file.FlatRow(b.anchor)+1, file.FlatCol(b.anchor)+1, message)
		f.Fix = r.braceFix(file, idx, b, mode, branches)
		ctx.Emit(f)
	}
}

// braceFix builds the autofix for a violating branch, or nil when no safe edit exists.
func (r *BracesOnIfStatementsRule) braceFix(file *scanner.File, root uint32, b ifBranch, mode string, branches []ifBranch) *scanner.Fix {
	switch mode {
	case "always":
		if b.isElse || !isStatementPositionExpr(file, root) {
			return nil
		}
		return buildBraceWrapFix(file, b.anchor, b.body)
	case "never", "necessary":
		stmt := bracedBodyRemovableStatement(file, b.body)
		if stmt == 0 {
			return nil
		}
		// `if (a) { if (b) x } else y` needs braces to keep the else binding to the outer if.
		if !b.isElse && len(branches) > 1 && file.FlatType(stmt) == "if_expression" && !ifHasElse(file, stmt) {
			return nil
		}
		return bracedBodyUnwrapFix(file, b.body, stmt)
	}
	return nil
}

// BracesOnWhenStatementsRule enforces braces on when branches.
type BracesOnWhenStatementsRule struct {
	FlatDispatchBase
	BaseRule
	SingleLine string
	MultiLine  string
}

// Confidence reports a tier-2 (medium) base confidence. Style/braces rule. Detection checks AST shape for if/when/else brace
// presence; the preferred form is a style preference. Classified per
// roadmap/17.
func (r *BracesOnWhenStatementsRule) Confidence() float64 { return api.ConfidenceMedium }

// whenEntryBody returns the control body of a when_entry.
func whenEntryBody(file *scanner.File, entry uint32) uint32 {
	body, _ := file.FlatFindChild(entry, "control_structure_body")
	return body
}

// whenBodyHasUnnecessaryBraces mirrors detekt: a non-empty block, except one holding only an arrowless lambda.
func whenBodyHasUnnecessaryBraces(file *scanner.File, body uint32) bool {
	if !controlBodyHasBraces(file, body) {
		return false
	}
	n := blockStatementCount(file, body)
	if n == 0 {
		return false
	}
	if n > 1 {
		return true
	}
	stmts, _ := file.FlatFindChild(body, "statements")
	only := file.FlatNamedChild(stmts, 0)
	if file.FlatType(only) != "lambda_literal" {
		return true
	}
	for c := file.FlatFirstChild(only); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "->" {
			return true
		}
	}
	return false
}

// whenIsMultiLine mirrors detekt: some entry has a newline after its arrow.
func whenIsMultiLine(file *scanner.File, whenExpr uint32) bool {
	for entry := file.FlatFirstChild(whenExpr); entry != 0; entry = file.FlatNextSib(entry) {
		if file.FlatType(entry) != "when_entry" {
			continue
		}
		arrow := whenArrow(file, entry)
		if arrow == 0 {
			continue
		}
		after := file.Content[file.FlatEndByte(arrow):file.FlatEndByte(entry)]
		if strings.Contains(string(after), "\n") {
			return true
		}
	}
	return false
}

// whenArrow returns the `->` token of a when_entry.
func whenArrow(file *scanner.File, entry uint32) uint32 {
	for child := file.FlatFirstChild(entry); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "->" {
			return child
		}
	}
	return 0
}

// whenKeyword returns the `when` token of a when_expression.
func whenKeyword(file *scanner.File, whenExpr uint32) uint32 {
	for child := file.FlatFirstChild(whenExpr); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "when" {
			return child
		}
	}
	return whenExpr
}

func (r *BracesOnWhenStatementsRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if scanner.IsTestFile(file.Path) {
		return
	}

	var entries []uint32
	for entry := file.FlatFirstChild(idx); entry != 0; entry = file.FlatNextSib(entry) {
		if file.FlatType(entry) == "when_entry" && whenEntryBody(file, entry) != 0 {
			entries = append(entries, entry)
		}
	}
	if len(entries) == 0 {
		return
	}

	multiLine := whenIsMultiLine(file, idx)
	mode := bracesPolicyOrDefault(r.SingleLine, multiLine, "never", "always")
	if multiLine {
		mode = bracesPolicyOrDefault(r.MultiLine, true, "never", "always")
	}

	hasBraces := func(e uint32) bool { return controlBodyHasBraces(file, whenEntryBody(file, e)) }
	unnecessary := func(e uint32) bool { return whenBodyHasUnnecessaryBraces(file, whenEntryBody(file, e)) }

	var violators []uint32
	var message string
	switch mode {
	case "always":
		message = "Missing braces on this branch, add them."
		for _, e := range entries {
			if !hasBraces(e) {
				violators = append(violators, e)
			}
		}
	case "necessary":
		message = "Extra braces exist on this branch, remove them."
		for _, e := range entries {
			if unnecessary(e) && blockStatementCount(file, whenEntryBody(file, e)) <= 1 {
				violators = append(violators, e)
			}
		}
	case "never":
		message = "Extra braces exist on this branch, remove them."
		for _, e := range entries {
			if unnecessary(e) {
				violators = append(violators, e)
			}
		}
	case "consistent":
		message = "Inconsistent braces, make sure all branches either have or don't have braces."
		bare, extra := 0, 0
		for _, e := range entries {
			if unnecessary(e) {
				extra++
			}
			if !hasBraces(e) {
				bare++
			}
		}
		if extra > 0 && bare > 0 {
			violators = entries[:1]
		}
	default:
		return
	}

	for _, e := range violators {
		anchor := whenArrow(file, e)
		if mode == "consistent" || anchor == 0 {
			anchor = whenKeyword(file, idx)
		}
		f := r.Finding(file, file.FlatRow(anchor)+1, file.FlatCol(anchor)+1, message)
		body := whenEntryBody(file, e)
		switch mode {
		case "always":
			if isStatementPositionExpr(file, idx) {
				f.Fix = buildBraceWrapFix(file, e, body)
			}
		case "never", "necessary":
			if stmt := bracedBodyRemovableStatement(file, body); stmt != 0 {
				f.Fix = bracedBodyUnwrapFix(file, body, stmt)
			}
		}
		ctx.Emit(f)
	}
}

// MandatoryBracesLoopsRule requires braces in for/while/do-while loops.
type MandatoryBracesLoopsRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/braces rule. Detection checks AST shape for if/when/else brace
// presence; the preferred form is a style preference. Classified per
// roadmap/17.
func (r *MandatoryBracesLoopsRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *MandatoryBracesLoopsRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File

	body, _ := file.FlatFindChild(idx, "control_structure_body")
	if body == 0 {
		return
	}
	if !controlBodyHasBraces(file, body) {
		f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
			"Loop body should use braces.")
		f.Fix = buildBraceWrapFix(file, idx, body)
		ctx.Emit(f)
		return
	}
}
