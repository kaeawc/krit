package scan

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"time"

	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/perf"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// firCheckerOpts groups every flag and runtime input runFIRCheckerPass
// needs. Pulled out so the call site reads as one struct literal.
type firCheckerOpts struct {
	Enabled bool // typically *firFlag && !*noFirFlag
	// Checker is the FirChecker that runs the actual subprocess or
	// daemon invocation. Production callers pass *ProductionFirChecker;
	// tests inject FakeFirChecker. When nil the pass is a no-op even if
	// Enabled is true.
	Checker     firchecks.FirChecker
	Verbose     bool
	ActiveRules []*api.Rule
	ParsedFiles []*scanner.File
	Tracker     perf.Tracker
	VerboseOut  io.Writer
	// Thorough mirrors --depth=thorough and tells ActiveFirRules to
	// project per-rule ThoroughIdentifiers / ThoroughAllFiles. False
	// keeps the existing balanced/fast filter shape.
	Thorough bool
}

// resolveFIRTargetFiles picks the Kotlin file list the FIR checker should
// run against. When summary.AllFiles is true (every parsed file is a
// candidate), the helper expands parsedFiles into absolute paths and
// returns them sorted for deterministic invocation; otherwise it returns
// summary.Paths verbatim.
//
// Pure: same inputs always produce the same output, no global state. The
// AllFiles branch is the only piece worth unit-testing — it's where the
// abs-path-then-sort behavior lives.
func resolveFIRTargetFiles(summary firchecks.FirFilterSummary, parsedFiles []*scanner.File) []string {
	if !summary.AllFiles {
		return summary.Paths
	}
	out := make([]string, 0, len(parsedFiles))
	for _, file := range parsedFiles {
		if file == nil {
			continue
		}
		abs, err := filepath.Abs(file.Path)
		if err != nil {
			abs = file.Path
		}
		out = append(out, abs)
	}
	sort.Strings(out)
	return out
}

// activeRuleIDs flattens a rule slice into its non-nil rule IDs.
func activeRuleIDs(rules []*api.Rule) []string {
	out := make([]string, 0, len(rules))
	for _, r := range rules {
		if r == nil {
			continue
		}
		out = append(out, r.ID)
	}
	return out
}

// runFIRCheckerPass invokes the FIR checker subprocess (or daemon) and
// merges its findings into base. No-op when opts.Enabled is false; the
// returned slice is base unchanged.
//
// Errors are silenced unless Verbose is set (FIR is gated behind --fir
// during the pilot phase, so a stray daemon failure must never break the
// main scan); a verbose-only line reports the error. Verbose mode also
// emits a one-line summary of timing, finding counts, file selection,
// and the FIR cache stats.
func runFIRCheckerPass(opts firCheckerOpts, base []scanner.Finding) []scanner.Finding {
	if !opts.Enabled || opts.Checker == nil {
		return base
	}
	active := firchecks.ActiveFirRules(activeRuleIDs(opts.ActiveRules), opts.Thorough)
	if len(active.Names) == 0 {
		// No FIR-eligible rules in the active set; skip the JVM
		// subprocess entirely. Matters once `--depth=thorough` defaults
		// FIR on for rule sets that may not include any FIR checks.
		return base
	}
	start := time.Now()
	subTracker := opts.Tracker.Serial("firCheck")
	summary := firchecks.CollectFirCheckFiles(active.Filters, opts.ParsedFiles)
	ktFiles := resolveFIRTargetFiles(summary, opts.ParsedFiles)
	result, err := opts.Checker.Check(ktFiles, nil, nil, active.Names)
	subTracker.End()

	if err != nil {
		if opts.Verbose && opts.VerboseOut != nil {
			fmt.Fprintf(opts.VerboseOut, "verbose: FIR checker error: %v\n", err)
		}
		return base
	}
	merged := firchecks.MergeFindings(base, result.Findings)
	if opts.Verbose && opts.VerboseOut != nil {
		stats := firchecks.Stats()
		fmt.Fprintf(opts.VerboseOut,
			"verbose: FIR checker in %v (%d findings, %d/%d files, cache hits=%d misses=%d)\n",
			time.Since(start).Round(time.Millisecond), len(result.Findings),
			summary.MarkedFiles, summary.TotalFiles, stats.Hits, stats.Misses)
	}
	return merged
}
