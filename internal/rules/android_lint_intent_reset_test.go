package rules_test

import "testing"

// Expected positions come from running Android Lint 32.4.1 on the same sources.

func TestIntentResetJavaLintParityBasics(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "IntentReset", `package app;
import android.content.Intent;
import android.net.Uri;
public class A {
  void t1(Uri u) { Intent i = new Intent(); i.setData(u); i.setType("text/plain"); }
  void t2(Uri u) { Intent i = new Intent(); i.setType("text/plain"); i.setData(u); }
  void t3(Uri u) { Intent i = new Intent("a", u); i.setType("text/plain"); }
  void t4(Uri u) { Intent i = new Intent(); i.setDataAndType(u, "x"); }
  void t5(Uri u) { Intent i = new Intent(); i.setData(u).setType("text/plain"); }
  void t6(Uri u) { Intent i = new Intent(); i.setType("t"); i.setData(null); }
  void t7(Uri u) { Intent i = new Intent(); i.setData(u); i.setData(u); i.setType("t"); }
  void t8(Uri u, boolean b) { Intent i = new Intent(); if (b) { i.setData(u); } i.setType("t"); }
  void t9(Uri u, boolean b) { Intent i = new Intent(); if (b) { i.setData(u); } else { i.setType("t"); } }
  void t10(Uri u) { Intent i = new Intent("a", null); i.setType("t"); }
  void t11(Uri u) { Intent i = new Intent("a", u); i.setData(u); }
  void t12(Uri u) { new Intent("a", u).setType("t"); }
  void t13(Uri u) { Intent i = new Intent(); i.setType("t"); Intent j = i; j.setData(u); }
  Intent t14(Uri u) { Intent i = new Intent(); i.setType("t"); return foo(i.setData(u)); }
  Intent foo(Intent i){return i;}
  void t15(Uri u) { Intent i = new Intent(); i.setData(u); Intent j = new Intent(); j.setType("t"); }
  void t16(Uri u) { Intent i = new Intent(); i.setData(u); i.setAction("x"); i.setType("t"); }
  void t17(Uri u) { Intent i = new Intent(); i.setData(u); i.setType(null); }
}
`))
	if want := "5:61,6:72,7:53,9:58,11:75,16:40,17:78,18:77,21:80"; got != want {
		t.Errorf("IntentReset positions = %s, want %s", got, want)
	}
}

func TestIntentResetJavaLintParityFlows(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "IntentReset", `package app;
import android.content.Intent;
import android.net.Uri;
public class J {
  Intent f = new Intent();
  void j1(Uri u) { f.setData(u); f.setType("t"); }
  void j2(Uri u) { Intent i = new Intent(); i.setData(u); switch (u.hashCode()) { case 1: i.setType("t"); break; } }
  void j3(Uri u, boolean b) { Intent i = new Intent(); if (b) { i.setData(u); i.setType("t"); } }
  void j4(Uri u, boolean b) { Intent i = new Intent(); i.setData(u); if (b) i.setType("t"); }
  void j5(Uri u, boolean b) { Intent i = new Intent(); if (b) i.setData(u); else i.setType("t"); }
  void j6(Uri u, boolean b) { Intent i; i = new Intent(); i.setData(u); i.setType("t"); }
  void j7(Uri u, boolean b) { Intent i = new Intent(); i.setData(u); i = new Intent(); i.setType("t"); }
  void j8(Uri u) { Intent i = (new Intent()); ((Intent) i).setData(u); i.setType("t"); }
  void j9(Uri u) { Intent i = new Intent(); Runnable r = () -> i.setData(u); i.setType("t"); }
  void j10(Uri u) { Intent i = new Intent("a", u); i.setType("t"); i.setType("t2"); }
  void j11(Uri u) { Intent i = new Intent(); i.setData(u); i.setType("t"); i.setData(u); i.setType("t"); }
  void j12(Uri u) { Intent i = new Intent("a", u); i.setType("t"); i.setData(u); i.setType("t"); }
  void j13(Uri u) { Intent i = new Intent("a", u); i.setType("t"); i.setData(u); i.setType("t"); }
  void j14(Uri u) { Intent i = new Intent(); i.setData(u); g(i); i.setType("t"); }
  void g(Intent i) {}
  void j15(Uri u) { Intent i = new Intent(); i.setData(u); i.putExtra("a", 1).setType("t"); }
  void j16(Uri u) { Intent i = new Intent(); i.setData(u); i.setType("t".toString()); }
  void j17(Uri u) { Intent i = new Intent(); i.setData(Uri.parse("x")); i.setType("t"); }
  void j18(Uri u) { Intent i = new Intent(null, u); i.setType("t"); }
  void j19(Uri u) { Intent i = new Intent(null, null); i.setType("t"); }
  void j20(Uri u) { Intent i = new Intent(); i.setData(null); i.setType("t"); }
  void j21(Uri u) { Intent i = new Intent(); i.setType("t"); i.setData((Uri) null); }
  void j22(Uri u) { Intent i = new Intent(); i.setType("t"); i.setData((u)); }
  void j23(Uri u) { Intent i = new Intent(); i.setType("t"); i.setData(u, 1); }
}
`))
	if want := "8:81,11:75,13:74,15:54,16:62,16:92,17:54,17:84,18:54,18:84,19:68,21:79,22:62,23:75,24:55,27:64,28:64"; got != want {
		t.Errorf("IntentReset positions = %s, want %s", got, want)
	}
}

func TestIntentResetKotlinLintParityFlows(t *testing.T) {
	got := findingPositions(runRuleByName(t, "IntentReset", `package app

import android.content.Intent
import android.net.Uri

class K {
    fun k1(u: Uri) { val i = Intent(); i.setData(u); i.setType("t") }
    fun k2(u: Uri) { val i = Intent(); i.data = u; i.type = "t" }
    fun k3(u: Uri) { val i = Intent().setData(u).setType("t") }
    fun k4(u: Uri) { Intent().apply { setData(u); setType("t") } }
    fun k5(u: Uri) { val i = Intent(); i.apply { setData(u) }; i.setType("t") }
    fun k6(u: Uri) { val i = Intent("a", u); i.setType("t") }
    fun k7(u: Uri) { val i = Intent("a", u); i.type = "t" }
    fun k8(u: Uri) { val i = Intent(); i.also { it.setData(u) }; i.setType("t") }
    fun k9(u: Uri) { var i = Intent(); i.setData(u); val j = i; j.setType("t") }
    fun k10(u: Uri, b: Boolean) { val i = Intent(); if (b) i.setData(u) else i.setType("t") }
    fun k11(u: Uri, b: Boolean) { val i = Intent(); if (b) { i.setData(u); i.setType("t") } }
    fun k12(u: Uri, b: Boolean) { val i = Intent(); i.setData(u); if (b) { i.setType("t") } }
    fun k13(u: Uri, b: Boolean) { val i = Intent(); when (b) { true -> i.setData(u); false -> i.setType("t") } }
    fun k14(u: Uri?) { val i = Intent(); i.setData(u); i.setType("t") }
    fun k15(u: Uri) { val i = Intent(Intent.ACTION_VIEW, u); i.setType("t") }
    fun k16(u: Uri) { val i = Intent(); i.setType("t"); i.setData(u); i.setType("x") }
    fun k17(u: Uri) { val i = Intent(); i.setType("t"); i.setData(u); i.setType("x"); i.setData(u) }
    fun k18(u: Uri) { val i = Intent(); i.setData(u); run { i.setType("t") } }
    fun k19(u: Uri) { val i = Intent(); i.setData(u); val f = { i.setType("t") } }
    fun k20(u: Uri) { val i = Intent(); i.setData(u); i.setType("a"); i.setType("b") }
    fun k21(u: Uri) { val i = Intent(); i.setData(u); i.setDataAndType(u, "b"); i.setType("c") }
}
`))
	if want := "7:56,8:54,9:50,12:48,13:48,15:67,17:78,20:58,21:64,22:59,23:59,23:89,26:57,27:83"; got != want {
		t.Errorf("IntentReset positions = %s, want %s", got, want)
	}
}

func TestIntentResetJavaLintParityScopes(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "IntentReset", `package app;
import android.content.Intent;
import android.net.Uri;
public class J {
  Intent f;
  J(Uri u) { Intent i = new Intent(); i.setData(u); i.setType("t"); }
  void b1(Uri u) { Intent i = new Intent(); i.setData(u); i = foo(); i.setType("t"); }
  Intent foo() { return null; }
  void b2(Uri u) { Intent i = new Intent(); Intent j = i; i.setData(u); j.setType("t"); }
  void b3(Uri u) { f = new Intent(); f.setData(u); f.setType("t"); }
  void b4(Uri u) { Intent i = new Intent(); this.f = i; i.setData(u); f.setType("t"); }
  void b5(Uri u, boolean b) { Intent i = b ? new Intent() : foo(); i.setData(u); i.setType("t"); }
  void b6(Uri u) { Intent i = new Intent(); i.setData(u); { i.setType("t"); } }
  void b7(Uri u) { Intent i = new Intent(); i.setData(u); synchronized (this) { i.setType("t"); } }
  void b8(Uri u) { Intent i = new Intent(); i.setData(u); try { i.setType("t"); } finally {} }
  void b9(Uri u) { Intent i = new Intent(); try { i.setData(u); i.setType("t"); } finally {} }
  void b10(Uri u) { Intent i = new Intent(); for (int k = 0; k < 2; k++) { i.setData(u); i.setType("t"); } }
  void b11(Uri u) { Intent i = new Intent(); new Object() { void q() { i.setData(u); i.setType("t"); } }; }
  void b12(Uri u) { Runnable r = () -> { Intent i = new Intent(); i.setData(u); i.setType("t"); }; }
  void b13(Uri u) { Intent i = new Intent(); i.setData(u); foo2(i).setType("t"); }
  Intent foo2(Intent i) { return i; }
  void b14(Uri u) { Intent i = new Intent(); i.setData(u).setAction("x").setType("t"); }
  void b15(Uri u) { Intent i = new Intent(); i.setData(u); i.setClassName("a", "b").setType("t"); }
  void b16(Uri u) { Intent i = new Intent(); i.setData(u); i.setPackage("a").setType("t"); }
  void b17(Uri u) { Intent i = new Intent(); i.setData(u); i.getType(); i.setType("t"); }
  void b18(Uri u) { Intent i = new Intent(); i.setData(u); i.cloneFilter().setType("t"); }
  void b19(Uri u) { Intent i = new Intent(); i.setData(u); i.setDataAndNormalize(u); i.setType("t"); }
  void b20(Uri u) { Intent i = new Intent(); i.setData(u); i.setTypeAndNormalize("t"); }
}
`))
	if want := "6:55,9:75,12:84,16:67,17:92,18:88,19:83,22:74,23:85,24:78,25:75,26:76,27:88"; got != want {
		t.Errorf("IntentReset positions = %s, want %s", got, want)
	}
}

func TestIntentResetKotlinLintParityScopes(t *testing.T) {
	got := findingPositions(runRuleByName(t, "IntentReset", `package app

import android.content.Intent
import android.net.Uri

class K {
    val fieldInit = Intent().setData(Uri.EMPTY).setType("t")
    var other: Intent? = null
    init { val i = Intent(); i.setData(Uri.EMPTY); i.setType("t") }
    constructor(u: Uri) { val i = Intent(); i.setData(u); i.setType("t") }
    fun a1(u: Uri) { var i = Intent(); i.setData(u); i = Intent(); i.setType("t") }
    fun a2(u: Uri) { var i = Intent(); i.setData(u); i = other!!; i.setType("t") }
    fun a3(u: Uri) { val i = Intent().apply { action = "x" }; i.setData(u); i.setType("t") }
    fun a4(u: Uri) { val i = Intent().also { it.action = "x" }; i.setData(u); i.setType("t") }
    fun a5(u: Uri) { val i = Intent().let { it.setData(u) }; i.setType("t") }
    fun a6(u: Uri, b: Boolean) { val i = if (b) Intent() else Intent("a"); i.setData(u); i.setType("t") }
    fun a7(u: Uri, b: Boolean) { val i = other ?: Intent(); i.setData(u); i.setType("t") }
    fun a8(u: Uri, b: Boolean) { val f = { val i = Intent(); i.setData(u); i.setType("t") } }
    fun a9(u: Uri, b: Boolean) { run { val i = Intent(); i.setData(u); i.setType("t") } }
    fun a10(u: Uri) { val i = Intent(); other = i; i.setData(u); other!!.setType("t") }
    fun a11(u: Uri) { val i = Intent(); other = i; other!!.setData(u); other!!.setType("t") }
    fun a12(u: Uri) = Intent().setData(u).setType("t")
    fun a13(u: Uri): Intent { return Intent().apply { setData(u) }.setType("t") }
    fun a14(u: Uri) { val i = Intent(); i.setData(u); i.setType("t"); i.setData(u); i.setType("t") }
    fun a15(u: Uri) { val i = Intent(); i?.setData(u); i?.setType("t") }
    fun a16(u: Uri) { val i = Intent(); i.setData(u); i.setType(type = "t") }
    fun a17(u: Uri) { val i = Intent(); i.setData(data = u); i.setType("t") }
    fun a18(u: Uri) { val i = Intent(); i.data = u; i.setType("t") }
    fun a19(u: Uri) { val i = Intent(); i.setData(u); i.type = null }
    fun a20(u: Uri) { val i = Intent(); i.setData(u); i.setNormalizeMimeType("t") }
    fun a21(u: Uri) { val i = Intent(); i.setData(u); i.setTypeAndNormalize("t") }
    fun a22(u: Uri) { val i = Intent(); i.setDataAndNormalize(u); i.setType("t") }
    fun a23(u: Uri) { val i = Intent(); i.setData(u); object { fun f() { i.setType("t") } } }
    fun a24(u: Uri) { val i = Intent(); i.setData(u); fun g() { i.setType("t") }; g() }
    fun a25(u: Uri) { val i = Intent(); i.setData(u); (i).setType("t") }
    fun a26(u: Uri) { val i = Intent(); i.setData(u); (i as Intent).setType("t") }
    fun a27(u: Uri) { val i = Intent(); i.setData(u); i!!.setType("t") }
    fun a28(u: Uri) { val i = Intent(); val j = i.setData(u); j.setType("t") }
    fun a29(u: Uri) { val i = Intent(); val j = i; i.setData(u); j.setType("t") }
    fun a30(u: Uri) { Intent(Intent.ACTION_VIEW).setData(u).setType("t") }
    fun a31(u: Uri) { val i = Intent(); if (u != Uri.EMPTY) { i.setData(u) }; i.setType("t") }
    fun a32(u: Uri) { val i = Intent(); try { i.setData(u); i.setType("t") } catch (e: Exception) {} }
    fun a33(u: Uri) { val i = Intent(); i.setData(u); try { i.setType("t") } catch (e: Exception) {} }
    fun a34(u: Uri) { val i = Intent(); for (x in 1..2) { i.setData(u); i.setType("t") } }
    fun a35(u: Uri) { val i = Intent(); i.setData(u); while (true) { i.setType("t"); break } }
    fun a36(u: Uri) { val i = Intent(); i.setData(u); synchronized(this) { i.setType("t") } }
}
`))
	if want := "9:54,10:61,16:92,16:92,17:77,18:78,19:74,22:43,24:57,24:87,25:59,26:57,27:64,28:55,35:59,36:69,37:59,38:65,39:68,40:61,42:63,44:75"; got != want {
		t.Errorf("IntentReset positions = %s, want %s", got, want)
	}
}

func TestIntentResetJavaLintParityConstructors(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "IntentReset", `package app;
import android.content.Intent;
import android.net.Uri;
public class J {
  void c1(Uri u) { Intent i = new Intent(); i.setData(u); i.getSelector().setType("t"); }
  void c2(Intent p, Uri u) { p = new Intent(); p.setData(u); p.setType("t"); }
  void c3(Uri u) { Intent i = new Intent(); i.setData(u); if (u != null) i.setType("t"); }
  void c4(Uri u) { Intent i = new Intent(); if (u != null) i.setData(u).setType("t"); }
  void c5(Uri u) { Intent i = new Intent(); if (i.setData(u) != null) i.setType("t"); }
  void c6(Uri u) { Intent i = new Intent(); if (i.setData(u) != null && i.setType("t") != null) {} }
  void c7(Uri u, boolean b) { Intent i = new Intent(); if (b) {} else i.setData(u).setType("t"); }
  void c8(Uri u) { Intent i = new Intent(); i.setData(u); for (int k=0;k<1;k++) i.setType("t"); }
  void c9(Uri u) { Intent i = new Intent("a", Uri.parse("x")); i.setType("t"); }
  void c10(Uri u) { Intent i = new Intent("a", getU()); i.setType("t"); }
  Uri getU() { return null; }
  void c11(String s) { Intent i = new Intent("a", Uri.parse(s)); i.setData(u2()); }
  Uri u2() { return null; }
  void c12(Class<?> c, android.content.Context ctx) { Intent i = new Intent(ctx, c); i.setType("t"); i.setData(u2()); }
  void c13(Uri u) { Intent i = new Intent(null, u); i.setType("t"); }
  void c14(Uri u) { Intent i = new Intent("a", u, null, null); i.setType("t"); }
  void c15(Uri u) { Intent i = new Intent(new Intent()); i.setData(u); i.setType("t"); }
  void c16(Uri u) { Intent i = new Intent(); i.setData(u); Intent j = i.cloneFilter(); j.setType("t"); }
  void c17(Uri u) { Intent i = new Intent(); i.setData(u); i.setAction("a").setData(u); }
}
`))
	if want := "5:75,6:64,8:73,10:75,11:84,12:83,13:66,14:59,18:104,19:55,20:66,21:74,22:90"; got != want {
		t.Errorf("IntentReset positions = %s, want %s", got, want)
	}
}

func TestIntentResetKotlinLintParityConstructors(t *testing.T) {
	got := findingPositions(runRuleByName(t, "IntentReset", `package app

import android.content.Intent
import android.net.Uri

class K {
    fun d1(s: String) { val i = Intent("a", Uri.fromParts(s, s, s)); i.setType("t") }
    fun d2(s: String) { val i = Intent("a", Uri.parse(s)); i.type = "t" }
    fun d3(c: android.content.Context) { val i = Intent(c, K::class.java); i.setType("t"); i.setData(Uri.EMPTY) }
    fun d4() { val i = android.content.Intent("a", Uri.EMPTY); i.setType("t") }
    fun d5(u: Uri) { val i = Intent(Intent.ACTION_VIEW, u); i.setType("t") }
    fun d6(u: Uri) { val i = Intent(action = "a"); i.setType("t") }
    fun d7(u: Uri) { val i = Intent("a", u).setType("t") }
    fun d8(u: Uri) { val i = Intent("a", u).apply { type = "t" } }
    fun d9(u: Uri) { val i = Intent("a", u); i.type = "t"; i.data = u; i.type = "t" }
    fun d10(u: Uri) { val i = Intent("a", u); i.setType("t", ) }
    fun d11(u: Uri) { val i = Intent("a", u); i.setType(null); i.setType("t") }
    fun d12(u: Uri) { val i = Intent(); i.setData(null); i.setType("t") ; i.setData(u) }
    fun d13(u: Uri) { val i = Intent(); i.data = null; i.setType("t") ; i.data = u }
    fun d14(u: Uri) { val i = Intent(); i.setType("t"); i.data = (u) }
    fun d15(u: Uri) { val i = Intent(); i.setType("t"); i.data = u as Uri }
}
`))
	if want := "7:72,8:62,9:94,10:66,11:63,13:45,15:48,15:74,16:49,17:66,18:77,19:75,20:59,21:59"; got != want {
		t.Errorf("IntentReset positions = %s, want %s", got, want)
	}
}

func TestIntentResetIgnoresOtherIntentClass(t *testing.T) {
	got := findingPositions(runRuleByName(t, "IntentReset", `package q

import com.example.Intent
import android.net.Uri

fun f(u: Uri) {
    val i = Intent()
    i.setData(u)
    i.setType("t")
}
`))
	if got != "" {
		t.Errorf("IntentReset reported a non-Android Intent: %s", got)
	}
}

func TestIntentWithNullActionLaunchJavaLintParityBasics(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "IntentWithNullActionLaunch", `package app;
import android.app.Activity;
import android.content.Intent;
import android.net.Uri;
public class J extends Activity {
  void a1() { Intent i = new Intent(); startActivity(i); }
  void a2() { Intent i = new Intent(); i.setAction("x"); startActivity(i); }
  void a3() { Intent i = new Intent(); i.setClassName("a", "b"); startActivity(i); }
  void a4() { Intent i = new Intent(this, J.class); startActivity(i); }
  void a5() { Intent i = new Intent("a"); startActivity(i); }
  void a6() { startActivity(new Intent()); }
  void a7() { Intent i = new Intent(); i.setPackage("p"); startActivity(i); }
  void a8() { Intent i = new Intent(); i.setComponent(null); startActivity(i); }
  void a9() { Intent i = new Intent(); sendBroadcast(i); }
  void a10() { Intent i = new Intent(); startService(i); }
  void a11() { Intent i = new Intent(); i.setData(Uri.EMPTY); startActivity(i); }
  void a12() { Intent i = new Intent(); foo(i); }
  void foo(Intent i) { startActivity(i); }
  void a13() { Intent i = new Intent(); setResult(0, i); }
  void a14() { Intent i = new Intent(); i.setClass(this, J.class); startActivity(i); }
  void a15() { Intent i = new Intent(); i.putExtra("a", 1); startActivity(i); }
  void a16() { Intent i = new Intent(); if (true) i.setAction("a"); startActivity(i); }
  void a17() { Intent i = new Intent((String) null); startActivity(i); }
  void a18() { Intent i = new Intent(new Intent("a")); startActivity(i); }
  void a19() { Intent i = new Intent(); startActivityForResult(i, 1); }
  void a20() { Intent i = new Intent(); android.app.PendingIntent.getActivity(this, 0, i, 0); }
}
`))
	if want := "6:26,11:29,12:26,14:26,16:27,21:27"; got != want {
		t.Errorf("IntentWithNullActionLaunch positions = %s, want %s", got, want)
	}
}

func TestIntentWithNullActionLaunchJavaLintParityFlows(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "IntentWithNullActionLaunch", `package app;
import android.app.Activity;
import android.content.Context;
import android.content.Intent;
public class J extends Activity {
  Intent field = new Intent();
  void c1(Context c, boolean b) { Intent i = b ? new Intent() : new Intent("a"); c.startActivity(i); }
  void c2(Context c) { Intent i = new Intent(); Intent j = i; j.setAction("a"); c.startActivity(i); }
  void c3(Context c) { Intent i = new Intent(); Runnable r = () -> c.startActivity(i); }
  void c4(Context c) { c.startActivity(new Intent().setPackage("p")); }
  void c5(Context c) { new Intent().setPackage("p"); }
  void c6(Context c) { Intent i = new Intent(); i.setPackage("p"); c.sendBroadcast(i); c.sendBroadcast(i); }
  void c7(Context c) { Intent i = new Intent(); this.startActivity(i); }
  void c8(Context c) { Intent i = new Intent(); super.startActivity(i); }
  void c9(Context c) { Intent i = new Intent(); (c).startActivity(i); }
  void c10(Context c) { Intent i = new Intent(); c.startActivity((Intent) i); }
  void c11(Context c) { Intent i = new Intent(); Object o = i; c.startActivity(i); }
  void c12(Context c) { Intent i = new Intent(); new Object() { void q() { c.startActivity(i); } }; }
  void c13(Context c) { Intent i = new Intent(); i = null; c.startActivity(i); }
  void c14(Context c) { Intent i = new Intent(); Intent k = new Intent(i); c.startActivity(i); }
  void c15(Context c) { Intent i = new Intent(); i.setFlags(0).addCategory("x"); c.startActivity(i); }
  void c16(Context c) { Intent i = new Intent(); c.startActivity(i.addFlags(1)); }
  void c17(Context c) { Intent i = new Intent(); String s = i.toString(); c.startActivity(i); }
  void c18(Context c) { Intent i = new Intent(); c.startActivity(i, null); }
  void c19(Context c) { Intent i = new Intent(); this.sendBroadcast(i); }
  void c20(Context c) { Intent i = new Intent(); i.cloneFilter().setAction("a"); c.startActivity(i); }
}
`))
	if want := "7:50,9:35,10:40,12:35,13:35,14:35,15:35,16:36,17:36,18:36,21:36,22:36,23:36,24:36,25:36"; got != want {
		t.Errorf("IntentWithNullActionLaunch positions = %s, want %s", got, want)
	}
}

func TestIntentWithNullActionLaunchKotlinLintParityFlows(t *testing.T) {
	got := findingPositions(runRuleByName(t, "IntentWithNullActionLaunch", `package app

import android.app.Activity
import android.content.Context
import android.content.Intent
import android.net.Uri

class K : Activity() {
    fun b1() { val i = Intent(); startActivity(i) }
    fun b2() { val i = Intent(); i.action = "x"; startActivity(i) }
    fun b3() { val i = Intent(); i.setAction("x"); startActivity(i) }
    fun b4() { startActivity(Intent()) }
    fun b5() { startActivity(Intent().setPackage("p")) }
    fun b6() { startActivity(Intent().setClass(this, K::class.java)) }
    fun b7() { startActivity(Intent().apply { action = "x" }) }
    fun b8() { startActivity(Intent().apply { setPackage("p") }) }
    fun b9(c: Context) { val i = Intent(); c.startActivity(i) }
    fun b10(c: Context) { val i = Intent(); c.sendBroadcast(i) }
    fun b11(c: Context) { val i = Intent(); c.sendBroadcast(i, "perm") }
    fun b12(c: Context) { val i = Intent(); c.sendOrderedBroadcast(i, null) }
    fun b13(c: Context) { val i = Intent(); c.sendBroadcastAsUser(i, null) }
    fun b14(c: Context) { val i = Intent(); c.startForegroundService(i) }
    fun b15(c: Context) { val i = Intent(); val j = i; c.startActivity(j) }
    fun b16(c: Context) { val i = Intent(); i.putExtra("a", 1); c.startActivity(i) }
    fun b17(c: Context) { val i = Intent(); c.startActivity(i); c.startActivity(i, null) }
    fun b18(c: Context) { val i = Intent(); log(i); c.startActivity(i) }
    fun b19(c: Context) { val i = Intent(); c.startActivity(i); log(i) }
    fun b20(c: Context) { val i = Intent(); i.data = Uri.EMPTY; c.startActivity(i) }
    fun b21(c: Context) { val i = Intent(); i.setComponent(null); c.startActivity(i) }
    fun b22(c: Context, b: Boolean) { val i = Intent(); if (b) i.action = "a"; c.startActivity(i) }
    fun b23(c: Context, b: Boolean) { val i = if (b) Intent() else Intent("a"); c.startActivity(i) }
    fun b24(c: Context) { val i = Intent("a"); c.startActivity(i) }
    fun b25(c: Context) { val i = Intent(c, K::class.java); c.startActivity(i) }
    fun b26(c: Context) { val i = Intent(Intent()); c.startActivity(i) }
    fun b27(c: Context) { val i = Intent(); c.startActivityForResult(i, 1) }
    fun b28(c: Context) { val i = Intent(); c.startActivities(arrayOf(i)) }
    fun b29(c: Context) { val i = Intent(); Intent(i); c.startActivity(i) }
    fun b30(c: Context) { val i = Intent(); val l = listOf(i); c.startActivity(i) }
    fun b31(c: Context) { Intent().also { c.startActivity(it) } }
    fun b32(c: Context) { Intent().let { c.startActivity(it) } }
    fun b33(c: Context) { val i = Intent(); run { c.startActivity(i) } }
    fun b34(c: Context) { val i = Intent(); c.startActivity(i.setAction("a")) }
    fun b35(c: Context) { val i = Intent(); i.setPackage("a").setClass(c, K::class.java); c.startActivity(i) }
    fun b36(c: Context) { val i = Intent(); x(); c.startActivity(i) }
    fun b37(c: Context) { var i = Intent(); i = Intent("a"); c.startActivity(i) }
    fun b38(c: Context) { var i = Intent("a"); i = Intent(); c.startActivity(i) }
    fun b39(c: Context) { val i = Intent(); c.startActivity(i.also { it.action = "a" }) }
    fun b40(c: Context) { android.content.Intent().let { c.startActivity(it) } ; val q = android.content.Intent(); c.startActivity(q) }
    fun x() {}
    fun log(i: Intent) {}
    val f = Intent().also { startActivity(it) }
    fun y() = startActivity(Intent())
}
`))
	if want := "9:24,12:30,13:30,17:34,18:35,19:35,21:35,23:35,24:35,25:35,26:35,27:35,28:35,31:54,41:35,44:35,46:52,48:90,52:29"; got != want {
		t.Errorf("IntentWithNullActionLaunch positions = %s, want %s", got, want)
	}
}

func TestIntentWithNullActionLaunchJavaLintParityCalls(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "IntentWithNullActionLaunch", `package app;
import android.app.Activity;
import android.content.Context;
import android.content.Intent;
public class J extends Activity {
  void log(Intent i) {}
  static void slog(Intent i) {}
  void both(Intent i, Intent j) {}
  void ret(Intent i) { return; }
  Intent id(Intent i) { return i; }
  void d1(Context c) { Intent i = new Intent(); log(i); c.startActivity(i); }
  void d2(Context c) { Intent i = new Intent(); slog(i); c.startActivity(i); }
  void d3(Context c) { Intent i = new Intent(); this.log(i); c.startActivity(i); }
  void d4(Context c) { Intent i = new Intent(); android.util.Log.d("a", String.valueOf(i)); c.startActivity(i); }
  void d5(Context c) { Intent i = new Intent(); id(i); c.startActivity(i); }
  void d6(Context c) { Intent i = new Intent(); Intent j = id(i); c.startActivity(j); }
  void d7(Context c) { Intent i = new Intent(); unknown(i); c.startActivity(i); }
  void d8(Context c) { Intent i = new Intent(); both(i, null); c.startActivity(i); }
  void d9(Context c) { Intent i = new Intent(); Object o = foo2(i); c.startActivity(i); }
  Object foo2(Intent i) { startActivity(i); return null; }
  void d10(Context c) { Intent i = new Intent(); foo3(i); }
  void foo3(Intent i) { startActivity(i); }
  void d11(Context c) { Intent i = new Intent(); foo4(i); c.startActivity(i); }
  void foo4(Intent i) { i.setAction("a"); }
  void d12(Context c) { Intent i = new Intent(); foo5(i); }
  void foo5(Intent i) { startActivity(i); i.setAction("a"); }
  void d13(Context c) { Intent i = new Intent(); android.app.PendingIntent.getBroadcast(c, 0, i, 0); c.startActivity(i); }
  void d14(Context c) { Intent i = new Intent(); java.util.Objects.requireNonNull(i); c.startActivity(i); }
  void d15(Context c) { Intent i = new Intent(); System.out.println(i); c.startActivity(i); }
  void d16(Context c) { Intent i = new Intent(); i.equals(i); c.startActivity(i); }
  void d17(Context c) { Intent i = new Intent(); i.putExtra("a", i); c.startActivity(i); }
  void d18(Context c) { Intent i = new Intent(); c.startActivity(Intent.createChooser(i, "t")); }
}
`))
	if want := "11:35,13:35,29:36"; got != want {
		t.Errorf("IntentWithNullActionLaunch positions = %s, want %s", got, want)
	}
}

func TestIntentWithNullActionLaunchKotlinLintParitySetters(t *testing.T) {
	got := findingPositions(runRuleByName(t, "IntentWithNullActionLaunch", `package app

import android.app.Activity
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.util.Log

class K : Activity() {
    fun e1(c: Context) { val i = Intent(); Log.d("t", i.toString()); c.startActivity(i) }
    fun e2(c: Context) { val i = Intent(); println(i); c.startActivity(i) }
    fun e3(c: Context) { val i = Intent(); Log.d("t", "$i"); c.startActivity(i) }
    fun e5(c: Context) { val f = { val i = Intent(); c.startActivity(i) } }
    fun e6(c: Context) { val i = Intent(); val f = { c.startActivity(i) } }
    fun e7(c: Context) { val i = Intent(); try { c.startActivity(i) } catch (e: Exception) {} }
    fun e8(c: Context) { val i = Intent(); when (1) { 1 -> c.startActivity(i) } }
    fun e9(c: Context) { val i = Intent(); i.component = null; c.startActivity(i) }
    fun e10(c: Context) { val i = Intent(); i.`+"`"+`package`+"`"+` = "p"; c.startActivity(i) }
    fun e11(c: Context) { val i = Intent(); i.flags = 1; c.startActivity(i) }
    fun e12(c: Context) { val i = Intent(); i.data = Uri.EMPTY; i.type = "t"; c.startActivity(i) }
    fun e13(c: Context) { val i = Intent(); i.setClassName(c, "x"); c.startActivity(i) }
    fun e14(c: Context) { val i = Intent(); i.setClassName("a", "x"); c.startActivity(i) }
    fun e15(c: Context) { val i = Intent(); i.setComponent(android.content.ComponentName("a","b")); c.startActivity(i) }
    fun e16(c: Context) { val i = Intent(); c.startActivity(i.setClassName("a", "x")) }
    fun e17(c: Context) { val i = Intent(); val j = i.setPackage("p"); j.setAction("a"); c.startActivity(i) }
    fun e18(c: Context) { val i = Intent(); c.startActivity(i); i.setAction("a") }
    fun e19(c: Context) { Intent().run { c.startActivity(this) } }
    fun e20(c: Context) { val i = Intent(); c.sendBroadcast(i); c.startActivity(Intent("a")) }
    fun e21(c: Context) { val i = Intent(); i.setAction(null); c.startActivity(i) }
    fun e22(c: Context) { val i = Intent(); i.action = null; c.startActivity(i) }
    fun e23(c: Context) { val i = Intent(); i!!.setPackage("p"); (i as Intent).setPackage("q"); c.startActivity(i) }
    fun e24(c: Context) { val i = Intent(); c.startActivity(i, null) }
    fun e25(c: Context) { val i = Intent(); Log.d("t", i.action ?: "none"); c.startActivity(i) }
    fun e26(c: Context) { val i = Intent(); c.startActivity(i); c.startActivity(i) }
    fun e27(c: Context) { val i: Intent = Intent(); c.startActivity(i) }
    fun e28(c: Context) { val i = Intent() ; val k = i ; c.startActivity(k); k.setAction("a") }
}
`))
	if want := "10:34,11:34,12:34,13:44,14:34,15:34,16:34,18:35,19:35,20:35,28:35,31:35,32:35,33:35,34:35,35:43"; got != want {
		t.Errorf("IntentWithNullActionLaunch positions = %s, want %s", got, want)
	}
}
