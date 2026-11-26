package rules_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// Expected positions come from running Android Lint 32.4.1 on the same sources (minSdk 21, targetSdk 34).

const javascriptInterfaceKotlinSource = `package test.pkg

import android.webkit.JavascriptInterface
import android.webkit.WebView

class Annotated {
    @JavascriptInterface
    fun call() {}
}

class NotAnnotated {
    fun call() {}
}

open class BaseAnnotated {
    @JavascriptInterface
    fun base() {}
}

class SubOfAnnotated : BaseAnnotated() {
    fun extra() {}
}

@JavascriptInterface
class ClassAnnotated {
    fun call() {}
}

class Visibility {
    @JavascriptInterface
    private fun priv() {}
    @JavascriptInterface
    internal fun intern() {}
    @JavascriptInterface
    protected fun prot() {}
    @JavascriptInterface
    fun pub() {}
    @other.JavascriptInterface
    private fun lookalike() {}
}

class MyWebView(ctx: android.content.Context) : WebView(ctx) {
    override fun addJavascriptInterface(obj: Any, name: String) {}
}

class Host : android.app.Activity() {
    fun f(webView: WebView, mine: MyWebView) {
        webView.addJavascriptInterface(Annotated(), "a")
        webView.addJavascriptInterface(NotAnnotated(), "b")
        webView.addJavascriptInterface(SubOfAnnotated(), "c")
        webView.addJavascriptInterface(ClassAnnotated(), "d")
        webView.addJavascriptInterface(Object(), "e")
        webView.addJavascriptInterface(Any(), "f")
        webView.addJavascriptInterface(this, "g")
        val local = NotAnnotated()
        webView.addJavascriptInterface(local, "h")
        val typed: Any = Annotated()
        webView.addJavascriptInterface(typed, "i")
        mine.addJavascriptInterface(NotAnnotated(), "j")
        webView.addJavascriptInterface(object { fun x() {} }, "k")
        webView.addJavascriptInterface("str", "l")
        webView.addJavascriptInterface(obj = NotAnnotated(), name = "m")
        webView.addJavascriptInterface(name = "n", obj = NotAnnotated())
        webView.addJavascriptInterface(Visibility(), "o")
        webView.addJavascriptInterface(JNot(), "p")
        webView.addJavascriptInterface(Runnable {}, "q")
    }
}
`

const javascriptInterfaceJavaSource = `package test.pkg;

import android.webkit.JavascriptInterface;
import android.webkit.WebView;

public class JNot {
    public void call() {}

    @JavascriptInterface
    void packagePrivate() {}

    @JavascriptInterface
    private int field;

    interface Bridge {
        @JavascriptInterface
        void implicitPublic();
    }

    static class Ann {
        @JavascriptInterface
        public void ok() {}
    }

    void f(WebView webView, Bridge bridge) {
        webView.addJavascriptInterface(new Ann(), "a");
        webView.addJavascriptInterface(new JNot(), "b");
        webView.addJavascriptInterface(bridge, "c");
        webView.addJavascriptInterface(this, "d");
        webView.addJavascriptInterface(new Object(), "e");
    }
}
`

func TestJavascriptInterfaceLintParityKotlin(t *testing.T) {
	findings := runAndroidSecurityProjectRule(t, "JavascriptInterface", "K.kt", 21, 34, javascriptInterfaceKotlinSource)
	want := "31:17,35:19,49:17,52:17,53:17,54:17,56:17,60:17,61:17,62:17,63:17,66:17"
	if got := findingPositions(findings); got != want {
		t.Fatalf("JavascriptInterface positions = %s, want %s", got, want)
	}
	names := map[int]string{49: "NotAnnotated", 52: "Object", 53: "Object", 54: "Host", 60: "null", 61: "String", 63: "String", 66: "Runnable"}
	for _, f := range findings {
		if name, ok := names[f.Line]; ok {
			want := "None of the methods in the added interface (" + name + ") have been annotated with " +
				"`@android.webkit.JavascriptInterface`; they will not be visible in API 17"
			if f.Message != want {
				t.Errorf("line %d message = %q, want %q", f.Line, f.Message, want)
			}
		}
		if f.Line == 31 && f.Message != "Must be public when using `@JavascriptInterface`" {
			t.Errorf("line 31 message = %q", f.Message)
		}
	}
}

func TestJavascriptInterfaceLintParityJava(t *testing.T) {
	findings := runAndroidSecurityProjectRule(t, "JavascriptInterface", "JNot.java", 21, 34, javascriptInterfaceJavaSource)
	if got, want := findingPositions(findings), "10:10,13:17,30:17"; got != want {
		t.Fatalf("JavascriptInterface positions = %s, want %s", got, want)
	}
}

func TestJavascriptInterfaceNeedsTargetSdk17(t *testing.T) {
	findings := runAndroidSecurityProjectRule(t, "JavascriptInterface", "K.kt", 16, 16, javascriptInterfaceKotlinSource)
	for _, f := range findings {
		if strings.HasPrefix(f.Message, "None of the methods") {
			t.Fatalf("reported a bridge below targetSdk 17: %v", f)
		}
	}
}

// AddJavascriptInterface no longer reports the missing-annotation finding; JavascriptInterface does.
func TestAddJavascriptInterfaceLeavesBridgeAnnotationsToJavascriptInterface(t *testing.T) {
	for _, f := range runAndroidSecurityProjectRule(t, "AddJavascriptInterface", "K.kt", 21, 34, javascriptInterfaceKotlinSource) {
		t.Errorf("AddJavascriptInterface reported %d:%d %q with minSdk 21", f.Line, f.Col, f.Message)
	}
}

const javascriptInterfaceLegacySuppressed = `package com.example

import android.webkit.WebView

class LegacyBridge {
    fun unannotated() = Unit
}

class LegacySuppressed {
    // AddJavascriptInterface used to report this finding, so its suppression still applies.
    @Suppress("AddJavascriptInterface")
    fun bind(webView: WebView) {
        webView.addJavascriptInterface(LegacyBridge(), "bridge")
    }
}
`

// A suppression written for AddJavascriptInterface, which used to report this finding, keeps silencing it.
func TestJavascriptInterfaceHonorsAddJavascriptInterfaceSuppression(t *testing.T) {
	if got := runAndroidSecurityProjectRule(t, "JavascriptInterface", "LegacySuppressed.kt", 21, 34, javascriptInterfaceLegacySuppressed); len(got) != 0 {
		t.Fatalf("@Suppress(\"AddJavascriptInterface\") did not silence JavascriptInterface: %v", got)
	}
	unsuppressed := strings.Replace(javascriptInterfaceLegacySuppressed, "@Suppress(\"AddJavascriptInterface\")", "", 1)
	if got := runAndroidSecurityProjectRule(t, "JavascriptInterface", "LegacySuppressed.kt", 21, 34, unsuppressed); len(got) != 1 {
		t.Fatalf("expected 1 finding without the suppression, got %v", got)
	}
}

// A baseline entry recorded when AddJavascriptInterface reported the finding keeps silencing it.
func TestJavascriptInterfaceHonorsAddJavascriptInterfaceBaseline(t *testing.T) {
	findings := runAndroidSecurityProjectRule(t, "JavascriptInterface", "K.kt", 21, 34, javascriptInterfaceKotlinSource)
	var bridge scanner.Finding
	for _, f := range findings {
		if f.Line == 49 {
			bridge = f
		}
	}
	if bridge.Rule != "JavascriptInterface" {
		t.Fatalf("no JavascriptInterface finding at line 49: %v", findings)
	}
	legacy := bridge
	legacy.Rule = "AddJavascriptInterface"
	path := filepath.Join(t.TempDir(), "baseline.xml")
	if err := scanner.WriteBaseline(path, []scanner.Finding{legacy}, ""); err != nil {
		t.Fatal(err)
	}
	baseline, err := scanner.LoadBaseline(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := scanner.FilterByBaseline([]scanner.Finding{bridge}, baseline, ""); len(got) != 0 {
		raw, _ := os.ReadFile(path)
		t.Fatalf("legacy AddJavascriptInterface baseline entry did not silence the finding:\n%s", raw)
	}
}

const unsanitizedFilenameKotlinSource = `package test.pkg

import android.database.Cursor
import android.provider.OpenableColumns
import java.io.File

class F {
    fun direct(cursor: Cursor, dir: File) {
        val idx = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME)
        val name = cursor.getString(idx)
        val f = File(dir, name)
    }
    fun literal(cursor: Cursor, dir: File) {
        val idx = cursor.getColumnIndexOrThrow("_display_name")
        val name = cursor.getString(idx)
        File(dir, name)
    }
    fun inline(cursor: Cursor, dir: File) {
        val name = cursor.getString(cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME))
        File(dir, name)
    }
    fun sanitizedFirst(cursor: Cursor, dir: File) {
        val idx = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME)
        val name = cursor.getString(idx)
        val safe = sanitize(name)
        File(dir, name)
    }
    fun receiverUse(cursor: Cursor, dir: File) {
        val idx = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME)
        val name = cursor.getString(idx)
        if (name.contains("..")) return
        File(dir, name)
    }
    fun otherColumn(cursor: Cursor, dir: File) {
        val idx = cursor.getColumnIndex("title")
        File(dir, cursor.getString(idx))
    }
    fun direct2(cursor: Cursor, dir: File) {
        val idx = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME)
        File(dir, cursor.getString(idx))
        File(cursor.getString(idx))
    }
    fun twice(cursor: Cursor, dir: File) {
        val idx = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME)
        val name = cursor.getString(idx)
        File(dir, name)
        File(dir, name)
    }
    fun template(cursor: Cursor, dir: File) {
        val idx = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME)
        val name = cursor.getString(idx)
        File(dir, "x/$name")
    }
    fun reassigned(cursor: Cursor, dir: File) {
        val idx = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME)
        var name = cursor.getString(idx)
        name = "fixed"
        File(dir, name)
    }
    fun sanitize(s: String) = s
}
`

const unsanitizedFilenameJavaSource = `package test.pkg;

import android.database.Cursor;
import android.provider.OpenableColumns;
import java.io.File;

class J {
    void direct(Cursor cursor, File dir) {
        int idx = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME);
        String name = cursor.getString(idx);
        File f = new File(dir, name);
    }
    void sanitized(Cursor cursor, File dir) {
        int idx = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME);
        String name = cursor.getString(idx);
        name = new File(name).getName();
        new File(dir, name);
    }
    static class MyFile extends File { MyFile(File d, String n) { super(d, n); } }
    void subclass(Cursor cursor, File dir) {
        int idx = cursor.getColumnIndex("_display_name");
        String name = cursor.getString(idx);
        new MyFile(dir, name);
    }
}
`

const unsanitizedFilenameLambdaSource = `package test.pkg

import android.content.Context
import android.net.Uri
import android.provider.OpenableColumns
import java.io.File

class FakeCursor {
    fun getColumnIndex(name: String) = 0
    fun getString(i: Int) = ""
}

class G(private val context: Context) {
    fun useLambda(uri: Uri, dir: File) {
        context.contentResolver.query(uri, null, null, null, null)?.use { cursor ->
            val idx = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME)
            cursor.moveToFirst()
            val name = cursor.getString(idx)
            File(dir, name)
        }
    }
    fun implicitIt(uri: Uri, dir: File) {
        context.contentResolver.query(uri, null, null, null, null)?.use {
            val name = it.getString(it.getColumnIndexOrThrow(OpenableColumns.DISPLAY_NAME))
            File(dir, name)
        }
    }
    fun lookalike(c: FakeCursor, dir: File) {
        val name = c.getString(c.getColumnIndex("_display_name"))
        File(dir, name)
    }
    fun nestedFunction(c: android.database.Cursor, dir: File) {
        val name = c.getString(c.getColumnIndex("_display_name"))
        fun inner() { File(dir, name) }
        inner()
    }
    fun inLambda(c: android.database.Cursor, dir: File) {
        val name = c.getString(c.getColumnIndex("_display_name"))
        listOf(1).forEach { File(dir, name) }
    }
    fun copy(c: android.database.Cursor, dir: File) {
        val name = c.getString(c.getColumnIndex("_display_name"))
        val other = name
        File(dir, other)
    }
    fun string(c: android.database.Cursor, dir: File) {
        val name = c.getString(c.getColumnIndex("_display_name"))
        File("$dir/$name")
        File(dir.path + "/" + name)
    }
}
`

func TestUnsanitizedFilenameFromContentProviderLintParityKotlin(t *testing.T) {
	findings := runRuleByName(t, "UnsanitizedFilenameFromContentProvider", unsanitizedFilenameKotlinSource)
	if got, want := findingPositions(findings), "11:27,16:19,20:19,40:19,41:14,46:19,47:19"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
	want := "Using `name` is unsafe as it is a filename obtained directly from a `ContentProvider`. You should " +
		"sanitize it before using it for creating a `File`."
	if findings[0].Message != want {
		t.Errorf("message = %q, want %q", findings[0].Message, want)
	}
	for _, f := range findings {
		if f.Line == 40 && !strings.HasPrefix(f.Message, "Using `cursor.getString(idx)` is unsafe") {
			t.Errorf("line 40 message = %q", f.Message)
		}
	}
}

func TestUnsanitizedFilenameFromContentProviderLintParityJava(t *testing.T) {
	findings := runRuleByNameOnJava(t, "UnsanitizedFilenameFromContentProvider", unsanitizedFilenameJavaSource)
	if got, want := findingPositions(findings), "11:32,16:25,23:25"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

// Lint with kotlin-stdlib on the classpath resolves ContentResolver.query(...).use { } lambdas, follows the value
// into local functions and lambdas, and through copies; it ignores lookalike cursors and string building.
func TestUnsanitizedFilenameFromContentProviderLintParityFlows(t *testing.T) {
	findings := runRuleByName(t, "UnsanitizedFilenameFromContentProvider", unsanitizedFilenameLambdaSource)
	if got, want := findingPositions(findings), "19:23,25:23,34:33,39:39,44:19"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

// #1621: a Cursor local whose type is inferred from a call is proven: ContentResolver.query, SQLiteDatabase.query or
// rawQuery, or a same-file function declared to return Cursor. Lines marked `// report` are expected findings.
const unsanitizedFilenameInferredSource = `package test.pkg

import android.content.ContentResolver
import android.database.Cursor
import android.database.sqlite.SQLiteDatabase
import android.net.Uri
import android.provider.OpenableColumns
import java.io.File

class NotACursor { fun getString(i: Int): String = ""; fun getColumnIndex(c: String): Int = 0 }

class F(private val resolver: ContentResolver, private val db: SQLiteDatabase) {
    fun openCursor(uri: Uri): Cursor = resolver.query(uri, null, null, null, null)!!
    fun openOther(): NotACursor = NotACursor()

    fun viaQuery(uri: Uri, dir: File) {
        val c = resolver.query(uri, null, null, null, null)!!
        val name = c.getString(c.getColumnIndex(OpenableColumns.DISPLAY_NAME))
        File(dir, name) // report
    }
    fun viaRawQuery(dir: File) {
        val c = db.rawQuery("select 1", null)
        val name = c.getString(c.getColumnIndex(OpenableColumns.DISPLAY_NAME))
        File(dir, name) // report
    }
    fun viaHelper(uri: Uri, dir: File) {
        val c = openCursor(uri)
        val name = c.getString(c.getColumnIndex(OpenableColumns.DISPLAY_NAME))
        File(dir, name) // report
    }
    fun lookalike(dir: File) {
        val c = openOther()
        val name = c.getString(c.getColumnIndex(OpenableColumns.DISPLAY_NAME))
        File(dir, name)
    }
    fun unknownCall(dir: File) {
        val c = somewhereElse()
        val name = c.getString(c.getColumnIndex(OpenableColumns.DISPLAY_NAME))
        File(dir, name)
    }
}
`

func TestUnsanitizedFilenameFromContentProviderInfersCursorLocals(t *testing.T) {
	findings := runRuleByName(t, "UnsanitizedFilenameFromContentProvider", unsanitizedFilenameInferredSource)
	var want []string
	for i, line := range strings.Split(unsanitizedFilenameInferredSource, "\n") {
		if strings.HasSuffix(line, "// report") {
			want = append(want, fmt.Sprint(i+1))
		}
	}
	var got []string
	for _, f := range findings {
		got = append(got, fmt.Sprint(f.Line))
	}
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("finding lines %v, want %v", got, want)
	}
}

const unsanitizedFilenameInferredJavaSource = `package test.pkg;

import android.app.Activity;
import android.database.sqlite.SQLiteDatabase;
import android.net.Uri;
import android.provider.OpenableColumns;
import java.io.File;

class J extends Activity {
    void viaQuery(Uri uri, File dir) {
        var c = getContentResolver().query(uri, null, null, null, null);
        String name = c.getString(c.getColumnIndex(OpenableColumns.DISPLAY_NAME));
        new File(dir, name); // report
    }
    void viaRaw(SQLiteDatabase db, File dir) {
        var c = db.rawQuery("select 1", null);
        String name = c.getString(c.getColumnIndex(OpenableColumns.DISPLAY_NAME));
        new File(dir, name); // report
    }
}
`

func TestUnsanitizedFilenameFromContentProviderInfersCursorLocalsJava(t *testing.T) {
	findings := runRuleByNameOnJava(t, "UnsanitizedFilenameFromContentProvider", unsanitizedFilenameInferredJavaSource)
	var want, got []string
	for i, line := range strings.Split(unsanitizedFilenameInferredJavaSource, "\n") {
		if strings.HasSuffix(line, "// report") {
			want = append(want, fmt.Sprint(i+1))
		}
	}
	for _, f := range findings {
		got = append(got, fmt.Sprint(f.Line))
	}
	sort.Strings(got)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("finding lines %v, want %v", got, want)
	}
}
