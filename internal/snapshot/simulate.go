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
	"time"
)

// defaultSimulateCommits caps a Simulate call when neither Since nor
// MaxCommits is set. Each commit pays one full krit run, so an
// unbounded walk on a long-history repo would burn hours unannounced;
// this default keeps a naive invocation under control.
const defaultSimulateCommits = 50

type SimulateOptions struct {
	RepoRoot   string
	Rule       string
	Branch     string
	Since      time.Duration
	MaxCommits int
	Workers    int
	// KritBin is the krit executable to invoke against each captured
	// worktree. Empty defaults to argv[0]; tests inject a path.
	KritBin string
	// NoSidecar forces the shell-out path even when persisted findings
	// sidecars are available. Tests use this to exercise the legacy
	// path explicitly; production callers should leave it false.
	NoSidecar bool
	Reporter  func(SimulateEvent)
}

// SimulateEvent is one commit's outcome. Kind is "scored" or "failed".
type SimulateEvent struct {
	Kind      string
	CommitSHA string
	Findings  int
	Duration  time.Duration
	Error     error
}

type SimulatePoint struct {
	CommitSHA   string `json:"commit_sha"`
	CommittedAt int64  `json:"committed_at"`
	Findings    int    `json:"findings"`
	// Source identifies where the count came from: "sidecar" when read
	// from a persisted findings.gob.zst, "scan" when produced by a
	// shell-out fallback.
	Source string `json:"source,omitempty"`
}

type SimulateResult struct {
	Rule   string          `json:"rule"`
	Points []SimulatePoint `json:"points"`
	Failed []string        `json:"failed,omitempty"`
}

// Simulate answers "if rule X had been active for the last <since>,
// how many findings would each commit have carried?" by walking
// history with detached worktrees and shelling out to krit per
// commit. Slow per call (one full analyse per commit) — the use case
// is one-off rule tuning.
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
	maxCommits := opts.MaxCommits
	if maxCommits == 0 && opts.Since == 0 {
		maxCommits = defaultSimulateCommits
	}
	kritBin := opts.KritBin
	if kritBin == "" {
		kritBin, err = resolveKritBin()
		if err != nil {
			return nil, err
		}
	}

	commits, err := listCommits(root, opts.Branch, int64(opts.Since/time.Second), maxCommits)
	if err != nil {
		return nil, err
	}
	if len(commits) == 0 {
		return &SimulateResult{Rule: opts.Rule}, nil
	}

	// Carve off any commits whose findings sidecar is on disk; those
	// become near-instant Timeline reads. The remaining shas fall
	// through to the existing shell-out path.
	var sidecarPoints []SimulatePoint
	var remaining []commitMeta
	if !opts.NoSidecar {
		sidecarPoints, remaining = collectSidecarPoints(root, opts.Rule, commits, opts.Reporter)
	} else {
		remaining = commits
	}
	if len(remaining) == 0 {
		points := append([]SimulatePoint{}, sidecarPoints...)
		sort.Slice(points, func(i, j int) bool { return points[i].CommittedAt > points[j].CommittedAt })
		return &SimulateResult{Rule: opts.Rule, Points: points}, nil
	}

	scratch, err := os.MkdirTemp("", "krit-simulate-*")
	if err != nil {
		return nil, fmt.Errorf("snapshot: scratch dir: %w", err)
	}
	defer os.RemoveAll(scratch)

	timestampBySHA := make(map[string]int64, len(remaining))
	shas := make([]string, len(remaining))
	for i, c := range remaining {
		shas[i] = c.SHA
		timestampBySHA[c.SHA] = c.CommittedAt
	}

	starts := make(map[string]time.Time, len(remaining))
	findings := make(map[string]int, len(remaining))
	var resultsMu sync.Mutex
	var failed []string

	worktreeWalk(root, scratch, workers, shas,
		func(job worktreeJob) error {
			resultsMu.Lock()
			starts[job.SHA] = time.Now()
			resultsMu.Unlock()
			count, err := runKritForRule(kritBin, opts.Rule, job.WorktreePath)
			if err != nil {
				return err
			}
			resultsMu.Lock()
			findings[job.SHA] = count
			resultsMu.Unlock()
			return nil
		},
		func(sha string, err error) {
			resultsMu.Lock()
			start := starts[sha]
			count := findings[sha]
			delete(starts, sha)
			resultsMu.Unlock()
			ev := SimulateEvent{CommitSHA: sha, Findings: count, Duration: time.Since(start)}
			if err != nil {
				ev.Kind = "failed"
				ev.Error = err
				resultsMu.Lock()
				failed = append(failed, sha)
				resultsMu.Unlock()
			} else {
				ev.Kind = "scored"
			}
			if opts.Reporter != nil {
				opts.Reporter(ev)
			}
		},
	)

	points := make([]SimulatePoint, 0, len(findings)+len(sidecarPoints))
	for sha, count := range findings {
		points = append(points, SimulatePoint{
			CommitSHA:   sha,
			CommittedAt: timestampBySHA[sha],
			Findings:    count,
			Source:      "scan",
		})
	}
	points = append(points, sidecarPoints...)
	sort.Slice(points, func(i, j int) bool { return points[i].CommittedAt > points[j].CommittedAt })
	sort.Strings(failed)
	return &SimulateResult{Rule: opts.Rule, Points: points, Failed: failed}, nil
}

// collectSidecarPoints opens each commit's findings sidecar (if any)
// and reads opts.Rule's count. Commits without a sidecar — or with one
// missing the rule's RuleSetHash — fall through unmodified so the
// caller can shell out for them.
//
// snapshotsRoot is .krit/snapshots under repoRoot; we recompute it
// locally because the caller has only the absolute repo path.
func collectSidecarPoints(repoRoot, rule string, commits []commitMeta, reporter func(SimulateEvent)) ([]SimulatePoint, []commitMeta) {
	root := SnapshotsDir(repoRoot)
	if _, err := os.Stat(root); err != nil {
		return nil, commits
	}
	points := make([]SimulatePoint, 0)
	remaining := make([]commitMeta, 0, len(commits))
	for _, c := range commits {
		f, err := LoadFindings(root, c.SHA)
		if err != nil || f == nil {
			remaining = append(remaining, c)
			continue
		}
		count := f.ByRule[rule]
		points = append(points, SimulatePoint{
			CommitSHA:   c.SHA,
			CommittedAt: c.CommittedAt,
			Findings:    count,
			Source:      "sidecar",
		})
		if reporter != nil {
			reporter(SimulateEvent{Kind: "scored", CommitSHA: c.SHA, Findings: count})
		}
	}
	return points, remaining
}

// runKritForRule invokes krit -f json -enable-rules <rule> against
// worktreePath and returns the rule's per-summary count. krit's exit
// code is non-zero when findings exist, so we keep the run error only
// to surface if stdout failed to parse.
func runKritForRule(kritBin, rule, worktreePath string) (int, error) {
	cmd := exec.CommandContext(context.Background(), kritBin, "-f", "json", "-enable-rules", rule, ".")
	cmd.Dir = worktreePath
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	runErr := cmd.Run()
	count, err := parseRuleFindingCount(out.Bytes(), rule)
	if err != nil {
		stderr := bytes.TrimSpace(errBuf.Bytes())
		if runErr != nil {
			return 0, fmt.Errorf("krit run failed (stderr=%q): %w", stderr, runErr)
		}
		return 0, fmt.Errorf("parse krit output (stderr=%q): %w", stderr, err)
	}
	return count, nil
}

// parseRuleFindingCount reads krit's `-f json` envelope. Absent rule
// keys are 0, not an error.
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
