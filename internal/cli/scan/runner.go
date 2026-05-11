package scan

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/cache"
	"github.com/kaeawc/krit/internal/cli/clishared"
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/diag"
	"github.com/kaeawc/krit/internal/experiment"
	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/javafacts"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/pipeline"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/store"
	"github.com/kaeawc/krit/internal/trackedfiles"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// runner threads the long-lived state of scan.Run through a set of phase
// methods. Every variable that crossed two or more phases as a closed-over
// local now lives on the runner, so the closeCaches / exit closure pattern
// becomes a real defer on a single struct method (close).
//
// The phase methods are intentionally small and preserve the surrounding
// tracker tree shape (parent/child Serial labels and End ordering) because
// the perfTiming JSON tree is part of the stable output schema. Re-arrange
// with care.
type runner struct {
	f               *scanFlags
	cfg             *config.Config
	paths           []string
	effectiveFormat string
	maxFixLevel     rules.FixLevel
	depthPreset     DepthPreset
	start           time.Time
	tracker         perf.Tracker
	cpuProfileFile  *os.File
	trackedFiles    trackedfiles.Index

	// Project detection
	androidProject     *android.Project
	libraryFacts       *librarymodel.Facts
	androidProviders   *pipeline.AndroidProjectProviders
	projectModelLoaded bool
	cacheFilePath      string

	// analysisCacheLoadFuture memoizes the background cache.Load kicked
	// off as soon as cacheFilePath is known, in parallel with
	// collectFiles / projectModel / filterRules. nil when no preload
	// was scheduled (--no-cache, --oracle-filter-fingerprint).
	analysisCacheLoadFuture *pipeline.AnalysisCacheLoadFuture

	// File collection
	files                []string
	allJavaPaths         []string
	javaPathsForDispatch []string
	cacheFilePaths       []string

	// Rule filtering
	activeRules []*api.Rule

	// Resolver / oracle / cache
	resolver          typeinfer.TypeResolver
	reporter          *diag.Reporter
	daemon            *oracle.Daemon
	typeOracle        *oracle.Oracle
	oracleStore       *store.FileStore
	oracleCacheWriter *oracle.CacheWriter
	analysisCache     *cache.Cache
	cacheResult       *cache.Result
	ruleHash          string
	cacheStats        *cache.Stats
	useCache          bool

	// Parse caches
	parseCache         *scanner.ParseCache
	xmlParseCache      *android.XMLParseCache
	resourceCache      *android.ResourceIndexCache
	androidCacheWriter *scanner.AndroidCacheWriter
	androidCacheDir    string
	cachesClosed       bool

	// Parse phase
	parseResult       pipeline.ParseResult
	parsedFiles       []*scanner.File
	sourceFiles       []*scanner.File
	javaSemanticFacts *javafacts.Facts

	// Dispatch phase
	ruleStart      time.Time
	dispatchResult pipeline.DispatchResult
	perfRuleStats  []rules.RuleExecutionStat
	allFindings    []scanner.Finding

	// Cross-file / module
	indexResult2 pipeline.IndexResult
	codeIndex    *scanner.CodeIndex
	// crossFindings carries suppression-filtered cross-file cache hits for
	// parse-skipped warm runs.
	crossFindings   *scanner.FindingColumns
	parsedJavaFiles []*scanner.File
	outputJavaFiles []*scanner.File
	moduleGraph     *module.Graph
	pmi             *module.PerModuleIndex
	crossTracker    perf.Tracker
	moduleTracker   perf.Tracker

	// Output
	allColumns *scanner.FindingColumns
	basePath   string
	w          *os.File
}

// newRunner builds the runner that drives scan.Run's phases. It performs
// every step that historically sat between the pre-config helper-call
// cluster (runVersionFlag etc.) and the file-collection walk: config load
// + rules.ApplyConfig + the validate-config / list-rules / experiment
// short-circuit pumps + paths resolution + experiment-flag application +
// experiment-matrix + Android project detection + cache directory
// resolution + clear-cache + tracker creation + hashutil reset + CPU
// profile start.
//
// Callers must invoke r.close() (typically via defer) to drain caches
// and stop the CPU profile.
//
// Returns ok=false (with an exit code) when CPU profile creation fails;
// other early-exit guards inside the function call os.Exit directly.
func newRunner(f *scanFlags) (*runner, int, bool) {
	// Resolve output format: --report takes precedence over -f. If no
	// format is explicitly set and stdout is a TTY, auto-promote to
	// "plain" (respecting NO_COLOR and the -o file redirect).
	effectiveFormat := *f.Format
	if *f.Report != "" {
		effectiveFormat = *f.Report
	} else if *f.Format == "json" && *f.Output == "" {
		if _, noColor := os.LookupEnv("NO_COLOR"); !noColor {
			if fi, err := os.Stdout.Stat(); err == nil && fi.Mode()&os.ModeCharDevice != 0 {
				effectiveFormat = "plain"
			}
		}
	}

	runVersionFlag(*f.Version, Version)
	runClearMatrixCacheFlag(*f.ClearMatrixCache)
	runPromoteExperimentFlag(*f.PromoteExperiment)
	runDeprecateExperimentFlag(*f.DeprecateExperiment)
	runListExperimentsFlag(*f.ListExperiments, effectiveFormat, Version)
	runCompletionsFlag(*f.Completions)
	runInitFlag(*f.Init)
	runDoctorFlag(*f.Doctor, Version)
	runGenerateSchemaFlag(*f.GenerateSchema)

	defaultCfgPath := config.FindDefaultConfig()
	userCfgPath := *f.Config
	if userCfgPath == "" {
		userCfgPath = detectConfigForScanArgs(flag.Args())
	}
	cfg, cfgErr := config.LoadAndMerge(userCfgPath, defaultCfgPath)
	if cfgErr != nil {
		fmt.Fprintf(os.Stderr, "warning: config: %v\n", cfgErr)
	}
	if cfg == nil {
		cfg = config.NewConfig()
	}
	scanner.InitTestPaths(cfg.TestSourcePaths(), cfg.TestSourcePathsOverride())
	rules.ApplyConfig(cfg)

	// Resolve the analysis depth dial: --depth wins, then krit.yml
	// analysis.depth, then balanced. The preset only overrides
	// individual flags the user did not pass explicitly, so existing
	// scripts that set --no-type-oracle directly continue to win.
	depthPreset := resolveDepthPreset(*f.Depth, cfg, os.Stderr)
	applyDepthPreset(depthPreset, f, flag.CommandLine)

	runValidateConfigFlag(*f.ValidateConfig, cfg)

	applyEditorConfigOverrides(cfg, *f.EditorConfig, flag.Args())

	runListRulesFlag(*f.List, *f.Verbose)

	maxFixLevel := rules.FixIdiomatic
	if *f.Fix || *f.DryRun {
		if parsed, ok := rules.ParseFixLevel(*f.FixLevel); ok {
			maxFixLevel = parsed
		} else {
			fmt.Fprintf(os.Stderr, "error: invalid fix level '%s'. Use: cosmetic, idiomatic, semantic\n", *f.FixLevel)
			return nil, 2, false
		}
	}

	paths := flag.Args()
	if len(paths) == 0 {
		paths = []string{"."}
	}

	applyExperimentFlags(*f.Experiment, *f.ExperimentOff, os.Stderr)

	runExperimentMatrixFlag(experimentMatrixOpts{
		Spec:       *f.ExperimentMatrix,
		Candidates: *f.ExperimentCandidates,
		Intent:     *f.ExperimentIntent,
		Runs:       *f.ExperimentRuns,
		Targets:    *f.ExperimentTargets,
		Format:     effectiveFormat,
		OutputPath: *f.Output,
		NoCache:    *f.NoMatrixCache,
		StoreDir:   f.StoreDir,
		Paths:      paths,
	})

	_, cacheFilePath := cache.ResolveCacheDir(*f.CacheDir, paths)

	runClearCacheFlag(*f.ClearCache, *f.CacheDir, cacheFilePath, paths)

	start := time.Now()
	tracker := perf.New(*f.Perf)

	// Per-run content-hash memo: every cache subsystem that hashes files
	// routes through hashutil.Default(), so clearing it here guarantees
	// the memo lifetime is scoped to a single invocation and never
	// returns stale hashes across runs with the same working directory.
	hashutil.ResetDefault()

	cpuProfileFile, cpuErr := startCPUProfile(*f.CPUProfile, os.Stderr)
	if cpuErr != nil {
		return nil, 2, false
	}

	r := &runner{
		f:               f,
		cfg:             cfg,
		paths:           paths,
		effectiveFormat: effectiveFormat,
		maxFixLevel:     maxFixLevel,
		depthPreset:     depthPreset,
		start:           start,
		tracker:         tracker,
		cpuProfileFile:  cpuProfileFile,
		trackedFiles:    trackedfiles.NewGitIndex(),
		libraryFacts:    librarymodel.DefaultFacts(),
		cacheFilePath:   cacheFilePath,
	}
	// Skip the preload on early-exit paths that never reach
	// runOracleIndex. --oracle-filter-fingerprint is the only one
	// detectable here without doing the file walk first; the empty-repo
	// short-circuit needs file collection to fire and isn't worth a
	// pre-walk just to save the wasted load.
	if !*f.NoCache && cacheFilePath != "" && !*f.OracleFilterFingerprint {
		r.analysisCacheLoadFuture = pipeline.NewAnalysisCacheLoadFuture(func() *cache.Cache {
			return cache.Load(cacheFilePath)
		})
		r.analysisCacheLoadFuture.Start()
	}
	return r, 0, true
}

// collectFiles walks the scan paths once for both Kotlin and Java files,
// recording the wall-clock under the "collectFiles" tracker label.
// On warm runs the per-directory mtime cache eliminates most ReadDir calls.
func (r *runner) collectFiles() (int, error) {
	var walkErr error
	r.tracker.TrackVoid("collectFiles", func() {
		repoDir := oracle.FindRepoDir(r.paths)
		cacheDir := ""
		if repoDir != "" {
			cacheDir = filepath.Join(repoDir, ".krit", filewalkCacheDirName)
		}
		filters := FilewalkFilters{
			Extensions: []string{".kt", ".kts", ".java"},
		}
		var all []string
		all, walkErr = CollectFilesCachedWithIndex(r.paths, filters, cacheDir, r.trackedFiles)
		if walkErr != nil {
			return
		}
		for _, f := range all {
			if strings.HasSuffix(f, ".kt") || strings.HasSuffix(f, ".kts") {
				r.files = append(r.files, f)
			} else {
				r.allJavaPaths = append(r.allJavaPaths, f)
			}
		}
	})
	if walkErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", walkErr)
		return 2, walkErr
	}
	return 0, nil
}

func (r *runner) ensureProjectModel() {
	if r == nil || r.projectModelLoaded {
		return
	}
	r.tracker.TrackVoid("projectModel", func() {
		r.androidProject = android.DetectProjectWithIndex(r.paths, r.trackedFiles)
		r.libraryFacts = librarymodel.DefaultFacts()
		if r.androidProject != nil {
			cacheDir := ""
			if r.f != nil && r.f.NoCache != nil && !*r.f.NoCache {
				cacheDir = librarymodel.ProjectProfileCacheDir(oracle.FindRepoDir(r.paths))
			}
			profile, ok := librarymodel.LoadProjectProfileCache(cacheDir, r.androidProject.GradlePaths)
			if !ok {
				profile = librarymodel.ProfileFromGradlePaths(r.androidProject.GradlePaths)
				if err := librarymodel.SaveProjectProfileCache(cacheDir, r.androidProject.GradlePaths, profile); err != nil &&
					r.f != nil && r.f.Verbose != nil && *r.f.Verbose {
					fmt.Fprintf(os.Stderr, "verbose: library model cache save failed: %v\n", err)
				}
			}
			r.libraryFacts = librarymodel.FactsForProfile(profile)
		}
		r.projectModelLoaded = true
	})
}

// filterRules computes the active rule set from --disable-rules /
// --enable-rules / --all-rules, derives the Java path subset Dispatch
// will consume, and short-circuits when there's nothing to scan or
// when the oracle-filter-fingerprint flag is set.
//
// Returns (handled=true, code) when the run should terminate early.
func (r *runner) filterRules() (handled bool, code int) {
	r.tracker.TrackVoid("filterRules", func() {
		disabledSet := clishared.ParseRuleNameSetCSV(*r.f.DisableRules)
		enabledSet := clishared.ParseRuleNameSetCSV(*r.f.EnableRules)
		experimental := *r.f.Experimental || r.cfg.GetTopLevelBool("experimental", false)
		r.activeRules = rules.ActiveRulesV2(disabledSet, enabledSet, *r.f.AllRules, experimental)
		if pipeline.RulesNeedProjectModel(r.activeRules) {
			r.ensureProjectModel()
		}

		if pipeline.NeedsJavaBeforeDispatch(r.activeRules) {
			r.javaPathsForDispatch = r.allJavaPaths
			if !*r.f.IncludeGenerated {
				r.javaPathsForDispatch = filterGeneratedPathStrings(r.javaPathsForDispatch)
			}
		}
		androidProjectEmpty := r.androidProject == nil || r.androidProject.IsEmpty()
		if len(r.files) == 0 && len(r.javaPathsForDispatch) == 0 && androidProjectEmpty {
			if !*r.f.Quiet {
				fmt.Fprintln(os.Stderr, "info: No Kotlin, Java, or Android project files found.")
			}
			handled, code = true, 0
			return
		}
		r.cacheFilePaths = append([]string{}, r.files...)
		r.cacheFilePaths = append(r.cacheFilePaths, r.javaPathsForDispatch...)

		if *r.f.OracleFilterFingerprint {
			handled, code = true, RunOracleFilterFingerprint(r.paths, r.files, r.activeRules, *r.f.AllRules)
			return
		}
		handled, code = false, 0
	})
	return handled, code
}

// bootstrapResolver wires up the type resolver, the diagnostic reporter,
// and runs --output-types if requested. Mirrors the pre-refactor block at
// scan.go:431-445 line-for-line.
func (r *runner) bootstrapResolver() {
	r.tracker.TrackVoid("bootstrapResolver", func() {
		if !*r.f.NoTypeInfer {
			r.resolver = typeinfer.NewResolver()
		}
		r.reporter = installDiagnosticReporter(*r.f.Verbose)

		runOutputTypesFlag(outputTypesOpts{
			OutputPath:    *r.f.OutputTypes,
			NoCacheOracle: *r.f.NoCacheOracle,
			Verbose:       *r.f.Verbose,
			StoreDir:      r.f.StoreDir,
			Paths:         flag.Args(),
		})
	})
}

// runOracleIndex executes the standalone IndexPhase invocation that
// loads the type oracle and the incremental analysis cache. The phase
// runs in oracle-only mode (SkipModules + SkipAndroid +
// SkipResolverIndex) so it doesn't need parsed Kotlin files yet.
func (r *runner) runOracleIndex() (int, error) {
	r.useCache = !*r.f.NoCache
	r.oracleStore = resolvedStore(r.f.StoreDir)
	if r.resolver != nil && !*r.f.NoTypeOracle && !*r.f.NoCacheOracle {
		r.oracleCacheWriter = oracle.NewCacheWriter(*r.f.Jobs)
	}
	var res pipeline.IndexResult
	var err error
	var preloadedCache *cache.Cache
	if r.useCache && r.analysisCacheLoadFuture != nil {
		preloadedCache = r.analysisCacheLoadFuture.Await()
		// Surface the actual load wall-time as a perf entry so warm
		// runs don't read 0ms cacheLoad — the pipeline's trackSerial
		// wraps the receive, which is near-instant on the preloaded
		// path (see #67/#84).
		perf.AddEntry(r.tracker, "cacheLoadAsync", r.analysisCacheLoadFuture.Duration())
		r.analysisCacheLoadFuture = nil
	}
	r.tracker.TrackVoid("oracleIndex", func() {
		in := pipeline.IndexInput{
			ParseResult:       pipeline.ParseResult{ActiveRules: r.activeRules},
			Reporter:          r.reporter,
			Tracker:           r.tracker,
			OracleEnabled:     r.resolver != nil && !*r.f.NoTypeOracle,
			BaseResolver:      r.resolver,
			OracleScanPaths:   flag.Args(),
			KotlinFilePaths:   r.files,
			InputTypesPath:    *r.f.InputTypes,
			NoCacheOracle:     *r.f.NoCacheOracle,
			NoOracleFilter:    *r.f.NoOracleFilter,
			OracleDiagnostics: *r.f.OracleDiagnostics,
			UseDaemon:         *r.f.Daemon,
			Store:             r.oracleStore,
			OracleCacheWriter: r.oracleCacheWriter,
			Verbose:           *r.f.Verbose,

			CacheEnabled:             r.useCache,
			CacheFilePath:            r.cacheFilePath,
			CacheDirExplicit:         *r.f.CacheDir != "",
			CacheScanPaths:           r.paths,
			CacheFilePaths:           r.cacheFilePaths,
			CacheConfig:              r.cfg,
			CacheEditorConfigEnabled: *r.f.EditorConfig,
			PreloadedAnalysisCache:   preloadedCache,
		}
		res, err = (pipeline.IndexPhase{
			SkipModules:       true,
			SkipAndroid:       true,
			SkipResolverIndex: true,
		}).Run(context.Background(), in)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2, err
	}
	if res.Resolver != nil {
		r.resolver = res.Resolver
	}
	if res.Daemon != nil {
		r.daemon = res.Daemon
	}
	r.typeOracle = res.Oracle
	if r.useCache {
		r.analysisCache = res.Cache
		r.cacheResult = res.CacheResult
		r.ruleHash = res.RuleHash
		r.cacheStats = res.CacheStats
	}
	return 0, nil
}

// printVerboseBanner emits the "verbose: Found N Kotlin files…" / "Type
// resolver active" / "Running N rules with M workers…" lines that
// historically sat between the oracle index block and the Android
// providers setup. No-op when --verbose is off.
// setupAndroidProviders builds the project providers used by AndroidPhase.
func (r *runner) setupAndroidProviders() {
	r.tracker.TrackVoid("setupAndroidProviders", func() {
		if !pipeline.RulesNeedAndroidProject(r.activeRules) {
			return
		}
		deps := pipeline.CollectAndroidDependenciesV2(r.activeRules)
		r.androidProviders = pipeline.NewAndroidProjectProviders(r.androidProject, deps, *r.f.Jobs)
	})
}

// setupParseCaches creates parseCache, xmlParseCache, and resourceCache
// when their respective --no-* flags allow. All three are owned by the
// runner so close() can drain them under "cacheBackgroundFlush".
func (r *runner) setupParseCaches() {
	r.tracker.TrackVoid("setupParseCaches", func() {
		parseWorkers := phaseWorkerCount("parse", *r.f.Jobs, len(r.files))
		repoDir := oracle.FindRepoDir(r.paths)

		// Android findings cache is needed even when the source parse is skipped:
		// AndroidPhase uses the writer as the cacheability gate for cache loads.
		if pipeline.RulesNeedAndroidProject(r.activeRules) && !*r.f.NoCache && repoDir != "" {
			r.androidCacheDir = scanner.AndroidFindingsCacheDir(repoDir)
			r.androidCacheWriter = scanner.NewAndroidCacheWriter(*r.f.Jobs)
		}

		skipSourceParse := r.canSkipParsePhase()
		if !*r.f.NoParseCache && !skipSourceParse {
			capBytes := resolveParseCacheCap(*r.f.ParseCacheCapMB, r.cfg)
			if pc, pcErr := scanner.NewParseCacheWithCap(repoDir, capBytes); pcErr == nil {
				r.parseCache = pc
				r.parseCache.SetAsyncWriter(newParseCacheAsyncWriter(parseWorkers))
			} else if *r.f.Verbose {
				fmt.Fprintf(os.Stderr, "verbose: parse cache disabled: %v\n", pcErr)
			}
			if pipeline.RulesNeedAndroidProject(r.activeRules) {
				if xmlPC, xmlErr := android.NewXMLParseCacheWithCap(repoDir, capBytes); xmlErr == nil {
					r.xmlParseCache = xmlPC
				} else if *r.f.Verbose {
					fmt.Fprintf(os.Stderr, "verbose: xml parse cache disabled: %v\n", xmlErr)
				}
			}
		}
		if shouldOpenResourceIndexCache(r.activeRules, *r.f.NoResourceCache, skipSourceParse, r.androidCacheDir != "" && r.androidCacheWriter != nil) {
			if rc, rcErr := android.NewResourceIndexCache(oracle.FindRepoDir(r.paths)); rcErr == nil {
				r.resourceCache = rc
				android.SetActiveResourceIndexCache(rc)
			} else if *r.f.Verbose {
				fmt.Fprintf(os.Stderr, "verbose: resource cache disabled: %v\n", rcErr)
			}
		} else {
			android.SetActiveResourceIndexCache(nil)
		}
	})
}

// parsePhase runs ParsePhase and feeds its output into Java semantic
// facts collection and the per-file type indexer.
func (r *runner) parsePhase() (int, error) {
	if r.canSkipParsePhase() {
		if r.tracker != nil {
			r.tracker.TrackVoid("parse", func() {})
		}
		r.parseResult = pipeline.ParseResult{
			Config:      r.cfg,
			ActiveRules: r.activeRules,
			KotlinPaths: append([]string(nil), r.files...),
			Paths:       r.paths,
		}
		r.parsedFiles = nil
		r.sourceFiles = nil
		if *r.f.Verbose {
			fmt.Fprintf(os.Stderr, "verbose: Skipped parse; findings cache covers %d files and no active phase needs parsed sources\n", len(r.files))
		}
		return 0, nil
	}

	parseWorkers := phaseWorkerCount("parse", *r.f.Jobs, len(r.files))
	kotlinPaths := r.files
	allowCrossFileDelta := r.canUseWarmCrossFindingsDelta()
	allowResourceSourceDelta := r.canUseWarmAndroidResourceSourceDelta()
	if pipeline.CanParseOnlyCacheMisses(r.activeRules, r.cacheResult, r.useCache, allowCrossFileDelta, allowResourceSourceDelta) {
		kotlinPaths = pipeline.CacheMissPaths(r.files, r.cacheResult)
		if *r.f.Verbose {
			fmt.Fprintf(os.Stderr, "verbose: Parsing %d cache-miss files; findings cache covers %d/%d files\n", len(kotlinPaths), r.cacheResult.TotalCached, r.cacheResult.TotalFiles)
		}
	}
	parseResult, err := pipeline.ParsePhase{}.Run(context.Background(), pipeline.ParseInput{
		Config:             r.cfg,
		Paths:              r.paths,
		ActiveRules:        r.activeRules,
		KotlinPaths:        kotlinPaths,
		JavaPaths:          r.javaPathsForDispatch,
		Workers:            parseWorkers,
		IncludeGenerated:   *r.f.IncludeGenerated,
		SkipJavaCollection: !pipeline.NeedsJavaBeforeDispatch(r.activeRules),
		Reporter:           r.reporter,
		Tracker:            r.tracker,
		ParseCache:         r.parseCache,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2, err
	}
	r.parsedFiles = parseResult.KotlinFiles
	r.sourceFiles = parseResult.SourceFiles()
	_ = parseResult.ParseErrors
	parseResult.ActiveRules = r.activeRules
	r.parseResult = parseResult

	if api.NeedsJavaFacts(r.activeRules) && len(parseResult.JavaFiles) > 0 {
		facts, warning, jerr := runJavaSemanticFacts(context.Background(), r.paths, parseResult.JavaFiles, r.libraryFacts, r.tracker)
		if jerr != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", jerr)
			return 2, jerr
		}
		r.javaSemanticFacts = facts
		if warning != "" {
			fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
		} else if *r.f.Verbose && facts != nil {
			fmt.Fprintf(os.Stderr, "verbose: Java semantic facts loaded (%d calls, %d classes)\n",
				len(facts.Calls), len(facts.Classes))
		}
	}

	if len(r.parsedFiles) > 0 {
		r.typeIndexPhase()
	}
	return 0, nil
}

func (r *runner) canSkipParsePhase() bool {
	if r == nil || r.cacheResult == nil || !r.useCache {
		if r != nil {
			r.logParseSkipBlocked("cache unavailable")
		}
		return false
	}
	if r.f == nil {
		return false
	}
	if r.cacheResult.TotalFiles == 0 || r.cacheResult.TotalCached != r.cacheResult.TotalFiles {
		r.logParseSkipBlocked("findings cache miss")
		return false
	}
	if len(r.cacheResult.CachedPaths) != len(r.cacheFilePaths) {
		r.logParseSkipBlocked("cached path set mismatch")
		return false
	}
	if *r.f.Fir && !*r.f.NoFir {
		r.logParseSkipBlocked("FIR enabled")
		return false
	}
	noCrossFileCache := false
	if r.f.NoCrossFileCache != nil {
		noCrossFileCache = *r.f.NoCrossFileCache
	}
	canUseCrossFileCache := resolveCrossFileCacheDir(r.paths, noCrossFileCache) != ""
	warmAndroidSourceCache := r.canUseWarmAndroidResourceSourceCache()
	if pipeline.RulesNeedParsedSource(r.activeRules, canUseCrossFileCache, warmAndroidSourceCache) {
		r.logParseSkipBlocked("active rule requires parsed source: " + pipeline.ParsedSourceBlockReason(r.activeRules, canUseCrossFileCache, warmAndroidSourceCache))
		return false
	}
	if canUseCrossFileCache && pipeline.RulesNeedCrossOrParsedFiles(r.activeRules) {
		if !r.loadWarmCrossFindings() {
			r.logParseSkipBlocked("cross-file findings cache miss")
			return false
		}
	}
	return true
}

func (r *runner) logParseSkipBlocked(reason string) {
	if r == nil || r.f == nil || r.f.Verbose == nil || !*r.f.Verbose {
		return
	}
	fmt.Fprintf(os.Stderr, "verbose: Parse skip unavailable: %s\n", reason)
}

func (r *runner) canUseWarmAndroidResourceSourceCache() bool {
	if r == nil || r.androidProject == nil || r.androidProject.IsEmpty() || r.androidCacheDir == "" || r.androidCacheWriter == nil || r.ruleHash == "" {
		return false
	}
	if !pipeline.HasResourceSourceRules(r.activeRules) {
		return true
	}
	if r.cacheResult != nil && len(r.cacheResult.CachedHashes) > 0 {
		return pipeline.EnsureWarmResourceSourceBundleWithHashes(
			r.androidCacheDir,
			r.cacheFilePaths,
			r.androidProject.ResDirs,
			r.cacheResult.CachedHashes,
			r.ruleHash,
			r.libraryFacts.Fingerprint(),
			r.javaSemanticFacts.Fingerprint(),
		)
	}
	return pipeline.HasWarmResourceSourceBundle(
		r.androidCacheDir,
		r.cacheFilePaths,
		r.androidProject.ResDirs,
		r.ruleHash,
		r.libraryFacts.Fingerprint(),
		r.javaSemanticFacts.Fingerprint(),
	)
}

func (r *runner) canUseWarmAndroidResourceSourceDelta() bool {
	if r == nil || r.androidProject == nil || r.androidProject.IsEmpty() || r.androidCacheDir == "" || r.androidCacheWriter == nil || r.ruleHash == "" {
		return false
	}
	if !pipeline.HasResourceSourceRules(r.activeRules) {
		return true
	}
	return pipeline.HasWarmResourceSourceBundleManifest(
		r.androidCacheDir,
		r.cacheFilePaths,
		r.androidProject.ResDirs,
		r.ruleHash,
		r.libraryFacts.Fingerprint(),
		r.javaSemanticFacts.Fingerprint(),
	)
}

func (r *runner) canUseWarmCrossFindingsDelta() bool {
	if r == nil || !pipeline.RulesNeedCrossOrParsedFiles(r.activeRules) {
		return true
	}
	if r.f != nil && r.f.NoCrossFileCache != nil && *r.f.NoCrossFileCache {
		return false
	}
	return r.loadWarmCrossFindings()
}

func (r *runner) loadWarmCrossFindings() bool {
	if r.crossFindings != nil {
		return true
	}
	noCrossFileCache := false
	if r.f != nil && r.f.NoCrossFileCache != nil {
		noCrossFileCache = *r.f.NoCrossFileCache
	}
	indexDir := resolveCrossFileCacheDir(r.paths, noCrossFileCache)
	findingsDir := resolveCrossFindingsCacheDir(r.paths, noCrossFileCache)
	if indexDir == "" || findingsDir == "" || r.ruleHash == "" {
		if r.f != nil && r.f.Verbose != nil && *r.f.Verbose {
			fmt.Fprintf(os.Stderr, "verbose: Cross-file findings warm load skipped (indexDir=%t findingsDir=%t ruleHash=%t)\n", indexDir != "", findingsDir != "", r.ruleHash != "")
		}
		return false
	}
	meta, ok := scanner.LoadCurrentCrossFileCacheMeta(indexDir)
	if !ok {
		if r.f != nil && r.f.Verbose != nil && *r.f.Verbose {
			fmt.Fprintln(os.Stderr, "verbose: Cross-file findings warm load skipped (missing current index metadata)")
		}
		return false
	}
	key := scanner.CrossFindingsKey(meta.Fingerprint, r.ruleHash)
	cols, ok := scanner.LoadCrossFindings(findingsDir, key)
	if !ok {
		if r.f != nil && r.f.Verbose != nil && *r.f.Verbose {
			fmt.Fprintln(os.Stderr, "verbose: Cross-file findings warm load skipped (findings cache miss)")
		}
		return false
	}
	r.crossFindings = &cols
	return true
}

func (r *runner) typeIndexPhase() {
	if r.resolver == nil {
		return
	}
	if !r.hasTypeAwareRule() {
		if *r.f.Verbose {
			fmt.Fprintln(os.Stderr, "verbose: Skipped type index (no active type-aware rules)")
		}
		return
	}
	indexStart := time.Now()
	indexWorkers := phaseWorkerCount("typeIndex", *r.f.Jobs, len(r.parsedFiles))
	if !r.runCachedTypeIndex(indexWorkers) {
		r.runUncachedTypeIndex(indexWorkers)
	}
	if *r.f.Verbose {
		fmt.Fprintf(os.Stderr, "verbose: Type-indexed %d files in %v\n",
			len(r.parsedFiles), time.Since(indexStart).Round(time.Millisecond))
	}
}

func (r *runner) hasTypeAwareRule() bool {
	for _, ru := range r.activeRules {
		if ru != nil && ru.Needs.Has(api.NeedsResolver) {
			return true
		}
	}
	return false
}

func (r *runner) runCachedTypeIndex(indexWorkers int) bool {
	if !r.useCache {
		return false
	}
	cacheDir := typeinfer.TypeIndexCacheDir(oracle.FindRepoDir(r.paths))
	if cacheDir == "" {
		return false
	}
	indexer, ok := r.resolver.(interface {
		IndexFilesParallelCachedWithTracker([]*scanner.File, int, string, perf.Tracker) (int, int)
	})
	if !ok {
		return false
	}
	typeTracker := r.tracker.Serial("typeIndex")
	hits, misses := indexer.IndexFilesParallelCachedWithTracker(r.parsedFiles, indexWorkers, cacheDir, typeTracker)
	perf.AddEntryDetails(typeTracker, "cacheSummary", 0, map[string]int64{
		"hits":   int64(hits),
		"misses": int64(misses),
	}, nil)
	typeTracker.End()
	return true
}

func (r *runner) runUncachedTypeIndex(indexWorkers int) {
	if indexer, ok := r.resolver.(interface {
		IndexFilesParallelWithTracker([]*scanner.File, int, perf.Tracker)
	}); ok {
		typeTracker := r.tracker.Serial("typeIndex")
		indexer.IndexFilesParallelWithTracker(r.parsedFiles, indexWorkers, typeTracker)
		typeTracker.End()
		return
	}
	if indexer, ok := r.resolver.(interface {
		IndexFilesParallel([]*scanner.File, int)
	}); ok {
		r.tracker.TrackVoid("typeIndex", func() {
			indexer.IndexFilesParallel(r.parsedFiles, indexWorkers)
		})
	}
}

// targetedResolution runs the on-demand expression-fact pre-pass when
// --depth=thorough is active and the daemon is available. For other
// depths or when no rule supplies an ExprPositions selector, the pass
// is a no-op (and the helper returns nil with no work). A resolver
// error is logged at warning level — we fall through to source-only
// inference rather than failing the whole scan.
//
// methods (parsePhase, dispatch, crossFile) which all return
// (exitCode, err) and dispatch through the same `code, err := r.X()`
// pattern in scan.go. Future failure modes here may need to surface a
// non-zero exit without restructuring callers.
//
//nolint:unparam // (int) result kept for symmetry with sibling phase
func (r *runner) targetedResolution() (int, error) {
	if r.depthPreset != DepthThorough {
		return 0, nil
	}
	if r.daemon == nil || r.typeOracle == nil {
		// Daemon is required for targeted resolution. One-shot mode
		// would have to spin up a fresh JVM per RPC call, which
		// defeats the cost-saving purpose. Quietly skip.
		if *r.f.Verbose {
			fmt.Fprintln(os.Stderr, "verbose: depth=thorough targeted resolution skipped (daemon unavailable)")
		}
		return 0, nil
	}
	err := pipeline.RunTargetedResolutionPass(pipeline.TargetedResolutionInput{
		ActiveRules: r.activeRules,
		Files:       r.parsedFiles,
		Resolver:    pipeline.DaemonExpressionResolver{Daemon: r.daemon},
		Sink:        r.typeOracle,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: targeted expression resolution failed: %v\n", err)
	}
	return 0, nil
}

// dispatch runs DispatchPhase and gathers per-rule execution stats when
// --perf-rules is set. Mirrors scan.go:701-742 exactly.
func (r *runner) dispatch() (int, error) {
	r.ruleStart = time.Now()
	ruleWorkers := phaseWorkerCount("ruleExecution", *r.f.Jobs, len(r.sourceFiles))
	dispatchIdx := pipeline.IndexResult{
		ParseResult:       r.parseResult,
		Resolver:          r.resolver,
		Oracle:            r.typeOracle,
		LibraryFacts:      r.libraryFacts,
		JavaSemanticFacts: r.javaSemanticFacts,
		CacheResult:       r.cacheResult,
		Cache:             r.analysisCache,
		RuleHash:          r.ruleHash,
		CacheFilePath:     r.cacheFilePath,
		CacheStats:        r.cacheStats,
		Reporter:          r.reporter,
		Tracker:           r.tracker,
		Jobs:              *r.f.Jobs,
		ProfileDispatch:   *r.f.ProfileDispatch,
		Version:           Version,
		CacheScanPaths:    r.paths,
		EmitPerFileStats:  true,
	}
	res, err := (pipeline.DispatchPhase{Workers: ruleWorkers}).Run(context.Background(), dispatchIdx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2, err
	}
	r.dispatchResult = res
	if *r.f.PerfRules {
		r.perfRuleStats = rules.SortedRuleExecutionStats(res.Stats)
		if !*r.f.Quiet {
			reportRuleExecutionRanking(os.Stderr, r.perfRuleStats, 20)
		}
	}
	r.allFindings = res.Findings.Findings()
	if *r.f.ProfileDispatch && len(res.FileTimings) > 0 {
		reportDispatchProfile(res.FileTimings, ruleWorkers, time.Since(r.ruleStart))
	}
	return 0, nil
}

// crossFile runs the post-dispatch IndexPhase + CrossFilePhase pair.
// Tracker tree shape (crossFileAnalysis / moduleAwareAnalysis parents
// nesting both indexing children from IndexPhase and rule-execution
// siblings from CrossFilePhase) is preserved bit-for-bit.
func (r *runner) crossFile() (int, error) {
	hasIndexBackedCrossFileRule, hasParsedFilesRule, hasModuleAwareRule := pipeline.ClassifyCrossFileNeeds(r.activeRules)
	crossFindingsCacheHit := r.prepareCrossFileTrackers(hasIndexBackedCrossFileRule, hasParsedFilesRule, hasModuleAwareRule)

	scanRoot := "."
	if len(r.paths) > 0 {
		scanRoot = r.paths[0]
	}

	if r.useWarmCrossFindings(crossFindingsCacheHit, hasModuleAwareRule) {
		return 0, nil
	}

	parseForIndex := r.parseResult
	parseForIndex.ActiveRules = r.activeRules
	idx2, err := (pipeline.IndexPhase{
		SkipModules:       true,
		SkipAndroid:       true,
		SkipResolverIndex: true,
	}).Run(context.Background(), pipeline.IndexInput{
		ParseResult:              parseForIndex,
		Reporter:                 r.reporter,
		Tracker:                  r.tracker,
		SkipOracle:               true,
		SkipCache:                true,
		Verbose:                  *r.f.Verbose,
		BuildCodeIndex:           hasIndexBackedCrossFileRule && !crossFindingsCacheHit,
		CrossFileParentTracker:   r.crossTracker,
		CrossFileJobsFlag:        *r.f.Jobs,
		CrossFileCacheDir:        resolveCrossFileCacheDir(r.paths, *r.f.NoCrossFileCache),
		CrossFindingsCacheDir:    resolveCrossFindingsCacheDir(r.paths, *r.f.NoCrossFileCache),
		CrossFileJavaPaths:       r.allJavaPaths,
		ParseCache:               r.parseCache,
		BuildModuleIndex:         hasModuleAwareRule,
		ModuleParentTracker:      r.moduleTracker,
		ModuleScanRoot:           scanRoot,
		ModuleJobsFlag:           *r.f.Jobs,
		ModuleHasAwareRule:       hasModuleAwareRule,
		PrebuiltLibraryFacts:     r.libraryFacts,
		CacheConfig:              r.cfg,
		CacheEditorConfigEnabled: *r.f.EditorConfig,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2, err
	}
	r.indexResult2 = idx2
	r.indexResult2.RuleHash = r.ruleHash
	idx2.RuleHash = r.ruleHash
	r.codeIndex = idx2.CodeIndex
	r.parsedJavaFiles = idx2.JavaFiles
	r.outputJavaFiles = r.parseResult.JavaFiles
	if len(r.parsedJavaFiles) > 0 {
		r.outputJavaFiles = r.parsedJavaFiles
	}
	r.moduleGraph = idx2.Graph
	r.pmi = idx2.ModuleIndex
	if r.moduleTracker != nil && (r.moduleGraph == nil || len(r.moduleGraph.Modules) == 0) {
		r.moduleTracker.End()
		r.moduleTracker = nil
	}

	dispatchForCross := r.dispatchResult
	dispatchForCross.IndexResult = idx2
	dispatchForCross.ActiveRules = r.activeRules
	if crossFindingsCacheHit {
		dispatchForCross.ActiveRules = pipeline.ModuleOnlyRules(r.activeRules)
	}
	dispatchForCross.Reporter = r.reporter
	dispatchForCross.Tracker = r.tracker
	dispatchForCross.CrossFileParentTracker = r.crossTracker
	dispatchForCross.ModuleParentTracker = r.moduleTracker
	dispatchForCross.Findings = scanner.CollectFindings(r.allFindings)
	crossResult, err := (pipeline.CrossFilePhase{Workers: *r.f.Jobs}).Run(context.Background(), dispatchForCross)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2, err
	}
	if r.crossTracker != nil {
		r.crossTracker.End()
	}
	if r.moduleTracker != nil {
		r.moduleTracker.End()
	}
	_ = r.parsedJavaFiles
	_ = r.codeIndex
	_ = r.moduleGraph
	_ = r.pmi
	r.allFindings = crossResult.Findings.Findings()

	if *r.f.Verbose {
		fmt.Fprintf(os.Stderr, "verbose: Analyzed in %v\n", time.Since(r.ruleStart).Round(time.Millisecond))
	}
	return 0, nil
}

func (r *runner) prepareCrossFileTrackers(hasIndexBackedCrossFileRule, hasParsedFilesRule, hasModuleAwareRule bool) bool {
	crossFindingsCacheHit := false
	if hasIndexBackedCrossFileRule || hasParsedFilesRule {
		crossFindingsCacheHit = r.loadWarmCrossFindings()
	}
	if (hasIndexBackedCrossFileRule || hasParsedFilesRule) && !crossFindingsCacheHit {
		r.crossTracker = r.tracker.Serial("crossFileAnalysis")
	}
	if hasModuleAwareRule {
		r.moduleTracker = r.tracker.Serial("moduleAwareAnalysis")
	}
	return crossFindingsCacheHit
}

func (r *runner) useWarmCrossFindings(crossFindingsCacheHit, hasModuleAwareRule bool) bool {
	if !crossFindingsCacheHit {
		return false
	}
	r.mergeWarmCrossFindings()
	if *r.f.Verbose {
		fmt.Fprintf(os.Stderr, "verbose: Cross-file findings cache: HIT (%d findings)\n", r.crossFindings.Len())
	}
	return !hasModuleAwareRule
}

func (r *runner) mergeWarmCrossFindings() {
	if r == nil || r.crossFindings == nil {
		return
	}
	base := scanner.CollectFindings(r.allFindings)
	merged := scanner.NewFindingCollector(base.Len() + r.crossFindings.Len())
	merged.AppendColumns(&base)
	merged.AppendColumns(r.crossFindings)
	r.allFindings = merged.Columns().Findings()
}

// androidPhase runs the project-level AndroidPhase (manifests, res
// dirs, Gradle, icons) and merges its findings into r.allFindings.
func (r *runner) androidPhase() (int, error) {
	if !pipeline.RulesNeedAndroidProject(r.activeRules) || r.androidProject == nil || r.androidProject.IsEmpty() {
		return 0, nil
	}
	androidStart := time.Now()
	androidTracker := r.tracker.Serial("androidProjectAnalysis")
	dispatcher := rules.NewDispatcher(r.activeRules, r.resolver)
	dispatcher.SetLibraryFacts(r.libraryFacts)
	dispatcher.SetJavaSemanticFacts(r.javaSemanticFacts)
	res, err := (pipeline.AndroidPhase{}).Run(context.Background(), pipeline.AndroidInput{
		Project:             r.androidProject,
		ActiveRules:         r.activeRules,
		Dispatcher:          dispatcher,
		SourceFiles:         r.sourceFiles,
		SourcePaths:         r.cacheFilePaths,
		SourceHashes:        pipeline.CachedHashesOrNil(r.cacheResult),
		Providers:           r.androidProviders,
		Tracker:             androidTracker,
		RuleHash:            r.ruleHash,
		LibraryFactsFP:      r.libraryFacts.Fingerprint(),
		JavaSemanticFactsFP: r.javaSemanticFacts.Fingerprint(),
		CacheDir:            r.androidCacheDir,
		CacheWriter:         r.androidCacheWriter,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2, err
	}
	cols := res.Findings
	androidTracker.End()
	if cols.Len() > 0 {
		r.allFindings = append(r.allFindings, cols.Findings()...)
	}
	if *r.f.Verbose && !r.androidProject.IsEmpty() {
		fmt.Fprintf(os.Stderr, "verbose: Android project analysis in %v (%d findings across %d manifests, %d res dirs, %d Gradle files)\n",
			time.Since(androidStart).Round(time.Millisecond), cols.Len(),
			len(r.androidProject.ManifestPaths), len(r.androidProject.ResDirs), len(r.androidProject.GradlePaths))
	}
	return 0, nil
}

// firCheckAndCollect runs the FIR checker pass (gated by --fir / --no-fir)
// and finalizes findings into the columnar form used by output.
func (r *runner) firCheckAndCollect() {
	r.tracker.TrackVoid("firCheckAndCollect", func() {
		r.allFindings = runFIRCheckerPass(firCheckerOpts{
			Enabled:     *r.f.Fir && !*r.f.NoFir,
			UseDaemon:   !*r.f.NoFirDaemon,
			Verbose:     *r.f.Verbose,
			Paths:       r.paths,
			ActiveRules: r.activeRules,
			ParsedFiles: r.parsedFiles,
			Tracker:     r.tracker,
			VerboseOut:  os.Stderr,
		}, r.allFindings)

		r.applySLOs()

		cols := scanner.CollectFindings(r.allFindings)
		r.allColumns = &cols

		r.basePath = *r.f.BasePath
		if r.basePath == "" && len(r.paths) > 0 {
			r.basePath, _ = filepath.Abs(r.paths[0]) // best-effort: error means relative path used
		}
	})
}

// applyBaselinesAndDiff handles --create-baseline, --baseline-audit,
// --baseline filtering, --diff/--delta filtering, and --remove-dead-code in the
// same order as the main scan flow.
//
// Returns (handled=true, code) when the run should terminate early.
func (r *runner) applyBaselinesAndDiff() (handled bool, code int) {
	r.tracker.TrackVoid("applyBaselinesAndDiff", func() {
		if *r.f.CreateBaseline != "" {
			if err := scanner.WriteBaselineColumns(*r.f.CreateBaseline, r.allColumns, r.basePath); err != nil {
				fmt.Fprintf(os.Stderr, "error: failed to write baseline: %v\n", err)
				handled, code = true, 2
				return
			}
			if !*r.f.Quiet {
				fmt.Fprintf(os.Stderr, "info: Created baseline with %d issue(s) at %s\n", r.allColumns.Len(), *r.f.CreateBaseline)
			}
			handled, code = true, 0
			return
		}

		if *r.f.BaselineAudit {
			baselinePath, err := ResolveBaselineAuditPath(*r.f.Baseline, r.paths)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				handled, code = true, 2
				return
			}
			baseline, err := scanner.LoadBaseline(baselinePath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: failed to load baseline: %v\n", err)
				handled, code = true, 2
				return
			}
			handled, code = true, RunBaselineAuditColumns(r.allColumns, baseline, baselinePath, r.basePath, r.paths, r.effectiveFormat)
			return
		}

		if *r.f.Baseline != "" {
			baseline, err := scanner.LoadBaseline(*r.f.Baseline)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: failed to load baseline: %v\n", err)
				handled, code = true, 2
				return
			}
			beforeCount := r.allColumns.Len()
			filtered := scanner.FilterColumnsByBaseline(r.allColumns, baseline, r.basePath)
			r.allColumns = &filtered
			if *r.f.Verbose {
				fmt.Fprintf(os.Stderr, "verbose: Baseline suppressed %d of %d findings\n",
					beforeCount-r.allColumns.Len(), beforeCount)
			}
		}

		if *r.f.Diff != "" {
			r.applyDiffFilter(*r.f.Diff)
		}

		if *r.f.Delta != "" {
			if h, c := r.applyDeltaFilter(*r.f.Delta); h {
				handled, code = true, c
				return
			}
		}

		if *r.f.RemoveDeadCode {
			handled, code = true, RunDeadCodeRemovalColumns(r.allColumns, r.effectiveFormat, *r.f.DryRun, *r.f.FixSuffix)
			return
		}
		handled, code = false, 0
	})
	return handled, code
}

func (r *runner) applyDiffFilter(diff string) {
	changedFiles, err := getChangedFiles(diff, r.paths)
	if err != nil {
		if *r.f.Verbose {
			fmt.Fprintf(os.Stderr, "verbose: git diff failed: %v (showing all findings)\n", err)
		}
		return
	}
	beforeCount := r.allColumns.Len()
	filtered := scanner.FilterColumnsByFilePaths(r.allColumns, changedFiles)
	r.allColumns = &filtered
	if *r.f.Verbose {
		fmt.Fprintf(os.Stderr, "verbose: --diff %s filtered %d → %d findings (file-level)\n",
			diff, beforeCount, r.allColumns.Len())
	}
	changedLines, err := getChangedLineIntervals(diff, r.paths)
	if err != nil {
		if *r.f.Verbose {
			fmt.Fprintf(os.Stderr, "verbose: git diff hunk parse failed: %v (using file-level filter)\n", err)
		}
		return
	}
	fileLevelCount := r.allColumns.Len()
	filtered2 := filterColumnsByChangedLines(r.allColumns, changedLines)
	r.allColumns = &filtered2
	if *r.f.Verbose {
		fmt.Fprintf(os.Stderr, "verbose: --diff %s filtered %d → %d findings (line-level)\n",
			diff, fileLevelCount, r.allColumns.Len())
	}
}

func (r *runner) applyDeltaFilter(delta string) (handled bool, code int) {
	if *r.f.Baseline != "" {
		fmt.Fprintln(os.Stderr, "error: --delta and --baseline are mutually exclusive")
		return true, 2
	}
	beforeCount := r.allColumns.Len()
	filtered, err := r.filterColumnsByDelta(delta)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: --delta %s: %v\n", delta, err)
		return true, 2
	}
	r.allColumns = &filtered
	if *r.f.Verbose {
		fmt.Fprintf(os.Stderr, "verbose: --delta %s filtered %d → %d new findings\n",
			delta, beforeCount, r.allColumns.Len())
	}
	return false, 0
}

// runFixup runs FixupPhase when --fix or --dry-run is set, prints the
// human-readable result lines, and short-circuits when --fix was used
// alone with no explicit output format.
//
// Returns (handled=true, code) when the run should terminate early.
func (r *runner) runFixup() (handled bool, code int) {
	if !*r.f.Fix && !*r.f.DryRun {
		return false, 0
	}
	r.tracker.TrackVoid("fixup", func() {
		fixRes, _ := (pipeline.FixupPhase{}).Run(context.Background(), pipeline.FixupInput{
			CrossFileResult: pipeline.CrossFileResult{
				DispatchResult: pipeline.DispatchResult{
					Findings: *r.allColumns,
				},
			},
			Apply:        *r.f.Fix && !*r.f.DryRun,
			ApplyBinary:  *r.f.FixBinary,
			Suffix:       *r.f.FixSuffix,
			MaxFixLevel:  r.maxFixLevel,
			DryRunBinary: *r.f.DryRun,
			CountOnly:    *r.f.DryRun,
		})
		postColumns := fixRes.Findings
		r.allColumns = &postColumns

		r.printFixupResult(fixRes)
		r.printBinaryFixResult(fixRes)

		if *r.f.Output == "" && r.effectiveFormat == "json" && *r.f.Report == "" && *r.f.Fix {
			if r.allColumns.Len()-fixRes.FixableCount > 0 {
				if !*r.f.Quiet {
					fmt.Fprintf(os.Stderr, "info: %d unfixable issue(s) remain.\n", r.allColumns.Len()-fixRes.FixableCount)
				}
				handled, code = true, 1
				return
			}
			handled, code = true, 0
			return
		}
		handled, code = false, 0
	})
	return handled, code
}

// openOutputWriter opens the -o output file (or returns os.Stdout) and
// stashes it on the runner. The scan flow flushes caches once before opening
// output and once after.
func (r *runner) openOutputWriter() (int, error) {
	var openErr error
	r.tracker.TrackVoid("openOutputWriter", func() {
		if *r.f.Output != "" {
			var w *os.File
			w, openErr = os.Create(*r.f.Output)
			if openErr != nil {
				return
			}
			r.w = w
			return
		}
		r.w = os.Stdout
	})
	if openErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", openErr)
		return 2, openErr
	}
	return 0, nil
}

// recordTotalTimingAndStopProfiles records the "total" wall-clock entry,
// stops the CPU profile, writes the heap profile (if --memprofile), and
// emits the diagnostic perf-reporting lines that surface oracle and
// content-hash memo hit-rates when --perf is on.
func (r *runner) recordTotalTimingAndStopProfiles() {
	perf.AddEntry(r.tracker, "total", time.Since(r.start))
	stopCPUProfile(r.cpuProfileFile)
	r.cpuProfileFile = nil // close() must not stop again

	writeMemProfile(*r.f.MemProfile, os.Stderr)

	if *r.f.Perf {
		reportOracleLookupStats(os.Stderr, r.resolver)
		reportContentHashMemoStats(os.Stderr)
	}
}

// outputPhase wires the Findings columns through OutputPhase, after first
// short-circuiting --sample-rule and --rule-audit.
func (r *runner) outputPhase() int {
	perfSnap := capturePerfSnapshot(*r.f.Perf, r.tracker, resolveParseCacheCap(*r.f.ParseCacheCapMB, r.cfg))

	if handled, code := runSampleRuleShortCircuit(r.allColumns, sampleRuleOpts{
		Rule:         *r.f.SampleRule,
		Count:        *r.f.SampleCount,
		ContextLines: *r.f.SampleContext,
		BasePath:     r.basePath,
	}); handled {
		return code
	}
	if handled, code := runRuleAuditShortCircuit(r.allColumns, *r.f.RuleAudit, RuleAuditOpts{
		MinFindings:    *r.f.RuleAuditMin,
		DetailRules:    *r.f.RuleAuditDetails,
		SamplesPerRule: *r.f.RuleAuditSamples,
		SampleContext:  *r.f.RuleAuditContext,
		ClusterFilter:  *r.f.RuleAuditCluster,
		Targets:        r.paths,
		Format:         r.effectiveFormat,
	}); handled {
		return code
	}

	warningsAsErrors := *r.f.WarningsAsErrors || r.cfg.GetTopLevelBool("warningsAsErrors", false)
	outRes, outErr := (pipeline.OutputPhase{}).Run(context.Background(), pipeline.OutputInput{
		FixupResult: pipeline.FixupResult{
			CrossFileResult: pipeline.CrossFileResult{
				DispatchResult: pipeline.DispatchResult{
					IndexResult: pipeline.IndexResult{
						ParseResult: pipeline.ParseResult{
							KotlinFiles: r.parsedFiles,
							JavaFiles:   r.outputJavaFiles,
							Paths:       r.paths,
							ActiveRules: r.activeRules,
						},
					},
					Findings: *r.allColumns,
				},
			},
		},
		Writer:           r.w,
		Format:           r.effectiveFormat,
		BasePath:         r.basePath,
		StartTime:        r.start,
		Version:          Version,
		ExperimentNames:  experiment.Current().Names(),
		PerfTimings:      perfSnap.Timings,
		PerfRuleStats:    r.perfRuleStats,
		CacheStats:       r.cacheStats,
		Caches:           perfSnap.Caches,
		CacheBudget:      perfSnap.Budget,
		WarningsAsErrors: warningsAsErrors,
		MinConfidence:    *r.f.MinConfidence,
	})
	if outErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", outErr)
		return 2
	}
	finalColumns := outRes.FinalFindings
	r.allColumns = &finalColumns

	return finalScanExit(os.Stderr, r.allColumns.Len(), time.Since(r.start), *r.f.Quiet)
}
