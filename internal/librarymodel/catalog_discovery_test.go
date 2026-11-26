package librarymodel

import (
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
)

// writeNestedBuildsRepo lays out a repository whose root provides a settings
// plugin that defines the catalog, and two nested builds that apply the plugin
// and include the root with includeBuild(".."). It returns their build files.
func writeNestedBuildsRepo(t *testing.T) []string {
	t.Helper()
	root := t.TempDir()
	pluginSource := filepath.Join(root, "src", "main", "kotlin", "com", "example")
	if err := os.MkdirAll(pluginSource, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "settings.gradle.kts"), "rootProject.name = \"root\"\n")
	writeTestFile(t, filepath.Join(root, "build.gradle.kts"), `
gradlePlugin {
    plugins {
        create("catalogSettings") {
            id = "com.example.catalog-settings"
            implementationClass = "com.example.CatalogSettingsPlugin"
        }
    }
}
`)
	writeTestFile(t, filepath.Join(pluginSource, "CatalogSettingsPlugin.kt"), `
package com.example

class CatalogSettingsPlugin {
    fun apply(settings: Any) {
        dependencyResolutionManagement {
            versionCatalogs {
                create("libs") {
                    library("room-runtime", "androidx.room", "room-runtime").version("2.7.0")
                }
            }
        }
    }
}
`)
	var builds []string
	for _, nested := range []string{"samples", "benchmark"} {
		writeTestFileMkdir(t, filepath.Join(root, nested, "settings.gradle.kts"), `
plugins {
    id("com.example.catalog-settings")
}
includeBuild("..")
`)
		for _, module := range []string{"a", "b"} {
			build := filepath.Join(root, nested, module, "build.gradle.kts")
			writeTestFileMkdir(t, build, "dependencies { implementation(libs.room.runtime) }\n")
			builds = append(builds, build)
		}
	}
	return builds
}

func writeTestFileMkdir(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, path, content)
}

func countBuildFileWalks(t *testing.T) *atomic.Int32 {
	t.Helper()
	var walks atomic.Int32
	prev := walkGradleBuildFilesFn
	walkGradleBuildFilesFn = func(root string) []string {
		walks.Add(1)
		return walkGradleBuildFiles(root)
	}
	t.Cleanup(func() { walkGradleBuildFilesFn = prev })
	return &walks
}

func TestCatalogDiscoveryMatchesUncachedDiscovery(t *testing.T) {
	builds := writeNestedBuildsRepo(t)
	discovery := NewCatalogDiscovery()
	for _, build := range builds {
		wantSources, wantOK := FindVersionCatalogSourcesForGradlePath(build)
		gotSources, gotOK := discovery.FindVersionCatalogSourcesForGradlePath(build)
		if gotOK != wantOK || !reflect.DeepEqual(gotSources, wantSources) {
			t.Fatalf("%s: memoized sources = %v, %v; uncached = %v, %v", build, gotSources, gotOK, wantSources, wantOK)
		}
		catalog, ok := discovery.LoadVersionCatalogForGradlePath(build)
		if !ok || catalog.Libraries["room-runtime"].Name != "room-runtime" {
			t.Fatalf("%s: catalog = %#v, %v; want room-runtime from the root settings plugin", build, catalog.Libraries, ok)
		}
	}
}

// Both nested builds include the same root, so one run walks it once, however
// many build files ask for their catalog.
func TestCatalogDiscoveryWalksEachIncludedBuildOnce(t *testing.T) {
	builds := writeNestedBuildsRepo(t)
	walks := countBuildFileWalks(t)
	discovery := NewCatalogDiscovery()
	for range 3 {
		for _, build := range builds {
			if _, ok := discovery.LoadVersionCatalogForGradlePath(build); !ok {
				t.Fatalf("%s: no catalog", build)
			}
		}
	}
	if got := walks.Load(); got != 1 {
		t.Fatalf("included build walked %d times, want 1", got)
	}
}

func TestCatalogDiscoveryConcurrentLookups(t *testing.T) {
	builds := writeNestedBuildsRepo(t)
	discovery := NewCatalogDiscovery()
	var wg sync.WaitGroup
	for range 8 {
		for _, build := range builds {
			wg.Go(func() {
				if _, ok := discovery.LoadVersionCatalogForGradlePath(build); !ok {
					t.Errorf("%s: no catalog", build)
				}
			})
		}
	}
	wg.Wait()
}

func TestNilCatalogDiscoveryDoesNotMemoize(t *testing.T) {
	builds := writeNestedBuildsRepo(t)
	walks := countBuildFileWalks(t)
	var discovery *CatalogDiscovery
	for _, build := range builds {
		if _, ok := discovery.LoadVersionCatalogForGradlePath(build); !ok {
			t.Fatalf("%s: no catalog", build)
		}
	}
	if got := walks.Load(); got != int32(len(builds)) {
		t.Fatalf("nil discovery walked %d times, want one per build file (%d)", got, len(builds))
	}
}
