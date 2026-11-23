package module

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

func modulePaths(g *Graph) []string {
	var paths []string
	for p := range g.Modules {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ktor declares projects through a custom settings DSL (`+"ktor-server-core"`)
// that the settings parser cannot evaluate. Build-script directories still
// become modules, and typesafe accessors that name the bare project resolve
// to them.
func TestDiscoverModulesFallsBackToBuildScriptDirectories(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, `
projects {
    server {
        +"ktor-server-core"
    }
    +"ktor-utils"
}
`)
	makeModule(t, dir, "ktor-server", "ktor-server-core")
	makeModule(t, dir, "ktor-utils")
	writeFile(t, filepath.Join(dir, "ktor-server", "ktor-server-core", "build.gradle.kts"), `
dependencies {
    api(projects.ktorUtils)
}
`)
	if err := os.MkdirAll(filepath.Join(dir, "ktor-server", "ktor-server-core", "src", "main", "kotlin"), 0o755); err != nil {
		t.Fatal(err)
	}

	graph, err := DiscoverModules(t.Context(), dir)
	if err != nil || graph == nil {
		t.Fatalf("DiscoverModules = %v, %v", graph, err)
	}
	want := []string{":ktor-server:ktor-server-core", ":ktor-utils"}
	if got := modulePaths(graph); !slices.Equal(got, want) {
		t.Fatalf("modules = %v, want %v", got, want)
	}
	core := graph.Modules[":ktor-server:ktor-server-core"]
	if len(core.SourceRoots) != 1 {
		t.Errorf("source roots = %v, want one", core.SourceRoots)
	}
	if err := ParseAllDependencies(graph); err != nil {
		t.Fatal(err)
	}
	if len(core.Dependencies) != 1 || core.Dependencies[0].ModulePath != ":ktor-utils" {
		t.Errorf("dependencies = %+v, want :ktor-utils", core.Dependencies)
	}
}

// kotlinx.coroutines declares modules through
// `fun module(path) { include(name); project(":$name").projectDir = file(path) }`.
// The include is visible but the directory is not, so the declared module
// moves to the one build directory with its name instead of producing a
// duplicate filesystem module.
func TestDiscoverModulesRelocatesDeclaredModuleByName(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, `
fun module(path: String) {
    val name = path.substringAfterLast("/")
    include(name)
    project(":$name").projectDir = file(path)
}
include("kotlinx-coroutines-core")
include("kotlinx-coroutines-guava")
module("integration/kotlinx-coroutines-guava")
`)
	makeModule(t, dir, "kotlinx-coroutines-core")
	makeModule(t, dir, "integration", "kotlinx-coroutines-guava")

	graph, err := DiscoverModules(t.Context(), dir)
	if err != nil || graph == nil {
		t.Fatalf("DiscoverModules = %v, %v", graph, err)
	}
	want := []string{":kotlinx-coroutines-core", ":kotlinx-coroutines-guava"}
	if got := modulePaths(graph); !slices.Equal(got, want) {
		t.Fatalf("modules = %v, want %v", got, want)
	}
	if got, want := graph.Modules[":kotlinx-coroutines-guava"].Dir, filepath.Join(dir, "integration", "kotlinx-coroutines-guava"); got != want {
		t.Errorf("guava dir = %q, want %q", got, want)
	}
}

// compose-samples has no root settings file: each sample is its own build.
// Their modules are namespaced by directory so `:app` in two samples does
// not collide, and project() dependencies resolve inside their own build.
func TestDiscoverModulesSeparateBuildsWithoutRootSettings(t *testing.T) {
	dir := t.TempDir()
	for _, sample := range []string{"JetNews", "Jetcaster"} {
		writeFile(t, filepath.Join(dir, sample, "settings.gradle.kts"), `include(":app", ":core")`)
		makeModule(t, dir, sample, "app")
		makeModule(t, dir, sample, "core")
	}
	writeFile(t, filepath.Join(dir, "Jetcaster", "app", "build.gradle.kts"), `
dependencies {
    implementation(project(":core"))
}
`)

	graph, err := DiscoverModules(t.Context(), dir)
	if err != nil || graph == nil {
		t.Fatalf("DiscoverModules = %v, %v", graph, err)
	}
	want := []string{":JetNews:app", ":JetNews:core", ":Jetcaster:app", ":Jetcaster:core"}
	if got := modulePaths(graph); !slices.Equal(got, want) {
		t.Fatalf("modules = %v, want %v", got, want)
	}
	if err := ParseAllDependencies(graph); err != nil {
		t.Fatal(err)
	}
	deps := graph.Modules[":Jetcaster:app"].Dependencies
	if len(deps) != 1 || deps[0].ModulePath != ":Jetcaster:core" {
		t.Errorf("dependencies = %+v, want :Jetcaster:core", deps)
	}
}

// Included builds stay independent graphs when the root has a settings
// file, and build scripts inside source sets (test projects) and build
// outputs are not modules.
func TestDiscoverModulesFallbackSkipsIncludedBuildsAndSourceSets(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, `
includeBuild("build-logic")
include(":app")
`)
	makeModule(t, dir, "app")
	writeFile(t, filepath.Join(dir, "build-logic", "settings.gradle.kts"), `include(":convention")`)
	makeModule(t, dir, "build-logic", "convention")
	makeModule(t, dir, "app", "src", "test", "resources", "fixture-project")
	makeModule(t, dir, "app", "build", "generated")
	makeModule(t, dir, ".gradle", "cache")

	graph, err := DiscoverModules(t.Context(), dir)
	if err != nil || graph == nil {
		t.Fatalf("DiscoverModules = %v, %v", graph, err)
	}
	if got, want := modulePaths(graph), []string{":app"}; !slices.Equal(got, want) {
		t.Fatalf("modules = %v, want %v", got, want)
	}
}

func TestDiscoverModulesSingleProjectWithoutSettingsIsNil(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "build.gradle.kts"), "")
	if err := os.MkdirAll(filepath.Join(dir, "src", "main", "kotlin"), 0o755); err != nil {
		t.Fatal(err)
	}
	graph, err := DiscoverModules(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if graph != nil {
		t.Errorf("graph = %v, want nil for a single-project build", modulePaths(graph))
	}
}

func TestResolveModulePathRequiresUniqueLeaf(t *testing.T) {
	g := NewModuleGraph("/r")
	for _, p := range []string{":a:common", ":b:common", ":a:unique"} {
		g.Modules[p] = &Module{Path: p}
	}
	if got := g.resolveModulePath(":common", ""); got != ":common" {
		t.Errorf("ambiguous leaf resolved to %q", got)
	}
	if got := g.resolveModulePath(":unique", ""); got != ":a:unique" {
		t.Errorf(":unique resolved to %q", got)
	}
	if got := g.resolveModulePath(":nique", ""); got != ":nique" {
		t.Errorf("partial segment resolved to %q", got)
	}
}

// kotlinx.coroutines and ktor keep each target's sources directly under
// <target>/src and <target>/test instead of src/<sourceSet>/kotlin.
func TestFindSourceRootsFlatMultiplatformLayout(t *testing.T) {
	dir := t.TempDir()
	for _, p := range []string{"common/src", "common/test", "jvm/src", "jvm/resources", "build/generated/src"} {
		if err := os.MkdirAll(filepath.Join(dir, filepath.FromSlash(p)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	makeModule(t, dir, "nested-module")
	if err := os.MkdirAll(filepath.Join(dir, "nested-module", "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, root := range findSourceRoots(dir) {
		rel, _ := filepath.Rel(dir, root)
		got = append(got, filepath.ToSlash(rel))
	}
	want := []string{"common/src", "common/test", "jvm/src"}
	if !slices.Equal(got, want) {
		t.Errorf("roots = %v, want %v", got, want)
	}

	standard := t.TempDir()
	for _, p := range []string{"src/main/kotlin", "common/src"} {
		if err := os.MkdirAll(filepath.Join(standard, filepath.FromSlash(p)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if roots := findSourceRoots(standard); len(roots) != 1 || !strings.HasSuffix(filepath.ToSlash(roots[0]), "src/main/kotlin") {
		t.Errorf("standard layout roots = %v, want only src/main/kotlin", roots)
	}
}
