package rules_test

import (
	"fmt"
	"strings"
	"testing"
)

// Expected positions come from running Android Lint 32.4.1 on the same sources (minSdk 21, targetSdk 34).

const mutableImplicitKotlinSource = `package test.pkg

import android.app.PendingIntent
import android.app.PendingIntent.FLAG_MUTABLE
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.net.Uri

class Target

class P(private val context: Context) {
    fun cases(other: Intent, uri: Uri) {
        PendingIntent.getActivity(context, 0, Intent("action"), PendingIntent.FLAG_MUTABLE)
        PendingIntent.getActivity(context, 0, Intent(), FLAG_MUTABLE)
        PendingIntent.getActivity(context, 0, Intent(context, Target::class.java), FLAG_MUTABLE)
        PendingIntent.getActivity(context, 0, Intent("action").setPackage("p"), FLAG_MUTABLE)
        PendingIntent.getActivity(context, 0, Intent("action").setPackage(null), FLAG_MUTABLE)
        PendingIntent.getActivity(context, 0, Intent("action").setClass(context, Target::class.java), FLAG_MUTABLE)
        PendingIntent.getActivity(context, 0, Intent("action").putExtra("k", 1), FLAG_MUTABLE)
        PendingIntent.getActivity(context, 0, Intent("action"), FLAG_MUTABLE or PendingIntent.FLAG_NO_CREATE)
        PendingIntent.getActivity(context, 0, Intent("action"), FLAG_MUTABLE or PendingIntent.FLAG_ALLOW_UNSAFE_IMPLICIT_INTENT)
        PendingIntent.getActivity(context, 0, Intent("action"), PendingIntent.FLAG_IMMUTABLE)
        PendingIntent.getActivity(context, 0, Intent("action"), FLAG_MUTABLE or PendingIntent.FLAG_UPDATE_CURRENT)
        PendingIntent.getBroadcast(context, 0, Intent("action"), FLAG_MUTABLE)
        PendingIntent.getService(context, 0, Intent("action"), FLAG_MUTABLE)
        PendingIntent.getActivity(context, 0, other, FLAG_MUTABLE)
        PendingIntent.getActivity(context, 0, Intent(other), FLAG_MUTABLE)
        PendingIntent.getActivity(context, 0, Intent("action", uri), FLAG_MUTABLE)
        PendingIntent.getActivity(context, 0, Intent("a", uri, context, Target::class.java), FLAG_MUTABLE)
        PendingIntent.getActivities(context, 0, arrayOf(Intent(context, Target::class.java), Intent("a")), FLAG_MUTABLE)
        PendingIntent.getActivities(context, 0, arrayOf(Intent(context, Target::class.java)), FLAG_MUTABLE)
        PendingIntent.getActivity(context, 0, Intent("action").apply { setPackage("p") }, FLAG_MUTABLE)
        PendingIntent.getActivity(context, 0, Intent("action").apply { putExtra("k", 1) }, FLAG_MUTABLE)
        PendingIntent.getActivity(context, 0, Intent("action").also { it.setPackage("p") }, FLAG_MUTABLE)
        PendingIntent.getActivity(context, 0, android.content.Intent("action"), FLAG_MUTABLE)
        PendingIntent.getActivity(context, 0, Intent("action").setComponent(ComponentName("p", "c")), FLAG_MUTABLE)
        val flags = FLAG_MUTABLE
        PendingIntent.getActivity(context, 0, Intent("action"), flags)
    }
    fun locals() {
        val a = Intent("action")
        PendingIntent.getActivity(context, 0, a, FLAG_MUTABLE)
        val b = Intent("action")
        b.setPackage("p")
        PendingIntent.getActivity(context, 0, b, FLAG_MUTABLE)
        val c = Intent("action")
        c.putExtra("k", 1)
        PendingIntent.getActivity(context, 0, c, FLAG_MUTABLE)
        val d = Intent("action")
        helper(d)
        PendingIntent.getActivity(context, 0, d, FLAG_MUTABLE)
        var e = Intent(context, Target::class.java)
        e = Intent("x")
        PendingIntent.getActivity(context, 0, e, FLAG_MUTABLE)
        val f = Intent("action")
        f.action = "y"
        PendingIntent.getActivity(context, 0, f, FLAG_MUTABLE)
        val g = Intent("action")
        g.` + "`package`" + ` = "p"
        PendingIntent.getActivity(context, 0, g, FLAG_MUTABLE)
        val h = Intent("action")
        PendingIntent.getActivity(context, 0, h, FLAG_MUTABLE)
        h.setPackage("late")
    }
    fun helper(i: Intent) {}
}
`

const mutableImplicitJavaSource = `package test.pkg;

import android.app.PendingIntent;
import android.content.Context;
import android.content.Intent;

class J {
    void f(Context context) {
        PendingIntent.getActivity(context, 0, new Intent("action"), PendingIntent.FLAG_MUTABLE);
        PendingIntent.getActivity(context, 0, new Intent(context, J.class), PendingIntent.FLAG_MUTABLE);
        Intent i = new Intent("a");
        i.setPackage("p");
        PendingIntent.getBroadcast(context, 0, i, PendingIntent.FLAG_MUTABLE | PendingIntent.FLAG_UPDATE_CURRENT);
        Intent j = new Intent("a");
        PendingIntent.getBroadcast(context, 0, j, PendingIntent.FLAG_MUTABLE | PendingIntent.FLAG_UPDATE_CURRENT);
        PendingIntent.getActivities(context, 0, new Intent[]{new Intent("a")}, PendingIntent.FLAG_MUTABLE);
        PendingIntent.getActivity(context, 0, new Intent("a").setPackage("p"), PendingIntent.FLAG_MUTABLE);
    }
}
`

func TestMutableImplicitPendingIntentLintParityKotlin(t *testing.T) {
	findings := runAndroidSecurityProjectRule(t, "MutableImplicitPendingIntent", "P.kt", 21, 34, mutableImplicitKotlinSource)
	want := "14:9,15:9,18:9,20:9,24:9,25:9,26:9,28:9,29:9,31:9,34:9,36:9,39:9,43:9,49:9,55:9,58:9,63:9"
	if got := findingPositions(findings); got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
	msg := "Mutable implicit `PendingIntent` will throw an exception, follow either of these recommendations: for an " +
		"existing `PendingIntent` use `FLAG_NO_CREATE` and for a new `PendingIntent` either make it immutable or make " +
		"the `Intent` within explicit"
	for _, f := range findings {
		if f.Message != msg {
			t.Fatalf("message = %q, want %q", f.Message, msg)
		}
		if f.Severity != "error" {
			t.Fatalf("severity = %q at targetSdk 34, want error", f.Severity)
		}
	}
}

func TestMutableImplicitPendingIntentLintParityJava(t *testing.T) {
	findings := runAndroidSecurityProjectRule(t, "MutableImplicitPendingIntent", "J.java", 21, 34, mutableImplicitJavaSource)
	if got, want := findingPositions(findings), "9:9,15:9,16:9"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestMutableImplicitPendingIntentTargetSdkTiers(t *testing.T) {
	warn := runAndroidSecurityProjectRule(t, "MutableImplicitPendingIntent", "J.java", 21, 30, mutableImplicitJavaSource)
	if len(warn) != 3 {
		t.Fatalf("targetSdk 30: expected 3 findings, got %d", len(warn))
	}
	for _, f := range warn {
		if f.Severity != "warning" || !strings.Contains(f.Message, "will throw an exception once this app starts targeting Android 14 or above, follow") {
			t.Fatalf("targetSdk 30 finding = %s %q", f.Severity, f.Message)
		}
	}
	if got := runAndroidSecurityProjectRule(t, "MutableImplicitPendingIntent", "J.java", 21, 22, mutableImplicitJavaSource); len(got) != 0 {
		t.Fatalf("targetSdk 22: expected no findings, got %v", got)
	}
}

func TestMutableImplicitPendingIntentLookalikes(t *testing.T) {
	kotlin := `package test.pkg

import android.content.Context

object PendingIntent {
    const val FLAG_MUTABLE = 33554432
    fun getActivity(context: Context, code: Int, intent: Intent, flags: Int) = Unit
}

class Intent(val action: String)

fun f(context: Context) {
    PendingIntent.getActivity(context, 0, Intent("action"), PendingIntent.FLAG_MUTABLE)
}
`
	if got := runAndroidSecurityProjectRule(t, "MutableImplicitPendingIntent", "L.kt", 21, 34, kotlin); len(got) != 0 {
		t.Fatalf("reported a local PendingIntent lookalike: %v", got)
	}
	java := `package test.pkg;

import android.app.PendingIntent;
import android.content.Context;

class L {
    static class Intent { Intent(String action) {} }
    void f(Context context, android.content.Intent unknown) {
        PendingIntent.getActivity(context, 0, unknown, PendingIntent.FLAG_MUTABLE);
        android.content.Intent fromHelper = make();
        PendingIntent.getActivity(context, 0, fromHelper, PendingIntent.FLAG_MUTABLE);
    }
    android.content.Intent make() { return null; }
}
`
	if got := runAndroidSecurityProjectRule(t, "MutableImplicitPendingIntent", "L.java", 21, 34, java); len(got) != 0 {
		t.Fatalf("reported an intent of unknown origin: %v", got)
	}
}

// #1622: getActivities intent arrays held in locals (arrayOf, arrayOfNulls with indexed writes) and let/run/with
// blocks made only of Intent method calls are followed, as Lint 32.4.1 does. Lines marked `// report` are expected
// findings; the rest abstain or are explicit.
const mutableImplicitFollowSource = `package test.pkg

import android.app.PendingIntent
import android.app.PendingIntent.FLAG_MUTABLE
import android.content.Context
import android.content.Intent

class Target

class P(private val context: Context) {
    fun arrays(n: Int) {
        PendingIntent.getActivities(context, 0, listOf(Intent("a")).toTypedArray(), FLAG_MUTABLE)
        PendingIntent.getActivities(context, 0, mutableListOf(Intent("a")).toTypedArray(), FLAG_MUTABLE)
        PendingIntent.getActivities(context, 0, arrayListOf(Intent("a")).toTypedArray(), FLAG_MUTABLE)
        val list = listOf(Intent("b"))
        PendingIntent.getActivities(context, 0, list.toTypedArray(), FLAG_MUTABLE)
        val arr = arrayOf(Intent("c"))
        PendingIntent.getActivities(context, 0, arr, FLAG_MUTABLE) // report
        val explicitArr = arrayOf(Intent(context, Target::class.java))
        PendingIntent.getActivities(context, 0, explicitArr, FLAG_MUTABLE)
        val nulls = arrayOfNulls<Intent>(2)
        nulls[0] = Intent(context, Target::class.java)
        nulls[1] = Intent("d")
        PendingIntent.getActivities(context, 0, nulls, FLAG_MUTABLE) // report
        val explicitNulls = arrayOfNulls<Intent>(1)
        explicitNulls[0] = Intent(context, Target::class.java)
        PendingIntent.getActivities(context, 0, explicitNulls, FLAG_MUTABLE)
        val overwritten = arrayOfNulls<Intent>(1)
        overwritten[0] = Intent("e")
        overwritten[0] = Intent(context, Target::class.java)
        PendingIntent.getActivities(context, 0, overwritten, FLAG_MUTABLE)
        val outOfRange = arrayOfNulls<Intent>(1)
        outOfRange[3] = Intent("f")
        PendingIntent.getActivities(context, 0, outOfRange, FLAG_MUTABLE)
        val sized = arrayOfNulls<Intent>(n)
        sized[0] = Intent("g")
        PendingIntent.getActivities(context, 0, sized, FLAG_MUTABLE)
        val escaped = arrayOfNulls<Intent>(1)
        escaped[0] = Intent("h")
        consume(escaped)
        PendingIntent.getActivities(context, 0, escaped, FLAG_MUTABLE)
        val deferred = arrayOfNulls<Intent>(1)
        val later = { deferred[0] = Intent("i") }
        PendingIntent.getActivities(context, 0, deferred, FLAG_MUTABLE)
    }
    fun scopes() {
        PendingIntent.getActivity(context, 0, Intent("a").let { it.putExtra("k", 1) }, FLAG_MUTABLE) // report
        PendingIntent.getActivity(context, 0, Intent("a").let { it.setPackage("p") }, FLAG_MUTABLE)
        PendingIntent.getActivity(context, 0, Intent("a").run { putExtra("k", 1) }, FLAG_MUTABLE) // report
        PendingIntent.getActivity(context, 0, Intent("a").run { setPackage("p") }, FLAG_MUTABLE)
        PendingIntent.getActivity(context, 0, with(Intent("a")) { setAction("b") }, FLAG_MUTABLE) // report
        PendingIntent.getActivity(context, 0, with(Intent("a")) { setClassName("p", "c") }, FLAG_MUTABLE)
        val viaLet = Intent("a").let { it.addFlags(1).putExtra("k", 2) }
        PendingIntent.getActivity(context, 0, viaLet, FLAG_MUTABLE)
        PendingIntent.getActivity(context, 0, Intent("a").let { it.addFlags(1); it.putExtra("k", 2) }, FLAG_MUTABLE) // report
        PendingIntent.getActivity(context, 0, Intent("a").let { it.putExtra("k", 1); it }, FLAG_MUTABLE)
        PendingIntent.getActivity(context, 0, Intent("a").let { i -> i.putExtra("k", 1) }, FLAG_MUTABLE)
        PendingIntent.getActivity(context, 0, with(Intent("a")) { action = "b"; this }, FLAG_MUTABLE)
        PendingIntent.getActivity(context, 0, Intent("a").let { other(it) }, FLAG_MUTABLE)
    }
    fun consume(any: Any?) {}
    fun other(i: Intent): Intent = i
}
`

const mutableImplicitFollowJavaSource = `package test.pkg;

import android.app.PendingIntent;
import android.content.Context;
import android.content.Intent;

class J {
    void f(Context context) {
        Intent[] arr = new Intent[2];
        arr[0] = new Intent("x");
        PendingIntent.getActivities(context, 0, arr, PendingIntent.FLAG_MUTABLE); // report
        Intent[] a = new Intent[]{new Intent("y")};
        PendingIntent.getActivities(context, 0, a, PendingIntent.FLAG_MUTABLE); // report
        Intent[] explicit = new Intent[1];
        explicit[0] = new Intent(context, J.class);
        PendingIntent.getActivities(context, 0, explicit, PendingIntent.FLAG_MUTABLE);
        Intent[] deferred = new Intent[1];
        Runnable later = () -> { deferred[0] = new Intent("z"); };
        PendingIntent.getActivities(context, 0, deferred, PendingIntent.FLAG_MUTABLE);
        Intent[] overwritten = new Intent[1];
        overwritten[0] = new Intent("w");
        overwritten[0] = new Intent(context, J.class);
        PendingIntent.getActivities(context, 0, overwritten, PendingIntent.FLAG_MUTABLE);
        Intent[] lastWins = new Intent[1];
        lastWins[0] = new Intent(context, J.class);
        lastWins[0] = new Intent("v");
        PendingIntent.getActivities(context, 0, lastWins, PendingIntent.FLAG_MUTABLE); // report
    }
}
`

func TestMutableImplicitPendingIntentFollowsListsArraysAndScopeBlocks(t *testing.T) {
	for _, test := range []struct{ name, src string }{
		{"P.kt", mutableImplicitFollowSource},
		{"J.java", mutableImplicitFollowJavaSource},
	} {
		findings := runAndroidSecurityProjectRule(t, "MutableImplicitPendingIntent", test.name, 21, 34, test.src)
		var want []string
		for i, line := range strings.Split(test.src, "\n") {
			if strings.HasSuffix(line, "// report") {
				want = append(want, fmt.Sprintf("%d:9", i+1))
			}
		}
		if got := findingPositions(findings); got != strings.Join(want, ",") {
			t.Fatalf("%s: positions = %s, want %s", test.name, got, strings.Join(want, ","))
		}
	}
}
