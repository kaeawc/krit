package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
	"github.com/kaeawc/krit/internal/scanner"
)

// checkTextViewEdits reports `textView.text as Editable` where the receiver is a plain
// android.widget.TextView. Only an EditText (or a TextView set to BufferType.EDITABLE)
// returns an Editable, so the cast fails or edits text the user cannot change.
func checkTextViewEdits(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	if file.FlatType(idx) != "as_expression" {
		return
	}
	var subject, target uint32
	for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "user_type", "nullable_type":
			target = c
		default:
			if file.FlatIsNamed(c) && subject == 0 {
				subject = c
			}
		}
	}
	if subject == 0 || target == 0 || !strings.HasSuffix(strings.TrimSuffix(file.FlatNodeText(target), "?"), "Editable") {
		return
	}
	receiver := textViewTextReceiver(file, flatUnwrapParenExpr(file, subject))
	if receiver == 0 {
		return
	}
	typ, ok := semantics.ExpressionType(ctx, receiver)
	if !ok || typ.Type == nil || typ.Type.FQN != "android.widget.TextView" {
		return
	}
	ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"TextView.getText() is only Editable for an EditText; use an EditText to edit text in place.")
}

// textViewTextReceiver returns the receiver of `.text` or `.getText()`, or 0.
func textViewTextReceiver(file *scanner.File, expr uint32) uint32 {
	switch file.FlatType(expr) {
	case "navigation_expression":
		if flatNavigationExpressionLastIdentifier(file, expr) == "text" {
			return file.FlatFirstChild(expr)
		}
	case "call_expression":
		navExpr, args := flatCallExpressionParts(file, expr)
		if navExpr != 0 && flatCallExpressionName(file, expr) == "getText" && (args == 0 || file.FlatNamedChildCount(args) == 0) {
			return file.FlatFirstChild(navExpr)
		}
	}
	return 0
}
