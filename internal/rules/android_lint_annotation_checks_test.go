package rules

import (
	"context"
	"os"
	"path/filepath"
	"slices"
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
				"android-application = { id = \"com.android.application\", version = \"9.4.1\" }\n"
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
