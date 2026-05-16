package scan

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/kaeawc/krit/internal/cli/daemonclient"
	"github.com/kaeawc/krit/internal/daemon"
)

// tryDaemonDelegate attempts to dispatch the current scan through a
// running krit daemon. handled=true means the daemon produced the
// output and the caller should exit with the returned code;
// handled=false means the caller falls back to the in-process path.
// Fallback is silent except for the daemon-talked-but-failed case,
// which logs a warning and continues.
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
	client, ok := daemonclient.TryConnect(repoDir, *f.DaemonSocket)
	if !ok {
		return false, 0
	}

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

	fmt.Fprintln(os.Stderr, "info: using daemon")
	w, closer, openErr := openDaemonOutputWriter(f)
	if openErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", openErr)
		return true, 2
	}
	if _, err := w.Write(res.Findings); err != nil {
		_ = closer()
		fmt.Fprintf(os.Stderr, "error: write findings: %v\n", err)
		return true, 2
	}
	if err := closer(); err != nil {
		fmt.Fprintf(os.Stderr, "error: close output: %v\n", err)
		return true, 2
	}
	return true, finalScanExit(os.Stderr, res.Stats.FindingsCount, time.Since(start), *f.Quiet)
}

// metaVerbName tags the routable read-only meta queries the daemon
// can answer (list-rules, list-experiments, validate-config,
// oracle-filter-fingerprint). Returned by metaVerbForFlags so the
// dispatch in tryDaemonDelegate stays a single switch.
type metaVerbName int

const (
	metaVerbNone metaVerbName = iota
	metaVerbListRules
	metaVerbListExperiments
	metaVerbValidateConfig
	metaVerbOracleFilterFingerprint
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
	}
	return daemon.MetaResult{}, fmt.Errorf("unknown meta verb %d", verb)
}

// daemonCompatibleFlags reports whether the requested flag set can be
// served by the daemon's analyze-project verb. Modes that write files,
// invoke profiling, or run meta commands stay on the in-process path.
// Order kept as named groups so a future flag's owner can decide which
// bucket their flag belongs in.
//
// --perf and --perf-rules ARE compatible: the daemon wires its own
// perf.Tracker when ShowPerf is set in args, and OutputPhase emits the
// hierarchical timing tree in the JSON envelope just like in-process.
// --profile-dispatch stays in-process because the per-file timing
// fan-out it needs is recorded against the *rules.Dispatcher state and
// isn't currently surfaced through the daemon wire.
func daemonCompatibleFlags(f *scanFlags) bool {
	mutating := []bool{*f.Fix, *f.DryRun, *f.RemoveDeadCode, *f.FixBinary}
	meta := []bool{*f.Init, *f.Doctor, *f.Version, *f.List, *f.ValidateConfig, *f.GenerateSchema,
		*f.BaselineAudit, *f.RuleAudit, *f.OracleFilterFingerprint, *f.ListExperiments}
	profiling := []bool{*f.ProfileDispatch}
	cacheOps := []bool{*f.NoCache, *f.ClearCache, *f.ClearMatrixCache}
	for _, group := range [][]bool{mutating, meta, profiling, cacheOps} {
		for _, on := range group {
			if on {
				return false
			}
		}
	}
	strs := []string{*f.CreateBaseline, *f.CPUProfile, *f.MemProfile,
		*f.Completions, *f.SampleRule, *f.InputTypes, *f.OutputTypes, *f.Delta,
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
	return daemon.AnalyzeProjectArgs{
		Paths:            paths,
		Format:           format,
		BaselinePath:     *f.Baseline,
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
		ClientBinaryHash: daemonclient.CurrentBinaryHash(),
	}
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
