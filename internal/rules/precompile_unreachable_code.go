package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// PrecompileUnreachableCodeRule flags statements that follow an
// unconditional jump (return/throw/break/continue) in the same
// statements block. Mirrors kotlinc's UNREACHABLE_CODE.
//
// Conservative by construction: fires only when the jump is in
// statement position (direct child of a `statements` node), so
// expression-position jumps like `x ?: return` and `if (b) return else
// ...` are skipped. Looking only at the jump's direct parent keeps the
// rule local — nested lambdas/anonymous functions cannot escape its
// scope.
type PrecompileUnreachableCodeRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *PrecompileUnreachableCodeRule) Confidence() float64 { return api.ConfidenceVeryHigh }

func (r *PrecompileUnreachableCodeRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	parent, ok := file.FlatParent(idx)
	if !ok || file.FlatType(parent) != "statements" {
		return
	}
	keyword := jumpExpressionKeyword(file, idx)
	if keyword == "" {
		return
	}
	next := firstNamedStatementSiblingAfter(file, idx)
	if next == 0 {
		return
	}
	ctx.EmitAt(file.FlatRow(next)+1, file.FlatCol(next)+1,
		"Unreachable code after `"+keyword+"`. Remove the statements following the unconditional jump.")
}

func jumpExpressionKeyword(file *scanner.File, idx uint32) string {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		t := file.FlatType(child)
		switch t {
		case "return", "throw", "break", "continue":
			return t
		}
	}
	return ""
}

func firstNamedStatementSiblingAfter(file *scanner.File, idx uint32) uint32 {
	for child := file.FlatNextSib(idx); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		switch file.FlatType(child) {
		case "line_comment", "multiline_comment":
			continue
		}
		return child
	}
	return 0
}
