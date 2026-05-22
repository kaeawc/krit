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
	if file.FlatRow(body) != file.FlatRow(control) {
		bodyIndent = detectIndent(file.Content, int(file.FlatStartByte(body)))
	}

	// Extend the replacement range back from body's start byte over plain
	// whitespace only so the new `{` lands on the header line. Stop at any
	// non-whitespace byte (including the end of a comment that sits between
	// the header `)` and the body) so we never delete that text.
	startByte := int(file.FlatStartByte(body))
	for startByte > 0 {
		c := file.Content[startByte-1]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			break
		}
		startByte--
	}

	return &scanner.Fix{
		ByteMode:    true,
		StartByte:   startByte,
		EndByte:     int(file.FlatEndByte(body)),
		Replacement: " {\n" + bodyIndent + bodyTrimmed + "\n" + parentIndent + "}",
	}
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

func (r *BracesOnIfStatementsRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if scanner.IsTestFile(file.Path) || isGradleBuildScript(file.Path) {
		return
	}

	if !isStatementPositionExpr(file, idx) {
		return
	}

	body, _ := file.FlatFindChild(idx, "control_structure_body")
	if body == 0 {
		return
	}

	// Check consistent mode first — it needs to see the whole chain
	startLine := file.FlatRow(idx)
	endLine := startLine + strings.Count(file.FlatNodeText(idx), "\n")
	isSingleLine := startLine == endLine
	mode := r.SingleLine
	if !isSingleLine {
		mode = r.MultiLine
	}
	if mode == "" {
		if isSingleLine {
			mode = "never"
		} else {
			mode = "always"
		}
	}
	if mode == "consistent" {
		r.checkConsistentIfFlat(ctx)
		return
	}
	if mode == "necessary" {
		return
	}

	if controlBodyHasBraces(file, body) {
		return // already has braces
	}

	msg := "Multi-line if statement should use braces."
	if isSingleLine {
		msg = "Single-line if statement should use braces."
	}
	f := r.Finding(file, startLine+1, 1, msg)
	f.Fix = buildBraceWrapFix(file, idx, body)
	ctx.Emit(f)
}

// checkConsistentIf checks if all branches in an if/else chain have consistent braces.
func (r *BracesOnIfStatementsRule) checkConsistentIfFlat(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	// Skip if this is an else-if (let the root if handle the chain)
	if p, ok := file.FlatParent(idx); ok && file.FlatType(p) == "control_structure_body" {
		return
	}

	// Collect all branches
	type branchInfo struct {
		body     uint32
		hasBrace bool
	}
	var branches []branchInfo

	current := idx
	for current != 0 && file.FlatType(current) == "if_expression" {
		var thenBody, elseBody uint32
		sawElse := false
		for child := file.FlatFirstChild(current); child != 0; child = file.FlatNextSib(child) {
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
		if thenBody != 0 {
			branches = append(branches, branchInfo{body: thenBody, hasBrace: controlBodyHasBraces(file, thenBody)})
		}
		if elseBody == 0 {
			break
		}
		// `else if` is encoded as an else-control_structure_body whose first
		// child is a nested if_expression. Descend into that to keep walking
		// the chain instead of recording the wrapper as a branch.
		inner := file.FlatFirstChild(elseBody)
		if inner != 0 && file.FlatType(inner) == "if_expression" {
			current = inner
			continue
		}
		branches = append(branches, branchInfo{body: elseBody, hasBrace: controlBodyHasBraces(file, elseBody)})
		break
	}

	if len(branches) < 2 {
		return
	}

	hasBraces := 0
	for _, b := range branches {
		if b.hasBrace {
			hasBraces++
		}
	}
	if hasBraces == 0 || hasBraces == len(branches) {
		return // all consistent
	}

	// Mixed — flag those without braces
	for _, b := range branches {
		if !b.hasBrace {
			ctx.Emit(r.Finding(file, file.FlatRow(b.body)+1, 1,
				"Inconsistent braces: some branches have braces and some don't."))
		}
	}
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

func (r *BracesOnWhenStatementsRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if scanner.IsTestFile(file.Path) {
		return
	}

	// idx is a when_entry; walk up to the enclosing when_expression and
	// require it to be in statement position. `val r = when (...) { ... }`
	// must not be flagged — wrapping its branch bodies in braces would
	// transform the expression-position when into a visually-broken block.
	whenExpr, ok := file.FlatParent(idx)
	if !ok || file.FlatType(whenExpr) != "when_expression" {
		return
	}
	if !isStatementPositionExpr(file, whenExpr) {
		return
	}

	body, _ := file.FlatFindChild(idx, "control_structure_body")
	if body == 0 {
		return
	}

	startLine := file.FlatRow(idx)
	endLine := startLine + strings.Count(file.FlatNodeText(idx), "\n")
	isSingleLine := startLine == endLine
	mode := r.SingleLine
	if !isSingleLine {
		mode = r.MultiLine
	}
	if mode == "" {
		if isSingleLine {
			mode = "never"
		} else {
			mode = "always"
		}
	}
	if mode == "consistent" {
		r.checkConsistentWhenFlat(ctx)
		return
	}
	if mode == "necessary" {
		return
	}

	if controlBodyHasBraces(file, body) {
		return // already has braces
	}

	msg := "Multi-line when branch should use braces."
	if isSingleLine {
		msg = "Single-line when branch should use braces."
	}
	f := r.Finding(file, startLine+1, 1, msg)
	f.Fix = buildBraceWrapFix(file, idx, body)
	ctx.Emit(f)
}

// checkConsistentWhen checks if all when entries have consistent braces.
func (r *BracesOnWhenStatementsRule) checkConsistentWhenFlat(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	// node is a when_entry. Find the parent when_expression.
	parent, ok := file.FlatParent(idx)
	if !ok || file.FlatType(parent) != "when_expression" {
		return
	}
	// Only process from the first when_entry to avoid duplicates
	for child := file.FlatFirstChild(parent); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "when_entry" {
			if child != idx {
				return // not the first entry
			}
			break
		}
	}

	type entryInfo struct {
		body     uint32
		hasBrace bool
	}
	var entries []entryInfo
	for child := file.FlatFirstChild(parent); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "when_entry" {
			body, _ := file.FlatFindChild(child, "control_structure_body")
			if body != 0 {
				entries = append(entries, entryInfo{body: body, hasBrace: controlBodyHasBraces(file, body)})
			}
		}
	}

	if len(entries) < 2 {
		return
	}
	hasBraces := 0
	for _, e := range entries {
		if e.hasBrace {
			hasBraces++
		}
	}
	if hasBraces == 0 || hasBraces == len(entries) {
		return
	}

	for _, e := range entries {
		if !e.hasBrace {
			ctx.Emit(r.Finding(file, file.FlatRow(e.body)+1, 1,
				"Inconsistent braces: some when entries have braces and some don't."))
		}
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
		f := r.Finding(file, file.FlatRow(idx)+1, 1,
			"Loop body should use braces.")
		f.Fix = buildBraceWrapFix(file, idx, body)
		ctx.Emit(f)
		return
	}
}
