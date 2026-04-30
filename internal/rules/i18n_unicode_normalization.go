package rules

import (
	"strings"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

// UnicodeNormalizationMissingRule flags `.contains(...)` calls inside
// search/find functions that have not normalized either operand. Unicode
// equivalent characters (precomposed vs. decomposed) compare unequal at
// the code-point level, so a search over user-supplied text without
// `Normalizer.normalize` will silently miss matches.
type UnicodeNormalizationMissingRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-3 (low) base confidence. The rule cannot
// resolve the receiver type, so a `List<T>.contains` inside a search
// function may match. The default-inactive setting compensates.
func (r *UnicodeNormalizationMissingRule) Confidence() float64 { return 0.5 }

func (r *UnicodeNormalizationMissingRule) check(ctx *v2.Context) {
	idx, file := ctx.Idx, ctx.File
	if file.FlatType(idx) != "call_expression" {
		return
	}
	if !flatCallExpressionNameEquals(file, idx, "contains") {
		return
	}
	fn, ok := flatEnclosingFunction(file, idx)
	if !ok {
		return
	}
	if !functionNameMatchesSearchPrefix(file, fn) {
		return
	}
	// Coarse heuristic: any `normalize(...)` call inside the enclosing
	// function is taken as evidence the developer handled equivalence.
	if subtreeHasCallName(file, fn, "normalize") {
		return
	}
	ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"contains() inside a search/find function will miss unicode-equivalent characters. Normalize both operands with Normalizer.normalize(..., Normalizer.Form.NFC) before comparing.")
}

func functionNameMatchesSearchPrefix(file *scanner.File, fn uint32) bool {
	if file == nil || fn == 0 || file.FlatType(fn) != "function_declaration" {
		return false
	}
	ident, _ := file.FlatFindChild(fn, "simple_identifier")
	if ident == 0 {
		return false
	}
	name := strings.ToLower(file.FlatNodeText(ident))
	return strings.HasPrefix(name, "search") || strings.HasPrefix(name, "find")
}
