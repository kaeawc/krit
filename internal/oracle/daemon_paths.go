package oracle

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

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
// indexes those responses by the paths it asked about. Files the daemon
// reports without being asked about them (walked from a source root) map
// back through the caller's spelling of that root.
type PathSpelling struct {
	caller map[string]string
	// dirs maps absolute source roots to the caller's relative spelling,
	// longest root first.
	dirs []dirSpelling
}

type dirSpelling struct {
	abs, caller string
}

// WithDirs adds the caller's source roots: a reported path under one of
// them that is not an exact request path is re-spelled under the caller's
// spelling of the root, the way Go's own walk of that root spells it.
func (s PathSpelling) WithDirs(dirs []string) PathSpelling {
	for _, dir := range dirs {
		abs := AbsolutePath(dir)
		if abs == dir || dir == "" {
			continue
		}
		s.dirs = append(s.dirs, dirSpelling{abs: filepath.Clean(abs), caller: dir})
	}
	sort.SliceStable(s.dirs, func(i, j int) bool { return len(s.dirs[i].abs) > len(s.dirs[j].abs) })
	return s
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
	for _, d := range s.dirs {
		if rest, ok := strings.CutPrefix(path, d.abs+string(filepath.Separator)); ok && rest != "" {
			return filepath.Join(d.caller, rest)
		}
	}
	return path
}

func (s PathSpelling) identity() bool { return len(s.caller) == 0 && len(s.dirs) == 0 }

// CallerPaths returns paths in the caller's spelling (paths itself when
// nothing was rewritten).
func (s PathSpelling) CallerPaths(paths []string) []string {
	if s.identity() || len(paths) == 0 {
		return paths
	}
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = s.Caller(p)
	}
	return out
}

// CallerKeys returns m re-keyed by the caller's spelling (m itself when no
// request path was rewritten).
func CallerKeys[V any](s PathSpelling, m map[string]V) map[string]V {
	if s.identity() || len(m) == 0 {
		return m
	}
	out := make(map[string]V, len(m))
	for k, v := range m {
		out[s.Caller(k)] = v
	}
	return out
}

// CallerData re-keys a daemon's oracle facts by the caller's spelling.
func (s PathSpelling) CallerData(d *Data) {
	if d != nil {
		d.Files = CallerKeys(s, d.Files)
	}
}

// CallerCacheDeps re-keys a daemon's dependency closure by the caller's
// spelling: the per-file entries, their dependency edges, and crash markers.
func (s PathSpelling) CallerCacheDeps(c *CacheDepsFile) {
	if c == nil || s.identity() {
		return
	}
	c.Files = CallerKeys(s, c.Files)
	for _, entry := range c.Files {
		if entry == nil {
			continue
		}
		entry.DepPaths = s.CallerPaths(entry.DepPaths)
		entry.PropagatingDepPaths = s.CallerPaths(entry.PropagatingDepPaths)
	}
	c.Crashed = CallerKeys(s, c.Crashed)
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
