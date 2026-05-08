package firchecks

import (
	"os"
	"sync/atomic"

	"github.com/kaeawc/krit/internal/diag"
)

var reporterRef atomic.Pointer[diag.Reporter]

func init() {
	reporterRef.Store(&diag.Reporter{Warning: os.Stderr})
}

// SetReporter installs r as the package-level Reporter for firchecks.
// Passing nil restores a default warnings-only Reporter.
func SetReporter(r *diag.Reporter) {
	if r == nil {
		reporterRef.Store(&diag.Reporter{Warning: os.Stderr})
		return
	}
	reporterRef.Store(r)
}

func reporter() *diag.Reporter { return reporterRef.Load() }
