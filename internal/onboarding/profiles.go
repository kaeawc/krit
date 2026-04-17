package onboarding

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// StrictStages lists the ordered sub-stages emitted by `krit -v` to
// stderr during a scan. The TUI uses this to render a live progress
// bar for the strict profile scan (the slowest, cold-cache one).
// Each entry pairs a stderr-line prefix with a short human label.
// Stages that don't fire in smaller projects (e.g. Android-only) are
// still listed — the progress bar simply advances when it sees them.
var StrictStages = []struct {
	Prefix string
	Label  string
}{
	{"verbose: Found", "discovered files"},
	{"verbose: Type resolver", "type resolver ready"},
	{"verbose: Running", "rules loaded"},
	{"verbose: Cache:", "cache loaded"},
	{"verbose: Parsed", "parsed sources"},
	{"verbose: Type-indexed", "type indexed"},
	{"verbose: Cross-file", "cross-file analyzed"},
	{"verbose: Analyzed", "rules executed"},
	{"verbose: Android project", "android analyzed"},
}

// ProgressEvent is emitted by ScanProfileWithProgress as each
// verbose-stderr stage marker is seen.
type ProgressEvent struct {
	// StageIndex is the 1-based index of the stage just completed
	// (1..len(StrictStages)).
	StageIndex int
	// StageLabel is the human-readable label from StrictStages.
	StageLabel string
	// TotalStages is len(StrictStages), duplicated here so callers
	// don't need to import the slice.
	TotalStages int
}

// ProfileNames lists the onboarding profiles in presentation order
// (tightest → loosest). This order also determines the order of rows
// in the comparison table.
var ProfileNames = []string{"strict", "balanced", "relaxed", "detekt-compat"}

// ProfilePath returns the path to a profile YAML file for the given
// profile name, rooted at repoRoot/config/profiles/.
func ProfilePath(repoRoot, name string) string {
	return filepath.Join(repoRoot, "config", "profiles", name+".yml")
}

// FindingSample is a trimmed finding stored for TUI preview. The explorer
// right pane shows at most 3 per rule — only file, line, and message are
// kept.
type FindingSample struct {
	File    string
	Line    int
	Message string
}

// ScanResult holds the parsed JSON summary for a single profile scan.
// Only the fields the onboarding flow actually uses are modeled; the
// full krit JSON schema has many more keys.
type ScanResult struct {
	Profile  string                     `json:"profile"`
	Total    int                        `json:"total"`
	Fixable  int                        `json:"fixable"`
	ByRule   map[string]int             `json:"byRule"`
	Findings map[string][]FindingSample `json:"-"` // keyed by rule name; capped at 3 per rule
}

// TopRules returns the top-N rules by count for this scan, in
// descending count order, tie-broken by rule name.
func (r *ScanResult) TopRules(n int) []RuleCount {
	counts := make([]RuleCount, 0, len(r.ByRule))
	for name, count := range r.ByRule {
		counts = append(counts, RuleCount{Name: name, Count: count})
	}
	sort.Slice(counts, func(i, j int) bool {
		if counts[i].Count != counts[j].Count {
			return counts[i].Count > counts[j].Count
		}
		return counts[i].Name < counts[j].Name
	})
	if n > 0 && len(counts) > n {
		counts = counts[:n]
	}
	return counts
}

// RuleCount is a (rule name, finding count) pair used by TopRules.
type RuleCount struct {
	Name  string
	Count int
}

// rawFinding is used to unmarshal the findings array from krit's JSON
// output. Only the fields needed for the explorer preview are included.
type rawFinding struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

// buildFindingsMap groups raw findings by rule name, capping at 3 per
// rule to bound memory on large codebases.
func buildFindingsMap(raw []rawFinding) map[string][]FindingSample {
	m := make(map[string][]FindingSample)
	for _, f := range raw {
		if len(m[f.Rule]) < 3 {
			m[f.Rule] = append(m[f.Rule], FindingSample{
				File:    f.File,
				Line:    f.Line,
				Message: f.Message,
			})
		}
	}
	return m
}

// ScanOptions configures a single krit invocation performed via the
// ScanProfile helper.
type ScanOptions struct {
	// KritBin is the path to the krit binary. Required.
	KritBin string
	// RepoRoot is the krit repository root (so ProfilePath can resolve
	// config/profiles/<name>.yml).
	RepoRoot string
	// Target is the directory to scan.
	Target string
}

// ScanProfile invokes `krit --config <profile.yml> -f json <target>`
// and parses the resulting summary. Krit exits non-zero when findings
// exist; that is treated as success here — only JSON parse errors
// bubble up.
func ScanProfile(ctx context.Context, opts ScanOptions, profile string) (*ScanResult, error) {
	configPath := ProfilePath(opts.RepoRoot, profile)
	if _, err := os.Stat(configPath); err != nil {
		return nil, fmt.Errorf("profile %q not found at %s: %w", profile, configPath, err)
	}

	cmd := exec.CommandContext(ctx, opts.KritBin,
		"--config", configPath,
		"-f", "json",
		opts.Target,
	)
	out, runErr := cmd.Output()
	// A non-zero exit from krit on findings is expected; only fail if
	// we got no stdout at all (indicates a harder failure like an
	// unknown flag or missing binary).
	if len(out) == 0 {
		return nil, fmt.Errorf("krit produced no output for profile %q: %v", profile, runErr)
	}

	var raw struct {
		Findings []rawFinding `json:"findings"`
		Summary  struct {
			Total   int            `json:"total"`
			Fixable int            `json:"fixable"`
			ByRule  map[string]int `json:"byRule"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parsing krit output for profile %q: %w", profile, err)
	}
	return &ScanResult{
		Profile:  profile,
		Total:    raw.Summary.Total,
		Fixable:  raw.Summary.Fixable,
		ByRule:   raw.Summary.ByRule,
		Findings: buildFindingsMap(raw.Findings),
	}, nil
}

// ScanProfileWithProgress is like ScanProfile but also emits
// ProgressEvents as `krit -v` advances through its verbose stages.
// Stderr is tailed line-by-line; each line matching a StrictStages
// prefix fires one callback. Progress events are delivered on the
// goroutine running this function — callers that need to hop onto
// another thread (e.g. a bubbletea message channel) should do that
// inside the callback.
//
// The JSON summary is still parsed from stdout. A progress callback
// of nil degrades to a plain scan.
func ScanProfileWithProgress(
	ctx context.Context,
	opts ScanOptions,
	profile string,
	progress func(ProgressEvent),
) (*ScanResult, error) {
	configPath := ProfilePath(opts.RepoRoot, profile)
	if _, err := os.Stat(configPath); err != nil {
		return nil, fmt.Errorf("profile %q not found at %s: %w", profile, configPath, err)
	}

	cmd := exec.CommandContext(ctx, opts.KritBin,
		"-v",
		"--config", configPath,
		"-f", "json",
		opts.Target,
	)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting krit: %w", err)
	}

	// Tail stderr and fire a ProgressEvent for each stage marker.
	// Done synchronously in a goroutine; we wait for it before Wait().
	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		s := bufio.NewScanner(stderrPipe)
		// Some verbose lines can be long (e.g. long paths); bump the
		// scanner buffer to 1MiB to avoid truncation.
		s.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for s.Scan() {
			line := s.Text()
			if progress == nil {
				continue
			}
			for i, stage := range StrictStages {
				if strings.HasPrefix(line, stage.Prefix) {
					progress(ProgressEvent{
						StageIndex:  i + 1,
						StageLabel:  stage.Label,
						TotalStages: len(StrictStages),
					})
					break
				}
			}
		}
	}()

	out, readErr := io.ReadAll(stdoutPipe)
	<-stderrDone
	waitErr := cmd.Wait()
	if len(out) == 0 {
		if readErr != nil {
			return nil, fmt.Errorf("reading krit stdout: %w", readErr)
		}
		return nil, fmt.Errorf("krit produced no output for profile %q: %v", profile, waitErr)
	}

	var raw struct {
		Findings []rawFinding `json:"findings"`
		Summary  struct {
			Total   int            `json:"total"`
			Fixable int            `json:"fixable"`
			ByRule  map[string]int `json:"byRule"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parsing krit output for profile %q: %w", profile, err)
	}
	return &ScanResult{
		Profile:  profile,
		Total:    raw.Summary.Total,
		Fixable:  raw.Summary.Fixable,
		ByRule:   raw.Summary.ByRule,
		Findings: buildFindingsMap(raw.Findings),
	}, nil
}

// ScanAllProfiles scans the target with every profile in ProfileNames
// sequentially. Returns a map keyed by profile name. The sequential
// execution lets krit's incremental cache warm up — the first scan
// parses every file, the rest only re-evaluate rules.
func ScanAllProfiles(ctx context.Context, opts ScanOptions) (map[string]*ScanResult, error) {
	results := make(map[string]*ScanResult, len(ProfileNames))
	for _, profile := range ProfileNames {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		res, err := ScanProfile(ctx, opts, profile)
		if err != nil {
			return nil, err
		}
		results[profile] = res
	}
	return results, nil
}
