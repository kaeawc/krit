package rules

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/librarymodel"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// runAndroidRuleMinSdk runs one registered rule on a Kotlin source with the given project minSdk.
func runAndroidRuleMinSdk(t *testing.T, ruleID, source string, minSdk int) []scanner.Finding {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.kt")
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	file, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	for _, r := range api.Registry {
		if r.ID != ruleID {
			continue
		}
		dispatcher := NewDispatcher([]*api.Rule{r}, nil)
		dispatcher.SetLibraryFacts(librarymodel.FactsForProfile(librarymodel.ProjectProfile{MinSdkVersion: minSdk}))
		cols := dispatcher.Run(file)
		return cols.Findings()
	}
	t.Fatalf("rule %s not registered", ruleID)
	return nil
}

func TestLongLogTagSkippedWhenMinSdk24(t *testing.T) {
	src := `package test
import android.util.Log
fun foo() { Log.d("ThisIsAVeryLongTagNameThatExceeds", "msg") }
`
	if n := len(runAndroidRuleMinSdk(t, "LongLogTag", src, 21)); n != 1 {
		t.Errorf("minSdk 21: expected 1 finding, got %d", n)
	}
	if n := len(runAndroidRuleMinSdk(t, "LongLogTag", src, 24)); n != 0 {
		t.Errorf("minSdk 24: expected 0 findings, got %d", n)
	}
}

func TestLogConditionalDebugConjunctGuards(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"debug and flag", `if (BuildConfig.DEBUG && flag) { Log.d("T", "m") }`, 0},
		{"flag and debug", `if (flag && BuildConfig.DEBUG) { Log.d("T", "m") }`, 0},
		{"debug or flag", `if (BuildConfig.DEBUG || flag) { Log.d("T", "m") }`, 1},
		{"bare debug", `if (BuildConfig.DEBUG) { Log.d("T", "m") }`, 0},
		{"negated debug", `if (!BuildConfig.DEBUG) { Log.d("T", "m") }`, 1},
		{"unrelated if", `if (flag) Log.d("T", "m")`, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := "package test\nimport android.util.Log\nfun foo(flag: Boolean) {\n" + tc.body + "\n}\n"
			if n := len(runAndroidRuleMinSdk(t, "LogConditional", src, 21)); n != tc.want {
				t.Errorf("expected %d findings, got %d", tc.want, n)
			}
		})
	}
}

func TestInstantiatableAllowsPrivateDynamicReceiver(t *testing.T) {
	src := `package test
import android.content.BroadcastReceiver
private class NetworkReceiver : BroadcastReceiver() {}
class Holder private constructor() : BroadcastReceiver() {}
private class Hidden : android.app.Service() {}
`
	got := runAndroidRuleMinSdk(t, "Instantiatable", src, 21)
	if len(got) != 1 {
		t.Fatalf("expected only the private Service to report, got %d", len(got))
	}
}

func TestAndroidComponentRulesIgnoreLookalikeSupertypes(t *testing.T) {
	// A domain interface named Service, or an imported non-Android Service, is not the framework class.
	cases := map[string]string{
		"interface supertype": "package test\nclass Impl : Service {}\n",
		"foreign import":      "package test\nimport com.example.Service\nclass Impl : Service() {}\n",
		"local declaration":   "package test\nopen class Service\nclass Impl : Service() {}\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if n := len(runAndroidRuleMinSdk(t, "OverrideAbstract", src, 21)); n != 0 {
				t.Errorf("OverrideAbstract: expected 0 findings, got %d", n)
			}
			inst := "package test\n" + "private " + src[len("package test\n"):]
			if n := len(runAndroidRuleMinSdk(t, "Instantiatable", inst, 21)); n != 0 {
				t.Errorf("Instantiatable: expected 0 findings, got %d", n)
			}
		})
	}
	real := "package test\nimport android.app.Service\nclass Impl : Service() {}\n"
	if n := len(runAndroidRuleMinSdk(t, "OverrideAbstract", real, 21)); n != 1 {
		t.Errorf("framework Service without onBind: expected 1 finding, got %d", n)
	}
}

func TestResourceTypeAcceptsEquivalentResources(t *testing.T) {
	src := `package test
fun f(ctx: Context, iv: ImageView, menu: Menu) {
    menuInflater.inflate(R.menu.main, menu)
    ctx.getDrawable(R.mipmap.ic_launcher)
    iv.setImageResource(R.color.primary)
    ctx.getString(R.plurals.foo)
    layoutInflater.inflate(R.string.foo, null)
}
`
	if got := runAndroidRuleMinSdk(t, "ResourceType", src, 21); len(got) != 2 {
		t.Fatalf("expected the two genuine mismatches, got %d: %v", len(got), got)
	}
}

func TestParcelCreatorSkipsAbstractTypes(t *testing.T) {
	src := `package test
import android.os.Parcelable
interface Marker : Parcelable
sealed class Base : Parcelable
abstract class Abs : Parcelable
class Plain(val x: Int) : Parcelable {}
`
	got := runAndroidRuleMinSdk(t, "ParcelCreator", src, 21)
	if len(got) != 1 || got[0].Line != 6 {
		t.Fatalf("expected one finding for Plain on line 6, got %v", got)
	}
}

func TestIsValidAndroidIDAllowsDots(t *testing.T) {
	for id, want := range map[string]bool{
		"@+id/my.id":    true,
		"@+id/my_id":    true,
		"@+id/my-id":    false,
		"@+id/":         false,
		"@android:id/x": true,
		"plain":         false,
	} {
		if got := isValidAndroidID(id); got != want {
			t.Errorf("isValidAndroidID(%q) = %v, want %v", id, got, want)
		}
	}
}

func TestRecycleOnlyReportsOwnedLocals(t *testing.T) {
	src := `package test
class Holder {
    lateinit var held: Cursor
    fun returned(): Cursor {
        val c: Cursor = resolver.query()!!
        return c
    }
    fun alias(other: Cursor) {
        val same: Cursor = other
        same.moveToFirst()
    }
    fun closed() {
        val c: Cursor = resolver.query()!!
        c.close()
    }
    fun leaked() {
        val t: TypedArray = ctx.obtainStyledAttributes(attrs)
        t.getString(0)
    }
}
`
	got := runAndroidRuleMinSdk(t, "Recycle", src, 21)
	if len(got) != 1 || got[0].Line != 17 {
		t.Fatalf("expected one finding for the leaked TypedArray on line 17, got %v", got)
	}
}

func TestPluralsCandidateEqualsOneBoundary(t *testing.T) {
	mk := func(cond string) string {
		return "package test\nfun f(count: Int) {\n    if (" + cond + ") { label(getString(R.string.one)) }\n}\n"
	}
	if n := len(runAndroidRuleMinSdk(t, "PluralsCandidate", mk("count == 1"), 21)); n != 1 {
		t.Errorf("count == 1: expected 1 finding, got %d", n)
	}
	if n := len(runAndroidRuleMinSdk(t, "PluralsCandidate", mk("count == 10"), 21)); n != 0 {
		t.Errorf("count == 10: expected 0 findings, got %d", n)
	}
}

func TestNewApiRespectsMinSdkAndCompatWrappers(t *testing.T) {
	src := `package test
import android.content.Context
class A(val ctx: Context) {
    fun a(v: View) {
        val compat = ContextCompat.getColor(ctx, 1)
        v.setElevation(2f)
        val own = getColor(2)
    }
    fun getColor(i: Int) = i
}
`
	got := runAndroidRuleMinSdk(t, "NewApi", src, 19)
	if len(got) != 1 || got[0].Line != 6 {
		t.Fatalf("minSdk 19: expected only setElevation (line 6), got %v", got)
	}
	if got := runAndroidRuleMinSdk(t, "NewApi", src, 21); len(got) != 0 {
		t.Fatalf("minSdk 21 covers setElevation, got %v", got)
	}
}

func TestInlinedApiRespectsMinSdkAndIgnoresVersionCodes(t *testing.T) {
	src := `package test
fun f() {
    val a = PackageManager.FEATURE_LEANBACK
    val b = Build.VERSION_CODES.S
}
`
	if got := runAndroidRuleMinSdk(t, "InlinedApi", src, 16); len(got) != 1 || got[0].Line != 3 {
		t.Fatalf("minSdk 16: expected only FEATURE_LEANBACK, got %v", got)
	}
	if got := runAndroidRuleMinSdk(t, "InlinedApi", src, 21); len(got) != 0 {
		t.Fatalf("minSdk 21 covers FEATURE_LEANBACK, got %v", got)
	}
}

func TestOverrideRequiresMatchingSignature(t *testing.T) {
	src := `package test
class MyActivity : Activity() {
    fun onOptionsItemSelected(id: Int, extra: Int) {}
    fun onBackPressed(reason: String) {}
    fun onBackPressed() {}
}
`
	got := runAndroidRuleMinSdk(t, "Override", src, 21)
	if len(got) != 1 || got[0].Line != 5 {
		t.Fatalf("expected only the no-arg onBackPressed on line 5, got %v", got)
	}
}

func TestResourceNameIgnoresStyleReferences(t *testing.T) {
	src := `package test
fun f() {
    val theme = R.style.Theme_MyApp_Dark
    val s = R.string.badName
}
`
	got := runAndroidRuleMinSdk(t, "ResourceName", src, 21)
	if len(got) != 1 || got[0].Line != 4 {
		t.Fatalf("expected only R.string.badName on line 4, got %v", got)
	}
}

func TestLongLogTagCountsUTF16Characters(t *testing.T) {
	cases := map[string]int{
		strings.Repeat("a", 24):     1,
		strings.Repeat("a", 23):     0,
		strings.Repeat("é", 20):     0, // 40 bytes in UTF-8, 20 characters
		strings.Repeat(`\u00e9`, 5): 0, // 30 source bytes, 5 characters
		strings.Repeat("é", 24):     1,
		strings.Repeat("😀", 12):     1, // 24 UTF-16 code units
		strings.Repeat("😀", 11):     0,
	}
	for tag, want := range cases {
		src := "package test\nimport android.util.Log\nfun f() { Log.d(\"" + tag + "\", \"x\") }\n"
		if got := len(runAndroidRuleMinSdk(t, "LongLogTag", src, 21)); got != want {
			t.Errorf("tag %q: expected %d findings, got %d", tag, want, got)
		}
	}
}
