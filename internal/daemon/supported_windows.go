//go:build windows

package daemon

// Supported reports whether daemon mode (serve, auto-start, delegation) can
// run on this platform. Windows lacks the peer-credential checks the daemon
// relies on, so callers must fall through to in-process analysis.
func Supported() bool { return false }
