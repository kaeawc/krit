package rules_test

import (
	"strings"
	"testing"
)

func TestWebViewUniversalAccessFromFileUrls_KotlinPropertyAssignment(t *testing.T) {
	findings := runRuleByName(t, "WebViewUniversalAccessFromFileUrls", `
import android.webkit.WebSettings
import android.webkit.WebView

class Page {
    fun bind(webView: WebView) {
        webView.settings.allowUniversalAccessFromFileURLs = true
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "allowUniversalAccessFromFileURLs") {
		t.Errorf("unexpected message: %q", findings[0].Message)
	}
}

func TestWebViewUniversalAccessFromFileUrls_KotlinSetterCall(t *testing.T) {
	findings := runRuleByName(t, "WebViewUniversalAccessFromFileUrls", `
import android.webkit.WebSettings
import android.webkit.WebView

fun configure(webView: WebView) {
    webView.settings.setAllowUniversalAccessFromFileURLs(true)
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestWebViewUniversalAccessFromFileUrls_NegativeFalse(t *testing.T) {
	findings := runRuleByName(t, "WebViewUniversalAccessFromFileUrls", `
import android.webkit.WebSettings
import android.webkit.WebView

fun configure(webView: WebView) {
    webView.settings.allowUniversalAccessFromFileURLs = false
    webView.settings.setAllowUniversalAccessFromFileURLs(false)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestWebViewUniversalAccessFromFileUrls_NegativeNoWebViewImport(t *testing.T) {
	// Same property name on an unrelated class — no WebView/WebSettings import.
	findings := runRuleByName(t, "WebViewUniversalAccessFromFileUrls", `
class Settings {
    var allowUniversalAccessFromFileURLs: Boolean = false
}

fun configure() {
    val s = Settings()
    s.allowUniversalAccessFromFileURLs = true
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestWebViewUniversalAccessFromFileUrls_JavaSetter(t *testing.T) {
	findings := runRuleByNameOnJava(t, "WebViewUniversalAccessFromFileUrls", `
import android.webkit.WebSettings;
import android.webkit.WebView;

class Page {
    void bind(WebView webView) {
        webView.getSettings().setAllowUniversalAccessFromFileURLs(true);
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 Java finding, got %d: %v", len(findings), findings)
	}
}

func TestWebViewUniversalAccessFromFileUrls_JavaNegativeFalse(t *testing.T) {
	findings := runRuleByNameOnJava(t, "WebViewUniversalAccessFromFileUrls", `
import android.webkit.WebSettings;
import android.webkit.WebView;

class Page {
    void bind(WebView webView) {
        webView.getSettings().setAllowUniversalAccessFromFileURLs(false);
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 Java findings, got %d: %v", len(findings), findings)
	}
}

func TestWebViewUniversalAccessFromFileUrls_NegativeStringLiteralLookalike(t *testing.T) {
	// The setter name appears inside a string literal — must not trigger.
	findings := runRuleByName(t, "WebViewUniversalAccessFromFileUrls", `
import android.webkit.WebSettings
import android.webkit.WebView

fun describe(): String = "webView.settings.setAllowUniversalAccessFromFileURLs(true)"
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestWebViewSettingsFindingsSpanTheWholeStatement(t *testing.T) {
	src := `
import android.webkit.WebView

fun configure(webView: WebView) {
    webView.settings.allowUniversalAccessFromFileURLs = true
    webView.settings.setAllowFileAccess(true)
}
`
	for rule, want := range map[string]string{
		"WebViewUniversalAccessFromFileUrls": "webView.settings.allowUniversalAccessFromFileURLs = true",
		"WebViewAllowFileAccess":             "webView.settings.setAllowFileAccess(true)",
	} {
		findings := runRuleByName(t, rule, src)
		if len(findings) != 1 {
			t.Fatalf("%s: expected 1 finding, got %d", rule, len(findings))
		}
		f := findings[0]
		if got := src[f.StartByte:f.EndByte]; got != want {
			t.Errorf("%s: span %q, want %q", rule, got, want)
		}
	}
}
