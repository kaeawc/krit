package pipeline

import (
	"context"

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

	// Build v1 adapter slice once for interface assertions.
	v1Rules := v2RulesToV1(in.ActiveRules)

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

	if hasIndexBackedCrossFileRule || hasParsedFilesRule {
		for i, r := range v1Rules {
			if err := ctx.Err(); err != nil {
				return CrossFileResult{}, err
			}
			v2Rule := in.ActiveRules[i]
			if pfr, ok := r.(interface {
				CheckParsedFiles(files []*scanner.File) []scanner.Finding
			}); ok {
				found := pfr.CheckParsedFiles(in.KotlinFiles)
				rules.ApplyRuleConfidence(found, r, 0.95)
				_ = v2Rule
				crossFindings = append(crossFindings, found...)
				continue
			}
			if cfr, ok := r.(interface {
				CheckCrossFile(index *scanner.CodeIndex) []scanner.Finding
			}); ok {
				found := cfr.CheckCrossFile(codeIndex)
				rules.ApplyRuleConfidence(found, r, 0.95)
				crossFindings = append(crossFindings, found...)
			}
		}
	}

	// Module-aware rules (v2-native). Mirrors the logic in
	// cmd/krit/main.go for module discovery + PerModuleIndex build +
	// dispatch.
	caps := unionNeeds(in.ActiveRules)
	if caps.Has(v2.NeedsModuleIndex) && in.ModuleGraph != nil && len(in.ModuleGraph.Modules) > 0 {
		moduleNeeds := rules.CollectModuleAwareNeeds(v1Rules)
		workers := p.Workers
		if workers <= 0 {
			workers = len(in.ModuleGraph.Modules)
			if workers < 1 {
				workers = 1
			}
		}

		if moduleNeeds.NeedsDependencies {
			// Best-effort: errors are non-fatal, mirroring main.go.
			_ = module.ParseAllDependencies(in.ModuleGraph)
		}

		pmi := in.ModuleIndex
		if pmi == nil {
			pmi = &module.PerModuleIndex{Graph: in.ModuleGraph}
			switch {
			case moduleNeeds.NeedsIndex:
				pmi = module.BuildPerModuleIndexWithGlobal(in.ModuleGraph, in.KotlinFiles, workers, codeIndex)
			case moduleNeeds.NeedsFiles:
				pmi.ModuleFiles = module.GroupFilesByModule(in.ModuleGraph, in.KotlinFiles)
			}
			result.ModuleIndex = pmi
		}

		for _, r := range in.ActiveRules {
			if !r.Needs.Has(v2.NeedsModuleIndex) {
				continue
			}
			if err := ctx.Err(); err != nil {
				return CrossFileResult{}, err
			}
			rctx := &v2.Context{ModuleIndex: pmi}
			r.Check(rctx)
			rules.ApplyV2Confidence(rctx.Findings, r, 0.95)
			crossFindings = append(crossFindings, rctx.Findings...)
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
