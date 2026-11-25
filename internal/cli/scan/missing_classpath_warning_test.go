package scan

import (
	"strings"
	"testing"
)

func TestMissingClasspathWarningSplitsBuildOutputs(t *testing.T) {
	if got := missingClasspathWarning("m", nil, 10); got != "" {
		t.Fatalf("no missing entries should be silent, got %q", got)
	}
	got := missingClasspathWarning("m", []string{"/p/mod/build/classes/main", "/p/mod/build/libs/a.jar", "/cache/dep.jar"}, 100)
	for _, want := range []string{"3 classpath entries", "2 module build outputs", "1 other files", "kritExportModel"} {
		if !strings.Contains(got, want) {
			t.Errorf("warning %q lacks %q", got, want)
		}
	}
	only := missingClasspathWarning("m", []string{"/cache/dep.jar"}, 100)
	if strings.Contains(only, "build outputs") {
		t.Errorf("warning %q mentions build outputs with none missing", only)
	}
}

// A model whose classpath is mostly gone gates most FIR files; the warning
// says so instead of only "resolve as unknown" (#1573).
func TestMissingClasspathWarningPredictsMostlyGatedResults(t *testing.T) {
	missing := make([]string, 30)
	for i := range missing {
		missing[i] = "/cache/dep.jar"
	}
	few := missingClasspathWarning("m", missing, 1000)
	if strings.Contains(few, "mostly gated") {
		t.Errorf("3%% missing should not predict mostly gated results: %q", few)
	}
	most := missingClasspathWarning("m", missing, 40)
	for _, want := range []string{"30 of 40 entries (75%)", "FIR results will be mostly gated", "rerun kritExportModel"} {
		if !strings.Contains(most, want) {
			t.Errorf("warning %q lacks %q", most, want)
		}
	}
}
