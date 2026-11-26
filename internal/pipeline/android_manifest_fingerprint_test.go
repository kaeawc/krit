package pipeline

import (
	"os"
	"path/filepath"
	"testing"
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
