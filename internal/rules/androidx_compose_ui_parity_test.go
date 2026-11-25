package rules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAndroidxComposeUILintParity checks the AndroidX Compose UI Lint ports against the real detectors.
//
// Each directory under testdata/androidx_compose_ui holds Kotlin/Java inputs, checks.txt (the issue IDs) and
// lint-1.12.1.txt: the findings the Lint CLI reported for those IDs on the same inputs, with the lint.jar files from
// androidx.compose.ui:ui-android, ui-graphics-android, ui-text-android and ui-test-manifest 1.12.1 on the rule path
// and the Compose classes.jar files, kotlin-stdlib, kotlinx-coroutines-core and android.jar on the classpath, one
// `path:line:column: ID: message` line per finding. known-false-negatives.txt, when present, lists Lint findings the
// Go path cannot prove from source (a lint-1.12.1.txt line, then ` # reason`); krit must report exactly the
// remaining findings and nothing else. The file layout matches TestAndroidxComposeRuntimeLintParity.
func TestAndroidxComposeUILintParity(t *testing.T) {
	root := filepath.Join("testdata", "androidx_compose_ui")
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
			knownFN := map[string]bool{}
			for _, line := range parityLines(readOptionalParityFile(t, filepath.Join(dir, "known-false-negatives.txt"))) {
				finding, _, _ := strings.Cut(line, " # ")
				knownFN[strings.TrimSpace(finding)] = true
			}
			var want []string
			lintSet := map[string]bool{}
			for _, line := range parityLines(readParityFile(t, filepath.Join(dir, "lint-1.12.1.txt"))) {
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
