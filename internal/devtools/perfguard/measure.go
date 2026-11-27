package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

// defaultScanArgs is the source-only scan the guard times: no FIR, no type
// oracle JVM and no daemon. --no-cache leaves the content-keyed parse cache
// on, so --no-parse-cache turns it off too (the cross-file and resource
// flags are implied by --no-cache and kept for older binaries), and runKrit
// deletes the corpus's .krit directory before every run: each run
// is cold and cannot reuse (or invalidate) the other binary's caches.
var defaultScanArgs = []string{
	"--no-fir", "--no-type-oracle", "--no-daemon",
	"--no-cache", "--no-parse-cache", "--no-cross-file-cache", "--no-resource-cache",
	"--perf", "--perf-rules", "-f", "json", "-q",
}

// firScanArgs is the FIR scan the guard times with -fir (#1729): FIR and the
// type oracle on, in one-shot JVMs (no krit daemon, no persistent krit-fir
// daemon, no oracle cache), so base and head never share a warm JVM or reuse
// each other's compiles. runKrit also gives every run a fresh KRIT_STATE_DIR.
var firScanArgs = []string{
	"--fir", "--no-fir-daemon", "--no-daemon",
	"--no-cache", "--no-parse-cache", "--no-cross-file-cache", "--no-resource-cache", "--no-cache-oracle",
	"--perf", "--perf-rules", "-f", "json", "-q",
}

// side is one binary under test and the environment it runs with (a FIR run
// sets its own KRIT_FIR_JAR, so base and head each use their krit-fir).
type side struct {
	Bin string
	Env []string
	// FIR requires each run to report files FIR analyzed, and runs it with a
	// fresh KRIT_STATE_DIR: a FIR timing without FIR (no jar, no JDK) or
	// with another run's module compiles measures nothing.
	FIR bool
}

// sample is one krit run: total wall time from the JSON report, per-phase
// durations keyed by their path in perfTiming, and per-rule durations.
type sample struct {
	TotalMs float64
	Phases  map[string]float64
	Leaves  map[string]bool
	Rules   map[string]float64
	Shares  map[string]float64
	// FIRFiles is summary.fir.authoritative: the files FIR analyzed.
	FIRFiles int
}

type perfNode struct {
	Name       string     `json:"name"`
	DurationMs float64    `json:"durationMs"`
	Children   []perfNode `json:"children"`
}

type perfRule struct {
	Rule       string  `json:"rule"`
	DurationMs float64 `json:"durationMs"`
	SharePct   float64 `json:"sharePct"`
}

type kritReport struct {
	DurationMs    float64    `json:"durationMs"`
	PerfTiming    []perfNode `json:"perfTiming"`
	PerfRuleStats []perfRule `json:"perfRuleStats"`
	Summary       struct {
		Fir *struct {
			Authoritative int `json:"authoritative"`
		} `json:"fir"`
	} `json:"summary"`
}

func parseSample(data []byte) (sample, error) {
	s, _, err := parseReport(data)
	return s, err
}

// parseReport is parseSample plus the number of files FIR analyzed
// (summary.fir.authoritative; 0 when FIR did not run).
func parseReport(data []byte) (sample, int, error) {
	var r kritReport
	if err := json.Unmarshal(data, &r); err != nil {
		return sample{}, 0, fmt.Errorf("parse krit JSON: %w", err)
	}
	if len(r.PerfTiming) == 0 {
		return sample{}, 0, errors.New("krit JSON has no perfTiming (was --perf passed?)")
	}
	firFiles := 0
	if r.Summary.Fir != nil {
		firFiles = r.Summary.Fir.Authoritative
	}
	s := sample{
		TotalMs: r.DurationMs,
		Phases:  map[string]float64{},
		Leaves:  map[string]bool{},
		Rules:   map[string]float64{},
		Shares:  map[string]float64{},
	}
	flattenPhases("", r.PerfTiming, s.Phases, s.Leaves)
	for _, rule := range r.PerfRuleStats {
		s.Rules[rule.Rule] += rule.DurationMs
		s.Shares[rule.Rule] += rule.SharePct
	}
	return s, firFiles, nil
}

// flattenPhases keys every node by its slash-joined path. A phase that runs
// more than once under the same parent (indexPhaseRun) gets #2, #3, ...
func flattenPhases(prefix string, nodes []perfNode, out map[string]float64, leaves map[string]bool) {
	seen := map[string]int{}
	for _, n := range nodes {
		seen[n.Name]++
		key := n.Name
		if seen[n.Name] > 1 {
			key += "#" + strconv.Itoa(seen[n.Name])
		}
		if prefix != "" {
			key = prefix + "/" + key
		}
		out[key] += n.DurationMs
		if len(n.Children) == 0 {
			leaves[key] = true
		} else {
			flattenPhases(key, n.Children, out, leaves)
		}
	}
}

// runKrit runs one scan and parses its report. krit exits 1 when it reports
// findings, which is the normal case here.
func runKrit(ctx context.Context, bin string, args []string, corpus string) (sample, error) {
	return runSide(ctx, side{Bin: bin}, args, corpus)
}

// runSide is runKrit for one side: its environment, and under FIR a fresh
// state directory and the check that FIR analyzed files.
func runSide(ctx context.Context, sd side, args []string, corpus string) (sample, error) {
	if err := removeState(corpus); err != nil {
		return sample{}, err
	}
	full := append(append([]string{}, args...), corpus)
	cmd := exec.CommandContext(ctx, sd.Bin, full...)
	cmd.Env = append(os.Environ(), sd.Env...)
	if sd.FIR {
		state, err := os.MkdirTemp("", "perfguard-state-*")
		if err != nil {
			return sample{}, err
		}
		defer os.RemoveAll(state)
		cmd.Env = append(cmd.Env, "KRIT_STATE_DIR="+state, "KRIT_NO_JAR_DOWNLOAD=1", "KRIT_NO_DAEMON_AUTOSTART=1")
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	var exitErr *exec.ExitError
	if err != nil && (!errors.As(err, &exitErr) || exitErr.ExitCode() != 1) {
		return sample{}, fmt.Errorf("%s %v: %w\n%s", sd.Bin, full, err, tail(stderr.String(), 2000))
	}
	s, firFiles, perr := parseReport(stdout.Bytes())
	if perr != nil {
		return sample{}, fmt.Errorf("%s on %s: %w\n%s", sd.Bin, corpus, perr, tail(stderr.String(), 2000))
	}
	if sd.FIR && firFiles == 0 {
		return sample{}, fmt.Errorf("%s on %s: FIR analyzed no file (summary.fir.authoritative is 0); is the krit-fir jar (KRIT_FIR_JAR) and a JDK 21 available?\n%s",
			sd.Bin, corpus, tail(stderr.String(), 2000))
	}
	s.FIRFiles = firFiles
	return s, nil
}

// removeState deletes the corpus's .krit directory, the caches a previous run
// left, so every run is cold. An exported build model there (gradle-model,
// bazel-model) is input, not state, and stays.
func removeState(corpus string) error {
	dir := filepath.Join(corpus, ".krit")
	info, err := os.Lstat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	// A symlinked .krit goes as the link; listing it would empty a directory
	// outside the corpus.
	if !info.IsDir() {
		return os.Remove(dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	kept := false
	for _, e := range entries {
		if e.Name() == "gradle-model" || e.Name() == "bazel-model" {
			kept = true
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	if !kept {
		return os.Remove(dir)
	}
	return nil
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "..." + s[len(s)-n:]
}

// series collects the samples of one binary on one corpus.
type series []sample

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	c := append([]float64(nil), xs...)
	sort.Float64s(c)
	if len(c)%2 == 1 {
		return c[len(c)/2]
	}
	return (c[len(c)/2-1] + c[len(c)/2]) / 2
}

// summary holds the per-metric medians of a series. A metric missing from a
// run counts as 0 in that run, so a phase that only sometimes runs does not
// look slower than it is.
type summary struct {
	TotalMs float64
	Phases  map[string]float64
	Leaves  map[string]bool
	Rules   map[string]float64
	Shares  map[string]float64
}

func summarize(ss series) summary {
	out := summary{Phases: map[string]float64{}, Leaves: map[string]bool{}, Rules: map[string]float64{}, Shares: map[string]float64{}}
	var totals []float64
	phaseKeys, ruleKeys := map[string]bool{}, map[string]bool{}
	for _, s := range ss {
		totals = append(totals, s.TotalMs)
		for k := range s.Phases {
			phaseKeys[k] = true
		}
		for k := range s.Leaves {
			out.Leaves[k] = true
		}
		for k := range s.Rules {
			ruleKeys[k] = true
		}
	}
	out.TotalMs = median(totals)
	for k := range phaseKeys {
		var xs []float64
		for _, s := range ss {
			xs = append(xs, s.Phases[k])
		}
		out.Phases[k] = median(xs)
	}
	for k := range ruleKeys {
		var xs, shares []float64
		for _, s := range ss {
			xs = append(xs, s.Rules[k])
			shares = append(shares, s.Shares[k])
		}
		out.Rules[k] = median(xs)
		out.Shares[k] = median(shares)
	}
	return out
}

// measureInterleaved runs warmup scans, then alternates base and head for
// the given rounds, flipping the order every round so neither binary always
// runs first. Both sides see the same runner, page cache and CPU state.
// Under FIR every run must analyze as many files as base's warmup did: a
// head that makes FIR skip files would otherwise pass as faster.
func measureInterleaved(ctx context.Context, base, head side, args []string, corpus string, rounds int, logf func(string, ...any)) (series, series, error) {
	var firFiles int
	checkFiles := func(name string, s sample) error {
		if base.FIR && s.FIRFiles != firFiles {
			return fmt.Errorf("%s's FIR analyzed %d files, base's %d: a timing over different files measures nothing, so check that side's FIR (its krit-fir jar, its scan log)", name, s.FIRFiles, firFiles)
		}
		return nil
	}
	for i, sd := range []side{base, head} {
		s, err := runSide(ctx, sd, args, corpus)
		if err != nil {
			return nil, nil, fmt.Errorf("warmup: %w", err)
		}
		if i == 0 {
			firFiles = s.FIRFiles
		} else if err := checkFiles("head", s); err != nil {
			return nil, nil, fmt.Errorf("warmup: %w", err)
		}
	}
	var bs, hs series
	for r := 0; r < rounds; r++ {
		order := []bool{true, false}
		if r%2 == 1 {
			order = []bool{false, true}
		}
		for _, isBase := range order {
			sd, name := head, "head"
			if isBase {
				sd, name = base, "base"
			}
			start := time.Now()
			s, err := runSide(ctx, sd, args, corpus)
			if err != nil {
				return nil, nil, err
			}
			if err := checkFiles(name, s); err != nil {
				return nil, nil, fmt.Errorf("round %d: %w", r+1, err)
			}
			if isBase {
				bs = append(bs, s)
			} else {
				hs = append(hs, s)
			}
			logf("  round %d %s: %.0fms (wall %s)", r+1, name, s.TotalMs, time.Since(start).Round(time.Millisecond))
		}
	}
	return bs, hs, nil
}

func measure(ctx context.Context, bin string, args []string, corpus string, rounds int) (series, error) {
	if _, err := runKrit(ctx, bin, args, corpus); err != nil {
		return nil, fmt.Errorf("warmup: %w", err)
	}
	var ss series
	for r := 0; r < rounds; r++ {
		s, err := runKrit(ctx, bin, args, corpus)
		if err != nil {
			return nil, err
		}
		ss = append(ss, s)
	}
	return ss, nil
}
