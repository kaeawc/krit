package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestAndroidLintProjectFixtures scans every project-shaped fixture under
// tests/fixtures/android-lint/<RuleID>/{positive,negative}. These cover rules
// that read manifests or resource folders, which the single-file fixture
// harness cannot exercise: the positive project must report the rule and the
// negative project must not.
func TestAndroidLintProjectFixtures(t *testing.T) {
	root := filepath.Join("..", "..", "tests", "fixtures", "android-lint")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	ran := 0
	for _, entry := range entries {
		rule := entry.Name()
		for _, kind := range []string{"positive", "negative"} {
			src := filepath.Join(root, rule, kind)
			if info, err := os.Stat(src); err != nil || !info.IsDir() {
				continue
			}
			ran++
			t.Run(rule+"/"+kind, func(t *testing.T) {
				project := filepath.Join(t.TempDir(), "project")
				copyDir(t, src, project)
				stdout, stderr, _ := runKrit(t, "--no-daemon", "--no-cache", "--all-rules", "-f", "json", project)
				var report struct {
					Findings []struct {
						Rule string `json:"rule"`
					} `json:"findings"`
				}
				if err := json.Unmarshal([]byte(stdout), &report); err != nil {
					t.Fatalf("parse report: %v\nstderr: %s", err, stderr)
				}
				count := 0
				for _, f := range report.Findings {
					if f.Rule == rule {
						count++
					}
				}
				if kind == "positive" && count == 0 {
					t.Fatalf("positive fixture reported no %s finding\nstdout: %s", rule, stdout)
				}
				if kind == "negative" && count != 0 {
					t.Fatalf("negative fixture reported %d %s findings\nstdout: %s", count, rule, stdout)
				}
			})
		}
	}
	if ran == 0 {
		t.Fatal("no project fixtures found")
	}
}
