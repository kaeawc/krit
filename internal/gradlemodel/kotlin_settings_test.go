package gradlemodel

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestLoadParsesKotlinSettingsPerSourceSet(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "model")
	write(t, filepath.Join(dir, "app.json"), fmt.Sprintf(`{"schema":2,"projects":[{"path":":app","dir":%q,"sourceSets":[
		{"name":"main","kind":"main","kotlin":{"languageVersion":"2.1","apiVersion":"2.0","optIn":["kotlin.RequiresOptIn"],
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
	if main == nil || main.LanguageVersion != "2.1" || main.APIVersion != "2.0" || !main.ProgressiveMode ||
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
	if app.LanguageVersion != "2.1" || app.APIVersion != "2.0" || !app.Progressive ||
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
		{"api defaults to language", &KotlinOptions{LanguageVersion: "2.1"},
			KotlinSettings{LanguageVersion: "2.1", APIVersion: "2.1"}, "defaults to the language version"},
		{"explicit wins over plugin", &KotlinOptions{LanguageVersion: "2.1", APIVersion: "2.0", PluginVersion: "2.1.0"},
			KotlinSettings{LanguageVersion: "2.1", APIVersion: "2.0"}, ""},
		{"free arg overrides DSL", &KotlinOptions{LanguageVersion: "2.1", FreeCompilerArgs: []string{"-language-version", "2.0", "-api-version=2.0"}},
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
		"-jvm-target", "2.0",
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
	for _, protected := range []string{"-d /tmp/out", "-classpath=/evil.jar", "-cp /evil.jar", "-jvm-target 2.0",
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
		LanguageVersion: "2.0", APIVersion: "2.1", OptIns: []string{"b.B", "a.A", "a.A"}, Progressive: true,
		Flags: []string{"-Xcontext-parameters"}, Notes: []string{"n"}, Rejected: []string{"r"},
	}
	want := []string{"-language-version=2.0", "-api-version=2.1", "-opt-in=a.A", "-opt-in=b.B", "-progressive", "-Xcontext-parameters"}
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
		"language":    {LanguageVersion: "2.1", APIVersion: "2.0", OptIns: base.OptIns, Flags: base.Flags},
		"api":         {LanguageVersion: "2.0", APIVersion: "2.1", OptIns: base.OptIns, Flags: base.Flags},
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
	old := KotlinSettings{LanguageVersion: "2.1", APIVersion: "2.1", OptIns: []string{"a.A"}, Flags: []string{"-Xcontext-parameters"}}

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

// helperSources are the two Kotlin copies of the settings allowlist and parser.
var helperSources = map[string]string{
	"krit-fir":   "../../tools/krit-fir/src/main/kotlin/dev/jasonpearson/krit/fir/runner/KotlinCompilerSettings.kt",
	"krit-types": "../../tools/krit-types/src/main/kotlin/dev/jasonpearson/krit/types/KotlinCompilerSettings.kt",
}

func readHelper(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(helperSources[name])
	if err != nil {
		t.Fatalf("%s helper source missing (moved? update this test): %v", name, err)
	}
	return string(data)
}

// kotlinStringSet returns the quoted strings of `private val NAME = setOf(...)`.
func kotlinStringSet(t *testing.T, text, name string) map[string]bool {
	t.Helper()
	m := regexp.MustCompile(`val ` + name + ` = setOf\(([^)]*)\)`).FindStringSubmatch(text)
	if m == nil {
		t.Fatalf("no set %s in helper", name)
	}
	out := map[string]bool{}
	for _, q := range regexp.MustCompile(`"([^"]+)"`).FindAllStringSubmatch(m[1], -1) {
		out[q[1]] = true
	}
	return out
}

// TestKotlinSettingsMatchHelpers keeps the three copies of the allowlist, the
// protected sets and the validation regexes (Go, krit-fir, krit-types) exactly
// equal, so a flag Go sends is never silently dropped by a helper and a helper
// never accepts what Go rejects.
func TestKotlinSettingsMatchHelpers(t *testing.T) {
	wantAllow := map[string]bool{}
	for name, kind := range kotlinFlagAllowlist {
		wantAllow[name] = kind == flagValued
	}
	wantProtected := map[string]bool{}
	for name := range protectedKotlinArgs {
		wantProtected[name] = true
	}
	wantValued := map[string]bool{}
	for name := range protectedValued {
		wantValued[name] = true
	}
	regexes := map[string]string{
		"VERSION": versionRe.String(), "OPT_IN": optInRe.String(),
		"FLAG_VALUE": flagValueRe.String(), "LANGUAGE_TOGGLE": languageRe.String(),
	}
	companions := map[string]string{}
	for name := range helperSources {
		text := readHelper(t, name)
		gotAllow := map[string]bool{}
		for _, m := range regexp.MustCompile(`"(-X[A-Za-z0-9-]+)" to (true|false), // allowlist:`).FindAllStringSubmatch(text, -1) {
			gotAllow[m[1]] = m[2] == "true"
		}
		if !reflect.DeepEqual(gotAllow, wantAllow) {
			t.Errorf("%s allowlist (name -> takes a value) differs from Go:\n got %v\nwant %v", name, gotAllow, wantAllow)
		}
		if got := kotlinStringSet(t, text, "PROTECTED"); !reflect.DeepEqual(got, wantProtected) {
			t.Errorf("%s protected set differs from Go:\n got %v\nwant %v", name, got, wantProtected)
		}
		if got := kotlinStringSet(t, text, "PROTECTED_VALUED"); !reflect.DeepEqual(got, wantValued) {
			t.Errorf("%s protected-valued set differs from Go:\n got %v\nwant %v", name, got, wantValued)
		}
		for ktName, goSource := range regexes {
			m := regexp.MustCompile(`val ` + ktName + ` = Regex\("(.*)"\)`).FindStringSubmatch(text)
			if m == nil {
				t.Errorf("%s has no %s regex", name, ktName)
				continue
			}
			if got := strings.ReplaceAll(m[1], `\\`, `\`); got != goSource {
				t.Errorf("%s %s regex = %q, Go = %q", name, ktName, got, goSource)
			}
		}
		// The prefixes isProtected checks beyond the exact names.
		for _, prefix := range []string{`"-Xfragment"`, `"-Xplugin"`} {
			if !strings.Contains(text, "startsWith("+prefix+")") {
				t.Errorf("%s does not protect the %s prefix", name, prefix)
			}
		}
		idx := strings.Index(text, "companion object {")
		if idx < 0 {
			t.Fatalf("%s has no companion object", name)
		}
		companions[name] = text[idx:]
	}
	if companions["krit-fir"] != companions["krit-types"] {
		t.Error("the companion-object blocks of the two Kotlin helper copies differ; they must stay identical")
	}
}

func TestKotlinSettingsNullabilityValuesAllowAtSigns(t *testing.T) {
	got := ClassifyKotlinArgs([]string{
		"-Xnullability-annotations=@org.jspecify.annotations:strict", "-Xjsr305=@org.example.Nullable:warn", "-Xjsr305=strict", "@-Xjsr305=strict",
	})
	want := []string{"-Xnullability-annotations=@org.jspecify.annotations:strict", "-Xjsr305=strict"}
	if !reflect.DeepEqual(got.Flags, want) {
		t.Fatalf("flags = %v want %v", got.Flags, want)
	}
	if len(got.Rejected) != 1 {
		t.Fatalf("only the argument starting with '@' is rejected: %v", got.Rejected)
	}
}

func TestKotlinSettingsProgressiveFalse(t *testing.T) {
	for args, want := range map[string]bool{"-progressive": true, "-progressive=true": true, "-progressive=false": false} {
		if got := ClassifyKotlinArgs([]string{args}).Progressive; got != want {
			t.Errorf("%s => %v", args, got)
		}
	}
	if ClassifyKotlinArgs([]string{"-progressive", "-progressive=false"}).Progressive {
		t.Error("-progressive=false must turn progressive mode off")
	}
	if got := ClassifyKotlinArgs([]string{"-progressive=maybe"}); got.Progressive || len(got.Rejected) != 1 {
		t.Errorf("malformed value: %+v", got)
	}
	if ResolveKotlinSettings(&KotlinOptions{LanguageVersion: "2.1", FreeCompilerArgs: []string{"-progressive=false"}}).Progressive {
		t.Error("resolved settings must honor -progressive=false")
	}
}

func TestResolveKotlinSettingsLevels(t *testing.T) {
	// The helpers cannot analyze below MinSupportedKotlinLanguage; Go must not apply,
	// report or identity-hash what they would drop.
	old := ResolveKotlinSettings(&KotlinOptions{LanguageVersion: "1.9", APIVersion: "1.8", OptIn: []string{"a.A"}})
	if old.LanguageVersion != "" || old.APIVersion != "" {
		t.Fatalf("unsupported levels applied: %+v", old)
	}
	if !strings.Contains(strings.Join(old.Notes, "\n"), "languageVersion 1.9 is not supported by the embedded compiler") {
		t.Fatalf("unsupported level not reported: %v", old.Notes)
	}
	if !reflect.DeepEqual(old.OptIns, []string{"a.A"}) {
		t.Fatalf("opt-ins must survive: %+v", old)
	}
	if got := ResolveKotlinSettings(&KotlinOptions{LanguageVersion: "1.9", APIVersion: "1.9"}); got.Identity() != "" {
		t.Fatalf("only-unsupported settings must have the embedded-default identity: %q", got.Identity())
	}
	// A plugin older than the minimum yields no derived level either.
	if got := ResolveKotlinSettings(&KotlinOptions{PluginVersion: "1.9.24"}); got.LanguageVersion != "" || got.APIVersion != "" {
		t.Fatalf("derived unsupported level applied: %+v", got)
	}
	above := ResolveKotlinSettings(&KotlinOptions{LanguageVersion: "2.1", APIVersion: "2.3"})
	if above.LanguageVersion != "2.1" || above.APIVersion != "" || !strings.Contains(strings.Join(above.Notes, "\n"), "greater than languageVersion") {
		t.Fatalf("api above language: %+v", above)
	}
	if got := ResolveKotlinSettings(&KotlinOptions{LanguageVersion: "2.2", APIVersion: "2.0"}); got.APIVersion != "2.0" {
		t.Fatalf("a lower API level is valid: %+v", got)
	}
}

// A schema 2 source set with no kotlin object has no Kotlin compile task (Java
// only): it analyzes no Kotlin, so it must not veto the whole-repo settings.
func TestJavaOnlySourceSetsDoNotVetoTheMerge(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "model")
	write(t, filepath.Join(dir, "a.json"), fmt.Sprintf(`{"schema":2,"projects":[
		{"path":":kt","dir":%q,"sourceSets":[{"name":"main","kind":"main","kotlin":{"languageVersion":"2.1","apiVersion":"2.1","optIn":["a.A"]}}]},
		{"path":":java","dir":%q,"sourceSets":[{"name":"main","kind":"main"},{"name":"test","kind":"test"}]}]}`, root, root))
	write(t, filepath.Join(dir, "b.json"), fmt.Sprintf(`{"schema":1,"projects":[{"path":":old","dir":%q,"sourceSets":[{"name":"main","kind":"main"}]}]}`, root))
	model, _, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	var labels []string
	for _, e := range model.KotlinSettingsBySourceSet() {
		labels = append(labels, e.Project+":"+e.SourceSet)
	}
	if !reflect.DeepEqual(labels, []string{":kt:main", ":old:main"}) {
		t.Fatalf("Java-only source sets must be skipped (and a schema 1 one kept as unknown): %v", labels)
	}
	// Without the unknown schema 1 project the Kotlin source set's settings apply.
	only := &Model{Projects: []Project{model.Projects[0], model.Projects[1]}}
	got := only.EffectiveKotlinSettings()
	if got.LanguageVersion != "2.1" || got.APIVersion != "2.1" || !reflect.DeepEqual(got.OptIns, []string{"a.A"}) {
		t.Fatalf("Java-only project vetoed the merge: %+v", got)
	}
}
