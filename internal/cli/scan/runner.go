package scan

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/kaeawc/krit/internal/experiment"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/pipeline"
	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func (r *runner) runProjectAnalysis() (int, error) {
	r.ruleStart = r.start
	analysis, err := pipeline.RunProjectAnalysis(context.Background(), r.projectInput())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2, err
	}
	r.applyProjectAnalysis(analysis)
	return 0, nil
}

func (r *runner) projectInput() pipeline.ProjectInput {
	host := pipeline.ProjectHostState{
		Reporter:                r.reporter,
		Tracker:                 r.tracker,
		ParseCache:              r.parseCache,
		PrebuiltResolver:        r.resolver,
		PrebuiltLibraryFacts:    r.libraryFacts,
		PrebuiltAndroidProject:  r.androidProject,
		JavaSemanticFacts:       r.javaSemanticFacts,
		JavaSemanticFactsLoader: runJavaSemanticFacts,
		CrossFileCacheDir:       resolveCrossFileCacheDir(r.paths, *r.f.NoCrossFileCache),
		CrossFindingsCacheDir:   resolveCrossFindingsCacheDir(r.paths, *r.f.NoCrossFileCache),
		AnalysisCache:           r.analysisCache,
		AnalysisCacheFilePath:   r.cacheFilePath,
		AnalysisCacheLookup:     r.useCache && r.analysisCache != nil,
		AnalysisCacheResult:     r.cacheResult,
		AnalysisCacheStats:      r.cacheStats,
		AnalysisCacheRuleHash:   r.ruleHash,
		Oracle:                  r.typeOracle,
		OracleDaemon:            r.daemon,
		AndroidProviders:        r.androidProviders,
		AndroidCacheDir:         r.androidCacheDir,
		AndroidCacheWriter:      r.androidCacheWriter,
	}
	if r.useCache {
		host.TypeIndexCacheDir = typeinfer.TypeIndexCacheDir(oracle.FindRepoDir(r.paths))
	}
	return pipeline.ProjectInput{
		Args: pipeline.ProjectArgs{
			Config:              r.cfg,
			Paths:               r.paths,
			KotlinPaths:         r.files,
			JavaPaths:           r.allJavaPaths,
			ActiveRules:         r.activeRules,
			Format:              r.effectiveFormat,
			IncludeGenerated:    *r.f.IncludeGenerated,
			EditorConfigEnabled: *r.f.EditorConfig,
			Workers:             *r.f.Jobs,
			StartTime:           r.start,
			Version:             Version,
			ExperimentNames:     experiment.Current().Names(),
			JSONCompact:         *r.f.Output != "",
			OracleEnabled:       false,
			TargetedResolution:  r.depthPreset == DepthThorough,
			ProfileDispatch:     *r.f.ProfileDispatch,
			EmitPerFileStats:    true,
		},
		Host: host,
	}
}

func (r *runner) applyProjectAnalysis(analysis pipeline.ProjectAnalysisResult) {
	r.parseResult = analysis.ParseResult
	r.parsedFiles = analysis.ParseResult.KotlinFiles
	r.sourceFiles = analysis.ParseResult.SourceFiles()
	r.javaSemanticFacts = analysis.IndexResult.JavaSemanticFacts
	r.outputJavaFiles = analysis.ParseResult.JavaFiles
	if len(analysis.IndexResult.JavaFiles) > 0 {
		r.outputJavaFiles = analysis.IndexResult.JavaFiles
	}
	r.moduleGraph = analysis.IndexResult.Graph
	r.pmi = analysis.IndexResult.ModuleIndex
	r.dispatchResult = analysis.DispatchResult
	r.cacheStats = analysis.IndexResult.CacheStats
	r.cacheResult = analysis.IndexResult.CacheResult
	r.analysisCache = analysis.IndexResult.Cache
	r.ruleHash = analysis.IndexResult.RuleHash
	if *r.f.PerfRules {
		r.perfRuleStats = rules.SortedRuleExecutionStats(analysis.DispatchResult.Stats)
		if !*r.f.Quiet {
			reportRuleExecutionRanking(os.Stderr, r.perfRuleStats, 20)
		}
	}
	if *r.f.ProfileDispatch && len(analysis.DispatchResult.FileTimings) > 0 {
		dispatchDuration := time.Duration(analysis.PhaseTimingsMs.Dispatch) * time.Millisecond
		reportDispatchProfile(analysis.DispatchResult.FileTimings, phaseWorkerCount("ruleExecution", *r.f.Jobs, len(r.sourceFiles)), dispatchDuration)
	}
	r.allFindings = analysis.CrossFileResult.Findings.Findings()
}
