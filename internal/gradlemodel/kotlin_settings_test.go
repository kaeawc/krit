package gradlemodel

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadParsesKotlinSettingsPerSourceSet(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "model")
	write(t, filepath.Join(dir, "app.json"), fmt.Sprintf(`{"schema":2,"projects":[{"path":":app","dir":%q,"sourceSets":[
		{"name":"main","kind":"main","kotlin":{"languageVersion":"1.9","apiVersion":"1.8","optIn":["kotlin.RequiresOptIn"],
			"freeCompilerArgs":["-Xcontext-receivers"],"progressiveMode":true,"pluginVersion":"2.1.20"}},
		{"name":"test","kind":"test","kotlin":{"languageVersion":"","pluginVersion":"2.1.20"}}]}]}`, root))
	write(t, filepath.Join(dir, "lib.json"), fmt.Sprintf(`{"schema":1,"projects":[{"path":":lib","dir":%q,"sourceSets":[{"name":"main","kind":"main"}]}]}`, root))
	write(t, filepath.Join(dir, "future.json"), `{"schema":3,"projects":[]}`)

	model, warnings, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "unsupported schema 3") {
		t.Fatalf("warnings = %v", warnings)
	}
	by := map[string]Project{}
	for _, p := range model.Projects {
		by[p.Path] = p
	}
	if by[":app"].SchemaVersion != 2 || by[":lib"].SchemaVersion != 1 {
		t.Fatalf("schema versions: app=%d lib=%d", by[":app"].SchemaVersion, by[":lib"].SchemaVersion)
	}
	main := by[":app"].SourceSets[0].Kotlin
	if main == nil || main.LanguageVersion != "1.9" || main.APIVersion != "1.8" || !main.ProgressiveMode ||
		!reflect.DeepEqual(main.OptIn, []string{"kotlin.RequiresOptIn"}) || main.PluginVersion != "2.1.20" {
		t.Fatalf("main kotlin options = %+v", main)
	}
	if by[":lib"].SourceSets[0].Kotlin != nil {
		t.Fatal("schema 1 source set must have no Kotlin options")
	}

	entries := model.KotlinSettingsBySourceSet()
	if len(entries) != 3 {
		t.Fatalf("entries = %+v", entries)
	}
	got := map[string]KotlinSettings{}
	for _, e := range entries {
		got[e.Project+":"+e.SourceSet] = e.Settings
	}
	app := got[":app:main"]
	if app.LanguageVersion != "1.9" || app.APIVersion != "1.8" || !app.Progressive ||
		!reflect.DeepEqual(app.Flags, []string{"-Xcontext-receivers"}) {
		t.Fatalf("app main = %+v", app)
	}
	// The test source set pins nothing: Gradle compiled it with the plugin's
	// language version, so that is its effective setting, and it differs from main.
	if test := got[":app:test"]; test.LanguageVersion != "2.1" || test.APIVersion != "2.1" {
		t.Fatalf("app test = %+v", test)
	}
	if lib := got[":lib:main"]; !lib.IsZero() || len(lib.Notes) == 0 {
		t.Fatalf("schema 1 source set must resolve to visible defaults: %+v", lib)
	}
}

func TestResolveKotlinSettingsDefaults(t *testing.T) {
	cases := []struct {
		name string
		opts *KotlinOptions
		want KotlinSettings
		note string
	}{
		{"absent model", nil, KotlinSettings{}, "embedded compiler's defaults apply"},
		{"nothing known", &KotlinOptions{}, KotlinSettings{}, "plugin version is unknown"},
		{"derived from plugin", &KotlinOptions{PluginVersion: "2.0.21-RC"},
			KotlinSettings{LanguageVersion: "2.0", APIVersion: "2.0"}, "using 2.0 from Kotlin Gradle plugin"},
		{"api defaults to language", &KotlinOptions{LanguageVersion: "1.9"},
			KotlinSettings{LanguageVersion: "1.9", APIVersion: "1.9"}, "defaults to the language version"},
		{"explicit wins over plugin", &KotlinOptions{LanguageVersion: "1.9", APIVersion: "1.8", PluginVersion: "2.1.0"},
			KotlinSettings{LanguageVersion: "1.9", APIVersion: "1.8"}, ""},
		{"free arg overrides DSL", &KotlinOptions{LanguageVersion: "1.9", FreeCompilerArgs: []string{"-language-version", "2.0", "-api-version=2.0"}},
			KotlinSettings{LanguageVersion: "2.0", APIVersion: "2.0"}, ""},
		{"malformed version ignored", &KotlinOptions{LanguageVersion: "latest"},
			KotlinSettings{}, `"latest" is not a major.minor version`},
		{"opt-ins merged and sorted", &KotlinOptions{LanguageVersion: "2.0", OptIn: []string{"b.B", "a.A", "bad marker"},
			FreeCompilerArgs: []string{"-opt-in=a.A", "-Xopt-in=c.C"}},
			KotlinSettings{LanguageVersion: "2.0", APIVersion: "2.0", OptIns: []string{"a.A", "b.B", "c.C"}}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveKotlinSettings(tc.opts)
			notes, rejected := got.Notes, got.Rejected
			got.Notes, got.Rejected = nil, nil
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v want %+v", got, tc.want)
			}
			if tc.note != "" && !strings.Contains(strings.Join(notes, "\n"), tc.note) {
				t.Fatalf("notes %v lack %q", notes, tc.note)
			}
			if tc.name == "opt-ins merged and sorted" && len(rejected) != 1 {
				t.Fatalf("invalid opt-in must be reported: %v", rejected)
			}
		})
	}
}

func TestKotlinFlagAllowlistAndProtectedArgs(t *testing.T) {
	args := []string{
		"-Xcontext-parameters",
		"-Xexplicit-backing-fields",
		"-Xannotation-default-target=param-property",
		"-XXLanguage:+ContextParameters",
		"-XXLanguage:-ExplicitBackingFields",
		"-XXLanguage:+ContextParameters", // repeated toggle keeps the last position
		"-Xjvm-default=all",
		// Krit-owned settings: classpath, output, plugin, friend, fragment, module identity.
		"-d", "/tmp/out",
		"-classpath=/evil.jar",
		"-cp", "/evil.jar",
		"-jvm-target", "1.8",
		"-module-name=evil",
		"-Xplugin=/evil-plugin.jar",
		"-P", "plugin:x:y=z",
		"-Xfriend-paths=/a",
		"-Xfragments=a,b",
		"-Xfragment-sources=a:/b.kt",
		"-Xfragment-refines=a:b",
		"-Xmulti-platform",
		"-Xjdk-release=8",
		// Not analysis-semantic, or malformed.
		"-Xno-param-assertions",
		"-Werror",
		"-Xcontext-parameters=true",
		"-Xannotation-default-target",
		"-Xannotation-default-target=a b",
		"-XXLanguage:+Bad Feature",
		"",
	}
	got := ClassifyKotlinArgs(args)
	wantFlags := []string{
		"-Xcontext-parameters", "-Xexplicit-backing-fields", "-Xannotation-default-target=param-property",
		"-XXLanguage:-ExplicitBackingFields", "-XXLanguage:+ContextParameters", "-Xjvm-default=all",
	}
	if !reflect.DeepEqual(got.Flags, wantFlags) {
		t.Fatalf("flags = %v want %v", got.Flags, wantFlags)
	}
	joined := strings.Join(got.Rejected, "\n")
	for _, protected := range []string{"-d /tmp/out", "-classpath=/evil.jar", "-cp /evil.jar", "-jvm-target 1.8",
		"-module-name=evil", "-Xplugin=/evil-plugin.jar", "-P plugin:x:y=z", "-Xfriend-paths=/a", "-Xfragments=a,b",
		"-Xfragment-sources=a:/b.kt", "-Xfragment-refines=a:b", "-Xmulti-platform", "-Xjdk-release=8"} {
		if !strings.Contains(joined, protected+" (managed by krit)") {
			t.Errorf("%q not reported as managed: %v", protected, got.Rejected)
		}
	}
	for _, unsupported := range []string{"-Xno-param-assertions", "-Werror", "-Xcontext-parameters=true",
		"-Xannotation-default-target (", "-Xannotation-default-target=a b", "-XXLanguage:+Bad Feature"} {
		if !strings.Contains(joined, unsupported) {
			t.Errorf("%q not reported as rejected: %v", unsupported, got.Rejected)
		}
	}
	// Nothing rejected may leak into the rendered compiler arguments.
	for _, arg := range got.CompilerArgs() {
		for _, banned := range []string{"/evil", "plugin", "friend", "fragment", "module-name", "jvm-target", "jdk-release"} {
			if strings.Contains(arg, banned) || arg == "-d" {
				t.Errorf("protected argument leaked into %q", arg)
			}
		}
	}
}

func TestKotlinSettingsCompilerArgs(t *testing.T) {
	s := KotlinSettings{
		LanguageVersion: "2.0", APIVersion: "1.9", OptIns: []string{"b.B", "a.A", "a.A"}, Progressive: true,
		Flags: []string{"-Xcontext-parameters"}, Notes: []string{"n"}, Rejected: []string{"r"},
	}
	want := []string{"-language-version=2.0", "-api-version=1.9", "-opt-in=a.A", "-opt-in=b.B", "-progressive", "-Xcontext-parameters"}
	if got := s.CompilerArgs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %v want %v", got, want)
	}
	// The rendered form round-trips through the same classifier the JVM side mirrors.
	round := ClassifyKotlinArgs(s.CompilerArgs())
	round.Notes, round.Rejected = nil, nil
	s.Notes, s.Rejected = nil, nil
	s.OptIns = []string{"a.A", "b.B"}
	if !reflect.DeepEqual(round, s) {
		t.Fatalf("round trip %+v != %+v", round, s)
	}
	if (KotlinSettings{}).CompilerArgs() != nil || (KotlinSettings{}).Describe() != "embedded compiler defaults" {
		t.Fatal("zero settings must render nothing")
	}
}

func TestKotlinSettingsIdentity(t *testing.T) {
	base := KotlinSettings{LanguageVersion: "2.0", APIVersion: "2.0", OptIns: []string{"a.A"}, Flags: []string{"-Xcontext-parameters"}}
	if (KotlinSettings{}).Identity() != "" || (KotlinSettings{Notes: []string{"x"}, Rejected: []string{"y"}}).Identity() != "" {
		t.Fatal("zero settings must have an empty identity so existing cache keys are unchanged")
	}
	seen := map[string]string{base.Identity(): "base"}
	variants := map[string]KotlinSettings{
		"language":    {LanguageVersion: "1.9", APIVersion: "2.0", OptIns: base.OptIns, Flags: base.Flags},
		"api":         {LanguageVersion: "2.0", APIVersion: "1.9", OptIns: base.OptIns, Flags: base.Flags},
		"optin":       {LanguageVersion: "2.0", APIVersion: "2.0", OptIns: []string{"a.A", "b.B"}, Flags: base.Flags},
		"flag":        {LanguageVersion: "2.0", APIVersion: "2.0", OptIns: base.OptIns, Flags: []string{"-Xexplicit-backing-fields"}},
		"flag order":  {LanguageVersion: "2.0", APIVersion: "2.0", OptIns: base.OptIns, Flags: []string{"-Xcontext-parameters", "-Xwhen-guards"}},
		"progressive": {LanguageVersion: "2.0", APIVersion: "2.0", OptIns: base.OptIns, Flags: base.Flags, Progressive: true},
		"no flags":    {LanguageVersion: "2.0", APIVersion: "2.0", OptIns: base.OptIns},
	}
	for name, v := range variants {
		id := v.Identity()
		if prior, dup := seen[id]; dup {
			t.Errorf("%s identity collides with %s: %q", name, prior, id)
		}
		seen[id] = name
	}
	// Opt-in order and duplicates, notes and rejected arguments are not semantic.
	same := KotlinSettings{LanguageVersion: "2.0", APIVersion: "2.0", OptIns: []string{"a.A", "a.A"}, Flags: base.Flags,
		Notes: []string{"different note"}, Rejected: []string{"-Werror"}}
	if same.Identity() != base.Identity() || same.IdentityHash() != base.IdentityHash() || base.IdentityHash() == "" {
		t.Fatalf("non-semantic differences changed the identity: %q vs %q", same.Identity(), base.Identity())
	}
}

func TestMergeKotlinSettingsNeverImposesOneModuleOnAnother(t *testing.T) {
	modern := KotlinSettings{LanguageVersion: "2.0", APIVersion: "2.0", OptIns: []string{"a.A", "b.B"}, Flags: []string{"-Xcontext-parameters", "-Xwhen-guards"}}
	old := KotlinSettings{LanguageVersion: "1.9", APIVersion: "1.9", OptIns: []string{"a.A"}, Flags: []string{"-Xcontext-parameters"}}

	agree := MergeKotlinSettings([]KotlinSettings{modern, modern}, []string{":a:main", ":b:main"})
	if agree.LanguageVersion != "2.0" || agree.APIVersion != "2.0" || !reflect.DeepEqual(agree.OptIns, modern.OptIns) || !reflect.DeepEqual(agree.Flags, modern.Flags) {
		t.Fatalf("agreeing sets must keep their settings: %+v", agree)
	}

	mixed := MergeKotlinSettings([]KotlinSettings{modern, old, modern}, []string{":a:main", ":legacy:main", ":c:main"})
	if mixed.LanguageVersion != "" || mixed.APIVersion != "" {
		t.Fatalf("disagreeing versions must not be applied: %+v", mixed)
	}
	if !reflect.DeepEqual(mixed.OptIns, []string{"a.A"}) || !reflect.DeepEqual(mixed.Flags, []string{"-Xcontext-parameters"}) {
		t.Fatalf("opt-ins and flags must be the intersection: %+v", mixed)
	}
	notes := strings.Join(mixed.Notes, "\n")
	for _, want := range []string{"languageVersion differs across source sets", ":legacy:main", "embedded compiler default", "opt-ins differ", "flags differ"} {
		if !strings.Contains(notes, want) {
			t.Errorf("notes lack %q:\n%s", want, notes)
		}
	}

	// A source set the model said nothing about is a disagreement, not a wildcard.
	unknown := MergeKotlinSettings([]KotlinSettings{modern, ResolveKotlinSettings(nil)}, []string{":a:main", ":old:main"})
	if unknown.LanguageVersion != "" || len(unknown.OptIns) != 0 || len(unknown.Flags) != 0 {
		t.Fatalf("unknown source set must not inherit another module's settings: %+v", unknown)
	}

	if got := MergeKotlinSettings(nil, nil); !got.IsZero() {
		t.Fatalf("empty merge = %+v", got)
	}
	if got := MergeKotlinSettings([]KotlinSettings{old}, nil); !reflect.DeepEqual(got, old) {
		t.Fatalf("single set must pass through: %+v", got)
	}
}

func TestEffectiveKotlinSettingsOnNilAndEmptyModel(t *testing.T) {
	var nilModel *Model
	if !nilModel.EffectiveKotlinSettings().IsZero() || (&Model{}).EffectiveKotlinSettings().Identity() != "" {
		t.Fatal("no model means embedded defaults")
	}
}

// TestKotlinFlagAllowlistMatchesHelpers keeps the three copies of the allowlist
// (Go, krit-fir, krit-types) identical, so a flag Go sends is never silently
// dropped by a helper and a helper never accepts what Go rejects.
func TestKotlinFlagAllowlistMatchesHelpers(t *testing.T) {
	for _, helper := range []string{
		"../../tools/krit-fir/src/main/kotlin/dev/jasonpearson/krit/fir/runner/KotlinCompilerSettings.kt",
		"../../tools/krit-types/src/main/kotlin/dev/jasonpearson/krit/types/KotlinCompilerSettings.kt",
	} {
		data, err := os.ReadFile(helper)
		if err != nil {
			t.Fatalf("helper allowlist source missing (moved? update this test): %v", err)
		}
		text := string(data)
		for _, name := range KotlinFlagAllowlist() {
			if !strings.Contains(text, `"`+name+`"`) {
				t.Errorf("%s does not allowlist %s", filepath.Base(filepath.Dir(helper)), name)
			}
		}
		listed := 0
		for _, line := range strings.Split(text, "\n") {
			if strings.Contains(line, "// allowlist:") {
				listed++
			}
		}
		if listed != len(kotlinFlagAllowlist) {
			t.Errorf("%s lists %d allowlisted flags, Go has %d", helper, listed, len(kotlinFlagAllowlist))
		}
	}
}
