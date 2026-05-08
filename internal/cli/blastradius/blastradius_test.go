package blastradius

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAnalyzeReportsConsumersForChangedSymbol(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test")
	writeFile(t, root, "Provider.kt", "package test\n\nfun shared() = 1\n")
	writeFile(t, root, "Consumer.kt", "package test\n\nfun use() = shared()\n")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-q", "-m", "base")
	writeFile(t, root, "Provider.kt", "package test\n\nfun shared() = 2\n")

	report, err := Analyze(root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if report.ChangedFiles != 1 {
		t.Fatalf("ChangedFiles = %d, want 1", report.ChangedFiles)
	}
	if len(report.TopFanIn) == 0 {
		t.Fatalf("expected fan-in entries: %+v", report)
	}
	if report.TopFanIn[0].ConsumerFiles == 0 {
		t.Fatalf("expected consumers for changed symbol: %+v", report.TopFanIn[0])
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
