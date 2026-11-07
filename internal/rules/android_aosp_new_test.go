package rules_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const wrongConstantAnnotatedApi = `
annotation class IntDef(vararg val value: Int, val flag: Boolean = false)

@IntDef(View.VISIBLE, View.INVISIBLE, View.GONE)
annotation class Visibility

@IntDef(View.LAYOUT_DIRECTION_LTR, View.LAYOUT_DIRECTION_RTL, View.LAYOUT_DIRECTION_INHERIT, View.LAYOUT_DIRECTION_LOCALE)
annotation class LayoutDirection

@IntDef(View.IMPORTANT_FOR_ACCESSIBILITY_AUTO, View.IMPORTANT_FOR_ACCESSIBILITY_YES, View.IMPORTANT_FOR_ACCESSIBILITY_NO, View.IMPORTANT_FOR_ACCESSIBILITY_NO_HIDE_DESCENDANTS)
annotation class ImportantForAccessibility

@IntDef(LinearLayout.HORIZONTAL, LinearLayout.VERTICAL)
annotation class Orientation

@IntDef(Gravity.LEFT, Gravity.RIGHT, Gravity.TOP, Gravity.BOTTOM, Gravity.CENTER, flag = true)
annotation class GravityInt

open class View {
    companion object {
        const val VISIBLE = 0
        const val INVISIBLE = 4
        const val GONE = 8
        const val LAYOUT_DIRECTION_LTR = 0
        const val LAYOUT_DIRECTION_RTL = 1
        const val LAYOUT_DIRECTION_INHERIT = 2
        const val LAYOUT_DIRECTION_LOCALE = 3
        const val IMPORTANT_FOR_ACCESSIBILITY_AUTO = 0
        const val IMPORTANT_FOR_ACCESSIBILITY_YES = 1
        const val IMPORTANT_FOR_ACCESSIBILITY_NO = 2
        const val IMPORTANT_FOR_ACCESSIBILITY_NO_HIDE_DESCENDANTS = 4
    }

    fun setVisibility(@Visibility value: Int) {}
    fun setLayoutDirection(@LayoutDirection value: Int) {}
    fun setImportantForAccessibility(@ImportantForAccessibility value: Int) {}
}

object Gravity {
    const val LEFT = 3
    const val RIGHT = 5
    const val TOP = 48
    const val BOTTOM = 80
    const val CENTER = 17
}

class TextView : View() {
    fun setGravity(@GravityInt value: Int) {}
}

class LinearLayout : View() {
    companion object {
        const val HORIZONTAL = 0
        const val VERTICAL = 1
    }

    fun setOrientation(@Orientation value: Int) {}
}
`

func wrongConstantFixture(body string) string {
	return "package test\n" + wrongConstantAnnotatedApi + body
}

// ---------------------------------------------------------------------------
// HardcodedRandomSeed
// ---------------------------------------------------------------------------

func TestHardcodedRandomSeed(t *testing.T) {
	t.Run("hardcoded seed triggers", func(t *testing.T) {
		findings := runRuleByName(t, "HardcodedRandomSeed", `
package test
import java.security.SecureRandom
fun example() {
    val rng = SecureRandom(byteArrayOf(1, 2, 3))
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("string seed triggers", func(t *testing.T) {
		findings := runRuleByName(t, "HardcodedRandomSeed", `
package test
import java.security.SecureRandom
fun example() {
    val rng = SecureRandom("seed".toByteArray())
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("default constructor is clean", func(t *testing.T) {
		findings := runRuleByName(t, "HardcodedRandomSeed", `
package test
import java.security.SecureRandom
fun example() {
    val rng = SecureRandom()
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("comment is clean", func(t *testing.T) {
		findings := runRuleByName(t, "HardcodedRandomSeed", `
package test
// Don't use SecureRandom(byteArrayOf(1)) in production
fun example() {}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// MissingPermission
// ---------------------------------------------------------------------------

func TestMissingPermission(t *testing.T) {
	t.Run("requestLocationUpdates without check triggers", func(t *testing.T) {
		findings := runRuleByName(t, "MissingPermission", `
package test
import android.location.LocationManager
fun startTracking(locationManager: LocationManager) {
    locationManager.requestLocationUpdates("gps", 1000, 10f, listener)
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("call inside try catching SecurityException is clean", func(t *testing.T) {
		findings := runRuleByName(t, "MissingPermission", `
package test
import android.location.LocationManager
fun getLocation(locationManager: LocationManager) {
    try {
        locationManager.getLastKnownLocation("gps")
    } catch (e: SecurityException) {
        println(e)
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("call inside the catch block of a SecurityException try still triggers", func(t *testing.T) {
		findings := runRuleByName(t, "MissingPermission", `
package test
import android.location.LocationManager
fun getLocation(locationManager: LocationManager) {
    try {
        println("x")
    } catch (e: SecurityException) {
        locationManager.getLastKnownLocation("gps")
    }
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("getLastKnownLocation without check triggers", func(t *testing.T) {
		findings := runRuleByName(t, "MissingPermission", `
package test
import android.location.LocationManager
fun getLocation(locationManager: LocationManager) {
    val loc = locationManager.getLastKnownLocation("gps")
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("with checkSelfPermission is clean", func(t *testing.T) {
		findings := runRuleByName(t, "MissingPermission", `
package test
import android.location.LocationManager
fun startTracking(locationManager: LocationManager) {
    if (checkSelfPermission(ACCESS_FINE_LOCATION) == PERMISSION_GRANTED) {
        locationManager.requestLocationUpdates("gps", 1000, 10f, listener)
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("with ContextCompat.checkSelfPermission is clean", func(t *testing.T) {
		findings := runRuleByName(t, "MissingPermission", `
package test
import android.location.LocationManager
fun startTracking(locationManager: LocationManager) {
    if (ContextCompat.checkSelfPermission(this, ACCESS_FINE_LOCATION) == PERMISSION_GRANTED) {
        locationManager.requestLocationUpdates("gps", 1000, 10f, listener)
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("permission check in another function does not suppress later call", func(t *testing.T) {
		findings := runRuleByName(t, "MissingPermission", `
package test
import android.location.LocationManager
fun preparePermissions() {
    if (checkSelfPermission(ACCESS_FINE_LOCATION) == PERMISSION_GRANTED) {
        println("ready")
    }
}
fun startTracking(locationManager: LocationManager) {
    locationManager.requestLocationUpdates("gps", 1000, 10f, listener)
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("Camera.open without check triggers", func(t *testing.T) {
		findings := runRuleByName(t, "MissingPermission", `
package test
import android.hardware.Camera
fun openCamera() {
    val camera = Camera.open()
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("comment line does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "MissingPermission", `
package test
// Call requestLocationUpdates here
fun example() {}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// WrongConstant
// ---------------------------------------------------------------------------

func TestWrongConstant(t *testing.T) {
	t.Run("setVisibility with literal triggers", func(t *testing.T) {
		findings := runRuleByName(t, "WrongConstant", wrongConstantFixture(`
fun example() {
    val view: View = View()
    view.setVisibility(0)
}`))
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("setVisibility with constant is clean", func(t *testing.T) {
		findings := runRuleByName(t, "WrongConstant", wrongConstantFixture(`
fun example() {
    val view: View = View()
    view.setVisibility(View.VISIBLE)
}`))
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("setOrientation with literal triggers", func(t *testing.T) {
		findings := runRuleByName(t, "WrongConstant", wrongConstantFixture(`
fun example() {
    val layout: LinearLayout = LinearLayout()
    layout.setOrientation(1)
}`))
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("setGravity with literal triggers", func(t *testing.T) {
		findings := runRuleByName(t, "WrongConstant", wrongConstantFixture(`
fun example() {
    val textView: TextView = TextView()
    textView.setGravity(17)
}`))
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("comment line is clean", func(t *testing.T) {
		findings := runRuleByName(t, "WrongConstant", wrongConstantFixture(`
// view.setVisibility(0)
fun example() {}`))
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// Instantiatable
// ---------------------------------------------------------------------------

func TestInstantiatable(t *testing.T) {
	t.Run("private constructor on Activity triggers", func(t *testing.T) {
		findings := runRuleByName(t, "Instantiatable", `
package test
class MyActivity private constructor() : Activity() {
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("private class as Service triggers", func(t *testing.T) {
		findings := runRuleByName(t, "Instantiatable", `
package test
private class MyService : Service() {
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("public Activity is clean", func(t *testing.T) {
		findings := runRuleByName(t, "Instantiatable", `
package test
class MyActivity : AppCompatActivity() {
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("non-component class is clean", func(t *testing.T) {
		findings := runRuleByName(t, "Instantiatable", `
package test
class MyHelper private constructor() {
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// RtlAware
// ---------------------------------------------------------------------------

func TestRtlAware(t *testing.T) {
	t.Run("getLeft triggers", func(t *testing.T) {
		findings := runRuleByName(t, "RtlAware", `
package test
fun example(view: View) {
    val x = view.getLeft()
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("getPaddingRight triggers", func(t *testing.T) {
		findings := runRuleByName(t, "RtlAware", `
package test
fun example(view: View) {
    val p = view.getPaddingRight()
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("getStart is clean", func(t *testing.T) {
		findings := runRuleByName(t, "RtlAware", `
package test
fun example(view: View) {
    val x = view.getStart()
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("comment line is clean", func(t *testing.T) {
		findings := runRuleByName(t, "RtlAware", `
package test
// view.getLeft() should be getStart()
fun example() {}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// RtlFieldAccess
// ---------------------------------------------------------------------------

func TestRtlFieldAccess(t *testing.T) {
	t.Run("mLeft reflection triggers", func(t *testing.T) {
		findings := runRuleByName(t, "RtlFieldAccess", `
package test
fun example() {
    val field = View::class.java.getDeclaredField("mLeft")
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("mPaddingRight reflection triggers", func(t *testing.T) {
		findings := runRuleByName(t, "RtlFieldAccess", `
package test
fun example() {
    val field = View::class.java.getDeclaredField("mPaddingRight")
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("normal field access is clean", func(t *testing.T) {
		findings := runRuleByName(t, "RtlFieldAccess", `
package test
fun example() {
    val field = View::class.java.getDeclaredField("mTag")
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("comment is clean", func(t *testing.T) {
		findings := runRuleByName(t, "RtlFieldAccess", `
package test
// Don't access "mLeft" directly
fun example() {}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// GridLayout
// ---------------------------------------------------------------------------

func TestGridLayout(t *testing.T) {
	t.Run("GridLayout without columnCount triggers", func(t *testing.T) {
		findings := runRuleByName(t, "GridLayout", `
package test
import android.widget.GridLayout
fun example() {
    val grid = GridLayout(context)
    grid.addView(child)
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("GridLayout with columnCount is clean", func(t *testing.T) {
		findings := runRuleByName(t, "GridLayout", `
package test
import android.widget.GridLayout
fun example() {
    val grid = GridLayout(context)
    grid.columnCount = 3
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("unrelated columnCount does not satisfy GridLayout", func(t *testing.T) {
		findings := runRuleByName(t, "GridLayout", `
package test
import android.widget.GridLayout
fun example() {
    val grid = GridLayout(context)
    val columnCount = 3
    grid.addView(child)
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("no GridLayout is clean", func(t *testing.T) {
		findings := runRuleByName(t, "GridLayout", `
package test
import android.widget.GridLayout
fun example() {
    val layout = LinearLayout(context)
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("configured in a scope function or by setter is clean", func(t *testing.T) {
		findings := runRuleByName(t, "GridLayout", `
package test
import android.widget.GridLayout
fun example() {
    val a = GridLayout(context).apply { columnCount = 2 }
    val b = GridLayout(context)
    b.setColumnCount(2)
    val c = GridLayout(context).also { it.columnCount = 4 }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %v", findings)
		}
	})

	t.Run("GridLayout not imported from a view library is clean", func(t *testing.T) {
		findings := runRuleByName(t, "GridLayout", `
package test
fun example() {
    val grid = GridLayout(context)
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %v", findings)
		}
	})
}

// ---------------------------------------------------------------------------
// MangledCRLF
// ---------------------------------------------------------------------------

func TestMangledCRLF(t *testing.T) {
	t.Run("mixed line endings triggers", func(t *testing.T) {
		findings := runRuleByName(t, "MangledCRLF", "package test\r\nfun a() {}\nfun b() {}\r\n")
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("all LF is clean", func(t *testing.T) {
		findings := runRuleByName(t, "MangledCRLF", "package test\nfun a() {}\nfun b() {}\n")
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("all CRLF is clean", func(t *testing.T) {
		findings := runRuleByName(t, "MangledCRLF", "package test\r\nfun a() {}\r\nfun b() {}\r\n")
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// ResourceName
// ---------------------------------------------------------------------------

func TestResourceName(t *testing.T) {
	t.Run("camelCase resource name triggers", func(t *testing.T) {
		findings := runRuleByName(t, "ResourceName", `
package test
fun example() {
    val layout = R.layout.myActivityLayout
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("snake_case resource name is clean", func(t *testing.T) {
		findings := runRuleByName(t, "ResourceName", `
package test
fun example() {
    val layout = R.layout.activity_main
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("multiple bad names trigger separately", func(t *testing.T) {
		findings := runRuleByName(t, "ResourceName", `
package test
fun example() {
    val a = R.drawable.myIcon
    val b = R.string.helloWorld
}`)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d", len(findings))
		}
	})

	t.Run("comment is clean", func(t *testing.T) {
		findings := runRuleByName(t, "ResourceName", `
package test
// R.layout.myBadName is deprecated
fun example() {}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// Proguard
// ---------------------------------------------------------------------------

func TestProguard(t *testing.T) {
	r := findGradleRule(t, "Proguard")
	run := func(path, content string) []scanner.Finding {
		cfg, _ := android.ParseBuildGradleContent(content)
		return runGradleRule(r, path, content, cfg)
	}

	t.Run("proguard.cfg reference triggers in Kotlin and Groovy scripts", func(t *testing.T) {
		kts := run("build.gradle.kts", "android {\n  buildTypes { release { proguardFiles(\"proguard.cfg\") } }\n}\n")
		groovy := run("build.gradle", "android {\n  buildTypes { release { proguardFile 'proguard.cfg' } }\n}\n")
		if len(kts) != 1 || kts[0].Line != 2 || len(groovy) != 1 {
			t.Fatalf("expected one finding per script, got kts=%v groovy=%v", kts, groovy)
		}
	})

	t.Run("proguard-rules.pro and comments are clean", func(t *testing.T) {
		got := run("build.gradle.kts", "// migrated from proguard.cfg\nandroid { buildTypes { release { proguardFiles(\"proguard-rules.pro\") } } } // not proguard.cfg\n")
		if len(got) != 0 {
			t.Fatalf("expected 0 findings, got %v", got)
		}
	})

	t.Run("Kotlin sources are not reported", func(t *testing.T) {
		if got := runRuleByName(t, "Proguard", "package test\nval f = \"proguard.cfg\"\n"); len(got) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(got))
		}
	})
}

// ---------------------------------------------------------------------------
// ProguardSplit
// ---------------------------------------------------------------------------

func TestProguardSplit(t *testing.T) {
	t.Run("mixed generic and specific triggers", func(t *testing.T) {
		findings := runRuleByName(t, "ProguardSplit", `
package test
val config = """
    -dontobfuscate
    -keep class com.example.MyClass { *; }
""".trimIndent()
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("only generic rules is clean", func(t *testing.T) {
		findings := runRuleByName(t, "ProguardSplit", `
package test
val config = """
    -dontobfuscate
    -dontwarn
""".trimIndent()
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no proguard content is clean", func(t *testing.T) {
		findings := runRuleByName(t, "ProguardSplit", `
package test
fun example() {}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// NfcTechWhitespace
// ---------------------------------------------------------------------------

func TestNfcTechWhitespace(t *testing.T) {
	r := findResourceRule(t, "NfcTechWhitespace")
	run := func(t *testing.T, nfcXML string) []scanner.Finding {
		t.Helper()
		resDir := filepath.Join(t.TempDir(), "res")
		if err := os.MkdirAll(filepath.Join(resDir, "values"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(resDir, "xml"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(resDir, "values", "strings.xml"), []byte(`<resources><string name="a">A</string></resources>`), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(resDir, "xml", "nfc_tech_filter.xml"), []byte(nfcXML), 0o644); err != nil {
			t.Fatal(err)
		}
		idx, err := android.ScanResourceDir(resDir)
		if err != nil {
			t.Fatal(err)
		}
		return runResourceRule(r, idx)
	}

	t.Run("leading whitespace in tech triggers", func(t *testing.T) {
		findings := run(t, "<resources>\n  <tech-list>\n    <tech> android.nfc.tech.Ndef</tech>\n  </tech-list>\n</resources>\n")
		if len(findings) != 1 || findings[0].Line != 3 {
			t.Fatalf("expected 1 finding on line 3, got %v", findings)
		}
		if want := "There should not be any whitespace inside `<tech>` elements"; findings[0].Message != want {
			t.Fatalf("message = %q", findings[0].Message)
		}
	})

	t.Run("tech outside a tech-list is clean", func(t *testing.T) {
		if findings := run(t, "<resources><tech> android.nfc.tech.Ndef</tech></resources>"); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("trailing whitespace in tech triggers", func(t *testing.T) {
		findings := run(t, "<resources><tech-list><tech>android.nfc.tech.Ndef\n</tech></tech-list></resources>")
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("trimmed tech names and Kotlin strings are clean", func(t *testing.T) {
		findings := run(t, "<resources><tech-list><tech>android.nfc.tech.Ndef</tech></tech-list></resources>")
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
		if got := runRuleByName(t, "NfcTechWhitespace", "package test\nval xml = \"<tech> android.nfc.tech.Ndef</tech>\"\n"); len(got) != 0 {
			t.Fatalf("a Kotlin string must not be reported, got %d", len(got))
		}
	})
}

// ---------------------------------------------------------------------------
// LibraryCustomView
// ---------------------------------------------------------------------------

func TestLibraryCustomView(t *testing.T) {
	r := findResourceRule(t, "LibraryCustomView")
	layout := func(ns string) *android.View {
		return &android.View{Type: "LinearLayout", Line: 1, Attributes: map[string]string{"xmlns:app": ns}}
	}
	moduleLayoutPath := func(t *testing.T, plugin string) string {
		t.Helper()
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "build.gradle.kts"), []byte("plugins {\n    id(\""+plugin+"\")\n}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return filepath.Join(dir, "src", "main", "res", "layout", "main.xml")
	}

	t.Run("hardcoded namespace in a library layout triggers", func(t *testing.T) {
		idx := indexWithLayout("main", moduleLayoutPath(t, "com.android.library"), layout("http://schemas.android.com/apk/res/com.example.mylib"))
		if got := runResourceRule(r, idx); len(got) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(got))
		}
	})

	t.Run("hardcoded namespace in an application layout is clean", func(t *testing.T) {
		idx := indexWithLayout("main", moduleLayoutPath(t, "com.android.application"), layout("http://schemas.android.com/apk/res/com.example.mylib"))
		if got := runResourceRule(r, idx); len(got) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(got))
		}
	})

	t.Run("res-auto and android namespaces are clean", func(t *testing.T) {
		for _, ns := range []string{"http://schemas.android.com/apk/res-auto", "http://schemas.android.com/apk/res/android"} {
			idx := indexWithLayout("main", moduleLayoutPath(t, "com.android.library"), layout(ns))
			if got := runResourceRule(r, idx); len(got) != 0 {
				t.Fatalf("%s: expected 0 findings, got %d", ns, len(got))
			}
		}
	})

	t.Run("Kotlin strings are not reported", func(t *testing.T) {
		if got := runRuleByName(t, "LibraryCustomView", "package test\nval ns = \"http://schemas.android.com/apk/res/com.example.mylib\"\n"); len(got) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(got))
		}
	})
}

// ---------------------------------------------------------------------------
// UnknownIdInLayout
// ---------------------------------------------------------------------------

func TestUnknownIdInLayout(t *testing.T) {
	r := findResourceRule(t, "UnknownIdInLayout")

	build := func(views ...*android.View) *android.ResourceIndex {
		idx := emptyIndex()
		for i, root := range views {
			name := fmt.Sprintf("layout%d", i)
			idx.Layouts[name] = &android.Layout{Name: name, FilePath: "res/layout/" + name + ".xml", RootView: root}
		}
		return idx
	}
	declares := func(id string) *android.View {
		return &android.View{Type: "TextView", Line: 2, Attributes: map[string]string{"android:id": "@+id/" + id}}
	}
	references := func(id string, children ...*android.View) *android.View {
		ref := &android.View{Type: "TextView", Line: 3, Attributes: map[string]string{"android:layout_below": "@+id/" + id}}
		return &android.View{Type: "RelativeLayout", Line: 1, Attributes: map[string]string{}, Children: append(children, ref)}
	}

	t.Run("id declared only in another layout triggers", func(t *testing.T) {
		idx := build(references("title"), &android.View{Type: "LinearLayout", Line: 1, Attributes: map[string]string{}, Children: []*android.View{declares("title")}})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("id declared in the same layout is clean", func(t *testing.T) {
		idx := build(references("title", declares("title")))
		if findings := runResourceRule(r, idx); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("id declared nowhere is left to UnknownId", func(t *testing.T) {
		idx := build(references("ghost"))
		if findings := runResourceRule(r, idx); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("@id references are not judged", func(t *testing.T) {
		ref := references("title")
		ref.Children[0].Attributes["android:layout_below"] = "@id/title"
		idx := build(ref, &android.View{Type: "LinearLayout", Line: 1, Attributes: map[string]string{}, Children: []*android.View{declares("title")}})
		if findings := runResourceRule(r, idx); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("message follows Lint", func(t *testing.T) {
		idx := build(references("title"), &android.View{Type: "LinearLayout", Line: 1, Attributes: map[string]string{}, Children: []*android.View{declares("title")}})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 || findings[0].Message != "The id \"`title`\" is not referring to any views in this layout" {
			t.Fatalf("got %+v", findings)
		}
	})

	t.Run("layout with an unresolvable include is skipped", func(t *testing.T) {
		include := &android.View{Type: "include", Line: 2, Attributes: map[string]string{"layout": "@layout/header"}}
		idx := build(references("title", include), &android.View{Type: "LinearLayout", Line: 1, Attributes: map[string]string{}, Children: []*android.View{declares("title")}})
		if findings := runResourceRule(r, idx); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// Registration sanity for all new rules
// ---------------------------------------------------------------------------

func TestNewAospRulesRegistered(t *testing.T) {
	expected := []string{
		"TrulyRandom",
		"HardcodedRandomSeed",
		"MissingPermission",
		"WrongConstant",
		"Instantiatable",
		"RtlAware",
		"RtlFieldAccess",
		"GridLayout",
		"LocaleFolder",
		"UseAlpha2",
		"MangledCRLF",
		"ResourceName",
		"Proguard",
		"ProguardSplit",
		"NfcTechWhitespace",
		"LibraryCustomView",
		"UnknownIdInLayout",
	}
	for _, name := range expected {
		found := false
		for _, r := range api.Registry {
			if r.ID == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected rule %q to be registered", name)
		}
	}
}
