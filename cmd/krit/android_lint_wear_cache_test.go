package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// WearBackNavigation decides from the module manifest whether a res directory belongs to a Wear app. A warm run must
// follow the manifest: dropping or restoring the watch feature has to change the findings exactly as a cold run does.
func TestWearBackNavigationWarmRunFollowsManifest(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	copyDir(t, filepath.Join("..", "..", "tests", "fixtures", "android-lint", "WearBackNavigation", "positive"), project)
	manifest := filepath.Join(project, "src", "main", "AndroidManifest.xml")
	wear, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	phone := strings.Replace(string(wear), `<uses-feature android:name="android.hardware.type.watch" />`, "", 1)
	if phone == string(wear) {
		t.Fatal("fixture manifest no longer declares the watch feature")
	}
	count := func(args ...string) int {
		t.Helper()
		stdout, stderr, _ := runKrit(t, append(append([]string{"--no-daemon", "-f", "json"}, args...), project)...)
		var report struct {
			Findings []struct {
				Rule string `json:"rule"`
			} `json:"findings"`
		}
		if err := json.Unmarshal([]byte(stdout), &report); err != nil {
			t.Fatalf("parse report: %v\nstderr: %s", err, stderr)
		}
		n := 0
		for _, f := range report.Findings {
			if f.Rule == "WearBackNavigation" {
				n++
			}
		}
		return n
	}
	write := func(content string) {
		t.Helper()
		if err := os.WriteFile(manifest, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := count(); got != 3 {
		t.Fatalf("cold run on the Wear module: %d findings, want 3", got)
	}
	write(phone)
	if warm, cold := count(), count("--no-cache"); warm != cold || cold != 0 {
		t.Fatalf("after removing the watch feature: warm run %d findings, cold run %d, want 0 for both", warm, cold)
	}
	write(string(wear))
	if warm, cold := count(), count("--no-cache"); warm != cold || cold != 3 {
		t.Fatalf("after restoring the watch feature: warm run %d findings, cold run %d, want 3 for both", warm, cold)
	}
}
