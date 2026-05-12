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
	"github.com/kaeawc/krit/internal/firchecks"
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
	outputJavaFiles []*scanner.File
	moduleGraph     *module.Graph
	pmi             *module.PerModuleIndex

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
	effectiveFormat := resolveEffectiveFormat(f)

	runVersionFlag(*f.Version, Version)
	runClearMatrixCacheFlag(*f.ClearMatrixCache)
	runPromoteExperimentFlag(*f.PromoteExperiment)
	runDeprecateExperimentFlag(*f.DeprecateExperiment)
	runListExperimentsFlag(*f.ListExperiments, effectiveFormat, Version)
	runCompletionsFlag(*f.Completions)
	runInitFlag(*f.Init)
	runDoctorFlag(*f.Doctor, Version)
	runGenerateSchemaFlag(*f.GenerateSchema)

	cfg := loadScanConfig(f)
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

	maxFixLevel, ok := resolveMaxFixLevel(f)
	if !ok {
		return nil, 2, false
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
	r.preloadOracleTypes()
	r.startAnalysisCachePreload()
	return r, 0, true
}

func resolveEffectiveFormat(f *scanFlags) string {
	// Resolve output format: --report takes precedence over -f. If no
	// format is explicitly set and stdout is a TTY, auto-promote to
	// "plain" (respecting NO_COLOR and the -o file redirect).
	effectiveFormat := *f.Format
	if *f.Report != "" {
		return *f.Report
	}
	if *f.Format == "json" && *f.Output == "" {
		if _, noColor := os.LookupEnv("NO_COLOR"); !noColor {
			if fi, err := os.Stdout.Stat(); err == nil && fi.Mode()&os.ModeCharDevice != 0 {
				effectiveFormat = "plain"
			}
		}
	}
	return effectiveFormat
}

func loadScanConfig(f *scanFlags) *config.Config {
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
	return cfg
}

func resolveMaxFixLevel(f *scanFlags) (rules.FixLevel, bool) {
	if !*f.Fix && !*f.DryRun {
		return rules.FixIdiomatic, true
	}
	parsed, ok := rules.ParseFixLevel(*f.FixLevel)
	if !ok {
		fmt.Fprintf(os.Stderr, "error: invalid fix level '%s'. Use: cosmetic, idiomatic, semantic\n", *f.FixLevel)
		return rules.FixIdiomatic, false
	}
	return parsed, true
}

func (r *runner) preloadOracleTypes() {
	if *r.f.NoTypeOracle || *r.f.NoCacheOracle {
		return
	}
	oraclePath := *r.f.InputTypes
	if oraclePath == "" {
		oraclePath = oracle.CachePath(r.paths)
	}
	if oraclePath == "" {
		return
	}
	if _, err := os.Stat(oraclePath); err == nil {
		oracle.PreloadPath(oraclePath)
	}
}

func (r *runner) startAnalysisCachePreload() {
	// Skip the preload on early-exit paths that never reach runOracleIndex.
	// --oracle-filter-fingerprint is the only one detectable here without
	// doing the file walk first; the empty-repo short-circuit needs file
	// collection to fire and isn't worth a pre-walk just to save the wasted
	// load.
	if *r.f.NoCache || r.cacheFilePath == "" || *r.f.OracleFilterFingerprint {
		return
	}
	r.analysisCacheLoadFuture = pipeline.NewAnalysisCacheLoadFuture(func() *cache.Cache {
		return cache.Load(r.cacheFilePath)
	})
	r.analysisCacheLoadFuture.Start()
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

			PreloadedAnalysisCache: nil,
		}
		res, err = (pipeline.IndexPhase{
			SkipModules:       true,
			SkipAndroid:       true,
			SkipResolverIndex: true,
		}).Run(context.Background(), in)
	})
	if r.useCache && r.analysisCacheLoadFuture != nil {
		preloadedCache = r.analysisCacheLoadFuture.Await()
		// Surface the actual load wall-time as a perf entry so warm
		// runs don't read 0ms cacheLoad — the pipeline's trackSerial
		// wraps the receive, which is near-instant on the preloaded
		// path (see #67/#84).
		perf.AddEntry(r.tracker, "cacheLoadAsync", r.analysisCacheLoadFuture.Duration())
		r.analysisCacheLoadFuture = nil
	}
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
		r.analysisCache = preloadedCache
		if r.analysisCache == nil {
			r.analysisCache = cache.Load(r.cacheFilePath)
		}
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

		if !*r.f.NoParseCache {
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
		if shouldOpenResourceIndexCache(r.activeRules, *r.f.NoResourceCache) {
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

// firCheckAndCollect runs the FIR checker pass (gated by --fir / --no-fir)
// and finalizes findings into the columnar form used by output.
func (r *runner) firCheckAndCollect() {
	r.tracker.TrackVoid("firCheckAndCollect", func() {
		enabled := *r.f.Fir && !*r.f.NoFir
		var checker firchecks.FirChecker
		if enabled {
			checker = &firchecks.ProductionFirChecker{
				JarPath:   firchecks.FindFirJar(r.paths),
				RepoDir:   oracle.FindRepoDir(r.paths),
				UseDaemon: !*r.f.NoFirDaemon,
				Verbose:   *r.f.Verbose,
			}
		}
		r.allFindings = runFIRCheckerPass(firCheckerOpts{
			Enabled:     enabled,
			Checker:     checker,
			Verbose:     *r.f.Verbose,
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
		JSONCompact:      *r.f.Output != "",
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
