package rules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSlackLintParity checks the slack-lints 0.11.1 ports outside MoshiUsage against the real detectors.
//
// Each directory under testdata/slack_lint (except stubs) is one project: Kotlin/Java inputs under src/,
// checks.txt (the issue IDs it checks, comma-separated) and lint-0.11.1.txt, what the Android Lint CLI 9.4.0-rc01
// running slack-lint-checks 0.11.1 reported for those IDs on the same inputs, one `path:line:column: ID: message`
// line per finding. testdata/slack_lint/README.md describes how to regenerate it.
//
// divergences.txt, when present, lists where krit deliberately differs, one finding per line with ` # reason`:
// `- finding` is a Lint line krit does not report and `+ finding` is a krit line Lint does not report. krit must
// report exactly Lint's findings with those adjustments.
func TestSlackLintParity(t *testing.T) {
	root := filepath.Join("testdata", "slack_lint")
	cases, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	ran := 0
	for _, c := range cases {
		if !c.IsDir() || c.Name() == "stubs" {
			continue
		}
		ran++
		dir := filepath.Join(root, c.Name())
		t.Run(c.Name(), func(t *testing.T) {
			checks := strings.Split(strings.TrimSpace(readParityFile(t, filepath.Join(dir, "checks.txt"))), ",")
			want := slackParityExpected(t, dir)
			got := runWorkParityChecks(t, dir, checks)
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("krit findings differ from slack-lints 0.11.1 (with divergences.txt applied)\n--- want\n%s\n--- got\n%s",
					strings.Join(want, "\n"), strings.Join(got, "\n"))
			}
		})
	}
	if ran == 0 {
		t.Fatal("no slack-lint parity cases found")
	}
}

// slackParityExpected applies divergences.txt to Lint's findings.
func slackParityExpected(t *testing.T, dir string) []string {
	t.Helper()
	lint := parityLines(readParityFile(t, filepath.Join(dir, "lint-0.11.1.txt")))
	remaining := map[string]int{}
	for _, line := range lint {
		remaining[line]++
	}
	var added []string
	for _, line := range parityLines(readOptionalParityFile(t, filepath.Join(dir, "divergences.txt"))) {
		finding, reason, _ := strings.Cut(line, " # ")
		if strings.TrimSpace(reason) == "" {
			t.Errorf("divergences.txt line %q has no ` # reason`", line)
		}
		switch {
		case strings.HasPrefix(finding, "- "):
			finding = strings.TrimSpace(strings.TrimPrefix(finding, "- "))
			if remaining[finding] == 0 {
				t.Errorf("divergences.txt removes %q, which Lint does not report (or reports fewer times)", finding)
				continue
			}
			remaining[finding]--
		case strings.HasPrefix(finding, "+ "):
			added = append(added, strings.TrimSpace(strings.TrimPrefix(finding, "+ ")))
		default:
			t.Errorf("divergences.txt line %q must start with `- ` or `+ `", line)
		}
	}
	var want []string
	for _, line := range lint {
		if remaining[line] > 0 {
			remaining[line]--
			want = append(want, line)
		}
	}
	return parityLines(strings.Join(append(want, added...), "\n"))
}
