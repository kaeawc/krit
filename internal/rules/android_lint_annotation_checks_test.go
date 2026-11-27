package rules

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Lint reports production calls to @VisibleForTesting members as VisibleForTests; krit's equivalent is
// VisibleForTestingCallerInNonTest, so the Lint ID must suppress it (and carry lint.xml and baseline entries over).
func TestVisibleForTestsIsALintAliasOfVisibleForTestingCallerInNonTest(t *testing.T) {
	if got := AllSuppressionAliases()["VisibleForTestingCallerInNonTest"]; !slices.Contains(got, "VisibleForTests") {
		t.Fatalf("aliases of VisibleForTestingCallerInNonTest = %v, want VisibleForTests among them", got)
	}
	path := filepath.Join(t.TempDir(), "Main.kt")
	src := `package p

import android.annotation.SuppressLint

@SuppressLint("VisibleForTests")
fun caller() = target()
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	filter := scanner.BuildSuppressionFilter(file, nil, allRuleExcludes(), "").WithRuleAliases(AllSuppressionAliases())
	var rule *api.Rule
	for _, r := range api.Registry {
		if r.ID == "VisibleForTestingCallerInNonTest" {
			rule = r
		}
	}
	if rule == nil {
		t.Fatal("VisibleForTestingCallerInNonTest is not registered")
	}
	if !filter.IsSuppressedAtColumn(rule.ID, rule.Category, 6, 16) {
		t.Fatal(`@SuppressLint("VisibleForTests") does not suppress VisibleForTestingCallerInNonTest`)
	}
}

// A library module is recognized through a version-catalog plugin alias, and an application module is not mistaken
// for one by a `-library` project dependency or a commented-out plugin.
func TestReadModuleIsLibrary(t *testing.T) {
	cases := []struct {
		name, script string
		want         bool
	}{
		{"catalog library", "plugins {\n    alias(libs.plugins.android.library)\n}\n", true},
		{"catalog application", "plugins {\n    alias(libs.plugins.android.application)\n}\n", false},
		{"id library", "plugins {\n    id(\"com.android.library\")\n}\n", true},
		{"application with -library dependency", "plugins {\n    id(\"com.android.application\")\n}\ndependencies {\n    implementation(project(\":core-library\"))\n}\n", false},
		{"commented application", "plugins {\n    id(\"com.android.library\")\n    // id(\"com.android.application\")\n}\n", true},
		{"commented library", "plugins {\n    // id(\"com.android.library\")\n}\n", false},
		// A quoted name outside the plugin declarations is no plugin.
		{"library with -application dependency", "plugins {\n    id(\"com.android.library\")\n}\ndependencies {\n    implementation(project(\":core-application\"))\n}\n", true},
		{"groovy apply plugin", "apply plugin: 'com.android.library'\n", true},
		// Convention plugins whose ids end in .library or .application, as nowinandroid's.
		{"catalog convention library", "plugins {\n    alias(libs.plugins.nowinandroid.android.library)\n    alias(libs.plugins.nowinandroid.android.library.compose)\n}\n", true},
		{"catalog convention application", "plugins {\n    alias(libs.plugins.nowinandroid.android.application)\n    alias(libs.plugins.nowinandroid.android.application.compose)\n}\n", false},
		{"id convention library", "plugins {\n    id(\"nowinandroid.android.library\")\n}\n", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			module := filepath.Join(root, "lib")
			if err := os.MkdirAll(filepath.Join(root, "gradle"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(module, 0o755); err != nil {
				t.Fatal(err)
			}
			catalog := "[plugins]\nandroid-library = { id = \"com.android.library\", version = \"9.4.1\" }\n" +
				"android-application = { id = \"com.android.application\", version = \"9.4.1\" }\n" +
				"nowinandroid-android-library = { id = \"nowinandroid.android.library\" }\n" +
				"nowinandroid-android-library-compose = { id = \"nowinandroid.android.library.compose\" }\n" +
				"nowinandroid-android-application = { id = \"nowinandroid.android.application\" }\n" +
				"nowinandroid-android-application-compose = { id = \"nowinandroid.android.application.compose\" }\n"
			if err := os.WriteFile(filepath.Join(root, "gradle", "libs.versions.toml"), []byte(catalog), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(module, "build.gradle.kts"), []byte(c.script), 0o644); err != nil {
				t.Fatal(err)
			}
			if got := readModuleIsLibrary(module); got != c.want {
				t.Fatalf("readModuleIsLibrary = %v, want %v", got, c.want)
			}
		})
	}
}

// ModulePluginIDs reads the plugin declarations alone: ids in every form a build script applies them, aliases resolved
// through the catalog, and nothing from comments or other strings. The pipeline keys per-file findings on it.
func TestModulePluginIDs(t *testing.T) {
	root := t.TempDir()
	module := filepath.Join(root, "lib")
	for _, dir := range []string{filepath.Join(root, "gradle"), module} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	catalog := "[plugins]\nconvention-library = { id = \"example.android.library\" }\n"
	if err := os.WriteFile(filepath.Join(root, "gradle", "libs.versions.toml"), []byte(catalog), 0o644); err != nil {
		t.Fatal(err)
	}
	script := "plugins {\n    id(\"org.jetbrains.kotlin.android\")\n    id 'kotlin-kapt'\n    alias(libs.plugins.convention.library)\n" +
		"    // id(\"com.android.application\")\n}\napply plugin: 'com.android.library'\napply(plugin = \"maven-publish\")\n" +
		"android { valid(\"not a plugin\"); android_id(\"neither\") }\ndependencies {\n    implementation(project(\":core-application\"))\n}\n"
	got := ModulePluginIDs(nil, LoadCatalogPluginTables, module, script)
	sort.Strings(got)
	want := []string{"com.android.library", "example.android.library", "kotlin-kapt", "maven-publish", "org.jetbrains.kotlin.android"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ModulePluginIDs = %q, want %q", got, want)
	}
}

// The LSP, the daemon and MCP keep one dispatcher, and its fact cache, across requests. The library verdict reads the
// build script, so a module switched to the application plugin must stop reporting at the next request.
func TestAnnotateVersionCheckRereadsTheModulePluginEachRequest(t *testing.T) {
	module := filepath.Join(t.TempDir(), "lib")
	source := filepath.Join(module, "src", "main", "java", "com", "example", "Versions.kt")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(module, "build.gradle.kts")
	writeScript := func(plugin string) {
		t.Helper()
		if err := os.WriteFile(script, []byte("plugins {\n    id(\""+plugin+"\")\n}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeScript("com.android.library")
	code := "package com.example\n\nimport android.os.Build\n\nobject Versions {\n    fun isAtLeastP() = Build.VERSION.SDK_INT >= 28\n}\n"
	if err := os.WriteFile(source, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	var rule *api.Rule
	for _, r := range api.Registry {
		if r != nil && r.ID == "AnnotateVersionCheck" {
			rule = r
		}
	}
	if rule == nil {
		t.Fatal("AnnotateVersionCheck is not registered")
	}
	d := NewDispatcher([]*api.Rule{rule}, nil)
	count := func() int {
		t.Helper()
		file, err := scanner.ParseFile(context.Background(), source)
		if err != nil {
			t.Fatal(err)
		}
		cols := d.Run(file)
		return len(cols.Findings())
	}
	d.BeginRequest()
	if n := count(); n != 1 {
		t.Fatalf("library module: %d findings, want 1", n)
	}
	writeScript("com.android.application")
	d.BeginRequest()
	if n := count(); n != 0 {
		t.Fatalf("after switching to the application plugin and beginning a request: %d findings, want 0", n)
	}
}
