package pipeline

import (
	"bytes"
	"context"
	"fmt"
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
	// OracleEnabled, when true, runs the oracle pipeline inside
	// IndexPhase (auto-detect / --input-types / --daemon paths). The
	// daemon sets this true when ensureOracleDaemon found a
	// krit-types JAR; the CLI sets it from --type-oracle.
	OracleEnabled bool
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
	// Suitable for inclusion verbatim in a daemon response payload.
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
	if err := ctx.Err(); err != nil {
		return ProjectResult{}, err
	}
	args := in.Args
	host := in.Host
	if args.Config == nil {
		return ProjectResult{}, fmt.Errorf("RunProject: Config is required")
	}
	if len(args.ActiveRules) == 0 {
		return ProjectResult{}, fmt.Errorf("RunProject: ActiveRules is empty")
	}
	if len(args.Paths) == 0 {
		return ProjectResult{}, fmt.Errorf("RunProject: Paths is empty")
	}

	startTime := args.StartTime
	if startTime.IsZero() {
		startTime = time.Now()
	}
	format := args.Format
	if format == "" {
		format = "json"
	}

	// Snapshot the parse-cache counters at the start of the run so the
	// post-run delta is the per-call hit/miss accounting we report back
	// to daemon clients. nil ParseCache returns the zero value.
	hits0, misses0 := parseCacheCounters(host.ParseCache)

	// Phase 1: parse.
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
	indexResult, err := IndexPhase{Workers: args.Workers}.Run(ctx, indexInput)
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

	// Phase 3: dispatch (per-file rules).
	dispatchResult, err := DispatchPhase{}.Run(ctx, indexResult)
	if err != nil {
		return ProjectResult{}, fmt.Errorf("dispatch: %w", err)
	}

	// Phase 4: cross-file rules.
	crossFileResult, err := CrossFilePhase{Workers: args.Workers}.Run(ctx, dispatchResult)
	if err != nil {
		return ProjectResult{}, fmt.Errorf("crossfile: %w", err)
	}

	// Phase 5: output to an in-memory buffer.
	var buf bytes.Buffer
	fixupView := FixupResult{CrossFileResult: crossFileResult}
	outResult, err := OutputPhase{}.Run(ctx, OutputInput{
		FixupResult:      fixupView,
		Writer:           &buf,
		Format:           format,
		BaselinePath:     args.BaselinePath,
		DiffRef:          args.DiffRef,
		StartTime:        startTime,
		Version:          args.Version,
		ExperimentNames:  args.ExperimentNames,
		WarningsAsErrors: args.WarningsAsErrors,
		MinConfidence:    args.MinConfidence,
	})
	if err != nil {
		return ProjectResult{}, fmt.Errorf("output: %w", err)
	}

	hits1, misses1 := parseCacheCounters(host.ParseCache)
	return ProjectResult{
		JSON:          buf.Bytes(),
		FinalFindings: outResult.FinalFindings,
		FilesScanned:  len(parseResult.KotlinFiles) + len(parseResult.JavaFiles),
		FindingsCount: outResult.FinalFindings.Len(),
		ParseErrors:   parseResult.ParseErrors,
		Stats:         dispatchResult.Stats,
		ParseHits:     hits1 - hits0,
		ParseMisses:   misses1 - misses0,
	}, nil
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
