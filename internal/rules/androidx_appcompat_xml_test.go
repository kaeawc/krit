package rules_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// appCompatProfile is a project that declares appcompat with the given minSdk.
func appCompatProfile(minSdk int) librarymodel.ProjectProfile {
	return librarymodel.ProjectProfile{
		HasGradle: true, DependencyExtractionComplete: true, MinSdkVersion: minSdk,
		Dependencies: []librarymodel.Dependency{{Group: "androidx.appcompat", Name: "appcompat", Version: "1.8.0"}},
	}
}

// appCompatResourceFindings writes files under a res directory and runs a resource rule with the profile's library
// facts, returning sorted "path:line:col message" entries relative to res.
func appCompatResourceFindings(t *testing.T, ruleID string, profile librarymodel.ProjectProfile, files map[string]string) []string {
	t.Helper()
	resDir := filepath.Join(t.TempDir(), "res")
	for name, body := range files {
		path := filepath.Join(resDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	idx, err := android.ScanResourceDir(resDir)
	if err != nil {
		t.Fatal(err)
	}
	rule := findResourceRule(t, ruleID)
	ctx := &api.Context{ResourceIndex: idx, Rule: rule, Collector: scanner.NewFindingCollector(0),
		LibraryFacts: librarymodel.FactsForProfile(profile)}
	rule.Check(ctx)
	var out []string
	for _, f := range api.ContextFindings(ctx) {
		rel, _ := filepath.Rel(resDir, f.File)
		out = append(out, fmt.Sprintf("%s:%d:%d %s", filepath.ToSlash(rel), f.Line, f.Col, f.Message))
	}
	sort.Strings(out)
	return out
}

const appCompatSelector = `<?xml version="1.0" encoding="utf-8"?>
<selector xmlns:android="http://schemas.android.com/apk/res/android" xmlns:app="http://schemas.android.com/apk/res-auto">
    <item android:color="#f00" app:alpha="0.5"/>
    <item android:color="#0f0" app:alpha="0.5" android:alpha="0.5"/>
    <item app:alpha="0.5"/>
    <selector><item android:color="#00f" app:alpha="0.3"/></selector>
</selector>
`

const appCompatLayout = `<?xml version="1.0" encoding="utf-8"?>
<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android" xmlns:a2="http://schemas.android.com/apk/res/android"
    xmlns:app="http://schemas.android.com/apk/res-auto" xmlns:tools="http://schemas.android.com/tools"
    android:layout_width="match_parent" android:layout_height="match_parent">
    <ImageView a2:tint="#fff" />
    <ImageButton android:tint="#fff" />
    <androidx.appcompat.widget.AppCompatImageView android:tint="#fff" />
    <android.widget.Switch />
    <Switch android:onClick="@{() -> vm.go()}" />
    <Button app:onClick="go" />
    <Button tools:onClick="go" />
    <Button android:onClick="" />
    <TextView android:drawableLeft="@drawable/d" android:drawableTint="#fff" android:drawableTintMode="src_in" android:drawableEnd="@drawable/d" />
    <android.widget.TextView android:drawableLeft="@drawable/d" />
    <Button android:drawableLeft="@drawable/d" />
    <selector><item android:color="#00f" app:alpha="0.3"/></selector>
    <ImageView android:tint="#fff" tools:ignore="UseAppTint" />
</LinearLayout>
`

// Expected positions and messages come from running the lint.jar of androidx.appcompat:appcompat 1.8.0 through the
// Android Lint CLI on the same files with minSdk 21.
func TestAppCompatXMLLintParity(t *testing.T) {
	files := map[string]string{
		"color/c.xml":    appCompatSelector,
		"drawable/d.xml": appCompatSelector,
		"xml/x.xml":      appCompatSelector,
		"layout/l.xml":   appCompatLayout,
	}
	alpha := "Must use `android:alpha` if `app:alpha` is used"
	want := map[string][]string{
		// Lint reports the nested selector's item twice at the same position (once per enclosing <selector>); krit
		// reports it once.
		"UseAndroidAlpha": {
			"color/c.xml:3:32 " + alpha, "color/c.xml:6:42 " + alpha,
			"drawable/d.xml:3:32 " + alpha, "drawable/d.xml:6:42 " + alpha,
			"layout/l.xml:16:42 " + alpha,
			"xml/x.xml:3:32 " + alpha, "xml/x.xml:6:42 " + alpha,
		},
		"UseAppTint": {
			"layout/l.xml:5:16 Must use `app:tint` instead of `android:tint`",
			"layout/l.xml:6:18 Must use `app:tint` instead of `android:tint`",
		},
		"UseSwitchCompatOrMaterialXml": {
			"layout/l.xml:9:5 Use `SwitchCompat` from AppCompat or `MaterialSwitch` from Material library",
		},
		"UseCompatTextViewDrawableXml": {
			"layout/l.xml:13:112 Use `app:drawableEndCompat` instead of `android:drawableEnd`",
			"layout/l.xml:13:15 Use `app:drawableLeftCompat` instead of `android:drawableLeft`",
			"layout/l.xml:13:50 Use `app:drawableTint` instead of `android:drawableTint`",
			"layout/l.xml:13:78 Use `app:drawableTintMode` instead of `android:drawableTintMode`",
		},
		"UsingOnClickInXml": {
			"layout/l.xml:10:13 Use databinding or explicit wiring of click listener in code",
			"layout/l.xml:11:13 Use databinding or explicit wiring of click listener in code",
			"layout/l.xml:12:13 Use databinding or explicit wiring of click listener in code",
		},
	}
	for rule, expected := range want {
		t.Run(rule, func(t *testing.T) {
			if got := appCompatResourceFindings(t, rule, appCompatProfile(21), files); !reflect.DeepEqual(got, expected) {
				t.Fatalf("findings:\n got  %q\n want %q", got, expected)
			}
		})
	}
}

// The checks ship in the appcompat AAR, so a project that does not declare appcompat (or Material, which brings it)
// gets none of them.
func TestAppCompatXMLChecksNeedAppCompat(t *testing.T) {
	files := map[string]string{"color/c.xml": appCompatSelector, "layout/l.xml": appCompatLayout}
	noAppCompat := librarymodel.ProjectProfile{HasGradle: true, DependencyExtractionComplete: true, MinSdkVersion: 21,
		Dependencies: []librarymodel.Dependency{{Group: "androidx.core", Name: "core", Version: "1.13.0"}}}
	material := librarymodel.ProjectProfile{HasGradle: true, MinSdkVersion: 21,
		Dependencies: []librarymodel.Dependency{{Group: "com.google.android.material", Name: "material", Version: "1.12.0"}}}
	for _, rule := range []string{"UseAndroidAlpha", "UseAppTint", "UseSwitchCompatOrMaterialXml", "UseCompatTextViewDrawableXml", "UsingOnClickInXml"} {
		if got := appCompatResourceFindings(t, rule, noAppCompat, files); len(got) != 0 {
			t.Errorf("%s without appcompat: got %q, want none", rule, got)
		}
		if got := appCompatResourceFindings(t, rule, material, files); len(got) == 0 {
			t.Errorf("%s with material: got no findings", rule)
		}
	}
}

// Lint reports android:onClick only below API 23. When the build does not show minSdk, Lint assumes API 1 and
// reports; krit abstains, since a minSdk set by a convention plugin is invisible to it.
func TestUsingOnClickInXmlMinSdk(t *testing.T) {
	files := map[string]string{"layout/l.xml": appCompatLayout}
	for _, tc := range []struct {
		minSdk int
		want   int
	}{{21, 3}, {22, 3}, {23, 0}, {26, 0}, {0, 0}} {
		if got := appCompatResourceFindings(t, "UsingOnClickInXml", appCompatProfile(tc.minSdk), files); len(got) != tc.want {
			t.Errorf("minSdk %d: got %d findings %q, want %d", tc.minSdk, len(got), got, tc.want)
		}
	}
}

// tools:ignore on the item or any ancestor silences UseAndroidAlpha, and a selector item without android:color is
// not a color state list entry.
func TestUseAndroidAlphaNegatives(t *testing.T) {
	files := map[string]string{"color/c.xml": `<selector xmlns:android="http://schemas.android.com/apk/res/android" xmlns:app="http://schemas.android.com/apk/res-auto" xmlns:tools="http://schemas.android.com/tools">
    <item android:color="#f00" app:alpha="0.5" tools:ignore="UseAndroidAlpha"/>
    <item android:drawable="@drawable/x" app:alpha="0.5"/>
    <item android:color="#f00" android:alpha="0.5"/>
</selector>`, "values/v.xml": `<resources><item name="a" type="color">#fff</item></resources>`}
	if got := appCompatResourceFindings(t, "UseAndroidAlpha", appCompatProfile(21), files); len(got) != 0 {
		t.Fatalf("got %q, want none", got)
	}
}
