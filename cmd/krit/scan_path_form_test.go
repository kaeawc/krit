package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
)

// `krit .` and `krit $PWD` must report the same findings. Rules test path
// segments such as "/test-utils/"; when scan paths stayed relative, a file
// under a top-level test-utils/ directory had no leading slash and was
// treated as production code under `krit .` only (72 extra findings on
// kotlinx.coroutines).
func TestScanPathFormDoesNotChangeFindings(t *testing.T) {
	dir := t.TempDir()
	src := "package helpers\n\nfun timeout(): Long = 1234L * 5678L\n\nfun main() {\n    println(timeout())\n}\n"
	for _, rel := range []string{"test-utils/common/src/Helpers.kt", "core/src/main/kotlin/helpers/Core.kt"} {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	scan := func(target string) []string {
		t.Helper()
		cmd := exec.Command(binPath, "--no-fir", "--no-daemon", "--no-cache", "--no-type-oracle", "-f", "json", "-q", target)
		cmd.Dir = dir
		out, err := cmd.Output()
		if err != nil {
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() > 1 {
				t.Fatalf("krit %s: %v", target, err)
			}
		}
		var report struct {
			Findings []scopeFinding `json:"findings"`
		}
		if err := json.Unmarshal(out, &report); err != nil {
			t.Fatalf("krit %s: invalid JSON: %v", target, err)
		}
		keys := make([]string, len(report.Findings))
		for i, f := range report.Findings {
			keys[i] = f.Rule + " " + f.File
		}
		sort.Strings(keys)
		return keys
	}
	relative, absolute := scan("."), scan(dir)
	if len(relative) == 0 {
		t.Fatal("expected findings in core/")
	}
	if len(relative) != len(absolute) {
		t.Fatalf("krit . found %d, krit <abs> found %d:\n%v\n%v", len(relative), len(absolute), relative, absolute)
	}
	for i := range relative {
		if relative[i] != absolute[i] {
			t.Errorf("finding %d: %q vs %q", i, relative[i], absolute[i])
		}
	}
}
