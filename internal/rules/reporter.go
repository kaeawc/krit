package rules

import (
	"os"
	"sync/atomic"

	"github.com/kaeawc/krit/internal/diag"
)

var reporterRef atomic.Pointer[diag.Reporter]

func init() {
	reporterRef.Store(&diag.Reporter{Warning: os.Stderr})
}

// SetReporter installs r as the package-level Reporter for diagnostic
// output (rule-panic stack traces, malformed-config warnings) emitted by
// the rules package. Passing nil restores a default warnings-only Reporter.
func SetReporter(r *diag.Reporter) {
	if r == nil {
		reporterRef.Store(&diag.Reporter{Warning: os.Stderr})
		return
	}
	reporterRef.Store(r)
}

func reporter() *diag.Reporter { return reporterRef.Load() }
