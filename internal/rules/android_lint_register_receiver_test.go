package rules_test

import "testing"

// Expected lines come from running Android Lint 32.4.1 on the same sources.
func TestUnspecifiedRegisterReceiverFlagJavaLintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "UnspecifiedRegisterReceiverFlag", `package com.example;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import androidx.core.content.ContextCompat;
public class A extends android.app.Activity {
  BroadcastReceiver r;
  static final String CUSTOM = "com.example.CUSTOM";
  void f(Context c, IntentFilter param) {
    registerReceiver(r, new IntentFilter(Intent.ACTION_BOOT_COMPLETED));
    registerReceiver(r, new IntentFilter("com.example.CUSTOM"));
    registerReceiver(r, new IntentFilter(CUSTOM));
    IntentFilter f = new IntentFilter();
    f.addAction(Intent.ACTION_SCREEN_ON);
    registerReceiver(r, f);
    IntentFilter g = new IntentFilter();
    g.addAction(Intent.ACTION_SCREEN_ON);
    g.addAction("custom.action");
    registerReceiver(r, g);
    registerReceiver(r, param);
    registerReceiver(r, new IntentFilter("x"), Context.RECEIVER_EXPORTED);
    registerReceiver(r, new IntentFilter("x"), Context.RECEIVER_NOT_EXPORTED);
    registerReceiver(r, new IntentFilter("x"), 0);
    registerReceiver(null, new IntentFilter("x"));
    c.registerReceiver(r, new IntentFilter("y"));
    c.registerReceiver(r, new IntentFilter("y"), "perm", null);
    c.registerReceiver(r, new IntentFilter("y"), "perm", null, Context.RECEIVER_EXPORTED);
    ContextCompat.registerReceiver(c, r, new IntentFilter("z"), ContextCompat.RECEIVER_NOT_EXPORTED);
    ContextCompat.registerReceiver(c, r, new IntentFilter("z"), 0);
    Other o = new Other();
    o.registerReceiver(r, new IntentFilter("q"));
  }
}
class Other { void registerReceiver(BroadcastReceiver r, IntentFilter f) {} }
`))
	if want := "12:5,13:5,20:5,21:5,24:5,26:5,27:5"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestUnspecifiedRegisterReceiverFlagKotlinLintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "UnspecifiedRegisterReceiverFlag", `package com.example
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
class B : android.app.Activity() {
  val r: BroadcastReceiver? = null
  fun f(c: Context, param: IntentFilter) {
    registerReceiver(r, IntentFilter(Intent.ACTION_BOOT_COMPLETED))
    registerReceiver(r, IntentFilter("com.example.CUSTOM"))
    c.registerReceiver(r, IntentFilter("y"))
    registerReceiver(r, param)
    registerReceiver(r, IntentFilter("x"), Context.RECEIVER_EXPORTED)
    registerReceiver(r, IntentFilter().apply { addAction(Intent.ACTION_SCREEN_ON) })
  }
}
`))
	if want := "10:5,11:5,12:5"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

// #1813 review: an inline filter's addAction calls run inside the registerReceiver call and still count; an action
// that cannot be evaluated is still ignored here, as before.
func TestUnspecifiedRegisterReceiverFlagInlineFilterActions(t *testing.T) {
	got := findingPositions(runRuleByName(t, "UnspecifiedRegisterReceiverFlag", `package com.example
import android.content.BroadcastReceiver
import android.content.Intent
import android.content.IntentFilter
class C : android.app.Activity() {
  val r: BroadcastReceiver? = null
  fun f(action: String) {
    registerReceiver(r, IntentFilter().apply { addAction("com.example.CUSTOM") })
    registerReceiver(r, IntentFilter().apply { addAction(Intent.ACTION_SCREEN_ON) })
    registerReceiver(r, IntentFilter(action))
    registerReceiver(r, IntentFilter().apply { addAction(action) })
  }
}
`))
	if want := "8:5"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

// #1813 review: the addAction calls of a filter registered in an init block or a constructor are read, and an
// addAction on `this.filter` counts for the property, so a custom action added there is reported.
func TestUnspecifiedRegisterReceiverFlagConstructorsAndThisReceiver(t *testing.T) {
	got := findingPositions(runRuleByName(t, "UnspecifiedRegisterReceiverFlag", `package com.example
import android.content.BroadcastReceiver
import android.content.Intent
import android.content.IntentFilter
class C : android.app.Activity() {
  val r: BroadcastReceiver? = null
  private val pf = IntentFilter(Intent.ACTION_SCREEN_OFF)
  init {
    val f = IntentFilter(Intent.ACTION_SCREEN_OFF)
    f.addAction("com.example.CUSTOM")
    registerReceiver(r, f)
    val g = IntentFilter(Intent.ACTION_SCREEN_OFF)
    g.addAction(Intent.ACTION_SCREEN_ON)
    registerReceiver(r, g)
  }
  fun f() {
    this.pf.addAction("com.example.CUSTOM")
    registerReceiver(r, pf)
  }
}
`))
	if want := "11:5,18:5"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
	got = findingPositions(runRuleByNameOnJava(t, "UnspecifiedRegisterReceiverFlag", `package com.example;
import android.content.BroadcastReceiver;
import android.content.Intent;
import android.content.IntentFilter;
public class J extends android.app.Activity {
  BroadcastReceiver r;
  private final IntentFilter jf = new IntentFilter(Intent.ACTION_SCREEN_OFF);
  public J() {
    IntentFilter f = new IntentFilter(Intent.ACTION_SCREEN_OFF);
    f.addAction("com.example.CUSTOM");
    registerReceiver(r, f);
    IntentFilter g = new IntentFilter(Intent.ACTION_SCREEN_OFF);
    g.addAction(Intent.ACTION_SCREEN_ON);
    registerReceiver(r, g);
  }
  void f() {
    this.jf.addAction("com.example.CUSTOM");
    registerReceiver(r, jf);
  }
}
`))
	if want := "11:5,18:5"; got != want {
		t.Fatalf("java positions = %s, want %s", got, want)
	}
}

// #1813 review: a same-name local in a scope that does not enclose the registration (a lambda, an anonymous class) does
// not hide the outer local the call reads, so its custom action is reported; a local in an enclosing scope is still
// read. A property or field shadowed that way is still resolved to the sibling local, as on main (#1835).
func TestUnspecifiedRegisterReceiverFlagFilterShadowedInSiblingScope(t *testing.T) {
	got := findingPositions(runRuleByName(t, "UnspecifiedRegisterReceiverFlag", `package com.example
import android.content.BroadcastReceiver
import android.content.Intent
import android.content.IntentFilter
class C : android.app.Activity() {
  val r: BroadcastReceiver? = null
  fun f(c: Boolean) {
    val sh = IntentFilter(Intent.ACTION_SCREEN_OFF)
    sh.addAction("com.example.CUSTOM")
    run { val sh = IntentFilter(Intent.ACTION_SCREEN_OFF) }
    registerReceiver(r, sh)
    val outer = IntentFilter(Intent.ACTION_SCREEN_OFF)
    val custom = IntentFilter(Intent.ACTION_SCREEN_OFF)
    custom.addAction("com.example.CUSTOM")
    if (c) {
      registerReceiver(r, outer)
      registerReceiver(r, custom)
    }
  }
}
`))
	if want := "11:5,17:7"; got != want {
		t.Errorf("positions = %s, want %s", got, want)
	}
	got = findingPositions(runRuleByNameOnJava(t, "UnspecifiedRegisterReceiverFlag", `package com.example;
import android.content.BroadcastReceiver;
import android.content.Intent;
import android.content.IntentFilter;
public class J extends android.app.Activity {
  BroadcastReceiver r;
  void f(boolean c) {
    IntentFilter ob = new IntentFilter(Intent.ACTION_SCREEN_OFF);
    ob.addAction("com.example.CUSTOM");
    Runnable task = new Runnable() { public void run() { IntentFilter ob = new IntentFilter(Intent.ACTION_SCREEN_OFF); } };
    registerReceiver(r, ob);
    IntentFilter outer = new IntentFilter(Intent.ACTION_SCREEN_OFF);
    IntentFilter custom = new IntentFilter(Intent.ACTION_SCREEN_OFF);
    custom.addAction("com.example.CUSTOM");
    if (c) {
      registerReceiver(r, outer);
      registerReceiver(r, custom);
    }
  }
}
`))
	if want := "11:5,17:7"; got != want {
		t.Fatalf("java positions = %s, want %s", got, want)
	}
}
