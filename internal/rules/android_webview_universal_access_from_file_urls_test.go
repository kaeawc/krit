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
