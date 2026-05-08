package rules_test

import (
	"strings"
	"testing"
)

func TestWebViewFileAccessFromFileUrls_KotlinPropertyAssignment(t *testing.T) {
	findings := runRuleByName(t, "WebViewFileAccessFromFileUrls", `
import android.webkit.WebSettings
import android.webkit.WebView

class Page {
    fun bind(webView: WebView) {
        webView.settings.allowFileAccessFromFileURLs = true
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "allowFileAccessFromFileURLs") {
		t.Errorf("unexpected message: %q", findings[0].Message)
	}
}

func TestWebViewFileAccessFromFileUrls_KotlinSetterCall(t *testing.T) {
	findings := runRuleByName(t, "WebViewFileAccessFromFileUrls", `
import android.webkit.WebSettings
import android.webkit.WebView

fun configure(webView: WebView) {
    webView.settings.setAllowFileAccessFromFileURLs(true)
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestWebViewFileAccessFromFileUrls_NegativeFalse(t *testing.T) {
	findings := runRuleByName(t, "WebViewFileAccessFromFileUrls", `
import android.webkit.WebSettings
import android.webkit.WebView

fun configure(webView: WebView) {
    webView.settings.allowFileAccessFromFileURLs = false
    webView.settings.setAllowFileAccessFromFileURLs(false)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestWebViewFileAccessFromFileUrls_NegativeNoWebViewImport(t *testing.T) {
	// Same property name on an unrelated class — no WebView/WebSettings import.
	findings := runRuleByName(t, "WebViewFileAccessFromFileUrls", `
class Settings {
    var allowFileAccessFromFileURLs: Boolean = false
}

fun configure() {
    val s = Settings()
    s.allowFileAccessFromFileURLs = true
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestWebViewFileAccessFromFileUrls_JavaSetter(t *testing.T) {
	findings := runRuleByNameOnJava(t, "WebViewFileAccessFromFileUrls", `
import android.webkit.WebSettings;
import android.webkit.WebView;

class Page {
    void bind(WebView webView) {
        webView.getSettings().setAllowFileAccessFromFileURLs(true);
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 Java finding, got %d: %v", len(findings), findings)
	}
}

func TestWebViewFileAccessFromFileUrls_JavaNegativeFalse(t *testing.T) {
	findings := runRuleByNameOnJava(t, "WebViewFileAccessFromFileUrls", `
import android.webkit.WebSettings;
import android.webkit.WebView;

class Page {
    void bind(WebView webView) {
        webView.getSettings().setAllowFileAccessFromFileURLs(false);
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 Java findings, got %d: %v", len(findings), findings)
	}
}

func TestWebViewFileAccessFromFileUrls_NegativeStringLiteralLookalike(t *testing.T) {
	// The setter name appears inside a string literal — must not trigger.
	findings := runRuleByName(t, "WebViewFileAccessFromFileUrls", `
import android.webkit.WebSettings
import android.webkit.WebView

fun describe(): String = "webView.settings.setAllowFileAccessFromFileURLs(true)"
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}
