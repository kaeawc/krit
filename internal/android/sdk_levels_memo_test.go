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

// TestResolveSDKLevelsSeesSameTickRewrite covers the racily-clean case: a
// same-size rewrite that lands in the same coarse mtime tick keeps the stamp
// the memo saw, so a recently modified build file must not be memoized. Once
// its mtime is old enough, the result is memoized again.
func TestResolveSDKLevelsSeesSameTickRewrite(t *testing.T) {
	dir := t.TempDir()
	build := filepath.Join(dir, "build.gradle.kts")
	write := func(minSdk string, mod time.Time) {
		t.Helper()
		if err := os.WriteFile(build, []byte("android {\n    defaultConfig {\n        minSdk = "+minSdk+"\n    }\n}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(build, mod, mod); err != nil {
			t.Fatal(err)
		}
	}
	source := filepath.Join(dir, "src", "main", "Foo.kt")
	tick := time.Now()

	write("21", tick)
	if got := ResolveSDKLevels(source).MinSdk; got != 21 {
		t.Fatalf("minSdk = %d, want 21", got)
	}
	write("26", tick)
	if got := ResolveSDKLevels(source).MinSdk; got != 26 {
		t.Fatalf("same-tick rewrite: minSdk = %d, want 26", got)
	}

	old := tick.Add(-time.Hour)
	write("23", old)
	if got := ResolveSDKLevels(source).MinSdk; got != 23 {
		t.Fatalf("minSdk = %d, want 23", got)
	}
	sdkDeclaredMu.RLock()
	entry, ok := sdkDeclaredMemo[dir]
	sdkDeclaredMu.RUnlock()
	if !ok || entry.levels.MinSdk != 23 {
		t.Fatalf("settled build file was not memoized: ok=%v levels=%+v", ok, entry.levels)
	}
}
