package pipeline

import (
	"context"
	"time"

	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/rules"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
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

	// Build v1 adapter slice once for interface assertions. Prefer the
	// explicitly-supplied v1 rule slice so main.go's rule set is dispatched
	// unchanged (the v2→v1 conversion drops V1CrossFile etc. wrappers that
	// aren't in main.go's registry anyway, but defending against a nil slice
	// here makes the phase drop-in-compatible with both callers).
	v1Rules := in.ActiveRulesV1
	if v1Rules == nil {
		v1Rules = v2RulesToV1(in.ActiveRules)
	}

	// Detect which cross-file paths any active rule needs.
	var hasIndexBackedCrossFileRule, hasParsedFilesRule bool
	for _, r := range v1Rules {
		if _, ok := r.(interface {
			CheckParsedFiles(files []*scanner.File) []scanner.Finding
		}); ok {
			hasParsedFilesRule = true
			continue
		}
		if _, ok := r.(interface {
			CheckCrossFile(index *scanner.CodeIndex) []scanner.Finding
		}); ok {
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

	// Collect cross-file findings in a single slice so we can apply
	// suppression uniformly at the end.
	var crossFindings []scanner.Finding

	crossStart := time.Now()
	if hasIndexBackedCrossFileRule || hasParsedFilesRule {
		crossTracker := in.CrossFileParentTracker
		runCrossRules := func() error {
			ruleTracker := crossTracker
			if ruleTracker != nil {
				ruleTracker = ruleTracker.Serial("crossRules")
			}
			for _, r := range v1Rules {
				if err := ctx.Err(); err != nil {
					return err
				}
				if pfr, ok := r.(interface {
					CheckParsedFiles(files []*scanner.File) []scanner.Finding
				}); ok {
					ruleName := r.Name()
					call := func() error {
						found := pfr.CheckParsedFiles(in.KotlinFiles)
						rules.ApplyRuleConfidence(found, r, 0.95)
						crossFindings = append(crossFindings, found...)
						return nil
					}
					if ruleTracker != nil {
						_ = ruleTracker.Track(ruleName, call)
					} else {
						_ = call()
					}
					continue
				}
				if cfr, ok := r.(interface {
					CheckCrossFile(index *scanner.CodeIndex) []scanner.Finding
				}); ok {
					ruleName := r.Name()
					call := func() error {
						found := cfr.CheckCrossFile(codeIndex)
						rules.ApplyRuleConfidence(found, r, 0.95)
						crossFindings = append(crossFindings, found...)
						return nil
					}
					if ruleTracker != nil {
						_ = ruleTracker.Track(ruleName, call)
					} else {
						_ = call()
					}
				}
			}
			if ruleTracker != nil {
				ruleTracker.End()
			}
			return nil
		}
		if crossTracker != nil {
			_ = crossTracker.Track("crossRuleExecution", runCrossRules)
		} else {
			_ = runCrossRules()
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

	// Module-aware rule execution. Main.go iterates
	// dispatcher.V2Rules().ModuleAware, which BuildV2Index derives from the
	// v1 rule slice. We reproduce that shape here so the phase produces the
	// same rule set regardless of whether the caller supplied v1 or v2.
	moduleStart := time.Now()
	moduleAwareRules := pickModuleAwareV2Rules(in.ActiveRules, v1Rules)
	hasModuleAwareRule := len(moduleAwareRules) > 0
	if in.ModuleGraph != nil && len(in.ModuleGraph.Modules) > 0 && hasModuleAwareRule {
		runModuleRules := func() error {
			for _, r := range moduleAwareRules {
				collector := scanner.NewFindingCollector(0)
				rctx := &v2.Context{ModuleIndex: in.ModuleIndex, Collector: collector}
				r.Check(rctx)
				if len(rctx.Findings) > 0 {
					rules.ApplyV2Confidence(rctx.Findings, r, 0.95)
					crossFindings = append(crossFindings, rctx.Findings...)
				}
				cols := *collector.Columns()
				for i := 0; i < cols.Len(); i++ {
					f := cols.Finding(i)
					if f.Confidence == 0 {
						f.Confidence = r.Confidence
						if f.Confidence == 0 {
							f.Confidence = 0.95
						}
					}
					crossFindings = append(crossFindings, f)
				}
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
		moduleNeeds := rules.CollectModuleAwareNeeds(v1Rules)
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
			pmi = module.BuildPerModuleIndexWithGlobal(in.ModuleGraph, in.KotlinFiles, workers, codeIndex)
		case moduleNeeds.NeedsFiles:
			pmi.ModuleFiles = module.GroupFilesByModule(in.ModuleGraph, in.KotlinFiles)
		}
		result.ModuleIndex = pmi
		for _, r := range in.ActiveRules {
			if !r.Needs.Has(v2.NeedsModuleIndex) {
				continue
			}
			if err := ctx.Err(); err != nil {
				return CrossFileResult{}, err
			}
			collector := scanner.NewFindingCollector(0)
			rctx := &v2.Context{ModuleIndex: pmi, Collector: collector}
			r.Check(rctx)
			if len(rctx.Findings) > 0 {
				rules.ApplyV2Confidence(rctx.Findings, r, 0.95)
				crossFindings = append(crossFindings, rctx.Findings...)
			}
			cols := *collector.Columns()
			for i := 0; i < cols.Len(); i++ {
				f := cols.Finding(i)
				if f.Confidence == 0 {
					f.Confidence = r.Confidence
					if f.Confidence == 0 {
						f.Confidence = 0.95
					}
				}
				crossFindings = append(crossFindings, f)
			}
		}
	}

	// Unified suppression. This is the behaviour change: every cross-file
	// finding now flows through the same SuppressionIndex that per-file
	// dispatch already honours.
	crossFindings = ApplySuppression(crossFindings, in.KotlinFiles)

	// Merge pre-file findings with cross-file findings.
	existing := in.Findings.Findings()
	merged := append(existing, crossFindings...)
	result.Findings = scanner.CollectFindings(merged)

	return result, nil
}

// pickModuleAwareV2Rules returns the v2 rules that should receive
// module-aware dispatch. Main.go's legacy path derives this from
// BuildV2Index(v1Rules).ModuleAware, but when the caller only supplied
// v2 rules (LSP / tests) we fall back to Needs-based filtering.
func pickModuleAwareV2Rules(v2Rules []*v2.Rule, v1Rules []rules.Rule) []*v2.Rule {
	if len(v1Rules) > 0 {
		idx := rules.BuildV2Index(v1Rules)
		return idx.ModuleAware
	}
	out := make([]*v2.Rule, 0, len(v2Rules))
	for _, r := range v2Rules {
		if r == nil {
			continue
		}
		if r.Needs.Has(v2.NeedsModuleIndex) {
			out = append(out, r)
		}
	}
	return out
}

// ApplySuppression drops findings whose target file, line, and rule/ruleset
// are covered by a @Suppress annotation visible at that byte offset.
// Findings whose target file is not in the parsed-file set pass through
// unchanged (e.g. findings reported against generated XML or Java files
// for which no SuppressionIndex was built).
//
// Exported so callers that invoke cross-file rules outside the pipeline
// (transitional CLI code in cmd/krit/main.go) can use the same
// suppression path as the per-file dispatcher — closing the gap where
// cross-file findings bypassed @Suppress.
func ApplySuppression(findings []scanner.Finding, files []*scanner.File) []scanner.Finding {
	if len(findings) == 0 {
		return findings
	}
	byPath := make(map[string]*scanner.File, len(files))
	for _, f := range files {
		byPath[f.Path] = f
	}
	kept := make([]scanner.Finding, 0, len(findings))
	for _, f := range findings {
		file, ok := byPath[f.File]
		if !ok || file.SuppressionIdx == nil {
			kept = append(kept, f)
			continue
		}
		byteOffset := 0
		if f.Line > 0 {
			byteOffset = file.LineOffset(f.Line - 1)
		}
		if !file.SuppressionIdx.IsSuppressed(byteOffset, f.Rule, f.RuleSet) {
			kept = append(kept, f)
		}
	}
	return kept
}

// Compile-time check.
var _ Phase[DispatchResult, CrossFileResult] = CrossFilePhase{}
