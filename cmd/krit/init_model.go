package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kaeawc/krit/internal/onboarding"
	"github.com/kaeawc/krit/internal/rules"
)

// ---------- bubbletea model types -------------------------------------------

type phase int

const (
	phaseScanning phase = iota
	phasePicker
	phaseQuestionnaire
	phaseThresholds
	phaseExplorer
	phaseWriting
	phaseAutofixConfirm
	phaseAutofixRunning
	phaseBaselineConfirm
	phaseBaselineRunning
	phaseDone
)

// thresholdSpec describes one tunable numeric rule threshold. The
// slider UI walks this list in declaration order and surfaces each
// threshold's current value (read from the selected profile YAML)
// alongside +/- controls. Keeping this hardcoded is a deliberate
// first-pass choice — the alternative (scraping every threshold via
// reflection off the rule registry) is bigger than the benefit.
type thresholdSpec struct {
	ruleset string
	rule    string
	field   string // YAML key inside the rule block
	label   string
	min     int
	max     int
	step    int
}

var thresholdSpecs = []thresholdSpec{
	{"complexity", "LongMethod", "allowedLines", "LongMethod — max lines per function", 10, 500, 10},
	{"complexity", "CyclomaticComplexMethod", "allowedComplexity", "CyclomaticComplexMethod — max complexity", 2, 50, 1},
	{"complexity", "LongParameterList", "allowedFunctionParameters", "LongParameterList — max function params", 2, 20, 1},
	{"complexity", "LargeClass", "allowedLines", "LargeClass — max lines per class", 100, 3000, 50},
	{"complexity", "NestedBlockDepth", "allowedDepth", "NestedBlockDepth — max nesting", 1, 10, 1},
	{"complexity", "TooManyFunctions", "allowedFunctionsPerFile", "TooManyFunctions — max per file", 2, 80, 1},
	{"complexity", "ComplexCondition", "allowedConditions", "ComplexCondition — max boolean ops", 1, 10, 1},
	{"style", "ReturnCount", "max", "ReturnCount — max returns per function", 1, 10, 1},
	{"style", "ThrowsCount", "max", "ThrowsCount — max throws per function", 1, 10, 1},
	{"style", "MaxLineLength", "maxLineLength", "MaxLineLength — max columns per line", 60, 250, 10},
}

var _ tea.Model = initModel{}

type initModel struct {
	opts      onboarding.ScanOptions
	registry  *onboarding.Registry
	target    string

	phase      phase
	profiles   []string
	scans      map[string]*onboarding.ScanResult
	scanCursor int
	picker     pickerModel
	selected   string

	// Questionnaire state
	visibleQs   []int            // indices into registry.Questions for non-cascaded questions
	qIdx        int              // current position in visibleQs
	qCursor     int              // 0=yes, 1=no
	answers     []onboarding.Answer
	cascaded    map[string]bool
	liveTotal   int              // findings after applying answers so far

	// Live code preview: loaded once at questionnaire start.
	// fixtureCache[questionID] -> (positive, negative) file contents.
	fixtureCache     map[string]fixturePair
	fixtureViewport  viewport.Model // scrollable fixture preview pane

	// Threshold sliders (tui-threshold-sliders).
	thresholdValues    []int // parallel to thresholdSpecs; current tunable value
	thresholdCursor    int
	thresholdOverrides []onboarding.ThresholdOverride // produced when the user advances past the slider phase

	// Rule explorer (tui-split-pane-explorer). Lazily populated when
	// the user presses 'b' from the picker. ruleItems is sorted by
	// ruleset then name. ruleActive is the live on/off state; it
	// starts seeded from the profile defaults and is toggled by the
	// user. explorerCursor and explorerOffset drive the scrolling
	// viewport in the left pane.
	ruleItems      []ruleExplorerItem
	ruleActive     map[string]bool
	explorerCursor       int
	explorerOffset       int
	explorerUsed         bool                    // true if the user committed the explorer flow
	explorerFixtureCache map[string]fixturePair  // lazily-loaded fixtures keyed by rule name

	// Preset flags
	presetProfile string
	acceptAll     bool

	// Scan progress tracking. The strict profile is the slowest
	// (cold cache, full parse) so it drives a live sub-stage
	// progress bar fed by `krit -v` stderr. The other profiles are
	// near-instant after strict warms the cache; they just show a
	// per-profile duration as they finish.
	scanStart         time.Time
	profileStart      map[string]time.Time
	profileDurations  map[string]time.Duration
	scanTotalDuration time.Duration
	strictStageIdx    int    // 0..len(StrictStages)
	strictStageLabel  string // label of the last-seen stage
	strictStageTotal  int    // len(StrictStages), cached once
	strictEvents      chan tea.Msg
	now               time.Time // updated by tickMsg so the view sees fresh elapsed
	scanProgress      progress.Model

	// Output
	configPath string
	err        error

	// Autofix + baseline phases (TUI parity with gum script).
	autofixConfirm  confirmModel
	baselineConfirm confirmModel
	prefixTotal     int // finding count before autofix (from post-write scan)
	postfixTotal    int // finding count after autofix
	fixedCount      int
	topFixedRules   []onboarding.RuleCount
	baselinePath    string
	baselineWritten bool
	autofixSkipped  bool
	baselineSkipped bool

	width  int
	height int
}

// fixturePair holds fixture contents for one rule. If a fixable
// fixture with a .expected file exists, fixBefore/fixAfter carry
// the autofix before/after content and the preview shows a real
// autofix patch. Otherwise, positive/negative carry the
// triggers/clean fixtures for a stacked view.
type fixturePair struct {
	positive string
	negative string
	posErr   string
	negErr   string
	fixBefore string // fixable fixture source (tests/fixtures/fixable/per-rule/{Rule}.kt)
	fixAfter  string // autofix result (.kt.expected)
}

// ruleExplorerItem is a single row in the explorer's left pane.
type ruleExplorerItem struct {
	name    string
	ruleset string
	count   int        // finding count from the selected profile's scan
	ruleRef rules.Rule // back-reference for DescriptionOf; may be nil
}

func newInitModel(opts onboarding.ScanOptions, reg *onboarding.Registry, target, preset string, acceptAll bool) initModel {
	now := time.Now()
	return initModel{
		opts:             opts,
		registry:         reg,
		target:           target,
		phase:            phaseScanning,
		profiles:         onboarding.ProfileNames,
		picker:           newPickerModel(onboarding.ProfileNames),
		scans:            make(map[string]*onboarding.ScanResult),
		cascaded:         make(map[string]bool),
		fixtureCache:     make(map[string]fixturePair),
		presetProfile:    preset,
		acceptAll:        acceptAll,
		scanStart:        now,
		now:              now,
		profileStart:     map[string]time.Time{onboarding.ProfileNames[0]: now},
		profileDurations: make(map[string]time.Duration),
		strictStageTotal: len(onboarding.StrictStages),
		strictEvents:     make(chan tea.Msg, 16),
		scanProgress:     progress.New(progress.WithDefaultGradient(), progress.WithWidth(30), progress.WithoutPercentage()),
	}
}

// ---------- messages --------------------------------------------------------

// fixturesLoadedMsg carries all fixture data loaded asynchronously
// by loadFixturesCmd. Keyed by question ID.
type fixturesLoadedMsg struct {
	cache map[string]fixturePair
}

// loadFixturesCmd returns a tea.Cmd that reads all fixture files from
// disk in a goroutine. The result is delivered as a fixturesLoadedMsg.
func loadFixturesCmd(questions []onboarding.Question, repoRoot string) tea.Cmd {
	return func() tea.Msg {
		cache := make(map[string]fixturePair, len(questions))
		for i := range questions {
			q := &questions[i]
			pair := fixturePair{}
			if q.PositiveFixture != nil {
				data, err := os.ReadFile(filepath.Join(repoRoot, *q.PositiveFixture))
				if err != nil {
					pair.posErr = err.Error()
				} else {
					pair.positive = string(data)
				}
			}
			if q.NegativeFixture != nil {
				data, err := os.ReadFile(filepath.Join(repoRoot, *q.NegativeFixture))
				if err != nil {
					pair.negErr = err.Error()
				} else {
					pair.negative = string(data)
				}
			}
			for _, ruleName := range q.Rules {
				fixPath := filepath.Join(repoRoot, "tests", "fixtures", "fixable", "per-rule", ruleName+".kt")
				expPath := fixPath + ".expected"
				before, errB := os.ReadFile(fixPath)
				after, errA := os.ReadFile(expPath)
				if errB == nil && errA == nil {
					pair.fixBefore = string(before)
					pair.fixAfter = string(after)
					break
				}
			}
			cache[q.ID] = pair
		}
		return fixturesLoadedMsg{cache: cache}
	}
}

type scanDoneMsg struct {
	profile string
	result  *onboarding.ScanResult
	err     error
}

// strictProgressMsg fires every time `krit -v` advances through a
// stage during the strict scan. Only the strict profile emits these.
type strictProgressMsg struct {
	index int
	label string
	total int
}

// tickMsg is a periodic heartbeat that re-renders the scanning view
// so live elapsed counters keep ticking while krit is running.
type tickMsg time.Time

type writeDoneMsg struct {
	path string
	err  error
}

type autofixDoneMsg struct {
	prefix  int
	postfix int
	top     []onboarding.RuleCount
	err     error
}

type baselineDoneMsg struct {
	path string
	err  error
}

// explorerFixtureLoadedMsg carries a lazily-loaded fixture pair for
// the explorer pane. Dispatched when the user moves the cursor to a
// rule whose fixtures have not yet been read from disk.
type explorerFixtureLoadedMsg struct {
	ruleName string
	pair     fixturePair
}

// ---------- Init + command factories ----------------------------------------

func (m initModel) Init() tea.Cmd {
	return tea.Batch(
		m.scanNextCmd(),
		m.drainStrictEventsCmd(),
		tickCmd(),
	)
}

// scanNextCmd kicks off the scan for the next profile in sequence.
// Sequential scans let krit's incremental cache warm up between
// profiles, making the second through fourth scans fast. The first
// profile (strict) is scanned with a progress-aware helper that
// tails krit's verbose stderr and pushes strictProgressMsg events
// through m.strictEvents.
func (m initModel) scanNextCmd() tea.Cmd {
	if m.scanCursor >= len(m.profiles) {
		return nil
	}
	profile := m.profiles[m.scanCursor]
	opts := m.opts
	if profile == "strict" {
		ch := m.strictEvents
		return func() tea.Msg {
			go func() {
				res, err := onboarding.ScanProfileWithProgress(
					context.Background(),
					opts,
					profile,
					func(ev onboarding.ProgressEvent) {
						ch <- strictProgressMsg{
							index: ev.StageIndex,
							label: ev.StageLabel,
							total: ev.TotalStages,
						}
					},
				)
				ch <- scanDoneMsg{profile: profile, result: res, err: err}
			}()
			return nil
		}
	}
	return func() tea.Msg {
		res, err := onboarding.ScanProfile(context.Background(), opts, profile)
		return scanDoneMsg{profile: profile, result: res, err: err}
	}
}

// drainStrictEventsCmd blocks on the strict event channel and
// returns the next message the goroutine pushed. It is re-issued
// after every strict progress / done event so the stream keeps
// flowing until scanning is complete.
func (m initModel) drainStrictEventsCmd() tea.Cmd {
	ch := m.strictEvents
	return func() tea.Msg {
		return <-ch
	}
}

// tickCmd schedules a tickMsg ~100ms from now. Used to refresh the
// scan view's elapsed counters; stops being rescheduled once the
// scan phase ends.
func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m initModel) writeConfigCmd() tea.Cmd {
	target := m.target
	repoRoot := m.opts.RepoRoot
	profile := m.selected
	answers := m.answers
	registry := m.registry
	thresholds := m.thresholdOverrides
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

// autofixCmd re-scans the target with the merged krit.yml to get a
// pre-fix count, runs krit --fix for its side effect (krit --fix
// mutates files and does not emit JSON), then re-scans to count
// remainders. The caller receives the delta and the top-5 most-fixed
// rules for the done view.
func (m initModel) autofixCmd() tea.Cmd {
	kritBin := m.opts.KritBin
	configPath := m.configPath
	target := m.target
	return func() tea.Msg {
		ctx := context.Background()

		// Pre-fix scan.
		pre, err := runKritJSON(ctx, kritBin, "--config", configPath, "-f", "json", target)
		if err != nil {
			return autofixDoneMsg{err: fmt.Errorf("pre-fix scan: %w", err)}
		}
		prefixTotal := pre.Summary.Total
		preByRule := pre.Summary.ByRule

		// Run --fix for its side effect. krit --fix returns non-zero
		// when unfixed findings remain; that is expected.
		_ = exec.CommandContext(ctx, kritBin, "--config", configPath, "--fix", target).Run()

		// Post-fix scan.
		post, err := runKritJSON(ctx, kritBin, "--config", configPath, "-f", "json", target)
		if err != nil {
			return autofixDoneMsg{err: fmt.Errorf("post-fix scan: %w", err)}
		}
		postByRule := post.Summary.ByRule

		// Compute top-5 most-fixed rules by delta.
		type delta struct {
			name  string
			count int
		}
		var deltas []delta
		seen := make(map[string]bool)
		for name, count := range preByRule {
			seen[name] = true
			d := count - postByRule[name]
			if d > 0 {
				deltas = append(deltas, delta{name: name, count: d})
			}
		}
		for name := range postByRule {
			if seen[name] {
				continue
			}
			// A rule that only appears in post cannot have been fixed; skip.
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

		return autofixDoneMsg{
			prefix:  prefixTotal,
			postfix: post.Summary.Total,
			top:     top,
		}
	}
}

// baselineCmd runs `krit --create-baseline .krit/baseline.xml` so the
// remaining findings are suppressed on future runs.
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
			// krit may exit non-zero when findings exist; the
			// baseline is still written. Confirm the file exists
			// before treating this as a hard failure.
			if _, statErr := os.Stat(baselinePath); statErr != nil {
				return baselineDoneMsg{err: fmt.Errorf("baseline not written: %w (run err: %v)", statErr, err)}
			}
		}
		return baselineDoneMsg{path: baselinePath}
	}
}

// writeExplorerCmd writes krit.yml using explorer overrides instead
// of the questionnaire-derived ones. The explorer produces a
// ruleset-qualified list directly from the rule registry.
func (m initModel) writeExplorerCmd() tea.Cmd {
	target := m.target
	repoRoot := m.opts.RepoRoot
	profile := m.selected
	items := m.ruleItems
	active := m.ruleActive
	thresholds := m.thresholdOverrides
	return func() tea.Msg {
		profileYAML, err := os.ReadFile(onboarding.ProfilePath(repoRoot, profile))
		if err != nil {
			return writeDoneMsg{err: fmt.Errorf("reading profile: %w", err)}
		}

		var overrides []onboarding.Override
		for _, item := range items {
			def := rules.IsDefaultActive(item.name)
			state := active[item.name]
			if state == def {
				continue
			}
			overrides = append(overrides, onboarding.Override{
				Ruleset: item.ruleset,
				Rule:    item.name,
				Active:  state,
			})
		}
		sort.SliceStable(overrides, func(i, j int) bool {
			if overrides[i].Ruleset != overrides[j].Ruleset {
				return overrides[i].Ruleset < overrides[j].Ruleset
			}
			return overrides[i].Rule < overrides[j].Rule
		})

		path, err := onboarding.WriteConfigFile(target, onboarding.WriteConfigOptions{
			ProfileYAML:        profileYAML,
			ProfileName:        profile,
			Overrides:          overrides,
			ThresholdOverrides: thresholds,
		})
		return writeDoneMsg{path: path, err: err}
	}
}

// loadExplorerFixtureCmd returns a tea.Cmd that reads positive and
// negative fixture files for ruleName from disk and delivers an
// explorerFixtureLoadedMsg. Called when the cursor lands on a rule
// not yet in explorerFixtureCache.
func (m initModel) loadExplorerFixtureCmd(ruleName, ruleset string) tea.Cmd {
	repoRoot := m.opts.RepoRoot
	return func() tea.Msg {
		pair := fixturePair{}
		posPath := filepath.Join(repoRoot, "tests", "fixtures", "positive", ruleset, ruleName+".kt")
		if data, err := os.ReadFile(posPath); err == nil {
			pair.positive = string(data)
		} else if !os.IsNotExist(err) {
			pair.posErr = err.Error()
		}
		negPath := filepath.Join(repoRoot, "tests", "fixtures", "negative", ruleset, ruleName+".kt")
		if data, err := os.ReadFile(negPath); err == nil {
			pair.negative = string(data)
		} else if !os.IsNotExist(err) {
			pair.negErr = err.Error()
		}
		// Check for autofix fixture.
		fixPath := filepath.Join(repoRoot, "tests", "fixtures", "fixable", "per-rule", ruleName+".kt")
		expPath := fixPath + ".expected"
		if before, err := os.ReadFile(fixPath); err == nil {
			if after, err := os.ReadFile(expPath); err == nil {
				pair.fixBefore = string(before)
				pair.fixAfter = string(after)
			}
		}
		return explorerFixtureLoadedMsg{ruleName: ruleName, pair: pair}
	}
}

// kritJSONOutput is the subset of krit's -f json output the TUI
// actually consumes. Mirrors the fields in internal/onboarding.ScanResult
// but duplicated here so autofixCmd doesn't have to reach into that
// package's unexported ScanProfile helper.
type kritJSONOutput struct {
	Summary struct {
		Total   int            `json:"total"`
		Fixable int            `json:"fixable"`
		ByRule  map[string]int `json:"byRule"`
	} `json:"summary"`
}

// runKritJSON invokes krit with the given args and parses its JSON
// output. A non-zero exit from krit is tolerated as long as the
// stdout payload parses.
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
