package librarymodel

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/kaeawc/krit/internal/buildid"
	"github.com/kaeawc/krit/internal/hashutil"
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
	// The load below stands in for a later krit run, which starts with an
	// empty content-hash memo (runner_state resets it). Without the reset, the
	// same-size rewrite above can land in the same coarse mtime tick and reuse
	// the hash memoized for the old content.
	hashutil.ResetDefault()
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

func TestProjectProfileCache_MissesAcrossBuilds(t *testing.T) {
	root := t.TempDir()
	cacheDir := ProjectProfileCacheDir(root)
	buildPath := filepath.Join(root, "app", "build.gradle.kts")
	if err := os.MkdirAll(filepath.Dir(buildPath), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, buildPath, "android {\n    compileSdk = 35\n}\n")

	buildid.OverrideTokenForTest(t, "rev:build-a")
	if err := SaveProjectProfileCache(cacheDir, []string{buildPath}, ProfileFromGradlePaths([]string{buildPath})); err != nil {
		t.Fatalf("SaveProjectProfileCache: %v", err)
	}
	if _, ok := LoadProjectProfileCache(cacheDir, []string{buildPath}); !ok {
		t.Fatal("same build must hit")
	}
	buildid.OverrideTokenForTest(t, "rev:build-b")
	if _, ok := LoadProjectProfileCache(cacheDir, []string{buildPath}); ok {
		t.Fatal("a profile written by another build must miss: the Gradle parsing code may have changed")
	}
}

func TestProjectProfileCache_ConcurrentSavesLeaveValidEntry(t *testing.T) {
	root := t.TempDir()
	cacheDir := ProjectProfileCacheDir(root)
	buildPath := filepath.Join(root, "build.gradle.kts")
	writeTestFile(t, buildPath, "android {\n    compileSdk = 35\n}\n")
	paths := []string{buildPath}
	profile := ProfileFromGradlePaths(paths)

	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := SaveProjectProfileCache(cacheDir, paths, profile); err != nil {
				t.Errorf("SaveProjectProfileCache: %v", err)
			}
		}()
	}
	wg.Wait()

	if _, ok := LoadProjectProfileCache(cacheDir, paths); !ok {
		t.Fatal("expected cache hit after concurrent saves")
	}
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Errorf("leftover temp file %s", e.Name())
		}
	}
}

// TestProjectProfileCache_TornEntryIsMiss covers an entry damaged by a crash
// (cache writes skip fsync): it must be a miss, never a wrong profile.
func TestProjectProfileCache_TornEntryIsMiss(t *testing.T) {
	root := t.TempDir()
	cacheDir := ProjectProfileCacheDir(root)
	buildPath := filepath.Join(root, "app", "build.gradle.kts")
	if err := os.MkdirAll(filepath.Dir(buildPath), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, buildPath, "android {\n    compileSdk = 35\n}\n")
	paths := []string{buildPath}
	if err := SaveProjectProfileCache(cacheDir, paths, ProfileFromGradlePaths(paths)); err != nil {
		t.Fatalf("SaveProjectProfileCache: %v", err)
	}
	entry := projectProfileCachePath(cacheDir, paths)
	valid, err := os.ReadFile(entry)
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{
		"empty":       {},
		"truncated":   valid[:len(valid)/2],
		"zero-filled": make([]byte, len(valid)),
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(entry, data, 0o644); err != nil {
				t.Fatal(err)
			}
			if _, ok := LoadProjectProfileCache(cacheDir, paths); ok {
				t.Fatal("damaged entry loaded as a hit")
			}
		})
	}
}
