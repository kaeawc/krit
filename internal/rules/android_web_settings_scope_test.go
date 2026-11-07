package rules_test

import "testing"

func TestWebSettingsScopeAwareReceivers(t *testing.T) {
	const header = "package test\nimport android.webkit.WebSettings\nimport android.webkit.WebView\n"
	cases := []struct {
		name string
		rule string
		body string
		want int
	}{
		{"apply_bare_assignment", "WebViewAllowFileAccess", "fun f(w: WebView) { w.settings.apply { allowFileAccess = true } }", 1},
		{"with_bare_assignment", "WebViewUniversalAccessFromFileUrls", "fun f(w: WebView) { with(w.settings) { allowUniversalAccessFromFileURLs = true } }", 1},
		{"run_bare_mixed_content", "WebViewMixedContentAllowAll", "fun f(w: WebView) { w.settings.run { mixedContentMode = WebSettings.MIXED_CONTENT_ALWAYS_ALLOW } }", 1},
		{"also_it", "WebViewFileAccessFromFileUrls", "fun f(w: WebView) { w.settings.also { it.allowFileAccessFromFileURLs = true } }", 1},
		{"typed_parameter", "WebViewAllowFileAccess", "fun f(s: WebSettings) { s.allowFileAccess = true }", 1},
		{"local_from_settings", "WebViewAllowContentAccess", "fun f(w: WebView) {\n val s = w.settings\n s.allowContentAccess = true\n}", 1},
		{"bare_setter_in_apply", "WebViewAllowFileAccess", "fun f(w: WebView) { w.settings.apply { setAllowFileAccess(true) } }", 1},
		{"unrelated_apply_receiver", "WebViewAllowFileAccess", "fun f(o: Other) { o.apply { allowFileAccess = true } }", 0},
		{"unrelated_local", "WebViewAllowFileAccess", "fun f(o: Other) {\n val s = o.config\n s.allowFileAccess = true\n}", 0},
		{"unrelated_also", "WebViewAllowFileAccess", "fun f(o: Other) { o.also { it.allowFileAccess = true } }", 0},
		{"false_value", "WebViewAllowFileAccess", "fun f(w: WebView) { w.settings.apply { allowFileAccess = false } }", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runRuleByName(t, tc.rule, header+tc.body+"\n")
			if len(got) != tc.want {
				t.Fatalf("expected %d findings, got %d", tc.want, len(got))
			}
		})
	}
}
