//go:build windows

package fsutil

// syncDir is a no-op on Windows: directory handles cannot be fsynced via the
// standard os.File.Sync path (FlushFileBuffers rejects directory handles with
// ERROR_ACCESS_DENIED), and NTFS rename durability follows different semantics
// from POSIX, so no parent-directory flush is needed here.
func syncDir(string) {}
