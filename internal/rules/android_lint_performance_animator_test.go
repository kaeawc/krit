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
	"github.com/kaeawc/krit/internal/scanner"
)

func animatorKeepFindings(t *testing.T, dir string, facts *librarymodel.Facts) []string {
	t.Helper()
	var out []string
	for _, f := range runParsedFilesRuleWithFacts(t, "AnimatorKeep", facts, parseParityDirFiles(t, dir)...) {
		out = append(out, fmt.Sprintf("%s:%d:%d", filepath.Base(f.File), f.Line, f.Col))
	}
	sort.Strings(out)
	return out
}

// Same-file setters, where the Lint 32.4.1 CLI reports these exact positions: a Java setter once however many
// animators use it, a Kotlin `var` at its name (a property-level @Keep annotates the property, not the setter, so
// Lint still reports it), a Kotlin `fun setX`; @Keep on the function and @set:Keep are honored.
func TestAnimatorKeep_SameFileMatchesLint(t *testing.T) {
	got := animatorKeepFindings(t, filepath.Join("testdata", "android_lint_performance_animator", "same"), nil)
	want := []string{"JSame.java:10:17", "JSame.java:12:17", "Same.kt:11:9", "Same.kt:14:9", "Same.kt:9:9"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

// Setters in another file than the animator. Lint 32.4.1 reports the same five setters (Gauge.level, Gauge.kept,
// Gauge.count, Gauge.setRadius, JGauge.setLevel) but places them at offsets of the declaring file applied to the
// calling file; krit reports each at its declaration. Library setters (View.setAlpha), @Keep classes and
// functions, mismatched signatures and static setters are not reported.
func TestAnimatorKeep_CrossFileSetters(t *testing.T) {
	got := animatorKeepFindings(t, filepath.Join("testdata", "android_lint_performance_animator", "cross"), nil)
	want := []string{"JGauge.java:8:17", "Views.kt:10:9", "Views.kt:11:9", "Views.kt:12:9", "Views.kt:8:9"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

// Lint checks only modules that shrink: a Gradle project without minification reports nothing.
func TestAnimatorKeep_RequiresShrinking(t *testing.T) {
	dir := filepath.Join("testdata", "android_lint_performance_animator", "same")
	noMinify := librarymodel.FactsForProfile(librarymodel.ProjectProfile{HasGradle: true})
	if got := animatorKeepFindings(t, dir, noMinify); len(got) != 0 {
		t.Fatalf("no minification, got %q", got)
	}
	minify := librarymodel.FactsForProfile(librarymodel.ProjectProfile{HasGradle: true,
		Android: librarymodel.AndroidProfile{Minify: librarymodel.PresencePresent}})
	if got := animatorKeepFindings(t, dir, minify); len(got) != 5 {
		t.Fatalf("minification on, got %q", got)
	}
}

func TestProjectProfileRecordsMinify(t *testing.T) {
	cases := map[string]bool{
		"android {\n    buildTypes {\n        release {\n            isMinifyEnabled = true\n        }\n    }\n}\n":  true,
		"android {\n    buildTypes {\n        release {\n            minifyEnabled true\n        }\n    }\n}\n":      true,
		"android {\n    buildTypes {\n        release {\n            isMinifyEnabled = false\n        }\n    }\n}\n": false,
		"android {\n    // minifyEnabled true\n}\n":                                                                  false,
	}
	for content, want := range cases {
		profile := librarymodel.ProfileFromGradleContentWithCatalog("build.gradle.kts", content, librarymodel.VersionCatalog{})
		if got := profile.Android.Minify == librarymodel.PresencePresent; got != want {
			t.Errorf("Minify = %v, want %v for:\n%s", got, want, strings.TrimSpace(content))
		}
	}
}

// parseParityDirFiles parses every Kotlin and Java file under dir.
func parseParityDirFiles(t *testing.T, dir string) []*scanner.File {
	t.Helper()
	var files []*scanner.File
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		var file *scanner.File
		switch filepath.Ext(path) {
		case ".kt":
			file, err = scanner.ParseFile(context.Background(), path)
		case ".java":
			file, err = scanner.ParseJavaFile(context.Background(), path)
		default:
			return nil
		}
		if err != nil {
			return err
		}
		files = append(files, file)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// A constructor `var` compiles to a setter and is reported at its name, as the Lint 32.4.1 CLI does; a `val` has no
// setter.
func TestAnimatorKeep_ConstructorProperty(t *testing.T) {
	got := animatorKeepFindings(t, filepath.Join("testdata", "android_lint_performance_animator", "ctor"), nil)
	if want := []string{"Ctor.kt:7:35"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}
