package oracle

import (
	"os"
	"sync/atomic"

	"github.com/kaeawc/krit/internal/diag"
)

// reporterRef holds the package-level diagnostic Reporter used for all
// "verbose: ..." and "warning: ..." stderr lines emitted by oracle. The
// default Reporter writes warnings to os.Stderr unconditionally and
// silences verbose unless a verbose-enabled Reporter is installed by the
// caller (cmd/krit/main.go calls SetReporter at startup).
//
// The atomic.Pointer is used because oracle.Invoke and friends are called
// from goroutines; SetReporter is normally called once at startup, but
// using atomic.Pointer keeps the package safe under concurrent reads.
var reporterRef atomic.Pointer[diag.Reporter]

func init() {
	reporterRef.Store(&diag.Reporter{Warning: os.Stderr})
}

// SetReporter installs r as the package-level Reporter used for verbose
// and warning output. Passing nil restores a default warnings-only
// Reporter so library code never panics on a nil dereference.
func SetReporter(r *diag.Reporter) {
	if r == nil {
		reporterRef.Store(&diag.Reporter{Warning: os.Stderr})
		return
	}
	reporterRef.Store(r)
}

// reporter returns the current Reporter. Always non-nil — init() seeds it.
func reporter() *diag.Reporter {
	return reporterRef.Load()
}
