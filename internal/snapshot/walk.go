package snapshot

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// commitMeta is one commit's identity + committer time. Returned in
// newest-first order by listCommits.
type commitMeta struct {
	SHA         string
	CommittedAt int64
}

// listCommits walks `git log` in repoRoot. branch / sinceSeconds /
// maxCount are optional filters; passing all-zero matches "everything
// reachable from HEAD". Each entry carries the committer-time so
// callers don't need a follow-up git log per sha.
func listCommits(repoRoot, branch string, sinceSeconds int64, maxCount int) ([]commitMeta, error) {
	args := []string{"log", "--format=%H %ct"}
	if sinceSeconds > 0 {
		args = append(args, "--since="+strconv.FormatInt(sinceSeconds, 10)+".seconds")
	}
	if maxCount > 0 {
		args = append(args, "--max-count="+strconv.Itoa(maxCount))
	}
	if branch != "" {
		args = append(args, branch)
	}
	out, err := runGitLine(repoRoot, args...)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	lines := strings.Split(out, "\n")
	commits := make([]commitMeta, 0, len(lines))
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 || len(fields[0]) < 7 {
			continue
		}
		ts, _ := strconv.ParseInt(fields[1], 10, 64)
		commits = append(commits, commitMeta{SHA: fields[0], CommittedAt: ts})
	}
	return commits, nil
}

// worktreeWalk runs perCommit against each commit in commits using a
// pool of workers. Each worker manages its own detached worktree under
// scratchDir; gitMu serialises `git worktree add/remove` against the
// primary repo's `.git/worktrees/` registry, where concurrent
// mutations race even though git's per-worktree locks are
// sequential-safe.
//
// perCommit returns nil for success or an error. Both outcomes are
// reported through onResult so the caller drives accounting (event
// emission, success/fail counts, captured/skipped semantics).
type worktreeJob struct {
	WorktreePath string
	SHA          string
	WorkerID     int
}

func worktreeWalk(repoRoot, scratchDir string, workers int, commits []string, perCommit func(worktreeJob) error, onResult func(sha string, err error)) {
	var gitMu sync.Mutex
	jobs := make(chan int)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(workerID int) {
			defer wg.Done()
			for idx := range jobs {
				sha := commits[idx]
				err := withWorktree(repoRoot, sha, scratchDir, workerID, &gitMu, func(path string) error {
					return perCommit(worktreeJob{WorktreePath: path, SHA: sha, WorkerID: workerID})
				})
				if onResult != nil {
					onResult(sha, err)
				}
			}
		}(i)
	}
	for i := range commits {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
}

// withWorktree adds a detached worktree at sha, runs fn against its
// path, and removes the worktree, all under gitMu for the git ops. The
// per-worker subdir keys on workerID + short-sha so two workers can
// never collide on path.
func withWorktree(repoRoot, sha, scratchDir string, workerID int, gitMu *sync.Mutex, fn func(path string) error) error {
	worktreePath := filepath.Join(scratchDir, fmt.Sprintf("w%d-%s", workerID, sha[:12]))

	gitMu.Lock()
	addErr := gitWorktreeAdd(repoRoot, worktreePath, sha)
	gitMu.Unlock()
	if addErr != nil {
		return addErr
	}
	defer func() {
		gitMu.Lock()
		gitWorktreeRemove(repoRoot, worktreePath)
		gitMu.Unlock()
	}()
	return fn(worktreePath)
}

func gitWorktreeAdd(repoRoot, worktreePath, sha string) error {
	cmd := exec.CommandContext(context.Background(), "git", "worktree", "add", "--detach", "--force", worktreePath, sha)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("git worktree add %s: %s", sha, msg)
	}
	return nil
}

func gitWorktreeRemove(repoRoot, worktreePath string) {
	// Force so the directory disappears even when the previous run wrote
	// transient artifacts (.krit/ inside the worktree before snapshots
	// were rerouted to the primary repo, etc.).
	cmd := exec.CommandContext(context.Background(), "git", "worktree", "remove", "--force", worktreePath)
	cmd.Dir = repoRoot
	_ = cmd.Run()
	_ = os.RemoveAll(worktreePath)
}
