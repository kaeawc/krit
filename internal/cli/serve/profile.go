package serve

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"

	"github.com/kaeawc/krit/internal/rules"
)

// startDaemonCPUProfile opens path on the daemon's filesystem and
// starts a CPU profile that wraps the analyze-project call. Returns
// the underlying file (so the caller can stop+close via
// stopDaemonCPUProfile) and any non-fatal warnings to surface in
// AnalyzeProjectStats.ProfileWarnings — profiling failures must not
// abort the verb because the scan result is independently useful.
//
// Mirrors the CLI's startCPUProfile so the daemon-served path
// captures the same profile shape (rule profile labels armed) as the
// in-process path; the only difference is the profile reflects the
// daemon process, not the CLI shim, which is the documented
// semantics of --cpu-profile when --no-daemon is not set.
//
// Returns (nil, nil) when path is empty (no profile requested).
func startDaemonCPUProfile(path string) (*os.File, []string) {
	if path == "" {
		return nil, nil
	}
	rules.SetRuleProfileLabels(true)
	f, err := os.Create(path)
	if err != nil {
		return nil, []string{fmt.Sprintf("daemon cpu profile create %s: %v", path, err)}
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		_ = f.Close()
		return nil, []string{fmt.Sprintf("daemon cpu profile start %s: %v", path, err)}
	}
	return f, nil
}

// stopDaemonCPUProfile flushes and closes a CPU profile started via
// startDaemonCPUProfile. No-op when f is nil. Close errors are
// intentionally swallowed: the profile bytes are already on disk by
// the time pprof.StopCPUProfile returns; a bad close on the file
// handle would only obscure the more interesting analyze result.
func stopDaemonCPUProfile(f *os.File) {
	if f == nil {
		return
	}
	pprof.StopCPUProfile()
	_ = f.Close()
}

// writeDaemonMemProfile writes a heap profile to path. No-op when
// path is empty. Returns any warnings to surface in
// AnalyzeProjectStats.ProfileWarnings — failures must not abort the
// verb. Triggers runtime.GC first so the heap reflects post-run state
// rather than transient allocations from concurrent goroutines.
func writeDaemonMemProfile(path string) []string {
	if path == "" {
		return nil
	}
	f, err := os.Create(path)
	if err != nil {
		return []string{fmt.Sprintf("daemon mem profile create %s: %v", path, err)}
	}
	defer f.Close()
	runtime.GC()
	if err := pprof.WriteHeapProfile(f); err != nil {
		return []string{fmt.Sprintf("daemon mem profile write %s: %v", path, err)}
	}
	return nil
}
