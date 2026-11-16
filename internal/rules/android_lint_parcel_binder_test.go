package rules_test

import "testing"

// Expected positions come from running Android Lint 32.4.1 on the same sources.

func TestParcelClassLoaderKotlinLintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "ParcelClassLoader", `package q

import android.os.Bundle
import android.os.Parcel
import android.os.Parcelable

class Fake { fun readBundle(): Any? = null }

class Holder(val parcel: Parcel) : Parcelable {
  val a = parcel.readParcelable<Parcelable>(null)
  val b = parcel.readParcelable<Parcelable>(Holder::class.java.classLoader)
  val c = parcel.readBundle()
  val d = parcel.readBundle(null)
  val e = parcel.readArray(null)
  val f = parcel.readValue(null)
  val g = parcel.readSerializable()
  fun h() {
    parcel.readBundle((null))
    parcel.readBundle(null as ClassLoader?)
    parcel.readBundle(loader = (null))
    this.parcel.readBundle()
  }
  fun i(p: Parcel) = p.readBundle()
  fun j(p: Parcel) {
    val b: Bundle? = p.readBundle()
    p.readSparseArray<Any>(null)
    p.readParcelableArray(null)
    p.readTypedList(mutableListOf<Bundle>(), Bundle.CREATOR)
    p.readValue(null)
    p.readArray(null)
    p.readBundle { it }
    p.readParcelable<Bundle>()
    p.readPersistableBundle()
  }
  override fun describeContents() = 0
  override fun writeToParcel(p: Parcel, f: Int) {}
}

fun parcel(p: Parcel?, f: Fake, q: Parcel) {
  p?.readBundle()
  p!!.readBundle()
  f.readBundle()
  q.readBundle(null)
  q.readBundle(loader = null)
  q.readParcelable<Bundle>(null)
  q.readParcelable(null, Bundle::class.java)
  q.readList(mutableListOf<Any>(), null)
  (q).readBundle()
  Parcel.obtain().readBundle()
  q.apply { readBundle() }
  with(q) { readValue(null) }
  q.run { readArray(null) }
  f.apply { readBundle() }
  q.readBundle(Thread.currentThread().contextClassLoader)
  val rb = q::readBundle
}
`))
	if want := "10:18,12:18,13:18,14:18,15:18,21:17,23:24,25:24,26:7,27:7,29:7,30:7,33:7,40:6,41:7,43:5,44:5,45:5,48:7,49:19,50:13,51:13,52:11"; got != want {
		t.Errorf("ParcelClassLoader positions = %s, want %s", got, want)
	}
}

func TestParcelClassLoaderJavaLintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "ParcelClassLoader", `package q;
import android.os.Bundle;
import android.os.Parcel;
import android.os.Parcelable;

public class F implements Parcelable {
  static class Fake { Object readBundle() { return null; } Object readParcelable(Object o) { return null; } }
  F(Parcel in) {
    Parcelable a = in.readParcelable(null);
    Parcelable b = in.readParcelable(F.class.getClassLoader());
    Bundle c = in.readBundle();
    Bundle d = in.readBundle(null);
    Object[] e = in.readArray(null);
    Object f = in.readValue(null);
    Parcelable[] g = in.readParcelableArray(null);
    android.util.SparseArray h = in.readSparseArray(null);
    Object i = in.readSerializable();
    Parcelable j = in.readParcelable(getClass().getClassLoader());
    Bundle k = in.readBundle(Thread.currentThread().getContextClassLoader());
  }
  void parcel(Parcel p, Fake f) {
    p.readBundle(null);
    p.readBundle((ClassLoader) null);
    p.readBundle((null));
    f.readBundle();
    f.readParcelable(null);
    p.readPersistableBundle();
    p.readPersistableBundle(null);
    p.readParcelable(null, Bundle.class);
    p.readList(null, null);
    p.readHashMap(null);
    p.readParcelableCreator(null);
    p.readTypedObject(null);
    ClassLoader cl = null;
    p.readBundle(cl);
    Parcel q = Parcel.obtain();
    q.readBundle();
    Parcel.obtain().readValue(null);
    android.os.Parcel r = p;
    r.readArray(null);
  }
  public int describeContents() { return 0; }
  public void writeToParcel(Parcel p, int f) {}
}
`))
	if want := "9:23,11:19,12:19,13:21,14:19,15:25,16:37,22:7,27:7,28:7,37:7,38:21,40:7"; got != want {
		t.Errorf("ParcelClassLoader positions = %s, want %s", got, want)
	}
}

func TestBinderGetCallingInMainThreadKotlinLintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "BinderGetCallingInMainThread", `package q

import android.app.Activity
import android.app.Fragment
import android.app.Service
import android.content.Intent
import android.os.Binder
import android.os.Bundle
import android.os.IBinder
import android.os.Parcel

open class Act : Activity() {
  val init = Binder.getCallingUid()
  init { Binder.getCallingPid() }
  override fun onCreate(b: Bundle?) {
    run { Binder.getCallingUid() }
    listOf(1).forEach { Binder.getCallingPid() }
    fun local() { Binder.getCallingUid() }
    val o = object : Runnable { override fun run() { Binder.getCallingUid() } }
    val x = android.os.Binder.getCallingUid()
    val y = Binder.getCallingUid().toLong()
    val z = if (true) Binder.getCallingUid() else 0
  }
  override fun onStart() = Binder.getCallingUid().let { }
  override fun onStop() { Binder.getCallingUid() }
  val prop: Int get() = Binder.getCallingUid()
  companion object { fun onCreate() { Binder.getCallingUid() } }
}

class Act3 : Act() { override fun onRestart() { Binder.getCallingUid() } }

class Frag : Fragment() {
  override fun onAttach(c: android.content.Context) { Binder.getCallingUid() }
  override fun onCreate(b: Bundle?) { Binder.getCallingUid() }
  override fun onResume() { Binder.getCallingUid() }
}

class Ser : Service() {
  override fun onBind(i: Intent?): IBinder? { Binder.getCallingUid(); return null }
  override fun onCreate() { Binder.getCallingPid() }
  override fun onRebind(i: Intent?) { Binder.getCallingPid() }
  override fun onStartCommand(i: Intent?, f: Int, s: Int): Int { Binder.getCallingPid(); return 0 }
}

class Plain { fun onCreate() { Binder.getCallingUid() } }
class MyBinder : Binder() { fun f() { getCallingUid() } }
class Mine { object Binder { fun getCallingUid() = 1 } }
class Own : Activity() { override fun onCreate(b: Bundle?) { Mine.Binder.getCallingUid(); android.os.Process.myUid() } }
`))
	if want := "16:11,17:25,18:19,20:13,21:13,22:23,24:28,30:49,33:55,34:39,39:47,40:29,41:39"; got != want {
		t.Errorf("BinderGetCallingInMainThread positions = %s, want %s", got, want)
	}
}

func TestBinderGetCallingInMainThreadJavaLintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "BinderGetCallingInMainThread", `package q;
import android.os.*;
import android.app.Activity;
import android.app.Fragment;
import android.app.Service;
import android.content.Intent;
import static android.os.Binder.getCallingUid;

public class J2 {
  static class A extends Activity {
    @Override protected void onCreate(Bundle b) {
      new Runnable() { public void run() { Binder.getCallingUid(); } };
      Runnable r = () -> Binder.getCallingPid();
      android.os.Binder.getCallingUid();
      getCallingUid();
      new Binder() { int x() { return getCallingUid(); } };
    }
    void onCreate(int x) { Binder.getCallingUid(); }
    class Inner { void onCreate() { Binder.getCallingUid(); } }
    @Override protected void onRestart() { Binder.getCallingUid(); }
    @Override protected void onStop() { Binder.getCallingUid(); }
  }
  static class Frag extends Fragment {
    @Override public void onAttach(android.content.Context c) { Binder.getCallingUid(); }
    @Override public void onCreate(Bundle b) { Binder.getCallingUid(); }
    @Override public void onViewCreated(android.view.View v, Bundle b) { Binder.getCallingUid(); }
    @Override public void onResume() { Binder.getCallingUid(); }
  }
  static class Ser extends Service {
    public IBinder onBind(Intent i) { return new Binder() { public boolean onTransact(int c, Parcel d, Parcel r, int f) { Binder.getCallingUid(); return true; } }; }
    @Override public void onRebind(Intent i) { Binder.getCallingPid(); }
    @Override public int onStartCommand(Intent i, int f, int s) { Binder.getCallingPid(); return 0; }
  }
  static class Plain { void onCreate() { Binder.getCallingUid(); } }
  static class MyBinder extends Binder { void f() { getCallingUid(); } }
  static class Act2 extends A { @Override protected void onStart() { Binder.getCallingUid(); } }
}
`))
	if want := "13:26,14:7,15:7,18:28,20:44,24:65,25:48,26:74,31:48,36:70"; got != want {
		t.Errorf("BinderGetCallingInMainThread positions = %s, want %s", got, want)
	}
}

func TestMutatingSharedPrefsKotlinLintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "MutatingSharedPrefs", `package q
import android.content.SharedPreferences

fun a(sp: SharedPreferences) {
  val s = sp.getStringSet("a", null)
  s?.add("safe")
  s!!.add("bang")
  val t = s
  t?.remove("alias")
  val m = s.toMutableSet()
  m.add("copy")
  val n = s?.toSet()
  (n as MutableSet<String>).add("n")
  sp.getStringSet("b", null)?.let { it.add("let") }
  sp.getStringSet("b", null)?.apply { add("apply") }
  sp.getStringSet("b", null).orEmpty().add("orEmpty")
  (sp.getStringSet("c", null) as MutableSet<String>).clear()
  val e = sp.getStringSet("e", null) ?: mutableSetOf()
  e.add("elvis")
  var v = sp.getStringSet("e", null)
  v = mutableSetOf()
  v.add("reassigned")
  s.size
  s.contains("a")
  s.forEach { println(it) }
  for (k in s) { s.remove(k) }
  s += "plus"
}
fun b(sp: SharedPreferences) {
  val s: MutableSet<String>? = sp.getStringSet("a", null)
  s?.add("typed")
  sp.getStringSet("a", null)!!.addAll(listOf("x"))
  sp.getStringSet("a", null)!!.removeAll(listOf("x"))
  sp.getStringSet("a", null)!!.retainAll(listOf("x"))
  sp.getStringSet("a", null)!!.removeIf { true }
  sp.getStringSet("a", null)!!.clear()
  sp.getStringSet("a", null)!!.addFirst("a")
}
class W(val sp: SharedPreferences) {
  val set = sp.getStringSet("a", null)
  fun f() { set!!.add("prop") }
  fun g() { val x = sp.getStringSet("a", setOf()); x.add("g") }
}

fun scope(sp: SharedPreferences) {
  val s = sp.getStringSet("a", null)!!
  s.also { it.add("also") }
  s.run { add("run") }
  with(s) { add("with") }
  s.takeIf { true }?.add("takeIf")
  s.let { x -> x.add("named") }
  s.apply { this.add("this") }
  s.let { it }.add("letreturns")
  s.also { }.add("alsoreturns")
  s.apply { }.add("applyreturns")
  s.run { this }.add("runreturns")
  val w = if (s.isEmpty()) s else mutableSetOf()
  w.add("ifexpr")
  val x = when { s.isEmpty() -> s; else -> mutableSetOf<String>() }
  x.add("when")
  val y: MutableSet<String> = s
  y.add("typedalias")
  s.forEach { s.add("inForEach") }
  val z = s.apply { }
  z.add("applyvar")
  val zz = s.also { }
  zz.add("alsovar")
  val l = s.let { it }
  l.add("letvar")
  listOf(s).first().add("list")
  val arr = arrayOf(s)
  arr[0].add("arr")
  var q = s
  q.add("var")
  q = mutableSetOf()
  q.add("varreassigned")
  val r = sp.getStringSet("r", null)
  val r2 = r.orEmpty()
  r2.add("orEmptyVar")
  r?.add("again")
}
`))
	if want := "6:3,7:3,9:3,14:37,15:39,17:3,19:3,26:18,31:3,32:3,33:3,34:3,35:3,36:3,37:3,42:52,47:12,48:11,49:13,50:3,51:16,52:13,53:3,54:3,55:3,56:3,58:3,62:3,63:15,65:3,67:3,69:3,74:3,80:3"; got != want {
		t.Errorf("MutatingSharedPrefs positions = %s, want %s", got, want)
	}
}

func TestMutatingSharedPrefsFollowsLexicalBinding(t *testing.T) {
	got := findingPositions(runRuleByName(t, "MutatingSharedPrefs", `package q
import android.content.SharedPreferences

fun shadowAssigned(sp: SharedPreferences) {
  val s = mutableSetOf<String>()
  run {
    var s = mutableSetOf<String>()
    s = sp.getStringSet("key", null)!!
    s.add("inner")
  }
  s.add("outer")
}
fun outerAssigned(sp: SharedPreferences) {
  var s = mutableSetOf<String>()
  run {
    s = sp.getStringSet("key", null)!!
  }
  s.add("outer")
}
fun shadowHidesOuter(sp: SharedPreferences) {
  var s = sp.getStringSet("key", null)!!
  run {
    var s = mutableSetOf<String>()
    s = mutableSetOf()
    s.add("inner")
  }
  s.add("outer")
}
`))
	if want := "9:5,18:3,27:3"; got != want {
		t.Errorf("MutatingSharedPrefs positions = %s, want %s", got, want)
	}
}

func TestMutatingSharedPrefsJavaLintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "MutatingSharedPrefs", `package q;
import android.content.SharedPreferences;
import java.util.*;

public class J3 {
  Set<String> field;
  void a(SharedPreferences sp) {
    Set<String> s = sp.getStringSet("a", null);
    Set<String> t = s;
    t.add("x");
    Set<String> c = new HashSet<>(s);
    c.add("y");
    s.addAll(c);
    s.removeAll(c);
    s.retainAll(c);
    s.removeIf(x -> true);
    s.contains("a");
    s.iterator().remove();
    s.size();
    foo(s);
    (sp.getStringSet("z", null)).add("q");
    sp.getStringSet("z", null).stream().count();
    sp.getStringSet("y", null).iterator().next();
  }
  void b(SharedPreferences sp) {
    Set<String> s = sp.getStringSet("a", null);
    s = new HashSet<>();
    s.add("fresh");
    Set<String> u;
    u = sp.getStringSet("a", null);
    u.add("assigned");
    field = sp.getStringSet("a", null);
    field.add("field");
    this.field.clear();
  }
  void c(SharedPreferences sp) {
    Set<String> s = sp.getStringSet("a", null);
    if (s != null) { s.add("inif"); }
    Runnable r = () -> s.add("lambda");
    for (String k : s) { s.remove(k); }
    Set<String> w = cond() ? s : new HashSet<>();
    w.add("ternary");
    Object o = s;
    ((Set<String>) o).add("cast");
    d(sp.getStringSet("k", null));
  }
  void d(Set<String> p) { p.add("param"); }
  boolean cond() { return true; }
  void foo(Set<String> s) {}
  void e(SharedPreferences.Editor ed, android.content.SharedPreferences sp2) {
    sp2.getStringSet("a", new HashSet<>()).add("x");
    sp2.getStringSet("a", null).add("x");
    getSp().getStringSet("a", null).add("m");
    String x = null;
    Collection<String> col = sp2.getStringSet("a", null);
    col.add("coll");
  }
  SharedPreferences getSp() { return null; }
  Set<String> ret(SharedPreferences sp) { Set<String> r = sp.getStringSet("a", null); r.add("ret"); return r; }
  interface Prefs { Set<String> getStringSet(String a, Set<String> b); }
  void f(Prefs p) { p.getStringSet("a", null).add("nope"); }
}
`))
	if want := "10:5,13:5,14:5,15:5,16:5,21:5,31:5,38:22,39:24,40:26,42:5,44:5,51:5,52:5,53:5,56:5,59:87"; got != want {
		t.Errorf("MutatingSharedPrefs positions = %s, want %s", got, want)
	}
}
