package gradlemodel

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func touchJar(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	write(t, path, "jar:"+name)
	return path
}

func TestLoadParsesCompilerPlugins(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "model")
	write(t, filepath.Join(dir, "app.json"), fmt.Sprintf(`{"schema":2,"projects":[{"path":":app","dir":%q,"sourceSets":[
		{"name":"main","kind":"main","kotlin":{"pluginVersion":"2.4.21",
			"compilerPluginClasspath":["/p/kotlin-scripting-compiler-embeddable-2.4.21.jar","/p/kotlin-serialization-compiler-plugin-embeddable-2.4.21.jar"],
			"compilerPluginOptions":["plugin:org.jetbrains.kotlinx.serialization:disableIntrinsic=true"]}}]}]}`, root))
	model, _, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	kotlin := model.Projects[0].SourceSets[0].Kotlin
	if len(kotlin.CompilerPluginClasspath) != 2 || kotlin.CompilerPluginOptions[0] != "plugin:org.jetbrains.kotlinx.serialization:disableIntrinsic=true" {
		t.Fatalf("kotlin options = %+v", kotlin)
	}
}

// KGP puts its scripting support on every compilation's plugin classpath; it
// cannot load into krit-fir's compiler and would take the real plugins down
// with it, so it is filtered out. Missing jars, malformed options and options
// that write build reports are rejected with a reason, never dropped silently.
func TestResolveCompilerPluginsFilters(t *testing.T) {
	dir := t.TempDir()
	serialization := touchJar(t, dir, "kotlin-serialization-compiler-plugin-embeddable-2.4.21.jar")
	compose := touchJar(t, dir, "kotlin-compose-compiler-plugin-embeddable-2.4.21.jar")
	var kgp []string
	for _, name := range []string{
		"kotlin-scripting-compiler-embeddable-2.4.21.jar", "kotlin-scripting-compiler-impl-embeddable-2.4.21.jar",
		"kotlin-scripting-jvm-2.4.21.jar", "kotlin-scripting-common-2.4.21.jar", "kotlin-stdlib-2.4.21.jar",
		"annotations-13.0.jar", "kotlin-script-runtime-2.4.21.jar", "kotlinx-coroutines-core-jvm-1.8.0.jar",
	} {
		kgp = append(kgp, touchJar(t, dir, name))
	}
	missing := filepath.Join(dir, "gone-1.0.jar")
	classpath := append(append([]string{}, kgp...), serialization, compose, missing, serialization, " ")
	options := []string{
		"plugin:androidx.compose.compiler.plugins.kotlin:reportsDestination=/out/reports",
		"plugin:androidx.compose.compiler.plugins.kotlin:generateFunctionKeyMetaAnnotations=true",
		"-P plugin:x:y=z",
		"plugin:org.jetbrains.kotlin.allopen:annotation=a.Open",
		"plugin:org.jetbrains.kotlin.allopen:annotation=a.Open",
	}
	plugins, notes, rejected := ResolveCompilerPlugins(classpath, options)
	if !reflect.DeepEqual(plugins.Classpath, []string{serialization, compose}) {
		t.Fatalf("classpath = %v", plugins.Classpath)
	}
	wantOptions := []string{
		"plugin:androidx.compose.compiler.plugins.kotlin:generateFunctionKeyMetaAnnotations=true",
		"plugin:org.jetbrains.kotlin.allopen:annotation=a.Open",
	}
	if !reflect.DeepEqual(plugins.Options, wantOptions) {
		t.Fatalf("options = %v", plugins.Options)
	}
	if len(notes) != 1 || !strings.Contains(notes[0], "8 Kotlin scripting support entries") {
		t.Fatalf("notes = %v", notes)
	}
	joined := strings.Join(rejected, "\n")
	for _, want := range []string{"gone-1.0.jar (not found)", "reportsDestination=/out/reports (writes build reports)", "-P plugin:x:y=z (expected plugin:<id>:<key>=<value>)"} {
		if !strings.Contains(joined, want) {
			t.Errorf("rejected %v lacks %q", rejected, want)
		}
	}
	if len(rejected) != 3 {
		t.Fatalf("rejected = %v", rejected)
	}

	// Options without a jar are not applied.
	orphan, notes, _ := ResolveCompilerPlugins(kgp, []string{"plugin:a:b=c"})
	if !orphan.IsZero() || orphan.Options != nil || len(notes) != 2 || !strings.Contains(notes[1], "no compiler plugin jar") {
		t.Fatalf("orphan = %+v notes = %v", orphan, notes)
	}
}

func TestResolveKotlinSettingsCarriesPlugins(t *testing.T) {
	dir := t.TempDir()
	jar := touchJar(t, dir, "kotlin-serialization-compiler-plugin-embeddable-2.4.21.jar")
	s := ResolveKotlinSettings(&KotlinOptions{
		LanguageVersion: "2.3", PluginVersion: "2.4.21",
		CompilerPluginClasspath: []string{filepath.Join(dir, "kotlin-scripting-compiler-embeddable-2.4.21.jar"), jar},
		CompilerPluginOptions:   []string{"plugin:org.jetbrains.kotlinx.serialization:disableIntrinsic=true"},
	})
	if !reflect.DeepEqual(s.Plugins.Classpath, []string{jar}) || len(s.Plugins.Options) != 1 {
		t.Fatalf("plugins = %+v", s.Plugins)
	}
	// Plugins are never compiler arguments.
	for _, arg := range s.CompilerArgs() {
		if strings.Contains(arg, "plugin") {
			t.Fatalf("compiler args carry a plugin: %v", s.CompilerArgs())
		}
	}
	if !strings.Contains(s.Describe(), "compiler plugins: kotlin-serialization-compiler-plugin-embeddable-2.4.21.jar") {
		t.Fatalf("describe = %q", s.Describe())
	}
	if !strings.Contains(strings.Join(s.Notes, "\n"), "scripting support") {
		t.Fatalf("notes = %v", s.Notes)
	}
}

// Plugins are part of the settings identity (so every cache, daemon key and
// fingerprint keyed on it follows a plugin change), including a jar rebuilt in
// place; settings without plugins keep the identity they always had.
func TestKotlinSettingsIdentityFollowsPlugins(t *testing.T) {
	dir := t.TempDir()
	jar := touchJar(t, dir, "plugin-1.0.jar")
	base := KotlinSettings{LanguageVersion: "2.3", APIVersion: "2.3"}
	before := base.Identity()
	if strings.Contains(before, "plugins") {
		t.Fatalf("identity without plugins mentions them: %q", before)
	}
	withPlugin := base
	withPlugin.Plugins = CompilerPlugins{Classpath: []string{jar}}
	withOption := withPlugin
	withOption.Plugins.Options = []string{"plugin:p:k=v"}
	ids := map[string]string{"base": before, "plugin": withPlugin.Identity(), "option": withOption.Identity()}
	seen := map[string]string{}
	for name, id := range ids {
		if prior, dup := seen[id]; dup {
			t.Fatalf("%s and %s share identity %q", name, prior, id)
		}
		seen[id] = name
	}
	if err := os.WriteFile(jar, []byte("a rebuilt plugin"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(jar, time.Now().Add(time.Hour), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if withPlugin.Identity() == ids["plugin"] {
		t.Fatal("a rebuilt plugin jar must change the identity")
	}
	onlyPlugins := KotlinSettings{Plugins: CompilerPlugins{Classpath: []string{jar}}}
	if onlyPlugins.IsZero() || onlyPlugins.Identity() == "" || onlyPlugins.IdentityHash() == "" {
		t.Fatal("plugins alone are settings")
	}
	if (KotlinSettings{Plugins: CompilerPlugins{Options: []string{"plugin:p:k=v"}}}).IsZero() != true {
		t.Fatal("options without a jar load nothing")
	}
}

// The whole-repo compilation loads the union of every source set's plugins: a
// plugin only acts on declarations that use it. One artifact is never loaded
// in two versions, and an option that differs between source sets stays at
// the plugin default.
func TestMergeKotlinSettingsUnionsPlugins(t *testing.T) {
	dir := t.TempDir()
	ser21 := touchJar(t, dir, "kotlin-serialization-compiler-plugin-embeddable-2.4.21.jar")
	ser20 := touchJar(t, filepath.Join(dir, "old"), "kotlin-serialization-compiler-plugin-embeddable-2.4.20.jar")
	compose := touchJar(t, dir, "kotlin-compose-compiler-plugin-embeddable-2.4.21.jar")
	allopen := touchJar(t, dir, "kotlin-allopen-compiler-plugin-embeddable-2.4.21.jar")
	sets := []KotlinSettings{
		{LanguageVersion: "2.3", Plugins: CompilerPlugins{Classpath: []string{ser21}, Options: []string{"plugin:s:k=1"}}},
		{LanguageVersion: "2.3", Plugins: CompilerPlugins{Classpath: []string{compose, ser21}, Options: []string{"plugin:c:x=1"}}},
		{LanguageVersion: "2.3"},
		{LanguageVersion: "2.3", Plugins: CompilerPlugins{Classpath: []string{ser20, allopen}, Options: []string{
			"plugin:s:k=2", "plugin:a:annotation=A", "plugin:a:annotation=B",
		}}},
		{LanguageVersion: "2.3", Plugins: CompilerPlugins{Classpath: []string{allopen}, Options: []string{"plugin:a:annotation=A", "plugin:a:annotation=B"}}},
	}
	merged := MergeKotlinSettings(sets, []string{":a:main", ":b:main", ":c:main", ":d:main", ":e:main"})
	if merged.LanguageVersion != "2.3" {
		t.Fatalf("language = %q", merged.LanguageVersion)
	}
	if want := []string{ser21, compose, allopen}; !reflect.DeepEqual(merged.Plugins.Classpath, want) {
		t.Fatalf("classpath = %v want %v", merged.Plugins.Classpath, want)
	}
	if want := []string{"plugin:c:x=1", "plugin:a:annotation=A", "plugin:a:annotation=B"}; !reflect.DeepEqual(merged.Plugins.Options, want) {
		t.Fatalf("options = %v want %v", merged.Plugins.Options, want)
	}
	notes := strings.Join(merged.Notes, "\n")
	for _, want := range []string{
		"compiler plugin kotlin-serialization-compiler-plugin-embeddable differs across source sets; loading kotlin-serialization-compiler-plugin-embeddable-2.4.21.jar",
		"compiler plugin option plugin:s:k differs across source sets",
	} {
		if !strings.Contains(notes, want) {
			t.Errorf("notes %q lack %q", notes, want)
		}
	}
}

func TestWithCompilerPluginsAddsConfiguredPlugins(t *testing.T) {
	dir := t.TempDir()
	model := touchJar(t, dir, "kotlin-serialization-compiler-plugin-embeddable-2.4.21.jar")
	configured := touchJar(t, dir, "kotlin-parcelize-compiler-2.4.21.jar")
	s := KotlinSettings{LanguageVersion: "2.3", Plugins: CompilerPlugins{Classpath: []string{model}}}
	if got := s.WithCompilerPlugins(nil, nil, "krit.yml"); !reflect.DeepEqual(got, s) {
		t.Fatalf("nothing configured changed the settings: %+v", got)
	}
	got := s.WithCompilerPlugins([]string{configured, model, filepath.Join(dir, "missing.jar")}, []string{"plugin:p:k=v"}, "krit.yml")
	if want := []string{model, configured}; !reflect.DeepEqual(got.Plugins.Classpath, want) {
		t.Fatalf("classpath = %v", got.Plugins.Classpath)
	}
	if !reflect.DeepEqual(got.Plugins.Options, []string{"plugin:p:k=v"}) {
		t.Fatalf("options = %v", got.Plugins.Options)
	}
	if len(got.Rejected) != 1 || !strings.HasPrefix(got.Rejected[0], "krit.yml: compiler plugin") {
		t.Fatalf("rejected = %v", got.Rejected)
	}
	if got.LanguageVersion != "2.3" {
		t.Fatal("configured plugins must not touch the language settings")
	}
	// No model at all: the configured plugins alone.
	alone := KotlinSettings{}.WithCompilerPlugins([]string{configured}, nil, "krit.yml")
	if !reflect.DeepEqual(alone.Plugins.Classpath, []string{configured}) || alone.IsZero() {
		t.Fatalf("alone = %+v", alone)
	}
}
