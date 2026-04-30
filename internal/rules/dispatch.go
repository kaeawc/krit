package rules

import (
	"fmt"
	"sort"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// RuleExecutionStat captures per-rule CPU timing for one per-file rule
// family. DurationNs is cumulative callback CPU time across files, not wall
// time; parallel runs can therefore sum above the ruleExecution wall bucket.
type RuleExecutionStat struct {
	Rule        string  `json:"rule"`
	Family      string  `json:"family"`
	Invocations int64   `json:"invocations"`
	DurationNs  int64   `json:"durationNs"`
	DurationMs  float64 `json:"durationMs"`
	AvgNs       int64   `json:"avgNs"`
	SharePct    float64 `json:"sharePct"`
}

// RunStats captures where per-file rule execution time is spent.
type RunStats struct {
	SuppressionIndexMs   int64
	DispatchWalkMs       int64
	DispatchRuleNs       int64
	AggregateCollectNs   int64
	AggregateFinalizeMs  int64
	LineRuleMs           int64
	SuppressionFilterMs  int64
	DispatchRuleNsByRule map[string]int64
	RuleStatsByRule      map[string]RuleExecutionStat
	Errors               []DispatchError
}

func (s *RunStats) recordRule(ruleID, family string, durationNs int64) {
	if ruleID == "" {
		return
	}
	if s.RuleStatsByRule == nil {
		s.RuleStatsByRule = make(map[string]RuleExecutionStat)
	}
	stat := s.RuleStatsByRule[ruleID]
	stat.Rule = ruleID
	stat.Family = family
	stat.Invocations++
	stat.DurationNs += durationNs
	s.RuleStatsByRule[ruleID] = stat
}

// SortedRuleExecutionStats returns deterministic, descending per-rule timing
// rows with derived averages and percentage share filled in.
func SortedRuleExecutionStats(stats RunStats) []RuleExecutionStat {
	if len(stats.RuleStatsByRule) == 0 {
		return nil
	}
	out := make([]RuleExecutionStat, 0, len(stats.RuleStatsByRule))
	var totalNs int64
	for _, stat := range stats.RuleStatsByRule {
		totalNs += stat.DurationNs
		out = append(out, stat)
	}
	for i := range out {
		out[i].DurationMs = float64(out[i].DurationNs) / 1_000_000
		if out[i].Invocations > 0 {
			out[i].AvgNs = out[i].DurationNs / out[i].Invocations
		}
		if totalNs > 0 {
			out[i].SharePct = float64(out[i].DurationNs) * 100 / float64(totalNs)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].DurationNs == out[j].DurationNs {
			return out[i].Rule < out[j].Rule
		}
		return out[i].DurationNs > out[j].DurationNs
	})
	return out
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
	var d *V2Dispatcher
	if res != nil {
		d = NewV2Dispatcher(activeRules, res)
	} else {
		d = NewV2Dispatcher(activeRules)
	}
	return &Dispatcher{v2: d, typeResolver: res}
}

// SetLibraryFacts wires project-wide library semantics into v2 rule contexts.
func (d *Dispatcher) SetLibraryFacts(facts *librarymodel.Facts) {
	if d == nil || d.v2 == nil {
		return
	}
	d.v2.SetLibraryFacts(facts)
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
func (d *Dispatcher) Stats() (dispatched, aggregate, lineRules, crossFile, moduleAware int) {
	return d.v2.Stats()
}

// ReportMissingCapabilities forwards to the underlying V2Dispatcher. See
// V2Dispatcher.ReportMissingCapabilities for semantics.
func (d *Dispatcher) ReportMissingCapabilities(oracleAvailable bool, logger func(format string, args ...any)) {
	d.v2.ReportMissingCapabilities(oracleAvailable, logger)
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

// RunIcons runs all registered icon rules against an IconIndex for a res/
// directory.
func (d *Dispatcher) RunIcons(file *scanner.File, idx *android.IconIndex) scanner.FindingColumns {
	return d.v2.RunIcons(file, idx)
}

// RunResourceSource runs source AST rules that require the Android resource
// index. See V2Dispatcher.RunResourceSource.
func (d *Dispatcher) RunResourceSource(file *scanner.File, idx *android.ResourceIndex) scanner.FindingColumns {
	return d.v2.RunResourceSource(file, idx)
}

// IconRules returns the icon rules classified by the underlying v2 dispatcher.
func (d *Dispatcher) IconRules() []*v2.Rule {
	return d.v2.IconRules()
}
