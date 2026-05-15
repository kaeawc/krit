package sessdaemon

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/pipeline"
	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

// handleAnalyze runs pipeline.RunProjectAnalysis under the daemon's
// resident scan.Session and streams the resulting findings as NDJSON:
//
//	{"kind":"finding","finding":{...}}\n   (0 or more)
//	{"kind":"summary","summary":{...}}\n   (terminal)
//
// EOF on the connection is the client's end-of-stream signal.
//
// All analyze calls are serialised through s.mu. v1 cannot run
// concurrent analyses because the parse-cache writers and oracle
// daemon are not yet contention-safe (see issue #201). RunProject's
// OutputPhase is skipped — the daemon emits per-row directly off the
// dispatcher's columnar findings.
func (s *Server) handleAnalyze(ctx context.Context, w *bufio.Writer, req Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var params AnalyzeParams
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			_ = writeError(w, req.ID, ErrCodeInvalidRequest, "decode params: "+err.Error())
			return
		}
	}
	if len(params.Paths) == 0 {
		params.Paths = []string{s.repoDir}
	}

	in, err := s.buildProjectInput(params.Paths)
	if err != nil {
		_ = writeError(w, req.ID, ErrCodeInternal, err.Error())
		return
	}

	start := time.Now()
	res, err := pipeline.RunProjectAnalysis(ctx, in)
	if err != nil {
		_ = writeError(w, req.ID, ErrCodeInternal, "run: "+err.Error())
		return
	}

	if s.strictVerify {
		if logPath, divErr := s.runStrictVerify(ctx, params.Paths, &res.CrossFileResult.Findings); divErr != nil {
			msg := "strict-verify: " + divErr.Error()
			if logPath != "" {
				msg += " (log: " + logPath + ")"
			}
			_ = writeError(w, req.ID, ErrCodeInternal, msg)
			return
		}
	}

	enc := json.NewEncoder(w)
	if err := emitFindings(enc, &res.CrossFileResult.Findings); err != nil {
		return
	}
	_ = enc.Encode(AnalyzeStreamSummary{
		Kind: "summary",
		Summary: AnalyzeSummary{
			FilesScanned:      res.FilesScanned,
			FindingsCount:     res.CrossFileResult.Findings.Len(),
			ParseHits:         res.ParseHits,
			ParseMisses:       res.ParseMisses,
			FindingsBundleHit: res.FindingsBundleHit,
			DurationMs:        time.Since(start).Milliseconds(),
		},
	})
	_ = w.Flush()
}

// buildProjectInput threads the Session's resident state into a
// pipeline.ProjectInput. v1 uses default config + the active rule set;
// future commits will plumb AnalyzeParams.Flags through.
func (s *Server) buildProjectInput(paths []string) (pipeline.ProjectInput, error) {
	cfg := config.NewConfig()
	rules.ApplyConfig(cfg)
	activeRules := rules.ActiveRulesV2(nil, nil, false, false, false)

	host := pipeline.ProjectHostState{}
	if sess := s.session; sess != nil {
		host.ParseCache = sess.ParseCache
		host.AnalysisCache = sess.AnalysisCache
		host.AnalysisCacheFilePath = sess.AnalysisCacheFilePath
		host.PrebuiltLibraryFacts = sess.LibraryFacts
		// The daemon's file watcher feeds WorkspaceState.Touch, so
		// CheckFilesIncremental can trust DrainDirty as the
		// "changed since last analyze" set. See issue #206.
		if sess.AnalysisCache != nil && sess.AnalysisCacheFilePath != "" {
			host.AnalysisCacheLookup = true
			dirty := sess.Workspace.DrainDirty()
			if dirty == nil {
				dirty = []string{}
			}
			host.AnalysisCacheDirty = dirty
		}
	}
	// Lazy-start (or recover) the resident krit-types JVM. ensureOracle
	// returns nil when the JAR is missing or after a crash-retry has
	// exhausted its budget — analyze proceeds without oracle in that
	// case so the client always gets a response.
	oracleDaemon := s.ensureOracle(paths)
	host.OracleDaemon = oracleDaemon
	// scan.NewSession leaves ParseCache nil — lazily attach one and
	// stash it on the session so subsequent analyze calls are warm.
	// Safe because analyze is serialised through s.mu.
	if host.ParseCache == nil {
		pc, err := scanner.NewParseCacheWithCap(s.repoDir, cacheutil.DefaultParseCacheCapBytes)
		if err != nil {
			return pipeline.ProjectInput{}, fmt.Errorf("parse cache: %w", err)
		}
		host.ParseCache = pc
		if s.session != nil {
			s.session.ParseCache = pc
		}
	}

	return pipeline.ProjectInput{
		Args: pipeline.ProjectArgs{
			Config:        cfg,
			Paths:         paths,
			ActiveRules:   activeRules,
			Format:        "json",
			Version:       "daemon",
			OracleEnabled: oracleDaemon != nil,
		},
		Host: host,
	}, nil
}

// runStrictVerify runs a fresh in-process baseline against the same
// paths the daemon just analyzed and compares the two finding sets via
// daemon.Compare. On divergence it persists a structured log under
// `${repoDir}/.krit/daemon-divergence-NNNN.log` and returns a non-nil
// error so the caller fails the analyze response. The baseline runs
// with an empty ProjectHostState so no resident cache contaminates the
// comparison.
func (s *Server) runStrictVerify(ctx context.Context, paths []string, daemonCols *scanner.FindingColumns) (string, error) {
	cfg := config.NewConfig()
	rules.ApplyConfig(cfg)
	activeRules := rules.ActiveRulesV2(nil, nil, false, false, false)

	pc, err := scanner.NewParseCacheWithCap(s.repoDir, cacheutil.DefaultParseCacheCapBytes)
	if err != nil {
		return "", fmt.Errorf("baseline parse cache: %w", err)
	}
	defer pc.Close() //nolint:errcheck // best effort

	baseline, err := pipeline.RunProjectAnalysis(ctx, pipeline.ProjectInput{
		Args: pipeline.ProjectArgs{
			Config:      cfg,
			Paths:       paths,
			ActiveRules: activeRules,
			Format:      "json",
			Version:     "daemon",
		},
		Host: pipeline.ProjectHostState{ParseCache: pc},
	})
	if err != nil {
		return "", fmt.Errorf("baseline analyze: %w", err)
	}

	diff := daemon.Compare(daemonCols, &baseline.CrossFileResult.Findings)
	if diff.IsClean() {
		return "", nil
	}
	logPath, pathErr := daemon.NextDivergenceLogPath(s.repoDir)
	if pathErr != nil {
		return "", fmt.Errorf("divergence detected; failed to allocate log path: %w", pathErr)
	}
	if writeErr := diff.WriteLog(logPath); writeErr != nil {
		return logPath, fmt.Errorf("divergence detected; failed to write log: %w", writeErr)
	}
	return logPath, fmt.Errorf("divergence: %d added, %d dropped across %d files",
		len(diff.AddedByDaemon), len(diff.DroppedByDaemon), len(diff.PathsTouched))
}

func emitFindings(enc *json.Encoder, cols *scanner.FindingColumns) error {
	if cols == nil {
		return nil
	}
	for _, f := range cols.Findings() {
		row := AnalyzeStreamFinding{Kind: "finding", Finding: wireFinding(f)}
		if err := enc.Encode(row); err != nil {
			return err
		}
	}
	return nil
}

// wireFinding flattens scanner.Finding into the daemon's wire shape.
// Fix / BinaryFix are intentionally dropped — v1 callers don't apply
// fixes through the daemon yet.
func wireFinding(f scanner.Finding) Finding {
	return Finding{
		File:       f.File,
		Line:       f.Line,
		Col:        f.Col,
		StartByte:  f.StartByte,
		EndByte:    f.EndByte,
		RuleSet:    f.RuleSet,
		Rule:       f.Rule,
		Severity:   f.Severity,
		Message:    f.Message,
		Confidence: f.Confidence,
	}
}
