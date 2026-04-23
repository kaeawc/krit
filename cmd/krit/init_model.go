package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kaeawc/krit/internal/onboarding"
)

// phaseModel is implemented by each TUI phase sub-model. The root
// initModel delegates Update and View to the currently active phase.
type phaseModel interface {
	Update(msg tea.Msg) (phaseModel, tea.Cmd)
	View() string
}

// phaseIniter may be implemented by sub-models that require async
// initialization (e.g. scanning, fixture loading, threshold reads).
type phaseIniter interface {
	Init() tea.Cmd
}

var _ tea.Model = initModel{}

type initModel struct {
	opts     onboarding.ScanOptions
	registry *onboarding.Registry
	target   string

	// Shared state accumulated across phases.
	selected  string
	scans     map[string]*onboarding.ScanResult
	answers   []onboarding.Answer
	liveTotal int

	// Preset flags.
	presetProfile string
	acceptAll     bool

	// Active phase sub-model.
	phase phaseModel

	// Accumulated output state (filled in as phases complete).
	configPath      string
	autofixSkipped  bool
	prefixTotal     int
	postfixTotal    int
	fixedCount      int
	topFixedRules   []onboarding.RuleCount
	baselinePath    string
	baselineWritten bool
	baselineSkipped bool

	err error

	width, height int
}

// fixturePair holds fixture contents for one rule. If a fixable
// fixture with a .expected file exists, fixBefore/fixAfter carry the
// autofix before/after content. Otherwise positive/negative carry the
// triggers/clean fixtures for a stacked view.
type fixturePair struct {
	positive  string
	negative  string
	posErr    string
	negErr    string
	fixBefore string
	fixAfter  string
}

func newInitModel(opts onboarding.ScanOptions, reg *onboarding.Registry, target, preset string, acceptAll bool) initModel {
	sm := newScanningModel(opts, target)
	return initModel{
		opts:          opts,
		registry:      reg,
		target:        target,
		scans:         make(map[string]*onboarding.ScanResult),
		presetProfile: preset,
		acceptAll:     acceptAll,
		phase:         sm,
	}
}

// ---------- messages --------------------------------------------------------

// questionnaireDoneMsg is sent by questionnaireModel when the user has
// answered every question and the answers have been applied.
type questionnaireDoneMsg struct {
	answers   []onboarding.Answer
	liveTotal int
}

// thresholdsDoneMsg is sent by thresholdsModel when the user commits
// or acceptAll is set, carrying the final threshold overrides.
type thresholdsDoneMsg struct {
	overrides []onboarding.ThresholdOverride
}

// explorerDoneMsg is sent by explorerModel when the user commits the
// rule explorer selections, carrying the computed overrides.
type explorerDoneMsg struct {
	overrides []onboarding.Override
}

// autofixAnsweredMsg is sent by autofixConfirmPhase when the user picks yes or no.
type autofixAnsweredMsg struct{ value bool }

// baselineAnsweredMsg is sent by baselineConfirmPhase when the user picks yes or no.
type baselineAnsweredMsg struct{ value bool }

// writeDoneMsg signals that writeConfigCmd completed.
type writeDoneMsg struct {
	path string
	err  error
}

// autofixDoneMsg signals that autofixCmd completed.
type autofixDoneMsg struct {
	prefix  int
	postfix int
	top     []onboarding.RuleCount
	err     error
}

// baselineDoneMsg signals that baselineCmd completed.
type baselineDoneMsg struct {
	path string
	err  error
}

// ---------- Init + command factories ----------------------------------------

func (m initModel) Init() tea.Cmd {
	if pi, ok := m.phase.(phaseIniter); ok {
		return pi.Init()
	}
	return nil
}

// writeConfigCmd writes a krit.yml from questionnaire answers and
// threshold overrides, returning writeDoneMsg.
func (m initModel) writeConfigCmd(thresholds []onboarding.ThresholdOverride) tea.Cmd {
	target := m.target
	repoRoot := m.opts.RepoRoot
	profile := m.selected
	answers := m.answers
	registry := m.registry
	return func() tea.Msg {
		profileYAML, err := os.ReadFile(onboarding.ProfilePath(repoRoot, profile))
		if err != nil {
			return writeDoneMsg{err: fmt.Errorf("reading profile: %w", err)}
		}
		overrides := onboarding.BuildOverrides(registry, answers)
		path, err := onboarding.WriteConfigFile(target, onboarding.WriteConfigOptions{
			ProfileYAML:        profileYAML,
			ProfileName:        profile,
			Overrides:          overrides,
			ThresholdOverrides: thresholds,
		})
		return writeDoneMsg{path: path, err: err}
	}
}

// writeExplorerCmd writes a krit.yml from pre-computed explorer overrides.
func (m initModel) writeExplorerCmd(overrides []onboarding.Override) tea.Cmd {
	target := m.target
	repoRoot := m.opts.RepoRoot
	profile := m.selected
	return func() tea.Msg {
		profileYAML, err := os.ReadFile(onboarding.ProfilePath(repoRoot, profile))
		if err != nil {
			return writeDoneMsg{err: fmt.Errorf("reading profile: %w", err)}
		}
		sorted := make([]onboarding.Override, len(overrides))
		copy(sorted, overrides)
		sort.SliceStable(sorted, func(i, j int) bool {
			if sorted[i].Ruleset != sorted[j].Ruleset {
				return sorted[i].Ruleset < sorted[j].Ruleset
			}
			return sorted[i].Rule < sorted[j].Rule
		})
		path, err := onboarding.WriteConfigFile(target, onboarding.WriteConfigOptions{
			ProfileYAML: profileYAML,
			ProfileName: profile,
			Overrides:   sorted,
		})
		return writeDoneMsg{path: path, err: err}
	}
}

// autofixCmd re-scans the target, runs krit --fix, re-scans again,
// and returns the before/after counts plus top-5 most-fixed rules.
func (m initModel) autofixCmd() tea.Cmd {
	kritBin := m.opts.KritBin
	configPath := m.configPath
	target := m.target
	return func() tea.Msg {
		ctx := context.Background()
		pre, err := runKritJSON(ctx, kritBin, "--config", configPath, "-f", "json", target)
		if err != nil {
			return autofixDoneMsg{err: fmt.Errorf("pre-fix scan: %w", err)}
		}
		prefixTotal := pre.Summary.Total
		preByRule := pre.Summary.ByRule
		// krit --fix returns non-zero when unfixed findings remain; expected.
		_ = exec.CommandContext(ctx, kritBin, "--config", configPath, "--fix", target).Run()
		post, err := runKritJSON(ctx, kritBin, "--config", configPath, "-f", "json", target)
		if err != nil {
			return autofixDoneMsg{err: fmt.Errorf("post-fix scan: %w", err)}
		}
		postByRule := post.Summary.ByRule
		type delta struct {
			name  string
			count int
		}
		var deltas []delta
		for name, count := range preByRule {
			d := count - postByRule[name]
			if d > 0 {
				deltas = append(deltas, delta{name: name, count: d})
			}
		}
		sort.Slice(deltas, func(i, j int) bool {
			if deltas[i].count != deltas[j].count {
				return deltas[i].count > deltas[j].count
			}
			return deltas[i].name < deltas[j].name
		})
		if len(deltas) > 5 {
			deltas = deltas[:5]
		}
		top := make([]onboarding.RuleCount, 0, len(deltas))
		for _, d := range deltas {
			top = append(top, onboarding.RuleCount{Name: d.name, Count: d.count})
		}
		return autofixDoneMsg{prefix: prefixTotal, postfix: post.Summary.Total, top: top}
	}
}

// baselineCmd runs `krit --create-baseline .krit/baseline.xml`.
func (m initModel) baselineCmd() tea.Cmd {
	kritBin := m.opts.KritBin
	configPath := m.configPath
	target := m.target
	return func() tea.Msg {
		baselineDir := filepath.Join(target, ".krit")
		if err := os.MkdirAll(baselineDir, 0o755); err != nil {
			return baselineDoneMsg{err: fmt.Errorf("mkdir %s: %w", baselineDir, err)}
		}
		baselinePath := filepath.Join(baselineDir, "baseline.xml")
		cmd := exec.CommandContext(context.Background(), kritBin,
			"--config", configPath, "--create-baseline", baselinePath, target)
		if err := cmd.Run(); err != nil {
			if _, statErr := os.Stat(baselinePath); statErr != nil {
				return baselineDoneMsg{err: fmt.Errorf("baseline not written: %w (run err: %v)", statErr, err)}
			}
		}
		return baselineDoneMsg{path: baselinePath}
	}
}

// ---------- helpers ---------------------------------------------------------

type kritJSONOutput struct {
	Summary struct {
		Total   int            `json:"total"`
		Fixable int            `json:"fixable"`
		ByRule  map[string]int `json:"byRule"`
	} `json:"summary"`
}

func runKritJSON(ctx context.Context, bin string, args ...string) (*kritJSONOutput, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	out, runErr := cmd.Output()
	if len(out) == 0 {
		return nil, fmt.Errorf("krit produced no output: %v", runErr)
	}
	var parsed kritJSONOutput
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, fmt.Errorf("parsing krit output: %w", err)
	}
	return &parsed, nil
}
