package rules_test

import (
	"strings"
	"testing"
)

func TestWebViewAllowFileAccess_KotlinPropertyAssignment(t *testing.T) {
	findings := runRuleByName(t, "WebViewAllowFileAccess", `
import android.webkit.WebSettings
import android.webkit.WebView

class Page {
    fun bind(webView: WebView) {
        webView.settings.allowFileAccess = true
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "allowFileAccess") {
		t.Errorf("unexpected message: %q", findings[0].Message)
	}
}

func TestWebViewAllowFileAccess_KotlinSetterCall(t *testing.T) {
	findings := runRuleByName(t, "WebViewAllowFileAccess", `
import android.webkit.WebSettings
import android.webkit.WebView

fun configure(webView: WebView) {
    webView.settings.setAllowFileAccess(true)
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestWebViewAllowFileAccess_NegativeFalse(t *testing.T) {
	findings := runRuleByName(t, "WebViewAllowFileAccess", `
import android.webkit.WebSettings
import android.webkit.WebView

fun configure(webView: WebView) {
    webView.settings.allowFileAccess = false
    webView.settings.setAllowFileAccess(false)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestWebViewAllowFileAccess_NegativeNoWebViewImport(t *testing.T) {
	// Same property name on an unrelated class — no WebView/WebSettings import.
	findings := runRuleByName(t, "WebViewAllowFileAccess", `
class Settings {
    var allowFileAccess: Boolean = false
}

fun configure() {
    val s = Settings()
    s.allowFileAccess = true
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestWebViewAllowFileAccess_JavaSetter(t *testing.T) {
	findings := runRuleByNameOnJava(t, "WebViewAllowFileAccess", `
import android.webkit.WebSettings;
import android.webkit.WebView;

class Page {
    void bind(WebView webView) {
        webView.getSettings().setAllowFileAccess(true);
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 Java finding, got %d: %v", len(findings), findings)
	}
}

func TestWebViewAllowFileAccess_JavaNegativeFalse(t *testing.T) {
	findings := runRuleByNameOnJava(t, "WebViewAllowFileAccess", `
import android.webkit.WebSettings;
import android.webkit.WebView;

class Page {
    void bind(WebView webView) {
        webView.getSettings().setAllowFileAccess(false);
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 Java findings, got %d: %v", len(findings), findings)
	}
}

func TestWebViewAllowFileAccess_NegativeStringLiteralLookalike(t *testing.T) {
	// The setter name appears inside a string literal — must not trigger.
	findings := runRuleByName(t, "WebViewAllowFileAccess", `
import android.webkit.WebSettings
import android.webkit.WebView

fun describe(): String = "webView.settings.setAllowFileAccess(true)"
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}
