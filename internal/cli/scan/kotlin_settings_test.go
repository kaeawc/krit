package scan

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/gradlemodel"
)

// writeKotlinModel writes a schema 2 model with one project whose source sets
// are given as JSON `kotlin` objects keyed by source set name.
func writeKotlinModel(t *testing.T, root string, sets map[string]string) string {
	t.Helper()
	modelDir := filepath.Join(root, ".krit", "gradle-model")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "settings.gradle.kts"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	var entries []string
	for _, name := range []string{"main", "test"} {
		kotlin, ok := sets[name]
		if !ok {
			continue
		}
		entries = append(entries, fmt.Sprintf(`{"name":%q,"kind":%q,"kotlin":%s}`, name, name, kotlin))
	}
	body := fmt.Sprintf(`{"schema":2,"projects":[{"path":":","dir":%q,"sourceSets":[%s]}]}`, root, strings.Join(entries, ","))
	if err := os.WriteFile(filepath.Join(modelDir, "_root.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return modelDir
}

func TestLoadGradleKotlinSettings(t *testing.T) {
	pinned := `{"languageVersion":"2.1","apiVersion":"2.1","optIn":["kotlin.RequiresOptIn"],"freeCompilerArgs":["-Xcontext-receivers","-Werror","-Xplugin=/x.jar"]}`
	root := t.TempDir()
	writeKotlinModel(t, root, map[string]string{"main": pinned, "test": pinned})

	var out bytes.Buffer
	got := loadGradleKotlinSettings([]string{root}, "", false, true, &out)
	want := gradlemodel.KotlinSettings{
		LanguageVersion: "2.1", APIVersion: "2.1", OptIns: []string{"kotlin.RequiresOptIn"}, Flags: []string{"-Xcontext-receivers"},
	}
	gotCore := got
	gotCore.Notes, gotCore.Rejected = nil, nil
	if !reflect.DeepEqual(gotCore, want) {
		t.Fatalf("settings = %+v want %+v", gotCore, want)
	}
	if len(got.Rejected) != 2 {
		t.Fatalf("unsupported and krit-owned arguments must be reported, got %v", got.Rejected)
	}
	log := out.String()
	for _, want := range []string{"-language-version=2.1", "-opt-in=kotlin.RequiresOptIn", "not applied: -Werror", "-Xplugin=/x.jar (managed by krit)"} {
		if !strings.Contains(log, want) {
			t.Errorf("verbose output lacks %q:\n%s", want, log)
		}
	}

	// Disabled model discovery means the embedded defaults, with no model read.
	var disabled bytes.Buffer
	if got := loadGradleKotlinSettings([]string{root}, "", true, true, &disabled); !got.IsZero() {
		t.Fatalf("disabled model produced settings: %+v", got)
	}
	if !strings.Contains(disabled.String(), "embedded compiler defaults (Gradle model discovery is disabled)") {
		t.Fatalf("a disabled model must still state the default: %q", disabled.String())
	}
}

func TestLoadGradleKotlinSettingsDoesNotMixSourceSets(t *testing.T) {
	root := t.TempDir()
	writeKotlinModel(t, root, map[string]string{
		"main": `{"languageVersion":"2.0","apiVersion":"2.0","optIn":["a.A"]}`,
		"test": `{"languageVersion":"2.1","apiVersion":"2.1"}`,
	})
	var out bytes.Buffer
	got := loadGradleKotlinSettings([]string{root}, "", false, true, &out)
	if got.LanguageVersion != "" || got.APIVersion != "" || len(got.OptIns) != 0 {
		t.Fatalf("source sets disagree, so none of their settings may be applied to the whole repo: %+v", got)
	}
	if !strings.Contains(out.String(), "languageVersion differs across source sets") || !strings.Contains(out.String(), ":test") {
		t.Fatalf("the disagreement must be visible:\n%s", out.String())
	}
}

func TestLoadGradleKotlinSettingsWithoutModelIsExplicitDefault(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "settings.gradle.kts"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	got := loadGradleKotlinSettings([]string{root}, "", false, true, &out)
	if !got.IsZero() || got.Identity() != "" {
		t.Fatalf("no model: %+v", got)
	}
	if !strings.Contains(out.String(), "embedded compiler defaults") || !strings.Contains(out.String(), "no Gradle model found") {
		t.Fatalf("the default must be stated, not silent:\n%s", out.String())
	}
}

func TestDaemonForwardsKotlinSettings(t *testing.T) {
	f := freshScanFlags(t)
	f.modelKotlin = gradlemodel.KotlinSettings{LanguageVersion: "2.1", APIVersion: "2.1", Flags: []string{"-Xcontext-receivers"}}
	args := buildDaemonAnalyzeArgs(f, []string{t.TempDir()})
	if args.OracleKotlin.Identity() != f.modelKotlin.Identity() || args.OracleKotlin.Identity() == "" {
		t.Fatalf("daemon request lost the Kotlin settings: %+v", args.OracleKotlin)
	}
}

// krit.yml's oracle.compilerPlugins reach the compile settings without a
// Gradle model, filtered like the model's, and verbose output says what loads.
func TestWithConfiguredCompilerPlugins(t *testing.T) {
	dir := t.TempDir()
	jar := filepath.Join(dir, "kotlin-serialization-compiler-plugin-embeddable-2.4.21.jar")
	if err := os.WriteFile(jar, []byte("jar"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.NewConfigFromData(map[string]interface{}{
		"oracle": map[string]interface{}{
			"compilerPlugins":       []interface{}{jar, filepath.Join(dir, "kotlin-scripting-compiler-embeddable-2.4.21.jar")},
			"compilerPluginOptions": []interface{}{"plugin:org.jetbrains.kotlinx.serialization:disableIntrinsic=true"},
		},
	})
	var out bytes.Buffer
	got := withConfiguredCompilerPlugins(gradlemodel.KotlinSettings{}, cfg, true, &out)
	if !reflect.DeepEqual(got.Plugins.Classpath, []string{jar}) || len(got.Plugins.Options) != 1 {
		t.Fatalf("plugins = %+v", got.Plugins)
	}
	if !strings.Contains(out.String(), "kotlin compiler plugins: kotlin-serialization-compiler-plugin-embeddable-2.4.21.jar") ||
		!strings.Contains(out.String(), "scripting support") {
		t.Fatalf("verbose output = %q", out.String())
	}
	none := withConfiguredCompilerPlugins(gradlemodel.KotlinSettings{LanguageVersion: "2.3"}, config.NewConfigFromData(nil), true, &out)
	if !none.Plugins.IsZero() || none.LanguageVersion != "2.3" {
		t.Fatalf("no configured plugins changed the settings: %+v", none)
	}
	if withConfiguredCompilerPlugins(gradlemodel.KotlinSettings{}, nil, false, &out).IsZero() != true {
		t.Fatal("nil config")
	}
}
