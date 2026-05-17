package serve

// watcherBackend abstracts the OS-level filesystem-watching primitive
// the daemon uses. The default backend wraps fsnotify (inotify on
// Linux, kqueue on macOS, ReadDirectoryChangesW on Windows); a Linux-
// only fanotify backend (filesystem-wide marks + FID resolution) is
// available behind a build tag and selected via --watch-backend.
//
// All backends emit the same backendEvent stream. Path classification
// and debouncing live in fileWatcher; backends only normalize
// platform events into a uniform shape.
type watcherBackend interface {
	// Add registers a watch on path. For directory-based backends
	// (fsnotify) this watches just that directory and is called
	// repeatedly during the recursive descent. For filesystem-wide
	// backends (fanotify) the first Add marks the containing
	// filesystem and subsequent calls are no-ops — path filtering
	// happens in the event loop.
	Add(path string) error
	// Close releases the backend's resources. After Close, both
	// Events and Errors are eventually closed by the backend.
	Close() error
	// Events streams normalized change notifications. Backends should
	// drop or coalesce events they consider noise (e.g. fanotify
	// FAN_OPEN) before forwarding.
	Events() <-chan backendEvent
	// Errors streams non-fatal backend errors. The watcher logs them
	// via diag.Reporter but does not terminate on receipt.
	Errors() <-chan error
}

// backendOp is a bitmask of change kinds a backend observed. The set
// is the union of what fsnotify and fanotify can report; backends
// translate native ops into these values. Chmod is included for
// fidelity with fsnotify but the watcher drops Chmod-only events as
// documented in fileWatcher.handle.
type backendOp uint32

const (
	opCreate backendOp = 1 << iota
	opWrite
	opRemove
	opRename
	opChmod
)

// backendEvent is a single normalized change notification. Path is
// absolute and cleaned by the backend.
type backendEvent struct {
	Path string
	Op   backendOp
}

// watcherBackendKind selects which backend to construct.
//
//   - kindAuto (default): try fanotify on Linux and fall back to
//     fsnotify silently when caps are missing. The right choice for
//     almost every user — power users who set up CAP_SYS_ADMIN +
//     CAP_DAC_READ_SEARCH (see docs/perf.md) transparently get the
//     fanotify path; everyone else stays on fsnotify with no warning.
//   - kindFsnotify: force fsnotify. Skips the fanotify probe entirely.
//   - kindFanotify: force fanotify. The dispatcher still falls back if
//     init fails, but logs a Warn so misconfigured caps are visible.
type watcherBackendKind int

const (
	kindAuto watcherBackendKind = iota
	kindFsnotify
	kindFanotify
)

// String renders the kind for diagnostics and the --watch-backend
// flag's help text.
func (k watcherBackendKind) String() string {
	switch k {
	case kindAuto:
		return "auto"
	case kindFanotify:
		return "fanotify"
	default:
		return "fsnotify"
	}
}

// parseWatcherBackendKind maps the CLI flag value to a kind. Unknown
// strings collapse to kindAuto so a typo in startup args doesn't fail
// the daemon — the warning is logged at construction time.
func parseWatcherBackendKind(s string) (watcherBackendKind, bool) {
	switch s {
	case "", "auto":
		return kindAuto, true
	case "fsnotify":
		return kindFsnotify, true
	case "fanotify":
		return kindFanotify, true
	default:
		return kindAuto, false
	}
}
