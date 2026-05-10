package serve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/cli/clishared"
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/daemon"
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
func handleAnalyzeProject(ctx context.Context, state *daemonState, raw json.RawMessage) (any, error) {
	var args daemon.AnalyzeProjectArgs
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, fmt.Errorf("decode args: %w", err)
		}
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

	in, err := state.buildProjectInput(args)
	if err != nil {
		state.analyzeMu.Unlock()
		return nil, err
	}

	out, err := pipeline.RunProject(ctx, in)
	state.analyzeMu.Unlock()
	if err != nil {
		return nil, err
	}
	state.coldDone.Store(true)
	xfile := state.workspace.CrossFileStats()

	return daemon.AnalyzeProjectResult{
		Findings: out.JSON,
		Stats: daemon.AnalyzeProjectStats{
			FilesScanned:    out.FilesScanned,
			FindingsCount:   out.FindingsCount,
			WallSeconds:     time.Since(start).Seconds(),
			CodeIndexHit:    xfile.HasCodeIndex,
			LibraryFactsHit: xfile.HasLibraryFacts,
			DirtyFiles:      len(dirty),
			Cold:            cold,
			ParseHits:       out.ParseHits,
			ParseMisses:     out.ParseMisses,
		},
	}, nil
}

// buildProjectInput translates wire args into a pipeline.ProjectInput
// against daemon-resident state. The function is the single seam
// where CLI-flag-style knobs (rule lists, format) are wired into the
// pipeline's typed value inputs.
func (s *daemonState) buildProjectInput(args daemon.AnalyzeProjectArgs) (pipeline.ProjectInput, error) {
	cfg, err := s.ensureConfig()
	if err != nil {
		return pipeline.ProjectInput{}, fmt.Errorf("load config: %w", err)
	}
	rules.ApplyConfig(cfg)

	disabledSet := clishared.ParseRuleNameSetCSV(args.DisableRules)
	enabledSet := clishared.ParseRuleNameSetCSV(args.EnableRules)
	experimental := args.Experimental || cfg.GetTopLevelBool("experimental", false)
	activeRules := rules.ActiveRulesV2(disabledSet, enabledSet, args.AllRules, experimental)

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
	parseCache, err := s.parseCacheFor(repoDir, cacheutil.DefaultParseCacheCapBytes)
	if err != nil {
		// A failed parse cache isn't fatal — RunProject tolerates
		// nil and falls back to direct tree-sitter parses. Log via
		// the reporter once available; for now silently degrade so
		// the verb stays useful even when the cache directory is
		// read-only.
		parseCache = nil
	}

	// AnalysisCache is daemon-resident: lazy-loaded on first request,
	// keyed by the resolved cache file path so distinct scan-path sets
	// don't share an entry. DispatchPhase will merge new findings into
	// it and persist to disk on each call.
	analysisCache, analysisCachePath := s.analysisCacheFor(paths)

	// OracleDaemon is daemon-resident: lazy-started on first request
	// when krit-types.jar is found. nil daemon means oracle stays
	// disabled (no JVM, no behavior change vs. pre-#125). Per-verb
	// ping happens at the call boundary in handleAnalyzeProject.
	oracleDaemon, err := s.ensureOracleDaemon(paths)
	if err != nil {
		// Best-effort degrade: a failed daemon start shouldn't fail
		// the whole verb. Log via stderr; the verb continues with
		// oracle disabled.
		fmt.Fprintf(os.Stderr, "warning: oracle daemon start: %v\n", err)
		oracleDaemon = nil
	}

	return pipeline.ProjectInput{
		Args: pipeline.ProjectArgs{
			Config:           cfg,
			Paths:            paths,
			ActiveRules:      activeRules,
			Format:           args.Format,
			BaselinePath:     args.BaselinePath,
			DiffRef:          args.DiffRef,
			MinConfidence:    args.MinConfidence,
			WarningsAsErrors: args.WarningsAsErrors,
			IncludeGenerated: args.IncludeGenerated,
			Version:          kritVersion(),
			OracleEnabled:    oracleDaemon != nil,
		},
		Host: pipeline.ProjectHostState{
			ParseCache:            parseCache,
			LibraryFactsCache:     s.workspace,
			CodeIndexCache:        s.workspace,
			ResolverCache:         s.workspace,
			OracleFilterCache:     s.workspace,
			CrossFileCacheDir:     scanner.CrossFileCacheDir(repoDir),
			TypeIndexCacheDir:     typeinfer.TypeIndexCacheDir(repoDir),
			AnalysisCache:         analysisCache,
			AnalysisCacheFilePath: analysisCachePath,
			OracleDaemon:          oracleDaemon,
		},
	}, nil
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
