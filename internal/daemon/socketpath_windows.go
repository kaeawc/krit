//go:build windows

package daemon

import "io/fs"

// ownedByCurrentUser always reports true on Windows: files carry no Unix
// uid, and the pointer file lives in the user's private profile directory.
func ownedByCurrentUser(fs.FileInfo) bool { return true }
