package scan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestScanSkipsDocumentationSamples runs the whole scan on a synthetic Git checkout and checks that
// a sample under references/examples with no build file above it is never analysed, tracked or not.
func TestScanSkipsDocumentationSamples(t *testing.T) {
	if !gitAvailable(t) {
		t.Skip("git not available")
	}
	const source = "package demo\n\nfun use(unused: Int): Int { return 1 }\n"
	for _, tracked := range []bool{true, false} {
		name := "walked"
		if tracked {
			name = "git-listed"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			sample := filepath.Join(dir, "docs", "plugin", "references", "examples")
			module := filepath.Join(dir, "app", "src", "main", "kotlin")
			for _, d := range []string{sample, module} {
				if err := os.MkdirAll(d, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			writeContractFile(t, filepath.Join(dir, "build.gradle.kts"), "")
			writeContractFile(t, filepath.Join(sample, "Sample.kt"), source)
			writeContractFile(t, filepath.Join(module, "Real.kt"), source)
			if tracked {
				gitInit(t, dir)
				gitAdd(t, dir, ".")
			} else if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
				// A bare .git directory marks the root but makes git ls-files fail, so the walk runs.
				t.Fatal(err)
			}

			out := runScanCLI(t, []string{"krit", "--no-cache", "--no-cross-file-cache", "--no-type-oracle", "-f", "json", dir})
			var report struct {
				Findings []struct {
					File string `json:"file"`
					Rule string `json:"rule"`
				} `json:"findings"`
			}
			if err := json.Unmarshal(out, &report); err != nil {
				t.Fatalf("decode report: %v\n%s", err, out)
			}
			sawReal := false
			for _, f := range report.Findings {
				if strings.HasSuffix(f.File, "Sample.kt") {
					t.Errorf("documentation sample was analysed: %s %s", f.Rule, f.File)
				}
				if strings.HasSuffix(f.File, "Real.kt") {
					sawReal = true
				}
			}
			if !sawReal {
				t.Fatalf("the module file produced no finding, so the check is vacuous: %s", out)
			}
		})
	}
}
