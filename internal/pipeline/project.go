package pipeline

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/cache"
	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/diag"
	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/javafacts"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/output"
	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// LibraryFactsCache memoizes *librarymodel.Facts across RunProject
// calls so the daemon doesn't repay Gradle/version-catalog discovery
// on every analyze.
type LibraryFactsCache = XFileCache[*librarymodel.Facts]

// ProjectArgs is the per-call subset of ProjectInput: caller-provided
// knobs that mirror a small, stable subset of CLI flags. These change
// per request and are never stashed by the daemon.
type ProjectArgs struct {
	// Config is the loaded krit.yml / .krit.yml. Required.
	Config *config.Config
	// Paths are the scan target paths (files or directories). Required.
	Paths []string
	// KotlinPaths, when non-nil, are the already-collected Kotlin
	// sources for this run. Supplying them lets RunProject perform
	// pre-parse cache lookup and parse only cache misses.
	KotlinPaths []string
	// JavaPaths, when non-nil, are the already-collected Java sources
	// for this run. Supplying them keeps Java cache lookup and parse
	// inputs aligned with the caller's file walk.
	JavaPaths []string
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
	// EditorConfigEnabled participates in the analysis-cache rule hash.
	// CLI callers set this from --editorconfig; false preserves the
	// daemon's existing hash contract.
	EditorConfigEnabled bool
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
	// CustomRuleJars are Kotlin custom-rule plugin jars loaded by the
	// krit-types daemon and merged into the standard finding stream.
	CustomRuleJars []string
	// TargetedResolution, when true, runs active rules' expression
	// selectors through the configured expression resolver before
	// dispatch. The pass is optional: missing resolver/sink or a rule
	// set with no selectors is treated as no work.
	TargetedResolution bool
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
	// ProfileDispatch records per-file dispatch timings in DispatchResult.
	ProfileDispatch bool
	// EmitPerFileStats adds detailed dispatch timing entries under the
	// ruleExecution tracker.
	EmitPerFileStats bool
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
	// ResidentFiles, when non-nil, is the daemon's per-path *scanner.File
	// cache. ParsePhase checks it before any disk read. See
	// ParseInput.ResidentFiles.
	ResidentFiles ResidentFileCache
	// PrebuiltResolver, when non-nil, short-circuits resolver
	// construction inside IndexPhase. The daemon keeps one resident.
	PrebuiltResolver typeinfer.TypeResolver
	// PrebuiltLibraryFacts, when non-nil, is forwarded to rule
	// contexts instead of being rebuilt from detected Gradle files.
	// Highest precedence — wins over LibraryFactsCache.
	PrebuiltLibraryFacts *librarymodel.Facts
	// PrebuiltAndroidProject, when non-nil, is forwarded to IndexPhase and
	// preparse fingerprint helpers instead of redetecting Android layout.
	PrebuiltAndroidProject *android.Project
	// JavaSemanticFacts, when non-nil, supplies javac-backed facts for
	// Java rules that request them.
	JavaSemanticFacts *javafacts.Facts
	// JavaSemanticFactsLoader lazily builds JavaSemanticFacts after parse
	// when active Java rules request compiler-backed facts.
	JavaSemanticFactsLoader func(context.Context, []string, []*scanner.File, *librarymodel.Facts, perf.Tracker) (*javafacts.Facts, string, error)
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
	// CodeIndexSnapshotLoader returns the daemon-resident prior
	// CodeIndex along with the meta it was built from, surviving
	// every watcher invalidation of CodeIndexCache. runCodeIndexBuild
	// hands it to scanner.BuildIndexCachedWithPrior so an overlay
	// rebuild reuses the in-memory prior instead of paying the
	// ~2.6 s disk decode on every .kt edit.
	// *WorkspaceState.LoadCodeIndexSnapshot satisfies the shape.
	CodeIndexSnapshotLoader func() (*scanner.CodeIndex, scanner.CrossFileCacheMeta, bool)
	// CodeIndexSnapshotSaver records the just-built CodeIndex and
	// meta as the new daemon-resident snapshot. Called after every
	// successful BuildIndexCachedWithPrior so the next analyze sees
	// it via the loader.
	// *WorkspaceState.StoreCodeIndexSnapshot satisfies the shape.
	CodeIndexSnapshotSaver func(*scanner.CodeIndex, scanner.CrossFileCacheMeta)
	// JavaSourceIndexCache, when non-nil, lets CrossFilePhase short-
	// circuit the ~100 ms content-hash key SourceIndexForFiles otherwise
	// computes on every warm call. The watcher's .java events drive
	// invalidation through WorkspaceState.BumpJavaSourceVersion.
	// *WorkspaceState.JavaSourceIndex satisfies the callback shape.
	JavaSourceIndexCache func(build func() *javafacts.SourceIndex) *javafacts.SourceIndex
	// ResolverCache, when non-nil, memoizes the typeinfer.TypeResolver
	// across calls. IndexPhase consults the slot before falling through
	// to a fresh resolver + IndexFilesParallel*. The slot is keyed by
	// the sorted (path, content-hash) pairs of all indexed Kotlin
	// files, so mismatches force a complete rebuild rather than a
	// stale-entry leak. *WorkspaceState satisfies this interface.
	ResolverCache ResolverCache
	// ResolverFingerprintCache, when non-nil, short-circuits the
	// resolverFingerprint compute that gates ResolverCache. The
	// fingerprint hashes every Kotlin file's content; on the kotlin
	// corpus that's ~135 ms even when the resolver itself is cached.
	// The cache returns the prior fingerprint when no source-path
	// watcher event has fired since the last successful compute.
	// *WorkspaceState.ResolverFingerprint satisfies the callback shape.
	ResolverFingerprintCache func(build func() string) string
	// OracleFilterCache, when non-nil and Args.OracleEnabled is true,
	// memoizes the oracle CallTargetFilterSummary across calls.
	// RunProject computes the filter once per file-set + rule-set
	// fingerprint and threads it into IndexInput.PrebuiltOracleCallFilter.
	// *WorkspaceState satisfies this interface.
	OracleFilterCache OracleFilterCache
	// AndroidProjectCache, when non-nil, memoizes the detected
	// *android.Project across calls. IndexPhase.detectAndroidProject
	// consults the slot before falling through to android.DetectProject
	// (which walks the source tree for AndroidManifest.xml + build.gradle
	// markers — ~1s on a 60k-file corpus). The fingerprint is the
	// repo-root scan-path set; the watcher's InvalidateLibraryFacts hook
	// (fired on build.gradle / version-catalog edits) also drops this
	// slot. *WorkspaceState satisfies this interface.
	AndroidProjectCache AndroidProjectCache
	// GradleFindingsCache, when non-nil, memoizes per-gradle-file
	// rule-dispatch findings across analyzes (~7ms each × hundreds of
	// gradle files on a kotlin-style monorepo = ~1.4s saved per warm
	// analyze). Daemon callers wire WorkspaceState.GradleFindings.
	// CLI passes nil — the existing on-disk AndroidCacheWriter path
	// covers that case.
	GradleFindingsCache func(key string, build func() scanner.FindingColumns) scanner.FindingColumns
	// BundleStatsClean / MarkBundleStatsClean are the daemon's
	// watcher-gated short-circuit for the manifest fileStatsMatch
	// sweep. When BundleStatsClean(key) returns true, preparseBundle-
	// FingerprintTracked skips the 18k os.Stat syscalls — the
	// watcher has guaranteed nothing changed since the last
	// successful match. Either both are nil (CLI path / first
	// daemon call) or both are set together. *WorkspaceState
	// satisfies the contract.
	BundleStatsClean     func(bundleKey string) bool
	MarkBundleStatsClean func(bundleKey string, version uint64)
	// SourceMTimeVersion returns the watcher's current version
	// counter. Callers snapshot it before the stat sweep so a
	// concurrent watcher event correctly invalidates the resulting
	// memo. nil is permitted and behaves like a constant 0 (no
	// caching).
	SourceMTimeVersion func() uint64
	// BundleOutput / StoreBundleOutput are the daemon-side cache for
	// pre-formatted bundle-hit JSON. When BundleOutput(key) returns
	// non-nil, the bundle-hit OutputPhase short-circuit emits the
	// cached findings bytes verbatim and rebuilds only the dynamic
	// envelope fields (durationMs, perf stats). Skips ~24 ms of
	// json.Marshal-equivalent byte concatenation on every warm
	// analyze. *WorkspaceState satisfies both signatures.
	BundleOutput      func(bundleKey string) *CachedBundleOutput
	StoreBundleOutput func(bundleKey string, output *CachedBundleOutput)
	// CrossFileCacheDir, when non-empty, enables the on-disk cross-file
	// CodeIndex cache (zstd-encoded shards under .krit/crossfile-cache).
	// Independent of CodeIndexCache: the disk cache is shared across
	// daemon restarts, while the in-memory cache survives within a
	// single daemon's lifetime.
	CrossFileCacheDir string
	// CrossFindingsCacheDir, when non-empty, enables the on-disk
	// cross-rule findings cache. RunProject can replay it on fully warm
	// analysis-cache runs without parsing source files.
	CrossFindingsCacheDir string
	// TypeIndexCacheDir, when non-empty, enables the per-file
	// FileTypeInfo on-disk cache so warm runs skip per-file
	// extraction for unchanged files. Empty disables the cache (the
	// CLI runner sets this from typeinfer.TypeIndexCacheDir(repoDir)
	// when --no-cache is not passed; the daemon does the same).
	TypeIndexCacheDir string
	// ResidentFileTypeInfo is the daemon's in-memory cache for
	// per-file *typeinfer.FileTypeInfo, consulted before the
	// disk-backed TypeIndexCacheDir. On a 18k-file warm baseline
	// this turns 18 k disk-cache hits into 18 k map lookups (~180 ms
	// → ~5 ms in typeIndex.perFileExtraction). Watcher's
	// Invalidate(path) drops the corresponding entry.
	// *WorkspaceState satisfies the interface.
	ResidentFileTypeInfo typeinfer.ResidentFileTypeInfoCache
	// AndroidProviders, AndroidCacheDir, and AndroidCacheWriter let CLI
	// callers keep Android project cache behavior while sharing the
	// core RunProjectAnalysis path.
	AndroidProviders   *AndroidProjectProviders
	AndroidCacheDir    string
	AndroidCacheWriter *scanner.AndroidCacheWriter
	// Oracle, when non-nil, is the resident type-oracle handle.
	Oracle *oracle.Oracle
	// OracleDaemon, when non-nil, is the long-lived krit-types JVM
	// daemon handle (used only when Oracle is also set).
	OracleDaemon *oracle.Daemon
	// TargetedExpressionResolver and TargetedExpressionSink override
	// the daemon/oracle pair used by TargetedResolution. Tests and
	// embedders can supply narrow fakes without standing up a JVM.
	TargetedExpressionResolver api.ExpressionTypeResolver
	TargetedExpressionSink     api.ExpressionFactSink
	// AnalysisCache, when non-nil, drives the incremental findings
	// cache. DispatchPhase merges new per-file findings into it and
	// saves the result to AnalysisCacheFilePath after dispatch. Nil
	// disables cache write-back entirely.
	AnalysisCache *cache.Cache
	// AnalysisCacheLoadFuture supplies AnalysisCache after a background
	// load. RunProject awaits it before IndexPhase so hosts can overlap
	// disk cache load with their own setup work.
	AnalysisCacheLoadFuture *AnalysisCacheLoadFuture
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
	// AnalysisCacheResult carries a cache lookup already performed by
	// the caller for the same paths/rules/config. CLI uses this to avoid
	// repeating the warm CheckFiles pass after oracle bootstrap.
	AnalysisCacheResult   *cache.Result
	AnalysisCacheStats    *cache.Stats
	AnalysisCacheRuleHash string
	// AnalysisCacheDirty opts the cacheCheck pass into
	// CheckFilesIncremental: only listed paths get stat'd, the rest hit
	// when an entry exists. nil = use the legacy CheckFiles path; a
	// non-nil (possibly empty) slice means "trust me, the watcher saw
	// every change." See issue #206.
	AnalysisCacheDirty []string
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
	// SourceSetClean lets a long-lived host that already tracks file
	// changes skip source discovery on a manifest-backed bundle hit.
	// False keeps RunProject conservative by re-collecting source paths.
	SourceSetClean bool
	// SourceSetDirty, when non-nil, lists the paths the host has
	// observed touched since the last analyze. preparseSourcePaths can
	// reuse the prior manifest's path list (skipping a 30-40s
	// filesystem walk on cold-OS-dentry-cache kotlin-corpus scale)
	// when every entry here also appears in the prior run's
	// ContentHashes — i.e., every dirty file is an EDIT of a known
	// path rather than an ADD or DELETE that would change the source
	// SET. Empty slice means "no edits, source set is clean"; nil
	// means "the host has no opinion, fall back to walking the
	// filesystem."
	SourceSetDirty []string
	// PriorContentHashes and PriorStructuralFPs, when non-nil, let
	// sourceSetFingerprint / crossFileStructuralFingerprint /
	// buildManifestData skip the ~3s per-call cost of recomputing
	// per-file hashes and structural fingerprints across the 16 k+
	// files of a kotlin-corpus scan. Entries for paths NOT in
	// SourceSetDirty are reused verbatim; only dirty paths are
	// rehashed and re-fingerprinted. Daemon callers populate these
	// from the resident bundle manifest; CLI callers leave them nil
	// and pay the recompute, which is only noticeable at very large
	// repos and only when the bundle-fingerprint check needs them.
	PriorContentHashes map[string]string
	PriorStructuralFPs map[string]string
	// FindingsBundleManifestLoader, when non-nil, overrides the disk
	// load of the prior-run manifest. Daemon callers wire this to an
	// in-memory cache so the 10.9 MB JSON file (kotlin-corpus scale)
	// isn't re-read + JSON-unmarshalled on every analyze-project
	// request — a 30-40s cold-OS-page-cache cost on kotlin. CLI callers
	// leave it nil and fall through to scanner.LoadFindingsBundleManifest.
	//
	// The loader returns (manifest, ok=true) when a manifest matching
	// the host's current scan set is available, (zero, false) when it
	// isn't (cold daemon, watcher detected a divergence, etc.).
	// Mismatches are conservative: the bundle-cache fingerprint check
	// inside preparseBundleFingerprint catches stale entries even when
	// the loader returns a value the host believes is current.
	FindingsBundleManifestLoader FindingsBundleManifestLoader
	// FindingsBundleManifestSaver, when non-nil, is called after a
	// successful disk save of a new manifest so the host can keep its
	// in-memory copy in sync. Same daemon-only motivation as
	// FindingsBundleManifestLoader. CLI callers leave nil.
	FindingsBundleManifestSaver FindingsBundleManifestSaver
}

// FindingsBundleManifestLoader resolves a manifest by its on-disk
// path. Daemon callers wire this to a process-local cache keyed by
// the manifest path so repeated analyze-project calls don't re-read
// the multi-MB JSON file from disk.
type FindingsBundleManifestLoader func(path string) (scanner.FindingsBundleManifest, bool)

// FindingsBundleManifestSaver receives the freshly persisted manifest
// so a daemon-side cache can replace its stored entry without
// re-reading from disk.
type FindingsBundleManifestSaver func(path string, manifest scanner.FindingsBundleManifest)

type warmAnalysisCachePlan struct {
	cache       *cache.Cache
	result      *cache.Result
	stats       *cache.Stats
	ruleHash    string
	filePaths   []string
	kotlinPaths []string
	javaPaths   []string
	cross       *scanner.FindingColumns
}

// ProjectInput is the value type that drives RunProject. The split
// between Args (per-call) and Host (long-lived) makes call sites
// self-documenting: in.Args.Format is request-scoped, in.Host.ParseCache
// is daemon-resident.
//
// The CLI and daemon both adapt their request state into ProjectInput so
// parse/index/dispatch/cross-file/Android analysis share one path while
// CLI-only concerns stay outside pipeline.
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

// ProjectAnalysisResult is the shared scan-core output before fixup and
// report formatting. CLI callers use this boundary to keep CLI-only FIR,
// baseline verbs, profile handling, and exit-code policy outside pipeline.
type ProjectAnalysisResult struct {
	ParseResult       ParseResult
	IndexResult       IndexResult
	DispatchResult    DispatchResult
	CrossFileResult   CrossFileResult
	FilesScanned      int
	ParseErrors       []error
	Stats             rules.RunStats
	ParseHits         int64
	ParseMisses       int64
	FindingsBundleHit bool
	PhaseTimingsMs    PhaseTimingsMs
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

	var phaseTimings PhaseTimingsMs
	if res, ok, err := tryLoadFindingsBundleBeforeParse(ctx, startTime, format, args, host, out, &phaseTimings); ok || err != nil {
		return res, err
	}
	analysis, err := RunProjectAnalysis(ctx, in)
	if err != nil {
		return ProjectResult{}, err
	}
	phaseTimings = analysis.PhaseTimingsMs

	fixupStart := time.Now()
	fixupView, err := runFixupPhase(ctx, args, analysis.CrossFileResult)
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
		perfRuleStats = rules.SortedRuleExecutionStats(analysis.DispatchResult.Stats)
	}
	// CacheStats is gated on ShowPerf so the daemon's analyze-project
	// JSON keeps omitting the "cache" key by default; auto-forwarding
	// would silently widen the daemon's wire format.
	var cacheStatsOut *cache.Stats
	if args.ShowPerf {
		cacheStatsOut = analysis.IndexResult.CacheStats
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

	return ProjectResult{
		FinalFindings:     outResult.FinalFindings,
		FilesScanned:      analysis.FilesScanned,
		FindingsCount:     outResult.FinalFindings.Len(),
		ParseErrors:       analysis.ParseErrors,
		Stats:             analysis.Stats,
		ParseHits:         analysis.ParseHits,
		ParseMisses:       analysis.ParseMisses,
		Fixup:             fixupView,
		FindingsBundleHit: analysis.FindingsBundleHit,
		PhaseTimingsMs:    phaseTimings,
	}, nil
}

// RunProjectAnalysis runs the shared parse/index/dispatch/cross-file/Android
// core and returns findings before fixup and output formatting.
func RunProjectAnalysis(ctx context.Context, in ProjectInput) (ProjectAnalysisResult, error) {
	if _, _, err := validateAndDefaultStreaming(ctx, in, io.Discard); err != nil {
		return ProjectAnalysisResult{}, err
	}
	args := in.Args
	host := awaitAnalysisCacheFuture(in.Host)
	hits0, misses0 := parseCacheCounters(host.ParseCache)
	warmPlan := buildWarmAnalysisCachePlan(args, host)
	if warmPlan.cache != nil {
		host.AnalysisCache = warmPlan.cache
	}

	var phaseTimings PhaseTimingsMs
	parseStart := time.Now()
	parseResult, err := runProjectParsePhase(ctx, args, host, warmPlan)
	phaseTimings.Parse = time.Since(parseStart).Milliseconds()
	if err != nil {
		return ProjectAnalysisResult{}, fmt.Errorf("parse: %w", err)
	}
	indexStart := time.Now()
	indexResult, err := runProjectIndexPhase(ctx, args, host, warmPlan, parseResult)
	phaseTimings.Index = time.Since(indexStart).Milliseconds()
	if err != nil {
		return ProjectAnalysisResult{}, fmt.Errorf("index: %w", err)
	}
	defer endProjectAnalysisTrackers(indexResult)
	if err := loadJavaSemanticFacts(ctx, args, host, parseResult.JavaFiles, &indexResult); err != nil {
		return ProjectAnalysisResult{}, err
	}
	runProjectTargetedResolution(args, host, parseResult.KotlinFiles, indexResult)

	runFP, bundleEnabled := computeRunFingerprint(args, host, parseResult, indexResult)
	manifestData := buildManifestData(args, host, parseResult, runFP, bundleEnabled)
	dispatchResult, crossFileResult, bundleHit, dispatchTimes, err := runDispatchOrLoadBundle(ctx, args, host, indexResult, parseResult, runFP, bundleEnabled, manifestData)
	if err != nil {
		return ProjectAnalysisResult{}, err
	}
	phaseTimings.Dispatch = dispatchTimes.dispatchMs
	phaseTimings.CrossFile = dispatchTimes.crossFileMs

	androidStart := time.Now()
	if err := runAndroidPhaseAndMerge(ctx, args, host, indexResult, &crossFileResult, bundleHit); err != nil {
		return ProjectAnalysisResult{}, err
	}
	if err := runKotlinPluginRulesAndMerge(ctx, args, host, indexResult, &crossFileResult, bundleHit); err != nil {
		return ProjectAnalysisResult{}, err
	}
	phaseTimings.Android = time.Since(androidStart).Milliseconds()

	if bundleEnabled {
		if !bundleHit {
			_ = host.FindingsBundleStore.Save(host.FindingsBundleCacheRoot, runFP, &crossFileResult.Findings)
		}
		_ = saveDeltaManifest(host, manifestData, runFP, &crossFileResult.Findings)
	}

	hits1, misses1 := parseCacheCounters(host.ParseCache)
	return ProjectAnalysisResult{
		ParseResult:       parseResult,
		IndexResult:       indexResult,
		DispatchResult:    dispatchResult,
		CrossFileResult:   crossFileResult,
		FilesScanned:      len(parseResult.KotlinFiles) + len(parseResult.JavaFiles),
		ParseErrors:       parseResult.ParseErrors,
		Stats:             dispatchResult.Stats,
		ParseHits:         hits1 - hits0,
		ParseMisses:       misses1 - misses0,
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

func awaitAnalysisCacheFuture(host ProjectHostState) ProjectHostState {
	if host.AnalysisCache != nil || host.AnalysisCacheLoadFuture == nil {
		return host
	}
	host.AnalysisCache = host.AnalysisCacheLoadFuture.Await()
	if host.AnalysisCache == nil && host.AnalysisCacheFilePath != "" {
		host.AnalysisCache = cache.Load(host.AnalysisCacheFilePath)
	}
	if host.Tracker != nil {
		perf.AddEntry(host.Tracker, "cacheLoadAsync", host.AnalysisCacheLoadFuture.Duration())
	}
	host.AnalysisCacheLoadFuture = nil
	return host
}

func runProjectIndexPhase(ctx context.Context, args ProjectArgs, host ProjectHostState, warm warmAnalysisCachePlan, parseResult ParseResult) (IndexResult, error) {
	hasIndexBackedCrossFileRule, hasParsedFilesRule, hasModuleAwareRule := ClassifyCrossFileNeeds(args.ActiveRules)
	var crossTracker perf.Tracker
	if host.Tracker != nil && (hasIndexBackedCrossFileRule || hasParsedFilesRule) {
		crossTracker = host.Tracker.Serial("crossFileAnalysis")
	}
	var moduleTracker perf.Tracker
	if host.Tracker != nil && hasModuleAwareRule {
		moduleTracker = host.Tracker.Serial("moduleAwareAnalysis")
	}
	scanRoot := "."
	if len(args.Paths) > 0 {
		scanRoot = args.Paths[0]
	}
	moduleNeeds := rules.CollectModuleAwareNeeds(args.ActiveRules)
	buildCodeIndex := hasIndexBackedCrossFileRule && warm.cross == nil
	if moduleNeeds.NeedsIndex {
		buildCodeIndex = true
	}
	indexInput := IndexInput{
		ParseResult:              parseResult,
		PrebuiltResolver:         host.PrebuiltResolver,
		PrebuiltLibraryFacts:     host.PrebuiltLibraryFacts,
		PrebuiltAndroidProject:   host.PrebuiltAndroidProject,
		LibraryFactsCache:        host.LibraryFactsCache,
		CodeIndexCache:           host.CodeIndexCache,
		CodeIndexSnapshotLoader:  host.CodeIndexSnapshotLoader,
		CodeIndexSnapshotSaver:   host.CodeIndexSnapshotSaver,
		JavaSourceIndexCache:     host.JavaSourceIndexCache,
		ResolverCache:            host.ResolverCache,
		ResolverFingerprintCache: host.ResolverFingerprintCache,
		AndroidProjectCache:      host.AndroidProjectCache,
		CrossFileCacheDir:        host.CrossFileCacheDir,
		CrossFindingsCacheDir:    host.CrossFindingsCacheDir,
		TypeIndexCacheDir:        host.TypeIndexCacheDir,
		ResidentFileTypeInfo:     host.ResidentFileTypeInfo,
		Reporter:                 host.Reporter,
		Tracker:                  host.Tracker,
		Verbose:                  host.Reporter.VerboseEnabled(),
		BuildCodeIndex:           buildCodeIndex,
		CrossFileParentTracker:   crossTracker,
		CrossFileJobsFlag:        args.Workers,
		CrossFileJavaPaths:       args.JavaPaths,
		ParseCache:               host.ParseCache,
		BuildModuleIndex:         hasModuleAwareRule,
		ModuleParentTracker:      moduleTracker,
		ModuleScanRoot:           scanRoot,
		ModuleJobsFlag:           args.Workers,
		ModuleHasAwareRule:       hasModuleAwareRule,
	}
	wireOracleHandles(&indexInput, args, host, parseResult.KotlinFiles)
	if warm.result == nil {
		wireAnalysisCacheLookup(&indexInput, args, host)
	}
	indexResult, err := IndexPhase{Workers: args.Workers}.Run(ctx, indexInput)
	if err != nil {
		return IndexResult{}, err
	}
	if moduleTracker != nil && (indexResult.Graph == nil || len(indexResult.Graph.Modules) == 0) {
		moduleTracker.End()
		moduleTracker = nil
	}
	indexResult.CrossFileParentTracker = crossTracker
	indexResult.ModuleParentTracker = moduleTracker
	completeRunProjectIndexResult(args, host, warm, &indexResult)
	return indexResult, nil
}

func endProjectAnalysisTrackers(indexResult IndexResult) {
	if indexResult.CrossFileParentTracker != nil {
		indexResult.CrossFileParentTracker.End()
	}
	if indexResult.ModuleParentTracker != nil {
		indexResult.ModuleParentTracker.End()
	}
}

func completeRunProjectIndexResult(args ProjectArgs, host ProjectHostState, warm warmAnalysisCachePlan, indexResult *IndexResult) {
	if host.Oracle != nil && indexResult.Oracle == nil {
		indexResult.Oracle = host.Oracle
	}
	if host.OracleDaemon != nil && indexResult.Daemon == nil {
		indexResult.Daemon = host.OracleDaemon
	}
	indexResult.Cache = host.AnalysisCache
	indexResult.Jobs = args.Workers
	indexResult.ProfileDispatch = args.ProfileDispatch
	indexResult.EmitPerFileStats = args.EmitPerFileStats
	indexResult.Reporter = host.Reporter
	indexResult.Tracker = host.Tracker
	if warm.result != nil {
		indexResult.CacheResult = warm.result
		indexResult.RuleHash = warm.ruleHash
		indexResult.CacheFilePath = host.AnalysisCacheFilePath
		indexResult.CacheStats = warm.stats
		indexResult.WarmCrossFindings = warm.cross
	}
	if host.AnalysisCache == nil {
		return
	}
	indexResult.CacheFilePath = host.AnalysisCacheFilePath
	indexResult.CacheScanPaths = args.Paths
	indexResult.Version = args.Version
	if indexResult.RuleHash == "" {
		indexResult.RuleHash = projectRuleHash(args.ActiveRules, args.Config)
	}
}

func loadJavaSemanticFacts(ctx context.Context, args ProjectArgs, host ProjectHostState, javaFiles []*scanner.File, indexResult *IndexResult) error {
	if host.JavaSemanticFacts != nil {
		indexResult.JavaSemanticFacts = host.JavaSemanticFacts
		return nil
	}
	if host.JavaSemanticFactsLoader == nil || !api.NeedsJavaFacts(args.ActiveRules) || len(javaFiles) == 0 {
		return nil
	}
	facts, warning, err := host.JavaSemanticFactsLoader(ctx, args.Paths, javaFiles, indexResult.LibraryFacts, host.Tracker)
	if err != nil {
		return fmt.Errorf("java semantic facts: %w", err)
	}
	if warning != "" {
		host.Reporter.Warnf("warning: %s\n", warning)
	} else if facts != nil {
		host.Reporter.Verbosef("verbose: Java semantic facts loaded (%d calls, %d classes)\n", len(facts.Calls), len(facts.Classes))
	}
	indexResult.JavaSemanticFacts = facts
	return nil
}

func runProjectTargetedResolution(args ProjectArgs, host ProjectHostState, files []*scanner.File, indexResult IndexResult) {
	if !args.TargetedResolution {
		return
	}
	resolver := host.TargetedExpressionResolver
	if resolver == nil && indexResult.Daemon != nil {
		resolver = DaemonExpressionResolver{Daemon: indexResult.Daemon}
	}
	sink := host.TargetedExpressionSink
	if sink == nil {
		sink = indexResult.Oracle
	}
	if resolver == nil || sink == nil {
		host.Reporter.Verbosef("verbose: targeted expression resolution skipped (resolver unavailable)\n")
		return
	}
	if err := RunTargetedResolutionPass(TargetedResolutionInput{
		ActiveRules: args.ActiveRules,
		Files:       files,
		Resolver:    resolver,
		Sink:        sink,
	}); err != nil {
		host.Reporter.Warnf("warning: targeted expression resolution failed: %v\n", err)
	}
}

func runProjectParsePhase(ctx context.Context, args ProjectArgs, host ProjectHostState, warm warmAnalysisCachePlan) (ParseResult, error) {
	kotlinPaths := args.KotlinPaths
	if kotlinPaths == nil {
		kotlinPaths = warm.kotlinPaths
	}
	javaPaths := args.JavaPaths
	if javaPaths == nil {
		javaPaths = warm.javaPaths
	}
	if canSkipRunProjectParse(args, host, warm) {
		host.Reporter.Verbosef("verbose: Skipped parse; findings cache covers %d files and no active phase needs parsed sources\n", len(warm.filePaths))
		if host.Tracker != nil {
			host.Tracker.TrackVoid("parse", func() {})
		}
		return ParseResult{
			Config:      args.Config,
			ActiveRules: args.ActiveRules,
			KotlinFiles: filesForPaths(kotlinPaths, scanner.LangKotlin),
			KotlinPaths: append([]string(nil), kotlinPaths...),
			JavaFiles:   filesForPaths(javaPaths, scanner.LangJava),
			Paths:       args.Paths,
		}, nil
	}
	allowCrossFile := warm.cross != nil
	if !allowCrossFile {
		allowCrossFile, _ = loadWarmCrossFindings(args, host, warm)
	}
	allowResourceSource := allowWarmResourceSourceDelta(args, host, warm)
	if CanParseOnlyCacheMisses(args.ActiveRules, warm.result, warm.result != nil, allowCrossFile, allowResourceSource) {
		kotlinPaths = CacheMissPaths(kotlinPaths, warm.result)
		javaPaths = CacheMissPaths(javaPaths, warm.result)
		host.Reporter.Verbosef("verbose: Parsing %d cache-miss files; findings cache covers %d/%d files\n", len(kotlinPaths), warm.result.TotalCached, warm.result.TotalFiles)
	} else if warm.result != nil && warm.result.TotalCached > 0 {
		host.Reporter.Verbosef("verbose: Parse miss-only unavailable: %s (cross=%t resourceSource=%t)\n",
			ParsedSourceBlockReason(args.ActiveRules, allowCrossFile, allowResourceSource), allowCrossFile, allowResourceSource)
	}
	skipJavaCollection := len(javaPaths) == 0 && allowCrossFile
	return ParsePhase{Workers: args.Workers}.Run(ctx, ParseInput{
		Config:             args.Config,
		Paths:              args.Paths,
		ActiveRules:        args.ActiveRules,
		IncludeGenerated:   args.IncludeGenerated,
		KotlinPaths:        kotlinPaths,
		JavaPaths:          javaPaths,
		Workers:            args.Workers,
		SkipJavaCollection: skipJavaCollection,
		Reporter:           host.Reporter,
		Tracker:            host.Tracker,
		ParseCache:         host.ParseCache,
		ResidentFiles:      host.ResidentFiles,
	})
}

func allowWarmResourceSourceDelta(args ProjectArgs, host ProjectHostState, warm warmAnalysisCachePlan) bool {
	if !HasResourceSourceRules(args.ActiveRules) || warm.result == nil || len(warm.filePaths) == 0 {
		return true
	}
	if host.AndroidCacheDir == "" || warm.ruleHash == "" {
		return false
	}
	project := host.PrebuiltAndroidProject
	if project == nil {
		project = cachedDetectAndroidProject(args, host)
	}
	if project == nil || project.IsEmpty() || len(project.ResDirs) == 0 {
		return false
	}
	_, libraryFactsFP := preparseProjectFingerprints(args, host)
	if len(warm.result.CachedHashes) > 0 {
		return true
	}
	if EnsureWarmResourceSourceBundleWithHashes(
		host.AndroidCacheDir,
		warm.filePaths,
		project.ResDirs,
		warm.result.CachedHashes,
		warm.ruleHash,
		libraryFactsFP,
		host.JavaSemanticFacts.Fingerprint(),
	) {
		return true
	}
	return HasWarmResourceSourceBundleManifest(
		host.AndroidCacheDir,
		warm.filePaths,
		project.ResDirs,
		warm.ruleHash,
		libraryFactsFP,
		host.JavaSemanticFacts.Fingerprint(),
	)
}

func buildWarmAnalysisCachePlan(args ProjectArgs, host ProjectHostState) warmAnalysisCachePlan {
	if host.AnalysisCache == nil || !host.AnalysisCacheLookup {
		return warmAnalysisCachePlan{}
	}
	kotlinPaths, javaPaths := warmSourcePaths(args)
	filePaths := warmCacheFilePaths(args, kotlinPaths, javaPaths)
	ruleHash := projectRuleHashWithEditorConfig(args.ActiveRules, args.Config, args.EditorConfigEnabled)
	result, stats := warmAnalysisCacheResult(args, host, filePaths, ruleHash)
	plan := warmAnalysisCachePlan{
		cache:       host.AnalysisCache,
		result:      result,
		stats:       stats,
		ruleHash:    ruleHash,
		filePaths:   filePaths,
		kotlinPaths: kotlinPaths,
		javaPaths:   javaPaths,
	}
	if ok, cols := loadWarmCrossFindings(args, host, plan); ok {
		plan.cross = cols
	}
	return plan
}

func warmAnalysisCacheResult(args ProjectArgs, host ProjectHostState, filePaths []string, ruleHash string) (*cache.Result, *cache.Stats) {
	if host.AnalysisCacheResult != nil && host.AnalysisCacheRuleHash == ruleHash && host.AnalysisCacheResult.TotalFiles == len(filePaths) {
		stats := host.AnalysisCacheStats
		if stats == nil {
			stats = &cache.Stats{
				Cached:    host.AnalysisCacheResult.TotalCached,
				Total:     host.AnalysisCacheResult.TotalFiles,
				LoadDurMs: 0,
			}
			if stats.Total > 0 {
				stats.HitRate = float64(stats.Cached) / float64(stats.Total)
			}
		}
		host.Reporter.Verbosef("verbose: Cache: %d/%d files cached (%d%% hit rate, reused)\n",
			host.AnalysisCacheResult.TotalCached, host.AnalysisCacheResult.TotalFiles, cacheHitPercent(host.AnalysisCacheResult))
		return host.AnalysisCacheResult, stats
	}
	start := time.Now()
	var result *cache.Result
	if host.AnalysisCacheDirty != nil {
		result = host.AnalysisCache.CheckFilesIncremental(filePaths, host.AnalysisCacheDirty, ruleHash, args.Paths...)
	} else {
		result = host.AnalysisCache.CheckFiles(filePaths, ruleHash, args.Paths...)
	}
	stats := &cache.Stats{
		Cached:    result.TotalCached,
		Total:     result.TotalFiles,
		LoadDurMs: 0,
	}
	if result.TotalFiles > 0 {
		stats.HitRate = float64(result.TotalCached) / float64(result.TotalFiles)
	}
	host.Reporter.Verbosef("verbose: Cache: %d/%d files cached (%d%% hit rate)\n",
		result.TotalCached, result.TotalFiles, cacheHitPercent(result))
	if host.Tracker != nil {
		perf.AddEntry(host.Tracker, "cacheCheck", time.Since(start))
	}
	return result, stats
}

func cacheHitPercent(result *cache.Result) int {
	if result == nil || result.TotalFiles == 0 {
		return 0
	}
	return 100 * result.TotalCached / result.TotalFiles
}

func warmSourcePaths(args ProjectArgs) ([]string, []string) {
	kotlinPaths := args.KotlinPaths
	if kotlinPaths == nil {
		if collected, err := scanner.CollectKotlinFiles(args.Paths, nil); err == nil {
			kotlinPaths = collected
		}
	}
	kotlinPaths = filterGeneratedSourcePaths(kotlinPaths, args.IncludeGenerated)
	javaPaths := args.JavaPaths
	if javaPaths == nil && NeedsJavaBeforeDispatch(args.ActiveRules) {
		if collected, err := scanner.CollectJavaFiles(args.Paths, nil); err == nil {
			javaPaths = collected
		}
	}
	javaPaths = filterGeneratedSourcePaths(javaPaths, args.IncludeGenerated)
	return kotlinPaths, javaPaths
}

func warmCacheFilePaths(args ProjectArgs, kotlinPaths, javaPaths []string) []string {
	filePaths := make([]string, 0, len(kotlinPaths)+len(javaPaths))
	filePaths = append(filePaths, kotlinPaths...)
	if NeedsJavaBeforeDispatch(args.ActiveRules) {
		filePaths = append(filePaths, javaPaths...)
	}
	return filePaths
}

func canSkipRunProjectParse(args ProjectArgs, host ProjectHostState, warm warmAnalysisCachePlan) bool {
	if warm.result == nil || warm.result.TotalFiles == 0 || warm.result.TotalCached != warm.result.TotalFiles {
		return false
	}
	if len(warm.result.CachedPaths) != len(warm.filePaths) {
		return false
	}
	allowCrossFile := warm.cross != nil
	if !allowCrossFile {
		var loaded *scanner.FindingColumns
		allowCrossFile, loaded = loadWarmCrossFindings(args, host, warm)
		warm.cross = loaded
	}
	allowResourceSource := allowWarmResourceSourceDelta(args, host, warm)
	if RulesNeedParsedSource(args.ActiveRules, allowCrossFile, allowResourceSource) {
		host.Reporter.Verbosef("verbose: Parse skip unavailable: active rule requires parsed source: %s\n", ParsedSourceBlockReason(args.ActiveRules, allowCrossFile, allowResourceSource))
		return false
	}
	return true
}

func loadWarmCrossFindings(args ProjectArgs, host ProjectHostState, warm warmAnalysisCachePlan) (bool, *scanner.FindingColumns) {
	if !RulesNeedCrossOrParsedFiles(args.ActiveRules) {
		return true, nil
	}
	if host.CrossFileCacheDir == "" || host.CrossFindingsCacheDir == "" || warm.ruleHash == "" {
		return false, nil
	}
	if warm.result != nil && warm.result.TotalFiles > 0 && warm.result.TotalCached != warm.result.TotalFiles {
		missPaths := CacheMissPaths(warm.filePaths, warm.result)
		if canReuseCrossFindingsForLexicallyIrrelevantMisses(args.ActiveRules, missPaths) {
			cols, ok := scanner.LoadLastCrossFindings(host.CrossFindingsCacheDir)
			if ok {
				return true, &cols
			}
		}
		return false, nil
	}
	meta, ok := scanner.LoadCurrentCrossFileCacheMeta(host.CrossFileCacheDir)
	if !ok {
		return false, nil
	}
	cols, ok := scanner.LoadCrossFindings(host.CrossFindingsCacheDir, scanner.CrossFindingsKey(meta.Fingerprint, warm.ruleHash))
	if !ok {
		return false, nil
	}
	return true, &cols
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
	return projectRuleHashWithEditorConfig(activeRules, cfg, false)
}

func projectRuleHashWithEditorConfig(activeRules []*api.Rule, cfg *config.Config, editorConfigEnabled bool) string {
	ruleNames := make([]string, 0, len(activeRules))
	for _, r := range activeRules {
		if r != nil {
			ruleNames = append(ruleNames, r.ID)
		}
	}
	return cache.ComputeConfigHash(ruleNames, cfg, editorConfigEnabled)
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
	in.CacheEditorConfigEnabled = args.EditorConfigEnabled
	ruleNames := make([]string, 0, len(args.ActiveRules))
	for _, r := range args.ActiveRules {
		if r != nil {
			ruleNames = append(ruleNames, r.ID)
		}
	}
	in.CacheRuleNames = ruleNames
	in.CacheDirty = host.AnalysisCacheDirty
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
	in.PrebuiltOracleCallFilter = host.OracleFilterCache(fp, func() *oracle.CallTargetFilterSummary {
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
	fileStats     map[string]scanner.FileStat
}

// buildManifestData prepares per-file content + structural fingerprints
// for the delta planner. Returns the inputs whether or not the bundle
// cache is enabled — callers gate on enabled before persisting.
func buildManifestData(args ProjectArgs, host ProjectHostState, parseResult ParseResult, _ scanner.RunFingerprint, bundleEnabled bool) deltaManifestData {
	if !bundleEnabled {
		return deltaManifestData{}
	}
	total := len(parseResult.KotlinFiles) + len(parseResult.JavaFiles)
	contentHashes := make(map[string]string, total)
	structuralFPs := make(map[string]string, total)
	fileStats := make(map[string]scanner.FileStat, total)
	dirty := dirtyPathSet(host.SourceSetDirty)
	for _, f := range parseResult.KotlinFiles {
		if f == nil {
			continue
		}
		contentHashes[f.Path] = priorOrCompute(host.PriorContentHashes, dirty, f.Path, func() string {
			return hashutil.Default().HashContent(f.Path, f.Content)
		})
		structuralFPs[f.Path] = priorOrCompute(host.PriorStructuralFPs, dirty, f.Path, func() string {
			return scanner.FileStructuralFingerprint(f)
		})
		if stat, ok := statForPath(f.Path); ok {
			fileStats[f.Path] = stat
		}
	}
	for _, f := range parseResult.JavaFiles {
		if f == nil {
			continue
		}
		contentHashes[f.Path] = priorOrCompute(host.PriorContentHashes, dirty, f.Path, func() string {
			return hashutil.Default().HashContent(f.Path, f.Content)
		})
		structuralFPs[f.Path] = priorOrCompute(host.PriorStructuralFPs, dirty, f.Path, func() string {
			return scanner.FileStructuralFingerprint(f)
		})
		if stat, ok := statForPath(f.Path); ok {
			fileStats[f.Path] = stat
		}
	}
	return deltaManifestData{
		enabled:       true,
		manifestKey:   scanner.FindingsBundleManifestKey(host.FindingsBundleCacheRoot, args.Paths),
		contentHashes: contentHashes,
		structuralFPs: structuralFPs,
		fileStats:     fileStats,
	}
}

// saveDeltaManifest persists the current run's manifest entry so a
// future run can detect single-file changes for the delta path.
// Best-effort — manifest write failures don't fail the verb. When
// the host wired FindingsBundleManifestSaver, the in-memory cache is
// updated after a successful disk save so the next analyze-project
// call doesn't pay the multi-MB JSON re-parse.
func saveDeltaManifest(host ProjectHostState, m deltaManifestData, runFP scanner.RunFingerprint, _ *scanner.FindingColumns) error {
	if !m.enabled || m.manifestKey == "" {
		return nil
	}
	manifest := scanner.FindingsBundleManifest{
		BundleKey:     scanner.FindingsBundleKey(runFP),
		Fingerprint:   runFP,
		ContentHashes: m.contentHashes,
		StructuralFPs: m.structuralFPs,
		FileStats:     m.fileStats,
	}
	if err := scanner.SaveFindingsBundleManifest(host.FindingsBundleCacheRoot, m.manifestKey, manifest); err != nil {
		return err
	}
	if host.FindingsBundleManifestSaver != nil {
		host.FindingsBundleManifestSaver(scanner.FindingsBundleManifestPath(host.FindingsBundleCacheRoot, m.manifestKey), manifest)
	}
	return nil
}

// loadBundleManifest reads the prior-run manifest, preferring the
// host-supplied in-memory loader (daemon) when available. Falls back
// to scanner.LoadFindingsBundleManifest so CLI callers see the
// existing behaviour.
func loadBundleManifest(host ProjectHostState, key string) (scanner.FindingsBundleManifest, bool) {
	if host.FindingsBundleManifestLoader != nil {
		path := scanner.FindingsBundleManifestPath(host.FindingsBundleCacheRoot, key)
		if path != "" {
			if m, ok := host.FindingsBundleManifestLoader(path); ok {
				return m, true
			}
		}
	}
	return scanner.LoadFindingsBundleManifest(host.FindingsBundleCacheRoot, key)
}

// runAndroidPhaseAndMerge runs AndroidPhase against the detected
// project (if any) and folds its findings into crossFileResult.Findings.
// A nil/empty project, no Android-needing rules, or a dispatcher-less
// run returns without mutating the findings. Mirrors the CLI runner's
// androidPhase step.
func runAndroidPhaseAndMerge(ctx context.Context, args ProjectArgs, host ProjectHostState, indexResult IndexResult, crossFileResult *CrossFileResult, bundleHit bool) error {
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
	// Hand AndroidPhase a child tracker so its gradleAnalysis /
	// manifestAnalysis / resourceAnalysis sub-scopes nest under
	// "androidPhase" in the --perf tree. Without this the sub-scopes
	// would sit at the top level alongside crossFileAnalysis (or vanish
	// entirely when in.Tracker is nil) — on a kotlin-style monorepo
	// that's 1+ seconds of otherwise-invisible time.
	var androidTracker perf.Tracker
	if host.Tracker != nil && host.Tracker.IsEnabled() {
		androidTracker = host.Tracker.Serial("androidPhase")
	}
	res, err := (AndroidPhase{}).Run(ctx, AndroidInput{
		Project:             project,
		ActiveRules:         args.ActiveRules,
		Dispatcher:          dispatcher,
		SourceFiles:         indexResult.SourceFiles(),
		SourcePaths:         androidSourcePaths(args, indexResult),
		SourceHashes:        CachedHashesOrNil(indexResult.CacheResult),
		Providers:           host.AndroidProviders,
		RuleHash:            indexResult.RuleHash,
		LibraryFactsFP:      indexResult.LibraryFacts.Fingerprint(),
		JavaSemanticFactsFP: indexResult.JavaSemanticFacts.Fingerprint(),
		CacheDir:            host.AndroidCacheDir,
		CacheWriter:         host.AndroidCacheWriter,
		GradleFindingsCache: host.GradleFindingsCache,
		Tracker:             androidTracker,
	})
	if androidTracker != nil {
		androidTracker.End()
	}
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

func androidSourcePaths(args ProjectArgs, indexResult IndexResult) []string {
	kotlinPaths := args.KotlinPaths
	if kotlinPaths == nil {
		kotlinPaths = indexResult.KotlinPaths
	}
	javaPaths := args.JavaPaths
	if javaPaths == nil {
		for _, f := range indexResult.JavaFiles {
			if f != nil {
				javaPaths = append(javaPaths, f.Path)
			}
		}
	}
	return warmCacheFilePaths(args, kotlinPaths, javaPaths)
}

// dispatchTimings is the per-phase wall-time slice returned from
// runDispatchOrLoadBundle so RunProjectStreaming can populate
// PhaseTimingsMs even on the bundle/delta paths where the phase
// boundaries are inside this helper.
type dispatchTimings struct {
	dispatchMs  int64
	crossFileMs int64
}

// runDispatchOrLoadBundle resolves dispatch + cross-file findings via
// four increasingly conservative paths:
//
//  1. Full-fingerprint Load. RunFingerprint identical to a prior run →
//     replay cached bundle, skip dispatch + cross-file entirely.
//  2. Structural reuse. Prior manifest exists, planner says exactly 1 file
//     changed + every non-SourceSet fingerprint field is stable →
//     load the prior bundle, save an alias under the current content
//     fingerprint, and skip dispatch + cross-file entirely.
//  3. Delta path fallback. If structural reuse cannot save the alias,
//     dispatch only on the changed file, re-run cross-file, filter
//     replacement to the changed path, ApplyDelta against the prior
//     bundle.
//  4. Full dispatch. Normal DispatchPhase + CrossFilePhase.
//
// Structural reuse is the load-bearing #55 perf win for body-only
// Kotlin edits: the source content hash moves, but the cross-file
// structural fingerprint remains stable, so the whole findings bundle
// can be replayed.
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
	// Track the bundle-fast-paths so --perf shows where a warm
	// "nothing structurally changed" analyze spends its 100-300 ms —
	// most of it is the disk-backed bundle Load. Without these
	// scopes the post-parse bundle hit shows up as unattributed time
	// under crossFileAnalysis (the parent scope opened in
	// runProjectIndexPhase wraps the entire IndexPhase + dispatch
	// flow, so a bundle hit halfway through leaves a "gap" with no
	// children for the user to investigate).
	tracker := host.Tracker
	tracked := tracker != nil && tracker.IsEnabled()
	if bundleEnabled {
		var cached *scanner.FindingColumns
		var ok bool
		loadFn := func() {
			cached, ok = host.FindingsBundleStore.Load(host.FindingsBundleCacheRoot, runFP)
		}
		if tracked {
			tracker.TrackVoid("dispatchBundleLoad", loadFn)
		} else {
			loadFn()
		}
		if ok && cached != nil {
			d := DispatchResult{IndexResult: indexResult, Findings: *cached}
			return d, CrossFileResult{DispatchResult: d}, true, dispatchTimings{}, nil
		}
		var stableCached *scanner.FindingColumns
		var stableOK bool
		stableFn := func() {
			stableCached, stableOK = tryLoadStructurallyStableBundle(host, runFP, manifest)
		}
		if tracked {
			tracker.TrackVoid("dispatchStableBundleLoad", stableFn)
		} else {
			stableFn()
		}
		if stableOK && stableCached != nil {
			d := DispatchResult{IndexResult: indexResult, Findings: *stableCached}
			return d, CrossFileResult{DispatchResult: d}, true, dispatchTimings{}, nil
		}
		reportFindingsBundleMiss(host, manifest, runFP)
		var deltaD DispatchResult
		var deltaC CrossFileResult
		var deltaOK bool
		var deltaErr error
		deltaFn := func() {
			deltaD, deltaC, deltaOK, deltaErr = tryDeltaDispatch(ctx, args, host, indexResult, parseResult, runFP, manifest)
		}
		if tracked {
			tracker.TrackVoid("dispatchDeltaPath", deltaFn)
		} else {
			deltaFn()
		}
		if deltaErr != nil {
			return DispatchResult{}, CrossFileResult{}, false, dispatchTimings{}, deltaErr
		}
		if deltaOK {
			return deltaD, deltaC, false, dispatchTimings{}, nil
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

func tryLoadStructurallyStableBundle(host ProjectHostState, runFP scanner.RunFingerprint, manifest deltaManifestData) (*scanner.FindingColumns, bool) {
	if !manifest.enabled || manifest.manifestKey == "" {
		return nil, false
	}
	prior, ok := loadBundleManifest(host, manifest.manifestKey)
	if !ok {
		return nil, false
	}
	changedPaths := diffContentHashes(prior.ContentHashes, manifest.contentHashes)
	plan := scanner.ConservativeDeltaPlanner{}.Plan(prior.Fingerprint, runFP, changedPaths)
	if !plan.ReusePrevious || len(plan.ChangedPaths) != 1 || !bodyOnlyKotlinChange(plan.ChangedPaths[0], prior.StructuralFPs, manifest.structuralFPs) {
		return nil, false
	}
	priorBundle, ok := host.FindingsBundleStore.Load(host.FindingsBundleCacheRoot, prior.Fingerprint)
	if !ok || priorBundle == nil {
		return nil, false
	}
	if err := host.FindingsBundleStore.Save(host.FindingsBundleCacheRoot, runFP, priorBundle); err != nil {
		host.Reporter.Verbosef("verbose: Findings bundle structural reuse skipped: save alias failed: %v\n", err)
		return nil, false
	}
	host.Reporter.Verbosef("verbose: Findings bundle cache: HIT (structural reuse: %s)\n", plan.ChangedPaths[0])
	return priorBundle, true
}

func bodyOnlyKotlinChange(path string, prior, current map[string]string) bool {
	if !strings.HasSuffix(path, ".kt") && !strings.HasSuffix(path, ".kts") {
		return false
	}
	if len(prior) == 0 || len(current) == 0 {
		return false
	}
	return prior[path] != "" && prior[path] == current[path]
}

func reportFindingsBundleMiss(host ProjectHostState, manifest deltaManifestData, runFP scanner.RunFingerprint) {
	if host.Reporter == nil || !host.Reporter.VerboseEnabled() || !manifest.enabled || manifest.manifestKey == "" {
		return
	}
	prior, ok := loadBundleManifest(host, manifest.manifestKey)
	if !ok {
		host.Reporter.Verbosef("verbose: Findings bundle cache: MISS (no prior manifest)\n")
		return
	}
	changed := runFingerprintDiffFields(prior.Fingerprint, runFP)
	if len(changed) == 0 {
		host.Reporter.Verbosef("verbose: Findings bundle cache: MISS (prior bundle missing)\n")
		return
	}
	host.Reporter.Verbosef("verbose: Findings bundle cache: MISS (changed: %s)\n", strings.Join(changed, ","))
}

func runFingerprintDiffFields(prior, current scanner.RunFingerprint) []string {
	var changed []string
	add := func(name, a, b string) {
		if a != b {
			changed = append(changed, name)
		}
	}
	add("version", prior.Version, current.Version)
	add("rules", prior.Rules, current.Rules)
	add("config", prior.Config, current.Config)
	add("sourceSet", prior.SourceSet, current.SourceSet)
	add("crossFile", prior.CrossFile, current.CrossFile)
	add("android", prior.Android, current.Android)
	add("libraryFacts", prior.LibraryFacts, current.LibraryFacts)
	return changed
}

func tryLoadFindingsBundleBeforeParse(
	ctx context.Context,
	startTime time.Time,
	format string,
	args ProjectArgs,
	host ProjectHostState,
	out io.Writer,
	phaseTimings *PhaseTimingsMs,
) (ProjectResult, bool, error) {
	if args.Fix || args.FixBinary || args.DryRun || len(args.CustomRuleJars) > 0 || host.FindingsBundleStore == nil || host.FindingsBundleCacheRoot == "" {
		return ProjectResult{}, false, nil
	}
	start := time.Now()
	// Wrap the warm-path bundle work in a serial tracker scope so
	// --perf surfaces the warm-baseline cost ("manifestLoad",
	// "fileStatsMatch", "bundleLoad", ...) as nested children of
	// "bundleHit" — sibling scopes would double-count against the
	// top-level total.
	var bundleTracker perf.Tracker
	if host.Tracker != nil && host.Tracker.IsEnabled() {
		bundleTracker = host.Tracker.Serial("bundleHit")
	}
	fp, kotlinFiles, javaFiles, ok := preparseBundleFingerprintTracked(args, host, bundleTracker)
	if phaseTimings != nil {
		phaseTimings.Parse = time.Since(start).Milliseconds()
	}
	if !ok {
		if bundleTracker != nil {
			bundleTracker.End()
		}
		return ProjectResult{}, false, nil
	}

	// Fast-fast path: when the formatted-output cache already has
	// bytes for this fingerprint we can skip the 30 MB
	// FindingsBundleStore.Load entirely (~11 ms zstd+gob decode).
	// The cached bytes are derived purely from the (FindingColumns,
	// active rules) pair the fingerprint already encodes, so by
	// definition they're valid for this request. Eligibility mirrors
	// the format-cache fast path's filter preconditions below.
	if canUseBundleOutputCache(args, host) {
		if cachedOut := host.BundleOutput(scanner.FindingsBundleKey(fp)); cachedOut != nil {
			if bundleTracker != nil {
				bundleTracker.TrackVoid("bundleLoadSkipped", func() {})
				bundleTracker.End()
			}
			return emitBundleHitOutput(ctx, args, host, out, format, startTime, phaseTimings,
				fp, nil, kotlinFiles, javaFiles)
		}
	}

	var cached *scanner.FindingColumns
	loadFn := func() {
		c, found := host.FindingsBundleStore.Load(host.FindingsBundleCacheRoot, fp)
		if found {
			cached = c
		} else {
			ok = false
		}
	}
	if bundleTracker != nil {
		bundleTracker.TrackVoid("bundleLoad", loadFn)
	} else {
		loadFn()
	}
	if bundleTracker != nil {
		bundleTracker.End()
	}
	if !ok || cached == nil {
		return ProjectResult{}, false, nil
	}

	return emitBundleHitOutput(ctx, args, host, out, format, startTime, phaseTimings,
		fp, cached, kotlinFiles, javaFiles)
}

// emitBundleHitOutput dispatches the bundle-hit response: cached
// findings bytes via serveBundleHitFromOutputCache when eligible,
// otherwise the full OutputPhase route. Extracted so
// tryLoadFindingsBundleBeforeParse stays under the cyclomatic-
// complexity gate.
func emitBundleHitOutput(
	ctx context.Context,
	args ProjectArgs,
	host ProjectHostState,
	out io.Writer,
	format string,
	startTime time.Time,
	phaseTimings *PhaseTimingsMs,
	fp scanner.RunFingerprint,
	cached *scanner.FindingColumns,
	kotlinFiles, javaFiles []*scanner.File,
) (ProjectResult, bool, error) {
	perfTimings, caches, budget := capturePerfOutputs(args, host)
	outputStart := time.Now()

	if canUseBundleOutputCache(args, host) {
		res, fastOK, fastErr := serveBundleHitFromOutputCache(
			cached, fp, kotlinFiles, javaFiles, args, host, out,
			startTime, perfTimings, caches, budget)
		if fastOK || fastErr != nil {
			if phaseTimings != nil {
				phaseTimings.Output = time.Since(outputStart).Milliseconds()
			}
			return res, fastOK, fastErr
		}
		// fastOK=false with nil error means the cache build path
		// declined (e.g. degenerate FindingColumns) — fall through
		// to OutputPhase for correctness.
	}
	outResult, err := OutputPhase{}.Run(ctx, OutputInput{
		FixupResult: FixupResult{
			CrossFileResult: CrossFileResult{
				DispatchResult: DispatchResult{
					IndexResult: IndexResult{
						ParseResult: ParseResult{
							KotlinFiles: kotlinFiles,
							JavaFiles:   javaFiles,
							Paths:       args.Paths,
							ActiveRules: args.ActiveRules,
						},
					},
					Findings: *cached,
				},
			},
		},
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
		Caches:           caches,
		CacheBudget:      budget,
	})
	if phaseTimings != nil {
		phaseTimings.Output = time.Since(outputStart).Milliseconds()
	}
	if err != nil {
		return ProjectResult{}, true, fmt.Errorf("output: %w", err)
	}
	return ProjectResult{
		FinalFindings:     outResult.FinalFindings,
		FilesScanned:      len(kotlinFiles) + len(javaFiles),
		FindingsCount:     outResult.FinalFindings.Len(),
		FindingsBundleHit: true,
		PhaseTimingsMs:    *phaseTimings,
	}, true, nil
}

// canUseBundleOutputCache reports whether the bundle-hit fast path
// (cached findings JSON + handwritten envelope) is safe for this
// request. The cached bytes are derived purely from the
// FindingColumns + active rules — anything that mutates the
// post-bundle column set (baseline filter, diff filter, severity
// promotion, min-confidence threshold) makes them incorrect, so
// those cases fall back to the regular OutputPhase route.
func canUseBundleOutputCache(args ProjectArgs, host ProjectHostState) bool {
	if host.BundleOutput == nil || host.StoreBundleOutput == nil {
		return false
	}
	if args.Format != "json" || !args.JSONCompact {
		return false
	}
	if args.BaselinePath != "" || args.DiffRef != "" {
		return false
	}
	if args.WarningsAsErrors || args.MinConfidence > 0 {
		return false
	}
	return true
}

// serveBundleHitFromOutputCache emits the bundle-hit response using
// pre-formatted findings bytes when available, or formats once and
// caches the result. Returns (result, true, nil) on success,
// (zero, false, nil) on a soft-fall-through (e.g. cache infrastructure
// gave us a nil cache entry on miss-store), or (zero, true, err) on
// a hard write error.
func serveBundleHitFromOutputCache(
	cached *scanner.FindingColumns,
	fp scanner.RunFingerprint,
	kotlinFiles, javaFiles []*scanner.File,
	args ProjectArgs,
	host ProjectHostState,
	out io.Writer,
	startTime time.Time,
	perfTimings []perf.TimingEntry,
	caches []cacheutil.NamedCacheStats,
	cacheBudget *cacheutil.BudgetReport,
) (ProjectResult, bool, error) {
	key := scanner.FindingsBundleKey(fp)
	if key == "" {
		return ProjectResult{}, false, nil
	}
	out2 := host.BundleOutput(key)
	if out2 == nil {
		// Cache miss: must have the decoded FindingColumns to
		// build the formatted bytes. Caller is responsible for
		// loading the bundle before calling on a cache miss.
		if cached == nil {
			return ProjectResult{}, false, nil
		}
		built, ok := buildCachedBundleOutput(cached, args.ActiveRules)
		if !ok {
			return ProjectResult{}, false, nil
		}
		host.StoreBundleOutput(key, built)
		out2 = built
	}

	durationMs := time.Since(startTime).Milliseconds()
	summary := JSONSummary{
		Total:     out2.Total,
		ByRuleSet: out2.ByRuleSet,
		ByRule:    out2.ByRule,
		Fixable:   out2.FixableCount,
	}
	if err := writeBundleHitCompactJSON(out, args, len(kotlinFiles)+len(javaFiles), len(args.ActiveRules),
		durationMs, summary, out2.FindingsBytes, perfTimings, caches, cacheBudget); err != nil {
		return ProjectResult{}, true, fmt.Errorf("output: %w", err)
	}
	// FinalFindings carries the decoded columns when available
	// (cache miss path that just built them). On the fast-fast
	// path where we skipped the bundle Load, the caller doesn't
	// need the column data — only FindingsCount, which we read
	// from out2.Total.
	var finalFindings scanner.FindingColumns
	if cached != nil {
		finalFindings = *cached
	}
	return ProjectResult{
		FinalFindings:     finalFindings,
		FilesScanned:      len(kotlinFiles) + len(javaFiles),
		FindingsCount:     out2.Total,
		FindingsBundleHit: true,
	}, true, nil
}

// buildCachedBundleOutput formats the bundle's findings into a
// reusable byte buffer + summary. Mirrors what FormatJSONColumnsCompact
// does for the findings array, but exposes the bytes so the daemon
// can stash them on WorkspaceState. ok=false on degenerate input
// so the caller can fall back to OutputPhase.
func buildCachedBundleOutput(cached *scanner.FindingColumns, activeRules []*api.Rule) (*CachedBundleOutput, bool) {
	if cached == nil {
		return nil, false
	}
	fixLevels := make(map[string]string)
	efforts := make(map[string]string)
	for _, r := range activeRules {
		if r == nil {
			continue
		}
		if lvl, ok := rules.GetV2FixLevel(r); ok {
			fixLevels[r.ID] = lvl.String()
		}
		if e := rules.V2RuleEffort(r); e != api.EffortUnset {
			efforts[r.ID] = e.String()
		}
	}
	byRuleSet := make(map[string]int)
	byRule := make(map[string]int)
	fixableCount := 0
	findingsBytes := output.BuildFindingsArrayCompact(cached, fixLevels, efforts, byRuleSet, byRule, &fixableCount)
	return &CachedBundleOutput{
		FindingsBytes: findingsBytes,
		Total:         cached.Len(),
		ByRuleSet:     byRuleSet,
		ByRule:        byRule,
		FixableCount:  fixableCount,
	}, true
}

// writeBundleHitCompactJSON emits the report envelope using the
// pre-formatted findingsBytes verbatim. Mirrors
// output.writeCompactReport's shape so the wire format is identical
// to the OutputPhase route a non-cacheable request would take.
func writeBundleHitCompactJSON(
	w io.Writer,
	args ProjectArgs,
	fileCount, ruleCount int,
	durationMs int64,
	summary JSONSummary,
	findingsBytes []byte,
	perfTimings []perf.TimingEntry,
	caches []cacheutil.NamedCacheStats,
	cacheBudget *cacheutil.BudgetReport,
) error {
	return output.WriteCompactReport(w, output.CompactReport{
		Success:      summary.Total == 0,
		Version:      args.Version,
		DurationMs:   durationMs,
		FileCount:    fileCount,
		RuleCount:    ruleCount,
		Experiments:  args.ExperimentNames,
		FindingsJSON: findingsBytes,
		Summary:      summary,
		Caches:       caches,
		CacheBudget:  cacheBudget,
		PerfTimings:  perfTimings,
	})
}

// JSONSummary mirrors output.JSONSummary so the bundle-hit fast
// path can construct one without an import cycle. Use a type alias
// to keep semantics in sync — any future field added to the output
// package's struct surfaces automatically here.
type JSONSummary = output.JSONSummary

// preparseBundleFingerprintTracked computes the bundle fingerprint
// used to decide whether the warm findings-bundle path can serve the
// request. When tracker is non-nil, sub-steps record their
// wall-clock under "manifestLoad", "sourcePaths", "fileStatsMatch",
// and "projectFingerprints" so --perf can show where warm-path
// bundle fingerprinting spends time. tracker may be nil (CLI path /
// non-perf calls), in which case the scopes are elided to keep the
// hot path zero-overhead.
func preparseBundleFingerprintTracked(args ProjectArgs, host ProjectHostState, tracker perf.Tracker) (scanner.RunFingerprint, []*scanner.File, []*scanner.File, bool) {
	track := func(name string, fn func()) {
		if tracker == nil {
			fn()
			return
		}
		tracker.TrackVoid(name, fn)
	}

	key := scanner.FindingsBundleManifestKey(host.FindingsBundleCacheRoot, args.Paths)
	if key == "" {
		return scanner.RunFingerprint{}, nil, nil, false
	}
	var (
		prior scanner.FindingsBundleManifest
		ok    bool
	)
	track("manifestLoad", func() { prior, ok = loadBundleManifest(host, key) })
	if !ok || len(prior.StructuralFPs) == 0 {
		return scanner.RunFingerprint{}, nil, nil, false
	}
	var kotlinPaths, javaPaths []string
	track("sourcePaths", func() {
		kotlinPaths, javaPaths, ok = preparseSourcePaths(args, host, prior)
	})
	if !ok {
		return scanner.RunFingerprint{}, nil, nil, false
	}
	paths := append(append([]string(nil), kotlinPaths...), javaPaths...)
	// Snapshot the watcher's version BEFORE the (potentially skipped)
	// stat sweep so a concurrent watcher event between the snapshot
	// and a successful match invalidates the resulting memo on its
	// own — we never store a "clean" record under a stale version.
	var preStatVersion uint64
	if host.SourceMTimeVersion != nil {
		preStatVersion = host.SourceMTimeVersion()
	}
	var statsOK bool
	track("fileStatsMatch", func() {
		// Daemon fast path: when the watcher hasn't seen a source-
		// path event since the last successful sweep AND the cache
		// records a clean version for this manifest, the sweep
		// result is necessarily true. Skip the 18k os.Stat calls.
		if host.BundleStatsClean != nil && host.BundleStatsClean(key) {
			statsOK = true
			return
		}
		statsOK = fileStatsMatch(paths, prior.FileStats)
	})
	if !statsOK {
		return scanner.RunFingerprint{}, nil, nil, false
	}
	if host.MarkBundleStatsClean != nil {
		host.MarkBundleStatsClean(key, preStatVersion)
	}
	rulesHash := projectRuleHash(args.ActiveRules, args.Config)
	var androidFP, libraryFactsFP string
	track("projectFingerprints", func() {
		androidFP, libraryFactsFP = preparseProjectFingerprints(args, host)
	})
	fp := scanner.RunFingerprint{
		Version:      args.Version,
		Rules:        rulesHash,
		Config:       rulesHash,
		SourceSet:    fingerprintPathHashMap(prior.ContentHashes),
		CrossFile:    fingerprintPathHashMap(prior.StructuralFPs),
		Android:      androidFP,
		LibraryFacts: libraryFactsFP,
	}
	if fp != prior.Fingerprint {
		return scanner.RunFingerprint{}, nil, nil, false
	}
	return fp, filesForPaths(kotlinPaths, scanner.LangKotlin), filesForPaths(javaPaths, scanner.LangJava), true
}

func preparseSourcePaths(args ProjectArgs, host ProjectHostState, prior scanner.FindingsBundleManifest) ([]string, []string, bool) {
	if host.SourceSetClean || dirtyPathsAllInManifest(host.SourceSetDirty, prior.ContentHashes) {
		kotlinPaths, javaPaths := pathsFromManifest(prior.ContentHashes)
		return kotlinPaths, javaPaths, true
	}
	kotlinPaths, err := scanner.CollectKotlinFiles(args.Paths, nil)
	if err != nil {
		return nil, nil, false
	}
	kotlinPaths = filterGeneratedSourcePaths(kotlinPaths, args.IncludeGenerated)
	javaPaths, err := collectJavaPathsForFingerprint(args)
	if err != nil {
		return nil, nil, false
	}
	javaPaths = filterGeneratedSourcePaths(javaPaths, args.IncludeGenerated)
	return kotlinPaths, javaPaths, true
}

// dirtyPathsAllInManifest reports whether every path in dirty is also
// a key in the prior manifest's ContentHashes — i.e., every recent
// edit is an UPDATE of a previously-indexed file rather than an ADD
// or DELETE that would change the source set. When that holds the
// caller can reuse the prior manifest's path list and skip a fresh
// filesystem walk, which costs 30-40s on kotlin-corpus scale when the
// OS dentry cache is cold.
//
// nil dirty means "host has no opinion" and we return false so the
// caller falls back to walking. Empty dirty is a clean source set,
// also acceptable.
func dirtyPathsAllInManifest(dirty []string, manifest map[string]string) bool {
	if dirty == nil {
		return false
	}
	if len(manifest) == 0 {
		return len(dirty) == 0
	}
	for _, p := range dirty {
		if _, ok := manifest[p]; !ok {
			return false
		}
	}
	return true
}

func pathsFromManifest(hashes map[string]string) ([]string, []string) {
	var kotlinPaths, javaPaths []string
	for path := range hashes {
		if strings.HasSuffix(path, ".java") {
			javaPaths = append(javaPaths, path)
		} else {
			kotlinPaths = append(kotlinPaths, path)
		}
	}
	sort.Strings(kotlinPaths)
	sort.Strings(javaPaths)
	return kotlinPaths, javaPaths
}

func collectJavaPathsForFingerprint(args ProjectArgs) ([]string, error) {
	caps := unionNeeds(args.ActiveRules)
	if !caps.Has(api.NeedsCrossFile) && !caps.Has(api.NeedsParsedFiles) && !NeedsJavaSourceDispatch(args.ActiveRules) {
		return nil, nil
	}
	return scanner.CollectJavaFiles(args.Paths, nil)
}

func filterGeneratedSourcePaths(paths []string, includeGenerated bool) []string {
	if includeGenerated {
		return paths
	}
	// Allocate a fresh slice — callers pass args.KotlinPaths /
	// args.JavaPaths whose backing arrays are owned by the CLI runner.
	// A paths[:0] in-place rewrite would mutate those caller slices
	// and leave duplicate entries in their tails, causing parse and
	// dispatch to process the same files multiple times.
	filtered := make([]string, 0, len(paths))
	for _, path := range paths {
		if !strings.Contains(filepath.ToSlash(path), "/generated/") {
			filtered = append(filtered, path)
		}
	}
	return filtered
}

func fileStatsMatch(paths []string, prior map[string]scanner.FileStat) bool {
	if len(paths) != len(prior) {
		return false
	}
	for _, path := range paths {
		current, ok := statForPath(path)
		if !ok || prior[path] != current {
			return false
		}
	}
	return true
}

func statForPath(path string) (scanner.FileStat, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return scanner.FileStat{}, false
	}
	return scanner.FileStat{
		Size:            info.Size(),
		ModTimeUnixNano: info.ModTime().UnixNano(),
	}, true
}

func filesForPaths(paths []string, lang scanner.Language) []*scanner.File {
	files := make([]*scanner.File, 0, len(paths))
	for _, path := range paths {
		files = append(files, &scanner.File{Path: path, Language: lang})
	}
	return files
}

// cachedDetectAndroidProject routes the project-fingerprint and
// resource-source paths through the same daemon-resident
// AndroidProjectCache that IndexPhase.detectAndroidProject uses, so
// preparseBundleFingerprint doesn't pay the ~1s DetectProject tree walk
// twice per analyze. Falls back to a fresh DetectProject when the host
// has no cache wired (CLI path).
func cachedDetectAndroidProject(args ProjectArgs, host ProjectHostState) *android.Project {
	if host.AndroidProjectCache == nil {
		return android.DetectProject(args.Paths)
	}
	return host.AndroidProjectCache(androidProjectFingerprint(args.Paths), func() *android.Project {
		return android.DetectProject(args.Paths)
	})
}

func preparseProjectFingerprints(args ProjectArgs, host ProjectHostState) (string, string) {
	project := host.PrebuiltAndroidProject
	if host.PrebuiltLibraryFacts != nil {
		androidFP := ""
		if project != nil {
			androidFP = libraryFactsFingerprint(project.GradlePaths)
		}
		return androidFP, host.PrebuiltLibraryFacts.Fingerprint()
	}
	if project == nil {
		project = cachedDetectAndroidProject(args, host)
	}
	if project == nil {
		return "", ""
	}
	gradle := project.GradlePaths
	build := func() *librarymodel.Facts {
		return librarymodel.FactsForProfile(librarymodel.ProfileFromGradlePaths(gradle))
	}
	if host.LibraryFactsCache != nil && len(gradle) > 0 {
		androidFP := libraryFactsFingerprint(gradle)
		return androidFP, host.LibraryFactsCache(androidFP, build).Fingerprint()
	}
	return libraryFactsFingerprint(gradle), build().Fingerprint()
}

func fingerprintPathHashMap(values map[string]string) string {
	entries := make([]string, 0, len(values))
	for path, hash := range values {
		entries = append(entries, path+"\x00"+hash)
	}
	sort.Strings(entries)
	return hashutil.HashHex([]byte(strings.Join(entries, "\x01")))
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
	prior, ok := loadBundleManifest(host, manifest.manifestKey)
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
// The byte-level SourceSet keeps direct bundle loads precise for
// identical input. Body-only Kotlin edits that move SourceSet but keep
// CrossFile stable are handled by runDispatchOrLoadBundle's structural
// reuse path.
func computeRunFingerprint(args ProjectArgs, host ProjectHostState, parseResult ParseResult, indexResult IndexResult) (scanner.RunFingerprint, bool) {
	if host.FindingsBundleStore == nil || host.FindingsBundleCacheRoot == "" {
		return scanner.RunFingerprint{}, false
	}
	if len(args.CustomRuleJars) > 0 {
		return scanner.RunFingerprint{}, false
	}
	rulesHash := projectRuleHash(args.ActiveRules, args.Config)
	dirty := dirtyPathSet(host.SourceSetDirty)
	fp := scanner.RunFingerprint{
		Version:   args.Version,
		Rules:     rulesHash,
		Config:    rulesHash,
		SourceSet: sourceSetFingerprint(parseResult.KotlinFiles, parseResult.JavaFiles, host.PriorContentHashes, dirty),
	}
	fp.CrossFile = crossFileStructuralFingerprint(parseResult.KotlinFiles, parseResult.JavaFiles, host.PriorStructuralFPs, dirty)
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
// crossFileStructuralFingerprint hashes sorted (path, structural-fp)
// pairs of every parsed Kotlin and Java file. When the optional prior
// map is non-nil, unchanged paths (per the dirty set) take their
// fingerprint verbatim from prior instead of paying
// scanner.FileStructuralFingerprint per file — a ~3s cost across
// kotlin-corpus's 16 k+ files. CLI callers leave both prior and
// dirty nil and pay the full recompute.
func crossFileStructuralFingerprint(kotlinFiles, javaFiles []*scanner.File, prior map[string]string, dirty map[string]bool) string {
	entries := make([]string, 0, len(kotlinFiles)+len(javaFiles))
	add := func(f *scanner.File) {
		if f == nil {
			return
		}
		fp := priorOrCompute(prior, dirty, f.Path, func() string {
			return scanner.FileStructuralFingerprint(f)
		})
		entries = append(entries, f.Path+"\x00"+fp)
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
// every parsed Kotlin and Java file. Same prior/dirty short-circuit
// shape as crossFileStructuralFingerprint.
func sourceSetFingerprint(kotlinFiles, javaFiles []*scanner.File, prior map[string]string, dirty map[string]bool) string {
	entries := make([]string, 0, len(kotlinFiles)+len(javaFiles))
	add := func(f *scanner.File) {
		if f == nil {
			return
		}
		hash := priorOrCompute(prior, dirty, f.Path, func() string {
			return hashutil.Default().HashContent(f.Path, f.Content)
		})
		entries = append(entries, f.Path+"\x00"+hash)
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

// priorOrCompute returns prior[path] when the path is NOT in the
// dirty set (meaning the watcher believes the file hasn't changed
// since prior was computed). When the path IS dirty, the dirty set
// is nil (no opinion), or the prior map lacks the entry, compute is
// invoked. Used by buildManifestData and the *WithPrior fingerprint
// helpers so daemon callers can skip ~3s per call of recomputing
// per-file hashes / structural fingerprints across the unchanged
// 16 k+ files of a kotlin-corpus scan.
func priorOrCompute(prior map[string]string, dirty map[string]bool, path string, compute func() string) string {
	if prior != nil {
		if dirty == nil || !dirty[path] {
			if v, ok := prior[path]; ok {
				return v
			}
		}
	}
	return compute()
}

// dirtyPathSet materialises a host's SourceSetDirty slice into a
// set for O(1) lookup. nil slice and empty slice are both treated
// as "no dirty paths"; the caller's nil-vs-non-nil semantics on the
// slice still drive whether priorOrCompute consults the prior map.
func dirtyPathSet(dirty []string) map[string]bool {
	if len(dirty) == 0 {
		return nil
	}
	set := make(map[string]bool, len(dirty))
	for _, p := range dirty {
		set[p] = true
	}
	return set
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
