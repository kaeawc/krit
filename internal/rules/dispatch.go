package rules

import (
	"fmt"
	"sort"
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

// SortDispatchErrors orders errors by (FilePath, Line, RuleName,
// PanicValue string) so that error emission has a stable cross-run
// ordering regardless of which goroutine recovered each panic first.
//
// Callers in pipeline phases (dispatch, crossfile, etc.) MUST invoke
// this before emitting `[]DispatchError` to a Reporter or returning
// it across a phase boundary — see issues #28 and #29. The helper
// lives in this package so every dispatch-error producer can reach it
// without re-deriving the comparator.
func SortDispatchErrors(errs []DispatchError) {
	if len(errs) <= 1 {
		return
	}
	sort.SliceStable(errs, func(i, j int) bool {
		a, b := errs[i], errs[j]
		if a.FilePath != b.FilePath {
			return a.FilePath < b.FilePath
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.RuleName != b.RuleName {
			return a.RuleName < b.RuleName
		}
		return fmt.Sprintf("%v", a.PanicValue) < fmt.Sprintf("%v", b.PanicValue)
	})
}

// Rule families are no longer expressed as named Go interfaces. Rules
// declare their dispatch intent structurally — callers type-assert to
// anonymous interface types describing just the methods they need.
// See api.Rule.Needs for the canonical classification.
