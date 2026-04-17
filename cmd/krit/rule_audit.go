package main

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/experiment"
	"github.com/kaeawc/krit/internal/scanner"
)

// ruleAuditOpts controls the rule-audit output.
type ruleAuditOpts struct {
	// MinFindings filters out rules with fewer findings than this from the
	// main table. Details section still uses the same filter.
	MinFindings int
	// DetailRules is the number of top unexperimented rules to show in the
	// sample-details section.
	DetailRules int
	// SamplesPerRule is the number of sample findings printed per rule in
	// the details section.
	SamplesPerRule int
	// SampleContext is the number of surrounding source lines shown above
	// and below each sample finding's line.
	SampleContext int
	// ClusterFilter, if non-empty, restricts the audit to rules whose
	// cluster label contains the given substring. Case-insensitive.
	ClusterFilter string
	// Targets is the set of scan paths the audit was invoked over. Used
	// to group findings in multi-target mode.
	Targets []string
	// Format is "plain" or "json".
	Format string
}

type ruleStats struct {
	rule        string
	count       int
	rows        []int
	experiments []string // experiment names whose TargetRules include this rule
	cluster     string   // short cluster label, or ""
}

type ruleAuditJSONEntry struct {
	Rule        string   `json:"rule"`
	Count       int      `json:"count"`
	Cluster     string   `json:"cluster,omitempty"`
	Experiments []string `json:"experiments,omitempty"`
	Samples     []string `json:"samples,omitempty"` // file:line:col:rule
}

type ruleAuditJSONTarget struct {
	Target         string               `json:"target"`
	TotalFindings  int                  `json:"totalFindings"`
	Rules          int                  `json:"rules"`
	Unexperimented int                  `json:"unexperimented"`
	Entries        []ruleAuditJSONEntry `json:"entries"`
}

type ruleAuditJSONReport struct {
	Version string                `json:"version"`
	Targets []ruleAuditJSONTarget `json:"targets"`
}

// runRuleAudit prints a prioritized audit of every rule that fires on the
// target(s), annotated with whether any experiment in the catalog already
// targets it and a short "cluster" tag (dominant file extension or file
// pattern) to help pick the next FP-hunt target. Returns the process exit
// code.
func runRuleAudit(findings []scanner.Finding, opts ruleAuditOpts) int {
	columns := scanner.CollectFindings(findings)
	return runRuleAuditColumns(&columns, opts)
}

func runRuleAuditColumns(columns *scanner.FindingColumns, opts ruleAuditOpts) int {
	opts = normalizeRuleAuditOpts(opts)
	experimentsByRule := buildRuleAuditExperimentsByRule()
	absTargets := absoluteRuleAuditTargets(opts.Targets)
	byTarget := make(map[string][]int, len(absTargets))
	for _, target := range absTargets {
		byTarget[target] = nil
	}
	if columns != nil {
		for row := 0; row < columns.Len(); row++ {
			target := ruleAuditTargetForFile(absTargets, columns.FileAt(row))
			byTarget[target] = append(byTarget[target], row)
		}
	}

	switch opts.Format {
	case "json":
		return emitRuleAuditJSON(columns, byTarget, absTargets, experimentsByRule, opts)
	default:
		return emitRuleAuditPlain(columns, byTarget, absTargets, experimentsByRule, opts)
	}
}

// buildStatsForTarget groups findings for a single target into ruleStats,
// applies MinFindings + ClusterFilter, and returns the sorted slice.
func buildStatsForTarget(columns *scanner.FindingColumns, rows []int, experimentsByRule map[string][]string, opts ruleAuditOpts) []*ruleStats {
	byRule := make(map[string]*ruleStats)
	for _, row := range rows {
		rule := columns.RuleAt(row)
		rs, ok := byRule[rule]
		if !ok {
			rs = &ruleStats{rule: rule}
			byRule[rule] = rs
		}
		rs.count++
		rs.rows = append(rs.rows, row)
	}
	for _, rs := range byRule {
		if names := experimentsByRule[rs.rule]; len(names) > 0 {
			rs.experiments = append([]string(nil), names...)
			sort.Strings(rs.experiments)
		}
		rs.cluster = computeCluster(columns, rs.rows)
	}
	filter := strings.ToLower(strings.TrimSpace(opts.ClusterFilter))
	stats := make([]*ruleStats, 0, len(byRule))
	for _, rs := range byRule {
		if rs.count < opts.MinFindings {
			continue
		}
		if filter != "" && !strings.Contains(strings.ToLower(rs.cluster), filter) {
			continue
		}
		stats = append(stats, rs)
	}
	sort.SliceStable(stats, func(i, j int) bool {
		iHas := len(stats[i].experiments) > 0
		jHas := len(stats[j].experiments) > 0
		if iHas != jHas {
			return !iHas
		}
		if stats[i].count != stats[j].count {
			return stats[i].count > stats[j].count
		}
		return stats[i].rule < stats[j].rule
	})
	return stats
}

// emitRuleAuditPlain prints the human-readable table + details section.
func emitRuleAuditPlain(columns *scanner.FindingColumns, byTarget map[string][]int, orderedTargets []string, experimentsByRule map[string][]string, opts ruleAuditOpts) int {
	multi := len(orderedTargets) > 1
	for i, target := range orderedTargets {
		stats := buildStatsForTarget(columns, byTarget[target], experimentsByRule, opts)
		if multi {
			if i > 0 {
				fmt.Println()
				fmt.Println(strings.Repeat("═", 100))
				fmt.Println()
			}
			fmt.Printf("━━━ Target: %s ━━━\n", target)
		} else {
			fmt.Printf("Rule Audit — %s\n", target)
		}
		total := len(byTarget[target])
		unexperimented := 0
		for _, rs := range stats {
			if len(rs.experiments) == 0 {
				unexperimented++
			}
		}
		prefix := "Total findings: %d across %d rules (%d unexperimented)"
		if opts.ClusterFilter != "" {
			prefix += fmt.Sprintf("; cluster filter: %q", opts.ClusterFilter)
		}
		fmt.Printf(prefix+"\n\n", total, len(stats), unexperimented)

		if len(stats) == 0 {
			fmt.Println("No rules match the current filters.")
			continue
		}

		// Main table.
		fmt.Printf("%8s  %-40s  %-14s  %s\n", "FINDINGS", "RULE", "CLUSTER", "EXPERIMENT")
		fmt.Printf("%s\n", strings.Repeat("─", 100))
		for _, rs := range stats {
			ruleCol := rs.rule
			if len(ruleCol) > 40 {
				ruleCol = ruleCol[:37] + "..."
			}
			cluster := rs.cluster
			if cluster == "" {
				cluster = "—"
			}
			expCol := "—"
			if len(rs.experiments) > 0 {
				joined := strings.Join(rs.experiments, ",")
				if len(joined) > 40 {
					joined = joined[:37] + "..."
				}
				expCol = joined
			}
			fmt.Printf("%8d  %-40s  %-14s  %s\n", rs.count, ruleCol, cluster, expCol)
		}
		fmt.Println()

		// Details: top unexperimented rules with samples.
		var unexp []*ruleStats
		for _, rs := range stats {
			if len(rs.experiments) == 0 {
				unexp = append(unexp, rs)
				if len(unexp) >= opts.DetailRules {
					break
				}
			}
		}
		if len(unexp) == 0 {
			fmt.Println("No unexperimented rules fired on this target under the current filter.")
			continue
		}
		fmt.Printf("Next-target details (top %d unexperimented rules):\n\n", len(unexp))
		absBase, _ := filepath.Abs(target)
		for _, rs := range unexp {
			fmt.Printf("── %s (%d findings, cluster: %s) ──\n", rs.rule, rs.count, fallback(rs.cluster, "mixed"))
			shuffled := deterministicShuffleRows(rs.rows, "audit|"+rs.rule+"|"+fmt.Sprintf("%d", opts.SamplesPerRule))
			n := opts.SamplesPerRule
			if n > len(shuffled) {
				n = len(shuffled)
			}
			for _, row := range shuffled[:n] {
				printAuditSample(columns, row, absBase, opts.SampleContext)
			}
			fmt.Println()
		}
		if i == len(orderedTargets)-1 {
			fmt.Printf("Tip: start hunting with\n")
			fmt.Printf("  krit --sample-rule=%s --sample-count=10 --sample-context=3 %s\n",
				unexp[0].rule, target)
			fmt.Printf("  krit -new-experiment=<name> -new-experiment-description=\"...\" \\\n")
			fmt.Printf("       -new-experiment-target-rules=%s \\\n", unexp[0].rule)
			fmt.Printf("       -new-experiment-wire-file=internal/rules/<file>.go\n")
		}
	}
	return 0
}

// emitRuleAuditJSON writes a JSON representation to stdout suitable for
// tooling (e.g., a CI audit report or a dashboard).
func emitRuleAuditJSON(columns *scanner.FindingColumns, byTarget map[string][]int, orderedTargets []string, experimentsByRule map[string][]string, opts ruleAuditOpts) int {
	report := ruleAuditJSONReport{Version: version}
	for _, target := range orderedTargets {
		stats := buildStatsForTarget(columns, byTarget[target], experimentsByRule, opts)
		unexperimented := 0
		for _, rs := range stats {
			if len(rs.experiments) == 0 {
				unexperimented++
			}
		}
		tgt := ruleAuditJSONTarget{
			Target:         target,
			TotalFindings:  len(byTarget[target]),
			Rules:          len(stats),
			Unexperimented: unexperimented,
		}
		absBase, _ := filepath.Abs(target)
		for _, rs := range stats {
			entry := ruleAuditJSONEntry{
				Rule:        rs.rule,
				Count:       rs.count,
				Cluster:     rs.cluster,
				Experiments: rs.experiments,
			}
			// Include deterministic sample keys (file:line:col:rule) so
			// downstream tooling can join back to findings.
			shuffled := deterministicShuffleRows(rs.rows, "audit|"+rs.rule+"|"+fmt.Sprintf("%d", opts.SamplesPerRule))
			n := opts.SamplesPerRule
			if n > len(shuffled) {
				n = len(shuffled)
			}
			for _, row := range shuffled[:n] {
				rel := columns.FileAt(row)
				if absBase != "" {
					if abs, err := filepath.Abs(columns.FileAt(row)); err == nil {
						if rp, err := filepath.Rel(absBase, abs); err == nil {
							rel = rp
						}
					}
				}
				entry.Samples = append(entry.Samples, fmt.Sprintf("%s:%d:%d:%s", rel, columns.LineAt(row), columns.ColumnAt(row), columns.RuleAt(row)))
			}
			tgt.Entries = append(tgt.Entries, entry)
		}
		report.Targets = append(report.Targets, tgt)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		fmt.Fprintf(os.Stderr, "error: encode rule-audit JSON: %v\n", err)
		return 2
	}
	return 0
}

// deterministicShuffleRows returns a copy of row indexes shuffled via a FNV-seeded
// RNG so that repeated audit runs show the same samples.
func deterministicShuffleRows(rows []int, seedKey string) []int {
	out := append([]int(nil), rows...)
	h := fnv.New64a()
	_, _ = h.Write([]byte(seedKey))
	//nolint:gosec // math/rand is intentional for deterministic sampling.
	rng := rand.New(rand.NewSource(int64(h.Sum64())))
	rng.Shuffle(len(out), func(i, j int) {
		out[i], out[j] = out[j], out[i]
	})
	return out
}

// printAuditSample prints one finding row with surrounding source context.
func printAuditSample(columns *scanner.FindingColumns, row int, absBase string, contextLines int) {
	file := columns.FileAt(row)
	line := columns.LineAt(row)
	col := columns.ColumnAt(row)
	message := columns.MessageAt(row)

	relPath := file
	if absBase != "" {
		if abs, err := filepath.Abs(file); err == nil {
			if rp, err := filepath.Rel(absBase, abs); err == nil {
				relPath = rp
			}
		}
	}
	fmt.Printf("  %s:%d:%d — %s\n", relPath, line, col, message)
	lines, err := readFileLines(file)
	if err != nil {
		return
	}
	start := line - contextLines
	if start < 1 {
		start = 1
	}
	end := line + contextLines
	if end > len(lines) {
		end = len(lines)
	}
	for ln := start; ln <= end; ln++ {
		marker := "   "
		if ln == line {
			marker = " >>"
		}
		fmt.Printf("  %s%5d   %s\n", marker, ln, lines[ln-1])
	}
}

// computeCluster returns a short label describing the dominant file-shape
// cluster for a rule's findings, or "" if no single cluster is dominant.
//
// Clusters are derived from (extension, is-test, is-resource) tuples. If
// >= 80% of findings share a property, that property becomes the cluster.
func computeCluster(columns *scanner.FindingColumns, rows []int) string {
	if len(rows) == 0 {
		return ""
	}
	extCounts := make(map[string]int)
	testCount := 0
	resCount := 0
	generatedCount := 0
	manifestCount := 0
	for _, row := range rows {
		file := columns.FileAt(row)
		ext := strings.ToLower(filepath.Ext(file))
		if ext == "" {
			ext = "(none)"
		}
		extCounts[ext]++
		lower := strings.ToLower(file)
		if strings.Contains(lower, "/test/") || strings.Contains(lower, "/androidtest/") ||
			strings.HasSuffix(lower, "test.kt") || strings.HasSuffix(lower, "tests.kt") {
			testCount++
		}
		if strings.Contains(lower, "/res/") || strings.Contains(lower, "/resources/") {
			resCount++
		}
		if strings.Contains(lower, "/build/generated/") || strings.Contains(lower, "/generated/") {
			generatedCount++
		}
		if strings.HasSuffix(lower, "androidmanifest.xml") {
			manifestCount++
		}
	}
	total := len(rows)
	threshold := (total * 80) / 100
	if threshold < 1 {
		threshold = 1
	}
	var domExt string
	var domCount int
	for ext, c := range extCounts {
		if c > domCount {
			domCount = c
			domExt = ext
		}
	}
	parts := []string{}
	if domCount >= threshold && domExt != "(none)" {
		pct := (domCount * 100) / total
		parts = append(parts, fmt.Sprintf("%s %d%%", strings.TrimPrefix(domExt, "."), pct))
	}
	if manifestCount >= threshold {
		parts = append(parts, "manifest")
	} else if resCount >= threshold {
		parts = append(parts, "res/")
	}
	if testCount >= threshold {
		parts = append(parts, "test")
	}
	if generatedCount >= threshold {
		parts = append(parts, "generated")
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ")
}

func normalizeRuleAuditOpts(opts ruleAuditOpts) ruleAuditOpts {
	if opts.MinFindings < 0 {
		opts.MinFindings = 0
	}
	if opts.DetailRules <= 0 {
		opts.DetailRules = 5
	}
	if opts.SamplesPerRule <= 0 {
		opts.SamplesPerRule = 2
	}
	if opts.SampleContext < 0 {
		opts.SampleContext = 2
	}
	if len(opts.Targets) == 0 {
		opts.Targets = []string{"."}
	}
	if opts.Format == "" {
		opts.Format = "plain"
	}
	return opts
}

func buildRuleAuditExperimentsByRule() map[string][]string {
	experimentsByRule := make(map[string][]string)
	for _, def := range experiment.Definitions() {
		for _, rule := range def.TargetRules {
			experimentsByRule[rule] = append(experimentsByRule[rule], def.Name)
		}
	}
	return experimentsByRule
}

func absoluteRuleAuditTargets(targets []string) []string {
	absTargets := make([]string, len(targets))
	for i, target := range targets {
		if abs, err := filepath.Abs(target); err == nil {
			absTargets[i] = abs
		} else {
			absTargets[i] = target
		}
	}
	return absTargets
}

func ruleAuditTargetForFile(absTargets []string, file string) string {
	abs, err := filepath.Abs(file)
	if err != nil {
		abs = file
	}
	best := ""
	for _, target := range absTargets {
		if strings.HasPrefix(abs, target) && len(target) > len(best) {
			best = target
		}
	}
	if best == "" && len(absTargets) > 0 {
		return absTargets[0]
	}
	return best
}

func fallback(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
