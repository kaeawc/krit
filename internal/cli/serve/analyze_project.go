package serve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"time"

	"github.com/kaeawc/krit/internal/cache"
	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/cli/clishared"
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/pipeline"
	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// errDaemonNotWarm is returned to RequireWarm clients on a cold
// invocation. Sentinel so clients can errors.Is against it without
// string matching.
var errDaemonNotWarm = errors.New("daemon not warm yet")

// handleAnalyzeProject runs the whole-project scan pipeline against
// the daemon's resident state and returns the formatted findings.
//
// Concurrency: serialised on daemonState.analyzeMu across the full
// call. Unlocking before pipeline.RunProject would allow concurrent
// runs, but rules.ApplyConfig (called inside buildProjectInput) writes
// to package-global state (DefaultInactive, rule excludes, custom
// pattern registry) that RunProject reads via AllSuppressionAliases
// and the dispatcher — a verified DATA RACE under -race. Loosening
// the lock requires making the rules config state per-call or
// initialising it once at daemon startup. See issue #53.
func handleAnalyzeProject(_ context.Context, state *daemonState, raw json.RawMessage) (any, error) {
	var args daemon.AnalyzeProjectArgs
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, fmt.Errorf("decode args: %w", err)
		}
	}

	if daemonHash := daemonBinaryHash(); args.ClientBinaryHash != "" && daemonHash != "" && args.ClientBinaryHash != daemonHash {
		return nil, fmt.Errorf("%s (daemon=%s client=%s)", daemon.ErrBinaryHashMismatchPrefix, daemonHash, args.ClientBinaryHash)
	}

	// Validate the requested oracle backend up front so the
	// daemon-resident OracleDaemon spawn picks the right jar.
	// ensureOracleDaemon routes BackendKAA to krit-types and
	// BackendFIR to krit-fir; unrecognised spellings surface as a
	// typed ErrUnsupportedOracleBackendPrefix so the CLI falls back
	// to in-process via runDaemonAnalyze.
	backend, err := oracle.ParseBackend(args.OracleBackend)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", daemon.ErrUnsupportedOracleBackendPrefix, err)
	}

	state.analyzeMu.Lock()
	cold := !state.coldDone.Load()
	if args.RequireWarm && cold {
		state.analyzeMu.Unlock()
		return nil, errDaemonNotWarm
	}
	// Health-check + auto-rebuild any cached oracle daemons before
	// the verb runs. A JVM that died (OOM, host kill) gets replaced
	// here so RunProject sees a live handle. No-op when no daemon is
	// cached. See issue #125 PR-A for the lifecycle plumbing.
	state.pingOracleDaemon()
	start := time.Now()
	dirty := state.workspace.DrainDirty()

	in, err := state.buildProjectInput(args, backend)
	if err != nil {
		state.analyzeMu.Unlock()
		return nil, err
	}
	in.Host.SourceSetClean = !cold && len(dirty) == 0
	if !cold {
		// Hand the watcher's observed-dirty set to the pipeline so
		// preparseSourcePaths can skip the 30-40s cold-OS-dentry-cache
		// filesystem walk when every dirty path is an EDIT of an
		// already-indexed file (not an ADD or DELETE). nil-vs-empty
		// matters: empty means "I'm sure nothing changed", nil means
		// "no opinion" and the pipeline walks.
		if dirty == nil {
			in.Host.SourceSetDirty = []string{}
			in.Host.AnalysisCacheDirty = []string{}
		} else {
			in.Host.SourceSetDirty = dirty
			in.Host.AnalysisCacheDirty = dirty
		}
		// AnalysisCacheDirty opts cacheCheck into the incremental
		// CheckFilesIncremental path: only files in the dirty set
		// get the os.Stat + content-hash NeedsReanalysis check; the
		// rest hit the cache directly. On the kotlin-corpus warm
		// baseline this collapses ~245 ms of 18 k-file stat sweep
		// to a map walk. The watcher's continuous presence is the
		// correctness gate — same trust contract as the resident
		// parsed-trees cache.
		//
		// Cold runs (first analyze after daemon start) leave
		// AnalysisCacheDirty nil so CheckFiles still validates
		// every cached entry against disk — the watcher hasn't been
		// running long enough to vouch for "non-dirty == unchanged".
	}

	// Defer pipeline execution into WriteRawResponse so the
	// OutputPhase JSON streams directly into the connection instead
	// of being buffered in memory (#60). The lock is held for the
	// duration of the run.
	//
	// Strict-verify takes a buffered detour: same envelope shape, but
	// the response runs the daemon path, a cold baseline, and a diff
	// before writing — see strict_verify.go.
	if state.strictVerify {
		return &strictVerifyAnalyzeResponse{
			state:       state,
			args:        args,
			in:          in,
			start:       start,
			cold:        cold,
			dirtyN:      len(dirty),
			releaseLock: state.analyzeMu.Unlock,
		}, nil
	}
	return &streamingAnalyzeResponse{
		state:           state,
		in:              in,
		start:           start,
		cold:            cold,
		dirtyN:          len(dirty),
		profileDispatch: args.ProfileDispatch,
		cpuProfilePath:  args.CPUProfilePath,
		memProfilePath:  args.MemProfilePath,
		basePath:        in.Args.BasePath,
		createBaseline:  args.CreateBaseline,
		dryRun:          args.DryRun,
		includeColumns:  args.IncludeColumns,
		releaseLock:     state.analyzeMu.Unlock,
	}, nil
}

type streamingAnalyzeResponse struct {
	state           *daemonState
	in              pipeline.ProjectInput
	start           time.Time
	cold            bool
	dirtyN          int
	profileDispatch bool
	cpuProfilePath  string
	memProfilePath  string
	basePath        string
	createBaseline  bool
	dryRun          bool
	includeColumns  bool
	releaseLock     func()
}

var _ daemon.RawResponseWriter = (*streamingAnalyzeResponse)(nil)

func (r *streamingAnalyzeResponse) WriteRawResponse(ctx context.Context, w io.Writer) error {
	defer r.releaseLock()

	cpuProfile, profileWarnings := startDaemonCPUProfile(r.cpuProfilePath)

	hw := &analyzeRespWriter{out: w, head: []byte(`{"ok":true,"data":{"findings":`)}
	res, err := pipeline.RunProjectStreaming(ctx, r.in, hw)
	stopDaemonCPUProfile(cpuProfile)
	profileWarnings = append(profileWarnings, writeDaemonMemProfile(r.memProfilePath)...)
	if err != nil {
		if !hw.headWritten {
			return daemon.WriteErrorResponseLine(w, err.Error())
		}
		// Head already flushed; the connection is borked and the
		// caller will close it after seeing the error.
		return err
	}
	r.state.coldDone.Store(true)
	xfile := r.state.workspace.CrossFileStats()

	stats := daemon.AnalyzeProjectStats{
		FilesScanned:      res.FilesScanned,
		FindingsCount:     res.FindingsCount,
		WallSeconds:       time.Since(r.start).Seconds(),
		CodeIndexHit:      xfile.HasCodeIndex,
		LibraryFactsHit:   xfile.HasLibraryFacts,
		ResolverHit:       xfile.HasResolver,
		OracleFilterHit:   xfile.HasOracleFilter,
		DirtyFiles:        r.dirtyN,
		Cold:              r.cold,
		ParseHits:         res.ParseHits,
		ParseMisses:       res.ParseMisses,
		FindingsBundleHit: res.FindingsBundleHit,
		PhaseTimingsMs: daemon.PhaseTimingsMs{
			Parse:     res.PhaseTimingsMs.Parse,
			Index:     res.PhaseTimingsMs.Index,
			Dispatch:  res.PhaseTimingsMs.Dispatch,
			CrossFile: res.PhaseTimingsMs.CrossFile,
			Android:   res.PhaseTimingsMs.Android,
			Fixup:     res.PhaseTimingsMs.Fixup,
			Output:    res.PhaseTimingsMs.Output,
		},
		ProfileWarnings: profileWarnings,
	}
	// CreateBaseline ships a sorted+deduped baseline-ID list so the
	// CLI can write the XML locally — daemon never writes user
	// files. See scanner.CollectBaselineIDs / WriteBaselineIDsXML.
	if r.createBaseline {
		stats.BaselineIDs = scanner.CollectBaselineIDs(&res.FinalFindings, r.basePath)
	}
	// DryRun ships the same file list runFixup → printDryRunFixResult
	// would write to stdout in-process. Iterate post-fixup findings so
	// the FixLevel cap is reflected (StripTextFixes runs inside
	// FixupPhase). FixupResult.Findings carries the post-strip view.
	if r.dryRun {
		dryFiles := collectFixableFiles(&res.Fixup.Findings)
		stats.DryRunFiles = dryFiles
		stats.DryRunFixableCount = res.Fixup.FixableCount
		stats.DryRunStrippedByLevel = res.Fixup.StrippedByLevel
	}
	statsBytes, err := json.Marshal(stats)
	if err != nil {
		return fmt.Errorf("marshal stats: %w", err)
	}
	if _, err := w.Write([]byte(`,"stats":`)); err != nil {
		return err
	}
	if _, err := w.Write(statsBytes); err != nil {
		return err
	}
	if r.profileDispatch && len(res.FileTimings) > 0 {
		profile := daemon.DispatchProfile{
			WallMs:  res.PhaseTimingsMs.Dispatch,
			Workers: runtime.NumCPU(),
			Timings: convertFileTimings(res.FileTimings),
		}
		profileBytes, err := json.Marshal(profile)
		if err != nil {
			return fmt.Errorf("marshal dispatch profile: %w", err)
		}
		if _, err := w.Write([]byte(`,"dispatch_profile":`)); err != nil {
			return err
		}
		if _, err := w.Write(profileBytes); err != nil {
			return err
		}
	}
	if r.includeColumns {
		// Ship the post-pipeline FindingColumns so CLI-side audit /
		// delta paths (--rule-audit, --baseline-audit, --delta) can
		// run their column-oriented work locally without needing to
		// reconstruct the column index from the findings JSON. The
		// segment is only emitted when the caller opts in so non-
		// audit scans keep the original envelope shape.
		columnsBytes, err := json.Marshal(&res.FinalFindings)
		if err != nil {
			return fmt.Errorf("marshal columns: %w", err)
		}
		if _, err := w.Write([]byte(`,"columns":`)); err != nil {
			return err
		}
		if _, err := w.Write(columnsBytes); err != nil {
			return err
		}
	}
	_, err = w.Write(envelopeTail)
	return err
}

// convertFileTimings copies pipeline.FileTiming entries into the
// wire-shaped daemon.FileTiming slice. Two distinct types so the
// daemon protocol package doesn't depend on internal/pipeline.
func convertFileTimings(in []pipeline.FileTiming) []daemon.FileTiming {
	if len(in) == 0 {
		return nil
	}
	out := make([]daemon.FileTiming, len(in))
	for i, t := range in {
		out[i] = daemon.FileTiming{
			Path:     t.Path,
			Size:     t.Size,
			QueueMs:  t.QueueMs,
			RunMs:    t.RunMs,
			LockMs:   t.LockMs,
			AggMs:    t.AggMs,
			TotalMs:  t.TotalMs,
			Findings: t.Findings,
		}
	}
	return out
}

// collectFixableFiles walks columns and returns the sorted, deduped
// list of file paths with at least one available text fix. Mirrors
// the in-process printDryRunFixResult enumeration so the CLI can
// replay the daemon's output line-for-line.
func collectFixableFiles(columns *scanner.FindingColumns) []string {
	if columns == nil {
		return nil
	}
	seen := make(map[string]struct{}, columns.Len())
	for row := 0; row < columns.Len(); row++ {
		if !columns.HasFix(row) {
			continue
		}
		seen[columns.FileAt(row)] = struct{}{}
	}
	if len(seen) == 0 {
		return nil
	}
	files := make([]string, 0, len(seen))
	for file := range seen {
		files = append(files, file)
	}
	sort.Strings(files)
	return files
}

// envelopeTail closes the streaming response: end-of-stats, end-of-data,
// end-of-envelope, line-delimited newline.
var envelopeTail = []byte("}}\n")

// newlineSlice is referenced from analyzeRespWriter when reissuing a
// held trailing newline so each held-flush doesn't allocate a 1-byte
// slice.
var newlineSlice = []byte{'\n'}

// analyzeRespWriter wraps the underlying writer with two transforms:
// it lazily emits an envelope head on the first non-empty Write (so a
// pre-output pipeline error can still produce a clean error envelope),
// and it strips the trailing '\n' that json.Encoder appends to its
// value (the daemon wire is line-delimited; an internal '\n' would let
// the client's bufio.Reader.ReadBytes('\n') return a truncated body).
// Held newlines are reissued before the next chunk so internal '\n'
// bytes survive — only the final one is silently dropped.
type analyzeRespWriter struct {
	out         io.Writer
	head        []byte
	headWritten bool
	heldNewline bool
}

func (w *analyzeRespWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if !w.headWritten {
		if _, err := w.out.Write(w.head); err != nil {
			return 0, err
		}
		w.headWritten = true
	}
	if w.heldNewline {
		if _, err := w.out.Write(newlineSlice); err != nil {
			return 0, err
		}
		w.heldNewline = false
	}
	n := len(p)
	if p[n-1] == '\n' {
		w.heldNewline = true
		p = p[:n-1]
		if len(p) == 0 {
			return n, nil
		}
	}
	if _, err := w.out.Write(p); err != nil {
		return 0, err
	}
	return n, nil
}

// buildProjectInput translates wire args into a pipeline.ProjectInput
// against daemon-resident state. The function is the single seam
// where CLI-flag-style knobs (rule lists, format) are wired into the
// pipeline's typed value inputs. backend selects which JVM jar the
// resident OracleDaemon spawns (krit-types vs krit-fir); the value
// is parsed/validated upstream in handleAnalyzeProject.
func (s *daemonState) buildProjectInput(args daemon.AnalyzeProjectArgs, backend oracle.Backend) (pipeline.ProjectInput, error) {
	cfg, err := s.ensureConfig()
	if err != nil {
		return pipeline.ProjectInput{}, fmt.Errorf("load config: %w", err)
	}
	rules.ApplyConfig(cfg)

	disabledSet := clishared.ParseRuleNameSetCSV(args.DisableRules)
	enabledSet := clishared.ParseRuleNameSetCSV(args.EnableRules)
	experimental := args.Experimental || cfg.GetTopLevelBool("experimental", false)
	strict := args.Strict || cfg.GetTopLevelBool("strict", false)
	activeRules := rules.ActiveRulesV2(disabledSet, enabledSet, args.AllRules, experimental, strict)

	paths := args.Paths
	if len(paths) == 0 {
		paths = []string{s.root}
	}

	// Cached at construction — oracle.FindRepoDir is a filesystem walk
	// for VCS markers and the answer can't change for the duration of
	// a daemon process. See #49.
	repoDir := s.repoDir
	if repoDir == "" {
		repoDir = s.root
	}
	androidCacheWriter, androidCacheDir := s.androidCacheWriterFor(repoDir)
	// --no-cache disables every on-disk cache for this single call:
	// parse cache, AnalysisCache, findings bundle store, cross-file/
	// findings/type-index disk shards. Resident WorkspaceState slots
	// stay live — they're per-process memory keyed on input
	// fingerprints, so a no-cache call can reuse them without
	// dirtying state for subsequent calls. The next analyze without
	// NoCache resumes normal cache behavior.
	parseCache, err := s.parseCacheFor(repoDir, cacheutil.DefaultParseCacheCapBytes)
	if err != nil {
		// A failed parse cache isn't fatal — RunProject tolerates
		// nil and falls back to direct tree-sitter parses. Log via
		// the reporter once available; for now silently degrade so
		// the verb stays useful even when the cache directory is
		// read-only.
		parseCache = nil
	}
	if args.NoCache {
		parseCache = nil
	}

	// AnalysisCache is daemon-resident: lazy-loaded on first request,
	// keyed by the resolved cache file path so distinct scan-path sets
	// don't share an entry. DispatchPhase will merge new findings into
	// it and persist to disk on each call.
	var (
		analysisCache     *cache.Cache
		analysisCachePath string
	)
	if !args.NoCache {
		analysisCache, analysisCachePath = s.analysisCacheFor(paths)
	}

	// OracleDaemon is daemon-resident: lazy-started on first request
	// when the matching jar (krit-types for BackendKAA, krit-fir for
	// BackendFIR) is found. nil daemon means oracle stays disabled
	// (no JVM, no behavior change). Per-verb ping happens at the
	// call boundary in handleAnalyzeProject.
	oracleDaemon, err := s.ensureOracleDaemon(paths, backend)
	if err != nil {
		// Best-effort degrade: a failed daemon start shouldn't fail
		// the whole verb. Log via stderr; the verb continues with
		// oracle disabled.
		fmt.Fprintf(os.Stderr, "warning: oracle daemon start: %v\n", err)
		oracleDaemon = nil
	}

	// Prepopulate the source-path slices from the prior manifest when
	// available so runProjectParsePhase doesn't walk the filesystem
	// again on every analyze. The pipeline already trusts these as
	// canonical when args.KotlinPaths is non-nil. fsnotify/manifest
	// drift is caught at the bundle-fingerprint comparison, which
	// runs whether or not we walk; bypassing the walk just saves the
	// 30-40s cold-OS-dentry-cache cost on kotlin-corpus scale.
	kotlinPaths, javaPaths := s.prepopulatedSourcePaths(repoDir, paths)
	// Pull the prior content-hash and structural-fp maps off the
	// resident manifest so RunProjectAnalysis can short-circuit per-
	// file fingerprint recomputation for the 16 k+ unchanged files of
	// a kotlin-corpus scan. Returns nil maps when no prior is cached;
	// in that case the pipeline falls back to recomputing per-file.
	priorManifest, _ := s.priorManifest(repoDir, paths)

	// Build a perf.Tracker only when the caller asked for --perf or
	// --perf-rules. A nil-interface (untyped) Tracker keeps every
	// pipeline.host.Tracker check a no-op; constructing one
	// unconditionally would steal the noopTracker fast path the
	// non-perf hot path relies on.
	var perfTracker perf.Tracker
	if args.ShowPerf || args.PerfRules {
		perfTracker = perf.New(true)
	}

	diskCache := s.resolveDiskCacheWiring(args, repoDir, androidCacheWriter, androidCacheDir)
	baselinePath, basePath, maxFixLevel := resolveBaselineDryRunArgs(args, paths)
	return pipeline.ProjectInput{
		Args: pipeline.ProjectArgs{
			Config:           cfg,
			Paths:            paths,
			KotlinPaths:      kotlinPaths,
			JavaPaths:        javaPaths,
			ActiveRules:      activeRules,
			Format:           args.Format,
			BaselinePath:     baselinePath,
			DiffRef:          args.DiffRef,
			MinConfidence:    args.MinConfidence,
			WarningsAsErrors: args.WarningsAsErrors,
			IncludeGenerated: args.IncludeGenerated,
			Version:          kritVersion(),
			OracleEnabled:    oracleDaemon != nil || args.InputTypesPath != "",
			ShowPerf:         args.ShowPerf || args.PerfRules,
			PerfRules:        args.PerfRules,
			ProfileDispatch:  args.ProfileDispatch,
			CustomRuleJars:   args.CustomRuleJars,
			InputTypesPath:   args.InputTypesPath,
			DryRun:           args.DryRun,
			MaxFixLevel:      maxFixLevel,
			BasePath:         basePath,
			// Wire is line-delimited; compact JSON keeps the body
			// free of internal newlines.
			JSONCompact: true,
			// IncludeColumns asks the pipeline for the post-pipeline
			// FindingColumns so streamingAnalyzeResponse can ship them
			// in the "columns" wire segment. Without this the pre-Load
			// BundleOutput cache fast-path returns FinalFindings empty
			// and the CLI gets {} for columns.
			RequireFinalFindings: args.IncludeColumns,
		},
		Host: pipeline.ProjectHostState{
			ParseCache:          parseCache,
			ResidentFiles:       s.workspace,
			Tracker:             perfTracker,
			LibraryFactsCache:   s.workspace.LibraryFacts,
			CodeIndexCache:      s.workspace.CodeIndex,
			XMLFilesLoader:      s.workspace.XMLFiles,
			ResolverCache:       s.workspace.Resolver,
			OracleFilterCache:   s.workspace.OracleFilter,
			AndroidProjectCache: s.workspace.AndroidProject,
			DaemonCaches: pipeline.DaemonCaches{
				CodeIndexSnapshotLoader:  s.workspace.LoadCodeIndexSnapshot,
				CodeIndexSnapshotSaver:   s.workspace.StoreCodeIndexSnapshot,
				JavaSourceIndexCache:     s.workspace.JavaSourceIndex,
				ResolverFingerprintCache: s.workspace.ResolverFingerprint,
				GradleFindingsCache:      s.workspace.GradleFindings,
				BundleStatsClean:         s.workspace.BundleStatsClean,
				MarkBundleStatsClean:     s.workspace.MarkBundleStatsClean,
				SourceMTimeVersion:       s.workspace.SourceMTimeVersion,
				BundleOutput:             s.workspace.BundleOutput,
				StoreBundleOutput:        s.workspace.StoreBundleOutput,
			},
			AndroidCacheWriter:           diskCache.androidCacheWriter,
			AndroidCacheDir:              diskCache.androidCacheDir,
			CrossFileCacheDir:            diskCache.crossFileCacheDir,
			CrossFindingsCacheDir:        diskCache.crossFindingsCacheDir,
			TypeIndexCacheDir:            diskCache.typeIndexCacheDir,
			ResidentFileTypeInfo:         s.workspace,
			AnalysisCache:                analysisCache,
			AnalysisCacheFilePath:        analysisCachePath,
			AnalysisCacheLookup:          analysisCache != nil,
			OracleDaemon:                 oracleDaemon,
			FindingsBundleStore:          diskCache.findingsBundleStore,
			FindingsBundleCacheRoot:      diskCache.findingsBundleRoot,
			FindingsBundleManifestLoader: diskCache.manifestLoader,
			FindingsBundleManifestSaver:  diskCache.manifestSaver,
			PriorContentHashes:           priorManifest.ContentHashes,
			PriorStructuralFPs:           priorManifest.StructuralFPs,
			PriorAbiHashes:               priorManifest.AbiHashes,
		},
	}, nil
}

// diskCacheWiring bundles the on-disk cache pointers buildProjectInput
// hands to the pipeline host. Aggregating these lets the --no-cache
// branch flip the whole set in one place without inflating
// buildProjectInput's cyclomatic complexity.
type diskCacheWiring struct {
	crossFileCacheDir     string
	crossFindingsCacheDir string
	typeIndexCacheDir     string
	androidCacheWriter    *scanner.AndroidCacheWriter
	androidCacheDir       string
	findingsBundleStore   scanner.FindingsBundleStore
	findingsBundleRoot    string
	manifestLoader        pipeline.FindingsBundleManifestLoader
	manifestSaver         pipeline.FindingsBundleManifestSaver
}

// resolveDiskCacheWiring computes the per-call on-disk cache pointer
// set. --no-cache zeroes every pointer for this single call;
// resident WorkspaceState slots stay live (they're per-process memory
// keyed on input fingerprints, so reuse is idempotent and a no-cache
// call cannot dirty them for subsequent calls).
func (s *daemonState) resolveDiskCacheWiring(args daemon.AnalyzeProjectArgs, repoDir string, androidCacheWriter *scanner.AndroidCacheWriter, androidCacheDir string) diskCacheWiring {
	if args.NoCache {
		// Drop Android cache pointers too — the
		// androidFindingsCacheable gate checks both writer and dir, so
		// emptying them matches the other on-disk cache zeroing.
		return diskCacheWiring{}
	}
	return diskCacheWiring{
		crossFileCacheDir:     scanner.CrossFileCacheDir(repoDir),
		crossFindingsCacheDir: scanner.CrossFindingsCacheDir(repoDir),
		typeIndexCacheDir:     typeinfer.TypeIndexCacheDir(repoDir),
		androidCacheWriter:    androidCacheWriter,
		androidCacheDir:       androidCacheDir,
		findingsBundleStore:   scanner.DiskFindingsBundleStore{},
		findingsBundleRoot:    repoDir,
		manifestLoader:        s.loadManifest,
		manifestSaver:         s.saveManifest,
	}
}

// resolveBaselineDryRunArgs resolves three knobs that the CLI-side
// dry-run / create-baseline routing relies on:
//   - baselinePath: dropped to "" when CreateBaseline is set so
//     OutputPhase's applyBaseline never runs (mirrors the in-process
//     short-circuit).
//   - basePath: defaults to the first scan path when empty so daemon-
//     computed baseline IDs match WriteBaselineColumns byte-for-byte.
//   - maxFixLevel: parses FixLevel only when DryRun is set; empty or
//     unknown values leave the cap at zero (matches FixupPhase
//     semantics: no cap).
func resolveBaselineDryRunArgs(args daemon.AnalyzeProjectArgs, paths []string) (string, string, rules.FixLevel) {
	baselinePath := args.BaselinePath
	if args.CreateBaseline {
		baselinePath = ""
	}
	basePath := args.BasePath
	if basePath == "" && len(paths) > 0 {
		basePath = paths[0]
	}
	maxFixLevel := rules.FixLevel(0)
	if args.DryRun && args.FixLevel != "" {
		if lvl, ok := rules.ParseFixLevel(args.FixLevel); ok {
			maxFixLevel = lvl
		}
	}
	return baselinePath, basePath, maxFixLevel
}

// loadDaemonConfig loads the krit.yml the daemon should honour for
// rule selection. Mirrors the CLI's two-source precedence (user
// config in repo + default config), but tolerates a missing config
// silently — the daemon should be useful out of the box without
// requiring krit.yml.
func loadDaemonConfig(root string) (*config.Config, error) {
	defaultCfgPath := config.FindDefaultConfig()
	userCfgPath := clishared.FindConfigInDir(root)
	cfg, mergeErr := config.LoadAndMerge(userCfgPath, defaultCfgPath)
	if cfg == nil {
		cfg = config.NewConfig()
	}
	// Surface load errors via wrap rather than fatal — the daemon
	// already returns a Config (default or partial) that callers can
	// proceed with.
	if mergeErr != nil {
		return cfg, fmt.Errorf("config merge: %w", mergeErr)
	}
	return cfg, nil
}
