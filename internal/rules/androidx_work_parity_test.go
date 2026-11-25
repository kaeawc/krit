package rules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAndroidxWorkLintParity checks the androidx.work 2.12.0 Lint ports against the real detectors.
//
// Each directory under testdata/androidx_work holds Kotlin/Java inputs, checks.txt (the issue IDs) and
// lint-2.12.0.txt: what the Android Lint CLI 9.4 reported for those IDs on the same inputs, with work-runtime,
// work-rxjava2/3, lifecycle and the other AndroidX classes.jar files plus kotlin-stdlib and RxJava on the classpath
// and the work-runtime lint.jar on the rule path, one `path:line:column: ID: message` line per finding.
//
// divergences.txt, when present, lists where krit deliberately differs, one finding per line with ` # reason`:
// `- finding` is a Lint line krit does not report and `+ finding` is a krit line Lint does not report. krit must
// report exactly Lint's findings with those adjustments.
func TestAndroidxWorkLintParity(t *testing.T) {
	root := filepath.Join("testdata", "androidx_work")
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
			want := workParityExpected(t, dir)
			got := runParityChecks(t, dir, checks)
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("krit findings differ from Lint 2.12.0 (with divergences.txt applied)\n--- want\n%s\n--- got\n%s",
					strings.Join(want, "\n"), strings.Join(got, "\n"))
			}
		})
	}
}

// workParityExpected applies divergences.txt to Lint's findings.
func workParityExpected(t *testing.T, dir string) []string {
	t.Helper()
	remaining := map[string]int{}
	lint := parityLines(readParityFile(t, filepath.Join(dir, "lint-2.12.0.txt")))
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
