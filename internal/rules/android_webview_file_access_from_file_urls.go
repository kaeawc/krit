package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

// WebViewFileAccessFromFileUrlsRule flags
// WebSettings.setAllowFileAccessFromFileURLs(true) and the property-assignment
// equivalent webView.settings.allowFileAccessFromFileURLs = true. Allowing
// file:// pages to read other file:// URLs enables a known WebView XSS vector
// that has been used for local-file exfiltration.
type WebViewFileAccessFromFileUrlsRule struct {
	FlatDispatchBase
	AndroidRule
}

func (r *WebViewFileAccessFromFileUrlsRule) Confidence() float64 { return 0.85 }

const webViewFileAccessFromFileUrlsMessage = "Avoid enabling WebSettings.allowFileAccessFromFileURLs; pages loaded from file:// URLs can read other file:// documents, enabling local-file exfiltration via XSS."

const webViewFileAccessFromFileUrlsSetter = "setAllowFileAccessFromFileURLs"
const webViewFileAccessFromFileUrlsProperty = "allowFileAccessFromFileURLs"

func (r *WebViewFileAccessFromFileUrlsRule) check(ctx *api.Context) {
	file := ctx.File

	switch file.FlatType(ctx.Idx) {
	case "call_expression":
		if flatCallExpressionName(file, ctx.Idx) != webViewFileAccessFromFileUrlsSetter {
			return
		}
		if !callBoolArgIsTrue(file, ctx.Idx) {
			return
		}
		receiver := kotlinCallReceiverChain(file, ctx.Idx)
		if !isWebSettingsReceiver(file, receiver) {
			return
		}
		line := file.FlatRow(ctx.Idx) + 1
		col := file.FlatCol(ctx.Idx) + 1
		emitWebSettingsBoolFinding(ctx, line, col, r.RuleSetName, r.RuleName, r.Sev, webViewFileAccessFromFileUrlsMessage, r.Confidence())

	case "method_invocation":
		if javaAwareCallName(file, ctx.Idx) != webViewFileAccessFromFileUrlsSetter {
			return
		}
		if !callBoolArgIsTrue(file, ctx.Idx) {
			return
		}
		receiver := javaMethodReceiverText(file, ctx.Idx)
		if !isWebSettingsReceiver(file, receiver) {
			return
		}
		line := file.FlatRow(ctx.Idx) + 1
		col := file.FlatCol(ctx.Idx) + 1
		emitWebSettingsBoolFinding(ctx, line, col, r.RuleSetName, r.RuleName, r.Sev, webViewFileAccessFromFileUrlsMessage, r.Confidence())

	case "assignment":
		target := kotlinAssignmentTargetChain(file, ctx.Idx)
		head, tail := chainSplitTrailing(target)
		if tail != webViewFileAccessFromFileUrlsProperty {
			return
		}
		if !isWebSettingsReceiver(file, head) {
			return
		}
		valueIdx := findKotlinAssignmentValue(file, ctx.Idx)
		if valueIdx == 0 || !isLiteralBoolTrue(file, valueIdx) {
			return
		}
		line := file.FlatRow(ctx.Idx) + 1
		col := file.FlatCol(ctx.Idx) + 1
		emitWebSettingsBoolFinding(ctx, line, col, r.RuleSetName, r.RuleName, r.Sev, webViewFileAccessFromFileUrlsMessage, r.Confidence())
	}
}
