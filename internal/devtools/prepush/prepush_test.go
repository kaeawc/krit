package prepush

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func requireBashAndGit(t *testing.T) {
	t.Helper()
	for _, tool := range []string{"bash", "git"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not on PATH", tool)
		}
	}
}

func trimLines(s string) []string {
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// TestPlanFixtures runs `scripts/pre-push.sh --dry-run` on each
// testdata/plans/*.txt changed-file list and compares the planned checks.
// Each fixture is `# comment` lines, changed files, `---`, expected checks.
func TestPlanFixtures(t *testing.T) {
	requireBashAndGit(t)
	root := repoRoot(t)
	fixtures, err := filepath.Glob(filepath.Join("testdata", "plans", "*.txt"))
	if err != nil || len(fixtures) == 0 {
		t.Fatalf("no plan fixtures: %v", err)
	}
	for _, fixture := range fixtures {
		t.Run(strings.TrimSuffix(filepath.Base(fixture), ".txt"), func(t *testing.T) {
			data, err := os.ReadFile(fixture)
			if err != nil {
				t.Fatal(err)
			}
			files, want, ok := strings.Cut(string(data), "---\n")
			if !ok {
				t.Fatalf("%s: missing --- separator", fixture)
			}
			var changed []string
			for _, line := range trimLines(files) {
				if !strings.HasPrefix(line, "#") {
					changed = append(changed, line)
				}
			}
			cmd := exec.Command("bash", filepath.Join(root, "scripts", "pre-push.sh"), "--dry-run")
			cmd.Dir = root
			cmd.Env = append(os.Environ(),
				"KRIT_PREPUSH_FILES="+strings.Join(changed, "\n"),
				// winvet depends on a mingw toolchain on PATH; keep plans host-independent.
				"KRIT_PREPUSH_SKIP=winvet",
			)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("pre-push.sh --dry-run: %v\n%s", err, out)
			}
			if got, want := strings.Join(trimLines(string(out)), "\n"), strings.Join(trimLines(want), "\n"); got != want {
				t.Errorf("plan mismatch\n--- got\n%s\n--- want\n%s", got, want)
			}
		})
	}
}

func TestPlanHonorsSkip(t *testing.T) {
	requireBashAndGit(t)
	root := repoRoot(t)
	cmd := exec.Command("bash", filepath.Join(root, "scripts", "pre-push.sh"), "--dry-run")
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"KRIT_PREPUSH_FILES=internal/oracle/daemon_jvm.go",
		"KRIT_PREPUSH_SKIP=winvet,lint,test,merge",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if got, want := strings.Join(trimLines(string(out)), " "), "gofmt build vet oraclefp warmclean"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// mergeRepo builds a throwaway clone whose origin/main renamed Greet to
// Hello, so a branch that adds a new Greet call builds alone but not merged.
// It returns the clone's path.
func mergeRepo(t *testing.T, root string, branchEdit func(dir string)) string {
	t.Helper()
	tmp := t.TempDir()
	origin := filepath.Join(tmp, "origin.git")
	work := filepath.Join(tmp, "work")
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@localhost",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@localhost",
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	script, err := os.ReadFile(filepath.Join(root, "scripts", "pre-push.sh"))
	if err != nil {
		t.Fatal(err)
	}

	git(tmp, "init", "-q", "--bare", "-b", "main", origin)
	git(tmp, "clone", "-q", origin, work)
	write(filepath.Join(work, "scripts", "pre-push.sh"), string(script))
	write(filepath.Join(work, "go.mod"), "module example.com/m\n\ngo 1.21\n")
	write(filepath.Join(work, "greet.go"), "package m\n\nfunc Greet() string { return \"hi\" }\n")
	git(work, "add", ".")
	git(work, "commit", "-q", "-m", "base")
	git(work, "push", "-q", "origin", "HEAD:main")

	// origin/main moves on: Greet becomes Hello.
	write(filepath.Join(work, "greet.go"), "package m\n\nfunc Hello() string { return \"hi\" }\n")
	git(work, "commit", "-q", "-am", "rename")
	git(work, "push", "-q", "origin", "HEAD:main")
	git(work, "reset", "-q", "--hard", "HEAD~1")

	git(work, "checkout", "-q", "-b", "feature")
	branchEdit(work)
	git(work, "add", ".")
	git(work, "commit", "-q", "-m", "feature")
	git(work, "fetch", "-q", "origin")
	return work
}

func runMergeOnly(t *testing.T, dir string) (string, error) {
	t.Helper()
	cmd := exec.Command("bash", "scripts/pre-push.sh")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"KRIT_PREPUSH_FETCH=0",
		"KRIT_PREPUSH_SKIP=gofmt,build,vet,lint,rules,oraclefp,schema,gomod,test,actions,docs,winvet",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		// Hooks can run with GIT_DIR exported; the merge must still happen
		// in the throwaway worktree, not in this checkout.
		"GIT_DIR="+filepath.Join(dir, ".git"),
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestMergeCheckCatchesSemanticConflict(t *testing.T) {
	requireBashAndGit(t)
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH")
	}
	root := repoRoot(t)
	work := mergeRepo(t, root, func(dir string) {
		if err := os.WriteFile(filepath.Join(dir, "use.go"),
			[]byte("package m\n\nfunc Use() string { return Greet() }\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	})
	out, err := runMergeOnly(t, work)
	if err == nil {
		t.Fatalf("merge check passed a branch that does not build on origin/main:\n%s", out)
	}
	if !strings.Contains(out, "not merged with origin/main") {
		t.Errorf("missing semantic-conflict message:\n%s", out)
	}
	assertCheckoutUntouched(t, work)
}

func TestMergeCheckCatchesTextConflict(t *testing.T) {
	requireBashAndGit(t)
	root := repoRoot(t)
	work := mergeRepo(t, root, func(dir string) {
		if err := os.WriteFile(filepath.Join(dir, "greet.go"),
			[]byte("package m\n\nfunc Greet() string { return \"hello\" }\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	})
	out, err := runMergeOnly(t, work)
	if err == nil || !strings.Contains(out, "does not merge cleanly") {
		t.Fatalf("want a merge-conflict failure, got err=%v:\n%s", err, out)
	}
	assertCheckoutUntouched(t, work)
}

func TestMergeCheckPassesCleanMerge(t *testing.T) {
	requireBashAndGit(t)
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH")
	}
	root := repoRoot(t)
	work := mergeRepo(t, root, func(dir string) {
		if err := os.WriteFile(filepath.Join(dir, "other.go"),
			[]byte("package m\n\nfunc Other() int { return 1 }\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	})
	if out, err := runMergeOnly(t, work); err != nil {
		t.Fatalf("clean merge failed: %v\n%s", err, out)
	}
	assertCheckoutUntouched(t, work)
}

// assertCheckoutUntouched checks that the merge check left the developer's
// checkout on its branch, with no merge in progress and no extra worktrees.
func assertCheckoutUntouched(t *testing.T, dir string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(dir, ".git", "MERGE_HEAD")); err == nil {
		t.Error("merge check left a merge in progress in the checkout")
	}
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(out), "worktree "); n != 1 {
		t.Errorf("merge check left %d worktrees:\n%s", n, out)
	}
	cmd = exec.Command("git", "symbolic-ref", "--short", "HEAD")
	cmd.Dir = dir
	if out, err := cmd.Output(); err != nil || strings.TrimSpace(string(out)) != "feature" {
		t.Errorf("checkout moved off its branch: %q %v", out, err)
	}
}
