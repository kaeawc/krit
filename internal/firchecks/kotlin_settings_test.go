package firchecks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/gradlemodel"
)

func settingsVariants() map[string]gradlemodel.KotlinSettings {
	base := gradlemodel.KotlinSettings{LanguageVersion: "2.0", APIVersion: "2.0", OptIns: []string{"a.A"}, Flags: []string{"-Xcontext-parameters"}}
	vary := func(f func(*gradlemodel.KotlinSettings)) gradlemodel.KotlinSettings {
		s := base
		s.OptIns = append([]string(nil), base.OptIns...)
		s.Flags = append([]string(nil), base.Flags...)
		f(&s)
		return s
	}
	return map[string]gradlemodel.KotlinSettings{
		"base":     base,
		"language": vary(func(s *gradlemodel.KotlinSettings) { s.LanguageVersion = "2.1" }),
		"api":      vary(func(s *gradlemodel.KotlinSettings) { s.APIVersion = "2.1" }),
		"optin":    vary(func(s *gradlemodel.KotlinSettings) { s.OptIns = append(s.OptIns, "b.B") }),
		"flag":     vary(func(s *gradlemodel.KotlinSettings) { s.Flags = []string{"-Xexplicit-backing-fields"} }),
		"progress": vary(func(s *gradlemodel.KotlinSettings) { s.Progressive = true }),
		"none":     {},
	}
}

// A settings-only change must change the FIR check fingerprint (and with it
// every cached FIR verdict), the daemon registry key, and the prefetch match.
func TestSettingsOnlyChangeChangesFirIdentities(t *testing.T) {
	jar := filepath.Join(t.TempDir(), "krit-fir.jar")
	if err := os.WriteFile(jar, []byte("jar"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := t.TempDir()
	file := filepath.Join(src, "A.kt")
	if err := os.WriteFile(file, []byte("class A"), 0o644); err != nil {
		t.Fatal(err)
	}
	seenInvocation := map[string]string{}
	seenCheck := map[string]string{}
	seenRegistry := map[string]string{}
	for name, settings := range settingsVariants() {
		target := CompileTarget{JvmTarget: "17", Kotlin: settings}
		inv := FirInvocationFingerprint(nil, jar, []string{"R"}, nil, FileFacts{}, target)
		check := CheckCacheFingerprint([]string{src}, []string{file}, nil, jar, []string{"R"}, nil, FileFacts{}, target)
		reg := firRegistryKeyFor("check", jar, []string{src}, nil, target)
		for kind, seen := range map[string]map[string]string{"invocation": seenInvocation, "check": seenCheck, "registry": seenRegistry} {
			value := map[string]string{"invocation": inv, "check": check, "registry": reg}[kind]
			if prior, dup := seen[value]; dup {
				t.Errorf("%s fingerprint of %q collides with %q", kind, name, prior)
			}
			seen[value] = name
		}
	}
	// Zero settings keep the pre-settings fingerprint shape exactly.
	withTarget := FirInvocationFingerprint(nil, jar, nil, nil, FileFacts{}, CompileTarget{JvmTarget: "17"})
	legacyShape := FirInvocationFingerprint(nil, jar, nil, nil, FileFacts{}, CompileTarget{JvmTarget: "17", Kotlin: gradlemodel.KotlinSettings{Notes: []string{"n"}, Rejected: []string{"r"}}})
	if withTarget != legacyShape {
		t.Fatal("notes and rejected arguments must not affect the fingerprint")
	}
	// Zero settings leave the registry key exactly as the JVM-target-only key was.
	if got, want := firRegistryKeyFor("", jar, []string{src}, nil, CompileTarget{JvmTarget: "17", Kotlin: gradlemodel.KotlinSettings{Notes: []string{"n"}}}),
		firRegistryKeyFor("", jar, []string{src}, nil, CompileTarget{JvmTarget: "17"}); got != want {
		t.Fatal("notes must not change the registry key")
	}
}

func TestCheckRequestCarriesKotlinSettings(t *testing.T) {
	settings := gradlemodel.KotlinSettings{LanguageVersion: "2.1", APIVersion: "2.0", OptIns: []string{"b.B", "a.A"}, Flags: []string{"-Xcontext-parameters"}}
	data, _, err := encodeCheckRequest(7, []fileRef{{Path: "/p/A.kt"}}, []string{"/p"}, nil, []string{"R"}, nil, FileFacts{}, CompileTarget{JvmTarget: "17", Kotlin: settings})
	if err != nil {
		t.Fatal(err)
	}
	var req struct {
		JvmTarget  string   `json:"jvmTarget"`
		KotlinArgs []string `json:"kotlinArgs"`
	}
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatal(err)
	}
	want := []string{"-language-version=2.1", "-api-version=2.0", "-opt-in=a.A", "-opt-in=b.B", "-Xcontext-parameters"}
	if req.JvmTarget != "17" || !reflect.DeepEqual(req.KotlinArgs, want) {
		t.Fatalf("request = %+v want kotlinArgs %v", req, want)
	}
	data, _, err = encodeCheckRequest(8, []fileRef{{Path: "/p/A.kt"}}, nil, nil, nil, nil, FileFacts{}, CompileTarget{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "kotlinArgs") {
		t.Fatalf("absent settings must be omitted, not sent as empty: %s", data)
	}
}

func TestPrefetchRiderRequiresSameKotlinSettings(t *testing.T) {
	p := newPrefetchProject(t)
	checker := p.opts.Checker.(*ProductionFirChecker)
	checker.Kotlin = gradlemodel.KotlinSettings{LanguageVersion: "2.0", APIVersion: "2.0"}
	prefetch := NewPrefetch(p.opts)
	same := CompileTarget{Kotlin: gradlemodel.KotlinSettings{LanguageVersion: "2.0", APIVersion: "2.0"}}
	if fmt.Sprint(prefetch.Rider(p.jar, []string{p.src}, []string{p.jar}, same)) == "<nil>" {
		t.Fatal("no rider for the checker's own compile context")
	}
	for name, other := range map[string]CompileTarget{
		"older language": {Kotlin: gradlemodel.KotlinSettings{LanguageVersion: "2.1", APIVersion: "2.0"}},
		"no settings":    {},
		"extra flag":     {Kotlin: gradlemodel.KotlinSettings{LanguageVersion: "2.0", APIVersion: "2.0", Flags: []string{"-Xcontext-parameters"}}},
	} {
		if fmt.Sprint(prefetch.Rider(p.jar, []string{p.src}, []string{p.jar}, other)) != "<nil>" {
			t.Errorf("%s: oracle compiled another language level but the check rode on it", name)
		}
	}
}

func TestResponseWarningsReachResult(t *testing.T) {
	var resp CheckResponse
	if err := json.Unmarshal([]byte(`{"id":1,"succeeded":1,"skipped":0,"findings":[],"warning":"krit-fir: ignoring compiler argument -Werror (managed by krit)"}`), &resp); err != nil {
		t.Fatal(err)
	}
	result := newResult()
	result.addResponse(&resp)
	result.addResponse(&resp)
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "-Werror") {
		t.Fatalf("warnings = %v", result.Warnings)
	}
}

func TestModuleSpecsFromModelKeepSettingsPerSourceSet(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "model")
	write := func(name, body string) {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("lib.json", fmt.Sprintf(`{"schema":2,"projects":[{"path":":lib","dir":%q,"sourceSets":[
		{"name":"main","kind":"main","platform":"jvm","sourceDirs":["/lib/src"],"jvmTarget":"11",
		 "kotlin":{"languageVersion":"2.1","apiVersion":"2.1","optIn":["lib.Only"],"freeCompilerArgs":["-Xcontext-receivers","-Xplugin=/evil.jar","-Xfriend-paths=/x"]}},
		{"name":"test","kind":"test","platform":"jvm","sourceDirs":["/lib/test"],
		 "kotlin":{"languageVersion":"2.1","apiVersion":"2.1"}}]}]}`, root))
	write("app.json", fmt.Sprintf(`{"schema":2,"projects":[{"path":":app","dir":%q,"sourceSets":[
		{"name":"main","kind":"main","platform":"jvm","projectDeps":[":lib"],"jvmTarget":"17",
		 "kotlin":{"languageVersion":"2.0","apiVersion":"2.0","freeCompilerArgs":["-Xwhen-guards"]}}]},
		{"path":":legacy","dir":%q,"sourceSets":[{"name":"main","kind":"main"}]}]}`, root, root))
	model, _, err := gradlemodel.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	specs := ModuleSpecsFromModel(model)
	by := map[string]ModuleSpec{}
	for _, s := range specs {
		by[s.ID] = s
	}
	if len(by) != 4 {
		t.Fatalf("specs = %+v", specs)
	}
	if got := by[":lib:main"].CompilerArgs; !reflect.DeepEqual(got, []string{"-language-version=2.1", "-api-version=2.1", "-opt-in=lib.Only", "-Xcontext-receivers"}) {
		t.Fatalf("lib main args = %v", got)
	}
	if got := by[":app:main"].CompilerArgs; !reflect.DeepEqual(got, []string{"-language-version=2.0", "-api-version=2.0", "-Xwhen-guards"}) {
		t.Fatalf("app main args = %v (must not inherit lib's settings)", got)
	}
	if got := by[":lib:test"].CompilerArgs; !reflect.DeepEqual(got, []string{"-language-version=2.1", "-api-version=2.1"}) {
		t.Fatalf("lib test args = %v (must not inherit lib main's opt-in or flags)", got)
	}
	if legacy := by[":legacy:main"]; len(legacy.CompilerArgs) != 0 || legacy.CompilerArgs == nil {
		t.Fatalf("source set without settings must carry an explicit empty list: %#v", legacy.CompilerArgs)
	}
	for _, s := range specs {
		for _, arg := range s.CompilerArgs {
			if strings.Contains(arg, "evil") || strings.Contains(arg, "friend") {
				t.Errorf("%s: krit-owned argument %q reached the module request", s.ID, arg)
			}
		}
	}
	if !reflect.DeepEqual(by[":app:main"].DependsOn, []string{":lib:main"}) ||
		!reflect.DeepEqual(by[":lib:test"].Friends, []string{":lib:main"}) {
		t.Fatalf("graph: app deps %v, lib test friends %v", by[":app:main"].DependsOn, by[":lib:test"].Friends)
	}
	if by[":lib:main"].JVMTarget != "11" || by[":app:main"].JVMTarget != "17" {
		t.Fatal("jvm targets must stay per module")
	}
	encoded, err := json.Marshal(AnalyzeModulesRequest{ID: 1, Command: "analyzeModules", Modules: specs})
	if err != nil || !strings.Contains(string(encoded), `"compilerArgs":[]`) {
		t.Fatalf("encoded request: %v %s", err, encoded)
	}
}
