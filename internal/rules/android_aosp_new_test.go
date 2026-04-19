package rules_test

import (
	"strings"
	"testing"

	v2rules "github.com/kaeawc/krit/internal/rules/v2"
)

// ---------------------------------------------------------------------------
// TrulyRandom
// ---------------------------------------------------------------------------

func TestTrulyRandom(t *testing.T) {
	t.Run("hardcoded seed triggers", func(t *testing.T) {
		findings := runRuleByName(t, "TrulyRandom", `
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
		findings := runRuleByName(t, "TrulyRandom", `
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
		findings := runRuleByName(t, "TrulyRandom", `
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
		findings := runRuleByName(t, "TrulyRandom", `
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
fun startTracking() {
    locationManager.requestLocationUpdates("gps", 1000, 10f, listener)
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("getLastKnownLocation without check triggers", func(t *testing.T) {
		findings := runRuleByName(t, "MissingPermission", `
package test
fun getLocation() {
    val loc = locationManager.getLastKnownLocation("gps")
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("with checkSelfPermission is clean", func(t *testing.T) {
		findings := runRuleByName(t, "MissingPermission", `
package test
fun startTracking() {
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
fun startTracking() {
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
fun preparePermissions() {
    if (checkSelfPermission(ACCESS_FINE_LOCATION) == PERMISSION_GRANTED) {
        println("ready")
    }
}
fun startTracking() {
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
		findings := runRuleByName(t, "WrongConstant", `
package test
fun example() {
    view.setVisibility(0)
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("setVisibility with constant is clean", func(t *testing.T) {
		findings := runRuleByName(t, "WrongConstant", `
package test
fun example() {
    view.setVisibility(View.VISIBLE)
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("setOrientation with literal triggers", func(t *testing.T) {
		findings := runRuleByName(t, "WrongConstant", `
package test
fun example() {
    layout.setOrientation(1)
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("setGravity with literal triggers", func(t *testing.T) {
		findings := runRuleByName(t, "WrongConstant", `
package test
fun example() {
    textView.setGravity(17)
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("comment line is clean", func(t *testing.T) {
		findings := runRuleByName(t, "WrongConstant", `
package test
// view.setVisibility(0)
fun example() {}`)
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
fun example() {
    val grid = GridLayout(context)
    grid.columnCount = 3
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no GridLayout is clean", func(t *testing.T) {
		findings := runRuleByName(t, "GridLayout", `
package test
fun example() {
    val layout = LinearLayout(context)
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// LocaleFolder
// ---------------------------------------------------------------------------

func TestLocaleFolder(t *testing.T) {
	t.Run("underscore locale triggers", func(t *testing.T) {
		findings := runRuleByName(t, "LocaleFolder", `
package test
val path = "res/values-en_US/strings.xml"
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("correct locale format is clean", func(t *testing.T) {
		findings := runRuleByName(t, "LocaleFolder", `
package test
val path = "res/values-en-rUS/strings.xml"
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("simple locale is clean", func(t *testing.T) {
		findings := runRuleByName(t, "LocaleFolder", `
package test
val path = "res/values-en/strings.xml"
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// UseAlpha2
// ---------------------------------------------------------------------------

func TestUseAlpha2(t *testing.T) {
	t.Run("3-letter code triggers", func(t *testing.T) {
		findings := runRuleByName(t, "UseAlpha2", `
package test
val path = "res/values-eng/strings.xml"
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "en") {
			t.Fatalf("expected message to suggest 'en', got %s", findings[0].Message)
		}
	})

	t.Run("2-letter code is clean", func(t *testing.T) {
		findings := runRuleByName(t, "UseAlpha2", `
package test
val path = "res/values-en/strings.xml"
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("unknown 3-letter code is clean", func(t *testing.T) {
		findings := runRuleByName(t, "UseAlpha2", `
package test
val path = "res/values-xyz/strings.xml"
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings (unknown code), got %d", len(findings))
		}
	})

	t.Run("Japanese code triggers", func(t *testing.T) {
		findings := runRuleByName(t, "UseAlpha2", `
package test
val path = "res/values-jpn/strings.xml"
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "ja") {
			t.Fatalf("expected message to suggest 'ja', got %s", findings[0].Message)
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
	t.Run("proguard.cfg reference triggers", func(t *testing.T) {
		findings := runRuleByName(t, "Proguard", `
package test
fun getProguardFile(): String {
    return "proguard.cfg"
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("proguard-rules.pro is clean", func(t *testing.T) {
		findings := runRuleByName(t, "Proguard", `
package test
fun getProguardFile(): String {
    return "proguard-rules.pro"
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("comment is clean", func(t *testing.T) {
		findings := runRuleByName(t, "Proguard", `
package test
// Migrated from proguard.cfg
fun example() {}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
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
	t.Run("leading whitespace in tech triggers", func(t *testing.T) {
		findings := runRuleByName(t, "NfcTechWhitespace", `
package test
val xml = "<tech> android.nfc.tech.Ndef</tech>"
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("trailing whitespace in tech triggers", func(t *testing.T) {
		findings := runRuleByName(t, "NfcTechWhitespace", `
package test
val xml = "<tech>android.nfc.tech.Ndef </tech>"
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("no tech element is clean", func(t *testing.T) {
		findings := runRuleByName(t, "NfcTechWhitespace", `
package test
val xml = "<tag>value</tag>"
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// LibraryCustomView
// ---------------------------------------------------------------------------

func TestLibraryCustomView(t *testing.T) {
	t.Run("hardcoded namespace triggers", func(t *testing.T) {
		findings := runRuleByName(t, "LibraryCustomView", `
package test
val ns = "http://schemas.android.com/apk/res/com.example.mylib"
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("res-auto is clean", func(t *testing.T) {
		findings := runRuleByName(t, "LibraryCustomView", `
package test
val ns = "http://schemas.android.com/apk/res-auto"
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("android namespace is clean", func(t *testing.T) {
		findings := runRuleByName(t, "LibraryCustomView", `
package test
val ns = "http://schemas.android.com/apk/res/android"
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("comment is clean", func(t *testing.T) {
		findings := runRuleByName(t, "LibraryCustomView", `
package test
// Use http://schemas.android.com/apk/res/com.example instead of hardcoded
fun example() {}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// UnknownIdInLayout
// ---------------------------------------------------------------------------

func TestUnknownIdInLayout(t *testing.T) {
	t.Run("suspicious double underscore id triggers", func(t *testing.T) {
		findings := runRuleByName(t, "UnknownIdInLayout", `
package test
fun example() {
    val view = findViewById(R.id.__bad_id)
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("leading underscore id triggers", func(t *testing.T) {
		findings := runRuleByName(t, "UnknownIdInLayout", `
package test
fun example() {
    val view = findViewById(R.id._private_id)
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("normal id is clean", func(t *testing.T) {
		findings := runRuleByName(t, "UnknownIdInLayout", `
package test
fun example() {
    val view = findViewById(R.id.my_button)
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("comment is clean", func(t *testing.T) {
		findings := runRuleByName(t, "UnknownIdInLayout", `
package test
// R.id.__old should not be used
fun example() {}`)
		if len(findings) != 0 {
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
		for _, r := range v2rules.Registry {
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
