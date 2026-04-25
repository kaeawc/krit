package rules_test

import (
	"strings"
	"testing"
)

// =====================================================================
// ViewHolderRule tests
// =====================================================================

func TestViewHolder_FlagsAdapterWithoutViewHolder(t *testing.T) {
	findings := runRuleByName(t, "ViewHolder", `
package test

class MyAdapter : RecyclerView.Adapter<MyItem>() {
    override fun getItemCount(): Int = 0
    override fun onBindViewHolder(holder: Any, position: Int) {}
}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "ViewHolder" {
			found = true
		}
	}
	if !found {
		t.Error("ViewHolderRule should flag Adapter without ViewHolder pattern")
	}
}

func TestViewHolder_IgnoresAdapterWithViewHolder(t *testing.T) {
	findings := runRuleByName(t, "ViewHolder", `
package test

class MyAdapter : RecyclerView.Adapter<MyAdapter.MyViewHolder>() {
    class MyViewHolder(view: View) : RecyclerView.ViewHolder(view)
    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): MyViewHolder {
        return MyViewHolder(parent)
    }
    override fun getItemCount(): Int = 0
    override fun onBindViewHolder(holder: MyViewHolder, position: Int) {}
}
`)
	for _, f := range findings {
		if f.Rule == "ViewHolder" {
			t.Error("ViewHolderRule should not flag Adapter with ViewHolder")
		}
	}
}

func TestViewHolder_IgnoresNonAdapterClass(t *testing.T) {
	findings := runRuleByName(t, "ViewHolder", `
package test

class MyService : Service() {
    override fun onBind(intent: Intent): IBinder? = null
}
`)
	for _, f := range findings {
		if f.Rule == "ViewHolder" {
			t.Error("ViewHolderRule should not flag non-Adapter classes")
		}
	}
}

// =====================================================================
// ObsoleteLayoutParamsRule tests
// =====================================================================

func TestObsoleteLayoutParam_FlagsPreferredWidth(t *testing.T) {
	findings := runRuleByName(t, "ObsoleteLayoutParam", `
package test

import androidx.compose.foundation.layout.preferredWidth

fun MyComposable() {
    Box(modifier = Modifier.preferredWidth(100.dp))
}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "ObsoleteLayoutParam" && strings.Contains(f.Message, "preferredWidth") {
			found = true
		}
	}
	if !found {
		t.Error("ObsoleteLayoutParamsRule should flag preferredWidth")
	}
}

func TestObsoleteLayoutParam_FlagsPreferredHeight(t *testing.T) {
	findings := runRuleByName(t, "ObsoleteLayoutParam", `
package test

fun MyComposable() {
    Box(modifier = Modifier.preferredHeight(200.dp))
}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "ObsoleteLayoutParam" && strings.Contains(f.Message, "preferredHeight") {
			found = true
		}
	}
	if !found {
		t.Error("ObsoleteLayoutParamsRule should flag preferredHeight")
	}
}

func TestObsoleteLayoutParam_FlagsPreferredSize(t *testing.T) {
	findings := runRuleByName(t, "ObsoleteLayoutParam", `
package test

fun MyComposable() {
    Box(modifier = Modifier.preferredSize(50.dp))
}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "ObsoleteLayoutParam" && strings.Contains(f.Message, "preferredSize") {
			found = true
		}
	}
	if !found {
		t.Error("ObsoleteLayoutParamsRule should flag preferredSize")
	}
}

func TestObsoleteLayoutParam_IgnoresCurrentAPIs(t *testing.T) {
	findings := runRuleByName(t, "ObsoleteLayoutParam", `
package test

fun MyComposable() {
    Box(modifier = Modifier.width(100.dp).height(200.dp).size(50.dp))
}
`)
	for _, f := range findings {
		if f.Rule == "ObsoleteLayoutParam" {
			t.Error("ObsoleteLayoutParamsRule should not flag current API names (width, height, size)")
		}
	}
}

func TestObsoleteLayoutParam_IgnoresComments(t *testing.T) {
	findings := runRuleByName(t, "ObsoleteLayoutParam", `
package test

// Use preferredWidth instead of width for old API
fun MyComposable() {
    Box(modifier = Modifier.width(100.dp))
}
`)
	for _, f := range findings {
		if f.Rule == "ObsoleteLayoutParam" {
			t.Error("ObsoleteLayoutParamsRule should not flag occurrences in comments")
		}
	}
}

// =====================================================================
// PluralsCandidateRule tests
// =====================================================================

func TestPluralsCandidate_FlagsIfCountEqualsOne(t *testing.T) {
	findings := runRuleByName(t, "PluralsCandidate", `
package test

fun formatItems(count: Int): String {
    val label = if (count == 1) {
        getString(R.string.item_singular)
    } else {
        getString(R.string.item_plural)
    }
    return label
}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "PluralsCandidate" {
			found = true
		}
	}
	if !found {
		t.Error("PluralsCandidateRule should flag if (count == 1) near getString")
	}
}

func TestPluralsCandidate_FlagsWhenCount(t *testing.T) {
	findings := runRuleByName(t, "PluralsCandidate", `
package test

fun formatItems(count: Int): String {
    return when (count) {
        1 -> getString(R.string.item_one)
        else -> getString(R.string.item_other)
    }
}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "PluralsCandidate" {
			found = true
		}
	}
	if !found {
		t.Error("PluralsCandidateRule should flag when (count) with getString")
	}
}

func TestPluralsCandidate_IgnoresNonPluralIf(t *testing.T) {
	findings := runRuleByName(t, "PluralsCandidate", `
package test

fun checkPermission(level: Int) {
    if (level == 1) {
        requestPermission()
    }
}
`)
	for _, f := range findings {
		if f.Rule == "PluralsCandidate" {
			t.Error("PluralsCandidateRule should not flag if (level == 1) without string formatting")
		}
	}
}

func TestPluralsCandidate_IgnoresGetQuantityString(t *testing.T) {
	// Code already using getQuantityString should not be flagged even if
	// it also has a when(count) — but since it uses proper API, the when
	// pattern wouldn't typically appear. This tests that simple code without
	// string formatting near when is not flagged.
	findings := runRuleByName(t, "PluralsCandidate", `
package test

fun classify(num: Int): Boolean {
    return when (num) {
        0 -> false
        else -> true
    }
}
`)
	for _, f := range findings {
		if f.Rule == "PluralsCandidate" {
			t.Error("PluralsCandidateRule should not flag when without string formatting context")
		}
	}
}

// =====================================================================
// PropertyEscapeRule tests
// =====================================================================

func TestPropertyEscape_FlagsInvalidEscape(t *testing.T) {
	findings := runRuleByName(t, "PropertyEscape", `
package test

fun example() {
    val s = "Hello \w world"
}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "PropertyEscape" && strings.Contains(f.Message, "\\w") {
			found = true
		}
	}
	if !found {
		t.Error("PropertyEscapeRule should flag invalid escape \\w")
	}
}

func TestPropertyEscape_FlagsBackslashP(t *testing.T) {
	findings := runRuleByName(t, "PropertyEscape", `
package test

val path = "C:\program files\app"
`)
	found := false
	for _, f := range findings {
		if f.Rule == "PropertyEscape" && strings.Contains(f.Message, "\\p") {
			found = true
		}
	}
	if !found {
		t.Error("PropertyEscapeRule should flag invalid escape \\p")
	}
}

func TestPropertyEscape_IgnoresValidEscapes(t *testing.T) {
	findings := runRuleByName(t, "PropertyEscape", `
package test

fun example() {
    val a = "line1\nline2"
    val b = "tab\there"
    val c = "return\r"
    val d = "slash\\"
    val e = "quote\""
    val f = "dollar\$"
}
`)
	for _, f := range findings {
		if f.Rule == "PropertyEscape" {
			t.Errorf("PropertyEscapeRule should not flag valid escapes, got: %s", f.Message)
		}
	}
}

func TestPropertyEscape_IgnoresComments(t *testing.T) {
	findings := runRuleByName(t, "PropertyEscape", `
package test

// path like C:\program files
fun example() {}
`)
	for _, f := range findings {
		if f.Rule == "PropertyEscape" {
			t.Error("PropertyEscapeRule should not flag content in comments")
		}
	}
}

func TestPropertyEscape_IgnoresRawStrings(t *testing.T) {
	findings := runRuleByName(t, "PropertyEscape", `
package test

val raw = """
C:\program files\app
"""
`)
	for _, f := range findings {
		if f.Rule == "PropertyEscape" {
			t.Error("PropertyEscapeRule should not flag content in raw/triple-quoted strings")
		}
	}
}

// =====================================================================
// PropertyUsedBeforeDeclarationRule tests
// =====================================================================

func TestPropertyUsedBeforeDeclaration_EarlierInitializer(t *testing.T) {
	// Class property referenced in earlier property initializer — SHOULD trigger
	findings := runRuleByName(t, "PropertyUsedBeforeDeclaration", `
package test

class Foo {
    val a = b + 1
    val b = 42
}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "PropertyUsedBeforeDeclaration" && strings.Contains(f.Message, "'a'") && strings.Contains(f.Message, "'b'") {
			found = true
		}
	}
	if !found {
		t.Error("PropertyUsedBeforeDeclaration should flag property 'a' using 'b' which is declared later")
	}
}

func TestPropertyUsedBeforeDeclaration_LocalVarInFunction(t *testing.T) {
	// Local variable used before declaration in function — should NOT trigger
	findings := runRuleByName(t, "PropertyUsedBeforeDeclaration", `
package test

class Foo {
    fun bar() {
        val x = y
        val y = 10
    }
}
`)
	for _, f := range findings {
		if f.Rule == "PropertyUsedBeforeDeclaration" {
			t.Errorf("PropertyUsedBeforeDeclaration should NOT flag local variables inside functions, got: %s", f.Message)
		}
	}
}

func TestPropertyUsedBeforeDeclaration_PropertyUsedInFunctionBody(t *testing.T) {
	// Property used in function body — should NOT trigger (functions execute lazily)
	findings := runRuleByName(t, "PropertyUsedBeforeDeclaration", `
package test

class Foo {
    fun bar(): Int = b
    val b = 42
}
`)
	for _, f := range findings {
		if f.Rule == "PropertyUsedBeforeDeclaration" {
			t.Errorf("PropertyUsedBeforeDeclaration should NOT flag property used inside function body, got: %s", f.Message)
		}
	}
}

func TestPropertyUsedBeforeDeclaration_InitBlockBeforeDeclaration(t *testing.T) {
	// Property used in init block before declaration — SHOULD trigger
	findings := runRuleByName(t, "PropertyUsedBeforeDeclaration", `
package test

class Foo {
    init {
        println(x)
    }
    val x = 10
}
`)
	found := false
	for _, f := range findings {
		if f.Rule == "PropertyUsedBeforeDeclaration" && strings.Contains(f.Message, "'x'") {
			found = true
		}
	}
	if !found {
		t.Error("PropertyUsedBeforeDeclaration should flag init block using 'x' which is declared later")
	}
}

func TestPropertyUsedBeforeDeclaration_MultiLineFunSignature(t *testing.T) {
	// Multi-line function signature — should NOT trigger (old brace-counting bug)
	findings := runRuleByName(t, "PropertyUsedBeforeDeclaration", `
package test

class Foo {
    fun bar(
        param1: Int,
        param2: Int
    ): Int {
        return b
    }
    val b = 42
}
`)
	for _, f := range findings {
		if f.Rule == "PropertyUsedBeforeDeclaration" {
			t.Errorf("PropertyUsedBeforeDeclaration should NOT flag property used inside multi-line function, got: %s", f.Message)
		}
	}
}

func TestPropertyUsedBeforeDeclaration_LambdaInitializer(t *testing.T) {
	// Property with lambda initializer referencing a later property — should NOT trigger
	// because the lambda executes lazily
	findings := runRuleByName(t, "PropertyUsedBeforeDeclaration", `
package test

class Foo {
    val action = { b + 1 }
    val b = 42
}
`)
	for _, f := range findings {
		if f.Rule == "PropertyUsedBeforeDeclaration" {
			t.Errorf("PropertyUsedBeforeDeclaration should NOT flag lambda body referencing later property, got: %s", f.Message)
		}
	}
}

func TestPropertyUsedBeforeDeclaration_NoIssue(t *testing.T) {
	// Properties declared in correct order — should NOT trigger
	findings := runRuleByName(t, "PropertyUsedBeforeDeclaration", `
package test

class Foo {
    val a = 1
    val b = a + 1
}
`)
	for _, f := range findings {
		if f.Rule == "PropertyUsedBeforeDeclaration" {
			t.Errorf("PropertyUsedBeforeDeclaration should NOT fire when properties are in order, got: %s", f.Message)
		}
	}
}

// =====================================================================
// TrulyRandom tests
// =====================================================================

func TestTrulyRandom_Extra(t *testing.T) {
	t.Run("positive - seeded SecureRandom", func(t *testing.T) {
		findings := runRuleByName(t, "TrulyRandom", `
package test

fun init() {
    val rng = SecureRandom(byteArrayOf(1, 2, 3))
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("negative - default constructor", func(t *testing.T) {
		findings := runRuleByName(t, "TrulyRandom", `
package test

fun init() {
    val rng = SecureRandom()
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// =====================================================================
// MissingPermission tests
// =====================================================================

func TestMissingPermission_Extra(t *testing.T) {
	t.Run("positive - location API without permission check", func(t *testing.T) {
		findings := runRuleByName(t, "MissingPermission", `
package test
import android.location.LocationManager

fun track(manager: LocationManager) {
    manager.requestLocationUpdates("gps", 0, 0f, listener)
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("negative - has checkSelfPermission", func(t *testing.T) {
		findings := runRuleByName(t, "MissingPermission", `
package test
import android.location.LocationManager

fun track(manager: LocationManager) {
    if (checkSelfPermission(ACCESS_FINE_LOCATION) == GRANTED) {
        manager.requestLocationUpdates("gps", 0, 0f, listener)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("negative - issue 542 manifest package manager guard suppresses", func(t *testing.T) {
		findings := runRuleByName(t, "MissingPermission", `
package test
import android.Manifest
import android.content.pm.PackageManager
import android.location.LocationManager

fun track(manager: LocationManager) {
    if (checkSelfPermission(Manifest.permission.ACCESS_FINE_LOCATION) == PackageManager.PERMISSION_GRANTED) {
        manager.requestLocationUpdates("gps", 1000L, 1f, listener)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("positive - issue 542 candidate API call still reports", func(t *testing.T) {
		findings := runRuleByName(t, "MissingPermission", `
package test
import android.location.LocationManager

fun track(manager: LocationManager) {
    manager.requestLocationUpdates(provider, 1000L, 1f, listener)
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("positive - multiline call without guard", func(t *testing.T) {
		findings := runRuleByName(t, "MissingPermission", `
package test
import android.location.LocationManager

fun track(manager: LocationManager) {
    manager.requestLocationUpdates(
        "gps",
        0,
        0f,
        listener
    )
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("negative - different permission check does not suppress", func(t *testing.T) {
		findings := runRuleByName(t, "MissingPermission", `
package test
import android.location.LocationManager

fun track(manager: LocationManager) {
    if (checkSelfPermission(CAMERA) == GRANTED) {
        manager.requestLocationUpdates("gps", 0, 0f, listener)
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("negative - project local same name method is clean", func(t *testing.T) {
		findings := runRuleByName(t, "MissingPermission", `
package test

class Fake { fun requestLocationUpdates() {} }
fun ok(fake: Fake) = fake.requestLocationUpdates()
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("negative - project local receiver type is clean", func(t *testing.T) {
		findings := runRuleByName(t, "MissingPermission", `
package test

class LocationManager { fun requestLocationUpdates(provider: String, minTime: Long, minDistance: Float, listener: Any) {} }
fun ok(manager: LocationManager) = manager.requestLocationUpdates("gps", 0L, 0f, listener)
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("negative - comments and strings do not affect detection", func(t *testing.T) {
		findings := runRuleByName(t, "MissingPermission", `
package test

fun ok() {
    // locationManager.requestLocationUpdates("gps", 0, 0f, listener)
    val text = "checkSelfPermission ACCESS_FINE_LOCATION requestLocationUpdates"
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("negative - string permission guard suppresses", func(t *testing.T) {
		findings := runRuleByName(t, "MissingPermission", `
package test
import android.location.LocationManager

fun track(manager: LocationManager) {
    if (checkSelfPermission("android.permission.ACCESS_FINE_LOCATION") == GRANTED) {
        manager.requestLocationUpdates("gps", 0, 0f, listener)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("negative - manifest permission guard suppresses", func(t *testing.T) {
		findings := runRuleByName(t, "MissingPermission", `
package test
import android.Manifest
import android.hardware.Camera

fun openCamera() {
    if (checkSelfPermission(Manifest.permission.CAMERA) == GRANTED) {
        Camera.open()
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("positive - request permission alone does not suppress", func(t *testing.T) {
		findings := runRuleByName(t, "MissingPermission", `
package test
import android.Manifest
import android.media.MediaRecorder

fun record(recorder: MediaRecorder) {
    requestPermissions(arrayOf(Manifest.permission.RECORD_AUDIO), 1)
    recorder.setAudioSource(MediaRecorder.AudioSource.MIC)
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("positive - permission granted else branch still triggers", func(t *testing.T) {
		findings := runRuleByName(t, "MissingPermission", `
package test
import android.Manifest
import android.hardware.Camera

fun openCamera() {
    if (checkSelfPermission(Manifest.permission.CAMERA) == PERMISSION_GRANTED) {
        println("safe branch")
    } else {
        Camera.open()
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("negative - denied early return guard suppresses", func(t *testing.T) {
		findings := runRuleByName(t, "MissingPermission", `
package test
import android.Manifest
import android.hardware.Camera

fun openCamera() {
    if (checkSelfPermission(Manifest.permission.CAMERA) != PERMISSION_GRANTED) {
        return
    }
    Camera.open()
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("positive - nested denied return does not guard outer call", func(t *testing.T) {
		findings := runRuleByName(t, "MissingPermission", `
package test
import android.Manifest
import android.hardware.Camera

fun openCamera(enabled: Boolean) {
    if (enabled) {
        if (checkSelfPermission(Manifest.permission.CAMERA) != PERMISSION_GRANTED) {
            return
        }
    }
    Camera.open()
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("positive - annotated wrapper without guard triggers", func(t *testing.T) {
		findings := runRuleByName(t, "MissingPermission", `
package test
import android.Manifest
import androidx.annotation.RequiresPermission

@RequiresPermission(Manifest.permission.CAMERA)
fun openCameraWrapper() {}

fun open() {
    openCameraWrapper()
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("negative - annotated same-file rule does not widen unrelated callees", func(t *testing.T) {
		findings := runRuleByName(t, "MissingPermission", `
package test
import android.Manifest
import androidx.annotation.RequiresPermission

@RequiresPermission(Manifest.permission.CAMERA)
fun openCameraWrapper() {}

fun open() {
    unrelatedCall()
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("negative - annotated wrapper in different owner is unresolved", func(t *testing.T) {
		findings := runRuleByName(t, "MissingPermission", `
package test
import android.Manifest
import androidx.annotation.RequiresPermission

class CameraWrapper {
    @RequiresPermission(Manifest.permission.CAMERA)
    fun openCameraWrapper() {}
}

class Screen {
    fun open() {
        openCameraWrapper()
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// =====================================================================
// WrongConstant tests
// =====================================================================

func TestWrongConstant_Extra(t *testing.T) {
	t.Run("positive - raw integer in setVisibility", func(t *testing.T) {
		findings := runRuleByName(t, "WrongConstant", wrongConstantFixture(`
fun hide(view: View) {
    view.setVisibility(0)
}
`))
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("negative - named constant in setVisibility", func(t *testing.T) {
		findings := runRuleByName(t, "WrongConstant", wrongConstantFixture(`
fun hide(view: View) {
    view.setVisibility(View.VISIBLE)
}
`))
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("positive - multiline raw integer in setLayoutDirection", func(t *testing.T) {
		findings := runRuleByName(t, "WrongConstant", wrongConstantFixture(`
fun configure(view: View) {
    view.setLayoutDirection(
        1
    )
}
`))
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Line == 0 || findings[0].Col == 0 {
			t.Fatalf("expected precise literal location, got line=%d col=%d", findings[0].Line, findings[0].Col)
		}
	})
	t.Run("positive - invalid local constant with annotated same-file API", func(t *testing.T) {
		findings := runRuleByName(t, "WrongConstant", wrongConstantFixture(`
const val BAD_VISIBILITY = 3

fun hide(view: View) {
    view.setVisibility(BAD_VISIBILITY)
}
`))
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("negative - valid local constant is clean", func(t *testing.T) {
		findings := runRuleByName(t, "WrongConstant", wrongConstantFixture(`
const val LOCAL_VISIBLE = 0

fun hide(view: View) {
    view.setVisibility(LOCAL_VISIBLE)
}
`))
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("negative - unrelated same-name method without annotation is clean", func(t *testing.T) {
		findings := runRuleByName(t, "WrongConstant", `
package test

class Fake {
    fun setVisibility(value: Int) {}
}

fun hide(fake: Fake) {
    fake.setVisibility(0)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("negative - strings and unresolved targets are clean", func(t *testing.T) {
		findings := runRuleByName(t, "WrongConstant", `
package test

fun hide() {
    val sample = "view.setVisibility(0)"
    view.setVisibility(0)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// =====================================================================
// Instantiatable tests
// =====================================================================

func TestInstantiatable_Extra(t *testing.T) {
	t.Run("positive - private constructor Activity", func(t *testing.T) {
		findings := runRuleByName(t, "Instantiatable", `
package test

class MyActivity private constructor() : Activity() {
    override fun onCreate(savedInstanceState: Bundle?) {}
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("negative - public constructor Activity", func(t *testing.T) {
		findings := runRuleByName(t, "Instantiatable", `
package test

class MyActivity : Activity() {
    override fun onCreate(savedInstanceState: Bundle?) {}
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// =====================================================================
// RtlAware tests
// =====================================================================

func TestRtlAware_Extra(t *testing.T) {
	t.Run("positive - getLeft call", func(t *testing.T) {
		findings := runRuleByName(t, "RtlAware", `
package test

fun measure(v: View) {
    val x = v.getLeft()
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("negative - getStart call", func(t *testing.T) {
		findings := runRuleByName(t, "RtlAware", `
package test

fun measure(v: View) {
    val x = v.getStart()
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// =====================================================================
// RtlFieldAccess tests
// =====================================================================

func TestRtlFieldAccess_Extra(t *testing.T) {
	t.Run("positive - mLeft string literal", func(t *testing.T) {
		findings := runRuleByName(t, "RtlFieldAccess", `
package test

fun hack(v: View) {
    val f = v.javaClass.getDeclaredField("mLeft")
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("negative - mStart string literal", func(t *testing.T) {
		findings := runRuleByName(t, "RtlFieldAccess", `
package test

fun hack(v: View) {
    val f = v.javaClass.getDeclaredField("mStart")
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// =====================================================================
// GridLayout tests
// =====================================================================

func TestGridLayout_Extra(t *testing.T) {
	t.Run("positive - GridLayout without columnCount", func(t *testing.T) {
		findings := runRuleByName(t, "GridLayout", `
package test

fun build(ctx: Context) {
    val grid = GridLayout(ctx)
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("negative - GridLayout with columnCount", func(t *testing.T) {
		findings := runRuleByName(t, "GridLayout", `
package test

fun build(ctx: Context) {
    val grid = GridLayout(ctx)
    grid.columnCount = 3
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// =====================================================================
// LocaleFolder tests
// =====================================================================

func TestLocaleFolder_Extra(t *testing.T) {
	t.Run("positive - underscore locale folder", func(t *testing.T) {
		findings := runRuleByName(t, "LocaleFolder", `
package test

val path = "res/values-en_US/strings.xml"
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("negative - correct locale folder", func(t *testing.T) {
		findings := runRuleByName(t, "LocaleFolder", `
package test

val path = "res/values-en-rUS/strings.xml"
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// =====================================================================
// UseAlpha2 tests
// =====================================================================

func TestUseAlpha2_Extra(t *testing.T) {
	t.Run("positive - 3-letter code", func(t *testing.T) {
		findings := runRuleByName(t, "UseAlpha2", `
package test

val path = "res/values-eng/strings.xml"
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("negative - 2-letter code", func(t *testing.T) {
		findings := runRuleByName(t, "UseAlpha2", `
package test

val path = "res/values-en/strings.xml"
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// =====================================================================
// MangledCRLF tests
// =====================================================================

func TestMangledCRLF_Extra(t *testing.T) {
	t.Run("positive - mixed line endings", func(t *testing.T) {
		findings := runRuleByName(t, "MangledCRLF", "package test\r\nval a = 1\nval b = 2\n")
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("negative - consistent LF", func(t *testing.T) {
		findings := runRuleByName(t, "MangledCRLF", "package test\nval a = 1\nval b = 2\n")
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// =====================================================================
// ResourceName tests
// =====================================================================

func TestResourceName_Extra(t *testing.T) {
	t.Run("positive - camelCase resource name", func(t *testing.T) {
		findings := runRuleByName(t, "ResourceName", `
package test

fun load() {
    val id = R.layout.myLayout
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("negative - snake_case resource name", func(t *testing.T) {
		findings := runRuleByName(t, "ResourceName", `
package test

fun load() {
    val id = R.layout.my_layout
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// =====================================================================
// Proguard tests
// =====================================================================

func TestProguard_Extra(t *testing.T) {
	t.Run("positive - obsolete proguard.cfg", func(t *testing.T) {
		findings := runRuleByName(t, "Proguard", `
package test

val config = "proguard.cfg"
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("negative - proguard-rules.pro", func(t *testing.T) {
		findings := runRuleByName(t, "Proguard", `
package test

val config = "proguard-rules.pro"
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// =====================================================================
// ProguardSplit tests
// =====================================================================

func TestProguardSplit_Extra(t *testing.T) {
	t.Run("positive - mixed generic and specific rules", func(t *testing.T) {
		findings := runRuleByName(t, "ProguardSplit", `
package test

val rules = """
-dontobfuscate
-keep class com.example.MyClass
"""
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("negative - only generic rules", func(t *testing.T) {
		findings := runRuleByName(t, "ProguardSplit", `
package test

val rules = """
-dontobfuscate
-dontwarn
"""
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// =====================================================================
// NfcTechWhitespace tests
// =====================================================================

func TestNfcTechWhitespace_Extra(t *testing.T) {
	t.Run("positive - whitespace in tech element", func(t *testing.T) {
		findings := runRuleByName(t, "NfcTechWhitespace", `
package test

val tech = "<tech> android.nfc.tech.NfcA</tech>"
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("negative - no whitespace in tech element", func(t *testing.T) {
		findings := runRuleByName(t, "NfcTechWhitespace", `
package test

val tech = "<tech>android.nfc.tech.NfcA</tech>"
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// =====================================================================
// LibraryCustomView tests
// =====================================================================

func TestLibraryCustomView_Extra(t *testing.T) {
	t.Run("positive - hardcoded namespace", func(t *testing.T) {
		findings := runRuleByName(t, "LibraryCustomView", `
package test

val ns = "http://schemas.android.com/apk/res/com.mylib"
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("negative - res-auto namespace", func(t *testing.T) {
		findings := runRuleByName(t, "LibraryCustomView", `
package test

val ns = "http://schemas.android.com/apk/res-auto"
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// =====================================================================
// UnknownIdInLayout tests
// =====================================================================

func TestUnknownIdInLayout_Extra(t *testing.T) {
	t.Run("positive - suspicious ID with underscore prefix", func(t *testing.T) {
		findings := runRuleByName(t, "UnknownIdInLayout", `
package test

fun bind() {
    val v = findViewById(R.id._hidden)
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("negative - normal ID", func(t *testing.T) {
		findings := runRuleByName(t, "UnknownIdInLayout", `
package test

fun bind() {
    val v = findViewById(R.id.my_button)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}
