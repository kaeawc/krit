package snapshot

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestBackfillCapturesAllCommits(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo := initTinyRepo(t)

	res, err := Backfill(BackfillOptions{
		RepoRoot:    repo,
		Workers:     2,
		KritVersion: "test",
	})
	if err != nil {
		t.Fatalf("Backfill: %v", err)
	}
	if res.Captured != 3 || res.Failed != 0 {
		t.Fatalf("expected 3 captured / 0 failed, got %+v", res)
	}

	root := SnapshotsDir(repo)
	entries, err := List(root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 captured entries, got %d", len(entries))
	}
	for _, e := range entries {
		m, err := LoadManifest(root, e.CommitSHA)
		if err != nil {
			t.Fatalf("LoadManifest %s: %v", e.CommitSHA, err)
		}
		if m.KritVersion != "test" || m.Files < 1 {
			t.Fatalf("manifest mismatch for %s: %+v", e.CommitSHA, m)
		}
	}
}

func TestBackfillResumesAfterPriorCapture(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo := initTinyRepo(t)

	first, err := Backfill(BackfillOptions{
		RepoRoot:    repo,
		Workers:     1,
		MaxCommits:  1,
		KritVersion: "test",
	})
	if err != nil {
		t.Fatalf("first Backfill: %v", err)
	}
	if first.Captured != 1 {
		t.Fatalf("expected 1 captured first time, got %+v", first)
	}

	second, err := Backfill(BackfillOptions{
		RepoRoot:    repo,
		Workers:     1,
		KritVersion: "test",
	})
	if err != nil {
		t.Fatalf("second Backfill: %v", err)
	}
	if second.Captured != 2 || second.Skipped != 1 {
		t.Fatalf("expected 2 captured / 1 skipped on resume, got %+v", second)
	}
}

func TestBackfillReporterFiresPerEvent(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo := initTinyRepo(t)

	var mu sync.Mutex
	kinds := make(map[string]int)
	_, err := Backfill(BackfillOptions{
		RepoRoot:    repo,
		Workers:     1,
		KritVersion: "test",
		Reporter: func(ev BackfillEvent) {
			mu.Lock()
			defer mu.Unlock()
			kinds[ev.Kind]++
		},
	})
	if err != nil {
		t.Fatalf("Backfill: %v", err)
	}
	if kinds["captured"] != 3 {
		t.Fatalf("expected 3 captured events, got %+v", kinds)
	}
}

// initTinyRepo creates a temp directory with a 3-commit git history,
// each commit adding one Kotlin file. Returns the repo root.
func initTinyRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()

	run := func(args ...string) {
		cmd := exec.CommandContext(context.Background(), "git", args...)
		cmd.Dir = repo
		// Hermetic config so the test is independent of the developer's
		// global git config.
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=krit-test",
			"GIT_AUTHOR_EMAIL=krit@test",
			"GIT_COMMITTER_NAME=krit-test",
			"GIT_COMMITTER_EMAIL=krit@test",
		)
		var errBuf strings.Builder
		cmd.Stderr = stringWriter{&errBuf}
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, errBuf.String())
		}
	}

	run("init", "-q", "-b", "main")
	run("config", "commit.gpgsign", "false")

	srcDir := filepath.Join(repo, "src", "main", "kotlin")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for i, content := range []string{
		"package demo\n\nfun one() = 1\n",
		"package demo\n\nfun one() = 1\nfun two() = 2\n",
		"package demo\n\nfun one() = 1\nfun two() = 2\nfun three() = 3\n",
	} {
		path := filepath.Join(srcDir, "Greet.kt")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		run("add", "src/main/kotlin/Greet.kt")
		run("commit", "-q", "-m", "step "+string(rune('0'+i+1)))
	}
	return repo
}

type stringWriter struct{ b *strings.Builder }

func (s stringWriter) Write(p []byte) (int, error) { return s.b.Write(p) }
