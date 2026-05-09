package pipeline

import (
	"context"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
	"golang.org/x/sync/errgroup"
)

// DispatchPhase runs per-file rule dispatch in parallel. One AST walk
// per file dispatches every active per-file rule through the existing
// Dispatcher (by way of rules.NewDispatcher). The dispatcher
// internally applies each file's SuppressionIndex, so the Findings
// returned here are already suppression-filtered.
type DispatchPhase struct {
	// Workers overrides the dispatch worker count. Zero =
	// runtime.NumCPU().
	Workers int
}

// Name returns the stable phase identifier used for timing and error tags.
func (DispatchPhase) Name() string { return "dispatch" }

// dispatchWorkerCount returns the effective number of parallel workers for
// the dispatch loop.
func (d DispatchPhase) dispatchWorkerCount(in IndexResult) int {
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
	return workers
}

// emitPanicDiagnostics reports any rule panics that occurred during dispatch.
//
// Errors are sorted before emission so that warning output has a stable
// ordering across runs regardless of which goroutine recovered each
// panic first — see #28.
func (DispatchPhase) emitPanicDiagnostics(in IndexResult, acc rules.RunStats) {
	if len(acc.Errors) == 0 {
		return
	}
	rules.SortDispatchErrors(acc.Errors)
	for _, de := range acc.Errors {
		in.Reporter.Warnf("%s\n", de.Error())
	}
	in.Reporter.Warnf("krit: %d rule panic(s) during scan\n", len(acc.Errors))
}

// emitRuleTimings emits per-family and top-rule timing entries under
// ruleTracker when EmitPerFileStats is set.
func (DispatchPhase) emitRuleTimings(in IndexResult, ruleTracker perf.Tracker, acc rules.RunStats) {
	if ruleTracker == nil || !in.EmitPerFileStats {
		return
	}
	perf.AddEntry(ruleTracker, "suppressionIndex", time.Duration(acc.SuppressionIndexMs)*time.Millisecond)
	ruleCallbackTotal := time.Duration(acc.DispatchRuleNs)
	perf.AddEntry(ruleTracker, "ruleCallbacks", ruleCallbackTotal)
	perf.AddEntry(ruleTracker, "aggregateCollect", time.Duration(acc.AggregateCollectNs))
	perf.AddEntry(ruleTracker, "aggregateFinalize", time.Duration(acc.AggregateFinalizeMs)*time.Millisecond)
	perf.AddEntry(ruleTracker, "lineRules", time.Duration(acc.LineRuleMs)*time.Millisecond)
	perf.AddEntry(ruleTracker, "suppressionFilter", time.Duration(acc.SuppressionFilterMs)*time.Millisecond)
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

// mergeCachedFindings appends any cache-hit findings to allColumns.
func (DispatchPhase) mergeCachedFindings(in IndexResult, allColumns scanner.FindingColumns) scanner.FindingColumns {
	if in.CacheResult == nil || in.CacheResult.CachedColumns.Len() == 0 {
		return allColumns
	}
	cachedCols := in.CacheResult.CachedColumns
	collector := scanner.NewFindingCollector(allColumns.Len() + cachedCols.Len())
	collector.AppendColumns(&allColumns)
	collector.AppendColumns(&cachedCols)
	return *collector.Columns()
}

// writeCacheBack updates per-file cache entries, prunes, and persists the
// cache to disk under a "cacheSave" tracker entry.
func (DispatchPhase) writeCacheBack(in IndexResult, findingsByFile map[string]scanner.FindingColumns) {
	if canSkipCacheSave(in) {
		if in.Tracker != nil {
			in.Tracker.TrackVoid("cacheSave", func() {})
		}
		if in.CacheStats != nil {
			in.CacheStats.SaveDurMs = 0
		}
		return
	}
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
	if in.Cache.ShouldSkipFullSaveForSmallDelta(in.CacheResult, 4) {
		if in.Tracker != nil {
			in.Tracker.TrackVoid("cacheSave", func() {})
		}
		if in.CacheStats != nil {
			in.CacheStats.SaveDurMs = 0
		}
		return
	}

	var saveStart time.Time
	saveFn := func() {
		saveStart = time.Now()
		if err := in.Cache.Save(in.CacheFilePath); err != nil {
			in.Reporter.Warnf("warning: Failed to save cache: %v\n", err)
		}
	}
	if in.Tracker != nil {
		in.Tracker.TrackVoid("cacheSave", saveFn)
	} else {
		saveFn()
	}
	if in.CacheStats != nil {
		in.CacheStats.SaveDurMs = time.Since(saveStart).Milliseconds()
	}
}

func canSkipCacheSave(in IndexResult) bool {
	if in.Cache == nil || in.CacheResult == nil {
		return false
	}
	if in.CacheResult.TotalFiles == 0 || in.CacheResult.TotalCached != in.CacheResult.TotalFiles {
		return false
	}
	if len(in.SourceFiles()) != 0 {
		return false
	}
	return true
}

// Run executes the Dispatch phase. It walks every non-cached source file in
// in.KotlinFiles and in.JavaFiles in parallel, dispatching per-file rules through the
// shared dispatcher, and accumulates the findings and run statistics.
//
// It creates the dispatcher from in.ActiveRules ([]*api.Rule) via
// NewDispatcher. When in.Tracker is non-nil it wraps the dispatch loop in a
// "ruleExecution" serial child and emits per-family timing entries plus a
// topDispatchRules breakdown. When in.UseCache / Cache / Version are set, the
// phase updates and saves the cache under a "cacheSave" tracker entry.
func (d DispatchPhase) Run(ctx context.Context, in IndexResult) (DispatchResult, error) {
	if err := ctx.Err(); err != nil {
		return DispatchResult{}, err
	}

	dispatcher := rules.NewDispatcher(in.ActiveRules, in.Resolver)
	dispatcher.SetLibraryFacts(in.LibraryFacts)
	dispatcher.SetJavaSemanticFacts(in.JavaSemanticFacts)

	if in.Reporter.VerboseEnabled() {
		dispatcher.ReportMissingCapabilities(dispatchOracleAvailable(in), in.Reporter.Verbosef)
	}

	workers := d.dispatchWorkerCount(in)
	useCache := in.Cache != nil && in.CacheFilePath != ""
	findingsByFile := map[string]scanner.FindingColumns{}

	var (
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

	g, _ := errgroup.WithContext(ctx)
	g.SetLimit(workers)
	sourceFiles := in.SourceFiles()
	sort.SliceStable(sourceFiles, func(i, j int) bool {
		return len(sourceFiles[i].Content) > len(sourceFiles[j].Content)
	})
	for _, f := range sourceFiles {
		if in.CacheResult != nil && in.CacheResult.CachedPaths[f.Path] {
			continue
		}
		var dispatchedAt time.Time
		if in.ProfileDispatch {
			dispatchedAt = time.Now()
		}
		file, dispatched := f, dispatchedAt
		g.Go(func() error {

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
			return nil
		})
	}
	_ = g.Wait()

	if err := ctx.Err(); err != nil {
		if ruleTracker != nil {
			ruleTracker.End()
		}
		return DispatchResult{}, err
	}

	d.emitPanicDiagnostics(in, acc)
	d.emitRuleTimings(in, ruleTracker, acc)
	if ruleTracker != nil {
		ruleTracker.End()
	}

	allColumns = d.mergeCachedFindings(in, allColumns)

	if useCache && in.Version != "" {
		d.writeCacheBack(in, findingsByFile)
	}

	return DispatchResult{
		IndexResult:    in,
		Findings:       allColumns,
		Stats:          acc,
		FileTimings:    fileTimings,
		FindingsByFile: findingsByFile,
	}, nil
}

type oracleLookupProvider interface {
	Oracle() oracle.Lookup
}

func dispatchOracleAvailable(in IndexResult) bool {
	if in.Oracle != nil {
		return true
	}
	if provider, ok := in.Resolver.(oracleLookupProvider); ok {
		return provider.Oracle() != nil
	}
	return false
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
