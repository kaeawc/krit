package rules

import (
	"fmt"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
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
// delegates to the v2 engine.
type Dispatcher struct {
	v2           *V2Dispatcher
	typeResolver typeinfer.TypeResolver
}

// NewDispatcherV2 creates a dispatcher directly from v2 rules, skipping
// the WrapAllAsV2 roundtrip. This is the preferred constructor now that
// all rules are native v2.
func NewDispatcherV2(activeRules []*v2.Rule, resolver ...typeinfer.TypeResolver) *Dispatcher {
	var res typeinfer.TypeResolver
	if len(resolver) > 0 && resolver[0] != nil {
		res = resolver[0]
	}
	if res != nil {
		for _, r := range activeRules {
			if r != nil && r.Needs.Has(v2.NeedsResolver) && r.SetResolverHook != nil {
				r.SetResolverHook(res)
			}
		}
	}
	var d *V2Dispatcher
	if res != nil {
		d = NewV2Dispatcher(activeRules, res)
	} else {
		d = NewV2Dispatcher(activeRules)
	}
	return &Dispatcher{v2: d, typeResolver: res}
}

// Run executes all rules on a file using single-pass dispatch.
// Delegates to the v2 engine.
func (d *Dispatcher) Run(file *scanner.File) scanner.FindingColumns {
	return d.v2.Run(file)
}

// RunWithStats executes all rules on a file and returns findings plus timing.
// Delegates to the v2 engine.
func (d *Dispatcher) RunWithStats(file *scanner.File) (scanner.FindingColumns, RunStats) {
	return d.v2.RunWithStats(file)
}

// RunColumnsWithStats executes all rules on a file using single-pass dispatch
// and returns filtered findings in columnar form plus execution stats.
func (d *Dispatcher) RunColumnsWithStats(file *scanner.File) (scanner.FindingColumns, RunStats) {
	return d.v2.RunColumnsWithStats(file)
}

// Stats returns counts for logging.
func (d *Dispatcher) Stats() (dispatched, aggregate, lineRules, crossFile, moduleAware, legacy int) {
	return d.v2.Stats()
}

// RunGradle runs all registered Gradle rules against a single parsed
// Gradle build script. See V2Dispatcher.RunGradle.
func (d *Dispatcher) RunGradle(file *scanner.File, cfg *android.BuildConfig) scanner.FindingColumns {
	return d.v2.RunGradle(file, cfg)
}

// RunManifest runs all registered manifest rules against a parsed
// AndroidManifest.xml. manifest must be a *Manifest; typed as interface{}
// to avoid import cycles in the v2 dispatcher.
func (d *Dispatcher) RunManifest(file *scanner.File, manifest *Manifest) scanner.FindingColumns {
	return d.v2.RunManifest(file, manifest)
}

// RunResource runs all registered resource rules against a merged
// ResourceIndex for a res/ directory.
func (d *Dispatcher) RunResource(file *scanner.File, idx *android.ResourceIndex) scanner.FindingColumns {
	return d.v2.RunResource(file, idx)
}


