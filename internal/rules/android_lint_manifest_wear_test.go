package rules_test

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/pipeline"
)

var wearManifestRuleIDs = []string{
	"InvalidWearFeatureAttribute", "WearStandaloneAppFlag", "WearRecents", "WearSplashScreen", "WatchFaceEditor",
	"WatchFaceFormatDeclaresHasNoCode", "WatchFaceFormatMissingVersion", "WatchFaceFormatInvalidVersion",
}

// wearProjectFindings writes the project files and runs every Wear check on it, returning "RuleID path:line:col".
func wearProjectFindings(t *testing.T, files map[string]string) []string {
	t.Helper()
	return wearFindingsAt(t, files, "AndroidManifest.xml", "res")
}

// wearFindingsAt is wearProjectFindings for a manifest and a res directory at the given paths inside the project.
func wearFindingsAt(t *testing.T, files map[string]string, manifestRel, resRel string) []string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out []string
	add := func(rule, file string, line, col int) {
		rel, _ := filepath.Rel(dir, file)
		out = append(out, rule+" "+filepath.ToSlash(rel)+":"+strconv.Itoa(line)+":"+strconv.Itoa(col))
	}
	manifestPath := filepath.Join(dir, manifestRel)
	parsed, err := android.ParseManifestBytes([]byte(files[manifestRel]))
	if err != nil {
		t.Fatal(err)
	}
	m := pipeline.ConvertManifestForRules(android.ConvertManifest(parsed, manifestPath))
	for _, id := range wearManifestRuleIDs {
		for _, f := range runManifestRule(findManifestRule(t, id), m) {
			add(id, f.File, f.Line, f.Col)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, resRel)); err == nil {
		idx, err := android.ScanResourceDir(filepath.Join(dir, resRel))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range runResourceRule(findResourceRule(t, "WearPasswordInput"), idx) {
			add("WearPasswordInput", f.File, f.Line, f.Col)
		}
	}
	sort.Strings(out)
	return out
}

// Expected findings come from running Android Lint 32.4.1 on the same projects.
func TestWearChecksMatchLint(t *testing.T) {
	for _, c := range wearLintCases {
		t.Run(c.name, func(t *testing.T) {
			got := wearProjectFindings(t, c.files)
			want := append([]string(nil), c.want...)
			sort.Strings(want)
			if len(got) == 0 && len(want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("findings differ from Lint\n got: %q\nwant: %q", got, want)
			}
		})
	}
}

const wearModuleManifest = `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example.synth">
  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="33"/>
  <uses-feature android:name="android.hardware.type.watch"/>
  <application android:label="x">
    <meta-data android:name="com.google.android.wearable.standalone" android:value="true"/>
    <activity android:name=".SplashActivity" android:exported="true" android:taskAffinity="">
      <intent-filter>
        <action android:name="android.intent.action.MAIN"/>
        <category android:name="android.intent.category.LAUNCHER"/>
      </intent-filter>
    </activity>
  </application>
</manifest>
`

func TestWearSplashScreenSkipsModulesUsingTheLibrary(t *testing.T) {
	manifestRel := "app/src/main/AndroidManifest.xml"
	files := func(build string) map[string]string {
		return map[string]string{manifestRel: wearModuleManifest, "app/build.gradle.kts": build}
	}
	got := wearFindingsAt(t, files(`plugins { id("com.android.application") }`), manifestRel, "app/src/main/res")
	if want := []string{"WearSplashScreen app/src/main/AndroidManifest.xml:6:5"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("without the library: got %q, want %q", got, want)
	}
	got = wearFindingsAt(t, files(`dependencies { implementation("androidx.core:core-splashscreen:1.0.1") }`), manifestRel, "app/src/main/res")
	if len(got) != 0 {
		t.Fatalf("with the library: got %q, want none", got)
	}
}

func TestWearStandaloneAppFlagSkipsLibraryModules(t *testing.T) {
	manifestRel := "wear/src/main/AndroidManifest.xml"
	noFlag := strings.Replace(wearModuleManifest, `    <meta-data android:name="com.google.android.wearable.standalone" android:value="true"/>
`, "", 1)
	files := map[string]string{manifestRel: noFlag, "wear/build.gradle.kts": `plugins { id("com.android.library") }`}
	for _, f := range wearFindingsAt(t, files, manifestRel, "wear/src/main/res") {
		if strings.HasPrefix(f, "WearStandaloneAppFlag") {
			t.Fatalf("library module reported: %q", f)
		}
	}
	files["wear/build.gradle.kts"] = `plugins { id("com.android.application") }`
	got := wearFindingsAt(t, files, manifestRel, "wear/src/main/res")
	if want := "WearStandaloneAppFlag wear/src/main/AndroidManifest.xml:4:4"; !slices.Contains(got, want) {
		t.Fatalf("application module: got %q, want %q", got, want)
	}
}

func TestWearChecksFollowTheMainManifestIntoVariantSourceSets(t *testing.T) {
	debugManifest := `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example.synth">
  <application>
    <activity android:name=".Debug" android:exported="true"/>
  </application>
</manifest>
`
	files := map[string]string{
		"app/src/main/AndroidManifest.xml":  wearModuleManifest,
		"app/src/debug/AndroidManifest.xml": debugManifest,
		"app/src/debug/res/layout/pw.xml": `<EditText xmlns:android="http://schemas.android.com/apk/res/android"
    android:layout_width="wrap_content" android:layout_height="wrap_content"
    android:inputType="textPassword"/>
`,
	}
	got := wearFindingsAt(t, files, "app/src/debug/AndroidManifest.xml", "app/src/debug/res")
	want := []string{
		"WearPasswordInput app/src/debug/res/layout/pw.xml:3:5",
		"WearRecents app/src/debug/AndroidManifest.xml:3:5",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}
