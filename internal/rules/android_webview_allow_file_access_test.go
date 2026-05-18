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

// TestWebViewAllowFileAccess_NegativeMultiArgOverload locks in the
// callBoolArgIsTrue arity check: a same-named custom 2-arg method that
// happens to pass `true` as its second argument must not be reported
// as an Android WebSettings violation. The pre-fix helper only looked
// at the first argument, so a call like `setAllowFileAccess(domain,
// true)` would not fire — but a flipped-argument call
// `setAllowFileAccess(true, domain)` on a WebSettings-like receiver
// would. Requiring arity == 1 closes both forms.
func TestWebViewAllowFileAccess_NegativeMultiArgOverload(t *testing.T) {
	findings := runRuleByName(t, "WebViewAllowFileAccess", `
import android.webkit.WebSettings
import android.webkit.WebView

// A custom WebSettings-shaped wrapper with a multi-arg overload.
class WebSettingsWrapper {
    var settings: WebSettingsWrapper = this
    fun setAllowFileAccess(value: Boolean, audit: String) {}
}

fun configure(webView: WebView, settings: WebSettingsWrapper) {
    settings.setAllowFileAccess(true, "audit-tag")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings on multi-arg overload, got %d: %v", len(findings), findings)
	}
}

func TestWebViewAllowFileAccess_JavaNegativeMultiArgOverload(t *testing.T) {
	findings := runRuleByNameOnJava(t, "WebViewAllowFileAccess", `
import android.webkit.WebSettings;
import android.webkit.WebView;

class WebSettingsWrapper {
    public WebSettingsWrapper getSettings() { return this; }
    public void setAllowFileAccess(boolean value, String audit) {}
}

class Page {
    void bind(WebSettingsWrapper s) {
        s.getSettings().setAllowFileAccess(true, "audit-tag");
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 Java findings on multi-arg overload, got %d: %v", len(findings), findings)
	}
}
