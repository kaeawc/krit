package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kaeawc/krit/internal/experiment"
	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/store"
)

type matrixChildReport struct {
	Success    bool              `json:"success"`
	Version    string            `json:"version"`
	DurationMs int64             `json:"durationMs"`
	Files      int               `json:"files"`
	Rules      int               `json:"rules"`
	Findings   []json.RawMessage `json:"findings"`
	Summary    struct {
		Total  int            `json:"total"`
		ByRule map[string]int `json:"byRule"`
	} `json:"summary"`
	PerfTiming []perf.TimingEntry `json:"perfTiming"`
}

type matrixFindingKey struct {
	File string
	Line int
	Col  int
	Rule string
}

type matrixSample struct {
	DurationMs     int64            `json:"durationMs"`
	ExitCode       int              `json:"exitCode"`
	Findings       int              `json:"findings"`
	Files          int              `json:"files"`
	Rules          int              `json:"rules"`
	Target         string           `json:"target,omitempty"`
	PerfBucketsMs  map[string]int64 `json:"perfBucketsMs,omitempty"`
	TopRulesMs     map[string]int64 `json:"topRulesMs,omitempty"`
	RuleCounts     map[string]int   `json:"ruleCounts,omitempty"`
	WallDurationMs int64            `json:"wallDurationMs"`
	FindingKeys    []string         `json:"-"`
}

type matrixTargetReport struct {
	Target           string           `json:"target"`
	MeanDurationMs   int64            `json:"meanDurationMs"`
	MinDurationMs    int64            `json:"minDurationMs"`
	MaxDurationMs    int64            `json:"maxDurationMs"`
	MeanFindings     int64            `json:"meanFindings"`
	MeanExitCode     float64          `json:"meanExitCode"`
	MeanPerfMs       map[string]int64 `json:"meanPerfMs,omitempty"`
	MeanTopRulesMs   map[string]int64 `json:"meanTopRulesMs,omitempty"`
	FindingsDelta    int              `json:"findingsDelta,omitempty"`
	EliminatedByRule map[string]int   `json:"eliminatedByRule,omitempty"`
	IntroducedByRule map[string]int   `json:"introducedByRule,omitempty"`
	SampleEliminated []string         `json:"sampleEliminated,omitempty"`
	SampleIntroduced []string         `json:"sampleIntroduced,omitempty"`
	SignalEliminated int              `json:"signalEliminated,omitempty"`
	SignalIntroduced int              `json:"signalIntroduced,omitempty"`
	NoiseEliminated  int              `json:"noiseEliminated,omitempty"`
	NoiseIntroduced  int              `json:"noiseIntroduced,omitempty"`
}

type matrixCaseReport struct {
	Name             string               `json:"name"`
	Enabled          []string             `json:"enabled"`
	TargetRules      []string             `json:"targetRules,omitempty"`
	MeanDurationMs   int64                `json:"meanDurationMs"`
	MinDurationMs    int64                `json:"minDurationMs"`
	MaxDurationMs    int64                `json:"maxDurationMs"`
	MeanFindings     int64                `json:"meanFindings"`
	MeanExitCode     float64              `json:"meanExitCode"`
	Samples          []matrixSample       `json:"samples"`
	MeanPerfMs       map[string]int64     `json:"meanPerfMs,omitempty"`
	MeanTopRulesMs   map[string]int64     `json:"meanTopRulesMs,omitempty"`
	ByTarget         []matrixTargetReport `json:"byTarget,omitempty"`
	FindingsDelta    int                  `json:"findingsDelta,omitempty"`
	EliminatedByRule map[string]int       `json:"eliminatedByRule,omitempty"`
	IntroducedByRule map[string]int       `json:"introducedByRule,omitempty"`
	SampleEliminated []string             `json:"sampleEliminated,omitempty"`
	SampleIntroduced []string             `json:"sampleIntroduced,omitempty"`
	// Signal = elim/intro restricted to the experiment's declared
	// TargetRules. Everything outside those rules is "noise" — flaky
	// findings from unrelated rules that jitter between runs. This
	// separates the intended effect of an experiment from typeinfer /
	// worker-order non-determinism in other rules.
	SignalEliminated int `json:"signalEliminated,omitempty"`
	SignalIntroduced int `json:"signalIntroduced,omitempty"`
	NoiseEliminated  int `json:"noiseEliminated,omitempty"`
	NoiseIntroduced  int `json:"noiseIntroduced,omitempty"`
}

type matrixReport struct {
	Version     string             `json:"version"`
	GeneratedAt string             `json:"generatedAt"`
	Targets     []string           `json:"targets"`
	Candidates  []string           `json:"candidates"`
	Cases       []matrixCaseReport `json:"cases"`
}

type matrixRunOptions struct {
	format     string
	outputPath string
	matrixSpec string
	candidates []string
	runs       int
	flagArgs   []string
	targets    []string
	noCache    bool
	store      *store.FileStore
}

func runExperimentMatrix(opts matrixRunOptions) int {
	format := opts.format
	outputPath := opts.outputPath
	candidates := opts.candidates
	runs := opts.runs
	flagArgs := opts.flagArgs
	targets := opts.targets
	noCache := opts.noCache
	cases, err := experiment.BuildMatrix(opts.matrixSpec, candidates)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: determine executable: %v\n", err)
		return 2
	}

	report := matrixReport{
		Version:     version,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Targets:     append([]string(nil), targets...),
		Candidates:  append([]string(nil), candidates...),
	}

	for _, c := range cases {
		if c.Name == "baseline" && !noCache {
			key, keyErr := computeMatrixBaselineCacheKey(exe, c.Enabled, flagArgs, targets)
			if keyErr == nil {
				if cached, ok := tryLoadBaseline(key, opts.store); ok {
					short := key
					if len(short) > 8 {
						short = short[:8]
					}
					fmt.Fprintf(os.Stderr, "matrix: baseline cache hit (%s)\n", short)
					report.Cases = append(report.Cases, *cached)
					continue
				}
				short := key
				if len(short) > 8 {
					short = short[:8]
				}
				fmt.Fprintf(os.Stderr, "matrix: baseline cache miss (%s)\n", short)
				caseReport, err := runExperimentMatrixCase(exe, c, runs, flagArgs, targets)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error: experiment case %s: %v\n", c.Name, err)
					return 2
				}
				saveBaseline(key, caseReport, opts.store)
				report.Cases = append(report.Cases, caseReport)
				continue
			}
		}
		caseReport, err := runExperimentMatrixCase(exe, c, runs, flagArgs, targets)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: experiment case %s: %v\n", c.Name, err)
			return 2
		}
		report.Cases = append(report.Cases, caseReport)
	}
	applyMatrixDiffs(&report)

	var out []byte
	switch format {
	case "plain":
		out = []byte(formatMatrixPlain(report))
	default:
		out, err = json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: encode experiment matrix: %v\n", err)
			return 2
		}
		out = append(out, '\n')
	}

	if outputPath != "" {
		if err := os.WriteFile(outputPath, out, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "error: write output: %v\n", err)
			return 2
		}
		return 0
	}
	_, _ = os.Stdout.Write(out)
	return 0
}

func runExperimentMatrixCase(exe string, c experiment.MatrixCase, runs int, flagArgs []string, targets []string) (matrixCaseReport, error) {
	report := matrixCaseReport{
		Name:    c.Name,
		Enabled: append([]string(nil), c.Enabled...),
		Samples: make([]matrixSample, 0, runs*len(targets)),
	}
	var totalDur, totalFindings int64
	var totalExit float64
	perfSum := make(map[string]int64)
	topRuleSum := make(map[string]int64)
	byTarget := make(map[string][]matrixSample, len(targets))

	for _, target := range targets {
		for i := 0; i < runs; i++ {
			sample, err := runExperimentMatrixSample(exe, c.Enabled, flagArgs, target)
			if err != nil {
				return matrixCaseReport{}, err
			}
			report.Samples = append(report.Samples, sample)
			byTarget[target] = append(byTarget[target], sample)
			if len(report.Samples) == 1 || sample.DurationMs < report.MinDurationMs {
				report.MinDurationMs = sample.DurationMs
			}
			if sample.DurationMs > report.MaxDurationMs {
				report.MaxDurationMs = sample.DurationMs
			}
			totalDur += sample.DurationMs
			totalFindings += int64(sample.Findings)
			totalExit += float64(sample.ExitCode)
			for k, v := range sample.PerfBucketsMs {
				perfSum[k] += v
			}
			for k, v := range sample.TopRulesMs {
				topRuleSum[k] += v
			}
		}
	}

	report.MeanDurationMs = totalDur / int64(len(report.Samples))
	report.MeanFindings = totalFindings / int64(len(report.Samples))
	report.MeanExitCode = totalExit / float64(len(report.Samples))
	report.MeanPerfMs = divideInt64Map(perfSum, int64(len(report.Samples)))
	report.MeanTopRulesMs = divideInt64Map(topRuleSum, int64(len(report.Samples)))
	report.ByTarget = make([]matrixTargetReport, 0, len(byTarget))
	for _, target := range targets {
		targetSamples := byTarget[target]
		if len(targetSamples) == 0 {
			continue
		}
		report.ByTarget = append(report.ByTarget, summarizeTargetSamples(target, targetSamples))
	}
	return report, nil
}

func runExperimentMatrixSample(exe string, enabled []string, flagArgs []string, target string) (matrixSample, error) {
	childArgs := []string{"-f", "json"}
	childArgs = append(childArgs, stripExperimentMatrixArgs(flagArgs)...)
	if !containsArg(childArgs, "--no-cache") {
		childArgs = append(childArgs, "--no-cache")
	}
	if len(enabled) > 0 {
		childArgs = append(childArgs, "--experiment", strings.Join(enabled, ","))
	}
	childArgs = append(childArgs, target)

	cmd := exec.Command(exe, childArgs...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	start := time.Now()
	err := cmd.Run()
	wallMs := time.Since(start).Milliseconds()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return matrixSample{}, fmt.Errorf("run child: %w", err)
		}
	}

	var parsed matrixChildReport
	if err := json.Unmarshal([]byte(stdout.String()), &parsed); err != nil {
		return matrixSample{}, fmt.Errorf("decode child JSON (exit=%d): %v stderr=%s", exitCode, err, stderr.String())
	}
	sample := matrixSample{
		DurationMs:     parsed.DurationMs,
		ExitCode:       exitCode,
		Findings:       parsed.Summary.Total,
		Files:          parsed.Files,
		Rules:          parsed.Rules,
		Target:         target,
		WallDurationMs: wallMs,
		PerfBucketsMs:  perfTopLevelMap(parsed.PerfTiming),
		TopRulesMs:     perfTopDispatchRules(parsed.PerfTiming),
		RuleCounts:     copyIntMap(parsed.Summary.ByRule),
		FindingKeys:    parseFindingKeys(parsed.Findings),
	}
	return sample, nil
}

func stripExperimentMatrixArgs(args []string) []string {
	// Go's flag package accepts both `-name` and `--name`. Strip both
	// forms so that child invocations never inherit the matrix driver
	// flags (which would cause recursive matrix spawning).
	longFlags := []string{
		"experiment-matrix", "experiment-candidates", "experiment-runs",
		"experiment-intent", "experiment-targets",
		"experiment", "experiment-off",
		"report",
	}
	// Bool flags: strip without consuming a following arg.
	boolFlags := []string{"no-matrix-cache", "clear-matrix-cache"}
	shortBareFlags := []string{"f", "o"}

	isStripFlagName := func(name string) bool {
		for _, f := range longFlags {
			if name == f {
				return true
			}
		}
		return false
	}
	isShortBareFlag := func(name string) bool {
		for _, f := range shortBareFlags {
			if name == f {
				return true
			}
		}
		return false
	}
	stripFlagName := func(arg string) (string, bool) {
		// Accept `-flag` or `--flag`, optionally with `=value`.
		trimmed := arg
		switch {
		case strings.HasPrefix(trimmed, "--"):
			trimmed = trimmed[2:]
		case strings.HasPrefix(trimmed, "-"):
			trimmed = trimmed[1:]
		default:
			return "", false
		}
		if idx := strings.Index(trimmed, "="); idx >= 0 {
			return trimmed[:idx], true
		}
		return trimmed, false
	}

	var out []string
	skipValue := false
	for i := 0; i < len(args); i++ {
		if skipValue {
			skipValue = false
			continue
		}
		arg := args[i]
		name, hasValue := stripFlagName(arg)
		if name == "" {
			out = append(out, arg)
			continue
		}
		if isStripFlagName(name) || isShortBareFlag(name) {
			if !hasValue {
				skipValue = true
			}
			continue
		}
		isBool := false
		for _, f := range boolFlags {
			if name == f {
				isBool = true
				break
			}
		}
		if isBool {
			// Bool flag: drop it, never consume the next arg.
			continue
		}
		out = append(out, arg)
	}
	return out
}

func perfTopLevelMap(entries []perf.TimingEntry) map[string]int64 {
	out := make(map[string]int64)
	for _, entry := range entries {
		out[entry.Name] = entry.DurationMs
	}
	return out
}

func perfTopDispatchRules(entries []perf.TimingEntry) map[string]int64 {
	out := make(map[string]int64)
	for _, entry := range entries {
		if entry.Name != "ruleExecution" {
			continue
		}
		for _, child := range entry.Children {
			if child.Name != "topDispatchRules" {
				continue
			}
			for _, rule := range child.Children {
				out[rule.Name] = rule.DurationMs
			}
		}
	}
	return out
}

func divideInt64Map(in map[string]int64, denom int64) map[string]int64 {
	if len(in) == 0 || denom == 0 {
		return nil
	}
	out := make(map[string]int64, len(in))
	for k, v := range in {
		out[k] = v / denom
	}
	return out
}

func summarizeTargetSamples(target string, samples []matrixSample) matrixTargetReport {
	report := matrixTargetReport{Target: target}
	var totalDur, totalFindings int64
	var totalExit float64
	perfSum := make(map[string]int64)
	topRuleSum := make(map[string]int64)
	for i, sample := range samples {
		if i == 0 || sample.DurationMs < report.MinDurationMs {
			report.MinDurationMs = sample.DurationMs
		}
		if sample.DurationMs > report.MaxDurationMs {
			report.MaxDurationMs = sample.DurationMs
		}
		totalDur += sample.DurationMs
		totalFindings += int64(sample.Findings)
		totalExit += float64(sample.ExitCode)
		for k, v := range sample.PerfBucketsMs {
			perfSum[k] += v
		}
		for k, v := range sample.TopRulesMs {
			topRuleSum[k] += v
		}
	}
	report.MeanDurationMs = totalDur / int64(len(samples))
	report.MeanFindings = totalFindings / int64(len(samples))
	report.MeanExitCode = totalExit / float64(len(samples))
	report.MeanPerfMs = divideInt64Map(perfSum, int64(len(samples)))
	report.MeanTopRulesMs = divideInt64Map(topRuleSum, int64(len(samples)))
	return report
}

func parseFindingKeys(raw []json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		var finding struct {
			File   string `json:"file"`
			Line   int    `json:"line"`
			Col    int    `json:"col"`
			Column int    `json:"column"`
			Rule   string `json:"rule"`
		}
		if err := json.Unmarshal(item, &finding); err != nil {
			continue
		}
		col := finding.Col
		if col == 0 {
			col = finding.Column
		}
		if finding.File == "" || finding.Line == 0 || finding.Rule == "" {
			continue
		}
		out = append(out, fmt.Sprintf("%s:%d:%d:%s", finding.File, finding.Line, col, finding.Rule))
	}
	sort.Strings(out)
	return out
}

func containsArg(args []string, name string) bool {
	for _, arg := range args {
		if arg == name || strings.HasPrefix(arg, name+"=") {
			return true
		}
	}
	return false
}

// experimentTargetRulesLookup returns a {experiment-name -> target-rules}
// map from the experiment catalog, used to annotate matrix case reports
// so the plain/JSON output can separate signal (diffs on declared target
// rules) from noise (diffs on unrelated rules, typically typeinfer jitter).
func experimentTargetRulesLookup() map[string][]string {
	out := make(map[string][]string)
	for _, def := range experiment.Definitions() {
		if len(def.TargetRules) == 0 {
			continue
		}
		out[def.Name] = append([]string(nil), def.TargetRules...)
	}
	return out
}

// caseTargetRules derives the union of TargetRules from every experiment
// enabled in a given case. Multi-flag cases (cumulative / pairs) union
// the target rules from each enabled experiment.
func caseTargetRules(c *matrixCaseReport, lookup map[string][]string) []string {
	if len(c.Enabled) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	for _, name := range c.Enabled {
		for _, r := range lookup[name] {
			if seen[r] {
				continue
			}
			seen[r] = true
			out = append(out, r)
		}
	}
	sort.Strings(out)
	return out
}

func applyMatrixDiffs(report *matrixReport) {
	var baseline *matrixCaseReport
	for i := range report.Cases {
		if report.Cases[i].Name == "baseline" {
			baseline = &report.Cases[i]
			break
		}
	}
	if baseline == nil {
		return
	}
	lookup := experimentTargetRulesLookup()
	baseByTarget := make(map[string][]matrixSample)
	for _, sample := range baseline.Samples {
		baseByTarget[sample.Target] = append(baseByTarget[sample.Target], sample)
	}

	for i := range report.Cases {
		caseReport := &report.Cases[i]
		caseByTarget := make(map[string][]matrixSample)
		for _, sample := range caseReport.Samples {
			caseByTarget[sample.Target] = append(caseByTarget[sample.Target], sample)
		}

		// Derive the signal-vs-noise rule set up front so that both
		// per-target and case-level classification can share it. For
		// baseline (or cases whose experiments declare no target rules)
		// targetSet stays nil, meaning no signal/noise split is applied.
		var targetSet map[string]bool
		if caseReport.Name != "baseline" {
			caseReport.TargetRules = caseTargetRules(caseReport, lookup)
			if len(caseReport.TargetRules) > 0 {
				targetSet = make(map[string]bool, len(caseReport.TargetRules))
				for _, r := range caseReport.TargetRules {
					targetSet[r] = true
				}
			}
		}

		var eliminatedAll, introducedAll []string
		for j := range caseReport.ByTarget {
			targetReport := &caseReport.ByTarget[j]
			baseSet := stableFindingSet(baseByTarget[targetReport.Target])
			caseSet := stableFindingSet(caseByTarget[targetReport.Target])
			eliminated, introduced := diffFindingSets(baseSet, caseSet)
			targetReport.FindingsDelta = len(caseSet) - len(baseSet)
			targetReport.EliminatedByRule = countFindingsByRule(eliminated)
			targetReport.IntroducedByRule = countFindingsByRule(introduced)
			targetReport.SampleEliminated = sampleFindingKeys(eliminated, 20)
			targetReport.SampleIntroduced = sampleFindingKeys(introduced, 20)
			if targetSet != nil {
				for rule, n := range targetReport.EliminatedByRule {
					if targetSet[rule] {
						targetReport.SignalEliminated += n
					} else {
						targetReport.NoiseEliminated += n
					}
				}
				for rule, n := range targetReport.IntroducedByRule {
					if targetSet[rule] {
						targetReport.SignalIntroduced += n
					} else {
						targetReport.NoiseIntroduced += n
					}
				}
			}
			eliminatedAll = append(eliminatedAll, eliminated...)
			introducedAll = append(introducedAll, introduced...)
		}
		sort.Strings(eliminatedAll)
		sort.Strings(introducedAll)
		caseReport.FindingsDelta = len(stableFindingSet(caseReport.Samples)) - len(stableFindingSet(baseline.Samples))
		caseReport.EliminatedByRule = countFindingsByRule(eliminatedAll)
		caseReport.IntroducedByRule = countFindingsByRule(introducedAll)
		caseReport.SampleEliminated = sampleFindingKeys(eliminatedAll, 20)
		caseReport.SampleIntroduced = sampleFindingKeys(introducedAll, 20)

		// Signal / noise split: the experiment declares which rules it
		// intends to affect; any diff outside that set is treated as
		// noise from unrelated non-deterministic rules.
		if targetSet != nil {
			for rule, n := range caseReport.EliminatedByRule {
				if targetSet[rule] {
					caseReport.SignalEliminated += n
				} else {
					caseReport.NoiseEliminated += n
				}
			}
			for rule, n := range caseReport.IntroducedByRule {
				if targetSet[rule] {
					caseReport.SignalIntroduced += n
				} else {
					caseReport.NoiseIntroduced += n
				}
			}
		}
	}
}

func stableFindingSet(samples []matrixSample) map[string]bool {
	if len(samples) == 0 {
		return nil
	}
	set := make(map[string]bool)
	for _, key := range samples[0].FindingKeys {
		set[key] = true
	}
	for _, sample := range samples[1:] {
		next := make(map[string]bool)
		for _, key := range sample.FindingKeys {
			if set[key] {
				next[key] = true
			}
		}
		set = next
	}
	return set
}

func diffFindingSets(base map[string]bool, current map[string]bool) ([]string, []string) {
	var eliminated, introduced []string
	for key := range base {
		if !current[key] {
			eliminated = append(eliminated, key)
		}
	}
	for key := range current {
		if !base[key] {
			introduced = append(introduced, key)
		}
	}
	sort.Strings(eliminated)
	sort.Strings(introduced)
	return eliminated, introduced
}

func countFindingsByRule(keys []string) map[string]int {
	if len(keys) == 0 {
		return nil
	}
	out := make(map[string]int)
	for _, key := range keys {
		if idx := strings.LastIndex(key, ":"); idx >= 0 && idx+1 < len(key) {
			out[key[idx+1:]]++
		}
	}
	return out
}

func sampleFindingKeys(keys []string, limit int) []string {
	if len(keys) == 0 {
		return nil
	}
	if len(keys) <= limit {
		return append([]string(nil), keys...)
	}
	return append([]string(nil), keys[:limit]...)
}

func copyIntMap(in map[string]int) map[string]int {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func formatMatrixPlain(report matrixReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Experiment Matrix\n")
	fmt.Fprintf(&b, "Version: %s\n", report.Version)
	fmt.Fprintf(&b, "Targets: %s\n\n", strings.Join(report.Targets, ", "))
	fmt.Fprintf(&b, "%-48s %9s %9s %8s %14s %14s\n",
		"Case", "Mean(ms)", "Findings", "Delta", "Signal -/+", "Noise -/+")
	showPerTarget := len(report.Targets) > 1
	for _, c := range report.Cases {
		name := c.Name
		if len(name) > 48 {
			name = name[:45] + "..."
		}
		signal := fmt.Sprintf("%d/%d", c.SignalEliminated, c.SignalIntroduced)
		noise := fmt.Sprintf("%d/%d", c.NoiseEliminated, c.NoiseIntroduced)
		fmt.Fprintf(&b, "%-48s %9d %9d %+8d %14s %14s\n",
			name, c.MeanDurationMs, c.MeanFindings, c.FindingsDelta, signal, noise)
		if !showPerTarget {
			continue
		}
		for _, t := range c.ByTarget {
			label := filepath.Base(t.Target)
			// "  \u2514 " prefix is 4 runes wide; keep the column width
			// consistent with the parent row's 48-char case column.
			const prefix = "  \u2514 "
			maxLabel := 48 - 4
			if len(label) > maxLabel {
				label = label[:maxLabel-3] + "..."
			}
			tSignal := fmt.Sprintf("%d/%d", t.SignalEliminated, t.SignalIntroduced)
			tNoise := fmt.Sprintf("%d/%d", t.NoiseEliminated, t.NoiseIntroduced)
			fmt.Fprintf(&b, "%s%-*s %9d %9d %+8d %14s %14s\n",
				prefix, maxLabel, label,
				t.MeanDurationMs, t.MeanFindings, t.FindingsDelta, tSignal, tNoise)
		}
	}
	fmt.Fprintf(&b, "\nSignal = diffs on the experiment's declared target rules.\n")
	fmt.Fprintf(&b, "Noise  = diffs on unrelated rules (typically typeinfer jitter).\n")

	// Per-case rule diff details — this is the information needed to
	// decide whether to keep, tweak, or discard an experiment treatment.
	for _, c := range report.Cases {
		if c.Name == "baseline" {
			continue
		}
		if len(c.EliminatedByRule) == 0 && len(c.IntroducedByRule) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n== %s ==\n", c.Name)
		if len(c.EliminatedByRule) > 0 {
			fmt.Fprintf(&b, "  eliminated:\n")
			for _, rc := range sortedRuleCounts(c.EliminatedByRule) {
				fmt.Fprintf(&b, "    -%-4d %s\n", rc.count, rc.rule)
			}
		}
		if len(c.IntroducedByRule) > 0 {
			fmt.Fprintf(&b, "  introduced:\n")
			for _, rc := range sortedRuleCounts(c.IntroducedByRule) {
				fmt.Fprintf(&b, "    +%-4d %s\n", rc.count, rc.rule)
			}
		}
		if len(c.SampleEliminated) > 0 {
			fmt.Fprintf(&b, "  sample eliminated:\n")
			for _, k := range c.SampleEliminated {
				fmt.Fprintf(&b, "    %s\n", k)
			}
		}
		if len(c.SampleIntroduced) > 0 {
			fmt.Fprintf(&b, "  sample introduced:\n")
			for _, k := range c.SampleIntroduced {
				fmt.Fprintf(&b, "    %s\n", k)
			}
		}
	}
	return b.String()
}

type ruleCountPair struct {
	rule  string
	count int
}

// sortedRuleCounts returns rule/count pairs sorted by descending count,
// tiebroken by rule name. Used for stable, human-readable matrix output.
func sortedRuleCounts(in map[string]int) []ruleCountPair {
	out := make([]ruleCountPair, 0, len(in))
	for r, n := range in {
		out = append(out, ruleCountPair{rule: r, count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].count != out[j].count {
			return out[i].count > out[j].count
		}
		return out[i].rule < out[j].rule
	})
	return out
}

func topExperimentDefinitionsPlain() string {
	defs := experiment.Definitions()
	var b strings.Builder
	fmt.Fprintf(&b, "Available Experiments\n\n")
	for _, def := range defs {
		intent := def.Intent
		if intent == "" {
			intent = "unspecified"
		}
		fmt.Fprintf(&b, "  %-40s [%s] %s\n", def.Name, intent, def.Description)
	}
	return b.String()
}

func sortedDefinitionNames() []string {
	defs := experiment.Definitions()
	out := make([]string, 0, len(defs))
	for _, def := range defs {
		out = append(out, def.Name)
	}
	sort.Strings(out)
	return out
}

func totalRuleCount(in map[string]int) int {
	total := 0
	for _, v := range in {
		total += v
	}
	return total
}
