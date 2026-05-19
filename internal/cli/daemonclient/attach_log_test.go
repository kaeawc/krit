package daemonclient

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAttachSpawnLogWiresStdoutAndStderr(t *testing.T) {
	tmp := t.TempDir()
	logPath := filepath.Join(tmp, "daemon.log")
	cmd := exec.Command("nonexistent")
	f, err := attachSpawnLog(cmd, logPath)
	if err != nil {
		t.Fatalf("attachSpawnLog: %v", err)
	}
	if f == nil {
		t.Fatal("non-empty path: expected non-nil log file")
	}
	if cmd.Stdout != f || cmd.Stderr != f {
		t.Errorf("attachSpawnLog must wire stdout/stderr to the opened file")
	}
	if err := f.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	// After the documented post-Start Close, the parent's handle must
	// be unusable — proves we are not silently leaking the fd.
	if _, err := f.Write([]byte("x")); !errors.Is(err, os.ErrClosed) {
		t.Errorf("expected os.ErrClosed after Close, got %v", err)
	}
}

func TestAttachSpawnLogEmptyPathClearsStdio(t *testing.T) {
	cmd := exec.Command("nonexistent")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	f, err := attachSpawnLog(cmd, "")
	if err != nil || f != nil {
		t.Fatalf("got (f=%v, err=%v); want (nil, nil)", f, err)
	}
	if cmd.Stdout != nil || cmd.Stderr != nil {
		t.Error("empty path must null out stdio so the child inherits no parent file descriptors")
	}
}

func TestAttachSpawnLogReportsOpenError(t *testing.T) {
	cmd := exec.Command("nonexistent")
	// A path under an existing file is guaranteed to fail to open.
	tmp := t.TempDir()
	parent := filepath.Join(tmp, "file")
	if err := os.WriteFile(parent, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(parent, "child.log")
	if _, err := attachSpawnLog(cmd, bad); err == nil {
		t.Errorf("expected open error for path under a regular file %q", bad)
	}
}
