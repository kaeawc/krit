package devjar

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSourceHashTracksInputsAndTool(t *testing.T) {
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
	for _, rel := range []string{
		"LICENSE", "tools/THIRD_PARTY_NOTICES.txt",
		"tools/krit-rule-api/build.gradle.kts", "tools/krit-rule-api/settings.gradle.kts",
		"tools/krit-rule-api/gradle.properties", "tools/krit-rule-api/gradle/wrapper/gradle-wrapper.properties",
		"tools/krit-rule-api/src/main/kotlin/Api.kt",
	} {
		write(rel, "original")
	}
	for _, tool := range []string{"krit-fir", "krit-types"} {
		for _, rel := range []string{"build.gradle.kts", "settings.gradle.kts", "gradle.properties", "gradle/wrapper/gradle-wrapper.properties", "src/main/kotlin/Main.kt"} {
			write(filepath.Join("tools", tool, rel), "original")
		}
	}
	first, err := SourceHash(root, "krit-fir")
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 20 {
		t.Fatalf("hash length = %d, want 20", len(first))
	}
	again, err := SourceHash(root, "krit-fir")
	if err != nil || again != first {
		t.Fatalf("same inputs: hash = %q, err = %v; want %q", again, err, first)
	}
	types, err := SourceHash(root, "krit-types")
	if err != nil || types == first {
		t.Fatalf("different tool: hash = %q, err = %v", types, err)
	}
	write("tools/krit-fir/src/main/kotlin/Main.kt", "changed")
	changed, err := SourceHash(root, "krit-fir")
	if err != nil || changed == first {
		t.Fatalf("changed source: hash = %q, err = %v", changed, err)
	}
	unchangedTypes, err := SourceHash(root, "krit-types")
	if err != nil || unchangedTypes != types {
		t.Fatalf("unrelated tool: hash = %q, err = %v; want %q", unchangedTypes, err, types)
	}
	write("tools/krit-rule-api/gradle.properties", "changed")
	changedAgain, err := SourceHash(root, "krit-fir")
	if err != nil || changedAgain == changed {
		t.Fatalf("shared build input: hash = %q, err = %v", changedAgain, err)
	}
}

func TestSourceHashRejectsUnknownTool(t *testing.T) {
	if _, err := SourceHash(t.TempDir(), "unknown"); err == nil {
		t.Fatal("expected unsupported tool error")
	}
}

func TestCacheDirHonorsOverride(t *testing.T) {
	override := t.TempDir()
	t.Setenv(DirEnv, override)
	if got := CacheDir(); got != override {
		t.Fatalf("CacheDir() = %q, want override %q", got, override)
	}
	path := CachePath("krit-fir", nil)
	if path == "" {
		t.Fatal("expected a cache path inside the krit checkout")
	}
	if filepath.Dir(filepath.Dir(path)) != override || filepath.Base(path) != "krit-fir.jar" {
		t.Fatalf("CachePath() = %q, want <override>/<hash>/krit-fir.jar", path)
	}

	home := t.TempDir()
	t.Setenv(DirEnv, "")
	t.Setenv("HOME", home)
	if got, want := CacheDir(), filepath.Join(home, ".krit", "jars", "dev"); got != want {
		t.Fatalf("CacheDir() without override = %q, want %q", got, want)
	}
}
