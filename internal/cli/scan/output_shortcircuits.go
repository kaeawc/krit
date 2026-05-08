package scan

import (
	"github.com/kaeawc/krit/internal/scanner"
)

// sampleRuleOpts groups the inputs RunSampleFindingsColumns needs.
type sampleRuleOpts struct {
	Rule         string
	Count        int
	ContextLines int
	BasePath     string
}

// runSampleRuleShortCircuit dispatches to RunSampleFindingsColumns when
// --sample-rule is set, otherwise reports handled=false so the caller
// can fall through to the normal OutputPhase. Returned exit code is the
// sampler's own exit code; callers pass it through their cleanup
// closure (e.g. closeCaches) before returning to main.
func runSampleRuleShortCircuit(columns *scanner.FindingColumns, opts sampleRuleOpts) (handled bool, code int) {
	if opts.Rule == "" {
		return false, 0
	}
	return true, RunSampleFindingsColumns(columns, opts.Rule, opts.Count, opts.ContextLines, opts.BasePath)
}

// runRuleAuditShortCircuit dispatches to RunRuleAuditColumns when
// --rule-audit is set; otherwise reports handled=false. Same return-
// code contract as runSampleRuleShortCircuit.
func runRuleAuditShortCircuit(columns *scanner.FindingColumns, enabled bool, opts RuleAuditOpts) (handled bool, code int) {
	if !enabled {
		return false, 0
	}
	return true, RunRuleAuditColumns(columns, opts)
}
