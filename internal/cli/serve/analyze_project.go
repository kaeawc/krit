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

	// Defer pipeline execution into the streaming response writer so
	// the OutputPhase JSON streams directly into the connection
	// instead of being buffered in memory (#60). The lock is held for
	// the duration of the run and released by WriteRawResponse.
	return &streamingAnalyzeResponse{
		ctx:     ctx,
		state:   state,
		in:      in,
		start:   start,
		cold:    cold,
		dirtyN:  len(dirty),
		release: state.analyzeMu.Unlock,
	}, nil
}

// streamingAnalyzeResponse implements daemon.RawResponseWriter. It runs
// pipeline.RunProjectStreaming with the connection writer bracketed by
// the daemon Response envelope head/tail so the multi-megabyte findings
// JSON never has to be staged in a heap buffer.
//
// Lock discipline: handleAnalyzeProject acquires state.analyzeMu and
// hands ownership to this carrier; release runs in WriteRawResponse so
// the lock spans the whole pipeline run, matching the prior behavior.
type streamingAnalyzeResponse struct {
	ctx     context.Context
	state   *daemonState
	in      pipeline.ProjectInput
	start   time.Time
	cold    bool
	dirtyN  int
	release func()
}

// Compile-time assertion: streamingAnalyzeResponse must satisfy the
// daemon's RawResponseWriter interface so dispatch routes it through
// the streaming path instead of the default json.Marshal envelope.
var _ daemon.RawResponseWriter = (*streamingAnalyzeResponse)(nil)

// WriteRawResponse writes a complete daemon Response line into w. On
// pipeline error it falls back to writing a normal {"ok":false,...}
// envelope; on write error after the head has flushed the connection
// is left in a partially written state and the caller closes it.
func (r *streamingAnalyzeResponse) WriteRawResponse(w io.Writer) error {
	defer r.release()

	// Head-flush is reversible: nothing has been written yet, so a
	// pipeline error here translates cleanly to an error envelope.
	// Run the pipeline into an envelope-aware writer that emits the
	// success head on the first byte and strips the OutputPhase's
	// trailing newline (the wire response is line-delimited so the
	// final \n must come from us, not from json.Encoder).
	hw := &analyzeRespWriter{out: w, head: []byte(`{"ok":true,"data":{"findings":`)}
	res, err := pipeline.RunProjectStreaming(r.ctx, r.in, hw)
	if err != nil {
		if !hw.headWritten {
			return writeJSONLine(w, errorResponseLine(err.Error()))
		}
		// Head already flushed; can't roll back. Surface the error
		// to the connection writer; the client will see a truncated
		// response and surface it as a decode error.
		return err
	}
	r.state.coldDone.Store(true)
	xfile := r.state.workspace.CrossFileStats()

	stats := daemon.AnalyzeProjectStats{
		FilesScanned:    res.FilesScanned,
		FindingsCount:   res.FindingsCount,
		WallSeconds:     time.Since(r.start).Seconds(),
		CodeIndexHit:    xfile.HasCodeIndex,
		LibraryFactsHit: xfile.HasLibraryFacts,
		ResolverHit:     xfile.HasResolver,
		OracleFilterHit: xfile.HasOracleFilter,
		DirtyFiles:      r.dirtyN,
		Cold:            r.cold,
		ParseHits:       res.ParseHits,
		ParseMisses:     res.ParseMisses,
	}
	statsBytes, err := json.Marshal(stats)
	if err != nil {
		// Stats marshaling cannot realistically fail; if it does the
		// envelope is already partially written, so close.
		return fmt.Errorf("marshal stats: %w", err)
	}
	if _, err := w.Write([]byte(`,"stats":`)); err != nil {
		return err
	}
	if _, err := w.Write(statsBytes); err != nil {
		return err
	}
	_, err = w.Write([]byte("}}\n"))
	return err
}

// analyzeRespWriter is the streaming envelope writer for the
// analyze-project verb. It serves two purposes:
//
//  1. Defer the success-envelope head until the first byte of
//     OutputPhase output is observed, so a pre-output error in
//     RunProjectStreaming can still produce a clean error envelope
//     before any wire bytes commit.
//  2. Strip a trailing '\n' from the streamed bytes. json.Encoder
//     terminates each value with a newline, but the daemon wire
//     protocol is line-delimited; an embedded newline mid-response
//     would let the client's bufio.Reader.ReadBytes('\n') return a
//     truncated payload. The stripped newline is reintroduced by
//     the tail writer (the closing "}}\n" of the envelope).
//
// The held newline is implemented as a one-byte look-behind: when a
// chunk ends in '\n' the byte is held back; the next Write flushes
// it before processing new bytes. Internal newlines (mid-chunk, or
// when a non-trailing chunk ends in '\n') are forwarded normally.
// The very last held newline is silently dropped because no further
// pipeline write follows.
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
	n := len(p)
	if w.heldNewline {
		if _, err := w.out.Write([]byte{'\n'}); err != nil {
			return 0, err
		}
		w.heldNewline = false
	}
	if p[len(p)-1] == '\n' {
		if len(p) > 1 {
			if _, err := w.out.Write(p[:len(p)-1]); err != nil {
				return 0, err
			}
		}
		w.heldNewline = true
		return n, nil
	}
	if _, err := w.out.Write(p); err != nil {
		return 0, err
	}
	return n, nil
}

// writeJSONLine marshals v and writes it as a single newline-terminated
// JSON object — the wire format the daemon's writeResponse uses.
// Mirrored here so the streaming carrier can emit error envelopes
// without exposing internal helpers from package daemon.
func writeJSONLine(w io.Writer, v any) error {
	buf, err := json.Marshal(v)
	if err != nil {
		return err
	}
	buf = append(buf, '\n')
	_, err = w.Write(buf)
	return err
}

func errorResponseLine(msg string) daemon.Response {
	return daemon.Response{OK: false, Error: msg}
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
			// Daemon wire protocol is line-delimited JSON; the
			// streaming response writer (#60) panics on the first
			// internal newline. Compact JSON keeps the payload
			// single-line so ReadBytes('\n') gets the full body.
			JSONCompact: true,
		},
		Host: pipeline.ProjectHostState{
			ParseCache:              parseCache,
			LibraryFactsCache:       s.workspace,
			CodeIndexCache:          s.workspace,
			ResolverCache:           s.workspace,
			OracleFilterCache:       s.workspace,
			CrossFileCacheDir:       scanner.CrossFileCacheDir(repoDir),
			TypeIndexCacheDir:       typeinfer.TypeIndexCacheDir(repoDir),
			AnalysisCache:           analysisCache,
			AnalysisCacheFilePath:   analysisCachePath,
			AnalysisCacheLookup:     analysisCache != nil,
			OracleDaemon:            oracleDaemon,
			FindingsBundleStore:     scanner.DiskFindingsBundleStore{},
			FindingsBundleCacheRoot: repoDir,
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
