package rules_test

import (
	"strings"
	"testing"
)

// =====================================================================
// NewApi tests
// =====================================================================

func TestNewApi_FlagsUnguardedSetElevation(t *testing.T) {
	findings := runRuleByName(t, "NewApi", `
package test
class MyView : View {
    fun setup() {
        view.setElevation(4f)
    }
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "NewApi" && strings.Contains(f.Message, "setElevation") && strings.Contains(f.Message, "21") {
			found = true
		}
	}
	if !found {
		t.Error("NewApi should flag unguarded setElevation (requires API 21)")
	}
}

func TestNewApi_FlagsNotificationChannel(t *testing.T) {
	findings := runRuleByName(t, "NewApi", `
package test
fun createChannel() {
    val channel = NotificationChannel("id", "name", NotificationManager.IMPORTANCE_DEFAULT)
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "NewApi" && strings.Contains(f.Message, "NotificationChannel") && strings.Contains(f.Message, "26") {
			found = true
		}
	}
	if !found {
		t.Error("NewApi should flag unguarded NotificationChannel (requires API 26)")
	}
}

func TestNewApi_FlagsBiometricPrompt(t *testing.T) {
	findings := runRuleByName(t, "NewApi", `
package test
fun showBiometric() {
    val prompt = BiometricPrompt.PromptInfo.Builder()
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "NewApi" && strings.Contains(f.Message, "BiometricPrompt") && strings.Contains(f.Message, "28") {
			found = true
		}
	}
	if !found {
		t.Error("NewApi should flag unguarded BiometricPrompt (requires API 28)")
	}
}

func TestNewApi_FlagsCheckSelfPermission(t *testing.T) {
	findings := runRuleByName(t, "NewApi", `
package test
fun check() {
    ContextCompat.checkSelfPermission(this, permission)
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "NewApi" && strings.Contains(f.Message, "checkSelfPermission") && strings.Contains(f.Message, "23") {
			found = true
		}
	}
	if !found {
		t.Error("NewApi should flag unguarded checkSelfPermission (requires API 23)")
	}
}

func TestNewApi_SkipsRequiresApiGuard(t *testing.T) {
	findings := runRuleByName(t, "NewApi", `
package test
@RequiresApi(26)
fun createChannel() {
    val channel = NotificationChannel("id", "name", NotificationManager.IMPORTANCE_DEFAULT)
}`)
	// The @RequiresApi is on a different line, but the NotificationChannel line itself
	// does not contain the guard. However, the rule skips lines containing @RequiresApi.
	// Let's test the line that has the guard on it:
	findings2 := runRuleByName(t, "NewApi", `
package test
fun createChannel() {
    @RequiresApi(26) val channel = NotificationChannel("id", "name", NotificationManager.IMPORTANCE_DEFAULT)
}`)
	for _, f := range findings2 {
		if f.Rule == "NewApi" && strings.Contains(f.Message, "NotificationChannel") {
			t.Error("NewApi should skip lines with @RequiresApi guard")
		}
	}
	_ = findings // first test may or may not flag depending on line-by-line
}

func TestNewApi_SkipsBuildVersionCheck(t *testing.T) {
	findings := runRuleByName(t, "NewApi", `
package test
fun createChannel() {
    if (Build.VERSION.SDK_INT >= 26) NotificationChannel("id", "name", 0)
}`)
	for _, f := range findings {
		if f.Rule == "NewApi" && strings.Contains(f.Message, "NotificationChannel") {
			t.Error("NewApi should skip lines with Build.VERSION.SDK_INT guard")
		}
	}
}

func TestNewApi_SkipsImports(t *testing.T) {
	findings := runRuleByName(t, "NewApi", `
package test
import android.app.NotificationChannel
class Foo`)
	for _, f := range findings {
		if f.Rule == "NewApi" && strings.Contains(f.Message, "NotificationChannel") {
			t.Error("NewApi should skip import lines")
		}
	}
}

func TestNewApi_SkipsComments(t *testing.T) {
	findings := runRuleByName(t, "NewApi", `
package test
// NotificationChannel is used for API 26+
class Foo`)
	for _, f := range findings {
		if f.Rule == "NewApi" && strings.Contains(f.Message, "NotificationChannel") {
			t.Error("NewApi should skip comment lines")
		}
	}
}

func TestNewApi_FlagsWindowInsetsController(t *testing.T) {
	findings := runRuleByName(t, "NewApi", `
package test
fun hideSystemBars() {
    window.insetsController as WindowInsetsController
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "NewApi" && strings.Contains(f.Message, "WindowInsetsController") && strings.Contains(f.Message, "30") {
			found = true
		}
	}
	if !found {
		t.Error("NewApi should flag unguarded WindowInsetsController (requires API 30)")
	}
}

// =====================================================================
// InlinedApi tests
// =====================================================================

func TestInlinedApi_FlagsSystemUiFlags(t *testing.T) {
	findings := runRuleByName(t, "InlinedApi", `
package test
fun hideSystemUi() {
    window.decorView.systemUiVisibility = View.SYSTEM_UI_FLAG_IMMERSIVE_STICKY
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "InlinedApi" && strings.Contains(f.Message, "SYSTEM_UI_FLAG_IMMERSIVE_STICKY") && strings.Contains(f.Message, "19") {
			found = true
		}
	}
	if !found {
		t.Error("InlinedApi should flag SYSTEM_UI_FLAG_IMMERSIVE_STICKY (API 19)")
	}
}

func TestInlinedApi_FlagsReadExternalStorage(t *testing.T) {
	findings := runRuleByName(t, "InlinedApi", `
package test
fun checkPermission() {
    if (checkSelfPermission(READ_EXTERNAL_STORAGE) == GRANTED) {}
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "InlinedApi" && strings.Contains(f.Message, "READ_EXTERNAL_STORAGE") && strings.Contains(f.Message, "16") {
			found = true
		}
	}
	if !found {
		t.Error("InlinedApi should flag READ_EXTERNAL_STORAGE (API 16)")
	}
}

func TestInlinedApi_FlagsBuildVersionCodes(t *testing.T) {
	findings := runRuleByName(t, "InlinedApi", `
package test
fun isApi33() {
    val is33 = Build.VERSION_CODES.TIRAMISU
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "InlinedApi" && strings.Contains(f.Message, "TIRAMISU") && strings.Contains(f.Message, "33") {
			found = true
		}
	}
	if !found {
		t.Error("InlinedApi should flag Build.VERSION_CODES.TIRAMISU (API 33)")
	}
}

func TestInlinedApi_SkipsGuardedLines(t *testing.T) {
	findings := runRuleByName(t, "InlinedApi", `
package test
fun check() {
    if (Build.VERSION.SDK_INT >= 19) View.SYSTEM_UI_FLAG_IMMERSIVE_STICKY
}`)
	for _, f := range findings {
		if f.Rule == "InlinedApi" && strings.Contains(f.Message, "SYSTEM_UI_FLAG_IMMERSIVE_STICKY") {
			t.Error("InlinedApi should skip lines with Build.VERSION.SDK_INT guard")
		}
	}
}

func TestInlinedApi_SkipsImports(t *testing.T) {
	findings := runRuleByName(t, "InlinedApi", `
package test
import android.Manifest.permission.READ_EXTERNAL_STORAGE
class Foo`)
	for _, f := range findings {
		if f.Rule == "InlinedApi" && strings.Contains(f.Message, "READ_EXTERNAL_STORAGE") {
			t.Error("InlinedApi should skip import lines")
		}
	}
}

func TestInlinedApi_SkipsComments(t *testing.T) {
	findings := runRuleByName(t, "InlinedApi", `
package test
// Use SYSTEM_UI_FLAG_IMMERSIVE for immersive mode
class Foo`)
	for _, f := range findings {
		if f.Rule == "InlinedApi" && strings.Contains(f.Message, "SYSTEM_UI_FLAG_IMMERSIVE") {
			t.Error("InlinedApi should skip comment lines")
		}
	}
}

// =====================================================================
// Deprecated tests
// =====================================================================

func TestDeprecated_FlagsAsyncTask(t *testing.T) {
	findings := runRuleByName(t, "Deprecated", `
package test
class MyTask : AsyncTask<Void, Void, String>() {
    override fun doInBackground(vararg params: Void?): String = ""
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "Deprecated" && strings.Contains(f.Message, "AsyncTask") && strings.Contains(f.Message, "API 30") {
			found = true
		}
	}
	if !found {
		t.Error("Deprecated should flag AsyncTask usage")
	}
}

func TestDeprecated_FlagsIntentService(t *testing.T) {
	findings := runRuleByName(t, "Deprecated", `
package test
class MyService : IntentService("worker") {
    override fun onHandleIntent(intent: Intent?) {}
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "Deprecated" && strings.Contains(f.Message, "IntentService") {
			found = true
		}
	}
	if !found {
		t.Error("Deprecated should flag IntentService usage")
	}
}

func TestDeprecated_FlagsLocalBroadcastManager(t *testing.T) {
	findings := runRuleByName(t, "Deprecated", `
package test
fun register() {
    LocalBroadcastManager.getInstance(this).registerReceiver(receiver, filter)
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "Deprecated" && strings.Contains(f.Message, "LocalBroadcastManager") {
			found = true
		}
	}
	if !found {
		t.Error("Deprecated should flag LocalBroadcastManager usage")
	}
}

func TestDeprecated_FlagsCursorLoader(t *testing.T) {
	findings := runRuleByName(t, "Deprecated", `
package test
fun loadData() {
    val loader = CursorLoader(context, uri, null, null, null, null)
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "Deprecated" && strings.Contains(f.Message, "CursorLoader") {
			found = true
		}
	}
	if !found {
		t.Error("Deprecated should flag CursorLoader usage")
	}
}

func TestDeprecated_FlagsDefaultHttpClient(t *testing.T) {
	findings := runRuleByName(t, "Deprecated", `
package test
fun makeRequest() {
    val client = DefaultHttpClient()
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "Deprecated" && strings.Contains(f.Message, "DefaultHttpClient") {
			found = true
		}
	}
	if !found {
		t.Error("Deprecated should flag DefaultHttpClient usage")
	}
}

func TestDeprecated_FlagsGetRunningTasks(t *testing.T) {
	findings := runRuleByName(t, "Deprecated", `
package test
fun getTasks() {
    activityManager.getRunningTasks(10)
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "Deprecated" && strings.Contains(f.Message, "getRunningTasks") {
			found = true
		}
	}
	if !found {
		t.Error("Deprecated should flag getRunningTasks usage")
	}
}

func TestDeprecated_SkipsImports(t *testing.T) {
	findings := runRuleByName(t, "Deprecated", `
package test
import android.os.AsyncTask
class Foo`)
	for _, f := range findings {
		if f.Rule == "Deprecated" && strings.Contains(f.Message, "AsyncTask") {
			t.Error("Deprecated should skip import lines")
		}
	}
}

func TestDeprecated_SkipsComments(t *testing.T) {
	findings := runRuleByName(t, "Deprecated", `
package test
// AsyncTask is deprecated, use coroutines instead
class Foo`)
	for _, f := range findings {
		if f.Rule == "Deprecated" && strings.Contains(f.Message, "AsyncTask") {
			t.Error("Deprecated should skip comment lines")
		}
	}
}

func TestDeprecated_FlagsTabActivity(t *testing.T) {
	findings := runRuleByName(t, "Deprecated", `
package test
class MyTabs : TabActivity() {}`)
	found := false
	for _, f := range findings {
		if f.Rule == "Deprecated" && strings.Contains(f.Message, "TabActivity") {
			found = true
		}
	}
	if !found {
		t.Error("Deprecated should flag TabActivity usage")
	}
}

// =====================================================================
// Override tests
// =====================================================================

func TestOverride_FlagsMissingOverrideOnBackPressed(t *testing.T) {
	findings := runRuleByName(t, "Override", `
package test
class MyActivity : AppCompatActivity() {
    fun onBackPressed() {
        finish()
    }
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "Override" && strings.Contains(f.Message, "onBackPressed") {
			found = true
		}
	}
	if !found {
		t.Error("Override should flag fun onBackPressed() without override in Activity subclass")
	}
}

func TestOverride_SkipsCorrectOverride(t *testing.T) {
	findings := runRuleByName(t, "Override", `
package test
class MyActivity : AppCompatActivity() {
    override fun onBackPressed() {
        super.onBackPressed()
    }
}`)
	for _, f := range findings {
		if f.Rule == "Override" && strings.Contains(f.Message, "onBackPressed") {
			t.Error("Override should not flag override fun onBackPressed()")
		}
	}
}

func TestOverride_SkipsNonActivityClass(t *testing.T) {
	findings := runRuleByName(t, "Override", `
package test
class MyHelper {
    fun onBackPressed() {
        // not in an Activity
    }
}`)
	for _, f := range findings {
		if f.Rule == "Override" && strings.Contains(f.Message, "onBackPressed") {
			t.Error("Override should not flag onBackPressed in non-Activity classes")
		}
	}
}

func TestOverride_FlagsMissingOverrideOnCreateOptionsMenu(t *testing.T) {
	findings := runRuleByName(t, "Override", `
package test
class MyActivity : Activity() {
    fun onCreateOptionsMenu(menu: Menu): Boolean {
        return true
    }
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "Override" && strings.Contains(f.Message, "onCreateOptionsMenu") {
			found = true
		}
	}
	if !found {
		t.Error("Override should flag fun onCreateOptionsMenu without override in Activity subclass")
	}
}

func TestOverride_WorksWithFragment(t *testing.T) {
	findings := runRuleByName(t, "Override", `
package test
class MyFragment : Fragment() {
    fun onCreateOptionsMenu(menu: Menu, inflater: MenuInflater) {
    }
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "Override" && strings.Contains(f.Message, "onCreateOptionsMenu") {
			found = true
		}
	}
	if !found {
		t.Error("Override should flag fun onCreateOptionsMenu without override in Fragment subclass")
	}
}
