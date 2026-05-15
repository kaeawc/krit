// Package triage implements `krit triage`, a post-processing filter
// over a JSON findings report that drops findings whose rule exceeds a
// requested manual-fix effort budget.
//
// Workflow: `krit --format=json . > findings.json && krit triage
// --max-effort=local findings.json`. Reading the same JSON report krit
// emits keeps the command independent of the scan pipeline; teams can
// run triage on archived reports without re-scanning.
package triage

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/kaeawc/krit/internal/output"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// Run is the CLI entry point invoked from cmd/krit.
func Run(args []string) int { return run(args, os.Stdin, os.Stdout, os.Stderr) }

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("triage", flag.ContinueOnError)
	fs.SetOutput(stderr)
	maxEffort := fs.String("max-effort", "local",
		"Drop findings whose rule exceeds this effort tier. One of: trivial, local, refactor, architectural.")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	budget, ok := api.ParseEffort(*maxEffort)
	if !ok {
		fmt.Fprintf(stderr, "triage: invalid --max-effort %q; valid: trivial, local, refactor, architectural\n", *maxEffort)
		return 1
	}

	src, err := openSource(fs.Args(), stdin)
	if err != nil {
		fmt.Fprintf(stderr, "triage: %v\n", err)
		return 1
	}
	defer src.Close()

	var report output.JSONReport
	if err := json.NewDecoder(src).Decode(&report); err != nil {
		fmt.Fprintf(stderr, "triage: decoding findings: %v\n", err)
		return 1
	}

	classifier := registryClassifier()
	report.Findings = FilterFindings(report.Findings, budget, classifier)
	rebuildSummary(&report)

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(&report); err != nil {
		fmt.Fprintf(stderr, "triage: encoding findings: %v\n", err)
		return 1
	}
	return 0
}

// FilterFindings returns findings whose effort is <= budget. The
// effort per finding is read from finding.Effort when populated;
// otherwise it is looked up via classify(ruleID). Findings whose rule
// is unknown to the classifier are kept (a missing rule means we can't
// make a triage decision, so the conservative choice is to surface it).
func FilterFindings(in []output.JSONFinding, budget api.Effort, classify func(rule string) api.Effort) []output.JSONFinding {
	out := make([]output.JSONFinding, 0, len(in))
	for _, f := range in {
		eff := api.EffortUnset
		if f.Effort != "" {
			if parsed, ok := api.ParseEffort(f.Effort); ok {
				eff = parsed
			}
		}
		if eff == api.EffortUnset && classify != nil {
			eff = classify(f.Rule)
		}
		if eff != api.EffortUnset && eff > budget {
			continue
		}
		out = append(out, f)
	}
	return out
}

func registryClassifier() func(string) api.Effort {
	cache := make(map[string]api.Effort, len(api.Registry))
	for _, r := range api.Registry {
		cache[r.ID] = rules.V2RuleEffort(r)
	}
	return func(id string) api.Effort {
		if e, ok := cache[id]; ok {
			return e
		}
		return api.EffortUnset
	}
}

func openSource(args []string, stdin io.Reader) (io.ReadCloser, error) {
	if len(args) == 0 || args[0] == "-" {
		return io.NopCloser(stdin), nil
	}
	f, err := os.Open(args[0])
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", args[0], err)
	}
	return f, nil
}

func rebuildSummary(report *output.JSONReport) {
	byRuleSet := make(map[string]int)
	byRule := make(map[string]int)
	fixable := 0
	for _, f := range report.Findings {
		byRuleSet[f.RuleSet]++
		byRule[f.Rule]++
		if f.Fixable {
			fixable++
		}
	}
	report.Summary = output.JSONSummary{
		Total:     len(report.Findings),
		ByRuleSet: byRuleSet,
		ByRule:    byRule,
		Fixable:   fixable,
	}
}
