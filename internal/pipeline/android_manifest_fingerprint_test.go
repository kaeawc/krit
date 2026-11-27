package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/android"
)

// The Wear resource checks read the source set's and the main AndroidManifest.xml to decide whether a res directory
// belongs to a Wear app, so editing either must change the res directory fingerprint that keys their findings.
func TestResDirFingerprintCoversModuleManifests(t *testing.T) {
	src := filepath.Join(t.TempDir(), "app", "src")
	write := func(rel, content string) {
		t.Helper()
		path := filepath.Join(src, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fingerprint := func(resDir string) string {
		fps := newResDirFingerprints(filepath.Join(t.TempDir(), "cache"), 1)
		return fps.fingerprint(filepath.Join(src, resDir))
	}
	write("main/res/values/themes.xml", "<resources/>")
	write("debug/res/values/themes.xml", "<resources/>")
	mainBase, debugBase := fingerprint("main/res"), fingerprint("debug/res")

	write("main/AndroidManifest.xml", `<manifest><uses-feature android:name="android.hardware.type.watch"/></manifest>`)
	mainWear, debugWear := fingerprint("main/res"), fingerprint("debug/res")
	if mainWear == mainBase {
		t.Fatal("adding the main manifest did not change the main res directory fingerprint")
	}
	if debugWear == debugBase {
		t.Fatal("adding the main manifest did not change the debug res directory fingerprint")
	}
	write("main/AndroidManifest.xml", `<manifest/>`)
	if fingerprint("main/res") == mainWear || fingerprint("debug/res") == debugWear {
		t.Fatal("editing the main manifest did not change the res directory fingerprints")
	}
	debugPhone := fingerprint("debug/res")
	write("debug/AndroidManifest.xml", `<manifest/>`)
	if fingerprint("debug/res") == debugPhone {
		t.Fatal("adding the debug manifest did not change the debug res directory fingerprint")
	}
}

// A variant manifest with no res directory of its own still has its tile previews read from main's res, and its
// rules read the main manifest, so editing either must change the variant manifest's module fingerprint.
func TestManifestModuleFingerprintCoversSourceSetResources(t *testing.T) {
	src := filepath.Join(t.TempDir(), "app", "src")
	write := func(rel, content string) {
		t.Helper()
		path := filepath.Join(src, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("main/AndroidManifest.xml", "<manifest/>")
	write("debug/AndroidManifest.xml", "<manifest/>")
	write("main/res/drawable/tile_preview.png", "one")
	write("test/res/drawable/tile_preview.png", "test")
	in := AndroidInput{Project: &android.Project{ResDirs: []string{
		filepath.Join(src, "main", "res"), filepath.Join(src, "test", "res")}}}
	debug := filepath.Join(src, "debug", "AndroidManifest.xml")
	fp := func() string {
		fps := newResDirFingerprints(filepath.Join(t.TempDir(), "cache"), 1)
		return in.manifestModuleFP(debug, &fps)
	}
	base := fp()
	write("main/res/drawable/tile_preview.png", "two, longer")
	afterRes := fp()
	if afterRes == base {
		t.Fatal("editing main's res did not change the debug manifest's module fingerprint")
	}
	write("main/AndroidManifest.xml", `<manifest><uses-feature android:name="android.hardware.type.watch"/></manifest>`)
	afterManifest := fp()
	if afterManifest == afterRes {
		t.Fatal("editing the main manifest did not change the debug manifest's module fingerprint")
	}
	write("test/res/drawable/tile_preview.png", "test, edited")
	if fp() != afterManifest {
		t.Fatal("a test source set's res changed the module fingerprint")
	}
}

// Switching a module between the application and library plugins must change the per-file cache key, since rules
// such as AnnotateVersionCheck report only in libraries; a dependency edit must not.
func TestProjectFactsCoverModulePlugins(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "app", "build.gradle.kts")
	write := func(content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(script, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	project := &android.Project{GradlePaths: []string{script}}
	write("plugins { id(\"com.android.application\") }\ndependencies { implementation(\"a:b:1\") }\n")
	app := withProjectFacts("rules", project, "")
	write("plugins { id(\"com.android.application\") }\ndependencies { implementation(\"a:b:2\") }\n")
	if withProjectFacts("rules", project, "") != app {
		t.Fatal("a dependency edit changed the per-file cache key")
	}
	write("plugins { alias(libs.plugins.android.library) }\ndependencies { implementation(\"a:b:2\") }\n")
	if withProjectFacts("rules", project, "") == app {
		t.Fatal("switching to the library plugin did not change the per-file cache key")
	}
}

// The plugins fingerprint spells build scripts relative to the directory they share, so the same checkout at another
// absolute path keeps its per-file cache keys (#1662), while a module's plugins still count under its own path.
func TestModulePluginsFingerprintIgnoresCheckoutRoot(t *testing.T) {
	fingerprint := func(root string, scripts map[string]string) string {
		t.Helper()
		project := &android.Project{}
		for rel, content := range scripts {
			path := filepath.Join(root, rel)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			project.GradlePaths = append(project.GradlePaths, path)
		}
		return modulePluginsFingerprint(project)
	}
	scripts := map[string]string{
		"build.gradle.kts":     "plugins { alias(libs.plugins.android.application) apply false }\n",
		"app/build.gradle.kts": "plugins { id(\"com.android.application\") }\n",
		"lib/build.gradle.kts": "plugins { id(\"com.android.library\") }\n",
	}
	first := fingerprint(filepath.Join(t.TempDir(), "one"), scripts)
	if first == "" {
		t.Fatal("no plugins fingerprint for a project that applies plugins")
	}
	if moved := fingerprint(filepath.Join(t.TempDir(), "elsewhere", "two"), scripts); moved != first {
		t.Fatalf("the same project at another root got plugins fingerprint %s, want %s", moved, first)
	}
	swapped := map[string]string{
		"build.gradle.kts":     scripts["build.gradle.kts"],
		"app/build.gradle.kts": scripts["lib/build.gradle.kts"],
		"lib/build.gradle.kts": scripts["app/build.gradle.kts"],
	}
	if fingerprint(filepath.Join(t.TempDir(), "one"), swapped) == first {
		t.Fatal("swapping which module is the library did not change the plugins fingerprint")
	}
}
