package rules

import (
	"fmt"

	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// RunStats captures where per-file rule execution time is spent.
type RunStats struct {
	SuppressionIndexMs   int64
	DispatchWalkMs       int64
	DispatchRuleNs       int64
	AggregateCollectNs   int64
	AggregateFinalizeMs  int64
	LineRuleMs           int64
	LegacyRuleMs         int64
	SuppressionFilterMs  int64
	DispatchRuleNsByRule map[string]int64
	Errors               []DispatchError
}

// DispatchError records a panic recovered during rule execution.
type DispatchError struct {
	RuleName   string
	FilePath   string
	Line       int
	PanicValue interface{}
}

func (e DispatchError) Error() string {
	return fmt.Sprintf("krit: panic in rule %s on %s:%d: %v", e.RuleName, e.FilePath, e.Line, e.PanicValue)
}

// Rule families are no longer expressed as named Go interfaces. Rules
// declare their dispatch intent structurally — callers type-assert to
// anonymous interface types describing just the methods they need.
// See v2.Rule.Needs for the canonical classification.

// Dispatcher is a thin wrapper around V2Dispatcher. All per-file execution
// delegates to the v2 engine; the v1 family slices have been removed as
// dead code now that WrapAllAsV2 handles classification internally.
type Dispatcher struct {
	v2           *V2Dispatcher
	typeResolver typeinfer.TypeResolver
	// activeRules is retained so V2Rules() (in v2dispatch.go) can rebuild
	// its cached V2Index from the original rule list.
	activeRules []Rule
}

// NewDispatcher creates a dispatcher from the given rules.
// An optional TypeResolver enables type-aware analysis for rules with a SetResolver method.
func NewDispatcher(activeRules []Rule, resolver ...typeinfer.TypeResolver) *Dispatcher {
	var res typeinfer.TypeResolver
	if len(resolver) > 0 && resolver[0] != nil {
		res = resolver[0]
	}

	// Set the resolver on any rule that declares a SetResolver method.
	if res != nil {
		for _, r := range activeRules {
			if ta, ok := r.(interface {
				SetResolver(resolver typeinfer.TypeResolver)
			}); ok {
				ta.SetResolver(res)
			}
		}
	}

	v2rules := WrapAllAsV2(activeRules)
	var d *V2Dispatcher
	if res != nil {
		d = NewV2Dispatcher(v2rules, res)
	} else {
		d = NewV2Dispatcher(v2rules)
	}

	return &Dispatcher{
		v2:           d,
		typeResolver: res,
		activeRules:  activeRules,
	}
}

// Run executes all rules on a file using single-pass dispatch.
// Delegates to the v2 engine.
func (d *Dispatcher) Run(file *scanner.File) []scanner.Finding {
	return d.v2.Run(file)
}

// RunWithStats executes all rules on a file and returns findings plus timing.
// Delegates to the v2 engine.
func (d *Dispatcher) RunWithStats(file *scanner.File) ([]scanner.Finding, RunStats) {
	return d.v2.RunWithStats(file)
}

// RunColumnsWithStats executes all rules on a file using single-pass dispatch
// and returns filtered findings in columnar form plus execution stats.
func (d *Dispatcher) RunColumnsWithStats(file *scanner.File) (scanner.FindingColumns, RunStats) {
	findings, stats := d.v2.RunWithStats(file)
	return scanner.CollectFindings(findings), stats
}

// Stats returns counts for logging.
func (d *Dispatcher) Stats() (dispatched, aggregate, lineRules, crossFile, moduleAware, legacy int) {
	return d.v2.Stats()
}

// setDefaultConfidence sets the Confidence field on findings that don't already have one.
func setDefaultConfidence(findings []scanner.Finding, confidence float64) {
	for i := range findings {
		if findings[i].Confidence == 0 {
			findings[i].Confidence = confidence
		}
	}
}

// setRuleConfidence applies a rule's declared base confidence to any
// findings that have not set their own, falling back to the rule-type
// default when the rule does not declare a Confidence() float64 method.
func setRuleConfidence(findings []scanner.Finding, r Rule, fallback float64) {
	ApplyRuleConfidence(findings, r, fallback)
}

// ApplyRuleConfidence is the exported version of setRuleConfidence
// used by call sites outside the dispatcher (cmd/krit for cross-file
// and module-aware rule execution). Per-finding overrides still win
// — the rule's base confidence is only applied to findings whose
// Confidence field is zero.
//
// fallback should be the same per-family default the dispatcher
// uses: 0.95 for cross-file/module-aware rules (AST-level accuracy
// when the index is correct), or the appropriate default for the
// rule family. Rules that want to advertise a lower confidence
// declare a Confidence() float64 method.
func ApplyRuleConfidence(findings []scanner.Finding, r Rule, fallback float64) {
	confidence := ConfidenceOf(r)
	if confidence == 0 {
		confidence = fallback
	}
	setDefaultConfidence(findings, confidence)
}
