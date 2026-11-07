package rules_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// --- ContentDescriptionRule ---

func TestContentDescription_Positive(t *testing.T) {
	findings := runRuleByName(t, "ContentDescription", `
package test
import androidx.compose.material.Icon
fun MyScreen() {
    Icon(imageVector = Icons.Default.Star)
}`)
	if len(findings) == 0 {
		t.Error("expected ContentDescription finding for Icon without contentDescription")
	}
}

func TestContentDescription_Negative(t *testing.T) {
	findings := runRuleByName(t, "ContentDescription", `
package test
import androidx.compose.material.Icon
fun MyScreen() {
    Icon(imageVector = Icons.Default.Star, contentDescription = "Star")
}`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestContentDescription_IgnoreSimilarNames(t *testing.T) {
	findings := runRuleByName(t, "ContentDescription", `
package test
fun MyScreen() {
    MyIcon(imageVector = Icons.Default.Star)
}`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-Image/Icon call, got %d", len(findings))
	}
}

// --- HardcodedTextRule ---

func TestHardcodedText_Positive(t *testing.T) {
	findings := runRuleByName(t, "HardcodedText", `
package test
import androidx.compose.material.Text
fun MyScreen() {
    Text(text = "Hello World")
}`)
	if len(findings) == 0 {
		t.Error("expected HardcodedText finding for hardcoded text")
	}
}

func TestHardcodedText_Negative(t *testing.T) {
	findings := runRuleByName(t, "HardcodedText", `
package test
import androidx.compose.material.Text
fun MyScreen() {
    Text(text = stringResource(R.string.hello))
}`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestHardcodedText_Negative_GetString(t *testing.T) {
	findings := runRuleByName(t, "HardcodedText", `
package test
import androidx.compose.material.Text
fun MyScreen() {
    Text(title = getString(R.string.hello))
}`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for getString usage, got %d", len(findings))
	}
}

// Data-class constructor with the same `text =` named-argument shape.
// Must not fire — the callee is not a known Compose composable.
func TestHardcodedText_Negative_DataClassConstructor(t *testing.T) {
	findings := runRuleByName(t, "HardcodedText", `
package test
import androidx.compose.material.Text
data class LogEntry(val text: String, val description: String)
fun build() = LogEntry(text = "raw log line", description = "raw description")`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for data-class constructor, got %d", len(findings))
	}
}

// Pure interpolation — no static literal text to localize.
func TestHardcodedText_Negative_PureInterpolation(t *testing.T) {
	findings := runRuleByName(t, "HardcodedText", `
package test
import androidx.compose.material.Text
fun MyScreen(name: String) {
    Text(text = "$name")
    Text(text = "${name.uppercase()}")
    Text(text = "$name$name")
}`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for pure-interpolation templates, got %d", len(findings))
	}
}

// Without any Android/Compose import the rule must stay silent — the
// callee name alone is too ambiguous (could be a domain class).
func TestHardcodedText_Negative_NoAndroidImport(t *testing.T) {
	findings := runRuleByName(t, "HardcodedText", `
package test
fun render() {
    Text(text = "raw label")
}`)
	if len(findings) != 0 {
		t.Errorf("expected no findings when no Android/Compose import is present, got %d", len(findings))
	}
}

// String concatenation that still includes a resource lookup must
// not fire — the Contains check already covers `getString(`.
func TestHardcodedText_Negative_ConcatWithGetString(t *testing.T) {
	findings := runRuleByName(t, "HardcodedText", `
package test
import androidx.compose.material.Text
fun MyScreen() {
    Text(text = "Hello " + getString(R.string.world))
}`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for concat including getString, got %d", len(findings))
	}
}

// --- LogDetectorRule (LogConditional) ---

func TestLogConditional_Positive(t *testing.T) {
	findings := runRuleByName(t, "LogConditional", `
package test
import android.util.Log
fun doWork() {
    Log.d("TAG", "doing work")
}`)
	if len(findings) == 0 {
		t.Error("expected LogConditional finding for unconditional Log call")
	}
}

func TestLogConditional_Negative(t *testing.T) {
	findings := runRuleByName(t, "LogConditional", `
package test
import android.util.Log
fun doWork() {
    if (Log.isLoggable("TAG", Log.DEBUG)) {
        Log.d("TAG", "doing work")
    }
}`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestLogConditional_NegativeBuildConfigDebugGuard(t *testing.T) {
	findings := runRuleByName(t, "LogConditional", `
package test
import android.util.Log
fun doWork() {
    if (BuildConfig.DEBUG) {
        Log.d("TAG", "doing work")
    }
}`)
	if len(findings) != 0 {
		t.Errorf("expected no findings inside BuildConfig.DEBUG guard, got %d", len(findings))
	}
}

func TestLogConditional_NegativeTimberLog(t *testing.T) {
	// timber.log.Log lookalike (same simple name "Log", different FQN);
	// must not be misclassified as android.util.Log.
	findings := runRuleByName(t, "LogConditional", `
package test
import timber.log.Log
fun doWork() {
    Log.d("TAG", "timber call")
}`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for timber.log.Log, got %d", len(findings))
	}
}

func TestLogConditional_NegativeSlf4jLogger(t *testing.T) {
	findings := runRuleByName(t, "LogConditional", `
package test
import org.slf4j.LoggerFactory
private val log = LoggerFactory.getLogger("X")
fun doWork() {
    log.debug("hello")
}`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for slf4j logger, got %d", len(findings))
	}
}

func TestLogConditional_NegativeKotlinLoggingKLogger(t *testing.T) {
	findings := runRuleByName(t, "LogConditional", `
package test
import mu.KotlinLogging
private val logger = KotlinLogging.logger {}
fun doWork() {
    logger.debug { "hello" }
}`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for kotlin-logging KLogger, got %d", len(findings))
	}
}

func TestLogConditional_NegativeLocalLogClassLookalike(t *testing.T) {
	// A same-file `class Log` lookalike must not be misclassified as
	// android.util.Log even when the receiver name matches.
	findings := runRuleByName(t, "LogConditional", `
package test
class Log {
    fun d(tag: String, msg: String) {}
}
fun doWork() {
    val Log = Log()
    Log.d("TAG", "lookalike")
}`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for local Log lookalike, got %d", len(findings))
	}
}

func TestLogConditional_NegativeTestSource(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "src", "test", "java", "com", "example", "MyTest.kt")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	code := `package test
import android.util.Log
class MyTest {
    fun runs() {
        Log.d("TAG", "from test")
    }
}`
	if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	findings := runRuleByNameOnFile(t, "LogConditional", file)
	if len(findings) != 0 {
		t.Errorf("expected no findings in test source path, got %d", len(findings))
	}
}

// --- SdCardPathRule ---

func TestSdCardPath_IgnoresPathsInsideProseAndUrls(t *testing.T) {
	findings := runRuleByName(t, "SdCardPath", `
package test
val a = "see /sdcard/ in docs"
val b = "https://example.com/data/data/export"
val c = "file:///sdcard/Download"
`)
	if len(findings) != 1 {
		t.Fatalf("expected only the file:// path to be reported, got %d", len(findings))
	}
}

func TestSdCardPath_Positive(t *testing.T) {
	findings := runRuleByName(t, "SdCardPath", `
package test
fun save() {
    val path = "/sdcard/myapp/data.txt"
}`)
	if len(findings) == 0 {
		t.Error("expected SdCardPath finding for hardcoded /sdcard path")
	}
}

func TestSdCardPath_Negative(t *testing.T) {
	findings := runRuleByName(t, "SdCardPath", `
package test
fun save() {
    val path = Environment.getExternalStorageDirectory().absolutePath
}`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestSdCardPath_PositiveDataDataPath(t *testing.T) {
	findings := runRuleByName(t, "SdCardPath", `
package test
fun save() {
    val path = "/data/data/com.example/files/cache"
}`)
	if len(findings) == 0 {
		t.Fatal("expected SdCardPath finding for hardcoded /data/data/ path")
	}
	if !strings.Contains(findings[0].Message, "/data/") {
		t.Errorf("expected message to call out /data/ path, got %q", findings[0].Message)
	}
}

func TestSdCardPath_PositiveDataUserPath(t *testing.T) {
	findings := runRuleByName(t, "SdCardPath", `
package test
fun save() {
    val path = "/data/user/0/com.example/files"
}`)
	if len(findings) == 0 {
		t.Fatal("expected SdCardPath finding for hardcoded /data/user/ path")
	}
}

func TestSdCardPath_NegativeBenignDataPath(t *testing.T) {
	findings := runRuleByName(t, "SdCardPath", `
package test
fun describe(): String {
    return "size in /data partition"
}`)
	if len(findings) != 0 {
		t.Errorf("expected no SdCardPath findings for benign /data string, got %d", len(findings))
	}
}

// --- WakelockRule ---

func TestWakelock_Positive(t *testing.T) {
	findings := runRuleByName(t, "Wakelock", `
package test
fun doWork() {
    val wl = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "tag")
    wl.acquire()
}`)
	if len(findings) == 0 {
		t.Error("expected Wakelock finding for acquire without release")
	}
}

func TestWakelock_Negative_AcquireWithTimeout(t *testing.T) {
	findings := runRuleByName(t, "Wakelock", `
package test
fun doWork() {
    val wl = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "tag")
    wl.acquire(10_000L)
}`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for acquire(timeout), got %d", len(findings))
	}
}

func TestWakelock_Negative(t *testing.T) {
	findings := runRuleByName(t, "Wakelock", `
package test
fun doWork() {
    val wl = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "tag")
    wl.acquire()
    wl.release()
}`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestWakelock_Positive_UnrelatedRelease(t *testing.T) {
	findings := runRuleByName(t, "Wakelock", `
package test
fun doWork() {
    val wl = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "tag")
    val other = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "other")
    wl.acquire()
    other.release()
}`)
	if len(findings) == 0 {
		t.Error("expected Wakelock finding when only an unrelated release exists")
	}
}

// --- SetJavaScriptEnabledRule ---

func TestSetJavaScriptEnabled_Positive(t *testing.T) {
	findings := runRuleByName(t, "SetJavaScriptEnabled", `
package test
fun setup(webView: WebView) {
    webView.settings.setJavaScriptEnabled(true)
}`)
	if len(findings) == 0 {
		t.Error("expected SetJavaScriptEnabled finding")
	}
}

func TestSetJavaScriptEnabled_AssignmentPositive(t *testing.T) {
	findings := runRuleByName(t, "SetJavaScriptEnabled", `
package test
fun setup(webView: WebView) {
    webView.settings.javaScriptEnabled = true
}`)
	if len(findings) == 0 {
		t.Error("expected SetJavaScriptEnabled finding for property assignment")
	}
}

func TestSetJavaScriptEnabled_Negative(t *testing.T) {
	findings := runRuleByName(t, "SetJavaScriptEnabled", `
package test
fun setup(webView: WebView) {
    webView.settings.setJavaScriptEnabled(false)
}`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestSetJavaScriptEnabled_Java(t *testing.T) {
	t.Run("positive WebView settings call", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "SetJavaScriptEnabled", `
package test;
import android.webkit.WebView;
class Browser {
  void setup(WebView webView) {
    webView.getSettings().setJavaScriptEnabled(true);
  }
}`)
		if len(findings) == 0 {
			t.Fatal("expected Java SetJavaScriptEnabled finding")
		}
	})
	t.Run("unresolved semantic facts fall back to imported type", func(t *testing.T) {
		findings := runRuleByNameOnJavaWithSemanticCalls(t, "SetJavaScriptEnabled", `
package test;
import android.webkit.WebView;
class Browser {
  void setup(WebView webView) {
    webView.getSettings().setJavaScriptEnabled(true);
  }
}`, javaSemanticCallSpec{Callee: "setJavaScriptEnabled", ReceiverType: "", ReturnType: "void"})
		if len(findings) == 0 {
			t.Fatal("expected Java SetJavaScriptEnabled finding")
		}
	})
	t.Run("negative false argument", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "SetJavaScriptEnabled", `
package test;
import android.webkit.WebView;
class Browser {
  void setup(WebView webView) {
    webView.getSettings().setJavaScriptEnabled(false);
  }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("negative local lookalike", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "SetJavaScriptEnabled", `
package test;
class Settings {
  void setJavaScriptEnabled(boolean enabled) {}
}
class Browser {
  void setup(Settings settings) {
    settings.setJavaScriptEnabled(true);
  }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for local lookalike, got %d", len(findings))
		}
	})
	t.Run("semantic facts suppress imported local lookalike", func(t *testing.T) {
		findings := runRuleByNameOnJavaWithSemanticCalls(t, "SetJavaScriptEnabled", `
package test;
import android.webkit.WebSettings;
class FakeSettings {
  void setJavaScriptEnabled(boolean enabled) {}
}
class Browser {
  void setup(FakeSettings webSettings) {
    webSettings.setJavaScriptEnabled(true);
  }
}`, javaSemanticCallSpec{Callee: "setJavaScriptEnabled", ReceiverType: "test.FakeSettings", ReturnType: "void"})
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for javac-confirmed local lookalike, got %d", len(findings))
		}
	})
	t.Run("semantic facts suppress same simple-name lookalike", func(t *testing.T) {
		findings := runRuleByNameOnJavaWithSemanticCalls(t, "SetJavaScriptEnabled", `
package test;
import android.webkit.WebSettings;
class WebSettings {
  void setJavaScriptEnabled(boolean enabled) {}
}
class Browser {
  void setup(WebSettings webSettings) {
    webSettings.setJavaScriptEnabled(true);
  }
}`, javaSemanticCallSpec{Callee: "setJavaScriptEnabled", ReceiverType: "test.WebSettings", ReturnType: "void"})
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for same simple-name javac-confirmed lookalike, got %d", len(findings))
		}
	})
}

// --- PrivateKeyRule (PackagedPrivateKey) ---

func TestPackagedPrivateKey_Positive(t *testing.T) {
	findings := runRuleByName(t, "PackagedPrivateKey", `
package test
val key = "-----BEGIN RSA PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQC7VJTUt9Us8cKjMzEfYyjiWA4R4/M2bS1GB4t7NXp98C3SC6dVMvDuictGeurT8jNbvJZHtCSuYEvuNMoSfm76oqFvAp8Gy0iz5sxjZmSnXyCdPEovGhLa0VzMaQ8s"
`)
	if len(findings) == 0 {
		t.Error("expected PackagedPrivateKey finding for embedded private key")
	}
}

func TestPackagedPrivateKey_Negative(t *testing.T) {
	findings := runRuleByName(t, "PackagedPrivateKey", `
package test
val key = KeyStore.getInstance("AndroidKeyStore")
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// TestPackagedPrivateKey_IgnoresKeyMarkerInComments confirms that the
// rule no longer reports private-key markers that appear only inside
// line comments, block comments, or KDoc — those are documentation, not
// embedded secrets. Each case fails on the pre-fix `strings.Contains`
// implementation.
func TestPackagedPrivateKey_IgnoresKeyMarkerInComments(t *testing.T) {
	for _, tc := range []struct {
		name string
		code string
	}{
		{
			name: "line comment mentions key marker",
			code: `
package test

// Example header format: -----BEGIN RSA PRIVATE KEY-----
val keystoreAlias = "AndroidKeyStore"
`,
		},
		{
			name: "single-line block comment mentions key marker",
			code: `
package test

/* The PEM payload starts with -----BEGIN PRIVATE KEY----- */
val keystoreAlias = "AndroidKeyStore"
`,
		},
		{
			name: "multi-line block comment mentions key marker",
			code: `
package test

/*
 * Documentation:
 *   -----BEGIN EC PRIVATE KEY-----
 *   <base64 here>
 *   -----END EC PRIVATE KEY-----
 */
val keystoreAlias = "AndroidKeyStore"
`,
		},
		{
			name: "kdoc mentions key marker",
			code: `
package test

/**
 * Loads a key whose PEM header is -----BEGIN RSA PRIVATE KEY-----.
 */
fun load(): String = "AndroidKeyStore"
`,
		},
		{
			name: "trailing line comment after code mentions key marker",
			code: `
package test

val keystoreAlias = "AndroidKeyStore" // headers like -----BEGIN PRIVATE KEY----- are PEM
`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			findings := runRuleByName(t, "PackagedPrivateKey", tc.code)
			if len(findings) != 0 {
				t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
			}
		})
	}
}

// TestPackagedPrivateKey_FlagsKeyInRawString locks in that triple-quoted
// raw strings — a natural place to paste a real PEM block — still
// trigger the rule on every line that contains a header marker.
func TestPackagedPrivateKey_FlagsKeyInRawString(t *testing.T) {
	findings := runRuleByName(t, "PackagedPrivateKey", "\n"+`package test

val key = """
    -----BEGIN RSA PRIVATE KEY-----
    MIIEowIBAAKCAQEAx3GJHxQ8xvVx9c2nGv5N0m0C5o8d4jQdYQd1XkOQ1j6y5Y9wq7r1l2B8Zk0o3pVbHhNn7sM
    -----END RSA PRIVATE KEY-----
""".trimIndent()
`)
	if len(findings) == 0 {
		t.Fatal("expected at least 1 finding for raw-string PEM block")
	}
}

// --- ObsoleteLayoutParamsRule (ObsoleteLayoutParam) ---

func TestObsoleteLayoutParam_Positive(t *testing.T) {
	findings := runRuleByName(t, "ObsoleteLayoutParam", `
package test
fun MyScreen() {
    Box(modifier = Modifier.preferredWidth(100.dp))
}`)
	if len(findings) == 0 {
		t.Error("expected ObsoleteLayoutParam finding for preferredWidth")
	}
}

func TestObsoleteLayoutParam_Negative(t *testing.T) {
	findings := runRuleByName(t, "ObsoleteLayoutParam", `
package test
fun MyScreen() {
    Box(modifier = Modifier.width(100.dp))
}`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// --- ViewHolderRule ---

func TestViewHolder_Positive(t *testing.T) {
	findings := runRuleByName(t, "ViewHolder", `
package test
class MyAdapter : BaseAdapter() {
    override fun getView(position: Int, convertView: View?, parent: ViewGroup): View {
        val v = LayoutInflater.from(parent.context).inflate(R.layout.row, parent, false)
        return v
    }
}`)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding for unconditional inflate in getView, got %d", len(findings))
	}
}

func TestViewHolder_Negative(t *testing.T) {
	cases := map[string]string{
		"elvis_on_convert_view": `val v = convertView ?: LayoutInflater.from(parent.context).inflate(R.layout.row, parent, false)
        return v`,
		"if_null_check": `var v = convertView
        if (v == null) {
            v = LayoutInflater.from(parent.context).inflate(R.layout.row, parent, false)
        }
        return v!!`,
		"no_inflate": `return convertView ?: parent`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			findings := runRuleByName(t, "ViewHolder", "package test\nclass MyAdapter : BaseAdapter() {\n    override fun getView(position: Int, convertView: View?, parent: ViewGroup): View {\n        "+body+"\n    }\n}")
			if len(findings) != 0 {
				t.Errorf("expected no findings, got %d", len(findings))
			}
		})
	}
}

func TestViewHolder_FollowsLintDetector(t *testing.T) {
	// Verified against Lint 32.4.1: the early-return guard and a getView outside an adapter are both reported.
	findings := runRuleByName(t, "ViewHolder", `
package test
class A : RecyclerView.Adapter<VH>() {
    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): VH = VH(inflater.inflate(R.layout.row, parent, false))
}
class Early : BaseAdapter() {
    override fun getView(position: Int, convertView: View?, parent: ViewGroup): View {
        if (convertView != null) return convertView
        return inflater.inflate(R.layout.row, parent, false)
    }
}
class Helper {
    fun getView(position: Int, convertView: View?, parent: ViewGroup): View = inflater.inflate(R.layout.row, parent, false)
    fun getView(position: Int, parent: ViewGroup): View = inflater.inflate(R.layout.row, parent, false)
}`)
	if len(findings) != 2 {
		t.Errorf("expected 2 findings, got %d: %+v", len(findings), findings)
	}
}

func TestContentDescription_PositionalArgumentsAndLookalikes(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want int
	}{
		{"positional description", "package test\nimport androidx.compose.material.Icon\nfun A() { Icon(Icons.Default.Add, \"Add\") }\n", 0},
		{"positional null is explicit", "package test\nimport androidx.compose.material3.Icon\nfun A() { Icon(Icons.Default.Add, null) }\n", 0},
		{"Image positional description", "package test\nimport androidx.compose.foundation.Image\nfun A() { Image(painter, \"desc\") }\n", 0},
		{"Image missing description", "package test\nimport androidx.compose.foundation.Image\nfun A() { Image(modifier = m, painter = p) }\n", 1},
		{"project Icon without Compose import", "package test\nfun A() { Icon(Icons.Default.Add) }\n", 0},
		{"member call", "package test\nimport androidx.compose.foundation.Image\nfun A(o: Other) { o.Image(3) }\n", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := len(runRuleByName(t, "ContentDescription", tc.src)); got != tc.want {
				t.Fatalf("expected %d findings, got %d", tc.want, got)
			}
		})
	}
}

func TestHardcodedText_PreviewsAndNonTextLiterals(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"real text", "@Composable fun A() { Text(text = \"Real text\") }", 1},
		{"preview function", "@Preview @Composable fun A() { Text(text = \"Sample\") }", 0},
		{"custom preview annotation", "@PreviewLightDark @Composable fun A() { Text(text = \"Sample\") }", 0},
		{"empty literal", "@Composable fun A() { Text(text = \"\") }", 0},
		{"whitespace literal", "@Composable fun A() { Text(text = \" \") }", 0},
		{"punctuation literal", "@Composable fun A() { Text(text = \" : \") }", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := "package test\nimport androidx.compose.material.Text\nimport androidx.compose.runtime.Composable\n" + tc.body + "\n"
			if got := len(runRuleByName(t, "HardcodedText", src)); got != tc.want {
				t.Fatalf("expected %d findings, got %d", tc.want, got)
			}
		})
	}
}

func TestPackagedPrivateKey_IgnoresHeaderWithoutBodyAndProse(t *testing.T) {
	findings := runRuleByName(t, "PackagedPrivateKey", `
package test
val header = "-----BEGIN PRIVATE KEY-----"
val footer = "-----END PRIVATE KEY-----"
val doc = "regex for BEGIN PRIVATE KEY detection"
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for a PEM header without a body, got %d", len(findings))
	}
}
