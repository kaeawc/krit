package daemoncmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/kaeawc/krit/internal/fsutil"
)

const PIDFileName = ".krit/daemon.pid"

func pidFilePath(repoDir string) string {
	return filepath.Join(repoDir, PIDFileName)
}

func writePIDFile(path string, pid int) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("daemoncmd: prepare pid dir: %w", err)
	}
	if err := fsutil.WriteFileAtomic(path, []byte(strconv.Itoa(pid)+"\n"), 0o644); err != nil {
		return fmt.Errorf("daemoncmd: write pid: %w", err)
	}
	return nil
}

func readPIDFile(path string) (int, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	s := strings.TrimSpace(string(body))
	if s == "" {
		return 0, errors.New("daemoncmd: pid file is empty")
	}
	pid, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("daemoncmd: parse pid file: %w", err)
	}
	if pid <= 0 {
		return 0, fmt.Errorf("daemoncmd: invalid pid %d", pid)
	}
	return pid, nil
}

func removePIDFile(path string) error {
	err := os.Remove(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// processAlive probes whether pid is live. signal-0 returns nil for a
// process we can signal, EPERM when the process exists but is owned by
// another user (treat as alive), and ESRCH for "no such process".
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = p.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	return errors.Is(err, syscall.EPERM)
}
