package rules_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// TestAndroidxRecyclerViewLintParity checks InvalidSetHasFixedSize against the real detector. Each directory under
// testdata/androidx_recyclerview is one project (sources plus res/) with lint-1.4.0.txt: what the Android Lint CLI
// 9.4 reported for checks.txt with the recyclerview 1.4.0 lint.jar on the rule path and the AndroidX classes.jar
// files on the classpath. divergences.txt lists `- finding # reason` lines krit deliberately does not report.
func TestAndroidxRecyclerViewLintParity(t *testing.T) {
	root := filepath.Join("testdata", "androidx_recyclerview")
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
			want := recyclerParityExpected(t, dir)
			got := runResourceSourceParity(t, dir, checks)
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("krit findings differ from Lint 1.4.0 (with divergences.txt applied)\n--- want\n%s\n--- got\n%s",
					strings.Join(want, "\n"), strings.Join(got, "\n"))
			}
		})
	}
}

func recyclerParityExpected(t *testing.T, dir string) []string {
	t.Helper()
	remaining := map[string]int{}
	lint := parityLines(readParityFile(t, filepath.Join(dir, "lint-1.4.0.txt")))
	for _, line := range lint {
		remaining[line]++
	}
	for _, line := range parityLines(readOptionalParityFile(t, filepath.Join(dir, "divergences.txt"))) {
		finding, reason, _ := strings.Cut(line, " # ")
		finding = strings.TrimSpace(strings.TrimPrefix(finding, "- "))
		if strings.TrimSpace(reason) == "" || remaining[finding] == 0 {
			t.Errorf("divergences.txt line %q must remove a Lint finding and give a ` # reason`", line)
			continue
		}
		remaining[finding]--
	}
	var want []string
	for _, line := range lint {
		if remaining[line] > 0 {
			remaining[line]--
			want = append(want, line)
		}
	}
	return want
}

// runResourceSourceParity runs resource-source checks over every source file with the merged index of the
// project's res/ directory.
func runResourceSourceParity(t *testing.T, dir string, checks []string) []string {
	t.Helper()
	idx, err := android.ScanResourceDir(filepath.Join(dir, "res"))
	if err != nil {
		t.Fatal(err)
	}
	var selected []*api.Rule
	for _, r := range api.Registry {
		for _, c := range checks {
			if r.ID == c {
				selected = append(selected, r)
			}
		}
	}
	d := rules.NewDispatcher(selected, nil)
	var out []string
	err = filepath.WalkDir(dir, func(path string, e os.DirEntry, err error) error {
		if err != nil || e.IsDir() {
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
		cols := d.RunResourceSource(file, idx)
		for _, f := range cols.Findings() {
			rel, _ := filepath.Rel(dir, path)
			out = append(out, fmt.Sprintf("%s:%d:%d: %s: %s", filepath.ToSlash(rel), f.Line, f.Col, f.Rule, f.Message))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}
