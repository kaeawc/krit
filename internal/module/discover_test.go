package module

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestParseIncludesKtsSinglePerLine(t *testing.T) {
	content := `
include(":app")
include(":messaging-service")
include(":core-util")
`
	paths := parseIncludes(content)
	expected := []string{":app", ":messaging-service", ":core-util"}
	if len(paths) != len(expected) {
		t.Fatalf("expected %d modules, got %d: %v", len(expected), len(paths), paths)
	}
	for i, p := range paths {
		if p != expected[i] {
			t.Errorf("paths[%d] = %q, want %q", i, p, expected[i])
		}
	}
}

func TestParseIncludesKtsMultiArg(t *testing.T) {
	content := `
include(
  ":backstack",
  ":circuit-codegen",
  ":circuit-foundation",
  ":circuitx:android",
  ":samples:star:apk",
)
`
	paths := parseIncludes(content)
	if len(paths) != 5 {
		t.Fatalf("expected 5 modules, got %d: %v", len(paths), paths)
	}
	if paths[0] != ":backstack" {
		t.Errorf("expected :backstack, got %q", paths[0])
	}
	if paths[3] != ":circuitx:android" {
		t.Errorf("expected :circuitx:android, got %q", paths[3])
	}
	if paths[4] != ":samples:star:apk" {
		t.Errorf("expected :samples:star:apk, got %q", paths[4])
	}
}

func TestParseIncludesGroovy(t *testing.T) {
	content := `
include ':app', ':lib', ':core'
`
	paths := parseIncludes(content)
	expected := []string{":app", ":lib", ":core"}
	if len(paths) != len(expected) {
		t.Fatalf("expected %d modules, got %d: %v", len(expected), len(paths), paths)
	}
	for i, p := range paths {
		if p != expected[i] {
			t.Errorf("paths[%d] = %q, want %q", i, p, expected[i])
		}
	}
}

func TestParseIncludesWithoutColon(t *testing.T) {
	content := `
include(
  "sentry",
  "sentry-android-core"
)
`
	paths := parseIncludes(content)
	if len(paths) != 2 {
		t.Fatalf("expected 2 modules, got %d: %v", len(paths), paths)
	}
	if paths[0] != ":sentry" {
		t.Errorf("expected :sentry, got %q", paths[0])
	}
	if paths[1] != ":sentry-android-core" {
		t.Errorf("expected :sentry-android-core, got %q", paths[1])
	}
}

func TestParseProjectDirOverrides(t *testing.T) {
	content := `
project(":paging").projectDir = file("paging/lib")
project(":paging-app").projectDir = file("paging/app")
project(":device-transfer").projectDir = file("device-transfer/lib")
`
	overrides := parseProjectDirOverrides(content)
	if len(overrides) != 3 {
		t.Fatalf("expected 3 overrides, got %d", len(overrides))
	}
	if overrides[":paging"] != "paging/lib" {
		t.Errorf("paging override = %q, want %q", overrides[":paging"], "paging/lib")
	}
	if overrides[":device-transfer"] != "device-transfer/lib" {
		t.Errorf("device-transfer override = %q", overrides[":device-transfer"])
	}
}

func TestModulePathToDir(t *testing.T) {
	root := "/project"
	overrides := map[string]string{
		":paging": "paging/lib",
	}

	tests := []struct {
		modPath string
		want    string
	}{
		{":app", filepath.Join(root, "app")},
		{":core:util", filepath.Join(root, "core", "util")},
		{":paging", filepath.Join(root, "paging", "lib")},
		{":circuitx:android", filepath.Join(root, "circuitx", "android")},
	}

	for _, tt := range tests {
		t.Run(tt.modPath, func(t *testing.T) {
			got := modulePathToDir(root, tt.modPath, overrides)
			if got != tt.want {
				t.Errorf("modulePathToDir(%q) = %q, want %q", tt.modPath, got, tt.want)
			}
		})
	}
}

// Templated include paths (containing `${...}`) must not be emitted as
// literal module paths — they are placeholders for dynamic include
// bodies and would otherwise create a bogus `:compiler-compat:${it.name}`
// module pointing at a nonexistent directory.
func TestParseIncludesSkipsTemplatedPaths(t *testing.T) {
	content := `
include(":app")
rootProject.projectDir.resolve("compat").listFiles()!!.forEach {
  include(":compat:${it.name}")
}
`
	paths := parseIncludes(content)
	for _, p := range paths {
		if strings.Contains(p, "${") {
			t.Errorf("parseIncludes returned templated path %q; should be skipped", p)
		}
	}
	// The static :app include should still be present.
	var sawApp bool
	for _, p := range paths {
		if p == ":app" {
			sawApp = true
		}
	}
	if !sawApp {
		t.Errorf("expected :app in %v", paths)
	}
}

// DiscoverModules should expand the dir.listFiles().forEach { include(...) }
// idiom by walking the filesystem and emitting one module per subdir
// containing a build.gradle(.kts).
func TestDiscoverModulesExpandsDynamicIncludes(t *testing.T) {
	dir := t.TempDir()
	content := `
include(":compiler")
rootProject.projectDir.resolve("compat").listFiles()!!.forEach {
  if (it.isDirectory && it.name.startsWith("k")) {
    include(":compat:${it.name}")
  }
}
`
	if err := os.WriteFile(filepath.Join(dir, "settings.gradle.kts"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	// Real submodules: k220, k230 (each with build.gradle.kts).
	for _, sub := range []string{"k220", "k230"} {
		subDir := filepath.Join(dir, "compat", sub)
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(subDir, "build.gradle.kts"), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// Non-module sibling: src/, no build script — must be skipped.
	if err := os.MkdirAll(filepath.Join(dir, "compat", "src"), 0755); err != nil {
		t.Fatal(err)
	}
	// Also create the static :compiler module's dir so findSourceRoots
	// can run (no source roots needed for the test).
	if err := os.MkdirAll(filepath.Join(dir, "compiler"), 0755); err != nil {
		t.Fatal(err)
	}

	graph, err := DiscoverModules(t.Context(), dir)
	if err != nil {
		t.Fatalf("DiscoverModules: %v", err)
	}
	var paths []string
	for p := range graph.Modules {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	want := []string{":compat:k220", ":compat:k230", ":compiler"}
	if len(paths) != len(want) {
		t.Fatalf("modules = %v, want %v", paths, want)
	}
	for i, p := range paths {
		if p != want[i] {
			t.Errorf("paths[%d] = %q, want %q", i, p, want[i])
		}
	}
	// Templated literal must not appear.
	if _, ok := graph.Modules[":compat:${it.name}"]; ok {
		t.Error("templated placeholder leaked into module graph")
	}
}

// Indirect receiver: dynamic include via a `val dir = ...` binding,
// the receiver of .listFiles() is a simple identifier instead of an
// inline resolve() chain. The regex parser couldn't handle this; the
// tree-sitter parser resolves the binding.
func TestDiscoverModulesDynamicIncludeViaValBinding(t *testing.T) {
	dir := t.TempDir()
	content := `
val compatDir = rootProject.projectDir.resolve("compat")
compatDir.listFiles()!!.forEach {
  include(":compat:${it.name}")
}
`
	if err := os.WriteFile(filepath.Join(dir, "settings.gradle.kts"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{"k220", "k230"} {
		subDir := filepath.Join(dir, "compat", sub)
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(subDir, "build.gradle.kts"), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}

	graph, err := DiscoverModules(t.Context(), dir)
	if err != nil {
		t.Fatalf("DiscoverModules: %v", err)
	}
	for _, p := range []string{":compat:k220", ":compat:k230"} {
		if _, ok := graph.Modules[p]; !ok {
			t.Errorf("missing module %s", p)
		}
	}
}

// Lambda with an explicit parameter name (`f ->`) — the template uses
// `${f.name}` rather than `${it.name}`. The walker must capture the
// parameter and accept the corresponding interpolation.
func TestDiscoverModulesDynamicIncludeNamedLambdaParam(t *testing.T) {
	dir := t.TempDir()
	content := `
rootProject.projectDir.resolve("samples").listFiles()?.forEach { f ->
  include(":samples:${f.name}")
}
`
	if err := os.WriteFile(filepath.Join(dir, "settings.gradle.kts"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{"alpha", "beta"} {
		subDir := filepath.Join(dir, "samples", sub)
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(subDir, "build.gradle.kts"), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}

	graph, err := DiscoverModules(t.Context(), dir)
	if err != nil {
		t.Fatalf("DiscoverModules: %v", err)
	}
	for _, p := range []string{":samples:alpha", ":samples:beta"} {
		if _, ok := graph.Modules[p]; !ok {
			t.Errorf("missing module %s", p)
		}
	}
}

// Multiple include() calls in a single forEach body.
func TestDiscoverModulesDynamicIncludeMultipleIncludes(t *testing.T) {
	dir := t.TempDir()
	content := `
rootProject.projectDir.resolve("apps").listFiles()!!.forEach {
  include(":apps:${it.name}")
  include(":apps:${it.name}-test")
}
`
	if err := os.WriteFile(filepath.Join(dir, "settings.gradle.kts"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	subDir := filepath.Join(dir, "apps", "alpha")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "build.gradle.kts"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	graph, err := DiscoverModules(t.Context(), dir)
	if err != nil {
		t.Fatalf("DiscoverModules: %v", err)
	}
	for _, p := range []string{":apps:alpha", ":apps:alpha-test"} {
		if _, ok := graph.Modules[p]; !ok {
			t.Errorf("missing %s", p)
		}
	}
}

// Template with an interpolation we can't resolve (e.g. ${it.path}) is
// skipped silently rather than emitted as garbage.
func TestDiscoverModulesDynamicIncludeUnknownInterpolation(t *testing.T) {
	dir := t.TempDir()
	content := `
rootProject.projectDir.resolve("apps").listFiles()!!.forEach {
  include(":apps:${it.path}")
}
`
	if err := os.WriteFile(filepath.Join(dir, "settings.gradle.kts"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	subDir := filepath.Join(dir, "apps", "alpha")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "build.gradle.kts"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	graph, err := DiscoverModules(t.Context(), dir)
	if err != nil {
		t.Fatalf("DiscoverModules: %v", err)
	}
	for p := range graph.Modules {
		if strings.Contains(p, "${") || strings.Contains(p, ".path") {
			t.Errorf("phantom templated module leaked: %s", p)
		}
	}
}

// Nested if() filter inside the lambda — we don't evaluate the
// predicate; build-script presence is what gates emission. Includes
// inside the if body must still be discovered.
func TestDiscoverModulesDynamicIncludeNestedIfBlock(t *testing.T) {
	dir := t.TempDir()
	content := `
rootProject.projectDir.resolve("compat").listFiles()!!.forEach {
  if (it.isDirectory && it.name.startsWith("k")) {
    include(":compat:${it.name}")
  }
}
`
	if err := os.WriteFile(filepath.Join(dir, "settings.gradle.kts"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{"k220", "k230"} {
		subDir := filepath.Join(dir, "compat", sub)
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(subDir, "build.gradle.kts"), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}

	graph, err := DiscoverModules(t.Context(), dir)
	if err != nil {
		t.Fatalf("DiscoverModules: %v", err)
	}
	for _, p := range []string{":compat:k220", ":compat:k230"} {
		if _, ok := graph.Modules[p]; !ok {
			t.Errorf("missing %s", p)
		}
	}
}

// for-statement iteration: `for (f in dir.listFiles()) { include(...) }`.
func TestDiscoverModulesForStatement(t *testing.T) {
	dir := t.TempDir()
	content := `
val compatDir = rootProject.projectDir.resolve("compat")
for (f in compatDir.listFiles()!!) {
  include(":compat:${f.name}")
}
`
	writeSettings(t, dir, content)
	makeModule(t, dir, "compat", "k220")
	makeModule(t, dir, "compat", "k230")

	graph, err := DiscoverModules(t.Context(), dir)
	if err != nil {
		t.Fatalf("DiscoverModules: %v", err)
	}
	for _, p := range []string{":compat:k220", ":compat:k230"} {
		if _, ok := graph.Modules[p]; !ok {
			t.Errorf("missing %s", p)
		}
	}
}

// .walk() recursive iteration. Modules can appear at any depth; we
// emit one per directory containing a build script under the iteration
// root, using `${it.name}` for the module path.
func TestDiscoverModulesWalkRecursive(t *testing.T) {
	dir := t.TempDir()
	content := `
rootProject.projectDir.resolve("apps").walk().forEach {
  if (it.isDirectory && File(it, "build.gradle.kts").exists()) {
    include(":apps:${it.name}")
  }
}
`
	writeSettings(t, dir, content)
	makeModule(t, dir, "apps", "alpha")
	makeModule(t, dir, "apps", "nested", "beta")

	graph, err := DiscoverModules(t.Context(), dir)
	if err != nil {
		t.Fatalf("DiscoverModules: %v", err)
	}
	for _, p := range []string{":apps:alpha", ":apps:beta"} {
		if _, ok := graph.Modules[p]; !ok {
			t.Errorf("missing %s", p)
		}
	}
}

// .filter { ... }.forEach { ... } chain. The pass-through filter is
// walked through to find the underlying .listFiles() iteration source.
func TestDiscoverModulesFilterChain(t *testing.T) {
	dir := t.TempDir()
	content := `
rootProject.projectDir.resolve("compat").listFiles()!!
  .filter { it.isDirectory }
  .forEach { include(":compat:${it.name}") }
`
	writeSettings(t, dir, content)
	makeModule(t, dir, "compat", "k220")

	graph, err := DiscoverModules(t.Context(), dir)
	if err != nil {
		t.Fatalf("DiscoverModules: %v", err)
	}
	if _, ok := graph.Modules[":compat:k220"]; !ok {
		t.Error("missing :compat:k220")
	}
}

// .map { ... }.toList().forEach { ... } — multiple pass-through ops.
func TestDiscoverModulesMapToListChain(t *testing.T) {
	dir := t.TempDir()
	content := `
rootProject.projectDir.resolve("apps").listFiles()!!
  .asSequence()
  .filter { it.isDirectory }
  .toList()
  .forEach { include(":apps:${it.name}") }
`
	writeSettings(t, dir, content)
	makeModule(t, dir, "apps", "alpha")

	graph, err := DiscoverModules(t.Context(), dir)
	if err != nil {
		t.Fatalf("DiscoverModules: %v", err)
	}
	if _, ok := graph.Modules[":apps:alpha"]; !ok {
		t.Error("missing :apps:alpha")
	}
}

// Files.list(<dir>) iteration source. Any qualifier on `Files` works
// (java.nio.file.Files.list, Files.list, kotlin.io.path.Files.list).
// Path entries use ${it.fileName} rather than ${it.name}.
func TestDiscoverModulesFilesList(t *testing.T) {
	dir := t.TempDir()
	content := `
java.nio.file.Files.list(rootProject.projectDir.resolve("apps")).forEach {
  include(":apps:${it.fileName}")
}
`
	writeSettings(t, dir, content)
	makeModule(t, dir, "apps", "alpha")

	graph, err := DiscoverModules(t.Context(), dir)
	if err != nil {
		t.Fatalf("DiscoverModules: %v", err)
	}
	if _, ok := graph.Modules[":apps:alpha"]; !ok {
		t.Error("missing :apps:alpha")
	}
}

// Multi-hop val chain: `val a = ...; val b = a.resolve("X"); b.listFiles()...`.
func TestDiscoverModulesMultiHopValChain(t *testing.T) {
	dir := t.TempDir()
	content := `
val root = rootProject.projectDir
val compatRoot = root.resolve("compat")
val k = compatRoot.resolve("k220")
include(":compat:k220")
compatRoot.listFiles()!!.forEach {
  include(":compat:${it.name}")
}
`
	_ = `unused: ` + "k" // ensure binding doesn't break anything
	writeSettings(t, dir, content)
	makeModule(t, dir, "compat", "k220")
	makeModule(t, dir, "compat", "k230")

	graph, err := DiscoverModules(t.Context(), dir)
	if err != nil {
		t.Fatalf("DiscoverModules: %v", err)
	}
	for _, p := range []string{":compat:k220", ":compat:k230"} {
		if _, ok := graph.Modules[p]; !ok {
			t.Errorf("missing %s", p)
		}
	}
}

// String-typed `val` substitution in include() arguments:
// `val prefix = "compat"; include(":${prefix}:foo")` resolves prefix.
func TestDiscoverModulesStringValSubstitution(t *testing.T) {
	dir := t.TempDir()
	content := `
val prefix = "compat"
include(":${prefix}:foo")
include(":${prefix}:bar")
`
	writeSettings(t, dir, content)

	graph, err := DiscoverModules(t.Context(), dir)
	if err != nil {
		t.Fatalf("DiscoverModules: %v", err)
	}
	for _, p := range []string{":compat:foo", ":compat:bar"} {
		if _, ok := graph.Modules[p]; !ok {
			t.Errorf("missing %s", p)
		}
	}
}

// String-typed val plus loop interpolation in the same template.
func TestDiscoverModulesStringValAndLoopInterpolation(t *testing.T) {
	dir := t.TempDir()
	content := `
val ns = "compat"
rootProject.projectDir.resolve("compat").listFiles()!!.forEach {
  include(":${ns}:${it.name}")
}
`
	writeSettings(t, dir, content)
	makeModule(t, dir, "compat", "k220")

	graph, err := DiscoverModules(t.Context(), dir)
	if err != nil {
		t.Fatalf("DiscoverModules: %v", err)
	}
	if _, ok := graph.Modules[":compat:k220"]; !ok {
		t.Error("missing :compat:k220")
	}
}

// includeBuild() is intentionally not handled — composite builds are
// independent module graphs. This test guards against accidentally
// emitting them as subprojects in the future.
func TestDiscoverModulesIgnoresIncludeBuild(t *testing.T) {
	dir := t.TempDir()
	content := `
includeBuild("build-logic")
include(":app")
`
	writeSettings(t, dir, content)

	graph, err := DiscoverModules(t.Context(), dir)
	if err != nil {
		t.Fatalf("DiscoverModules: %v", err)
	}
	if _, ok := graph.Modules[":build-logic"]; ok {
		t.Error("includeBuild should not be emitted as a subproject")
	}
	if _, ok := graph.Modules[":app"]; !ok {
		t.Error("missing :app")
	}
}

// --- test helpers --------------------------------------------------------

func writeSettings(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "settings.gradle.kts"), []byte(content), 0644); err != nil {
		t.Fatalf("write settings.gradle.kts: %v", err)
	}
}

// makeModule creates a directory hierarchy under dir and drops a
// build.gradle.kts in the leaf so DiscoverModules treats it as a module.
func makeModule(t *testing.T, dir string, parts ...string) {
	t.Helper()
	full := filepath.Join(append([]string{dir}, parts...)...)
	if err := os.MkdirAll(full, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(full, "build.gradle.kts"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverModulesNoSettingsFile(t *testing.T) {
	dir := t.TempDir()
	graph, err := DiscoverModules(t.Context(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if graph != nil {
		t.Error("expected nil graph for non-Gradle project")
	}
}

func TestDiscoverModulesInlineSettings(t *testing.T) {
	dir := t.TempDir()
	content := `
rootProject.name = "test-project"

include(":app")
include(":lib:core", ":lib:ui")
include(":tools")

project(":tools").projectDir = file("build-tools/custom")
`
	err := os.WriteFile(filepath.Join(dir, "settings.gradle.kts"), []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	graph, err := DiscoverModules(t.Context(), dir)
	if err != nil {
		t.Fatalf("DiscoverModules: %v", err)
	}
	if graph == nil {
		t.Fatal("expected non-nil graph")
	}

	if len(graph.Modules) != 4 {
		t.Fatalf("expected 4 modules, got %d", len(graph.Modules))
	}

	var paths []string
	for p := range graph.Modules {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	expected := []string{":app", ":lib:core", ":lib:ui", ":tools"}
	sort.Strings(expected)
	for i, p := range paths {
		if p != expected[i] {
			t.Errorf("paths[%d] = %q, want %q", i, p, expected[i])
		}
	}

	// Check projectDir override.
	tools := graph.Modules[":tools"]
	wantDir := filepath.Join(dir, "build-tools", "custom")
	if tools.Dir != wantDir {
		t.Errorf("tools dir = %q, want %q", tools.Dir, wantDir)
	}

	// :lib:core should map to <root>/lib/core.
	libCore := graph.Modules[":lib:core"]
	wantDir = filepath.Join(dir, "lib", "core")
	if libCore.Dir != wantDir {
		t.Errorf("lib:core dir = %q, want %q", libCore.Dir, wantDir)
	}
}
