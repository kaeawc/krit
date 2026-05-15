package daemon

// RepoLock implements single-instance enforcement per repo via flock(2)
// on ${repoDir}/.krit/daemon.lock. The kernel releases the fd on process
// exit, so SIGKILL recovery needs no manual cleanup.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const (
	lockFileName = ".krit/daemon.lock"
	pidFileName  = ".krit/daemon.pid"
)

// ErrAlreadyHeld is returned by AcquireRepoLock when another process
// already holds the repo's daemon lock.
var ErrAlreadyHeld = errors.New("daemon lock already held")

// RepoLock holds an exclusive flock(2) on the repo's daemon.lock file.
// The lock is released by the kernel when the fd closes — either via
// Close or on process exit.
type RepoLock struct {
	f       *os.File
	pidPath string
}

// AcquireRepoLock takes an exclusive non-blocking flock on
// ${repoDir}/.krit/daemon.lock and writes the current PID to
// ${repoDir}/.krit/daemon.pid. Returns ErrAlreadyHeld if another
// process holds the lock; callers can then read the PID file to
// report who owns it.
func AcquireRepoLock(repoDir string) (*RepoLock, error) {
	dir := filepath.Join(repoDir, ".krit")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("daemon: prepare lock dir: %w", err)
	}

	lockPath := filepath.Join(repoDir, lockFileName)
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("daemon: open lock file: %w", err)
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrAlreadyHeld
		}
		return nil, fmt.Errorf("daemon: flock: %w", err)
	}

	pidPath := filepath.Join(repoDir, pidFileName)
	if err := writePIDFile(pidPath, os.Getpid()); err != nil {
		_ = f.Close()
		return nil, err
	}

	return &RepoLock{f: f, pidPath: pidPath}, nil
}

// Close closes the underlying fd, releasing the flock as a kernel
// side-effect, and removes the PID file. Safe to call multiple times.
func (l *RepoLock) Close() error {
	if l == nil || l.f == nil {
		return nil
	}
	err := l.f.Close()
	l.f = nil
	if l.pidPath != "" {
		_ = os.Remove(l.pidPath)
	}
	return err
}

// ReadPIDFile returns the PID recorded in ${repoDir}/.krit/daemon.pid,
// or 0 if the file is missing or malformed. Used by callers on
// ErrAlreadyHeld to report which process holds the lock.
func ReadPIDFile(repoDir string) int {
	data, err := os.ReadFile(filepath.Join(repoDir, pidFileName))
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return 0
	}
	return pid
}

// writePIDFile writes pid atomically: write to a temp file in the same
// directory, then rename over the target. Same-directory rename is
// atomic on POSIX, so concurrent readers never see a truncated file.
func writePIDFile(path string, pid int) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".daemon.pid.*")
	if err != nil {
		return fmt.Errorf("daemon: create pid temp: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := fmt.Fprintf(tmp, "%d\n", pid); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("daemon: write pid temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("daemon: close pid temp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("daemon: rename pid file: %w", err)
	}
	return nil
}
