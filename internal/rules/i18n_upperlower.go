package rules

import (
	"fmt"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// UpperLowerInvariantMisuseRule flags Kotlin 1.5+ `uppercase()` /
// `lowercase()` calls that omit an explicit Locale argument. Although
// these methods are locale-invariant (they delegate to Locale.ROOT),
// passing `Locale.ROOT` explicitly documents intent and prevents
// accidental confusion with the deprecated default-locale variants
// (`toUpperCase()` / `toLowerCase()`).
type UpperLowerInvariantMisuseRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence because the rule
// matches on method name without type resolution; same-named local
// helpers will produce false positives.
func (r *UpperLowerInvariantMisuseRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *UpperLowerInvariantMisuseRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if strings.HasSuffix(file.Path, ".gradle.kts") {
		return
	}

	navExpr, args := flatCallExpressionParts(file, idx)
	if navExpr == 0 {
		return
	}

	methodName := flatNavigationExpressionLastIdentifier(file, navExpr)
	if methodName != "uppercase" && methodName != "lowercase" {
		return
	}

	_, argCount := flatValueArgumentStats(file, args)
	if argCount != 0 {
		return
	}

	var receiverIdx uint32
	if file.FlatNamedChildCount(navExpr) > 0 {
		receiverIdx = file.FlatNamedChild(navExpr, 0)
	}
	if receiverIdx != 0 {
		recvText := file.FlatNodeText(receiverIdx)
		if containsASCIIInvariantIdentifier(recvText) {
			return
		}
	}

	ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		fmt.Sprintf("'%s()' called without explicit Locale. Pass 'Locale.ROOT' for case-insensitive comparison or use a display-locale variant for user-facing text.", methodName))
}
