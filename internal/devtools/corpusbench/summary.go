package main

import (
	"fmt"
	"sort"
	"strings"
)

// Totals aggregates the per-repo results into the headline metrics that
// are tracked across iterations.
type Totals struct {
	Repos, OK, Crashes, Timeouts     int
	KtFiles, SyntaxErrorFiles        int
	KotlinLOC                        int
	Findings                         int
	WallMs                           int64
	MaxRSSMB                         float64
	ReposWithModules, ReposWithModel int
}

func totals(rep report) Totals {
	var t Totals
	for _, r := range rep.Results {
		t.Repos++
		switch r.Status {
		case "ok":
			t.OK++
		case "timeout":
			t.Timeouts++
		case "crash", "bad-output":
			t.Crashes++
		}
		t.KtFiles += r.KtFiles
		t.KotlinLOC += r.KotlinLOC
		t.SyntaxErrorFiles += r.SyntaxErrorFiles
		t.Findings += r.Findings
		t.WallMs += r.WallMs
		if r.MaxRSSMB > t.MaxRSSMB {
			t.MaxRSSMB = r.MaxRSSMB
		}
		if r.Modules > 0 {
			t.ReposWithModules++
		}
		if r.ModuleCoverage >= 0.9 {
			t.ReposWithModel++
		}
	}
	return t
}

func renderSummary(rep report, prev *report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Corpus run\n\nkrit %s (commit %s), args `%s`, started %s\n\n",
		rep.KritVersion, short(rep.KritCommit), strings.Join(rep.Args, " "), rep.StartedAt.Format("2006-01-02 15:04Z"))
	writeTotals(&b, rep, prev)
	writeRepoTable(&b, rep, prev)
	writeRuleTable(&b, rep, prev)
	writeProblems(&b, rep)
	return b.String()
}

// tableHeader writes a markdown header row, adding a "previous" column when a
// comparison run is present.
func tableHeader(b *strings.Builder, prev *report, cols ...string) {
	if prev != nil {
		cols = append(cols, "previous")
	}
	fmt.Fprintf(b, "| %s |\n|%s\n", strings.Join(cols, " | "), strings.Repeat("---|", len(cols)))
}

func writeTotals(b *strings.Builder, rep report, prev *report) {
	t, pt := totals(rep), totals(derefOr(prev))
	b.WriteString("## Totals\n\n")
	tableHeader(b, prev, "metric", "value")
	row := func(name string, cur, old any) {
		fmt.Fprintf(b, "| %s | %v |", name, cur)
		if prev != nil {
			fmt.Fprintf(b, " %v |", old)
		}
		b.WriteString("\n")
	}
	row("repos completed", fmt.Sprintf("%d/%d", t.OK, t.Repos), fmt.Sprintf("%d/%d", pt.OK, pt.Repos))
	row("crashes / bad output", t.Crashes, pt.Crashes)
	row("timeouts", t.Timeouts, pt.Timeouts)
	row("Kotlin files / LOC", fmt.Sprintf("%d / %d", t.KtFiles, t.KotlinLOC), fmt.Sprintf("%d / %d", pt.KtFiles, pt.KotlinLOC))
	row("files with syntax errors", t.SyntaxErrorFiles, pt.SyntaxErrorFiles)
	row("repos with ≥90% files in a module", fmt.Sprintf("%d/%d", t.ReposWithModel, t.Repos), fmt.Sprintf("%d/%d", pt.ReposWithModel, pt.Repos))
	row("total findings", t.Findings, pt.Findings)
	row("total wall time (s)", fmt.Sprintf("%.1f", float64(t.WallMs)/1000), fmt.Sprintf("%.1f", float64(pt.WallMs)/1000))
	row("peak RSS (MB)", fmt.Sprintf("%.0f", t.MaxRSSMB), fmt.Sprintf("%.0f", pt.MaxRSSMB))
}

func writeRepoTable(b *strings.Builder, rep report, prev *report) {
	b.WriteString("\n## Per repository\n\n")
	b.WriteString("| repo | build | detected | status | kt files | LOC | wall ms | RSS MB | syntax-err files | modules (w/ src) | module coverage | findings | Δ findings |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|---|---|---|\n")
	prevBy := map[string]Result{}
	for _, r := range derefOr(prev).Results {
		prevBy[r.Name] = r
	}
	for _, r := range rep.Results {
		delta := ""
		if p, ok := prevBy[r.Name]; ok {
			delta = fmt.Sprintf("%+d", r.Findings-p.Findings)
		}
		fmt.Fprintf(b, "| %s | %s | %s | %s | %d | %d | %d | %.0f | %d | %d (%d) | %.0f%% | %d | %s |\n",
			r.Name, r.Build, r.DetectedBuild, r.Status, r.KtFiles, r.KotlinLOC, r.WallMs, r.MaxRSSMB,
			r.SyntaxErrorFiles, r.Modules, r.ModulesWithSources, r.ModuleCoverage*100, r.Findings, delta)
	}
}

func writeRuleTable(b *strings.Builder, rep report, prev *report) {
	b.WriteString("\n## Findings by rule (all repos)\n\n")
	tableHeader(b, prev, "rule", "findings", "repos")
	cur, curRepos := byRule(rep)
	old, _ := byRule(derefOr(prev))
	rules := make([]string, 0, len(cur))
	for k := range cur {
		rules = append(rules, k)
	}
	for k := range old {
		if _, ok := cur[k]; !ok {
			rules = append(rules, k)
		}
	}
	sort.Slice(rules, func(i, j int) bool {
		if cur[rules[i]] != cur[rules[j]] {
			return cur[rules[i]] > cur[rules[j]]
		}
		return rules[i] < rules[j]
	})
	for _, k := range rules {
		fmt.Fprintf(b, "| %s | %d | %d |", k, cur[k], curRepos[k])
		if prev != nil {
			fmt.Fprintf(b, " %d |", old[k])
		}
		b.WriteString("\n")
	}
}

func writeProblems(b *strings.Builder, rep report) {
	b.WriteString("\n## Problems\n\n")
	for _, r := range rep.Results {
		if r.Error == "" && len(r.Warnings) == 0 && len(r.SyntaxErrorSamples) == 0 {
			continue
		}
		fmt.Fprintf(b, "### %s\n\n", r.Name)
		if r.Error != "" {
			fmt.Fprintf(b, "- error: `%s`\n", strings.ReplaceAll(r.Error, "\n", " ⏎ "))
		}
		for _, w := range r.Warnings {
			fmt.Fprintf(b, "- stderr: `%s`\n", w)
		}
		for _, s := range r.SyntaxErrorSamples {
			fmt.Fprintf(b, "- syntax error: `%s`\n", s)
		}
		b.WriteString("\n")
	}
}

func derefOr(r *report) report {
	if r == nil {
		return report{}
	}
	return *r
}

func byRule(rep report) (map[string]int, map[string]int) {
	counts, repos := map[string]int{}, map[string]int{}
	for _, r := range rep.Results {
		for rule, n := range r.FindingsByRul {
			counts[rule] += n
			repos[rule]++
		}
	}
	return counts, repos
}

func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
