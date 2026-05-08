package scan

import (
	"github.com/kaeawc/krit/internal/diag"
	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// installDiagnosticReporter creates a stderr-backed diagnostic reporter
// and propagates it to every package that pulls warnings/verbose lines
// via package-level SetReporter (oracle, firchecks, rules, rules/v2).
// Returns the reporter so the caller can also hand it to pipeline
// phases that take a Reporter field directly.
//
// Warnings always go to stderr; verbose output only when verbose=true.
// The fan-out exists because many entry points in those packages
// pre-date structured Reporter plumbing.
func installDiagnosticReporter(verbose bool) *diag.Reporter {
	r := diag.NewStderr(verbose)
	oracle.SetReporter(r)
	firchecks.SetReporter(r)
	rules.SetReporter(r)
	api.SetReporter(r)
	return r
}
