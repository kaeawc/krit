package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// LocaleGetDefaultForFormattingRule flags `Locale.getDefault()` passed to a
// formatter used in a persistence or network IO context (a `DateTimeFormatter`
// ISO/RFC constant). For machine-readable output the locale should be
// `Locale.ROOT` or `Locale.US` so the bytes do not vary by device locale.
type LocaleGetDefaultForFormattingRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *LocaleGetDefaultForFormattingRule) Confidence() float64 { return api.ConfidenceMediumHigh }

// isPersistenceOrNetworkFormatterReceiver returns true if the receiver of a
// `.withLocale(...)` call is a known machine-readable formatter constant. The
// rule is gated on this so user-facing formatters (which may legitimately use
// the device locale) are not flagged.
func isPersistenceOrNetworkFormatterReceiver(text string) bool {
	t := compactKotlinExpr(text)
	t = strings.TrimPrefix(t, "java.time.format.")
	return strings.HasPrefix(t, "DateTimeFormatter.ISO_") ||
		strings.HasPrefix(t, "DateTimeFormatter.RFC_") ||
		strings.HasPrefix(t, "DateTimeFormatter.BASIC_ISO_")
}

func (r *LocaleGetDefaultForFormattingRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File

	navExpr, args := flatCallExpressionParts(file, idx)
	if navExpr == 0 || args == 0 {
		return
	}
	if flatNavigationExpressionLastIdentifier(file, navExpr) != "withLocale" {
		return
	}

	receiver := file.FlatNamedChild(navExpr, 0)
	if receiver == 0 {
		return
	}
	receiverText := file.FlatNodeText(receiver)
	if !isPersistenceOrNetworkFormatterReceiver(receiverText) {
		return
	}

	if !sourceImportsOrMentions(file, "java.time.format.DateTimeFormatter") &&
		!strings.Contains(compactKotlinExpr(receiverText), "java.time.format.DateTimeFormatter") {
		return
	}

	firstArg, argCount := flatValueArgumentStats(file, args)
	if argCount != 1 || firstArg == 0 {
		return
	}
	argText := compactKotlinExpr(file.FlatNodeText(firstArg))
	if argText != "Locale.getDefault()" && argText != "java.util.Locale.getDefault()" {
		return
	}

	ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"'withLocale(Locale.getDefault())' on a persistence/network formatter; pass Locale.ROOT (or Locale.US) so output is locale-independent.")
}
