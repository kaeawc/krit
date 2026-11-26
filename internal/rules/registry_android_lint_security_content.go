package rules

import (
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// registerAndroidLintSecurityContentPorts registers the Android Lint 32.4.1 Security checks for WebView bridges and
// ContentProvider file names, in android_lint_security_javascript.go and android_lint_security_filename.go.
func registerAndroidLintSecurityContentPorts() {
	js := &JavascriptInterfaceRule{AndroidRule: lintAndroidRule("JavascriptInterface",
		"Missing @JavascriptInterface on methods", ALCSecurity, ALSError, 8)}
	api.Register(&api.Rule{
		ID: js.RuleName, Category: js.RuleSetName, Description: js.Description(), Sev: api.Severity(js.Sev),
		NodeTypes: javascriptInterfaceNodeTypes,
		// The WebView receiver is proven the way AddJavascriptInterface proves it.
		Needs:     api.NeedsTypeInfo | api.NeedsOracleCallTargets | api.NeedsOracleSupertypes,
		Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava},
		// AddJavascriptInterface used to report the missing-annotation finding, so its suppressions and baseline
		// entries keep silencing it.
		Aliases: []string{"AddJavascriptInterface"},
		LanguageSupport: map[string]api.LanguageSupport{JavaLanguageSupportKey: {
			Status:   api.LanguageSupportSupported,
			Evidence: []string{"internal/rules/android_lint_security_content_test.go"},
		}},
		Confidence: js.Confidence(), Implementation: js, NeedsLibraryFacts: true,
		Check: func(ctx *api.Context) {
			if !scanner.IsTestFile(projectPath(ctx, ctx.File.Path)) {
				js.check(ctx)
			}
		},
	})
	filename := &UnsanitizedFilenameFromContentProviderRule{AndroidRule: lintAndroidRule(
		"UnsanitizedFilenameFromContentProvider", "Trusting ContentProvider filenames without any sanitization",
		ALCSecurity, ALSWarning, 6)}
	registerLintRule(filename.AndroidRule, filename, filename.Confidence(), filename.check, lintCallNodeTypes[:2], 0, nil)
}
