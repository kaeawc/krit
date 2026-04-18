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
	// ActiveRulesV1 is the v1 rule slice fed to
	// rules.BuildOracleFilterRules. Pipeline already tracks v2 rules, but
	// the oracle filter bridge works in v1 terms.
	ActiveRulesV1 []rules.Rule
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
	if in.OracleEnabled && resolverForOracle != nil {
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

	// Module graph
	if !p.SkipModules {
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

	// Incremental analysis cache load + per-file lookup. Write-back
	// (post-dispatch merge of new findings) still lives in the CLI. This
	// mirrors the pre-refactor block in cmd/krit/main.go verbatim so
	// tracker labels ("cacheLoad") and verbose stderr lines stay
	// byte-identical.
	if in.CacheEnabled {
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
		ruleNames = make([]string, len(in.ActiveRulesV1))
		for i, r := range in.ActiveRulesV1 {
			ruleNames[i] = r.Name()
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
			oracleTracker.Track("jvmAnalyze", func() error {
				jarPath := oracle.FindJar(scanPaths)
				if jarPath == "" {
					return nil
				}
				sourceDirs := oracle.FindSourceDirs(scanPaths)
				if len(sourceDirs) == 0 {
					return nil
				}
				cacheDest := oracle.CachePath(scanPaths)
				if cacheDest == "" {
					cacheDest = filepath.Join(os.TempDir(), "krit-types.json")
				}
				if in.Verbose {
					fmt.Fprintf(os.Stderr, "verbose: Running krit-types (%d source dirs)...\n", len(sourceDirs))
				}
				// Pre-scan step: compute the subset of files any enabled
				// rule has declared (via OracleFilter) it actually needs
				// oracle access on. Files where no filter matches are
				// tree-sitter-sufficient and can be dropped from the
				// krit-types analyze loop. Unclassified rules fall
				// through to AllFiles: true, so this is a no-op until
				// a meaningful fraction of rules has been audited. Guard
				// with -no-oracle-filter for diagnostics / baseline
				// reproduction.
				var filterListPath string
				if !in.NoOracleFilter {
					filterRules := rules.BuildOracleFilterRules(in.ActiveRulesV1)
					lightFiles := loadFilesForOracleFilter(in.KotlinFilePaths)
					summary := oracle.CollectOracleFiles(filterRules, lightFiles)
					if in.Verbose {
						switch {
						case summary.AllFiles:
							fmt.Fprintf(os.Stderr, "verbose: Oracle filter: %d/%d files (AllFiles short-circuit — no reduction)\n",
								summary.MarkedFiles, summary.TotalFiles)
						case summary.MarkedFiles == summary.TotalFiles:
							fmt.Fprintf(os.Stderr, "verbose: Oracle filter: %d/%d files (no reduction)\n",
								summary.MarkedFiles, summary.TotalFiles)
						default:
							fmt.Fprintf(os.Stderr, "verbose: Oracle filter: %d/%d files (%.1f%% of corpus)\n",
								summary.MarkedFiles, summary.TotalFiles,
								100*float64(summary.MarkedFiles)/float64(maxIntLocal(summary.TotalFiles, 1)))
						}
					}
					// Skip the filter entirely if it doesn't reduce the
					// file set (no benefit, just overhead of writing the
					// temp file and a krit-types flag).
					if !summary.AllFiles && summary.MarkedFiles < summary.TotalFiles {
						fp, werr := oracle.WriteFilterListFile(summary, "")
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
					res, err = oracle.InvokeWithFiles(jarPath, sourceDirs, cacheDest, filterListPath, in.Verbose)
				} else {
					res, err = oracle.InvokeCached(jarPath, sourceDirs, oracle.FindRepoDir(scanPaths), cacheDest, filterListPath, in.Verbose, in.Store)
				}
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: krit-types: %v\n", err)
					return nil
				}
				oraclePath = res
				return nil
			})
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
