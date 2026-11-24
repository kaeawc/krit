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

// A build file rewritten just before the stat stays out of the memo even when the parse takes
// longer than sdkRacyWindow: raciness is judged against when the read started.
func TestSDKLevelsSlowReadOfFreshInputIsNotMemoized(t *testing.T) {
	dir := t.TempDir()
	build := filepath.Join(dir, "build.gradle.kts")
	if err := os.WriteFile(build, []byte("android {\n    defaultConfig {\n        minSdk = 21\n    }\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mod := time.Now()
	if err := os.Chtimes(build, mod, mod); err != nil {
		t.Fatal(err)
	}

	// The first clock read is the read start; every later read is past the racy window.
	calls := 0
	sdkNow = func() time.Time {
		calls++
		if calls == 1 {
			return mod.Add(time.Second)
		}
		return mod.Add(10 * sdkRacyWindow)
	}
	t.Cleanup(func() { sdkNow = time.Now })

	if got := sdkLevelsDeclaredIn(dir).MinSdk; got != 21 {
		t.Fatalf("minSdk = %d, want 21", got)
	}
	sdkDeclaredMu.RLock()
	_, memoized := sdkDeclaredMemo[dir]
	sdkDeclaredMu.RUnlock()
	if memoized {
		t.Fatal("input modified within the racy window of the read start was memoized")
	}
}
