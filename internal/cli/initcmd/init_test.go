package initcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/onboarding"
)

// TestRunHeadlessInitInProcess calls runHeadlessInit directly
// instead of via exec, so Go coverage can see everything under it.
// This complements TestInitSubcommandHeadless which exec's the
// binary and therefore doesn't contribute to in-process coverage.
func TestRunHeadlessInitInProcess(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(repoRoot, "playground", "kotlin-webservice")
	if _, err := os.Stat(src); err != nil {
		t.Skipf("playground missing: %v", err)
	}

	target := t.TempDir()
	copyDir(t, src, target)
	_ = os.Remove(filepath.Join(target, "krit.yml"))
	_ = os.RemoveAll(filepath.Join(target, ".krit"))

	reg, err := onboarding.LoadRegistry(filepath.Join(repoRoot, "config", "onboarding", "controversial-rules.json"))
	if err != nil {
		t.Fatal(err)
	}

	code := runHeadlessInit(onboarding.ScanOptions{
		KritBin:  binPath,
		RepoRoot: repoRoot,
		Target:   target,
	}, reg, "balanced")

	if code != 0 {
		t.Errorf("runHeadlessInit exit code = %d, want 0", code)
	}
	if _, err := os.Stat(filepath.Join(target, "krit.yml")); err != nil {
		t.Errorf("krit.yml missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, ".krit", "baseline.xml")); err != nil {
		t.Errorf("baseline.xml missing: %v", err)
	}
}

// TestRunHeadlessInitUnknownProfile exercises the validation branch.
func TestRunHeadlessInitUnknownProfile(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	reg, err := onboarding.LoadRegistry(filepath.Join(repoRoot, "config", "onboarding", "controversial-rules.json"))
	if err != nil {
		t.Fatal(err)
	}

	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() { os.Stderr = origStderr }()

	code := runHeadlessInit(onboarding.ScanOptions{
		KritBin:  binPath,
		RepoRoot: repoRoot,
		Target:   t.TempDir(),
	}, reg, "bogus")

	w.Close()
	os.Stderr = origStderr
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	msg := string(buf[:n])

	if code != 2 {
		t.Errorf("runHeadlessInit exit code = %d, want 2", code)
	}
	if !strings.Contains(msg, "unknown profile") {
		t.Errorf("stderr missing 'unknown profile'; got: %q", msg)
	}
}

// TestFindOnboardingRepoRootEnv verifies the KRIT_REPO_ROOT override
// takes precedence, then falls through to directory walking.
func TestFindOnboardingRepoRootEnv(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("KRIT_REPO_ROOT", repoRoot)
	got, err := findOnboardingRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	if got != repoRoot {
		t.Errorf("findOnboardingRepoRoot = %q, want %q", got, repoRoot)
	}
}

func TestFindOnboardingRepoRootBogusEnvFallsThrough(t *testing.T) {
	t.Setenv("KRIT_REPO_ROOT", "/nonexistent/path/to/nothing")
	got, err := findOnboardingRepoRoot()
	if err != nil {
		t.Fatalf("expected fallback resolution, got error: %v", err)
	}
	if got == "/nonexistent/path/to/nothing" {
		t.Errorf("findOnboardingRepoRoot returned the bogus env value without validating it")
	}
}

// TestRunInitSubcommandHelp covers runInitSubcommand's help branch.
func TestRunInitSubcommandHelp(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("KRIT_REPO_ROOT", repoRoot)
	t.Setenv("KRIT_BIN", binPath)

	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() { os.Stderr = origStderr }()

	code := Run([]string{"--help"})

	w.Close()
	os.Stderr = origStderr
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	msg := string(buf[:n])

	if code != 0 {
		t.Errorf("runInitSubcommand(--help) code = %d, want 0", code)
	}
	if !strings.Contains(msg, "Usage: krit init") {
		t.Errorf("help output missing usage line; got: %q", msg)
	}
}

// TestRunInitSubcommandMissingTarget exercises the "target not a
// directory" error branch.
func TestRunInitSubcommandMissingTarget(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("KRIT_REPO_ROOT", repoRoot)
	t.Setenv("KRIT_BIN", binPath)

	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() { os.Stderr = origStderr }()

	code := Run([]string{"/definitely/does/not/exist"})

	w.Close()
	os.Stderr = origStderr
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	msg := string(buf[:n])

	if code != 2 {
		t.Errorf("runInitSubcommand with missing target code = %d, want 2", code)
	}
	if !strings.Contains(msg, "is not a directory") {
		t.Errorf("error message missing expected text; got: %q", msg)
	}
}

// TestRunInitSubcommandHeadlessDelegate confirms that passing
// --profile + --yes routes through runHeadlessInit without ever
// constructing a bubbletea program.
func TestRunInitSubcommandHeadlessDelegate(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(repoRoot, "playground", "kotlin-webservice")
	if _, err := os.Stat(src); err != nil {
		t.Skipf("playground missing: %v", err)
	}

	target := t.TempDir()
	copyDir(t, src, target)
	_ = os.Remove(filepath.Join(target, "krit.yml"))
	_ = os.RemoveAll(filepath.Join(target, ".krit"))

	t.Setenv("KRIT_REPO_ROOT", repoRoot)
	t.Setenv("KRIT_BIN", binPath)

	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	code := Run([]string{"--profile", "balanced", "--yes", target})

	w.Close()
	os.Stdout = origStdout
	buf := make([]byte, 2048)
	n, _ := r.Read(buf)
	msg := string(buf[:n])

	if code != 0 {
		t.Errorf("runInitSubcommand delegate code = %d, want 0; stdout: %s", code, msg)
	}
	if !strings.Contains(msg, "wrote ") || !strings.Contains(msg, "baseline written to ") {
		t.Errorf("stdout missing expected summary lines; got: %q", msg)
	}
}

// TestResolveKritBinEnv covers the KRIT_BIN env resolution path.
func TestResolveKritBinEnv(t *testing.T) {
	t.Setenv("KRIT_BIN", binPath)
	got, err := resolveKritBin()
	if err != nil {
		t.Fatal(err)
	}
	if got != binPath {
		t.Errorf("resolveKritBin = %q, want %q", got, binPath)
	}
}
