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
	"github.com/kaeawc/krit/internal/pipeline"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const wearBindListenerManifest = `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="p">
    <application>
        <service android:name=".Listener">
            <intent-filter>
                <action android:name="com.google.android.gms.wearable.BIND_LISTENER" />
            </intent-filter>
        </service>
        <activity android:name=".Main">
            <intent-filter>
                <action android:name="com.google.android.gms.wearable.BIND_LISTENER" />
            </intent-filter>
        </activity>
    </application>
</manifest>
`

func wearBindListenerFindings(t *testing.T, profile librarymodel.ProjectProfile) []string {
	t.Helper()
	parsed, err := android.ParseManifestBytes([]byte(wearBindListenerManifest))
	if err != nil {
		t.Fatal(err)
	}
	m := pipeline.ConvertManifestForRules(android.ConvertManifest(parsed, "AndroidManifest.xml"))
	ctx := &api.Context{Manifest: m, Rule: findManifestRule(t, "WearableBindListener"),
		Collector: scanner.NewFindingCollector(0), LibraryFacts: librarymodel.FactsForProfile(profile)}
	ctx.Rule.Check(ctx)
	var out []string
	for _, f := range api.ContextFindings(ctx) {
		out = append(out, fmt.Sprintf("%d:%d %s", f.Line, f.Col, f.Message))
	}
	return out
}

// Expected findings come from Android Lint 32.4.1's ManifestDetector through lint-tests' TestLintTask, with the
// dependency and compileSdkVersion in a mocked build.gradle (and an SDK home for the upgrade message).
func TestWearableBindListener_MatchesLint(t *testing.T) {
	const deprecated = "5:25 The `com.google.android.gms.wearable.BIND_LISTENER` action is deprecated"
	const upgrade = deprecated + ". Please upgrade to the latest version of play-services-wearable 8.2.0 or later"
	wearable := func(version string) []librarymodel.Dependency {
		return []librarymodel.Dependency{{Group: "com.google.android.gms", Name: "play-services-wearable", Version: version}}
	}
	cases := []struct {
		name    string
		profile librarymodel.ProjectProfile
		want    []string
	}{
		{"recent wearable", librarymodel.ProjectProfile{HasGradle: true, Dependencies: wearable("17.0.0"),
			Android: librarymodel.AndroidProfile{CompileSDK: librarymodel.KnownVersion("34")}}, []string{deprecated}},
		{"old wearable", librarymodel.ProjectProfile{HasGradle: true, Dependencies: wearable("7.8.0"),
			Android: librarymodel.AndroidProfile{CompileSDK: librarymodel.KnownVersion("34")}}, []string{upgrade}},
		{"no wearable", librarymodel.ProjectProfile{HasGradle: true,
			Android: librarymodel.AndroidProfile{CompileSDK: librarymodel.KnownVersion("34")}}, []string{upgrade}},
		{"old build SDK", librarymodel.ProjectProfile{HasGradle: true,
			Android: librarymodel.AndroidProfile{CompileSDK: librarymodel.KnownVersion("23")}}, nil},
		// Lint checks services only in Gradle projects.
		{"not Gradle", librarymodel.ProjectProfile{Dependencies: wearable("17.0.0")}, nil},
	}
	for _, c := range cases {
		if got := wearBindListenerFindings(t, c.profile); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}

// writeResFiles writes name -> content under dir/res and returns the res directory.
func writeResFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	res := filepath.Join(t.TempDir(), "res")
	for name, body := range files {
		path := filepath.Join(res, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return res
}

// The expected findings come from the Android Lint 32.4.1 CLI on the same resource folders.
func TestDuplicateStrings_MatchesLint(t *testing.T) {
	res := writeResFiles(t, map[string]string{
		"values/strings.xml": `<resources xmlns:xliff="urn:oasis:names:tc:xliff:document:1.2">
    <string name="ok">OK</string>
    <string name="ok2">ok</string>
    <string name="title">Settings</string>
    <string name="title_upper">SETTINGS</string>
    <string name="settings">Settings</string>
    <string name="a">Hello</string>
    <string name="b">Hello</string>
    <string name="c">Hello</string>
    <string name="d">Hello</string>
    <string name="e">Hello</string>
    <string name="nt" translatable="false">Unique</string>
    <string name="nt2" translatable="false">Unique</string>
    <string name="markup">Bold <b>text</b></string>
    <string name="markup2">Bold text</string>
    <string name="q1">"Quoted"</string>
    <string name="q2">Quoted</string>
    <string name="sp1"> Space</string>
    <string name="sp2">Space</string>
    <string name="empty1"></string>
    <string name="empty2"></string>
    <string
        name="multi">Multi</string>
    <string name="multi2">Multi</string>
</resources>
`,
		"values-night/strings.xml": `<resources>
    <string name="night_ok">OK</string>
</resources>
`,
		"values-fr/strings.xml": `<resources>
    <string name="ok">D\'accord</string>
    <string name="ok2">d\'accord</string>
    <string name="title">Paramètres</string>
</resources>
`,
		"values-fr-rCA/strings.xml": `<resources>
    <string name="title">Paramètres</string>
</resources>
`,
	})
	idx, err := android.ScanResourceDir(res)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range runResourceRule(findResourceRule(t, "DuplicateStrings"), idx) {
		rel, _ := filepath.Rel(res, f.File)
		got = append(got, fmt.Sprintf("%s:%d:%d: %s", filepath.ToSlash(rel), f.Line, f.Col, f.Message))
	}
	sort.Strings(got)
	const vary = ". Use `android:inputType` or `android:capitalize` to treat these as the same and avoid string duplication."
	want := []string{
		"values-fr/strings.xml:2:5: Duplicate string value `d\\'accord`, used in `ok2` and `ok`" + vary,
		"values/strings.xml:16:5: Duplicate string value `Quoted`, used in `q1` and `q2`",
		"values/strings.xml:22:5: Duplicate string value `Multi`, used in `multi2` and `multi`",
		"values/strings.xml:2:5: Duplicate string value `OK`, used in `night_ok`, `ok2` and `ok`" + vary,
		"values/strings.xml:4:5: Duplicate string value `Settings`, used in `settings`, `title_upper` and `title`" + vary,
		"values/strings.xml:7:5: Duplicate string value `Hello`, used in `a`, `b`, `c`, `d` and `e`",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findings differ from Lint\n got %q\nwant %q", got, want)
	}
}

// The resource index records, for each <string>, where its tag opens, whether it is translatable, and whether it
// has child elements (markup or xliff:g), which DuplicateStrings needs.
func TestStringDefinitionsRecordElementFacts(t *testing.T) {
	res := writeResFiles(t, map[string]string{"values/strings.xml": `<resources xmlns:xliff="urn:oasis:names:tc:xliff:document:1.2">
  <string name="plain">Plain</string>
  <string name="nt" translatable="false">No</string>
  <string name="g">Hi <xliff:g id="n">%s</xliff:g></string>
  <string
      name="wrapped">Wrapped</string>
</resources>
`})
	idx, err := android.ScanResourceDir(res)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, d := range idx.StringDefinitions {
		got[d.Name] = fmt.Sprintf("%d:%d translatable=%q elements=%v", d.ElementLine, d.ElementCol, d.Translatable, d.HasElements)
	}
	want := map[string]string{
		"plain":   `2:3 translatable="" elements=false`,
		"nt":      `3:3 translatable="false" elements=false`,
		"g":       `4:3 translatable="" elements=true`,
		"wrapped": `5:3 translatable="" elements=false`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}
