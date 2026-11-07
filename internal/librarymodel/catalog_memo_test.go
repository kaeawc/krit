package librarymodel

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadVersionCatalogForGradlePathReloadsAfterEdit(t *testing.T) {
	dir := t.TempDir()
	gradleDir := filepath.Join(dir, "gradle")
	if err := os.MkdirAll(gradleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	catalogPath := filepath.Join(gradleDir, "libs.versions.toml")
	write := func(content string, mod time.Time) {
		t.Helper()
		if err := os.WriteFile(catalogPath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(catalogPath, mod, mod); err != nil {
			t.Fatal(err)
		}
	}
	buildPath := filepath.Join(dir, "build.gradle.kts")
	now := time.Now()
	write("[plugins]\nfoo = { id = \"com.example.foo\" }\n", now)

	first, ok := LoadVersionCatalogForGradlePath(buildPath)
	if !ok || first.Plugins["foo"].ID != "com.example.foo" {
		t.Fatalf("first load = %+v ok=%v", first.Plugins, ok)
	}
	again, _ := LoadVersionCatalogForGradlePath(buildPath)
	if again.Plugins["foo"].ID != "com.example.foo" {
		t.Fatalf("memoized load = %+v", again.Plugins)
	}

	write("[plugins]\nbar = { id = \"com.example.bar\" }\n", now.Add(time.Hour))
	edited, ok := LoadVersionCatalogForGradlePath(buildPath)
	if !ok {
		t.Fatal("edited catalog not found")
	}
	if _, stale := edited.Plugins["foo"]; stale || edited.Plugins["bar"].ID != "com.example.bar" {
		t.Fatalf("edited load = %+v", edited.Plugins)
	}
}
