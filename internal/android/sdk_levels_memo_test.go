package android

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestResolveSDKLevelsSeesEditedBuildFile(t *testing.T) {
	dir := t.TempDir()
	build := filepath.Join(dir, "build.gradle.kts")
	write := func(content string, mod time.Time) {
		t.Helper()
		if err := os.WriteFile(build, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(build, mod, mod); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	source := filepath.Join(dir, "src", "main", "Foo.kt")

	write("android {\n    defaultConfig {\n        minSdk = 21\n    }\n}\n", now)
	if got := ResolveSDKLevels(source).MinSdk; got != 21 {
		t.Fatalf("minSdk = %d, want 21", got)
	}
	if got := ResolveSDKLevels(source).MinSdk; got != 21 {
		t.Fatalf("memoized minSdk = %d, want 21", got)
	}
	write("android {\n    defaultConfig {\n        minSdk = 26\n    }\n}\n", now.Add(time.Hour))
	if got := ResolveSDKLevels(source).MinSdk; got != 26 {
		t.Fatalf("edited minSdk = %d, want 26", got)
	}
}
