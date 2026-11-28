package rules

import (
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// registerAndroidLintAPIUsageChecks registers the Android Lint 32.4.1 WebViewApiAvailability, EnqueueWork and
// ReportShortcutUsage checks.
func registerAndroidLintAPIUsageChecks() {
	webView := &WebViewAPIAvailabilityRule{AndroidRule: lintAndroidRule("WebViewApiAvailability", "WebView API Availability",
		ALCCorrectness, ALSWarning, 7)}
	registerLintRule(webView.AndroidRule, webView, webView.Confidence(), webView.check, lintCallNodeTypes[:2], 0, nil)

	enqueue := &EnqueueWorkRule{AndroidRule: lintAndroidRule("EnqueueWork", "WorkManager Enqueue", ALCCorrectness, ALSWarning, 6)}
	registerLintRule(enqueue.AndroidRule, enqueue, enqueue.Confidence(), enqueue.check, lintCallNodeTypes[:2], 0, nil)

	shortcut := &ReportShortcutUsageRule{AndroidRule: lintAndroidRule("ReportShortcutUsage", "Report shortcut usage",
		ALCUsability, ALSInformational, 2)}
	api.Register(&api.Rule{
		ID: shortcut.RuleName, Category: shortcut.RuleSetName, Description: shortcut.Description(), Sev: api.Severity(shortcut.Sev),
		Needs: api.NeedsParsedFiles, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava},
		Confidence: shortcut.Confidence(), Implementation: shortcut, Check: shortcut.check,
	})
}
