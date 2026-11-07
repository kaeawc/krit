package pipeline

import (
	"os"
	"path/filepath"
	"testing"
)

// Gradle rules read gradle.properties (android.builtInKotlin, obsolete Kotlin keys), so an edit there
// must change the cached findings keys of the scripts that see it.
func TestGradleKeysFoldInGradleProperties(t *testing.T) {
	root := t.TempDir()
	module := filepath.Join(root, "app")
	if err := os.MkdirAll(module, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "settings.gradle.kts"), "include(\":app\")\n")
	script := filepath.Join(module, "build.gradle.kts")
	write(script, "plugins {}\n")
	props := filepath.Join(root, "gradle.properties")

	in := AndroidInput{RuleHash: "rules"}
	paths := []string{filepath.Join(root, "settings.gradle.kts"), script}

	noProps := in.gradleKey(script, "content")
	bundleNoProps, ok := (AndroidPhase{}).gradleBundleFingerprint(paths)
	if !ok {
		t.Fatal("bundle fingerprint failed without gradle.properties")
	}

	write(props, "android.builtInKotlin=false\n")
	withProps := in.gradleKey(script, "content")
	bundleWithProps, _ := (AndroidPhase{}).gradleBundleFingerprint(paths)
	if withProps == noProps || bundleWithProps == bundleNoProps {
		t.Fatalf("adding gradle.properties must change the keys: file %v bundle %v", withProps != noProps, bundleWithProps != bundleNoProps)
	}

	write(props, "android.builtInKotlin=true  # edited\n")
	edited := in.gradleKey(script, "content")
	bundleEdited, _ := (AndroidPhase{}).gradleBundleFingerprint(paths)
	if edited == withProps || bundleEdited == bundleWithProps {
		t.Fatalf("editing gradle.properties must change the keys: file %v bundle %v", edited != withProps, bundleEdited != bundleWithProps)
	}

	if in.gradleKey(script, "content") != edited {
		t.Fatal("the key must be stable while nothing changes")
	}
}
