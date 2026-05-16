package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// WebViewMixedContentAllowAllRule flags WebSettings.setMixedContentMode and the
// property-assignment equivalent webView.settings.mixedContentMode being set
// to MIXED_CONTENT_ALWAYS_ALLOW (or its int value 0). Allowing all mixed
// content lets http subresources load on https pages — a TLS-stripping vector.
type WebViewMixedContentAllowAllRule struct {
	FlatDispatchBase
	AndroidRule
}

func (r *WebViewMixedContentAllowAllRule) Confidence() float64 { return 0.9 }

const webViewMixedContentAllowAllMessage = "Avoid setting WebSettings.mixedContentMode to MIXED_CONTENT_ALWAYS_ALLOW; it permits http subresources on https pages and enables TLS stripping. Prefer MIXED_CONTENT_NEVER_ALLOW or MIXED_CONTENT_COMPATIBILITY_MODE."

func (r *WebViewMixedContentAllowAllRule) check(ctx *api.Context) {
	file := ctx.File

	switch file.FlatType(ctx.Idx) {
	case "call_expression":
		if flatCallExpressionName(file, ctx.Idx) != "setMixedContentMode" {
			return
		}
		if !callFirstArgIsMixedContentAllowAll(file, ctx.Idx) {
			return
		}
		receiver := kotlinCallReceiverChain(file, ctx.Idx)
		if !isWebSettingsReceiver(file, receiver) {
			return
		}
		line := file.FlatRow(ctx.Idx) + 1
		col := file.FlatCol(ctx.Idx) + 1
		emitWebSettingsBoolFinding(ctx, line, col, r.RuleSetName, r.RuleName, r.Sev, webViewMixedContentAllowAllMessage, r.Confidence())

	case "method_invocation":
		if javaAwareCallName(file, ctx.Idx) != "setMixedContentMode" {
			return
		}
		if !callFirstArgIsMixedContentAllowAll(file, ctx.Idx) {
			return
		}
		receiver := javaMethodReceiverText(file, ctx.Idx)
		if !isWebSettingsReceiver(file, receiver) {
			return
		}
		line := file.FlatRow(ctx.Idx) + 1
		col := file.FlatCol(ctx.Idx) + 1
		emitWebSettingsBoolFinding(ctx, line, col, r.RuleSetName, r.RuleName, r.Sev, webViewMixedContentAllowAllMessage, r.Confidence())

	case "assignment":
		target := kotlinAssignmentTargetChain(file, ctx.Idx)
		head, tail := chainSplitTrailing(target)
		if tail != "mixedContentMode" {
			return
		}
		if !isWebSettingsReceiver(file, head) {
			return
		}
		valueIdx := findKotlinAssignmentValue(file, ctx.Idx)
		if valueIdx == 0 || !isMixedContentAllowAllExpr(file, valueIdx) {
			return
		}
		line := file.FlatRow(ctx.Idx) + 1
		col := file.FlatCol(ctx.Idx) + 1
		emitWebSettingsBoolFinding(ctx, line, col, r.RuleSetName, r.RuleName, r.Sev, webViewMixedContentAllowAllMessage, r.Confidence())
	}
}

// isMixedContentAllowAllExpr reports whether the expression at idx textually
// matches MIXED_CONTENT_ALWAYS_ALLOW (optionally qualified with WebSettings)
// or the literal int 0 after unwrapping parentheses.
func isMixedContentAllowAllExpr(file *scanner.File, idx uint32) bool {
	idx = flatUnwrapParenExpr(file, idx)
	text := strings.TrimSpace(file.FlatNodeText(idx))
	switch text {
	case "MIXED_CONTENT_ALWAYS_ALLOW",
		"WebSettings.MIXED_CONTENT_ALWAYS_ALLOW",
		"android.webkit.WebSettings.MIXED_CONTENT_ALWAYS_ALLOW",
		"0":
		return true
	}
	return false
}

// callFirstArgIsMixedContentAllowAll reports whether the first argument of a
// Kotlin call_expression or Java method_invocation is one of the accepted
// MIXED_CONTENT_ALWAYS_ALLOW expressions.
func callFirstArgIsMixedContentAllowAll(file *scanner.File, call uint32) bool {
	switch file.FlatType(call) {
	case "call_expression":
		_, args := flatCallExpressionParts(file, call)
		if args == 0 || file.FlatNamedChildCount(args) == 0 {
			return false
		}
		first := file.FlatNamedChild(args, 0)
		if first == 0 {
			return false
		}
		inner := first
		if file.FlatType(first) == "value_argument" {
			if c := file.FlatNamedChild(first, 0); c != 0 {
				inner = c
			}
		}
		return isMixedContentAllowAllExpr(file, inner)
	case "method_invocation":
		args, ok := file.FlatFindChild(call, "argument_list")
		if !ok || file.FlatNamedChildCount(args) == 0 {
			return false
		}
		first := file.FlatNamedChild(args, 0)
		return isMixedContentAllowAllExpr(file, first)
	}
	return false
}
