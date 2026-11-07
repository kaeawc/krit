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
