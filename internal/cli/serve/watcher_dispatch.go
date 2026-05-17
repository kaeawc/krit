package serve

import "github.com/kaeawc/krit/internal/diag"

// newWatcherBackend constructs the watcherBackend named by kind and
// returns the kind that was actually resolved (callers log this so
// users can see whether auto-detect picked fanotify or fsnotify).
//
// Resolution rules:
//
//   - kindAuto: probe fanotify first. On Linux with CAP_SYS_ADMIN +
//     CAP_DAC_READ_SEARCH this succeeds and we get the kernel-side-
//     filtered backend. Everywhere else (Linux without caps, darwin,
//     windows) fanotify init returns errFanotifyUnsupported and we
//     fall back to fsnotify silently — auto is the default for
//     most users so we don't want a per-startup warning.
//   - kindFsnotify: skip the fanotify probe entirely.
//   - kindFanotify: try fanotify; if init fails, warn loudly through
//     the reporter and fall back to fsnotify. The user explicitly
//     asked for fanotify, so a quiet fallback would hide a real
//     misconfiguration.
//
// Defaulting to a working backend on every failure path keeps the
// daemon safe against startup-arg typos, missing caps, and platform
// mismatch all at once.
func newWatcherBackend(kind watcherBackendKind, root string, reporter *diag.Reporter) (watcherBackend, watcherBackendKind, error) {
	if kind == kindAuto || kind == kindFanotify {
		b, err := newFanotifyBackend(root)
		if err == nil {
			return b, kindFanotify, nil
		}
		if kind == kindFanotify && reporter != nil {
			reporter.Warnf("watch backend fanotify unavailable (%v); falling back to fsnotify", err)
		}
	}
	b, err := newFsnotifyBackend()
	if err != nil {
		return nil, kindFsnotify, err
	}
	return b, kindFsnotify, nil
}
