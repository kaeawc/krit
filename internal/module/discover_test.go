package module

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)


func TestParseIncludesKtsSinglePerLine(t *testing.T) {
	content := `
include(":app")
include(":libsignal-service")
include(":core-util")
`
	paths := parseIncludes(content)
	expected := []string{":app", ":libsignal-service", ":core-util"}
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


func TestDiscoverModulesNoSettingsFile(t *testing.T) {
	dir := t.TempDir()
	graph, err := DiscoverModules(dir)
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

	graph, err := DiscoverModules(dir)
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
