package rules_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func TestAddJavascriptInterface(t *testing.T) {
	t.Run("heuristic webView identifier without resolver fires at 0.85", func(t *testing.T) {
		findings := runRuleByName(t, "AddJavascriptInterface", `
package test
class MyWebView {
    fun setup() {
        webView.addJavascriptInterface(bridge, "Android")
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Confidence != 0.85 {
			t.Errorf("expected confidence 0.85, got %v", findings[0].Confidence)
		}
	})
	t.Run("clean code passes", func(t *testing.T) {
		findings := runRuleByName(t, "AddJavascriptInterface", `
package test
class MyWebView {
    fun setup() {
        webView.loadUrl("https://example.com")
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("resolver-backed WebView receiver fires at 1.0", func(t *testing.T) {
		findings := runRuleByNameWithResolver(t, "AddJavascriptInterface", `
package test
import android.webkit.WebView
class MyWebView {
    fun setup(wv: WebView) {
        wv.addJavascriptInterface(bridge, "Android")
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Confidence != 1.0 {
			t.Errorf("expected confidence 1.0, got %v", findings[0].Confidence)
		}
	})
	t.Run("non-WebView receiver does not fire", func(t *testing.T) {
		findings := runRuleByNameWithResolver(t, "AddJavascriptInterface", `
package test
class Wrapper {
    fun addJavascriptInterface(obj: Any, name: String) {}
}
fun caller() {
    val wrapper = Wrapper()
    wrapper.addJavascriptInterface(Any(), "bridge")
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings on non-WebView receiver, got %d", len(findings))
		}
	})
	t.Run("comment and string literal do not fire", func(t *testing.T) {
		findings := runRuleByName(t, "AddJavascriptInterface", `
package test
class Misc {
    // do not call addJavascriptInterface(
    fun describe(): String = "addJavascriptInterface(bridge)"
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings in comment/string, got %d", len(findings))
		}
	})
	t.Run("unrelated identifier without resolver does not fire", func(t *testing.T) {
		findings := runRuleByName(t, "AddJavascriptInterface", `
package test
class Random {
    fun setup() {
        someOther.addJavascriptInterface(bridge, "Android")
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("minSdk 17 suppresses old reflection hazard", func(t *testing.T) {
		findings := runAddJavascriptInterfaceProjectRule(t, 17, 16, `
package test
import android.webkit.WebView
class MyWebView {
    fun setup(wv: WebView) {
        wv.addJavascriptInterface(Any(), "Android")
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for minSdk 17 without targetSdk 17 annotation check, got %d", len(findings))
		}
	})
	t.Run("targetSdk 17 reports bridge without annotated methods", func(t *testing.T) {
		findings := runAddJavascriptInterfaceProjectRule(t, 17, 17, `
package test
import android.webkit.WebView
class Bridge {
    fun unannotated() = Unit
}
class MyWebView {
    fun setup(wv: WebView) {
        wv.addJavascriptInterface(Bridge(), "Android")
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding for missing JavascriptInterface annotation, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "@JavascriptInterface") {
			t.Fatalf("expected missing annotation message, got %q", findings[0].Message)
		}
	})
	t.Run("targetSdk 17 accepts annotated bridge method", func(t *testing.T) {
		findings := runAddJavascriptInterfaceProjectRule(t, 17, 17, `
package test
import android.webkit.JavascriptInterface
import android.webkit.WebView
class Bridge {
    @JavascriptInterface
    fun exposed() = Unit
}
class MyWebView {
    fun setup(wv: WebView) {
        wv.addJavascriptInterface(Bridge(), "Android")
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for annotated bridge, got %d", len(findings))
		}
	})
	t.Run("minSdk below 17 still reports reflection hazard", func(t *testing.T) {
		findings := runAddJavascriptInterfaceProjectRule(t, 16, 16, `
package test
import android.webkit.WebView
class MyWebView {
    fun setup(wv: WebView) {
        wv.addJavascriptInterface(Any(), "Android")
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding for minSdk 16 hazard, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "minSdk is below 17") {
			t.Fatalf("expected minSdk message, got %q", findings[0].Message)
		}
	})
}

func runAddJavascriptInterfaceProjectRule(t *testing.T, minSdk, targetSdk int, code string) []scanner.Finding {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "build.gradle.kts"), []byte(`
plugins {
    id("com.android.application")
}
android {
    minSdk = `+strconv.Itoa(minSdk)+`
    targetSdk = `+strconv.Itoa(targetSdk)+`
}
`), 0644); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "src", "main", "java", "test")
	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(src, "Test.kt")
	if err := os.WriteFile(path, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	for _, r := range v2rules.Registry {
		if r.ID == "AddJavascriptInterface" {
			dispatcher := rules.NewDispatcherV2([]*v2rules.Rule{r}, resolver)
			cols := dispatcher.Run(file)
			return cols.Findings()
		}
	}
	t.Fatal("AddJavascriptInterface rule not found")
	return nil
}

func TestGetInstance(t *testing.T) {
	t.Run("triggers on insecure Cipher algorithm", func(t *testing.T) {
		findings := runRuleByName(t, "GetInstance", `
package test
import javax.crypto.Cipher
class Crypto {
    fun encrypt() {
        val cipher = Cipher.getInstance("DES/CBC/PKCS5Padding")
    }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("triggers on ECB mode with AES", func(t *testing.T) {
		findings := runRuleByName(t, "GetInstance", `
package test
import javax.crypto.Cipher
class Crypto {
    fun encrypt() {
        val cipher = Cipher.getInstance("AES/ECB/PKCS5Padding")
    }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean code with AES passes", func(t *testing.T) {
		findings := runRuleByName(t, "GetInstance", `
package test
import javax.crypto.Cipher
class Crypto {
    fun encrypt() {
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("ignores comments mentioning insecure algorithm", func(t *testing.T) {
		findings := runRuleByName(t, "GetInstance", `
package test
import javax.crypto.Cipher
class Crypto {
    fun encrypt() {
        // Cipher.getInstance("DES") was removed in favour of AES
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("ignores custom Cipher class not from javax.crypto", func(t *testing.T) {
		findings := runRuleByName(t, "GetInstance", `
package test
class Cipher {
    companion object {
        fun getInstance(name: String): Cipher = Cipher()
    }
}
class Crypto {
    fun encrypt() {
        val cipher = Cipher.getInstance("DES")
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestEasterEgg(t *testing.T) {
	t.Run("triggers on easter egg comment", func(t *testing.T) {
		findings := runRuleByName(t, "EasterEgg", `
package test
class Game {
    // This is an easter egg feature
    fun secretStuff() {}
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean code passes", func(t *testing.T) {
		findings := runRuleByName(t, "EasterEgg", `
package test
class Game {
    // Normal game logic
    fun play() {}
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestExportedContentProvider(t *testing.T) {
	t.Run("triggers on ContentProvider without permission check", func(t *testing.T) {
		findings := runRuleByName(t, "ExportedContentProvider", `
package test
import android.content.ContentProvider
class MyProvider : ContentProvider() {
    override fun query() {}
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean code with permission enforcement passes", func(t *testing.T) {
		findings := runRuleByName(t, "ExportedContentProvider", `
package test
import android.content.ContentProvider
class MyProvider : ContentProvider() {
    override fun query() {
        enforceCallingPermission("com.example.READ", "No permission")
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("sibling class enforcement does not suppress provider finding", func(t *testing.T) {
		findings := runRuleByName(t, "ExportedContentProvider", `
package test
import android.content.ContentProvider
class Helper {
    fun check() {
        enforceCallingPermission("com.example.READ", "No permission")
    }
}
class MyProvider : ContentProvider() {
    override fun query() {}
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings; sibling class enforcement must not suppress")
		}
	})
	t.Run("class named ContentProvider without android import does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "ExportedContentProvider", `
package test
class MyProvider : ContentProvider() {
    override fun query() {}
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings without import, got %d", len(findings))
		}
	})
}

func TestExportedReceiver(t *testing.T) {
	t.Run("triggers on BroadcastReceiver subclass", func(t *testing.T) {
		findings := runRuleByName(t, "ExportedReceiver", `
package test
import android.content.BroadcastReceiver
class MyReceiver : BroadcastReceiver() {
    override fun onReceive() {}
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean code without receiver passes", func(t *testing.T) {
		findings := runRuleByName(t, "ExportedReceiver", `
package test
class MyService {
    fun doWork() {}
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestGrantAllUris(t *testing.T) {
	t.Run("triggers on grantUriPermission", func(t *testing.T) {
		findings := runRuleByName(t, "GrantAllUris", `
package test
class MyActivity {
    fun share() {
        grantUriPermission("com.example", uri, Intent.FLAG_GRANT_READ_URI_PERMISSION)
    }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean code passes", func(t *testing.T) {
		findings := runRuleByName(t, "GrantAllUris", `
package test
class MyActivity {
    fun share() {
        startActivity(intent)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestSecureRandom(t *testing.T) {
	t.Run("triggers on java.util.Random usage", func(t *testing.T) {
		findings := runRuleByName(t, "SecureRandom", `
package test
class TokenGen {
    fun generate() {
        val rng = java.util.Random()
    }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean code with SecureRandom passes", func(t *testing.T) {
		findings := runRuleByName(t, "SecureRandom", `
package test
import java.security.SecureRandom
class TokenGen {
    fun generate() {
        val rng = SecureRandom()
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestTrustedServer(t *testing.T) {
	t.Run("triggers on TrustAllCertificates", func(t *testing.T) {
		findings := runRuleByName(t, "TrustedServer", `
package test
class HttpClient {
    fun setup() {
        val manager = TrustAllCertificates()
    }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean code passes", func(t *testing.T) {
		findings := runRuleByName(t, "TrustedServer", `
package test
class HttpClient {
    fun setup() {
        val factory = SSLSocketFactory.getDefault()
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestWorldReadableFiles(t *testing.T) {
	t.Run("triggers on MODE_WORLD_READABLE", func(t *testing.T) {
		findings := runRuleByName(t, "WorldReadableFiles", `
package test
class Prefs {
    fun open() {
        getSharedPreferences("prefs", MODE_WORLD_READABLE)
    }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean code passes", func(t *testing.T) {
		findings := runRuleByName(t, "WorldReadableFiles", `
package test
class Prefs {
    fun open() {
        getSharedPreferences("prefs", MODE_PRIVATE)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestWorldWriteableFiles(t *testing.T) {
	t.Run("triggers on MODE_WORLD_WRITEABLE", func(t *testing.T) {
		findings := runRuleByName(t, "WorldWriteableFiles", `
package test
class FileHelper {
    fun create() {
        openFileOutput("data", MODE_WORLD_WRITEABLE)
    }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean code passes", func(t *testing.T) {
		findings := runRuleByName(t, "WorldWriteableFiles", `
package test
class FileHelper {
    fun create() {
        openFileOutput("data", MODE_PRIVATE)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestDrawAllocation(t *testing.T) {
	t.Run("triggers on allocation inside onDraw", func(t *testing.T) {
		findings := runRuleByName(t, "DrawAllocation", `
package test
class MyView {
    override fun onDraw(canvas: Canvas) {
        val paint = Paint()
        canvas.drawLine(0f, 0f, 100f, 100f, paint)
    }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean code with allocation outside onDraw passes", func(t *testing.T) {
		findings := runRuleByName(t, "DrawAllocation", `
package test
class MyView {
    private val paint = Paint()
    override fun onDraw(canvas: Canvas) {
        canvas.drawLine(0f, 0f, 100f, 100f, paint)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestFieldGetter(t *testing.T) {
	t.Run("triggers on getter call inside loop", func(t *testing.T) {
		findings := runRuleByName(t, "FieldGetter", `
package test
class MyClass {
    fun process(items: List<Item>) {
        for (item in items) {
            val name = item.getName()
        }
    }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean code without loop passes", func(t *testing.T) {
		findings := runRuleByName(t, "FieldGetter", `
package test
class MyClass {
    fun process(item: Item) {
        val name = item.getName()
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestFloatMath(t *testing.T) {
	t.Run("triggers on FloatMath usage", func(t *testing.T) {
		findings := runRuleByName(t, "FloatMath", `
package test
class Calc {
    fun compute(x: Float): Float {
        return FloatMath.sin(x)
    }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean code with kotlin.math passes", func(t *testing.T) {
		findings := runRuleByName(t, "FloatMath", `
package test
import kotlin.math.sin
class Calc {
    fun compute(x: Float): Float {
        return sin(x)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestHandlerLeak(t *testing.T) {
	t.Run("triggers on Kotlin inner Handler class", func(t *testing.T) {
		findings := runRuleByName(t, "HandlerLeak", `
package test
import android.os.Handler
class MyActivity {
    inner class MyHandler : Handler()
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("triggers on anonymous Handler object expression", func(t *testing.T) {
		findings := runRuleByName(t, "HandlerLeak", `
package test
class MyActivity {
    val handler = object : Handler(Looper.getMainLooper()) {
        override fun handleMessage(msg: Message) {}
    }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("triggers on Java non-static inner Handler class", func(t *testing.T) {
		findings := runJavaRuleByName(t, "HandlerLeak", `
package test;
import android.os.Handler;
class Outer {
    class MyHandler extends Handler {
    }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("triggers on Java anonymous Handler", func(t *testing.T) {
		findings := runJavaRuleByName(t, "HandlerLeak", `
package test;
import android.os.Handler;
class Outer {
    Object handler = new Handler() {
    };
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("ignores Java static Handler class", func(t *testing.T) {
		findings := runJavaRuleByName(t, "HandlerLeak", `
package test;
import android.os.Handler;
class Outer {
    static class MyHandler extends Handler {
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("ignores Java Handler constructor with Looper super call", func(t *testing.T) {
		findings := runJavaRuleByName(t, "HandlerLeak", `
package test;
import android.os.Handler;
import android.os.Looper;
class Outer {
    class MyHandler extends Handler {
        MyHandler(Looper looper) {
            super(looper);
        }
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("ignores Kotlin inner Handler with Looper constructor", func(t *testing.T) {
		findings := runRuleByName(t, "HandlerLeak", `
package test
import android.os.Handler
import android.os.Looper
class Outer {
    inner class MyHandler(looper: Looper) : Handler(looper)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("resolver rejects local Handler supertype", func(t *testing.T) {
		findings := runRuleByNameWithResolver(t, "HandlerLeak", `
package test
open class Handler
class Outer {
    inner class MyHandler : Handler()
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("clean code without Handler passes", func(t *testing.T) {
		findings := runRuleByName(t, "HandlerLeak", `
package test
class MyActivity {
    fun doWork() {
        val executor = Executors.newSingleThreadExecutor()
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func runJavaRuleByName(t *testing.T, ruleName string, code string) []scanner.Finding {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.java")
	if err := os.WriteFile(path, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseJavaFile(path)
	if err != nil {
		t.Fatal(err)
	}
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	for _, r := range v2rules.Registry {
		if r.ID == ruleName {
			d := rules.NewDispatcherV2([]*v2rules.Rule{r}, resolver)
			cols := d.Run(file)
			return cols.Findings()
		}
	}
	t.Fatalf("rule %q not found in registry", ruleName)
	return nil
}

func TestRecycle(t *testing.T) {
	t.Run("triggers on TypedArray without recycle", func(t *testing.T) {
		findings := runRuleByName(t, "Recycle", `
package test
class MyView {
    fun init() {
        val a: TypedArray = context.obtainStyledAttributes(attrs)
        val color = a.getColor(0, 0)
    }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean code with recycle call passes", func(t *testing.T) {
		findings := runRuleByName(t, "Recycle", `
package test
class MyView {
    fun init() {
        val a: TypedArray = context.obtainStyledAttributes(attrs)
        val color = a.getColor(0, 0)
        a.recycle()
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestByteOrderMark(t *testing.T) {
	t.Run("triggers on file with BOM", func(t *testing.T) {
		findings := runRuleByName(t, "ByteOrderMark", "\xEF\xBB\xBF"+`
package test
class MyClass {}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean file without BOM passes", func(t *testing.T) {
		findings := runRuleByName(t, "ByteOrderMark", `
package test
class MyClass {}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}
