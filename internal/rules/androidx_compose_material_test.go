package rules

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/librarymodel"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// runAxlRule runs one registered rule on a Kotlin source under the given project profile.
func runAxlRule(t *testing.T, ruleID, name, source string, profile librarymodel.ProjectProfile) []scanner.Finding {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range api.Registry {
		if r.ID != ruleID {
			continue
		}
		dispatcher := NewDispatcher([]*api.Rule{r}, nil)
		dispatcher.SetLibraryFacts(librarymodel.FactsForProfile(profile))
		cols := dispatcher.Run(file)
		return cols.Findings()
	}
	t.Fatalf("rule %s not registered", ruleID)
	return nil
}

func TestUsingMaterialAndMaterial3LibrariesNeedsMaterial3(t *testing.T) {
	src := `package test

import androidx.compose.material.Button
import androidx.compose.material.Text
`
	if got := runAxlRule(t, "UsingMaterialAndMaterial3Libraries", "M2.kt", src, librarymodel.ProjectProfile{}); len(got) != 0 {
		t.Fatalf("material-only file without material3 evidence: want no findings, got %v", got)
	}
	withM3 := librarymodel.ProjectProfile{HasGradle: true, Dependencies: []librarymodel.Dependency{
		{Group: "androidx.compose.material3", Name: "material3"},
	}}
	got := runAxlRule(t, "UsingMaterialAndMaterial3Libraries", "M2.kt", src, withM3)
	if len(got) != 2 {
		t.Fatalf("material imports in a build depending on material3: want 2 findings, got %v", got)
	}
	if got[0].Line != 3 || got[0].Col != 8 {
		t.Errorf("want the finding at the import reference (3:8), got %d:%d", got[0].Line, got[0].Col)
	}
	material3Lookalike := `package test

import androidx.compose.material3x.Card
import androidx.compose.material.Text
`
	if got := runAxlRule(t, "UsingMaterialAndMaterial3Libraries", "X.kt", material3Lookalike,
		librarymodel.ProjectProfile{}); len(got) != 0 {
		t.Fatalf("a material3x import is not material3: want no findings, got %v", got)
	}
}

func TestScaffoldPaddingInnerLambdaShadowing(t *testing.T) {
	src := `package test

import androidx.compose.material.Scaffold
import androidx.compose.material.Text
import androidx.compose.runtime.Composable

fun <T> withOne(block: (T) -> Unit) {}
fun withNone(block: () -> Unit) {}

@Composable
fun Screen() {
    Scaffold { withOne<Int> { Text("$it") } }
    Scaffold { withNone { Text("$it") } }
    Scaffold { unknownHelper { Text("$it") } }
    Scaffold { padding -> withOne<Int> { padding -> Text("$padding") } }
    Scaffold { padding -> fun(padding: Int) = padding }
}
`
	got := runAxlRule(t, "UnusedMaterialScaffoldPaddingParameter", "S.kt", src, librarymodel.ProjectProfile{})
	var lines []int
	for _, f := range got {
		lines = append(lines, f.Line)
	}
	// Line 12: withOne's lambda has its own `it`. Line 15: the inner lambda's padding shadows the outer one. An
	// anonymous function (line 16) does not shadow, as in Lint, and an unresolved helper (line 14) is assumed not
	// to rebind `it`.
	want := []int{12, 15}
	if len(lines) != len(want) || lines[0] != want[0] || lines[1] != want[1] {
		t.Fatalf("want findings on lines %v, got %v", want, lines)
	}
}

// #1728: Lint runs material3's checks only in modules that depend on material3, so in a multi-module build a
// material-only module is not reported even though another module depends on material3.
func TestUsingMaterialAndMaterial3LibrariesIsModuleScoped(t *testing.T) {
	src := `package legacy

import androidx.compose.material.Button
import androidx.compose.material.Text
`
	root := t.TempDir()
	app, legacy := filepath.Join(root, "app"), filepath.Join(root, "legacy")
	profile := librarymodel.ProjectProfile{HasGradle: true, Dependencies: []librarymodel.Dependency{
		{Group: "androidx.compose.material3", Name: "material3", Source: filepath.Join(app, "build.gradle.kts")},
		{Group: "androidx.compose.material", Name: "material", Source: filepath.Join(legacy, "build.gradle.kts")},
	}}
	run := func(path string) int {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
		file, err := scanner.ParseFile(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range api.Registry {
			if r.ID == "UsingMaterialAndMaterial3Libraries" {
				d := NewDispatcher([]*api.Rule{r}, nil)
				d.SetLibraryFacts(librarymodel.FactsForProfile(profile))
				cols := d.Run(file)
				return len(cols.Findings())
			}
		}
		t.Fatal("rule not registered")
		return 0
	}
	if n := run(filepath.Join(legacy, "src", "main", "kotlin", "A.kt")); n != 0 {
		t.Errorf("material-only module of a multi-module build: want no findings, got %d", n)
	}
	if n := run(filepath.Join(app, "src", "main", "kotlin", "B.kt")); n != 2 {
		t.Errorf("module that depends on material3: want 2 findings, got %d", n)
	}
	if n := run(filepath.Join(root, "other", "C.kt")); n != 0 {
		t.Errorf("file outside every module with dependencies: want no findings, got %d", n)
	}
}
