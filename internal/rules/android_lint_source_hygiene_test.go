package rules_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// Expected positions come from running Android Lint 32.4.1 on the same sources.

func TestConstantLocaleKotlinLintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "ConstantLocale", `package q.a

import java.util.Locale

fun foo(l: Locale) = l
val C7 = foo(Locale.getDefault())
val C8 = lazy { Locale.getDefault() }
val C9 = "x" + Locale.getDefault()
val C10 = if (C7 == null) Locale.getDefault() else Locale.US
val C11 = Locale.getDefault(Locale.Category.FORMAT)
val C12: Locale = (Locale.getDefault())
val C13 = Locale.getDefault()!!
val C14 by lazy { Locale.getDefault() }
val C15: Locale get() = Locale.getDefault()
val C16 = listOf(Locale.getDefault())
val C17 = Locale.getDefault().also { println(it) }
val C18 = run { Locale.getDefault() }
val C19: () -> Locale = { Locale.getDefault() }
val C20 = object { val x = Locale.getDefault() }
object O1 {
  val X = Locale.getDefault()
  @JvmStatic val Y = Locale.getDefault()
  var Z = Locale.getDefault()
  private val W = Locale.getDefault()
  val V by lazy { Locale.getDefault() }
}
class K2 {
  val inst = Locale.getDefault()
  fun f() { val local = Locale.getDefault() }
  companion object {
    val A: Locale = Locale.getDefault()
    val B = Locale.forLanguageTag("en")
    private val C = Locale.getDefault().toString()
    internal val D = foo(Locale.getDefault())
    val E = Locale.getDefault(); val F = 1
  }
}
`))
	if want := "6:14,8:16,9:27,10:11,11:20,12:11,15:18,16:11,21:11,22:22,24:19,31:21,33:21,34:26,35:13"; got != want {
		t.Errorf("ConstantLocale positions = %s, want %s", got, want)
	}
}

func TestConstantLocaleJavaLintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "ConstantLocale", `package q.a;
import java.util.Locale;
import java.util.List;
public class J2 {
  static final String A = Locale.getDefault().getLanguage();
  static final Locale B = foo(Locale.getDefault());
  static final Locale C = true ? Locale.getDefault() : Locale.US;
  public static final Locale D = Locale.getDefault(Locale.Category.FORMAT);
  final static Locale E = java.util.Locale.getDefault();
  static final Locale F = (Locale.getDefault());
  static final List<Locale> G = java.util.Arrays.asList(Locale.getDefault());
  static final Locale H, I = Locale.getDefault();
  static final Object[] J = { Locale.getDefault() };
  static final Runnable K = () -> Locale.getDefault();
  static final Runnable L = new Runnable() { public void run() { Locale l = Locale.getDefault(); } };
  static final Locale M = Locale.getDefault().stripExtensions();
  static { H = Locale.getDefault(); }
  interface IFace { Locale X = Locale.getDefault(); }
  enum En { A; static final Locale Y = Locale.getDefault(); }
  static class N { static final Locale Z = Locale.getDefault(); }
  static Locale foo(Locale l) { return l; }
  private static final Locale P = Locale.getDefault();
  protected static final Locale Q = Locale.getDefault(), R = Locale.getDefault();
}
`))
	if want := "5:27,6:31,7:34,8:34,9:27,10:28,11:57,12:30,13:31,16:27,18:32,19:40,20:44,22:35,23:37,23:62"; got != want {
		t.Errorf("ConstantLocale positions = %s, want %s", got, want)
	}
}

func TestNotConstructorKotlinLintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "NotConstructor", `package q.a
import android.content.Context
import android.graphics.Canvas
import android.view.View

class K4 {
  /** doc */
  @Deprecated("x")
  fun K4() {}
  fun K4(x: Int): Int = x
  private fun k4() {}
  fun <T> K4(t: T): T = t
  suspend fun K4(s: String) {}
  inline fun K4(a: Any) {}
  override fun toString() = ""
  companion object { fun K4() {} fun Companion() {} }
  inner class In { fun In() {} fun K4() {} }
  object Obj { fun Obj() {} }
}
fun K4() {}
fun K4(x: Int) {}
interface K4I { fun K4I() }
abstract class K4A { abstract fun K4A() ; open fun K4A(x: Int) {} }
enum class K4E { A; fun K4E() {} }
data class K4D(val a: Int) { fun K4D() {} }
object K4O { fun K4O() {} }
class K4G<T> { fun K4G() {} }

`))
	if want := "7:3,10:3,12:3,13:3,14:3,17:20,22:17,23:22,23:43,24:21,25:30,27:16"; got != want {
		t.Errorf("NotConstructor positions = %s, want %s", got, want)
	}
}

func TestNotConstructorJavaLintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "NotConstructor", `package q.a;
import android.content.Context;
import android.graphics.Canvas;
import android.view.View;
public class J6 {
  /** doc */
  public void J6() {}
  @Deprecated
  void J6(int a) {}
  // line comment
  public static void J6(String a) {}
  static void J6(Object a) {}
  public native void J6(long a);
  @SuppressWarnings("x") public final void J6(float a) {}
  public J6 J6(double a) { return this; }
  void j6() {}
  void J6_() {}
  void Foo() {}
  class In { void In() {} void J6() {} }
  static class SN { static void SN() {} void SN() {} }
  void m() { class Loc { void Loc() {} } }
}
class Other { void Other() {} }
class Gen<T> { void Gen() {} <R> R Gen(R r) { return r; } }
`))
	if want := "6:3,8:3,13:3,14:3,15:3,19:14,20:41,21:26,23:15,24:16,24:30"; got != want {
		t.Errorf("NotConstructor positions = %s, want %s", got, want)
	}
}

func TestCanvasSizeKotlinLintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "CanvasSize", `package q.a
import android.content.Context
import android.graphics.Canvas
import android.view.View
import android.graphics.drawable.Drawable
import android.graphics.ColorFilter

class K3(c: Context) : View(c) {
  override fun onDraw(canvas: Canvas) {
    super.onDraw(canvas)
    val w = canvas.width
    val c2 = canvas
    val h = c2.height
    val r = { canvas.width }
    canvas.apply { val x = width }
    val y = canvas.clipBounds.width()
    val z = canvas.getWidth()
    val q = canvas?.width
    val n = (canvas).height
    fun local() = canvas.width
  }
  override fun dispatchDraw(canvas: Canvas) { val w = canvas.width }
  override fun draw(canvas: Canvas) { val w = canvas.width }
  fun myDraw(canvas: Canvas) { val w = canvas.width }
  fun onDraw(canvas: Canvas, i: Int) { val w = canvas.width }
}
class K4 : Drawable() {
  override fun draw(canvas: Canvas) { val w = canvas.width }
  override fun setAlpha(a: Int) {}
  override fun setColorFilter(f: ColorFilter?) {}
  override fun getOpacity() = 0
}
class K5 { fun onDraw(canvas: Canvas) { val w = canvas.width } }
class K6(c: Context) : View(c) {
  override fun onDraw(canvas: Canvas) = Unit.also { val w = canvas.width }
}
`))
	if want := "11:13,13:13,14:15,17:13,18:13,19:13,20:19,23:47,25:48,28:47,35:61"; got != want {
		t.Errorf("CanvasSize positions = %s, want %s", got, want)
	}
}

func TestCanvasSizeNestedKotlinLintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "CanvasSize", `package q.a
import android.content.Context
import android.graphics.Canvas
import android.view.View

abstract class AbsV(c: Context) : View(c)
class V1(c: Context) : AbsV(c) {
  override fun onDraw(canvas: Canvas) { val w = canvas.width }
  fun draw() { val c = Canvas(); val w = c.width }
  fun draw(x: Int) { val c = Canvas(); val w = c.width }
}
fun anon(ctx: Context) {
  object : View(ctx) { override fun onDraw(c: Canvas) { val w = c.width } }
  object { fun onDraw(c: Canvas) { val w = c.width } }
}
class Outer(c: Context) : View(c) {
  inner class In { fun draw(c: Canvas) { val w = c.width } }
  class SIn { fun draw(c: Canvas) { val w = c.width } }
  fun foo() { val c = Canvas(); val w = c.width }
  val prop: Int get() { val c = Canvas(); return c.width }
  override fun draw(canvas: Canvas) { fun h(c: Canvas) = c.height; canvas.also { it.width } }
}
`))
	if want := "8:49,13:65,21:58"; got != want {
		t.Errorf("CanvasSize positions = %s, want %s", got, want)
	}
}

func TestCanvasSizeJavaLintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "CanvasSize", `package q.a;
import android.content.Context;
import android.graphics.Canvas;
import android.graphics.drawable.Drawable;
import android.graphics.ColorFilter;
import android.view.View;
import android.view.ViewGroup;
import android.widget.TextView;
public class J3 extends ViewGroup {
  public J3(Context c) { super(c); }
  @Override protected void onLayout(boolean c, int l, int t, int r, int b) {}
  @Override protected void onDraw(Canvas canvas) {
    super.onDraw(canvas);
    int w = canvas.getWidth();
    Canvas c2 = canvas;
    int h = c2.getHeight();
    Runnable r = () -> canvas.getWidth();
    new Runnable() { public void run() { canvas.getHeight(); } };
    int x = (canvas).getWidth();
    int y = canvas.getClipBounds().width();
    float z = canvas.getHeight() / 2f;
  }
  @Override protected void dispatchDraw(Canvas canvas) { int w = canvas.getWidth(); }
  @Override public void draw(Canvas canvas) { int w = canvas.getWidth(); }
  @Override public void onDrawForeground(Canvas canvas) { int w = canvas.getWidth(); }
  void myDraw(Canvas canvas) { int w = canvas.getWidth(); }
  void onDraw(Canvas canvas, int x) { int w = canvas.getWidth(); }
  static class D extends Drawable {
    public void draw(Canvas canvas) { int w = canvas.getWidth(); }
    public void setAlpha(int a) {}
    public void setColorFilter(ColorFilter f) {}
    public int getOpacity() { return 0; }
  }
  static class NotView {
    void onDraw(Canvas canvas) { int w = canvas.getWidth(); }
  }
  static class TV extends TextView {
    TV(Context c) { super(c); }
    @Override protected void onDraw(Canvas canvas) { int w = canvas.getWidth(); int ww = getWidth(); }
  }
  static class Inner extends View {
    Inner(Context c) { super(c); }
    protected void onDraw(Canvas canvas) {
      class Local { int f() { return canvas.getWidth(); } }
      int w = canvas.getWidth();
    }
  }
}
`))
	if want := "14:13,16:13,17:24,18:42,19:13,21:15,24:55,27:47,29:47,39:62,44:38,45:15"; got != want {
		t.Errorf("CanvasSize positions = %s, want %s", got, want)
	}
}

func TestCanvasSizeReceiversJavaLintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "CanvasSize", `package q.a;
import android.content.Context;
import android.graphics.Canvas;
import android.graphics.Bitmap;
import android.view.View;
public class J5 extends View {
  Canvas field;
  public J5(Context c) { super(c); }
  void draw(Canvas c, Canvas d) { int a = c.getWidth(); int b = d.getWidth(); }
  void draw(int z, Canvas d) { int b = d.getWidth(); }
  void onDraw(Canvas c, Canvas d) { int a = c.getWidth(); int b = d.getWidth(); }
  @Override protected void onDraw(Canvas canvas) {
    Canvas c2 = canvas;
    Canvas c3 = new Canvas(Bitmap.createBitmap(1, 1, null));
    Canvas c4 = field;
    int a = c2.getWidth();
    int b = c3.getWidth();
    int c = c4.getWidth();
    int d = field.getWidth();
    int e = canvas.getSaveCount();
    Canvas c5; c5 = canvas; int g = c5.getWidth();
    Canvas c6 = (Canvas) canvas; int h = c6.getWidth();
    Object o = canvas; int i = ((Canvas) o).getWidth();
  }
  Canvas getCanvas() { return null; }
  @Override public void draw(Canvas canvas) {
    Canvas q = new Canvas();
    q.getHeight();
    helper(canvas);
  }
  void helper(Canvas z) { z.getWidth(); }
}
`))
	if want := "11:45,11:67,16:13,17:13,18:13,19:13,21:37,22:42,23:32,28:5"; got != want {
		t.Errorf("CanvasSize positions = %s, want %s", got, want)
	}
}

func TestCanvasSizeMethodsJavaLintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "CanvasSize", `package q.a;
import android.content.Context;
import android.graphics.Canvas;
import android.view.View;
class V extends View {
  V(Context c) { super(c); }
  void onDraw() { Canvas c = new Canvas(); c.getWidth(); }
  void draw(String s) { Canvas c = new Canvas(); c.getWidth(); }
  void draw(Canvas c) { c.getWidth(); }
  void onDraw(Canvas c, Canvas d, int e) { d.getWidth(); }
  void onDrawX(Canvas c) { c.getWidth(); }
  void ondraw(Canvas c) { c.getWidth(); }
  protected void onDraw(Canvas canvas, boolean x) { c.getMaximumBitmapWidth(); canvas.getMaximumBitmapWidth(); canvas.getWidth(); canvas.getHeight(); canvas.width(); }
}
`))
	if want := "7:44,9:25,10:44,13:112,13:131"; got != want {
		t.Errorf("CanvasSize positions = %s, want %s", got, want)
	}
}

func TestCanvasSizeClassesJavaLintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "CanvasSize", `package q.a;
import android.content.Context;
import android.graphics.Canvas;
import android.graphics.Bitmap;
import android.graphics.drawable.Drawable;
import android.graphics.ColorFilter;
import android.view.View;
public class J4 {
  abstract static class AbsV extends View {
    AbsV(Context c) { super(c); }
  }
  static class V1 extends AbsV {
    V1(Context c) { super(c); }
    @Override protected void onDraw(Canvas canvas) { int w = canvas.getWidth(); }
    void draw() { Canvas c = new Canvas(Bitmap.createBitmap(1, 1, null)); int w = c.getWidth(); }
    void draw(int x) { Canvas c = new Canvas(Bitmap.createBitmap(1, 1, null)); int w = c.getWidth(); }
    void draw(Canvas c, Canvas d) { int w = d.getHeight(); }
  }
  void anon(Context ctx) {
    new View(ctx) { protected void onDraw(Canvas c) { int w = c.getWidth(); } };
    new Drawable() {
      public void draw(Canvas c) { int w = c.getWidth(); }
      public void setAlpha(int a) {}
      public void setColorFilter(ColorFilter f) {}
      public int getOpacity() { return 0; }
    };
    new Object() { void onDraw(Canvas c) { int w = c.getWidth(); } };
  }
  static class Outer extends View {
    Outer(Context c) { super(c); }
    class In { void draw(Canvas c) { int w = c.getWidth(); } }
    static class SIn { void draw(Canvas c) { int w = c.getWidth(); } }
    void foo() { Canvas c = null; int w = c.getWidth(); }
  }
}
`))
	if want := "14:62,20:63,22:44"; got != want {
		t.Errorf("CanvasSize positions = %s, want %s", got, want)
	}
}

func TestWifiManagerLeakJavaLintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "WifiManagerLeak", `package q.a;
import android.app.Activity;
import android.app.Application;
import android.app.Service;
import android.content.Context;
import android.content.ContextWrapper;
import android.net.wifi.WifiManager;
import android.view.View;
import static android.content.Context.WIFI_SERVICE;
public class J7 extends Activity {
  Context mContext; Context applicationContext; Application app; static Context sCtx;
  void a() {
    Object o1 = getSystemService(Context.WIFI_SERVICE);
    Object o2 = this.getSystemService(Context.WIFI_SERVICE);
    Object o3 = mContext.getSystemService(Context.WIFI_SERVICE);
    Object o4 = applicationContext.getSystemService(Context.WIFI_SERVICE);
    Object o5 = app.getSystemService(Context.WIFI_SERVICE);
    Object o6 = getApplication().getSystemService(Context.WIFI_SERVICE);
    Object o7 = getApplicationContext().getSystemService(Context.WIFI_SERVICE);
    Object o8 = getBaseContext().getSystemService(Context.WIFI_SERVICE);
    Object o9 = sCtx.getSystemService(Context.WIFI_SERVICE);
    Object o10 = getSystemService(WIFI_SERVICE);
    Object o11 = getSystemService(android.content.Context.WIFI_SERVICE);
    Object o12 = J7.this.getSystemService(Context.WIFI_SERVICE);
    Object o13 = getSystemService("wifi");
    String k = "wifi"; Object o14 = getSystemService(k);
    Object o15 = (WifiManager) getSystemService(Context.WIFI_SERVICE);
    Object o16 = ((Context) mContext).getSystemService(Context.WIFI_SERVICE);
    Context local = getApplicationContext(); Object o17 = local.getSystemService(Context.WIFI_SERVICE);
    Context local2 = this; Object o18 = local2.getSystemService(Context.WIFI_SERVICE);
    Object o19 = getWindow().getContext().getSystemService(Context.WIFI_SERVICE);
    Object o20 = getSystemService(Context.WIFI_SERVICE).toString();
    Object o21 = getApplicationContext().getApplicationContext().getSystemService(Context.WIFI_SERVICE);
    Object o22 = mContext.getApplicationContext().getSystemService(Context.WIFI_SERVICE);
    Object o23 = null; o23 = mContext.getSystemService(Context.WIFI_SERVICE);
    Object o24 = new View(this).getContext().getSystemService(Context.WIFI_SERVICE);
  }
  void b(Context applicationContext, Context appContext, Context ctx, Application application, Activity act, View view, Service svc) {
    Object p1 = applicationContext.getSystemService(Context.WIFI_SERVICE);
    Object p2 = appContext.getSystemService(Context.WIFI_SERVICE);
    Object p3 = ctx.getSystemService(Context.WIFI_SERVICE);
    Object p4 = application.getSystemService(Context.WIFI_SERVICE);
    Object p5 = act.getSystemService(Context.WIFI_SERVICE);
    Object p6 = view.getContext().getSystemService(Context.WIFI_SERVICE);
    Object p7 = svc.getSystemService(Context.WIFI_SERVICE);
    Object p8 = ctx.getBaseContext();
  }
  static class Svc extends Service {
    public android.os.IBinder onBind(android.content.Intent i) { return null; }
    void c() { Object q1 = getSystemService(Context.WIFI_SERVICE); }
  }
  static class App extends Application {
    void c() { Object q1 = getSystemService(Context.WIFI_SERVICE); }
  }
  static class Wrap extends ContextWrapper {
    Wrap(Context c) { super(c); }
    void c() { Object q1 = getSystemService(Context.WIFI_SERVICE); }
  }
  static class Plain {
    Context ctx;
    void c() { Object q1 = ctx.getSystemService(Context.WIFI_SERVICE); }
  }
}
`))
	if want := "13:17,14:17,18:17,20:17,22:18,23:18,24:18,27:32,31:18,32:18,36:18,43:17,44:17,45:17,50:28,57:28"; got != want {
		t.Errorf("WifiManagerLeak positions = %s, want %s", got, want)
	}
}

func TestWifiManagerPotentialLeakJavaLintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "WifiManagerPotentialLeak", `package q.a;
import android.app.Activity;
import android.app.Application;
import android.app.Service;
import android.content.Context;
import android.content.ContextWrapper;
import android.net.wifi.WifiManager;
import android.view.View;
import static android.content.Context.WIFI_SERVICE;
public class J7 extends Activity {
  Context mContext; Context applicationContext; Application app; static Context sCtx;
  void a() {
    Object o1 = getSystemService(Context.WIFI_SERVICE);
    Object o2 = this.getSystemService(Context.WIFI_SERVICE);
    Object o3 = mContext.getSystemService(Context.WIFI_SERVICE);
    Object o4 = applicationContext.getSystemService(Context.WIFI_SERVICE);
    Object o5 = app.getSystemService(Context.WIFI_SERVICE);
    Object o6 = getApplication().getSystemService(Context.WIFI_SERVICE);
    Object o7 = getApplicationContext().getSystemService(Context.WIFI_SERVICE);
    Object o8 = getBaseContext().getSystemService(Context.WIFI_SERVICE);
    Object o9 = sCtx.getSystemService(Context.WIFI_SERVICE);
    Object o10 = getSystemService(WIFI_SERVICE);
    Object o11 = getSystemService(android.content.Context.WIFI_SERVICE);
    Object o12 = J7.this.getSystemService(Context.WIFI_SERVICE);
    Object o13 = getSystemService("wifi");
    String k = "wifi"; Object o14 = getSystemService(k);
    Object o15 = (WifiManager) getSystemService(Context.WIFI_SERVICE);
    Object o16 = ((Context) mContext).getSystemService(Context.WIFI_SERVICE);
    Context local = getApplicationContext(); Object o17 = local.getSystemService(Context.WIFI_SERVICE);
    Context local2 = this; Object o18 = local2.getSystemService(Context.WIFI_SERVICE);
    Object o19 = getWindow().getContext().getSystemService(Context.WIFI_SERVICE);
    Object o20 = getSystemService(Context.WIFI_SERVICE).toString();
    Object o21 = getApplicationContext().getApplicationContext().getSystemService(Context.WIFI_SERVICE);
    Object o22 = mContext.getApplicationContext().getSystemService(Context.WIFI_SERVICE);
    Object o23 = null; o23 = mContext.getSystemService(Context.WIFI_SERVICE);
    Object o24 = new View(this).getContext().getSystemService(Context.WIFI_SERVICE);
  }
  void b(Context applicationContext, Context appContext, Context ctx, Application application, Activity act, View view, Service svc) {
    Object p1 = applicationContext.getSystemService(Context.WIFI_SERVICE);
    Object p2 = appContext.getSystemService(Context.WIFI_SERVICE);
    Object p3 = ctx.getSystemService(Context.WIFI_SERVICE);
    Object p4 = application.getSystemService(Context.WIFI_SERVICE);
    Object p5 = act.getSystemService(Context.WIFI_SERVICE);
    Object p6 = view.getContext().getSystemService(Context.WIFI_SERVICE);
    Object p7 = svc.getSystemService(Context.WIFI_SERVICE);
    Object p8 = ctx.getBaseContext();
  }
  static class Svc extends Service {
    public android.os.IBinder onBind(android.content.Intent i) { return null; }
    void c() { Object q1 = getSystemService(Context.WIFI_SERVICE); }
  }
  static class App extends Application {
    void c() { Object q1 = getSystemService(Context.WIFI_SERVICE); }
  }
  static class Wrap extends ContextWrapper {
    Wrap(Context c) { super(c); }
    void c() { Object q1 = getSystemService(Context.WIFI_SERVICE); }
  }
  static class Plain {
    Context ctx;
    void c() { Object q1 = ctx.getSystemService(Context.WIFI_SERVICE); }
  }
}
`))
	if want := "15:17,16:17,21:17,35:30,39:17,40:17,41:17,61:28"; got != want {
		t.Errorf("WifiManagerPotentialLeak positions = %s, want %s", got, want)
	}
}

func TestWifiManagerLeakKotlinLintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "WifiManagerLeak", `package q.a
import android.app.Activity
import android.app.Application
import android.content.Context
import android.net.wifi.WifiManager
import android.view.View
import androidx.core.content.getSystemService

class K8(val ctxField: Context, private val act: Activity, val app: Application) {
  var lateCtx: Context? = null
  fun a(c: Context?, cn: Context, a2: Activity, v: View) {
    val l1 = c?.getSystemService(Context.WIFI_SERVICE)
    val l2 = c!!.getSystemService(Context.WIFI_SERVICE)
    val l3 = ctxField.getSystemService(Context.WIFI_SERVICE)
    val l4 = act.getSystemService(Context.WIFI_SERVICE)
    val l5 = app.getSystemService(Context.WIFI_SERVICE)
    val l6 = lateCtx?.getSystemService(Context.WIFI_SERVICE)
    val l7 = (cn as Context).getSystemService(Context.WIFI_SERVICE)
    val l8 = cn.getSystemService(WifiManager::class.java)
    val l9 = cn.getSystemService<WifiManager>()
    val loc = cn
    val l10 = loc.getSystemService(Context.WIFI_SERVICE)
    val loc2 = cn.applicationContext
    val l11 = loc2.getSystemService(Context.WIFI_SERVICE)
    val loc3: Context = v.context
    val l12 = loc3.getSystemService(Context.WIFI_SERVICE)
    var loc4 = cn.applicationContext
    val l13 = loc4.getSystemService(Context.WIFI_SERVICE)
    val l14 = v.context.getSystemService(Context.WIFI_SERVICE)
    val l15 = cn.applicationContext?.getSystemService(Context.WIFI_SERVICE)
    val l16 = a2.applicationContext.getSystemService(Context.WIFI_SERVICE)
    val l17 = a2.application.getSystemService(Context.WIFI_SERVICE)
    val l18 = Context.WIFI_SERVICE.let { cn.getSystemService(it) }
    val l19 = cn.getSystemService(android.content.Context.WIFI_SERVICE)
    val l20: Any? = cn.getSystemService(Context.WIFI_SERVICE) as? WifiManager
    val l21 = with(cn) { getSystemService(Context.WIFI_SERVICE) }
    val l22 = cn.run { getSystemService(Context.WIFI_SERVICE) }
    val l23 = a2.run { getSystemService(Context.WIFI_SERVICE) }
    val l24 = with(a2) { this.getSystemService(Context.WIFI_SERVICE) }
    val l25 = cn.applicationContext.also { it.getSystemService(Context.WIFI_SERVICE) }
    val l26 = cn.also { it.getSystemService(Context.WIFI_SERVICE) }
  }
}
class A9 : Activity() {
  fun f() { val x = getSystemService(Context.WIFI_SERVICE); val y = this.getSystemService(Context.WIFI_SERVICE); val z = getSystemService(WIFI_SERVICE) }
}
class App9 : Application() {
  fun f() { val x = getSystemService(Context.WIFI_SERVICE) }
}
`))
	if want := "15:14,26:15,29:15,32:15,38:24,39:26,45:21,45:69,45:122"; got != want {
		t.Errorf("WifiManagerLeak positions = %s, want %s", got, want)
	}
}

func TestWifiManagerPotentialLeakKotlinLintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "WifiManagerPotentialLeak", `package q.a
import android.app.Activity
import android.app.Application
import android.content.Context
import android.net.wifi.WifiManager
import android.view.View
import androidx.core.content.getSystemService

class K8(val ctxField: Context, private val act: Activity, val app: Application) {
  var lateCtx: Context? = null
  fun a(c: Context?, cn: Context, a2: Activity, v: View) {
    val l1 = c?.getSystemService(Context.WIFI_SERVICE)
    val l2 = c!!.getSystemService(Context.WIFI_SERVICE)
    val l3 = ctxField.getSystemService(Context.WIFI_SERVICE)
    val l4 = act.getSystemService(Context.WIFI_SERVICE)
    val l5 = app.getSystemService(Context.WIFI_SERVICE)
    val l6 = lateCtx?.getSystemService(Context.WIFI_SERVICE)
    val l7 = (cn as Context).getSystemService(Context.WIFI_SERVICE)
    val l8 = cn.getSystemService(WifiManager::class.java)
    val l9 = cn.getSystemService<WifiManager>()
    val loc = cn
    val l10 = loc.getSystemService(Context.WIFI_SERVICE)
    val loc2 = cn.applicationContext
    val l11 = loc2.getSystemService(Context.WIFI_SERVICE)
    val loc3: Context = v.context
    val l12 = loc3.getSystemService(Context.WIFI_SERVICE)
    var loc4 = cn.applicationContext
    val l13 = loc4.getSystemService(Context.WIFI_SERVICE)
    val l14 = v.context.getSystemService(Context.WIFI_SERVICE)
    val l15 = cn.applicationContext?.getSystemService(Context.WIFI_SERVICE)
    val l16 = a2.applicationContext.getSystemService(Context.WIFI_SERVICE)
    val l17 = a2.application.getSystemService(Context.WIFI_SERVICE)
    val l18 = Context.WIFI_SERVICE.let { cn.getSystemService(it) }
    val l19 = cn.getSystemService(android.content.Context.WIFI_SERVICE)
    val l20: Any? = cn.getSystemService(Context.WIFI_SERVICE) as? WifiManager
    val l21 = with(cn) { getSystemService(Context.WIFI_SERVICE) }
    val l22 = cn.run { getSystemService(Context.WIFI_SERVICE) }
    val l23 = a2.run { getSystemService(Context.WIFI_SERVICE) }
    val l24 = with(a2) { this.getSystemService(Context.WIFI_SERVICE) }
    val l25 = cn.applicationContext.also { it.getSystemService(Context.WIFI_SERVICE) }
    val l26 = cn.also { it.getSystemService(Context.WIFI_SERVICE) }
  }
}
class A9 : Activity() {
  fun f() { val x = getSystemService(Context.WIFI_SERVICE); val y = this.getSystemService(Context.WIFI_SERVICE); val z = getSystemService(WIFI_SERVICE) }
}
class App9 : Application() {
  fun f() { val x = getSystemService(Context.WIFI_SERVICE) }
}
`))
	if want := "12:14,14:14,22:15,34:15,35:21,40:44,41:25"; got != want {
		t.Errorf("WifiManagerPotentialLeak positions = %s, want %s", got, want)
	}
}

func TestDiffUtilEqualsKotlinLintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "DiffUtilEquals", `package q.a
import androidx.recyclerview.widget.DiffUtil

data class DItem(val id: Int, val name: String)
class PItem(val id: Int)
open class OItem(val id: Int) { override fun equals(other: Any?) = other is OItem && other.id == id; override fun hashCode() = id }
enum class E1 { A }
sealed class S1
interface I1

class D1 : DiffUtil.ItemCallback<DItem>() {
  override fun areItemsTheSame(a: DItem, b: DItem) = a === b
  override fun areContentsTheSame(a: DItem, b: DItem) = a === b
}
class D2 : DiffUtil.ItemCallback<DItem>() {
  override fun areItemsTheSame(a: DItem, b: DItem) = a.id == b.id
  override fun areContentsTheSame(a: DItem, b: DItem) = a == b
}
class D3 : DiffUtil.ItemCallback<PItem>() {
  override fun areItemsTheSame(a: PItem, b: PItem) = a.id == b.id
  override fun areContentsTheSame(a: PItem, b: PItem) = a == b
}
class D4 : DiffUtil.ItemCallback<PItem>() {
  override fun areItemsTheSame(a: PItem, b: PItem) = a.id == b.id
  override fun areContentsTheSame(a: PItem, b: PItem): Boolean {
    return a.equals(b)
  }
}
class D5 : DiffUtil.ItemCallback<OItem>() {
  override fun areItemsTheSame(a: OItem, b: OItem) = a.id == b.id
  override fun areContentsTheSame(a: OItem, b: OItem) = a == b
}
class D6 : DiffUtil.ItemCallback<E1>() {
  override fun areItemsTheSame(a: E1, b: E1) = a == b
  override fun areContentsTheSame(a: E1, b: E1) = a == b
}
class D7 : DiffUtil.ItemCallback<S1>() {
  override fun areItemsTheSame(a: S1, b: S1) = a == b
  override fun areContentsTheSame(a: S1, b: S1) = a == b
}
class D8 : DiffUtil.ItemCallback<I1>() {
  override fun areItemsTheSame(a: I1, b: I1) = a == b
  override fun areContentsTheSame(a: I1, b: I1) = a == b
}
class D9 : DiffUtil.ItemCallback<Any>() {
  override fun areItemsTheSame(a: Any, b: Any) = a == b
  override fun areContentsTheSame(a: Any, b: Any) = a == b
}
class D10 : DiffUtil.ItemCallback<PItem>() {
  override fun areItemsTheSame(a: PItem, b: PItem) = a.id == b.id
  override fun areContentsTheSame(a: PItem, b: PItem): Boolean = a.id == b.id && a != b
}
class D11 : DiffUtil.ItemCallback<DItem>() {
  override fun areItemsTheSame(a: DItem, b: DItem) = a.id == b.id
  override fun areContentsTheSame(a: DItem, b: DItem): Boolean = a.name === b.name
}
class D12 : DiffUtil.ItemCallback<PItem>() {
  override fun areItemsTheSame(a: PItem, b: PItem) = a.id == b.id
  override fun areContentsTheSame(a: PItem, b: PItem): Boolean = a.id == b.id || a === b
}
class D13 : DiffUtil.ItemCallback<PItem>() {
  override fun areItemsTheSame(a: PItem, b: PItem) = a.id == b.id
  override fun areContentsTheSame(a: PItem, b: PItem): Boolean = if (a === b) true else a.id == b.id
}
class D14 : DiffUtil.ItemCallback<Pair<Int, Int>>() {
  override fun areItemsTheSame(a: Pair<Int, Int>, b: Pair<Int, Int>) = a == b
  override fun areContentsTheSame(a: Pair<Int, Int>, b: Pair<Int, Int>) = a == b
}
class D15 : DiffUtil.ItemCallback<List<PItem>>() {
  override fun areItemsTheSame(a: List<PItem>, b: List<PItem>) = a == b
  override fun areContentsTheSame(a: List<PItem>, b: List<PItem>) = a == b
}
`))
	if want := "13:59,21:59,26:14,43:53,47:55,55:73,59:84,63:72,71:71"; got != want {
		t.Errorf("DiffUtilEquals positions = %s, want %s", got, want)
	}
}

func TestDiffUtilEqualsJavaLintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "DiffUtilEquals", `package q.a;
import androidx.recyclerview.widget.DiffUtil;

public class J1 {
  static class Item { int id; String name; }
  static class CB extends DiffUtil.ItemCallback<Item> {
    public boolean areItemsTheSame(Item a, Item b) { return a == b; }
    public boolean areContentsTheSame(Item a, Item b) { return a == b; }
  }
  static class CB2 extends DiffUtil.ItemCallback<Item> {
    public boolean areItemsTheSame(Item a, Item b) { return a.id == b.id; }
    public boolean areContentsTheSame(Item a, Item b) { return a.equals(b); }
  }
  static class CB3 extends DiffUtil.ItemCallback<String> {
    public boolean areItemsTheSame(String a, String b) { return a == b; }
    public boolean areContentsTheSame(String a, String b) { return a == b; }
  }
  static class CB4 extends DiffUtil.ItemCallback<Integer> {
    public boolean areItemsTheSame(Integer a, Integer b) { return a == b; }
    public boolean areContentsTheSame(Integer a, Integer b) { return a == b; }
  }
  static class CB5 extends DiffUtil.ItemCallback<Item> {
    public boolean areItemsTheSame(Item a, Item b) { return a.equals(b); }
    public boolean areContentsTheSame(Item a, Item b) { return a.id == b.id && a.name.equals(b.name); }
  }
  static class CB6 extends DiffUtil.Callback {
    Item[] xs;
    public boolean areItemsTheSame(int a, int b) { return xs[a] == xs[b]; }
    public boolean areContentsTheSame(int a, int b) { return xs[a] == xs[b]; }
  }
  static class CB7 extends DiffUtil.ItemCallback<Item> {
    public boolean areItemsTheSame(Item a, Item b) { return a == b; }
    public boolean areContentsTheSame(Item a, Item b) { return a != b; }
  }

}
`))
	if want := "8:66,12:66,16:70,20:72"; got != want {
		t.Errorf("DiffUtilEquals positions = %s, want %s", got, want)
	}
}

func TestDiffUtilEqualsClassesKotlinLintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "DiffUtilEquals", `package q.a
import androidx.recyclerview.widget.DiffUtil
import android.net.Uri
import java.util.Date

abstract class Abs(val id: Int)
open class Opn(val id: Int)
class Sub1(id: Int) : Opn(id)
open class WithEq(val id: Int) { override fun equals(other: Any?) = true; override fun hashCode() = 1 }
class Sub2(id: Int) : WithEq(id)
data class Dat(val id: Int)
class Sub3(id: Int) : Opn(id) { override fun equals(other: Any?) = true; override fun hashCode() = 1 }
sealed class Sld
enum class En { A }
interface Ifc
@JvmInline value class Vc(val v: Int)
class Gen<T>(val t: T)
sealed interface SI
class Impl : Ifc
fun interface FI { fun f() }
annotation class Ann

class T1 : DiffUtil.ItemCallback<Abs>() { override fun areItemsTheSame(a: Abs, b: Abs) = true; override fun areContentsTheSame(a: Abs, b: Abs) = a == b }
class T2 : DiffUtil.ItemCallback<Opn>() { override fun areItemsTheSame(a: Opn, b: Opn) = true; override fun areContentsTheSame(a: Opn, b: Opn) = a == b }
class T3 : DiffUtil.ItemCallback<Sub1>() { override fun areItemsTheSame(a: Sub1, b: Sub1) = true; override fun areContentsTheSame(a: Sub1, b: Sub1) = a == b }
class T4 : DiffUtil.ItemCallback<WithEq>() { override fun areItemsTheSame(a: WithEq, b: WithEq) = true; override fun areContentsTheSame(a: WithEq, b: WithEq) = a == b }
class T5 : DiffUtil.ItemCallback<Sub2>() { override fun areItemsTheSame(a: Sub2, b: Sub2) = true; override fun areContentsTheSame(a: Sub2, b: Sub2) = a == b }
class T7 : DiffUtil.ItemCallback<Vc>() { override fun areItemsTheSame(a: Vc, b: Vc) = true; override fun areContentsTheSame(a: Vc, b: Vc) = a == b }
class T8 : DiffUtil.ItemCallback<Gen<Int>>() { override fun areItemsTheSame(a: Gen<Int>, b: Gen<Int>) = true; override fun areContentsTheSame(a: Gen<Int>, b: Gen<Int>) = a == b }
class T10 : DiffUtil.ItemCallback<SI>() { override fun areItemsTheSame(a: SI, b: SI) = true; override fun areContentsTheSame(a: SI, b: SI) = a == b }
class T12 : DiffUtil.ItemCallback<Uri>() { override fun areItemsTheSame(a: Uri, b: Uri) = true; override fun areContentsTheSame(a: Uri, b: Uri) = a == b }
class T13 : DiffUtil.ItemCallback<Date>() { override fun areItemsTheSame(a: Date, b: Date) = true; override fun areContentsTheSame(a: Date, b: Date) = a == b }
class T14 : DiffUtil.ItemCallback<String>() { override fun areItemsTheSame(a: String, b: String) = true; override fun areContentsTheSame(a: String, b: String) = a == b }
class T15 : DiffUtil.ItemCallback<Int>() { override fun areItemsTheSame(a: Int, b: Int) = true; override fun areContentsTheSame(a: Int, b: Int) = a == b }
class T16 : DiffUtil.ItemCallback<Opn?>() { override fun areItemsTheSame(a: Opn?, b: Opn?) = true; override fun areContentsTheSame(a: Opn?, b: Opn?) = a == b }
class T17 : DiffUtil.ItemCallback<Opn>() { override fun areItemsTheSame(a: Opn, b: Opn) = true; override fun areContentsTheSame(a: Opn, b: Opn) = a.id == b.id && a.hashCode() == b.hashCode() }
class T19 : DiffUtil.ItemCallback<Opn>() { override fun areItemsTheSame(a: Opn, b: Opn) = true; override fun areContentsTheSame(a: Opn, b: Opn): Boolean { return a == b || a.id == 1 } }
class T20 : DiffUtil.ItemCallback<Opn>() { override fun areItemsTheSame(a: Opn, b: Opn) = a == b; override fun areContentsTheSame(a: Opn, b: Opn): Boolean { return a.id == 1 } }
class T21 : DiffUtil.ItemCallback<Opn>() { override fun areItemsTheSame(a: Opn, b: Opn) = true; override fun areContentsTheSame(a: Opn, b: Opn): Boolean { return a.equals(b) || b.equals(a) } }
class T22 : DiffUtil.ItemCallback<Opn>() { override fun areItemsTheSame(a: Opn, b: Opn) = true; override fun areContentsTheSame(a: Opn, b: Opn): Boolean { return b == a } }
class T23 : DiffUtil.ItemCallback<Opn>() { override fun areItemsTheSame(a: Opn, b: Opn) = true; override fun areContentsTheSame(a: Opn, b: Opn): Boolean { return a == null } }
class T24 : DiffUtil.ItemCallback<Opn>() { override fun areItemsTheSame(a: Opn, b: Opn) = true; override fun areContentsTheSame(a: Opn, b: Opn): Boolean { return a.id == 1 || a != b } }
class T25 : DiffUtil.ItemCallback<Opn>() { override fun areItemsTheSame(a: Opn, b: Opn) = true; override fun areContentsTheSame(a: Opn, b: Opn): Boolean { return !a.equals(b) } }
class T26 : DiffUtil.ItemCallback<Opn>() { override fun areItemsTheSame(a: Opn, b: Opn) = true; override fun areContentsTheSame(a: Opn, b: Opn): Boolean { return Objects.equals(a, b) } }
class T28 : DiffUtil.ItemCallback<Opn>() { override fun areItemsTheSame(a: Opn, b: Opn) = true; override fun areContentsTheSame(a: Opn, b: Opn): Boolean { return a.id.equals(b.id) } }
class T29 : DiffUtil.ItemCallback<Opn>() { override fun areItemsTheSame(a: Opn, b: Opn) = true; override fun areContentsTheSame(a: Opn, b: Opn): Boolean { return a.toString() == b.toString() } }
class T30 : DiffUtil.ItemCallback<Opn>() { override fun areItemsTheSame(a: Opn, b: Opn) = true; override fun areContentsTheSame(a: Opn, b: Opn): Boolean { return a.id == b.id && b == a } }
`))
	if want := "23:148,24:148,25:153,29:173,35:154,37:165,39:165,39:180,40:165,43:166,47:181"; got != want {
		t.Errorf("DiffUtilEquals positions = %s, want %s", got, want)
	}
}

func TestWifiManagerLeakSkippedFromApi24(t *testing.T) {
	const source = `package q
import android.app.Activity
import android.content.Context

class A : Activity() {
  fun f(c: Context) {
    getSystemService(Context.WIFI_SERVICE)
    c.getSystemService(Context.WIFI_SERVICE)
  }
}
`
	for _, tc := range []struct {
		minSdk string
		want   string
	}{{"23", "7:5"}, {"24", ""}} {
		dir := t.TempDir()
		manifest := `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="q"><uses-sdk android:minSdkVersion="` + tc.minSdk + `"/></manifest>`
		if err := os.WriteFile(filepath.Join(dir, "AndroidManifest.xml"), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "A.kt")
		if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
		file, err := scanner.ParseFile(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		if got := findingPositions(runRuleByNameOnFile(t, "WifiManagerLeak", file)); got != tc.want {
			t.Errorf("minSdk %s: WifiManagerLeak positions = %q, want %q", tc.minSdk, got, tc.want)
		}
		if got := findingPositions(runRuleByNameOnFile(t, "WifiManagerPotentialLeak", file)); got != map[string]string{"23": "8:5", "24": ""}[tc.minSdk] {
			t.Errorf("minSdk %s: WifiManagerPotentialLeak positions = %q", tc.minSdk, got)
		}
	}
}
