package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

// WebViewUniversalAccessFromFileUrlsRule flags
// WebSettings.setAllowUniversalAccessFromFileURLs(true) and the property
// assignment equivalent webView.settings.allowUniversalAccessFromFileURLs =
// true. Universal access from file:// URLs lets a page loaded from a local
// file make cross-origin requests to any domain — one of the most dangerous
// WebView misconfigurations.
type WebViewUniversalAccessFromFileUrlsRule struct {
	FlatDispatchBase
	AndroidRule
}

func (r *WebViewUniversalAccessFromFileUrlsRule) Confidence() float64 { return api.ConfidenceHigher }

const webViewUniversalAccessFromFileUrlsMessage = "Avoid enabling WebSettings.allowUniversalAccessFromFileURLs; pages loaded from file:// URLs can read arbitrary cross-origin URLs, exposing local data."

const (
	webViewUniversalAccessSetter   = "setAllowUniversalAccessFromFileURLs"
	webViewUniversalAccessProperty = "allowUniversalAccessFromFileURLs"
)

func (r *WebViewUniversalAccessFromFileUrlsRule) check(ctx *api.Context) {
	file := ctx.File

	switch file.FlatType(ctx.Idx) {
	case "call_expression":
		if flatCallExpressionName(file, ctx.Idx) != webViewUniversalAccessSetter {
			return
		}
		if !callBoolArgIsTrue(file, ctx.Idx) {
			return
		}
		receiver := kotlinCallReceiverChain(file, ctx.Idx)
		if !isWebSettingsReceiverAt(file, ctx.Idx, receiver) {
			return
		}
		emitWebSettingsBoolFinding(ctx, ctx.Idx, r.RuleSetName, r.RuleName, r.Sev, webViewUniversalAccessFromFileUrlsMessage, r.Confidence())

	case "method_invocation":
		if javaAwareCallName(file, ctx.Idx) != webViewUniversalAccessSetter {
			return
		}
		if !callBoolArgIsTrue(file, ctx.Idx) {
			return
		}
		receiver := javaMethodReceiverText(file, ctx.Idx)
		if !isWebSettingsReceiverAt(file, ctx.Idx, receiver) {
			return
		}
		emitWebSettingsBoolFinding(ctx, ctx.Idx, r.RuleSetName, r.RuleName, r.Sev, webViewUniversalAccessFromFileUrlsMessage, r.Confidence())

	case "assignment":
		target := kotlinAssignmentTargetChain(file, ctx.Idx)
		head, tail := chainSplitTrailing(target)
		if tail != webViewUniversalAccessProperty {
			return
		}
		if !isWebSettingsReceiverAt(file, ctx.Idx, head) {
			return
		}
		valueIdx := findKotlinAssignmentValue(file, ctx.Idx)
		if valueIdx == 0 || !isLiteralBoolTrue(file, valueIdx) {
			return
		}
		emitWebSettingsBoolFinding(ctx, ctx.Idx, r.RuleSetName, r.RuleName, r.Sev, webViewUniversalAccessFromFileUrlsMessage, r.Confidence())
	}
}
