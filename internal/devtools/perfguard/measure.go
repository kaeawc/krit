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
// oracle JVM and no daemon. --no-cache only covers the incremental and FIR
// caches, so the parse, cross-file and resource caches are off too, and
// runKrit deletes the corpus's .krit directory before every run: each run
// is cold and cannot reuse (or invalidate) the other binary's caches.
var defaultScanArgs = []string{
	"--no-fir", "--no-type-oracle", "--no-daemon",
	"--no-cache", "--no-parse-cache", "--no-cross-file-cache", "--no-resource-cache",
	"--perf", "--perf-rules", "-f", "json", "-q",
}

// sample is one krit run: total wall time from the JSON report, per-phase
// durations keyed by their path in perfTiming, and per-rule durations.
type sample struct {
	TotalMs float64
	Phases  map[string]float64
	Leaves  map[string]bool
	Rules   map[string]float64
	Shares  map[string]float64
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
}

func parseSample(data []byte) (sample, error) {
	var r kritReport
	if err := json.Unmarshal(data, &r); err != nil {
		return sample{}, fmt.Errorf("parse krit JSON: %w", err)
	}
	if len(r.PerfTiming) == 0 {
		return sample{}, errors.New("krit JSON has no perfTiming (was --perf passed?)")
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
	return s, nil
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
	if err := os.RemoveAll(filepath.Join(corpus, ".krit")); err != nil {
		return sample{}, err
	}
	full := append(append([]string{}, args...), corpus)
	cmd := exec.CommandContext(ctx, bin, full...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	var exitErr *exec.ExitError
	if err != nil && (!errors.As(err, &exitErr) || exitErr.ExitCode() != 1) {
		return sample{}, fmt.Errorf("%s %v: %w\n%s", bin, full, err, tail(stderr.String(), 2000))
	}
	s, perr := parseSample(stdout.Bytes())
	if perr != nil {
		return sample{}, fmt.Errorf("%s on %s: %w\n%s", bin, corpus, perr, tail(stderr.String(), 2000))
	}
	return s, nil
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
func measureInterleaved(ctx context.Context, base, head string, args []string, corpus string, rounds int, logf func(string, ...any)) (series, series, error) {
	for _, bin := range []string{base, head} {
		if _, err := runKrit(ctx, bin, args, corpus); err != nil {
			return nil, nil, fmt.Errorf("warmup: %w", err)
		}
	}
	var bs, hs series
	for r := 0; r < rounds; r++ {
		order := []string{base, head}
		if r%2 == 1 {
			order = []string{head, base}
		}
		for _, bin := range order {
			start := time.Now()
			s, err := runKrit(ctx, bin, args, corpus)
			if err != nil {
				return nil, nil, err
			}
			if bin == base {
				bs = append(bs, s)
			} else {
				hs = append(hs, s)
			}
			logf("  round %d %s: %.0fms (wall %s)", r+1, sideName(bin, base), s.TotalMs, time.Since(start).Round(time.Millisecond))
		}
	}
	return bs, hs, nil
}

func sideName(bin, base string) string {
	if bin == base {
		return "base"
	}
	return "head"
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
