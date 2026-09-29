// Package selfexec resolves the running krit binary for code paths that
// re-launch krit as a subprocess (daemon autostart, score, metrics,
// --delta, experiment matrices, snapshot simulate).
//
// Under `go test`, os.Executable() is the package's test binary, not
// krit. A test binary ignores positional arguments such as `serve`, so
// re-launching it reruns the whole test suite, which reaches the same
// spawn site and launches another copy. Detached daemon spawns outlive
// their parent, so the chain grows until the process table fills.
// Executable refuses to hand out a test binary so every self-exec site
// fails closed instead.
package selfexec

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrTestBinary reports that the running executable is a Go test
// binary and must not be re-launched as krit.
var ErrTestBinary = errors.New("selfexec: running executable is a Go test binary, not krit")

// Executable returns the path of the running krit binary, or an error
// wrapping ErrTestBinary when running inside a Go test binary.
func Executable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if IsTestBinary(exe) {
		return "", fmt.Errorf("%w: %s", ErrTestBinary, exe)
	}
	return exe, nil
}

// IsTestBinary reports whether path names a Go test binary. `go test`
// always names them `<pkg>.test` (`<pkg>.test.exe` on Windows).
func IsTestBinary(path string) bool {
	base := strings.TrimSuffix(filepath.Base(path), ".exe")
	return strings.HasSuffix(base, ".test")
}
