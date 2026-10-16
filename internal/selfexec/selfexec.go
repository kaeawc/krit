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
	"testing"
)

// ErrTestBinary reports that the running executable is a Go test
// binary and must not be re-launched as krit.
var ErrTestBinary = errors.New("selfexec: running executable is a Go test binary, not krit")

// Go's linker marks test binaries even when -o gives them a custom name.
// The test status is kept apart from the resolved path so a failed
// executable lookup still leaves the os.Args[0] check in place.
var currentTestBinary = detectTestBinary(testing.Testing(), os.Executable)

// testBinary describes the running process when it is a Go test binary.
type testBinary struct {
	running bool        // the process is a Go test binary
	path    string      // os.Executable result; "" when the lookup failed
	info    os.FileInfo // identity of the running binary, when it could be stat'ed
}

func detectTestBinary(isTest bool, lookup func() (string, error)) testBinary {
	if !isTest {
		return testBinary{}
	}
	tb := testBinary{running: true}
	if exe, err := lookup(); err == nil {
		tb.path = exe
		tb.info, _ = os.Stat(exe)
	}
	if tb.info == nil {
		// Without a resolved executable, os.Args[0] (relative to the
		// startup directory) is the best identity available.
		tb.info, _ = os.Stat(os.Args[0])
	}
	return tb
}

// matches reports whether path names the running test binary, either by
// spelling or, for symlinks and other spellings, by file identity.
func (tb testBinary) matches(path string) bool {
	if !tb.running {
		return false
	}
	if path == os.Args[0] || tb.path != "" && path == tb.path {
		return true
	}
	if tb.info == nil {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && os.SameFile(info, tb.info)
}

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

// IsTestBinary reports whether path names a Go test binary. It recognizes
// the conventional .test suffix and the currently running test binary,
// including a custom name supplied with go test -c -o and any symlink or
// other path that resolves to the same file.
func IsTestBinary(path string) bool {
	base := strings.TrimSuffix(filepath.Base(path), ".exe")
	return strings.HasSuffix(base, ".test") || currentTestBinary.matches(path)
}
