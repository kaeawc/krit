package rules_test

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
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
	t.Run("Java WebView receiver reports reflection hazard", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "AddJavascriptInterface", `
package test;
import android.webkit.WebView;
class Browser {
  void setup(WebView webView, Object bridge) {
    webView.addJavascriptInterface(bridge, "Android");
  }
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 Java finding, got %d", len(findings))
		}
	})
	t.Run("Java local lookalike does not fire", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "AddJavascriptInterface", `
package test;
class Wrapper {
  void addJavascriptInterface(Object bridge, String name) {}
}
class Browser {
  void setup(Wrapper wrapper, Object bridge) {
    wrapper.addJavascriptInterface(bridge, "Android");
  }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for Java local lookalike, got %d", len(findings))
		}
	})
}

func runAddJavascriptInterfaceProjectRule(t *testing.T, minSdk, targetSdk int, code string) []scanner.Finding {
	return runAndroidSecurityProjectRule(t, "AddJavascriptInterface", "Test.kt", minSdk, targetSdk, code)
}

func runAndroidSecurityProjectRule(t *testing.T, ruleName, fileName string, minSdk, targetSdk int, code string) []scanner.Finding {
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
	src := filepath.Join(root, "src", "main", "java", "example")
	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(src, fileName)
	if err := os.WriteFile(path, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	var file *scanner.File
	var err error
	if strings.HasSuffix(fileName, ".java") {
		file, err = scanner.ParseJavaFile(context.Background(), path)
	} else {
		file, err = scanner.ParseFile(context.Background(), path)
	}
	if err != nil {
		t.Fatal(err)
	}
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	for _, r := range api.Registry {
		if r.ID == ruleName {
			dispatcher := rules.NewDispatcher([]*api.Rule{r}, resolver)
			cols := dispatcher.Run(file)
			return cols.Findings()
		}
	}
	t.Fatalf("%s rule not found", ruleName)
	return nil
}

func TestUnprotectedDynamicReceiver(t *testing.T) {
	t.Run("Kotlin four argument null permission", func(t *testing.T) {
		findings := runRuleByName(t, "UnprotectedDynamicReceiver", `
package test
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter

class ReceiverSetup(private val context: Context, private val receiver: BroadcastReceiver) {
    fun setup() {
        context.registerReceiver(receiver, IntentFilter(Intent.ACTION_SCREEN_ON), null, null)
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("Kotlin two argument overload", func(t *testing.T) {
		findings := runRuleByName(t, "UnprotectedDynamicReceiver", `
package test
import android.app.Activity
import android.content.BroadcastReceiver
import android.content.Intent
import android.content.IntentFilter

class MainActivity : Activity() {
    fun setup(receiver: BroadcastReceiver) {
        registerReceiver(receiver, IntentFilter(Intent.ACTION_USER_PRESENT))
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("Kotlin apply filter action", func(t *testing.T) {
		findings := runRuleByName(t, "UnprotectedDynamicReceiver", `
package test
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter

class ReceiverSetup(private val context: Context) {
    fun setup(receiver: BroadcastReceiver) {
        context.registerReceiver(receiver, IntentFilter().apply {
            addAction(Intent.ACTION_BOOT_COMPLETED)
        }, null, null)
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding for inline apply filter action, got %d", len(findings))
		}
	})
	t.Run("Kotlin non-null permission is clean", func(t *testing.T) {
		findings := runRuleByName(t, "UnprotectedDynamicReceiver", `
package test
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter

class ReceiverSetup(private val context: Context, private val receiver: BroadcastReceiver) {
    fun setup() {
        context.registerReceiver(receiver, IntentFilter(Intent.ACTION_SCREEN_ON), "com.example.PRIVATE", null)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings with non-null permission, got %d", len(findings))
		}
	})
	t.Run("Kotlin local lookalike is clean", func(t *testing.T) {
		findings := runRuleByName(t, "UnprotectedDynamicReceiver", `
package test
class Registry {
    fun registerReceiver(receiver: Any, filter: Any, permission: Any?, handler: Any?) {}
}
class ReceiverSetup(private val registry: Registry) {
    fun setup(receiver: Any, filter: Any) {
        registry.registerReceiver(receiver, filter, null, null)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for local lookalike, got %d", len(findings))
		}
	})
	t.Run("Java four argument null permission", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "UnprotectedDynamicReceiver", `
package test;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;

class ReceiverSetup {
  void setup(Context context, BroadcastReceiver receiver) {
    context.registerReceiver(receiver, new IntentFilter(Intent.ACTION_SCREEN_ON), null, null);
  }
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 Java finding, got %d", len(findings))
		}
	})
	t.Run("Java non-null permission is clean", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "UnprotectedDynamicReceiver", `
package test;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;

class ReceiverSetup {
  void setup(Context context, BroadcastReceiver receiver) {
    context.registerReceiver(receiver, new IntentFilter(Intent.ACTION_SCREEN_ON), "com.example.PRIVATE", null);
  }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 Java findings with non-null permission, got %d", len(findings))
		}
	})
}

func TestBroadcastReceiverExportedFlagMissing(t *testing.T) {
	t.Run("Kotlin missing flags on target sdk 34", func(t *testing.T) {
		findings := runAndroidSecurityProjectRule(t, "BroadcastReceiverExportedFlagMissing", "Test.kt", 23, 34, `
package test
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter

class ReceiverSetup(private val context: Context, private val receiver: BroadcastReceiver) {
    fun setup() {
        context.registerReceiver(receiver, IntentFilter(Intent.ACTION_SCREEN_ON))
        context.registerReceiver(receiver, IntentFilter(Intent.ACTION_USER_PRESENT), 0)
    }
}
`)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d", len(findings))
		}
	})
	t.Run("Kotlin exported flags are clean", func(t *testing.T) {
		findings := runAndroidSecurityProjectRule(t, "BroadcastReceiverExportedFlagMissing", "Test.kt", 23, 34, `
package test
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import androidx.core.content.ContextCompat

class ReceiverSetup(private val context: Context, private val receiver: BroadcastReceiver) {
    fun setup() {
        context.registerReceiver(receiver, IntentFilter(Intent.ACTION_SCREEN_ON), Context.RECEIVER_NOT_EXPORTED)
        context.registerReceiver(receiver, IntentFilter(Intent.ACTION_USER_PRESENT), Context.RECEIVER_EXPORTED or Context.RECEIVER_VISIBLE_TO_INSTANT_APPS)
        ContextCompat.registerReceiver(context, receiver, IntentFilter(Intent.ACTION_SCREEN_ON), ContextCompat.RECEIVER_NOT_EXPORTED)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings with exported flags, got %d", len(findings))
		}
	})
	t.Run("Kotlin target sdk below 34 is clean", func(t *testing.T) {
		findings := runAndroidSecurityProjectRule(t, "BroadcastReceiverExportedFlagMissing", "Test.kt", 23, 33, `
package test
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter

class ReceiverSetup(private val context: Context, private val receiver: BroadcastReceiver) {
    fun setup() {
        context.registerReceiver(receiver, IntentFilter(Intent.ACTION_SCREEN_ON))
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings below targetSdk 34, got %d", len(findings))
		}
	})
	t.Run("Kotlin local lookalike is clean", func(t *testing.T) {
		findings := runAndroidSecurityProjectRule(t, "BroadcastReceiverExportedFlagMissing", "Test.kt", 23, 34, `
package test
class Registry {
    fun registerReceiver(receiver: Any, filter: Any) {}
}
class ReceiverSetup(private val registry: Registry) {
    fun setup(receiver: Any, filter: Any) {
        registry.registerReceiver(receiver, filter)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for local lookalike, got %d", len(findings))
		}
	})
	t.Run("Kotlin no sdk context uses reduced confidence", func(t *testing.T) {
		findings := runRuleByName(t, "BroadcastReceiverExportedFlagMissing", `
package test
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter

class ReceiverSetup(private val context: Context, private val receiver: BroadcastReceiver) {
    fun setup() {
        context.registerReceiver(receiver, IntentFilter(Intent.ACTION_SCREEN_ON))
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding without sdk context, got %d", len(findings))
		}
		if findings[0].Confidence != 0.65 {
			t.Fatalf("expected reduced confidence 0.65, got %.2f", findings[0].Confidence)
		}
	})
	t.Run("Java missing flags on target sdk 34", func(t *testing.T) {
		findings := runAndroidSecurityProjectRule(t, "BroadcastReceiverExportedFlagMissing", "Test.java", 23, 34, `
package test;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;

class ReceiverSetup {
  void setup(Context context, BroadcastReceiver receiver) {
    context.registerReceiver(receiver, new IntentFilter(Intent.ACTION_SCREEN_ON));
    context.registerReceiver(receiver, new IntentFilter(Intent.ACTION_USER_PRESENT), 0);
  }
}`)
		if len(findings) != 2 {
			t.Fatalf("expected 2 Java findings, got %d", len(findings))
		}
	})
	t.Run("Java exported flags are clean", func(t *testing.T) {
		findings := runAndroidSecurityProjectRule(t, "BroadcastReceiverExportedFlagMissing", "Test.java", 23, 34, `
package test;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import androidx.core.content.ContextCompat;

class ReceiverSetup {
  void setup(Context context, BroadcastReceiver receiver) {
    context.registerReceiver(receiver, new IntentFilter(Intent.ACTION_SCREEN_ON), Context.RECEIVER_NOT_EXPORTED);
    ContextCompat.registerReceiver(context, receiver, new IntentFilter(Intent.ACTION_SCREEN_ON), ContextCompat.RECEIVER_NOT_EXPORTED);
  }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 Java findings with exported flags, got %d", len(findings))
		}
	})
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

func TestRsaNoPadding(t *testing.T) {
	t.Run("Kotlin flags RSA NoPadding transformations", func(t *testing.T) {
		for _, algo := range []string{"RSA/ECB/NoPadding", "RSA/NONE/NoPadding"} {
			findings := runRuleByName(t, "RsaNoPadding", `
package test
import javax.crypto.Cipher

class Crypto {
    fun cipher() {
        Cipher.getInstance("`+algo+`")
    }
}
`)
			if len(findings) != 1 {
				t.Fatalf("expected 1 finding for %s, got %d: %v", algo, len(findings), findings)
			}
		}
	})
	t.Run("Kotlin accepts padded RSA, AES, and local lookalikes", func(t *testing.T) {
		findings := runRuleByName(t, "RsaNoPadding", `
package test

class Cipher {
    companion object {
        fun getInstance(name: String): Cipher = Cipher()
    }
}

class Crypto {
    fun cipher() {
        javax.crypto.Cipher.getInstance("RSA/ECB/OAEPWithSHA-256AndMGF1Padding")
        javax.crypto.Cipher.getInstance("RSA/ECB/PKCS1Padding")
        javax.crypto.Cipher.getInstance("AES/GCM/NoPadding")
        Cipher.getInstance("RSA/ECB/NoPadding")
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Java flags RSA NoPadding transformations", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "RsaNoPadding", `
package test;
import javax.crypto.Cipher;

class Crypto {
    void cipher() throws Exception {
        Cipher.getInstance("RSA/ECB/NoPadding");
        javax.crypto.Cipher.getInstance("rsa/none/nopadding");
    }
}
`)
		if len(findings) != 2 {
			t.Fatalf("expected 2 Java findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Java accepts padded RSA, AES, and local lookalikes", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "RsaNoPadding", `
package test;

class Cipher {
    static Cipher getInstance(String name) { return new Cipher(); }
}

class Crypto {
    void cipher() throws Exception {
        javax.crypto.Cipher.getInstance("RSA/ECB/OAEPWithSHA-256AndMGF1Padding");
        javax.crypto.Cipher.getInstance("RSA/ECB/PKCS1Padding");
        javax.crypto.Cipher.getInstance("AES/GCM/NoPadding");
        Cipher.getInstance("RSA/ECB/NoPadding");
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 Java findings, got %d: %v", len(findings), findings)
		}
	})
}

func TestPrngFromSystemTime(t *testing.T) {
	t.Run("Kotlin flags time-seeded Random in crypto file", func(t *testing.T) {
		findings := runRuleByName(t, "PrngFromSystemTime", `
package test
import java.util.Random
import javax.crypto.Cipher

class Crypto {
    fun rng() {
        Random(System.currentTimeMillis())
        Random(System.nanoTime())
    }
}
`)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Kotlin skips non-crypto and SecureRandom", func(t *testing.T) {
		findings := runRuleByName(t, "PrngFromSystemTime", `
package test
import java.security.SecureRandom
import java.util.Random

class Crypto {
    fun rng(seed: Long) {
        SecureRandom()
        Random(seed)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Java flags time-seeded Random in crypto file", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "PrngFromSystemTime", `
package test;
import java.util.Random;
import javax.crypto.Cipher;

class Crypto {
    void rng() {
        new Random(System.currentTimeMillis());
        new java.util.Random(System.nanoTime());
    }
}
`)
		if len(findings) != 2 {
			t.Fatalf("expected 2 Java findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Java skips non-crypto and SecureRandom", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "PrngFromSystemTime", `
package test;
import java.security.SecureRandom;
import java.util.Random;

class Crypto {
    void rng(long seed) {
        new SecureRandom();
        new Random(seed);
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 Java findings, got %d: %v", len(findings), findings)
		}
	})
}

func TestOkHttpDisableSslValidation(t *testing.T) {
	t.Run("Kotlin flags always-true verifier and unsafe trust manager", func(t *testing.T) {
		findings := runRuleByName(t, "OkHttpDisableSslValidation", `
package test
import okhttp3.OkHttpClient
import javax.net.ssl.SSLSocketFactory
import javax.net.ssl.X509TrustManager

class ClientFactory {
    fun verifier() = OkHttpClient.Builder()
        .hostnameVerifier { _, _ -> true }
        .build()

    fun trust(socketFactory: SSLSocketFactory, unsafeTrustManager: X509TrustManager) =
        OkHttpClient.Builder()
            .sslSocketFactory(socketFactory, unsafeTrustManager)
            .build()
}
`)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Kotlin accepts default builder and validating callbacks", func(t *testing.T) {
		findings := runRuleByName(t, "OkHttpDisableSslValidation", `
package test
import okhttp3.OkHttpClient
import javax.net.ssl.SSLSocketFactory
import javax.net.ssl.X509TrustManager

class ClientFactory {
    fun defaultClient() = OkHttpClient.Builder().build()

    fun verifier() = OkHttpClient.Builder()
        .hostnameVerifier { host, session -> host == session.peerHost }
        .build()

    fun trust(socketFactory: SSLSocketFactory, validatingManager: X509TrustManager) =
        OkHttpClient.Builder()
            .sslSocketFactory(socketFactory, validatingManager)
            .build()
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Java flags always-true verifier and unsafe trust manager", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "OkHttpDisableSslValidation", `
package test;
import okhttp3.OkHttpClient;
import javax.net.ssl.SSLSocketFactory;
import javax.net.ssl.X509TrustManager;

class ClientFactory {
    OkHttpClient verifier() {
        return new OkHttpClient.Builder()
            .hostnameVerifier((hostname, session) -> true)
            .build();
    }

    OkHttpClient trust(SSLSocketFactory socketFactory, X509TrustManager unsafeTrustManager) {
        return new OkHttpClient.Builder()
            .sslSocketFactory(socketFactory, unsafeTrustManager)
            .build();
    }
}
`)
		if len(findings) != 2 {
			t.Fatalf("expected 2 Java findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Java accepts default builder and validating callbacks", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "OkHttpDisableSslValidation", `
package test;
import okhttp3.OkHttpClient;
import javax.net.ssl.SSLSocketFactory;
import javax.net.ssl.X509TrustManager;

class ClientFactory {
    OkHttpClient defaultClient() {
        return new OkHttpClient.Builder().build();
    }

    OkHttpClient verifier() {
        return new OkHttpClient.Builder()
            .hostnameVerifier((hostname, session) -> hostname.equals(session.getPeerHost()))
            .build();
    }

    OkHttpClient trust(SSLSocketFactory socketFactory, X509TrustManager validatingManager) {
        return new OkHttpClient.Builder()
            .sslSocketFactory(socketFactory, validatingManager)
            .build();
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 Java findings, got %d: %v", len(findings), findings)
		}
	})
}

func TestDisableCertificatePinning(t *testing.T) {
	t.Run("Kotlin flags empty CertificatePinner builder", func(t *testing.T) {
		findings := runRuleByName(t, "DisableCertificatePinning", `
package test
import okhttp3.CertificatePinner

fun pinner() {
    CertificatePinner.Builder().build()
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Kotlin accepts add and local lookalikes", func(t *testing.T) {
		findings := runRuleByName(t, "DisableCertificatePinning", `
package test
import okhttp3.CertificatePinner

class Builder {
    fun build() = Any()
}

fun pinner() {
    CertificatePinner.Builder()
        .add("example.com", "sha256/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
        .build()
    Builder().build()
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Java flags empty CertificatePinner builder", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "DisableCertificatePinning", `
package test;
import okhttp3.CertificatePinner;

class Pins {
    void pinner() {
        new CertificatePinner.Builder().build();
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 Java finding, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Java accepts add and local lookalikes", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "DisableCertificatePinning", `
package test;
import okhttp3.CertificatePinner;

class Builder {
    Object build() { return new Object(); }
}
class Pins {
    void pinner() {
        new CertificatePinner.Builder()
            .add("example.com", "sha256/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
            .build();
        new Builder().build();
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 Java findings, got %d: %v", len(findings), findings)
		}
	})
}

func TestAllowAllHostnameVerifier(t *testing.T) {
	t.Run("Kotlin flags expression and return true verifiers", func(t *testing.T) {
		findings := runRuleByName(t, "AllowAllHostnameVerifier", `
package test
import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

class AllowAllExpression : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = true
}

class AllowAllBlock : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean {
        return true
    }
}
`)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Kotlin skips validating, delegated, and local lookalike verifiers", func(t *testing.T) {
		findings := runRuleByName(t, "AllowAllHostnameVerifier", `
package test
import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

class ValidatingVerifier : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = hostname == session.peerHost
}

class DelegatingVerifier(delegate: HostnameVerifier) : HostnameVerifier by delegate
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}

		lookalike := runRuleByName(t, "AllowAllHostnameVerifier", `
package test
interface HostnameVerifier
class LocalVerifier : HostnameVerifier {
    fun verify(hostname: String, session: Any): Boolean = true
}
`)
		if len(lookalike) != 0 {
			t.Fatalf("expected 0 local-lookalike findings, got %d: %v", len(lookalike), lookalike)
		}
	})
	t.Run("Java flags return true verifier", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "AllowAllHostnameVerifier", `
package test;
import javax.net.ssl.HostnameVerifier;
import javax.net.ssl.SSLSession;

class AllowAll implements HostnameVerifier {
    public boolean verify(String hostname, SSLSession session) {
        return true;
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 Java finding, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Java skips validating and local lookalike verifiers", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "AllowAllHostnameVerifier", `
package test;
import javax.net.ssl.HostnameVerifier;
import javax.net.ssl.SSLSession;

class ValidatingVerifier implements HostnameVerifier {
    public boolean verify(String hostname, SSLSession session) {
        return hostname.equals(session.getPeerHost());
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 Java findings, got %d: %v", len(findings), findings)
		}

		lookalike := runRuleByNameOnJava(t, "AllowAllHostnameVerifier", `
package test;
interface HostnameVerifier {}
class LocalVerifier implements HostnameVerifier {
    public boolean verify(String hostname, Object session) {
        return true;
    }
}
`)
		if len(lookalike) != 0 {
			t.Fatalf("expected 0 Java local-lookalike findings, got %d: %v", len(lookalike), lookalike)
		}
	})
}

func TestInsecureTrustManager(t *testing.T) {
	t.Run("Kotlin flags empty and bare-return trust checks", func(t *testing.T) {
		findings := runRuleByName(t, "InsecureTrustManager", `
package test
import java.security.cert.X509Certificate
import javax.net.ssl.X509TrustManager

class TrustAll : X509TrustManager {
    override fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {}
    override fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {
        return
    }
    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}

fun manager(): X509TrustManager = object : X509TrustManager {
    override fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {}
    override fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {}
    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}
`)
		if len(findings) != 4 {
			t.Fatalf("expected 4 findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Kotlin skips delegate, real checks, throws, and local lookalikes", func(t *testing.T) {
		findings := runRuleByName(t, "InsecureTrustManager", `
package test
import java.security.cert.CertificateException
import java.security.cert.X509Certificate
import javax.net.ssl.X509TrustManager

class DelegatingTrustManager(delegate: X509TrustManager) : X509TrustManager by delegate

class ValidatingTrustManager : X509TrustManager {
    override fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {
        if (chain.isNullOrEmpty()) throw CertificateException("missing chain")
    }
    override fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {
        throw CertificateException("untrusted")
    }
    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}

		lookalike := runRuleByName(t, "InsecureTrustManager", `
package test
interface X509TrustManager
class LocalTrustAll : X509TrustManager {
    fun checkServerTrusted() {}
}
`)
		if len(lookalike) != 0 {
			t.Fatalf("expected 0 local-lookalike findings, got %d: %v", len(lookalike), lookalike)
		}
	})
	t.Run("Java flags empty and bare-return trust checks", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "InsecureTrustManager", `
package test;
import java.security.cert.X509Certificate;
import javax.net.ssl.X509TrustManager;

class TrustAll implements X509TrustManager {
    public void checkClientTrusted(X509Certificate[] chain, String authType) {}
    public void checkServerTrusted(X509Certificate[] chain, String authType) {
        return;
    }
    public X509Certificate[] getAcceptedIssuers() { return new X509Certificate[0]; }
}
`)
		if len(findings) != 2 {
			t.Fatalf("expected 2 Java findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Java skips real checks and local lookalikes", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "InsecureTrustManager", `
package test;
import java.security.cert.CertificateException;
import java.security.cert.X509Certificate;
import javax.net.ssl.X509TrustManager;

class ValidatingTrustManager implements X509TrustManager {
    public void checkClientTrusted(X509Certificate[] chain, String authType) throws CertificateException {
        if (chain == null || chain.length == 0) throw new CertificateException("missing chain");
    }
    public void checkServerTrusted(X509Certificate[] chain, String authType) throws CertificateException {
        throw new CertificateException("untrusted");
    }
    public X509Certificate[] getAcceptedIssuers() { return new X509Certificate[0]; }
}

interface X509TrustManagerLocal {}
class LocalTrustAll implements X509TrustManagerLocal {
    public void checkServerTrusted() {}
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 Java findings, got %d: %v", len(findings), findings)
		}
	})
}

func TestImplicitPendingIntent(t *testing.T) {
	t.Run("Kotlin targetSdk 31 flags factory calls without mutability", func(t *testing.T) {
		findings := runAndroidSecurityProjectRule(t, "ImplicitPendingIntent", "Test.kt", 23, 31, `
package test
import android.app.PendingIntent
import android.content.Context
import android.content.Intent

fun schedule(context: Context, intent: Intent) {
    PendingIntent.getBroadcast(context, 0, intent, PendingIntent.FLAG_UPDATE_CURRENT)
    PendingIntent.getActivities(context, 0, arrayOf(intent), PendingIntent.FLAG_UPDATE_CURRENT)
}
`)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Kotlin accepts explicit mutability and PendingIntentCompat", func(t *testing.T) {
		findings := runAndroidSecurityProjectRule(t, "ImplicitPendingIntent", "Test.kt", 23, 31, `
package test
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import androidx.core.app.PendingIntentCompat

fun schedule(context: Context, intent: Intent) {
    PendingIntent.getBroadcast(context, 0, intent, PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE)
    PendingIntent.getService(context, 0, intent, flags = PendingIntent.FLAG_MUTABLE)
    PendingIntentCompat.getBroadcast(context, 0, intent, PendingIntent.FLAG_UPDATE_CURRENT, false)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("targetSdk below 31 is clean", func(t *testing.T) {
		findings := runAndroidSecurityProjectRule(t, "ImplicitPendingIntent", "Test.kt", 23, 30, `
package test
import android.app.PendingIntent
import android.content.Context
import android.content.Intent

fun schedule(context: Context, intent: Intent) {
    PendingIntent.getActivity(context, 0, intent, PendingIntent.FLAG_UPDATE_CURRENT)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for targetSdk 30, got %d: %v", len(findings), findings)
		}
	})
	t.Run("local lookalike without android import does not fire", func(t *testing.T) {
		findings := runRuleByName(t, "ImplicitPendingIntent", `
package test
class PendingIntent {
    companion object {
        const val FLAG_UPDATE_CURRENT = 1
        fun getBroadcast(context: Any, requestCode: Int, intent: Any, flags: Int) = Unit
    }
}
fun schedule(context: Any, intent: Any) {
    PendingIntent.getBroadcast(context, 0, intent, PendingIntent.FLAG_UPDATE_CURRENT)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for local lookalike, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Java targetSdk 31 flags factory call without mutability", func(t *testing.T) {
		findings := runAndroidSecurityProjectRule(t, "ImplicitPendingIntent", "Test.java", 23, 31, `
package test;
import android.app.PendingIntent;
import android.content.Context;
import android.content.Intent;

class Scheduler {
    void schedule(Context context, Intent intent) {
        PendingIntent.getBroadcast(context, 0, intent, PendingIntent.FLAG_UPDATE_CURRENT);
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 Java finding, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Java accepts explicit mutability", func(t *testing.T) {
		findings := runAndroidSecurityProjectRule(t, "ImplicitPendingIntent", "Test.java", 23, 31, `
package test;
import android.app.PendingIntent;
import android.content.Context;
import android.content.Intent;

class Scheduler {
    void schedule(Context context, Intent intent) {
        PendingIntent.getService(context, 0, intent, PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE);
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 Java findings, got %d: %v", len(findings), findings)
		}
	})
}

func TestWeakMessageDigest(t *testing.T) {
	t.Run("Kotlin flags weak digest algorithms", func(t *testing.T) {
		for _, algo := range []string{"MD5", "SHA-1", "SHA1", "MD2", "MD4"} {
			findings := runRuleByName(t, "WeakMessageDigest", `
package test
import java.security.MessageDigest

class Crypto {
    fun hash() {
        MessageDigest.getInstance("`+algo+`")
    }
}
`)
			if len(findings) != 1 {
				t.Fatalf("expected 1 finding for %s, got %d: %v", algo, len(findings), findings)
			}
		}
	})
	t.Run("Kotlin accepts stronger digest algorithms", func(t *testing.T) {
		findings := runRuleByName(t, "WeakMessageDigest", `
package test
import java.security.MessageDigest

class Crypto {
    fun hash() {
        MessageDigest.getInstance("SHA-256")
        MessageDigest.getInstance("SHA-384")
        MessageDigest.getInstance("SHA-512")
        MessageDigest.getInstance("SHA3-256")
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Kotlin ignores local lookalike", func(t *testing.T) {
		findings := runRuleByName(t, "WeakMessageDigest", `
package test

class MessageDigest {
    companion object {
        fun getInstance(name: String): MessageDigest = MessageDigest()
    }
}

class Crypto {
    fun hash() {
        MessageDigest.getInstance("MD5")
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Java flags weak digest algorithms", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "WeakMessageDigest", `
package test;
import java.security.MessageDigest;

class Crypto {
    void hash() throws Exception {
        MessageDigest.getInstance("md5");
        java.security.MessageDigest.getInstance("SHA1");
    }
}
`)
		if len(findings) != 2 {
			t.Fatalf("expected 2 Java findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Java accepts stronger algorithms and local lookalikes", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "WeakMessageDigest", `
package test;

class MessageDigest {
    static MessageDigest getInstance(String name) { return new MessageDigest(); }
}

class Crypto {
    void hash() throws Exception {
        MessageDigest.getInstance("MD5");
        java.security.MessageDigest.getInstance("SHA-512");
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 Java findings, got %d: %v", len(findings), findings)
		}
	})
}

func TestWeakMacAlgorithm(t *testing.T) {
	t.Run("Kotlin flags weak HMAC algorithms", func(t *testing.T) {
		for _, algo := range []string{"HmacMD5", "HmacSHA1", "HmacMD2", "HmacSHA0"} {
			findings := runRuleByName(t, "WeakMacAlgorithm", `
package test
import javax.crypto.Mac

class Crypto {
    fun mac() {
        Mac.getInstance("`+algo+`")
    }
}
`)
			if len(findings) != 1 {
				t.Fatalf("expected 1 finding for %s, got %d: %v", algo, len(findings), findings)
			}
		}
	})
	t.Run("Kotlin accepts stronger HMAC algorithms", func(t *testing.T) {
		findings := runRuleByName(t, "WeakMacAlgorithm", `
package test
import javax.crypto.Mac

class Crypto {
    fun mac() {
        Mac.getInstance("HmacSHA256")
        Mac.getInstance("HmacSHA384")
        Mac.getInstance("HmacSHA512")
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Kotlin ignores local lookalike", func(t *testing.T) {
		findings := runRuleByName(t, "WeakMacAlgorithm", `
package test

class Mac {
    companion object {
        fun getInstance(name: String): Mac = Mac()
    }
}

class Crypto {
    fun mac() {
        Mac.getInstance("HmacSHA1")
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Java flags weak HMAC algorithms", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "WeakMacAlgorithm", `
package test;
import javax.crypto.Mac;

class Crypto {
    void mac() throws Exception {
        Mac.getInstance("hmacmd5");
        javax.crypto.Mac.getInstance("HmacSHA1");
    }
}
`)
		if len(findings) != 2 {
			t.Fatalf("expected 2 Java findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Java accepts stronger algorithms and local lookalikes", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "WeakMacAlgorithm", `
package test;

class Mac {
    static Mac getInstance(String name) { return new Mac(); }
}

class Crypto {
    void mac() throws Exception {
        Mac.getInstance("HmacSHA1");
        javax.crypto.Mac.getInstance("HmacSHA512");
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 Java findings, got %d: %v", len(findings), findings)
		}
	})
}

func TestWeakKeySize(t *testing.T) {
	t.Run("Kotlin flags weak RSA and AES sizes", func(t *testing.T) {
		findings := runRuleByName(t, "WeakKeySize", `
package test
import java.security.KeyPairGenerator
import javax.crypto.KeyGenerator

class Crypto {
    fun keys() {
        val rsa = KeyPairGenerator.getInstance("RSA")
        rsa.initialize(1024)
        val aes = KeyGenerator.getInstance("AES")
        aes.init(64)
    }
}
`)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Kotlin accepts strong and dynamic sizes", func(t *testing.T) {
		findings := runRuleByName(t, "WeakKeySize", `
package test
import java.security.KeyPairGenerator
import javax.crypto.KeyGenerator

class Crypto {
    fun keys(size: Int) {
        val rsa = KeyPairGenerator.getInstance("RSA")
        rsa.initialize(2048)
        val aes = KeyGenerator.getInstance("AES")
        aes.init(128)
        val hmac = KeyGenerator.getInstance("HmacSHA256")
        hmac.init(256)
        aes.init(size)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Kotlin ignores unrelated receiver", func(t *testing.T) {
		findings := runRuleByName(t, "WeakKeySize", `
package test

class Generator {
    fun initialize(size: Int) {}
}

fun keys() {
    val rsa = Generator()
    rsa.initialize(1024)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Java flags weak RSA and AES sizes", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "WeakKeySize", `
package test;
import java.security.KeyPairGenerator;
import javax.crypto.KeyGenerator;

class Crypto {
    void keys() throws Exception {
        KeyPairGenerator rsa = KeyPairGenerator.getInstance("RSA");
        rsa.initialize(1024);
        KeyGenerator aes = KeyGenerator.getInstance("AES");
        aes.init(64);
    }
}
`)
		if len(findings) != 2 {
			t.Fatalf("expected 2 Java findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Java accepts strong sizes and local lookalikes", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "WeakKeySize", `
package test;

class KeyGenerator {
    static KeyGenerator getInstance(String algorithm) { return new KeyGenerator(); }
    void init(int size) {}
}

class Crypto {
    void keys(int size) throws Exception {
        java.security.KeyPairGenerator rsa = java.security.KeyPairGenerator.getInstance("RSA");
        rsa.initialize(2048);
        javax.crypto.KeyGenerator aes = javax.crypto.KeyGenerator.getInstance("AES");
        aes.init(128);
        KeyGenerator local = KeyGenerator.getInstance("AES");
        local.init(64);
        aes.init(size);
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 Java findings, got %d: %v", len(findings), findings)
		}
	})
}

func TestStaticIv(t *testing.T) {
	t.Run("Kotlin flags literal byte arrays and literal string bytes", func(t *testing.T) {
		findings := runRuleByName(t, "StaticIv", `
package test
import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.IvParameterSpec

class Crypto {
    fun params() {
        IvParameterSpec(byteArrayOf(0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0))
        GCMParameterSpec(128, "000000000000".toByteArray())
    }
}
`)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Kotlin ignores parameter, field, random-derived, and local lookalikes", func(t *testing.T) {
		findings := runRuleByName(t, "StaticIv", `
package test
import java.security.SecureRandom
import javax.crypto.spec.IvParameterSpec

class IvParameterSpec(bytes: ByteArray)

class Crypto {
    private val field = ByteArray(16)
    fun params(param: ByteArray) {
        val random = ByteArray(16)
        SecureRandom().nextBytes(random)
        javax.crypto.spec.IvParameterSpec(param)
        javax.crypto.spec.IvParameterSpec(field)
        javax.crypto.spec.IvParameterSpec(random)
        IvParameterSpec(byteArrayOf(0, 0, 0, 0))
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Java flags literal byte arrays and literal string bytes", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "StaticIv", `
package test;
import javax.crypto.spec.GCMParameterSpec;
import javax.crypto.spec.IvParameterSpec;

class Crypto {
    void params() {
        new IvParameterSpec(new byte[] {0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0});
        new GCMParameterSpec(128, "000000000000".getBytes());
    }
}
`)
		if len(findings) != 2 {
			t.Fatalf("expected 2 Java findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Java ignores parameter, field, random-derived, and local lookalikes", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "StaticIv", `
package test;
import java.security.SecureRandom;

class IvParameterSpec {
    IvParameterSpec(byte[] bytes) {}
}

class Crypto {
    private byte[] field = new byte[16];
    void params(byte[] param) {
        byte[] random = new byte[16];
        new SecureRandom().nextBytes(random);
        new javax.crypto.spec.IvParameterSpec(param);
        new javax.crypto.spec.IvParameterSpec(field);
        new javax.crypto.spec.IvParameterSpec(random);
        new IvParameterSpec(new byte[] {0, 0, 0, 0});
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 Java findings, got %d: %v", len(findings), findings)
		}
	})
}

func TestHardcodedSecretKey(t *testing.T) {
	t.Run("Kotlin flags literal byte arrays and literal string bytes", func(t *testing.T) {
		findings := runRuleByName(t, "HardcodedSecretKey", `
package test
import android.util.Base64
import javax.crypto.spec.SecretKeySpec

class Crypto {
    fun keys() {
        SecretKeySpec(byteArrayOf(1, 2, 3, 4), "AES")
        SecretKeySpec("p@ssw0rd12345678".toByteArray(), "AES")
        SecretKeySpec("0011223344556677".hexToByteArray(), "AES")
        SecretKeySpec(Base64.decode("c2VjcmV0MTIzNDU2Nzg=", Base64.DEFAULT), "AES")
    }
}
`)
		if len(findings) != 4 {
			t.Fatalf("expected 4 findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Kotlin ignores parameter, keystore-derived, and local lookalikes", func(t *testing.T) {
		findings := runRuleByName(t, "HardcodedSecretKey", `
package test
import java.security.KeyStore
import javax.crypto.SecretKey
import javax.crypto.spec.SecretKeySpec

class SecretKeySpec(bytes: ByteArray, algorithm: String)

class Crypto(private val keyStore: KeyStore) {
    fun keys(param: ByteArray, runtime: ByteArray) {
        javax.crypto.spec.SecretKeySpec(param, "AES")
        javax.crypto.spec.SecretKeySpec(runtime, "AES")
        keyStore.getKey("alias", null) as SecretKey
        SecretKeySpec(byteArrayOf(1, 2, 3, 4), "AES")
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Java flags literal byte arrays and literal string bytes", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "HardcodedSecretKey", `
package test;
import java.util.Base64;
import javax.crypto.spec.SecretKeySpec;

class Crypto {
    void keys() {
        new SecretKeySpec(new byte[] {1, 2, 3, 4}, "AES");
        new SecretKeySpec("p@ssw0rd12345678".getBytes(), "AES");
        new SecretKeySpec(Base64.getDecoder().decode("c2VjcmV0MTIzNDU2Nzg="), "AES");
    }
}
`)
		if len(findings) != 3 {
			t.Fatalf("expected 3 Java findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Java ignores parameter, runtime-derived, and local lookalikes", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "HardcodedSecretKey", `
package test;
import java.security.KeyStore;

class SecretKeySpec {
    SecretKeySpec(byte[] bytes, String algorithm) {}
}

class Crypto {
    void keys(KeyStore keyStore, byte[] param, byte[] runtime) throws Exception {
        new javax.crypto.spec.SecretKeySpec(param, "AES");
        new javax.crypto.spec.SecretKeySpec(runtime, "AES");
        keyStore.getKey("alias", null);
        new SecretKeySpec(new byte[] {1, 2, 3, 4}, "AES");
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 Java findings, got %d: %v", len(findings), findings)
		}
	})
}

func TestHardcodedHttpUrl(t *testing.T) {
	t.Run("Kotlin flags network builders and URL constructor", func(t *testing.T) {
		findings := runRuleByName(t, "HardcodedHttpUrl", `
package test
import okhttp3.Request
import retrofit2.Retrofit
import java.net.URL

fun network() {
    Retrofit.Builder().baseUrl("http://api.example.com/").build()
    Request.Builder().url("http://cdn.example.com/file").build()
    URL("http://files.example.com/data")
}
`)
		if len(findings) != 3 {
			t.Fatalf("expected 3 findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Kotlin skips safe hosts, https, interpolation, and local lookalikes", func(t *testing.T) {
		findings := runRuleByName(t, "HardcodedHttpUrl", `
package test
import okhttp3.Request
import retrofit2.Retrofit
import java.net.URL

class URL(value: String)
class LocalBuilder {
    fun url(value: String) = this
}

fun network(host: String) {
    Retrofit.Builder().baseUrl("https://api.example.com/").build()
    Retrofit.Builder().baseUrl("http://localhost:8080/").build()
    Retrofit.Builder().baseUrl("http://10.0.2.2:8080/").build()
    Retrofit.Builder().baseUrl("http://$host/").build()
    Request.Builder().url("http://127.0.0.1:8080/file").build()
    URL("http://files.example.com/data")
    LocalBuilder().url("http://api.example.com/")
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Java flags network builders and URL constructor", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "HardcodedHttpUrl", `
package test;
import java.net.URL;
import okhttp3.Request;
import retrofit2.Retrofit;

class Network {
    void build() throws Exception {
        new Retrofit.Builder().baseUrl("http://api.example.com/").build();
        new Request.Builder().url("http://cdn.example.com/file").build();
        new URL("http://files.example.com/data");
    }
}
`)
		if len(findings) != 3 {
			t.Fatalf("expected 3 Java findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Java skips safe hosts, https, variables, and local lookalikes", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "HardcodedHttpUrl", `
package test;
import okhttp3.Request;
import retrofit2.Retrofit;

class URL {
    URL(String value) {}
}
class LocalBuilder {
    LocalBuilder url(String value) { return this; }
}
class Network {
    void build(String endpoint) throws Exception {
        new Retrofit.Builder().baseUrl("https://api.example.com/").build();
        new Retrofit.Builder().baseUrl("http://0.0.0.0:8080/").build();
        new Request.Builder().url("http://127.0.0.1:8080/file").build();
        new Request.Builder().url(endpoint).build();
        new URL("http://files.example.com/data");
        new LocalBuilder().url("http://api.example.com/");
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 Java findings, got %d: %v", len(findings), findings)
		}
	})
}

func TestStartActivityWithUntrustedIntent(t *testing.T) {
	t.Run("Kotlin flags parsed intent launch without guard", func(t *testing.T) {
		findings := runRuleByName(t, "StartActivityWithUntrustedIntent", `
package test
import android.app.Activity
import android.content.Intent

class Screen : Activity() {
    fun launch(uri: String) {
        val intent = Intent.parseUri(uri, 0)
        startActivity(intent)
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Kotlin accepts package guard and unrelated intents", func(t *testing.T) {
		findings := runRuleByName(t, "StartActivityWithUntrustedIntent", `
package test
import android.app.Activity
import android.content.ComponentName
import android.content.Intent

class Screen : Activity() {
    fun guarded(uri: String, param: Intent) {
        val intent = Intent.parseUri(uri, 0)
        intent.setPackage(packageName)
        startActivity(intent)
        val intent2 = Intent()
        startActivity(intent2)
        startActivity(param)
    }
    fun nested(uri: String) {
        fun build(): Intent = Intent.parseUri(uri, 0)
        val safe = Intent()
        startActivity(safe)
    }
    fun componentGuard(uri: String) {
        val intent = Intent.parseUri(uri, 0)
        intent.component = ComponentName(packageName, "Target")
        startActivityForResult(intent, 7)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Java flags parsed intent launch without guard", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "StartActivityWithUntrustedIntent", `
package test;
import android.app.Activity;
import android.content.Intent;

class Screen extends Activity {
    void launch(String uri) throws Exception {
        Intent intent = Intent.parseUri(uri, 0);
        startActivity(intent);
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 Java finding, got %d: %v", len(findings), findings)
		}
	})
	t.Run("Java accepts component guard and unrelated intents", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "StartActivityWithUntrustedIntent", `
package test;
import android.app.Activity;
import android.content.Intent;

class Screen extends Activity {
    void guarded(String uri, Intent param) throws Exception {
        Intent intent = Intent.parseUri(uri, 0);
        intent.setComponent(null);
        startActivity(intent);
        Intent intent2 = new Intent();
        startActivity(intent2);
        startActivity(param);
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 Java findings, got %d: %v", len(findings), findings)
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

func TestExportedService(t *testing.T) {
	t.Run("triggers on Service subclass", func(t *testing.T) {
		findings := runRuleByName(t, "ExportedService", `
package test
import android.app.Service
class MyService : Service() {
    override fun onBind(intent: Intent) = null
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("permission enforced passes", func(t *testing.T) {
		findings := runRuleByName(t, "ExportedService", `
package test
import android.app.Service
class SecureService : Service() {
    override fun onBind(intent: Intent): IBinder? {
        enforceCallingPermission("p", "no")
        return null
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("non-service class passes", func(t *testing.T) {
		findings := runRuleByName(t, "ExportedService", `
package test
class NotAService {
    fun doWork() {}
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("missing import passes", func(t *testing.T) {
		findings := runRuleByName(t, "ExportedService", `
package test
class MyService : Service() {
    override fun onBind(intent: Intent) = null
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings without import, got %d", len(findings))
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
	t.Run("triggers on Java Context receiver", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "GrantAllUris", `
package test;
import android.content.Context;

class Sharing {
  void share(Context context, android.net.Uri uri) {
    context.grantUriPermission("pkg", uri, 3);
  }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected Java GrantAllUris finding")
		}
	})
	t.Run("triggers on Java Activity receiver", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "GrantAllUris", `
package test;

class Sharing extends android.app.Activity {
  void share(android.net.Uri uri) {
    this.grantUriPermission("pkg", uri, 3);
  }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected Java Activity GrantAllUris finding")
		}
	})
	t.Run("Java local lookalike receiver is ignored", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "GrantAllUris", `
package test;

class Sharing {
  static class Helper {
    void grantUriPermission(String pkg, Object uri, int flags) {}
  }
  void share(Helper helper, Object uri) {
    helper.grantUriPermission("pkg", uri, 3);
  }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for Java local lookalike, got %d", len(findings))
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
	t.Run("triggers on Kotlin literal SecureRandom setSeed", func(t *testing.T) {
		findings := runRuleByName(t, "SecureRandom", `
package test
import java.security.SecureRandom
class TokenGen {
    fun generate() {
        val rng = SecureRandom()
        rng.setSeed(1234L)
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("triggers on Kotlin time-based SecureRandom setSeed", func(t *testing.T) {
		findings := runRuleByName(t, "SecureRandom", `
package test
import java.security.SecureRandom
class TokenGen {
    fun generate() {
        val rng = SecureRandom()
        rng.setSeed(System.currentTimeMillis())
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("triggers on Java literal SecureRandom setSeed", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "SecureRandom", `
package test;
import java.security.SecureRandom;
class TokenGen {
    void generate() {
        SecureRandom rng = new SecureRandom();
        rng.setSeed(1L);
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("triggers on Java nanoTime SecureRandom setSeed", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "SecureRandom", `
package test;
import java.security.SecureRandom;
class TokenGen {
    void generate() {
        SecureRandom rng = new SecureRandom();
        rng.setSeed(System.nanoTime());
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
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
	t.Run("does not trigger on SecureRandom byte-array seed", func(t *testing.T) {
		findings := runRuleByName(t, "SecureRandom", `
package test
import java.security.SecureRandom
class TokenGen {
    fun generate(seedBytes: ByteArray) {
        val rng = SecureRandom()
        rng.setSeed(seedBytes)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("does not trigger on kotlin.random.Random setSeed", func(t *testing.T) {
		findings := runRuleByName(t, "SecureRandom", `
package test
import kotlin.random.Random
class CustomRandom {
    fun generate() {
        val rng = Random(1234)
        rng.nextLong()
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("does not trigger on unrelated Java setSeed", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "SecureRandom", `
package test;
class TokenGen {
    static class Seeder {
        void setSeed(long seed) {}
    }
    void generate() {
        Seeder rng = new Seeder();
        rng.setSeed(1L);
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
	t.Run("triggers on empty X509TrustManager object", func(t *testing.T) {
		findings := runRuleByName(t, "TrustedServer", `
package test
import javax.net.ssl.X509TrustManager
class HttpClient {
    val tm = object : X509TrustManager {
        override fun checkClientTrusted(chain: Array<out java.security.cert.X509Certificate>?, authType: String?) {}
        override fun checkServerTrusted(chain: Array<out java.security.cert.X509Certificate>?, authType: String?) {}
        override fun getAcceptedIssuers(): Array<java.security.cert.X509Certificate> = arrayOf()
    }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected finding for empty X509TrustManager override")
		}
	})
	t.Run("X509TrustManager as generic argument is not a supertype", func(t *testing.T) {
		findings := runRuleByName(t, "TrustedServer", `
package test
import javax.net.ssl.X509TrustManager
interface Provider<T>
class GenericArgOnly : Provider<X509TrustManager> {
    fun checkClientTrusted() {}
    fun checkServerTrusted() {}
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for generic-arg-only X509TrustManager, got %d", len(findings))
		}
	})
	t.Run("qualified X509TrustManager as generic argument is not a supertype", func(t *testing.T) {
		findings := runRuleByName(t, "TrustedServer", `
package test
interface Provider<T>
class GenericArgOnly : Provider<javax.net.ssl.X509TrustManager> {
    fun checkClientTrusted() {}
    fun checkServerTrusted() {}
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for qualified generic-arg-only X509TrustManager, got %d", len(findings))
		}
	})
	t.Run("qualified X509TrustManager supertype with empty overrides still fires", func(t *testing.T) {
		findings := runRuleByName(t, "TrustedServer", `
package test
class HttpClient {
    val tm = object : javax.net.ssl.X509TrustManager {
        override fun checkClientTrusted(chain: Array<out java.security.cert.X509Certificate>?, authType: String?) {}
        override fun checkServerTrusted(chain: Array<out java.security.cert.X509Certificate>?, authType: String?) {}
        override fun getAcceptedIssuers(): Array<java.security.cert.X509Certificate> = arrayOf()
    }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected finding for qualified X509TrustManager supertype with empty overrides")
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
	t.Run("triggers on Java MODE_WORLD_READABLE", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "WorldReadableFiles", `
package test;
class Prefs {
  void open(android.content.Context context) {
    context.getSharedPreferences("prefs", android.content.Context.MODE_WORLD_READABLE);
  }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected Java MODE_WORLD_READABLE finding")
		}
	})
	t.Run("Java local lookalike declaration is ignored", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "WorldReadableFiles", `
package test;
class Prefs {
  static final int MODE_WORLD_READABLE = 0;
  void open() {
    int mode = MODE_PRIVATE;
  }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for Java declaration lookalike, got %d", len(findings))
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
	t.Run("triggers on Java MODE_WORLD_WRITEABLE", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "WorldWriteableFiles", `
package test;
class FileHelper {
  void create(android.content.Context context) throws java.io.IOException {
    context.openFileOutput("data", android.content.Context.MODE_WORLD_WRITEABLE);
  }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected Java MODE_WORLD_WRITEABLE finding")
		}
	})
	t.Run("triggers on Java MODE_WORLD_WRITABLE alias", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "WorldWriteableFiles", `
package test;
class FileHelper {
  void create(android.content.Context context) throws java.io.IOException {
    context.openFileOutput("data", android.content.Context.MODE_WORLD_WRITABLE);
  }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected Java MODE_WORLD_WRITABLE finding")
		}
	})
	t.Run("Java local lookalike declaration is ignored", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "WorldWriteableFiles", `
package test;
class FileHelper {
  static final int MODE_WORLD_WRITEABLE = 0;
  void create() {
    int mode = MODE_PRIVATE;
  }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for Java declaration lookalike, got %d", len(findings))
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
	file, err := scanner.ParseJavaFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	for _, r := range api.Registry {
		if r.ID == ruleName {
			d := rules.NewDispatcher([]*api.Rule{r}, resolver)
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
