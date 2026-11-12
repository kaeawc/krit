package rules_test

import "testing"

func TestServiceCast_JavaParity(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"wrong qualified constant", `LayoutInflater a = (LayoutInflater) getSystemService(Context.WIFI_SERVICE);`, 1},
		{"wrong bare constant", `AlarmManager a = (AlarmManager) getSystemService(CONNECTIVITY_SERVICE);`, 1},
		{"wrong with receiver", `NotificationManager a = (NotificationManager) ctx.getSystemService(Context.ALARM_SERVICE);`, 1},
		{"wrong with this receiver", `NotificationManager a = (NotificationManager) this.getSystemService(Context.ALARM_SERVICE);`, 1},
		{"wrong with parenthesized call", `NotificationManager a = (NotificationManager) (getSystemService(Context.ALARM_SERVICE));`, 1},
		{"wrong qualified cast type", `Object a = (android.app.NotificationManager) getSystemService(Context.ALARM_SERVICE);`, 1},
		{"correct cast", `AlarmManager a = (AlarmManager) getSystemService(Context.ALARM_SERVICE);`, 0},
		{"string literal name", `NotificationManager a = (NotificationManager) getSystemService("alarm");`, 0},
		{"not a getSystemService call", `NotificationManager a = (NotificationManager) lookup(Context.ALARM_SERVICE);`, 0},
		{"cast of a stored object", `Object o = getSystemService(Context.ALARM_SERVICE); NotificationManager a = (NotificationManager) o;`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := runRuleByNameOnJava(t, "ServiceCast", `
package test;
import android.content.Context;
class Host extends Context {
  void run(Context ctx) {
    `+tc.body+`
  }
}`)
			if len(findings) != tc.want {
				t.Fatalf("got %d findings, want %d: %v", len(findings), tc.want, findings)
			}
		})
	}
}

func TestServiceCast_KotlinParenthesizedCallAndAny(t *testing.T) {
	findings := runRuleByName(t, "ServiceCast", `
package test
import android.content.Context
fun run(ctx: Context) {
    val a = (ctx.getSystemService(Context.ALARM_SERVICE)) as NotificationManager
    val b = ctx.getSystemService(Context.ALARM_SERVICE) as Any
}`)
	if len(findings) != 2 {
		t.Fatalf("got %d findings, want 2: %v", len(findings), findings)
	}
	if got := findings[1].Message; got != "Suspicious cast to `Object` for a `ALARM_SERVICE`: expected `AlarmManager`" {
		t.Errorf("unexpected message %q", got)
	}
}

func TestSetJavaScriptEnabled_JavaLintParity(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want int
	}{
		{"java negated false", `package test;
import android.webkit.WebSettings;
class A { void f(WebSettings s) { s.setJavaScriptEnabled(!false); } }`, 1},
		{"java constant field", `package test;
import android.webkit.WebSettings;
class A { static final boolean ON = true; void f(WebSettings s) { s.setJavaScriptEnabled(ON); } }`, 1},
		{"java local variable", `package test;
import android.webkit.WebSettings;
class A { void f(WebSettings s) { boolean on = true; s.setJavaScriptEnabled(on); } }`, 1},
		{"java parameter value", `package test;
import android.webkit.WebSettings;
class A { void f(WebSettings s, boolean on) { s.setJavaScriptEnabled(on); } }`, 0},
		{"java conditional value", `package test;
import android.webkit.WebSettings;
class A { void f(WebSettings s, boolean on) { s.setJavaScriptEnabled(on ? true : false); } }`, 0},
		{"java field receiver", `package test;
import android.webkit.WebSettings;
class A { WebSettings settings; void f() { settings.setJavaScriptEnabled(true); } }`, 1},
		{"java local receiver", `package test;
import android.webkit.WebSettings;
import android.webkit.WebView;
class A { void f(WebView w) { WebSettings s = w.getSettings(); s.setJavaScriptEnabled(true); } }`, 1},
		{"java unrelated receiver type", `package test;
import android.webkit.WebSettings;
class Other { void setJavaScriptEnabled(boolean b) {} }
class A { void f(Other o) { o.setJavaScriptEnabled(true); } }`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if n := len(runRuleByNameOnJava(t, "SetJavaScriptEnabled", tc.src)); n != tc.want {
				t.Fatalf("got %d findings, want %d", n, tc.want)
			}
		})
	}
}

func TestServiceCast_JavaLocalGetSystemService(t *testing.T) {
	findings := runRuleByNameOnJava(t, "ServiceCast", `
package test;
import android.app.NotificationManager;
import android.content.Context;
class Registry {
  Object getSystemService(String name) { return null; }
  void run() {
    NotificationManager a = (NotificationManager) getSystemService(Context.ALARM_SERVICE);
  }
}`)
	if len(findings) != 0 {
		t.Fatalf("got %d findings, want 0: %v", len(findings), findings)
	}
}

func TestServiceCast_JavaOverriddenGetSystemServiceStillChecked(t *testing.T) {
	findings := runRuleByNameOnJava(t, "ServiceCast", `
package test;
import android.app.NotificationManager;
import android.content.Context;
import android.content.ContextWrapper;
class Wrapper extends ContextWrapper {
  Wrapper(Context base) { super(base); }
  @Override public Object getSystemService(String name) { return super.getSystemService(name); }
  void run() {
    NotificationManager a = (NotificationManager) getSystemService(Context.ALARM_SERVICE);
  }
}`)
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1: %v", len(findings), findings)
	}
}

func TestSetJavaScriptEnabled_JavaLocalWebSettingsShadowsWildcardImport(t *testing.T) {
	src := `package test;
import android.webkit.*;
class WebSettings { void setJavaScriptEnabled(boolean on) {} }
class A { void f(WebSettings w) { w.setJavaScriptEnabled(true); } }`
	if n := len(runRuleByNameOnJava(t, "SetJavaScriptEnabled", src)); n != 0 {
		t.Fatalf("got %d findings, want 0", n)
	}
	platform := `package test;
import android.webkit.*;
class A { void f(WebSettings w) { w.setJavaScriptEnabled(true); } }`
	if n := len(runRuleByNameOnJava(t, "SetJavaScriptEnabled", platform)); n != 1 {
		t.Fatalf("got %d findings, want 1", n)
	}
}
