package snapshot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// BackfillOptions controls a Backfill invocation. RepoRoot is the
// primary repo whose `.krit/snapshots/` receives every captured blob;
// individual commits are checked out into temporary git worktrees that
// are cleaned up after capture.
type BackfillOptions struct {
	RepoRoot    string
	Branch      string        // "" = current branch / HEAD
	Since       time.Duration // 0 = no time filter
	MaxCommits  int           // 0 = unlimited
	Workers     int           // <=0 = runtime.NumCPU()
	Force       bool          // recapture even if a snapshot already exists
	KritVersion string
	Reporter    func(BackfillEvent)
}

// BackfillEvent is emitted for each commit handled. Kind is one of
// "captured", "skipped", or "failed".
type BackfillEvent struct {
	Kind      string
	CommitSHA string
	Error     error
	Duration  time.Duration
}

// BackfillResult summarises outcomes across the whole walk.
type BackfillResult struct {
	Captured int
	Skipped  int
	Failed   int
}

// Backfill captures snapshots for every commit reachable from
// opts.Branch (or HEAD) within opts.Since. Already-captured shas are
// skipped unless Force is set.
func Backfill(opts BackfillOptions) (BackfillResult, error) {
	if opts.RepoRoot == "" {
		return BackfillResult{}, errors.New("snapshot: BackfillOptions.RepoRoot required")
	}
	root, err := filepath.Abs(opts.RepoRoot)
	if err != nil {
		return BackfillResult{}, fmt.Errorf("snapshot: abs RepoRoot: %w", err)
	}
	workers := opts.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	snapshotsRoot := SnapshotsDir(root)

	shas, err := listBackfillShas(root, opts.Branch, opts.Since, opts.MaxCommits)
	if err != nil {
		return BackfillResult{}, err
	}

	pending := shas
	if !opts.Force {
		pending = filterUncaptured(snapshotsRoot, shas)
	}

	scratch, err := os.MkdirTemp("", "krit-backfill-*")
	if err != nil {
		return BackfillResult{}, fmt.Errorf("snapshot: scratch dir: %w", err)
	}
	defer os.RemoveAll(scratch)

	var captured, skipped, failed atomic.Int64
	skipped.Add(int64(len(shas) - len(pending)))
	if opts.Reporter != nil {
		for _, sha := range shas {
			if !contains(pending, sha) {
				opts.Reporter(BackfillEvent{Kind: "skipped", CommitSHA: sha})
			}
		}
	}

	// gitMu serialises `git worktree add/remove` invocations against the
	// same primary repo. Git's loose locking around `.git/worktrees/` is
	// fine for sequential callers but races under concurrent registry
	// mutation; the parse/index work that dominates capture time stays
	// fully parallel.
	var gitMu sync.Mutex

	jobs := make(chan string)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(workerID int) {
			defer wg.Done()
			for sha := range jobs {
				start := time.Now()
				err := captureSha(root, sha, scratch, workerID, snapshotsRoot, opts.KritVersion, &gitMu)
				ev := BackfillEvent{CommitSHA: sha, Duration: time.Since(start)}
				if err != nil {
					ev.Kind = "failed"
					ev.Error = err
					failed.Add(1)
				} else {
					ev.Kind = "captured"
					captured.Add(1)
				}
				if opts.Reporter != nil {
					opts.Reporter(ev)
				}
			}
		}(i)
	}
	for _, sha := range pending {
		jobs <- sha
	}
	close(jobs)
	wg.Wait()

	return BackfillResult{
		Captured: int(captured.Load()),
		Skipped:  int(skipped.Load()),
		Failed:   int(failed.Load()),
	}, nil
}

// listBackfillShas returns commit shas reachable from branch (or HEAD)
// within the optional time and count limits. Order is newest-first to
// match `git log`.
func listBackfillShas(repoRoot, branch string, since time.Duration, maxCommits int) ([]string, error) {
	args := []string{"log", "--format=%H"}
	if since > 0 {
		args = append(args, "--since="+strconv.FormatInt(int64(since/time.Second), 10)+".seconds")
	}
	if maxCommits > 0 {
		args = append(args, "--max-count="+strconv.Itoa(maxCommits))
	}
	if branch != "" {
		args = append(args, branch)
	}
	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Dir = repoRoot
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("git log: %s", msg)
	}
	var shas []string
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		line = strings.TrimSpace(line)
		if len(line) >= 7 {
			shas = append(shas, line)
		}
	}
	return shas, nil
}

func filterUncaptured(snapshotsRoot string, shas []string) []string {
	out := make([]string, 0, len(shas))
	for _, sha := range shas {
		path, err := BlobPath(snapshotsRoot, sha)
		if err != nil {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			continue
		}
		out = append(out, sha)
	}
	return out
}

func contains(s []string, target string) bool {
	for _, v := range s {
		if v == target {
			return true
		}
	}
	return false
}

// captureSha checks sha out into a fresh git worktree under scratchDir,
// runs Capture against it, persists the result back to snapshotsRoot,
// and removes the worktree. Each worker uses a distinct subdirectory so
// parallel scans never collide on path; gitMu serialises the
// add/remove against the primary repo's `.git/worktrees/` registry.
func captureSha(primaryRepoRoot, sha, scratchDir string, workerID int, snapshotsRoot, kritVersion string, gitMu *sync.Mutex) error {
	worktreePath := filepath.Join(scratchDir, fmt.Sprintf("w%d-%s", workerID, sha[:12]))

	gitMu.Lock()
	addErr := gitWorktreeAdd(primaryRepoRoot, worktreePath, sha)
	gitMu.Unlock()
	if addErr != nil {
		return addErr
	}
	defer func() {
		gitMu.Lock()
		gitWorktreeRemove(primaryRepoRoot, worktreePath)
		gitMu.Unlock()
	}()

	res, err := Capture(CaptureOptions{
		RepoRoot:    worktreePath,
		CommitSHA:   sha,
		KritVersion: kritVersion,
	})
	if err != nil {
		return err
	}
	if _, err := SaveResult(snapshotsRoot, res, primaryRepoRoot, kritVersion); err != nil {
		return err
	}
	return nil
}

func gitWorktreeAdd(repoRoot, worktreePath, sha string) error {
	cmd := exec.CommandContext(context.Background(), "git", "worktree", "add", "--detach", "--force", worktreePath, sha)
	cmd.Dir = repoRoot
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("git worktree add %s: %s", sha, msg)
	}
	return nil
}

func gitWorktreeRemove(repoRoot, worktreePath string) {
	// Force-remove so the worktree directory disappears even with
	// uncommitted artifacts (capture may have written to .krit/ inside
	// the worktree before we routed snapshots elsewhere).
	cmd := exec.CommandContext(context.Background(), "git", "worktree", "remove", "--force", worktreePath)
	cmd.Dir = repoRoot
	_ = cmd.Run()
	// Belt-and-suspenders: if git left the directory behind, drop it.
	_ = os.RemoveAll(worktreePath)
}
