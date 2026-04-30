package pipeline

import (
	"context"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/rules"
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

// Run executes the Dispatch phase. It walks every non-cached source file in
// in.KotlinFiles and in.JavaFiles in parallel, dispatching per-file rules through the
// shared dispatcher, and accumulates the findings and run statistics.
//
// It creates the dispatcher from in.ActiveRules ([]*v2.Rule) via
// NewDispatcherV2. When in.Tracker is non-nil it wraps the dispatch loop in a
// "ruleExecution" serial child and emits per-family timing entries plus a
// topDispatchRules breakdown. When in.UseCache / Cache / Version are set, the
// phase updates and saves the cache under a "cacheSave" tracker entry.
func (d DispatchPhase) Run(ctx context.Context, in IndexResult) (DispatchResult, error) {
	if err := ctx.Err(); err != nil {
		return DispatchResult{}, err
	}

	// Pass the resolver through unconditionally — NewDispatcherV2 handles a
	// nil resolver gracefully.
	dispatcher := rules.NewDispatcherV2(in.ActiveRules, in.Resolver)
	dispatcher.SetLibraryFacts(in.LibraryFacts)
	dispatcher.SetJavaSemanticFacts(in.JavaSemanticFacts)

	// Emit --verbose diagnostics naming any active rule whose declared
	// capability (NeedsResolver / NeedsOracle) is not satisfied by the
	// dispatcher wiring. No-op when in.Logger is nil. Emitted once per
	// run at dispatcher startup (sync.Once inside the dispatcher), not
	// per-file — avoids log volume explosion in the hot loop.
	if in.Logger != nil {
		dispatcher.ReportMissingCapabilities(in.Oracle != nil, in.Logger)
	}

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
		allColumns  scanner.FindingColumns
		fileTimings []FileTiming
		acc         = rules.RunStats{
			DispatchRuleNsByRule: make(map[string]int64),
			RuleStatsByRule:      make(map[string]rules.RuleExecutionStat),
		}
	)

	ruleTracker := perf.Tracker(nil)
	if in.Tracker != nil {
		ruleTracker = in.Tracker.Serial("ruleExecution")
	}

	sem := make(chan struct{}, workers)
	sourceFiles := in.SourceFiles()
	sort.SliceStable(sourceFiles, func(i, j int) bool {
		return len(sourceFiles[i].Content) > len(sourceFiles[j].Content)
	})
	for _, f := range sourceFiles {
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

			fileColumns, fileStats := dispatcher.RunWithStats(file)

			if in.ProfileDispatch {
				finishedRunAt = time.Now()
			}

			mu.Lock()
			if in.ProfileDispatch {
				lockedAt = time.Now()
			}
			if fileColumns.Len() > 0 {
				collector := scanner.NewFindingCollector(allColumns.Len() + fileColumns.Len())
				collector.AppendColumns(&allColumns)
				collector.AppendColumns(&fileColumns)
				allColumns = *collector.Columns()
			}
			if useCache {
				findingsByFile[file.Path] = fileColumns
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
					Findings: fileColumns.Len(),
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
		if len(acc.RuleStatsByRule) > 0 {
			topRules := rules.SortedRuleExecutionStats(acc)
			if len(topRules) > 20 {
				topRules = topRules[:20]
			}
			topRuleTracker := ruleTracker.Serial("topRuleExecution")
			for _, tr := range topRules {
				perf.AddEntry(topRuleTracker, tr.Rule, time.Duration(tr.DurationNs))
			}
			topRuleTracker.End()
		}
	}
	if ruleTracker != nil {
		ruleTracker.End()
	}

	// Merge cached findings back in, mirroring main.go.
	if in.CacheResult != nil && in.CacheResult.CachedColumns.Len() > 0 {
		cachedCols := in.CacheResult.CachedColumns
		collector := scanner.NewFindingCollector(allColumns.Len() + cachedCols.Len())
		collector.AppendColumns(&allColumns)
		collector.AppendColumns(&cachedCols)
		allColumns = *collector.Columns()
	}

	// Cache write-back: update per-file entries, set header metadata,
	// prune, and save under a "cacheSave" tracker entry.
	if useCache && in.Version != "" {
		for _, pf := range in.SourceFiles() {
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
		Findings:       allColumns,
		Stats:          acc,
		FileTimings:    fileTimings,
		FindingsByFile: findingsByFile,
	}, nil
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
	dst.SuppressionFilterMs += src.SuppressionFilterMs
	if src.DispatchRuleNsByRule != nil {
		if dst.DispatchRuleNsByRule == nil {
			dst.DispatchRuleNsByRule = make(map[string]int64, len(src.DispatchRuleNsByRule))
		}
		for name, dur := range src.DispatchRuleNsByRule {
			dst.DispatchRuleNsByRule[name] += dur
		}
	}
	if src.RuleStatsByRule != nil {
		if dst.RuleStatsByRule == nil {
			dst.RuleStatsByRule = make(map[string]rules.RuleExecutionStat, len(src.RuleStatsByRule))
		}
		for name, stat := range src.RuleStatsByRule {
			existing := dst.RuleStatsByRule[name]
			if existing.Rule == "" {
				existing.Rule = stat.Rule
			}
			if existing.Family == "" {
				existing.Family = stat.Family
			}
			existing.Invocations += stat.Invocations
			existing.DurationNs += stat.DurationNs
			dst.RuleStatsByRule[name] = existing
		}
	}
	if len(src.Errors) > 0 {
		dst.Errors = append(dst.Errors, src.Errors...)
	}
}

// Compile-time check: DispatchPhase satisfies Phase[IndexResult, DispatchResult].
var _ Phase[IndexResult, DispatchResult] = DispatchPhase{}
