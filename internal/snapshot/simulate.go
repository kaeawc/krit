package snapshot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// SimulateOptions controls a `backfill_simulate` walk: what rule to
// score, which history to walk, how to invoke krit, etc.
type SimulateOptions struct {
	RepoRoot   string
	Rule       string
	Branch     string
	Since      time.Duration
	MaxCommits int
	Workers    int
	// KritBin is the krit executable to invoke against each captured
	// worktree. Empty defaults to argv[0]; tests inject a path.
	KritBin  string
	Reporter func(SimulateEvent)
}

// SimulateEvent fires once per commit handled.
type SimulateEvent struct {
	CommitSHA string
	Findings  int
	Duration  time.Duration
	Error     error
}

// SimulatePoint is one (commit, count) reading. Sorted by capture time
// (newest-first to match `git log` order).
type SimulatePoint struct {
	CommitSHA   string `json:"commit_sha"`
	CommittedAt int64  `json:"committed_at"`
	Findings    int    `json:"findings"`
}

// SimulateResult is the time series of per-commit finding counts
// produced by Simulate.
type SimulateResult struct {
	Rule   string          `json:"rule"`
	Points []SimulatePoint `json:"points"`
	Failed []string        `json:"failed,omitempty"`
}

// Simulate answers "if rule X had been active for the last <since>,
// how many findings would each commit have carried?" by walking
// history with detached worktrees and shelling out to krit -f json
// against each one. Slow per call (one full analyse per commit) but
// the use case is one-off rule tuning, not a hot path.
func Simulate(opts SimulateOptions) (*SimulateResult, error) {
	if opts.RepoRoot == "" {
		return nil, errors.New("snapshot: SimulateOptions.RepoRoot required")
	}
	if opts.Rule == "" {
		return nil, errors.New("snapshot: SimulateOptions.Rule required")
	}
	root, err := filepath.Abs(opts.RepoRoot)
	if err != nil {
		return nil, fmt.Errorf("snapshot: abs RepoRoot: %w", err)
	}
	workers := opts.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	kritBin := opts.KritBin
	if kritBin == "" {
		kritBin, err = resolveKritBin()
		if err != nil {
			return nil, err
		}
	}

	shas, err := listBackfillShas(root, opts.Branch, opts.Since, opts.MaxCommits)
	if err != nil {
		return nil, err
	}
	if len(shas) == 0 {
		return &SimulateResult{Rule: opts.Rule}, nil
	}

	scratch, err := os.MkdirTemp("", "krit-simulate-*")
	if err != nil {
		return nil, fmt.Errorf("snapshot: scratch dir: %w", err)
	}
	defer os.RemoveAll(scratch)

	var gitMu sync.Mutex
	var failedMu sync.Mutex
	var failed []string
	points := make([]SimulatePoint, len(shas))
	var done atomic.Int64

	jobs := make(chan int)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(workerID int) {
			defer wg.Done()
			for idx := range jobs {
				sha := shas[idx]
				start := time.Now()
				findings, committedAt, err := simulateSha(root, sha, kritBin, opts.Rule, scratch, workerID, &gitMu)
				ev := SimulateEvent{CommitSHA: sha, Findings: findings, Duration: time.Since(start), Error: err}
				if err != nil {
					failedMu.Lock()
					failed = append(failed, sha)
					failedMu.Unlock()
				} else {
					points[idx] = SimulatePoint{CommitSHA: sha, CommittedAt: committedAt, Findings: findings}
				}
				done.Add(1)
				if opts.Reporter != nil {
					opts.Reporter(ev)
				}
			}
		}(i)
	}
	for i := range shas {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	// Drop placeholder points for failed shas and sort newest-first.
	out := points[:0]
	for _, p := range points {
		if p.CommitSHA != "" {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CommittedAt > out[j].CommittedAt })
	sort.Strings(failed)
	return &SimulateResult{Rule: opts.Rule, Points: out, Failed: failed}, nil
}

// simulateSha runs krit -f json --enable-rules <rule> against a
// detached worktree at sha and returns the count of findings the rule
// produced plus the commit's committer-time. Uses gitMu to serialise
// the worktree add/remove against the primary repo.
func simulateSha(primaryRepoRoot, sha, kritBin, rule, scratchDir string, workerID int, gitMu *sync.Mutex) (int, int64, error) {
	worktreePath := filepath.Join(scratchDir, fmt.Sprintf("w%d-%s", workerID, sha[:12]))

	gitMu.Lock()
	addErr := gitWorktreeAdd(primaryRepoRoot, worktreePath, sha)
	gitMu.Unlock()
	if addErr != nil {
		return 0, 0, addErr
	}
	defer func() {
		gitMu.Lock()
		gitWorktreeRemove(primaryRepoRoot, worktreePath)
		gitMu.Unlock()
	}()

	committedAt, _ := commitTimestamp(primaryRepoRoot, sha)

	cmd := exec.CommandContext(context.Background(), kritBin, "-f", "json", "-enable-rules", rule, ".")
	cmd.Dir = worktreePath
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	// krit returns non-zero when findings exist; ignore exit status and
	// rely on whether stdout parses.
	_ = cmd.Run()

	count, err := parseRuleFindingCount(out.Bytes(), rule)
	if err != nil {
		return 0, committedAt, fmt.Errorf("parse krit output for %s: %w (stderr: %s)", sha, err, errBuf.String())
	}
	return count, committedAt, nil
}

// parseRuleFindingCount reads krit's `-f json` envelope and returns
// the number of findings in summary.byRule[rule]. Absent rule keys are
// 0, not an error.
func parseRuleFindingCount(payload []byte, rule string) (int, error) {
	if len(payload) == 0 {
		return 0, errors.New("empty payload")
	}
	var env struct {
		Summary struct {
			ByRule map[string]int `json:"byRule"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(payload, &env); err != nil {
		return 0, err
	}
	return env.Summary.ByRule[rule], nil
}

// commitTimestamp returns the committer-time (unix seconds) for sha.
// Returns 0 + nil error when git can't resolve it; the simulate flow
// tolerates a missing timestamp without aborting.
func commitTimestamp(repoRoot, sha string) (int64, error) {
	cmd := exec.CommandContext(context.Background(), "git", "log", "-1", "--format=%ct", sha)
	cmd.Dir = repoRoot
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return 0, err
	}
	var ts int64
	if _, err := fmt.Sscanf(out.String(), "%d", &ts); err != nil {
		return 0, err
	}
	return ts, nil
}

func resolveKritBin() (string, error) {
	if exe, err := os.Executable(); err == nil {
		if abs, err := filepath.Abs(exe); err == nil {
			return abs, nil
		}
	}
	if path, err := exec.LookPath("krit"); err == nil {
		return path, nil
	}
	return "", errors.New("snapshot: cannot locate krit binary; set KritBin")
}
