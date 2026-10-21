//go:build unix

package daemon

// Supported reports whether daemon mode (serve, auto-start, delegation) can
// run on this platform.
func Supported() bool { return true }
