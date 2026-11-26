package librarymodel

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// serialProfileFromGradlePaths is the pre-parallel ProfileFromGradlePaths loop.
func serialProfileFromGradlePaths(paths []string) ProjectProfile {
	var profile ProjectProfile
	catalogs := make(map[string]VersionCatalog)
	discovery := NewCatalogDiscovery()
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var catalog VersionCatalog
		if catalogSources, ok := discovery.FindVersionCatalogSourcesForGradlePath(path); ok {
			catalogKey := catalogSourceCacheKey(catalogSources)
			catalog, ok = catalogs[catalogKey]
			if !ok {
				catalog, _ = loadVersionCatalogFromSources(catalogSources)
				catalogs[catalogKey] = catalog
			}
		}
		profile.Merge(ProfileFromGradleContentWithCatalog(path, string(content), catalog))
	}
	applyFoundrySdkProperties(&profile, paths)
	return profile
}

func TestProfileFromGradlePathsMatchesSerialMerge(t *testing.T) {
	root := t.TempDir()
	gradleDir := filepath.Join(root, "gradle")
	if err := os.MkdirAll(gradleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(gradleDir, "libs.versions.toml"), `
[versions]
agp = "8.7.0"
kotlin = "2.0.21"
room = "2.6.1"

[plugins]
android-library = { id = "com.android.library", version.ref = "agp" }
kotlin-android = { id = "org.jetbrains.kotlin.android", version.ref = "kotlin" }

[libraries]
room-runtime = { module = "androidx.room:room-runtime", version.ref = "room" }
`)
	paths := []string{filepath.Join(root, "missing", "build.gradle.kts")}
	for i := 0; i < 24; i++ {
		dir := filepath.Join(root, fmt.Sprintf("m%02d", i))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "build.gradle.kts")
		writeTestFile(t, path, fmt.Sprintf(`
plugins {
    alias(libs.plugins.android.library)
    alias(libs.plugins.kotlin.android)
}

android {
    compileSdk = %d
    defaultConfig { minSdk = %d }
}

dependencies {
    implementation(libs.room.runtime)
    implementation("com.example:lib%02d:1.%d.0")
    implementation(libs.unknown.alias%d)
}
`, 30+i%5, 21+i%3, i, i, i%4))
		paths = append(paths, path)
	}
	got := ProfileFromGradlePaths(paths)
	want := serialProfileFromGradlePaths(paths)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parallel profile differs from serial merge:\n got %#v\nwant %#v", got, want)
	}
	if len(got.Dependencies) == 0 {
		t.Fatal("expected dependencies")
	}
}
