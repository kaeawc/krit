package serve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
		} else {
			in.Host.SourceSetDirty = dirty
		}
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
		state:       state,
		in:          in,
		start:       start,
		cold:        cold,
		dirtyN:      len(dirty),
		releaseLock: state.analyzeMu.Unlock,
	}, nil
}

type streamingAnalyzeResponse struct {
	state       *daemonState
	in          pipeline.ProjectInput
	start       time.Time
	cold        bool
	dirtyN      int
	releaseLock func()
}

var _ daemon.RawResponseWriter = (*streamingAnalyzeResponse)(nil)

func (r *streamingAnalyzeResponse) WriteRawResponse(ctx context.Context, w io.Writer) error {
	defer r.releaseLock()

	hw := &analyzeRespWriter{out: w, head: []byte(`{"ok":true,"data":{"findings":`)}
	res, err := pipeline.RunProjectStreaming(ctx, r.in, hw)
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

	statsBytes, err := json.Marshal(daemon.AnalyzeProjectStats{
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
	})
	if err != nil {
		return fmt.Errorf("marshal stats: %w", err)
	}
	if _, err := w.Write([]byte(`,"stats":`)); err != nil {
		return err
	}
	if _, err := w.Write(statsBytes); err != nil {
		return err
	}
	_, err = w.Write(envelopeTail)
	return err
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

	// Prepopulate the source-path slices from the prior manifest when
	// available so runProjectParsePhase doesn't walk the filesystem
	// again on every analyze. The pipeline already trusts these as
	// canonical when args.KotlinPaths is non-nil. fsnotify/manifest
	// drift is caught at the bundle-fingerprint comparison, which
	// runs whether or not we walk; bypassing the walk just saves the
	// 30-40s cold-OS-dentry-cache cost on kotlin-corpus scale.
	kotlinPaths, javaPaths := s.prepopulatedSourcePaths(repoDir, paths)

	return pipeline.ProjectInput{
		Args: pipeline.ProjectArgs{
			Config:           cfg,
			Paths:            paths,
			KotlinPaths:      kotlinPaths,
			JavaPaths:        javaPaths,
			ActiveRules:      activeRules,
			Format:           args.Format,
			BaselinePath:     args.BaselinePath,
			DiffRef:          args.DiffRef,
			MinConfidence:    args.MinConfidence,
			WarningsAsErrors: args.WarningsAsErrors,
			IncludeGenerated: args.IncludeGenerated,
			Version:          kritVersion(),
			OracleEnabled:    oracleDaemon != nil,
			// Wire is line-delimited; compact JSON keeps the body
			// free of internal newlines.
			JSONCompact: true,
		},
		Host: pipeline.ProjectHostState{
			ParseCache:                   parseCache,
			LibraryFactsCache:            s.workspace.LibraryFacts,
			CodeIndexCache:               s.workspace.CodeIndex,
			ResolverCache:                s.workspace.Resolver,
			OracleFilterCache:            s.workspace.OracleFilter,
			CrossFileCacheDir:            scanner.CrossFileCacheDir(repoDir),
			CrossFindingsCacheDir:        scanner.CrossFindingsCacheDir(repoDir),
			TypeIndexCacheDir:            typeinfer.TypeIndexCacheDir(repoDir),
			AnalysisCache:                analysisCache,
			AnalysisCacheFilePath:        analysisCachePath,
			AnalysisCacheLookup:          analysisCache != nil,
			OracleDaemon:                 oracleDaemon,
			FindingsBundleStore:          scanner.DiskFindingsBundleStore{},
			FindingsBundleCacheRoot:      repoDir,
			FindingsBundleManifestLoader: s.loadManifest,
			FindingsBundleManifestSaver:  s.saveManifest,
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
