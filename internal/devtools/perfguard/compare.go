package main

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
)

// gates are the base-versus-head regression thresholds. A metric regresses
// only when it is both Pct percent slower and MinMs milliseconds slower, so
// sub-noise jitter on tiny phases never fails a PR.
type gates struct {
	TotalPct, TotalMinMs float64
	PhasePct, PhaseMinMs float64
	RulePct, RuleMinMs   float64
	// MaxRuleShare fails a rule that newly takes more than this percentage
	// of rule time (a rule already above it fails only through RulePct).
	MaxRuleShare float64
	// NewRuleMaxMs fails a rule that only head runs once it costs this much.
	NewRuleMaxMs float64
}

var defaultGates = gates{
	TotalPct: 15, TotalMinMs: 300,
	PhasePct: 25, PhaseMinMs: 200,
	RulePct: 30, RuleMinMs: 100,
	MaxRuleShare: 25,
	NewRuleMaxMs: 250,
}

// defaultFIRGates are the gates compare -fir starts from (#1729). A FIR run
// is mostly one JVM compile, so the total and phase floors are seconds, not
// milliseconds. Rule gates are off: per-rule times are Go dispatch, which the
// source-only guard already times, and under FIR a one-time cost (loading the
// oracle's facts) lands on whichever rule asks first, so a rule's time moves
// by hundreds of milliseconds between identical runs.
var defaultFIRGates = gates{
	TotalPct: 15, TotalMinMs: 2000,
	PhasePct: 25, PhaseMinMs: 1500,
	RulePct: math.Inf(1), RuleMinMs: math.Inf(1),
	MaxRuleShare: math.Inf(1),
	NewRuleMaxMs: math.Inf(1),
}

type regression struct {
	Corpus string
	Kind   string // total, phase, rule, rule-share, new-rule
	Key    string
	Base   float64
	Head   float64
	Why    string
}

func slower(base, head, pct, minMs float64) bool {
	return head-base >= minMs && head > base*(1+pct/100)
}

// rulesGated reports whether any rule gate is on.
func (g gates) rulesGated() bool {
	return !math.IsInf(g.RuleMinMs, 1) || !math.IsInf(g.MaxRuleShare, 1) || !math.IsInf(g.NewRuleMaxMs, 1)
}

func compareSummaries(corpus string, base, head summary, g gates) []regression {
	var out []regression
	if slower(base.TotalMs, head.TotalMs, g.TotalPct, g.TotalMinMs) {
		out = append(out, regression{corpus, "total", "durationMs", base.TotalMs, head.TotalMs,
			fmt.Sprintf("> %.0f%% and >= %.0fms slower", g.TotalPct, g.TotalMinMs)})
	}
	// An infinite rule gate (compare -fir) never trips.
	out = append(out, comparePhases(corpus, base, head, g)...)
	return append(out, compareRules(corpus, base, head, g)...)
}

func comparePhases(corpus string, base, head summary, g gates) []regression {
	var out []regression
	for _, k := range sortedKeys(head.Phases) {
		b, ok := base.Phases[k]
		if !ok {
			// A new phase: judge it against zero with the phase floor.
			b = 0
		}
		if slower(b, head.Phases[k], g.PhasePct, g.PhaseMinMs) {
			out = append(out, regression{corpus, "phase", k, b, head.Phases[k],
				fmt.Sprintf("> %.0f%% and >= %.0fms slower", g.PhasePct, g.PhaseMinMs)})
		}
	}
	return out
}

func compareRules(corpus string, base, head summary, g gates) []regression {
	var out []regression
	for _, k := range sortedKeys(head.Rules) {
		h := head.Rules[k]
		b, ok := base.Rules[k]
		if !ok {
			if h >= g.NewRuleMaxMs {
				out = append(out, regression{corpus, "new-rule", k, 0, h,
					fmt.Sprintf("new rule costs >= %.0fms", g.NewRuleMaxMs)})
			}
			continue
		}
		if slower(b, h, g.RulePct, g.RuleMinMs) {
			out = append(out, regression{corpus, "rule", k, b, h,
				fmt.Sprintf("> %.0f%% and >= %.0fms slower", g.RulePct, g.RuleMinMs)})
		}
		if head.Shares[k] > g.MaxRuleShare && base.Shares[k] <= g.MaxRuleShare && h-b >= g.RuleMinMs {
			out = append(out, regression{corpus, "rule-share", k, base.Shares[k], head.Shares[k],
				fmt.Sprintf("now takes > %.0f%% of rule time", g.MaxRuleShare)})
		}
	}
	return out
}

func sortedKeys(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// topKeys returns up to n keys ordered by the larger of their base and head
// values, descending (ties by name), restricted by keep when non-nil.
func topKeys(base, head map[string]float64, n int, keep func(string) bool) []string {
	set := map[string]bool{}
	for k := range base {
		set[k] = true
	}
	for k := range head {
		set[k] = true
	}
	keys := make([]string, 0, len(set))
	for k := range set {
		if keep == nil || keep(k) {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		vi, vj := math.Max(base[keys[i]], head[keys[i]]), math.Max(base[keys[j]], head[keys[j]])
		if vi != vj {
			return vi > vj
		}
		return keys[i] < keys[j]
	})
	if len(keys) > n {
		keys = keys[:n]
	}
	return keys
}

func delta(base, head float64) string {
	d := head - base
	if base <= 0 {
		return fmt.Sprintf("%+.0fms (new)", d)
	}
	return fmt.Sprintf("%+.0fms (%+.1f%%)", d, 100*d/base)
}

type corpusResult struct {
	Name       string
	Base, Head summary
	Rounds     int
}

// writeReport renders the comparison as GitHub-flavored markdown, for the
// job summary and the PR comment.
func writeReport(w io.Writer, results []corpusResult, regs []regression, g gates, title string) {
	fmt.Fprintf(w, "## %s\n\n", title)
	switch {
	case len(regs) == 0 && !g.rulesGated():
		fmt.Fprintf(w, "No performance regressions over the gates (total > %.0f%% and >= %.0fms; phase > %.0f%% and >= %.0fms; rules not gated).\n\n",
			g.TotalPct, g.TotalMinMs, g.PhasePct, g.PhaseMinMs)
	case len(regs) == 0:
		fmt.Fprintf(w, "No performance regressions over the gates (total > %.0f%% and >= %.0fms; phase > %.0f%% and >= %.0fms; rule > %.0f%% and >= %.0fms; new rule >= %.0fms).\n\n",
			g.TotalPct, g.TotalMinMs, g.PhasePct, g.PhaseMinMs, g.RulePct, g.RuleMinMs, g.NewRuleMaxMs)
	default:
		fmt.Fprintf(w, "**%d regression(s):**\n\n| Corpus | Kind | Metric | Base | Head | Gate |\n|---|---|---|---|---|---|\n", len(regs))
		for _, r := range regs {
			unit := "ms"
			if r.Kind == "rule-share" {
				unit = "%"
			}
			fmt.Fprintf(w, "| %s | %s | `%s` | %.1f%s | %.1f%s | %s |\n", r.Corpus, r.Kind, r.Key, r.Base, unit, r.Head, unit, r.Why)
		}
		fmt.Fprintln(w)
	}
	for _, res := range results {
		fmt.Fprintf(w, "<details><summary><b>%s</b>: total %.0fms → %.0fms, %s (median of %d interleaved runs)</summary>\n\n",
			res.Name, res.Base.TotalMs, res.Head.TotalMs, delta(res.Base.TotalMs, res.Head.TotalMs), res.Rounds)
		fmt.Fprintf(w, "| Phase (leaf) | Base | Head | Δ |\n|---|---|---|---|\n")
		for _, k := range topKeys(res.Base.Phases, res.Head.Phases, 15, func(k string) bool { return res.Head.Leaves[k] || res.Base.Leaves[k] }) {
			fmt.Fprintf(w, "| `%s` | %.0fms | %.0fms | %s |\n", k, res.Base.Phases[k], res.Head.Phases[k], delta(res.Base.Phases[k], res.Head.Phases[k]))
		}
		fmt.Fprintf(w, "\n| Rule | Base | Head | Δ | Head share |\n|---|---|---|---|---|\n")
		for _, k := range topKeys(res.Base.Rules, res.Head.Rules, 15, nil) {
			fmt.Fprintf(w, "| `%s` | %.1fms | %.1fms | %s | %.1f%% |\n", k, res.Base.Rules[k], res.Head.Rules[k], delta(res.Base.Rules[k], res.Head.Rules[k]), res.Head.Shares[k])
		}
		var added []string
		for k := range res.Head.Rules {
			if _, ok := res.Base.Rules[k]; !ok {
				added = append(added, fmt.Sprintf("`%s` %.1fms", k, res.Head.Rules[k]))
			}
		}
		sort.Strings(added)
		if len(added) > 0 {
			fmt.Fprintf(w, "\nRules new in head: %s\n", strings.Join(added, ", "))
		}
		fmt.Fprintf(w, "\n</details>\n\n")
	}
}
