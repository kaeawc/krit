package rules_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func fragmentGradlePositions(t *testing.T, path, content string) []string {
	t.Helper()
	var out []string
	for _, f := range runLintGradleRule(t, "FragmentGradleConfiguration", path, content) {
		out = append(out, fmt.Sprintf("%d:%d %s", f.Line, f.Col, f.Message))
	}
	sort.Strings(out)
	return out
}

// Expected positions come from running the lint.jars of androidx.fragment:fragment-testing and
// fragment-testing-manifest 1.9.1 through the Android Lint CLI on the same Groovy build file. Both jars register
// FragmentGradleConfiguration, so Lint reports a fragment-testing-manifest line twice; krit reports it once.
func TestFragmentGradleConfigurationLintParity(t *testing.T) {
	content := `dependencies {
    implementation 'androidx.fragment:fragment-testing-manifest:1.9.1'
    debugImplementation 'androidx.fragment:fragment-testing-manifest:1.9.1'
    testImplementation "androidx.fragment:fragment-testing:1.9.1"
    androidTestImplementation('androidx.fragment:fragment-testing:1.9.1')
    implementation libs.fragment.testing
    api "androidx.fragment:fragment:1.2.5"
    implementation("androidx.fragment:fragment-testing:1.9.1") {
        exclude group: "x"
    }
    implementation platform("androidx.fragment:fragment-testing:1.9.1")
}
`
	msg := "Replace with debugImplementation."
	want := []string{"2:5 " + msg, "4:5 " + msg, "5:5 " + msg}
	if got := fragmentGradlePositions(t, "build.gradle", content); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// The Lint CLI cannot run Gradle checks on Kotlin build scripts, so these follow the detector's source: a bare string
// literal on any configuration but debugImplementation.
func TestFragmentGradleConfigurationKotlinDSL(t *testing.T) {
	content := `dependencies {
    implementation("androidx.fragment:fragment-testing-manifest:1.9.1")
    debugImplementation("androidx.fragment:fragment-testing-manifest:1.9.1")
    testImplementation("androidx.fragment:fragment-testing:1.9.1")
    implementation(libs.fragment.testing)
    implementation(group = "androidx.fragment", name = "fragment-testing", version = "1.9.1")
}
`
	msg := "Replace with debugImplementation."
	want := []string{"2:5 " + msg, "4:5 " + msg}
	if got := fragmentGradlePositions(t, "build.gradle.kts", content); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// Lint's detector reports any assignment of such a string, so it also flags ext constants, local variables and an
// unrelated testInstrumentationRunner value with the same prefix. Those are not dependency declarations, and krit
// only reads the dependencies closure.
func TestFragmentGradleConfigurationIgnoresNonDependencyStrings(t *testing.T) {
	content := `ext {
    fragmentTesting = "androidx.fragment:fragment-testing:1.9.1"
}
def other = "androidx.fragment:fragment-testing:1.9.1"
android {
    defaultConfig {
        testInstrumentationRunner "androidx.fragment:fragment-testing-runner"
    }
}
dependencies {
    debugImplementation fragmentTesting
}
`
	if got := fragmentGradlePositions(t, "build.gradle", content); len(got) != 0 {
		t.Fatalf("got %q, want none", got)
	}
}

func fragmentResourceFindings(t *testing.T, profile librarymodel.ProjectProfile, files map[string]string) []string {
	t.Helper()
	resDir := filepath.Join(t.TempDir(), "res")
	for name, body := range files {
		path := filepath.Join(resDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	idx, err := android.ScanResourceDir(resDir)
	if err != nil {
		t.Fatal(err)
	}
	rule := findResourceRule(t, "FragmentTagUsage")
	ctx := &api.Context{ResourceIndex: idx, Rule: rule, Collector: scanner.NewFindingCollector(0),
		LibraryFacts: librarymodel.FactsForProfile(profile)}
	rule.Check(ctx)
	var out []string
	for _, f := range api.ContextFindings(ctx) {
		rel, _ := filepath.Rel(resDir, f.File)
		out = append(out, fmt.Sprintf("%s:%d:%d %s", filepath.ToSlash(rel), f.Line, f.Col, f.Message))
	}
	sort.Strings(out)
	return out
}

// Expected positions come from running the lint.jar of androidx.fragment:fragment 1.9.1 through the Android Lint CLI on
// the same files: the tag name of each layout <fragment>, not navigation graph destinations.
func TestFragmentTagUsageLintParity(t *testing.T) {
	files := map[string]string{
		"layout/main.xml": `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools">
    <fragment android:name="com.example.A" />
    <FrameLayout>
        <fragment android:name="com.example.B" />
    </FrameLayout>
    <androidx.fragment.app.FragmentContainerView android:name="com.example.C" />
    <fragment android:name="com.example.D" tools:ignore="FragmentTagUsage" />
</LinearLayout>`,
		"navigation/graph.xml": `<navigation xmlns:android="http://schemas.android.com/apk/res/android">
    <fragment android:id="@+id/home" android:name="com.example.Home" />
</navigation>`,
	}
	fragment := librarymodel.ProjectProfile{HasGradle: true,
		Dependencies: []librarymodel.Dependency{{Group: "androidx.fragment", Name: "fragment-ktx", Version: "1.9.1"}}}
	msg := "Replace the <fragment> tag with FragmentContainerView."
	want := []string{"layout/main.xml:2:6 " + msg, "layout/main.xml:4:10 " + msg}
	if got := fragmentResourceFindings(t, fragment, files); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	// The check ships in the fragment AAR, so a project without fragment (or a library that brings it) gets none.
	if got := fragmentResourceFindings(t, librarymodel.ProjectProfile{HasGradle: true}, files); len(got) != 0 {
		t.Fatalf("without fragment: got %q, want none", got)
	}
}
