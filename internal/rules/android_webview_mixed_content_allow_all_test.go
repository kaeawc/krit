package rules_test

import (
	"strings"
	"testing"
)

func TestWebViewMixedContentAllowAll_KotlinPropertyAssignment(t *testing.T) {
	findings := runRuleByName(t, "WebViewMixedContentAllowAll", `
import android.webkit.WebSettings
import android.webkit.WebView

class Page {
    fun bind(webView: WebView) {
        webView.settings.mixedContentMode = WebSettings.MIXED_CONTENT_ALWAYS_ALLOW
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "MIXED_CONTENT_ALWAYS_ALLOW") {
		t.Errorf("unexpected message: %q", findings[0].Message)
	}
}

func TestWebViewMixedContentAllowAll_KotlinSetterCallWithConstant(t *testing.T) {
	findings := runRuleByName(t, "WebViewMixedContentAllowAll", `
import android.webkit.WebSettings
import android.webkit.WebView

fun configure(webView: WebView) {
    webView.settings.setMixedContentMode(WebSettings.MIXED_CONTENT_ALWAYS_ALLOW)
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestWebViewMixedContentAllowAll_KotlinSetterCallWithLiteralZero(t *testing.T) {
	findings := runRuleByName(t, "WebViewMixedContentAllowAll", `
import android.webkit.WebSettings
import android.webkit.WebView

fun configure(webView: WebView) {
    webView.settings.setMixedContentMode(0)
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestWebViewMixedContentAllowAll_NegativeNeverAllow(t *testing.T) {
	findings := runRuleByName(t, "WebViewMixedContentAllowAll", `
import android.webkit.WebSettings
import android.webkit.WebView

fun configure(webView: WebView) {
    webView.settings.mixedContentMode = WebSettings.MIXED_CONTENT_NEVER_ALLOW
    webView.settings.setMixedContentMode(WebSettings.MIXED_CONTENT_COMPATIBILITY_MODE)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestWebViewMixedContentAllowAll_NegativeNoWebViewImport(t *testing.T) {
	// Same property name on an unrelated class — no WebView/WebSettings import.
	findings := runRuleByName(t, "WebViewMixedContentAllowAll", `
class Settings {
    var mixedContentMode: Int = 0
}

fun configure() {
    val s = Settings()
    s.mixedContentMode = 0
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestWebViewMixedContentAllowAll_JavaSetter(t *testing.T) {
	findings := runRuleByNameOnJava(t, "WebViewMixedContentAllowAll", `
import android.webkit.WebSettings;
import android.webkit.WebView;

class Page {
    void bind(WebView webView) {
        webView.getSettings().setMixedContentMode(WebSettings.MIXED_CONTENT_ALWAYS_ALLOW);
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 Java finding, got %d: %v", len(findings), findings)
	}
}

func TestWebViewMixedContentAllowAll_NegativeStringLiteralLookalike(t *testing.T) {
	findings := runRuleByName(t, "WebViewMixedContentAllowAll", `
import android.webkit.WebSettings
import android.webkit.WebView

fun describe(): String = "webView.settings.setMixedContentMode(WebSettings.MIXED_CONTENT_ALWAYS_ALLOW)"
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}
