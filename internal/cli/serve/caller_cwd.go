package serve

import (
	"fmt"
	"os"
	"path/filepath"
)

// enterCallerCwd makes the daemon process resolve relative paths the way
// the calling CLI does, by moving into the caller's working directory.
//
// A daemon is found by repo directory, not by working directory, so it
// serves invocations from any directory under its root, and it starts in
// whatever directory the invocation that spawned it happened to be in.
// The pipeline is working-directory-relative throughout: scan paths,
// baseline and custom-rule-jar paths, the per-file cache keys and
// filepath.Abs calls behind them all read the process working
// directory, and findings are reported in the spelling the walk
// produces from the scan path. Running the scan from the caller's
// directory is what keeps every one of those identical to the
// in-process run the CLI would otherwise do; translating paths at the
// wire instead would have to re-spell each of them.
//
// State the daemon keeps resident between scans is keyed by those
// relative spellings ("src/A.kt"), so it is dropped when the directory
// changes: the same spelling names a different file from there.
//
// cwd is empty for callers that predate the field or could not read
// their working directory; the daemon then stays where it is.
//
// The working directory is process-wide. Callers must hold analyzeMu,
// as must every verb that resolves a caller-relative path.
func (s *daemonState) enterCallerCwd(cwd string) error {
	target := "."
	if cwd != "" {
		if !filepath.IsAbs(cwd) {
			return fmt.Errorf("caller working directory %q is not absolute", cwd)
		}
		target = cwd
	}
	info, err := os.Stat(target)
	if err != nil {
		if cwd == "" {
			// The daemon's own directory is gone. Nothing to enter;
			// relative paths fail on their own with a clear error.
			return nil
		}
		return fmt.Errorf("caller working directory: %w", err)
	}
	if s.analysisCwd != nil && !os.SameFile(s.analysisCwd, info) {
		// Before leaving: queued saves resolve their paths against
		// the directory they were enqueued in.
		s.dropCwdScopedState()
	}
	if cwd != "" {
		if err := chdirAs(cwd, info); err != nil {
			return fmt.Errorf("caller working directory: %w", err)
		}
	}
	s.analysisCwd = info
	return nil
}

// chdirAs moves the process into dir and records dir as its spelling.
// os.Getwd, and filepath.Abs through it, prefers $PWD when it names the
// working directory; the CLI computed its own absolute paths (the
// baseline base path, for one) under the caller's spelling, which may
// run through a symlink, so absolute paths derived here must agree.
func chdirAs(dir string, info os.FileInfo) error {
	if current, err := os.Stat("."); err != nil || !os.SameFile(current, info) {
		if err := os.Chdir(dir); err != nil {
			return err
		}
	}
	return os.Setenv("PWD", dir)
}

// dropCwdScopedState discards everything resident that was built from
// paths spelled relative to the previous working directory, and returns
// the daemon to cold so the next scan validates against disk instead of
// trusting the watcher's account of a different file set.
//
// On-disk caches survive: they are shared with in-process runs from any
// directory already, and the findings-bundle manifest is keyed by scan
// path spelling as well as location (scanner.FindingsBundleManifestKey).
func (s *daemonState) dropCwdScopedState() {
	s.workspace.DrainBackgroundSaves()
	s.workspace.InvalidateAll()
	// Oracle handles map JVM responses back to the source-root
	// spellings they were connected with.
	s.closeOracleDaemons()
	s.coldDone.Store(false)
}
