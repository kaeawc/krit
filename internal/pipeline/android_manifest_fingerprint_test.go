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
