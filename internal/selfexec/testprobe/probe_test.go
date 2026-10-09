package testprobe

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/selfexec"
)

// This package has no self-exec paths; the parent test runs its compiled
// binary with and without test flags to check custom output names safely.
func TestSelfExecProbe(t *testing.T) {
	if !selfexec.IsTestBinary(os.Args[0]) {
		t.Errorf("IsTestBinary(%q) = false, want true", os.Args[0])
	}
	if exe, err := selfexec.Executable(); !errors.Is(err, selfexec.ErrTestBinary) {
		t.Errorf("Executable() = %q, %v; want ErrTestBinary", exe, err)
	}
	// A symlink with an ordinary name, as an explicit spawn binary would
	// pass, must still be recognized as the running custom-named binary.
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "krit")
	if err := os.Symlink(exe, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if !selfexec.IsTestBinary(link) {
		t.Errorf("IsTestBinary(symlink %q -> %q) = false, want true", link, exe)
	}
}
