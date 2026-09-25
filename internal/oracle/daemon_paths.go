package oracle

import (
	"os"
	"path/filepath"

	"github.com/kaeawc/krit/internal/fsutil"
)

// Persistent JVM daemons are shared machine-wide through the user-level
// registry (~/.krit/cache/daemons), so one daemon can serve krit invocations
// started from different working directories. A relative path means nothing
// to such a daemon: before paths were absolutized, two projects both scanned
// as `krit .` produced the same relative source roots, hashed to the same
// registry key, and the second project was analyzed against the first
// project's sources. Every path that enters a daemon registry key, a daemon
// command line, or a daemon request is therefore absolute, and daemons run
// in DaemonWorkDir so nothing can depend on the caller's working directory.

// AbsolutePath resolves a relative path against the current working
// directory. Absolute and empty paths are returned unchanged, so a caller's
// absolute spelling (a symlinked root, say) is never rewritten.
func AbsolutePath(path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

// AbsolutePaths applies AbsolutePath to each entry. It returns a new slice
// (nil for nil input) and never modifies paths.
func AbsolutePaths(paths []string) []string {
	if paths == nil {
		return nil
	}
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = AbsolutePath(p)
	}
	return out
}

// PathSpelling maps the absolute paths sent to a daemon back to the caller's
// spelling of them. Daemons echo request paths in their responses, and Go
// indexes those responses by the paths it asked about.
type PathSpelling struct {
	caller map[string]string
}

// AbsoluteRequestPaths absolutizes paths for a daemon request and returns
// the mapping back to the caller's spelling.
func AbsoluteRequestPaths(paths []string) ([]string, PathSpelling) {
	out := make([]string, len(paths))
	var spelling PathSpelling
	for i, p := range paths {
		abs := AbsolutePath(p)
		out[i] = abs
		if abs == p {
			continue
		}
		if spelling.caller == nil {
			spelling.caller = make(map[string]string)
		}
		if _, ok := spelling.caller[abs]; !ok {
			spelling.caller[abs] = p
		}
	}
	return out, spelling
}

// Caller returns the caller's spelling of a path the daemon reported.
func (s PathSpelling) Caller(path string) string {
	if orig, ok := s.caller[path]; ok {
		return orig
	}
	return path
}

// CallerKeys returns m re-keyed by the caller's spelling (m itself when no
// request path was rewritten).
func CallerKeys[V any](s PathSpelling, m map[string]V) map[string]V {
	if len(s.caller) == 0 || len(m) == 0 {
		return m
	}
	out := make(map[string]V, len(m))
	for k, v := range m {
		out[s.Caller(k)] = v
	}
	return out
}

// DaemonWorkDir is the working directory persistent daemons are started in:
// a fixed per-user directory, so that no daemon behavior depends on the
// directory of whichever krit invocation happened to spawn it.
func DaemonWorkDir() string {
	if dir, err := fsutil.UserKritDir(); err == nil {
		return dir
	}
	return os.TempDir()
}
