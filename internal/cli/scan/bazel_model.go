package scan

import (
	"fmt"
	"path/filepath"
)

// bazelModelMissingWarning is missingClasspathWarning for a model the krit
// Bazel module exported (.krit/bazel-model): its entries live in Bazel's
// output tree, which `bazel clean` or a changed BUILD graph removes, and
// re-exporting rebuilds them. "" for a Gradle model or nothing missing.
func bazelModelMissingWarning(dir string, missing []string) string {
	if len(missing) == 0 || filepath.Base(dir) != "bazel-model" {
		return ""
	}
	return fmt.Sprintf("warning: bazel model %s lists %d classpath entries missing on disk (after bazel clean or a BUILD change); rerun `bazel run @krit//:export_model`; types from them resolve as unknown", dir, len(missing))
}
