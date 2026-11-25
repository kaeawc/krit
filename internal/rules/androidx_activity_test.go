package rules_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// activityProjectFindings writes files into a temp project and runs one rule over the source file at srcPath with the
// profile's library facts, returning sorted "line:col message" entries.
func activityProjectFindings(t *testing.T, ruleID, srcPath string, files map[string]string, profile librarymodel.ProjectProfile) []string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(dir, srcPath)
	parse := scanner.ParseFile
	if strings.HasSuffix(path, ".java") {
		parse = scanner.ParseJavaFile
	}
	file, err := parse(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	rule := buildRuleIndex()[ruleID]
	if rule == nil {
		t.Fatalf("%s not registered", ruleID)
	}
	dispatcher := rules.NewDispatcher([]*api.Rule{rule}, nil)
	dispatcher.SetLibraryFacts(librarymodel.FactsForProfile(profile))
	cols := dispatcher.Run(file)
	var out []string
	for _, f := range cols.Findings() {
		out = append(out, fmt.Sprintf("%d:%d %s", f.Line, f.Col, f.Message))
	}
	sort.Strings(out)
	return out
}

const activityResultSource = `package com.example

import androidx.activity.ComponentActivity
import androidx.activity.result.contract.ActivityResultContracts

class A : ComponentActivity() {
    val launcher = registerForActivityResult(ActivityResultContracts.StartActivityForResult()) { }
    fun other() {
        this.registerForActivityResult(ActivityResultContracts.RequestPermission()) { }
    }
}
`

func activityBuild(deps string) string {
	return "plugins {\n    id 'com.android.application'\n}\n\ndependencies {\n" + deps + "\n}\n"
}

// Expected positions come from running the lint.jar of androidx.activity:activity 1.13.0 through the Android Lint CLI
// on the same source with `api "androidx.fragment:fragment:1.2.5"`. Lint reports each call once per old declaration;
// krit reports it once.
func TestInvalidFragmentVersionForActivityResultLintParity(t *testing.T) {
	msg := "Upgrade Fragment version to at least 1.3.0."
	for _, tc := range []struct {
		name string
		deps string
		want []string
	}{
		{"api literal", `    api "androidx.fragment:fragment:1.2.5"`, []string{"7:20 " + msg, "9:9 " + msg}},
		// Lint reads implementation dependencies from the resolved Gradle model; krit reads the declaration.
		{"implementation", `    implementation "androidx.fragment:fragment:1.1.0"`, []string{"7:20 " + msg, "9:9 " + msg}},
		{"current fragment", `    implementation "androidx.fragment:fragment:1.9.1"`, nil},
		// A pre-release of 1.3.0 counts as 1.3.0, as in Lint (which drops the text after the last '-').
		{"1.3.0 pre-release", `    api "androidx.fragment:fragment:1.3.0-alpha01"`, nil},
		// Lint compares versions as strings, so it reports 1.10.0 as older than 1.3.0; krit compares the numbers.
		{"1.10.0", `    api "androidx.fragment:fragment:1.10.0"`, nil},
		{"unknown version", `    api "androidx.fragment:fragment:$fragmentVersion"`, nil},
		// fragment-ktx is not the coordinate Lint checks.
		{"fragment-ktx", `    api "androidx.fragment:fragment-ktx:1.2.5"`, nil},
		// A fragment version that only arrives transitively (here through appcompat 1.1.0) is a known false
		// negative until krit reads resolved dependency artifacts (#1120).
		{"transitive", `    implementation "androidx.appcompat:appcompat:1.1.0"`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string]string{"app/build.gradle": activityBuild(tc.deps), "app/src/main/java/com/example/A.kt": activityResultSource}
			got := activityProjectFindings(t, "InvalidFragmentVersionForActivityResult", "app/src/main/java/com/example/A.kt", files, librarymodel.ProjectProfile{})
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// The version comes from the source file's own module, through the version catalog when the build uses one, so an
// old fragment in another module does not leak into this one.
func TestInvalidFragmentVersionForActivityResultReadsTheModule(t *testing.T) {
	src := "app/src/main/java/com/example/A.kt"
	catalog := map[string]string{
		"gradle/libs.versions.toml": "[versions]\nfragment = \"1.2.0\"\n\n[libraries]\nandroidx-fragment = { module = \"androidx.fragment:fragment\", version.ref = \"fragment\" }\n",
		"settings.gradle.kts":       "include(\":app\")\n",
		"app/build.gradle.kts":      "dependencies {\n    implementation(libs.androidx.fragment)\n}\n",
		src:                         activityResultSource,
	}
	if got := activityProjectFindings(t, "InvalidFragmentVersionForActivityResult", src, catalog, librarymodel.ProjectProfile{}); len(got) != 2 {
		t.Errorf("catalog: got %q, want 2 findings", got)
	}
	split := map[string]string{
		"lib/build.gradle": activityBuild(`    api "androidx.fragment:fragment:1.2.5"`),
		"app/build.gradle": activityBuild(`    implementation "androidx.fragment:fragment:1.9.1"`),
		src:                activityResultSource,
	}
	old := librarymodel.ProjectProfile{Dependencies: []librarymodel.Dependency{{Group: "androidx.fragment", Name: "fragment", Version: "1.2.5"}}}
	if got := activityProjectFindings(t, "InvalidFragmentVersionForActivityResult", src, split, old); len(got) != 0 {
		t.Errorf("another module's old fragment: got %q, want none", got)
	}
}

const backPressedSource = `package com.example

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.OnBackPressedCallback

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        onBackPressedDispatcher.addCallback(this, object : OnBackPressedCallback(true) {
            override fun handleOnBackPressed() {
                onBackPressed()
            }
        })
    }

    fun handledOnBackPressed() {
        onBackPressed()
        run { this.onBackPressed() }
    }
}
`

// Lint's OnBackPressedDetector reports onBackPressed() only inside a method named handledOnBackPressed, a misspelling
// of OnBackPressedCallback.handleOnBackPressed. krit keeps Lint's verdicts: the real callback method is not reported.
// Positions come from the Lint CLI run on the same source.
func TestInvalidUseOfOnBackPressedMatchesLintsMethodName(t *testing.T) {
	activity := librarymodel.ProjectProfile{Dependencies: []librarymodel.Dependency{{Group: "androidx.activity", Name: "activity-ktx", Version: "1.13.0"}}}
	msg := "Should not call onBackPressed inside of OnBackPressedCallback.handledOnBackPressed"
	files := map[string]string{"src/main/java/com/example/MainActivity.kt": backPressedSource}
	got := activityProjectFindings(t, "InvalidUseOfOnBackPressed", "src/main/java/com/example/MainActivity.kt", files, activity)
	if want := []string{"18:9 " + msg, "19:15 " + msg}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got := activityProjectFindings(t, "InvalidUseOfOnBackPressed", "src/main/java/com/example/MainActivity.kt", files, librarymodel.ProjectProfile{}); len(got) != 0 {
		t.Fatalf("without androidx.activity: got %q, want none", got)
	}
}
