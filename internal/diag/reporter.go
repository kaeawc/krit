// Package diag provides a Reporter type for analyzer libraries to emit
// verbose progress and warning messages without coupling to os.Stderr.
//
// Library code in internal/ should never call fmt.Fprintf(os.Stderr, ...)
// directly. Instead, accept a *Reporter (nil is valid and means silent)
// and call Verbosef / Warnf. cmd/ packages construct a Reporter that
// routes to os.Stderr; tests can substitute a buffer.
package diag

import (
	"fmt"
	"io"
	"os"
)

// Reporter routes verbose progress and warning lines to writers chosen by
// the caller. A nil *Reporter is valid: every method is a no-op. A
// Reporter with a nil writer for one stream silences only that stream.
type Reporter struct {
	Verbose io.Writer
	Warning io.Writer
}

// NewStderr returns a Reporter that writes warnings to os.Stderr
// unconditionally, and verbose progress to os.Stderr only when verbose
// is true. This matches the historical CLI behaviour.
func NewStderr(verbose bool) *Reporter {
	r := &Reporter{Warning: os.Stderr}
	if verbose {
		r.Verbose = os.Stderr
	}
	return r
}

// Verbosef writes format/args to the verbose stream when configured.
// The format string is passed through to fmt.Fprintf verbatim, so callers
// keep ownership of any "verbose: " prefix and trailing newline. Nil
// receiver and nil Verbose writer are no-ops.
func (r *Reporter) Verbosef(format string, args ...any) {
	if r == nil || r.Verbose == nil {
		return
	}
	fmt.Fprintf(r.Verbose, format, args...)
}

// Warnf writes format/args to the warning stream when configured. Like
// Verbosef, the format string passes through verbatim. Nil receiver and
// nil Warning writer are no-ops.
func (r *Reporter) Warnf(format string, args ...any) {
	if r == nil || r.Warning == nil {
		return
	}
	fmt.Fprintf(r.Warning, format, args...)
}

// VerboseEnabled reports whether Verbosef will write anything.
func (r *Reporter) VerboseEnabled() bool {
	return r != nil && r.Verbose != nil
}
