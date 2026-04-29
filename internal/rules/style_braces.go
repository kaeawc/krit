package rules

import (
	"strings"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

// controlBodyHasBraces returns true when the control_structure_body node
// at body has an opening brace token as its first child — i.e. the body is
// a `{ ... }` block. A body without a brace child is a single expression
// form (`if (x) foo()`), regardless of whitespace or comments.
func controlBodyHasBraces(file *scanner.File, body uint32) bool {
	first := file.FlatFirstChild(body)
	return first != 0 && file.FlatType(first) == "{"
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
func (r *BracesOnIfStatementsRule) Confidence() float64 { return 0.75 }

func (r *BracesOnIfStatementsRule) check(ctx *v2.Context) {
	idx, file := ctx.Idx, ctx.File
	if isTestFile(file.Path) || isGradleBuildScript(file.Path) {
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
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   int(file.FlatStartByte(body)),
		EndByte:     int(file.FlatEndByte(body)),
		Replacement: "{\n" + strings.TrimSpace(file.FlatNodeText(body)) + "\n}",
	}
	ctx.Emit(f)
}

// checkConsistentIf checks if all branches in an if/else chain have consistent braces.
func (r *BracesOnIfStatementsRule) checkConsistentIfFlat(ctx *v2.Context) {
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
func (r *BracesOnWhenStatementsRule) Confidence() float64 { return 0.75 }

func (r *BracesOnWhenStatementsRule) check(ctx *v2.Context) {
	idx, file := ctx.Idx, ctx.File
	if isTestFile(file.Path) {
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
	raw := file.FlatNodeText(body)
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   int(file.FlatStartByte(body)),
		EndByte:     int(file.FlatEndByte(body)),
		Replacement: "{\n" + strings.TrimSpace(raw) + "\n}",
	}
	ctx.Emit(f)
}

// checkConsistentWhen checks if all when entries have consistent braces.
func (r *BracesOnWhenStatementsRule) checkConsistentWhenFlat(ctx *v2.Context) {
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
func (r *MandatoryBracesLoopsRule) Confidence() float64 { return 0.75 }

func (r *MandatoryBracesLoopsRule) check(ctx *v2.Context) {
	idx, file := ctx.Idx, ctx.File

	body, _ := file.FlatFindChild(idx, "control_structure_body")
	if body == 0 {
		return
	}
	if !controlBodyHasBraces(file, body) {
		f := r.Finding(file, file.FlatRow(idx)+1, 1,
			"Loop body should use braces.")
		raw := file.FlatNodeText(body)
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   int(file.FlatStartByte(body)),
			EndByte:     int(file.FlatEndByte(body)),
			Replacement: "{\n" + strings.TrimSpace(raw) + "\n}",
		}
		ctx.Emit(f)
		return
	}
}
