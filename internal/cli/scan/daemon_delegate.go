package scan

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/kaeawc/krit/internal/cli/daemonclient"
	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/output"
	"github.com/kaeawc/krit/internal/scanner"
)

var ensureDaemonForScan = daemonclient.EnsureCompatible

// tryDaemonDelegate attempts to dispatch the current scan through a
// compatible krit daemon, starting or replacing one when needed.
// handled=true means the daemon produced the output and the caller
// should exit with the returned code; handled=false means the caller
// falls back to the in-process path. Fallback is silent except when
// daemon startup or a daemon call fails after the flag set has been
// accepted for delegation.
func tryDaemonDelegate(f *scanFlags, paths []string, repoDir string) (bool, int) {
	if f == nil || *f.NoDaemon {
		return false, 0
	}
	if verb, ok := metaVerbForFlags(f); ok {
		client, ok := daemonclient.TryConnect(repoDir, *f.DaemonSocket)
		if !ok {
			return false, 0
		}
		return runDaemonMetaVerb(client, f, paths, verb)
	}
	if !daemonCompatibleFlags(f) {
		return false, 0
	}
	if *f.DaemonSocket != "" {
		client, ok := daemonclient.TryConnect(repoDir, *f.DaemonSocket)
		if !ok {
			return false, 0
		}
		return runDaemonAnalyze(client, f, paths)
	}
	if daemonAutoStartDisabled() {
		client, ok := daemonclient.TryConnect(repoDir, "")
		if !ok {
			return false, 0
		}
		return runDaemonAnalyze(client, f, paths)
	}
	client, ok, err := ensureDaemonForScan(repoDir, daemonclient.SpawnOptions{
		Binary:      daemonBinaryForScan(),
		WaitTimeout: 30 * time.Second,
		LogPath:     filepath.Join(repoDir, ".krit", "daemon.log"),
		AutoRestart: true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: krit daemon unavailable (%v); falling back to in-process\n", err)
		return false, 0
	}
	if !ok {
		return false, 0
	}
	return runDaemonAnalyze(client, f, paths)
}

func runDaemonAnalyze(client *daemonclient.Client, f *scanFlags, paths []string) (bool, int) {
	start := time.Now()
	res, err := client.AnalyzeProject(buildDaemonAnalyzeArgs(f, paths))
	if err != nil {
		if daemonclient.IsBinaryHashMismatch(err) {
			fmt.Fprintf(os.Stderr, "warning: krit daemon rejected request (%v); falling back to in-process. Restart it with: krit daemon restart\n", err)
			return false, 0
		}
		fmt.Fprintf(os.Stderr, "warning: krit daemon call failed (%v); falling back to in-process\n", err)
		return false, 0
	}

	if !*f.Quiet {
		fmt.Fprintln(os.Stderr, "info: using daemon")
	}
	if handled, code := dispatchDaemonShortCircuits(f, paths, res); handled {
		return true, code
	}
	return true, writeDaemonFindings(f, res, start)
}

// dispatchDaemonShortCircuits handles the per-flag post-response
// branches that consume the daemon's findings JSON (or its columns
// segment) instead of streaming bytes to the output writer. Pulled
// out of tryDaemonDelegate so the per-call body stays under the
// gocyclo cap as new column-consuming flags (--rule-audit,
// --baseline-audit, --delta) join the daemon path.
//
// handled=false means no short-circuit fired and the caller should
// fall through to writeDaemonFindings; handled=true returns the
// short-circuit's exit code as-is.
func dispatchDaemonShortCircuits(f *scanFlags, paths []string, res daemon.AnalyzeProjectResult) (handled bool, code int) {
	switch {
	case *f.SampleRule != "":
		// --sample-rule consumes the JSON envelope and prints a
		// deterministic per-rule sample. Forwarding the envelope
		// verbatim would be confusing for an interactive flag.
		return true, runDaemonSampleRule(f, paths, res.Findings)
	case *f.CreateBaseline != "":
		// CreateBaseline replaces the findings-write path entirely:
		// no findings JSON, just the XML baseline file.
		return true, runDaemonCreateBaseline(f, res.Stats)
	case *f.DryRun:
		// DryRun prints the fixable-file list + summary lines; no
		// findings JSON. Mirrors printDryRunFixResult byte-for-byte.
		runDaemonDryRun(f, res.Stats)
		return true, 0
	case *f.RuleAudit:
		return true, runDaemonRuleAudit(f, paths, res.Columns)
	case *f.BaselineAudit:
		return true, runDaemonBaselineAudit(f, paths, res.Columns)
	case *f.Delta != "":
		return true, runDaemonDelta(f, paths, res.Columns)
	}
	return false, 0
}

// writeDaemonFindings handles the common path: stream the daemon's
// findings JSON to the configured output writer, surface any
// profile warnings, render the dispatch-profile table when armed,
// and produce the final exit code.
func writeDaemonFindings(f *scanFlags, res daemon.AnalyzeProjectResult, start time.Time) int {
	w, closer, openErr := openDaemonOutputWriter(f)
	if openErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", openErr)
		return 2
	}
	if _, err := w.Write(res.Findings); err != nil {
		_ = closer()
		fmt.Fprintf(os.Stderr, "error: write findings: %v\n", err)
		return 2
	}
	if err := closer(); err != nil {
		fmt.Fprintf(os.Stderr, "error: close output: %v\n", err)
		return 2
	}
	for _, msg := range res.Stats.ProfileWarnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", msg)
	}
	if *f.ProfileDispatch && res.DispatchProfile != nil && len(res.DispatchProfile.Timings) > 0 {
		emitDaemonDispatchProfile(res.DispatchProfile)
	}
	return finalScanExit(os.Stderr, res.Stats.FindingsCount, time.Since(start), *f.Quiet)
}

// metaVerbName tags the routable read-only meta queries the daemon
// can answer (list-rules, list-experiments, validate-config,
// oracle-filter-fingerprint, dump-types). Returned by metaVerbForFlags
// so the dispatch in tryDaemonDelegate stays a single switch.
//
// dump-types is "meta" in the routing sense (short-circuits before
// rule dispatch, writes a single artifact, captured stderr replayed
// on the CLI) even though the daemon does invoke the krit-types JVM
// to produce it — same as the in-process --output-types flow.
type metaVerbName int

const (
	metaVerbNone metaVerbName = iota
	metaVerbListRules
	metaVerbListExperiments
	metaVerbValidateConfig
	metaVerbOracleFilterFingerprint
	metaVerbDumpTypes
)

// metaVerbForFlags inspects f and returns which read-only meta verb
// the daemon should answer (when any). Mutually exclusive with the
// analyze-project route — these flags short-circuit before any rule
// dispatch in the in-process path, and the daemon mirrors that.
//
// Returns metaVerbNone when no meta flag is set, OR when other
// excluded flags would force in-process anyway (e.g. --custom-rule-jars
// for list-rules, --config for validate-config). The caller falls
// back to in-process in those cases.
func metaVerbForFlags(f *scanFlags) (metaVerbName, bool) {
	switch {
	case *f.List:
		// CustomRuleJars requires the krit-types JVM the daemon
		// doesn't manage on behalf of meta queries; defer to
		// in-process so plugin descriptors still surface.
		if *f.CustomRuleJars != "" {
			return metaVerbNone, false
		}
		return metaVerbListRules, true
	case *f.ListExperiments:
		return metaVerbListExperiments, true
	case *f.ValidateConfig:
		return metaVerbValidateConfig, true
	case *f.OracleFilterFingerprint:
		return metaVerbOracleFilterFingerprint, true
	case *f.OutputTypes != "":
		return metaVerbDumpTypes, true
	}
	return metaVerbNone, false
}

// runDaemonMetaVerb dispatches verb against the connected client and
// replays the captured Stdout/Stderr/ExitCode against the CLI's own
// streams. Any daemon-side error falls back to in-process by
// returning handled=false (matching the analyze-project route).
func runDaemonMetaVerb(client *daemonclient.Client, f *scanFlags, paths []string, verb metaVerbName) (bool, int) {
	res, err := callMetaVerb(client, f, paths, verb)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: krit daemon call failed (%v); falling back to in-process\n", err)
		return false, 0
	}
	if len(res.Stdout) > 0 {
		_, _ = os.Stdout.Write(res.Stdout)
	}
	if len(res.Stderr) > 0 {
		_, _ = os.Stderr.Write(res.Stderr)
	}
	return true, res.ExitCode
}

// callMetaVerb dispatches the appropriate daemonclient verb for
// verb. Pulled out of runDaemonMetaVerb so the switch is local and
// the caller stays small.
func callMetaVerb(client *daemonclient.Client, f *scanFlags, paths []string, verb metaVerbName) (daemon.MetaResult, error) {
	switch verb {
	case metaVerbListRules:
		return client.ListRules(daemon.ListRulesArgs{
			Verbose:    *f.Verbose,
			Maturity:   *f.Maturity,
			TaxonomyID: *f.ListRulesCWE,
		})
	case metaVerbListExperiments:
		// Mirror runner_state.go's effectiveFormat resolution so
		// daemon-routed --list-experiments matches the in-process
		// auto-promote-to-plain TTY behaviour.
		return client.ListExperiments(daemon.ListExperimentsArgs{
			Format: resolveEffectiveFormat(f),
		})
	case metaVerbValidateConfig:
		return client.ValidateConfig(daemon.ValidateConfigArgs{
			ConfigPath: *f.Config,
		})
	case metaVerbOracleFilterFingerprint:
		return client.OracleFilterFingerprint(daemon.OracleFilterFingerprintArgs{
			Paths:    paths,
			AllRules: *f.AllRules,
		})
	case metaVerbDumpTypes:
		// Absolutise the output path: the daemon runs from the
		// project root and would otherwise resolve a caller-relative
		// path against the wrong directory. filepath.Abs only fails
		// when CWD is unreadable; fall back to the raw value so the
		// daemon surfaces a clean create-file error rather than
		// silently writing somewhere unexpected.
		outputPath := *f.OutputTypes
		if abs, err := filepath.Abs(outputPath); err == nil {
			outputPath = abs
		}
		return client.DumpTypes(daemon.DumpTypesArgs{
			Paths:         paths,
			OutputPath:    outputPath,
			NoCacheOracle: *f.NoCacheOracle,
			Verbose:       *f.Verbose,
		})
	}
	return daemon.MetaResult{}, fmt.Errorf("unknown meta verb %d", verb)
}

// runDaemonCreateBaseline writes the daemon-computed sorted baseline
// ID list out to *f.CreateBaseline. Mirrors the in-process
// applyBaselinesAndDiff early-exit: no findings JSON, just the XML
// file plus a "Created baseline" stderr summary.
func runDaemonCreateBaseline(f *scanFlags, stats daemon.AnalyzeProjectStats) int {
	if err := scanner.WriteBaselineIDsXML(*f.CreateBaseline, stats.BaselineIDs); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to write baseline: %v\n", err)
		return 2
	}
	if !*f.Quiet {
		fmt.Fprintf(os.Stderr, "info: Created baseline with %d issue(s) at %s\n", len(stats.BaselineIDs), *f.CreateBaseline)
	}
	return 0
}

// runDaemonDryRun replays the daemon-computed fixable-file list and
// summary lines. Matches printDryRunFixResult byte-for-byte:
// file-per-line on stdout plus the "N fix(es) available across M
// file(s)" stderr summary (or a level-aware "no fixable" hint).
func runDaemonDryRun(f *scanFlags, stats daemon.AnalyzeProjectStats) {
	for _, file := range stats.DryRunFiles {
		fmt.Println(file)
	}
	if *f.Quiet {
		return
	}
	if stats.DryRunFixableCount == 0 {
		if stats.DryRunStrippedByLevel > 0 {
			fmt.Fprintf(os.Stderr, "info: No auto-fixable issues at level %s. %d fix(es) available at higher levels (use --fix-level=semantic).\n",
				*f.FixLevel, stats.DryRunStrippedByLevel)
		} else {
			fmt.Fprintln(os.Stderr, "info: No auto-fixable issues found.")
		}
		return
	}
	fmt.Fprintf(os.Stderr, "info: %d fix(es) available across %d file(s).\n",
		stats.DryRunFixableCount, len(stats.DryRunFiles))
}

// runDaemonSampleRule decodes the daemon's JSON envelope into a
// FindingColumns and runs the sampler. Mirrors the in-process
// runner.outputPhase short-circuit so users see the same human output
// regardless of whether the daemon served the call.
func runDaemonSampleRule(f *scanFlags, paths []string, findingsJSON []byte) int {
	var report output.JSONReport
	if err := json.Unmarshal(findingsJSON, &report); err != nil {
		fmt.Fprintf(os.Stderr, "error: decode daemon findings: %v\n", err)
		return 2
	}
	collector := scanner.NewFindingCollector(len(report.Findings))
	for _, finding := range report.Findings {
		collector.Append(scanner.Finding{
			File:    finding.File,
			Line:    finding.Line,
			Col:     finding.Column,
			RuleSet: finding.RuleSet,
			Rule:    finding.Rule,
			Message: finding.Message,
		})
	}
	columns := collector.Columns()
	basePath := *f.BasePath
	if basePath == "" && len(paths) > 0 {
		basePath, _ = filepath.Abs(paths[0])
	}
	return RunSampleFindingsColumns(columns, *f.SampleRule, *f.SampleCount, *f.SampleContext, basePath)
}

// tryDaemonClearCache routes --clear-cache through a running daemon
// when one is reachable. Returning true means the daemon performed
// the clear and the caller should exit 0; false means the caller
// falls back to in-process clear-cache (which still works when no
// daemon is up). See tryDaemonClearMatrixCache below for the matrix-
// baseline cache equivalent.
func tryDaemonClearCache(f *scanFlags, repoDir string) bool {
	if f == nil || *f.NoDaemon || !*f.ClearCache {
		return false
	}
	client, ok := daemonclient.TryConnect(repoDir, *f.DaemonSocket)
	if !ok {
		return false
	}
	res, err := client.ClearCache(daemon.ClearCacheArgs{})
	if err != nil {
		if daemonclient.IsBinaryHashMismatch(err) {
			fmt.Fprintf(os.Stderr, "warning: krit daemon rejected request (%v); falling back to in-process. Restart it with: krit daemon restart\n", err)
			return false
		}
		fmt.Fprintf(os.Stderr, "warning: krit daemon clear-cache failed (%v); falling back to in-process\n", err)
		return false
	}
	if res.Cleared {
		fmt.Fprintln(os.Stderr, "info: Cache cleared (via daemon).")
	} else {
		fmt.Fprintln(os.Stderr, "info: Cache clear completed with errors (via daemon).")
	}
	return true
}

// tryDaemonClearMatrixCache routes --clear-matrix-cache through a
// running daemon when one is reachable. Returning true means the
// daemon performed the clear and the caller should exit 0; false
// means the caller falls back to in-process ClearMatrixCache (which
// still works when no daemon is up).
//
// The matrix cache lives at a host-wide path (~/.cache/krit/
// matrix-baseline). Multiple per-repo daemons can share it, but the
// clear itself is best-effort (matrixSave/Load tolerate missing or
// partial entries — a wiped slot just triggers a recompute). The
// daemon serialises against its own analyze loop on state.analyzeMu
// to avoid intra-daemon races; cross-daemon races are tolerated by
// the matrix runner's own miss-then-recompute design, so no extra
// host-level lock is needed.
func tryDaemonClearMatrixCache(f *scanFlags, repoDir string) bool {
	if f == nil || *f.NoDaemon || !*f.ClearMatrixCache {
		return false
	}
	client, ok := daemonclient.TryConnect(repoDir, *f.DaemonSocket)
	if !ok {
		return false
	}
	res, err := client.ClearMatrixCache(daemon.ClearMatrixCacheArgs{})
	if err != nil {
		if daemonclient.IsBinaryHashMismatch(err) {
			fmt.Fprintf(os.Stderr, "warning: krit daemon rejected request (%v); falling back to in-process. Restart it with: krit daemon restart\n", err)
			return false
		}
		fmt.Fprintf(os.Stderr, "warning: krit daemon clear-matrix-cache failed (%v); falling back to in-process\n", err)
		return false
	}
	if res.Cleared {
		fmt.Fprintln(os.Stderr, "info: Matrix baseline cache cleared (via daemon).")
	} else {
		fmt.Fprintln(os.Stderr, "info: Matrix baseline cache clear completed with errors (via daemon).")
	}
	return true
}

// emitDaemonDispatchProfile renders the daemon-served dispatch
// profile through the same reportDispatchProfile path the in-process
// runner uses. The wire-shaped daemon.FileTiming entries are copied
// into the pipeline.FileTiming slice the renderer expects so the
// stderr distribution table is byte-identical regardless of which
// path executed the scan.
func emitDaemonDispatchProfile(p *daemon.DispatchProfile) {
	if p == nil || len(p.Timings) == 0 {
		return
	}
	timings := make([]fileTiming, len(p.Timings))
	for i, t := range p.Timings {
		timings[i] = fileTiming{
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
	reportDispatchProfile(timings, p.Workers, time.Duration(p.WallMs)*time.Millisecond)
}

func daemonBinaryForScan() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return exe
}

func daemonAutoStartDisabled() bool {
	return os.Getenv("KRIT_NO_DAEMON_AUTOSTART") == "1"
}

// daemonCompatibleFlags reports whether the requested flag set can be
// served by the daemon's analyze-project verb. Modes that write files
// or run meta commands stay on the in-process path. Order kept as
// named groups so a future flag's owner can decide which bucket their
// flag belongs in.
//
// The flags in the meta / mutating buckets below are intentionally
// pinned in-process. See docs/daemon-flag-routing.md for the per-flag
// rationale (writes file at client CWD, reports on calling binary's
// env, mutates checked-in registry source, re-execs subprocesses,
// etc.) and for the wire change each one would need before daemon
// routing becomes safe. Update that doc when adding to or removing
// from these lists.
//
// Profiling flags ARE compatible:
//
//   - --perf / --perf-rules: the daemon wires its own perf.Tracker
//     when ShowPerf is set in args; OutputPhase emits the
//     hierarchical timing tree in the JSON envelope.
//   - --profile-dispatch: per-file timings ride back through the
//     daemon response's dispatch_profile field; the CLI renders the
//     same distribution table either way.
//   - --cpuprofile / --memprofile: the daemon process is wrapped in
//     pprof.StartCPUProfile / WriteHeapProfile around the
//     analyze-project call and writes the profile to the path the
//     CLI provided. Profiling the daemon is the documented behavior
//     when a daemon is in use; pair with --no-daemon to profile the
//     short-lived CLI process instead.
//
// --no-cache, --clear-cache, --clear-matrix-cache are also daemon-
// routable: --no-cache rides on AnalyzeProjectArgs.NoCache (the
// daemon nils every disk-cache pointer for this single call); the
// two clear-* flags are handled by tryDaemonClearCache and
// tryDaemonClearMatrixCache before tryDaemonDelegate runs (early-
// exit verbs). --clear-cache also drops the daemon's resident
// WorkspaceState slots so the next analyze rebuilds from cold;
// --clear-matrix-cache only touches the host-wide matrix baseline
// directory because the matrix cache has no resident counterpart.
//
// --create-baseline and --dry-run ARE compatible: the daemon computes
// the baseline-ID list / fixable-file list from the same FindingColumns
// the in-process flow uses, ships them in AnalyzeProjectStats, and the
// CLI writes the baseline file (or prints the dry-run lines) locally.
// The daemon never touches user files; the file write happens with the
// CLI's CWD and permissions, preserving the daemon's read-only invariant.
//
// --rule-audit, --baseline-audit, and --delta are daemon-compatible
// too: the daemon ships post-pipeline FindingColumns on the wire
// (under AnalyzeProjectResult.Columns) when AnalyzeProjectArgs.
// IncludeColumns is true, and the CLI runs the audit / delta filter
// locally against the deserialized columns. --delta's worktree
// orchestration stays CLI-side (daemon-side worktree spawning would
// add filesystem side-effects to a long-lived service); only the
// current-tree scan and the delta diff step are daemon-routable.
//
// --fix, --fix-binary, and --remove-dead-code stay in-process. Each
// would need a separate fix-payload-over-the-wire design where the
// daemon serialises text edits / binary operations and the CLI replays
// them; the existing fix application code path is intertwined with
// per-rule context that doesn't currently survive serialisation. Land
// those in follow-up PRs once a payload schema exists.
func daemonCompatibleFlags(f *scanFlags) bool {
	mutating := []bool{*f.Fix, *f.RemoveDeadCode, *f.FixBinary}
	meta := []bool{*f.Init, *f.Doctor, *f.Version, *f.List, *f.ValidateConfig, *f.GenerateSchema,
		*f.OracleFilterFingerprint, *f.ListExperiments}
	for _, group := range [][]bool{mutating, meta} {
		for _, on := range group {
			if on {
				return false
			}
		}
	}
	strs := []string{*f.Completions,
		*f.PromoteExperiment, *f.DeprecateExperiment, *f.NewExperiment, *f.ExperimentMatrix}
	for _, s := range strs {
		if s != "" {
			return false
		}
	}
	return true
}

// buildDaemonAnalyzeArgs translates the scan-flag set into the
// daemon's wire args. Only flags daemonCompatibleFlags admits are
// propagated; anything outside that set should have been filtered
// upstream.
func buildDaemonAnalyzeArgs(f *scanFlags, paths []string) daemon.AnalyzeProjectArgs {
	format := *f.Format
	if *f.Report != "" {
		format = *f.Report
	}
	// --sample-rule asks for JSON findings post-processed on the CLI
	// side. Force the daemon to emit JSON regardless of the user's
	// --format choice so the writer can decode the envelope. The
	// human-formatted sampler output is then printed by the CLI.
	if *f.SampleRule != "" {
		format = "json"
	}
	// Resolve --input-types to an absolute path before forwarding:
	// the daemon process has its own CWD (the project root) and a
	// caller-relative path would resolve elsewhere. filepath.Abs
	// only fails when CWD is unreadable; in that case forward the
	// raw value so the daemon can return a clean "open" error rather
	// than silently swallowing the flag.
	inputTypesPath := *f.InputTypes
	if inputTypesPath != "" {
		if abs, err := filepath.Abs(inputTypesPath); err == nil {
			inputTypesPath = abs
		}
	}
	// CreateBaseline mirrors applyBaselinesAndDiff: when set, the
	// in-process flow short-circuits before baseline filtering, so
	// the daemon must too. The daemon's analyze_project drops
	// BaselinePath internally when CreateBaseline is true, but
	// leaving it empty here keeps the wire intent explicit.
	baselinePath := *f.Baseline
	if *f.CreateBaseline != "" {
		baselinePath = ""
	}
	// --rule-audit / --baseline-audit / --delta need the post-pipeline
	// FindingColumns shipped back so the CLI can run the audit or
	// delta filter locally. The daemon ships the columns alongside
	// the normal findings JSON only when IncludeColumns is true so
	// non-audit / non-delta scans stay on the original
	// {findings,stats[,dispatch_profile]} wire shape.
	//
	// Also force JSON format when --rule-audit's caller picked a
	// non-default --format: the audit short-circuit consumes columns
	// directly and discards the findings JSON, but the daemon still
	// streams it. --baseline-audit / --delta tolerate any format on
	// the daemon side because the bytes are discarded the same way.
	includeColumns := *f.RuleAudit || *f.BaselineAudit || *f.Delta != ""
	return daemon.AnalyzeProjectArgs{
		Paths:            paths,
		Format:           format,
		BaselinePath:     baselinePath,
		DiffRef:          *f.Diff,
		MinConfidence:    *f.MinConfidence,
		WarningsAsErrors: *f.WarningsAsErrors,
		IncludeGenerated: *f.IncludeGenerated,
		AllRules:         *f.AllRules,
		Experimental:     *f.Experimental,
		EnableRules:      *f.EnableRules,
		DisableRules:     *f.DisableRules,
		ShowPerf:         *f.Perf || *f.PerfRules,
		PerfRules:        *f.PerfRules,
		InputTypesPath:   inputTypesPath,
		NoCache:          *f.NoCache,
		ProfileDispatch:  *f.ProfileDispatch,
		CPUProfilePath:   absoluteProfilePath(*f.CPUProfile),
		MemProfilePath:   absoluteProfilePath(*f.MemProfile),
		BasePath:         resolveBasePath(*f.BasePath, paths),
		CreateBaseline:   *f.CreateBaseline != "",
		DryRun:           *f.DryRun,
		FixLevel:         *f.FixLevel,
		IncludeColumns:   includeColumns,
		ClientBinaryHash: daemonclient.CurrentBinaryHash(),
	}
}

// absoluteProfilePath converts a CLI-supplied profile path into an
// absolute path so the daemon writes next to the calling user's
// working directory regardless of where `krit serve` was launched
// from. Empty input returns empty (no profile requested). Failures
// to resolve fall back to the original value — the daemon will
// surface a create error in ProfileWarnings.
func absoluteProfilePath(p string) string {
	if p == "" {
		return ""
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}

// resolveBasePath mirrors runner_state.go's basePath resolution so
// daemon-computed baseline IDs match the in-process
// WriteBaselineColumns output byte-for-byte. Empty BasePath falls
// back to the absolute first scan path. The CLI's runner_state.go
// uses `r.basePath, _ = filepath.Abs(...)` — discarding the error and
// keeping whatever filepath.Abs returned (the empty string on
// failure). We match that exactly so cross-process IDs collide on
// every input runner_state would.
func resolveBasePath(explicit string, paths []string) string {
	if explicit != "" {
		return explicit
	}
	if len(paths) == 0 {
		return ""
	}
	abs, _ := filepath.Abs(paths[0])
	return abs
}

// openDaemonOutputWriter mirrors runner.openOutputWriter for the
// daemon path. Returns the writer, a closer (no-op for stdout), and
// any creation error.
func openDaemonOutputWriter(f *scanFlags) (io.Writer, func() error, error) {
	if *f.Output != "" {
		file, err := os.Create(*f.Output)
		if err != nil {
			return nil, func() error { return nil }, fmt.Errorf("create %s: %w", *f.Output, err)
		}
		return file, file.Close, nil
	}
	return os.Stdout, func() error { return nil }, nil
}
