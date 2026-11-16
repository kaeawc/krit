package rules_test

import "testing"

// Expected positions come from running Android Lint 32.4.1 on the same sources.

func TestNotificationId0KotlinLintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "NotificationId0", `package q

import android.app.Notification
import android.app.Service
import android.content.Context
import android.content.Intent
import android.os.IBinder
import androidx.core.app.ServiceCompat

const val TOP_ID = 0
const val NONZERO = 5

class Svc1 : Service() {
    companion object { const val CID = 0 }
    private val FIELD_ID = 0
    private var mutableField = 0
    override fun onBind(i: Intent?): IBinder? = null
    override fun onCreate() {
        startForeground(0, Notification())
        startForeground(0, Notification(), 1)
        startForeground(NONZERO, Notification())
        startForeground(TOP_ID, Notification())
        startForeground(CID, Notification())
        startForeground(FIELD_ID, Notification())
        startForeground(mutableField, Notification())
        this.startForeground(0, Notification())
        startForeground(id = 0, notification = Notification())
        startForeground(notification = Notification(), id = 0)
        val zero = 0
        startForeground(zero, Notification())
        var unchanged = 0
        startForeground(unchanged, Notification())
        startForeground(0x0, Notification())
        startForeground(1 - 1, Notification())
        startForeground((0), Notification())
        startForeground(0L, Notification())
        startForeground(0L.toInt(), Notification())
        startForeground(Companion.CID, Notification())
        startForeground(Svc1.CID, Notification())
        startForeground(compute(), Notification())
        ServiceCompat.startForeground(this, 0, Notification(), 1)
        run { startForeground(0, Notification()) }
        object : Runnable { override fun run() { startForeground(0, Notification()) } }
    }
    fun compute() = 0
    inner class Inner { fun go() { startForeground(0, Notification()) } }
}

class NotService {
    fun startForeground(id: Int, n: Notification) {}
    fun go() {
        startForeground(0, Notification())
    }
    fun go2(s: Service, c: Context) {
        s.startForeground(0, Notification())
        (c as Service).startForeground(0, Notification())
    }
}
`))
	if want := "19:25,20:25,22:25,23:25,24:25,26:30,27:30,30:25,32:25,33:25,34:25,35:25,37:25,38:25,39:25,42:31,43:66,46:52,55:27,56:40"; got != want {
		t.Errorf("NotificationId0 positions = %s, want %s", got, want)
	}
}

func TestNotificationId0JavaLintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "NotificationId0", `package q;

import android.app.Notification;
import android.app.Service;
import android.content.Intent;
import android.os.IBinder;

public class J extends Service {
    static final int ZERO = 0;
    final int inst = 0;
    int mut = 0;
    @Override public IBinder onBind(Intent i) { return null; }
    @Override public void onCreate() {
        startForeground(0, new Notification());
        startForeground(ZERO, new Notification());
        startForeground(J.ZERO, new Notification());
        startForeground(inst, new Notification());
        startForeground(mut, new Notification());
        this.startForeground(0, new Notification());
        startForeground(5, new Notification());
        int z = 0;
        startForeground(z, new Notification());
        startForeground(0L, new Notification());
        new Runnable() { public void run() { startForeground(0, new Notification()); } };
    }
    static class Plain {
        void startForeground(int id, Notification n) {}
        void go() { startForeground(0, null); }
    }
}
`))
	if want := "14:25,15:25,16:25,17:25,19:30,22:25,24:62"; got != want {
		t.Errorf("NotificationId0 positions = %s, want %s", got, want)
	}
}

func TestGetContentDescriptionOverrideKotlinLintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "GetContentDescriptionOverride", `package q

import android.content.Context
import android.view.View
import android.widget.ImageButton
import android.widget.LinearLayout
import android.widget.TextView

open class MyView(c: Context) : View(c) {
    override fun getContentDescription(): CharSequence = "x"
    companion object { fun getContentDescription(): CharSequence = "comp" }
}
class MyView2(c: Context) : MyView(c) {
    override fun getContentDescription(): CharSequence = "y"
}
class MyTv(c: Context) : TextView(c) {
    override fun getContentDescription(): CharSequence? = null
    fun getContentDescription(x: Int): CharSequence? = null
}
class MyButton(c: Context) : ImageButton(c) {
    override fun getContentDescription(): CharSequence = "b"
}
interface Marker
class Row(c: Context) : LinearLayout(c), Marker {
    override fun getContentDescription(): CharSequence = "row"
}
class NotAView : Marker {
    fun getContentDescription(): CharSequence = "n"
}
class Wrapper(val c: Context) {
    val v = object : View(c) {
        override fun getContentDescription(): CharSequence = "anon"
    }
    inner class InnerView : View(c) {
        override fun getContentDescription(): CharSequence = "inner"
    }
    fun notView() = object : Any() { fun getContentDescription(): CharSequence = "q" }
}
`))
	if want := "10:18,14:18,17:18,21:18,25:18,32:22,35:22"; got != want {
		t.Errorf("GetContentDescriptionOverride positions = %s, want %s", got, want)
	}
}

func TestGetContentDescriptionOverrideJavaLintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "GetContentDescriptionOverride", `package q;

import android.content.Context;
import android.view.View;
import android.widget.FrameLayout;

public class J extends FrameLayout {
    public J(Context c) { super(c); }
    @Override public CharSequence getContentDescription() { return "j"; }
    public CharSequence getContentDescription(int x) { return "jx"; }
    public static class W extends J {
        public W(Context c) { super(c); }
        @Override public CharSequence getContentDescription() { return "w"; }
    }
    static class NotView { public CharSequence getContentDescription() { return ""; } }
    void g(Context c) {
        View v = new View(c) {
            @Override public CharSequence getContentDescription() { return "anon"; }
        };
    }
}
`))
	if want := "9:35,13:39,18:43"; got != want {
		t.Errorf("GetContentDescriptionOverride positions = %s, want %s", got, want)
	}
}

func TestAccessibilityWindowStateChangedEventKotlinLintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "AccessibilityWindowStateChangedEvent", `package q

import android.content.Context
import android.view.View
import android.view.accessibility.AccessibilityEvent
import android.view.accessibility.AccessibilityEvent.TYPE_WINDOW_STATE_CHANGED

const val EV = 32

class Acc(c: Context) : View(c) {
    val member = 32
    companion object { const val CE = 32 }
    override fun dispatchPopulateAccessibilityEvent(e: AccessibilityEvent): Boolean {
        e.eventType = AccessibilityEvent.TYPE_WINDOW_STATE_CHANGED
        val t = TYPE_WINDOW_STATE_CHANGED
        return true
    }
    override fun onPopulateAccessibilityEvent(e: AccessibilityEvent) {
        e.eventType = AccessibilityEvent.TYPE_WINDOW_STATE_CHANGED
    }
    override fun onInitializeAccessibilityEvent(e: AccessibilityEvent) {
        e.eventType = AccessibilityEvent.TYPE_WINDOW_STATE_CHANGED
    }
    fun send(v: View) {
        sendAccessibilityEvent(32)
        sendAccessibilityEvent(AccessibilityEvent.TYPE_WINDOW_STATE_CHANGED)
        sendAccessibilityEvent(TYPE_WINDOW_STATE_CHANGED)
        this.sendAccessibilityEvent(32)
        super.sendAccessibilityEvent(32)
        sendAccessibilityEvent(AccessibilityEvent.TYPE_VIEW_CLICKED)
        sendAccessibilityEvent(8 * 4)
        val k = 32
        sendAccessibilityEvent(k)
        var m = 32
        sendAccessibilityEvent(m)
        sendAccessibilityEvent(EV)
        sendAccessibilityEvent(CE)
        sendAccessibilityEvent(Companion.CE)
        sendAccessibilityEvent(member)
        sendAccessibilityEvent(32.toInt())
        val n = 1
        sendAccessibilityEvent(n)
        sendAccessibilityEventUnchecked(AccessibilityEvent.obtain(32))
        v.sendAccessibilityEvent(32)
        (v as View).sendAccessibilityEvent(AccessibilityEvent.TYPE_WINDOW_STATE_CHANGED)
        val w = View(context)
        w.sendAccessibilityEvent(32)
    }
    inner class Helper { fun f() { sendAccessibilityEvent(32) } }
}

class NotView {
    fun sendAccessibilityEvent(i: Int) {}
    fun f() { sendAccessibilityEvent(32) }
}

class Other(c: Context) : View(c) {
    override fun onPopulateAccessibilityEvent(e: AccessibilityEvent) {
        e.eventType = AccessibilityEvent.TYPE_WINDOW_STATE_CHANGED
    }
}

class NotViewPopulate {
    fun onPopulateAccessibilityEvent(e: AccessibilityEvent) {
        e.eventType = AccessibilityEvent.TYPE_WINDOW_STATE_CHANGED
    }
}
`))
	if want := "14:42,15:17,19:42,25:9,26:9,27:9,28:9,29:9,31:9,33:9,36:9,37:9,38:9,39:9,40:9,44:9,45:9,47:9,49:36,59:42"; got != want {
		t.Errorf("AccessibilityWindowStateChangedEvent positions = %s, want %s", got, want)
	}
}

func TestAccessibilityWindowStateChangedEventJavaLintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "AccessibilityWindowStateChangedEvent", `package q;

import android.content.Context;
import android.view.View;
import android.view.accessibility.AccessibilityEvent;

public class Jw extends View {
    public Jw(Context c) { super(c); }
    static final int SF = 32;
    @Override public boolean dispatchPopulateAccessibilityEvent(AccessibilityEvent e) {
        e.setEventType(AccessibilityEvent.TYPE_WINDOW_STATE_CHANGED);
        return true;
    }
    @Override public void onInitializeAccessibilityEvent(AccessibilityEvent e) {
        e.setEventType(AccessibilityEvent.TYPE_WINDOW_STATE_CHANGED);
    }
    void f(View o) {
        sendAccessibilityEvent(AccessibilityEvent.TYPE_WINDOW_STATE_CHANGED);
        sendAccessibilityEvent(32);
        sendAccessibilityEvent(SF);
        sendAccessibilityEvent(Jw.SF);
        o.sendAccessibilityEvent(32);
        super.sendAccessibilityEvent(32);
        this.sendAccessibilityEvent(32);
        Jw.this.sendAccessibilityEvent(32);
        final int fl = 32;
        sendAccessibilityEvent(fl);
        int nf = 32;
        sendAccessibilityEvent(nf);
        sendAccessibilityEvent(AccessibilityEvent.TYPE_VIEW_CLICKED);
    }
    static class NotView {
        void sendAccessibilityEvent(int i) {}
        void f() { sendAccessibilityEvent(32); }
    }
}
`))
	if want := "11:43,18:9,19:9,20:9,21:9,22:9,23:9,24:9,25:9,27:9"; got != want {
		t.Errorf("AccessibilityWindowStateChangedEvent positions = %s, want %s", got, want)
	}
}

func TestAccessibilityFocusKotlinLintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "AccessibilityFocus", `package q

import android.content.Context
import android.os.Bundle
import android.view.View
import android.view.accessibility.AccessibilityNodeInfo

class Foc(c: Context) : View(c) {
    fun f(v: View, info: AccessibilityNodeInfo) {
        performAccessibilityAction(64, null)
        performAccessibilityAction(AccessibilityNodeInfo.ACTION_ACCESSIBILITY_FOCUS, null)
        performAccessibilityAction(AccessibilityNodeInfo.ACTION_CLICK, null)
        performAccessibilityAction(AccessibilityNodeInfo.ACTION_ACCESSIBILITY_FOCUS or 1, Bundle())
        this.performAccessibilityAction(64, Bundle())
        super.performAccessibilityAction(64, Bundle())
        performAccessibilityAction(0x40, null)
        performAccessibilityAction(32 + 32, null)
        v.performAccessibilityAction(64, null)
        (v as View).performAccessibilityAction(64, null)
        info.performAction(AccessibilityNodeInfo.ACTION_ACCESSIBILITY_FOCUS)
        v.performAccessibilityAction(AccessibilityNodeInfo.ACTION_CLICK, null)
    }
}

class NotView {
    fun performAccessibilityAction(a: Int, b: Bundle?) {}
    fun f() { performAccessibilityAction(64, null) }
}
`))
	if want := "10:9,11:9,13:9,14:9,15:9,16:9,17:9,18:9,19:9"; got != want {
		t.Errorf("AccessibilityFocus positions = %s, want %s", got, want)
	}
}

func TestAccessibilityFocusJavaLintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "AccessibilityFocus", `package q;

import android.content.Context;
import android.os.Bundle;
import android.view.View;
import android.view.accessibility.AccessibilityNodeInfo;

public class Jf extends View {
    public Jf(Context c) { super(c); }
    void f(View o, AccessibilityNodeInfo info) {
        performAccessibilityAction(AccessibilityNodeInfo.ACTION_ACCESSIBILITY_FOCUS, null);
        o.performAccessibilityAction(64, new Bundle());
        super.performAccessibilityAction(64, null);
        performAccessibilityAction(AccessibilityNodeInfo.ACTION_CLICK, null);
        info.performAction(AccessibilityNodeInfo.ACTION_ACCESSIBILITY_FOCUS);
    }
}
`))
	if want := "11:9,12:9,13:9"; got != want {
		t.Errorf("AccessibilityFocus positions = %s, want %s", got, want)
	}
}

func TestGetContentDescriptionOverrideCrossFileBases(t *testing.T) {
	findings := runRuleByNameMultiFile(t, "GetContentDescriptionOverride", map[string]string{
		"Base.kt": `package q

import android.content.Context
import android.view.View
import android.widget.FrameLayout

open class BaseView(c: Context) : View(c)
open class BaseGroup<T>(c: Context) : FrameLayout(c)
open class Plain
`,
		"Derived.kt": `package q

import android.content.Context

class D1(c: Context) : BaseView(c) {
    override fun getContentDescription(): CharSequence = "d1"
}
class D2(c: Context) : BaseGroup<String>(c) {
    override fun getContentDescription(): CharSequence = "d2"
}
class D3 : Plain() {
    fun getContentDescription(): CharSequence = "d3"
}
`,
	})
	if got, want := findingPositions(findings), "6:18,9:18"; got != want {
		t.Errorf("positions = %s, want %s", got, want)
	}
}

func TestAccessibilityWindowStateChangedEventIgnoresOtherEventTypes(t *testing.T) {
	findings := runRuleByName(t, "AccessibilityWindowStateChangedEvent", `package q

import android.content.Context
import android.view.View
import android.view.accessibility.AccessibilityEvent

class V(c: Context) : View(c) {
    override fun onPopulateAccessibilityEvent(e: AccessibilityEvent) {
        e.eventType = Other.TYPE_WINDOW_STATE_CHANGED
        e.eventType = AccessibilityEvent.TYPE_VIEW_CLICKED
    }
    fun f() {
        sendAccessibilityEvent(Other.TYPE_WINDOW_STATE_CHANGED)
        sendAccessibilityEvent(AccessibilityEvent.TYPE_ANNOUNCEMENT)
    }
}

object Other { const val TYPE_WINDOW_STATE_CHANGED = 1 }
`)
	if len(findings) != 0 {
		t.Errorf("got %s, want no findings", findingPositions(findings))
	}
}
