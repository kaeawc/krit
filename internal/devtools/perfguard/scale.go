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
	// PreExisting marks a super-linear metric that the base binary scales
	// just as badly: main's behavior, not this change's.
	PreExisting  bool
	BaseExponent float64
}

// exponentMargin is how much worse than base a metric's growth exponent may
// be before a metric base already scales super-linearly counts against head.
const exponentMargin = 0.15

// applyBase marks head metrics that are super-linear on base as well, with no
// more than exponentMargin worse growth, as pre-existing. Timing one binary
// alone let a borderline phase on main (I/O-bound, near the gate) fail
// unrelated PRs at random; with base measured on the same runner only a
// change in scaling counts.
func applyBase(head, base []scaleResult) []scaleResult {
	byKey := make(map[string]scaleResult, len(base))
	for _, b := range base {
		byKey[b.Kind+":"+b.Key] = b
	}
	for i, r := range head {
		b, ok := byKey[r.Kind+":"+r.Key]
		if !ok {
			continue
		}
		head[i].BaseExponent = b.Exponent
		if r.Super && b.Super && r.Exponent <= b.Exponent+exponentMargin {
			head[i].PreExisting = true
		}
	}
	return head
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
	var bad, pre []scaleResult
	for _, r := range results {
		switch {
		case r.Super && r.Known == "" && !r.PreExisting:
			bad = append(bad, r)
		case r.Super && r.Known == "" && r.PreExisting:
			pre = append(pre, r)
		}
	}
	if len(bad) > 0 {
		failed = true
		fmt.Fprintf(w, "**%d metric(s) scale super-linearly:**\n\n", len(bad))
		writeScaleTable(w, bad, g)
		fmt.Fprintf(w, "Fix the hot path, or, when an issue already tracks it, list it in internal/devtools/perfguard/known-superlinear.txt with the issue number.\n\n")
	}
	if len(pre) > 0 {
		fmt.Fprintf(w, "**Super-linear on the base too, not caused by this change** (head exponent within %.2f of base's; worth an issue):\n\n", exponentMargin)
		writeScaleTable(w, pre, g)
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
	fmt.Fprintf(w, "| Kind | Metric | Small | Large | Growth | ~O(n^k) | Base ~O(n^k) | Known |\n|---|---|---|---|---|---|---|---|\n")
	for _, r := range rs {
		base := math.Max(r.Small, g.MinMs/(g.Factor*g.Slack))
		baseExp := "—"
		if r.BaseExponent != 0 {
			baseExp = fmt.Sprintf("%.2f", r.BaseExponent)
		}
		fmt.Fprintf(w, "| %s | `%s` | %.0fms | %.0fms | %.1f× | %.2f | %s | %s |\n", r.Kind, r.Key, r.Small, r.Large, r.Large/base, r.Exponent, baseExp, r.Known)
	}
	fmt.Fprintln(w)
}
