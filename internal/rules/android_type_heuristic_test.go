package rules_test

import (
	"strings"
	"testing"
)

// =====================================================================
// WrongViewCast tests
// =====================================================================

func TestWrongViewCast_GenericSyntaxMismatch(t *testing.T) {
	findings := runRuleByName(t, "WrongViewCast", `
package test
class MyActivity {
    fun setup() {
        val view = findViewById<TextView>(R.id.btn_submit)
    }
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "WrongViewCast" && strings.Contains(f.Message, "btn_") {
			found = true
		}
	}
	if !found {
		t.Error("Should flag TextView cast for btn_ prefixed id")
	}
}

func TestWrongViewCast_AsCastMismatch(t *testing.T) {
	findings := runRuleByName(t, "WrongViewCast", `
package test
class MyActivity {
    fun setup() {
        val view = findViewById(R.id.iv_avatar) as TextView
    }
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "WrongViewCast" && strings.Contains(f.Message, "iv_") {
			found = true
		}
	}
	if !found {
		t.Error("Should flag TextView cast for iv_ prefixed id")
	}
}

func TestWrongViewCast_CorrectCast(t *testing.T) {
	findings := runRuleByName(t, "WrongViewCast", `
package test
class MyActivity {
    fun setup() {
        val btn = findViewById<Button>(R.id.btn_submit)
        val tv = findViewById<TextView>(R.id.tv_title)
        val iv = findViewById<ImageView>(R.id.iv_avatar)
        val rv = findViewById<RecyclerView>(R.id.rv_list)
        val et = findViewById<EditText>(R.id.et_name)
    }
}`)
	for _, f := range findings {
		if f.Rule == "WrongViewCast" {
			t.Errorf("Should not flag correct casts, got: %s", f.Message)
		}
	}
}

func TestWrongViewCast_NoPrefixNoFlag(t *testing.T) {
	findings := runRuleByName(t, "WrongViewCast", `
package test
class MyActivity {
    fun setup() {
        val view = findViewById<TextView>(R.id.someView)
    }
}`)
	for _, f := range findings {
		if f.Rule == "WrongViewCast" {
			t.Errorf("Should not flag ids without recognized prefix, got: %s", f.Message)
		}
	}
}

func TestWrongViewCast_RecyclerPrefixMismatch(t *testing.T) {
	findings := runRuleByName(t, "WrongViewCast", `
package test
class MyActivity {
    fun setup() {
        val view = findViewById<TextView>(R.id.recycler_items)
    }
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "WrongViewCast" {
			found = true
		}
	}
	if !found {
		t.Error("Should flag TextView cast for recycler_ prefixed id")
	}
}

func TestWrongViewCast_EditTextPrefixMatch(t *testing.T) {
	findings := runRuleByName(t, "WrongViewCast", `
package test
class MyActivity {
    fun setup() {
        val et = findViewById<TextInputEditText>(R.id.et_email)
    }
}`)
	for _, f := range findings {
		if f.Rule == "WrongViewCast" {
			t.Errorf("Should not flag TextInputEditText for et_ prefix, got: %s", f.Message)
		}
	}
}

// =====================================================================
// Range tests
// =====================================================================

func TestRange_SetAlphaOutOfRange(t *testing.T) {
	findings := runRuleByName(t, "Range", `
package test
fun example() {
    view.setAlpha(300)
    view.setAlpha(-1)
}`)
	count := 0
	for _, f := range findings {
		if f.Rule == "Range" && strings.Contains(f.Message, "setAlpha") {
			count++
		}
	}
	if count != 2 {
		t.Errorf("Expected 2 setAlpha range findings, got %d", count)
	}
}

func TestRange_SetAlphaInRange(t *testing.T) {
	findings := runRuleByName(t, "Range", `
package test
fun example() {
    view.setAlpha(0)
    view.setAlpha(128)
    view.setAlpha(255)
}`)
	for _, f := range findings {
		if f.Rule == "Range" && strings.Contains(f.Message, "setAlpha") {
			t.Errorf("Should not flag valid setAlpha values, got: %s", f.Message)
		}
	}
}

func TestRange_ColorArgbOutOfRange(t *testing.T) {
	findings := runRuleByName(t, "Range", `
package test
fun example() {
    val c = Color.argb(256, 0, 0, 0)
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "Range" && strings.Contains(f.Message, "Color.argb") {
			found = true
		}
	}
	if !found {
		t.Error("Should flag Color.argb with value 256")
	}
}

func TestRange_ColorRgbOutOfRange(t *testing.T) {
	findings := runRuleByName(t, "Range", `
package test
fun example() {
    val c = Color.rgb(-1, 0, 0)
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "Range" && strings.Contains(f.Message, "Color.rgb") {
			found = true
		}
	}
	if !found {
		t.Error("Should flag Color.rgb with value -1")
	}
}

func TestRange_SetProgressOutOfRange(t *testing.T) {
	findings := runRuleByName(t, "Range", `
package test
fun example() {
    progressBar.setProgress(150)
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "Range" && strings.Contains(f.Message, "setProgress") {
			found = true
		}
	}
	if !found {
		t.Error("Should flag setProgress(150)")
	}
}

func TestRange_SetProgressInRange(t *testing.T) {
	findings := runRuleByName(t, "Range", `
package test
fun example() {
    progressBar.setProgress(50)
    progressBar.setProgress(0)
    progressBar.setProgress(100)
}`)
	for _, f := range findings {
		if f.Rule == "Range" && strings.Contains(f.Message, "setProgress") {
			t.Errorf("Should not flag valid setProgress values, got: %s", f.Message)
		}
	}
}

func TestRange_SetRotationOutOfRange(t *testing.T) {
	findings := runRuleByName(t, "Range", `
package test
fun example() {
    view.setRotation(720)
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "Range" && strings.Contains(f.Message, "setRotation") {
			found = true
		}
	}
	if !found {
		t.Error("Should flag setRotation(720)")
	}
}

// =====================================================================
// OverrideAbstract tests
// =====================================================================

func TestOverrideAbstract_ServiceMissingOnBind(t *testing.T) {
	findings := runRuleByName(t, "OverrideAbstract", `
package test
class MyService : Service() {
    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        return START_STICKY
    }
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "OverrideAbstract" && strings.Contains(f.Message, "onBind") {
			found = true
		}
	}
	if !found {
		t.Error("Should flag Service subclass missing onBind")
	}
}

func TestOverrideAbstract_ServiceWithOnBind(t *testing.T) {
	findings := runRuleByName(t, "OverrideAbstract", `
package test
class MyService : Service() {
    override fun onBind(intent: Intent?): IBinder? {
        return null
    }
}`)
	for _, f := range findings {
		if f.Rule == "OverrideAbstract" {
			t.Errorf("Should not flag Service with onBind, got: %s", f.Message)
		}
	}
}

func TestOverrideAbstract_BroadcastReceiverMissingOnReceive(t *testing.T) {
	findings := runRuleByName(t, "OverrideAbstract", `
package test
class MyReceiver : BroadcastReceiver() {
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "OverrideAbstract" && strings.Contains(f.Message, "onReceive") {
			found = true
		}
	}
	if !found {
		t.Error("Should flag BroadcastReceiver missing onReceive")
	}
}

func TestOverrideAbstract_BroadcastReceiverWithOnReceive(t *testing.T) {
	findings := runRuleByName(t, "OverrideAbstract", `
package test
class MyReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        // handle
    }
}`)
	for _, f := range findings {
		if f.Rule == "OverrideAbstract" {
			t.Errorf("Should not flag BroadcastReceiver with onReceive, got: %s", f.Message)
		}
	}
}

func TestOverrideAbstract_ContentProviderPartialOverride(t *testing.T) {
	findings := runRuleByName(t, "OverrideAbstract", `
package test
class MyProvider : ContentProvider() {
    override fun onCreate(): Boolean = true
    override fun query(uri: Uri, projection: Array<String>?, selection: String?, args: Array<String>?, sort: String?): Cursor? = null
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "OverrideAbstract" && strings.Contains(f.Message, "insert") {
			found = true
		}
	}
	if !found {
		t.Error("Should flag ContentProvider with missing insert/update/delete/getType")
	}
}

func TestOverrideAbstract_AbstractClassSkipped(t *testing.T) {
	findings := runRuleByName(t, "OverrideAbstract", `
package test
abstract class BaseService : Service() {
}`)
	for _, f := range findings {
		if f.Rule == "OverrideAbstract" {
			t.Errorf("Should not flag abstract class, got: %s", f.Message)
		}
	}
}

// =====================================================================
// ObjectAnimatorBinding tests
// =====================================================================

func TestObjectAnimator_UnknownProperty(t *testing.T) {
	findings := runRuleByName(t, "ObjectAnimatorBinding", `
package test
fun animate() {
    ObjectAnimator.ofFloat(view, "translatoinX", 0f, 100f)
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "ObjectAnimatorBinding" && strings.Contains(f.Message, "translatoinX") {
			found = true
		}
	}
	if !found {
		t.Error("Should flag misspelled property 'translatoinX'")
	}
}

func TestObjectAnimator_KnownProperty(t *testing.T) {
	findings := runRuleByName(t, "ObjectAnimatorBinding", `
package test
fun animate() {
    ObjectAnimator.ofFloat(view, "alpha", 0f, 1f)
    ObjectAnimator.ofFloat(view, "translationX", 0f, 100f)
    ObjectAnimator.ofFloat(view, "translationY", 0f, 100f)
    ObjectAnimator.ofFloat(view, "rotation", 0f, 360f)
    ObjectAnimator.ofFloat(view, "scaleX", 1f, 2f)
    ObjectAnimator.ofFloat(view, "scaleY", 1f, 2f)
    ObjectAnimator.ofInt(view, "elevation", 0, 10)
}`)
	for _, f := range findings {
		if f.Rule == "ObjectAnimatorBinding" {
			t.Errorf("Should not flag known properties, got: %s", f.Message)
		}
	}
}

func TestObjectAnimator_OfIntUnknownProperty(t *testing.T) {
	findings := runRuleByName(t, "ObjectAnimatorBinding", `
package test
fun animate() {
    ObjectAnimator.ofInt(view, "backgroundTint", 0, 255)
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "ObjectAnimatorBinding" && strings.Contains(f.Message, "backgroundTint") {
			found = true
		}
	}
	if !found {
		t.Error("Should flag non-standard property 'backgroundTint'")
	}
}

func TestObjectAnimator_OfObjectUnknownProperty(t *testing.T) {
	findings := runRuleByName(t, "ObjectAnimatorBinding", `
package test
fun animate() {
    ObjectAnimator.ofObject(view, "colour", evaluator, startColor, endColor)
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "ObjectAnimatorBinding" && strings.Contains(f.Message, "colour") {
			found = true
		}
	}
	if !found {
		t.Error("Should flag non-standard property 'colour'")
	}
}

func TestObjectAnimator_CommentIgnored(t *testing.T) {
	findings := runRuleByName(t, "ObjectAnimatorBinding", `
package test
fun animate() {
    // ObjectAnimator.ofFloat(view, "foo", 0f, 1f)
}`)
	for _, f := range findings {
		if f.Rule == "ObjectAnimatorBinding" {
			t.Errorf("Should not flag commented-out code, got: %s", f.Message)
		}
	}
}

// =====================================================================
// SwitchIntDef tests
// =====================================================================

func TestSwitchIntDef_MissingVisibilityConstant(t *testing.T) {
	findings := runRuleByName(t, "SwitchIntDef", `
package test
fun checkVisibility(view: View) {
    when (view.visibility) {
        View.VISIBLE -> show()
        View.GONE -> hide()
    }
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "SwitchIntDef" && strings.Contains(f.Message, "INVISIBLE") {
			found = true
		}
	}
	if !found {
		t.Error("Should flag missing INVISIBLE in visibility when")
	}
}

func TestSwitchIntDef_AllVisibilityCovered(t *testing.T) {
	findings := runRuleByName(t, "SwitchIntDef", `
package test
fun checkVisibility(view: View) {
    when (view.visibility) {
        View.VISIBLE -> show()
        View.INVISIBLE -> dim()
        View.GONE -> hide()
    }
}`)
	for _, f := range findings {
		if f.Rule == "SwitchIntDef" {
			t.Errorf("Should not flag when all visibility constants covered, got: %s", f.Message)
		}
	}
}

func TestSwitchIntDef_ElseBranchPresent(t *testing.T) {
	findings := runRuleByName(t, "SwitchIntDef", `
package test
fun checkVisibility(view: View) {
    when (view.visibility) {
        View.VISIBLE -> show()
        else -> hide()
    }
}`)
	for _, f := range findings {
		if f.Rule == "SwitchIntDef" {
			t.Errorf("Should not flag when else branch present, got: %s", f.Message)
		}
	}
}

func TestSwitchIntDef_NonVisibilityWhen(t *testing.T) {
	findings := runRuleByName(t, "SwitchIntDef", `
package test
fun check(x: Int) {
    when (x) {
        1 -> one()
        2 -> two()
    }
}`)
	for _, f := range findings {
		if f.Rule == "SwitchIntDef" {
			t.Errorf("Should not flag when on non-visibility values, got: %s", f.Message)
		}
	}
}

func TestSwitchIntDef_MissingTwoConstants(t *testing.T) {
	findings := runRuleByName(t, "SwitchIntDef", `
package test
fun checkVisibility(view: View) {
    when (view.visibility) {
        View.VISIBLE -> show()
    }
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "SwitchIntDef" && strings.Contains(f.Message, "INVISIBLE") && strings.Contains(f.Message, "GONE") {
			found = true
		}
	}
	if !found {
		t.Error("Should flag missing INVISIBLE and GONE")
	}
}
