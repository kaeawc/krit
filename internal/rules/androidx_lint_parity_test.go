package rules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// axlParityRoots are the testdata directories of the AndroidX Lint ports checked by TestAndroidxLintParity.
var axlParityRoots = []string{
	"androidx_annotation_experimental",
	"androidx_activity_compose",
	"androidx_compose_animation",
	"androidx_compose_material",
	"androidx_uiautomator",
	"androidx_webkit",
}

// TestAndroidxLintParity checks the AndroidX Lint ports against the real detectors.
//
// Each case directory under a root holds Kotlin/Java inputs, checks.txt (the issue IDs) and one lint-<version>.txt:
// what the Android Lint CLI 9.4 reported for those IDs on the same inputs, run as a standalone project with the
// artifact's lint.jar on the rule path and its classes.jar (plus the Compose, kotlin-stdlib and kotlinx-coroutines
// jars it needs) on the classpath, one `path:line:column: ID: message` line per finding.
//
// divergences.txt, when present, lists where krit deliberately differs, one finding per line with ` # reason`:
// `- finding` is a Lint line krit does not report and `+ finding` is a krit line Lint does not report. krit must
// report exactly Lint's findings with those adjustments. dependencies.txt, when present, lists the `group:name`
// artifacts the project declares, for checks that, like Lint, run only when the artifact shipping them is one.
func TestAndroidxLintParity(t *testing.T) {
	for _, root := range axlParityRoots {
		dir := filepath.Join("testdata", root)
		cases, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range cases {
			if !c.IsDir() {
				continue
			}
			caseDir := filepath.Join(dir, c.Name())
			t.Run(root+"/"+c.Name(), func(t *testing.T) {
				checks := strings.Split(strings.TrimSpace(readParityFile(t, filepath.Join(caseDir, "checks.txt"))), ",")
				lintFiles, _ := filepath.Glob(filepath.Join(caseDir, "lint-*.txt"))
				if len(lintFiles) != 1 {
					t.Fatalf("want exactly one lint-<version>.txt in %s, got %v", caseDir, lintFiles)
				}
				want := map[string]bool{}
				for _, line := range parityLines(readParityFile(t, lintFiles[0])) {
					want[line] = true
				}
				for _, line := range parityLines(readOptionalParityFile(t, filepath.Join(caseDir, "divergences.txt"))) {
					finding, _, _ := strings.Cut(line, " # ")
					finding = strings.TrimSpace(finding)
					switch {
					case strings.HasPrefix(finding, "- "):
						finding = strings.TrimPrefix(finding, "- ")
						if !want[finding] {
							t.Errorf("divergences.txt drops %q, which Lint does not report", finding)
						}
						delete(want, finding)
					case strings.HasPrefix(finding, "+ "):
						want[strings.TrimPrefix(finding, "+ ")] = true
					default:
						t.Errorf("divergences.txt line %q must start with - or +", line)
					}
				}
				var wantLines []string
				for line := range want {
					wantLines = append(wantLines, line)
				}
				wantText := strings.Join(parityLines(strings.Join(wantLines, "\n")), "\n")
				got := strings.Join(runWorkParityChecks(t, caseDir, checks), "\n")
				if got != wantText {
					t.Errorf("krit findings differ from Lint (with divergences.txt)\n--- want\n%s\n--- got\n%s", wantText, got)
				}
			})
		}
	}
}
