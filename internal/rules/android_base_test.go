package rules_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
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
    Log.d("TAG", "doing work " + System.nanoTime())
}`)
	if len(findings) == 0 {
		t.Error("expected LogConditional finding for unconditional Log call")
	}
	for _, finding := range findings {
		want := "The log call Log.d(...) should be conditional: surround with `if (Log.isLoggable(...))` or `if (BuildConfig.DEBUG) { ... }`"
		if finding.Message != want {
			t.Errorf("expected Lint message %q, got %q", want, finding.Message)
		}
	}
}

func TestLogConditional_Negative(t *testing.T) {
	findings := runRuleByName(t, "LogConditional", `
package test
import android.util.Log
fun doWork() {
    if (Log.isLoggable("TAG", Log.DEBUG)) {
        Log.d("TAG", "doing work " + System.nanoTime())
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
        Log.d("TAG", "doing work " + System.nanoTime())
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
        Log.d("TAG", "from test " + System.nanoTime())
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

func TestLogConditionalReceiverShadowing(t *testing.T) {
	source := `package example
import android.util.Log
class FakeLog { fun d(tag: String, message: String) {} }
fun compute(): String = "computed"
fun local() { val Log = FakeLog(); Log.d("T", compute()) }
fun parameter(Log: FakeLog) { Log.d("T", compute()) }
fun lambda(loggers: List<FakeLog>) { loggers.forEach { Log -> Log.d("T", compute()) } }
fun real() { Log.d("T", compute()) }
`
	if findings := runRuleByName(t, "LogConditional", source); len(findings) != 1 {
		t.Errorf("without resolver: expected only the real call, got %v", findings)
	}
	if findings := runRuleByNameWithResolver(t, "LogConditional", source); len(findings) != 1 {
		t.Errorf("with resolver: expected only the real call, got %v", findings)
	}
}

func TestLogConditionalCallWorkAndGuards(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"literal", `Log.d("T", "literal")`, 0},
		{"reference", `Log.d("T", value)`, 0},
		{"call", `Log.d("T", compute())`, 1},
		{"concat reference", `Log.d("T", "x" + value)`, 0},
		{"template reference", `Log.d("T", "x$value")`, 0},
		{"concat call right", `Log.d("T", "x" + compute())`, 1},
		{"concat call left", `Log.d("T", compute() + "x")`, 1},
		{"template call", `Log.d("T", "x${compute()}")`, 1},
		{"raw template call", `Log.d("T", """x${value.toString()}""")`, 1},
		{"nested concat call", `Log.d("T", "x" + (value + compute()))`, 1},
		{"receiver call property", `Log.d("T", compute().length)`, 0},
		{"constant concat", `Log.d("T", "a" + "b")`, 0},
		{"constant template", `Log.d("T", "x$CONSTANT")`, 0},
		{"constant interpolation", `Log.d("T", "${1}")`, 0},
		{"constant arithmetic interpolation", `Log.d("T", "${2 * 3}")`, 0},
		{"constant comparison interpolation", `Log.d("T", "${1 > 0}")`, 0},
		{"constant path", `Log.d("T", "x${Constants.NAME}")`, 0},
		{"local literal concat", `val local = "literal"; Log.d("T", local + "x")`, 0},
		{"local literal template", `val local = "literal"; Log.d("T", "x$local")`, 0},
		{"local computed reference", `val local = compute(); Log.d("T", "x$local")`, 0},
		{"top val template", `Log.d("T", "x$topValue")`, 0},
		{"qualified property", `Log.d("T", value.length)`, 0},
		{"Java field", `Log.d("T", java.io.File.separator)`, 0},
		{"Java field import", `Log.d("T", File.separator)`, 0},
		{"imported constant template", `Log.d("T", "x${Integer.MAX_VALUE}")`, 0},
		{"parenthesized reference", `Log.d("T", (value))`, 0},
		{"parenthesized call", `Log.d("T", (compute()))`, 1},
		{"elvis reference", `Log.d("T", value ?: "fallback")`, 0},
		{"elvis left call", `Log.d("T", compute() ?: "fallback")`, 1},
		{"elvis parenthesized left call", `Log.d("T", ((compute())) ?: "fallback")`, 1},
		{"parenthesized elvis left call", `Log.d("T", ((compute() ?: "fallback")))`, 1},
		{"elvis left navigation call", `Log.d("T", value?.toString() ?: "fallback")`, 1},
		{"elvis call", `Log.d("T", value ?: compute())`, 0},
		{"elvis left property after call", `Log.d("T", packageManager.getPackageInfo(packageName, 0).versionName ?: "fallback")`, 0},
		{"not null reference", `Log.d("T", value!!)`, 0},
		{"not null call", `Log.d("T", compute()!!)`, 0},
		{"if message references", `Log.d("T", if (flag) value else "fallback")`, 0},
		{"if message call", `Log.d("T", if (flag) value else compute())`, 0},
		{"property after call receiver", `Log.d("T", packageManager.getPackageInfo(packageName, 0).versionName)`, 0},
		{"property after chained call receiver", `Log.d("T", context.packageManager.getApplicationInfo(pkg, 0).packageName)`, 0},
		{"template property after call receiver", `Log.d("T", "${packageManager.getPackageInfo(packageName, 0).versionName}")`, 0},
		{"tag call only", `Log.d(compute(), value)`, 0},
		{"throwable call only", `Log.d("T", value, error())`, 0},
		{"call text literal", `Log.d("T", "compute()")`, 0},
		{"named arguments", `Log.d(tag = "T", msg = compute())`, 1},
		{"missing message", `Log.d("T")`, 0},
		{"verbose", `Log.v("T", compute())`, 1},
		{"info", `Log.i("T", compute())`, 1},
		{"warn", `Log.w("T", compute())`, 0},
		{"error", `Log.e("T", compute())`, 0},
		{"wtf", `Log.wtf("T", compute())`, 0},
		{"println", `Log.println(Log.INFO, "T", compute())`, 1},
		{"println tag work", `Log.println(Log.INFO, compute(), "literal")`, 0},
		{"any if", `if (flag) Log.d("T", compute())`, 0},
		{"false if", `if (false) Log.d("T", compute())`, 0},
		{"true if", `if (true) Log.d("T", compute())`, 1},
		{"nearest true if", `if (flag) { if (true) Log.d("T", compute()) }`, 1},
		{"else if", `if (true) {} else if (flag) {} else { Log.d("T", compute()) }`, 0},
		{"when case", `when (number) { 1 -> Log.d("T", compute()) }`, 0},
		{"when else", `when (number) { else -> Log.d("T", compute()) }`, 1},
		{"when true", `when { true -> Log.d("T", compute()) }`, 1},
		{"when parenthesized true", `when { (true) -> Log.d("T", compute()) }`, 1},
		{"when false", `when { false -> Log.d("T", compute()) }`, 0},
		{"when multiple", `when (flag) { true, false -> Log.d("T", compute()) }`, 0},
		{"when subject", `when (Log.d("T", compute())) { else -> 0 }`, 1},
		{"when condition", `when { Log.d("T", compute()) > 0 -> 0; else -> 0 }`, 0},
		{"call lambda", `if (flag) { run { Log.d("T", compute()) } }`, 1},
		{"call argument", `if (flag) { println(Log.d("T", compute())) }`, 1},
		{"guarded receiver", `if (flag) Log.d("T", compute()).also { println(it) }`, 0},
		{"unguarded receiver", `Log.d("T", compute()).also { println(it) }`, 1},
		{"receiver in argument", `if (flag) println(Log.d("T", compute()).also { println(it) })`, 1},
		{"unknown camelCase constant", `Log.d("T", "x${Config.timeoutMs}"); Log.d("T", "x" + timeoutMs)`, 0},
		{"guard in lambda", `run { if (flag) Log.d("T", compute()) }`, 0},
		{"standalone lambda", `if (flag) { val callback = { Log.d("T", compute()) } }`, 0},
		{"standalone anonymous function", `if (flag) { val callback = fun() { Log.d("T", compute()) } }`, 0},
		{"local function", `if (flag) { fun local() { Log.d("T", compute()) } }`, 0},
		{"local class", `if (flag) { class Local { val field = Log.d("T", compute()) } }`, 1},
		{"anonymous object", `if (flag) { val holder = object { val field = Log.d("T", compute()) } }`, 1},
		{"while", `while (flag) { Log.d("T", compute()) }`, 1},
		{"try", `try { Log.d("T", compute()) } catch (error: Exception) { Log.d("T", compute()) }`, 2},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			source := `package example
import android.util.Log
import java.io.File
const val CONSTANT = "constant"
val topValue = "value"
object Constants { const val NAME = "name" }
fun compute(): String = "computed"
fun work(value: String, flag: Boolean, number: Int) {
` + test.body + "\n}"
			if findings := runRuleByNameWithResolver(t, "LogConditional", source); len(findings) != test.want {
				t.Errorf("expected %d findings, got %d: %v", test.want, len(findings), findings)
			}
		})
	}
}

func TestLogConditionalInitializerAndConstructorLocals(t *testing.T) {
	source := `import android.util.Log
fun compute(): String = "computed"
class Screen(flag: Boolean) {
    init {
        if (flag) {
            val result = Log.d("T", compute())
            val callback = { Log.d("T", compute()) }
            fun trace() { Log.d("T", compute()) }
        }
    }
    constructor(number: Int) : this(true) {
        if (number > 0) {
            val result = Log.d("T", compute())
            val callback = { Log.d("T", compute()) }
            fun trace() { Log.d("T", compute()) }
        }
    }
}`
	if findings := runRuleByNameWithResolver(t, "LogConditional", source); len(findings) != 0 {
		t.Fatalf("expected guarded locals to be silent, got %v", findings)
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

// A release in another method only counts when it names the same field, not a same-named local or parameter.
func TestWakelock_CrossMethodReleaseMatchesTheField(t *testing.T) {
	findings := runRuleWithSourceResolver(t, "Wakelock", `
package test
import android.os.PowerManager
import android.os.PowerManager.WakeLock
class LocalShadow(private val pm: PowerManager) {
    private val wl: WakeLock = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "tag")
    fun start() { wl.acquire() }
    fun cleanup() {
        val wl = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "other")
        wl.release()
    }
}
class ParamShadow(private val pm: PowerManager) {
    private val wl: WakeLock = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "tag")
    fun start() { wl.acquire() }
    fun cleanup(wl: WakeLock) { wl.release() }
}
class FieldRelease(private val pm: PowerManager) {
    private val wl: WakeLock = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "tag")
    fun start() { wl.acquire() }
    fun stop() { wl.release() }
}
class LambdaRelease(private val pm: PowerManager) {
    private val wl: WakeLock = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "tag")
    fun start() { wl.acquire() }
    fun stop() { listOf(1).forEach { wl.release() } }
}
`)
	var lines []int
	for _, f := range findings {
		lines = append(lines, f.Line)
	}
	if want := []int{7, 15}; !reflect.DeepEqual(lines, want) {
		t.Fatalf("lines = %v, want %v", lines, want)
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

// Lint reports no lock for lack of a release in the acquiring function: the
// usual shape acquires in one lifecycle method and releases in another.
func TestWakelock_Negative_ReleasedInAnotherFunction(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "Wakelock", `
package test
import android.os.PowerManager
import android.os.PowerManager.WakeLock
class Holder(private val pm: PowerManager) {
    private var wl: WakeLock? = null
    fun onResume() {
        wl = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "a:b")
        wl?.acquire()
    }
    fun onPause() {
        wl?.release()
    }
}`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for a release in another function, got %d", len(findings))
	}
}

func TestWakelock_Positive_PropertyNeverReleased(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "Wakelock", `
package test
import android.os.PowerManager.WakeLock
class Holder {
    private var wl: WakeLock? = null
    private var other: WakeLock? = null
    fun onResume() {
        wl?.acquire()
    }
    fun onPause() {
        other?.release()
    }
}`)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding for a property released nowhere, got %d", len(findings))
	}
}

// A lock passed in belongs to the caller, which releases it.
func TestWakelock_Negative_ParameterReceiver(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "Wakelock", `
package test
import android.os.PowerManager.WakeLock
fun hold(lock: WakeLock) {
    lock.acquire()
}`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for a parameter receiver, got %d", len(findings))
	}
}

// Lint reports a release in an Activity's onDestroy: it should be in onPause.
func TestWakelock_ReleaseInActivityOnDestroy(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "Wakelock", `
package test
import android.app.Activity
import android.app.Service
import android.os.PowerManager.WakeLock
class Player : Activity() {
    private var wl: WakeLock? = null
    override fun onDestroy() {
        super.onDestroy()
        wl?.release()
    }
}
class Worker : Service() {
    private var wl: WakeLock? = null
    override fun onDestroy() {
        wl?.release()
    }
}`)
	if len(findings) != 1 || findings[0].Line != 10 ||
		findings[0].Message != "Wakelocks should be released in `onPause`, not `onDestroy`" {
		t.Fatalf("expected one onDestroy finding on line 10, got %v", findings)
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

// Lint's ConstantEvaluator folds the value, so a constant or `!false` enables JavaScript as the literal does.
func TestSetJavaScriptEnabled_ConstantValue(t *testing.T) {
	findings := runRuleByName(t, "SetJavaScriptEnabled", `
package test
import android.webkit.WebView
const val ENABLE_SCRIPTS = true
const val DISABLE_SCRIPTS = false
fun setup(webView: WebView, flag: Boolean) {
    webView.settings.setJavaScriptEnabled(ENABLE_SCRIPTS)
    webView.settings.javaScriptEnabled = ENABLE_SCRIPTS
    webView.settings.javaScriptEnabled = !false
    webView.settings.javaScriptEnabled = DISABLE_SCRIPTS
    webView.settings.javaScriptEnabled = flag
}`)
	var lines []int
	for _, f := range findings {
		lines = append(lines, f.Line)
	}
	sort.Ints(lines)
	if fmt.Sprint(lines) != "[7 8 9]" {
		t.Fatalf("expected findings on lines 7, 8 and 9, got %v", lines)
	}
}

// Without types, a `settings` receiver (explicit, or implicit in a scope function on one) in a file that imports the
// WebView API is the WebSettings, as on the Java path.
func TestSetJavaScriptEnabled_NamedSettingsReceiver(t *testing.T) {
	findings := runRuleByName(t, "SetJavaScriptEnabled", `
package test
import android.webkit.WebView
class Holder {
    private var webView: WebView? = null
    fun setup(context: android.content.Context) {
        webView?.let { view -> view.settings.javaScriptEnabled = true }
        WebView(context).apply { settings.javaScriptEnabled = true }
        WebView(context).apply { settings.setJavaScriptEnabled(true) }
        WebView(context).apply { settings.javaScriptEnabled = false }
        webView?.settings?.apply { javaScriptEnabled = true }
        webView?.let { with(it.settings) { setJavaScriptEnabled(true) } }
    }
}`)
	var lines []int
	for _, f := range findings {
		lines = append(lines, f.Line)
	}
	sort.Ints(lines)
	if fmt.Sprint(lines) != "[7 8 9 11 12]" {
		t.Fatalf("expected findings on lines 7, 8, 9, 11 and 12, got %v", lines)
	}
	unrelated := runRuleByName(t, "SetJavaScriptEnabled", `
package test
class Options { var javaScriptEnabled = false }
class Holder(val settings: Options) {
    fun setup() { settings.javaScriptEnabled = true }
}`)
	if len(unrelated) != 0 {
		t.Fatalf("expected no finding without the WebView API, got %d", len(unrelated))
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

// Lint does not analyze test sources: there acquire() is usually a mock
// verification, and onDraw a test calling the callback on purpose.
func TestWakelockAndWrongCall_SkipTestSources(t *testing.T) {
	wakelock := `
package test
fun doWork() {
    val wl = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "tag")
    wl.acquire()
}`
	if got := runRuleByNameOnPath(t, "Wakelock", "src/main/kotlin/Work.kt", wakelock); len(got) != 1 {
		t.Fatalf("expected 1 Wakelock finding in main sources, got %d", len(got))
	}
	if got := runRuleByNameOnPath(t, "Wakelock", "src/test/kotlin/WorkTest.kt", wakelock); len(got) != 0 {
		t.Fatalf("expected no Wakelock finding in test sources, got %d", len(got))
	}
	wrongCall := `
package test
import android.view.View
class CustomView : View() {
    fun redraw(canvas: android.graphics.Canvas) {
        onDraw(canvas)
    }
}`
	if got := runRuleByNameOnPath(t, "WrongCall", "src/main/kotlin/CustomView.kt", wrongCall); len(got) != 1 {
		t.Fatalf("expected 1 WrongCall finding in main sources, got %d", len(got))
	}
	if got := runRuleByNameOnPath(t, "WrongCall", "src/test/kotlin/CustomViewTest.kt", wrongCall); len(got) != 0 {
		t.Fatalf("expected no WrongCall finding in test sources, got %d", len(got))
	}
}

// The receiver of a WakeLock extension is the caller's lock.
func TestWakelock_Negative_ExtensionReceiver(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "Wakelock", `
package test
import android.os.PowerManager.WakeLock
fun WakeLock.hold() {
    acquire()
}
fun WakeLock.holdThis() {
    this.acquire()
}`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for an extension receiver, got %d", len(findings))
	}
}
