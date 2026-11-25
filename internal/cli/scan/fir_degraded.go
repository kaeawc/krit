package scan

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/kaeawc/krit/internal/cli/clishared"
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/output"
	"github.com/kaeawc/krit/internal/rules/api"
)

// firFallbackRules lists the selected rules that have a FIR twin and so run
// their Go implementation when the FIR pass is unavailable. A non-nil cfg also
// drops rules the config turns off, for selections made before the config is
// applied to the registry; enabled names override the config.
func firFallbackRules(selected []*api.Rule, cfg *config.Config, enabled map[string]bool) []string {
	var ids []string
	for _, r := range selected {
		if !firchecks.IsKnownFIRRule(r.ID) {
			continue
		}
		if cfg != nil && !enabled[r.ID] {
			if v := cfg.IsRuleActive(r.Category, r.ID); v != nil && !*v {
				continue
			}
		}
		ids = append(ids, r.ID)
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}

// preflightFallbackRules applies the scan's rule-selection flags before the
// runner exists, so the preflight warning names only rules the scan runs.
func preflightFallbackRules(f *scanFlags, cfg *config.Config) []string {
	selected, _, _ := selectScanRules(f, cfg)
	return firFallbackRules(selected, cfg, clishared.ParseRuleNameSetCSV(*f.EnableRules))
}

// degradedMessage is the stderr text shared by the warning and the
// --fail-on-degraded error.
func degradedMessage(d *output.FirDegraded) string {
	return fmt.Sprintf("FIR rules were skipped (%s); %d default-on rules fell back to their Go implementation: %s",
		d.Reason, len(d.FallbackRules), strings.Join(d.FallbackRules, ", "))
}

// handleFirPreflightFailure turns a failed FIR preflight into a Go-only scan
// with a warning, or into an error under --fail-on-degraded.
func handleFirPreflightFailure(f *scanFlags, cfg *config.Config, cause error, warn io.Writer) error {
	d := &output.FirDegraded{Reason: cause.Error(), FallbackRules: preflightFallbackRules(f, cfg)}
	if f.FailOnDegraded != nil && *f.FailOnDegraded {
		return fmt.Errorf("%s", degradedMessage(d))
	}
	fmt.Fprintf(warn, "warning: %s\n", degradedMessage(d))
	*f.NoFir = true
	f.firDegraded = d
	return nil
}

func (r *runner) markFirDegraded(err error) {
	r.firDegraded = CheckerFailureDegraded(err, r.activeRules)
}

// CheckerFailureDegraded describes a FIR checker that failed mid-scan, listing
// the FIR twins among the active rules as Go fallbacks.
func CheckerFailureDegraded(err error, active []*api.Rule) *output.FirDegraded {
	return &output.FirDegraded{
		Reason:        fmt.Sprintf("checker failed: %v", err),
		FallbackRules: firFallbackRules(active, nil, nil),
	}
}

// currentFirDegraded returns the degraded state from preflight or mid-scan.
func (r *runner) currentFirDegraded() *output.FirDegraded {
	if r.firDegraded != nil {
		return r.firDegraded
	}
	return r.f.firDegraded
}

// reportFirCoverage prints the one-line FIR coverage summary when the pass
// gated files (unless -q), and enforces --min-fir-coverage: it returns 2 when
// FIR analyzed less than the required share, or did not run at all
// (degraded, or no coverage recorded although FIR was on), else 0.
func reportFirCoverage(w io.Writer, f *scanFlags, cov *output.FirCoverage, degraded bool) int {
	if cov != nil && (f.Quiet == nil || !*f.Quiet) {
		if line := output.FirCoverageLine(*cov); line != "" {
			fmt.Fprintf(w, "info: %s\n", line)
		}
	}
	if f.MinFirCoverage == nil || *f.MinFirCoverage <= 0 {
		return 0
	}
	min := *f.MinFirCoverage
	switch {
	case degraded || (cov == nil && firRequested(f)):
		fmt.Fprintf(w, "error: --min-fir-coverage=%g: the FIR pass did not run\n", min)
		return 2
	case cov == nil:
		return 0
	case cov.Ratio() < min:
		fmt.Fprintf(w, "error: --min-fir-coverage=%g: FIR analyzed %.1f%% of the files it checked (%d of %d; %d gated)\n",
			min, 100*cov.Ratio(), cov.Authoritative, cov.Authoritative+cov.Gated, cov.Gated)
		return 2
	}
	return 0
}

// firRequested reports whether the scan asked for the FIR pass.
func firRequested(f *scanFlags) bool {
	return f.Fir != nil && *f.Fir && (f.NoFir == nil || !*f.NoFir)
}
