package riskmap

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestFileComplexityCountsDecisions(t *testing.T) {
	got := fileComplexity([]string{
		"fun risky(x: Int) {",
		"if (x > 0 && x < 10) println(x)",
		"when (x) { 1 -> println(1) }",
		"}",
	})
	if got < 4 {
		t.Fatalf("fileComplexity() = %d, want at least 4", got)
	}
}

func TestAnalyzeRanksChurnTimesComplexity(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test")
	writeFile(t, root, "src/Low.kt", "fun low() = 1\n")
	writeFile(t, root, "src/High.kt", "fun high(x: Int) { if (x > 0) println(x) }\n")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-q", "-m", "initial")
	writeFile(t, root, "src/High.kt", "fun high(x: Int) { if (x > 0 && x < 10) println(x) }\n")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-q", "-m", "touch high")

	report, err := Analyze(root, "1970-01-01")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Files) == 0 {
		t.Fatal("expected risk entries")
	}
	if report.Files[0].File != "src/High.kt" {
		t.Fatalf("top risk = %+v, want High.kt first", report.Files)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
