package snapshot

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestSimulateUsesSidecarFastPath confirms that when every walked commit
// has a persisted findings sidecar, Simulate returns from the in-process
// Timeline read and never has to spin up a worktree. We assert that by
// supplying a bogus KritBin path — if the shell-out fallback fired, it
// would error.
func TestSimulateUsesSidecarFastPath(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo := initSimulateRepo(t)

	// Persist sidecars for every commit in history so the sidecar fast
	// path covers the full walk.
	shas := listAllSHAs(t, repo)
	if len(shas) < 3 {
		t.Fatalf("expected 3 commits in fixture repo, got %d", len(shas))
	}
	root := SnapshotsDir(repo)
	for i, sha := range shas {
		blob := &Blob{SchemaVersion: SchemaVersion, CommitSHA: sha, CapturedAt: int64(i + 1)}
		if _, err := Save(root, blob); err != nil {
			t.Fatalf("save blob: %v", err)
		}
		f := &Findings{
			SchemaVersion: FindingsSchemaVersion,
			CommitSHA:     sha,
			RuleSetHash:   "test-ruleset",
			ByRule:        map[string]int{"MagicNumber": (i + 1) * 5},
		}
		if _, err := SaveFindings(root, f); err != nil {
			t.Fatalf("save findings: %v", err)
		}
	}

	res, err := Simulate(SimulateOptions{
		RepoRoot: repo,
		Rule:     "MagicNumber",
		Workers:  2,
		// Intentionally invalid: the shell-out path would fail to invoke
		// this, surfacing as a non-empty Failed list. A clean run proves
		// the sidecar fast path covered every commit.
		KritBin: filepath.Join(repo, "does-not-exist"),
	})
	if err != nil {
		t.Fatalf("Simulate: %v", err)
	}
	if len(res.Failed) != 0 {
		t.Fatalf("expected no failed commits with sidecar fast path, got %v", res.Failed)
	}
	if len(res.Points) != len(shas) {
		t.Fatalf("expected %d points, got %d", len(shas), len(res.Points))
	}
	for _, p := range res.Points {
		if p.Source != SimulateSourceSidecar {
			t.Fatalf("expected source=sidecar, got %q for %s", p.Source, p.CommitSHA)
		}
		if p.Findings == 0 {
			t.Fatalf("expected nonzero findings count for %s", p.CommitSHA)
		}
	}
	// Newest-first ordering still holds.
	for i := 1; i < len(res.Points); i++ {
		if res.Points[i-1].CommittedAt < res.Points[i].CommittedAt {
			t.Fatalf("points not sorted newest-first: %+v", res.Points)
		}
	}
}

// TestSimulateNoSidecarHonored ensures the NoSidecar opt-out bypasses
// persisted findings even when they exist on disk. Useful for callers
// that want to validate the shell-out path keeps working.
func TestSimulateNoSidecarHonored(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo := initSimulateRepo(t)
	shas := listAllSHAs(t, repo)
	root := SnapshotsDir(repo)
	for _, sha := range shas {
		f := &Findings{
			SchemaVersion: FindingsSchemaVersion,
			CommitSHA:     sha,
			RuleSetHash:   "test-ruleset",
			ByRule:        map[string]int{"MagicNumber": 999},
		}
		if _, err := SaveFindings(root, f); err != nil {
			t.Fatalf("save findings: %v", err)
		}
	}

	kritBin := buildKritForTest(t)
	res, err := Simulate(SimulateOptions{
		RepoRoot:  repo,
		Rule:      "MagicNumber",
		Workers:   2,
		KritBin:   kritBin,
		NoSidecar: true,
	})
	if err != nil {
		t.Fatalf("Simulate: %v", err)
	}
	for _, p := range res.Points {
		if p.Source == SimulateSourceSidecar {
			t.Fatalf("expected NoSidecar to bypass sidecar, got %+v", p)
		}
		if p.Findings == 999 {
			t.Fatalf("got stub-sidecar count 999 despite NoSidecar: %+v", p)
		}
	}
}

func listAllSHAs(t *testing.T, repo string) []string {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "git", "log", "--format=%H")
	cmd.Dir = repo
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	return strings.Fields(string(out))
}
