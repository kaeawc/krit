package serve

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/cli/clishared"
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/pipeline"
	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

// strictVerifyAnalyzeResponse is the analyze-project response when the
// daemon was started with --strict-verify. It buffers both the daemon
// path and an in-process baseline scan, compares finding-row diffs via
// daemon.Compare, and either emits the daemon's output verbatim or
// fails the response with a divergence log path.
//
// The intentional regression vs streamingAnalyzeResponse:
//   - Doubles wall time (daemon + baseline run sequentially).
//   - Holds findings JSON in memory until the comparison completes.
//
// Both are acceptable under strict-verify, which is designed for
// alpha-period correctness hunts, not steady-state production load.
//
// Ported from internal/sessdaemon/analyze.go's strict-verify logic
// (now retired in favour of internal/cli/serve as the canonical
// daemon).
type strictVerifyAnalyzeResponse struct {
	state       *daemonState
	args        daemon.AnalyzeProjectArgs
	in          pipeline.ProjectInput
	start       time.Time
	cold        bool
	dirtyN      int
	releaseLock func()
}

var _ daemon.RawResponseWriter = (*strictVerifyAnalyzeResponse)(nil)

func (r *strictVerifyAnalyzeResponse) WriteRawResponse(ctx context.Context, w io.Writer) error {
	defer r.releaseLock()

	// Phase 1: daemon-state analyze (RunProjectAnalysis to keep raw
	// pre-fixup findings around for the comparison; we re-format
	// through OutputPhase ourselves in phase 4).
	daemonAnalysis, err := pipeline.RunProjectAnalysis(ctx, r.in)
	if err != nil {
		return daemon.WriteErrorResponseLine(w, "strict-verify daemon analyze: "+err.Error())
	}

	// Phase 2: cold in-process baseline against the same paths +
	// rule selection. runStrictVerify is the single seam where rule
	// selection is mirrored — keep it in sync with buildProjectInput.
	logPath, divErr := r.state.runStrictVerify(ctx, r.args, &daemonAnalysis.CrossFileResult.Findings)
	if divErr != nil {
		msg := "strict-verify: " + divErr.Error()
		if logPath != "" {
			msg += " (log: " + logPath + ")"
		}
		return daemon.WriteErrorResponseLine(w, msg)
	}

	// Phase 3 + 4: clean — re-run through the full pipeline to
	// produce the actual output JSON (the baseline run in phase 2
	// already proved the resident state is correct; this pass is the
	// cheap formatting step). Using RunProjectStreaming into a
	// bytes.Buffer is the same shape RunProject uses internally;
	// going through it ensures the wire output matches the
	// non-strict-verify streaming path byte-for-byte.
	var findingsBuf bytes.Buffer
	res, err := pipeline.RunProjectStreaming(ctx, r.in, &findingsBuf)
	if err != nil {
		return daemon.WriteErrorResponseLine(w, "strict-verify daemon format: "+err.Error())
	}
	r.state.coldDone.Store(true)
	xfile := r.state.workspace.CrossFileStats()

	// Strip the trailing newline RunProject's OutputPhase appends so
	// the wire envelope stays single-line. Matches analyzeRespWriter.
	findings := findingsBuf.Bytes()
	if n := len(findings); n > 0 && findings[n-1] == '\n' {
		findings = findings[:n-1]
	}

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
	}
	// Mirror streamingAnalyzeResponse: CreateBaseline / DryRun get
	// the same payload under strict-verify so client behavior doesn't
	// silently change between modes.
	if r.args.CreateBaseline {
		basePath := r.in.Args.BasePath
		stats.BaselineIDs = scanner.CollectBaselineIDs(&res.FinalFindings, basePath)
	}
	if r.args.DryRun {
		stats.DryRunFiles = collectFixableFiles(&res.Fixup.Findings)
		stats.DryRunFixableCount = res.Fixup.FixableCount
		stats.DryRunStrippedByLevel = res.Fixup.StrippedByLevel
	}
	statsBytes, err := json.Marshal(stats)
	if err != nil {
		return fmt.Errorf("marshal stats: %w", err)
	}

	// Single-shot write of the full envelope: head + findings + stats
	// + tail. Mirrors the streaming envelope so daemonclient sees the
	// same shape under both modes.
	if _, err := w.Write([]byte(`{"ok":true,"data":{"findings":`)); err != nil {
		return err
	}
	if _, err := w.Write(findings); err != nil {
		return err
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

// runStrictVerify runs a fresh in-process baseline against the same
// paths and rule selection the daemon just analyzed, compares the two
// finding sets via daemon.Compare, and persists a structured log under
// `${repoDir}/.krit/daemon-divergence-NNNN.log` on divergence.
//
// The baseline runs with an empty ProjectHostState so no resident cache
// contaminates the comparison: a fresh parse cache, no resident
// CodeIndex / Resolver / OracleFilter / AnalysisCache, no resident
// oracle daemon. The intent is to reproduce what `krit --no-daemon`
// would emit and surface any drift the daemon's resident state
// introduced.
//
// On divergence the returned log path is non-empty and the error
// describes the diff shape; callers should fail the analyze response
// so the client sees the mismatch instead of a silently-wrong row set.
//
// Ported from internal/sessdaemon/analyze.go.runStrictVerify (now
// retired in favour of internal/cli/serve as the canonical daemon).
func (s *daemonState) runStrictVerify(ctx context.Context, args daemon.AnalyzeProjectArgs, daemonCols *scanner.FindingColumns) (string, error) {
	cfg, err := loadDaemonConfig(s.root)
	if err != nil {
		// Mirror buildProjectInput: a config-merge error is surfaced
		// but not fatal — fall through with whatever cfg the loader
		// produced (or a fresh default).
		if cfg == nil {
			cfg = config.NewConfig()
		}
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

	pc, err := scanner.NewParseCacheWithCap(s.repoDir, cacheutil.DefaultParseCacheCapBytes)
	if err != nil {
		return "", fmt.Errorf("baseline parse cache: %w", err)
	}
	defer pc.Close() //nolint:errcheck // best effort

	baseline, err := pipeline.RunProjectAnalysis(ctx, pipeline.ProjectInput{
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
