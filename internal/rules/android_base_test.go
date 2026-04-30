package rules_test

import "testing"

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
fun MyScreen() {
    Text(title = getString(R.string.hello))
}`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for getString usage, got %d", len(findings))
	}
}

// --- LogDetectorRule (LogConditional) ---

func TestLogConditional_Positive(t *testing.T) {
	findings := runRuleByName(t, "LogConditional", `
package test
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
fun doWork() {
    if (Log.isLoggable("TAG", Log.DEBUG)) {
        Log.d("TAG", "doing work")
    }
}`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// --- SdCardPathRule ---

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
}

// --- PrivateKeyRule (PackagedPrivateKey) ---

func TestPackagedPrivateKey_Positive(t *testing.T) {
	findings := runRuleByName(t, "PackagedPrivateKey", `
package test
val key = "-----BEGIN RSA PRIVATE KEY-----\nMIIE..."
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
class MyAdapter : RecyclerView.Adapter<MyVH>() {
    override fun onBindViewHolder(holder: MyVH, position: Int) {}
    override fun getItemCount(): Int = 0
}`)
	if len(findings) == 0 {
		t.Error("expected ViewHolder finding for Adapter without ViewHolder pattern")
	}
}

func TestViewHolder_Negative(t *testing.T) {
	findings := runRuleByName(t, "ViewHolder", `
package test
class MyAdapter : RecyclerView.Adapter<MyAdapter.MyViewHolder>() {
    class MyViewHolder(view: View) : RecyclerView.ViewHolder(view)
    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): MyViewHolder {
        return MyViewHolder(LayoutInflater.from(parent.context).inflate(R.layout.item, parent, false))
    }
    override fun onBindViewHolder(holder: MyViewHolder, position: Int) {}
    override fun getItemCount(): Int = 0
}`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}
