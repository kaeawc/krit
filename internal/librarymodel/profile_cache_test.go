package librarymodel

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectProfileCache_HitAndGradleInvalidation(t *testing.T) {
	root := t.TempDir()
	cacheDir := ProjectProfileCacheDir(root)
	buildPath := filepath.Join(root, "app", "build.gradle.kts")
	if err := os.MkdirAll(filepath.Dir(buildPath), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, buildPath, `
android {
    compileSdk = 35
}
`)
	profile := ProfileFromGradlePaths([]string{buildPath})
	if err := SaveProjectProfileCache(cacheDir, []string{buildPath}, profile); err != nil {
		t.Fatalf("SaveProjectProfileCache: %v", err)
	}
	got, ok := LoadProjectProfileCache(cacheDir, []string{buildPath})
	if !ok {
		t.Fatal("expected warm project profile cache hit")
	}
	if got.CompileSdkVersion != 35 {
		t.Fatalf("CompileSdkVersion = %d, want 35", got.CompileSdkVersion)
	}

	writeTestFile(t, buildPath, `
android {
    compileSdk = 36
}
`)
	if _, ok := LoadProjectProfileCache(cacheDir, []string{buildPath}); ok {
		t.Fatal("expected cache miss after Gradle file change")
	}
}

func TestProjectProfileCache_MissesWhenNewCatalogAppears(t *testing.T) {
	root := t.TempDir()
	cacheDir := ProjectProfileCacheDir(root)
	buildPath := filepath.Join(root, "app", "build.gradle.kts")
	if err := os.MkdirAll(filepath.Dir(buildPath), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, buildPath, `
dependencies {
    implementation(libs.room.runtime)
}
`)
	profile := ProfileFromGradlePaths([]string{buildPath})
	if err := SaveProjectProfileCache(cacheDir, []string{buildPath}, profile); err != nil {
		t.Fatalf("SaveProjectProfileCache: %v", err)
	}
	if _, ok := LoadProjectProfileCache(cacheDir, []string{buildPath}); !ok {
		t.Fatal("expected cache hit before catalog appears")
	}

	if err := os.MkdirAll(filepath.Join(root, "gradle"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "gradle", "libs.versions.toml"), `
[libraries]
room-runtime = { module = "androidx.room:room-runtime", version = "2.7.0" }
`)
	if _, ok := LoadProjectProfileCache(cacheDir, []string{buildPath}); ok {
		t.Fatal("expected cache miss after new ancestor version catalog appears")
	}
}
