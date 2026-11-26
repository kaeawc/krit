package main

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
)

// scaleGates decide when a metric grows faster than the corpus. Factor is
// how many times larger the large corpus is; a linear metric grows about
// Factor times, a quadratic one about Factor² times. A metric fails when it
// costs at least MinMs on the large corpus and grew more than Slack×Factor.
type scaleGates struct {
	Factor float64
	Slack  float64
	MinMs  float64
}

var defaultScaleGates = scaleGates{Slack: 2, MinMs: 500}

type scaleResult struct {
	Kind     string // phase or rule
	Key      string
	Small    float64
	Large    float64
	Exponent float64 // growth as Factor^Exponent
	Super    bool
	Known    string // issue from the allowlist, when listed
}

// knownSuperlinear is the allowlist of metrics that are already known to
// scale super-linearly, each tied to the issue that fixes it. It only
// shrinks: a fix must delete its line, and the guard fails until it does.
type knownSuperlinear map[string]string

func loadKnown(path string) (knownSuperlinear, error) {
	known := knownSuperlinear{}
	if path == "" {
		return known, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for line := 1; sc.Scan(); line++ {
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		fields := strings.Fields(text)
		if len(fields) < 2 || !strings.HasPrefix(fields[1], "#") {
			return nil, fmt.Errorf("%s:%d: want `<phase:path|rule:Name> #<issue>`, got %q", path, line, text)
		}
		known[fields[0]] = fields[1]
	}
	return known, sc.Err()
}

func checkScaling(small, large summary, g scaleGates, known knownSuperlinear) (results []scaleResult, stale []string) {
	floor := g.MinMs / (g.Factor * g.Slack)
	judge := func(kind, key string, s, l float64) {
		if l < g.MinMs {
			return
		}
		base := math.Max(s, floor)
		r := scaleResult{Kind: kind, Key: key, Small: s, Large: l,
			Exponent: math.Log(l/base) / math.Log(g.Factor),
			Super:    l > g.Slack*g.Factor*base}
		r.Known = known[kind+":"+key]
		results = append(results, r)
	}
	for _, k := range sortedKeys(large.Phases) {
		// Leaves only: a super-linear leaf makes every ancestor look
		// super-linear too, and the leaf is the one to fix.
		// "total" is perfTiming's own whole-run node, not a phase to fix.
		if large.Leaves[k] && k != "total" {
			judge("phase", k, small.Phases[k], large.Phases[k])
		}
	}
	for _, k := range sortedKeys(large.Rules) {
		judge("rule", k, small.Rules[k], large.Rules[k])
	}
	superKeys := map[string]bool{}
	for _, r := range results {
		if r.Super {
			superKeys[r.Kind+":"+r.Key] = true
		}
	}
	for k := range known {
		if !superKeys[k] {
			stale = append(stale, k)
		}
	}
	sort.Strings(stale)
	sort.Slice(results, func(i, j int) bool { return results[i].Exponent > results[j].Exponent })
	return results, stale
}

func writeScaleReport(w io.Writer, smallName, largeName string, results []scaleResult, stale []string, g scaleGates, known knownSuperlinear) (failed bool) {
	fmt.Fprintf(w, "## Scaling: %s → %s (%.1f× larger)\n\n", smallName, largeName, g.Factor)
	fmt.Fprintf(w, "A metric fails when it costs >= %.0fms on the large corpus and grew more than %.1f× the corpus (exponent > %.2f).\n\n",
		g.MinMs, g.Slack*g.Factor, math.Log(g.Slack*g.Factor)/math.Log(g.Factor))
	var bad []scaleResult
	for _, r := range results {
		if r.Super && r.Known == "" {
			bad = append(bad, r)
		}
	}
	if len(bad) > 0 {
		failed = true
		fmt.Fprintf(w, "**%d metric(s) scale super-linearly:**\n\n", len(bad))
		writeScaleTable(w, bad, g)
		fmt.Fprintf(w, "Fix the hot path, or, when an issue already tracks it, list it in internal/devtools/perfguard/known-superlinear.txt with the issue number.\n\n")
	}
	if len(stale) > 0 {
		failed = true
		fmt.Fprintf(w, "**known-superlinear.txt lists metrics that no longer scale super-linearly; delete these lines:**\n\n")
		for _, k := range stale {
			fmt.Fprintf(w, "- `%s %s`\n", k, known[k])
		}
		fmt.Fprintln(w)
	}
	if !failed {
		fmt.Fprintf(w, "Every metric scales within the gate.\n\n")
	}
	fmt.Fprintf(w, "<details><summary>All metrics >= %.0fms on the large corpus</summary>\n\n", g.MinMs)
	writeScaleTable(w, results, g)
	fmt.Fprintf(w, "</details>\n\n")
	return failed
}

func writeScaleTable(w io.Writer, rs []scaleResult, g scaleGates) {
	fmt.Fprintf(w, "| Kind | Metric | Small | Large | Growth | ~O(n^k) | Known |\n|---|---|---|---|---|---|---|\n")
	for _, r := range rs {
		base := math.Max(r.Small, g.MinMs/(g.Factor*g.Slack))
		fmt.Fprintf(w, "| %s | `%s` | %.0fms | %.0fms | %.1f× | %.2f | %s |\n", r.Kind, r.Key, r.Small, r.Large, r.Large/base, r.Exponent, r.Known)
	}
	fmt.Fprintln(w)
}
