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
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// CrossFilePhase runs the rule families that cannot be decided from a
// single file: cross-file reference/dead-code rules, rules that see the
// whole parsed-file set, and module-aware rules. After all findings are
// collected it filters them through each finding's target-file
// SuppressionIndex so cross-file findings respect @Suppress just like
// per-file findings — closing the pre-refactor suppression gap
// (see roadmap/clusters/core-infra/phase-pipeline.md, acceptance #3).
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

// Run implements Phase.
func (p CrossFilePhase) Run(ctx context.Context, in DispatchResult) (CrossFileResult, error) {
	if err := ctx.Err(); err != nil {
		return CrossFileResult{}, err
	}

	result := CrossFileResult{DispatchResult: in}

	// Detect which cross-file paths any active rule needs.
	var hasIndexBackedCrossFileRule, hasParsedFilesRule bool
	for _, r := range in.ActiveRules {
		if r == nil {
			continue
		}
		if r.Needs.Has(v2.NeedsParsedFiles) {
			hasParsedFilesRule = true
		} else if r.Needs.Has(v2.NeedsCrossFile) {
			hasIndexBackedCrossFileRule = true
		}
	}

	// Build the CodeIndex only when a rule asks for it. Reuse an
	// IndexResult-provided CodeIndex if the caller pre-built one (LSP
	// caches this across edits).
	codeIndex := in.CodeIndex
	if hasIndexBackedCrossFileRule && codeIndex == nil {
		workers := p.Workers
		if workers <= 0 {
			workers = len(in.KotlinFiles)
			if workers < 1 {
				workers = 1
			}
		}
		codeIndex = scanner.BuildIndex(in.KotlinFiles, workers, in.JavaFiles...)
		result.CodeIndex = codeIndex
	}

	// Collect cross-file findings into a single shared columnar collector.
	// Each rule's Context carries DefaultConfidence so Emit stamps the
	// family default on findings that leave Confidence unset.
	crossCollector := scanner.NewFindingCollector(0)

	crossStart := time.Now()
	if hasIndexBackedCrossFileRule || hasParsedFilesRule {
		parsedFiles := crossRuleParsedFiles(in.KotlinFiles, in.JavaFiles)
		javaSourceIndex := javaSourceIndexForParsedFiles(parsedFiles)
		crossTracker := in.CrossFileParentTracker

		// Cross-rule findings cache. Skips runCrossRules entirely when
		// (codeIndex.Fingerprint or parsed-files fingerprint, ruleHash)
		// matches a previous run. Cache miss runs cross-rules and writes
		// the merged column slice back. Any error path falls through to
		// the normal run.
		crossFindingsKey, crossFindingsCacheable := crossFindingsCacheKey(codeIndex, parsedFiles, in.RuleHash)
		var crossFindingsCacheHit bool
		if crossFindingsCacheable && in.CrossFindingsCacheDir != "" {
			if cached, ok := scanner.LoadCrossFindings(in.CrossFindingsCacheDir, crossFindingsKey); ok {
				crossCollector.AppendColumns(&cached)
				crossFindingsCacheHit = true
				if in.Logger != nil {
					in.Logger("verbose: Cross-file findings cache: HIT (%d findings)\n", cached.Len())
				}
			}
		}
		runCrossRules := func() error {
			ruleTracker := crossTracker
			if ruleTracker != nil {
				ruleTracker = ruleTracker.Serial("crossRules")
			}
			serialRules, concurrentRules := splitConcurrentCrossRules(in.ActiveRules)
			for _, r := range serialRules {
				if err := ctx.Err(); err != nil {
					return err
				}
				ruleID := r.ID
				call := func() error {
					rctx := buildCrossRuleContext(r, codeIndex, parsedFiles, in.Resolver, in.LibraryFacts, javaSourceIndex, crossCollector)
					r.Check(rctx)
					return nil
				}
				if ruleTracker != nil {
					_ = ruleTracker.Track(ruleID, call)
				} else {
					_ = call()
				}
			}
			if len(concurrentRules) > 0 {
				if err := ctx.Err(); err != nil {
					return err
				}
				runConcurrentCrossRules(ctx, concurrentRules, codeIndex, parsedFiles, in.Resolver, in.LibraryFacts, javaSourceIndex, crossCollector, p.Workers, ruleTracker)
			}
			if ruleTracker != nil {
				ruleTracker.End()
			}
			return nil
		}
		if !crossFindingsCacheHit {
			if crossTracker != nil {
				_ = crossTracker.Track("crossRuleExecution", runCrossRules)
			} else {
				_ = runCrossRules()
			}
			if crossFindingsCacheable && in.CrossFindingsCacheDir != "" {
				snapshot := crossCollector.Columns().Clone()
				if err := scanner.SaveCrossFindings(in.CrossFindingsCacheDir, crossFindingsKey, snapshot); err != nil {
					if in.Logger != nil {
						in.Logger("verbose: Cross-file findings cache: save failed: %v\n", err)
					}
				} else if in.Logger != nil {
					in.Logger("verbose: Cross-file findings cache: MISS (saved %d findings)\n", snapshot.Len())
				}
			}
		}
		if in.Logger != nil {
			if codeIndex != nil {
				in.Logger("verbose: Cross-file analysis in %v (indexed %d symbols, %d references from %d kt + %d java files)\n",
					time.Since(crossStart).Round(time.Millisecond), len(codeIndex.Symbols), len(codeIndex.References),
					len(in.KotlinFiles), len(in.JavaFiles))
			} else {
				in.Logger("verbose: Cross-file analysis in %v (%d kt files, no shared code index needed)\n",
					time.Since(crossStart).Round(time.Millisecond), len(in.KotlinFiles))
			}
		}
	} else if in.Logger != nil {
		in.Logger("verbose: Skipped cross-file analysis (no active cross-file rules)\n")
	}

	// Module-aware rule execution. The phase derives the same module-aware
	// v2 rule set as the CLI dispatcher path.
	moduleStart := time.Now()
	moduleAwareRules := pickModuleAwareV2Rules(in.ActiveRules)
	hasModuleAwareRule := len(moduleAwareRules) > 0
	if in.ModuleGraph != nil && len(in.ModuleGraph.Modules) > 0 && hasModuleAwareRule {
		runModuleRules := func() error {
			for _, r := range moduleAwareRules {
				rctx := &v2.Context{ModuleIndex: in.ModuleIndex, Collector: crossCollector, Rule: r, DefaultConfidence: 0.95}
				r.Check(rctx)
			}
			return nil
		}
		if in.ModuleParentTracker != nil {
			_ = in.ModuleParentTracker.Track("moduleRuleExecution", runModuleRules)
		} else {
			_ = runModuleRules()
		}
		if in.Logger != nil {
			in.Logger("verbose: Module-aware analysis in %v\n", time.Since(moduleStart).Round(time.Millisecond))
		}
	}

	// On-demand PerModuleIndex build for callers that did not pre-build
	// one (e.g. tests or LSP paths that only supplied a ModuleGraph).
	// Skipped when in.ModuleIndex is already populated, which is the
	// main.go path.
	caps := unionNeeds(in.ActiveRules)
	if caps.Has(v2.NeedsModuleIndex) && in.ModuleGraph != nil && len(in.ModuleGraph.Modules) > 0 && in.ModuleIndex == nil {
		moduleNeeds := rules.CollectModuleAwareNeedsV2(in.ActiveRules)
		workers := p.Workers
		if workers <= 0 {
			workers = len(in.ModuleGraph.Modules)
			if workers < 1 {
				workers = 1
			}
		}
		if moduleNeeds.NeedsDependencies {
			_ = module.ParseAllDependencies(in.ModuleGraph)
		}
		pmi := &module.PerModuleIndex{Graph: in.ModuleGraph}
		switch {
		case moduleNeeds.NeedsIndex:
			pmi = module.BuildPerModuleIndexWithGlobal(in.ModuleGraph, in.SourceFiles(), workers, codeIndex)
		case moduleNeeds.NeedsFiles:
			pmi.ModuleFiles = module.GroupFilesByModule(in.ModuleGraph, in.SourceFiles())
		}
		result.ModuleIndex = pmi
		for _, r := range in.ActiveRules {
			if !r.Needs.Has(v2.NeedsModuleIndex) {
				continue
			}
			if err := ctx.Err(); err != nil {
				return CrossFileResult{}, err
			}
			rctx := &v2.Context{ModuleIndex: pmi, Collector: crossCollector, Rule: r, DefaultConfidence: 0.95}
			r.Check(rctx)
		}
	}

	// Unified suppression: every cross-file finding flows through the same
	// SuppressionIndex that per-file dispatch already honours.
	crossCols := *crossCollector.Columns()
	suppressed := applySuppressionColumns(&crossCols, in.SourceFiles())

	// Merge pre-file findings with suppressed cross-file findings in columnar form.
	merged := scanner.NewFindingCollector(in.Findings.Len() + suppressed.Len())
	merged.AppendColumns(&in.Findings)
	merged.AppendColumns(&suppressed)
	result.Findings = *merged.Columns()

	return result, nil
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
func pickModuleAwareV2Rules(v2Rules []*v2.Rule) []*v2.Rule {
	out := make([]*v2.Rule, 0, len(v2Rules))
	for _, r := range v2Rules {
		if r != nil && r.Needs.Has(v2.NeedsModuleIndex) {
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
func splitConcurrentCrossRules(active []*v2.Rule) (serial, concurrent []*v2.Rule) {
	for _, r := range active {
		if r == nil {
			continue
		}
		if !r.Needs.Has(v2.NeedsParsedFiles) && !r.Needs.Has(v2.NeedsCrossFile) {
			continue
		}
		if r.Needs.Has(v2.NeedsConcurrent) {
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

func buildCrossRuleContext(r *v2.Rule, codeIndex *scanner.CodeIndex, parsedFiles []*scanner.File, resolver typeinfer.TypeResolver, libraryFacts *librarymodel.Facts, javaSourceIndex *javafacts.SourceIndex, collector *scanner.FindingCollector) *v2.Context {
	rctx := &v2.Context{Collector: collector, Rule: r, DefaultConfidence: 0.95, LibraryFacts: libraryFacts, JavaSourceIndex: javaSourceIndex}
	if r.Needs.Has(v2.NeedsResolver) {
		rctx.Resolver = resolver
	}
	if r.Needs.Has(v2.NeedsParsedFiles) {
		rctx.ParsedFiles = parsedFiles
	}
	if r.Needs.Has(v2.NeedsCrossFile) {
		rctx.CodeIndex = codeIndex
	}
	return rctx
}

// concurrentCrossRuleThreshold is the minimum number of NeedsConcurrent
// rules required before the phase spins up parallel workers. Below this
// threshold the goroutine / merge overhead outweighs the wall-time win,
// so we fall back to serial execution on the shared collector.
const concurrentCrossRuleThreshold = 2

// runConcurrentCrossRules executes rules in parallel across worker
// goroutines with per-worker collectors merged serially at the end. The
// merge preserves each worker's relative finding order, and the phase
// owner re-sorts the full columnar result by file/line before output
// (see applySuppressionColumns + result.Findings path). Rule panics are
// recovered per-rule so one broken rule does not take down the phase.
func runConcurrentCrossRules(ctx context.Context, rules []*v2.Rule, codeIndex *scanner.CodeIndex, parsedFiles []*scanner.File, resolver typeinfer.TypeResolver, libraryFacts *librarymodel.Facts, javaSourceIndex *javafacts.SourceIndex, dst *scanner.FindingCollector, workers int, tracker perf.Tracker) {
	if len(rules) == 0 {
		return
	}
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers > len(rules) {
		workers = len(rules)
	}
	if workers < 1 {
		workers = 1
	}
	// Small rule-set threshold: overhead of goroutines + merge exceeds
	// the win below ~2 concurrent rules.
	if len(rules) < concurrentCrossRuleThreshold || workers == 1 {
		for _, r := range rules {
			if err := ctx.Err(); err != nil {
				return
			}
			ruleID := r.ID
			call := func() error {
				runConcurrentCrossRule(r, codeIndex, parsedFiles, resolver, libraryFacts, javaSourceIndex, dst)
				return nil
			}
			if tracker != nil {
				_ = tracker.Track(ruleID, call)
			} else {
				_ = call()
			}
		}
		return
	}

	locals := make([]*scanner.FindingCollector, workers)
	for i := range locals {
		locals[i] = scanner.NewFindingCollector(0)
	}

	jobs := make(chan int, len(rules))
	for i := range rules {
		jobs <- i
	}
	close(jobs)

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
				r := rules[idx]
				runConcurrentCrossRule(r, codeIndex, parsedFiles, resolver, libraryFacts, javaSourceIndex, local)
			}
		}(w)
	}
	wg.Wait()

	// Merge serially in worker order so output is a deterministic
	// function of rule set + worker count. Downstream sorting by
	// file/line makes the final JSON row order independent of worker
	// count, satisfying the issue's finding-equivalence requirement.
	scanner.MergeCollectors(dst, locals...)
}

// runConcurrentCrossRule invokes a single rule's Check against a given
// collector, recovering from panics the same way the serial path does.
// Each caller hands its own collector so the goroutines never contend.
func runConcurrentCrossRule(r *v2.Rule, codeIndex *scanner.CodeIndex, parsedFiles []*scanner.File, resolver typeinfer.TypeResolver, libraryFacts *librarymodel.Facts, javaSourceIndex *javafacts.SourceIndex, local *scanner.FindingCollector) {
	defer func() { _ = recover() }()
	rctx := buildCrossRuleContext(r, codeIndex, parsedFiles, resolver, libraryFacts, javaSourceIndex, local)
	r.Check(rctx)
}

// Compile-time check.
var _ Phase[DispatchResult, CrossFileResult] = CrossFilePhase{}
