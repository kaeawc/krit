//go:build !linux

package serve

import "errors"

// errFanotifyUnsupported is returned on non-Linux platforms when the
// caller requests the fanotify backend. The dispatcher logs and
// falls back to fsnotify, so this is a soft failure.
var errFanotifyUnsupported = errors.New("fanotify backend is Linux-only")

// newFanotifyBackend stubs the linux-only implementation. On darwin/
// windows/etc this always errors so the dispatcher falls back to
// fsnotify; the error is emitted via diag.Reporter.Warnf at startup.
func newFanotifyBackend(string) (watcherBackend, error) {
	return nil, errFanotifyUnsupported
}
