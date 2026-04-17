package pipeline

import (
	"context"
	"runtime"
	"sync"

	"github.com/kaeawc/krit/internal/rules"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

// DispatchPhase runs per-file rule dispatch in parallel. One AST walk
// per file dispatches every active per-file rule through the existing
// V2Dispatcher (by way of rules.NewDispatcher). The dispatcher
// internally applies each file's SuppressionIndex, so the Findings
// returned here are already suppression-filtered.
type DispatchPhase struct {
	// Workers overrides the dispatch worker count. Zero =
	// runtime.NumCPU().
	Workers int
}

// Name returns the stable phase identifier used for timing and error tags.
func (DispatchPhase) Name() string { return "dispatch" }

// Run executes the Dispatch phase. It walks every non-cached file in
// in.KotlinFiles in parallel, dispatching per-file rules through the
// shared dispatcher, and accumulates the findings and run statistics.
func (d DispatchPhase) Run(ctx context.Context, in IndexResult) (DispatchResult, error) {
	if err := ctx.Err(); err != nil {
		return DispatchResult{}, err
	}

	v1Rules := v2RulesToV1(in.ActiveRules)

	// Pass the resolver through unconditionally — NewDispatcher handles a
	// nil resolver gracefully (internal/rules/dispatch.go:54-84 checks
	// res != nil before wiring SetResolver / the v2 dispatcher).
	dispatcher := rules.NewDispatcher(v1Rules, in.Resolver)

	workers := d.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers <= 0 {
		workers = 1
	}

	var (
		wg          sync.WaitGroup
		mu          sync.Mutex
		allFindings []scanner.Finding
		acc         = rules.RunStats{
			DispatchRuleNsByRule: make(map[string]int64),
		}
	)

	sem := make(chan struct{}, workers)
	for _, f := range in.KotlinFiles {
		if in.CacheResult != nil && in.CacheResult.CachedPaths[f.Path] {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(file *scanner.File) {
			defer wg.Done()
			defer func() { <-sem }()

			fileFindings, fileStats := dispatcher.RunWithStats(file)

			mu.Lock()
			if len(fileFindings) > 0 {
				allFindings = append(allFindings, fileFindings...)
			}
			mergeStats(&acc, fileStats)
			mu.Unlock()
		}(f)
	}
	wg.Wait()

	if err := ctx.Err(); err != nil {
		return DispatchResult{}, err
	}

	return DispatchResult{
		IndexResult: in,
		Findings:    scanner.CollectFindings(allFindings),
		Stats:       acc,
	}, nil
}

// v2RulesToV1 converts the IndexResult's []*v2.Rule active set into the
// []rules.Rule slice the legacy dispatcher expects. Each v2.Rule is
// routed through v2.ToV1, whose return type is interface{} — we type-
// assert to rules.Rule and drop entries that don't satisfy it. In
// practice this drops cross-file, module-aware, manifest, resource, and
// gradle wrappers (V1CrossFile, V1ModuleAware, V1Manifest, V1Resource,
// V1Gradle) which do not implement the v1 Rule interface — those rules
// are handled by later phases (CrossFile) rather than per-file dispatch.
// This mirrors the conversion done in internal/rules/zzz_v2bridge.go.
func v2RulesToV1(rs []*v2.Rule) []rules.Rule {
	out := make([]rules.Rule, 0, len(rs))
	for _, r := range rs {
		if r == nil {
			continue
		}
		if v1, ok := v2.ToV1(r).(rules.Rule); ok {
			out = append(out, v1)
		}
	}
	return out
}

// mergeStats folds src's counters into dst. Counter fields add, the
// per-rule map merges by summing matching keys, and the error slice
// appends. Matches the manual accumulation in cmd/krit/main.go:1039-1052.
func mergeStats(dst *rules.RunStats, src rules.RunStats) {
	dst.SuppressionIndexMs += src.SuppressionIndexMs
	dst.DispatchWalkMs += src.DispatchWalkMs
	dst.DispatchRuleNs += src.DispatchRuleNs
	dst.AggregateCollectNs += src.AggregateCollectNs
	dst.AggregateFinalizeMs += src.AggregateFinalizeMs
	dst.LineRuleMs += src.LineRuleMs
	dst.LegacyRuleMs += src.LegacyRuleMs
	dst.SuppressionFilterMs += src.SuppressionFilterMs
	if src.DispatchRuleNsByRule != nil {
		if dst.DispatchRuleNsByRule == nil {
			dst.DispatchRuleNsByRule = make(map[string]int64, len(src.DispatchRuleNsByRule))
		}
		for name, dur := range src.DispatchRuleNsByRule {
			dst.DispatchRuleNsByRule[name] += dur
		}
	}
	if len(src.Errors) > 0 {
		dst.Errors = append(dst.Errors, src.Errors...)
	}
}

// Compile-time check: DispatchPhase satisfies Phase[IndexResult, DispatchResult].
var _ Phase[IndexResult, DispatchResult] = DispatchPhase{}
