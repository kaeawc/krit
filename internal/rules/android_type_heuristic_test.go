package rules_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
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

func runRangeRuleWithCallTargets(t *testing.T, code string, targets map[string]string) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	fake := oracle.NewFakeOracle()
	fake.CallTargets[file.Path] = map[string]string{}
	file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
		text := file.FlatNodeText(idx)
		for needle, target := range targets {
			if strings.Contains(text, needle) {
				key := fmt.Sprintf("%d:%d", file.FlatRow(idx)+1, file.FlatCol(idx)+1)
				fake.CallTargets[file.Path][key] = target
			}
		}
	})
	composite := oracle.NewCompositeResolver(fake, resolver)
	for _, r := range v2rules.Registry {
		if r.ID == "Range" {
			cols := rules.NewDispatcherV2([]*v2rules.Rule{r}, composite).Run(file)
			return cols.Findings()
		}
	}
	t.Fatal("rule Range not found in registry")
	return nil
}

func TestRange_SetAlphaOutOfRange(t *testing.T) {
	findings := runRangeRuleWithCallTargets(t, `
package test
fun example() {
    view.setAlpha(300)
    view.setAlpha(-1)
}`, map[string]string{"setAlpha": "android.view.View.setAlpha"})
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
	findings := runRangeRuleWithCallTargets(t, `
package test
fun example() {
    view.setAlpha(0)
    view.setAlpha(128)
    view.setAlpha(255)
}`, map[string]string{"setAlpha": "android.view.View.setAlpha"})
	for _, f := range findings {
		if f.Rule == "Range" && strings.Contains(f.Message, "setAlpha") {
			t.Errorf("Should not flag valid setAlpha values, got: %s", f.Message)
		}
	}
}

func TestRange_ColorArgbOutOfRange(t *testing.T) {
	findings := runRangeRuleWithCallTargets(t, `
package test
fun example() {
    val c = Color.argb(256, 0, 0, 0)
}`, map[string]string{"Color.argb": "android.graphics.Color.argb"})
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
	findings := runRangeRuleWithCallTargets(t, `
package test
fun example() {
    val c = Color.rgb(-1, 0, 0)
}`, map[string]string{"Color.rgb": "android.graphics.Color.rgb"})
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
	findings := runRangeRuleWithCallTargets(t, `
package test
fun example() {
    progressBar.setProgress(150)
}`, map[string]string{"setProgress": "android.widget.ProgressBar.setProgress"})
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
	findings := runRangeRuleWithCallTargets(t, `
package test
fun example() {
    progressBar.setProgress(50)
    progressBar.setProgress(0)
    progressBar.setProgress(100)
}`, map[string]string{"setProgress": "android.widget.ProgressBar.setProgress"})
	for _, f := range findings {
		if f.Rule == "Range" && strings.Contains(f.Message, "setProgress") {
			t.Errorf("Should not flag valid setProgress values, got: %s", f.Message)
		}
	}
}

func TestRange_SetRotationOutOfRange(t *testing.T) {
	findings := runRangeRuleWithCallTargets(t, `
package test
fun example() {
    view.setRotation(720)
}`, map[string]string{"setRotation": "android.view.View.setRotation"})
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

func TestRange_MultilineAndUnaryNegative(t *testing.T) {
	findings := runRangeRuleWithCallTargets(t, `
package test
fun example() {
    Color.rgb(
        255,
        255,
        999
    )
    progressBar.setProgress(-1)
}`, map[string]string{
		"Color.rgb":   "android.graphics.Color.rgb",
		"setProgress": "android.widget.ProgressBar.setProgress",
	})
	count := 0
	for _, f := range findings {
		if f.Rule == "Range" {
			count++
		}
	}
	if count != 2 {
		t.Errorf("Expected 2 range findings for multiline rgb and unary negative progress, got %d", count)
	}
}

func TestRange_LocalAnnotatedParameter(t *testing.T) {
	findings := runRuleByName(t, "Range", `
package test
fun bounded(@IntRange(from = 0, to = 10) value: Int) {}
fun example() {
    bounded(11)
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "Range" && strings.Contains(f.Message, "bounded") {
			found = true
		}
	}
	if !found {
		t.Error("Should flag same-file @IntRange parameter")
	}
}

func TestRange_LocalConstantInRange(t *testing.T) {
	findings := runRuleByName(t, "Range", `
package test
const val OK = 7
fun bounded(@IntRange(from = 0, to = 10) value: Int) {}
fun example() {
    bounded(OK)
}`)
	for _, f := range findings {
		if f.Rule == "Range" {
			t.Errorf("Should not flag same-file constant inside range, got: %s", f.Message)
		}
	}
}

func TestRange_LocalConstantShadowingUsesNearestScope(t *testing.T) {
	findings := runRuleByName(t, "Range", `
package test
const val LIMIT = 99
fun bounded(@IntRange(from = 0, to = 10) value: Int) {}
fun example() {
    val LIMIT = 7
    bounded(LIMIT)
}`)
	for _, f := range findings {
		if f.Rule == "Range" {
			t.Errorf("Should resolve same-function constant before top-level constant, got: %s", f.Message)
		}
	}
}

func TestRange_DynamicAndUnresolvedValuesAreSkipped(t *testing.T) {
	findings := runRuleByName(t, "Range", `
package test
fun bounded(@IntRange(from = 0, to = 10) value: Int) {}
fun example(input: Int) {
    bounded(input + 100)
    bounded(MISSING)
}`)
	for _, f := range findings {
		if f.Rule == "Range" {
			t.Errorf("Should not flag dynamic or unresolved values, got: %s", f.Message)
		}
	}
}

func TestRange_SameNameAnnotatedMemberDoesNotMatchUnqualifiedTopLevelCall(t *testing.T) {
	findings := runRuleByName(t, "Range", `
package test
class Limits {
    fun bounded(@IntRange(from = 0, to = 10) value: Int) {}
}
fun bounded(value: Int) {}
fun example() {
    bounded(11)
}`)
	for _, f := range findings {
		if f.Rule == "Range" {
			t.Errorf("Should not apply same-file member annotation to top-level call, got: %s", f.Message)
		}
	}
}

func TestRange_QualifiedSameFileOwnerMatch(t *testing.T) {
	findings := runRuleByName(t, "Range", `
package test
object Limits {
    fun bounded(@IntRange(from = 0, to = 10) value: Int) {}
}
fun example() {
    Limits.bounded(11)
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "Range" && strings.Contains(f.Message, "bounded") {
			found = true
		}
	}
	if !found {
		t.Error("Should flag same-file declaration when receiver qualifies the declaring owner")
	}
}

func TestRange_FloatExclusiveBound(t *testing.T) {
	findings := runRuleByName(t, "Range", `
package test
fun bounded(@FloatRange(from = 0.0, to = 1.0, toInclusive = false) value: Float) {}
fun example() {
    bounded(1.0f)
}`)
	found := false
	for _, f := range findings {
		if f.Rule == "Range" && strings.Contains(f.Message, "bounded") {
			found = true
		}
	}
	if !found {
		t.Error("Should flag exclusive @FloatRange upper bound")
	}
}

func TestRange_UnresolvedFrameworkAndUnannotatedProjectCallsSkipped(t *testing.T) {
	findings := runRuleByName(t, "Range", `
package test
import android.graphics.Color as PaintColor
class Gauge { fun setProgress(value: Int) {} }
fun example(gauge: Gauge) {
    // view.setAlpha(300)
    val text = "Color.rgb(255, 255, 999)"
    PaintColor.rgb(255, 255, 999)
    gauge.setProgress(150)
}`)
	for _, f := range findings {
		if f.Rule == "Range" {
			t.Errorf("Should skip comments, strings, unresolved framework aliases, and unannotated project calls, got: %s", f.Message)
		}
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
