package scan

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kaeawc/krit/internal/cli/clishared"
	"github.com/kaeawc/krit/internal/experiment"
	"github.com/kaeawc/krit/internal/pipeline"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// FixupLevelResolver resolves the CLI-side fix-level cap for the
// daemon-routed --fix path. Re-declared as a package-private helper so
// runDaemonFix can stay decoupled from the runner_state-bound resolver
// in resolveMaxFixLevel (which also probes *f.Fix / *f.DryRun for the
// no-cap fast path).
func resolveDaemonMaxFixLevel(f *scanFlags) (rules.FixLevel, bool) {
	parsed, ok := rules.ParseFixLevel(*f.FixLevel)
	if !ok {
		fmt.Fprintf(os.Stderr, "error: invalid fix level '%s'. Use: cosmetic, idiomatic, semantic\n", *f.FixLevel)
		return rules.FixIdiomatic, false
	}
	return parsed, true
}

// decodeDaemonColumns parses the wire-form FindingColumns segment
// shipped under AnalyzeProjectResult.Columns. An empty payload is
// surfaced as a typed error so the caller can fall back to the
// in-process path; the daemon should only omit the segment when the
// CLI didn't request it via AnalyzeProjectArgs.IncludeColumns, so an
// audit / delta path seeing nil columns means a daemon-side bug.
func decodeDaemonColumns(raw json.RawMessage) (*scanner.FindingColumns, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("daemon response missing columns payload")
	}
	cols := &scanner.FindingColumns{}
	if err := json.Unmarshal(raw, cols); err != nil {
		return nil, fmt.Errorf("decode columns: %w", err)
	}
	return cols, nil
}

// runDaemonRuleAudit replays RunRuleAuditColumns against the daemon-
// shipped FindingColumns. The CLI mirrors the in-process
// outputPhase short-circuit so audit output is byte-identical
// regardless of which path executed the scan.
func runDaemonRuleAudit(f *scanFlags, paths []string, rawColumns json.RawMessage) int {
	cols, err := decodeDaemonColumns(rawColumns)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: --rule-audit via daemon: %v\n", err)
		return 2
	}
	return RunRuleAuditColumns(cols, RuleAuditOpts{
		MinFindings:    *f.RuleAuditMin,
		DetailRules:    *f.RuleAuditDetails,
		SamplesPerRule: *f.RuleAuditSamples,
		SampleContext:  *f.RuleAuditContext,
		ClusterFilter:  *f.RuleAuditCluster,
		Targets:        paths,
		Format:         resolveEffectiveFormat(f),
	})
}

// runDaemonBaselineAudit resolves the baseline path the same way the
// in-process flow does, loads it, then replays
// RunBaselineAuditColumns against the daemon-shipped columns.
func runDaemonBaselineAudit(f *scanFlags, paths []string, rawColumns json.RawMessage) int {
	cols, err := decodeDaemonColumns(rawColumns)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: --baseline-audit via daemon: %v\n", err)
		return 2
	}
	baselinePath, err := ResolveBaselineAuditPath(*f.Baseline, paths)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	baseline, err := scanner.LoadBaseline(baselinePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to load baseline: %v\n", err)
		return 2
	}
	basePath := *f.BasePath
	if basePath == "" && len(paths) > 0 {
		basePath, _ = filepath.Abs(paths[0])
	}
	return RunBaselineAuditColumns(cols, baseline, baselinePath, basePath, paths, resolveEffectiveFormat(f))
}

// runDaemonDelta replays the in-process --delta flow against the
// daemon-shipped columns. The base-ref worktree spawn + re-exec still
// happens client-side (daemon-side worktree management would add
// filesystem-mutating side effects to a long-lived service); only the
// current-tree scan was routed through the daemon. After filtering
// the daemon-supplied columns by base-ref baseline IDs the CLI
// re-emits the filtered findings locally via pipeline.OutputPhase so
// the output format (json / plain / sarif / checkstyle) matches the
// CLI's --format / --report choice.
func runDaemonDelta(f *scanFlags, paths []string, rawColumns json.RawMessage) int {
	cols, err := decodeDaemonColumns(rawColumns)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: --delta via daemon: %v\n", err)
		return 2
	}
	if *f.Baseline != "" {
		fmt.Fprintln(os.Stderr, "error: --delta and --baseline are mutually exclusive")
		return 2
	}
	basePath := *f.BasePath
	if basePath == "" && len(paths) > 0 {
		basePath, _ = filepath.Abs(paths[0])
	}
	beforeCount := cols.Len()
	baseIDs, err := deltaBaseFindingIDsForFlags(*f.Delta, f, paths)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: --delta %s: %v\n", *f.Delta, err)
		return 2
	}
	filtered := filterColumnsNewSince(cols, baseIDs, basePath)
	if *f.Verbose {
		fmt.Fprintf(os.Stderr, "verbose: --delta %s filtered %d → %d new findings\n",
			*f.Delta, beforeCount, filtered.Len())
	}
	return emitDaemonFilteredColumns(f, paths, &filtered, basePath)
}

// emitDaemonFilteredColumns writes a column-filter result (currently
// only --delta) to the user-requested output sink in the user-
// requested format. The daemon's pre-filter findings JSON is
// discarded; this path re-runs pipeline.OutputPhase locally so the
// filtered set goes through the same formatter the in-process path
// uses. ActiveRules / Version metadata are taken from the CLI's
// resident rule registry to keep fixLevel / effort annotations in
// the JSON output identical to the in-process emit.
func emitDaemonFilteredColumns(f *scanFlags, paths []string, columns *scanner.FindingColumns, basePath string) int {
	w, closer, openErr := openDaemonOutputWriter(f)
	if openErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", openErr)
		return 2
	}
	defer func() {
		if err := closer(); err != nil {
			fmt.Fprintf(os.Stderr, "error: close output: %v\n", err)
		}
	}()

	format := resolveEffectiveFormat(f)
	// JSONCompact mirrors runner.outputPhase: stay compact only when
	// -o (output file) is set, so terminal output stays indented.
	jsonCompact := *f.Output != ""
	activeRules := activeRulesForDaemonEmit(f)
	out, err := (pipeline.OutputPhase{}).Run(context.Background(), pipeline.OutputInput{
		FixupResult: pipeline.FixupResult{
			CrossFileResult: pipeline.CrossFileResult{
				DispatchResult: pipeline.DispatchResult{
					IndexResult: pipeline.IndexResult{
						ParseResult: pipeline.ParseResult{
							Paths:       paths,
							ActiveRules: activeRules,
						},
					},
					Findings: *columns,
				},
			},
		},
		Writer:           w,
		Format:           format,
		BasePath:         basePath,
		StartTime:        time.Now(),
		Version:          Version,
		ExperimentNames:  experiment.Current().Names(),
		JSONCompact:      jsonCompact,
		WarningsAsErrors: *f.WarningsAsErrors,
		MinConfidence:    *f.MinConfidence,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	_ = out
	return 0
}

// activeRulesForDaemonEmit resolves the active rule set the daemon-
// served --delta emit needs to attach fixLevel / effort metadata to
// JSON output. Mirrors rules.ActiveRulesV2 with the flags the daemon
// path admits (NO custom-rule-jars: daemon-eligibility gate keeps
// callers with --custom-rule-jars on the in-process path).
func activeRulesForDaemonEmit(f *scanFlags) []*api.Rule {
	disabledSet := clishared.ParseRuleNameSetCSV(*f.DisableRules)
	enabledSet := clishared.ParseRuleNameSetCSV(*f.EnableRules)
	return rules.ActiveRulesV2(disabledSet, enabledSet, *f.AllRules, *f.Experimental, *f.Strict)
}

// runDaemonFix replays the in-process --fix flow against the daemon-
// shipped FindingColumns. The daemon never writes user files — it
// only computes the FixPool / BinaryFixPool and ships them as part of
// FindingColumns under AnalyzeProjectResult.Columns. The CLI applies
// the writes locally via pipeline.FixupPhase against the user's CWD
// and permissions, preserving the daemon's read-only invariant.
//
// Mirrors runner.runFixup byte-for-byte: same Apply / ApplyBinary /
// Suffix / MaxFixLevel / DryRunBinary / CountOnly inputs, same
// printFixupResult / printBinaryFixResult human output, same
// no-explicit-output JSON-fix exit-code carve-out.
func runDaemonFix(f *scanFlags, paths []string, rawColumns json.RawMessage, start time.Time) int {
	cols, err := decodeDaemonColumns(rawColumns)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: --fix via daemon: %v\n", err)
		return 2
	}
	maxFixLevel, ok := resolveDaemonMaxFixLevel(f)
	if !ok {
		return 2
	}
	fixRes, _ := (pipeline.FixupPhase{}).Run(context.Background(), pipeline.FixupInput{
		CrossFileResult: pipeline.CrossFileResult{
			DispatchResult: pipeline.DispatchResult{
				Findings: *cols,
			},
		},
		Apply:        *f.Fix && !*f.DryRun,
		ApplyBinary:  *f.FixBinary,
		Suffix:       *f.FixSuffix,
		MaxFixLevel:  maxFixLevel,
		DryRunBinary: *f.DryRun,
		CountOnly:    *f.DryRun,
	})
	postColumns := fixRes.Findings
	printDaemonFixupResult(f, &postColumns, fixRes, start)
	printDaemonBinaryFixResult(f, fixRes)

	effectiveFormat := resolveEffectiveFormat(f)
	if *f.Output == "" && effectiveFormat == "json" && *f.Report == "" && *f.Fix {
		if postColumns.Len()-fixRes.FixableCount > 0 {
			if !*f.Quiet {
				fmt.Fprintf(os.Stderr, "info: %d unfixable issue(s) remain.\n", postColumns.Len()-fixRes.FixableCount)
			}
			return 1
		}
		return 0
	}

	// Fall through to emit the post-fix findings via OutputPhase, the
	// same as the in-process runner.outputPhase tail does after a --fix
	// run that didn't short-circuit on the JSON carve-out above.
	basePath := *f.BasePath
	if basePath == "" && len(paths) > 0 {
		basePath, _ = filepath.Abs(paths[0])
	}
	return emitDaemonFilteredColumns(f, paths, &postColumns, basePath)
}

// printDaemonFixupResult mirrors runner.printFixupResult for the
// daemon path. The CLI-side runner version reads r.allColumns to walk
// fixable rows in dry-run mode; the daemon equivalent walks the post-
// fix columns the local FixupPhase produced.
func printDaemonFixupResult(f *scanFlags, postColumns *scanner.FindingColumns, fixRes pipeline.FixupResult, start time.Time) {
	fixableCount := fixRes.FixableCount
	strippedByLevel := fixRes.StrippedByLevel
	if fixableCount == 0 {
		if !*f.Quiet {
			if strippedByLevel > 0 {
				fmt.Fprintf(os.Stderr, "info: No auto-fixable issues at level %s. %d fix(es) available at higher levels (use --fix-level=semantic).\n",
					*f.FixLevel, strippedByLevel)
			} else {
				fmt.Fprintln(os.Stderr, "info: No auto-fixable issues found.")
			}
		}
		return
	}
	if *f.DryRun {
		printDaemonDryRunFixResult(f, postColumns, fixableCount)
	} else {
		printDaemonAppliedFixResult(f, fixRes, start)
	}
}

func printDaemonDryRunFixResult(f *scanFlags, postColumns *scanner.FindingColumns, fixableCount int) {
	seen := make(map[string]bool)
	for row := 0; row < postColumns.Len(); row++ {
		if !postColumns.HasFix(row) {
			continue
		}
		file := postColumns.FileAt(row)
		if !seen[file] {
			seen[file] = true
			fmt.Println(file)
		}
	}
	if !*f.Quiet {
		fmt.Fprintf(os.Stderr, "info: %d fix(es) available across %d file(s).\n", fixableCount, len(seen))
	}
}

func printDaemonAppliedFixResult(f *scanFlags, fixRes pipeline.FixupResult, start time.Time) {
	binarySet := make(map[error]bool, len(fixRes.BinaryErrors))
	for _, e := range fixRes.BinaryErrors {
		binarySet[e] = true
	}
	for _, e := range fixRes.FixErrors {
		if binarySet[e] {
			continue
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", e)
	}
	if !*f.Quiet {
		suffix := "in place"
		if *f.FixSuffix != "" {
			suffix = "with suffix '" + *f.FixSuffix + "'"
		}
		fmt.Fprintf(os.Stderr, "info: Applied %d fix(es) across %d file(s) %s in %v.\n",
			fixRes.TextApplied, len(fixRes.ModifiedFiles), suffix, time.Since(start).Round(time.Millisecond))
	}
}

func printDaemonBinaryFixResult(f *scanFlags, fixRes pipeline.FixupResult) {
	if !*f.FixBinary {
		return
	}
	for _, e := range fixRes.BinaryErrors {
		fmt.Fprintf(os.Stderr, "error: binary fix: %v\n", e)
	}
	if fixRes.BinaryApplied > 0 && !*f.Quiet {
		mode := "applied"
		if *f.DryRun {
			mode = "available"
		}
		fmt.Fprintf(os.Stderr, "info: %d binary fix(es) %s.\n", fixRes.BinaryApplied, mode)
	}
}

// runDaemonRemoveDeadCode replays --remove-dead-code against the
// daemon-shipped FindingColumns. As with --fix, the daemon never
// writes user files; it ships the dead-code findings (which carry
// their Fix payloads) and the CLI runs deadcode.BuildPlanColumns +
// plan.Apply locally so writes happen with the CLI's CWD and
// permissions.
func runDaemonRemoveDeadCode(f *scanFlags, rawColumns json.RawMessage) int {
	cols, err := decodeDaemonColumns(rawColumns)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: --remove-dead-code via daemon: %v\n", err)
		return 2
	}
	return RunDeadCodeRemovalColumns(cols, resolveEffectiveFormat(f), *f.DryRun, *f.FixSuffix)
}
