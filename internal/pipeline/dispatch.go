package pipeline

import (
	"context"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/kaeawc/krit/internal/perf"
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
//
// When in.ActiveRulesV1 is non-nil the phase uses it directly. Otherwise
// it derives a v1 rule slice from in.ActiveRules via v2.ToV1. When
// in.Tracker is non-nil it wraps the dispatch loop in a "ruleExecution"
// serial child and emits per-family timing entries + a topDispatchRules
// breakdown matching the pre-refactor CLI. When in.UseCache / Cache /
// Version are set, the phase updates and saves the cache under a
// "cacheSave" tracker entry.
func (d DispatchPhase) Run(ctx context.Context, in IndexResult) (DispatchResult, error) {
	if err := ctx.Err(); err != nil {
		return DispatchResult{}, err
	}

	v1Rules := in.ActiveRulesV1
	if v1Rules == nil {
		v1Rules = v2RulesToV1(in.ActiveRules)
	}

	// Pass the resolver through unconditionally — NewDispatcher handles a
	// nil resolver gracefully (internal/rules/dispatch.go:54-84 checks
	// res != nil before wiring SetResolver / the v2 dispatcher).
	dispatcher := rules.NewDispatcher(v1Rules, in.Resolver)

	workers := d.Workers
	if workers <= 0 {
		workers = in.Jobs
	}
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers <= 0 {
		workers = 1
	}

	useCache := in.Cache != nil && in.CacheFilePath != ""
	findingsByFile := map[string]scanner.FindingColumns{}

	var (
		wg          sync.WaitGroup
		mu          sync.Mutex
		allFindings []scanner.Finding
		fileTimings []FileTiming
		acc         = rules.RunStats{
			DispatchRuleNsByRule: make(map[string]int64),
		}
	)

	ruleTracker := perf.Tracker(nil)
	if in.Tracker != nil {
		ruleTracker = in.Tracker.Serial("ruleExecution")
	}

	sem := make(chan struct{}, workers)
	for _, f := range in.KotlinFiles {
		if in.CacheResult != nil && in.CacheResult.CachedPaths[f.Path] {
			continue
		}
		wg.Add(1)
		var dispatchedAt time.Time
		if in.ProfileDispatch {
			dispatchedAt = time.Now()
		}
		sem <- struct{}{}
		go func(file *scanner.File, dispatched time.Time) {
			defer wg.Done()
			defer func() { <-sem }()

			var startedAt, finishedRunAt, lockedAt time.Time
			if in.ProfileDispatch {
				startedAt = time.Now()
			}

			fileFindings, fileStats := dispatcher.RunWithStats(file)

			if in.ProfileDispatch {
				finishedRunAt = time.Now()
			}

			mu.Lock()
			if in.ProfileDispatch {
				lockedAt = time.Now()
			}
			if len(fileFindings) > 0 {
				allFindings = append(allFindings, fileFindings...)
			}
			if useCache {
				findingsByFile[file.Path] = scanner.CollectFindings(fileFindings)
			}
			mergeStats(&acc, fileStats)
			if in.ProfileDispatch {
				endAt := time.Now()
				fileTimings = append(fileTimings, FileTiming{
					Path:     file.Path,
					Size:     len(file.Content),
					QueueMs:  startedAt.Sub(dispatched).Milliseconds(),
					RunMs:    finishedRunAt.Sub(startedAt).Milliseconds(),
					LockMs:   lockedAt.Sub(finishedRunAt).Milliseconds(),
					AggMs:    endAt.Sub(lockedAt).Milliseconds(),
					TotalMs:  endAt.Sub(dispatched).Milliseconds(),
					Findings: len(fileFindings),
				})
			}
			mu.Unlock()
		}(f, dispatchedAt)
	}
	wg.Wait()

	if err := ctx.Err(); err != nil {
		if ruleTracker != nil {
			ruleTracker.End()
		}
		return DispatchResult{}, err
	}

	// Emit panic diagnostics in the same format as cmd/krit/main.go.
	if len(acc.Errors) > 0 && in.Logger != nil {
		for _, de := range acc.Errors {
			in.Logger("%s\n", de.Error())
		}
		in.Logger("krit: %d rule panic(s) during scan\n", len(acc.Errors))
	}

	// Emit per-family + topDispatchRules timing entries, matching the
	// pre-refactor AddEntry sequence.
	if ruleTracker != nil && in.EmitPerFileStats {
		perf.AddEntry(ruleTracker, "suppressionIndex", time.Duration(acc.SuppressionIndexMs)*time.Millisecond)
		walkTotal := time.Duration(acc.DispatchWalkMs) * time.Millisecond
		ruleCallbackTotal := time.Duration(acc.DispatchRuleNs)
		walkTraversal := walkTotal - ruleCallbackTotal
		if walkTraversal < 0 {
			walkTraversal = 0
		}
		perf.AddEntry(ruleTracker, "walkTraversal", walkTraversal)
		perf.AddEntry(ruleTracker, "ruleCallbacks", ruleCallbackTotal)
		perf.AddEntry(ruleTracker, "aggregateCollect", time.Duration(acc.AggregateCollectNs))
		perf.AddEntry(ruleTracker, "aggregateFinalize", time.Duration(acc.AggregateFinalizeMs)*time.Millisecond)
		perf.AddEntry(ruleTracker, "lineRules", time.Duration(acc.LineRuleMs)*time.Millisecond)
		perf.AddEntry(ruleTracker, "legacyRules", time.Duration(acc.LegacyRuleMs)*time.Millisecond)
		perf.AddEntry(ruleTracker, "suppressionFilter", time.Duration(acc.SuppressionFilterMs)*time.Millisecond)
		if len(acc.DispatchRuleNsByRule) > 0 {
			type timedRule struct {
				name string
				dur  int64
			}
			var topRules []timedRule
			for name, dur := range acc.DispatchRuleNsByRule {
				topRules = append(topRules, timedRule{name: name, dur: dur})
			}
			sort.Slice(topRules, func(i, j int) bool {
				if topRules[i].dur == topRules[j].dur {
					return topRules[i].name < topRules[j].name
				}
				return topRules[i].dur > topRules[j].dur
			})
			if len(topRules) > 10 {
				topRules = topRules[:10]
			}
			topDispatchTracker := ruleTracker.Serial("topDispatchRules")
			for _, tr := range topRules {
				perf.AddEntry(topDispatchTracker, tr.name, time.Duration(tr.dur))
			}
			topDispatchTracker.End()
		}
	}
	if ruleTracker != nil {
		ruleTracker.End()
	}

	// Merge cached findings back in, mirroring main.go.
	if in.CacheResult != nil && in.CacheResult.CachedColumns.Len() > 0 {
		allFindings = append(allFindings, in.CacheResult.CachedColumns.Findings()...)
	}

	// Cache write-back: update per-file entries, set header metadata,
	// prune, and save under a "cacheSave" tracker entry.
	if useCache && in.Version != "" {
		for _, pf := range in.KotlinFiles {
			if in.CacheResult == nil || !in.CacheResult.CachedPaths[pf.Path] {
				fileColumns := findingsByFile[pf.Path]
				in.Cache.UpdateEntryColumns(pf.Path, &fileColumns)
			}
		}
		in.Cache.Version = in.Version
		in.Cache.RuleHash = in.RuleHash
		if len(in.CacheScanPaths) > 0 {
			in.Cache.ScanPaths = in.CacheScanPaths
		}
		in.Cache.Prune()

		var saveStart time.Time
		saveFn := func() error {
			saveStart = time.Now()
			if err := in.Cache.Save(in.CacheFilePath); err != nil {
				if in.Logger != nil {
					in.Logger("warning: Failed to save cache: %v\n", err)
				}
			}
			return nil
		}
		if in.Tracker != nil {
			_ = in.Tracker.Track("cacheSave", saveFn)
		} else {
			_ = saveFn()
		}
		if in.CacheStats != nil {
			in.CacheStats.SaveDurMs = time.Since(saveStart).Milliseconds()
		}
	}

	return DispatchResult{
		IndexResult:    in,
		Findings:       scanner.CollectFindings(allFindings),
		Stats:          acc,
		FileTimings:    fileTimings,
		FindingsByFile: findingsByFile,
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
