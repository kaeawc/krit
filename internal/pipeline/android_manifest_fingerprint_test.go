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

// RelativeClassResolution reads the module's build script and version catalog, so editing either must change the
// fingerprint of the module's res directories.
func TestResDirFingerprintCoversBuildScripts(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fingerprint := func() string {
		fps := newResDirFingerprints(filepath.Join(t.TempDir(), "cache"), 1)
		return fps.fingerprint(filepath.Join(root, "app", "src", "main", "res"))
	}
	write("app/src/main/res/values/strings.xml", "<resources/>")
	write("app/build.gradle.kts", `android { namespace = "com.example" }`)
	base := fingerprint()
	write("app/build.gradle.kts", `android { namespace = "com.example"; defaultConfig { applicationId = "com.other" } }`)
	edited := fingerprint()
	if edited == base {
		t.Fatal("editing the build script did not change the res directory fingerprint")
	}
	write("gradle/libs.versions.toml", "[plugins]\nandroid-library = { id = \"com.android.library\" }\n")
	if fingerprint() == edited {
		t.Fatal("adding a version catalog did not change the res directory fingerprint")
	}
}
