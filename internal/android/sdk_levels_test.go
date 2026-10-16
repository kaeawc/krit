package android

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSDKFile(t *testing.T, root, rel, content string) string {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAncestorDirs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{".", 0},
		{"/a", 2},
		{"/a/b", 3},
	}
	for _, c := range cases {
		got := ancestorDirs(c.in)
		if len(got) != c.want {
			t.Errorf("ancestorDirs(%q) returned %d entries, want %d (%v)", c.in, len(got), c.want, got)
		}
	}
	dirs := ancestorDirs(filepath.Join("/x", "y", "z"))
	if len(dirs) == 0 || dirs[0] != filepath.Clean("/x/y/z") {
		t.Errorf("ancestorDirs leading entry mismatch: %v", dirs)
	}
}

func TestResolveSDKLevels(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSDKFile(t, root, "build.gradle.kts", "android {\n    defaultConfig {\n        minSdk = 21\n        targetSdk = 30\n    }\n}\n")
	writeSDKFile(t, root, "app/build.gradle", "android {\n    defaultConfig {\n        minSdkVersion 23\n        targetSdkVersion 34\n    }\n}\n")
	// A build file that declares no SDK level does not stop the walk.
	writeSDKFile(t, root, "lib/build.gradle.kts", "plugins { id(\"com.android.library\") }\n")
	writeSDKFile(t, root, "legacy/build.gradle", "apply plugin: 'com.android.library'\n")
	writeSDKFile(t, root, "legacy/src/main/AndroidManifest.xml",
		"<manifest xmlns:android=\"http://schemas.android.com/apk/res/android\">\n"+
			"    <uses-sdk android:minSdkVersion=\"16\" android:targetSdkVersion=\"28\" />\n</manifest>\n")

	cases := []struct {
		file string
		want SDKLevels
	}{
		{"app/src/main/kotlin/A.kt", SDKLevels{MinSdk: 23, TargetSdk: 34}},
		{"lib/src/main/kotlin/L.kt", SDKLevels{MinSdk: 21, TargetSdk: 30}},
		{"legacy/src/main/kotlin/M.kt", SDKLevels{MinSdk: 16, TargetSdk: 28}},
	}
	var resolver SDKLevelResolver
	for _, c := range cases {
		path := filepath.Join(root, c.file)
		if got := ResolveSDKLevels(path); got != c.want {
			t.Errorf("ResolveSDKLevels(%s) = %+v, want %+v", c.file, got, c.want)
		}
		if got := resolver.Resolve(path); got != c.want {
			t.Errorf("SDKLevelResolver.Resolve(%s) = %+v, want %+v", c.file, got, c.want)
		}
	}
	if got := ResolveSDKLevels("Bare.kt"); !got.IsZero() {
		t.Errorf("a file with no directory resolves nothing, got %+v", got)
	}
}
