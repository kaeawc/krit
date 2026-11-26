package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestAndroidLintParityFixtures replays krit on every project fixture under
// tests/fixtures/android-lint/<RuleID>/<case>/ that carries a lint-32.4.1.txt:
// what Android Lint 32.4.1 (AGP 9.4.1) reported for <RuleID> on the same
// Gradle module, recorded by scripts/lint-parity-record.sh. krit must report
// exactly those `path:line:column: ID: message` lines for the rule, adjusted
// by divergences.txt when present: `- finding # reason` is a Lint line krit
// deliberately does not report and `+ finding # reason` a krit line Lint does
// not report. An empty lint-32.4.1.txt pins a case where Lint reports nothing.
// An optional severity.txt names the severity Lint gave every finding of the
// case (error, warning, ...), for checks whose severity depends on the project.
func TestAndroidLintParityFixtures(t *testing.T) {
	root := filepath.Join("..", "..", "tests", "fixtures", "android-lint")
	cases, err := filepath.Glob(filepath.Join(root, "*", "*", "lint-32.4.1.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("no recorded Lint parity fixtures found")
	}
	for _, recorded := range cases {
		src := filepath.Dir(recorded)
		rule := filepath.Base(filepath.Dir(src))
		t.Run(rule+"/"+filepath.Base(src), func(t *testing.T) {
			t.Parallel()
			want := lintParityExpected(t, src, rule)
			project := filepath.Join(t.TempDir(), "project")
			copyDir(t, src, project)
			stdout, stderr, _ := runKrit(t, "--no-daemon", "--no-cache", "--all-rules", "-f", "json", project)
			var report struct {
				Findings []struct {
					File     string `json:"file"`
					Line     int    `json:"line"`
					Column   int    `json:"column"`
					Rule     string `json:"rule"`
					Severity string `json:"severity"`
					Message  string `json:"message"`
				} `json:"findings"`
			}
			if err := json.Unmarshal([]byte(stdout), &report); err != nil {
				t.Fatalf("parse report: %v\nstderr: %s", err, stderr)
			}
			wantSeverity := ""
			if data, err := os.ReadFile(filepath.Join(src, "severity.txt")); err == nil {
				wantSeverity = strings.TrimSpace(string(data))
			}
			var got []string
			for _, f := range report.Findings {
				if f.Rule != rule {
					continue
				}
				if wantSeverity != "" && f.Severity != wantSeverity {
					t.Errorf("%s:%d: severity %q, Lint reported %q", f.File, f.Line, f.Severity, wantSeverity)
				}
				// krit prints paths relative to the scanned root, or absolute ones outside it.
				rel := f.File
				if filepath.IsAbs(rel) {
					if r, err := filepath.Rel(project, rel); err == nil {
						rel = r
					}
				}
				got = append(got, fmt.Sprintf("%s:%d:%d: %s: %s", filepath.ToSlash(rel), f.Line, f.Column, f.Rule, f.Message))
			}
			sort.Strings(got)
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("krit findings differ from Lint 32.4.1 (divergences.txt applied)\n--- want\n%s\n--- got\n%s",
					strings.Join(want, "\n"), strings.Join(got, "\n"))
			}
		})
	}
}

// lintParityExpected reads lint-32.4.1.txt and applies divergences.txt.
func lintParityExpected(t *testing.T, dir, rule string) []string {
	t.Helper()
	lint := lintParityLines(t, filepath.Join(dir, "lint-32.4.1.txt"))
	remaining := map[string]int{}
	for _, line := range lint {
		if !strings.Contains(line, ": "+rule+": ") {
			t.Fatalf("lint-32.4.1.txt line %q is not a %s finding; record each case with only its rule", line, rule)
		}
		remaining[line]++
	}
	var added []string
	divergences := filepath.Join(dir, "divergences.txt")
	if _, err := os.Stat(divergences); err == nil {
		for _, line := range lintParityLines(t, divergences) {
			finding, reason, _ := strings.Cut(line, " # ")
			if strings.TrimSpace(reason) == "" {
				t.Errorf("divergences.txt line %q has no ` # reason`", line)
			}
			switch {
			case strings.HasPrefix(finding, "- "):
				finding = strings.TrimSpace(strings.TrimPrefix(finding, "- "))
				if remaining[finding] == 0 {
					t.Errorf("divergences.txt removes %q, which Lint does not report", finding)
					continue
				}
				remaining[finding]--
			case strings.HasPrefix(finding, "+ "):
				added = append(added, strings.TrimSpace(strings.TrimPrefix(finding, "+ ")))
			default:
				t.Errorf("divergences.txt line %q must start with `- ` or `+ `", line)
			}
		}
	}
	var want []string
	for _, line := range lint {
		if remaining[line] > 0 {
			remaining[line]--
			want = append(want, line)
		}
	}
	want = append(want, added...)
	sort.Strings(want)
	return want
}

func lintParityLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}
