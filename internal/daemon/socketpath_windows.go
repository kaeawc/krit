//go:build windows

package daemon

import "io/fs"

// ownedByCurrentUser always reports false on Windows: files carry no Unix
// uid, so ownership cannot be verified, and daemon mode is unsupported there.
// Returning false keeps pointer-file reads and removals from trusting an
// unverified file.
func ownedByCurrentUser(fs.FileInfo) bool { return false }
