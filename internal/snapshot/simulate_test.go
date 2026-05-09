package snapshot

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
)

func TestParseRuleFindingCountReadsByRuleMap(t *testing.T) {
	payload := []byte(`{
		"summary": {
			"byRule": { "MagicNumber": 7, "LongMethod": 3 }
		}
	}`)
	got, err := parseRuleFindingCount(payload, "MagicNumber")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got != 7 {
		t.Fatalf("MagicNumber: got %d, want 7", got)
	}
	got, err = parseRuleFindingCount(payload, "Missing")
	if err != nil {
		t.Fatalf("parse missing: %v", err)
	}
	if got != 0 {
		t.Fatalf("Missing: got %d, want 0 (absent rule = 0 count)", got)
	}
}

func TestParseRuleFindingCountRejectsEmpty(t *testing.T) {
	if _, err := parseRuleFindingCount(nil, "X"); err == nil {
		t.Fatal("expected error for empty payload")
	}
}

func TestSimulateReturnsNewestFirstSeries(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	kritBin := buildKritForTest(t)
	repo := initSimulateRepo(t)

	res, err := Simulate(SimulateOptions{
		RepoRoot: repo,
		Rule:     "MagicNumber",
		Workers:  2,
		KritBin:  kritBin,
	})
	if err != nil {
		t.Fatalf("Simulate: %v", err)
	}
	if len(res.Points) != 3 {
		t.Fatalf("expected 3 points, got %d (failed=%v)", len(res.Points), res.Failed)
	}
	// Newest-first by committer time.
	if !sort.SliceIsSorted(res.Points, func(i, j int) bool {
		return res.Points[i].CommittedAt > res.Points[j].CommittedAt
	}) {
		t.Fatalf("points not newest-first: %+v", res.Points)
	}
	if len(res.Failed) != 0 {
		t.Fatalf("unexpected failures: %v", res.Failed)
	}
}

// buildKritForTest compiles the krit CLI into a temp file and returns
// its path. Skipping when go isn't on PATH keeps simulate tests
// portable.
func buildKritForTest(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not available")
	}
	bin := filepath.Join(t.TempDir(), "krit-test-bin")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.CommandContext(context.Background(), "go", "build", "-o", bin, "github.com/kaeawc/krit/cmd/krit")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Skipf("krit binary build failed: %v", err)
	}
	return bin
}

// initSimulateRepo creates a 3-commit Kotlin repo where each commit
// adds another magic-number-bearing line. Symbol counts are
// uninteresting for the test; we only need enough variation that
// MagicNumber can fire.
func initSimulateRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()

	run := func(args ...string) {
		cmd := exec.CommandContext(context.Background(), "git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=krit-test",
			"GIT_AUTHOR_EMAIL=krit@test",
			"GIT_COMMITTER_NAME=krit-test",
			"GIT_COMMITTER_EMAIL=krit@test",
		)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "commit.gpgsign", "false")

	srcDir := filepath.Join(repo, "src", "main", "kotlin")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for i, body := range []string{
		"fun a() = 13\n",
		"fun a() = 13\nfun b() = 17\n",
		"fun a() = 13\nfun b() = 17\nfun c() = 19\n",
	} {
		_ = i
		path := filepath.Join(srcDir, "Magic.kt")
		if err := os.WriteFile(path, []byte("package demo\n\n"+body), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		run("add", "src/main/kotlin/Magic.kt")
		run("commit", "-q", "-m", "step")
	}
	return repo
}
