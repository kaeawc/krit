package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"time"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/cache"
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/rules"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/store"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// IndexPhase builds the non-per-file analysis state downstream phases
// need: a type resolver (with its parallel index), a module graph, and
// the Android project descriptor. Oracle, CodeIndex, and incremental
// cache are populated by callers that pre-build them (LSP caches them
// across edits, the CLI wires them through the IndexInput hooks). This
// phase's job is the cheap, deterministic pieces that always need to run.
type IndexPhase struct {
	// Workers overrides the type-index worker count. Zero = runtime.NumCPU().
	Workers int
	// ScanRoot, if non-empty, is passed to module.DiscoverModules.
	// Empty means "use the first element of ParseResult.Paths, or '.'".
	ScanRoot string
	// SkipModules, when true, leaves ModuleGraph nil. Used by the LSP
	// server where a single open file is analysed without a Gradle
	// project around it.
	SkipModules bool
	// SkipAndroid, when true, leaves AndroidProject nil.
	SkipAndroid bool
	// SkipResolverIndex, when true, prevents IndexPhase from calling
	// IndexFilesParallel on the resolver even when NeedsResolver is set
	// and no PrebuiltResolver was supplied. CLI callers that drive the
	// parallel type-index themselves (so the existing typeIndex perf
	// label stays at the CLI tracker scope) pass SkipResolverIndex=true.
	SkipResolverIndex bool
}

// IndexInput carries ParseResult plus optional pre-built resolver and
// oracle from callers that already have them (LSP, MCP, --input-types).
// When these are nil, IndexPhase builds fresh state.
type IndexInput struct {
	ParseResult
	// PrebuiltResolver, when non-nil, is used as-is; no IndexFilesParallel
	// call is made. When nil, IndexPhase builds one if any active rule
	// declares NeedsResolver.
	PrebuiltResolver typeinfer.TypeResolver
	// Logger, when non-nil, receives verbose progress messages. Nil
	// means no-op.
	Logger func(format string, args ...any)
	// Tracker, when non-nil, wraps expensive sub-phases with
	// Tracker.Serial(name). Nil means no tracking.
	Tracker perf.Tracker

	// --- Oracle construction knobs. These mirror the CLI flags that used
	// to drive the oracle block in cmd/krit/main.go. When OracleEnabled
	// is false, IndexPhase leaves Oracle/Resolver untouched by the oracle
	// pipeline and only wires PrebuiltResolver / NeedsResolver handling.

	// OracleEnabled, when true, runs the oracle construction pipeline
	// (auto-detect, --input-types, --daemon) inside IndexPhase. When
	// false, the oracle block is skipped entirely (matches --no-type-oracle
	// or absence of a base resolver in the CLI).
	OracleEnabled bool
	// BaseResolver is the v1 typeinfer resolver the CLI pre-built before
	// wrapping in oracle.CompositeResolver. When nil, no oracle wiring
	// happens (there is nothing to wrap).
	BaseResolver typeinfer.TypeResolver
	// OracleScanPaths are the raw CLI scan paths (flag.Args) used by the
	// oracle helpers (FindJar, FindSourceDirs, CachePath, FindRepoDir).
	// When nil, IndexInput.Paths is used.
	OracleScanPaths []string
	// KotlinFilePaths is the collected list of Kotlin file paths used by
	// the oracle filter's byte-only pre-scan. Kept separate from
	// ParseResult.KotlinFiles because the filter runs before the parse
	// and so fully-parsed *scanner.File values are not available.
	KotlinFilePaths []string
	// InputTypesPath mirrors --input-types: explicit oracle JSON path
	// that short-circuits auto-detect.
	InputTypesPath string
	// NoCacheOracle mirrors --no-cache-oracle: disables the on-disk
	// incremental oracle cache.
	NoCacheOracle bool
	// NoOracleFilter mirrors --no-oracle-filter: disables the
	// rule-classification oracle filter pre-scan.
	NoOracleFilter bool
	// UseDaemon mirrors --daemon: use the long-lived krit-types daemon
	// instead of one-shot invocation.
	UseDaemon bool
	// Store is the optional unified store for oracle cache entries.
	Store *store.FileStore
	// OracleCacheWriter, when non-nil, defers cold oracle cache-entry
	// persistence until the caller flushes it later in the run.
	OracleCacheWriter *oracle.OracleCacheWriter
	// Verbose gates oracle stderr diagnostics. Matches --verbose.
	Verbose bool

	// --- Incremental cache knobs. These mirror the CLI flags that used to
	// drive the cache load/lookup block in cmd/krit/main.go. When
	// CacheEnabled is false, IndexPhase skips cache load/lookup entirely.
	// Cache write-back (post-dispatch merge) stays in the caller for now.

	// CacheEnabled, when true, runs cache.Load + CheckFiles inside
	// IndexPhase. When false, no cache work happens (matches --no-cache).
	CacheEnabled bool
	// CacheFilePath is the resolved cache file location (from
	// cache.ResolveCacheDir). Required when CacheEnabled is true.
	CacheFilePath string
	// CacheDirExplicit, when true, matches "--cache-dir was set
	// explicitly". Controls the "verbose: Cache file: ..." log line.
	CacheDirExplicit bool
	// CacheScanPaths are the raw CLI scan paths passed to
	// analysisCache.CheckFiles (base path calculation). When nil,
	// IndexInput.Paths is used.
	CacheScanPaths []string
	// CacheFilePaths is the collected list of Kotlin file paths used for
	// the per-file cache lookup. When nil, IndexInput.KotlinFilePaths is
	// used.
	CacheFilePaths []string
	// CacheConfig is the *config.Config used to compute the rule hash.
	CacheConfig *config.Config
	// CacheRuleNames are the active rule names (v1) used to compute the
	// rule hash. When nil, derived from ActiveRulesV1.
	CacheRuleNames []string
	// CacheEditorConfigEnabled is the --editorconfig flag value used by
	// ComputeConfigHash.
	CacheEditorConfigEnabled bool

	// --- Oracle/cache bypass knobs. The CLI runs IndexPhase twice: once
	// before ParsePhase for oracle + cache (pre-parse block) and once
	// after ParsePhase for CodeIndex / module graph / Android. The second
	// call sets these to true so oracle/cache work doesn't re-run.

	// SkipOracle, when true, bypasses the oracle construction block
	// regardless of OracleEnabled. Used by the post-parse IndexPhase call.
	SkipOracle bool
	// SkipCache, when true, bypasses the cache load block regardless of
	// CacheEnabled. Used by the post-parse IndexPhase call.
	SkipCache bool

	// --- CodeIndex construction knobs. These mirror the pre-refactor
	// cross-file block in cmd/krit/main.go. IndexPhase builds the
	// scanner.CodeIndex (optionally with Java files parsed in parallel)
	// under a caller-supplied parent tracker so the "crossFileAnalysis"
	// perf label stays at the CLI tracker scope.

	// BuildCodeIndex, when true, runs Java collection + parse and
	// scanner.BuildIndex. Matches the pre-refactor
	// hasIndexBackedCrossFileRule branch.
	BuildCodeIndex bool
	// CrossFileParentTracker is the caller-created parent tracker
	// (tracker.Serial("crossFileAnalysis")) under which IndexPhase nests
	// the "javaIndexing", "codeIndexBuild", and inner "indexBuild"
	// children. The caller is responsible for End()-ing it so rule
	// execution siblings can nest under the same parent.
	CrossFileParentTracker perf.Tracker
	// CrossFileJobsFlag is the -jobs flag value used by
	// phaseWorkerCount("crossFileAnalysis", ...).
	CrossFileJobsFlag int
	// CrossFileCacheDir enables the on-disk cross-file index cache when
	// non-empty; empty forces a full rebuild every run.
	CrossFileCacheDir string
	// CrossFileWorkers is the pre-computed initial worker count for the
	// Kotlin-only case (before Java file paths are known). IndexPhase
	// recomputes when Java files land. When zero, IndexPhase derives it
	// from CrossFileJobsFlag + len(KotlinFiles).
	CrossFileWorkers int
	// ParseCache, when non-nil, is consulted during the Java parse step
	// inside the javaIndexing tracker. Cache hits skip tree-sitter
	// entirely; misses parse and populate the cache. Shared with the
	// Kotlin parse loop in ParsePhase — Java entries live in a sibling
	// subdir keyed on the Java grammar version, so a tree-sitter-java
	// upgrade evicts Java entries without touching cached Kotlin trees.
	ParseCache *scanner.ParseCache

	// --- Module graph + PerModuleIndex knobs. Mirrors the pre-refactor
	// moduleAwareAnalysis block in cmd/krit/main.go.

	// BuildModuleIndex, when true, runs module.DiscoverModules and, when
	// any active rule declares module-awareness, builds the
	// PerModuleIndex. Matches the pre-refactor moduleAwareAnalysis block.
	BuildModuleIndex bool
	// ModuleParentTracker is the caller-created parent tracker
	// (tracker.Serial("moduleAwareAnalysis")) under which IndexPhase
	// nests moduleDiscovery / moduleDependencies / moduleIndexBuild
	// children. The caller End()-s it so moduleRuleExecution can nest
	// under the same parent.
	ModuleParentTracker perf.Tracker
	// ModuleScanRoot is the root path passed to module.DiscoverModules.
	// Empty means "use Paths[0] or '.'".
	ModuleScanRoot string
	// ModuleJobsFlag is the -jobs flag value used by
	// phaseWorkerCount("moduleAwareAnalysis", ...).
	ModuleJobsFlag int
	// ModuleHasAwareRule is the pre-computed boolean indicating whether
	// any active rule declares module-awareness. When false, IndexPhase
	// only runs moduleDiscovery (matches pre-refactor behaviour).
	ModuleHasAwareRule bool
}

// logf invokes Logger when set; nil Logger is a no-op.
func (in IndexInput) logf(format string, args ...any) {
	if in.Logger != nil {
		in.Logger(format, args...)
	}
}

// trackSerial runs fn under a child Tracker named name when Tracker is
// non-nil; otherwise it just runs fn.
func (in IndexInput) trackSerial(name string, fn func() error) error {
	if in.Tracker == nil {
		return fn()
	}
	child := in.Tracker.Serial(name)
	err := fn()
	child.End()
	return err
}

// Name implements Phase.
func (IndexPhase) Name() string { return "index" }

// Run implements Phase.
func (p IndexPhase) Run(ctx context.Context, in IndexInput) (IndexResult, error) {
	if err := ctx.Err(); err != nil {
		return IndexResult{}, err
	}

	result := IndexResult{ParseResult: in.ParseResult}

	caps := unionNeeds(in.ActiveRules)

	// Oracle construction (auto-detect / --input-types / --daemon).
	// When OracleEnabled is false or BaseResolver is nil there's nothing
	// to wrap; the CLI caller sets those gates to match --no-type-oracle.
	// When enabled, this block mirrors the pre-refactor main.go oracle
	// pipeline verbatim so verbose log lines and perf tracker labels
	// stay byte-identical.
	resolverForOracle := in.BaseResolver
	if !in.SkipOracle && in.OracleEnabled && resolverForOracle != nil {
		resolverForOracle = p.runOracle(in, resolverForOracle, &result)
	}

	// Type resolver
	if in.PrebuiltResolver != nil {
		result.Resolver = in.PrebuiltResolver
	} else if resolverForOracle != nil {
		// When the CLI supplies a BaseResolver (optionally wrapped by
		// the oracle pipeline above), surface it here. The CLI drives
		// the typeIndex serial tracker itself for byte-identical perf
		// output, so no IndexFilesParallel call happens in this branch.
		result.Resolver = resolverForOracle
	} else if caps.Has(v2.NeedsResolver) {
		r := typeinfer.NewResolver()
		if err := ctx.Err(); err != nil {
			return IndexResult{}, err
		}
		workers := p.Workers
		if workers <= 0 {
			workers = runtime.NumCPU()
		}
		if indexer, ok := interface{}(r).(interface {
			IndexFilesParallel([]*scanner.File, int)
		}); ok {
			_ = in.trackSerial("typeIndex", func() error {
				indexer.IndexFilesParallel(in.KotlinFiles, workers)
				return nil
			})
			in.logf("verbose: Indexed %d Kotlin files for type resolution", len(in.KotlinFiles))
		}
		result.Resolver = r
	}

	// Module graph (legacy LSP-style path: no tracker, best-effort, no
	// dependencies/PerModuleIndex). Superseded by BuildModuleIndex for
	// CLI callers that need tracker parity.
	if !p.SkipModules && !in.BuildModuleIndex {
		scanRoot := p.ScanRoot
		if scanRoot == "" {
			scanRoot = "."
			if len(in.Paths) > 0 {
				scanRoot = in.Paths[0]
			}
		}
		if graph, err := module.DiscoverModules(scanRoot); err == nil {
			result.ModuleGraph = graph
		}
	}

	// Android project
	if !p.SkipAndroid {
		result.AndroidProject = android.DetectAndroidProject(in.Paths)
	}

	// CodeIndex build (+ Java collection/parse) under the
	// caller-supplied crossFileAnalysis parent tracker. Mirrors the
	// pre-refactor hasIndexBackedCrossFileRule branch in cmd/krit/main.go.
	if in.BuildCodeIndex {
		p.runCodeIndexBuild(in, &result)
	}

	// Module graph + PerModuleIndex build under the caller-supplied
	// moduleAwareAnalysis parent tracker. Mirrors the pre-refactor
	// moduleAwareAnalysis block's indexing phase (rule execution stays
	// in main.go).
	if in.BuildModuleIndex {
		p.runModuleIndexBuild(in, &result)
	}

	// Incremental analysis cache load + per-file lookup. Write-back
	// (post-dispatch merge of new findings) still lives in the CLI. This
	// mirrors the pre-refactor block in cmd/krit/main.go verbatim so
	// tracker labels ("cacheLoad") and verbose stderr lines stay
	// byte-identical.
	if !in.SkipCache && in.CacheEnabled {
		p.runCacheLoad(in, &result)
	}

	return result, nil
}

// runCacheLoad is a verbatim port of the pre-refactor cache load/lookup
// block in cmd/krit/main.go. It computes the rule hash, loads the cache
// JSON, optionally attaches the unified store, runs CheckFiles for
// per-path hit/miss classification, and populates IndexResult.Cache,
// CacheResult, RuleHash, CacheFilePath, and CacheStats for downstream
// consumers. The cache write-back (UpdateEntryColumns + Save) is NOT
// done here — it lives with dispatch in main.go and moves to Fixup in a
// later slice.
func (p IndexPhase) runCacheLoad(in IndexInput, result *IndexResult) {
	ruleNames := in.CacheRuleNames
	if ruleNames == nil {
		ruleNames = make([]string, 0, len(in.ActiveRules))
		for _, r := range in.ActiveRules {
			if r != nil {
				ruleNames = append(ruleNames, r.ID)
			}
		}
	}
	ruleHash := cache.ComputeConfigHash(ruleNames, in.CacheConfig, in.CacheEditorConfigEnabled)

	var loadStart time.Time
	var analysisCache *cache.Cache
	_ = in.trackSerial("cacheLoad", func() error {
		loadStart = time.Now()
		analysisCache = cache.Load(in.CacheFilePath)
		return nil
	})

	// Attach the unified store when available so incremental cache
	// entries are persisted per-file in the store instead of the JSON file.
	if in.Store != nil {
		analysisCache.AttachStore(in.Store, cache.ParseRuleSetHash(ruleHash))
	}
	loadDur := time.Since(loadStart).Milliseconds()

	scanPaths := in.CacheScanPaths
	if scanPaths == nil {
		scanPaths = in.Paths
	}
	filePaths := in.CacheFilePaths
	if filePaths == nil {
		filePaths = in.KotlinFilePaths
	}
	cacheResult := analysisCache.CheckFiles(filePaths, ruleHash, scanPaths...)

	cacheStats := &cache.CacheStats{
		Cached:    cacheResult.TotalCached,
		Total:     cacheResult.TotalFiles,
		LoadDurMs: loadDur,
	}
	if cacheResult.TotalFiles > 0 {
		cacheStats.HitRate = float64(cacheResult.TotalCached) / float64(cacheResult.TotalFiles)
	}

	if in.Verbose && cacheResult.TotalCached > 0 {
		pct := 100 * cacheResult.TotalCached / cacheResult.TotalFiles
		fmt.Fprintf(os.Stderr, "verbose: Cache: %d/%d files cached (%d%% hit rate)\n",
			cacheResult.TotalCached, cacheResult.TotalFiles, pct)
		if in.CacheDirExplicit {
			fmt.Fprintf(os.Stderr, "verbose: Cache file: %s\n", in.CacheFilePath)
		}
	}

	result.Cache = analysisCache
	result.CacheResult = cacheResult
	result.RuleHash = ruleHash
	result.CacheFilePath = in.CacheFilePath
	result.CacheStats = cacheStats
}

// Compile-time check.
var _ Phase[IndexInput, IndexResult] = IndexPhase{}

// runOracle is a verbatim port of the pre-refactor oracle block in
// cmd/krit/main.go (previously lines 716-886). It auto-detects a
// krit-types oracle JSON, honours --input-types, --daemon,
// --no-cache-oracle, and --no-oracle-filter, starts the JVM daemon
// where requested, and wraps base in an oracle.CompositeResolver on
// success. Verbose stderr lines and perf tracker labels match the
// pre-refactor output exactly.
func (p IndexPhase) runOracle(in IndexInput, base typeinfer.TypeResolver, result *IndexResult) typeinfer.TypeResolver {
	resolver := base
	scanPaths := in.OracleScanPaths
	if scanPaths == nil {
		scanPaths = in.Paths
	}

	oracleTracker := p.oracleTracker(in)

	if in.UseDaemon {
		// Daemon mode: start a long-lived JVM process
		var d *oracle.Daemon
		var daemonErr error
		oracleTracker.Track("jvmStart", func() error {
			d, daemonErr = oracle.InvokeDaemon(scanPaths, in.Verbose)
			return daemonErr
		})
		if daemonErr != nil {
			fmt.Fprintf(os.Stderr, "warning: daemon: %v\n", daemonErr)
		} else {
			// Surface the daemon handle so main can defer Close() at
			// program exit, matching the pre-refactor lifetime.
			result.Daemon = d

			var oracleData *oracle.OracleData
			oracleTracker.Track("jvmAnalyze", func() error {
				od, err := d.AnalyzeAll()
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: daemon analyzeAll: %v\n", err)
					return err
				}
				oracleData = od
				return nil
			})
			if oracleData != nil {
				var oracleLoaded *oracle.Oracle
				oracleTracker.Track("indexBuild", func() error {
					ol, err := oracle.LoadFromData(oracleData)
					if err != nil {
						fmt.Fprintf(os.Stderr, "warning: daemon oracle: %v\n", err)
						return err
					}
					oracleLoaded = ol
					return nil
				})
				if oracleLoaded != nil {
					resolver = oracle.NewCompositeResolver(oracleLoaded, resolver)
					result.Oracle = oracleLoaded
					if in.Verbose {
						depCount := len(oracleLoaded.Dependencies())
						fmt.Fprintf(os.Stderr, "verbose: Type oracle loaded from daemon (%d dependency types)\n", depCount)
					}
				}
			}
		}
	} else {
		var oraclePath string

		oracleTracker.Track("findSources", func() error {
			if in.InputTypesPath != "" {
				// Explicit input path
				oraclePath = in.InputTypesPath
				return nil
			}
			// Auto-detect: try cached types.json
			cached := oracle.CachePath(scanPaths)
			if cached != "" {
				if _, err := os.Stat(cached); err == nil {
					oraclePath = cached
				}
			}
			return nil
		})

		// If no cached oracle, try to run krit-types automatically
		if oraclePath == "" {
			jvmTracker := oracleTracker.Serial("jvmAnalyze")
			_ = func() error {
				var jarPath string
				jvmTracker.Track("findJar", func() error {
					jarPath = oracle.FindJar(scanPaths)
					return nil
				})
				if jarPath == "" {
					return nil
				}
				var sourceDirs []string
				jvmTracker.Track("findSourceDirs", func() error {
					sourceDirs = oracle.FindSourceDirs(scanPaths)
					return nil
				})
				if len(sourceDirs) == 0 {
					return nil
				}
				perf.AddEntryDetails(jvmTracker, "sourceDirsFound", 0, map[string]int64{"sourceDirs": int64(len(sourceDirs))}, nil)
				var cacheDest string
				jvmTracker.Track("resolveOracleCachePath", func() error {
					cacheDest = oracle.CachePath(scanPaths)
					return nil
				})
				if cacheDest == "" {
					cacheDest = filepath.Join(os.TempDir(), "krit-types.json")
				}
				if in.Verbose {
					fmt.Fprintf(os.Stderr, "verbose: Running krit-types (%d source dirs)...\n", len(sourceDirs))
				}
				// Pre-scan step: compute the subset of files any enabled
				// rule has declared (via NeedsOracle + OracleFilter) it
				// actually needs oracle access on. Rules that do NOT
				// declare NeedsOracle are skipped entirely — only rules
				// that opted in contribute to the union. Guard with
				// -no-oracle-filter for diagnostics / baseline
				// reproduction.
				var filterListPath string
				if !in.NoOracleFilter {
					var filterRules []oracle.OracleFilterRule
					jvmTracker.Track("oracleFilterBuildRules", func() error {
						filterRules = rules.BuildOracleFilterRulesV2(in.ActiveRules)
						return nil
					})
					var lightFiles []*scanner.File
					jvmTracker.Track("oracleFilterLoadFiles", func() error {
						lightFiles = loadFilesForOracleFilter(in.KotlinFilePaths)
						return nil
					})
					var summary oracle.OracleFilterSummary
					jvmTracker.Track("oracleFilterCollect", func() error {
						summary = oracle.CollectOracleFiles(filterRules, lightFiles)
						return nil
					})
					perf.AddEntryDetails(jvmTracker, "oracleFilterSummary", 0, map[string]int64{
						"totalFiles":  int64(summary.TotalFiles),
						"markedFiles": int64(summary.MarkedFiles),
					}, map[string]string{"fingerprint": summary.Fingerprint})
					if in.Verbose {
						switch {
						case summary.AllFiles:
							fmt.Fprintf(os.Stderr, "verbose: Oracle filter: %d/%d files (AllFiles short-circuit — no reduction)\n",
								summary.MarkedFiles, summary.TotalFiles)
						case summary.MarkedFiles == summary.TotalFiles:
							fmt.Fprintf(os.Stderr, "verbose: Oracle filter: %d/%d files (no reduction)\n",
								summary.MarkedFiles, summary.TotalFiles)
						default:
							fmt.Fprintf(os.Stderr, "verbose: Oracle filter: %d/%d files (%.1f%% of corpus) fingerprint=%s\n",
								summary.MarkedFiles, summary.TotalFiles,
								100*float64(summary.MarkedFiles)/float64(maxIntLocal(summary.TotalFiles, 1)),
								summary.Fingerprint)
						}
					}
					if summary.Fingerprint != "" {
						perf.AddEntry(jvmTracker, "filterFingerprint/"+summary.Fingerprint, 0)
					}
					// Skip the filter entirely if it doesn't reduce the
					// file set (no benefit, just overhead of writing the
					// temp file and a krit-types flag).
					if !summary.AllFiles && summary.MarkedFiles < summary.TotalFiles {
						var fp string
						werr := jvmTracker.Track("oracleFilterWriteList", func() error {
							var err error
							fp, err = oracle.WriteFilterListFile(summary, "")
							return err
						})
						if werr != nil {
							fmt.Fprintf(os.Stderr, "warning: oracle filter list: %v\n", werr)
						} else if fp != "" {
							filterListPath = fp
							defer os.Remove(fp)
						}
					}
				}

				// Route to cache-aware path unless the user opted out.
				// Both paths accept filterListPath so rule filtering and
				// per-file caching compose: the filter narrows the
				// universe first, then the cache classifies what's left.
				var res string
				var err error
				if in.NoCacheOracle {
					res, err = oracle.InvokeWithFilesWithOptions(jarPath, sourceDirs, cacheDest, filterListPath, in.Verbose, oracle.InvocationOptions{Tracker: jvmTracker})
				} else {
					var repoDir string
					jvmTracker.Track("findRepoDir", func() error {
						repoDir = oracle.FindRepoDir(scanPaths)
						return nil
					})
					res, err = oracle.InvokeCachedWithOptions(jarPath, sourceDirs, repoDir, cacheDest, filterListPath, in.Verbose, in.Store, oracle.InvocationOptions{Tracker: jvmTracker, CacheWriter: in.OracleCacheWriter})
				}
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: krit-types: %v\n", err)
					return nil
				}
				oraclePath = res
				return nil
			}()
			jvmTracker.End()
		}

		if oraclePath != "" {
			var oracleData *oracle.Oracle
			oracleTracker.Track("jsonLoad", func() error {
				od, err := oracle.Load(oraclePath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: type oracle: %v\n", err)
					return err
				}
				oracleData = od
				return nil
			})
			if oracleData != nil {
				resolver = oracle.NewCompositeResolver(oracleData, resolver)
				result.Oracle = oracleData
				if in.Verbose {
					depCount := len(oracleData.Dependencies())
					fmt.Fprintf(os.Stderr, "verbose: Type oracle loaded from %s (%d dependency types)\n", oraclePath, depCount)
				}
			}
		}
	}

	oracleTracker.End()
	return resolver
}

// oracleTracker returns a child tracker scoped to "typeOracle" when
// in.Tracker is non-nil, otherwise a noop tracker. The separate helper
// keeps the nil-check off the critical path of runOracle.
func (IndexPhase) oracleTracker(in IndexInput) perf.Tracker {
	if in.Tracker == nil {
		return perf.New(false)
	}
	return in.Tracker.Serial("typeOracle")
}

// loadFilesForOracleFilter reads the raw bytes of each .kt path into a
// lightweight *scanner.File so oracle.CollectOracleFiles can run its
// substring-based filter before krit-types is invoked. The returned
// files are NOT fully parsed (no FlatTree) — the filter is byte-only.
// Files that fail to read are dropped silently; they will surface later
// as parse errors in the real parse loop. Ported verbatim from
// cmd/krit/main.go.
func loadFilesForOracleFilter(paths []string) []*scanner.File {
	out := make([]*scanner.File, 0, len(paths))
	for _, p := range paths {
		content, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		out = append(out, &scanner.File{Path: p, Content: content})
	}
	return out
}

// runCodeIndexBuild is a verbatim port of the pre-refactor
// hasIndexBackedCrossFileRule branch in cmd/krit/main.go. It collects
// Java files, parses them in parallel, and invokes
// scanner.BuildIndexWithTracker to produce the cross-file code index.
// Tracker labels ("javaIndexing", "codeIndexBuild", inner "indexBuild")
// and verbose stderr lines match the pre-refactor output byte-for-byte.
// The caller supplies the "crossFileAnalysis" parent tracker so rule
// execution siblings can continue to nest under it.
func (p IndexPhase) runCodeIndexBuild(in IndexInput, result *IndexResult) {
	if in.CrossFileParentTracker == nil {
		return
	}
	crossTracker := in.CrossFileParentTracker
	parsedFiles := in.KotlinFiles
	paths := in.Paths

	crossWorkers := in.CrossFileWorkers
	if crossWorkers <= 0 {
		crossWorkers = phaseWorkerCount("crossFileAnalysis", in.CrossFileJobsFlag, len(parsedFiles))
	}

	var javaFilePaths []string
	var parsedJavaFiles []*scanner.File

	_ = crossTracker.Track("javaIndexing", func() error {
		var err error
		javaFilePaths, err = scanner.CollectJavaFiles(paths, nil) // err non-fatal: Java indexing is best-effort
		if err != nil && in.Verbose {
			fmt.Fprintf(os.Stderr, "verbose: Java file collection: %v\n", err)
		}
		if len(javaFilePaths) > 0 {
			crossWorkers = phaseWorkerCount("crossFileAnalysis", in.CrossFileJobsFlag, len(parsedFiles)+len(javaFilePaths))
			var javaErrs []error
			parsedJavaFiles, javaErrs = scanner.ScanJavaFilesCached(javaFilePaths, crossWorkers, in.ParseCache)
			if len(javaErrs) > 0 && in.Verbose {
				fmt.Fprintf(os.Stderr, "verbose: Java file parsing: %d errors\n", len(javaErrs))
			}
			if in.Verbose {
				fmt.Fprintf(os.Stderr, "verbose: Parsed %d Java files for cross-reference indexing\n", len(parsedJavaFiles))
			}
		}
		return nil
	})

	var codeIndex *scanner.CodeIndex
	_ = crossTracker.Track("codeIndexBuild", func() error {
		indexTracker := crossTracker.Serial("indexBuild")
		if in.CrossFileCacheDir != "" {
			var hit bool
			codeIndex, hit = scanner.BuildIndexCached(in.CrossFileCacheDir, parsedFiles, crossWorkers, indexTracker, parsedJavaFiles...)
			if in.Verbose {
				if hit {
					fmt.Fprintf(os.Stderr, "verbose: Cross-file index cache: HIT\n")
				} else {
					fmt.Fprintf(os.Stderr, "verbose: Cross-file index cache: MISS (rebuilt + persisted)\n")
				}
			}
		} else {
			codeIndex = scanner.BuildIndexWithTracker(parsedFiles, crossWorkers, indexTracker, parsedJavaFiles...)
		}
		indexTracker.End()
		return nil
	})

	result.CodeIndex = codeIndex
	result.JavaFiles = parsedJavaFiles
}

// runModuleIndexBuild is a verbatim port of the pre-refactor
// moduleAwareAnalysis block's indexing phase in cmd/krit/main.go. It
// discovers Gradle modules, optionally parses their dependencies, and
// builds the PerModuleIndex. The caller supplies the
// "moduleAwareAnalysis" parent tracker and End()-s it after rule
// execution siblings have run. Rule execution itself stays in main.go.
func (p IndexPhase) runModuleIndexBuild(in IndexInput, result *IndexResult) {
	if in.ModuleParentTracker == nil {
		return
	}
	moduleTracker := in.ModuleParentTracker

	scanRoot := in.ModuleScanRoot
	if scanRoot == "" {
		scanRoot = "."
		if len(in.Paths) > 0 {
			scanRoot = in.Paths[0]
		}
	}

	var (
		graph  *module.ModuleGraph
		modErr error
	)
	_ = moduleTracker.Track("moduleDiscovery", func() error {
		graph, modErr = module.DiscoverModules(scanRoot)
		return nil
	})
	if modErr != nil && in.Verbose {
		fmt.Fprintf(os.Stderr, "verbose: Module discovery error: %v\n", modErr)
	}
	result.ModuleGraph = graph

	if graph != nil && len(graph.Modules) > 0 && in.ModuleHasAwareRule {
		moduleNeeds := rules.CollectModuleAwareNeedsV2(in.ActiveRules)
		moduleWorkers := phaseWorkerCount("moduleAwareAnalysis", in.ModuleJobsFlag, len(graph.Modules))

		var pmi *module.PerModuleIndex
		if moduleNeeds.NeedsDependencies {
			_ = moduleTracker.Track("moduleDependencies", func() error {
				if err := module.ParseAllDependencies(graph); err != nil {
					if in.Verbose {
						fmt.Fprintf(os.Stderr, "verbose: Module dependency parse error: %v\n", err)
					}
				}
				return nil
			})
		}
		if in.Verbose {
			fmt.Fprintf(os.Stderr, "verbose: Detected %d Gradle modules\n", len(graph.Modules))
		}

		_ = moduleTracker.Track("moduleIndexBuild", func() error {
			pmi = &module.PerModuleIndex{Graph: graph}
			switch {
			case moduleNeeds.NeedsIndex:
				pmi = module.BuildPerModuleIndexWithGlobal(graph, in.KotlinFiles, moduleWorkers, result.CodeIndex)
			case moduleNeeds.NeedsFiles:
				pmi.ModuleFiles = module.GroupFilesByModule(graph, in.KotlinFiles)
			}
			return nil
		})

		result.ModuleIndex = pmi
	}
}

// phaseWorkerCount is a pipeline-local copy of the CLI's worker-count
// helper. Kept in sync with cmd/krit/main.go phaseWorkerCount so tracker
// labels using the same phase name get the same worker count.
func phaseWorkerCount(phase string, maxWorkers, workItems int) int {
	if maxWorkers < 1 {
		maxWorkers = 1
	}
	if workItems < 1 {
		return 1
	}
	workers := maxWorkers
	if workItems < workers {
		workers = workItems
	}
	var phaseCap int
	switch phase {
	case "moduleAwareAnalysis":
		phaseCap = 8
	case "ruleExecution", "parse", "typeIndex", "crossFileAnalysis":
		phaseCap = 16
	default:
		phaseCap = workers
	}
	if phaseCap < 1 {
		phaseCap = 1
	}
	if workers > phaseCap {
		workers = phaseCap
	}
	return workers
}

// maxIntLocal avoids a division-by-zero in the oracle filter's verbose
// reporting when the file set is empty. Named maxIntLocal so it doesn't
// collide with the identical helper in cmd/krit/main.go during
// incremental migration.
func maxIntLocal(a, b int) int {
	if a > b {
		return a
	}
	return b
}
