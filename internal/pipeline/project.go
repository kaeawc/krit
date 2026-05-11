package pipeline

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/kaeawc/krit/internal/cache"
	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/diag"
	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// LibraryFactsCache lets a long-lived host (typically *WorkspaceState)
// memoize *librarymodel.Facts across RunProject calls so the daemon
// doesn't repay Gradle/version-catalog discovery on every analyze.
// build is invoked on a fingerprint mismatch.
type LibraryFactsCache interface {
	LibraryFacts(fingerprint string, build func() *librarymodel.Facts) *librarymodel.Facts
}

// ProjectArgs is the per-call subset of ProjectInput: caller-provided
// knobs that mirror a small, stable subset of CLI flags. These change
// per request and are never stashed by the daemon.
type ProjectArgs struct {
	// Config is the loaded krit.yml / .krit.yml. Required.
	Config *config.Config
	// Paths are the scan target paths (files or directories). Required.
	Paths []string
	// ActiveRules is the rule set to dispatch. Required and non-empty.
	ActiveRules []*api.Rule
	// Format is the output format ("json", "plain", "sarif",
	// "checkstyle"). Empty defaults to "json".
	Format string
	// BaselinePath, when non-empty, points at a baseline file used to
	// suppress known findings.
	BaselinePath string
	// DiffRef, when non-empty, restricts output to files changed since
	// the given git ref.
	DiffRef string
	// MinConfidence drops findings below the threshold from output.
	MinConfidence float64
	// WarningsAsErrors promotes warning-severity findings to errors
	// before format dispatch.
	WarningsAsErrors bool
	// IncludeGenerated retains files under */generated/* during parse.
	IncludeGenerated bool
	// Workers overrides per-phase worker counts. Zero falls back to
	// runtime.NumCPU().
	Workers int
	// StartTime is the wall-clock origin used by Output's JSON header.
	// Zero means time.Now() is captured at RunProject entry.
	StartTime time.Time
	// Version is the krit version string written into JSON output.
	Version string
	// ExperimentNames are the active experiment flag names echoed in
	// JSON output.
	ExperimentNames []string
	// JSONCompact mirrors OutputInput.JSONCompact: when true the
	// "json" formatter omits indentation. The daemon's streaming
	// response path (#60) sets this; the CLI leaves it false.
	JSONCompact bool
	// OracleEnabled, when true, runs the oracle pipeline inside
	// IndexPhase (auto-detect / --input-types / --daemon paths). The
	// daemon sets this true when ensureOracleDaemon found a
	// krit-types JAR; the CLI sets it from --type-oracle.
	OracleEnabled bool
	// Fix, when true, applies safe text auto-fixes to disk between
	// cross-file analysis and output. The set of applied fixes is
	// capped by MaxFixLevel. False (default) leaves files untouched.
	Fix bool
	// FixBinary, when true, additionally applies binary-format fixes
	// (file renames, resource moves). Independent of Fix.
	FixBinary bool
	// FixSuffix, when non-empty, writes fixed content to
	// "<path><suffix>" rather than in place.
	FixSuffix string
	// DryRun, when true, runs FixupPhase in count-only mode: no text
	// fixes are applied, but FixableCount and StrippedByLevel still
	// reflect what would have been done. Also forwards a "dry run"
	// signal to binary fix application via FixupInput.DryRunBinary.
	DryRun bool
	// MaxFixLevel caps which fix levels FixupPhase applies. Zero
	// means "apply everything FixupPhase would otherwise apply".
	MaxFixLevel rules.FixLevel
	// BasePath is the base for relative paths in the formatted
	// output. Empty means OutputPhase falls back to the first scan
	// path.
	BasePath string
	// ShowPerf, when true, snapshots the host tracker + cache state
	// at OutputPhase time and forwards the result through
	// OutputInput.PerfTimings/Caches/CacheBudget.
	ShowPerf bool
	// PerfRules, when true, forwards the dispatcher's sorted per-rule
	// execution stats through OutputInput.PerfRuleStats.
	PerfRules bool
	// ParseCacheCapBytes is the effective parse-cache cap used when
	// ShowPerf builds OutputInput.CacheBudget. Zero falls back to
	// cacheutil.DefaultParseCacheCapBytes.
	ParseCacheCapBytes int64
}

// ProjectHostState is the long-lived subset of ProjectInput: state a
// daemon (or any other long-lived host) wants to keep resident across
// calls. Every field is optional; RunProject tolerates nil and lets the
// embedded phases construct fresh state when needed.
type ProjectHostState struct {
	// Reporter routes verbose progress and warning lines.
	Reporter *diag.Reporter
	// Tracker, when non-nil, wraps expensive sub-phases for --perf.
	Tracker perf.Tracker
	// ParseCache, when non-nil, is consulted by ParsePhase to skip
	// tree-sitter on files whose content hash matches a previously-
	// cached FlatTree. The daemon constructs one at startup and reuses
	// it across calls.
	ParseCache *scanner.ParseCache
	// PrebuiltResolver, when non-nil, short-circuits resolver
	// construction inside IndexPhase. The daemon keeps one resident.
	PrebuiltResolver typeinfer.TypeResolver
	// PrebuiltLibraryFacts, when non-nil, is forwarded to rule
	// contexts instead of being rebuilt from detected Gradle files.
	// Highest precedence — wins over LibraryFactsCache.
	PrebuiltLibraryFacts *librarymodel.Facts
	// LibraryFactsCache, when non-nil, is consulted in IndexPhase to
	// reuse a daemon-resident *librarymodel.Facts across calls. Cache
	// invalidation is the host's responsibility (the daemon's file
	// watcher fires WorkspaceState.InvalidateLibraryFacts on Gradle /
	// version-catalog edits). *WorkspaceState satisfies this interface.
	LibraryFactsCache LibraryFactsCache
	// CodeIndexCache, when non-nil, memoizes *scanner.CodeIndex across
	// calls. CrossFilePhase consults the slot before falling through to
	// the disk-backed cross-file cache (CrossFileCacheDir) and finally
	// to scanner.BuildIndex. *WorkspaceState satisfies this interface.
	CodeIndexCache CodeIndexCache
	// ResolverCache, when non-nil, memoizes the typeinfer.TypeResolver
	// across calls. IndexPhase consults the slot before falling through
	// to a fresh resolver + IndexFilesParallel*. The slot is keyed by
	// the sorted (path, content-hash) pairs of all indexed Kotlin
	// files, so mismatches force a complete rebuild rather than a
	// stale-entry leak. *WorkspaceState satisfies this interface.
	ResolverCache ResolverCache
	// OracleFilterCache, when non-nil and Args.OracleEnabled is true,
	// memoizes the oracle CallTargetFilterSummary across calls.
	// RunProject computes the filter once per file-set + rule-set
	// fingerprint and threads it into IndexInput.PrebuiltOracleCallFilter.
	// *WorkspaceState satisfies this interface.
	OracleFilterCache OracleFilterCache
	// CrossFileCacheDir, when non-empty, enables the on-disk cross-file
	// CodeIndex cache (zstd-encoded shards under .krit/crossfile-cache).
	// Independent of CodeIndexCache: the disk cache is shared across
	// daemon restarts, while the in-memory cache survives within a
	// single daemon's lifetime.
	CrossFileCacheDir string
	// TypeIndexCacheDir, when non-empty, enables the per-file
	// FileTypeInfo on-disk cache so warm runs skip per-file
	// extraction for unchanged files. Empty disables the cache (the
	// CLI runner sets this from typeinfer.TypeIndexCacheDir(repoDir)
	// when --no-cache is not passed; the daemon does the same).
	TypeIndexCacheDir string
	// Oracle, when non-nil, is the resident type-oracle handle.
	Oracle *oracle.Oracle
	// OracleDaemon, when non-nil, is the long-lived krit-types JVM
	// daemon handle (used only when Oracle is also set).
	OracleDaemon *oracle.Daemon
	// AnalysisCache, when non-nil, drives the incremental findings
	// cache. DispatchPhase merges new per-file findings into it and
	// saves the result to AnalysisCacheFilePath after dispatch. Nil
	// disables cache write-back entirely.
	AnalysisCache *cache.Cache
	// AnalysisCacheFilePath is where AnalysisCache is persisted on
	// dispatch write-back. Required when AnalysisCache is non-nil. The
	// daemon derives this from cache.FilePath(cacheDir, scanPaths).
	AnalysisCacheFilePath string
	// AnalysisCacheLookup, when true alongside a non-nil AnalysisCache,
	// also enables the read side: IndexPhase.runCacheLoad runs
	// cache.CheckFiles, populates CacheResult.CachedPaths, and
	// DispatchPhase skips per-file rule execution on cache hits.
	// Cached findings are merged back via mergeCachedFindings. False
	// keeps the pre-#126 write-only behavior. The daemon flips this
	// true after #126 lands. See issue #126 for the drift-risk
	// analysis (baseline/diff, MinConfidence, RuleHash drift).
	AnalysisCacheLookup bool
	// FindingsBundleStore, when non-nil, enables the whole-run findings
	// cache (#55). RunProject computes a RunFingerprint from rules +
	// config + source set + cross-file state + Android + library
	// facts; a Load hit returns the prior run's FindingColumns
	// directly and skips dispatch + cross-file work entirely. Cache
	// miss runs the full pipeline and Saves the result.
	//
	// scanner.DiskFindingsBundleStore{} satisfies this interface. nil
	// disables the bundle cache. Daemon wires this together with
	// FindingsBundleCacheRoot.
	FindingsBundleStore scanner.FindingsBundleStore
	// FindingsBundleCacheRoot is the on-disk root the bundle store
	// persists to (typically the repo dir). Empty disables the bundle
	// cache even if a store is set. The daemon supplies this from
	// scanner.FindingsBundleCacheDir's parent (the repo dir).
	FindingsBundleCacheRoot string
}

// ProjectInput is the value type that drives RunProject. The split
// between Args (per-call) and Host (long-lived) makes call sites
// self-documenting: in.Args.Format is request-scoped, in.Host.ParseCache
// is daemon-resident.
//
// The CLI's existing scan.runner remains the canonical orchestrator for
// `krit -f json`; ProjectInput exists so the daemon's analyze-project
// verb can share one execution path with the CLI without dragging in
// CLI-only concerns (CPU profiling, baseline-audit verb scaffolding,
// experiment-matrix logic, fix application, output-file routing).
type ProjectInput struct {
	Args ProjectArgs
	Host ProjectHostState
}

// ProjectResult is the value type returned from RunProject.
type ProjectResult struct {
	// JSON is the formatted output bytes (in the requested Format).
	// Populated only by RunProject; RunProjectStreaming leaves this
	// nil because the bytes are already written to the caller's
	// io.Writer. Suitable for inclusion verbatim in a daemon
	// response payload when populated.
	JSON []byte
	// FinalFindings is the set of findings actually emitted (after
	// baseline / diff / min-confidence filters).
	FinalFindings scanner.FindingColumns
	// FilesScanned is len(KotlinFiles)+len(JavaFiles) from ParseResult.
	FilesScanned int
	// FindingsCount is FinalFindings.Len().
	FindingsCount int
	// ParseErrors captures non-fatal per-file parse failures.
	ParseErrors []error
	// Stats carries the dispatcher's per-rule timing and panic counters.
	Stats rules.RunStats
	// Caches is the unified cache stats array for the run.
	Caches []cacheutil.NamedCacheStats
	// ParseHits and ParseMisses report the per-call delta against
	// ProjectInput.Host.ParseCache (when one is attached). Both stay 0 when
	// the input ran without a parse cache.
	ParseHits   int64
	ParseMisses int64
	// Fixup is the FixupPhase output for the run. Populated only when
	// the caller set one of the fix knobs in ProjectArgs (Fix,
	// FixBinary, DryRun). Zero-valued otherwise.
	Fixup FixupResult
	// FindingsBundleHit is true when the whole-run findings cache
	// (host.FindingsBundleStore) served the result without redoing
	// dispatch or cross-file analysis. False covers cache-miss runs,
	// hosts without a bundle store, and runs where the conservative
	// delta planner ran a partial dispatch instead of a full reuse.
	FindingsBundleHit bool
	// PhaseTimingsMs reports per-phase wall-time in milliseconds for
	// the run. Phases that were skipped on a bundle-cache hit (dispatch,
	// crossfile, android) report 0. Useful for diagnosing which phase
	// dominates warm-call latency without a full pprof capture.
	PhaseTimingsMs PhaseTimingsMs
}

// PhaseTimingsMs carries per-phase wall-time deltas captured around
// each phase call in RunProjectStreaming. Zero values mean the phase
// was skipped (most often on a findings-bundle hit) or completed in
// under a millisecond. Sum-of-phases is less than ProjectResult's
// total because Output writes asynchronously into the caller's
// writer and phase boundaries here exclude phase wiring overhead.
type PhaseTimingsMs struct {
	Parse     int64 `json:"parse"`
	Index     int64 `json:"index"`
	Dispatch  int64 `json:"dispatch"`
	CrossFile int64 `json:"crossfile"`
	Android   int64 `json:"android"`
	Fixup     int64 `json:"fixup"`
	Output    int64 `json:"output"`
}

// RunProject runs the core scan pipeline against the given input and
// returns the formatted output bytes alongside the parsed result.
//
// The function intentionally does not own:
//   - File enumeration: callers pass Paths; ParsePhase walks them.
//   - Configuration loading: callers pass a constructed *config.Config.
//   - Fix application, baseline-audit, FIR check: those remain CLI-only.
//   - CPU/memory profiling: those wrap RunProject at the call site.
//
// Callers that need any of the above continue to use scan.Run (the CLI
// front door) or compose the phase types directly.
func RunProject(ctx context.Context, in ProjectInput) (ProjectResult, error) {
	var buf bytes.Buffer
	res, err := RunProjectStreaming(ctx, in, &buf)
	if err != nil {
		return ProjectResult{}, err
	}
	res.JSON = buf.Bytes()
	return res, nil
}

// RunProjectStreaming is the streaming form of RunProject (#60). It runs
// the same scan pipeline but writes the OutputPhase's formatted bytes
// directly into out instead of buffering them on the heap. The returned
// ProjectResult.JSON is nil — callers that need the bytes in memory
// should wrap a bytes.Buffer (RunProject does exactly that).
//
// On the JetBrains/kotlin corpus the OutputPhase JSON is ~27 MB;
// streaming it lets the daemon write directly into the response socket
// without allocating an intermediate copy per call.
func RunProjectStreaming(ctx context.Context, in ProjectInput, out io.Writer) (ProjectResult, error) {
	startTime, format, err := validateAndDefaultStreaming(ctx, in, out)
	if err != nil {
		return ProjectResult{}, err
	}
	args := in.Args
	host := in.Host

	// Snapshot the parse-cache counters at the start of the run so the
	// post-run delta is the per-call hit/miss accounting we report back
	// to daemon clients. nil ParseCache returns the zero value.
	hits0, misses0 := parseCacheCounters(host.ParseCache)

	var phaseTimings PhaseTimingsMs

	// Phase 1: parse.
	parseStart := time.Now()
	parseResult, err := ParsePhase{Workers: args.Workers}.Run(ctx, ParseInput{
		Config:           args.Config,
		Paths:            args.Paths,
		ActiveRules:      args.ActiveRules,
		IncludeGenerated: args.IncludeGenerated,
		Workers:          args.Workers,
		Reporter:         host.Reporter,
		Tracker:          host.Tracker,
		ParseCache:       host.ParseCache,
	})
	phaseTimings.Parse = time.Since(parseStart).Milliseconds()
	if err != nil {
		return ProjectResult{}, fmt.Errorf("parse: %w", err)
	}

	// Phase 2: index. Builds resolver, library facts, code index, module
	// graph, and (when an oracle handle is supplied) wires it through.
	indexInput := IndexInput{
		ParseResult:          parseResult,
		PrebuiltResolver:     host.PrebuiltResolver,
		PrebuiltLibraryFacts: host.PrebuiltLibraryFacts,
		LibraryFactsCache:    host.LibraryFactsCache,
		CodeIndexCache:       host.CodeIndexCache,
		ResolverCache:        host.ResolverCache,
		CrossFileCacheDir:    host.CrossFileCacheDir,
		TypeIndexCacheDir:    host.TypeIndexCacheDir,
		Reporter:             host.Reporter,
		Tracker:              host.Tracker,
	}
	wireOracleHandles(&indexInput, args, host, parseResult.KotlinFiles)
	wireAnalysisCacheLookup(&indexInput, args, host)
	indexStart := time.Now()
	indexResult, err := IndexPhase{Workers: args.Workers}.Run(ctx, indexInput)
	phaseTimings.Index = time.Since(indexStart).Milliseconds()
	if err != nil {
		return ProjectResult{}, fmt.Errorf("index: %w", err)
	}
	// The daemon-resident oracle handle is wired in here rather than
	// reconstructed inside IndexPhase. Today IndexPhase does not accept
	// a prebuilt oracle on its input; expose the handles via the result
	// so DispatchPhase / CrossFilePhase see the resident state. When
	// IndexInput is later extended to accept a prebuilt oracle this
	// fallback becomes obsolete.
	if host.Oracle != nil && indexResult.Oracle == nil {
		indexResult.Oracle = host.Oracle
	}
	if host.OracleDaemon != nil && indexResult.Daemon == nil {
		indexResult.Daemon = host.OracleDaemon
	}
	indexResult.Cache = host.AnalysisCache
	if host.AnalysisCache != nil {
		// Wire the dispatch-side write-back fields so writeCacheBack
		// can update the cache and persist it on each call. Lookup-side
		// (file-skip on hit) requires CacheResult population in
		// IndexPhase.runCacheLoad and stays out of scope for this
		// promotion — populating the cache without skipping is safe
		// (no output drift) and benefits subsequent CLI runs.
		indexResult.CacheFilePath = host.AnalysisCacheFilePath
		indexResult.CacheScanPaths = args.Paths
		indexResult.Version = args.Version
		if indexResult.RuleHash == "" {
			indexResult.RuleHash = projectRuleHash(args.ActiveRules, args.Config)
		}
	}

	runFP, bundleEnabled := computeRunFingerprint(args, host, parseResult, indexResult)
	manifestData := buildManifestData(args, host, parseResult, runFP, bundleEnabled)
	dispatchResult, crossFileResult, bundleHit, dispatchTimes, err := runDispatchOrLoadBundle(ctx, args, host, indexResult, parseResult, runFP, bundleEnabled, manifestData)
	if err != nil {
		return ProjectResult{}, err
	}
	phaseTimings.Dispatch = dispatchTimes.dispatchMs
	phaseTimings.CrossFile = dispatchTimes.crossFileMs

	// Phase 4.5: Android project analysis. The findings-bundle hit
	// returns a pre-merged set (Android findings were merged before the
	// save on the original miss), so we only run AndroidPhase on misses.
	androidStart := time.Now()
	if err := runAndroidPhaseAndMerge(ctx, args, indexResult, &crossFileResult, bundleHit); err != nil {
		return ProjectResult{}, err
	}
	phaseTimings.Android = time.Since(androidStart).Milliseconds()

	fixupStart := time.Now()
	fixupView, err := runFixupPhase(ctx, args, crossFileResult)
	phaseTimings.Fixup = time.Since(fixupStart).Milliseconds()
	if err != nil {
		return ProjectResult{}, err
	}
	// Phase 5: output. Writes formatted bytes directly to the
	// caller-provided writer; #60 lets the daemon stream this into
	// the response socket without an intermediate 27 MB copy.
	perfTimings, caches, budget := capturePerfOutputs(args, host)
	var perfRuleStats []rules.RuleExecutionStat
	if args.PerfRules {
		perfRuleStats = rules.SortedRuleExecutionStats(dispatchResult.Stats)
	}
	// CacheStats is gated on ShowPerf so the daemon's analyze-project
	// JSON keeps omitting the "cache" key by default; auto-forwarding
	// would silently widen the daemon's wire format.
	var cacheStatsOut *cache.Stats
	if args.ShowPerf {
		cacheStatsOut = indexResult.CacheStats
	}
	outputStart := time.Now()
	outResult, err := OutputPhase{}.Run(ctx, OutputInput{
		FixupResult:      fixupView,
		Writer:           out,
		Format:           format,
		BaselinePath:     args.BaselinePath,
		DiffRef:          args.DiffRef,
		BasePath:         args.BasePath,
		StartTime:        startTime,
		Version:          args.Version,
		ExperimentNames:  args.ExperimentNames,
		WarningsAsErrors: args.WarningsAsErrors,
		MinConfidence:    args.MinConfidence,
		JSONCompact:      args.JSONCompact,
		ShowPerf:         args.ShowPerf,
		PerfTimings:      perfTimings,
		PerfRuleStats:    perfRuleStats,
		CacheStats:       cacheStatsOut,
		Caches:           caches,
		CacheBudget:      budget,
	})
	phaseTimings.Output = time.Since(outputStart).Milliseconds()
	if err != nil {
		return ProjectResult{}, fmt.Errorf("output: %w", err)
	}

	// Save the bundle on every miss so a subsequent identical run is
	// a load+skip. Hits don't need a re-save; the cached bundle is
	// already on disk under the same key. Best-effort: save errors
	// are not surfaced — the verb already succeeded and the run's
	// output is unaffected.
	if bundleEnabled && !bundleHit {
		_ = host.FindingsBundleStore.Save(host.FindingsBundleCacheRoot, runFP, &crossFileResult.Findings)
		_ = saveDeltaManifest(host, manifestData, runFP, &crossFileResult.Findings)
	}

	hits1, misses1 := parseCacheCounters(host.ParseCache)
	return ProjectResult{
		FinalFindings: outResult.FinalFindings,
		FilesScanned:  len(parseResult.KotlinFiles) + len(parseResult.JavaFiles),
		FindingsCount: outResult.FinalFindings.Len(),
		ParseErrors:   parseResult.ParseErrors,
		Stats:             dispatchResult.Stats,
		ParseHits:         hits1 - hits0,
		ParseMisses:       misses1 - misses0,
		Fixup:             fixupView,
		FindingsBundleHit: bundleHit,
		PhaseTimingsMs:    phaseTimings,
	}, nil
}

// capturePerfOutputs snapshots the host tracker plus the global
// cacheutil stats so OutputPhase can emit them in the JSON header.
// Returns (nil, nil, nil) when args.ShowPerf is false.
func capturePerfOutputs(args ProjectArgs, host ProjectHostState) ([]perf.TimingEntry, []cacheutil.NamedCacheStats, *cacheutil.BudgetReport) {
	if !args.ShowPerf {
		return nil, nil, nil
	}
	var timings []perf.TimingEntry
	if host.Tracker != nil && host.Tracker.IsEnabled() {
		timings = host.Tracker.GetTimings()
	}
	capBytes := args.ParseCacheCapBytes
	if capBytes == 0 {
		capBytes = cacheutil.DefaultParseCacheCapBytes
	}
	caches := cacheutil.AllStats()
	b := cacheutil.Budget(capBytes)
	return timings, caches, &b
}

// runFixupPhase invokes FixupPhase when one of the fix knobs is set.
// Fixup runs after Android merge so cached/delta findings paths
// receive on-disk fixes too — symmetric with the CLI's ordering.
func runFixupPhase(ctx context.Context, args ProjectArgs, crossFile CrossFileResult) (FixupResult, error) {
	if !args.Fix && !args.FixBinary && !args.DryRun {
		return FixupResult{CrossFileResult: crossFile}, nil
	}
	res, err := FixupPhase{}.Run(ctx, FixupInput{
		CrossFileResult: crossFile,
		Apply:           args.Fix && !args.DryRun,
		ApplyBinary:     args.FixBinary,
		Suffix:          args.FixSuffix,
		MaxFixLevel:     args.MaxFixLevel,
		DryRunBinary:    args.DryRun,
		CountOnly:       args.DryRun,
	})
	if err != nil {
		return FixupResult{}, fmt.Errorf("fixup: %w", err)
	}
	return res, nil
}

// validateAndDefaultStreaming checks RunProjectStreaming's preconditions
// and resolves the StartTime / Format defaults. Extracted to keep the
// orchestrator under the gocyclo budget.
func validateAndDefaultStreaming(ctx context.Context, in ProjectInput, out io.Writer) (time.Time, string, error) {
	if out == nil {
		return time.Time{}, "", fmt.Errorf("RunProjectStreaming: out writer is nil")
	}
	if err := ctx.Err(); err != nil {
		return time.Time{}, "", err
	}
	args := in.Args
	if args.Config == nil {
		return time.Time{}, "", fmt.Errorf("RunProject: Config is required")
	}
	if len(args.ActiveRules) == 0 {
		return time.Time{}, "", fmt.Errorf("RunProject: ActiveRules is empty")
	}
	if len(args.Paths) == 0 {
		return time.Time{}, "", fmt.Errorf("RunProject: Paths is empty")
	}
	startTime := args.StartTime
	if startTime.IsZero() {
		startTime = time.Now()
	}
	format := args.Format
	if format == "" {
		format = "json"
	}
	return startTime, format, nil
}

// parseCacheCounters extracts the cumulative Hits/Misses pair from a
// *scanner.ParseCache. nil pc returns (0, 0). RunProject snaps these
// before and after the run so the delta is the per-call accounting.
func parseCacheCounters(pc *scanner.ParseCache) (int64, int64) {
	if pc == nil {
		return 0, 0
	}
	s := pc.Stats()
	return s.Hits, s.Misses
}

// projectRuleHash mirrors IndexPhase.computeRuleHash for the daemon's
// AnalysisCache write-back path. RuleHash is set on the saved cache so
// subsequent CLI lookups reject the cache when the rule set / config
// has drifted.
func projectRuleHash(activeRules []*api.Rule, cfg *config.Config) string {
	ruleNames := make([]string, 0, len(activeRules))
	for _, r := range activeRules {
		if r != nil {
			ruleNames = append(ruleNames, r.ID)
		}
	}
	return cache.ComputeConfigHash(ruleNames, cfg, false)
}

// wireAnalysisCacheLookup turns on IndexPhase.runCacheLoad when the
// host opted into lookup mode. runCacheLoad populates CacheResult so
// DispatchPhase can skip per-file rule execution on hits; the
// existing post-IndexPhase write-back wiring stays unchanged.
//
// Cache-config fields are derived from args here (rule names from
// ActiveRules, config from args.Config, scan paths from args.Paths)
// to match how the CLI runner populates the same IndexInput slots.
// RuleHash drift between daemon and CLI is the load-bearing risk
// flagged in #126 — feeding both code paths through identical inputs
// keeps the hash byte-stable.
func wireAnalysisCacheLookup(in *IndexInput, args ProjectArgs, host ProjectHostState) {
	if host.AnalysisCache == nil || !host.AnalysisCacheLookup {
		return
	}
	in.CacheEnabled = true
	in.PreloadedAnalysisCache = host.AnalysisCache
	in.CacheFilePath = host.AnalysisCacheFilePath
	in.CacheScanPaths = args.Paths
	in.CacheConfig = args.Config
	ruleNames := make([]string, 0, len(args.ActiveRules))
	for _, r := range args.ActiveRules {
		if r != nil {
			ruleNames = append(ruleNames, r.ID)
		}
	}
	in.CacheRuleNames = ruleNames
}

// wireOracleHandles fills in the oracle-related IndexInput fields when
// the caller opted in via args.OracleEnabled and the host supplied a
// daemon handle. Extracted from RunProject to keep its cyclomatic
// complexity manageable.
func wireOracleHandles(in *IndexInput, args ProjectArgs, host ProjectHostState, kotlinFiles []*scanner.File) {
	if !args.OracleEnabled || host.OracleDaemon == nil {
		return
	}
	in.OracleEnabled = true
	in.UseDaemon = true
	in.PrebuiltOracleDaemon = host.OracleDaemon
	in.OracleScanPaths = args.Paths
	if host.OracleFilterCache == nil {
		return
	}
	fp := oracleFilterFingerprint(args.ActiveRules, kotlinFiles)
	in.PrebuiltOracleCallFilter = host.OracleFilterCache.OracleFilter(fp, func() *oracle.CallTargetFilterSummary {
		summary := rules.BuildOracleCallTargetFilterV2ForFiles(args.ActiveRules, kotlinFiles)
		return &summary
	})
}

// deltaManifestData carries the precomputed per-file inputs the delta
// planner needs. Populated by buildManifestData before dispatch so
// runDispatchOrLoadBundle can quickly compare against the prior run.
type deltaManifestData struct {
	enabled       bool
	manifestKey   string
	contentHashes map[string]string
	structuralFPs map[string]string
}

// buildManifestData prepares per-file content + structural fingerprints
// for the delta planner. Returns the inputs whether or not the bundle
// cache is enabled — callers gate on enabled before persisting.
func buildManifestData(args ProjectArgs, host ProjectHostState, parseResult ParseResult, _ scanner.RunFingerprint, bundleEnabled bool) deltaManifestData {
	if !bundleEnabled {
		return deltaManifestData{}
	}
	contentHashes := make(map[string]string, len(parseResult.KotlinFiles)+len(parseResult.JavaFiles))
	structuralFPs := make(map[string]string, len(parseResult.KotlinFiles)+len(parseResult.JavaFiles))
	for _, f := range parseResult.KotlinFiles {
		if f == nil {
			continue
		}
		contentHashes[f.Path] = hashutil.Default().HashContent(f.Path, f.Content)
		structuralFPs[f.Path] = scanner.FileStructuralFingerprint(f)
	}
	for _, f := range parseResult.JavaFiles {
		if f == nil {
			continue
		}
		contentHashes[f.Path] = hashutil.Default().HashContent(f.Path, f.Content)
		structuralFPs[f.Path] = scanner.FileStructuralFingerprint(f)
	}
	return deltaManifestData{
		enabled:       true,
		manifestKey:   scanner.FindingsBundleManifestKey(host.FindingsBundleCacheRoot, args.Paths),
		contentHashes: contentHashes,
		structuralFPs: structuralFPs,
	}
}

// saveDeltaManifest persists the current run's manifest entry so a
// future run can detect single-file changes for the delta path.
// Best-effort — manifest write failures don't fail the verb.
func saveDeltaManifest(host ProjectHostState, m deltaManifestData, runFP scanner.RunFingerprint, _ *scanner.FindingColumns) error {
	if !m.enabled || m.manifestKey == "" {
		return nil
	}
	return scanner.SaveFindingsBundleManifest(host.FindingsBundleCacheRoot, m.manifestKey, scanner.FindingsBundleManifest{
		BundleKey:     scanner.FindingsBundleKey(runFP),
		Fingerprint:   runFP,
		ContentHashes: m.contentHashes,
		StructuralFPs: m.structuralFPs,
	})
}

// runDispatchOrLoadBundle resolves dispatch + cross-file findings via
// three increasingly conservative paths:
//
//  1. Full-fingerprint Load. RunFingerprint identical to a prior run →
//     replay cached bundle, skip dispatch + cross-file entirely.
//  2. Delta path. Prior manifest exists, planner says exactly 1 file
//     changed + every non-SourceSet fingerprint field is stable →
//     dispatch only on the changed file, optionally re-run cross-file
//     rules (no scope savings there), filter replacement to the
//     changed path, ApplyDelta against the prior bundle.
//  3. Full dispatch. Normal DispatchPhase + CrossFilePhase.
//
// The delta path is the load-bearing #55 perf win for body-only edits:
// per-file rule dispatch is the dominant warm-call cost and the delta
// path runs it on just one file.

// runAndroidPhaseAndMerge runs AndroidPhase against the detected
// project (if any) and folds its findings into crossFileResult.Findings.
// A nil/empty project, no Android-needing rules, or a dispatcher-less
// run returns without mutating the findings. Mirrors the CLI runner's
// androidPhase step.
func runAndroidPhaseAndMerge(ctx context.Context, args ProjectArgs, indexResult IndexResult, crossFileResult *CrossFileResult, bundleHit bool) error {
	if bundleHit {
		return nil
	}
	project := indexResult.AndroidProject
	if project == nil || project.IsEmpty() {
		return nil
	}
	dispatcher := rules.NewDispatcher(args.ActiveRules, indexResult.Resolver)
	dispatcher.SetLibraryFacts(indexResult.LibraryFacts)
	dispatcher.SetJavaSemanticFacts(indexResult.JavaSemanticFacts)
	res, err := (AndroidPhase{}).Run(ctx, AndroidInput{
		Project:             project,
		ActiveRules:         args.ActiveRules,
		Dispatcher:          dispatcher,
		SourceFiles:         indexResult.SourceFiles(),
		RuleHash:            indexResult.RuleHash,
		LibraryFactsFP:      indexResult.LibraryFacts.Fingerprint(),
		JavaSemanticFactsFP: indexResult.JavaSemanticFacts.Fingerprint(),
	})
	if err != nil {
		return fmt.Errorf("android: %w", err)
	}
	if res.Findings.Len() == 0 {
		return nil
	}
	merged := scanner.NewFindingCollector(crossFileResult.Findings.Len() + res.Findings.Len())
	merged.AppendColumns(&crossFileResult.Findings)
	merged.AppendColumns(&res.Findings)
	crossFileResult.Findings = *merged.Columns()
	return nil
}

// dispatchTimings is the per-phase wall-time slice returned from
// runDispatchOrLoadBundle so RunProjectStreaming can populate
// PhaseTimingsMs even on the bundle/delta paths where the phase
// boundaries are inside this helper.
type dispatchTimings struct {
	dispatchMs  int64
	crossFileMs int64
}

func runDispatchOrLoadBundle(
	ctx context.Context,
	args ProjectArgs,
	host ProjectHostState,
	indexResult IndexResult,
	parseResult ParseResult,
	runFP scanner.RunFingerprint,
	bundleEnabled bool,
	manifest deltaManifestData,
) (DispatchResult, CrossFileResult, bool, dispatchTimings, error) {
	if bundleEnabled {
		if cached, ok := host.FindingsBundleStore.Load(host.FindingsBundleCacheRoot, runFP); ok && cached != nil {
			d := DispatchResult{IndexResult: indexResult, Findings: *cached}
			return d, CrossFileResult{DispatchResult: d}, true, dispatchTimings{}, nil
		}
		if d, c, ok, err := tryDeltaDispatch(ctx, args, host, indexResult, parseResult, runFP, manifest); err != nil {
			return DispatchResult{}, CrossFileResult{}, false, dispatchTimings{}, err
		} else if ok {
			return d, c, false, dispatchTimings{}, nil
		}
	}
	dispatchStart := time.Now()
	d, err := DispatchPhase{}.Run(ctx, indexResult)
	dispatchMs := time.Since(dispatchStart).Milliseconds()
	if err != nil {
		return DispatchResult{}, CrossFileResult{}, false, dispatchTimings{}, fmt.Errorf("dispatch: %w", err)
	}
	crossStart := time.Now()
	c, err := CrossFilePhase{Workers: args.Workers}.Run(ctx, d)
	crossMs := time.Since(crossStart).Milliseconds()
	if err != nil {
		return DispatchResult{}, CrossFileResult{}, false, dispatchTimings{}, fmt.Errorf("crossfile: %w", err)
	}
	return d, c, false, dispatchTimings{dispatchMs: dispatchMs, crossFileMs: crossMs}, nil
}

// tryDeltaDispatch attempts the ConservativeDeltaPlanner's single-file
// delta path. Returns (_, _, false, nil) when the prior manifest is
// missing, the planner refuses (multi-file change, non-SourceSet
// fingerprint drift, etc.), or any other safety check fails. The
// caller falls back to full dispatch in that case.
//
// On a successful delta:
//   - DispatchPhase runs against an IndexResult scoped to just the
//     changed file. Per-file rules see only the dirty file.
//   - CrossFilePhase still runs with the full IndexResult.CodeIndex
//     because cross-file rule outputs depend on the aggregate
//     structural state, which the planner gate guarantees is stable.
//     The replacement is filtered to the changed path post-hoc.
//   - scanner.ApplyDelta(prior, replacement, [changedPath]) produces
//     the merged FindingColumns.
func tryDeltaDispatch(
	ctx context.Context,
	args ProjectArgs,
	host ProjectHostState,
	indexResult IndexResult,
	parseResult ParseResult,
	runFP scanner.RunFingerprint,
	manifest deltaManifestData,
) (DispatchResult, CrossFileResult, bool, error) {
	if !manifest.enabled || manifest.manifestKey == "" {
		return DispatchResult{}, CrossFileResult{}, false, nil
	}
	prior, ok := scanner.LoadFindingsBundleManifest(host.FindingsBundleCacheRoot, manifest.manifestKey)
	if !ok {
		return DispatchResult{}, CrossFileResult{}, false, nil
	}
	changedPaths := diffContentHashes(prior.ContentHashes, manifest.contentHashes)
	plan := scanner.ConservativeDeltaPlanner{}.Plan(prior.Fingerprint, runFP, changedPaths)
	if !plan.ReusePrevious || len(plan.ChangedPaths) != 1 {
		return DispatchResult{}, CrossFileResult{}, false, nil
	}
	priorBundle, ok := host.FindingsBundleStore.Load(host.FindingsBundleCacheRoot, prior.Fingerprint)
	if !ok || priorBundle == nil {
		return DispatchResult{}, CrossFileResult{}, false, nil
	}

	changedPath := plan.ChangedPaths[0]
	changedFile := findFileByPath(parseResult, changedPath)
	if changedFile == nil {
		return DispatchResult{}, CrossFileResult{}, false, nil
	}

	scoped := indexResult
	scoped.ParseResult = scopeParseResult(parseResult, changedFile)
	d, err := DispatchPhase{}.Run(ctx, scoped)
	if err != nil {
		return DispatchResult{}, CrossFileResult{}, false, fmt.Errorf("dispatch (delta): %w", err)
	}
	c, err := CrossFilePhase{Workers: args.Workers}.Run(ctx, d)
	if err != nil {
		return DispatchResult{}, CrossFileResult{}, false, fmt.Errorf("crossfile (delta): %w", err)
	}

	replacement := scanner.FilterColumnsByFilePaths(&c.Findings, map[string]bool{changedPath: true})
	merged := scanner.ApplyDelta(priorBundle, &replacement, []string{changedPath})

	d.Findings = merged
	c.DispatchResult = d
	c.Findings = merged
	return d, c, true, nil
}

// diffContentHashes returns the sorted list of paths where the
// current content hash differs from the prior, OR where a file
// exists in only one side (treated as a change). Empty if the two
// maps describe an identical file set with matching hashes.
func diffContentHashes(prior, current map[string]string) []string {
	seen := make(map[string]bool, len(prior)+len(current))
	for path := range prior {
		seen[path] = true
	}
	for path := range current {
		seen[path] = true
	}
	changed := make([]string, 0)
	for path := range seen {
		if prior[path] != current[path] {
			changed = append(changed, path)
		}
	}
	sort.Strings(changed)
	return changed
}

func findFileByPath(p ParseResult, path string) *scanner.File {
	for _, f := range p.KotlinFiles {
		if f != nil && f.Path == path {
			return f
		}
	}
	for _, f := range p.JavaFiles {
		if f != nil && f.Path == path {
			return f
		}
	}
	return nil
}

// scopeParseResult returns a copy of p with KotlinFiles + JavaFiles
// narrowed to the single supplied file. The narrowed result feeds
// DispatchPhase + CrossFilePhase in the delta path so per-file rules
// only run on the dirty file. IndexResult.CodeIndex retains the full
// project index (cross-file rules need the aggregate view).
func scopeParseResult(p ParseResult, only *scanner.File) ParseResult {
	scoped := p
	scoped.KotlinFiles = nil
	scoped.JavaFiles = nil
	if only == nil {
		return scoped
	}
	if only.Language == scanner.LangJava {
		scoped.JavaFiles = []*scanner.File{only}
	} else {
		scoped.KotlinFiles = []*scanner.File{only}
	}
	return scoped
}

// computeRunFingerprint builds a scanner.RunFingerprint from the run's
// inputs so the whole-run findings cache can key on it. Returns a
// (fingerprint, enabled) pair: enabled is false when the host didn't
// wire a store + root, so callers can skip the cache plumbing.
//
// Every field that affects rule output flows in:
//
//   - Version: args.Version (the binary's release identifier; bumps
//     after the wire format / output shape changes).
//   - Rules: cache.ComputeConfigHash over the active rule IDs + Config.
//     Drift in either invalidates the bundle.
//   - Config: same hash as Rules today; kept separate so a future
//     split (e.g. rule-set hash vs. user-tunable knobs) doesn't
//     require a fingerprint shape change.
//   - SourceSet: sorted (path, content-hash) pairs of every Kotlin
//     and Java file in the parse result.
//   - CrossFile: indexResult.CodeIndex.Fingerprint when present;
//     captures the cross-file symbol/reference index.
//   - Android: sorted Gradle paths from the detected Android project.
//   - LibraryFacts: indexResult.LibraryFacts.Fingerprint when present.
//
// The function is deliberately conservative: any input change
// produces a fresh fingerprint and forces a fresh dispatch. The
// delta planner's "exactly 1 file changed and cross-file is stable"
// optimisation is out of scope for this PR (#55 PR-A) — that path
// requires a structural CrossFile fingerprint that doesn't move with
// every byte change. Pending follow-up.
func computeRunFingerprint(args ProjectArgs, host ProjectHostState, parseResult ParseResult, indexResult IndexResult) (scanner.RunFingerprint, bool) {
	if host.FindingsBundleStore == nil || host.FindingsBundleCacheRoot == "" {
		return scanner.RunFingerprint{}, false
	}
	rulesHash := projectRuleHash(args.ActiveRules, args.Config)
	fp := scanner.RunFingerprint{
		Version:   args.Version,
		Rules:     rulesHash,
		Config:    rulesHash,
		SourceSet: sourceSetFingerprint(parseResult.KotlinFiles, parseResult.JavaFiles),
	}
	fp.CrossFile = crossFileStructuralFingerprint(parseResult.KotlinFiles, parseResult.JavaFiles)
	if indexResult.AndroidProject != nil {
		fp.Android = libraryFactsFingerprint(indexResult.AndroidProject.GradlePaths)
	}
	if indexResult.LibraryFacts != nil {
		fp.LibraryFacts = indexResult.LibraryFacts.Fingerprint()
	}
	return fp, true
}

// crossFileStructuralFingerprint hashes sorted (path, structural-fp)
// pairs of every parsed Kotlin and Java file. The per-file structural
// fingerprint excludes body content (positions, whitespace, comments
// that aren't KDoc references) so intra-file body edits — the steady-
// state dev-loop case — keep the aggregate CrossFile fp stable. That
// stability is the gate the ConservativeDeltaPlanner needs to take
// the single-file delta path.
//
// Any change to a file's Symbols (added/removed/renamed/visibility/
// signature) or References (added/removed names) moves its structural
// fp, which moves the aggregate, which makes the planner refuse the
// delta and fall back to full dispatch. Safe-by-default.
func crossFileStructuralFingerprint(kotlinFiles, javaFiles []*scanner.File) string {
	entries := make([]string, 0, len(kotlinFiles)+len(javaFiles))
	add := func(f *scanner.File) {
		if f == nil {
			return
		}
		entries = append(entries, f.Path+"\x00"+scanner.FileStructuralFingerprint(f))
	}
	for _, f := range kotlinFiles {
		add(f)
	}
	for _, f := range javaFiles {
		add(f)
	}
	sort.Strings(entries)
	return hashutil.HashHex([]byte(strings.Join(entries, "\x01")))
}

// sourceSetFingerprint hashes sorted (path, content-hash) tuples of
// every parsed Kotlin and Java file. Any added, removed, renamed, or
// edited file moves the fingerprint, forcing a bundle miss.
func sourceSetFingerprint(kotlinFiles, javaFiles []*scanner.File) string {
	entries := make([]string, 0, len(kotlinFiles)+len(javaFiles))
	add := func(f *scanner.File) {
		if f == nil {
			return
		}
		entries = append(entries, f.Path+"\x00"+hashutil.Default().HashContent(f.Path, f.Content))
	}
	for _, f := range kotlinFiles {
		add(f)
	}
	for _, f := range javaFiles {
		add(f)
	}
	sort.Strings(entries)
	return hashutil.HashHex([]byte(strings.Join(entries, "\x01")))
}

// oracleFilterFingerprint hashes the active rule IDs together with
// the sorted (path, content-hash) tuples of every Kotlin file. Both
// inputs feed BuildOracleCallTargetFilterV2ForFiles; either changing
// invalidates the cached filter.
func oracleFilterFingerprint(activeRules []*api.Rule, kotlinFiles []*scanner.File) string {
	ruleIDs := make([]string, 0, len(activeRules))
	for _, r := range activeRules {
		if r != nil {
			ruleIDs = append(ruleIDs, r.ID)
		}
	}
	sort.Strings(ruleIDs)
	fileEntries := make([]string, 0, len(kotlinFiles))
	for _, f := range kotlinFiles {
		if f == nil {
			continue
		}
		fileEntries = append(fileEntries, f.Path+"\x00"+hashutil.Default().HashContent(f.Path, f.Content))
	}
	sort.Strings(fileEntries)
	return hashutil.HashHex([]byte(strings.Join(ruleIDs, "\x01") + "\x02" + strings.Join(fileEntries, "\x01")))
}
