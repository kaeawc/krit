package rules_test

import (
	"strings"
	"testing"
)

func TestWebViewAllowContentAccess_KotlinPropertyAssignment(t *testing.T) {
	findings := runRuleByName(t, "WebViewAllowContentAccess", `
import android.webkit.WebSettings
import android.webkit.WebView

class Page {
    fun bind(webView: WebView) {
        webView.settings.allowContentAccess = true
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "allowContentAccess") {
		t.Errorf("unexpected message: %q", findings[0].Message)
	}
}

func TestWebViewAllowContentAccess_KotlinSetterCall(t *testing.T) {
	findings := runRuleByName(t, "WebViewAllowContentAccess", `
import android.webkit.WebSettings
import android.webkit.WebView

fun configure(webView: WebView) {
    webView.settings.setAllowContentAccess(true)
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestWebViewAllowContentAccess_NegativeFalse(t *testing.T) {
	findings := runRuleByName(t, "WebViewAllowContentAccess", `
import android.webkit.WebSettings
import android.webkit.WebView

fun configure(webView: WebView) {
    webView.settings.allowContentAccess = false
    webView.settings.setAllowContentAccess(false)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestWebViewAllowContentAccess_NegativeNoWebViewImport(t *testing.T) {
	// Same property name on an unrelated class — no WebView/WebSettings import.
	findings := runRuleByName(t, "WebViewAllowContentAccess", `
class Settings {
    var allowContentAccess: Boolean = false
}

fun configure() {
    val s = Settings()
    s.allowContentAccess = true
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestWebViewAllowContentAccess_JavaSetter(t *testing.T) {
	findings := runRuleByNameOnJava(t, "WebViewAllowContentAccess", `
import android.webkit.WebSettings;
import android.webkit.WebView;

class Page {
    void bind(WebView webView) {
        webView.getSettings().setAllowContentAccess(true);
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 Java finding, got %d: %v", len(findings), findings)
	}
}

func TestWebViewAllowContentAccess_JavaNegativeFalse(t *testing.T) {
	findings := runRuleByNameOnJava(t, "WebViewAllowContentAccess", `
import android.webkit.WebSettings;
import android.webkit.WebView;

class Page {
    void bind(WebView webView) {
        webView.getSettings().setAllowContentAccess(false);
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 Java findings, got %d: %v", len(findings), findings)
	}
}

func TestWebViewAllowContentAccess_NegativeStringLiteralLookalike(t *testing.T) {
	// The setter name appears inside a string literal — must not trigger.
	findings := runRuleByName(t, "WebViewAllowContentAccess", `
import android.webkit.WebSettings
import android.webkit.WebView

fun describe(): String = "webView.settings.setAllowContentAccess(true)"
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}
