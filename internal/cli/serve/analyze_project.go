package serve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/pipeline"
	"github.com/kaeawc/krit/internal/rules"
)

// errDaemonNotWarm is returned to RequireWarm clients on a cold
// invocation. Sentinel so clients can errors.Is against it without
// string matching.
var errDaemonNotWarm = errors.New("daemon not warm yet")

// handleAnalyzeProject runs the whole-project scan pipeline against
// the daemon's resident state and returns the formatted findings.
//
// Concurrency: serialised on daemonState.analyzeMu. The pipeline
// mutates resolver / oracle state and the analysis-cache write-back
// path isn't safe under concurrent runs; queueing is the documented
// behaviour. Wire-protocol clients that need to issue many calls in
// a burst should batch them on the client side rather than attempt
// to parallelise here.
func handleAnalyzeProject(ctx context.Context, state *daemonState, raw json.RawMessage) (any, error) {
	var args daemon.AnalyzeProjectArgs
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, fmt.Errorf("decode args: %w", err)
		}
	}

	state.analyzeMu.Lock()
	defer state.analyzeMu.Unlock()

	cold := !state.coldDone.Load()
	if args.RequireWarm && cold {
		return nil, errDaemonNotWarm
	}

	start := time.Now()
	dirty := state.workspace.DrainDirty()

	in, err := state.buildProjectInput(args)
	if err != nil {
		return nil, err
	}

	out, err := pipeline.RunProject(ctx, in)
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
		},
	}, nil
}

// buildProjectInput translates wire args into a pipeline.ProjectInput
// against daemon-resident state. The function is the single seam
// where CLI-flag-style knobs (rule lists, format) are wired into the
// pipeline's typed value inputs.
func (s *daemonState) buildProjectInput(args daemon.AnalyzeProjectArgs) (pipeline.ProjectInput, error) {
	cfg, err := loadDaemonConfig(s.root)
	if err != nil {
		return pipeline.ProjectInput{}, fmt.Errorf("load config: %w", err)
	}
	rules.ApplyConfig(cfg)

	disabledSet := parseRuleNameSetCSV(args.DisableRules)
	enabledSet := parseRuleNameSetCSV(args.EnableRules)
	experimental := args.Experimental || cfg.GetTopLevelBool("experimental", false)
	activeRules := rules.ActiveRulesV2(disabledSet, enabledSet, args.AllRules, experimental)

	paths := args.Paths
	if len(paths) == 0 {
		paths = []string{s.root}
	}

	repoDir := oracle.FindRepoDir(paths)
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

	return pipeline.ProjectInput{
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
		ParseCache:       parseCache,
		// Resolver, Oracle, AnalysisCache, PrebuiltLibraryFacts will
		// be wired in as the daemon promotes them to resident state
		// in follow-up commits. RunProject treats nil as "construct
		// per-call as the CLI runner does today", so the verb is
		// already correct — just slower than its eventual ceiling.
	}, nil
}

// loadDaemonConfig loads the krit.yml the daemon should honour for
// rule selection. Mirrors the CLI's two-source precedence (user
// config in repo + default config), but tolerates a missing config
// silently — the daemon should be useful out of the box without
// requiring krit.yml.
func loadDaemonConfig(root string) (*config.Config, error) {
	defaultCfgPath := config.FindDefaultConfig()
	userCfgPath := findRepoConfig(root)
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

// findRepoConfig probes root for a krit.yml or .krit.yml file and
// returns its path, or "" when neither is present. Mirrors the CLI's
// detectConfigForScanArgs helper, which lives in internal/cli/scan
// and isn't importable from here.
func findRepoConfig(root string) string {
	if root == "" {
		return ""
	}
	for _, name := range []string{"krit.yml", ".krit.yml"} {
		candidate := filepath.Join(root, name)
		if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
			return candidate
		}
	}
	return ""
}

// parseRuleNameSetCSV is the daemon-side equivalent of the CLI's
// parseRuleNameSet helper (internal/cli/scan/rule_name_set.go).
// The two should be unified in `internal/cli/clishared` in a
// follow-up; today the CLI version is unexported and exporting it
// would touch the scan package's blast radius. Tracked separately.
func parseRuleNameSetCSV(csv string) map[string]bool {
	out := make(map[string]bool)
	if csv == "" {
		return out
	}
	for _, name := range strings.Split(csv, ",") {
		out[strings.TrimSpace(name)] = true
	}
	return out
}
