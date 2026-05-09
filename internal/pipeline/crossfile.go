package pipeline

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/kaeawc/krit/internal/javafacts"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// CrossFilePhase runs the rule families that cannot be decided from a
// single file: cross-file reference/dead-code rules, rules that see the
// whole parsed-file set, and module-aware rules. After all findings are
// collected it filters them through each finding's target-file
// SuppressionIndex so cross-file findings respect @Suppress just like
// per-file findings.
//
// Android project analysis (manifest/resource/gradle) is deliberately
// out of scope here; it has its own separate provider/dependency
// machinery and stays in the CLI driver until a follow-up refactor.
type CrossFilePhase struct {
	// Workers overrides the cross-file index worker count. Zero =
	// runtime.NumCPU().
	Workers int
}

// Name implements Phase.
func (CrossFilePhase) Name() string { return "crossfile" }

// classifyCrossFileNeeds inspects the active rules and returns whether any
// rule needs an index-backed cross-file pass and/or a parsed-files pass.
func (CrossFilePhase) classifyCrossFileNeeds(activeRules []*api.Rule) (hasIndexBacked, hasParsedFiles bool) {
	for _, r := range activeRules {
		if r == nil {
			continue
		}
		if r.Needs.Has(api.NeedsParsedFiles) {
			hasParsedFiles = true
		} else if r.Needs.Has(api.NeedsCrossFile) {
			hasIndexBacked = true
		}
	}
	return hasIndexBacked, hasParsedFiles
}

// buildOrReuseCodeIndex returns the pre-built CodeIndex when available;
// otherwise it builds a new one when hasIndexBacked is true.
func (p CrossFilePhase) buildOrReuseCodeIndex(in DispatchResult, hasIndexBacked bool) *scanner.CodeIndex {
	if in.CodeIndex != nil || !hasIndexBacked {
		return in.CodeIndex
	}
	workers := p.Workers
	if workers <= 0 {
		workers = len(in.KotlinFiles)
		if workers < 1 {
			workers = 1
		}
	}
	return scanner.BuildIndex(in.KotlinFiles, workers, in.JavaFiles...)
}

// runCrossRuleSet executes serial and concurrent cross-file rules against
// crossCollector, tracking under crossTracker when non-nil.
func (p CrossFilePhase) runCrossRuleSet(ctx context.Context, in DispatchResult, codeIndex *scanner.CodeIndex, parsedFiles []*scanner.File, javaSourceIndex *javafacts.SourceIndex, crossCollector *scanner.FindingCollector, crossTracker perf.Tracker, result *CrossFileResult) {
	ruleTracker := crossTracker
	if ruleTracker != nil {
		ruleTracker = ruleTracker.Serial("crossRules")
	}
	serialRules, concurrentRules := splitConcurrentCrossRules(in.ActiveRules)
	for _, r := range serialRules {
		if ctx.Err() != nil {
			return
		}
		ruleID := r.ID
		call := func() {
			rctx := buildCrossRuleContext(r, codeIndex, parsedFiles, in.Resolver, in.LibraryFacts, javaSourceIndex, crossCollector)
			r.Check(rctx)
		}
		if ruleTracker != nil {
			ruleTracker.TrackVoid(ruleID, call)
		} else {
			call()
		}
	}
	if len(concurrentRules) > 0 && ctx.Err() == nil {
		runConcurrentCrossRules(ctx, concurrentRules, codeIndex, parsedFiles, in.Resolver, in.LibraryFacts, javaSourceIndex, crossCollector, p.Workers, ruleTracker, &result.Stats.Errors)
	}
	if ruleTracker != nil {
		ruleTracker.End()
	}
}

// runCrossPhase runs all index-backed and parsed-files cross-file rules,
// honouring the cross-findings cache when configured.
func (p CrossFilePhase) runCrossPhase(ctx context.Context, in DispatchResult, codeIndex *scanner.CodeIndex, crossCollector *scanner.FindingCollector, crossStart time.Time, result *CrossFileResult) (string, bool, bool) {
	parsedFiles := crossRuleParsedFiles(in.KotlinFiles, in.JavaFiles)
	javaSourceIndex := javaSourceIndexForParsedFiles(parsedFiles)
	crossTracker := in.CrossFileParentTracker

	crossFindingsKey, crossFindingsCacheable := crossFindingsCacheKey(codeIndex, parsedFiles, in.RuleHash)
	var crossFindingsCacheHit bool
	if crossFindingsCacheable && in.CrossFindingsCacheDir != "" {
		if cached, ok := scanner.LoadCrossFindings(in.CrossFindingsCacheDir, crossFindingsKey); ok {
			crossCollector.AppendColumns(&cached)
			crossFindingsCacheHit = true
			if in.Reporter != nil {
				in.Reporter.Verbosef("verbose: Cross-file findings cache: HIT (%d findings)\n", cached.Len())
			}
		}
	}
	if !crossFindingsCacheHit {
		runCrossRules := func() {
			p.runCrossRuleSet(ctx, in, codeIndex, parsedFiles, javaSourceIndex, crossCollector, crossTracker, result)
		}
		if crossTracker != nil {
			crossTracker.TrackVoid("crossRuleExecution", runCrossRules)
		} else {
			runCrossRules()
		}
	}
	if in.Reporter != nil {
		if codeIndex != nil {
			in.Reporter.Verbosef("verbose: Cross-file analysis in %v (indexed %d symbols, %d references from %d kt + %d java files)\n",
				time.Since(crossStart).Round(time.Millisecond), len(codeIndex.Symbols), len(codeIndex.References),
				len(in.KotlinFiles), len(in.JavaFiles))
		} else {
			in.Reporter.Verbosef("verbose: Cross-file analysis in %v (%d kt files, no shared code index needed)\n",
				time.Since(crossStart).Round(time.Millisecond), len(in.KotlinFiles))
		}
	}
	return crossFindingsKey, crossFindingsCacheable, crossFindingsCacheHit
}

// runModuleAwareRules executes pre-built module-aware rules when the graph
// and module index are available.
func (CrossFilePhase) runModuleAwareRules(in DispatchResult, moduleAwareRules []*api.Rule, crossCollector *scanner.FindingCollector) {
	runModuleRules := func() {
		for _, r := range moduleAwareRules {
			rctx := &api.Context{ModuleIndex: in.ModuleIndex, Collector: crossCollector, Rule: r, DefaultConfidence: 0.95}
			r.Check(rctx)
		}
	}
	if in.ModuleParentTracker != nil {
		in.ModuleParentTracker.TrackVoid("moduleRuleExecution", runModuleRules)
	} else {
		runModuleRules()
	}
}

// runOnDemandModuleIndex builds a PerModuleIndex on demand (for callers
// that did not pre-build one) and executes module-aware rules against it.
func (p CrossFilePhase) runOnDemandModuleIndex(ctx context.Context, in DispatchResult, codeIndex *scanner.CodeIndex, crossCollector *scanner.FindingCollector, result *CrossFileResult) error {
	moduleNeeds := rules.CollectModuleAwareNeeds(in.ActiveRules)
	workers := p.Workers
	if workers <= 0 {
		workers = len(in.Graph.Modules)
		if workers < 1 {
			workers = 1
		}
	}
	if moduleNeeds.NeedsDependencies {
		_ = module.ParseAllDependencies(in.Graph)
	}
	pmi := &module.PerModuleIndex{Graph: in.Graph}
	switch {
	case moduleNeeds.NeedsIndex:
		pmi = module.BuildPerModuleIndexWithGlobal(in.Graph, in.SourceFiles(), workers, codeIndex)
	case moduleNeeds.NeedsFiles:
		pmi.ModuleFiles = module.GroupFilesByModule(in.Graph, in.SourceFiles())
	}
	result.ModuleIndex = pmi
	for _, r := range in.ActiveRules {
		if !r.Needs.Has(api.NeedsModuleIndex) {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rctx := &api.Context{ModuleIndex: pmi, Collector: crossCollector, Rule: r, DefaultConfidence: 0.95}
		r.Check(rctx)
	}
	return nil
}

// Run implements Phase.
func (p CrossFilePhase) Run(ctx context.Context, in DispatchResult) (CrossFileResult, error) {
	if err := ctx.Err(); err != nil {
		return CrossFileResult{}, err
	}

	result := CrossFileResult{DispatchResult: in}

	hasIndexBackedCrossFileRule, hasParsedFilesRule := p.classifyCrossFileNeeds(in.ActiveRules)

	codeIndex := p.buildOrReuseCodeIndex(in, hasIndexBackedCrossFileRule)
	if codeIndex != in.CodeIndex && codeIndex != nil {
		result.CodeIndex = codeIndex
	}

	crossCollector := scanner.NewFindingCollector(0)
	p.collectCrossFileFindings(ctx, in, codeIndex, hasIndexBackedCrossFileRule, hasParsedFilesRule, crossCollector, &result)
	p.collectModuleAwareFindings(in, crossCollector)
	if err := p.collectModuleIndexFindings(ctx, in, codeIndex, crossCollector, &result); err != nil {
		return CrossFileResult{}, err
	}
	p.mergeCrossFindings(in, crossCollector, &result)
	return result, nil
}

func (p CrossFilePhase) collectCrossFileFindings(ctx context.Context, in DispatchResult, codeIndex *scanner.CodeIndex, hasIndexBackedCrossFileRule, hasParsedFilesRule bool, crossCollector *scanner.FindingCollector, result *CrossFileResult) {
	if !hasIndexBackedCrossFileRule && !hasParsedFilesRule {
		if in.Reporter != nil {
			in.Reporter.Verbosef("verbose: Skipped cross-file analysis (no active cross-file rules)\n")
		}
		return
	}

	crossStart := time.Now()
	crossFindingsKey, crossFindingsCacheable, crossFindingsCacheHit := p.runCrossPhase(ctx, in, codeIndex, crossCollector, crossStart, result)
	if crossFindingsCacheHit || !crossFindingsCacheable || in.CrossFindingsCacheDir == "" {
		return
	}
	p.saveCrossFindingsCache(in, crossFindingsKey, crossCollector)
}

func (p CrossFilePhase) saveCrossFindingsCache(in DispatchResult, crossFindingsKey string, crossCollector *scanner.FindingCollector) {
	crossOnlyCols := *crossCollector.Columns()
	crossOnlySuppressed := applySuppressionColumns(&crossOnlyCols, in.SourceFiles())
	snapshot := crossOnlySuppressed.Clone()
	if err := scanner.SaveCrossFindings(in.CrossFindingsCacheDir, crossFindingsKey, snapshot); err != nil {
		if in.Reporter != nil {
			in.Reporter.Verbosef("verbose: Cross-file findings cache: save failed: %v\n", err)
		}
		return
	}
	if in.Reporter != nil {
		in.Reporter.Verbosef("verbose: Cross-file findings cache: MISS (saved %d findings)\n", snapshot.Len())
	}
}

func (p CrossFilePhase) collectModuleAwareFindings(in DispatchResult, crossCollector *scanner.FindingCollector) {
	moduleAwareRules := pickModuleAwareV2Rules(in.ActiveRules)
	if in.Graph == nil || len(in.Graph.Modules) == 0 || len(moduleAwareRules) == 0 {
		return
	}
	moduleStart := time.Now()
	p.runModuleAwareRules(in, moduleAwareRules, crossCollector)
	if in.Reporter != nil {
		in.Reporter.Verbosef("verbose: Module-aware analysis in %v\n", time.Since(moduleStart).Round(time.Millisecond))
	}
}

func (p CrossFilePhase) collectModuleIndexFindings(ctx context.Context, in DispatchResult, codeIndex *scanner.CodeIndex, crossCollector *scanner.FindingCollector, result *CrossFileResult) error {
	caps := unionNeeds(in.ActiveRules)
	shouldRunModuleIndex := caps.Has(api.NeedsModuleIndex) &&
		in.Graph != nil && len(in.Graph.Modules) > 0 && in.ModuleIndex == nil
	if !shouldRunModuleIndex {
		return nil
	}
	return p.runOnDemandModuleIndex(ctx, in, codeIndex, crossCollector, result)
}

func (p CrossFilePhase) mergeCrossFindings(in DispatchResult, crossCollector *scanner.FindingCollector, result *CrossFileResult) {
	crossCols := *crossCollector.Columns()
	suppressed := applySuppressionColumns(&crossCols, in.SourceFiles())
	merged := scanner.NewFindingCollector(in.Findings.Len() + suppressed.Len())
	merged.AppendColumns(&in.Findings)
	merged.AppendColumns(&suppressed)
	result.Findings = *merged.Columns()
}

// crossFindingsCacheKey derives the cache key for the cross-rule
// findings cache. Returns (key, cacheable). Caching is only enabled
// when the codeIndex has a non-empty Fingerprint (set by
// scanner.BuildIndexCached) and ruleHash is populated; the parsed-files
// only path skips caching to keep the fingerprint truly cheap.
func crossFindingsCacheKey(codeIndex *scanner.CodeIndex, _ []*scanner.File, ruleHash string) (string, bool) {
	if codeIndex == nil || codeIndex.Fingerprint == "" || ruleHash == "" {
		return "", false
	}
	return scanner.CrossFindingsKey(codeIndex.Fingerprint, ruleHash), true
}

// pickModuleAwareV2Rules returns the v2 rules that need module-aware dispatch.
func pickModuleAwareV2Rules(v2Rules []*api.Rule) []*api.Rule {
	out := make([]*api.Rule, 0, len(v2Rules))
	for _, r := range v2Rules {
		if r != nil && r.Needs.Has(api.NeedsModuleIndex) {
			out = append(out, r)
		}
	}
	return out
}

// applySuppressionColumns drops rows whose target file, line, and
// rule/ruleset are covered by any of the per-file suppression sources:
// @Suppress annotations, config excludes, or inline `// krit:ignore`
// comments. Rows whose target file is not in the parsed-file set pass
// through unchanged (e.g. rows reported against generated XML or Java
// files for which no SuppressionFilter was built).
func applySuppressionColumns(cols *scanner.FindingColumns, files []*scanner.File) scanner.FindingColumns {
	if cols == nil || cols.Len() == 0 {
		return scanner.FindingColumns{}
	}
	byPath := make(map[string]*scanner.File, len(files))
	for _, f := range files {
		byPath[f.Path] = f
	}
	return cols.FilterRows(func(row int) bool {
		file, ok := byPath[cols.FileAt(row)]
		if !ok || file.Suppression == nil {
			return true
		}
		return !file.Suppression.IsSuppressed(cols.RuleAt(row), cols.RuleSetAt(row), cols.LineAt(row))
	})
}

// splitConcurrentCrossRules partitions ActiveRules into those that must
// run serially on the shared collector and those that declared
// NeedsConcurrent and can be executed in parallel with worker-local
// collectors. Only NeedsParsedFiles / NeedsCrossFile rules are eligible
// — other families are skipped at this phase boundary and returned in
// neither slice. Ordering within each slice mirrors ActiveRules so the
// zero-concurrent-rule case is byte-identical to the pre-change loop.
func splitConcurrentCrossRules(active []*api.Rule) (serial, concurrent []*api.Rule) {
	for _, r := range active {
		if r == nil {
			continue
		}
		if !r.Needs.Has(api.NeedsParsedFiles) && !r.Needs.Has(api.NeedsCrossFile) {
			continue
		}
		if r.Needs.Has(api.NeedsConcurrent) {
			concurrent = append(concurrent, r)
			continue
		}
		serial = append(serial, r)
	}
	return serial, concurrent
}

// buildCrossRuleContext produces a Context populated with the cross-file
// inputs a rule declares it needs. Shared between the serial and
// concurrent execution paths so both families see identical Context
// shapes.
func crossRuleParsedFiles(kotlinFiles, javaFiles []*scanner.File) []*scanner.File {
	if len(javaFiles) == 0 {
		return kotlinFiles
	}
	parsedFiles := make([]*scanner.File, 0, len(kotlinFiles)+len(javaFiles))
	parsedFiles = append(parsedFiles, kotlinFiles...)
	parsedFiles = append(parsedFiles, javaFiles...)
	return parsedFiles
}

func javaSourceIndexForParsedFiles(parsedFiles []*scanner.File) *javafacts.SourceIndex {
	for _, file := range parsedFiles {
		if file != nil && file.Language == scanner.LangJava {
			return javafacts.SourceIndexForFiles(parsedFiles)
		}
	}
	return nil
}

func buildCrossRuleContext(r *api.Rule, codeIndex *scanner.CodeIndex, parsedFiles []*scanner.File, resolver typeinfer.TypeResolver, libraryFacts *librarymodel.Facts, javaSourceIndex *javafacts.SourceIndex, collector *scanner.FindingCollector) *api.Context {
	rctx := &api.Context{Collector: collector, Rule: r, DefaultConfidence: 0.95, LibraryFacts: libraryFacts, JavaSourceIndex: javaSourceIndex}
	if r.Needs.Has(api.NeedsResolver) {
		rctx.Resolver = resolver
	}
	if r.Needs.Has(api.NeedsParsedFiles) {
		rctx.ParsedFiles = parsedFiles
	}
	if r.Needs.Has(api.NeedsCrossFile) {
		rctx.CodeIndex = codeIndex
	}
	return rctx
}

// concurrentCrossRuleThreshold is the minimum number of NeedsConcurrent
// rules required before the phase spins up parallel workers. Below this
// threshold the goroutine / merge overhead outweighs the benefit, so we fall
// back to serial execution on the shared collector.
const concurrentCrossRuleThreshold = 2

// runConcurrentCrossRules executes rules in parallel across worker
// goroutines with per-worker collectors merged serially at the end. The
// merge preserves each worker's relative finding order, and the phase
// owner re-sorts the full columnar result by file/line before output
// (see applySuppressionColumns + result.Findings path). Rule panics are
// recovered per-rule so one broken rule does not take down the phase.
func runConcurrentCrossRules(ctx context.Context, ruleSet []*api.Rule, codeIndex *scanner.CodeIndex, parsedFiles []*scanner.File, resolver typeinfer.TypeResolver, libraryFacts *librarymodel.Facts, javaSourceIndex *javafacts.SourceIndex, dst *scanner.FindingCollector, workers int, tracker perf.Tracker, errs *[]rules.DispatchError) {
	if len(ruleSet) == 0 {
		return
	}
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers > len(ruleSet) {
		workers = len(ruleSet)
	}
	if workers < 1 {
		workers = 1
	}
	// Small rule-set threshold: overhead of goroutines + merge exceeds
	// the win below ~2 concurrent rules.
	if len(ruleSet) < concurrentCrossRuleThreshold || workers == 1 {
		for _, r := range ruleSet {
			if err := ctx.Err(); err != nil {
				return
			}
			ruleID := r.ID
			call := func() {
				runConcurrentCrossRule(r, codeIndex, parsedFiles, resolver, libraryFacts, javaSourceIndex, dst, errs)
			}
			if tracker != nil {
				tracker.TrackVoid(ruleID, call)
			} else {
				call()
			}
		}
		return
	}

	locals := make([]*scanner.FindingCollector, workers)
	localErrs := make([][]rules.DispatchError, workers)
	for i := range locals {
		locals[i] = scanner.NewFindingCollector(0)
	}

	jobs := make(chan int, len(ruleSet))
	for i := range ruleSet {
		jobs <- i
	}
	close(jobs)

	// NOTE: long-lived worker-pool with per-worker locals[workerID]
	// FindingCollector + per-worker localErrs[workerID] error slice.
	// Not migrated to errgroup because the per-worker buffer scheme is
	// the whole point — it minimizes lock contention on the cross-rule
	// merge by giving each long-lived worker a distinct collector. An
	// errgroup-style refactor would allocate one collector per Go()
	// call, then merge them all serially after Wait. Refactor candidate
	// only with a before/after benchmark.
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(workerID int) {
			defer wg.Done()
			local := locals[workerID]
			for idx := range jobs {
				if err := ctx.Err(); err != nil {
					return
				}
				r := ruleSet[idx]
				runConcurrentCrossRule(r, codeIndex, parsedFiles, resolver, libraryFacts, javaSourceIndex, local, &localErrs[workerID])
			}
		}(w)
	}
	wg.Wait()

	// Merge serially in worker order so output is a deterministic
	// function of rule set + worker count. Downstream sorting by
	// file/line makes the final JSON row order independent of worker
	// count, satisfying the issue's finding-equivalence requirement.
	scanner.MergeCollectors(dst, locals...)
	if errs != nil {
		*errs = append(*errs, mergeSortedLocalErrs(localErrs)...)
	}
}

// mergeSortedLocalErrs flattens per-worker DispatchError slices and
// sorts the result through the canonical comparator. Worker slot
// ordering is deterministic, but the *content* of each slot reflects
// goroutine completion order (workers pull from a shared jobs
// channel), so a flat append yields a non-deterministic slice across
// runs. Sorting at this seam keeps `runConcurrentCrossRules`'
// returned-errs contract — "canonical order regardless of worker
// schedule" — self-contained. See #29.
func mergeSortedLocalErrs(localErrs [][]rules.DispatchError) []rules.DispatchError {
	var n int
	for _, le := range localErrs {
		n += len(le)
	}
	if n == 0 {
		return nil
	}
	out := make([]rules.DispatchError, 0, n)
	for _, le := range localErrs {
		out = append(out, le...)
	}
	rules.SortDispatchErrors(out)
	return out
}

// runConcurrentCrossRule invokes a single rule's Check against a given
// collector, recovering from panics the same way the serial path does.
// Each caller hands its own collector so the goroutines never contend.
func runConcurrentCrossRule(r *api.Rule, codeIndex *scanner.CodeIndex, parsedFiles []*scanner.File, resolver typeinfer.TypeResolver, libraryFacts *librarymodel.Facts, javaSourceIndex *javafacts.SourceIndex, local *scanner.FindingCollector, errs *[]rules.DispatchError) {
	defer func() {
		if rec := recover(); rec != nil {
			if errs != nil {
				ruleID := ""
				if r != nil {
					ruleID = r.ID
				}
				*errs = append(*errs, rules.DispatchError{RuleName: ruleID, PanicValue: rec})
			}
		}
	}()
	rctx := buildCrossRuleContext(r, codeIndex, parsedFiles, resolver, libraryFacts, javaSourceIndex, local)
	r.Check(rctx)
}

// Compile-time check.
var _ Phase[DispatchResult, CrossFileResult] = CrossFilePhase{}
