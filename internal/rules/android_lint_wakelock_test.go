package rules_test

import "testing"

// Expected positions come from running Android Lint 32.4.1 on the same sources.
func TestWakelockTimeoutJavaLintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "WakelockTimeout", `package com.example;
import android.content.Context;
import android.os.PowerManager;
import android.os.PowerManager.WakeLock;
public class A {
  WakeLock fld;
  void f(Context c, PowerManager pm, WakeLock p) {
    PowerManager.WakeLock local = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "t");
    local.acquire();
    local.acquire(1000);
    p.acquire();
    fld.acquire();
    pm.newWakeLock(1, "t").acquire();
    Other o = new Other();
    o.acquire();
    p.release();
  }
}
class Other { void acquire() {} }
`))
	if want := "9:5,11:5,12:5,13:5"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestWakelockTimeoutKotlinLintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "WakelockTimeout", `package com.example
import android.content.Context
import android.os.PowerManager
class B {
  var wl: PowerManager.WakeLock? = null
  fun f(c: Context, pm: PowerManager, p: PowerManager.WakeLock) {
    val local = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "t")
    local.acquire()
    local.acquire(1000L)
    p.acquire()
    wl?.acquire()
    pm.newWakeLock(1, "t").acquire()
    p.release()
  }
}
`))
	if want := "8:5,10:5,11:5,12:5"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}
