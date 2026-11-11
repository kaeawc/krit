//go:build unix

package scan

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestAtomicOutputNewReportHonorsUmask(t *testing.T) {
	old := syscall.Umask(0o077)
	defer syscall.Umask(old)
	path := filepath.Join(t.TempDir(), "report.json")
	out := openAtomicOutput(path)
	if out == nil {
		t.Fatal("open returned nil")
	}
	if err := out.Finish(0); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600 under umask 077", info.Mode().Perm())
	}
}
