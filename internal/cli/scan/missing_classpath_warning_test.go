package scan

import (
	"strings"
	"testing"
)

func TestMissingClasspathWarningSplitsBuildOutputs(t *testing.T) {
	if got := missingClasspathWarning("m", nil); got != "" {
		t.Fatalf("no missing entries should be silent, got %q", got)
	}
	got := missingClasspathWarning("m", []string{"/p/mod/build/classes/main", "/p/mod/build/libs/a.jar", "/cache/dep.jar"})
	for _, want := range []string{"3 classpath entries", "2 module build outputs", "1 other files", "kritExportModel"} {
		if !strings.Contains(got, want) {
			t.Errorf("warning %q lacks %q", got, want)
		}
	}
	only := missingClasspathWarning("m", []string{"/cache/dep.jar"})
	if strings.Contains(only, "build outputs") {
		t.Errorf("warning %q mentions build outputs with none missing", only)
	}
}
