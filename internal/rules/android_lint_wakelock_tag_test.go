package rules_test

import "testing"

// Expected positions come from running Android Lint 32.4.1 on the same sources.
func TestInvalidWakeLockTagKotlinLintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "InvalidWakeLockTag", `package p

import android.content.Context
import android.content.SharedPreferences
import android.os.PowerManager

const val CT = "ct"
const val CT2 = "ct2:ok"

class K(private val prefs: SharedPreferences, private val ctx: Context) {
  val fieldTag = "ft"
  fun wl(pm: PowerManager, p: String) {
    pm.newWakeLock(1, "t")
    pm.newWakeLock(1, "myapp:tag")
    pm.newWakeLock(1, "")
    pm.newWakeLock(1, "*foo*")
    pm.newWakeLock(1, "*")
    pm.newWakeLock(1, "**")
    pm.newWakeLock(1, "*a")
    pm.newWakeLock(1, CT)
    pm.newWakeLock(1, CT2)
    pm.newWakeLock(1, p)
    pm.newWakeLock(1, "$p:x")
    pm.newWakeLock(1, "a" + ":b")
    pm.newWakeLock(1, "a" + p)
    pm.newWakeLock(1, fieldTag)
    pm.newWakeLock(1, tag = "named")
    pm.newWakeLock(flags = 1, tag = "named2")
    pm.newWakeLock(1, """raw""")
    pm.newWakeLock(1, ":")
    pm.newWakeLock(1)
    val local = "loc"
    pm.newWakeLock(1, local)
    var mut = "m"
    pm.newWakeLock(1, mut)
    Other().newWakeLock(1, "zz")
  }
  fun sp() {
    prefs.edit().putString("a", "b").commit()
    prefs.edit().commit()
    val ok1 = prefs.edit().putString("a", "b").commit()
    if (prefs.edit().putString("a", "b").commit()) { }
    val e = prefs.edit()
    e.putInt("x", 1)
    e.commit()
    val e2 = prefs.edit()
    e2.putInt("x", 1)
    val ok2 = e2.commit()
    val e3 = prefs.edit()
    e3.putInt("x", 1)
    e3.apply()
    e3.commit()
    prefs.edit().putString("a", "b").apply()
    prefs.edit().apply { putString("a", "b") }.commit()
    prefs.edit().also { it.putString("a", "b") }.commit()
    foo(prefs.edit().putString("a", "b").commit())
    val z = 1;
    (prefs.edit()).commit()
    prefs.edit().putString("a", "b").commit().let { }
    prefs.edit().run { commit() }
    println(prefs.edit().commit())
    val l = { prefs.edit().commit() }
    val l2 = { prefs.edit().commit(); 1 }
    if (ok1) prefs.edit().commit()
    if (ok1) { prefs.edit().commit() } else { prefs.edit().commit() }
    when (ok1) { true -> prefs.edit().commit() else -> {} }
    while (!prefs.edit().commit()) { }
    do { } while (prefs.edit().commit())
    for (i in 0..1) prefs.edit().commit()
    try { prefs.edit().commit() } finally { }
    ctx.getSharedPreferences("a", 0).edit().commit()
    ctx.getSharedPreferences("a", 0).edit().putInt("a", 1).commit()
    other().commit()
    ctx.getSharedPreferences("a", 0).edit { putInt("a", 1) }
    Fake().edit().commit()
  }
  fun r() = prefs.edit().commit()
  fun r2(): Boolean { return prefs.edit().commit() }
  fun r3(): Boolean { prefs.edit().commit(); return true }
  fun foo(b: Boolean) {}
  fun other() = Fake().edit()
}
class Fake { fun edit() = Ed(); fun commit() = true }
class Ed { fun commit() = true }
class Other { fun newWakeLock(a: Int, b: String) {} }
`))
	if want := "13:5,15:5,16:5,17:5,18:5,19:5,20:5,26:5,27:5,28:5,29:5,33:5,35:5"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestInvalidWakeLockTagJavaLintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "InvalidWakeLockTag", `package p;

import android.content.Context;
import android.content.SharedPreferences;
import android.os.PowerManager;

public class J {
  static final String CT = "ct";
  static final String CT2 = "ct2:ok";
  final String field = "ff";
  void wl(PowerManager pm, String p) {
    pm.newWakeLock(1, "t");
    pm.newWakeLock(1, "myapp:tag");
    pm.newWakeLock(1, "");
    pm.newWakeLock(1, "*foo*");
    pm.newWakeLock(1, "*");
    pm.newWakeLock(1, CT);
    pm.newWakeLock(1, CT2);
    pm.newWakeLock(1, p);
    pm.newWakeLock(1, "a" + ":b");
    pm.newWakeLock(1, "a" + p);
    pm.newWakeLock(1, field);
    pm.newWakeLock(1);
    String local = "loc";
    pm.newWakeLock(1, local);
  }
  void sp(SharedPreferences prefs, Context ctx) {
    prefs.edit().putString("a", "b").commit();
    boolean ok = prefs.edit().putString("a", "b").commit();
    if (prefs.edit().commit()) { }
    SharedPreferences.Editor e = prefs.edit();
    e.putInt("x", 1);
    e.commit();
    SharedPreferences.Editor e2 = prefs.edit();
    boolean r = e2.commit();
    prefs.edit().apply();
    ctx.getSharedPreferences("a", 0).edit().commit();
    foo(prefs.edit().commit());
    Runnable rn = () -> prefs.edit().commit();
    Runnable rn2 = () -> { prefs.edit().commit(); };
    if (ok) prefs.edit().commit();
    for (;;) prefs.edit().commit();
    try { prefs.edit().commit(); } finally { }
    new Fake().edit().commit();
  }
  boolean ret(SharedPreferences prefs) { return prefs.edit().commit(); }
  void foo(boolean b) {}
}
class Fake { Ed edit() { return null; } }
class Ed { boolean commit() { return true; } }
`))
	if want := "12:5,14:5,15:5,16:5,17:5,22:5,25:5"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}
