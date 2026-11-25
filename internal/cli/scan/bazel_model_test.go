package scan

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestMissingClasspathWarningBazelModel(t *testing.T) {
	dir := filepath.Join("ws", ".krit", "bazel-model")
	got := missingClasspathWarning(dir, []string{"/execroot/bazel-out/k8-fastbuild/bin/lib/lib.abi.jar"}, 4)
	if !strings.Contains(got, "bazel model") || !strings.Contains(got, "bazel run @krit//:export_model") {
		t.Fatalf("warning = %q, want the bazel export_model hint", got)
	}
	if strings.Contains(got, "kritExportModel") {
		t.Fatalf("warning = %q names the Gradle task for a Bazel model", got)
	}
	if got := missingClasspathWarning(dir, nil, 4); got != "" {
		t.Fatalf("no missing entries: %q", got)
	}
	gradle := missingClasspathWarning(filepath.Join("ws", ".krit", "gradle-model"), []string{"/x/build/libs/a.jar"}, 4)
	if !strings.Contains(gradle, "gradle model") {
		t.Fatalf("gradle warning = %q", gradle)
	}
}
