package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// gitIn runs git in dir with a fixed identity, so the test does not depend on
// the machine's git config.
func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-c", "user.name=krit-test", "-c", "user.email=krit-test@example.invalid", "-c", "commit.gpgsign=false"}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// An uncommitted edit in a git checkout is stored in the per-file cache. The
// first run after the edit skipped the cache save for a small git-dirty
// delta, expecting the next run to reanalyze the file, but the next runs were
// served from the findings bundle and never saved either. The edited file
// stayed a miss on every later run, and warm cross-file findings stopped
// being reused (#1967).
func TestUncommittedEditIsCachedForLaterRuns(t *testing.T) {
	root := filepath.Join(neutralTempDir(t), "proj")
	files := map[string]string{
		"src/main/kotlin/demo/A.kt": "package demo\n\nclass A {\n    fun a() = B().b()\n}\n",
		"src/main/kotlin/demo/B.kt": "package demo\n\nclass B {\n    fun b() = 1\n}\n",
		"src/main/kotlin/demo/C.kt": "package demo\n\nprivate fun unused() = 2\n",
	}
	for rel, content := range files {
		writeRepoFile(t, root, rel, content)
	}
	gitIn(t, root, "init", "-q")
	gitIn(t, root, "add", ".")
	gitIn(t, root, "commit", "-q", "-m", "seed")

	scan := func(extra ...string) (findings, stderr string) {
		t.Helper()
		args := append([]string{"--no-daemon", "--no-fir", "--no-type-oracle", "-f", "json", "-v"}, extra...)
		stdout, stderr, code := runKritInDir(t, root, append(args, ".")...)
		if code > 1 {
			t.Fatalf("krit %v: exit=%d stderr=%q", args, code, stderr)
		}
		return stdout, stderr
	}
	cached := func(stderr string) string {
		t.Helper()
		m := cachedFilesRE.FindStringSubmatch(stderr)
		if m == nil {
			t.Fatalf("no cache summary in -v output:\n%s", stderr)
		}
		return m[1] + "/" + m[2]
	}

	scan()
	if _, stderr := scan(); cached(stderr) != "3/3" {
		t.Fatalf("no-op run after the prime: cache %s, want 3/3", cached(stderr))
	}
	edited := filepath.Join(root, "src/main/kotlin/demo/B.kt")
	f, err := os.OpenFile(edited, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("\n// comment edit\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if _, stderr := scan(); cached(stderr) != "2/3" {
		t.Fatalf("first run after the edit: cache %s, want 2/3", cached(stderr))
	}
	var warm string
	for run := 2; run <= 3; run++ {
		findings, stderr := scan()
		if cached(stderr) != "3/3" {
			t.Fatalf("run %d after the edit: cache %s, want 3/3 (the edited file's entry was never stored)", run, cached(stderr))
		}
		warm = findings
	}
	fresh, _ := scan("--no-cache")
	if len(findingKeys(t, fresh)) == 0 {
		t.Fatal("--no-cache reported no findings; the test project no longer exercises the cache")
	}
	if got, want := findingKeys(t, warm), findingKeys(t, fresh); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("warm findings after the edit differ from --no-cache:\nwarm:  %v\nfresh: %v", got, want)
	}
}

// findingKeys returns each finding's rule, file and position, sorted, from a
// JSON report.
func findingKeys(t *testing.T, report string) []string {
	t.Helper()
	var out struct {
		Findings []struct {
			Rule   string `json:"rule"`
			File   string `json:"file"`
			Line   int    `json:"line"`
			Column int    `json:"column"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(report), &out); err != nil {
		t.Fatalf("invalid JSON report: %v\n%s", err, report)
	}
	keys := make([]string, len(out.Findings))
	for i, f := range out.Findings {
		keys[i] = fmt.Sprintf("%s %s:%d:%d", f.Rule, f.File, f.Line, f.Column)
	}
	sort.Strings(keys)
	return keys
}
