package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

// WebViewAllowContentAccessRule flags WebSettings.setAllowContentAccess(true)
// and the property-assignment equivalent webView.settings.allowContentAccess
// = true. The content:// URI scheme grants WebView access to content
// providers; explicit enablement is rarely needed on modern Android and
// expands the WebView attack surface.
type WebViewAllowContentAccessRule struct {
	FlatDispatchBase
	AndroidRule
}

func (r *WebViewAllowContentAccessRule) Confidence() float64 { return api.ConfidenceHigh }

const webViewAllowContentAccessMessage = "Avoid enabling WebSettings.allowContentAccess; the content:// URI scheme grants the WebView access to content providers and is rarely required on modern Android."

func (r *WebViewAllowContentAccessRule) check(ctx *api.Context) {
	checkWebSettingsBoolToggle(ctx, webSettingsBoolToggleSpec{
		Setter:   "setAllowContentAccess",
		Property: "allowContentAccess",
		RuleSet:  r.RuleSetName,
		Rule:     r.RuleName,
		Severity: r.Sev,
		Message:  webViewAllowContentAccessMessage,
		Conf:     r.Confidence(),
	})
}
