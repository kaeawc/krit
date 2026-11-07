package android

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSDKLevelResolverMatchesResolveSDKLevelsAcrossNestedDirs(t *testing.T) {
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
	write("app/build.gradle.kts", "android { defaultConfig { minSdk = 26\n targetSdk = 34 } }\n")
	write("app/src/main/kotlin/a/b/A.kt", "package a.b\n")
	write("app/src/main/kotlin/a/B.kt", "package a\n")
	write("lib/src/main/AndroidManifest.xml", `<manifest xmlns:android="http://schemas.android.com/apk/res/android"><uses-sdk android:minSdkVersion="21" android:targetSdkVersion="33"/></manifest>`)
	write("lib/src/main/kotlin/L.kt", "class L\n")
	write("plain/src/main/kotlin/P.kt", "class P\n")

	var resolver SDKLevelResolver
	for _, rel := range []string{
		"app/src/main/kotlin/a/b/A.kt", "app/src/main/kotlin/a/B.kt", "app/src/main/kotlin/a/b/A.kt",
		"lib/src/main/kotlin/L.kt", "plain/src/main/kotlin/P.kt",
	} {
		path := filepath.Join(root, rel)
		got, want := resolver.Resolve(path), ResolveSDKLevels(path)
		if got != want {
			t.Errorf("%s: resolver %+v, direct %+v", rel, got, want)
		}
	}
	if got := resolver.Resolve(filepath.Join(root, "app/src/main/kotlin/a/b/A.kt")); got.IsZero() || got != ResolveSDKLevels(filepath.Join(root, "app/build.gradle.kts")) {
		t.Errorf("nested file did not inherit the module build file: %+v", got)
	}
	if got := resolver.Resolve("A.kt"); !got.IsZero() {
		t.Errorf("file in the working directory should resolve to zero levels, got %+v", got)
	}
}
