package daemon

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestAcquireRepoLock_Success(t *testing.T) {
	dir := t.TempDir()
	lock, err := AcquireRepoLock(dir)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	t.Cleanup(func() { _ = lock.Close() })

	if pid := ReadPIDFile(dir); pid != os.Getpid() {
		t.Fatalf("pid file = %d, want %d", pid, os.Getpid())
	}
	if _, err := os.Stat(filepath.Join(dir, lockFileName)); err != nil {
		t.Fatalf("lock file should exist: %v", err)
	}
}

func TestAcquireRepoLock_ConflictReturnsErrAlreadyHeld(t *testing.T) {
	dir := t.TempDir()
	first, err := AcquireRepoLock(dir)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	t.Cleanup(func() { _ = first.Close() })

	second, err := AcquireRepoLock(dir)
	if !errors.Is(err, ErrAlreadyHeld) {
		t.Fatalf("second acquire err = %v, want ErrAlreadyHeld", err)
	}
	if second != nil {
		t.Fatalf("second acquire returned non-nil lock on conflict")
	}
	if pid := ReadPIDFile(dir); pid != os.Getpid() {
		t.Fatalf("pid file lost on conflict: %d", pid)
	}
}

func TestAcquireRepoLock_ReacquireAfterClose(t *testing.T) {
	dir := t.TempDir()
	first, err := AcquireRepoLock(dir)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	second, err := AcquireRepoLock(dir)
	if err != nil {
		t.Fatalf("second acquire after close: %v", err)
	}
	_ = second.Close()
}

func TestAcquireRepoLock_StalePIDFileOverwritten(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".krit"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Write a stale PID — acquisition should succeed and overwrite.
	if err := os.WriteFile(filepath.Join(dir, pidFileName), []byte("999999\n"), 0o644); err != nil {
		t.Fatalf("seed pid: %v", err)
	}

	lock, err := AcquireRepoLock(dir)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	t.Cleanup(func() { _ = lock.Close() })

	if pid := ReadPIDFile(dir); pid != os.Getpid() {
		t.Fatalf("stale pid not replaced: %d", pid)
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	lock, err := AcquireRepoLock(dir)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

func TestReadPIDFile_MissingReturnsZero(t *testing.T) {
	dir := t.TempDir()
	if pid := ReadPIDFile(dir); pid != 0 {
		t.Fatalf("missing pid = %d, want 0", pid)
	}
}

func TestReadPIDFile_MalformedReturnsZero(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".krit"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, pidFileName), []byte("not a pid"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if pid := ReadPIDFile(dir); pid != 0 {
		t.Fatalf("malformed pid = %d, want 0", pid)
	}
}

// Sanity: after Close, the lock file still exists on disk (it's a
// long-lived sentinel) but a fresh AcquireRepoLock with a known PID
// succeeds.
func TestReadPIDFile_AfterRecycle(t *testing.T) {
	dir := t.TempDir()
	first, err := AcquireRepoLock(dir)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	_ = first.Close()

	second, err := AcquireRepoLock(dir)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })

	want := strconv.Itoa(os.Getpid())
	got := strconv.Itoa(ReadPIDFile(dir))
	if got != want {
		t.Fatalf("recycle pid = %s, want %s", got, want)
	}
}
