package snapshot

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// BackfillOptions controls a Backfill invocation. Snapshots land in
// the primary repo's `.krit/snapshots/`; commits are checked out into
// temporary worktrees that are torn down after capture.
type BackfillOptions struct {
	RepoRoot    string
	Branch      string
	Since       time.Duration
	MaxCommits  int
	Workers     int
	Force       bool
	KritVersion string
	Reporter    func(BackfillEvent)
}

// BackfillEvent is one commit's outcome. Kind is "captured", "skipped",
// or "failed".
type BackfillEvent struct {
	Kind      string
	CommitSHA string
	Error     error
	Duration  time.Duration
}

type BackfillResult struct {
	Captured int
	Skipped  int
	Failed   int
}

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

	commits, err := listCommits(root, opts.Branch, int64(opts.Since/time.Second), opts.MaxCommits)
	if err != nil {
		return BackfillResult{}, err
	}

	allShas := make([]string, len(commits))
	for i, c := range commits {
		allShas[i] = c.SHA
	}
	pending := allShas
	if !opts.Force {
		pending = filterUncaptured(snapshotsRoot, allShas)
	}

	var captured, skipped, failed atomic.Int64
	skipped.Add(int64(len(allShas) - len(pending)))
	if opts.Reporter != nil {
		pendingSet := make(map[string]struct{}, len(pending))
		for _, sha := range pending {
			pendingSet[sha] = struct{}{}
		}
		for _, sha := range allShas {
			if _, ok := pendingSet[sha]; !ok {
				opts.Reporter(BackfillEvent{Kind: "skipped", CommitSHA: sha})
			}
		}
	}

	if len(pending) == 0 {
		return BackfillResult{Skipped: int(skipped.Load())}, nil
	}

	scratch, err := os.MkdirTemp("", "krit-backfill-*")
	if err != nil {
		return BackfillResult{}, fmt.Errorf("snapshot: scratch dir: %w", err)
	}
	defer os.RemoveAll(scratch)

	starts := make(map[string]time.Time, len(pending))
	var startsMu sync.Mutex
	worktreeWalk(root, scratch, workers, pending,
		func(job worktreeJob) error {
			startsMu.Lock()
			starts[job.SHA] = time.Now()
			startsMu.Unlock()
			res, err := Capture(CaptureOptions{
				RepoRoot:    job.WorktreePath,
				CommitSHA:   job.SHA,
				KritVersion: opts.KritVersion,
			})
			if err != nil {
				return err
			}
			_, err = SaveResult(snapshotsRoot, res, root, opts.KritVersion)
			return err
		},
		func(sha string, err error) {
			startsMu.Lock()
			start := starts[sha]
			delete(starts, sha)
			startsMu.Unlock()
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
		},
	)

	return BackfillResult{
		Captured: int(captured.Load()),
		Skipped:  int(skipped.Load()),
		Failed:   int(failed.Load()),
	}, nil
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
