package scan

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/pprof"

	"github.com/kaeawc/krit/internal/rules"
)

// startCPUProfile opens path, enables rule profile labels, and starts a
// CPU profile. Returns the underlying file so the caller can stop+close
// the profile via stopCPUProfile after work completes.
//
// Returns (nil, nil) when path is empty (no profile requested). On any
// error the function reports to errOut and returns the error so the
// caller can propagate a non-zero exit code.
//
// Lifetime is managed explicitly (not via defer): scan.Run's exit path
// runs through os.Exit/return after the profile work is done, and defer
// won't fire through os.Exit.
func startCPUProfile(path string, errOut io.Writer) (*os.File, error) {
	if path == "" {
		return nil, nil
	}
	rules.SetRuleProfileLabels(true)
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(errOut, "error: could not create CPU profile: %v\n", err)
		return nil, err
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		fmt.Fprintf(errOut, "error: could not start CPU profile: %v\n", err)
		f.Close()
		return nil, err
	}
	return f, nil
}

// stopCPUProfile flushes and closes a CPU profile started via
// startCPUProfile. No-op when f is nil.
func stopCPUProfile(f *os.File) {
	if f == nil {
		return
	}
	pprof.StopCPUProfile()
	f.Close()
}

// writeMemProfile writes a heap profile to path. No-op when path is empty.
// Errors (create or write) are reported to errOut; this never aborts the
// caller — diagnostic profiling failures must not mask the real exit code.
func writeMemProfile(path string, errOut io.Writer) {
	if path == "" {
		return
	}
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(errOut, "error: could not create memory profile: %v\n", err)
		return
	}
	defer f.Close()
	runtime.GC()
	if err := pprof.WriteHeapProfile(f); err != nil {
		fmt.Fprintf(errOut, "error: could not write memory profile: %v\n", err)
	}
}
