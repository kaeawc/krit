//go:build windows

package daemoncmd

import (
	"os/exec"
	"testing"
)

func TestDetachProcessUnsupportedOnWindows(t *testing.T) {
	err := detachProcess(exec.Command("krit-daemon"))
	if err == nil || err.Error() != "daemon mode is not supported on Windows" {
		t.Fatalf("detachProcess error = %v, want unsupported-on-Windows error", err)
	}
}
