package testprobe

import (
	"errors"
	"os"
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
}
