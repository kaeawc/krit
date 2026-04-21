package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

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

	bodyText := file.FlatNodeText(body)
	trimmed := strings.TrimSpace(bodyText)
	if strings.HasPrefix(trimmed, "{") {
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
		for child := file.FlatFirstChild(current); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "control_structure_body" {
				text := strings.TrimSpace(file.FlatNodeText(child))
				branches = append(branches, branchInfo{body: child, hasBrace: strings.HasPrefix(text, "{")})
			}
		}
		// (The original code had a dead inner loop here that scanned for
		// nested ifs but was immediately followed by an unconditional
		// break; the dead loop has been removed along with the quadratic.)
		break // Simple: just check immediate branches for now
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

	bodyText := strings.TrimSpace(file.FlatNodeText(body))
	if strings.HasPrefix(bodyText, "{") {
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
				text := strings.TrimSpace(file.FlatNodeText(body))
				entries = append(entries, entryInfo{body: body, hasBrace: strings.HasPrefix(text, "{")})
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
	bodyText := strings.TrimSpace(file.FlatNodeText(body))
	if !strings.HasPrefix(bodyText, "{") {
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
