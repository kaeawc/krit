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

// daemonCompatibleFlags reports whether the requested flag set can be
// served by the daemon's analyze-project verb. Modes that write files,
// invoke profiling, or run meta commands stay on the in-process path.
// Order kept as named groups so a future flag's owner can decide which
// bucket their flag belongs in.
func daemonCompatibleFlags(f *scanFlags) bool {
	mutating := []bool{*f.Fix, *f.DryRun, *f.RemoveDeadCode, *f.FixBinary}
	meta := []bool{*f.Init, *f.Doctor, *f.Version, *f.List, *f.ValidateConfig, *f.GenerateSchema,
		*f.BaselineAudit, *f.RuleAudit, *f.OracleFilterFingerprint, *f.ListExperiments}
	profiling := []bool{*f.Perf, *f.PerfRules, *f.ProfileDispatch}
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
