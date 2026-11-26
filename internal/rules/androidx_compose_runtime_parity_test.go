package rules_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// TestAndroidxComposeRuntimeLintParity checks the AndroidX Compose runtime Lint ports against the real detectors.
//
// Each directory under testdata/androidx_compose_runtime holds Kotlin/Java inputs, checks.txt (the issue IDs),
// and lint-1.12.1.txt: the findings the Lint CLI reported for those IDs on the same inputs, with the lint.jar
// files from androidx.compose.runtime:runtime-android, runtime-retain-android and runtime-saveable-android 1.12.1
// on the rule path and their classes.jar files, kotlin-stdlib and kotlinx-coroutines-core on the classpath, one
// `path:line:column: ID: message` line per finding. known-false-negatives.txt, when present, lists Lint findings
// the Go path cannot prove from source (each line is a lint-1.12.1.txt line, then ` # reason`); krit must report
// exactly the remaining findings and nothing else.
func TestAndroidxComposeRuntimeLintParity(t *testing.T) {
	root := filepath.Join("testdata", "androidx_compose_runtime")
	cases, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		if !c.IsDir() {
			continue
		}
		dir := filepath.Join(root, c.Name())
		t.Run(c.Name(), func(t *testing.T) {
			checks := strings.Split(strings.TrimSpace(readParityFile(t, filepath.Join(dir, "checks.txt"))), ",")
			lint := parityLines(readParityFile(t, filepath.Join(dir, "lint-1.12.1.txt")))
			knownFN := map[string]bool{}
			for _, line := range parityLines(readOptionalParityFile(t, filepath.Join(dir, "known-false-negatives.txt"))) {
				finding, _, _ := strings.Cut(line, " # ")
				knownFN[strings.TrimSpace(finding)] = true
			}
			var want []string
			lintSet := map[string]bool{}
			for _, line := range lint {
				lintSet[line] = true
				if !knownFN[line] {
					want = append(want, line)
				}
			}
			for fn := range knownFN {
				if !lintSet[fn] {
					t.Errorf("known-false-negatives.txt lists %q, which Lint does not report", fn)
				}
			}
			got := runParityChecks(t, dir, checks)
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("krit findings differ from Lint 1.12.1 (minus known false negatives)\n--- want\n%s\n--- got\n%s",
					strings.Join(want, "\n"), strings.Join(got, "\n"))
			}
		})
	}
}

func runParityChecks(t *testing.T, dir string, checks []string) []string {
	t.Helper()
	deps := parityDependencies(t, dir)
	var out []string
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
		rel, _ := filepath.Rel(dir, path)
		for _, check := range checks {
			for _, f := range runParityRule(t, check, file, deps) {
				out = append(out, fmt.Sprintf("%s:%d:%d: %s: %s", filepath.ToSlash(rel), f.Line, f.Col, f.Rule, f.Message))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}

// parityDependencies reads dependencies.txt, when present: the `group:name` artifacts the case's project declares,
// for checks that, like Lint, run only when the artifact shipping them is a dependency.
func parityDependencies(t *testing.T, dir string) []librarymodel.Dependency {
	t.Helper()
	var deps []librarymodel.Dependency
	for _, line := range parityLines(readOptionalParityFile(t, filepath.Join(dir, "dependencies.txt"))) {
		group, name, ok := strings.Cut(line, ":")
		if !ok {
			t.Fatalf("dependencies.txt line %q must be group:name", line)
		}
		deps = append(deps, librarymodel.Dependency{Group: group, Name: name})
	}
	return deps
}

// runParityRule runs one rule on file, with library facts declaring deps when there are any.
func runParityRule(t *testing.T, ruleID string, file *scanner.File, deps []librarymodel.Dependency) []scanner.Finding {
	t.Helper()
	if len(deps) == 0 {
		return runRuleByNameOnFile(t, ruleID, file)
	}
	rule := buildRuleIndex()[ruleID]
	if rule == nil {
		t.Fatalf("rule %q not found in registry", ruleID)
	}
	d := rules.NewDispatcher([]*api.Rule{rule}, nil)
	d.SetLibraryFacts(librarymodel.FactsForProfile(librarymodel.ProjectProfile{Dependencies: deps}))
	cols := d.Run(file)
	return cols.Findings()
}

func readParityFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func readOptionalParityFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func parityLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	sort.Strings(out)
	return out
}
