package rules_test

import "testing"

func TestWebViewDebuggingEnabled_KotlinPositive(t *testing.T) {
	findings := runRuleByName(t, "WebViewDebuggingEnabled", `
import android.webkit.WebView

class App {
    fun onCreate() {
        WebView.setWebContentsDebuggingEnabled(true)
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestWebViewDebuggingEnabled_KotlinNegatives(t *testing.T) {
	tests := []string{
		`
import android.webkit.WebView

fun configure() {
    if (BuildConfig.DEBUG) {
        WebView.setWebContentsDebuggingEnabled(true)
    }
}
`,
		`
import android.content.pm.ApplicationInfo
import android.webkit.WebView

fun configure(applicationInfo: ApplicationInfo) {
    if ((applicationInfo.flags and ApplicationInfo.FLAG_DEBUGGABLE) != 0) {
        WebView.setWebContentsDebuggingEnabled(true)
    }
}
`,
		`
import android.webkit.WebView

fun configure() {
    WebView.setWebContentsDebuggingEnabled(BuildConfig.DEBUG)
}
`,
		`
class WebView {
    companion object {
        fun setWebContentsDebuggingEnabled(enabled: Boolean) {}
    }
}

fun configure() {
    WebView.setWebContentsDebuggingEnabled(true)
}
`,
	}
	for _, code := range tests {
		findings := runRuleByName(t, "WebViewDebuggingEnabled", code)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v\n%s", len(findings), findings, code)
		}
	}
}

func TestWebViewDebuggingEnabled_KotlinElseBranchStillReports(t *testing.T) {
	findings := runRuleByName(t, "WebViewDebuggingEnabled", `
import android.webkit.WebView

fun configure() {
    if (BuildConfig.DEBUG) {
        println("debug")
    } else {
        WebView.setWebContentsDebuggingEnabled(true)
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for else branch, got %d: %v", len(findings), findings)
	}
}

func TestWebViewDebuggingEnabled_JavaPositive(t *testing.T) {
	findings := runRuleByNameOnJava(t, "WebViewDebuggingEnabled", `
import android.webkit.WebView;

class App {
    void onCreate() {
        WebView.setWebContentsDebuggingEnabled(true);
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 Java finding, got %d: %v", len(findings), findings)
	}
}

func TestWebViewDebuggingEnabled_JavaNegatives(t *testing.T) {
	tests := []string{
		`
import android.webkit.WebView;

class App {
    void configure() {
        if (BuildConfig.DEBUG) {
            WebView.setWebContentsDebuggingEnabled(true);
        }
    }
}
`,
		`
import android.content.pm.ApplicationInfo;
import android.webkit.WebView;

class App {
    void configure(ApplicationInfo applicationInfo) {
        if ((applicationInfo.flags & ApplicationInfo.FLAG_DEBUGGABLE) != 0) {
            WebView.setWebContentsDebuggingEnabled(true);
        }
    }
}
`,
		`
import android.webkit.WebView;

class App {
    void configure() {
        WebView.setWebContentsDebuggingEnabled(BuildConfig.DEBUG);
    }
}
`,
		`
class WebView {
    static void setWebContentsDebuggingEnabled(boolean enabled) {}
}

class App {
    void configure() {
        WebView.setWebContentsDebuggingEnabled(true);
    }
}
`,
	}
	for _, code := range tests {
		findings := runRuleByNameOnJava(t, "WebViewDebuggingEnabled", code)
		if len(findings) != 0 {
			t.Fatalf("expected 0 Java findings, got %d: %v\n%s", len(findings), findings, code)
		}
	}
}
